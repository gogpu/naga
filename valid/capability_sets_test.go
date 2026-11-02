package valid_test

import (
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
)

// meshPointsShader is Rust's mesh-shader-points.wgsl: a task shader and a
// mesh shader that outputs points.
const meshPointsShader = `enable wgpu_mesh_shader;
struct TaskPayload { dummy: u32 }
struct VertexOutput { @builtin(position) position: vec4<f32> }
struct PrimitiveOutput { @builtin(point_index) indices: u32 }
var<task_payload> taskPayload: TaskPayload;
@task @payload(taskPayload) @workgroup_size(64)
fn ts_main() -> @builtin(mesh_task_size) vec3<u32> { return vec3(1, 1, 1); }
struct MeshOutput {
    @builtin(vertices) vertices: array<VertexOutput, 1>,
    @builtin(primitives) primitives: array<PrimitiveOutput, 1>,
    @builtin(vertex_count) vertex_count: u32,
    @builtin(primitive_count) primitive_count: u32,
}
var<workgroup> mesh_output: MeshOutput;
@mesh(mesh_output) @payload(taskPayload) @workgroup_size(64)
fn ms_main() {}
`

// TestMeshShaderCapabilities: task and mesh entry points, the task_payload
// address space and the mesh built-ins need MESH_SHADER (Rust
// interface.rs:1264, :1098 and :666); a mesh shader that outputs points
// also needs MESH_SHADER_POINT_TOPOLOGY (interface.rs:1640).
func TestMeshShaderCapabilities(t *testing.T) {
	m := parse(t, meshPointsShader)
	for _, want := range []string{
		"Entry point ts_main at Task is invalid: Capability Capabilities(MESH_SHADER) is not supported",
		"Entry point ms_main at Mesh is invalid: Capability Capabilities(MESH_SHADER) is not supported",
		"Global variable [0] 'taskPayload' is invalid: Capability Capabilities(MESH_SHADER) is not supported",
		`entry point "ts_main": result: Capability Capabilities(MESH_SHADER) is not supported`,
	} {
		expectRejected(t, m, valid.Options{}, want)
	}
	expectRejected(t, m, valid.Options{Capabilities: valid.CapMeshShader},
		"Entry point ms_main at Mesh is invalid: Capability Capabilities(MESH_SHADER_POINT_TOPOLOGY) is not supported")
	expectValid(t, m, valid.Options{Capabilities: valid.CapMeshShader | valid.CapMeshShaderPointTopology})
}

// TestPerPrimitiveInputCapability: a @per_primitive fragment input needs
// MESH_SHADER (Rust interface.rs:698).
func TestPerPrimitiveInputCapability(t *testing.T) {
	m := parse(t, `enable wgpu_mesh_shader;
struct In { @per_primitive @location(1) mask: vec4<f32> }
@fragment fn main(in: In) -> @location(0) vec4<f32> { return in.mask; }`)
	expectRejected(t, m, valid.Options{},
		`entry point "main": argument 0: Capability Capabilities(MESH_SHADER) is not supported`)
	expectValid(t, m, valid.Options{Capabilities: valid.CapMeshShader})
}

// TestAccelerationStructureBindingArrayCapability: a binding array of
// acceleration structures needs ACCELERATION_STRUCTURE_BINDING_ARRAY (Rust
// interface.rs:997), on top of RAY_QUERY for the type.
func TestAccelerationStructureBindingArrayCapability(t *testing.T) {
	m := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.AccelerationStructureType{}},
			{Inner: ir.BindingArrayType{Base: 0}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "accs", Space: ir.SpaceHandle, Binding: &ir.ResourceBinding{}, Type: 1},
		},
	}
	expectRejected(t, m, valid.Options{Capabilities: valid.CapRayQuery},
		"Global variable [0] 'accs' is invalid: Capability Capabilities(ACCELERATION_STRUCTURE_BINDING_ARRAY) is not supported")
	expectValid(t, m, valid.Options{Capabilities: valid.CapRayQuery | valid.CapAccelerationStructureBindingArray})
}

// TestShaderInt16Capability: i16 and u16 scalars need SHADER_INT16 (Rust
// type.rs:320 and :344, check_width).
func TestShaderInt16Capability(t *testing.T) {
	for _, kind := range []ir.ScalarKind{ir.ScalarSint, ir.ScalarUint} {
		m := &ir.Module{Types: []ir.Type{{Inner: ir.ScalarType{Kind: kind, Width: 2}}}}
		expectRejected(t, m, valid.Options{}, "16 requires the SHADER_INT16 capability")
		expectValid(t, m, valid.Options{Capabilities: valid.CapShaderInt16})
	}
}

// TestLiteralWidthCapability: a literal of a 64-bit or f16 scalar needs the
// capability of its type, in a function and among the global expressions
// (Rust expression.rs:1461 validate_literal), with Rust's WidthError text.
func TestLiteralWidthCapability(t *testing.T) {
	m := parse(t, `const c = 1lu;
@compute @workgroup_size(1) fn main() { var x: u64 = c; x = 2lu; }`)
	expectRejected(t, m, valid.Options{},
		"in function main, expression 2: Using `u64` values requires the `naga::valid::Capabilities::SHADER_INT64` flag")
	expectRejected(t, m, valid.Options{},
		"Constant expression [0] is invalid: Using `u64` values requires the `naga::valid::Capabilities::SHADER_INT64` flag")
	expectValid(t, m, valid.Options{Capabilities: valid.CapShaderInt64})

	lit := func(v ir.LiteralValue) *ir.Module {
		return &ir.Module{GlobalExpressions: []ir.Expression{{Kind: ir.Literal{Value: v}}}}
	}
	expectRejected(t, lit(ir.LiteralF64(1)), valid.Options{},
		"Using `f64` values requires the `naga::valid::Capabilities::FLOAT64` flag")
	expectRejected(t, lit(ir.LiteralI64(1)), valid.Options{},
		"Using `i64` values requires the `naga::valid::Capabilities::SHADER_INT64` flag")
	// Rust's text names the f16 flag FLOAT16.
	expectRejected(t, lit(ir.LiteralF16(1)), valid.Options{},
		"Using `f16` values requires the `naga::valid::Capabilities::FLOAT16` flag")
	expectValid(t, lit(ir.LiteralF64(1)), valid.Options{Capabilities: valid.CapFloat64})
	expectValid(t, lit(ir.LiteralF16(1)), valid.Options{Capabilities: valid.CapShaderFloat16})
}

// TestFloat32AtomicOperators: with SHADER_FLOAT32_ATOMIC an f32 atomic
// allows add, subtract and exchange only (Rust function.rs:538,
// AtomicError::InvalidOperator).
func TestFloat32AtomicOperators(t *testing.T) {
	m := parse(t, `@group(0) @binding(0) var<storage, read_write> a: atomic<f32>;
@compute @workgroup_size(1) fn main() { _ = atomicExchange(&a, 1.0); atomicSub(&a, 1.0); atomicMax(&a, 1.0); }`)
	expectRejected(t, m, valid.Options{Capabilities: valid.CapShaderFloat32Atomic},
		"Atomic operation is invalid: Operator Max is not supported")
}

// subgroupShader uses one operation of each class in a compute shader.
const subgroupShader = `enable subgroups;
@compute @workgroup_size(64) fn main(@builtin(local_invocation_index) i: u32) {
    _ = subgroupBallot(true);
    _ = subgroupAll(i == 0u);
    _ = subgroupAdd(i);
    _ = subgroupShuffle(i, 1u);
    _ = subgroupShuffleDown(i, 1u);
    _ = quadSwapX(i);
}`

// TestSubgroupOperations: a non-zero Options.SubgroupOperations replaces
// the set CapSubgroup derives (Rust Validator::subgroup_operations); each
// operation outside it is SubgroupError::UnsupportedOperation with the
// class Rust requires for it (mod.rs:307 and :319 required_operations).
func TestSubgroupOperations(t *testing.T) {
	m := parse(t, subgroupShader)
	all := valid.SubgroupBallot | valid.SubgroupVote | valid.SubgroupArithmetic | valid.SubgroupShuffle |
		valid.SubgroupShuffleRelative | valid.SubgroupQuadFragmentCompute
	for _, tt := range []struct {
		class valid.SubgroupOperationSet
		name  string
	}{
		{valid.SubgroupBallot, "BALLOT"},
		{valid.SubgroupVote, "VOTE"},
		{valid.SubgroupArithmetic, "ARITHMETIC"},
		{valid.SubgroupShuffle, "SHUFFLE"},
		{valid.SubgroupShuffleRelative, "SHUFFLE_RELATIVE"},
		{valid.SubgroupQuadFragmentCompute, "QUAD_FRAGMENT_COMPUTE"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			expectRejected(t, m, valid.Options{Capabilities: valid.CapSubgroup, SubgroupOperations: all &^ tt.class},
				"Subgroup operation is invalid: Support for subgroup operation SubgroupOperationSet("+tt.name+") is required")
		})
	}
	expectValid(t, m, valid.Options{Capabilities: valid.CapSubgroup, SubgroupOperations: all})
	// The zero set is every operation, with CapSubgroup.
	expectValid(t, m, valid.Options{Capabilities: valid.CapSubgroup})
	// Operations alone enable nothing: SUBGROUP is still required.
	expectRejected(t, m, valid.Options{SubgroupOperations: all}, "Shader requires capability Capabilities(SUBGROUP)")

	barrier := parse(t, `@compute @workgroup_size(64) fn main() { subgroupBarrier(); }`)
	expectRejected(t, barrier, valid.Options{
		Capabilities:       valid.CapSubgroup | valid.CapSubgroupBarrier,
		SubgroupOperations: valid.SubgroupVote,
	}, "Support for subgroup operation SubgroupOperationSet(BASIC) is required")
}

// TestSubgroupStages: a non-zero Options.SubgroupStages replaces the stages
// CapSubgroup derives (Rust Validator::subgroup_stages); an entry point
// outside them that reaches a subgroup operation, directly or through a
// call, is EntryPointError::ForbiddenStageOperations.
func TestSubgroupStages(t *testing.T) {
	direct := parse(t, subgroupShader)
	expectRejected(t, direct, valid.Options{Capabilities: valid.CapSubgroup, SubgroupStages: valid.ShaderStagesFragment},
		"Entry point main at Compute is invalid: Uses operations forbidden at this stage")
	expectValid(t, direct, valid.Options{Capabilities: valid.CapSubgroup, SubgroupStages: valid.ShaderStagesCompute})

	called := parse(t, `enable subgroups;
fn f(i: u32) -> u32 { return subgroupAdd(i); }
@compute @workgroup_size(64) fn main(@builtin(local_invocation_index) i: u32) { _ = f(i); }
@fragment fn fs() -> @location(0) vec4f { return vec4f(f32(f(1u))); }`)
	expectRejected(t, called, valid.Options{Capabilities: valid.CapSubgroup, SubgroupStages: valid.ShaderStagesFragment},
		"Entry point main at Compute is invalid: Uses operations forbidden at this stage")
	expectValid(t, called, valid.Options{Capabilities: valid.CapSubgroup})
}

// TestCoreFeaturesNeedNoCapability: var<immediate> and texture_external are
// core WebGPU (the immediate-data feature, maxImmediateSize; WGSL
// texture_external), so they validate with the zero Options. Rust gates
// them behind IMMEDIATES (interface.rs:1106) and TEXTURE_EXTERNAL
// (type.rs:917) because wgpu ships them as native features; here
// Capabilities holds optional features only (ADR-010 N10).
func TestCoreFeaturesNeedNoCapability(t *testing.T) {
	m := parse(t, `struct P { scale: f32 }
var<immediate> p: P;
@group(0) @binding(0) var tex: texture_external;
@group(0) @binding(1) var samp: sampler;
@fragment fn main() -> @location(0) vec4f {
    return textureSampleBaseClampToEdge(tex, samp, vec2f(0.0)) * p.scale;
}`)
	expectValid(t, m, valid.Options{})
}

// TestShaderStagesAreRustBits pins ShaderStages and SubgroupOperationSet
// to Rust's bits (valid/mod.rs:335 and :279 at the pinned commit).
func TestShaderStagesAreRustBits(t *testing.T) {
	for _, tt := range []struct {
		name      string
		got, rust uint64
	}{
		{"VERTEX", uint64(valid.ShaderStagesVertex), 0x1},
		{"FRAGMENT", uint64(valid.ShaderStagesFragment), 0x2},
		{"COMPUTE", uint64(valid.ShaderStagesCompute), 0x4},
		{"MESH", uint64(valid.ShaderStagesMesh), 0x8},
		{"TASK", uint64(valid.ShaderStagesTask), 0x10},
		{"COMPUTE_LIKE", uint64(valid.ShaderStagesComputeLike), 0x4 | 0x10 | 0x8},
		{"BASIC", uint64(valid.SubgroupBasic), 1 << 0},
		{"VOTE", uint64(valid.SubgroupVote), 1 << 1},
		{"ARITHMETIC", uint64(valid.SubgroupArithmetic), 1 << 2},
		{"BALLOT", uint64(valid.SubgroupBallot), 1 << 3},
		{"SHUFFLE", uint64(valid.SubgroupShuffle), 1 << 4},
		{"SHUFFLE_RELATIVE", uint64(valid.SubgroupShuffleRelative), 1 << 5},
		{"QUAD_FRAGMENT_COMPUTE", uint64(valid.SubgroupQuadFragmentCompute), 1 << 7},
	} {
		if tt.got != tt.rust {
			t.Errorf("%s = %#x, Rust's is %#x", tt.name, tt.got, tt.rust)
		}
	}
}
