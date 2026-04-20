// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestValidate_LdexpScalarStructural is the blob-level regression gate
// for BUG-DXIL-023. It builds a minimal IR module containing
// `ldexp(f32_arg, i32_literal)`, runs the full public Compile path,
// and then walks the resulting DXBC container through
// dxil.Validate(ValidateBitcode) — the pure-Go structural checker.
//
// Before the fix, emitMathLdexp passed the raw i32 exponent directly
// as the second operand of dx.op.unary.f32, producing a CALL record
// with an i32 operand at a float-declared slot. The bitcode stream
// would then fail parse at the first reader pass with HRESULT
// 0x80aa0009 "Invalid record". This test exercises the same code path
// at unit-test time without touching dxil.dll.
//
// Pairs with TestEmitMathLdexp_BitcodeRecordValid in the emit package:
// that test asserts the emitted instruction SHAPE (sitofp + unary.exp
// + fmul); this test asserts the SERIALIZED BLOB parses cleanly.
func TestValidate_LdexpScalarStructural(t *testing.T) {
	irMod := buildLdexpRegressionModule()

	blob, err := Compile(irMod, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(blob) == 0 {
		t.Fatal("Compile produced empty blob")
	}

	// Structural + bitcode-level validation is cross-platform and does
	// not require dxil.dll. Any record-level malformation (wrong
	// operand types, wrong operand count, wrong abbrev selection)
	// must be caught here, not silently pass until IDxcValidator runs.
	if err := Validate(blob, ValidateBitcode); err != nil {
		t.Fatalf("ValidateBitcode rejected ldexp blob: %v", err)
	}
}

// TestValidate_ChainedOverrideDefault is the regression gate for the
// `override height = 2.0 * depth` class of bugs surfaced by
// snapshot/testdata/in/overrides.wgsl. Before the fix,
// emitGlobalExpression handled only Literal / Compose / ZeroValue /
// Splat / Constant / Override cases — a chained override whose init
// is a binary expression fell through to the `default:` branch and
// returned getIntConstID(0), producing an i32 value where an f32 was
// expected. Downstream uses encoded a CALL/STORE record with the
// wrong operand type and LLVM's bitcode reader rejected it with
// HRESULT 0x80aa0009 "Invalid record" before any semantic validation.
//
// The fix extends emitGlobalExpression to constant-fold ExprBinary
// and ExprUnary at emit time via evalGlobalExpressionFloat. This
// matches the "no pipeline constants" convention already documented
// on emitOverride: when an override has no pipeline value and no
// default, it folds to 0; chained initializers fold through the zero.
func TestValidate_ChainedOverrideDefault(t *testing.T) {
	irMod := buildChainedOverrideModule()

	blob, err := Compile(irMod, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(blob) == 0 {
		t.Fatal("Compile produced empty blob")
	}
	if err := Validate(blob, ValidateBitcode); err != nil {
		t.Fatalf("ValidateBitcode rejected chained-override blob: %v", err)
	}
}

// buildChainedOverrideModule constructs a minimal compute shader
// matching the problematic pattern from overrides.wgsl:
//
//	override depth: f32;
//	override height = 2.0 * depth;
//
//	@compute @workgroup_size(1) fn main() { var t = height * 5.0; _ = t; }
//
// The globals[height].Init points at an ExprBinary whose RHS is an
// ExprOverride reference to `depth`. Before the fix,
// emitGlobalExpression hit the default branch on that ExprBinary and
// returned an i32 zero where an f32 was expected, corrupting the
// bitcode. Fold now produces the literal 0.0 (f32) as expected.
func buildChainedOverrideModule() *ir.Module {
	f32Handle := ir.TypeHandle(0)

	// Global expressions indexed by handle:
	//   [0] Literal 2.0  (lhs of height binary)
	//   [1] Override(depth)  (rhs of height binary)
	//   [2] Binary(Mul, [0], [1])  (height.Init)
	depthIdx := uint32(0)
	globals := []ir.Expression{
		{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},
		{Kind: ir.ExprOverride{Override: ir.OverrideHandle(depthIdx)}},
		{Kind: ir.ExprBinary{Op: ir.BinaryMultiply, Left: 0, Right: 1}},
	}
	heightInit := ir.ExpressionHandle(2)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		GlobalExpressions: globals,
		Overrides: []ir.Override{
			{Name: "depth", Ty: f32Handle, Init: nil},
			{Name: "height", Ty: f32Handle, Init: &heightInit},
		},
	}

	// Function body: read override(height), multiply by 5.0, discard.
	heightExprHandle := ir.ExpressionHandle(0)
	fiveHandle := ir.ExpressionHandle(1)
	mulHandle := ir.ExpressionHandle(2)
	fn := ir.Function{
		Name: "main",
		Expressions: []ir.Expression{
			{Kind: ir.ExprOverride{Override: 1}},                            // [0] height
			{Kind: ir.Literal{Value: ir.LiteralF32(5.0)}},                   // [1] 5.0
			{Kind: ir.ExprBinary{Op: ir.BinaryMultiply, Left: 0, Right: 1}}, // [2] height*5
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle},
			{Handle: &f32Handle},
			{Handle: &f32Handle},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
			// Hold-alive: we don't actually store t anywhere but the
			// Emit range pins all expressions so the binary op lands
			// in the emitted basic block.
			{Kind: ir.StmtReturn{}},
		},
	}
	_ = heightExprHandle
	_ = fiveHandle
	_ = mulHandle

	irMod.EntryPoints = []ir.EntryPoint{
		{
			Name:      "main",
			Stage:     ir.StageCompute,
			Function:  fn,
			Workgroup: [3]uint32{1, 1, 1},
		},
	}
	return irMod
}

// buildLdexpRegressionModule builds a fragment shader that only calls
// ldexp with a mixed f32 × i32 signature — the exact pattern from
// snapshot/testdata/in/math-functions.wgsl that tripped
// BUG-DXIL-023 in production.
//
// Kept separate from dxil/internal/emit test helpers because this
// lives in the public dxil package and exercises the full Compile
// pipeline rather than just the internal Emit function.
func buildLdexpRegressionModule() *ir.Module {
	f32Handle := ir.TypeHandle(0)
	i32Handle := ir.TypeHandle(1)
	vec4Handle := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	xBinding := ir.Binding(ir.LocationBinding{Location: 0})
	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})

	arg1Handle := ir.ExpressionHandle(1)
	retHandle := ir.ExpressionHandle(4)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "x", Type: f32Handle, Binding: &xBinding},
		},
		Result: &ir.FunctionResult{
			Type:    vec4Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0] x
			{Kind: ir.Literal{Value: ir.LiteralI32(2)}},                           // [1] 2
			{Kind: ir.ExprMath{Fun: ir.MathLdexp, Arg: 0, Arg1: &arg1Handle}},     // [2] ldexp(x, 2)
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [3] 1.0
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{2, 2, 2, 3}}}, // [4] vec4(r,r,r,1)
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle},
			{Handle: &i32Handle},
			{Handle: &f32Handle},
			{Handle: &f32Handle},
			{Handle: &vec4Handle},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 5}}},
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}
	return mod
}
