package snapshot_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/internal/testtools"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
)

// straightSPVASM stores an OpConstant that has no OpName. The SPIR-V
// frontend makes it an unnamed IR constant, which no text backend declares
// (Rust naga writes only named constants), so its value must be written in
// place, as Rust's put_possibly_const_expression (MSL),
// write_possibly_const_expression (HLSL) and write_expr (GLSL) do for
// constant.name.is_none(). The writers printed the name the namer gave it
// (const_type_2, const_type_0_2, const_2_) and the text did not compile.
const straightSPVASM = `
               OpCapability Shader
               OpMemoryModel Logical GLSL450
               OpEntryPoint GLCompute %main "main" %buf
               OpExecutionMode %main LocalSize 1 1 1
               OpDecorate %S Block
               OpMemberDecorate %S 0 Offset 0
               OpMemberDecorate %S 1 Offset 4
               OpDecorate %buf DescriptorSet 0
               OpDecorate %buf Binding 0
       %void = OpTypeVoid
         %fn = OpTypeFunction %void
        %int = OpTypeInt 32 1
          %S = OpTypeStruct %int %int
        %ptr = OpTypePointer StorageBuffer %S
       %pint = OpTypePointer StorageBuffer %int
        %buf = OpVariable %ptr StorageBuffer
      %int_0 = OpConstant %int 0
      %int_1 = OpConstant %int 1
     %int_10 = OpConstant %int 10
       %main = OpFunction %void None %fn
      %entry = OpLabel
         %p0 = OpAccessChain %pint %buf %int_0
          %x = OpLoad %int %p0
         %p1 = OpAccessChain %pint %buf %int_1
               OpStore %p1 %int_10
               OpReturn
               OpFunctionEnd
`

// assembleSPVASM assembles SPIR-V text with spirv-as, which CI requires
// through NAGA_REQUIRE_TOOLS.
func assembleSPVASM(t *testing.T, src string) []byte {
	t.Helper()
	as := testtools.Look(t, "spirv-as", testtools.InstallSPIRVTools)
	dir := t.TempDir()
	in, out := filepath.Join(dir, "m.spvasm"), filepath.Join(dir, "m.spv")
	if err := os.WriteFile(in, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if msg, err := exec.Command(as, "--target-env", "spv1.3", "-o", out, in).CombinedOutput(); err != nil {
		t.Fatalf("spirv-as: %v\n%s", err, msg)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestSPVUnnamedConstantWrittenInPlace(t *testing.T) {
	module, _, err := spirv.Parse(assembleSPVASM(t, straightSPVASM), spirv.ParseOptions{})
	if err != nil {
		t.Fatalf("spirv.Parse: %v", err)
	}
	mslSrc, _, err := msl.Compile(module, msl.DefaultOptions())
	if err != nil {
		t.Fatalf("msl: %v", err)
	}
	hlslSrc, _, err := hlsl.Compile(module, hlsl.DefaultOptions())
	if err != nil {
		t.Fatalf("hlsl: %v", err)
	}
	glslSrc, _, err := glsl.Compile(module, glsl.Options{LangVersion: glsl.Version450})
	if err != nil {
		t.Fatalf("glsl: %v", err)
	}
	for _, tc := range []struct{ backend, src, want string }{
		{"msl", mslSrc, "global.member_1 = 10;"},
		{"hlsl", hlslSrc, "global_0_.Store(4, asuint(int(10)));"},
		{"glsl", glslSrc, "_group_0_binding_0_cs.member_1 = 10;"},
	} {
		if !strings.Contains(tc.src, tc.want) {
			t.Errorf("%s: no line %q in\n%s", tc.backend, tc.want, tc.src)
		}
		if undeclared := undeclaredConstants(tc.src); len(undeclared) > 0 {
			t.Errorf("%s: refers to undeclared constants %v:\n%s", tc.backend, undeclared, tc.src)
		}
	}
	compileGLSL(t, "straight", ir.StageCompute, glslSrc)
	compileHLSL(t, "straight", ir.StageCompute, "main", hlslSrc)
}

// TestSPVCorpusTextBackendsCompile writes every module of the SPIR-V input
// corpus that the frontend parses as MSL, HLSL and GLSL, and checks that the
// text refers to no constant it does not declare; with glslangValidator and
// dxc on PATH (required through NAGA_REQUIRE_TOOLS), the GLSL and HLSL also
// compile. Inputs and entry points a frontend or writer does not support yet
// are logged, not failed: this test guards what is written, not coverage.
func TestSPVCorpusTextBackendsCompile(t *testing.T) {
	bins, err := filepath.Glob(filepath.Join(rustDerivedDir, "spv", "*.spv"))
	if err != nil {
		t.Fatal(err)
	}
	if len(bins) == 0 {
		t.Fatalf("no SPIR-V corpus binaries under %s", filepath.Join(rustDerivedDir, "spv"))
	}
	written := 0
	for _, bin := range bins {
		name := strings.TrimSuffix(filepath.Base(bin), ".spv")
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(bin)
			if err != nil {
				t.Fatal(err)
			}
			module, _, err := spirv.Parse(data, spirv.ParseOptions{})
			if err != nil {
				t.Logf("not parsed: %v", err)
				return
			}
			if src, _, err := msl.Compile(module, msl.DefaultOptions()); err != nil {
				t.Logf("msl: %v", err)
			} else {
				written++
				if undeclared := undeclaredConstants(src); len(undeclared) > 0 {
					t.Errorf("msl refers to undeclared constants %v", undeclared)
				}
			}
			for i := range module.EntryPoints {
				ep := &module.EntryPoints[i]
				sel := &backend.EntryPointSelector{Stage: ep.Stage, Name: ep.Name}
				if src, info, err := hlsl.Compile(module, hlsl.Options{EntryPoint: sel}); err != nil {
					t.Logf("hlsl %s: %v", ep.Name, err)
				} else {
					written++
					if undeclared := undeclaredConstants(src); len(undeclared) > 0 {
						t.Errorf("hlsl %s refers to undeclared constants %v", ep.Name, undeclared)
					}
					compileHLSL(t, name+"."+ep.Name, ep.Stage, info.EntryPointNames[ep.Name], src)
				}
				if src, _, err := glsl.Compile(module, glsl.Options{LangVersion: glsl.Version450, EntryPoint: sel}); err != nil {
					t.Logf("glsl %s: %v", ep.Name, err)
				} else {
					written++
					if undeclared := undeclaredConstants(src); len(undeclared) > 0 {
						t.Errorf("glsl %s refers to undeclared constants %v", ep.Name, undeclared)
					}
					compileGLSL(t, name+"."+ep.Name, ep.Stage, src)
				}
			}
		})
	}
	if written == 0 {
		t.Error("no corpus module was written by any text backend")
	}
}

var (
	// constRef is a name the namer gives a constant without one of its
	// own (const_<type>…, or HLSL's const_<handle> fallback).
	constRef = regexp.MustCompile(`\bconst_\w+`)
	// constDecl is a module-scope constant declaration in MSL
	// (`constant T n =`), HLSL (`static const T n =`) or GLSL
	// (`const T n =`), with an optional array suffix.
	constDecl = regexp.MustCompile(`(?m)^\s*(?:constant|static const|const)\s.*?\b(\w+)\s*(?:\[[^\]]*\])?\s*=`)
)

// undeclaredConstants lists the namer-made constant names the text refers to
// without declaring them.
func undeclaredConstants(src string) []string {
	declared := map[string]bool{}
	for _, m := range constDecl.FindAllStringSubmatch(src, -1) {
		declared[m[1]] = true
	}
	var missing []string
	seen := map[string]bool{}
	for _, ref := range constRef.FindAllString(src, -1) {
		if !declared[ref] && !seen[ref] {
			seen[ref] = true
			missing = append(missing, ref)
		}
	}
	return missing
}

// installGlslang is the hint for a missing glslangValidator.
const installGlslang = "install glslang, e.g. from the Vulkan SDK"

// stageSuffix is glslangValidator's -S stage and dxc's target prefix.
var stageSuffix = map[ir.ShaderStage][2]string{
	ir.StageVertex:   {"vert", "vs"},
	ir.StageFragment: {"frag", "ps"},
	ir.StageCompute:  {"comp", "cs"},
}

// compileGLSL compiles src with glslangValidator when it is on PATH.
func compileGLSL(t *testing.T, label string, stage ir.ShaderStage, src string) {
	t.Helper()
	s, ok := stageSuffix[stage]
	if !ok {
		return
	}
	glslang, found := testtools.Optional(t, "glslangValidator", installGlslang)
	if !found {
		return
	}
	path := filepath.Join(t.TempDir(), "shader."+s[0])
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(glslang, path).CombinedOutput()
	checkCompiled(t, "glslangValidator", label, err, out, src)
}

// knownTextCompileFailures are corpus outputs that do not compile for a
// reason other than the one these tests guard, keyed "<input>.<entry> <tool>".
// An entry that starts compiling fails the test, so the list cannot go stale.
var knownTextCompileFailures = map[string]string{
	// Our HLSL writer prints asinh/acosh/atanh, which HLSL does not have;
	// Rust writes them as log() expressions (back/hlsl/writer.rs,
	// Function::Asincosh and Function::Atanh). A separate bug.
	"inv-hyperbolic-trig-functions.main dxc": "HLSL has no asinh, acosh, atanh",
	// subgroupMemoryBarrier needs GL_KHR_shader_subgroup_basic, which is not
	// requested. Rust's reference output
	// (spv-subgroup-barrier.main.Compute.glsl) omits it too.
	"subgroup-barrier.main glslangValidator": "no #extension GL_KHR_shader_subgroup_basic, as in Rust",
}

// checkCompiled reports a tool's verdict on src against the known failures.
func checkCompiled(t *testing.T, tool, label string, err error, out []byte, src string) {
	t.Helper()
	if reason, known := knownTextCompileFailures[label+" "+tool]; known {
		if err == nil {
			t.Errorf("%s %s compiles now; remove it from knownTextCompileFailures (%s)", tool, label, reason)
		} else {
			t.Logf("%s %s: known failure: %s", tool, label, reason)
		}
		return
	}
	if err != nil {
		t.Errorf("%s %s: %v\n%s\n%s", tool, label, err, out, src)
	}
}

// compileHLSL compiles src with dxc when it is on PATH.
func compileHLSL(t *testing.T, label string, stage ir.ShaderStage, entry, src string) {
	t.Helper()
	s, ok := stageSuffix[stage]
	if !ok || entry == "" {
		return
	}
	dxc, found := testtools.Optional(t, "dxc", testtools.InstallDXC)
	if !found {
		return
	}
	path := filepath.Join(t.TempDir(), "shader.hlsl")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(dxc, "-T", s[1]+"_6_0", "-E", entry, path).CombinedOutput()
	checkCompiled(t, "dxc", label, err, out, src)
}
