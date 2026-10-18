package registry

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestNewTypeRegistryFrom: existing types keep their handles and are found
// by GetOrCreate; a new type is appended after them.
func TestNewTypeRegistryFrom(t *testing.T) {
	u32 := ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	types := []ir.Type{{Inner: u32}, {Name: "S", Inner: ir.StructType{}}, {Inner: u32}}
	r := NewTypeRegistryFrom(types)
	if h := r.GetOrCreate("", u32); h != 0 {
		t.Errorf("u32: got handle %d, want 0 (first of two equal entries)", h)
	}
	if h := r.GetOrCreate("S", ir.StructType{}); h != 1 {
		t.Errorf("S: got handle %d, want 1", h)
	}
	if h := r.GetOrCreate("", f32); h != 3 {
		t.Errorf("f32: got handle %d, want 3 (appended)", h)
	}
	if got := len(r.GetTypes()); got != 4 {
		t.Errorf("types: got %d, want 4", got)
	}
	types[0].Name = "changed"
	if r.GetTypes()[0].Name != "" {
		t.Error("registry shares the caller's slice")
	}
}
