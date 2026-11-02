package codegen

import (
	"testing"

	"github.com/gogpu/naga/backend"
)

// TestReadZeroSkipWriteTernaryIsParenthesized (P0-MSL-104): a ReadZeroSkipWrite access is
// written as `cond ? access : DefaultConstructible()`, and a ternary binds
// looser than every binary operator. Rust naga's put_bounds_checks opens "("
// when the access is written with is_scoped=false (binary operands, a select
// condition), so the expected lines below are Rust naga 14328f49 output with
// both policies ReadZeroSkipWrite. naga v0.21.1 printed
// `t = _e16 + uint(_e17) < 4 ? v.inner[_e17] : DefaultConstructible();`, which
// C++ reads as `(t + uint(i)) < 4 ? … : …`: a silently wrong value.
func TestReadZeroSkipWriteTernaryIsParenthesized(t *testing.T) {
	src := `
@group(0) @binding(0) var<storage, read_write> b: array<u32>;
@group(0) @binding(1) var<storage, read_write> o: array<f32, 8>;
@compute @workgroup_size(1) fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let v = array<u32, 4>(1u, 2u, 3u, 4u);
    let fv = array<f32, 4>(1.0, 2.0, 3.0, 4.0);
    let bl = array<bool, 2>(true, false);
    var t = 0u;
    t = t + v[gid.x];
    t = v[gid.y] * 2u + b[gid.z];
    o[1] = select(fv[gid.y], fv[gid.z], bl[gid.x]);
    o[6] = f32(t); }
`
	opts := DefaultOptions()
	opts.BoundsCheckPolicies.Index = backend.BoundsCheckReadZeroSkipWrite
	opts.BoundsCheckPolicies.Buffer = backend.BoundsCheckReadZeroSkipWrite
	code := compileWGSLWithOpts(t, src, opts)
	for _, want := range []string{
		"t = _e16 + (uint(_e17) < 4 ? v.inner[_e17] : DefaultConstructible());",
		"t = ((uint(_e20) < 4 ? v.inner[_e20] : DefaultConstructible()) * 2u) + _e27;",
		// Rust also parenthesizes the accept and reject operands
		// (`? (uint(_e33) < 4 ? …) : (uint(_e31) < 4 ? …)`); a ternary there
		// parses the same either way (?: is right-associative), so only the
		// condition is pinned here (text parity: P2-MSL-105).
		"o.inner[1] = (uint(_e35) < 2 ? bl.inner[_e35] : DefaultConstructible()) ? ",
	} {
		mustContainMSL(t, code, want)
	}
}
