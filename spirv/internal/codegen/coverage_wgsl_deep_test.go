package codegen

import (
	"testing"
)

// ---------------------------------------------------------------------------
// WGSL-based tests to increase coverage on partially-covered functions.
// Each test targets a specific uncovered code path in the SPIR-V backend.
// ---------------------------------------------------------------------------

// TestAtomicAllRMWOperations exercises emitAtomic (48.5%) with every RMW
// variant to increase coverage of the atomic opcode selection and result paths.
func TestAtomicAllRMWOperations(t *testing.T) {
	source := `
@group(0) @binding(0) var<storage, read_write> val: atomic<u32>;

@compute @workgroup_size(1)
fn main() {
    let old_add = atomicAdd(&val, 1u);
    let old_sub = atomicSub(&val, 1u);
    let old_max = atomicMax(&val, 100u);
    let old_min = atomicMin(&val, 0u);
    let old_and = atomicAnd(&val, 0xFFu);
    let old_or = atomicOr(&val, 0x0Fu);
    let old_xor = atomicXor(&val, 0xAAu);
    let old_xchg = atomicExchange(&val, 42u);
    // Use all results to prevent DCE
    atomicStore(&val, old_add + old_sub + old_max + old_min + old_and + old_or + old_xor + old_xchg);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)

	expectedOps := []OpCode{
		OpAtomicIAdd, OpAtomicISub, OpAtomicUMax, OpAtomicUMin,
		OpAtomicAnd, OpAtomicOr, OpAtomicXor, OpAtomicExchange,
	}
	for _, op := range expectedOps {
		if !hasOpcodeInInstrs(instrs, op) {
			t.Errorf("expected %v in atomic RMW output", op)
		}
	}
}

// TestAtomicI32AllOps exercises emitAtomic with signed i32 atomics for
// the signed-integer opcode selection path (OpAtomicSMax, OpAtomicSMin).
func TestAtomicI32AllOps(t *testing.T) {
	source := `
@group(0) @binding(0) var<storage, read_write> val: atomic<i32>;

@compute @workgroup_size(1)
fn main() {
    let v1 = atomicMax(&val, 100i);
    let v2 = atomicMin(&val, -100i);
    let v3 = atomicAdd(&val, 1i);
    atomicStore(&val, v1 + v2 + v3);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)

	if !hasOpcodeInInstrs(instrs, OpAtomicSMax) {
		t.Error("expected OpAtomicSMax for signed i32 atomicMax")
	}
	if !hasOpcodeInInstrs(instrs, OpAtomicSMin) {
		t.Error("expected OpAtomicSMin for signed i32 atomicMin")
	}
}

// TestSelectScalarBoolVecOperands exercises emitSelect (38.5%) where
// the condition is a scalar bool but the operands are vectors, triggering
// the bool-to-vector splat path.
func TestSelectScalarBoolVecOperands(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let a = vec4<f32>(1.0, 0.0, 0.0, 1.0);
    let b = vec4<f32>(0.0, 1.0, 0.0, 1.0);
    // Scalar bool condition with vec4 operands → bool splat to vec4<bool>
    let flag = uv.x > 0.5;
    return select(a, b, flag);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	if !hasOpcodeInInstrs(instrs, OpSelect) {
		t.Error("expected OpSelect for scalar bool + vec4 operands")
	}
	// The splat should produce an OpCompositeConstruct for the bool vector
	if !hasOpcodeInInstrs(instrs, OpCompositeConstruct) {
		t.Error("expected OpCompositeConstruct for bool-to-vector splat")
	}
}

// TestAccessDynamicIndex exercises emitAccess (50%) with runtime-computed
// array indices to cover the dynamic index path.
func TestAccessDynamicIndex(t *testing.T) {
	source := `
struct Data {
    values: array<f32, 16>,
}
@group(0) @binding(0) var<uniform> data: Data;

@fragment
fn main(@location(0) @interpolate(flat) idx: u32) -> @location(0) vec4<f32> {
    let v0 = data.values[idx];
    let v1 = data.values[idx + 1u];
    return vec4<f32>(v0, v1, 0.0, 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	if !hasOpcodeInInstrs(instrs, OpAccessChain) {
		t.Error("expected OpAccessChain for dynamic array index")
	}
}

// TestConversionAllPaths exercises emitAs / selectConversionOp (58.8% / 60%)
// with a wider range of type conversions.
func TestConversionAllPaths(t *testing.T) {
	source := `
@fragment
fn main(@location(0) @interpolate(flat) u: u32) -> @location(0) vec4<f32> {
    // u32 → f32 (ConvertUToF)
    let f_from_u = f32(u);
    // u32 → i32 (Bitcast for same-width reinterpret)
    let i_from_u = i32(u);
    // i32 → f32 (ConvertSToF)
    let f_from_i = f32(i_from_u);
    // f32 → u32 (ConvertFToU)
    let u_from_f = u32(f_from_u);
    // f32 → i32 (ConvertFToS)
    let i_from_f = i32(f_from_u);
    // bool → u32 (Select)
    let b = u > 10u;
    let u_from_b = select(0u, 1u, b);
    return vec4<f32>(f_from_u, f_from_i, f32(u_from_f), f32(i_from_f + i32(u_from_b)));
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	if !hasOpcodeInInstrs(instrs, OpConvertUToF) {
		t.Error("expected OpConvertUToF")
	}
	if !hasOpcodeInInstrs(instrs, OpConvertSToF) {
		t.Error("expected OpConvertSToF")
	}
}

// TestVectorConversions exercises emitAs with vector type conversions.
func TestVectorConversions(t *testing.T) {
	source := `
@fragment
fn main(@location(0) @interpolate(flat) v: vec4<u32>) -> @location(0) vec4<f32> {
    let fv = vec4<f32>(v);          // vec4<u32> → vec4<f32>
    let iv = vec4<i32>(v);          // vec4<u32> → vec4<i32>
    let fv2 = vec4<f32>(iv);        // vec4<i32> → vec4<f32>
    return fv + fv2;
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
}

// TestConstExpressionNested exercises emitConstExpression (34.1%) with
// nested const expressions (ZeroValue, Literal, Compose).
func TestConstExpressionNested(t *testing.T) {
	source := `
const OFFSET: vec2<i32> = vec2<i32>(3, -2);

@group(0) @binding(0) var my_texture: texture_2d<f32>;
@group(0) @binding(1) var my_sampler: sampler;

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSample(my_texture, my_sampler, uv, OFFSET);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
}

// TestFunctionCallWithStructArg exercises emitCall (65.6%) with a struct
// argument and deferred store paths.
func TestFunctionCallWithStructArg(t *testing.T) {
	source := `
struct Params {
    scale: f32,
    offset: f32,
}

fn transform(p: Params, v: f32) -> f32 {
    return v * p.scale + p.offset;
}

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    var params: Params;
    params.scale = 2.0;
    params.offset = 0.5;
    let r = transform(params, uv.x);
    let g = transform(params, uv.y);
    return vec4<f32>(r, g, 0.0, 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	callCount := countOpcodeInInstrs(instrs, OpFunctionCall)
	if callCount < 2 {
		t.Errorf("expected at least 2 OpFunctionCall, got %d", callCount)
	}
}

// TestGlobalVarLoadWithSampler exercises emitGlobalVarValue (19.6%) with
// sampler and texture globals which have different load semantics.
func TestGlobalVarLoadWithSampler(t *testing.T) {
	source := `
@group(0) @binding(0) var my_texture: texture_2d<f32>;
@group(0) @binding(1) var my_sampler: sampler;
@group(0) @binding(2) var<uniform> scale: f32;

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let color = textureSample(my_texture, my_sampler, uv);
    return color * scale;
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
}

// TestMultipleStorageBuffers exercises emitGlobals (62.5%) and
// typeNeedsLayoutDecoration (50.0%) with multiple storage buffers.
func TestMultipleStorageBuffers(t *testing.T) {
	source := `
struct InputData {
    values: array<vec4<f32>>,
}

struct OutputData {
    results: array<vec4<f32>>,
}

@group(0) @binding(0) var<storage, read> input: InputData;
@group(0) @binding(1) var<storage, read_write> output: OutputData;

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    let val = input.values[id.x];
    output.results[id.x] = val * 2.0;
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
}

// TestDeeperArrayAccess exercises emitAccess and emitAccessIndex with
// nested struct and array access patterns.
func TestDeeperArrayAccess(t *testing.T) {
	source := `
struct Inner {
    x: f32,
    y: f32,
}

struct Outer {
    items: array<Inner, 4>,
    count: u32,
}

@group(0) @binding(0) var<uniform> data: Outer;

@fragment
fn main(@location(0) @interpolate(flat) idx: u32) -> @location(0) vec4<f32> {
    let item = data.items[idx];
    return vec4<f32>(item.x, item.y, f32(data.count), 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	if !hasOpcodeInInstrs(instrs, OpAccessChain) {
		t.Error("expected OpAccessChain for nested struct/array access")
	}
}

// TestMaybeCopyLogical exercises maybeCopyLogicalForStore (44.1%) via
// struct assignment through a storage buffer.
func TestMaybeCopyLogical(t *testing.T) {
	source := `
struct Data {
    a: vec4<f32>,
    b: vec4<f32>,
}

@group(0) @binding(0) var<storage, read_write> buf: Data;

@compute @workgroup_size(1)
fn main() {
    var local: Data;
    local.a = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    local.b = vec4<f32>(5.0, 6.0, 7.0, 8.0);
    buf = local;
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
}

// TestCompareExpressions exercises emitBinary with comparison operations
// to increase coverage of the comparison opcode selection path.
func TestCompareExpressions(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    var r = 0.0;
    if uv.x > 0.1 { r += 0.1; }
    if uv.x < 0.9 { r += 0.1; }
    if uv.x >= 0.5 { r += 0.1; }
    if uv.x <= 0.5 { r += 0.1; }
    if uv.x == 0.5 { r += 0.1; }
    if uv.x != 0.5 { r += 0.1; }
    return vec4<f32>(r, r, r, 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
}

// TestIntegerCompareExpressions exercises comparison with unsigned integers.
func TestIntegerCompareExpressions(t *testing.T) {
	source := `
@fragment
fn main(@location(0) @interpolate(flat) v: u32) -> @location(0) vec4<f32> {
    var r = 0.0;
    if v > 10u { r += 0.1; }
    if v < 100u { r += 0.1; }
    if v >= 50u { r += 0.1; }
    if v <= 50u { r += 0.1; }
    if v == 42u { r += 0.1; }
    if v != 0u { r += 0.1; }
    return vec4<f32>(r, r, r, 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
}

// TestSignedIntegerCompare exercises comparison with signed integers.
func TestSignedIntegerCompare(t *testing.T) {
	source := `
@fragment
fn main(@location(0) @interpolate(flat) v: i32) -> @location(0) vec4<f32> {
    var r = 0.0;
    if v > -10i { r += 0.1; }
    if v < 100i { r += 0.1; }
    return vec4<f32>(r, r, r, 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
}

// TestUnaryOperations exercises emitUnary for not/negate operations.
func TestUnaryOperations(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let neg = -uv.x;
    let flag = !(uv.x > 0.5);
    let result = select(neg, uv.x, flag);
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	if !hasOpcodeInInstrs(instrs, OpFNegate) {
		t.Error("expected OpFNegate for unary negation")
	}
}

// TestBitwiseNotOperation exercises emitUnary with bitwise NOT on integers.
func TestBitwiseNotOperation(t *testing.T) {
	source := `
@fragment
fn main(@location(0) @interpolate(flat) v: u32) -> @location(0) vec4<f32> {
    let inverted = ~v;
    return vec4<f32>(f32(inverted), 0.0, 0.0, 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	if !hasOpcodeInInstrs(instrs, OpNot) {
		t.Error("expected OpNot for bitwise NOT")
	}
}

// TestDeepSwitch exercises emitSwitch more deeply with fallthrough-like
// patterns and larger case counts.
func TestDeepSwitch(t *testing.T) {
	source := `
@fragment
fn main(@location(0) @interpolate(flat) mode: i32) -> @location(0) vec4<f32> {
    var color = vec4<f32>(0.0);
    switch mode {
        case 0i: { color.x = 1.0; }
        case 1i: { color.y = 1.0; }
        case 2i: { color.z = 1.0; }
        case 3i: { color.w = 1.0; }
        case 4i: { color = vec4<f32>(0.5); }
        case 5i, 6i: { color = vec4<f32>(0.25); }
        default: { color = vec4<f32>(1.0); }
    }
    return color;
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
}

// TestSwizzlePatterns exercises emitSwizzle with various component patterns.
func TestSwizzlePatterns(t *testing.T) {
	source := `
@fragment
fn main(@location(0) color: vec4<f32>) -> @location(0) vec4<f32> {
    let rg = color.rg;
    let bgr = color.bgr;
    let aaaa = color.wwww;
    return vec4<f32>(rg.x + bgr.x, rg.y + bgr.y, bgr.z, aaaa.x);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	shuffleCount := countOpcodeInInstrs(instrs, OpVectorShuffle)
	if shuffleCount < 2 {
		t.Errorf("expected at least 2 OpVectorShuffle for swizzle patterns, got %d", shuffleCount)
	}
}

// TestDepthTextureSampling exercises image operations with depth textures
// to cover depth-specific paths in the image sampling code.
func TestDepthTextureSampling(t *testing.T) {
	source := `
@group(0) @binding(0) var depth_tex: texture_depth_2d;
@group(0) @binding(1) var depth_sampler: sampler_comparison;

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let depth = textureSampleCompare(depth_tex, depth_sampler, uv, 0.5);
    return vec4<f32>(depth, depth, depth, 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
}

// TestTextureGather exercises image gather operations.
func TestTextureGather(t *testing.T) {
	source := `
@group(0) @binding(0) var my_texture: texture_2d<f32>;
@group(0) @binding(1) var my_sampler: sampler;

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureGather(0, my_texture, my_sampler, uv);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
}

// TestF16PolyfillPath exercises getF16PolyfillTypeID (40.0%) by using
// pack2x16float/unpack2x16float which may trigger f16 polyfill.
func TestF16PolyfillPath(t *testing.T) {
	source := `
@fragment
fn main(@location(0) @interpolate(flat) v: u32) -> @location(0) vec4<f32> {
    let unpacked = unpack2x16float(v);
    let repacked = pack2x16float(unpacked);
    return vec4<f32>(unpacked, f32(repacked), 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
}

// TestScalarConstantEmission exercises emitScalarConstant (54.5%) with
// various scalar literal types used in expressions.
func TestScalarConstantEmission(t *testing.T) {
	source := `
@fragment
fn main(@location(0) @interpolate(flat) v: u32) -> @location(0) vec4<f32> {
    let a = 3.14159;
    let b = 2.71828;
    let c = 0.0;
    let d = 1.0;
    let e = -1.0;
    return vec4<f32>(a, b, c + d + e + f32(v), 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
}

// TestStorageTextureMultipleFormats exercises requestImageFormatCapabilities (50%)
// with different storage texture formats.
func TestStorageTextureMultipleFormats(t *testing.T) {
	tests := []struct {
		name   string
		format string
	}{
		{"r32uint", "r32uint"},
		{"r32sint", "r32sint"},
		{"r32float", "r32float"},
		{"rgba8unorm", "rgba8unorm"},
		{"rgba16float", "rgba16float"},
		{"rgba32float", "rgba32float"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := `
@group(0) @binding(0) var output: texture_storage_2d<` + tt.format + `, write>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    textureStore(output, vec2<i32>(vec2<u32>(id.x, id.y)), vec4<f32>(1.0, 0.0, 0.0, 1.0));
}
`
			spv := compileWGSL(t, source)
			assertValidSPIRV(t, spv)
		})
	}
}
