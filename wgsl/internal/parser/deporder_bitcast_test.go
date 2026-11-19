package parser

import "testing"

// TestDependencyOrderBitcastDeps: an identifier inside bitcast<T>(...) is a
// dependency — WGSL module-scope declarations are visible regardless of where
// they appear, so the operand's identifier and the target type's name must
// pull the declaration ahead of the user (issue #261, card P2-WGSL-120).
// Before the fix, collectExprDeps had no *BitcastExpr case and the function
// was ordered before the declarations it reads there.
func TestDependencyOrderBitcastDeps(t *testing.T) {
	returnBitcast := func(target, operand string) *FunctionDecl {
		return &FunctionDecl{Name: "f", Body: &BlockStmt{Statements: []Stmt{
			&ReturnStmt{Value: &BitcastExpr{Type: &NamedType{Name: target}, Expr: &Ident{Name: operand}}},
		}}}
	}

	t.Run("operand identifier", func(t *testing.T) {
		decls := []Decl{
			returnBitcast("f32", "G"),
			&VarDecl{Name: "G", Init: &Literal{Kind: TokenIntLiteral, Value: "1"}},
		}
		ordered := mustOrder(t, decls)
		if _, ok := ordered[0].(*VarDecl); !ok {
			t.Fatalf("expected the later var first, got %T", ordered[0])
		}
	})

	t.Run("operand function call", func(t *testing.T) {
		decls := []Decl{
			&FunctionDecl{Name: "f", Body: &BlockStmt{Statements: []Stmt{
				&ReturnStmt{Value: &BitcastExpr{Type: &NamedType{Name: "f32"},
					Expr: &CallExpr{Func: &Ident{Name: "g"}}}},
			}}},
			&FunctionDecl{Name: "g", Body: &BlockStmt{Statements: []Stmt{&ReturnStmt{}}}},
		}
		ordered := mustOrder(t, decls)
		if got := ordered[0].(*FunctionDecl).Name; got != "g" {
			t.Fatalf("expected g first, got %s", got)
		}
	})

	t.Run("target type name", func(t *testing.T) {
		decls := []Decl{
			returnBitcast("S", "G"),
			&StructDecl{Name: "S", Members: []*StructMember{{Name: "x", Type: &NamedType{Name: "f32"}}}},
			&VarDecl{Name: "G", Init: &Literal{Kind: TokenIntLiteral, Value: "1"}},
		}
		ordered := mustOrder(t, decls)
		if _, ok := ordered[len(ordered)-1].(*FunctionDecl); !ok {
			t.Fatalf("expected the function last, got %T", ordered[len(ordered)-1])
		}
	})
}
