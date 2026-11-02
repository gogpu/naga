package codegen

import (
	"testing"

	"github.com/gogpu/naga/backend"
)

// Bounds checks on runtime-sized arrays (P0-MSL-100). As Rust naga's
// put_dynamic_array_max_index (back/msl/writer.rs), the array's global comes
// from proc::Function::originating_global, a top-level `array<T>` global sits
// at offset 0, a struct's runtime array is its last member, and the size field
// is named after the global's handle (`sizeN`). Every expected line below is the
// output of Rust naga 14328f49 run with the same policy on Index and Buffer,
// except where an element's size is below its stride (see below).
//
// naga v0.21.1 left a top-level runtime array unclamped under Restrict: an
// out-of-bounds write landed in the next binding on Metal (wgpu's
// TestStorageBufferOutOfBoundsContained).
//
// Deviation from Rust: the bound is NRuntime - 1 = (EBBS - offset - stride) /
// stride, the WGSL spec's NRuntime = truncate((EBBS - offset) / stride)
// (wgsl/index.bs, "element count of runtime-sized array"). Rust subtracts the
// element size instead, `(... - 0 - 12) / 16` for array<vec3<f32>>, which
// admits index N on a binding of 16N+12 bytes, out of bounds by the spec.

// Rust naga 14328f49 output for runtimeArrayVec3WGSL, for reference:
//
//	Restrict:          a[metal::min(unsigned(gid.x), (_buffer_sizes.size0 - 0 - 12) / 16)]
//	ReadZeroSkipWrite: uint(_eN) < 1 + (_buffer_sizes.size0 - 0 - 12) / 16

const runtimeArrayTopLevelWGSL = `
@group(0) @binding(0) var<storage, read_write> a: array<u32>;
@group(0) @binding(1) var<storage, read_write> b: array<u32, 4>;
@compute @workgroup_size(1) fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let r = a[66u + gid.x]; a[65u + gid.x] = 161u; b[2] = r; }
`

const runtimeArrayPointerWGSL = `
@group(0) @binding(0) var<storage, read_write> a: array<u32>;
@compute @workgroup_size(1) fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let p = &a; let r = (*p)[gid.x + 1u]; (*p)[gid.x] = r; }
`

const runtimeArrayMemberWGSL = `
struct S { n: u32, d: array<u32> }
@group(0) @binding(0) var<storage, read_write> s: S;
@compute @workgroup_size(1) fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let r = s.d[66u + gid.x]; s.d[65u + gid.x] = r; }
`

const runtimeArrayVec3WGSL = `
@group(0) @binding(0) var<storage, read_write> a: array<vec3<f32>>;
struct S { n: u32, d: array<vec3<f32>> }
@group(0) @binding(1) var<storage, read_write> s: S;
@compute @workgroup_size(1) fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let r = a[gid.x]; a[gid.x + 1u] = r; let q = s.d[gid.x]; s.d[gid.x + 2u] = q; }
`

const runtimeArrayConstIndexWGSL = `
@group(0) @binding(0) var<storage, read_write> a: array<u32>;
struct S { n: u32, d: array<u32> }
@group(0) @binding(1) var<storage, read_write> s: S;
@compute @workgroup_size(1) fn main() {
    let r = a[3]; a[5] = r; let q = s.d[7]; s.d[2] = q; let p = &a; (*p)[4] = q; }
`

// A sized global comes first, so the runtime arrays' handles (1, 2) differ from
// their positions in _mslBufferSizes (0, 1).
const runtimeArrayGlobalOrderWGSL = `
@group(0) @binding(0) var<storage, read_write> b: array<u32, 4>;
struct S { n: u32, d: array<u32> }
@group(0) @binding(1) var<storage, read_write> s: S;
@group(0) @binding(2) var<storage, read_write> a: array<u32>;
@compute @workgroup_size(1) fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let r = a[gid.x]; a[gid.x + 1u] = r; let q = s.d[gid.x]; s.d[gid.x + 2u] = q; b[gid.x] = q; }
`

type runtimeArrayCase struct {
	name string
	src  string
	want []string
}

func runRuntimeArrayCases(t *testing.T, policy backend.BoundsCheckPolicy, tests []runtimeArrayCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DefaultOptions()
			opts.BoundsCheckPolicies.Index = policy
			opts.BoundsCheckPolicies.Buffer = policy
			code := compileWGSLWithOpts(t, tt.src, opts)
			for _, w := range tt.want {
				mustContainMSL(t, code, w)
			}
		})
	}
}

func TestRestrictRuntimeSizedArray(t *testing.T) {
	runRuntimeArrayCases(t, backend.BoundsCheckRestrict, []runtimeArrayCase{
		{
			name: "top-level global",
			src:  runtimeArrayTopLevelWGSL,
			want: []string{
				"uint r = a[metal::min(unsigned(66u + gid.x), (_buffer_sizes.size0 - 0 - 4) / 4)];",
				"a[metal::min(unsigned(65u + gid.x), (_buffer_sizes.size0 - 0 - 4) / 4)] = 161u;",
				"b.inner[2] = r;",
			},
		},
		{
			name: "through a pointer to the global",
			src:  runtimeArrayPointerWGSL,
			want: []string{
				"uint r = a[metal::min(unsigned(gid.x + 1u), (_buffer_sizes.size0 - 0 - 4) / 4)];",
				"a[metal::min(unsigned(gid.x), (_buffer_sizes.size0 - 0 - 4) / 4)] = r;",
			},
		},
		{
			name: "struct member",
			src:  runtimeArrayMemberWGSL,
			want: []string{
				"uint r = s.d[metal::min(unsigned(66u + gid.x), (_buffer_sizes.size0 - 4 - 4) / 4)];",
				"s.d[metal::min(unsigned(65u + gid.x), (_buffer_sizes.size0 - 4 - 4) / 4)] = r;",
			},
		},
		{
			name: "element size below stride (spec bound, not Rust's)",
			src:  runtimeArrayVec3WGSL,
			want: []string{
				"metal::float3 r = a[metal::min(unsigned(gid.x), (_buffer_sizes.size0 - 0 - 16) / 16)];",
				"a[metal::min(unsigned(gid.x + 1u), (_buffer_sizes.size0 - 0 - 16) / 16)] = r;",
				"metal::float3 q = s.d[metal::min(unsigned(gid.x), (_buffer_sizes.size1 - 16 - 16) / 16)];",
				"s.d[metal::min(unsigned(gid.x + 2u), (_buffer_sizes.size1 - 16 - 16) / 16)] = q;",
			},
		},
		{
			name: "constant index",
			src:  runtimeArrayConstIndexWGSL,
			want: []string{
				"uint r = a[metal::min(unsigned(3), (_buffer_sizes.size0 - 0 - 4) / 4)];",
				"a[metal::min(unsigned(5), (_buffer_sizes.size0 - 0 - 4) / 4)] = r;",
				"uint q = s.d[metal::min(unsigned(7), (_buffer_sizes.size1 - 4 - 4) / 4)];",
				"s.d[metal::min(unsigned(2), (_buffer_sizes.size1 - 4 - 4) / 4)] = q;",
				"a[metal::min(unsigned(4), (_buffer_sizes.size0 - 0 - 4) / 4)] = q;",
			},
		},
		{
			name: "size field named after the global",
			src:  runtimeArrayGlobalOrderWGSL,
			want: []string{
				"uint size1;",
				"uint size2;",
				"uint r = a[metal::min(unsigned(gid.x), (_buffer_sizes.size2 - 0 - 4) / 4)];",
				"a[metal::min(unsigned(gid.x + 1u), (_buffer_sizes.size2 - 0 - 4) / 4)] = r;",
				"uint q = s.d[metal::min(unsigned(gid.x), (_buffer_sizes.size1 - 4 - 4) / 4)];",
				"s.d[metal::min(unsigned(gid.x + 2u), (_buffer_sizes.size1 - 4 - 4) / 4)] = q;",
				"b.inner[metal::min(unsigned(gid.x), 3u)] = q;",
			},
		},
	})
}

func TestReadZeroSkipWriteRuntimeSizedArray(t *testing.T) {
	runRuntimeArrayCases(t, backend.BoundsCheckReadZeroSkipWrite, []runtimeArrayCase{
		{
			name: "top-level global",
			src:  runtimeArrayTopLevelWGSL,
			want: []string{
				"uint r = uint(_e4) < 1 + (_buffer_sizes.size0 - 0 - 4) / 4 ? a[_e4] : DefaultConstructible();",
				"if (uint(_e10) < 1 + (_buffer_sizes.size0 - 0 - 4) / 4) {",
				"a[_e10] = 161u;",
			},
		},
		{
			name: "element size below stride (spec bound, not Rust's)",
			src:  runtimeArrayVec3WGSL,
			want: []string{
				" < 1 + (_buffer_sizes.size0 - 0 - 16) / 16 ? a[",
				" < 1 + (_buffer_sizes.size1 - 16 - 16) / 16 ? s.d[",
			},
		},
		{
			// Only the stored-to pointer is written unchecked inside the
			// store's `if`; the stored value keeps its own checks (Rust
			// put_store).
			name: "stored value keeps its checks",
			src: `
@group(0) @binding(0) var<storage, read_write> b: array<u32>;
@group(0) @binding(1) var<storage, read_write> m: array<vec4<f32>>;
@compute @workgroup_size(1) fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let v = array<u32, 4>(1u, 2u, 3u, 4u);
    b[gid.x] = v[gid.y];
    m[gid.z] = vec4(f32(v[gid.x]));
    b[gid.y + 1u] = b[gid.z] + v[gid.z]; }
`,
			want: []string{
				"b[_e7] = uint(_e9) < 4 ? v.inner[_e9] : DefaultConstructible();",
				"m[_e12] = metal::float4(static_cast<float>(uint(_e14) < 4 ? v.inner[_e14] : DefaultConstructible()));",
				"uint _e26 = uint(_e24) < 1 + (_buffer_sizes.size0 - 0 - 4) / 4 ? b[_e24] : DefaultConstructible();",
				"b[_e21] = _e26 + (uint(_e27) < 4 ? v.inner[_e27] : DefaultConstructible());",
			},
		},
		{
			name: "size field named after the global",
			src:  runtimeArrayGlobalOrderWGSL,
			want: []string{
				"uint r = uint(_e2) < 1 + (_buffer_sizes.size2 - 0 - 4) / 4 ? a[_e2] : DefaultConstructible();",
				"if (uint(_e8) < 1 + (_buffer_sizes.size2 - 0 - 4) / 4) {",
				"uint q = uint(_e12) < 1 + (_buffer_sizes.size1 - 4 - 4) / 4 ? s.d[_e12] : DefaultConstructible();",
				"if (uint(_e19) < 1 + (_buffer_sizes.size1 - 4 - 4) / 4) {",
				"if (uint(_e22) < 4) {",
			},
		},
	})
}
