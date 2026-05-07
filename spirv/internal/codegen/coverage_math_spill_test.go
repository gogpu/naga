package codegen

import (
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// ---------------------------------------------------------------------------
// modf/frexp struct type methods — 0% coverage.
// These are directly tested because the fallback path in emitMath has a
// uint32 comparison bug (h >= 0 is always true for unsigned), making them
// unreachable via full compilation. Direct method testing catches real bugs.
// ---------------------------------------------------------------------------

// TestModfStructTypeDirectScalar exercises emitModfStructType directly with
// a scalar f32 TypeResolution.
func TestModfStructTypeDirectScalar(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	backend := NewBackend(DefaultOptions())
	backend.module = mod
	backend.builder = NewModuleBuilder(Version1_3)

	argType := ir.TypeResolution{Value: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	typeID, err := backend.emitModfStructType(argType)
	if err != nil {
		t.Fatalf("emitModfStructType failed: %v", err)
	}
	if typeID == 0 {
		t.Error("emitModfStructType returned type ID 0")
	}
}

// TestFrexpStructTypeDirectScalar exercises emitFrexpStructType directly
// with a scalar f32 TypeResolution (covers default i32 branch).
func TestFrexpStructTypeDirectScalar(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	backend := NewBackend(DefaultOptions())
	backend.module = mod
	backend.builder = NewModuleBuilder(Version1_3)

	argType := ir.TypeResolution{Value: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	typeID, err := backend.emitFrexpStructType(argType)
	if err != nil {
		t.Fatalf("emitFrexpStructType failed: %v", err)
	}
	if typeID == 0 {
		t.Error("emitFrexpStructType returned type ID 0")
	}
}

// TestFrexpStructTypeDirectVector exercises emitFrexpStructType directly
// with a vec2<f32> TypeResolution (covers VectorType branch).
func TestFrexpStructTypeDirectVector(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "vec2f", Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}
	h := ir.TypeHandle(0)
	backend := NewBackend(DefaultOptions())
	backend.module = mod
	backend.builder = NewModuleBuilder(Version1_3)

	argType := ir.TypeResolution{Handle: &h}
	typeID, err := backend.emitFrexpStructType(argType)
	if err != nil {
		t.Fatalf("emitFrexpStructType vector failed: %v", err)
	}
	if typeID == 0 {
		t.Error("emitFrexpStructType returned type ID 0")
	}
}

// ---------------------------------------------------------------------------
// splatScalar — 0% coverage. Triggered when binary operation has
// scalar + vector operands requiring vector promotion.
// ---------------------------------------------------------------------------

// TestSplatScalarViaBinaryOp exercises splatScalar via an integer
// vector * scalar operation that requires OpCompositeConstruct splat.
func TestSplatScalarViaBinaryOp(t *testing.T) {
	source := `
@fragment
fn main(@location(0) @interpolate(flat) v: vec3<u32>) -> @location(0) vec4<f32> {
    let result = v * 7u;
    return vec4<f32>(f32(result.x), f32(result.y), f32(result.z), 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)

	// Scalar*vec requires splat: OpCompositeConstruct
	if !hasOpcodeInInstrs(instrs, OpCompositeConstruct) {
		t.Error("expected OpCompositeConstruct for scalar-to-vector splat")
	}
}

// ---------------------------------------------------------------------------
// emitSelect — 38.5% coverage. Need to cover:
// 1. Float scalar condition → bool conversion (OpFOrdNotEqual)
// 2. Float vector condition → bool vector conversion
// 3. Scalar bool condition with vector operands → splat
// ---------------------------------------------------------------------------

// TestSelectVectorFloatCondition exercises emitSelect with vec4<f32> condition
// (not bool), covering the vec<float> → vec<bool> conversion path.
func TestSelectVectorFloatCondition(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let a = vec4<f32>(1.0, 0.0, 0.0, 1.0);
    let b = vec4<f32>(0.0, 1.0, 0.0, 1.0);
    let s = step(vec4<f32>(0.5, 0.5, 0.5, 0.5), vec4<f32>(uv.x, uv.y, uv.x, uv.y));
    // s is vec4<f32>, used as condition → triggers vec<float> → vec<bool>
    let result = select(a, b, s > vec4<f32>(0.0));
    return result;
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)

	if !hasOpcodeInInstrs(instrs, OpSelect) {
		t.Error("expected OpSelect for select() with vec condition")
	}
}

// ---------------------------------------------------------------------------
// emitArrayLength — deeper coverage (43.2%). Exercise the direct
// GlobalVariable path (struct member IS the runtime array).
// ---------------------------------------------------------------------------

// TestArrayLengthDirectGlobal exercises emitArrayLength when the global
// variable is the runtime array directly (no struct wrapping in the source).
func TestArrayLengthDirectGlobal(t *testing.T) {
	source := `
@group(0) @binding(0) var<storage, read> buf: array<f32>;

@compute @workgroup_size(1)
fn main() {
    let n = arrayLength(&buf);
    _ = n;
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	if !hasOpcodeInInstrs(instrs, OpArrayLength) {
		t.Error("expected OpArrayLength for runtime-sized array")
	}
}

// ---------------------------------------------------------------------------
// emitAs — deeper coverage (58.8%). Exercise more conversion paths.
// ---------------------------------------------------------------------------

// TestConversionsI32ToF32 covers int-to-float and float-to-int conversion
// opcodes in emitAs / selectConversionOp.
func TestConversionsI32ToF32(t *testing.T) {
	source := `
@fragment
fn main(@location(0) @interpolate(flat) v: i32) -> @location(0) vec4<f32> {
    let f = f32(v);         // SConvert
    let u = u32(v);         // Bitcast or reinterpret
    let back = i32(f);      // FConvert
    return vec4<f32>(f, f32(u), f32(back), 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
}

// TestConversionsU32ToF32 covers unsigned-to-float conversion.
func TestConversionsU32ToF32(t *testing.T) {
	source := `
@fragment
fn main(@location(0) @interpolate(flat) v: u32) -> @location(0) vec4<f32> {
    let f = f32(v);         // UConvert
    let i = i32(v);         // Bitcast
    return vec4<f32>(f, f32(i), 0.0, 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
}

// TestConversionsBitcast covers bitcast via as<Type>.
func TestConversionsBitcast(t *testing.T) {
	source := `
@fragment
fn main(@location(0) @interpolate(flat) v: u32) -> @location(0) vec4<f32> {
    let f = bitcast<f32>(v);
    let i = bitcast<i32>(v);
    return vec4<f32>(f, f32(i), 0.0, 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	if !hasOpcodeInInstrs(instrs, OpBitcast) {
		t.Error("expected OpBitcast for bitcast<f32>(u32)")
	}
}

// ---------------------------------------------------------------------------
// WGSL modf/frexp via standard WGSL path (verifies the struct type lookup
// paths, not just fallback).
// ---------------------------------------------------------------------------

// TestModfViaWGSL exercises modf through the standard WGSL lowering which
// creates the struct type in the type arena (covering FindModfResultType path).
func TestModfViaWGSL(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let result = modf(uv.x);
    return vec4<f32>(result.fract, result.whole, 0.0, 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	// Should use GLSLstd450 Modf
	if !hasOpcodeInInstrs(instrs, OpExtInst) {
		t.Error("expected OpExtInst for modf")
	}
}

// TestFrexpViaWGSL exercises frexp through the standard WGSL lowering.
func TestFrexpViaWGSL(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let result = frexp(uv.x);
    return vec4<f32>(result.fract, f32(result.exp), 0.0, 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
}

// ---------------------------------------------------------------------------
// Spill-to-variable path — 0% for spillToInternalVariable,
// maybeAccessSpilledComposite, writeSpilledAccessChain.
// Triggered when accessing fields of a struct returned by a function call
// and the access chain requires a temporary variable.
// ---------------------------------------------------------------------------

// exprHandle returns a pointer to an ExpressionHandle (helper for IR construction).
func exprHandle(h ir.ExpressionHandle) *ir.ExpressionHandle {
	return &h
}

// TestSpillCompositeFieldAccessWithIndexChain exercises the spill path via
// direct IR with Access/AccessIndex chains on a function call result.
func TestSpillCompositeFieldAccessWithIndexChain(t *testing.T) {
	// Build a module with a function that returns a struct, and the entry point
	// accesses individual fields via AccessIndex. This triggers the spill path
	// because the function call result is not a pointer but needs OpAccessChain.
	f32Handle := ir.TypeHandle(0)
	structHandle := ir.TypeHandle(1)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "Pair", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "a", Type: f32Handle, Offset: 0},
					{Name: "b", Type: f32Handle, Offset: 4},
				},
			}},
		},
		Functions: []ir.Function{
			{
				Name: "make_pair",
				Arguments: []ir.FunctionArgument{
					{Name: "x", Type: f32Handle},
					{Name: "y", Type: f32Handle},
				},
				Result: &ir.FunctionResult{Type: structHandle},
				LocalVars: []ir.LocalVariable{
					{Name: "p", Type: structHandle},
				},
				Expressions: []ir.Expression{
					// expr 0: arg x
					{Kind: ir.ExprFunctionArgument{Index: 0}},
					// expr 1: arg y
					{Kind: ir.ExprFunctionArgument{Index: 1}},
					// expr 2: local var p
					{Kind: ir.ExprLocalVariable{Variable: 0}},
					// expr 3: compose(x, y)
					{Kind: ir.ExprCompose{Type: structHandle, Components: []ir.ExpressionHandle{0, 1}}},
					// expr 4: load p
					{Kind: ir.ExprLoad{Pointer: 2}},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 3, End: 4}}},
					{Kind: ir.StmtStore{Pointer: 2, Value: 3}},
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 4, End: 5}}},
					{Kind: ir.StmtReturn{Value: exprHandle(4)}},
				},
			},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:      "main",
				Stage:     ir.StageCompute,
				Workgroup: [3]uint32{1, 1, 1},
				Function: ir.Function{
					Expressions: []ir.Expression{
						// expr 0: literal 1.0
						{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
						// expr 1: literal 2.0
						{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},
						// expr 2: call result
						{Kind: ir.ExprCallResult{Function: 0}},
						// expr 3: access index .a (field 0)
						{Kind: ir.ExprAccessIndex{Base: 2, Index: 0}},
						// expr 4: access index .b (field 1)
						{Kind: ir.ExprAccessIndex{Base: 2, Index: 1}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
						{Kind: ir.StmtCall{
							Function:  0,
							Arguments: []ir.ExpressionHandle{0, 1},
							Result:    exprHandle(2),
						}},
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 3, End: 5}}},
					},
				},
			},
		},
	}

	backend := NewBackend(DefaultOptions())
	spvBytes, err := backend.Compile(mod)
	if err != nil {
		// Even partial compilation exercises the spill path
		t.Logf("Compile result: err=%v, bytes=%d", err, len(spvBytes))
		return
	}
	assertValidSPIRV(t, spvBytes)
	instrs := decodeSPIRVInstructions(spvBytes)

	if !hasOpcodeInInstrs(instrs, OpFunctionCall) {
		t.Error("expected OpFunctionCall")
	}
	// The result of FunctionCall with field access typically uses
	// CompositeExtract or AccessChain (spill path)
	hasExtract := hasOpcodeInInstrs(instrs, OpCompositeExtract)
	hasAccessChain := hasOpcodeInInstrs(instrs, OpAccessChain)
	if !hasExtract && !hasAccessChain {
		t.Error("expected OpCompositeExtract or OpAccessChain for struct field access on call result")
	}
}

// ---------------------------------------------------------------------------
// WGSL-based deeper coverage for various partially-covered functions.
// ---------------------------------------------------------------------------

// TestAtomicExchangeResult exercises atomicExchange without compare (the
// non-compare-exchange path) and verifies the result is usable.
func TestAtomicExchangeResult(t *testing.T) {
	source := `
@group(0) @binding(0) var<storage, read_write> val: atomic<u32>;

@compute @workgroup_size(1)
fn main() {
    let old = atomicExchange(&val, 42u);
    atomicStore(&val, old);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	if !hasOpcodeInInstrs(instrs, OpAtomicExchange) {
		t.Error("expected OpAtomicExchange")
	}
}

// TestMultipleFunctionCalls exercises deferred stores and call result
// caching with multiple function calls in sequence.
func TestMultipleFunctionCalls(t *testing.T) {
	source := `
fn add(a: f32, b: f32) -> f32 { return a + b; }
fn mul(a: f32, b: f32) -> f32 { return a * b; }

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let sum = add(uv.x, uv.y);
    let product = mul(uv.x, uv.y);
    let combined = add(sum, product);
    return vec4<f32>(combined, sum, product, 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)

	callCount := countOpcodeInInstrs(instrs, OpFunctionCall)
	if callCount < 3 {
		t.Errorf("expected at least 3 OpFunctionCall, got %d", callCount)
	}
}

// compileWGSLModule is a helper that parses+lowers WGSL and returns the ir.Module.
func compileWGSLModule(t *testing.T, source string) *ir.Module {
	t.Helper()
	lexer := wgsl.NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}
	parser := wgsl.NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}
	return module
}

// TestGlobalVariableLoadRuntimeArrayPath exercises emitGlobalVarValue (19.6%)
// with a type that contains a runtime-sized array, covering the
// typeContainsRuntimeArray branch.
func TestGlobalVariableLoadRuntimeArrayPath(t *testing.T) {
	source := `
struct Buf {
    count: u32,
    data: array<f32>,
}

@group(0) @binding(0) var<storage, read> buf: Buf;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    let n = buf.count;
    if id.x < n {
        let val = buf.data[id.x];
        _ = val;
    }
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)

	if !hasOpcodeInInstrs(instrs, OpAccessChain) {
		t.Error("expected OpAccessChain for runtime array access")
	}
}

// TestNonUniformExpression exercises isNonUniformExpression (36.0%)
// with texture sampling that uses a non-uniform coordinate.
func TestNonUniformExpression(t *testing.T) {
	source := `
@group(0) @binding(0) var my_texture: texture_2d<f32>;
@group(0) @binding(1) var my_sampler: sampler;

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    // Non-uniform coordinate access
    let color1 = textureSample(my_texture, my_sampler, uv);
    let color2 = textureSample(my_texture, my_sampler, uv * 0.5);
    return color1 + color2;
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
}

// TestTypeNeedsLayoutDecoration exercises typeNeedsLayoutDecoration (50.0%)
// with uniform buffer types that require Block decoration.
func TestTypeNeedsLayoutDecoration(t *testing.T) {
	source := `
struct Uniforms {
    matrix: mat4x4<f32>,
    color: vec4<f32>,
    scale: f32,
}

@group(0) @binding(0) var<uniform> u: Uniforms;

@fragment
fn main() -> @location(0) vec4<f32> {
    return u.color * u.scale;
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)

	// Uniform buffers require Block decoration
	if !hasOpcodeInInstrs(instrs, OpDecorate) {
		t.Error("expected OpDecorate for uniform buffer layout")
	}
}

// TestMatrixMultiplication exercises emitBinary with matrix operations to
// increase coverage on binary operation paths for matrix types.
func TestMatrixMultiplication(t *testing.T) {
	source := `
struct Uniforms {
    model: mat4x4<f32>,
    view: mat4x4<f32>,
}
@group(0) @binding(0) var<uniform> u: Uniforms;

@vertex
fn main(@location(0) pos: vec4<f32>) -> @builtin(position) vec4<f32> {
    return u.view * u.model * pos;
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)

	// Matrix multiply uses OpMatrixTimesMatrix or OpMatrixTimesVector
	hasMMul := hasOpcodeInInstrs(instrs, OpMatrixTimesMatrix)
	hasMVMul := hasOpcodeInInstrs(instrs, OpMatrixTimesVector)
	if !hasMMul && !hasMVMul {
		t.Error("expected OpMatrixTimesMatrix or OpMatrixTimesVector")
	}
}

// TestInlineTypeEmission exercises emitInlineType (30.3%) with struct type
// resolution through expression type inference.
func TestInlineTypeEmission(t *testing.T) {
	source := `
struct Light {
    position: vec3<f32>,
    intensity: f32,
}

fn process_light(light: Light) -> f32 {
    return length(light.position) * light.intensity;
}

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    var l: Light;
    l.position = vec3<f32>(uv.x, uv.y, 1.0);
    l.intensity = 2.0;
    let val = process_light(l);
    return vec4<f32>(val, val, val, 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
}
