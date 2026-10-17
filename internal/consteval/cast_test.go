package consteval

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/gogpu/naga/ir"
)

var (
	tBool = ir.ScalarType{Kind: ir.ScalarBool, Width: 1}
	tI32  = ir.ScalarType{Kind: ir.ScalarSint, Width: 4}
	tU32  = ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	tI64  = ir.ScalarType{Kind: ir.ScalarSint, Width: 8}
	tU64  = ir.ScalarType{Kind: ir.ScalarUint, Width: 8}
	tF16  = ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}
	tF32  = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	tF64  = ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}
	tAInt = ir.ScalarType{Kind: ir.ScalarAbstractInt, Width: 8}
	tAF   = ir.ScalarType{Kind: ir.ScalarAbstractFloat, Width: 8}
)

type aint = ir.LiteralAbstractInt
type afloat = ir.LiteralAbstractFloat

// Expected values are Rust naga v30 results (naga-cli on `const X = T(e);`),
// except where a comment says the WGSL specification overrides Rust.
func TestCastScalar(t *testing.T) {
	tests := []struct {
		name string
		in   ir.LiteralValue
		to   ir.ScalarType
		want ir.LiteralValue
	}{
		// AbstractInt -> concrete, in range.
		{"aint i32 max", aint(2147483647), tI32, ir.LiteralI32(2147483647)},
		{"aint i32 min", aint(-2147483648), tI32, ir.LiteralI32(-2147483648)},
		{"aint u32 max", aint(4294967295), tU32, ir.LiteralU32(4294967295)},
		{"aint i64", aint(math.MaxInt64), tI64, ir.LiteralI64(math.MaxInt64)},
		{"aint u64", aint(math.MaxInt64), tU64, ir.LiteralU64(math.MaxInt64)},
		{"aint f32 rounds to even", aint(16777217), tF32, ir.LiteralF32(16777216)},
		{"aint f32 rounds up", aint(16777219), tF32, ir.LiteralF32(16777220)},
		{"aint f16 rounds to even", aint(2049), tF16, ir.LiteralF16(2048)},
		{"aint f16 max", aint(65504), tF16, ir.LiteralF16(65504)},
		{"aint f64", aint(1 << 53), tF64, ir.LiteralF64(1 << 53)},
		{"aint abstract float", aint(-3), tAF, afloat(-3)},
		{"aint bool", aint(2), tBool, ir.LiteralBool(true)},
		// AbstractFloat -> concrete.
		{"afloat f32", afloat(0.1), tF32, ir.LiteralF32(0.1)},
		{"afloat f32 max", afloat(3.4028234663852886e38), tF32, ir.LiteralF32(math.MaxFloat32)},
		{"afloat f16", afloat(0.1), tF16, ir.LiteralF16(0.0999755859375)},
		{"afloat f16 rounds below max", afloat(65519.99), tF16, ir.LiteralF16(65504)},
		{"afloat i32 truncates", afloat(2.9), tI32, ir.LiteralI32(2)},
		{"afloat i32 truncates negative", afloat(-2.9), tI32, ir.LiteralI32(-2)},
		{"afloat i32 saturates high", afloat(1e10), tI32, ir.LiteralI32(math.MaxInt32)},
		{"afloat i32 saturates low", afloat(-1e10), tI32, ir.LiteralI32(math.MinInt32)},
		{"afloat i32 near max", afloat(2147483647.9), tI32, ir.LiteralI32(math.MaxInt32)},
		{"afloat u32 negative", afloat(-1.5), tU32, ir.LiteralU32(0)},
		{"afloat u32 near max", afloat(4294967295.5), tU32, ir.LiteralU32(math.MaxUint32)},
		{"afloat u32 saturates", afloat(1e10), tU32, ir.LiteralU32(math.MaxUint32)},
		{"afloat bool zero", afloat(0), tBool, ir.LiteralBool(false)},
		{"afloat bool negative zero", afloat(math.Copysign(0, -1)), tBool, ir.LiteralBool(false)},
		// Concrete -> concrete.
		{"u32 to i32 reinterprets", ir.LiteralU32(4294967295), tI32, ir.LiteralI32(-1)},
		{"i32 to u32 reinterprets", ir.LiteralI32(-1), tU32, ir.LiteralU32(4294967295)},
		{"f32 to i32 truncates", ir.LiteralF32(2.9), tI32, ir.LiteralI32(2)},
		{"f32 to i32 truncates negative", ir.LiteralF32(-2.9), tI32, ir.LiteralI32(-2)},
		// Spec: "the value in T closest to truncate(X) and also exactly
		// representable in the original floating point type".
		{"f32 to i32 saturates to largest f32 in range", ir.LiteralF32(1e10), tI32, ir.LiteralI32(2147483520)},
		{"f32 to i32 saturates low", ir.LiteralF32(-1e10), tI32, ir.LiteralI32(math.MinInt32)},
		{"f32 to u32 negative", ir.LiteralF32(-1.5), tU32, ir.LiteralU32(0)},
		{"f32 to u32 saturates to largest f32 in range", ir.LiteralF32(1e10), tU32, ir.LiteralU32(4294967040)},
		{"f32 to u32 1e20 (spec example)", ir.LiteralF32(1e20), tU32, ir.LiteralU32(4294967040)},
		{"f32 to i64 saturates", ir.LiteralF32(1e30), tI64, ir.LiteralI64(9223371487098961920)},
		{"f64 to i64 saturates", ir.LiteralF64(1e30), tI64, ir.LiteralI64(9223372036854774784)},
		{"f32 to u64 saturates", ir.LiteralF32(1e30), tU64, ir.LiteralU64(18446742974197923840)},
		{"f64 to u64 saturates", ir.LiteralF64(1e30), tU64, ir.LiteralU64(18446744073709549568)},
		{"i32 to f32 rounds", ir.LiteralI32(16777217), tF32, ir.LiteralF32(16777216)},
		{"u32 to f32 rounds", ir.LiteralU32(4294967295), tF32, ir.LiteralF32(4294967296)},
		{"f32 to f16", ir.LiteralF32(65504), tF16, ir.LiteralF16(65504)},
		{"f32 to f16 underflows to zero", ir.LiteralF32(1e-8), tF16, ir.LiteralF16(0)},
		{"f16 to f32", ir.LiteralF16(0.0999755859375), tF32, ir.LiteralF32(0.0999755859375)},
		{"f16 to i32", ir.LiteralF16(65504), tI32, ir.LiteralI32(65504)},
		{"bool to u32", ir.LiteralBool(true), tU32, ir.LiteralU32(1)},
		{"bool to f32", ir.LiteralBool(false), tF32, ir.LiteralF32(0)},
		{"u32 to bool", ir.LiteralU32(0), tBool, ir.LiteralBool(false)},
		{"i64 to u64 reinterprets", ir.LiteralI64(-1), tU64, ir.LiteralU64(math.MaxUint64)},
		{"same type", ir.LiteralF32(1.5), tF32, ir.LiteralF32(1.5)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Cast(Scalar(tt.in), tt.to)
			if err != nil {
				t.Fatalf("Cast(%v %T, %+v): %v", tt.in, tt.in, tt.to, err)
			}
			if !reflect.DeepEqual(got.Component(0), tt.want) {
				t.Errorf("Cast(%v %T) = %v %T, want %v %T", tt.in, tt.in, got.Component(0), got.Component(0), tt.want, tt.want)
			}
		})
	}
}

func TestCastErrors(t *testing.T) {
	tests := []struct {
		name string
		in   ir.LiteralValue
		to   ir.ScalarType
		msg  string // empty: only the kind is checked
	}{
		{"aint above i32", aint(2147483648), tI32, "the concrete type `i32` cannot represent the abstract value `2147483648` accurately"},
		{"aint below i32", aint(-2147483649), tI32, "the concrete type `i32` cannot represent the abstract value `-2147483649` accurately"},
		{"aint above u32", aint(4294967296), tU32, "the concrete type `u32` cannot represent the abstract value `4294967296` accurately"},
		{"aint negative u32", aint(-1), tU32, "the concrete type `u32` cannot represent the abstract value `-1` accurately"},
		{"aint negative u64", aint(-1), tU64, "the concrete type `u64` cannot represent the abstract value `-1` accurately"},
		{"afloat above f32", afloat(3.4028236e38), tF32, "the concrete type `f32` cannot represent the abstract value `3.4028236e38` accurately"},
		{"afloat 1e39", afloat(1e39), tF32, "the concrete type `f32` cannot represent the abstract value `1e39` accurately"},
		{"afloat above f16", afloat(65520), tF16, "the concrete type `f16` cannot represent the abstract value `65520.0` accurately"},
		// Rust v30 yields the literal F16(inf) for these (quirk 6); the
		// spec makes a non-finite const-expression a shader-creation error.
		{"aint above f16 (spec over Rust)", aint(65520), tF16, "the concrete type `f16` cannot represent the abstract value `65520` accurately"},
		{"f32 above f16 (spec over Rust)", ir.LiteralF32(65520), tF16, ""},
		{"f64 above f32", ir.LiteralF64(1e39), tF32, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Cast(Scalar(tt.in), tt.to)
			if err == nil {
				t.Fatalf("Cast(%v %T, %+v) = %v, want an error", tt.in, tt.in, tt.to, got.Component(0))
			}
			if !errors.Is(err, ErrNotRepresentable) {
				t.Errorf("error kind: %v", err)
			}
			if tt.msg != "" && err.Error() != tt.msg {
				t.Errorf("error = %q\nwant    %q", err.Error(), tt.msg)
			}
		})
	}
}

func TestCastVector(t *testing.T) {
	// Rust v30: vec3<u32>(vec3(-1.5f, 2.5f, 1e20f)) = (0, 2, 4294967040).
	got, err := Cast(Vector(ir.LiteralF32(-1.5), ir.LiteralF32(2.5), ir.LiteralF32(1e20)), tU32)
	if err != nil {
		t.Fatal(err)
	}
	want := []ir.LiteralValue{ir.LiteralU32(0), ir.LiteralU32(2), ir.LiteralU32(4294967040)}
	if !reflect.DeepEqual(got.Components(), want) || got.IsScalar() {
		t.Errorf("Cast = %v, want vector %v", got.Components(), want)
	}
	// One bad component fails the whole vector.
	if _, err := Cast(Vector(aint(1), aint(-1)), tU32); !errors.Is(err, ErrNotRepresentable) {
		t.Errorf("vector with an unrepresentable component: %v", err)
	}
}

func TestCastRejectsAbstractTargetsFromConcrete(t *testing.T) {
	for _, tt := range []struct {
		in ir.LiteralValue
		to ir.ScalarType
	}{
		{ir.LiteralF32(1), tAF},
		{ir.LiteralI32(1), tAInt},
		{afloat(1), tAInt},
	} {
		if _, err := Cast(Scalar(tt.in), tt.to); !errors.Is(err, ErrInvalidArgs) {
			t.Errorf("Cast(%T -> %+v): %v, want InvalidArgs", tt.in, tt.to, err)
		}
	}
}

// Rust's {:?} for f64, as naga prints values in messages.
func TestRustFloat(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{0, "0.0"},
		{math.Copysign(0, -1), "-0.0"},
		{65520, "65520.0"},
		{123.25, "123.25"},
		{1e-4, "0.0001"},
		{9.9e-5, "9.9e-5"},
		{1e16, "1e16"},
		{9999999999999998, "9999999999999998.0"},
		{1e39, "1e39"},
		{-3.4028236e38, "-3.4028236e38"},
		{1.5e-300, "1.5e-300"},
	}
	for _, tt := range tests {
		if got := rustFloat(tt.in); got != tt.want {
			t.Errorf("rustFloat(%g) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
