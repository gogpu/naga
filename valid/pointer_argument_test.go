package valid

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// pointerArgumentModule is a module with one function `f` whose only
// argument `p` has the pointer type ptr.
func pointerArgumentModule(ptr ir.TypeInner) *ir.Module {
	u32 := ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	return &ir.Module{
		Types: []ir.Type{
			{Inner: u32},
			{Inner: ptr},
		},
		Functions: []ir.Function{{
			Name:      "f",
			Arguments: []ir.FunctionArgument{{Name: "p", Type: 1}},
		}},
	}
}

// TestValidateFunctionArgumentPointerSpace checks the address space of a
// pointer argument of a user-defined function, as Rust naga's
// validate_function does (valid/function.rs, FunctionError::
// InvalidArgumentPointerSpace). WGSL allows only the function and private
// address spaces for such a parameter unless the implementation supports
// the unrestricted_pointer_parameters language extension (wgsl/index.bs,
// "Language extensions"); neither Rust naga nor naga supports it, so the
// check is unconditional, as in Rust.
func TestValidateFunctionArgumentPointerSpace(t *testing.T) {
	const wantFragment = "which can't be passed into functions"
	tests := []struct {
		name string
		ptr  ir.TypeInner
		want string // "" = the argument is valid
	}{
		{"function", ir.PointerType{Base: 0, Space: ir.SpaceFunction}, ""},
		{"private", ir.PointerType{Base: 0, Space: ir.SpacePrivate}, ""},
		{"storage", ir.PointerType{Base: 0, Space: ir.SpaceStorage},
			"Argument 'p' at index 0 is a pointer of space Storage { .. }, which can't be passed into functions."},
		{"uniform", ir.PointerType{Base: 0, Space: ir.SpaceUniform},
			"Argument 'p' at index 0 is a pointer of space Uniform, which can't be passed into functions."},
		{"workgroup", ir.PointerType{Base: 0, Space: ir.SpaceWorkGroup},
			"Argument 'p' at index 0 is a pointer of space WorkGroup, which can't be passed into functions."},
		{"immediate", ir.PointerType{Base: 0, Space: ir.SpaceImmediate},
			"Argument 'p' at index 0 is a pointer of space Immediate, which can't be passed into functions."},
		{"value pointer storage", ir.ValuePointerType{Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, Space: ir.SpaceStorage},
			"Argument 'p' at index 0 is a pointer of space Storage { .. }, which can't be passed into functions."},
		{"value pointer private", ir.ValuePointerType{Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, Space: ir.SpacePrivate}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs, err := validateErrs(pointerArgumentModule(tt.ptr), CapAll)
			if err != nil {
				t.Fatalf("Validate returned error: %v", err)
			}
			if tt.want == "" {
				for _, e := range errs {
					if strings.Contains(e.Error(), wantFragment) {
						t.Fatalf("unexpected argument error: %v", e)
					}
				}
				return
			}
			if !containsError(errs, tt.want) {
				t.Fatalf("want error %q, got %v", tt.want, errs)
			}
		})
	}
}
