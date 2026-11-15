package codegen

import (
	"strings"
	"testing"
)

// Rust naga writes the operand of every unary operator with is_scoped=false
// (back/msl/writer.rs, Expression::Unary), so a binary operand carries its own
// parentheses inside the operator's: `~((a | i))`, `!((a != 1u))`.
//
// The scalar ternary Select takes the same path for all three of its operands
// (condition, accept and reject), so a binary accept or reject is
// parenthesized too: `(a < 3u) ? (a - i) : (a + i)`.
//
// The expected lines are Rust naga 29.0.0 output for this exact source, with
// the ReadZeroSkipWrite policies of DefaultBoundsCheckPolicies.
func TestUnaryAndSelectOperandParens(t *testing.T) {
	const src = `@group(0) @binding(0) var<storage, read_write> buf: array<u32>;

@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    let a = buf[0];
    buf[1] = ~(a | i);
    buf[4] = select(a + i, a - i, a < 3u);
    if (!(a != 1u)) {
        buf[3] = 1u;
    }
}
`
	code := compileWGSL(t, src)
	want := []string{
		"buf[1] = ~((a | i));",
		"buf[4] = (a < 3u) ? (a - i) : (a + i);",
		"if (!((a != 1u))) {",
	}
	for _, line := range want {
		if !containsLine(code, line) {
			t.Errorf("missing line (Rust naga output):\n  %s\ngot:\n%s",
				line, linesContaining(code, "buf[")+linesContaining(code, "if ("))
		}
	}
	if t.Failed() {
		t.Logf("full output:\n%s", code)
	}
}

// A unary operator whose operand is itself scoped (a cast, a call, a name)
// must not gain a second pair of parentheses: Rust writes
// `static_cast<uint>(naga_neg(as_type<int>(...)))`, not `naga_neg((...))`.
func TestUnaryOperandScopedStaysUnwrapped(t *testing.T) {
	const src = `@group(0) @binding(0) var<storage, read_write> buf: array<u32>;

@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    let a = buf[0];
    buf[2] = u32(-(i32(a) + i32(i)));
}
`
	code := compileWGSL(t, src)
	for _, line := range strings.Split(code, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "buf[2] = ") {
			continue
		}
		if strings.Contains(line, "naga_neg((") {
			t.Errorf("scoped unary operand got extra parentheses: %s", line)
		}
		return
	}
	t.Fatalf("buf[2] statement not found in output:\n%s", code)
}
