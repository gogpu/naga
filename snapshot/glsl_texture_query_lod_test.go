package snapshot_test

// The GLSL textureQueryLod matrix (card P1-GLSLIN-012): the level-of-detail
// query of GLSL 4.00 §8.9, end to end.
//
// An "ok" row must parse, validate, compile to SPIR-V that spirv-val accepts,
// and (unless the row records the backend as unable) compile through the MSL,
// HLSL and GLSL backends, with the HLSL it produces compiled by dxc when the
// tool is installed. Before any of that its source is compiled by
// glslangValidator, so a row cannot assert support for a form the reference
// GLSL compiler rejects. A "refuse" row must fail with a *glsl.ParseError whose
// message carries the recorded text, and its reason names what justifies the
// refusal.
//
// The SPIR-V shape is pinned to glslang's: OpCapability ImageQuery, an
// OpSampledImage built from the image and the sampler, and an OpImageQueryLod
// whose result is a vec2 of float on that sampled image.
//
// A backend the row records in backendSkip is asserted to still refuse the
// module, so a recorded limitation cannot rot into a silent skip. The recorded
// limitations are the reference implementations':
//   - msl, 1D: Metal has no calculate_clamped_lod for texture1d (SPIRV-Cross
//     refuses OpImageQueryLod on a 1D texture);
//   - hlsl, depth images: the sampler of a depth image is a
//     SamplerComparisonState, and dxc accepts CalculateLevelOfDetail with one
//     only from shader model 6.8;
//   - hlsl, integer images: dxc's Texture2D<int4> has no
//     CalculateLevelOfDetail member at all.

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// glslTextureQueryLodMatrix is the matrix: the sampler classes glslang accepts
// the query for, the combined-sampler split, and the refusals.
var glslTextureQueryLodMatrix = []glslMatrixRow{
	// --- the sampler classes -------------------------------------------------

	{
		name:      "texture-query-lod-2d",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureQueryLod"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2D s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = vec4(textureQueryLod(s, uv), 0.0, 0.0); }
`,
	},
	{
		name:      "texture-query-lod-1d",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureQueryLod"},
		backendSkip: map[string]backendRefusal{
			"msl": {
				reason: "Metal has no calculate_clamped_lod for texture1d; SPIRV-Cross refuses " +
					"OpImageQueryLod on a 1D texture too",
				wantError: "no calculate_clamped_lod for texture1d",
			},
		},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler1D s;
layout(location = 0) in float u;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = vec4(textureQueryLod(s, u), 0.0, 0.0); }
`,
	},
	{
		name:      "texture-query-lod-3d",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureQueryLod"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler3D s;
layout(location = 0) in vec3 p;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = vec4(textureQueryLod(s, p), 0.0, 0.0); }
`,
	},
	{
		name:      "texture-query-lod-cube",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureQueryLod"},
		source: `#version 450
layout(set = 0, binding = 0) uniform samplerCube s;
layout(location = 0) in vec3 d;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = vec4(textureQueryLod(s, d), 0.0, 0.0); }
`,
	},
	{
		// The coordinate is the spatial one: a 2D array takes a vec2, not the
		// vec3 a sampling call would take (glslang's overload set).
		name:      "texture-query-lod-2d-array",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureQueryLod"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2DArray s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = vec4(textureQueryLod(s, uv), 0.0, 0.0); }
`,
	},
	{
		name:      "texture-query-lod-cube-array",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureQueryLod"},
		source: `#version 450
layout(set = 0, binding = 0) uniform samplerCubeArray s;
layout(location = 0) in vec3 d;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = vec4(textureQueryLod(s, d), 0.0, 0.0); }
`,
	},
	{
		name:      "texture-query-lod-isampler",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureQueryLod"},
		backendSkip: map[string]backendRefusal{
			"hlsl": {
				reason:    "dxc's Texture2D<int4> has no CalculateLevelOfDetail member",
				wantError: "CalculateLevelOfDetail exists only for floating-point",
			},
		},
		source: `#version 450
layout(set = 0, binding = 0) uniform isampler2D s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = vec4(textureQueryLod(s, uv), 0.0, 0.0); }
`,
	},
	{
		name:      "texture-query-lod-usampler",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureQueryLod"},
		backendSkip: map[string]backendRefusal{
			"hlsl": {
				reason:    "dxc's Texture2D<uint4> has no CalculateLevelOfDetail member",
				wantError: "CalculateLevelOfDetail exists only for floating-point",
			},
		},
		source: `#version 450
layout(set = 0, binding = 0) uniform usampler2D s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = vec4(textureQueryLod(s, uv), 0.0, 0.0); }
`,
	},
	{
		name:      "texture-query-lod-shadow-2d",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureQueryLod"},
		backendSkip: map[string]backendRefusal{
			"hlsl": {
				reason: "the sampler of a depth image is a SamplerComparisonState, and dxc accepts " +
					"CalculateLevelOfDetail with one only from shader model 6.8",
				wantError: "SamplerComparisonState",
			},
		},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2DShadow s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = vec4(textureQueryLod(s, uv), 0.0, 0.0); }
`,
	},
	{
		name:      "texture-query-lod-shadow-cube",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureQueryLod"},
		backendSkip: map[string]backendRefusal{
			"hlsl": {
				reason: "the sampler of a depth image is a SamplerComparisonState, and dxc accepts " +
					"CalculateLevelOfDetail with one only from shader model 6.8",
				wantError: "SamplerComparisonState",
			},
		},
		source: `#version 450
layout(set = 0, binding = 0) uniform samplerCubeShadow s;
layout(location = 0) in vec3 d;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = vec4(textureQueryLod(s, d), 0.0, 0.0); }
`,
	},
	{
		// The combined-sampler split: the constructor pairs two separate
		// declarations, and the query carries the sampler the pair recorded.
		name:      "texture-query-lod-separate-sampler",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureQueryLod"},
		source: `#version 450
layout(set = 0, binding = 0) uniform texture2D t;
layout(set = 0, binding = 1) uniform sampler smp;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = vec4(textureQueryLod(sampler2D(t, smp), uv), 0.0, 0.0); }
`,
	},

	// --- the refusals --------------------------------------------------------

	{
		name:      "texture-query-lod-vertex-stage",
		stage:     ir.StageVertex,
		extension: ".vert",
		builtins:  []string{"textureQueryLod"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2D s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() {
    fragColor = vec4(textureQueryLod(s, uv), 0.0, 0.0);
    gl_Position = vec4(uv, 0.0, 1.0);
}
`,
		refuse: "textureQueryLod is only available in a fragment shader",
		reason: "SPIR-V defines OpImageQueryLod for the Fragment execution model only, and " +
			"glslang declares no textureQueryLod overload outside it ('no matching overloaded " +
			"function found' for the vertex probe)",
	},
	{
		name:      "texture-query-lod-compute-stage",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"textureQueryLod"},
		source: `#version 450
layout(local_size_x = 1) in;
layout(set = 0, binding = 0) uniform sampler2D s;
layout(set = 0, binding = 1) buffer Out { vec4 v; } o;
void main() { o.v = vec4(textureQueryLod(s, vec2(0.5)), 0.0, 0.0); }
`,
		refuse: "textureQueryLod is only available in a fragment shader",
		reason: "the lod query is fragment-only in SPIR-V, and glslang has no overload for a " +
			"compute shader",
	},
	{
		name:      "texture-query-lod-gl330",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureQueryLod"},
		source: `#version 330
uniform sampler2D s;
out vec4 fragColor;
void main() { fragColor = vec4(textureQueryLod(s, gl_FragCoord.xy), 0.0, 0.0); }
`,
		refuse: "textureQueryLod requires GLSL 4.00; no GLSL ES version has it",
		reason: "the GLSL 3.30 dialect has no textureQueryLod without an extension (glslang: " +
			"'textureQueryLod' : no matching overloaded function found)",
	},
	{
		name:      "texture-query-lod-es300",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureQueryLod"},
		source: `#version 300 es
precision highp float;
uniform sampler2D s;
out vec4 fragColor;
void main() { fragColor = vec4(textureQueryLod(s, gl_FragCoord.xy), 0.0, 0.0); }
`,
		refuse: "textureQueryLod requires GLSL 4.00; no GLSL ES version has it",
		reason: "no GLSL ES version has textureQueryLod (glslang refuses it in 300, 310 and " +
			"320 es)",
	},
	{
		name:      "texture-query-lod-sampler2dms",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureQueryLod"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2DMS s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = vec4(textureQueryLod(s, uv), 0.0, 0.0); }
`,
		refuse: "Unknown function 'textureQueryLod'",
		reason: "glslang declares no textureQueryLod overload for a multisampled sampler ('no " +
			"matching overloaded function found' for sampler2DMS), and a multisampled texture " +
			"has no mip levels to query",
	},
}

// TestGLSLTextureQueryLodMatrix runs every row through the frontend and, for an
// ok row, through glslang, the validator and all the backends.
func TestGLSLTextureQueryLodMatrix(t *testing.T) {
	glslang, haveGLSLang := testtools.Optional(t, "glslangValidator", glslangInstall)
	dxc := dxcPath()
	if dxc == "" {
		t.Logf("dxc not found: the HLSL rows were not compiled; install the Windows SDK or put dxc in PATH")
	}

	for _, row := range glslTextureQueryLodMatrix {
		t.Run(row.name, func(t *testing.T) {
			if row.refuse == "" {
				checkGLSLSourceWithGLSLang(t, glslang, haveGLSLang, row)
			}

			module, _, err := glsl.Parse(row.source, glsl.ParseOptions{Stage: row.stage})
			if row.refuse != "" {
				if err == nil {
					t.Fatalf("parsed, want a refusal containing %q (%s)\n\nsource:\n%s",
						row.refuse, row.reason, row.source)
				}
				var parseErr *glsl.ParseError
				if !errors.As(err, &parseErr) {
					t.Fatalf("error is %T, want *glsl.ParseError: %v", err, err)
				}
				if !strings.Contains(parseErr.Message, row.refuse) {
					t.Fatalf("refusal is %q, want it to contain %q\nreason: %s\n\nsource:\n%s",
						parseErr.Message, row.refuse, row.reason, row.source)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse: %v\n\nsource:\n%s", err, row.source)
			}
			if verr := validateGLSL(t, module); verr != nil {
				t.Fatalf("the parsed module does not validate: %v\n\nsource:\n%s", verr, row.source)
			}
			if step, cerr := compileTextureQueryLodModule(t, row, module, dxc); cerr != nil {
				t.Fatalf("%s step: %v\n\nsource:\n%s", step, cerr, row.source)
			}
			// The frontend's own output goes back through the GLSL backend,
			// where textureQueryLod's feature gate (desktop 330+, never ES) runs.
			glslSrc, _, gerr := glsl.Compile(module, glsl.Options{LangVersion: glsl.Version450})
			if gerr != nil {
				t.Fatalf("glsl backend round trip: %v\n\nsource:\n%s", gerr, row.source)
			}
			if !strings.Contains(glslSrc, "textureQueryLod(") {
				t.Fatalf("the glsl backend wrote no textureQueryLod call:\n%s", glslSrc)
			}
			// The emitted GLSL must itself be valid: the combined sampler the
			// query names has to be declared for it.
			if haveGLSLang {
				path := filepath.Join(t.TempDir(), row.name+".frag")
				if werr := os.WriteFile(path, []byte(glslSrc), 0o600); werr != nil {
					t.Fatalf("write the glsl backend's output: %v", werr)
				}
				if out, gerr := exec.Command(glslang, path).CombinedOutput(); gerr != nil {
					t.Fatalf("glslangValidator rejects the glsl backend's output: %v\n%s\n\noutput:\n%s",
						gerr, out, glslSrc)
				}
			}
			// GLSL ES has no textureQueryLod in any version, so the backend
			// must refuse the module there.
			if _, _, gerr := glsl.Compile(module, glsl.Options{LangVersion: glsl.VersionES320}); gerr == nil {
				t.Fatalf("the glsl backend wrote textureQueryLod for GLSL ES 3.20, which has no such function")
			}
		})
	}
}

// compileTextureQueryLodModule runs a parsed row through SPIR-V (pinning its
// shape), spirv-val, MSL, HLSL and dxc. A backend the row records in
// backendSkip is asserted to still refuse the module.
func compileTextureQueryLodModule(t *testing.T, row glslMatrixRow, module *ir.Module, dxc string) (string, error) {
	t.Helper()
	binary, _, err := spirv.Compile(module, spirv.Options{})
	if err != nil {
		return "spirv", err
	}
	assertTextureQueryLodSPIRV(t, binary)
	if validator, ok := testtools.Optional(t, "spirv-val", testtools.InstallSPIRVTools); ok {
		path := filepath.Join(t.TempDir(), "module.spv")
		if err := os.WriteFile(path, binary, 0o600); err != nil {
			return "spirv-val", err
		}
		if out, err := exec.Command(validator, path).CombinedOutput(); err != nil {
			return "spirv-val", fmt.Errorf("spirv-val: %w: %s", err, out)
		}
	}

	// MSL must be written at 2.2: OpImageQueryLod's lowering
	// (calculate_clamped_lod / calculate_unclamped_lod) arrived in MSL 2.2
	// (SPIRV-Cross's supports_msl_version (2, 2) check).
	mslSrc, _, mslErr := msl.Compile(module, msl.Options{LangVersion: msl.Version{Major: 2, Minor: 2}})
	if refusal, recorded := row.backendSkip["msl"]; recorded {
		assertRecordedRefusal(t, row, "msl", refusal, mslErr)
	} else if mslErr != nil {
		return "msl", mslErr
	} else {
		assertLodBackendText(t, row, "msl", mslSrc)
	}

	hlslSrc, _, hlslErr := hlsl.Compile(module, hlsl.Options{})
	if refusal, recorded := row.backendSkip["hlsl"]; recorded {
		assertRecordedRefusal(t, row, "hlsl", refusal, hlslErr)
		return "", nil
	}
	if hlslErr != nil {
		return "hlsl", hlslErr
	}
	assertLodBackendText(t, row, "hlsl", hlslSrc)
	if dxc != "" {
		if err := dxcCompileFragment(t, dxc, hlslSrc); err != nil {
			return "dxc", err
		}
	}
	return "", nil
}

// assertLodBackendText pins the backend's own text: the query must ask for the
// clamped level first and the unclamped one second, or a backend would compile
// and silently swap the two components of the result. Compilation cannot catch
// that, so the ordering is a golden fact.
func assertLodBackendText(t *testing.T, row glslMatrixRow, backendName, source string) {
	t.Helper()
	switch backendName {
	case "msl":
		clamped := strings.Index(source, ".calculate_clamped_lod(")
		unclamped := strings.Index(source, ".calculate_unclamped_lod(")
		if clamped < 0 || unclamped < 0 {
			t.Fatalf("%s: the MSL output has no calculate_clamped_lod/calculate_unclamped_lod pair:\n%s",
				row.name, source)
		}
		if clamped > unclamped {
			t.Fatalf("%s: the MSL output asks for the unclamped level before the clamped one:\n%s",
				row.name, source)
		}
		if !strings.Contains(source, "metal::float2(") {
			t.Fatalf("%s: the MSL output does not pack the two levels into a metal::float2:\n%s",
				row.name, source)
		}
	case "hlsl":
		want := "CalculateLevelOfDetail(samp, coord), tex.CalculateLevelOfDetailUnclamped(samp, coord)"
		if !strings.Contains(source, want) {
			t.Fatalf("%s: the HLSL output does not pack the clamped and unclamped levels as %q:\n%s",
				row.name, want, source)
		}
	}
}

// dxcCompileFragment compiles the HLSL the backend wrote as a pixel shader.
func dxcCompileFragment(t *testing.T, dxc, source string) error {
	t.Helper()
	dir := t.TempDir()
	hlslPath := filepath.Join(dir, "module.hlsl")
	if err := os.WriteFile(hlslPath, []byte(source), 0o600); err != nil {
		return err
	}
	blob := filepath.Join(dir, "module.dxil")
	args := []string{"-T", stageDxcProfile(ir.StageFragment, false, 0), "-E", "main",
		"-HV", "2018", "-Vd", "-Fo", blob, hlslPath}
	out, err := exec.Command(dxc, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("dxc: %w: %s", err, out)
	}
	return nil
}

// SPIR-V constants the shape assertion reads (SPIR-V 1.6, the unified
// specification: OpCapability, OpTypeFloat, OpTypeVector, OpSampledImage,
// OpImageQueryLod, and Capability ImageQuery).
const (
	spvOpCapability    = 17
	spvOpTypeFloat     = 22
	spvOpTypeVector    = 23
	spvOpSampledImage  = 86
	spvOpImageQueryLod = 105

	spvCapabilityImageQuery = 50
)

// spvInstructionsByOpcode returns the operands of every instruction of the
// binary whose opcode matches.
func spvInstructionsByOpcode(spv []byte, opcode uint16) [][]uint32 {
	// The header is five words; every instruction starts with a word holding
	// its word count in the high 16 bits and its opcode in the low 16.
	var out [][]uint32
	for i := 5; i < len(spv)/4; {
		word := binary.LittleEndian.Uint32(spv[i*4:])
		count := int(word >> 16)
		if count == 0 || i+count > len(spv)/4 {
			return out
		}
		operands := make([]uint32, count-1)
		for j := 0; j < count-1; j++ {
			operands[j] = binary.LittleEndian.Uint32(spv[(i+1+j)*4:])
		}
		if uint16(word) == opcode {
			out = append(out, operands)
		}
		i += count
	}
	return out
}

// assertTextureQueryLodSPIRV pins the query's SPIR-V shape to glslang's:
// OpCapability ImageQuery, one OpImageQueryLod on the result of an
// OpTypeSampledImage, with a vec2<float> result.
func assertTextureQueryLodSPIRV(t *testing.T, spv []byte) {
	t.Helper()
	imageQueryCapability := false
	for _, operands := range spvInstructionsByOpcode(spv, spvOpCapability) {
		if len(operands) == 1 && operands[0] == spvCapabilityImageQuery {
			imageQueryCapability = true
		}
	}
	if !imageQueryCapability {
		t.Fatal("the SPIR-V has no Capability ImageQuery")
	}

	sampledImages := make(map[uint32]bool)
	for _, operands := range spvInstructionsByOpcode(spv, spvOpSampledImage) {
		// Result Type, Result Id, Image, Sampler.
		if len(operands) == 4 {
			sampledImages[operands[1]] = true
		}
	}
	vectors := make(map[uint32][]uint32)
	for _, operands := range spvInstructionsByOpcode(spv, spvOpTypeVector) {
		vectors[operands[0]] = operands
	}
	floats := make(map[uint32]uint32)
	for _, operands := range spvInstructionsByOpcode(spv, spvOpTypeFloat) {
		floats[operands[0]] = operands[1]
	}

	lods := spvInstructionsByOpcode(spv, spvOpImageQueryLod)
	if len(lods) != 1 {
		t.Fatalf("the SPIR-V has %d OpImageQueryLod instructions, want exactly one", len(lods))
	}
	lod := lods[0]
	// Result Type, Result Id, Sampled Image, Coordinate.
	if len(lod) != 4 {
		t.Fatalf("OpImageQueryLod has %d operands, want 4 (result type, result id, sampled image, coordinate)", len(lod))
	}
	if !sampledImages[lod[2]] {
		t.Fatalf("OpImageQueryLod's image operand %%%d is not an OpTypeSampledImage result (%v); "+
			"glslang combines image and sampler first", lod[2], sampledImages)
	}
	vector, ok := vectors[lod[0]]
	if !ok {
		t.Fatalf("OpImageQueryLod's result type %%%d is not a vector", lod[0])
	}
	if len(vector) != 3 || vector[2] != 2 {
		t.Fatalf("OpImageQueryLod's result vector is %v, want two components", vector)
	}
	if width, ok := floats[vector[1]]; !ok || width != 32 {
		t.Fatalf("OpImageQueryLod's result component %%%d is not a 32-bit float (width %d)", vector[1], width)
	}
}

// TestGLSLTextureQueryLodMSLVersion: the lod query lowers to
// calculate_clamped_lod, which MSL 2.1 does not have, so the writer must refuse
// the module below 2.2 instead of writing a library Metal rejects.
func TestGLSLTextureQueryLodMSLVersion(t *testing.T) {
	var row glslMatrixRow
	for _, candidate := range glslTextureQueryLodMatrix {
		if candidate.name == "texture-query-lod-2d" {
			row = candidate
		}
	}
	if row.source == "" {
		t.Fatal("the 2D row is missing from the matrix")
	}
	module, _, err := glsl.Parse(row.source, glsl.ParseOptions{Stage: row.stage})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	_, _, cerr := msl.Compile(module, msl.DefaultOptions())
	if cerr == nil {
		t.Fatal("the MSL backend wrote the lod query at its default 2.1; calculate_clamped_lod needs 2.2")
	}
	var versionErr *backend.VersionError
	if !errors.As(cerr, &versionErr) {
		t.Fatalf("the MSL refusal is %T (%v), want a *backend.VersionError", cerr, cerr)
	}
	if versionErr.Required.Major != 2 || versionErr.Required.Minor != 2 {
		t.Fatalf("the MSL refusal asks for %v, want 2.2", versionErr.Required)
	}
}
