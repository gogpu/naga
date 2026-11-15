package front

// Contracts of GLSL expression lowering that the IR does not have operators
// for: matrix division and comparison, the scalar/vector/matrix mixtures and
// the relational vector results, all Rust naga's Context::lower_inner arms
// (front/glsl/context.rs:544-1394).

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// binaryLowered lowers `left op right` for two declared locals.
func binaryLowered(t *testing.T, c *Context, left, right *glslSymbol, op ir.BinaryOperator) ir.ExpressionHandle {
	t.Helper()
	arena := &hirArena{}
	root := arena.add(hirBinary{Op: op,
		Left:  arena.add(hirVariable{Sym: left}, ir.Span{}),
		Right: arena.add(hirVariable{Sym: right}, ir.Span{}),
	}, ir.Span{})
	h, _, err := c.lowerExpect(arena, root, rhsPos)
	if err != nil {
		t.Fatalf("lowerExpect(%v): %v", op, err)
	}
	return h
}

// TestLowerMatrixDivisionRebuildsColumns is the ExprBinary::Divide matrix arm
// (context.rs:634-663): the IR has no matrix division, so every column is
// divided on its own and the matrix is rebuilt with a compose.
func TestLowerMatrixDivisionRebuildsColumns(t *testing.T) {
	fe, c := newTestContext(t)
	mat := fe.registerType(ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: f32Scalar})
	left := declareLocal(t, c, "a", mat)
	right := declareLocal(t, c, "b", mat)

	h := binaryLowered(t, c, left, right, ir.BinaryDivide)
	compose := exprAs[ir.ExprCompose](t, c, h, "the rebuilt matrix")
	if len(compose.Components) != 2 {
		t.Fatalf("the rebuilt matrix has %d columns, want 2", len(compose.Components))
	}
	for i, component := range compose.Components {
		column := exprAs[ir.ExprBinary](t, c, component, "the column division")
		if column.Op != ir.BinaryDivide {
			t.Fatalf("column %d operation = %v, want a division", i, column.Op)
		}
	}
}

// TestLowerMatrixComparisonFoldsColumns is the Equal/NotEqual matrix arm
// (context.rs:664-724): the columns are compared, each result folded with a
// relational All (equality) or Any (inequality) and the column results
// combined with And or Or.
func TestLowerMatrixComparisonFoldsColumns(t *testing.T) {
	tests := []struct {
		op      ir.BinaryOperator
		wantRel ir.RelationalFunction
		wantCom ir.BinaryOperator
	}{
		{ir.BinaryEqual, ir.RelationalAll, ir.BinaryLogicalAnd},
		{ir.BinaryNotEqual, ir.RelationalAny, ir.BinaryLogicalOr},
	}
	for _, tt := range tests {
		fe, c := newTestContext(t)
		mat := fe.registerType(ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: f32Scalar})
		left := declareLocal(t, c, "a", mat)
		right := declareLocal(t, c, "b", mat)

		h := binaryLowered(t, c, left, right, tt.op)
		combine := exprAs[ir.ExprBinary](t, c, h, "the fold of the column comparisons")
		if combine.Op != tt.wantCom {
			t.Fatalf("fold operation = %v, want %v", combine.Op, tt.wantCom)
		}
		for _, side := range []ir.ExpressionHandle{combine.Left, combine.Right} {
			rel := exprAs[ir.ExprRelational](t, c, side, "a column comparison")
			if rel.Fun != tt.wantRel {
				t.Fatalf("column comparison function = %v, want %v", rel.Fun, tt.wantRel)
			}
		}
	}
}

// TestLowerMatrixMultiplyStaysBinary keeps the multiplication the IR does have
// (context.rs:726-729): the dimensions must agree on columns == rows and the
// result is a plain binary expression.
func TestLowerMatrixMultiplyStaysBinary(t *testing.T) {
	fe, c := newTestContext(t)
	square := fe.registerType(ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: f32Scalar})
	left := declareLocal(t, c, "a", square)
	right := declareLocal(t, c, "b", square)

	h := binaryLowered(t, c, left, right, ir.BinaryMultiply)
	product := exprAs[ir.ExprBinary](t, c, h, "the matrix product")
	if product.Op != ir.BinaryMultiply {
		t.Fatalf("operation = %v, want the multiplication itself", product.Op)
	}
}

// TestLowerMatrixBinaryReportsMismatch is the dimension/scalar check of
// context.rs:616-619: an operation the shapes do not allow is reported with
// Rust's Debug spelling of both types and lowering goes on.
func TestLowerMatrixBinaryReportsMismatch(t *testing.T) {
	fe, c := newTestContext(t)
	leftTy := ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec3, Scalar: f32Scalar}
	rightTy := ir.MatrixType{Columns: ir.Vec3, Rows: ir.Vec2, Scalar: f32Scalar}
	left := mustExpr(t, c, ir.Literal{Value: ir.LiteralF32(0)})
	right := mustExpr(t, c, ir.Literal{Value: ir.LiteralF32(0)})

	if _, err := c.lowerMatrixBinary(left, right, leftTy, rightTy, ir.BinaryAdd, ir.Span{}); err != nil {
		t.Fatalf("lowerMatrixBinary: %v", err)
	}
	if len(fe.errors) == 0 {
		t.Fatal("a matrix operation with mismatched dimensions recorded no error")
	}
	message := fe.errors[0].Message
	for _, want := range []string{"Cannot apply operation to", "rows: Tri", "columns: Tri"} {
		if !strings.Contains(message, want) {
			t.Fatalf("error = %q, want it to name the types (%q)", message, want)
		}
	}
}

// TestLowerScalarVectorBinarySplats is context.rs:812-832: the scalar left
// operand of a division (and the other listed operations) is splatted to the
// vector's size; a multiplication keeps the plain binary expression.
func TestLowerScalarVectorBinarySplats(t *testing.T) {
	tests := []struct {
		op       ir.BinaryOperator
		wantLeft bool // the left operand becomes a splat
	}{
		{ir.BinaryDivide, true},
		{ir.BinaryAdd, true},
		{ir.BinaryMultiply, false},
	}
	for _, tt := range tests {
		fe, c := newTestContext(t)
		vec := fe.registerType(ir.VectorType{Size: ir.Vec3, Scalar: f32Scalar})
		scalar := declareLocal(t, c, "s", fe.registerType(f32Scalar))
		vector := declareLocal(t, c, "v", vec)

		h := binaryLowered(t, c, scalar, vector, tt.op)
		binary := exprAs[ir.ExprBinary](t, c, h, "the scalar-vector operation")
		_, isSplat := c.Expression(binary.Left).(ir.ExprSplat)
		if isSplat != tt.wantLeft {
			t.Fatalf("%v: left operand is %T, want splat %t", tt.op, c.Expression(binary.Left), tt.wantLeft)
		}
	}
}

// TestLowerVectorScalarBinarySplats is context.rs:789-811: the scalar right
// operand of an addition is splatted; a multiplication keeps it a scalar.
func TestLowerVectorScalarBinarySplats(t *testing.T) {
	tests := []struct {
		op        ir.BinaryOperator
		wantSplat bool
	}{
		{ir.BinaryAdd, true},
		{ir.BinarySubtract, true},
		{ir.BinaryMultiply, false},
		{ir.BinaryDivide, true},
	}
	for _, tt := range tests {
		fe, c := newTestContext(t)
		vec := fe.registerType(ir.VectorType{Size: ir.Vec3, Scalar: f32Scalar})
		vector := declareLocal(t, c, "v", vec)
		scalar := declareLocal(t, c, "s", fe.registerType(f32Scalar))

		h := binaryLowered(t, c, vector, scalar, tt.op)
		binary := exprAs[ir.ExprBinary](t, c, h, "the vector-scalar operation")
		splat, isSplat := c.Expression(binary.Right).(ir.ExprSplat)
		if isSplat != tt.wantSplat {
			t.Fatalf("%v: right operand is %T, want splat %t", tt.op, c.Expression(binary.Right), tt.wantSplat)
		}
		if isSplat && splat.Size != ir.Vec3 {
			t.Fatalf("%v: splat size = %v, want vec3", tt.op, splat.Size)
		}
	}
}

// TestLowerVectorBinaryComparisonIsRelational is context.rs:769-788: a vector
// comparison produces a vector of bools folded by All (equality) or Any.
func TestLowerVectorBinaryComparisonIsRelational(t *testing.T) {
	tests := []struct {
		op   ir.BinaryOperator
		want ir.RelationalFunction
	}{
		{ir.BinaryEqual, ir.RelationalAll},
		{ir.BinaryNotEqual, ir.RelationalAny},
	}
	for _, tt := range tests {
		fe, c := newTestContext(t)
		vec := fe.registerType(ir.VectorType{Size: ir.Vec2, Scalar: f32Scalar})
		left := declareLocal(t, c, "a", vec)
		right := declareLocal(t, c, "b", vec)

		h := binaryLowered(t, c, left, right, tt.op)
		rel := exprAs[ir.ExprRelational](t, c, h, "the folded comparison")
		if rel.Fun != tt.want {
			t.Fatalf("%v: relational function = %v, want %v", tt.op, rel.Fun, tt.want)
		}
		compare := exprAs[ir.ExprBinary](t, c, rel.Argument, "the component comparison")
		if compare.Op != tt.op {
			t.Fatalf("%v: component comparison = %v, want %v", tt.op, compare.Op, tt.op)
		}
	}
}

// TestLowerScalarMatrixBinarySplats is context.rs:833-912: an addition,
// subtraction or division of a scalar and a matrix splats the scalar to the
// matrix's rows and applies it to every column.
func TestLowerScalarMatrixBinarySplats(t *testing.T) {
	fe, c := newTestContext(t)
	mat := ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec3, Scalar: f32Scalar}
	matrix := declareLocal(t, c, "m", fe.registerType(mat))
	scalar := declareLocal(t, c, "s", fe.registerType(f32Scalar))

	h := binaryLowered(t, c, scalar, matrix, ir.BinaryAdd)
	compose := exprAs[ir.ExprCompose](t, c, h, "the rebuilt matrix")
	if len(compose.Components) != 2 {
		t.Fatalf("the rebuilt matrix has %d columns, want 2", len(compose.Components))
	}
	column := exprAs[ir.ExprBinary](t, c, compose.Components[0], "the column sum")
	splat := exprAs[ir.ExprSplat](t, c, column.Left, "the splatted scalar")
	if splat.Size != ir.Vec3 {
		t.Fatalf("splat size = %v, want the matrix's rows", splat.Size)
	}
}

// TestLowerMatrixScalarBinarySplats is context.rs:913-993: the same rule with
// the matrix on the left, and a multiplication kept as a plain binary.
func TestLowerMatrixScalarBinarySplats(t *testing.T) {
	tests := []struct {
		op        ir.BinaryOperator
		wantCompo bool
	}{
		{ir.BinaryDivide, true},
		{ir.BinaryMultiply, false},
	}
	for _, tt := range tests {
		fe, c := newTestContext(t)
		mat := ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec3, Scalar: f32Scalar}
		matrix := declareLocal(t, c, "m", fe.registerType(mat))
		scalar := declareLocal(t, c, "s", fe.registerType(f32Scalar))

		h := binaryLowered(t, c, matrix, scalar, tt.op)
		_, isCompose := c.Expression(h).(ir.ExprCompose)
		if isCompose != tt.wantCompo {
			t.Fatalf("%v: result is %T, want compose %t", tt.op, c.Expression(h), tt.wantCompo)
		}
	}
}

// TestLowerScalarMatrixBinaryReportsMismatch is context.rs:850-855: the scalar
// must have the matrix's scalar type, and the mismatch is reported with both
// types named.
func TestLowerScalarMatrixBinaryReportsMismatch(t *testing.T) {
	fe, c := newTestContext(t)
	mat := ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: f32Scalar}
	left := mustExpr(t, c, ir.Literal{Value: ir.LiteralI32(0)})
	right := mustExpr(t, c, ir.Literal{Value: ir.LiteralF32(0)})

	if _, err := c.lowerScalarMatrixBinary(left, right, i32Scalar, mat, ir.BinaryMultiply, ir.Span{}); err != nil {
		t.Fatalf("lowerScalarMatrixBinary: %v", err)
	}
	if len(fe.errors) == 0 || !strings.Contains(fe.errors[0].Message, "Cannot apply operation to Scalar(Scalar { kind: Sint, width: 4 }) and Matrix") {
		t.Fatalf("errors = %+v, want the scalar/matrix mismatch", fe.errors)
	}
}

// TestLowerMatrixScalarBinaryReportsMismatch is the mirror check of
// context.rs:920-925.
func TestLowerMatrixScalarBinaryReportsMismatch(t *testing.T) {
	fe, c := newTestContext(t)
	mat := ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: f32Scalar}
	left := mustExpr(t, c, ir.Literal{Value: ir.LiteralF32(0)})
	right := mustExpr(t, c, ir.Literal{Value: ir.LiteralI32(0)})

	if _, err := c.lowerMatrixScalarBinary(left, right, mat, i32Scalar, ir.BinaryMultiply, ir.Span{}); err != nil {
		t.Fatalf("lowerMatrixScalarBinary: %v", err)
	}
	if len(fe.errors) == 0 || !strings.Contains(fe.errors[0].Message, "Cannot apply operation to Matrix") {
		t.Fatalf("errors = %+v, want the matrix/scalar mismatch", fe.errors)
	}
}

// TestLowerPrePostfixMatrix is context.rs:1232-1313 on a matrix target: the
// one is splatted to the rows and composed into a full matrix, so the addition
// is a whole-matrix operation.
func TestLowerPrePostfixMatrix(t *testing.T) {
	fe, c := newTestContext(t)
	mat := ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: f32Scalar}
	sym := declareLocal(t, c, "m", fe.registerType(mat))

	arena := &hirArena{}
	root := arena.add(hirPrePostfix{Op: ir.BinaryAdd, Postfix: false,
		Expr: arena.add(hirVariable{Sym: sym}, ir.Span{})}, ir.Span{})
	h, _, err := c.lowerExpect(arena, root, rhsPos)
	if err != nil {
		t.Fatalf("lowerExpect: %v", err)
	}
	add := exprAs[ir.ExprBinary](t, c, h, "the prefix increment")
	if add.Op != ir.BinaryAdd {
		t.Fatalf("operation = %v, want an addition", add.Op)
	}
	compose := exprAs[ir.ExprCompose](t, c, add.Right, "the matrix one")
	if len(compose.Components) != 2 {
		t.Fatalf("the matrix one has %d columns, want 2", len(compose.Components))
	}
	stmtAs[ir.StmtStore](t, c.body[len(c.body)-1], "the increment store")
}

// TestLowerPrePostfixVector is the vector arm of the same rule: the one is
// splatted to the vector's size.
func TestLowerPrePostfixVector(t *testing.T) {
	fe, c := newTestContext(t)
	sym := declareLocal(t, c, "v", fe.registerType(ir.VectorType{Size: ir.Vec3, Scalar: f32Scalar}))

	arena := &hirArena{}
	root := arena.add(hirPrePostfix{Op: ir.BinarySubtract, Postfix: true,
		Expr: arena.add(hirVariable{Sym: sym}, ir.Span{})}, ir.Span{})
	if _, _, err := c.lowerExpect(arena, root, rhsPos); err != nil {
		t.Fatalf("lowerExpect: %v", err)
	}
	var add ir.ExprBinary
	for h := range c.expressions {
		if b, ok := c.expressions[h].Kind.(ir.ExprBinary); ok {
			add = b
		}
	}
	splat, ok := c.expressions[add.Right].Kind.(ir.ExprSplat)
	if !ok || splat.Size != ir.Vec3 {
		t.Fatalf("the decrement's right operand = %#v, want a vec3 splat", c.expressions[add.Right].Kind)
	}
}

// TestLowerPrePostfixSwizzle keeps the swizzle itself as the left operand
// (context.rs:1240-1256) and splits the store back into one store per
// component, so `v.xy++` increments the two named components.
func TestLowerPrePostfixSwizzle(t *testing.T) {
	fe, c := newTestContext(t)
	vec4 := fe.registerType(ir.VectorType{Size: ir.Vec4, Scalar: f32Scalar})
	sym := declareLocal(t, c, "v", vec4)

	arena := &hirArena{}
	target := arena.add(hirSelect{Base: arena.add(hirVariable{Sym: sym}, ir.Span{}), Field: "xy"}, ir.Span{})
	root := arena.add(hirPrePostfix{Op: ir.BinaryAdd, Postfix: false, Expr: target}, ir.Span{})
	if _, _, err := c.lowerExpect(arena, root, rhsPos); err != nil {
		t.Fatalf("lowerExpect: %v", err)
	}
	stores := 0
	for _, stmt := range c.body {
		if _, ok := stmt.Kind.(ir.StmtStore); ok {
			stores++
		}
	}
	if stores != 2 {
		t.Fatalf("body has %d stores, want one per swizzled component", stores)
	}
	var hasSwizzle bool
	for _, expr := range c.expressions {
		if _, ok := expr.Kind.(ir.ExprSwizzle); ok {
			hasSwizzle = true
		}
	}
	if !hasSwizzle {
		t.Fatal("the swizzle target was loaded away; the operation should keep the swizzle")
	}
}

// TestLowerPrePostfixRejectsNonNumeric is context.rs:1257-1266: a target that
// is neither scalar, vector nor matrix - or whose scalar has no one - is
// reported and the operand lowered anyway.
func TestLowerPrePostfixRejectsNonNumeric(t *testing.T) {
	tests := []struct {
		name string
		ty   ir.TypeInner
	}{
		{"struct", ir.StructType{}},
		{"bool", boolScalar},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fe, c := newTestContext(t)
			sym := declareLocal(t, c, "x", fe.registerType(tt.ty))
			arena := &hirArena{}
			root := arena.add(hirPrePostfix{Op: ir.BinaryAdd,
				Expr: arena.add(hirVariable{Sym: sym}, ir.Span{})}, ir.Span{})
			if _, _, err := c.lowerExpect(arena, root, rhsPos); err != nil {
				t.Fatalf("lowerExpect: %v", err)
			}
			if len(fe.errors) == 0 || fe.errors[0].Message != "Increment/decrement only works on scalar/vector/matrix" {
				t.Fatalf("errors = %+v, want the non-numeric increment error", fe.errors)
			}
		})
	}
}

// TestLowerMethodLengthOfRuntimeArray is context.rs:1314-1379: a dynamically
// sized array's length is the runtime ArrayLength, converted to int.
func TestLowerMethodLengthOfRuntimeArray(t *testing.T) {
	fe, c := newTestContext(t)
	arrayTy := fe.registerType(ir.ArrayType{Base: fe.registerType(f32Scalar), Size: ir.ArraySize{}, Stride: 4})
	sym := declareLocal(t, c, "a", arrayTy)

	arena := &hirArena{}
	root := arena.add(hirMethod{Expr: arena.add(hirVariable{Sym: sym}, ir.Span{}), Name: "length"}, ir.Span{})
	h, _, err := c.lowerExpect(arena, root, rhsPos)
	if err != nil {
		t.Fatalf("lowerExpect: %v", err)
	}
	as := exprAs[ir.ExprAs](t, c, h, "the length conversion")
	if as.Kind != ir.ScalarSint {
		t.Fatalf("length converted to %v, want sint", as.Kind)
	}
	if _, ok := c.Expression(as.Expr).(ir.ExprArrayLength); !ok {
		t.Fatalf("converted expression is %T, want ExprArrayLength", c.Expression(as.Expr))
	}
}

// TestLowerMethodErrors covers the two diagnostics of the method arm: an
// unknown method name is a lowering error, and arguments given to .length()
// are reported while the length is lowered anyway.
func TestLowerMethodErrors(t *testing.T) {
	t.Run("unknown", func(t *testing.T) {
		fe, c := newTestContext(t)
		sym := declareLocal(t, c, "a", fe.registerType(f32Scalar))
		arena := &hirArena{}
		root := arena.add(hirMethod{Expr: arena.add(hirVariable{Sym: sym}, ir.Span{}), Name: "foo"}, ir.Span{})
		_, _, err := c.lowerExpect(arena, root, rhsPos)
		if err == nil || err.Error() != "unknown method 'foo'" {
			t.Fatalf("error = %v, want Rust's unknown-method error", err)
		}
	})
	t.Run("length with argument", func(t *testing.T) {
		fe, c := newTestContext(t)
		four := uint32(4)
		arrayTy := fe.registerType(ir.ArrayType{Base: fe.registerType(f32Scalar), Size: ir.ArraySize{Constant: &four}, Stride: 4})
		sym := declareLocal(t, c, "a", arrayTy)
		arena := &hirArena{}
		node := hirMethod{Name: "length",
			Expr: arena.add(hirVariable{Sym: sym}, ir.Span{}),
			Args: []hirHandle{arena.add(hirLiteral{Value: ir.LiteralI32(0)}, ir.Span{})},
		}
		root := arena.add(node, ir.Span{})
		if _, _, err := c.lowerExpect(arena, root, rhsPos); err != nil {
			t.Fatalf("lowerExpect: %v", err)
		}
		if len(fe.errors) == 0 || fe.errors[0].Message != ".length() doesn't take any arguments" {
			t.Fatalf("errors = %+v, want the argument error", fe.errors)
		}
	})
}

// TestLowerSequenceYieldsLast is context.rs:1360-1379: a comma expression
// lowers every item and yields the last one's value.
func TestLowerSequenceYieldsLast(t *testing.T) {
	fe, c := newTestContext(t)
	first := declareLocal(t, c, "a", fe.registerType(f32Scalar))
	last := declareLocal(t, c, "b", fe.registerType(ir.VectorType{Size: ir.Vec2, Scalar: f32Scalar}))

	arena := &hirArena{}
	root := arena.add(hirSequence{Exprs: []hirHandle{
		arena.add(hirVariable{Sym: first}, ir.Span{}),
		arena.add(hirVariable{Sym: last}, ir.Span{}),
	}}, ir.Span{})
	h, _, err := c.lowerExpect(arena, root, rhsPos)
	if err != nil {
		t.Fatalf("lowerExpect: %v", err)
	}
	if _, ok := c.Expression(h).(ir.ExprLoad); !ok {
		t.Fatalf("sequence lowered to %T, want the last item's load", c.Expression(h))
	}
	if got, err := c.resolveType(h, ir.Span{}); err != nil {
		t.Fatalf("resolveType: %v", err)
	} else if _, ok := got.(ir.VectorType); !ok {
		t.Fatalf("sequence type = %v, want the last item's vector", got)
	}
}

// TestLowerInnerRejectsLhsForValueKinds is the pos.kind == posLHS guard of
// every value-producing expression kind, Rust's error text
// (context.rs:1380-1388): none of them can be assigned through.
func TestLowerInnerRejectsLhsForValueKinds(t *testing.T) {
	tests := []struct {
		name string
		node hirExpr
	}{
		{"literal", hirLiteral{Value: ir.LiteralF32(1)}},
		{"binary", hirBinary{Op: ir.BinaryAdd}},
		{"unary", hirUnary{Op: ir.UnaryNegate}},
		{"call", hirCall{}},
		{"conditional", hirConditional{}},
		{"assign", hirAssign{}},
		{"prepostfix", hirPrePostfix{}},
		{"method", hirMethod{Name: "length"}},
		{"sequence", hirSequence{Exprs: []hirHandle{0}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, c := newTestContext(t)
			arena := &hirArena{}
			root := arena.add(tt.node, ir.Span{})
			_, _, err := c.lowerExpect(arena, root, lhsPos)
			if err == nil {
				t.Fatalf("lowering %T in Lhs position was accepted", tt.node)
			}
			if got := err.Error(); got != "Expression cannot be in the left hand side" {
				t.Fatalf("error = %q, want Rust's left-hand-side error", got)
			}
		})
	}
}
