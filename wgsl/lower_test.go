package wgsl

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

func TestLowerSimpleVertexShader(t *testing.T) {
	// Simple vertex shader:
	// @vertex
	// fn main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
	//     return vec4<f32>(0.0, 0.0, 0.0, 1.0);
	// }

	ast := &Module{
		Functions: []*FunctionDecl{
			{
				Name: "main",
				Params: []*Parameter{
					{
						Name: "idx",
						Type: &NamedType{Name: "u32"},
						Attributes: []Attribute{
							{
								Name: "builtin",
								Args: []Expr{&Ident{Name: "vertex_index"}},
							},
						},
					},
				},
				ReturnType: &NamedType{
					Name:       "vec4",
					TypeParams: []Type{&NamedType{Name: "f32"}},
				},
				Attributes: []Attribute{
					{Name: "vertex"},
					{
						Name: "builtin",
						Args: []Expr{&Ident{Name: "position"}},
					},
				},
				Body: &BlockStmt{
					Statements: []Stmt{
						&ReturnStmt{
							Value: &CallExpr{
								Func: &Ident{Name: "vec4"},
								Args: []Expr{
									&Literal{Kind: TokenFloatLiteral, Value: "0.0"},
									&Literal{Kind: TokenFloatLiteral, Value: "0.0"},
									&Literal{Kind: TokenFloatLiteral, Value: "0.0"},
									&Literal{Kind: TokenFloatLiteral, Value: "1.0"},
								},
							},
						},
					},
				},
			},
		},
	}

	module, err := Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	// Verify module structure
	if len(module.Functions) != 1 {
		t.Errorf("expected 1 function, got %d", len(module.Functions))
	}

	// Verify entry point
	if len(module.EntryPoints) != 1 {
		t.Errorf("expected 1 entry point, got %d", len(module.EntryPoints))
	}

	ep := module.EntryPoints[0]
	if ep.Name != "main" {
		t.Errorf("expected entry point name 'main', got '%s'", ep.Name)
	}
	if ep.Stage != ir.StageVertex {
		t.Errorf("expected vertex stage, got %v", ep.Stage)
	}

	// Verify function
	fn := module.Functions[0]
	if fn.Name != "main" {
		t.Errorf("expected function name 'main', got '%s'", fn.Name)
	}

	if len(fn.Arguments) != 1 {
		t.Errorf("expected 1 argument, got %d", len(fn.Arguments))
	}

	// Verify argument binding
	arg := fn.Arguments[0]
	if arg.Name != "idx" {
		t.Errorf("expected argument name 'idx', got '%s'", arg.Name)
	}
	if arg.Binding == nil {
		t.Fatal("expected argument binding, got nil")
	}

	if binding, ok := (*arg.Binding).(ir.BuiltinBinding); ok {
		if binding.Builtin != ir.BuiltinVertexIndex {
			t.Errorf("expected BuiltinVertexIndex, got %v", binding.Builtin)
		}
	} else {
		t.Errorf("expected BuiltinBinding, got %T", *arg.Binding)
	}

	// Verify return type
	if fn.Result == nil {
		t.Fatal("expected function result, got nil")
	}
	if fn.Result.Binding == nil {
		t.Fatal("expected result binding, got nil")
	}

	if binding, ok := (*fn.Result.Binding).(ir.BuiltinBinding); ok {
		if binding.Builtin != ir.BuiltinPosition {
			t.Errorf("expected BuiltinPosition, got %v", binding.Builtin)
		}
	} else {
		t.Errorf("expected BuiltinBinding, got %T", *fn.Result.Binding)
	}

	// Verify function body has a return statement
	if len(fn.Body) != 1 {
		t.Errorf("expected 1 statement in body, got %d", len(fn.Body))
	}

	if stmt, ok := fn.Body[0].Kind.(ir.StmtReturn); ok {
		if stmt.Value == nil {
			t.Error("expected return value, got nil")
		}
	} else {
		t.Errorf("expected StmtReturn, got %T", fn.Body[0].Kind)
	}

	t.Logf("Successfully lowered vertex shader with %d expressions", len(fn.Expressions))
}

func TestLowerTypes(t *testing.T) {
	tests := []struct {
		name     string
		wgslType Type
		wantKind string
	}{
		{
			name:     "f32 scalar",
			wgslType: &NamedType{Name: "f32"},
			wantKind: "ScalarType",
		},
		{
			name:     "vec3<f32>",
			wgslType: &NamedType{Name: "vec3", TypeParams: []Type{&NamedType{Name: "f32"}}},
			wantKind: "VectorType",
		},
		{
			name:     "mat4x4<f32>",
			wgslType: &NamedType{Name: "mat4x4", TypeParams: []Type{&NamedType{Name: "f32"}}},
			wantKind: "MatrixType",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &Lowerer{
				module: &ir.Module{},
				types:  make(map[string]ir.TypeHandle),
			}
			l.registerBuiltinTypes()

			_, err := l.resolveType(tt.wgslType)
			if err != nil {
				t.Fatalf("resolveType failed: %v", err)
			}

			// Type was added to module
			if len(l.module.Types) == 0 {
				t.Error("expected types in module")
			}
		})
	}
}

func TestLowerExpressions(t *testing.T) {
	l := &Lowerer{
		module: &ir.Module{},
		types:  make(map[string]ir.TypeHandle),
		locals: make(map[string]ir.ExpressionHandle),
	}
	l.registerBuiltinTypes()
	l.currentFunc = &ir.Function{
		Expressions: []ir.Expression{},
	}

	tests := []struct {
		name     string
		expr     Expr
		wantKind string
	}{
		{
			name:     "integer literal",
			expr:     &Literal{Kind: TokenIntLiteral, Value: "42"},
			wantKind: "Literal",
		},
		{
			name:     "float literal",
			expr:     &Literal{Kind: TokenFloatLiteral, Value: "3.14"},
			wantKind: "Literal",
		},
		{
			name:     "bool literal",
			expr:     &Literal{Kind: TokenTrue},
			wantKind: "Literal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var target []ir.Statement
			_, err := l.lowerExpression(tt.expr, &target)
			if err != nil {
				t.Fatalf("lowerExpression failed: %v", err)
			}
		})
	}
}
