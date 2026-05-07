package codegen

import (
	"math"
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// Test: Pipeline constants with texture operations
// (covers adjustExprHandles image sample/load/query paths, adjustImageSampleLevel)
// =============================================================================

func TestIntegration7_PipelineConstantsWithTextureSample(t *testing.T) {
	src := `
override LOD_BIAS: f32 = 0.0;
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;
@fragment fn fs(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSampleLevel(tex, samp, uv, LOD_BIAS);
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"LOD_BIAS": 2.0}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, ".sample(")
	mustContainMSL(t, code, "level(")
}

func TestIntegration7_PipelineConstantsWithTextureLoad(t *testing.T) {
	src := `
override X_COORD: i32 = 0;
override Y_COORD: i32 = 0;
@group(0) @binding(0) var tex: texture_2d<f32>;
struct Out { v: vec4<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = textureLoad(tex, vec2(X_COORD, Y_COORD), 0i);
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"X_COORD": 10, "Y_COORD": 20}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, ".read(")
}

// =============================================================================
// Test: Pipeline constants with math operations
// (covers adjustExprHandles for ExprMath, ExprBinary, ExprUnary, ExprSelect)
// =============================================================================

func TestIntegration7_PipelineConstantsSelect(t *testing.T) {
	src := `
override USE_RED: bool = true;
struct Out { v: vec4<f32> };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    let r = vec4(1.0, 0.0, 0.0, 1.0);
    let g = vec4(0.0, 1.0, 0.0, 1.0);
    out.v = select(g, r, USE_RED);
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"USE_RED": 0.0}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "kernel")
}

func TestIntegration7_PipelineConstantsComplex(t *testing.T) {
	src := `
override SCALE: f32 = 1.0;
override OFFSET: f32 = 0.0;
struct In { v: f32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    let val = clamp(inp.v * SCALE + OFFSET, 0.0, 1.0);
    out.v = val;
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"SCALE": 2.5, "OFFSET": 0.1}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "2.5")
	mustContainMSL(t, code, "0.1")
	mustContainMSL(t, code, "clamp")
}

// =============================================================================
// Test: Pipeline constants with splat and compose
// (covers adjustExprHandles for ExprSplat, ExprCompose)
// =============================================================================

func TestIntegration7_PipelineConstantsSplat(t *testing.T) {
	src := `
override FILL: f32 = 0.5;
struct Out { v: vec4<f32> };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = vec4(FILL);
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"FILL": 0.75}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "0.75")
}

func TestIntegration7_PipelineConstantsCompose(t *testing.T) {
	src := `
override R: f32 = 1.0;
override G: f32 = 0.0;
override B: f32 = 0.0;
struct Out { v: vec3<f32> };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = vec3(R, G, B);
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"R": 0.2, "G": 0.4, "B": 0.6}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "0.2")
	mustContainMSL(t, code, "0.4")
	mustContainMSL(t, code, "0.6")
}

// =============================================================================
// Test: Pipeline constants with derivative operations
// (covers adjustExprHandles for ExprDerivative)
// =============================================================================

func TestIntegration7_PipelineConstantsDerivative(t *testing.T) {
	src := `
override SCALE: f32 = 1.0;
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;
@fragment fn fs(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let c = textureSample(tex, samp, uv * SCALE);
    return vec4(dpdx(c.x), dpdy(c.x), 0.0, 1.0);
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"SCALE": 2.0}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "dfdx")
	mustContainMSL(t, code, "dfdy")
}

// =============================================================================
// Test: Pipeline constants with array access
// (covers adjustExprHandles for ExprAccess, ExprAccessIndex, ExprArrayLength)
// =============================================================================

func TestIntegration7_PipelineConstantsArrayAccess(t *testing.T) {
	src := `
override IDX: u32 = 0u;
struct Data { vals: array<f32, 8> };
@group(0) @binding(0) var<storage, read> data: Data;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = data.vals[IDX];
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"IDX": 3}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "3u")
}

// =============================================================================
// Test: Pipeline constants with relational expressions
// (covers adjustExprHandles for ExprRelational)
// =============================================================================

func TestIntegration7_PipelineConstantsRelational(t *testing.T) {
	src := `
override THRESHOLD: f32 = 0.5;
struct In { v: vec4<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    if any(inp.v > vec4(THRESHOLD)) { out.v = 1u; }
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"THRESHOLD": 0.8}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "any")
	mustContainMSL(t, code, "0.8")
}

// =============================================================================
// Test: Pipeline constants with swizzle
// (covers adjustExprHandles for ExprSwizzle)
// =============================================================================

func TestIntegration7_PipelineConstantsSwizzle(t *testing.T) {
	src := `
override MULTIPLIER: f32 = 1.0;
struct In { v: vec4<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: vec2<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    let scaled = inp.v * MULTIPLIER;
    out.v = scaled.xy;
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"MULTIPLIER": 3.0}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "3.0")
}

// =============================================================================
// Test: Private variable with vector compose initializer
// (covers writeGlobalExpression compose vector path)
// =============================================================================

func TestIntegration7_PrivateVarVecCompose(t *testing.T) {
	src := `
var<private> dir: vec3<f32> = vec3(0.0, 1.0, 0.0);
struct Out { v: vec3<f32> };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = dir;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "float3(")
}

func TestIntegration7_PrivateVarMatCompose(t *testing.T) {
	src := `
var<private> identity: mat2x2<f32> = mat2x2(
    vec2(1.0, 0.0),
    vec2(0.0, 1.0)
);
struct Out { v: mat2x2<f32> };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = identity;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "float2x2(")
}

// =============================================================================
// Test: Private variable with zero-init vector (zero-arg constructor)
// (covers writeGlobalExpression 0-component Compose for vectors)
// =============================================================================

func TestIntegration7_PrivateVarZeroVec(t *testing.T) {
	src := `
var<private> origin: vec3<f32> = vec3(0.0, 0.0, 0.0);
struct Out { v: vec3<f32> };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = origin;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "float3(")
}

// =============================================================================
// Test: Float-to-int conversion for various types
// (covers f2iFunctionName, f2iClampBounds, writeF2IHelper paths)
// =============================================================================

func TestIntegration7_CastF32ToU32(t *testing.T) {
	src := `
struct In { v: f32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = u32(inp.v);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "naga_f2u")
}

func TestIntegration7_CastF32ToI32Vec(t *testing.T) {
	src := `
struct In { v: vec4<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: vec4<i32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = vec4<i32>(inp.v);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "naga_f2i")
}

// =============================================================================
// Test: Various MSL type names (covers writeTypeInnerName edge cases)
// =============================================================================

func TestIntegration7_TypeSampledTexture3D(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_3d<f32>;
@group(0) @binding(1) var samp: sampler;
struct Out { v: vec4<f32> };
@group(0) @binding(2) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = textureSampleLevel(tex, samp, vec3(0.5, 0.5, 0.5), 0.0);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "texture3d<float")
}

func TestIntegration7_TypeTextureCube(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_cube<f32>;
@group(0) @binding(1) var samp: sampler;
@fragment fn fs(@location(0) uv: vec3<f32>) -> @location(0) vec4<f32> {
    return textureSample(tex, samp, uv);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "texturecube<float")
}

// =============================================================================
// Test: WriteAccessChain struct member access through pointer
// (covers writeAccessChain StructType path and VectorType path)
// =============================================================================

func TestIntegration7_AccessChainStruct(t *testing.T) {
	src := `
struct A { x: f32, y: f32 };
struct B { a: A };
struct C { b: B };
@group(0) @binding(0) var<storage, read> data: C;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = data.b.a.x + data.b.a.y;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".b")
	mustContainMSL(t, code, ".a")
	mustContainMSL(t, code, ".x")
	mustContainMSL(t, code, ".y")
}

// =============================================================================
// Test: WriteExpressionKind — Load expression
// (covers writeLoad, writeUncheckedLoad)
// =============================================================================

func TestIntegration7_LoadExpression(t *testing.T) {
	src := `
struct Data { v: f32 };
@group(0) @binding(0) var<storage, read> data: Data;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    var local_copy: f32 = 0.0;
    local_copy = data.v;
    local_copy = local_copy * 2.0;
    out.v = local_copy;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "local_copy")
}

// =============================================================================
// Test: Texture dimensions of specific types for edge cases
// (covers writeImageQuerySize for various dims)
// =============================================================================

func TestIntegration7_TextureDimensions1D(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_1d<f32>;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = textureDimensions(tex);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "get_width()")
}

func TestIntegration7_TextureDimensionsCube(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_cube<f32>;
struct Out { v: vec2<u32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = textureDimensions(tex);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "get_width()")
}

func TestIntegration7_TextureNumSamples(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_multisampled_2d<f32>;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = textureNumSamples(tex);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "get_num_samples()")
}

// =============================================================================
// Test: Additional binary operations (covers writeBinary edge cases)
// =============================================================================

func TestIntegration7_BinaryBooleanOps(t *testing.T) {
	src := `
struct In { a: u32, b: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    let x = inp.a & inp.b;
    let y = inp.a | inp.b;
    let z = inp.a ^ inp.b;
    out.v = x + y + z;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "&")
	mustContainMSL(t, code, "|")
	mustContainMSL(t, code, "^")
}

func TestIntegration7_BinaryComparison(t *testing.T) {
	src := `
struct In { a: f32, b: f32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    var count: u32 = 0u;
    if inp.a == inp.b { count = count + 1u; }
    if inp.a != inp.b { count = count + 1u; }
    if inp.a < inp.b { count = count + 1u; }
    if inp.a > inp.b { count = count + 1u; }
    if inp.a <= inp.b { count = count + 1u; }
    if inp.a >= inp.b { count = count + 1u; }
    out.v = count;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "==")
	mustContainMSL(t, code, "!=")
}

// =============================================================================
// Test: Vertex shader with multiple outputs (covers write output struct)
// =============================================================================

func TestIntegration7_VertexMultipleOutputs(t *testing.T) {
	src := `
struct VertexOutput {
    @builtin(position) pos: vec4<f32>,
    @location(0) uv: vec2<f32>,
    @location(1) color: vec3<f32>,
    @location(2) normal: vec3<f32>,
};
@vertex fn vs(@builtin(vertex_index) idx: u32) -> VertexOutput {
    var out: VertexOutput;
    out.pos = vec4(0.0, 0.0, 0.0, 1.0);
    out.uv = vec2(0.0, 0.0);
    out.color = vec3(1.0, 0.0, 0.0);
    out.normal = vec3(0.0, 1.0, 0.0);
    return out;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "[[position]]")
	mustContainMSL(t, code, "[[user(loc0)")
	mustContainMSL(t, code, "[[user(loc1)")
	mustContainMSL(t, code, "[[user(loc2)")
}

// =============================================================================
// Test: Fragment shader with multiple inputs (covers input struct generation)
// =============================================================================

func TestIntegration7_FragmentMultipleInputs(t *testing.T) {
	src := `
struct FragInput {
    @location(0) uv: vec2<f32>,
    @location(1) color: vec3<f32>,
};
@fragment fn fs(input: FragInput) -> @location(0) vec4<f32> {
    return vec4(input.color * input.uv.x, 1.0);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "stage_in")
}

// =============================================================================
// Test: Pipeline constants evalBinaryOp coverage for div/sub/mul
// =============================================================================

func TestEvalBinaryOp(t *testing.T) {
	tests := []struct {
		name   string
		op     ir.BinaryOperator
		left   ir.LiteralValue
		right  ir.LiteralValue
		expect float64
		isNil  bool
	}{
		{"add_f32", ir.BinaryAdd, ir.LiteralF32(1.0), ir.LiteralF32(2.0), 3.0, false},
		{"sub_f32", ir.BinarySubtract, ir.LiteralF32(5.0), ir.LiteralF32(3.0), 2.0, false},
		{"mul_f32", ir.BinaryMultiply, ir.LiteralF32(3.0), ir.LiteralF32(4.0), 12.0, false},
		{"div_f32", ir.BinaryDivide, ir.LiteralF32(10.0), ir.LiteralF32(2.0), 5.0, false},
		{"div_by_zero", ir.BinaryDivide, ir.LiteralF32(1.0), ir.LiteralF32(0.0), 0, true},
		{"add_i32", ir.BinaryAdd, ir.LiteralI32(3), ir.LiteralI32(4), 7, false},
		{"mul_u32", ir.BinaryMultiply, ir.LiteralU32(5), ir.LiteralU32(6), 30, false},
		{"unsupported_op", ir.BinaryModulo, ir.LiteralF32(1.0), ir.LiteralF32(2.0), 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evalBinaryOp(tt.op, tt.left, tt.right)
			if tt.isNil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil")
			}
			// Verify approximate value
			f, ok := literalToFloat64(got)
			if !ok {
				t.Fatalf("result not convertible to float64: %v", got)
			}
			if f != tt.expect {
				t.Errorf("got %v, want %v", f, tt.expect)
			}
		})
	}
}

// =============================================================================
// Test: evalTypeConversion edge cases
// =============================================================================

func TestEvalTypeConversion(t *testing.T) {
	tests := []struct {
		name   string
		lit    ir.LiteralValue
		kind   ir.ScalarKind
		width  uint8
		isNil  bool
		verify func(ir.LiteralValue) bool
	}{
		{"f32_to_u32", ir.LiteralF32(42.9), ir.ScalarUint, 4, false,
			func(v ir.LiteralValue) bool { u, ok := v.(ir.LiteralU32); return ok && uint32(u) == 42 }},
		{"bool_true_to_f32", ir.LiteralBool(true), ir.ScalarFloat, 4, false,
			func(v ir.LiteralValue) bool { f, ok := v.(ir.LiteralF32); return ok && float32(f) == 1.0 }},
		{"bool_false_to_i32", ir.LiteralBool(false), ir.ScalarSint, 4, false,
			func(v ir.LiteralValue) bool { i, ok := v.(ir.LiteralI32); return ok && int32(i) == 0 }},
		{"f64_to_i32", ir.LiteralF64(99.9), ir.ScalarSint, 4, false,
			func(v ir.LiteralValue) bool { i, ok := v.(ir.LiteralI32); return ok && int32(i) == 99 }},
		{"u32_to_f64", ir.LiteralU32(100), ir.ScalarFloat, 8, false,
			func(v ir.LiteralValue) bool { f, ok := v.(ir.LiteralF64); return ok && float64(f) == 100.0 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evalTypeConversion(tt.lit, tt.kind, tt.width)
			if tt.isNil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil")
			}
			if !tt.verify(got) {
				t.Errorf("unexpected result: %v (%T)", got, got)
			}
		})
	}
}

// =============================================================================
// Test: literalToFloat64 edge cases
// =============================================================================

func TestLiteralToFloat64(t *testing.T) {
	tests := []struct {
		name  string
		lit   ir.LiteralValue
		value float64
		ok    bool
	}{
		{"f32", ir.LiteralF32(3.14), 3.14, true},
		{"f64", ir.LiteralF64(2.71), 2.71, true},
		{"i32", ir.LiteralI32(-5), -5, true},
		{"i64", ir.LiteralI64(123456789), 123456789, true},
		{"u32", ir.LiteralU32(42), 42, true},
		{"u64", ir.LiteralU64(999), 999, true},
		{"bool", ir.LiteralBool(true), 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := literalToFloat64(tt.lit)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && tt.name == "f32" {
				// float32 conversion loses precision
				if float32(got) != float32(tt.value) {
					t.Errorf("value = %v, want %v", got, tt.value)
				}
			} else if ok && got != tt.value {
				t.Errorf("value = %v, want %v", got, tt.value)
			}
		})
	}
}

// =============================================================================
// Test: scalarValueToLiteral edge cases (NaN, Inf)
// =============================================================================

func TestScalarValueToLiteral_EdgeCases(t *testing.T) {
	t.Run("nan_to_bool_false", func(t *testing.T) {
		// NaN should convert to false for bool
		got := scalarValueToLiteral(nan64(), ir.ScalarType{Kind: ir.ScalarBool, Width: 1})
		if b, ok := got.(ir.LiteralBool); !ok || bool(b) {
			t.Errorf("NaN -> bool should be false, got %v", got)
		}
	})
	t.Run("inf_to_i32_nil", func(t *testing.T) {
		got := scalarValueToLiteral(inf64(), ir.ScalarType{Kind: ir.ScalarSint, Width: 4})
		if got != nil {
			t.Errorf("Inf -> i32 should be nil, got %v", got)
		}
	})
	t.Run("inf_to_u32_nil", func(t *testing.T) {
		got := scalarValueToLiteral(inf64(), ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
		if got != nil {
			t.Errorf("Inf -> u32 should be nil, got %v", got)
		}
	})
	t.Run("negative_to_u32_nil", func(t *testing.T) {
		got := scalarValueToLiteral(-1.0, ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
		if got != nil {
			t.Errorf("-1.0 -> u32 should be nil, got %v", got)
		}
	})
}

func nan64() float64 {
	return math.NaN()
}

func inf64() float64 {
	return math.Inf(1)
}
