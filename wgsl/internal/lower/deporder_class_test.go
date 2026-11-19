package lower

import "testing"

// The dependency-order class of issue #261 beyond bitcast (card P2-WGSL-120,
// expert validation): every expression position that can name a module-scope
// declaration must accept one declared later in the source. WGSL declarations
// are in scope across the entire program (spec §"Declaration and Scope"), so
// rejecting these is a shader-creation error the spec does not have.
func TestLowerForwardRefsClass(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"const_assert module scope", `const_assert(C > 0);
const C: i32 = 1;`},
		{"const_assert function scope", `fn f() {
    const_assert(C > 0);
}
const C: i32 = 1;`},
		{"binding array element", `enable wgpu_binding_array;
@group(0) @binding(0) var a: binding_array<Tex, 2>;
alias Tex = texture_2d<f32>;`},
		{"binding array size", `enable wgpu_binding_array;
@group(0) @binding(0) var a: binding_array<texture_2d<f32>, N>;
const N: u32 = 2;`},
		{"workgroup_size argument", `@compute @workgroup_size(N) fn cs() {}
const N: u32 = 8;`},
		{"workgroup_size two arguments", `@compute @workgroup_size(N, M) fn cs() {}
const N: u32 = 8;
const M: u32 = 1;`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mustCompile(t, tc.src)
		})
	}
}
