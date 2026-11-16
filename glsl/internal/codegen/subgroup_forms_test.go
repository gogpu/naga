package codegen

// The subgroup forms ADR-014 added to the IR: elect and all-equal, written as
// the GLSL builtins of the basic and vote families.

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestGLSL_SubgroupElect: subgroupElect is a basic-family builtin; the
// backend's feature preamble already requests GL_KHR_shader_subgroup_basic.
func TestGLSL_SubgroupElect(t *testing.T) {
	source, _, err := Compile(glslElectModule(), Options{LangVersion: Version450})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if !strings.Contains(source, "subgroupElect();") {
		t.Errorf("output lacks subgroupElect():\n%s", source)
	}
}

// TestGLSL_SubgroupAllEqual: subgroupAllEqual is a vote-family builtin, and its
// result is the bool the IR expression has even though the argument is a uint.
func TestGLSL_SubgroupAllEqual(t *testing.T) {
	source, _, err := Compile(glslAllEqualModule(), Options{LangVersion: Version450})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if !strings.Contains(source, "subgroupAllEqual(") {
		t.Errorf("output lacks subgroupAllEqual(:\n%s", source)
	}
	// The result is declared bool, not the argument's uint.
	if !strings.Contains(source, "bool _e") {
		t.Errorf("the all-equal result is not a bool:\n%s", source)
	}
}

// glslElectModule builds a compute entry point whose only statement is
// subgroupElect, with a bool result expression.
func glslElectModule() *ir.Module {
	return &ir.Module{
		Types: []ir.Type{{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}}},
		EntryPoints: []ir.EntryPoint{{
			Name:  "main",
			Stage: ir.StageCompute,
			Function: ir.Function{
				Name:        "main",
				Expressions: []ir.Expression{{Kind: ir.ExprSubgroupOperationResult{Type: 0}}},
				Body: []ir.Statement{
					{Kind: ir.StmtSubgroupElect{Result: 0}},
					{Kind: ir.StmtReturn{}},
				},
			},
		}},
	}
}

// glslAllEqualModule builds a compute entry point whose only statement is one
// subgroup all-equal over a uint literal, with a bool result.
func glslAllEqualModule() *ir.Module {
	return &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
		},
		EntryPoints: []ir.EntryPoint{{
			Name:  "main",
			Stage: ir.StageCompute,
			Function: ir.Function{
				Name: "main",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralU32(1)}},     // [0] argument
					{Kind: ir.ExprSubgroupOperationResult{Type: 1}}, // [1] bool result
				},
				Body: []ir.Statement{
					{Kind: ir.StmtSubgroupCollectiveOperation{
						Op:           ir.SubgroupOperationAllEqual,
						CollectiveOp: ir.CollectiveReduce,
						Argument:     0,
						Result:       1,
					}},
					{Kind: ir.StmtReturn{}},
				},
			},
		}},
	}
}
