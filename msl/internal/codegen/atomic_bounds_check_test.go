package codegen

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
)

// TestReadZeroSkipWriteAtomicRuntimeSizedArray pins the ReadZeroSkipWrite check
// of an atomic to Rust naga's put_bounds_checks, which serves atomics with the
// same bounds_check_iter walk as loads and stores: the condition wraps the whole
// call, checks run from the pointer outward, and the size field is named after
// the global's handle. Every expected line is Rust naga 14328f49 output with
// --index-bounds-check-policy and --buffer-bounds-check-policy
// ReadZeroSkipWrite (element size equals stride in every case, so the WGSL-spec
// bound we use prints the same text).
//
// P0-MSL-102: the atomic path had its own copy of the bound logic. It named the
// size field by its position among the runtime-sized globals (another buffer's
// size, or a field the struct does not declare), skipped the check of a
// runtime-sized array reached through a struct element (`e[i].c[j]`), and for a
// top-level array or a compare-exchange wrapped the operand, taking the address
// of a conditional.
func TestReadZeroSkipWriteAtomicRuntimeSizedArray(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "top-level runtime array",
			src: `
@group(0) @binding(0) var<storage, read_write> a: array<atomic<u32>>;
@compute @workgroup_size(1) fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let o = atomicAdd(&a[gid.x], 1u); atomicStore(&a[gid.y], o); let x = atomicExchange(&a[3], 2u); }
`,
			want: []string{
				"uint _e5 = uint(_e2) < 1 + (_buffer_sizes.size0 - 0 - 4) / 4 ? metal::atomic_fetch_add_explicit(&a[_e2], 1u, metal::memory_order_relaxed) : DefaultConstructible();",
				"if (uint(_e7) < 1 + (_buffer_sizes.size0 - 0 - 4) / 4) {",
				"metal::atomic_store_explicit(&a[_e7], _e5, metal::memory_order_relaxed);",
				"uint _e12 = uint(3) < 1 + (_buffer_sizes.size0 - 0 - 4) / 4 ? metal::atomic_exchange_explicit(&a[3], 2u, metal::memory_order_relaxed) : DefaultConstructible();",
			},
		},
		{
			name: "two runtime-sized globals",
			src: `
@group(0) @binding(0) var<storage, read_write> f: array<u32, 4>;
struct X { n: u32, d: array<u32> }
@group(0) @binding(1) var<storage, read_write> x: X;
struct Y { n: u32, d: array<atomic<u32>> }
@group(0) @binding(2) var<storage, read_write> y: Y;
@compute @workgroup_size(1) fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let o = atomicAdd(&y.d[gid.y], 1u); x.d[gid.x] = o; f[gid.z] = o; }
`,
			want: []string{
				"uint _e6 = uint(_e3) < 1 + (_buffer_sizes.size2 - 4 - 4) / 4 ? metal::atomic_fetch_add_explicit(&y.d[_e3], 1u, metal::memory_order_relaxed) : DefaultConstructible();",
				"if (uint(_e9) < 1 + (_buffer_sizes.size1 - 4 - 4) / 4) {",
				"if (uint(_e12) < 4) {",
			},
		},
		{
			name: "sized global first",
			src: `
@group(0) @binding(0) var<storage, read_write> f: array<u32, 4>;
struct Y { n: u32, d: array<atomic<u32>> }
@group(0) @binding(1) var<storage, read_write> y: Y;
@compute @workgroup_size(1) fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let o = atomicMax(&y.d[gid.y], 7u); f[0] = o; }
`,
			want: []string{
				"uint size1;",
				"uint _e6 = uint(_e3) < 1 + (_buffer_sizes.size1 - 4 - 4) / 4 ? metal::atomic_fetch_max_explicit(&y.d[_e3], 7u, metal::memory_order_relaxed) : DefaultConstructible();",
			},
		},
		{
			name: "runtime array of structs and compare-exchange",
			src: `
struct E { k: u32, c: array<atomic<i32>, 4> }
@group(0) @binding(0) var<storage, read_write> e: array<E>;
@compute @workgroup_size(1) fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let o = atomicSub(&e[gid.x].c[gid.y], 1i);
    let r = atomicCompareExchangeWeak(&e[gid.y].c[gid.x], 0i, o); }
`,
			want: []string{
				"int _e8 = uint(_e5) < 4 && uint(_e2) < 1 + (_buffer_sizes.size0 - 0 - 20) / 20 ? metal::atomic_fetch_sub_explicit(&e[_e2].c.inner[_e5], 1, metal::memory_order_relaxed) : DefaultConstructible();",
				"_atomic_compare_exchange_result_Sint_4_ _e16 = uint(_e13) < 4 && uint(_e10) < 1 + (_buffer_sizes.size0 - 0 - 20) / 20 ? naga_atomic_compare_exchange_weak_explicit(&e[_e10].c.inner[_e13], 0, _e8) : DefaultConstructible();",
			},
		},
	}
	tests = append(tests, []struct {
		name string
		src  string
		want []string
	}{
		{
			// Only the pointer chain is written unchecked inside the condition;
			// the value and compare operands keep their own checks, as Rust's
			// put_atomic_operation writes them with the normal policy.
			name: "operands keep their checks",
			src: `
@group(0) @binding(0) var<storage, read_write> a: array<atomic<u32>>;
@group(0) @binding(1) var<storage, read_write> o: array<u32, 4>;
@compute @workgroup_size(1) fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let v = array<u32, 4>(1u, 2u, 3u, 4u);
    let w = vec4<u32>(gid, 7u);
    atomicAdd(&a[gid.x], v[gid.y]);
    atomicAdd(&a[gid.z], w[gid.x]);
    let r = atomicCompareExchangeWeak(&a[gid.y], v[gid.z], w[gid.y]);
    var lv = array<u32, 4>(5u, 6u, 7u, 8u);
    atomicStore(&a[gid.x], lv[gid.z]);
    o[0] = r.old_value; }
`,
			want: []string{
				"uint _e13 = uint(_e9) < 1 + (_buffer_sizes.size0 - 0 - 4) / 4 ? metal::atomic_fetch_add_explicit(&a[_e9], uint(_e11) < 4 ? v.inner[_e11] : DefaultConstructible(), metal::memory_order_relaxed) : DefaultConstructible();",
				"uint _e19 = uint(_e15) < 1 + (_buffer_sizes.size0 - 0 - 4) / 4 ? metal::atomic_fetch_add_explicit(&a[_e15], uint(_e17) < 4 ? w[_e17] : DefaultConstructible(), metal::memory_order_relaxed) : DefaultConstructible();",
				"_atomic_compare_exchange_result_Uint_4_ _e27 = uint(_e21) < 1 + (_buffer_sizes.size0 - 0 - 4) / 4 ? naga_atomic_compare_exchange_weak_explicit(&a[_e21], uint(_e23) < 4 ? v.inner[_e23] : DefaultConstructible(), uint(_e25) < 4 ? w[_e25] : DefaultConstructible()) : DefaultConstructible();",
			},
		},
	}...)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DefaultOptions()
			opts.BoundsCheckPolicies.Index = backend.BoundsCheckReadZeroSkipWrite
			opts.BoundsCheckPolicies.Buffer = backend.BoundsCheckReadZeroSkipWrite
			code := compileWGSLWithOpts(t, tt.src, opts)
			for _, want := range tt.want {
				mustContainMSL(t, code, want)
			}
			// No operand of an atomic call may be a conditional.
			if strings.Contains(code, "(&uint(") {
				t.Errorf("an atomic takes the address of a conditional:\n%s", code)
			}
		})
	}
}
