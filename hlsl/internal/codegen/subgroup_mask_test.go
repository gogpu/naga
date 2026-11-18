package codegen

// The five ballot-mask built-ins (card P2-IR-041, ADR-014 D5). HLSL has no
// semantic and no mask intrinsic for them, and the spec's mask spans every
// invocation of the wave while WaveActiveBallot(true) holds only the active
// lanes, so the writer refuses with the limitation named.

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestHLSL_SubgroupMaskBuiltinsRefused: every ballot-mask built-in is refused
// with the limitation named, never written with a wrong semantic or value.
func TestHLSL_SubgroupMaskBuiltinsRefused(t *testing.T) {
	for _, builtin := range []ir.BuiltinValue{
		ir.BuiltinSubgroupEqMask, ir.BuiltinSubgroupGeMask, ir.BuiltinSubgroupGtMask,
		ir.BuiltinSubgroupLeMask, ir.BuiltinSubgroupLtMask,
	} {
		_, _, err := Compile(subgroupMaskModule(builtin), DefaultOptions())
		if err == nil {
			t.Fatalf("%s compiled, want a refusal", builtin)
		}
		if !strings.Contains(err.Error(), "ballot-mask") || !strings.Contains(err.Error(), builtin.String()) {
			t.Errorf("%s: error = %v, want the ballot-mask limitation naming the built-in", builtin, err)
		}
	}
}

// TestBuiltInToSemanticSubgroupMasks: the five masks are explicitly
// unsupported, so the semantic table's coverage test can tell an unmapped
// addition from a refused one.
func TestBuiltInToSemanticSubgroupMasks(t *testing.T) {
	for _, builtin := range []ir.BuiltinValue{
		ir.BuiltinSubgroupEqMask, ir.BuiltinSubgroupGeMask, ir.BuiltinSubgroupGtMask,
		ir.BuiltinSubgroupLeMask, ir.BuiltinSubgroupLtMask,
	} {
		if semantic, supported := BuiltInToSemantic(builtin); supported || semantic != "" {
			t.Errorf("BuiltInToSemantic(%s) = %q, %v; want unsupported with no semantic", builtin, semantic, supported)
		}
	}
}

// subgroupMaskModule builds a compute entry point whose only argument is one
// ballot-mask built-in of type vec4<u32>.
func subgroupMaskModule(builtin ir.BuiltinValue) *ir.Module {
	binding := ir.Binding(ir.BuiltinBinding{Builtin: builtin})
	return &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
		},
		EntryPoints: []ir.EntryPoint{{
			Name:      "main",
			Stage:     ir.StageCompute,
			Workgroup: [3]uint32{64, 1, 1},
			Function: ir.Function{
				Name:      "main",
				Arguments: []ir.FunctionArgument{{Name: "mask", Type: 1, Binding: &binding}},
				Body:      []ir.Statement{{Kind: ir.StmtReturn{}}},
			},
		}},
	}
}
