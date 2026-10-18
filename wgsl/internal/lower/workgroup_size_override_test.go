package lower

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestWorkgroupSizeOverride: a workgroup size given by an override-expression
// is kept as a global expression for pipeline creation (Rust
// workgroup_size_overrides); a const size is folded into Workgroup. The
// literal Workgroup entry for an override dimension stays 1, as in Rust.
func TestWorkgroupSizeOverride(t *testing.T) {
	m := mustCompile(t, `
override wg: u32 = 4u;
@compute @workgroup_size(wg * 2u, 2) fn main() {}
`)
	ep := m.EntryPoints[0]
	if ep.Workgroup != [3]uint32{1, 2, 1} {
		t.Errorf("Workgroup = %v, want [1 2 1]", ep.Workgroup)
	}
	o := ep.WorkgroupSizeOverrides
	if o[0] == nil || o[1] != nil || o[2] != nil {
		t.Fatalf("WorkgroupSizeOverrides = %v, want only x set", o)
	}
	b, ok := m.GlobalExpressions[*o[0]].Kind.(ir.ExprBinary)
	if !ok || b.Op != ir.BinaryMultiply {
		t.Fatalf("x size = %#v, want wg * 2u", m.GlobalExpressions[*o[0]].Kind)
	}
	if _, ok := m.GlobalExpressions[b.Left].Kind.(ir.ExprOverride); !ok {
		t.Errorf("x size left operand = %#v, want Override(wg)", m.GlobalExpressions[b.Left].Kind)
	}
}

// TestWorkgroupSizeErrors: sizes Rust rejects are errors, not a silent 1
// (Rust naga v30 messages).
func TestWorkgroupSizeErrors(t *testing.T) {
	tests := []struct{ name, src, want string }{
		{"negative", `@compute @workgroup_size(-1) fn main() {}`, "must be non-negative (>= 0)"},
		{"float", `@compute @workgroup_size(1.5) fn main() {}`,
			"must be a const-expression that resolves to a concrete integer scalar (`u32` or `i32`)"},
		{"float override", "override f: f32 = 2.0;\n@compute @workgroup_size(f) fn main() {}",
			"must be a const-expression that resolves to a concrete integer scalar (`u32` or `i32`)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expectError(t, tc.src, tc.want)
		})
	}
}

// TestArraySizeOverride: an array sized by an override-expression has a
// pending size (Rust ArraySize::Pending): the override itself for a bare
// override, a new override without name or id initialized with the
// expression otherwise (Rust array_size_override).
func TestArraySizeOverride(t *testing.T) {
	m := mustCompile(t, `
override n: u32 = 4u;
var<workgroup> a: array<u32, n>;
var<workgroup> b: array<u32, n * 2u>;
@compute @workgroup_size(1) fn main() { a[0] = 1u; b[0] = a[0]; }
`)
	sizes := map[string]ir.ArraySize{}
	for _, gv := range m.GlobalVariables {
		if at, ok := m.Types[gv.Type].Inner.(ir.ArrayType); ok {
			sizes[gv.Name] = at.Size
		}
	}
	a, b := sizes["a"], sizes["b"]
	if a.Constant != nil || a.Pending == nil || m.Overrides[*a.Pending].Name != "n" {
		t.Fatalf("a: size %+v, want pending on override n", a)
	}
	if b.Constant != nil || b.Pending == nil {
		t.Fatalf("b: size %+v, want pending", b)
	}
	ob := m.Overrides[*b.Pending]
	if ob.Name != "" || ob.ID != nil || ob.Init == nil {
		t.Fatalf("b's override = %+v, want no name or id and an initializer", ob)
	}
	if bin, ok := m.GlobalExpressions[*ob.Init].Kind.(ir.ExprBinary); !ok || bin.Op != ir.BinaryMultiply {
		t.Errorf("b's override initializer = %#v, want n * 2u", m.GlobalExpressions[*ob.Init].Kind)
	}
	if a.IsRuntimeSized() || b.IsRuntimeSized() {
		t.Error("a pending size is not runtime-sized")
	}
}

// TestPendingArrayTypeName: diagnostics name an array sized by an override
// with its override (Rust writes array<n>, without the element type).
func TestPendingArrayTypeName(t *testing.T) {
	expectError(t, "override n: u32 = 4u;\nvar<workgroup> a: array<u32, n>;\n@compute @workgroup_size(1) fn main() { let x: f32 = a; }\n", "array<u32, n>")
}
