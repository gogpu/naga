// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"errors"
	"testing"

	"github.com/gogpu/naga/internal/typifier"
	"github.com/gogpu/naga/ir"
)

// constWriter returns a writer for module with the module's types, as
// Compile sets it up before writing constants.
func constWriter(t *testing.T, module *ir.Module) *Writer {
	t.Helper()
	types, err := typifier.ForModule(module)
	if err != nil {
		t.Fatalf("typifier: %v", err)
	}
	w := newWriter(module, &Options{LangVersion: Version430})
	w.typifiers = types
	return w
}

// splatModule is a module whose constant v splats global expression value,
// with the constant c = 7i before it: a splat whose value is not a
// literal, which another frontend makes and the WGSL one folds away.
func splatModule(value ir.Expression, sc ir.ScalarType) *ir.Module {
	return &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Inner: sc},
			{Inner: ir.VectorType{Size: ir.Vec3, Scalar: sc}},
		},
		Constants: []ir.Constant{{Name: "c", Type: 0, Init: 0}},
		GlobalExpressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralI32(7)}},
			value,
			{Kind: ir.ExprSplat{Size: ir.Vec3, Value: 1}},
		},
	}
}

// TestConstSplatType: the vector type of a constant splat is its value's
// type, whatever expression the value is, as Rust naga's
// write_possibly_const_expression writes it. It was vec (float) for any
// value but an i32, u32 or bool literal: a splat of an i32 constant became
// vec3(7) and an f64 splat vec3(1.0LF).
func TestConstSplatType(t *testing.T) {
	tI32 := ir.ScalarType{Kind: ir.ScalarSint, Width: 4}
	tF64 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}
	for name, tc := range map[string]struct {
		module *ir.Module
		want   string
	}{
		"i32 constant": {splatModule(ir.Expression{Kind: ir.ExprConstant{Constant: 0}}, tI32), "ivec3(7)"},
		"f64 literal":  {splatModule(ir.Expression{Kind: ir.Literal{Value: ir.LiteralF64(1)}}, tF64), "dvec3(1.0LF)"},
	} {
		got, err := constWriter(t, tc.module).writeConstExpr(2)
		if err != nil || got != tc.want {
			t.Errorf("%s: writeConstExpr = %q, %v; want %q", name, got, err, tc.want)
		}
	}
}

// TestConstExprErrors: an expression that cannot be a constant is
// ir.ErrUnresolvedOverride, as Rust's Error::Override, and a handle out of
// range an error; both were written as the literal 0.
func TestConstExprErrors(t *testing.T) {
	tI32 := ir.ScalarType{Kind: ir.ScalarSint, Width: 4}
	m := splatModule(ir.Expression{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 0}}, tI32)
	w := constWriter(t, m)
	if _, err := w.writeConstExpr(1); !errors.Is(err, ir.ErrUnresolvedOverride) {
		t.Errorf("binary: err = %v, want ir.ErrUnresolvedOverride", err)
	}
	if _, err := w.writeConstExpr(9); err == nil {
		t.Errorf("handle out of range: no error")
	}
}
