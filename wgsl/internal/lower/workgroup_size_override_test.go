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
