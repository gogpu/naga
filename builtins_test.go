package naga

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/wgsl"
)

// Rust naga's draw-index.wgsl and barycentrics.wgsl (tests/in/wgsl).
const (
	drawIndexShader = `enable draw_index;
struct Input { @builtin(draw_index) draw_index: u32, }
@vertex fn vertex(input: Input) -> @builtin(position) vec4<f32> {
    return vec4<f32>(f32(input.draw_index), 1.0, 1.0, 1.0);
}`
	barycentricsShader = `@fragment
fn fs_main(@builtin(barycentric) bary: vec3<f32>) -> @location(0) vec4<f32> { return vec4(bary, 1.0); }
@fragment
fn fs_main_no_perspective(@builtin(barycentric_no_perspective) bary: vec3<f32>) -> @location(0) vec4<f32> { return vec4(bary, 1.0); }`
)

func lowerShader(t *testing.T, src string) *ir.Module {
	t.Helper()
	module, _, err := wgsl.Parse(src, wgsl.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return module
}

// TestUnknownBuiltinIsAnError: the lowerer named an unknown built-in
// position, so draw_index and barycentric_no_perspective compiled as the
// fragment coordinate. Rust naga's parser rejects an unknown name.
func TestUnknownBuiltinIsAnError(t *testing.T) {
	src := `@fragment fn fs(@builtin(no_such_builtin) x: vec4<f32>) -> @location(0) vec4<f32> { return x; }`
	_, _, err := wgsl.Parse(src, wgsl.ParseOptions{})
	if err == nil || !strings.Contains(err.Error(), "unknown builtin: `no_such_builtin`") {
		t.Fatalf("error %v, want unknown builtin: `no_such_builtin`", err)
	}
}

// spirvBuiltins returns the BuiltIn decorations and the capabilities of a
// SPIR-V binary.
func spirvBuiltins(t *testing.T, bin []byte) (builtins, caps []uint32) {
	t.Helper()
	const opCapability, decorationBuiltIn = 17, 11
	ops, operands := spirvOps(t, bin)
	for i, op := range ops {
		switch {
		case op == opDecorate && len(operands[i]) == 3 && operands[i][1] == decorationBuiltIn:
			builtins = append(builtins, operands[i][2])
		case op == opCapability:
			caps = append(caps, operands[i][0])
		}
	}
	return builtins, caps
}

func TestDrawIndexBuiltin(t *testing.T) {
	module := lowerShader(t, drawIndexShader)

	t.Run("spirv", func(t *testing.T) {
		bin, err := spirvBinary(module, spirv.Options{})
		if err != nil {
			t.Fatal(err)
		}
		builtins, caps := spirvBuiltins(t, bin)
		const builtInDrawIndex, builtInFragCoord, capDrawParameters = 4426, 15, 4427
		if !slices.Contains(builtins, builtInDrawIndex) || slices.Contains(builtins, builtInFragCoord) {
			t.Errorf("BuiltIn decorations %v, want DrawIndex (4426) and no FragCoord", builtins)
		}
		if !slices.Contains(caps, capDrawParameters) {
			t.Errorf("capabilities %v, want DrawParameters (4427)", caps)
		}
	})

	t.Run("glsl", func(t *testing.T) {
		code := func(v glsl.Version, drawParameters bool) (string, error) {
			src, _, err := glsl.Compile(module, glsl.Options{LangVersion: v, DrawParameters: drawParameters})
			return src, err
		}
		// gl_DrawID is core in GLSL 4.60 only.
		if _, err := code(glsl.Version330, false); err == nil || !strings.Contains(err.Error(), "DRAW_INDEX") {
			t.Errorf("GLSL 3.30: error %v, want DRAW_INDEX missing", err)
		}
		if _, err := code(glsl.VersionES310, true); err == nil {
			t.Error("GLSL ES 3.10 with DrawParameters: no error, the ARB extension is desktop only")
		}
		if src, err := code(glsl.Version460, false); err != nil || !strings.Contains(src, "Input(gl_DrawID)") {
			t.Errorf("GLSL 4.60: %v\n%s", err, src)
		}
		// Rust writes gl_DrawID with the extension, which declares gl_DrawIDARB.
		src, err := code(glsl.Version330, true)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"#extension GL_ARB_shader_draw_parameters : require", "Input(gl_DrawIDARB)"} {
			if !strings.Contains(src, want) {
				t.Errorf("GLSL 3.30 with DrawParameters: no %q in\n%s", want, src)
			}
		}
		if strings.Contains(src, "naga_vs_first_instance") {
			t.Errorf("GLSL with DrawParameters declares the first-instance uniform:\n%s", src)
		}
	})

	// HLSL, MSL and D3D have no draw index (Rust: "Unsupported builtin
	// DrawIndex", UnsupportedBuiltIn); all three wrote position before.
	t.Run("hlsl", func(t *testing.T) {
		_, _, err := hlsl.Compile(module, hlsl.Options{})
		if err == nil || !strings.Contains(err.Error(), "no HLSL semantic") {
			t.Errorf("error %v, want a refusal", err)
		}
	})
	t.Run("msl", func(t *testing.T) {
		_, _, err := msl.Compile(module, msl.Options{})
		if err == nil || !strings.Contains(err.Error(), "draw_index is not supported") {
			t.Errorf("error %v, want a refusal", err)
		}
	})
	t.Run("dxil", func(t *testing.T) {
		_, err := dxilBlob(module, dxil.Options{})
		if err == nil || !strings.Contains(err.Error(), "draw_index is not supported") {
			t.Errorf("error %v, want a refusal", err)
		}
	})
}

// TestGLSLDrawParametersInstanceIndex: with DrawParameters the first instance
// comes from gl_BaseInstanceARB, not the naga_vs_first_instance uniform
// (Rust naga's DRAW_PARAMETERS).
func TestGLSLDrawParametersInstanceIndex(t *testing.T) {
	module := lowerShader(t, `@vertex fn vs(@builtin(instance_index) i: u32) -> @builtin(position) vec4<f32> {
    return vec4<f32>(f32(i));
}`)
	for _, tc := range []struct {
		drawParameters bool
		want, notWant  string
	}{
		{false, "(uint(gl_InstanceID) + naga_vs_first_instance)", "GL_ARB_shader_draw_parameters"},
		{true, "(uint(gl_InstanceID) + uint(gl_BaseInstanceARB))", "naga_vs_first_instance"},
	} {
		src, _, err := glsl.Compile(module, glsl.Options{DrawParameters: tc.drawParameters})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(src, tc.want) || strings.Contains(src, tc.notWant) {
			t.Errorf("DrawParameters %v: want %q and no %q in\n%s", tc.drawParameters, tc.want, tc.notWant, src)
		}
	}
}

func TestBarycentricNoPerspectiveBuiltin(t *testing.T) {
	module := lowerShader(t, barycentricsShader)
	fs := &backend.EntryPointSelector{Stage: ir.StageFragment, Name: "fs_main_no_perspective"}

	t.Run("spirv", func(t *testing.T) {
		bin, err := spirvBinary(module, spirv.Options{EntryPoint: fs})
		if err != nil {
			t.Fatal(err)
		}
		builtins, _ := spirvBuiltins(t, bin)
		const builtInBaryCoordNoPerspKHR = 5287
		if !slices.Equal(builtins, []uint32{builtInBaryCoordNoPerspKHR}) {
			t.Errorf("BuiltIn decorations %v, want [BaryCoordNoPerspKHR (5287)]", builtins)
		}
	})
	t.Run("glsl", func(t *testing.T) {
		src, _, err := glsl.Compile(module, glsl.Options{LangVersion: glsl.Version450, EntryPoint: fs})
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"#extension GL_EXT_fragment_shader_barycentric : require", "= gl_BaryCoordNoPerspEXT;"} {
			if !strings.Contains(src, want) {
				t.Errorf("no %q in\n%s", want, src)
			}
		}
	})
	t.Run("hlsl", func(t *testing.T) {
		// SV_Barycentrics is a shader model 6.1 semantic; Rust writes it at
		// any model.
		_, _, err := hlsl.Compile(module, hlsl.Options{})
		var vErr *backend.VersionError
		if !errors.As(err, &vErr) || vErr.Space != backend.VersionShaderModel ||
			vErr.Required != (backend.Version{Major: 6, Minor: 1}) {
			t.Fatalf("shader model 5.1: error %v, want a shader model VersionError for 6.1", err)
		}
		src, _, err := hlsl.Compile(module, hlsl.Options{ShaderModel: backend.ShaderModel{Major: 6, Minor: 1}})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(src, "noperspective float3 bary_3 : SV_Barycentrics;") {
			t.Errorf("no noperspective SV_Barycentrics in\n%s", src)
		}
	})
	t.Run("msl", func(t *testing.T) {
		// Rust naga fc9060f5: barycentrics travel in the [[stage_in]] struct.
		// They need MSL 2.3: the zero version, 2.1, is refused, not raised.
		_, _, err := msl.Compile(module, msl.Options{EntryPoint: fs})
		var vErr *backend.VersionError
		if !errors.As(err, &vErr) || vErr.Space != backend.VersionMSL || vErr.Required != (backend.Version{Major: 2, Minor: 3}) {
			t.Fatalf("MSL 2.1: error %v, want an MSL VersionError for 2.3", err)
		}
		src, _, err := msl.Compile(module, msl.Options{EntryPoint: fs, LangVersion: msl.Version2_3})
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{
			"// language: metal2.3",
			"metal::float3 bary_1 [[barycentric_coord, center_no_perspective]];",
			"fs_main_no_perspectiveInput varyings [[stage_in]]",
			"const auto bary_1 = varyings.bary_1;",
		} {
			if !strings.Contains(src, want) {
				t.Errorf("no %q in\n%s", want, src)
			}
		}
	})
	t.Run("dxil", func(t *testing.T) {
		// Not written yet (P2-DXIL-073); the semantic fallback named it
		// SV_Unknown.
		_, err := dxilBlob(module, dxil.Options{EntryPoint: fs, ShaderModel: backend.ShaderModel{Major: 6, Minor: 1}})
		if err == nil || !strings.Contains(err.Error(), "barycentric_no_perspective is not supported") {
			t.Errorf("error %v, want a refusal", err)
		}
	})
}
