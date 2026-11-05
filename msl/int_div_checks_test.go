// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package msl_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/wgsl"
)

// TestSkipIntDivChecks: Options.SkipIntDivChecks writes integer `/` and `%`
// as the plain operators and no naga_div / naga_mod helper, as Rust naga's
// emit_int_div_checks = false does (back/msl/writer.rs, the
// `context.emit_int_div_checks` guards in put_expression and
// write_wrapped_binary_op). Float `/` and `%` are not affected: they never
// had a helper.
func TestSkipIntDivChecks(t *testing.T) {
	const src = `
struct Data { a: array<i32, 2>, u: array<u32, 2>, v: array<vec2<i32>, 2>, f: array<f32, 2> }
@group(0) @binding(0) var<storage, read_write> d: Data;
@compute @workgroup_size(1) fn main() {
    d.a[0] = d.a[0] / d.a[1];
    d.a[1] = d.a[0] % d.a[1];
    d.u[0] = d.u[0] / d.u[1];
    d.u[1] = d.u[0] % d.u[1];
    d.v[0] = d.v[0] / d.v[1];
    d.f[0] = d.f[0] / d.f[1];
    d.u[1] = d.u[0] / d.u[1] * 2u;
}`
	module, _, err := wgsl.Parse(src, wgsl.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}

	def, _, err := msl.Compile(module, msl.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"int naga_div(int lhs, int rhs)", "uint naga_mod(uint lhs, uint rhs)", "naga_div(", "naga_mod("} {
		if !strings.Contains(def, want) {
			t.Errorf("default output lacks %q:\n%s", want, def)
		}
	}

	skip, _, err := msl.Compile(module, msl.Options{SkipIntDivChecks: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"naga_div", "naga_mod"} {
		if strings.Contains(skip, bad) {
			t.Errorf("SkipIntDivChecks output still has %q:\n%s", bad, skip)
		}
	}
	for _, want := range []string{
		`d\.a\.inner\[0\] = _e\d+ / _e\d+;`,
		`d\.a\.inner\[1\] = _e\d+ % _e\d+;`,
		`d\.u\.inner\[0\] = _e\d+ / _e\d+;`,
		`d\.u\.inner\[1\] = _e\d+ % _e\d+;`,
		`d\.v\.inner\[0\] = _e\d+ / _e\d+;`,
		`d\.f\.inner\[0\] = _e\d+ / _e\d+;`,
		// A nested operand is parenthesized, as Rust writes it.
		`d\.u\.inner\[1\] = \(_e\d+ / _e\d+\) \* 2u;`,
	} {
		if !regexp.MustCompile(want).MatchString(skip) {
			t.Errorf("SkipIntDivChecks output lacks %q:\n%s", want, skip)
		}
	}
}
