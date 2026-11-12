package valid

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// cullDistanceModule is a vertex shader writing @builtin(cull_distance). WGSL
// cannot spell the built-in, so the module is built by hand, the way the
// SPIR-V frontend will build it from a SPIR-V BuiltIn decoration.
func cullDistanceModule() *ir.Module {
	one := uint32(1)
	pos := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
	cull := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinCullDistance})
	return &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "array<f32, 1>", Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &one}, Stride: 4}},
			{Name: "vec4<f32>", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "Out", Inner: ir.StructType{Members: []ir.StructMember{
				{Name: "position", Type: 2, Offset: 0, Binding: &pos},
				{Name: "cull", Type: 1, Offset: 16, Binding: &cull},
			}, Span: 20}},
		},
		EntryPoints: []ir.EntryPoint{{
			Name:  "main",
			Stage: ir.StageVertex,
			Function: ir.Function{
				Name:   "vs",
				Result: &ir.FunctionResult{Type: 3},
			},
		}},
	}
}

// TestCullDistanceCapability is the capability gate for the SPIR-V frontend's
// cull_distance: rejected without CapCullDistances, accepted with it, and
// reported by MissingCapabilities so a backend refuses exactly what Validate
// rejects (Rust interface.rs:263 BuiltIn::CullDistance).
func TestCullDistanceCapability(t *testing.T) {
	module := cullDistanceModule()

	if got := MissingCapabilities(module, 0); got != CapCullDistances {
		t.Errorf("MissingCapabilities = %v, want CapCullDistances", got)
	}

	errors, err := validateErrs(module, CapCullDistances)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if len(errors) > 0 {
		t.Errorf("expected no errors with CapCullDistances, got: %v", errors)
	}

	errors, err = validateErrs(module, 0)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if len(errors) == 0 || !strings.Contains(errors[0].Error(), "CULL_DISTANCE") {
		t.Errorf("expected a CULL_DISTANCE capability error, got: %v", errors)
	}
}
