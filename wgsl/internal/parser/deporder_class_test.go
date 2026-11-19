package parser

import "testing"

// This file closes the dependency-order class of issue #261 (card P2-WGSL-120):
// every AST node kind that can carry a reference must reach the walkers, or a
// module-scope declaration used through that node is ordered too late. The
// walkers are per-node switches, so a new node type is a new hole until the
// exhaustiveness guard below says otherwise.

func collectDeclNames(d Decl) map[string]bool {
	got := make(map[string]bool)
	for _, ref := range collectDeclDependencies(d) {
		got[ref] = true
	}
	return got
}

// TestDependencyOrderConstAssertDeps: const_assert conditions name
// declarations in both walkers — a module-scope declaration is used by the
// condition exactly like by any other expression (issue #261 validation by the
// expert, hole 1).
func TestDependencyOrderConstAssertDeps(t *testing.T) {
	t.Run("module scope", func(t *testing.T) {
		decls := []Decl{
			&ConstAssertDecl{Condition: &BinaryExpr{Op: TokenGreater,
				Left: &Ident{Name: "C"}, Right: &Literal{Value: "0"}}},
			&ConstDecl{Name: "C", Init: &Literal{Value: "1"}},
		}
		ordered := mustOrder(t, decls)
		if _, ok := ordered[0].(*ConstDecl); !ok {
			t.Fatalf("expected the const first, got %T", ordered[0])
		}
	})
	t.Run("function scope", func(t *testing.T) {
		decls := []Decl{
			&FunctionDecl{Name: "f", Body: &BlockStmt{Statements: []Stmt{
				&ConstAssertDecl{Condition: &Ident{Name: "C"}},
			}}},
			&ConstDecl{Name: "C", Init: &Literal{Value: "1"}},
		}
		ordered := mustOrder(t, decls)
		if _, ok := ordered[0].(*ConstDecl); !ok {
			t.Fatalf("expected the const first, got %T", ordered[0])
		}
	})
}

// TestDependencyOrderBindingArrayDeps: a binding_array type names its element
// type and its size expression (expert hole 2).
func TestDependencyOrderBindingArrayDeps(t *testing.T) {
	t.Run("element type", func(t *testing.T) {
		decls := []Decl{
			&VarDecl{Name: "a", Type: &BindingArrayType{Element: &NamedType{Name: "Tex"}}},
			&AliasDecl{Name: "Tex", Type: &NamedType{Name: "texture_2d<f32>"}},
		}
		ordered := mustOrder(t, decls)
		if _, ok := ordered[0].(*AliasDecl); !ok {
			t.Fatalf("expected the alias first, got %T", ordered[0])
		}
	})
	t.Run("size expression", func(t *testing.T) {
		decls := []Decl{
			&VarDecl{Name: "a", Type: &BindingArrayType{
				Element: &NamedType{Name: "texture_2d<f32>"}, Size: &Ident{Name: "N"}}},
			&ConstDecl{Name: "N", Init: &Literal{Value: "4"}},
		}
		ordered := mustOrder(t, decls)
		if _, ok := ordered[0].(*ConstDecl); !ok {
			t.Fatalf("expected the const first, got %T", ordered[0])
		}
	})
}

// TestDependencyOrderAttributeDeps: @workgroup_size takes constant
// expressions, so a name there is a dependency (expert hole 3); the
// keyword-valued attributes (@builtin and friends) are not references.
func TestDependencyOrderAttributeDeps(t *testing.T) {
	t.Run("workgroup_size", func(t *testing.T) {
		decls := []Decl{
			&FunctionDecl{Name: "cs", Attributes: []Attribute{
				{Name: "workgroup_size", Args: []Expr{&Ident{Name: "N"}, &Ident{Name: "M"}}},
			}},
			&ConstDecl{Name: "N", Init: &Literal{Value: "8"}},
			&ConstDecl{Name: "M", Init: &Literal{Value: "1"}},
		}
		ordered := mustOrder(t, decls)
		if _, ok := ordered[len(ordered)-1].(*FunctionDecl); !ok {
			t.Fatalf("expected the function last, got %T", ordered[len(ordered)-1])
		}
	})
	t.Run("builtin names are not references", func(t *testing.T) {
		d := &FunctionDecl{Name: "f", Attributes: []Attribute{
			{Name: "builtin", Args: []Expr{&Ident{Name: "position"}}},
		}}
		if collectDeclNames(d)["position"] {
			t.Fatal("@builtin(position) must not create a dependency on a declaration named position")
		}
	})
}

// TestDependencyOrderWalkersAreExhaustive is the guard the expert recommended:
// one node of every Expr/Type/Stmt/Decl type carrying a sentinel reference in
// each reference position, asserting its walker collects the sentinel. A new
// AST node type that the walkers miss fails here instead of shipping an
// ordering hole. Nodes that carry no reference (literals, break, continue,
// discard) are listed as such and must stay reference-free.
func TestDependencyOrderWalkersAreExhaustive(t *testing.T) {
	sentinel := func() Expr { return &Ident{Name: "ZZSentinel"} }
	sentinelType := func() Type { return &NamedType{Name: "ZZSentinel"} }
	lit := func() Expr { return &Literal{Kind: TokenIntLiteral, Value: "1"} }
	litType := func() Type { return &NamedType{Name: "f32"} }

	exprCases := []struct {
		name string
		node Expr
		ref  bool
	}{
		{"Ident", sentinel(), true},
		{"Literal", lit(), false},
		{"BinaryExpr/left", &BinaryExpr{Left: sentinel(), Right: lit()}, true},
		{"BinaryExpr/right", &BinaryExpr{Left: lit(), Right: sentinel()}, true},
		{"UnaryExpr", &UnaryExpr{Operand: sentinel()}, true},
		{"CallExpr/callee", &CallExpr{Func: &Ident{Name: "ZZSentinel"}}, true},
		{"CallExpr/arg", &CallExpr{Func: &Ident{Name: "f"}, Args: []Expr{sentinel()}}, true},
		{"IndexExpr/base", &IndexExpr{Expr: sentinel(), Index: lit()}, true},
		{"IndexExpr/index", &IndexExpr{Expr: lit(), Index: sentinel()}, true},
		{"MemberExpr", &MemberExpr{Expr: sentinel()}, true},
		{"ConstructExpr/type", &ConstructExpr{Type: sentinelType()}, true},
		{"ConstructExpr/arg", &ConstructExpr{Type: litType(), Args: []Expr{sentinel()}}, true},
		{"BitcastExpr/operand", &BitcastExpr{Type: litType(), Expr: sentinel()}, true},
		{"BitcastExpr/type", &BitcastExpr{Type: sentinelType(), Expr: lit()}, true},
	}
	for _, tc := range exprCases {
		t.Run("expr/"+tc.name, func(t *testing.T) {
			got := make(map[string]bool)
			collectExprDeps(tc.node, nil, func(n string) { got[n] = true })
			if got["ZZSentinel"] != tc.ref {
				t.Fatalf("collectExprDeps collected sentinel = %v, want %v", got["ZZSentinel"], tc.ref)
			}
		})
	}

	typeCases := []struct {
		name string
		node Type
		ref  bool
	}{
		{"NamedType", sentinelType(), true},
		{"NamedType/type param", &NamedType{Name: "array", TypeParams: []Type{sentinelType()}}, true},
		{"ArrayType/element", &ArrayType{Element: sentinelType()}, true},
		{"ArrayType/size", &ArrayType{Element: litType(), Size: sentinel()}, true},
		{"BindingArrayType/element", &BindingArrayType{Element: sentinelType()}, true},
		{"BindingArrayType/size", &BindingArrayType{Element: litType(), Size: sentinel()}, true},
		{"PtrType", &PtrType{PointeeType: sentinelType()}, true},
	}
	for _, tc := range typeCases {
		t.Run("type/"+tc.name, func(t *testing.T) {
			got := make(map[string]bool)
			collectTypeRefs(tc.node, func(n string) { got[n] = true })
			if got["ZZSentinel"] != tc.ref {
				t.Fatalf("collectTypeRefs collected sentinel = %v, want %v", got["ZZSentinel"], tc.ref)
			}
		})
	}

	stmtCases := []struct {
		name string
		node Stmt
		ref  bool
	}{
		{"VarDecl/type", &VarDecl{Name: "v", Type: sentinelType()}, true},
		{"VarDecl/init", &VarDecl{Name: "v", Type: litType(), Init: sentinel()}, true},
		{"ConstDecl/init", &ConstDecl{Name: "c", Type: litType(), Init: sentinel()}, true},
		{"ConstAssertDecl", &ConstAssertDecl{Condition: sentinel()}, true},
		{"AssignStmt/left", &AssignStmt{Left: sentinel(), Right: lit()}, true},
		{"AssignStmt/right", &AssignStmt{Left: lit(), Right: sentinel()}, true},
		{"ReturnStmt", &ReturnStmt{Value: sentinel()}, true},
		{"IfStmt/condition", &IfStmt{Condition: sentinel()}, true},
		{"IfStmt/body", &IfStmt{Condition: lit(), Body: &BlockStmt{Statements: []Stmt{&ExprStmt{Expr: sentinel()}}}}, true},
		{"IfStmt/else", &IfStmt{Condition: lit(), Else: &BlockStmt{Statements: []Stmt{&ExprStmt{Expr: sentinel()}}}}, true},
		{"ForStmt/init", &ForStmt{Init: &VarDecl{Name: "i", Type: litType(), Init: sentinel()}}, true},
		{"ForStmt/condition", &ForStmt{Condition: sentinel()}, true},
		{"ForStmt/update", &ForStmt{Update: &AssignStmt{Left: sentinel(), Right: lit()}}, true},
		{"ForStmt/body", &ForStmt{Body: &BlockStmt{Statements: []Stmt{&ExprStmt{Expr: sentinel()}}}}, true},
		{"WhileStmt/condition", &WhileStmt{Condition: sentinel()}, true},
		{"LoopStmt/body", &LoopStmt{Body: &BlockStmt{Statements: []Stmt{&ExprStmt{Expr: sentinel()}}}}, true},
		{"LoopStmt/continuing", &LoopStmt{Body: &BlockStmt{}, Continuing: &BlockStmt{Statements: []Stmt{&ExprStmt{Expr: sentinel()}}}}, true},
		{"BreakIfStmt", &BreakIfStmt{Condition: sentinel()}, true},
		{"SwitchStmt/selector", &SwitchStmt{Selector: sentinel()}, true},
		{"SwitchStmt/case selector", &SwitchStmt{Selector: lit(), Cases: []*SwitchCaseClause{
			{Selectors: []Expr{sentinel()}},
		}}, true},
		{"SwitchStmt/case body", &SwitchStmt{Selector: lit(), Cases: []*SwitchCaseClause{
			{Body: &BlockStmt{Statements: []Stmt{&ExprStmt{Expr: sentinel()}}}},
		}}, true},
		{"ExprStmt", &ExprStmt{Expr: sentinel()}, true},
		{"BreakStmt", &BreakStmt{}, false},
		{"ContinueStmt", &ContinueStmt{}, false},
		{"DiscardStmt", &DiscardStmt{}, false},
	}
	for _, tc := range stmtCases {
		t.Run("stmt/"+tc.name, func(t *testing.T) {
			got := make(map[string]bool)
			collectStmtDeps(tc.node, make(map[string]bool), func(n string) { got[n] = true })
			if got["ZZSentinel"] != tc.ref {
				t.Fatalf("collectStmtDeps collected sentinel = %v, want %v", got["ZZSentinel"], tc.ref)
			}
		})
	}

	declCases := []struct {
		name string
		node Decl
		ref  bool
	}{
		{"StructDecl/member type", &StructDecl{Name: "S", Members: []*StructMember{{Name: "x", Type: sentinelType()}}}, true},
		{"StructDecl/member attr", &StructDecl{Name: "S", Members: []*StructMember{{Name: "x", Type: litType(),
			Attributes: []Attribute{{Name: "size", Args: []Expr{sentinel()}}}}}}, true},
		{"FunctionDecl/param type", &FunctionDecl{Name: "f", Params: []*Parameter{{Name: "p", Type: sentinelType()}}}, true},
		{"FunctionDecl/param attr", &FunctionDecl{Name: "f", Params: []*Parameter{{Name: "p", Type: litType(),
			Attributes: []Attribute{{Name: "size", Args: []Expr{sentinel()}}}}}}, true},
		{"FunctionDecl/return type", &FunctionDecl{Name: "f", ReturnType: sentinelType()}, true},
		{"FunctionDecl/attr", &FunctionDecl{Name: "f",
			Attributes: []Attribute{{Name: "workgroup_size", Args: []Expr{sentinel()}}}}, true},
		{"FunctionDecl/body", &FunctionDecl{Name: "f", Body: &BlockStmt{Statements: []Stmt{
			&ExprStmt{Expr: sentinel()}}}}, true},
		{"VarDecl/type", &VarDecl{Name: "v", Type: sentinelType()}, true},
		{"VarDecl/attr", &VarDecl{Name: "v", Type: litType(),
			Attributes: []Attribute{{Name: "size", Args: []Expr{sentinel()}}}}, true},
		{"ConstDecl", &ConstDecl{Name: "c", Type: litType(), Init: sentinel()}, true},
		{"OverrideDecl/init", &OverrideDecl{Name: "o", Type: litType(), Init: sentinel()}, true},
		{"OverrideDecl/attr", &OverrideDecl{Name: "o", Type: litType(),
			Attributes: []Attribute{{Name: "id", Args: []Expr{sentinel()}}}}, true},
		{"AliasDecl", &AliasDecl{Name: "a", Type: sentinelType()}, true},
		{"ConstAssertDecl", &ConstAssertDecl{Condition: sentinel()}, true},
	}
	for _, tc := range declCases {
		t.Run("decl/"+tc.name, func(t *testing.T) {
			got := collectDeclNames(tc.node)
			if got["ZZSentinel"] != tc.ref {
				t.Fatalf("collectDeclDependencies collected sentinel = %v, want %v", got["ZZSentinel"], tc.ref)
			}
		})
	}
}
