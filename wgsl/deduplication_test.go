package wgsl

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// parseWGSL is a helper to parse WGSL source code
func parseWGSL(source string) (*Module, error) {
	lexer := NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		return nil, err
	}
	parser := NewParser(tokens)
	return parser.Parse()
}

func TestTypeDeduplication(t *testing.T) {
	// Test that identical types are deduplicated
	source := `
		struct Vertex {
			position: vec4<f32>,
			color: vec4<f32>,
		}

		@vertex
		fn main(v: Vertex) -> @builtin(position) vec4<f32> {
			return v.position;
		}
	`

	ast, err := parseWGSL(source)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	module, err := Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	// Count types
	typeCount := len(module.Types)

	// Expected types:
	// Built-ins: f32, f16, i32, u32, bool (5 types)
	// User types:
	// 1. vec4<f32> (vector) - deduplicated, should appear only once
	// 2. Vertex (struct)
	// Total: 7 unique types

	if typeCount != 7 {
		t.Errorf("Expected 7 unique types, got %d", typeCount)
		for i, typ := range module.Types {
			t.Logf("Type %d: %s (%T)", i, typ.Name, typ.Inner)
		}
	}

	// Verify vec4<f32> is only registered once
	vec4Count := 0
	for _, typ := range module.Types {
		if vec, ok := typ.Inner.(ir.VectorType); ok {
			if vec.Size == ir.Vec4 && vec.Scalar.Kind == ir.ScalarFloat {
				vec4Count++
			}
		}
	}

	if vec4Count != 1 {
		t.Errorf("Expected vec4<f32> to appear exactly once, got %d occurrences", vec4Count)
	}
}

func TestTypeDeduplicationMultipleStructs(t *testing.T) {
	// Test deduplication across multiple structs using the same types
	source := `
		struct A {
			x: vec4<f32>,
		}

		struct B {
			y: vec4<f32>,
		}

		@vertex
		fn main() -> @builtin(position) vec4<f32> {
			return vec4<f32>(0.0, 0.0, 0.0, 1.0);
		}
	`

	ast, err := parseWGSL(source)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	module, err := Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	// Expected types:
	// Built-ins: f32, f16, i32, u32, bool (5 types)
	// User types:
	// 1. vec4<f32> (vector) - used in both structs but deduplicated
	// 2. A (struct)
	// 3. B (struct)
	// Total: 8 unique types

	if len(module.Types) != 8 {
		t.Errorf("Expected 8 unique types, got %d", len(module.Types))
		for i, typ := range module.Types {
			t.Logf("Type %d: %s (%T)", i, typ.Name, typ.Inner)
		}
	}

	// Verify vec4<f32> is only registered once
	vec4Count := 0
	for _, typ := range module.Types {
		if vec, ok := typ.Inner.(ir.VectorType); ok {
			if vec.Size == ir.Vec4 && vec.Scalar.Kind == ir.ScalarFloat {
				vec4Count++
			}
		}
	}

	if vec4Count != 1 {
		t.Errorf("Expected vec4<f32> to appear exactly once, got %d occurrences", vec4Count)
	}
}

func TestTypeDeduplicationMatrices(t *testing.T) {
	// Test deduplication of matrix types
	source := `
		struct Transforms {
			model: mat4x4<f32>,
			view: mat4x4<f32>,
			projection: mat4x4<f32>,
		}

		@vertex
		fn main() -> @builtin(position) vec4<f32> {
			return vec4<f32>(0.0, 0.0, 0.0, 1.0);
		}
	`

	ast, err := parseWGSL(source)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	module, err := Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	// Count mat4x4<f32> occurrences
	mat4Count := 0
	for _, typ := range module.Types {
		if mat, ok := typ.Inner.(ir.MatrixType); ok {
			if mat.Columns == ir.Vec4 && mat.Rows == ir.Vec4 && mat.Scalar.Kind == ir.ScalarFloat {
				mat4Count++
			}
		}
	}

	if mat4Count != 1 {
		t.Errorf("Expected mat4x4<f32> to appear exactly once, got %d occurrences", mat4Count)
	}

	t.Logf("Total types: %d", len(module.Types))
	for i, typ := range module.Types {
		t.Logf("Type %d: %s (%T)", i, typ.Name, typ.Inner)
	}
}
