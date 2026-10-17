package consteval

import (
	"errors"
	"reflect"
	"testing"

	"github.com/gogpu/naga/ir"
)

func TestRelational(t *testing.T) {
	tests := []struct {
		name string
		fun  ir.RelationalFunction
		v    Value
		want Value
	}{
		{"any of vector", ir.RelationalAny, bools(false, true), bools(true)},
		{"any all false", ir.RelationalAny, bools(false, false), bools(false)},
		{"all of vector", ir.RelationalAll, bools(true, false, true), bools(false)},
		{"all true", ir.RelationalAll, bools(true, true), bools(true)},
		{"any of scalar is itself", ir.RelationalAny, bools(true), bools(true)},
		// Rust v30 does not evaluate isNan/isInf; a constant is always
		// finite, so both are false.
		{"isNan of constant", ir.RelationalIsNan, f32v(1, 2), bools(false, false)},
		{"isInf of constant", ir.RelationalIsInf, af(1), bools(false)},
	}
	for _, tt := range tests {
		got, err := Relational(tt.fun, tt.v)
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s = %v, want %v", tt.name, got.Components(), tt.want.Components())
		}
	}
	if _, err := Relational(ir.RelationalAny, i32s(1)); !errors.Is(err, ErrInvalidOperand) ||
		err.Error() != "Cannot apply relational function to type" {
		t.Errorf("any of i32: %v", err)
	}
	if _, err := Relational(ir.RelationalIsNan, bools(true)); !errors.Is(err, ErrInvalidOperand) {
		t.Errorf("isNan of bool: %v", err)
	}
}

func TestRustEvaluatesRelational(t *testing.T) {
	for fun, want := range map[ir.RelationalFunction]bool{
		ir.RelationalAll: true, ir.RelationalAny: true, ir.RelationalIsNan: false, ir.RelationalIsInf: false,
	} {
		if got := RustEvaluatesRelational(fun); got != want {
			t.Errorf("RustEvaluatesRelational(%d) = %v", fun, got)
		}
	}
}

// select(reject, accept, condition), Rust v30 semantics.
func TestSelect(t *testing.T) {
	tests := []struct {
		name                string
		reject, accept, cnd Value
		want                Value
	}{
		{"scalar true", af(1), af(2), bools(true), af(2)},
		{"scalar false", af(1), af(2), bools(false), af(1)},
		{"vector by scalar", f32v(1, 2), f32v(3, 4), bools(true), f32v(3, 4)},
		{"vector by vector", f32v(1, 2), f32v(3, 4), bools(false, true), f32v(1, 4)},
		// Rust converts accept to reject's type.
		{"accept converted", f32s(1), af(2.5), bools(true), f32s(2.5)},
	}
	for _, tt := range tests {
		got, err := Select(tt.reject, tt.accept, tt.cnd)
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s = %v, want %v", tt.name, got.Components(), tt.want.Components())
		}
	}
	errs := []struct {
		name                string
		reject, accept, cnd Value
		msg                 string
	}{
		{"scalar non-bool condition", af(1), af(2), i32s(1),
			"Expected boolean expression for condition argument of `select`, got something else"},
		{"vector size mismatch", f32v(1, 2), f32v(1, 2, 3), bools(true),
			"Expected vectors of the same size for reject and accept args., got Bi and Tri"},
		{"condition size mismatch", f32v(1, 2), f32v(3, 4), bools(true, false, true),
			"Expected same number of vector components between condition, accept, and reject args., got something else"},
		{"vector non-bool condition", f32v(1, 2), f32v(3, 4), f32v(1, 0),
			"Expected boolean vector for condition arg., got something else"},
		{"scalar and vector", af(1), afv(1, 2), bools(true),
			"Expected reject and accept args. to be scalars of vectors of the same type, got something else"},
		{"accept not convertible even when unused", i32s(1), ai(10000000000), bools(false),
			"the concrete type `i32` cannot represent the abstract value `10000000000` accurately"},
	}
	for _, tt := range errs {
		if _, err := Select(tt.reject, tt.accept, tt.cnd); err == nil || err.Error() != tt.msg {
			t.Errorf("%s: %v\nwant %q", tt.name, err, tt.msg)
		}
	}
}

