package front

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// mustExpr appends kind and fails the test on error.
func mustExpr(t *testing.T, c *Context, kind ir.ExpressionKind) ir.ExpressionHandle {
	t.Helper()
	h, err := c.addExpression(kind, ir.Span{})
	if err != nil {
		t.Fatalf("addExpression(%T): %v", kind, err)
	}
	return h
}

// declareLocal declares a writable local of ty and returns its symbol.
func declareLocal(t *testing.T, c *Context, name string, ty ir.TypeHandle) *glslSymbol {
	t.Helper()
	h := ir.LocalVariableHandle(len(c.locals))
	c.locals = append(c.locals, ir.LocalVariable{Name: name, Type: ty})
	expr := mustExpr(t, c, ir.ExprLocalVariable{Variable: h})
	sym := &glslSymbol{Kind: symLocal, Name: name}
	c.addLocalVar(sym, expr, true)
	return sym
}

// declareOutParam declares an out parameter of ty and returns its symbol.
func declareOutParam(t *testing.T, fe *Frontend, c *Context, name string, ty ir.TypeHandle) *glslSymbol {
	t.Helper()
	sym := &glslSymbol{Kind: symParam, Name: name}
	if err := c.addFunctionArg(sym, ty, paramOut, ir.Span{}); err != nil {
		t.Fatalf("addFunctionArg: %v", err)
	}
	return sym
}

func TestLowerLiteral(t *testing.T) {
	_, c := newTestContext(t)
	arena := &hirArena{}
	root := arena.add(hirLiteral{Value: ir.LiteralF32(2.5)}, ir.Span{})

	h, span, err := c.lowerExpect(arena, root, rhsPos)
	if err != nil {
		t.Fatalf("lowerExpect: %v", err)
	}
	if span != (ir.Span{}) {
		t.Fatalf("span = %v, want the node's span", span)
	}
	lit, ok := c.Expression(h).(ir.Literal)
	if !ok || lit.Value != ir.LiteralF32(2.5) {
		t.Fatalf("lowered to %#v, want LiteralF32(2.5)", c.Expression(h))
	}
}

func TestLowerVariableLoadsLocal(t *testing.T) {
	fe, c := newTestContext(t)
	sym := declareLocal(t, c, "a", fe.registerType(f32Scalar))
	arena := &hirArena{}
	root := arena.add(hirVariable{Sym: sym}, ir.Span{})

	h, _, err := c.lowerExpect(arena, root, rhsPos)
	if err != nil {
		t.Fatalf("lowerExpect: %v", err)
	}
	load, ok := c.Expression(h).(ir.ExprLoad)
	if !ok {
		t.Fatalf("lowered to %T, want ExprLoad", c.Expression(h))
	}
	if load.Pointer != c.vars[sym].expr {
		t.Fatalf("load pointer = %d, want the local's expression %d", load.Pointer, c.vars[sym].expr)
	}
}

func TestLowerVariableLhsNeedsMutable(t *testing.T) {
	fe, c := newTestContext(t)
	ty := fe.registerType(f32Scalar)
	h := ir.LocalVariableHandle(len(c.locals))
	c.locals = append(c.locals, ir.LocalVariable{Type: ty})
	expr := mustExpr(t, c, ir.ExprLocalVariable{Variable: h})
	sym := &glslSymbol{Kind: symLocal, Name: "c"}
	c.addLocalVar(sym, expr, false)
	arena := &hirArena{}
	root := arena.add(hirVariable{Sym: sym}, ir.Span{})

	got, _, err := c.lowerExpect(arena, root, lhsPos)
	if err != nil {
		t.Fatalf("lowerExpect: %v", err)
	}
	if got != expr {
		t.Fatalf("lhs lowered to %d, want the local itself %d", got, expr)
	}
	if len(c.fe.errors) != 1 || c.fe.errors[0].Message != "Variable cannot be used in LHS position" {
		t.Fatalf("errors = %+v, want the immutability error", c.fe.errors)
	}
}

func TestLowerAccessConstantIndex(t *testing.T) {
	fe, c := newTestContext(t)
	one := uint32(4)
	arrayTy := fe.registerType(ir.ArrayType{Base: fe.registerType(f32Scalar), Size: ir.ArraySize{Constant: &one}, Stride: 4})
	sym := declareLocal(t, c, "a", arrayTy)

	arena := &hirArena{}
	base := arena.add(hirVariable{Sym: sym}, ir.Span{})
	index := arena.add(hirLiteral{Value: ir.LiteralU32(2)}, ir.Span{})
	root := arena.add(hirAccess{Base: base, Index: index}, ir.Span{})

	h, _, err := c.lowerExpect(arena, root, rhsPos)
	if err != nil {
		t.Fatalf("lowerExpect: %v", err)
	}
	// The element is a value in an Rhs position, so the access is loaded.
	load, ok := c.Expression(h).(ir.ExprLoad)
	if !ok {
		t.Fatalf("lowered to %T, want ExprLoad", c.Expression(h))
	}
	acc, ok := c.Expression(load.Pointer).(ir.ExprAccessIndex)
	if !ok {
		t.Fatalf("load pointer is %T, want ExprAccessIndex", c.Expression(load.Pointer))
	}
	if acc.Index != 2 {
		t.Fatalf("access index = %d, want 2", acc.Index)
	}
}

func TestLowerAccessDynamicIndex(t *testing.T) {
	fe, c := newTestContext(t)
	one := uint32(4)
	arrayTy := fe.registerType(ir.ArrayType{Base: fe.registerType(f32Scalar), Size: ir.ArraySize{Constant: &one}, Stride: 4})
	sym := declareLocal(t, c, "a", arrayTy)
	indexSym := declareOutParam(t, fe, c, "i", fe.registerType(i32Scalar))

	arena := &hirArena{}
	base := arena.add(hirVariable{Sym: sym}, ir.Span{})
	// The index is a runtime value: a load of the out parameter.
	indexValue := arena.add(hirVariable{Sym: indexSym}, ir.Span{})
	root := arena.add(hirAccess{Base: base, Index: indexValue}, ir.Span{})

	h, _, err := c.lowerExpect(arena, root, rhsPos)
	if err != nil {
		t.Fatalf("lowerExpect: %v", err)
	}
	load, ok := c.Expression(h).(ir.ExprLoad)
	if !ok {
		t.Fatalf("lowered to %T, want ExprLoad", c.Expression(h))
	}
	if _, ok := c.Expression(load.Pointer).(ir.ExprAccess); !ok {
		t.Fatalf("load pointer is %T, want a dynamic ExprAccess", c.Expression(load.Pointer))
	}
}

func TestLowerUnaryNegateMatrix(t *testing.T) {
	fe, c := newTestContext(t)
	matTy := fe.registerType(ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec3, Scalar: f32Scalar})
	sym := declareLocal(t, c, "m", matTy)

	arena := &hirArena{}
	operand := arena.add(hirVariable{Sym: sym}, ir.Span{})
	root := arena.add(hirUnary{Op: ir.UnaryNegate, Expr: operand}, ir.Span{})

	h, _, err := c.lowerExpect(arena, root, rhsPos)
	if err != nil {
		t.Fatalf("lowerExpect: %v", err)
	}
	mul, ok := c.Expression(h).(ir.ExprBinary)
	if !ok || mul.Op != ir.BinaryMultiply {
		t.Fatalf("lowered to %#v, want a multiplication by -1", c.Expression(h))
	}
	minusOne, ok := c.Expression(mul.Left).(ir.Literal)
	if !ok || minusOne.Value != ir.LiteralF32(-1) {
		t.Fatalf("left operand = %#v, want LiteralF32(-1)", c.Expression(mul.Left))
	}
}

func TestLowerConditional(t *testing.T) {
	fe, c := newTestContext(t)
	boolTy := fe.registerType(boolScalar)
	f32Ty := fe.registerType(f32Scalar)
	cond := declareOutParam(t, fe, c, "c", boolTy)
	accept := declareOutParam(t, fe, c, "a", f32Ty)
	reject := declareOutParam(t, fe, c, "b", f32Ty)

	arena := &hirArena{}
	root := arena.add(hirConditional{
		Condition: arena.add(hirVariable{Sym: cond}, ir.Span{}),
		Accept:    arena.add(hirVariable{Sym: accept}, ir.Span{}),
		Reject:    arena.add(hirVariable{Sym: reject}, ir.Span{}),
	}, ir.Span{})

	h, _, err := c.lowerExpect(arena, root, rhsPos)
	if err != nil {
		t.Fatalf("lowerExpect: %v", err)
	}
	if _, ok := c.Expression(h).(ir.ExprLoad); !ok {
		t.Fatalf("result is %T, want the load of the temporary", c.Expression(h))
	}
	if len(c.locals) != 1 {
		t.Fatalf("context has %d locals, want the one ternary temporary", len(c.locals))
	}
	var ifStmt ir.StmtIf
	found := false
	for _, stmt := range c.body {
		if s, ok := stmt.Kind.(ir.StmtIf); ok {
			ifStmt, found = s, true
		}
	}
	if !found {
		t.Fatalf("body has no StmtIf: %+v", c.body)
	}
	for name, block := range map[string]ir.Block{"accept": ifStmt.Accept, "reject": ifStmt.Reject} {
		last := block[len(block)-1].Kind
		store, ok := last.(ir.StmtStore)
		if !ok {
			t.Fatalf("%s branch ends with %T, want a store to the temporary", name, last)
		}
		if store.Pointer != mustLocalExpr(t, c) {
			t.Fatalf("%s branch stores through %d, want the temporary %d", name, store.Pointer, mustLocalExpr(t, c))
		}
	}
}

// mustLocalExpr returns the expression of the only local of c.
func mustLocalExpr(t *testing.T, c *Context) ir.ExpressionHandle {
	t.Helper()
	for h := ir.ExpressionHandle(0); int(h) < len(c.expressions); h++ {
		if lv, ok := c.expressions[h].Kind.(ir.ExprLocalVariable); ok && int(lv.Variable) == len(c.locals)-1 {
			return h
		}
	}
	t.Fatalf("no expression reads local %d", len(c.locals)-1)
	return 0
}

func TestLowerPrePostfix(t *testing.T) {
	fe, c := newTestContext(t)
	sym := declareLocal(t, c, "i", fe.registerType(u32Scalar))

	arena := &hirArena{}
	operand := arena.add(hirVariable{Sym: sym}, ir.Span{})
	root := arena.add(hirPrePostfix{Op: ir.BinaryAdd, Postfix: true, Expr: operand}, ir.Span{})

	h, _, err := c.lowerExpect(arena, root, rhsPos)
	if err != nil {
		t.Fatalf("lowerExpect: %v", err)
	}
	// A postfix increment yields the old value, which is the load.
	if _, ok := c.Expression(h).(ir.ExprLoad); !ok {
		t.Fatalf("result is %T, want the load of the old value", c.Expression(h))
	}
	var store ir.StmtStore
	found := false
	for _, stmt := range c.body {
		if s, ok := stmt.Kind.(ir.StmtStore); ok {
			store, found = s, true
		}
	}
	if !found {
		t.Fatalf("body has no store: %+v", c.body)
	}
	add, ok := c.Expression(store.Value).(ir.ExprBinary)
	if !ok || add.Op != ir.BinaryAdd {
		t.Fatalf("stored value is %#v, want an addition", c.Expression(store.Value))
	}
}

func TestLowerMethodLength(t *testing.T) {
	fe, c := newTestContext(t)
	four := uint32(4)
	arrayTy := fe.registerType(ir.ArrayType{Base: fe.registerType(f32Scalar), Size: ir.ArraySize{Constant: &four}, Stride: 4})
	sym := declareLocal(t, c, "a", arrayTy)

	arena := &hirArena{}
	object := arena.add(hirVariable{Sym: sym}, ir.Span{})
	root := arena.add(hirMethod{Expr: object, Name: "length"}, ir.Span{})

	h, _, err := c.lowerExpect(arena, root, rhsPos)
	if err != nil {
		t.Fatalf("lowerExpect: %v", err)
	}
	lit, ok := c.Expression(h).(ir.Literal)
	if !ok || lit.Value != ir.LiteralI32(4) {
		t.Fatalf("length lowered to %#v, want LiteralI32(4)", c.Expression(h))
	}
}

func TestLowerAssignStoresConvertedValue(t *testing.T) {
	fe, c := newTestContext(t)
	sym := declareLocal(t, c, "x", fe.registerType(f32Scalar))

	arena := &hirArena{}
	target := arena.add(hirVariable{Sym: sym}, ir.Span{})
	value := arena.add(hirLiteral{Value: ir.LiteralU32(3)}, ir.Span{})
	root := arena.add(hirAssign{Tgt: target, Value: value}, ir.Span{})

	h, _, err := c.lowerExpect(arena, root, rhsPos)
	if err != nil {
		t.Fatalf("lowerExpect: %v", err)
	}
	var store ir.StmtStore
	found := false
	for _, stmt := range c.body {
		if s, ok := stmt.Kind.(ir.StmtStore); ok {
			store, found = s, true
		}
	}
	if !found {
		t.Fatalf("body has no store: %+v", c.body)
	}
	if store.Value != h {
		t.Fatalf("the assignment yields %d, but stored %d", h, store.Value)
	}
	lit, ok := c.Expression(h).(ir.Literal)
	if !ok || lit.Value != ir.LiteralF32(3) {
		t.Fatalf("stored value = %#v, want the u32 literal converted to f32", c.Expression(h))
	}
}

func TestLowerSwizzleStoreSplitsComponents(t *testing.T) {
	fe, c := newTestContext(t)
	vec2 := fe.registerType(ir.VectorType{Size: ir.Vec2, Scalar: f32Scalar})
	dst := declareLocal(t, c, "a", vec2)
	src := declareLocal(t, c, "b", vec2)

	arena := &hirArena{}
	target := arena.add(hirSelect{Base: arena.add(hirVariable{Sym: dst}, ir.Span{}), Field: "xy"}, ir.Span{})
	value := arena.add(hirVariable{Sym: src}, ir.Span{})
	root := arena.add(hirAssign{Tgt: target, Value: value}, ir.Span{})

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
}
