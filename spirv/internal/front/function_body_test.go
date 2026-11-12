package front

import (
	"strconv"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// bodyModule assembles a module with a fixed set of types, constants and one
// function, so a test can exercise one instruction arm at a time.
//
// The ids are fixed: 1 void, 2 fn void(), 3 i32, 4 u32, 5 f32, 6 bool,
// 7 vec2<f32>, 8 vec3<f32>, 9 vec4<f32>, 10 mat2x2<f32>, 11/12/13 pointers to
// i32/f32/vec4<f32> in the Function space, 14 struct {i32, vec4<f32>},
// 15 its pointer, 17 array<i32,4>, 18 its pointer, 30 vec2<bool>,
// 31 pointer to mat2x2, 32 fn i32(), 33 pointer to vec2<f32>,
// 40 the GLSL.std.450 set when withExtInst is called. Constants start at 20.
type bodyModule struct {
	groups   [][]uint32
	extInst  bool
	resultID uint32 // 1 for a void function
	// raw leaves the function unclosed, so the last body instruction can be
	// the end of the input.
	raw bool
}

// newBodyModule returns the shared preamble.
func newBodyModule() *bodyModule {
	return &bodyModule{resultID: 1}
}

// withResult makes the function return i32 instead of void.
func (m *bodyModule) withResult() *bodyModule {
	m.resultID = 3
	return m
}

// withExtInst imports the GLSL.std.450 set as id 40.
func (m *bodyModule) withExtInst() *bodyModule {
	m.extInst = true
	return m
}

// add appends instructions before the function.
func (m *bodyModule) add(groups ...[]uint32) *bodyModule {
	m.groups = append(m.groups, groups...)
	return m
}

// buildRaw assembles the function with body and closes it without reading
// past the input: the last instruction may claim more words than it has, so
// its operands are really missing.
func (m *bodyModule) buildRaw(body ...[]uint32) []byte {
	m.raw = true
	defer func() { m.raw = false }()
	return m.build(body...)
}

// build assembles the binary with body inside one function.
func (m *bodyModule) build(body ...[]uint32) []byte {
	groups := [][]uint32{
		headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
	}
	if m.extInst {
		// The extension section precedes the memory model.
		groups = append(groups, instWithString(spvenums.OpExtInstImport, []uint32{40}, "GLSL.std.450"))
	}
	groups = append(groups,
		instWords(spvenums.OpMemoryModel, 0, 1),
		instWords(spvenums.OpTypeVoid, 1),
		instWords(spvenums.OpTypeFunction, 2, 1),
		instWords(spvenums.OpTypeInt, 3, 32, 1),
		instWords(spvenums.OpTypeInt, 4, 32, 0),
		instWords(spvenums.OpTypeFloat, 5, 32),
		instWords(spvenums.OpTypeBool, 6),
		instWords(spvenums.OpTypeVector, 7, 5, 2),
		instWords(spvenums.OpTypeVector, 8, 5, 3),
		instWords(spvenums.OpTypeVector, 9, 5, 4),
		instWords(spvenums.OpTypeMatrix, 10, 7, 2),
		instWords(spvenums.OpTypePointer, 11, uint32(spvenums.StorageClassFunction), 3),
		instWords(spvenums.OpTypePointer, 12, uint32(spvenums.StorageClassFunction), 5),
		instWords(spvenums.OpTypePointer, 13, uint32(spvenums.StorageClassFunction), 9),
		instWords(spvenums.OpTypeStruct, 14, 3, 9),
		instWords(spvenums.OpTypePointer, 15, uint32(spvenums.StorageClassFunction), 14),
		instWords(spvenums.OpConstant, 4, 16, 4),
		instWords(spvenums.OpTypeArray, 17, 3, 16),
		instWords(spvenums.OpTypePointer, 18, uint32(spvenums.StorageClassFunction), 17),
		instWords(spvenums.OpTypeVector, 30, 6, 2),
		instWords(spvenums.OpTypePointer, 31, uint32(spvenums.StorageClassFunction), 10),
		instWords(spvenums.OpTypeFunction, 32, 3),
		instWords(spvenums.OpTypePointer, 33, uint32(spvenums.StorageClassFunction), 7),
		instWords(spvenums.OpConstant, 3, 20, 5),
		instWords(spvenums.OpConstant, 4, 21, 7),
		instWords(spvenums.OpConstant, 5, 22, 0x3f800000),
		instWords(spvenums.OpConstant, 3, 23, 0xffffffff),
		instWords(spvenums.OpConstantTrue, 6, 24),
		instWords(spvenums.OpConstantFalse, 6, 25),
		instWords(spvenums.OpConstant, 3, 26, 1),
		instWords(spvenums.OpConstant, 4, 27, 1),
		instWords(spvenums.OpConstant, 5, 28, 0x40000000),
		instWords(spvenums.OpConstant, 4, 29, 2),    // Workgroup scope
		instWords(spvenums.OpConstant, 4, 30, 1),    // Device scope
		instWords(spvenums.OpConstant, 4, 31, 3),    // Subgroup scope
		instWords(spvenums.OpConstant, 4, 32, 64),   // UniformMemory
		instWords(spvenums.OpConstant, 4, 33, 256),  // WorkgroupMemory
		instWords(spvenums.OpConstant, 4, 34, 128),  // SubgroupMemory
		instWords(spvenums.OpConstant, 4, 35, 2048), // ImageMemory
		instWords(spvenums.OpConstant, 4, 36, 4),    // Invocation scope
		instWords(spvenums.OpConstant, 4, 37, 320),  // UniformMemory|WorkgroupMemory
		instWords(spvenums.OpConstantNull, 4, 38),
	)
	if m.resultID == 3 {
		groups = append(groups,
			instWords(spvenums.OpFunction, 3, 100, 0, 32),
			instWords(spvenums.OpLabel, 101),
		)
	} else {
		groups = append(groups,
			instWords(spvenums.OpFunction, 1, 100, 0, 2),
			instWords(spvenums.OpLabel, 101),
		)
	}
	groups = append(groups, body...)
	if m.raw {
		return spvWords(groups...)
	}
	// A test may end its block itself; the builder then closes the function.
	if len(body) > 0 {
		last := body[len(body)-1][0]
		switch spvenums.Op(last & 0xffff) {
		case spvenums.OpKill, spvenums.OpUnreachable, spvenums.OpReturnValue:
			groups = append(groups, instWords(spvenums.OpFunctionEnd))
			return spvWords(groups...)
		}
	}
	groups = append(groups, instWords(spvenums.OpReturn), instWords(spvenums.OpFunctionEnd))
	return spvWords(groups...)
}

// parse parses the module and returns its function.
func (m *bodyModule) parse(t *testing.T, body ...[]uint32) *ir.Function {
	t.Helper()
	module, _, err := Parse(m.build(body...), Options{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(module.Functions) != 1 {
		t.Fatalf("functions = %d, want 1", len(module.Functions))
	}
	return &module.Functions[0]
}

// parseError assembles a module whose body must be refused.
func (m *bodyModule) parseError(t *testing.T, body ...[]uint32) *Error {
	t.Helper()
	_, _, err := Parse(m.build(body...), Options{})
	if err == nil {
		t.Fatal("Parse accepted the module")
	}
	return requireError(t, err)
}

// findExpression returns the first expression the predicate accepts.
func findExpression(fn *ir.Function, accept func(ir.ExpressionKind) bool) (ir.ExpressionKind, bool) {
	for _, expression := range fn.Expressions {
		if accept(expression.Kind) {
			return expression.Kind, true
		}
	}
	return nil, false
}

// hasStatement reports whether the function body holds a statement the
// predicate accepts.
func hasStatement(fn *ir.Function, accept func(ir.StatementKind) bool) bool {
	for _, statement := range fn.Body {
		if accept(statement.Kind) {
			return true
		}
	}
	return false
}

func TestBodyExpressionArms(t *testing.T) {
	cases := []struct {
		name  string
		body  [][]uint32
		check func(t *testing.T, fn *ir.Function)
	}{
		{
			name: "undef",
			body: [][]uint32{instWords(spvenums.OpUndef, 3, 200)},
			check: func(t *testing.T, fn *ir.Function) {
				if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
					_, ok := k.(ir.ExprZeroValue)
					return ok
				}); !ok {
					t.Error("no ZeroValue expression")
				}
			},
		},
		{
			name: "local variable with initializer",
			body: [][]uint32{instWords(spvenums.OpVariable, 11, 200, uint32(spvenums.StorageClassFunction), 20)},
			check: func(t *testing.T, fn *ir.Function) {
				if len(fn.LocalVars) != 1 || fn.LocalVars[0].Init == nil || fn.LocalVars[0].Type != 0 {
					t.Errorf("local variables = %#v, want one i32 with an initializer", fn.LocalVars)
				}
			},
		},
		{
			name: "access chain into a struct member",
			body: [][]uint32{
				instWords(spvenums.OpVariable, 15, 200, uint32(spvenums.StorageClassFunction)),
				instWords(spvenums.OpAccessChain, 11, 201, 200, 26),
			},
			check: func(t *testing.T, fn *ir.Function) {
				if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
					access, ok := k.(ir.ExprAccessIndex)
					return ok && access.Index == 1
				}); !ok {
					t.Error("no AccessIndex expression")
				}
			},
		},
		{
			name: "access chain into an array",
			body: [][]uint32{
				instWords(spvenums.OpVariable, 18, 200, uint32(spvenums.StorageClassFunction)),
				instWords(spvenums.OpAccessChain, 11, 201, 200, 27),
			},
			check: func(t *testing.T, fn *ir.Function) {
				if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
					_, ok := k.(ir.ExprAccess)
					return ok
				}); !ok {
					t.Error("no Access expression")
				}
			},
		},
		{
			name: "access chain into a matrix column by constant",
			body: [][]uint32{
				instWords(spvenums.OpVariable, 31, 200, uint32(spvenums.StorageClassFunction)),
				instWords(spvenums.OpAccessChain, 33, 201, 200, 26),
			},
			check: func(t *testing.T, fn *ir.Function) {
				if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
					access, ok := k.(ir.ExprAccessIndex)
					return ok && access.Index == 1
				}); !ok {
					t.Error("no AccessIndex for the matrix column")
				}
			},
		},
		{
			name: "composite extract from a vector",
			body: [][]uint32{
				instWords(spvenums.OpCompositeConstruct, 9, 200, 22, 22, 22, 22),
				instWords(spvenums.OpCompositeExtract, 5, 201, 200, 2),
			},
			check: func(t *testing.T, fn *ir.Function) {
				if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
					access, ok := k.(ir.ExprAccessIndex)
					return ok && access.Index == 2
				}); !ok {
					t.Error("no AccessIndex expression")
				}
			},
		},
		{
			name: "composite construct with a splat",
			body: [][]uint32{instWords(spvenums.OpCompositeConstruct, 9, 200, 22, 22, 22, 22)},
			check: func(t *testing.T, fn *ir.Function) {
				if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
					_, ok := k.(ir.ExprSplat)
					return ok
				}); !ok {
					t.Error("no Splat expression")
				}
			},
		},
		{
			name: "composite construct of distinct components",
			body: [][]uint32{instWords(spvenums.OpCompositeConstruct, 7, 200, 22, 28)},
			check: func(t *testing.T, fn *ir.Function) {
				if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
					compose, ok := k.(ir.ExprCompose)
					return ok && len(compose.Components) == 2
				}); !ok {
					t.Error("no Compose expression")
				}
			},
		},
		{
			name: "load and store",
			body: [][]uint32{
				instWords(spvenums.OpVariable, 11, 200, uint32(spvenums.StorageClassFunction)),
				instWords(spvenums.OpStore, 200, 20),
				instWords(spvenums.OpLoad, 3, 201, 200),
			},
			check: func(t *testing.T, fn *ir.Function) {
				if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
					_, ok := k.(ir.ExprLoad)
					return ok
				}); !ok {
					t.Error("no Load expression")
				}
				if !hasStatement(fn, func(k ir.StatementKind) bool {
					_, ok := k.(ir.StmtStore)
					return ok
				}) {
					t.Error("no Store statement")
				}
			},
		},
		{
			name: "load with a memory access operand",
			body: [][]uint32{
				instWords(spvenums.OpVariable, 11, 200, uint32(spvenums.StorageClassFunction)),
				instWords(spvenums.OpLoad, 3, 201, 200, 2),
				instWords(spvenums.OpStore, 200, 23, 2),
			},
			check: func(t *testing.T, fn *ir.Function) {
				if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
					_, ok := k.(ir.ExprLoad)
					return ok
				}); !ok {
					t.Error("no Load expression")
				}
			},
		},
		{
			name: "copy object",
			body: [][]uint32{instWords(spvenums.OpCopyObject, 3, 200, 20)},
			check: func(t *testing.T, fn *ir.Function) {
				if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
					_, ok := k.(ir.ExprConstant)
					return ok
				}); !ok {
					t.Error("no constant reference")
				}
			},
		},
		{
			name: "signed negation of another type",
			body: [][]uint32{instWords(spvenums.OpSNegate, 3, 200, 21)},
			check: func(t *testing.T, fn *ir.Function) {
				if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
					unary, ok := k.(ir.ExprUnary)
					return ok && unary.Op == ir.UnaryNegate
				}); !ok {
					t.Error("no negate expression")
				}
				if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
					_, ok := k.(ir.ExprAs)
					return ok
				}); !ok {
					t.Error("the u32 operand was not converted to i32")
				}
			},
		},
		{
			name: "logical and bitwise not",
			body: [][]uint32{
				instWords(spvenums.OpLogicalNot, 6, 200, 24),
				instWords(spvenums.OpNot, 3, 201, 20),
			},
			check: func(t *testing.T, fn *ir.Function) {
				for _, op := range []ir.UnaryOperator{ir.UnaryLogicalNot, ir.UnaryBitwiseNot} {
					want := op
					if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
						unary, ok := k.(ir.ExprUnary)
						return ok && unary.Op == want
					}); !ok {
						t.Errorf("no unary operator %v", op)
					}
				}
			},
		},
		{
			name: "float arithmetic and comparisons",
			body: [][]uint32{
				instWords(spvenums.OpFAdd, 5, 200, 22, 28),
				instWords(spvenums.OpFSub, 5, 201, 22, 28),
				instWords(spvenums.OpFMul, 5, 202, 22, 28),
				instWords(spvenums.OpFDiv, 5, 203, 22, 28),
				instWords(spvenums.OpFRem, 5, 204, 22, 28),
				instWords(spvenums.OpFOrdEqual, 6, 205, 22, 28),
				instWords(spvenums.OpFUnordNotEqual, 6, 206, 22, 28),
			},
			check: func(t *testing.T, fn *ir.Function) {
				if countExpressions[ir.ExprBinary](fn) != 7 {
					t.Errorf("binary expressions = %d, want 7", countExpressions[ir.ExprBinary](fn))
				}
			},
		},
		{
			name: "integer arithmetic with sign adjustment",
			body: [][]uint32{
				instWords(spvenums.OpIAdd, 4, 200, 20, 21),
				instWords(spvenums.OpISub, 3, 201, 20, 26),
				instWords(spvenums.OpIMul, 3, 202, 20, 26),
				instWords(spvenums.OpBitwiseAnd, 3, 203, 20, 26),
				instWords(spvenums.OpBitwiseOr, 3, 204, 20, 26),
				instWords(spvenums.OpBitwiseXor, 3, 205, 20, 26),
				instWords(spvenums.OpSDiv, 3, 206, 20, 26),
				instWords(spvenums.OpSRem, 3, 207, 20, 26),
				instWords(spvenums.OpIEqual, 6, 208, 20, 26),
				instWords(spvenums.OpINotEqual, 6, 209, 20, 26),
			},
			check: func(t *testing.T, fn *ir.Function) {
				if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
					_, ok := k.(ir.ExprAs)
					return ok
				}); !ok {
					t.Error("the i32/u32 mismatch was not converted")
				}
			},
		},
		{
			name: "signed modulo decomposition",
			body: [][]uint32{instWords(spvenums.OpSMod, 3, 200, 20, 26)},
			check: func(t *testing.T, fn *ir.Function) {
				if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
					math, ok := k.(ir.ExprMath)
					return ok && math.Fun == ir.MathFloor
				}); !ok {
					t.Error("no floor in the SMod decomposition")
				}
			},
		},
		{
			name: "float modulo decomposition",
			body: [][]uint32{instWords(spvenums.OpFMod, 5, 200, 22, 28)},
			check: func(t *testing.T, fn *ir.Function) {
				if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
					math, ok := k.(ir.ExprMath)
					return ok && math.Fun == ir.MathFloor
				}); !ok {
					t.Error("no floor in the FMod decomposition")
				}
			},
		},
		{
			name: "shifts",
			body: [][]uint32{
				instWords(spvenums.OpShiftLeftLogical, 3, 200, 20, 26),
				instWords(spvenums.OpShiftRightLogical, 4, 201, 21, 27),
				instWords(spvenums.OpShiftRightArithmetic, 3, 202, 20, 26),
			},
			check: func(t *testing.T, fn *ir.Function) {
				if countExpressions[ir.ExprBinary](fn) != 3 {
					t.Errorf("binary expressions = %d, want 3", countExpressions[ir.ExprBinary](fn))
				}
			},
		},
		{
			name: "signed and unsigned comparisons",
			body: [][]uint32{
				instWords(spvenums.OpSLessThan, 6, 200, 20, 26),
				instWords(spvenums.OpSLessThanEqual, 6, 201, 20, 26),
				instWords(spvenums.OpSGreaterThan, 6, 202, 20, 26),
				instWords(spvenums.OpSGreaterThanEqual, 6, 203, 20, 26),
				instWords(spvenums.OpULessThan, 6, 204, 21, 27),
				instWords(spvenums.OpUGreaterThanEqual, 6, 205, 21, 27),
			},
			check: func(t *testing.T, fn *ir.Function) {
				if countExpressions[ir.ExprBinary](fn) != 6 {
					t.Errorf("binary expressions = %d, want 6", countExpressions[ir.ExprBinary](fn))
				}
			},
		},
		{
			name: "relational functions",
			body: [][]uint32{
				instWords(spvenums.OpUndef, 30, 200),
				instWords(spvenums.OpAny, 6, 201, 200),
				instWords(spvenums.OpAll, 6, 202, 200),
				instWords(spvenums.OpIsNan, 6, 203, 22),
				instWords(spvenums.OpIsInf, 6, 204, 22),
			},
			check: func(t *testing.T, fn *ir.Function) {
				if countExpressions[ir.ExprRelational](fn) != 4 {
					t.Errorf("relational expressions = %d, want 4", countExpressions[ir.ExprRelational](fn))
				}
			},
		},
		{
			name: "select",
			body: [][]uint32{instWords(spvenums.OpSelect, 3, 200, 24, 20, 26)},
			check: func(t *testing.T, fn *ir.Function) {
				if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
					_, ok := k.(ir.ExprSelect)
					return ok
				}); !ok {
					t.Error("no Select expression")
				}
			},
		},
		{
			name: "vector shuffle swizzle",
			body: [][]uint32{
				instWords(spvenums.OpCompositeConstruct, 9, 200, 22, 22, 22, 22),
				instWords(spvenums.OpVectorShuffle, 8, 201, 200, 200, 0, 1, 2),
			},
			check: func(t *testing.T, fn *ir.Function) {
				if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
					swizzle, ok := k.(ir.ExprSwizzle)
					return ok && swizzle.Size == ir.Vec3
				}); !ok {
					t.Error("no Swizzle expression")
				}
			},
		},
		{
			name: "vector shuffle across both vectors",
			body: [][]uint32{
				instWords(spvenums.OpCompositeConstruct, 9, 200, 22, 22, 22, 22),
				instWords(spvenums.OpCompositeConstruct, 9, 201, 28, 28, 28, 28),
				instWords(spvenums.OpVectorShuffle, 9, 202, 200, 201, 0, 1, 4, 5),
			},
			check: func(t *testing.T, fn *ir.Function) {
				if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
					compose, ok := k.(ir.ExprCompose)
					return ok && len(compose.Components) == 4
				}); !ok {
					t.Error("no Compose expression for the shuffle")
				}
			},
		},
		{
			name: "vector shuffle with an undefined component",
			body: [][]uint32{
				instWords(spvenums.OpCompositeConstruct, 9, 200, 22, 22, 22, 22),
				instWords(spvenums.OpVectorShuffle, 7, 201, 200, 200, 0, 0xffffffff),
			},
			check: func(t *testing.T, fn *ir.Function) {
				if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
					swizzle, ok := k.(ir.ExprSwizzle)
					return ok && swizzle.Pattern[1] == ir.SwizzleX
				}); !ok {
					t.Error("an undefined shuffle component did not read as X")
				}
			},
		},
		{
			// A shuffle with more components than an IR vector holds: Rust
			// zips the pattern with the indices and keeps the first four
			// (next_block.rs, the VectorShuffle arm). Writing every index into
			// the four-slot pattern panicked here.
			name: "vector shuffle with more than four components",
			body: [][]uint32{
				instWords(spvenums.OpCompositeConstruct, 9, 200, 22, 22, 22, 22),
				instWords(spvenums.OpVectorShuffle, 9, 201, 200, 200, 0, 1, 2, 3, 0),
			},
			check: func(t *testing.T, fn *ir.Function) {
				if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
					swizzle, ok := k.(ir.ExprSwizzle)
					return ok && swizzle.Size == ir.Vec4 && swizzle.Pattern ==
						[4]ir.SwizzleComponent{ir.SwizzleX, ir.SwizzleY, ir.SwizzleZ, ir.SwizzleW}
				}); !ok {
					t.Error("no xyzw Swizzle for the five-component shuffle")
				}
			},
		},
		{
			name: "bitcast and conversions",
			body: [][]uint32{
				instWords(spvenums.OpBitcast, 3, 200, 22),
				instWords(spvenums.OpConvertSToF, 5, 201, 20),
				instWords(spvenums.OpConvertFToU, 4, 202, 22),
				instWords(spvenums.OpConvertFToS, 3, 203, 22),
				instWords(spvenums.OpConvertUToF, 5, 204, 21),
				instWords(spvenums.OpSConvert, 3, 205, 20),
				instWords(spvenums.OpFConvert, 5, 206, 22),
				instWords(spvenums.OpUConvert, 4, 207, 21),
			},
			check: func(t *testing.T, fn *ir.Function) {
				if countExpressions[ir.ExprAs](fn) != 8 {
					t.Errorf("As expressions = %d, want 8", countExpressions[ir.ExprAs](fn))
				}
			},
		},
		{
			name: "transpose and dot",
			body: [][]uint32{
				instWords(spvenums.OpUndef, 10, 200),
				instWords(spvenums.OpTranspose, 10, 201, 200),
				instWords(spvenums.OpUndef, 9, 202),
				instWords(spvenums.OpDot, 5, 203, 202, 202),
			},
			check: func(t *testing.T, fn *ir.Function) {
				for _, fun := range []ir.MathFunction{ir.MathTranspose, ir.MathDot} {
					want := fun
					if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
						math, ok := k.(ir.ExprMath)
						return ok && math.Fun == want
					}); !ok {
						t.Errorf("no math function %v", fun)
					}
				}
			},
		},
		{
			name: "derivatives",
			body: [][]uint32{
				instWords(spvenums.OpDPdx, 5, 200, 22),
				instWords(spvenums.OpDPdy, 5, 201, 22),
				instWords(spvenums.OpFwidth, 5, 202, 22),
				instWords(spvenums.OpDPdxCoarse, 5, 203, 22),
				instWords(spvenums.OpDPdyCoarse, 5, 204, 22),
				instWords(spvenums.OpFwidthCoarse, 5, 205, 22),
				instWords(spvenums.OpDPdxFine, 5, 206, 22),
				instWords(spvenums.OpDPdyFine, 5, 207, 22),
				instWords(spvenums.OpFwidthFine, 5, 208, 22),
			},
			check: func(t *testing.T, fn *ir.Function) {
				if countExpressions[ir.ExprDerivative](fn) != 9 {
					t.Errorf("derivative expressions = %d, want 9", countExpressions[ir.ExprDerivative](fn))
				}
			},
		},
		{
			name: "line and no-line instructions",
			body: [][]uint32{
				instWords(spvenums.OpLine, 1, 1, 1),
				instWords(spvenums.OpNoLine),
			},
			check: func(t *testing.T, fn *ir.Function) {},
		},
		{
			name: "kill",
			body: [][]uint32{instWords(spvenums.OpKill)},
			check: func(t *testing.T, fn *ir.Function) {
				if !hasStatement(fn, func(k ir.StatementKind) bool {
					_, ok := k.(ir.StmtKill)
					return ok
				}) {
					t.Error("no Kill statement")
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, newBodyModule().parse(t, tc.body...))
		})
	}
}

func TestBodyExtInst(t *testing.T) {
	t.Run("one two and three arguments", func(t *testing.T) {
		fn := newBodyModule().withExtInst().parse(t,
			instWords(spvenums.OpExtInst, 5, 200, 40, uint32(spvenums.GlslStd450Radians), 22),
			instWords(spvenums.OpExtInst, 5, 201, 40, uint32(spvenums.GlslStd450Pow), 22, 28),
			instWords(spvenums.OpExtInst, 5, 202, 40, uint32(spvenums.GlslStd450SmoothStep), 22, 28, 22),
		)
		count := 0
		for _, expression := range fn.Expressions {
			if _, ok := expression.Kind.(ir.ExprMath); ok {
				count++
			}
		}
		if count != 3 {
			t.Errorf("math expressions = %d, want 3", count)
		}
	})

	t.Run("unsupported instruction", func(t *testing.T) {
		m := newBodyModule().withExtInst()
		want := "unsupported extension instantiation %" + itoaTest(uint32(spvenums.GlslStd450IMix))
		if got := m.parseError(t, instWords(spvenums.OpExtInst, 5, 200, 40, uint32(spvenums.GlslStd450IMix), 22)).Message; got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
	})

	t.Run("unsupported set", func(t *testing.T) {
		m := newBodyModule()
		if got := m.parseError(t, instWords(spvenums.OpExtInst, 5, 200, 99, uint32(spvenums.GlslStd450Radians), 22)).Message; got != "unsupported extension instantiation set %99" {
			t.Errorf("message = %q", got)
		}
	})

	t.Run("wrong operand count", func(t *testing.T) {
		m := newBodyModule().withExtInst()
		if got := m.parseError(t, instWords(spvenums.OpExtInst, 5, 200, 40, uint32(spvenums.GlslStd450Pow), 22)).Message; got != "invalid operand count 6 for ExtInst" {
			t.Errorf("message = %q", got)
		}
	})
}

func TestBodyBarriers(t *testing.T) {
	t.Run("control and memory barriers", func(t *testing.T) {
		fn := newBodyModule().parse(t,
			instWords(spvenums.OpControlBarrier, 29, 29, 32),
			instWords(spvenums.OpControlBarrier, 31, 29, 33),
			instWords(spvenums.OpMemoryBarrier, 30, 37),
			instWords(spvenums.OpMemoryBarrier, 31, 34),
			instWords(spvenums.OpMemoryBarrier, 29, 32),
		)
		var control, memory []ir.BarrierFlags
		for _, statement := range fn.Body {
			switch kind := statement.Kind.(type) {
			case ir.StmtControlBarrier:
				control = append(control, kind.Flags)
			case ir.StmtMemoryBarrier:
				memory = append(memory, kind.Flags)
			}
		}
		if len(control) != 2 {
			t.Fatalf("control barriers = %d, want 2", len(control))
		}
		if control[0] != ir.BarrierStorage {
			t.Errorf("first control barrier = %v, want storage", control[0])
		}
		if control[1] != ir.BarrierWorkGroup {
			t.Errorf("second control barrier = %v, want workgroup", control[1])
		}
		if len(memory) != 3 {
			t.Fatalf("memory barriers = %d, want 3", len(memory))
		}
		if memory[0] != ir.BarrierStorage|ir.BarrierWorkGroup {
			t.Errorf("device barrier = %v, want storage and workgroup", memory[0])
		}
		if memory[1] != ir.BarrierSubGroup {
			t.Errorf("subgroup barrier = %v, want subgroup", memory[1])
		}
		if memory[2] != ir.BarrierStorage|ir.BarrierWorkGroup {
			t.Errorf("workgroup barrier = %v, want storage and workgroup", memory[2])
		}
	})

	t.Run("unsupported execution scope is a warning", func(t *testing.T) {
		module, info, err := Parse(newBodyModule().build(
			instWords(spvenums.OpControlBarrier, 36, 29, 32),
		), Options{})
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if len(info.Warnings) != 1 {
			t.Errorf("warnings = %d, want 1", len(info.Warnings))
		}
		for _, statement := range module.Functions[0].Body {
			if _, ok := statement.Kind.(ir.StmtControlBarrier); ok {
				t.Error("an unsupported scope produced a barrier statement")
			}
		}
	})

	t.Run("invalid scope and semantics", func(t *testing.T) {
		// A constant that is not an integer literal cannot be a barrier scope.
		m := newBodyModule()
		if got := m.parseError(t, instWords(spvenums.OpControlBarrier, 38, 29, 32)).Message; got != "invalid barrier scope %38" {
			t.Errorf("scope message = %q", got)
		}
		m = newBodyModule()
		if got := m.parseError(t, instWords(spvenums.OpControlBarrier, 29, 29, 38)).Message; got != "invalid barrier memory semantics %38" {
			t.Errorf("semantics message = %q", got)
		}
	})
}

func TestBodyTerminators(t *testing.T) {
	t.Run("return value", func(t *testing.T) {
		module, _, err := Parse(newBodyModule().withResult().build(
			instWords(spvenums.OpReturnValue, 20),
		), Options{})
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		fn := module.Functions[0]
		if fn.Result == nil || fn.Result.Type != 0 {
			t.Errorf("result = %#v, want i32", fn.Result)
		}
		found := false
		for _, statement := range fn.Body {
			if ret, ok := statement.Kind.(ir.StmtReturn); ok && ret.Value != nil {
				found = true
			}
		}
		if !found {
			t.Error("no Return with a value")
		}
	})

	t.Run("unreachable", func(t *testing.T) {
		if fn := newBodyModule().parse(t, instWords(spvenums.OpUnreachable)); hasStatement(fn, func(ir.StatementKind) bool { return true }) {
			// The block ends with no terminator statement; an Emit for the
			// preceding expressions is all that may remain.
			for _, statement := range fn.Body {
				if _, ok := statement.Kind.(ir.StmtEmit); !ok {
					t.Errorf("statement %T after unreachable", statement.Kind)
				}
			}
		}
	})
}

func TestBodyInstructionMistakes(t *testing.T) {
	cases := []struct {
		name string
		body [][]uint32
		want string
	}{
		{
			name: "unknown operand id",
			body: [][]uint32{instWords(spvenums.OpLoad, 3, 200, 99)},
			want: "invalid id %99",
		},
		{
			name: "wrong operand count",
			body: [][]uint32{instWords(spvenums.OpIAdd, 3, 200, 20)},
			want: "invalid operand count 4 for IAdd",
		},
		{
			name: "struct index must be constant",
			body: [][]uint32{
				instWords(spvenums.OpVariable, 15, 200, uint32(spvenums.StorageClassFunction)),
				instWords(spvenums.OpVariable, 11, 201, uint32(spvenums.StorageClassFunction)),
				instWords(spvenums.OpAccessChain, 11, 202, 200, 201),
			},
			want: "invalid access",
		},
		{
			name: "struct index out of range",
			body: [][]uint32{
				instWords(spvenums.OpVariable, 15, 200, uint32(spvenums.StorageClassFunction)),
				instWords(spvenums.OpAccessChain, 11, 201, 200, 20),
			},
			want: "invalid access",
		},
		{
			name: "unsupported instruction in a body",
			body: [][]uint32{instWords(spvenums.OpBranchConditional, 24, 101, 101)},
			want: "unsupported instruction BranchConditional at Function",
		},
		{
			name: "is-finite has no relational function",
			body: [][]uint32{instWords(spvenums.OpIsFinite, 6, 200, 22)},
			want: "unknown relational function IsFinite",
		},
		{
			name: "shuffle index out of range",
			body: [][]uint32{
				instWords(spvenums.OpCompositeConstruct, 9, 200, 22, 22, 22, 22),
				instWords(spvenums.OpVectorShuffle, 9, 201, 200, 200, 0, 1, 2, 9),
			},
			want: "invalid access index %9",
		},
		{
			name: "shuffle result is not a vector",
			body: [][]uint32{
				instWords(spvenums.OpCompositeConstruct, 9, 200, 22, 22, 22, 22),
				instWords(spvenums.OpVectorShuffle, 3, 201, 200, 200, 0, 1, 2, 3, 0, 1, 2),
			},
			want: "invalid inner type %3",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newBodyModule()
			if got := m.parseError(t, tc.body...).Message; got != tc.want {
				t.Errorf("message = %q, want %q", got, tc.want)
			}
		})
	}
}

// countExpressions counts the expressions of the given kind.
func countExpressions[T ir.ExpressionKind](fn *ir.Function) int {
	count := 0
	for _, expression := range fn.Expressions {
		if _, ok := expression.Kind.(T); ok {
			count++
		}
	}
	return count
}

// itoaTest formats an id for an expected message.
func itoaTest(v uint32) string {
	return strconv.FormatUint(uint64(v), 10)
}
