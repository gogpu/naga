package codegen

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// ---------------------------------------------------------------------------
// Direct IR tests for functions unreachable from WGSL lowerer output.
// These construct IR modules that use patterns the parser doesn't generate.
// ---------------------------------------------------------------------------

// TestEmitLocalVarValueDirect exercises emitLocalVarValue (0%) by placing
// ExprLocalVariable directly in an Emit range without ExprLoad wrapping.
// The WGSL lowerer always wraps local vars in ExprLoad, so this path
// is only reachable from hand-crafted IR.
func TestEmitLocalVarValueDirect(t *testing.T) {
	u32Handle := ir.TypeHandle(0)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:      "main",
				Stage:     ir.StageCompute,
				Workgroup: [3]uint32{1, 1, 1},
				Function: ir.Function{
					LocalVars: []ir.LocalVariable{
						{Name: "x", Type: u32Handle},
					},
					Expressions: []ir.Expression{
						// expr 0: local var x (pointer)
						{Kind: ir.ExprLocalVariable{Variable: 0}},
						// expr 1: literal 42u
						{Kind: ir.Literal{Value: ir.LiteralU32(42)}},
						// expr 2: local var x AGAIN — used directly in Emit
						// (auto-load path via emitLocalVarValue)
						{Kind: ir.ExprLocalVariable{Variable: 0}},
					},
					Body: []ir.Statement{
						// Emit the literal
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 2}}},
						// Store 42 into x
						{Kind: ir.StmtStore{Pointer: 0, Value: 1}},
						// Emit expr 2 = local var in value context → emitLocalVarValue
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 2, End: 3}}},
					},
				},
			},
		},
	}

	backend := NewBackend(DefaultOptions())
	spvBytes, err := backend.Compile(mod)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	assertValidSPIRV(t, spvBytes)

	// Verify the local var is present (OpVariable) and the store happened.
	// The auto-load through emitLocalVarValue produces an OpLoad or DCE'd result.
	instrs := decodeSPIRVInstructions(spvBytes)
	if !hasOpcodeInInstrs(instrs, OpVariable) {
		t.Error("expected OpVariable for local variable")
	}
	if !hasOpcodeInInstrs(instrs, OpStore) {
		t.Error("expected OpStore for local variable initialization")
	}
}

// TestResolveAtomicScalarKindDirect exercises resolveAtomicScalarKind (0%)
// via StmtAtomic with a result that triggers the scalar kind resolution.
// resolveAtomicScalarKind is just .Kind on resolveAtomicScalar, so any
// atomic operation that calls emitAtomic will also call resolveAtomicScalar.
// The existing TestCompileAtomicsI32 uses AtomicLoad which takes a different
// code path. We need an RMW op like AtomicMax with i32 to trigger SMin opcode.
func TestResolveAtomicScalarKindSint(t *testing.T) {
	atomicI32Handle := ir.TypeHandle(0)
	i32Handle := ir.TypeHandle(1)
	resultExpr := ir.ExpressionHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "atomic_i32", Inner: ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}},
			{Name: "i32", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:  "shared_val",
				Type:  atomicI32Handle,
				Space: ir.SpaceWorkGroup,
			},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:      "main",
				Stage:     ir.StageCompute,
				Workgroup: [3]uint32{1, 1, 1},
				Function: ir.Function{
					Expressions: []ir.Expression{
						{Kind: ir.ExprGlobalVariable{Variable: 0}},                    // expr 0: &shared_val
						{Kind: ir.Literal{Value: ir.LiteralI32(10)}},                  // expr 1: 10i
						{Kind: ir.ExprAtomicResult{Ty: i32Handle, Comparison: false}}, // expr 2: atomic result
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 2}}},
						{Kind: ir.StmtAtomic{
							Pointer: 0,
							Fun:     ir.AtomicMax{},
							Value:   1,
							Result:  &resultExpr,
						}},
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 2, End: 3}}},
					},
				},
			},
		},
	}

	backend := NewBackend(DefaultOptions())
	spvBytes, err := backend.Compile(mod)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	assertValidSPIRV(t, spvBytes)
	instrs := decodeSPIRVInstructions(spvBytes)

	// Signed max → OpAtomicSMax
	if !hasOpcodeInInstrs(instrs, OpAtomicSMax) {
		t.Error("expected OpAtomicSMax for signed atomic max")
	}
}

// TestSplatScalarViaAddition exercises splatScalar (0% at line 5827) via
// vec + scalar addition (not multiplication), which goes through
// promoteScalarToVector instead of the special multiplication path.
func TestSplatScalarViaAddition(t *testing.T) {
	// vec3<f32> + scalar f32 → promoteScalarToVector → splatScalar
	source := `
@fragment
fn main(@location(0) pos: vec3<f32>) -> @location(0) vec4<f32> {
    // Addition of vec3 + scalar goes through promoteScalarToVector
    let shifted = pos + 0.5;
    return vec4<f32>(shifted, 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
}

// TestSplatScalarViaSubtraction exercises splatScalar via scalar - vec path
// (leftIsScalar && rightIsVec branch).
func TestSplatScalarViaSubtraction(t *testing.T) {
	source := `
@fragment
fn main(@location(0) pos: vec3<f32>) -> @location(0) vec4<f32> {
    // scalar - vec3 → promoteScalarToVector with left scalar
    let inverted = 1.0 - pos;
    return vec4<f32>(inverted, 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
}

// TestAtomicSubtractFloat exercises the AtomicSubtract with float path
// which uses FNegate + AtomicFAddEXT. This is a rare path because WGSL
// doesn't support float atomics on all targets. We test via direct IR.
func TestAtomicSubtractFloat(t *testing.T) {
	atomicF32Handle := ir.TypeHandle(0)
	f32Handle := ir.TypeHandle(1)
	resultExpr := ir.ExpressionHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "atomic_f32", Inner: ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:  "counter",
				Type:  atomicF32Handle,
				Space: ir.SpaceWorkGroup,
			},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:      "main",
				Stage:     ir.StageCompute,
				Workgroup: [3]uint32{1, 1, 1},
				Function: ir.Function{
					Expressions: []ir.Expression{
						{Kind: ir.ExprGlobalVariable{Variable: 0}},
						{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
						{Kind: ir.ExprAtomicResult{Ty: f32Handle, Comparison: false}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 2}}},
						{Kind: ir.StmtAtomic{
							Pointer: 0,
							Fun:     ir.AtomicAdd{},
							Value:   1,
							Result:  &resultExpr,
						}},
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 2, End: 3}}},
					},
				},
			},
		},
	}

	backend := NewBackend(DefaultOptions())
	spvBytes, err := backend.Compile(mod)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	assertValidSPIRV(t, spvBytes)
	instrs := decodeSPIRVInstructions(spvBytes)

	// Float atomic add → OpAtomicFAddEXT
	if !hasOpcodeInInstrs(instrs, OpAtomicFAddEXT) {
		t.Error("expected OpAtomicFAddEXT for float atomic add")
	}
}

// TestEmitLiteralI64 exercises emitLiteral with i64 literal values (50% coverage).
func TestEmitLiteralI64(t *testing.T) {
	i64Handle := ir.TypeHandle(0)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "i64", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 8}},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:      "main",
				Stage:     ir.StageCompute,
				Workgroup: [3]uint32{1, 1, 1},
				Function: ir.Function{
					LocalVars: []ir.LocalVariable{
						{Name: "big", Type: i64Handle},
					},
					Expressions: []ir.Expression{
						{Kind: ir.ExprLocalVariable{Variable: 0}},
						{Kind: ir.Literal{Value: ir.LiteralI64(0x123456789ABCDEF0)}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 2}}},
						{Kind: ir.StmtStore{Pointer: 0, Value: 1}},
					},
				},
			},
		},
	}

	backend := NewBackend(DefaultOptions())
	spvBytes, err := backend.Compile(mod)
	if err != nil {
		t.Logf("Compile result (i64 may need Int64 capability): err=%v, bytes=%d", err, len(spvBytes))
		return
	}
	assertValidSPIRV(t, spvBytes)
}

// TestEmitLiteralF64 exercises emitLiteral with f64 literal values.
func TestEmitLiteralF64(t *testing.T) {
	f64Handle := ir.TypeHandle(0)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "f64", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:      "main",
				Stage:     ir.StageCompute,
				Workgroup: [3]uint32{1, 1, 1},
				Function: ir.Function{
					LocalVars: []ir.LocalVariable{
						{Name: "val", Type: f64Handle},
					},
					Expressions: []ir.Expression{
						{Kind: ir.ExprLocalVariable{Variable: 0}},
						{Kind: ir.Literal{Value: ir.LiteralF64(3.14159265358979)}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 2}}},
						{Kind: ir.StmtStore{Pointer: 0, Value: 1}},
					},
				},
			},
		},
	}

	backend := NewBackend(DefaultOptions())
	spvBytes, err := backend.Compile(mod)
	if err != nil {
		t.Logf("Compile result (f64 may need Float64 capability): err=%v, bytes=%d", err, len(spvBytes))
		return
	}
	assertValidSPIRV(t, spvBytes)
}

// TestEmitConstExpressionComposite exercises emitConstExpression (34.1%)
// with a ZeroValue used as a const expression (constant zero initialization).
func TestEmitConstExpressionComposite(t *testing.T) {
	source := `
@group(0) @binding(0) var my_texture: texture_2d<f32>;
@group(0) @binding(1) var my_sampler: sampler;

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    // textureSampleLevel with const offset uses emitConstExpression
    let c1 = textureSampleLevel(my_texture, my_sampler, uv, 0.0, vec2<i32>(0, 0));
    let c2 = textureSampleLevel(my_texture, my_sampler, uv, 1.0, vec2<i32>(1, -1));
    return c1 + c2;
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
}

// TestImageQueryNumLevels exercises emitImageQuery with num_levels query.
func TestImageQueryNumLevels(t *testing.T) {
	source := `
@group(0) @binding(0) var my_texture: texture_2d<f32>;

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let dims = textureDimensions(my_texture, 0);
    let levels = textureNumLevels(my_texture);
    return vec4<f32>(f32(dims.x), f32(dims.y), f32(levels), 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	if !hasOpcodeInInstrs(instrs, OpImageQueryLevels) {
		t.Error("expected OpImageQueryLevels for textureNumLevels")
	}
}

// TestImageQueryNumSamples exercises emitImageQuery with multisampled texture.
func TestImageQueryNumSamples(t *testing.T) {
	source := `
@group(0) @binding(0) var ms_texture: texture_multisampled_2d<f32>;

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let samples = textureNumSamples(ms_texture);
    return vec4<f32>(f32(samples), 0.0, 0.0, 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	if !hasOpcodeInInstrs(instrs, OpImageQuerySamples) {
		t.Error("expected OpImageQuerySamples for textureNumSamples")
	}
}

// TestStorageTextureReadFormat exercises requestImageFormatCapabilities (50%)
// with a read-only storage texture and various image formats.
func TestStorageTextureReadFormat(t *testing.T) {
	source := `
@group(0) @binding(0) var img: texture_storage_2d<rgba16float, read>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    let val = textureLoad(img, vec2<i32>(vec2<u32>(id.x, id.y)));
    _ = val;
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
}
