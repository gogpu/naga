package front

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// bodyExprOf parses body as the contents of main and returns the expression
// arena of its last statement, which must be an expression statement.
func bodyExprOf(t *testing.T, body string) *exprs {
	t.Helper()
	stmts := mainBody(t, body)
	if len(stmts) == 0 {
		t.Fatal("main has no statements")
	}
	e := wantStmtKind[stmtExpr](t, stmts[len(stmts)-1]).Expr
	return &e
}

// sourceExprOf parses a complete source and returns the expression arena of the
// last statement of main, which must be an expression statement. It is for
// sources that declare a prototype or a module-scope struct before main.
func sourceExprOf(t *testing.T, src string) *exprs {
	t.Helper()
	fn := funcDecl(t, wantParsed(t, versioned(src)), "main")
	if len(fn.Body) == 0 {
		t.Fatal("main has no statements")
	}
	e := wantStmtKind[stmtExpr](t, fn.Body[len(fn.Body)-1]).Expr
	return &e
}

// wantExprKind asserts the kind of the HIR expression at h and returns it.
func wantExprKind[T hirExpr](t *testing.T, e *exprs, h hirHandle) T {
	t.Helper()
	kind, ok := e.Arena.get(h).Kind.(T)
	if !ok {
		var want T
		t.Fatalf("expression %d = %T, want %T", h, e.Arena.get(h).Kind, want)
	}
	return kind
}

// wantVariableName asserts the expression at h is a variable reference and
// returns the name it refers to.
func wantVariableName(t *testing.T, e *exprs, h hirHandle) string {
	t.Helper()
	return wantExprKind[hirVariable](t, e, h).Sym.Name
}

func TestParseAccessExpression(t *testing.T) {
	t.Run("index", func(t *testing.T) {
		e := bodyExprOf(t, "float a[3];\na[0];")
		access := wantExprKind[hirAccess](t, e, e.Root)
		if got := wantVariableName(t, e, access.Base); got != "a" {
			t.Errorf("base = %q, want a", got)
		}
		if got := wantExprKind[hirLiteral](t, e, access.Index).Value; got != ir.LiteralI32(0) {
			t.Errorf("index = %v, want the literal 0", got)
		}
	})

	t.Run("chained", func(t *testing.T) {
		e := bodyExprOf(t, "mat4 m;\nint i;\nint j;\nm[i][j];")
		outer := wantExprKind[hirAccess](t, e, e.Root)
		inner := wantExprKind[hirAccess](t, e, outer.Base)
		if got := wantVariableName(t, e, inner.Base); got != "m" {
			t.Errorf("base = %q, want m", got)
		}
		if got := wantVariableName(t, e, inner.Index); got != "i" {
			t.Errorf("inner index = %q, want i", got)
		}
		if got := wantVariableName(t, e, outer.Index); got != "j" {
			t.Errorf("outer index = %q, want j", got)
		}
		if outer.Index == inner.Index {
			t.Error("both accesses index the same expression handle")
		}
	})
}

func TestParseSelectExpression(t *testing.T) {
	t.Run("field", func(t *testing.T) {
		e := bodyExprOf(t, "struct S { float f; };\nS s;\ns.f;")
		sel := wantExprKind[hirSelect](t, e, e.Root)
		if sel.Field != "f" {
			t.Errorf("field = %q, want f", sel.Field)
		}
		if got := wantVariableName(t, e, sel.Base); got != "s" {
			t.Errorf("base = %q, want s", got)
		}
	})

	t.Run("swizzle", func(t *testing.T) {
		e := bodyExprOf(t, "vec4 v;\nv.xyzw;")
		sel := wantExprKind[hirSelect](t, e, e.Root)
		if sel.Field != "xyzw" {
			t.Errorf("field = %q, want xyzw", sel.Field)
		}
		if got := wantVariableName(t, e, sel.Base); got != "v" {
			t.Errorf("base = %q, want v", got)
		}
	})
}

func TestParseMethodCall(t *testing.T) {
	e := bodyExprOf(t, "float a[3];\na.length();")
	method := wantExprKind[hirMethod](t, e, e.Root)
	if method.Name != "length" {
		t.Errorf("method = %q, want length", method.Name)
	}
	if len(method.Args) != 0 {
		t.Errorf("%d arguments, want none", len(method.Args))
	}
	if got := wantVariableName(t, e, method.Expr); got != "a" {
		t.Errorf("receiver = %q, want a", got)
	}
}

func TestParseCallExpressions(t *testing.T) {
	t.Run("builtin type constructor", func(t *testing.T) {
		e := bodyExprOf(t, "vec4(1.0);")
		call := wantExprKind[hirCall](t, e, e.Root)
		if call.Callee.Kind != calleeTypeName {
			t.Errorf("callee kind = %v, want a builtin type constructor", call.Callee.Kind)
		}
		if call.Callee.Typ == nil || call.Callee.Typ.Builtin == nil {
			t.Errorf("callee type = %+v, want a builtin type", call.Callee.Typ)
		}
		if len(call.Args) != 1 {
			t.Fatalf("%d arguments, want 1", len(call.Args))
		}
		if got := wantExprKind[hirLiteral](t, e, call.Args[0]).Value; got != ir.LiteralF32(1.0) {
			t.Errorf("argument = %v, want the literal 1.0", got)
		}
	})

	t.Run("struct constructor", func(t *testing.T) {
		e := bodyExprOf(t, "struct S { float x; };\nS(1.0);")
		call := wantExprKind[hirCall](t, e, e.Root)
		if call.Callee.Kind != calleeStruct {
			t.Errorf("callee kind = %v, want a struct constructor", call.Callee.Kind)
		}
		if call.Callee.Typ == nil || call.Callee.Typ.Name != "S" {
			t.Errorf("callee type = %+v, want the named type S", call.Callee.Typ)
		}
		if len(call.Args) != 1 {
			t.Errorf("%d arguments, want 1", len(call.Args))
		}
	})

	t.Run("function call", func(t *testing.T) {
		e := sourceExprOf(t, "float f(float x);\nvoid main() { f(1.0); }")
		call := wantExprKind[hirCall](t, e, e.Root)
		if call.Callee.Kind != calleeFunction || call.Callee.Name != "f" {
			t.Errorf("callee = %+v, want the function f", call.Callee)
		}
		if len(call.Args) != 1 {
			t.Errorf("%d arguments, want 1", len(call.Args))
		}
	})
}

func TestParseSequenceExpression(t *testing.T) {
	t.Run("parenthesised", func(t *testing.T) {
		e := bodyExprOf(t, "int a;\nint b;\n(a, b);")
		seq := wantExprKind[hirSequence](t, e, e.Root)
		if len(seq.Exprs) != 2 {
			t.Fatalf("%d expressions, want 2", len(seq.Exprs))
		}
		if got := wantVariableName(t, e, seq.Exprs[0]); got != "a" {
			t.Errorf("first expression = %q, want a", got)
		}
		if got := wantVariableName(t, e, seq.Exprs[1]); got != "b" {
			t.Errorf("second expression = %q, want b", got)
		}
	})

	t.Run("expression statement", func(t *testing.T) {
		e := bodyExprOf(t, "int a;\nint b;\na, b;")
		seq := wantExprKind[hirSequence](t, e, e.Root)
		if len(seq.Exprs) != 2 {
			t.Fatalf("%d expressions, want 2", len(seq.Exprs))
		}
	})
}

func TestParseConditionalExpression(t *testing.T) {
	t.Run("operands", func(t *testing.T) {
		e := bodyExprOf(t, "int a;\nint b;\ntrue ? a : b;")
		cond := wantExprKind[hirConditional](t, e, e.Root)
		if got := wantExprKind[hirLiteral](t, e, cond.Condition).Value; got != ir.LiteralBool(true) {
			t.Errorf("condition = %v, want the literal true", got)
		}
		if got := wantVariableName(t, e, cond.Accept); got != "a" {
			t.Errorf("accept = %q, want a", got)
		}
		if got := wantVariableName(t, e, cond.Reject); got != "b" {
			t.Errorf("reject = %q, want b", got)
		}
	})

	t.Run("comma in the accept branch", func(t *testing.T) {
		e := bodyExprOf(t, "int a;\nint b;\nint c;\ntrue ? a, b : c;")
		cond := wantExprKind[hirConditional](t, e, e.Root)
		seq := wantExprKind[hirSequence](t, e, cond.Accept)
		if len(seq.Exprs) != 2 {
			t.Errorf("%d accept expressions, want 2", len(seq.Exprs))
		}
		if got := wantVariableName(t, e, cond.Reject); got != "c" {
			t.Errorf("reject = %q, want c", got)
		}
	})

	t.Run("nested ternary in the reject branch", func(t *testing.T) {
		e := bodyExprOf(t, "int a;\nint b;\nint c;\ntrue ? a : false ? b : c;")
		outer := wantExprKind[hirConditional](t, e, e.Root)
		inner := wantExprKind[hirConditional](t, e, outer.Reject)
		if got := wantVariableName(t, e, inner.Accept); got != "b" {
			t.Errorf("inner accept = %q, want b", got)
		}
		if got := wantVariableName(t, e, inner.Reject); got != "c" {
			t.Errorf("inner reject = %q, want c", got)
		}
	})
}

func TestParseUnaryExpression(t *testing.T) {
	tests := []struct {
		name string
		body string
		op   ir.UnaryOperator
	}{
		{"negate", "float x;\n-x;", ir.UnaryNegate},
		{"logical not", "bool x;\n!x;", ir.UnaryLogicalNot},
		{"bitwise not", "int x;\n~x;", ir.UnaryBitwiseNot},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := bodyExprOf(t, tt.body)
			unary := wantExprKind[hirUnary](t, e, e.Root)
			if unary.Op != tt.op {
				t.Errorf("operator = %v, want %v", unary.Op, tt.op)
			}
			if got := wantVariableName(t, e, unary.Expr); got != "x" {
				t.Errorf("operand = %q, want x", got)
			}
		})
	}

	t.Run("unary plus is the identity", func(t *testing.T) {
		e := bodyExprOf(t, "int x;\n+x;")
		wantExprKind[hirVariable](t, e, e.Root)
		if got := len(e.Arena.Nodes); got != 1 {
			t.Errorf("%d arena nodes, want 1: unary plus adds no node", got)
		}
	})

	t.Run("nested negation", func(t *testing.T) {
		e := bodyExprOf(t, "float x;\n- -x;")
		outer := wantExprKind[hirUnary](t, e, e.Root)
		inner := wantExprKind[hirUnary](t, e, outer.Expr)
		if outer.Op != ir.UnaryNegate || inner.Op != ir.UnaryNegate {
			t.Errorf("operators = %v and %v, want two negations", outer.Op, inner.Op)
		}
		if got := wantVariableName(t, e, inner.Expr); got != "x" {
			t.Errorf("operand = %q, want x", got)
		}
	})
}

func TestParsePrePostfixExpression(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		op      ir.BinaryOperator
		postfix bool
	}{
		{"prefix increment", "int a;\n++a;", ir.BinaryAdd, false},
		{"prefix decrement", "int a;\n--a;", ir.BinarySubtract, false},
		{"postfix increment", "int a;\na++;", ir.BinaryAdd, true},
		{"postfix decrement", "int a;\na--;", ir.BinarySubtract, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := bodyExprOf(t, tt.body)
			pp := wantExprKind[hirPrePostfix](t, e, e.Root)
			if pp.Op != tt.op {
				t.Errorf("operator = %v, want %v", pp.Op, tt.op)
			}
			if pp.Postfix != tt.postfix {
				t.Errorf("postfix = %t, want %t", pp.Postfix, tt.postfix)
			}
			if got := wantVariableName(t, e, pp.Expr); got != "a" {
				t.Errorf("operand = %q, want a", got)
			}
		})
	}
}

func TestParseLiteralExpressions(t *testing.T) {
	tests := []struct {
		name string
		body string
		want ir.LiteralValue
	}{
		{"signed int", "1;", ir.LiteralI32(1)},
		{"unsigned int", "1u;", ir.LiteralU32(1)},
		{"float", "1.5;", ir.LiteralF32(1.5)},
		{"true", "true;", ir.LiteralBool(true)},
		{"false", "false;", ir.LiteralBool(false)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := bodyExprOf(t, tt.body)
			lit := wantExprKind[hirLiteral](t, e, e.Root)
			if lit.Value != tt.want {
				t.Errorf("literal = %#v (%T), want %#v (%T)", lit.Value, lit.Value, tt.want, tt.want)
			}
		})
	}
}

func TestParseOperatorPrecedence(t *testing.T) {
	t.Run("multiplication before addition", func(t *testing.T) {
		e := bodyExprOf(t, "1 + 2 * 3;")
		add := wantExprKind[hirBinary](t, e, e.Root)
		if add.Op != ir.BinaryAdd {
			t.Errorf("root operator = %v, want addition", add.Op)
		}
		if got := wantExprKind[hirLiteral](t, e, add.Left).Value; got != ir.LiteralI32(1) {
			t.Errorf("left operand = %v, want the literal 1", got)
		}
		mul := wantExprKind[hirBinary](t, e, add.Right)
		if mul.Op != ir.BinaryMultiply {
			t.Errorf("right operand operator = %v, want multiplication", mul.Op)
		}
		if got := wantExprKind[hirLiteral](t, e, mul.Left).Value; got != ir.LiteralI32(2) {
			t.Errorf("multiplication left = %v, want the literal 2", got)
		}
		if got := wantExprKind[hirLiteral](t, e, mul.Right).Value; got != ir.LiteralI32(3) {
			t.Errorf("multiplication right = %v, want the literal 3", got)
		}
	})

	t.Run("subtraction is left associative", func(t *testing.T) {
		e := bodyExprOf(t, "1 - 2 - 3;")
		outer := wantExprKind[hirBinary](t, e, e.Root)
		if outer.Op != ir.BinarySubtract {
			t.Errorf("root operator = %v, want subtraction", outer.Op)
		}
		inner := wantExprKind[hirBinary](t, e, outer.Left)
		if inner.Op != ir.BinarySubtract {
			t.Errorf("left operand operator = %v, want subtraction", inner.Op)
		}
		if got := wantExprKind[hirLiteral](t, e, outer.Right).Value; got != ir.LiteralI32(3) {
			t.Errorf("right operand = %v, want the literal 3", got)
		}
	})

	t.Run("and binds tighter than or", func(t *testing.T) {
		e := bodyExprOf(t, "bool a;\nbool b;\nbool c;\na || b && c;")
		or := wantExprKind[hirBinary](t, e, e.Root)
		if or.Op != ir.BinaryLogicalOr {
			t.Errorf("root operator = %v, want logical or", or.Op)
		}
		if got := wantVariableName(t, e, or.Left); got != "a" {
			t.Errorf("left operand = %q, want a", got)
		}
		and := wantExprKind[hirBinary](t, e, or.Right)
		if and.Op != ir.BinaryLogicalAnd {
			t.Errorf("right operand operator = %v, want logical and", and.Op)
		}
		if got := wantVariableName(t, e, and.Left); got != "b" {
			t.Errorf("conjunction left = %q, want b", got)
		}
		if got := wantVariableName(t, e, and.Right); got != "c" {
			t.Errorf("conjunction right = %q, want c", got)
		}
	})

	t.Run("assignment is right associative", func(t *testing.T) {
		e := bodyExprOf(t, "int a;\nint b;\nint c;\na = b = c;")
		outer := wantExprKind[hirAssign](t, e, e.Root)
		if got := wantVariableName(t, e, outer.Tgt); got != "a" {
			t.Errorf("outer target = %q, want a", got)
		}
		inner := wantExprKind[hirAssign](t, e, outer.Value)
		if got := wantVariableName(t, e, inner.Tgt); got != "b" {
			t.Errorf("inner target = %q, want b", got)
		}
		if got := wantVariableName(t, e, inner.Value); got != "c" {
			t.Errorf("inner value = %q, want c", got)
		}
	})

	t.Run("relational binds tighter than equality", func(t *testing.T) {
		e := bodyExprOf(t, "int a;\nint b;\nint c;\na < b == c;")
		eq := wantExprKind[hirBinary](t, e, e.Root)
		if eq.Op != ir.BinaryEqual {
			t.Errorf("root operator = %v, want equality", eq.Op)
		}
		less := wantExprKind[hirBinary](t, e, eq.Left)
		if less.Op != ir.BinaryLess {
			t.Errorf("left operand operator = %v, want less than", less.Op)
		}
		if got := wantVariableName(t, e, eq.Right); got != "c" {
			t.Errorf("right operand = %q, want c", got)
		}
	})

	t.Run("additive binds tighter than shift", func(t *testing.T) {
		e := bodyExprOf(t, "int a;\na << 1 + 1;")
		shift := wantExprKind[hirBinary](t, e, e.Root)
		if shift.Op != ir.BinaryShiftLeft {
			t.Errorf("root operator = %v, want a left shift", shift.Op)
		}
		add := wantExprKind[hirBinary](t, e, shift.Right)
		if add.Op != ir.BinaryAdd {
			t.Errorf("right operand operator = %v, want addition", add.Op)
		}
	})

	t.Run("logical xor is inequality", func(t *testing.T) {
		e := bodyExprOf(t, "bool a;\nbool b;\na ^^ b;")
		bin := wantExprKind[hirBinary](t, e, e.Root)
		if bin.Op != ir.BinaryNotEqual {
			t.Errorf("operator = %v, want not-equal, which is what ^^ lowers to", bin.Op)
		}
	})
}

func TestParseCompoundAssignment(t *testing.T) {
	tests := []struct {
		op   string
		want ir.BinaryOperator
	}{
		{"+=", ir.BinaryAdd},
		{"-=", ir.BinarySubtract},
		{"*=", ir.BinaryMultiply},
		{"/=", ir.BinaryDivide},
		{"%=", ir.BinaryModulo},
		{"<<=", ir.BinaryShiftLeft},
		{">>=", ir.BinaryShiftRight},
		{"&=", ir.BinaryAnd},
		{"^=", ir.BinaryExclusiveOr},
		{"|=", ir.BinaryInclusiveOr},
	}
	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			e := bodyExprOf(t, "int a;\nint b;\na "+tt.op+" b;")
			assign := wantExprKind[hirAssign](t, e, e.Root)
			if got := wantVariableName(t, e, assign.Tgt); got != "a" {
				t.Errorf("target = %q, want a", got)
			}
			binary := wantExprKind[hirBinary](t, e, assign.Value)
			if binary.Op != tt.want {
				t.Errorf("operator = %v, want %v", binary.Op, tt.want)
			}
			if binary.Left != assign.Tgt {
				t.Errorf("binary left handle = %d, want the assignment target %d", binary.Left, assign.Tgt)
			}
			if got := wantVariableName(t, e, binary.Right); got != "b" {
				t.Errorf("right operand = %q, want b", got)
			}
		})
	}
}

func TestParsePrimaryLiteralWidth(t *testing.T) {
	// The preprocessor only produces 32-bit literal tokens, so these guards are
	// unreachable through a shader and are exercised on a parser built directly.
	t.Run("non-32bit integer", func(t *testing.T) {
		p := &parser{tokens: []glslToken{{kind: tokInt, intValue: 7, intSigned: true, intWidth: 16}}}
		e := &exprs{}
		h, err := p.parsePrimary(e)
		if err != nil {
			t.Fatalf("parsePrimary returned %v", err)
		}
		if len(p.errors) != 1 {
			t.Fatalf("%d errors, want 1: %v", len(p.errors), p.errors)
		}
		const want = "Unsupported non-32bit integer"
		if got := p.errors[0].Message; got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
		if got := wantExprKind[hirLiteral](t, e, h).Value; got != ir.LiteralI32(7) {
			t.Errorf("literal = %v, want the 32-bit value 7", got)
		}
	})

	t.Run("64-bit float", func(t *testing.T) {
		p := &parser{
			tokens:  []glslToken{{kind: tokFloat, floatValue: 1.5, floatWidth: 64}},
			dialect: dialect{version: 450, profile: ppProfileCore},
		}
		e := &exprs{}
		h, err := p.parsePrimary(e)
		if err != nil {
			t.Fatalf("parsePrimary returned %v", err)
		}
		if len(p.errors) != 0 {
			t.Fatalf("%d errors, want none: %v", len(p.errors), p.errors)
		}
		if got := wantExprKind[hirLiteral](t, e, h).Value; got != ir.LiteralF64(1.5) {
			t.Errorf("literal = %v, want the double-precision value 1.5", got)
		}
	})

	t.Run("64-bit float in GL 3.30", func(t *testing.T) {
		p := &parser{
			tokens:  []glslToken{{kind: tokFloat, floatValue: 1.5, floatWidth: 64}},
			dialect: dialect{version: 330, profile: ppProfileCore},
		}
		e := &exprs{}
		h, err := p.parsePrimary(e)
		if err != nil {
			t.Fatalf("parsePrimary returned %v", err)
		}
		if len(p.errors) != 1 || !strings.Contains(p.errors[0].Message, "double") {
			t.Fatalf("errors = %v, want one naming the double rule", p.errors)
		}
		if got := wantExprKind[hirLiteral](t, e, h).Value; got != ir.LiteralF64(1.5) {
			t.Errorf("literal = %v, want the double-precision value 1.5", got)
		}
	})

	t.Run("16-bit float", func(t *testing.T) {
		p := &parser{
			tokens:  []glslToken{{kind: tokFloat, floatValue: 1.5, floatWidth: 16}},
			dialect: dialect{version: 450, profile: ppProfileCore},
		}
		e := &exprs{}
		h, err := p.parsePrimary(e)
		if err != nil {
			t.Fatalf("parsePrimary returned %v", err)
		}
		if len(p.errors) != 0 {
			t.Fatalf("%d errors, want none: %v", len(p.errors), p.errors)
		}
		if got := wantExprKind[hirLiteral](t, e, h).Value; got != ir.LiteralF16(1.5) {
			t.Errorf("literal = %v, want the half-precision value 1.5", got)
		}
	})

	t.Run("16-bit float in GLSL ES 3.00", func(t *testing.T) {
		p := &parser{
			tokens:  []glslToken{{kind: tokFloat, floatValue: 1.5, floatWidth: 16}},
			dialect: dialect{version: 300, profile: ppProfileES},
		}
		e := &exprs{}
		if _, err := p.parsePrimary(e); err != nil {
			t.Fatalf("parsePrimary returned %v", err)
		}
		if len(p.errors) != 1 || !strings.Contains(p.errors[0].Message, "float16") {
			t.Fatalf("errors = %v, want one naming the float16 rule", p.errors)
		}
	})
}
