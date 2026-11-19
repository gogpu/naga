package lower

import "testing"

// TestLowerBitcastForwardRefs: an identifier inside bitcast<T>(...) naming a
// module-scope declaration that appears later in the source lowers — WGSL
// declarations are visible regardless of order, and every other expression
// already follows that rule (issue #261, card P2-WGSL-120). Before the fix
// the dependency-order pass skipped the bitcast's contents: a later var, const
// or texture was "unresolved identifier", and a later function call inside the
// bitcast was "``{unknown type}``".
func TestLowerBitcastForwardRefs(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"later var", `fn f() -> f32 {
    return bitcast<f32>(G);
}
var<private> G: u32 = 1u;`},
		{"later const", `fn f() -> f32 {
    return bitcast<f32>(C);
}
const C: u32 = 1u;`},
		{"later function", `fn f() -> f32 {
    return bitcast<f32>(g());
}
fn g() -> u32 { return 1u; }`},
		{"later texture", `fn f() -> f32 {
    return bitcast<f32>(pack4x8unorm(textureLoad(Tex, vec2<i32>(0, 0))));
}
var Tex: texture_2d<f32>;`},
		// The target type's own references are covered by the parser-level
		// test (deporder_bitcast_test.go): bitcast targets are concrete
		// scalars or vectors, so an end-to-end "later target type" needs a
		// type alias, which the lowerer does not accept in bitcast at all
		// (a separate, pre-existing limitation — card P2-WGSL-121).
		// A module `const A = bitcast<f32>(B)` orders correctly now (proven at
		// the parser level), but the constant evaluator does not support
		// bitcast in const expressions yet — a separate limitation, card
		// P2-WGSL-123.
		{"if condition", `fn f() -> i32 {
    if bitcast<i32>(G) > 0 { return 1; }
    return 0;
}
var<private> G: u32 = 1u;`},
		{"local let initializer", `fn f() -> f32 {
    let v = bitcast<f32>(G);
    return v;
}
var<private> G: u32 = 1u;`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mustCompile(t, tc.src)
		})
	}
}
