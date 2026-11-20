package lower

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// findStruct returns the lowered struct type with the given name.
func findStruct(t *testing.T, module *ir.Module, name string) ir.StructType {
	t.Helper()
	for i := range module.Types {
		if module.Types[i].Name != name {
			continue
		}
		if st, ok := module.Types[i].Inner.(ir.StructType); ok {
			return st
		}
	}
	t.Fatalf("struct %q not found in the lowered module", name)
	return ir.StructType{}
}

// TestLowerSizeAlignConstExpressions: @size and @align take const-expressions
// that resolve to an i32 or u32 (WGSL §size-attr, §align-attr), so a named
// constant, a constant expression and a hex literal must all apply. Before the
// fix only a plain decimal literal was read and every other spelling was
// silently ignored, which compiled a default layout (card P1-WGSL-122).
// Two-member structs expose the effects: @size moves the NEXT member's offset,
// @align moves the member itself and raises the struct's span.
func TestLowerSizeAlignConstExpressions(t *testing.T) {
	cases := []struct {
		name string
		src  string
		xOff uint32 // offset of the first member
		yOff uint32 // offset of the second member
		span uint32
	}{
		{
			"const size",
			`const N: u32 = 16u;
struct S { @size(N) x: f32, y: f32 }
@group(0) @binding(0) var<uniform> s: S;
fn f() -> f32 { return s.y; }`,
			0, 16, 20,
		},
		{
			"const align",
			`const A: u32 = 16u;
struct S { a: f32, @align(A) b: f32 }
@group(0) @binding(0) var<uniform> s: S;
fn f() -> f32 { return s.b; }`,
			0, 16, 32,
		},
		{
			"const expression",
			`const N: u32 = 8u;
struct S { @size(N * 2u) x: f32, y: f32 }
@group(0) @binding(0) var<uniform> s: S;
fn f() -> f32 { return s.y; }`,
			0, 16, 20,
		},
		{
			"named constant chain",
			`const A: u32 = 8u;
const B: u32 = A * 2u;
struct S { @size(B) x: f32, y: f32 }
@group(0) @binding(0) var<uniform> s: S;
fn f() -> f32 { return s.y; }`,
			0, 16, 20,
		},
		{
			"hex literal",
			`struct S { @size(0x10) x: f32, y: f32 }
@group(0) @binding(0) var<uniform> s: S;
fn f() -> f32 { return s.y; }`,
			0, 16, 20,
		},
		{
			// Control: a suffixed decimal literal applied even before the fix
			// (Sscanf stopped at the `u`) and must keep applying.
			"literal suffix",
			`struct S { @size(16u) x: f32, y: f32 }
@group(0) @binding(0) var<uniform> s: S;
fn f() -> f32 { return s.y; }`,
			0, 16, 20,
		},
		{
			"size on the last member",
			`struct S { x: f32, @size(16) y: f32 }
@group(0) @binding(0) var<uniform> s: S;
fn f() -> f32 { return s.y; }`,
			0, 4, 20,
		},
		{
			// @size on the last member extends the raw size; the struct's
			// span then rounds it up to the largest member alignment.
			"last member size with alignment interaction",
			`struct S { @align(16) x: f32, @size(16) y: f32 }
@group(0) @binding(0) var<uniform> s: S;
fn f() -> f32 { return s.y; }`,
			0, 4, 32,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := findStruct(t, mustCompile(t, tc.src), "S")
			if got := st.Members[0].Offset; got != tc.xOff {
				t.Errorf("member x offset = %d, want %d", got, tc.xOff)
			}
			if got := st.Members[1].Offset; got != tc.yOff {
				t.Errorf("member y offset = %d, want %d", got, tc.yOff)
			}
			if st.Span != tc.span {
				t.Errorf("struct span = %d, want %d", st.Span, tc.span)
			}
		})
	}
}

// TestLowerSizeAlignRules: the values the spec requires of the attributes —
// positive, @align a power of two and at least the type's alignment, @size at
// least the type's size and on a type with creation-fixed footprint; the
// messages follow Rust naga's (front/wgsl/error.rs). A runtime name or an
// override in either attribute is not a const-expression. The literal cases
// pin the argument's span in the message.
func TestLowerSizeAlignRules(t *testing.T) {
	const uniform = ` @group(0) @binding(0) var<uniform> s: S;`
	errCases := []struct {
		name, src, want string
	}{
		{"negative align", `struct S { @align(-4) x: f32 }` + uniform,
			"must be non-negative (>= 0)"},
		{"negative size", `struct S { @size(-4) x: f32 }` + uniform,
			"must be non-negative (>= 0)"},
		{"zero align", `struct S { @align(0) x: f32 }` + uniform,
			"1:19: struct member alignment must be a power of 2"},
		{"non power of two align", `struct S { @align(3) x: f32 }` + uniform,
			"1:19: struct member alignment must be a power of 2"},
		{"align below the type's", `struct S { @align(1) x: vec4<f32> }` + uniform,
			"1:19: struct member alignment must be at least 16"},
		{"size below the type's", `struct S { @size(2) x: f32 }` + uniform,
			"1:18: struct member size must be at least 4"},
		{"zero size", `struct S { @size(0) x: f32 }` + uniform,
			"1:18: struct member size must be at least 4"},
		{"size on a runtime-sized array", `struct S { @size(16) tail: array<u32> } @group(0) @binding(0) var<storage, read> s: S;`,
			"1:18: @size attribute requires a type with creation-fixed footprint"},
		{"override size", `override N: u32 = 16u;
struct S { @size(N) x: f32 }` + uniform,
			"2:18: Unexpected override-expression"},
		{"override align", `override N: u32 = 16u;
struct S { @align(N) x: f32 }` + uniform,
			"2:19: Unexpected override-expression"},
		{"function call in size", `fn g() -> u32 { return 16u; }
struct S { @size(g()) x: f32 }` + uniform,
			"Unexpected runtime-expression"},
		{"float size", `struct S { @size(1.5) x: f32 }` + uniform,
			"integer scalar"},
		{"size above the maximum", `struct S { @size(0x80000000) x: f32 }` + uniform,
			"type is too large"},
		{"overflow by accumulation", `struct S { @size(0x7FFFFFFF) x: f32, y: f32 }` + uniform,
			"type is too large"},
		{"no argument", `struct S { @size() x: f32 }` + uniform,
			"requires exactly one argument"},
		{"two arguments", `struct S { @align(16, 2) x: f32 }` + uniform,
			"requires exactly one argument"},
	}
	for _, tc := range errCases {
		t.Run(tc.name, func(t *testing.T) {
			expectError(t, tc.src, tc.want)
		})
	}
}
