package codegen

import (
	"strings"
	"testing"
)

// TestWrappedHelpersInExpressionOrder: the helpers a function needs first
// are written before it in the order of the first expression that needs
// each, whatever their kind, as Rust naga's write_wrapped_functions walks
// func_ctx.expressions in handle order. naga wrote them grouped by kind
// (float-to-int casts, negations, div/mod, abs, dot), so a negation after a
// division came first. A use in a later function does not move a helper
// the earlier function already needed.
func TestWrappedHelpersInExpressionOrder(t *testing.T) {
	code := compileWGSLWithOpts(t, `
@group(0) @binding(0) var<storage, read_write> o: array<i32, 4>;
fn first(a: i32, b: i32) -> i32 {
    let d = a / b;
    let n = -d;
    let m = a % b;
    return abs(n + m);
}
@compute @workgroup_size(1) fn main() {
    o[0] = first(o[1], o[2]);
    o[1] = -o[3];
}
`, DefaultOptions())
	var order []string
	for _, line := range strings.Split(code, "\n") {
		for _, h := range []string{"naga_div(int lhs", "naga_neg(int val", "naga_mod(int lhs", "naga_abs(int val"} {
			if strings.HasPrefix(line, "int "+strings.SplitN(h, "(", 2)[0]) && strings.Contains(line, h) {
				order = append(order, strings.SplitN(h, "(", 2)[0])
			}
		}
	}
	want := []string{"naga_div", "naga_neg", "naga_mod", "naga_abs"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("helpers in order %v, want %v (Rust's expression order); output:\n%s", order, want, code)
	}
}
