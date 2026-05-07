package codegen

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// Test: Pipeline constants with various operations
// =============================================================================

func TestIntegration_PipelineConstantsWithBinary(t *testing.T) {
	src := `
override A: f32 = 1.0;
override B: f32 = 2.0;
struct In { v: f32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = inp.v * A + B;
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"A": 5.0, "B": 10.0}
	code := compileWGSLWithOpts(t, src, opts)
	// A=5.0, B=10.0 should appear as constants.
	mustContainMSL(t, code, "5.0")
	mustContainMSL(t, code, "10.0")
}

func TestIntegration_PipelineConstantsI32(t *testing.T) {
	src := `
override COUNT: i32 = 4;
struct In { v: i32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: i32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = inp.v + COUNT;
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"COUNT": 8}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "8")
}

func TestIntegration_PipelineConstantsU32(t *testing.T) {
	src := `
override SIZE: u32 = 16u;
struct In { v: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = inp.v + SIZE;
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"SIZE": 32}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "32u")
}

func TestIntegration_PipelineConstantsBool(t *testing.T) {
	src := `
override ENABLED: bool = true;
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    if ENABLED { out.v = 1u; }
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"ENABLED": 0}
	code := compileWGSLWithOpts(t, src, opts)
	// ENABLED=0 means false.
	mustContainMSL(t, code, "false")
}

func TestIntegration_PipelineConstantsDefault(t *testing.T) {
	// Test without providing pipeline constants — use defaults.
	src := `
override SCALE: f32 = 2.0;
struct In { v: f32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = inp.v * SCALE; }
`
	opts := DefaultOptions()
	// No PipelineConstants set — use default value.
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "2.0")
}

// =============================================================================
// Test: More scalar value types for constant emission
// =============================================================================

func TestIntegration_ConstantF32Values(t *testing.T) {
	// Test f32 scalar value emission with various float patterns.
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Constants: []ir.Constant{
			{Name: "PI", Type: 0, Value: ir.ScalarValue{Bits: 0x40490FDB, Kind: ir.ScalarFloat}},      // 3.14159...
			{Name: "ZERO", Type: 0, Value: ir.ScalarValue{Bits: 0, Kind: ir.ScalarFloat}},             // 0.0
			{Name: "NEG_ONE", Type: 0, Value: ir.ScalarValue{Bits: 0xBF800000, Kind: ir.ScalarFloat}}, // -1.0
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "PI")
	mustContainMSL(t, result, "ZERO")
	mustContainMSL(t, result, "NEG_ONE")
}

func TestIntegration_ConstantI32Values(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		Constants: []ir.Constant{
			{Name: "POS", Type: 0, Value: ir.ScalarValue{Bits: 42, Kind: ir.ScalarSint}},
			{Name: "NEG", Type: 0, Value: ir.ScalarValue{Bits: 0xFFFFFFFFFFFFFFF6, Kind: ir.ScalarSint}}, // -10
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "42")
	mustContainMSL(t, result, "-10")
}

// =============================================================================
// Test: Struct with array (covers writeArrayWrapper, struct constant init)
// =============================================================================

func TestIntegration_StructWithArray(t *testing.T) {
	src := `
struct Data {
    items: array<f32, 8>,
    count: u32,
};
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read> data: Data;
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = data.items[data.count];
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "struct")
	mustContainMSL(t, code, "inner")
}

// =============================================================================
// Test: Multiple named overrides used in expressions
// =============================================================================

func TestIntegration_PipelineConstantsMultiple(t *testing.T) {
	src := `
override WIDTH: u32 = 8u;
override HEIGHT: u32 = 8u;
struct In { idx: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { x: u32, y: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.x = inp.idx % WIDTH;
    out.y = inp.idx / HEIGHT;
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"WIDTH": 16, "HEIGHT": 16}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "16u")
}

// =============================================================================
// Test: Access chain through buffer struct member (covers writeAccess deeper)
// =============================================================================

func TestIntegration_AccessChainDeep(t *testing.T) {
	src := `
struct Inner { vals: array<f32, 4> };
struct Outer { inner: Inner };
@group(0) @binding(0) var<storage, read> data: Outer;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    out.v = data.inner.vals[gid.x];
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".inner")
}

// =============================================================================
// Test: Matrix column access
// =============================================================================

func TestIntegration_MatrixColumnAccess(t *testing.T) {
	src := `
struct In { m: mat4x4<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: vec4<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = inp.m[2];
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "[2]")
}

// =============================================================================
// Test: Matrix element access
// =============================================================================

func TestIntegration_MatrixElementAccess(t *testing.T) {
	src := `
struct In { m: mat4x4<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = inp.m[1][2];
}
`
	code := compileWGSL(t, src)
	// Matrix access: m[1].z (column 1, row 2 = z component)
	mustContainMSL(t, code, "[1]")
}

// =============================================================================
// Test: Vector component access via index (covers writeAccess for vectors)
// =============================================================================

func TestIntegration_VectorComponentIndex(t *testing.T) {
	src := `
struct In { v: vec4<f32>, idx: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = inp.v[inp.idx];
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "[")
}

// =============================================================================
// Test: Struct member access via AccessIndex (covers writeAccessIndex)
// =============================================================================

func TestIntegration_StructMemberAccess(t *testing.T) {
	src := `
struct Vertex { pos: vec3<f32>, uv: vec2<f32> };
@group(0) @binding(0) var<storage, read> vtx: Vertex;
struct Out { p: vec3<f32>, u: vec2<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.p = vtx.pos;
    out.u = vtx.uv;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".pos")
	mustContainMSL(t, code, ".uv")
}

// =============================================================================
// Test: More atomic operations (covers writeAtomic variants)
// =============================================================================

func TestIntegration_AtomicMax(t *testing.T) {
	src := `
var<workgroup> wg: atomic<u32>;
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(64) fn main(@builtin(local_invocation_index) idx: u32) {
    out.v = atomicMax(&wg, idx);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "atomic_fetch_max_explicit")
}

func TestIntegration_AtomicMin(t *testing.T) {
	src := `
var<workgroup> wg: atomic<u32>;
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(64) fn main(@builtin(local_invocation_index) idx: u32) {
    out.v = atomicMin(&wg, idx);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "atomic_fetch_min_explicit")
}

func TestIntegration_AtomicAnd(t *testing.T) {
	src := `
var<workgroup> wg: atomic<u32>;
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(64) fn main(@builtin(local_invocation_index) idx: u32) {
    out.v = atomicAnd(&wg, 0xFFu);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "atomic_fetch_and_explicit")
}

func TestIntegration_AtomicOr(t *testing.T) {
	src := `
var<workgroup> wg: atomic<u32>;
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(64) fn main(@builtin(local_invocation_index) idx: u32) {
    out.v = atomicOr(&wg, 0x01u);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "atomic_fetch_or_explicit")
}

func TestIntegration_AtomicXor(t *testing.T) {
	src := `
var<workgroup> wg: atomic<u32>;
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(64) fn main(@builtin(local_invocation_index) idx: u32) {
    out.v = atomicXor(&wg, 0x01u);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "atomic_fetch_xor_explicit")
}

func TestIntegration_AtomicExchange(t *testing.T) {
	src := `
var<workgroup> wg: atomic<u32>;
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(64) fn main(@builtin(local_invocation_index) idx: u32) {
    out.v = atomicExchange(&wg, idx);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "atomic_exchange_explicit")
}

// =============================================================================
// Test: Signed atomic operations
// =============================================================================

func TestIntegration_AtomicI32Add(t *testing.T) {
	src := `
var<workgroup> wg: atomic<i32>;
struct Out { v: i32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(64) fn main(@builtin(local_invocation_index) idx: u32) {
    out.v = atomicAdd(&wg, 1i);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "atomic_fetch_add_explicit")
	mustContainMSL(t, code, "atomic_int")
}

// =============================================================================
// Test: Storage buffer atomic (covers isAtomicPointer with storage)
// =============================================================================

func TestIntegration_StorageAtomicAdd(t *testing.T) {
	src := `
struct Counter { val: atomic<u32> };
@group(0) @binding(0) var<storage, read_write> counter: Counter;
@compute @workgroup_size(1) fn main() {
    atomicAdd(&counter.val, 1u);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "atomic_fetch_add_explicit")
}

// =============================================================================
// Test: Literal integer widths (covers writeLiteral branches)
// =============================================================================

func TestIntegration_LiteralI64(t *testing.T) {
	tI64 := ir.TypeHandle(0)
	retExpr := ir.ExpressionHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 8}},
		},
		Functions: []ir.Function{
			{
				Name:   "test_fn",
				Result: &ir.FunctionResult{Type: tI64},
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralI64(1000)}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tI64},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "1000")
}

func TestIntegration_LiteralF16(t *testing.T) {
	tF16 := ir.TypeHandle(0)
	retExpr := ir.ExpressionHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}},
		},
		Functions: []ir.Function{
			{
				Name:   "test_fn",
				Result: &ir.FunctionResult{Type: tF16},
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralF16(0x3C00)}}, // 1.0 in f16
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tF16},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "half")
}

// =============================================================================
// Test: wrappedScalarSuffix coverage
// =============================================================================

func TestIntegration_WrappedScalarSuffix(t *testing.T) {
	tests := []struct {
		name   string
		scalar ir.ScalarType
		want   string
	}{
		{"i32", ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, "i32"},
		{"u32", ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, "u32"},
		{"f32", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, "f32"},
		{"f16", ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}, "f16"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrappedScalarSuffix(tt.scalar)
			if got != tt.want {
				t.Errorf("wrappedScalarSuffix(%+v) = %q, want %q", tt.scalar, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Test: Texture 2D array textureDimensions
// =============================================================================

func TestIntegration_Texture2DArrayDimensions(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d_array<f32>;
struct Out { v: vec2<u32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = textureDimensions(tex); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".get_width(")
	mustContainMSL(t, code, ".get_height(")
}

func TestIntegration_Texture2DArrayLayers(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d_array<f32>;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = textureNumLayers(tex); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".get_array_size(")
}

// =============================================================================
// Test: Texture 3D dimensions
// =============================================================================

func TestIntegration_Texture3DDimensions(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_3d<f32>;
struct Out { v: vec3<u32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = textureDimensions(tex); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".get_width(")
	mustContainMSL(t, code, ".get_depth(")
}

// =============================================================================
// Test: Texture cube dimensions
// =============================================================================

func TestIntegration_TextureCubeDimensions(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_cube<f32>;
struct Out { v: vec2<u32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = textureDimensions(tex); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".get_width(")
}

// =============================================================================
// Test: RZSW bounds checking mode (covers writeStore, buildRZSWBoundsCheck)
// =============================================================================

func TestIntegration_BoundsCheckRZSW(t *testing.T) {
	src := `
struct Data { arr: array<f32, 16> };
@group(0) @binding(0) var<storage, read_write> data: Data;
@compute @workgroup_size(1) fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    data.arr[gid.x] = 1.0f;
}
`
	opts := DefaultOptions()
	opts.BoundsCheckPolicies = BoundsCheckPolicies{
		Index:  BoundsCheckReadZeroSkipWrite,
		Buffer: BoundsCheckReadZeroSkipWrite,
		Image:  BoundsCheckReadZeroSkipWrite,
	}
	code := compileWGSLWithOpts(t, src, opts)
	// RZSW should emit bounds check for the store.
	if !strings.Contains(code, "if (") && !strings.Contains(code, "DefaultConstructible") {
		t.Log("MSL output:\n", code)
	}
	mustContainMSL(t, code, "DefaultConstructible")
}

// =============================================================================
// Test: Multiple return values from fragment shader
// =============================================================================

func TestIntegration_MultipleOutputLocations(t *testing.T) {
	src := `
struct FragOutput {
    @location(0) color: vec4<f32>,
    @location(1) normal: vec4<f32>,
};
@fragment fn fs(@builtin(position) pos: vec4<f32>) -> FragOutput {
    var out: FragOutput;
    out.color = vec4(1.0, 0.0, 0.0, 1.0);
    out.normal = vec4(0.0, 1.0, 0.0, 1.0);
    return out;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "[[color(0)]]")
	mustContainMSL(t, code, "[[color(1)]]")
}

// =============================================================================
// Test: Fragment shader with depth output
// =============================================================================

func TestIntegration_FragmentDepthOutput(t *testing.T) {
	src := `
struct FragOutput {
    @location(0) color: vec4<f32>,
    @builtin(frag_depth) depth: f32,
};
@fragment fn fs(@builtin(position) pos: vec4<f32>) -> FragOutput {
    var out: FragOutput;
    out.color = vec4(1.0);
    out.depth = 0.5;
    return out;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "[[color(0)]]")
	mustContainMSL(t, code, "[[depth(any)]]")
}

// =============================================================================
// Test: Vertex shader with invariant position
// =============================================================================

func TestIntegration_InvariantPosition(t *testing.T) {
	src := `
@vertex fn vs(@builtin(vertex_index) idx: u32) -> @invariant @builtin(position) vec4<f32> {
    return vec4(0.0, 0.0, 0.0, 1.0);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "[[position, invariant]]")
}

// =============================================================================
// Test: Vertex with sample_mask output
// =============================================================================

func TestIntegration_SampleMaskOutput(t *testing.T) {
	src := `
struct FragOutput {
    @location(0) color: vec4<f32>,
    @builtin(sample_mask) mask: u32,
};
@fragment fn fs(@builtin(position) pos: vec4<f32>) -> FragOutput {
    var out: FragOutput;
    out.color = vec4(1.0);
    out.mask = 0xFFu;
    return out;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "[[sample_mask]]")
}

// =============================================================================
// Test: Workgroup zero-init with struct type
// =============================================================================

func TestIntegration_WorkgroupZeroInitStruct(t *testing.T) {
	src := `
struct SharedData {
    value: f32,
    count: u32,
};
var<workgroup> shared: SharedData;
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;

@compute @workgroup_size(64) fn main(@builtin(local_invocation_index) lid: u32) {
    shared.value = f32(lid);
    shared.count = lid;
    workgroupBarrier();
    out.v = shared.value;
}
`
	opts := DefaultOptions()
	opts.ZeroInitializeWorkgroupMemory = true
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "threadgroup")
	mustContainMSL(t, code, "threadgroup_barrier")
}

// =============================================================================
// Test: WriteFunctions (non-entry-point functions with various patterns)
// =============================================================================

func TestIntegration_MultipleFunctions(t *testing.T) {
	src := `
fn square(x: f32) -> f32 { return x * x; }
fn cube(x: f32) -> f32 { return x * square(x); }
fn dist(a: vec3<f32>, b: vec3<f32>) -> f32 { return length(a - b); }

struct In { a: vec3<f32>, b: vec3<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    let d = dist(inp.a, inp.b);
    out.v = cube(d);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "square(")
	mustContainMSL(t, code, "cube(")
	mustContainMSL(t, code, "dist(")
}

// =============================================================================
// Test: Recursive function calling (covers writeFunctions iteration)
// =============================================================================

func TestIntegration_FunctionWithMultipleArgs(t *testing.T) {
	src := `
fn lerp3(a: vec3<f32>, b: vec3<f32>, t: f32) -> vec3<f32> {
    return a * (1.0 - t) + b * t;
}

struct In { a: vec3<f32>, b: vec3<f32>, t: f32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: vec3<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = lerp3(inp.a, inp.b, inp.t);
}
`
	code := compileWGSL(t, src)
	// Name may be sanitized (e.g., lerp3_) to avoid MSL keyword conflicts.
	if !strings.Contains(code, "lerp3(") && !strings.Contains(code, "lerp3_(") {
		t.Error("Expected lerp3 or lerp3_ function call in output")
	}
}
