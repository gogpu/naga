package emit

// The five ballot-mask built-ins (card P2-IR-041, ADR-014 D5). DXIL has no
// ballot-mask value, and the spec's mask spans every invocation of the wave
// while dx.op.ballot(true) holds only the active lanes, so the emitter refuses
// with the limitation named.

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestEmitSubgroupMaskBuiltinsRefused: every ballot-mask built-in is refused
// with the limitation named, never written as a wrong intrinsic.
func TestEmitSubgroupMaskBuiltinsRefused(t *testing.T) {
	for _, builtin := range []ir.BuiltinValue{
		ir.BuiltinSubgroupEqMask, ir.BuiltinSubgroupGeMask, ir.BuiltinSubgroupGtMask,
		ir.BuiltinSubgroupLeMask, ir.BuiltinSubgroupLtMask,
	} {
		irMod := dxilSubgroupMaskModule(builtin)
		_, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
		if err == nil {
			t.Fatalf("%s emitted, want a refusal", builtin)
		}
		if !strings.Contains(err.Error(), "ballot-mask") || !strings.Contains(err.Error(), builtin.String()) {
			t.Errorf("%s: error = %v, want the ballot-mask limitation naming the built-in", builtin, err)
		}
	}
}

// dxilSubgroupMaskModule builds a compute entry point whose only argument is
// one ballot-mask built-in of type vec4<u32>.
func dxilSubgroupMaskModule(builtin ir.BuiltinValue) *ir.Module {
	binding := ir.Binding(ir.BuiltinBinding{Builtin: builtin})
	return &ir.Module{
		Types: []ir.Type{
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Name: "vec4u32", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
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
