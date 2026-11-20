package lower

import "testing"

// TestLowerTypeSizeLimit: every type's size is bounded by ir.MaxTypeSize
// (Rust naga's valid::MAX_TYPE_SIZE), checked where the size is derived so a
// uint32 wrap cannot happen (card P2-WGSL-116). An array's size is its
// element count times the stride: one element for a runtime-sized array, none
// for an override count not evaluated yet.
func TestLowerTypeSizeLimit(t *testing.T) {
	const uniform = ` @group(0) @binding(0) var<uniform> s: S;`
	errCases := []struct{ name, src, want string }{
		// 4 × 2^30 = 2^32: in uint32 arithmetic the size wrapped to 0 and the
		// struct compiled with Span 0.
		{"array size wraps", `struct S { a: array<f32, 0x40000000> }` + uniform,
			"1:12: struct member is too large"},
		{"array size just above the limit", `struct S { a: array<f32, 0x20000001> }` + uniform,
			"struct member is too large"},
		{"array size just above the limit without a struct", `@group(0) @binding(0) var<uniform> a: array<f32, 0x20000001>;`,
			"type is too large"},
		// The inner array is over the limit on its own; the outer count never
		// comes into play.
		{"nested array element over the limit", `@group(0) @binding(0) var<uniform> a: array<array<f32, 0x20000000>, 2>;`,
			"type is too large"},
		// 64 bytes a column-major mat4x4 element × 2^29 columns.
		{"matrix array over the limit", `@group(0) @binding(0) var<uniform> a: array<mat4x4<f32>, 0x20000000>;`,
			"type is too large"},
	}
	for _, tc := range errCases {
		t.Run(tc.name, func(t *testing.T) {
			expectError(t, tc.src, tc.want)
		})
	}
}

// TestLowerTypeSizeLimitBoundary: MaxTypeSize is a strict upper bound. A u32
// array of 0x1FFFFFFF elements is 0x7FFFFFFC bytes and stays accepted;
// 0x20000000 elements are 0x80000000 and are refused.
func TestLowerTypeSizeLimitBoundary(t *testing.T) {
	mustCompile(t, `alias A = array<u32, 0x1FFFFFFF>;`)
	expectError(t, `alias B = array<u32, 0x20000000>;`, "type is too large")
}
