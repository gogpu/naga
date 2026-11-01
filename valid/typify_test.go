package valid

import (
	"errors"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestTypeResolutionFailed: a function whose expressions cannot be typed is
// invalid, reported as Rust reports it (FunctionError::Expression with
// ExpressionError::Type: "Type resolution failed"), on the expression.
func TestTypeResolutionFailed(t *testing.T) {
	m := &ir.Module{
		Types: []ir.Type{{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		Functions: []ir.Function{{
			Name: "f",
			Expressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralF32(1)}},
				{Kind: ir.ExprSwizzle{Size: ir.Vec2, Vector: 0, Pattern: [4]ir.SwizzleComponent{0, 0, 0, 0}}}, // a swizzle of a scalar
			},
			Body: ir.Block{{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 2}}}},
		}},
	}
	_, err := Validate(m, Options{})
	var errs Errors
	if !errors.As(err, &errs) || len(errs) != 1 {
		t.Fatalf("err = %v, want one validation error", err)
	}
	e := errs[0]
	if e.Function != "f" || e.Expression == nil || *e.Expression != 1 ||
		!strings.HasPrefix(e.Message, "Type resolution failed: ") || !strings.Contains(e.Message, "swizzle base must be vector") {
		t.Errorf("error %+v (%q), want the type resolution failure of expression 1 in f", e, e.Error())
	}
}

// TestTypedArrayLength: arrayLength reads the pointer's type from the
// typifier: a pointer to a sized array is refused, a runtime-sized one is not.
func TestTypedArrayLength(t *testing.T) {
	four := uint32(4)
	for _, tc := range []struct {
		name    string
		size    ir.ArraySize
		wantErr bool
	}{
		{"sized", ir.ArraySize{Constant: &four}, true},
		{"runtime-sized", ir.ArraySize{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &ir.Module{
				Types: []ir.Type{
					{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
					{Inner: ir.ArrayType{Base: 0, Size: tc.size, Stride: 4}},
				},
				GlobalVariables: []ir.GlobalVariable{{Name: "a", Space: ir.SpaceStorage, Access: ir.StorageAccessLoad,
					Binding: &ir.ResourceBinding{}, Type: 1}},
				Functions: []ir.Function{{
					Name: "f",
					Expressions: []ir.Expression{
						{Kind: ir.ExprGlobalVariable{Variable: 0}},
						{Kind: ir.ExprArrayLength{Array: 0}},
					},
					Body: ir.Block{{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 2}}}},
				}},
			}
			errs, err := validateErrs(m, 0)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, e := range errs {
				found = found || strings.Contains(e.Message, "can't be done")
			}
			if found != tc.wantErr {
				t.Errorf("errors %v, want the array length error: %v", errs, tc.wantErr)
			}
		})
	}
}
