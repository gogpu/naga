package naga

import (
	"errors"
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/valid"
	"github.com/gogpu/naga/wgsl"
)

// capabilityLists are the optional capabilities each backend lists, written
// out here from Rust naga's supported_capabilities (naga/src/back/spv/mod.rs
// :1181, msl/mod.rs:836, hlsl/mod.rs:787, glsl/mod.rs:481 at the pin), minus
// the capabilities package valid has no bit for (IMMEDIATES,
// MULTISAMPLED_SHADING, CUBE_ARRAY_TEXTURES, TEXTURE_EXTERNAL,
// SHADER_FLOAT16_IN_FLOAT32, LINEAR_INTERPOLATION, the NON_UNIFORM_INDEXING
// ones, COOPERATIVE_MATRIX, RAY_HIT_VERTEX_POSITION, RAY_TRACING_PIPELINE,
// DEBUG_PRINTF). The memory decorations have bits since the IR carries them:
// MSL and HLSL list Coherent only, GLSL and SPIR-V list both, exactly as
// Rust's lists do.
var capabilityLists = []struct {
	name string
	got  valid.Capabilities
	want valid.Capabilities
}{
	{"spirv", spirv.SupportedCapabilities(),
		valid.CapFloat64 | valid.CapPrimitiveIndex | valid.CapTextureAndSamplerBindingArray |
			valid.CapBufferBindingArray | valid.CapStorageTextureBindingArray | valid.CapStorageBufferBindingArray |
			valid.CapAccelerationStructureBindingArray | valid.CapClipDistances |
			valid.CapStorageTexture16BitNormFormats | valid.CapMultiview | valid.CapEarlyDepthTest |
			valid.CapRayQuery | valid.CapDualSourceBlending | valid.CapShaderInt64 | valid.CapSubgroup |
			valid.CapSubgroupBarrier | valid.CapSubgroupVertexStage | valid.CapShaderInt64AtomicMinMax |
			valid.CapShaderInt64AtomicAllOps | valid.CapShaderFloat32Atomic | valid.CapTextureAtomic |
			valid.CapTextureInt64Atomic | valid.CapShaderFloat16 | valid.CapShaderInt16 |
			valid.CapShaderBarycentrics | valid.CapMeshShader | valid.CapMeshShaderPointTopology |
			valid.CapPerVertex | valid.CapDrawIndex | valid.CapMemoryDecorationCoherent |
			valid.CapMemoryDecorationVolatile},
	{"msl", msl.SupportedCapabilities(),
		valid.CapPrimitiveIndex | valid.CapTextureAndSamplerBindingArray | valid.CapStorageTextureBindingArray |
			valid.CapStorageBufferBindingArray | valid.CapClipDistances | valid.CapStorageTexture16BitNormFormats |
			valid.CapMultiview | valid.CapRayQuery | valid.CapDualSourceBlending | valid.CapShaderInt64 |
			valid.CapSubgroup | valid.CapSubgroupBarrier | valid.CapShaderInt64AtomicMinMax |
			valid.CapShaderFloat32Atomic | valid.CapTextureAtomic | valid.CapTextureInt64Atomic |
			valid.CapShaderFloat16 | valid.CapShaderInt16 | valid.CapShaderBarycentrics | valid.CapMeshShader |
			valid.CapMeshShaderPointTopology | valid.CapPerVertex | valid.CapMemoryDecorationCoherent},
	// Rust's HLSL list has no CLIP_DISTANCES; this backend writes
	// SV_ClipDistance (hlsl clip_distances_test.go) and wgpu offers the
	// feature on DX12, so it is listed here.
	{"hlsl", hlsl.SupportedCapabilities(),
		valid.CapFloat64 | valid.CapPrimitiveIndex | valid.CapTextureAndSamplerBindingArray |
			valid.CapStorageTextureBindingArray | valid.CapAccelerationStructureBindingArray |
			valid.CapStorageTexture16BitNormFormats | valid.CapMultiview | valid.CapRayQuery |
			valid.CapDualSourceBlending | valid.CapShaderInt64 | valid.CapSubgroup |
			valid.CapShaderInt64AtomicMinMax | valid.CapShaderInt64AtomicAllOps | valid.CapTextureAtomic |
			valid.CapTextureInt64Atomic | valid.CapShaderFloat16 | valid.CapShaderInt16 |
			valid.CapShaderBarycentrics | valid.CapMeshShader | valid.CapPerVertex | valid.CapClipDistances |
			valid.CapMemoryDecorationCoherent},
	{"glsl", glsl.SupportedCapabilities(),
		valid.CapFloat64 | valid.CapPrimitiveIndex | valid.CapClipDistances | valid.CapMultiview |
			valid.CapEarlyDepthTest | valid.CapDualSourceBlending | valid.CapShaderInt64 |
			valid.CapShaderInt64AtomicAllOps | valid.CapTextureAtomic | valid.CapTextureInt64Atomic |
			valid.CapSubgroup | valid.CapSubgroupBarrier | valid.CapShaderFloat16 | valid.CapShaderInt16 |
			valid.CapShaderBarycentrics | valid.CapDrawIndex | valid.CapStorageTexture16BitNormFormats |
			valid.CapMemoryDecorationCoherent | valid.CapMemoryDecorationVolatile},
	// Rust has no DXIL backend: this list is ours (dxil/capabilities.go).
	{"dxil", dxil.SupportedCapabilities(),
		valid.CapFloat64 | valid.CapEarlyDepthTest | valid.CapPrimitiveIndex | valid.CapTextureAndSamplerBindingArray |
			valid.CapStorageTextureBindingArray | valid.CapStorageBufferBindingArray | valid.CapClipDistances |
			valid.CapStorageTexture16BitNormFormats | valid.CapMultiview | valid.CapRayQuery |
			valid.CapDualSourceBlending | valid.CapShaderInt64 | valid.CapSubgroup | valid.CapSubgroupBarrier |
			valid.CapShaderInt64AtomicMinMax | valid.CapShaderInt64AtomicAllOps | valid.CapTextureAtomic |
			valid.CapTextureInt64Atomic | valid.CapShaderFloat16 | valid.CapMeshShader |
			valid.CapMeshShaderPointTopology},
}

// TestSupportedCapabilities pins each backend's list.
func TestSupportedCapabilities(t *testing.T) {
	for _, tt := range capabilityLists {
		if tt.got != tt.want {
			t.Errorf("%s: SupportedCapabilities = %v, want %v (differ by %v)", tt.name, tt.got, tt.want, tt.got^tt.want)
		}
	}
}

func compileFor(backendName string, m *ir.Module) error {
	var err error
	switch backendName {
	case "spirv":
		_, _, err = spirv.Compile(m, spirv.Options{})
	case "msl":
		_, _, err = msl.Compile(m, msl.Options{})
	case "hlsl":
		_, _, err = hlsl.Compile(m, hlsl.Options{})
	case "glsl":
		_, _, err = glsl.Compile(m, glsl.Options{LangVersion: glsl.Version450})
	case "dxil":
		_, _, err = dxil.Compile(m, dxil.Options{ShaderModel: backend.ShaderModel{Major: 6, Minor: 9}})
	}
	return err
}

// TestCompileRefusesUnsupportedCapability: a module that needs a capability
// outside a backend's list is refused with an error that names the
// capability and wraps backend.ErrUnsupported, where GLSL and HLSL used to
// write f32 atomics no compiler accepts (P2-API-005).
func TestCompileRefusesUnsupportedCapability(t *testing.T) {
	const f32Atomic = `@group(0) @binding(0) var<storage, read_write> a: atomic<f32>;
@compute @workgroup_size(1) fn main() { atomicAdd(&a, 1.5); }`
	const cullPrimitive = `enable wgpu_mesh_shader;
struct VertexOutput { @builtin(position) position: vec4<f32> }
struct PrimitiveOutput { @builtin(triangle_indices) indices: vec3<u32> }
struct MeshOutput {
    @builtin(vertices) vertices: array<VertexOutput, 3>,
    @builtin(primitives) primitives: array<PrimitiveOutput, 1>,
    @builtin(vertex_count) vertex_count: u32,
    @builtin(primitive_count) primitive_count: u32,
}
var<workgroup> mesh_output: MeshOutput;
@mesh(mesh_output) @workgroup_size(1) fn ms_main() {}`
	parse := func(t *testing.T, src string) *ir.Module {
		t.Helper()
		m, _, err := wgsl.Parse(src, wgsl.ParseOptions{})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	for _, tt := range []struct {
		name    string
		src     string
		backend string
		refused string // the capability named in the error, "" when it compiles
	}{
		{"f32 atomic", f32Atomic, "spirv", ""},
		{"f32 atomic", f32Atomic, "msl", ""},
		{"f32 atomic", f32Atomic, "hlsl", "SHADER_FLOAT32_ATOMIC"},
		{"f32 atomic", f32Atomic, "glsl", "SHADER_FLOAT32_ATOMIC"},
		{"f32 atomic", f32Atomic, "dxil", "SHADER_FLOAT32_ATOMIC"},
		{"mesh shader", cullPrimitive, "glsl", "MESH_SHADER"},
	} {
		t.Run(tt.name+"/"+tt.backend, func(t *testing.T) {
			err := compileFor(tt.backend, parse(t, tt.src))
			if tt.refused == "" {
				if err != nil {
					t.Fatalf("refused: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("compiled; want a refusal naming %s", tt.refused)
			}
			if !errors.Is(err, backend.ErrUnsupported) {
				t.Errorf("error %q does not wrap backend.ErrUnsupported", err)
			}
			if !strings.Contains(err.Error(), "Capabilities("+tt.refused+")") {
				t.Errorf("error %q does not name Capabilities(%s)", err, tt.refused)
			}
		})
	}
}
