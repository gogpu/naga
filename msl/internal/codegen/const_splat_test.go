// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestConstSplatF16: a module-scope f16 splat has a half vector type
// (BUG-F16-SILENT-BACKENDS-001). The type came from the literal's kind, with
// no f16 case, so `constant metal::half3 h = metal::float3(1.5h);` gave the
// constant a float vector initializer.
func TestConstSplatF16(t *testing.T) {
	got := compileWGSLWithOpts(t, `
enable f16;
const h = vec3<f16>(1.5h);
@group(0) @binding(0) var<storage, read_write> o: f32;
@compute @workgroup_size(1) fn main() { o = f32(h.x); }`, DefaultOptions())
	mustContainMSL(t, got, "constant metal::half3 h = metal::half3(1.5h);")
	if runtime.GOOS == "darwin" {
		verifyMSLWithXcrun(t, got)
	}
}

// constSplatModule is a module whose constant v splats the constant c of
// scalar type sc: a Splat whose value is not a literal, which another
// frontend makes and the WGSL one folds away.
func constSplatModule(sc ir.ScalarType) *ir.Module {
	return &ir.Module{
		Types: []ir.Type{
			{Inner: sc},
			{Inner: ir.VectorType{Size: ir.Vec3, Scalar: sc}},
		},
		Constants: []ir.Constant{
			{Name: "c", Type: 0, Init: 0},
			{Name: "v", Type: 1, Init: 2},
		},
		GlobalExpressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralI32(7)}},
			{Kind: ir.ExprConstant{Constant: 0}},
			{Kind: ir.ExprSplat{Size: ir.Vec3, Value: 1}},
		},
	}
}

// TestConstSplatOfConstant: the type of a constant Splat is its value's
// type, whatever expression the value is (Rust naga's
// put_possibly_const_expression takes it from the const expression types).
// It was float for any value but a literal.
func TestConstSplatOfConstant(t *testing.T) {
	sc := ir.ScalarType{Kind: ir.ScalarSint, Width: 4}
	code, _, err := Compile(constSplatModule(sc), DefaultOptions())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	mustContainMSL(t, code, "constant metal::int3 v = metal::int3(c);")
}

// TestConstExpressionUnsupportedKind: an expression kind that cannot be a
// constant is an error, as Rust's Error::Override, instead of an empty
// initializer `{}`.
func TestConstExpressionUnsupportedKind(t *testing.T) {
	m := constSplatModule(ir.ScalarType{Kind: ir.ScalarSint, Width: 4})
	m.GlobalExpressions[2] = ir.Expression{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 1, Right: 1}}
	m.Types[1] = ir.Type{Inner: m.Types[0].Inner}
	_, _, err := Compile(m, DefaultOptions())
	if !errors.Is(err, ir.ErrUnresolvedOverride) {
		t.Errorf("err = %v, want ir.ErrUnresolvedOverride", err)
	}
	if err != nil && strings.Count(err.Error(), "msl:") != 1 {
		t.Errorf("err = %q, want one msl: prefix", err)
	}
}
