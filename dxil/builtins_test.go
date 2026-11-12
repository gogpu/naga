package dxil

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// builtinEntryPointModule wraps one built-in argument in the smallest module
// checkBuiltins accepts: a type table with the argument's type and a vertex
// entry point.
func builtinEntryPointModule(builtin ir.BuiltinValue) *ir.Module {
	binding := ir.Binding(ir.BuiltinBinding{Builtin: builtin})
	return &ir.Module{
		Types: []ir.Type{{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
		EntryPoints: []ir.EntryPoint{{
			Name:  "main",
			Stage: ir.StageVertex,
			Function: ir.Function{
				Name:      "vs",
				Arguments: []ir.FunctionArgument{{Name: "x", Type: 0, Binding: &binding}},
			},
		}},
	}
}

// TestCheckBuiltinsRefusesUnsupported pins the four SPIR-V-only built-ins the
// emitter cannot express: each is refused with a clear error naming it, and
// the signature builders leave it out rather than inventing a semantic.
func TestCheckBuiltinsRefusesUnsupported(t *testing.T) {
	tests := []struct {
		name    string
		builtin ir.BuiltinValue
		want    string
	}{
		{"base_instance", ir.BuiltinBaseInstance, "base_instance"},
		{"base_vertex", ir.BuiltinBaseVertex, "base_vertex"},
		{"point_coord", ir.BuiltinPointCoord, "point_coord"},
		{"workgroup_size", ir.BuiltinWorkGroupSize, "workgroup_size"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := builtinEntryPointModule(tt.builtin)
			err := checkBuiltins(module, &module.EntryPoints[0])
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("checkBuiltins(%s) = %v, want an error naming %q", tt.name, err, tt.want)
			}
		})
	}
}

// TestCheckBuiltinsAcceptsCullDistance: cull_distance is expressible in DXIL
// (SV_CullDistance), so the check must let it through.
func TestCheckBuiltinsAcceptsCullDistance(t *testing.T) {
	module := builtinEntryPointModule(ir.BuiltinCullDistance)
	if err := checkBuiltins(module, &module.EntryPoints[0]); err != nil {
		t.Fatalf("checkBuiltins refused cull_distance: %v", err)
	}
}
