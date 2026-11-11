// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl_test

import "testing"

// TestE2E_IntegerDivisionGuard pins the safe-divisor form of naga_div that the
// writer emits for run-time integer division.
//
// WGSL §arithmetic-expr defines two cases the hardware instruction leaves
// undefined: e1 / 0 is e1, and the most negative value / -1 is that value.
// The divisor selects 1 in those cases, so lhs / 1 returns the dividend.
// Rust naga writes the same expression (back/hlsl/help.rs, DIV_FUNCTION).
func TestE2E_IntegerDivisionGuard(t *testing.T) {
	source := `
@group(0) @binding(0) var<storage, read_write> buf: array<u32>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let a = i32(buf[0]);
    let b = i32(buf[1]);
    buf[4] = u32(a / b);
    buf[5] = buf[2] / buf[3];
}
`
	code := compileWGSLToHLSL(t, source)

	// The int minimum is spelled int(-2147483647 - 1): a bare -2147483648
	// literal would not fit the HLSL int type (same as Rust's write_literal).
	const intGuard = `int naga_div(int lhs, int rhs) {
    return lhs / (((lhs == int(-2147483647 - 1) & rhs == -1) | (rhs == 0)) ? 1 : rhs);
}`
	const uintGuard = `uint naga_div(uint lhs, uint rhs) {
    return lhs / (rhs == 0u ? 1u : rhs);
}`
	assertContains(t, code, intGuard)
	assertContains(t, code, uintGuard)

	// A guard that returns 0 for a zero divisor is the bug this test guards
	// against: WGSL's result is the dividend, never 0.
	assertNotContains(t, code, "? a / b : 0")
	assertNotContains(t, code, "? a / b : 0u")
}
