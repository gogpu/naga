package valid_test

import (
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
)

// storageVariable returns the module's storage global, or fails the test.
func storageVariable(t *testing.T, module *ir.Module) *ir.GlobalVariable {
	t.Helper()
	for i := range module.GlobalVariables {
		if module.GlobalVariables[i].Space == ir.SpaceStorage {
			return &module.GlobalVariables[i]
		}
	}
	t.Fatal("the module has no storage variable")
	return nil
}

// TestMemoryDecorations: Coherent/Volatile are storage-only and gated by their
// capability, as Rust naga's validate_global_var requires
// (valid/interface.rs:1157-1179).
func TestMemoryDecorations(t *testing.T) {
	caps := valid.Options{Capabilities: valid.CapMemoryDecorationCoherent | valid.CapMemoryDecorationVolatile}
	storage := parse(t, "@group(0) @binding(0) var<storage, read_write> data: array<u32>;\n"+
		"@compute @workgroup_size(1) fn main() { data[0] = 1u; }\n")

	t.Run("both decorations with both capabilities", func(t *testing.T) {
		storageVariable(t, storage).MemoryDecorations = ir.MemoryCoherent | ir.MemoryVolatile
		if _, err := valid.Validate(storage, caps); err != nil {
			t.Fatalf("Validate: %v", err)
		}
	})
	t.Run("coherent without its capability", func(t *testing.T) {
		storageVariable(t, storage).MemoryDecorations = ir.MemoryCoherent
		expectRejected(t, storage, valid.Options{}, "MEMORY_DECORATION_COHERENT")
	})
	t.Run("volatile without its capability", func(t *testing.T) {
		storageVariable(t, storage).MemoryDecorations = ir.MemoryVolatile
		expectRejected(t, storage, valid.Options{}, "MEMORY_DECORATION_VOLATILE")
	})
	t.Run("a private variable", func(t *testing.T) {
		private := parse(t, "var<private> x: u32;\n@compute @workgroup_size(1) fn main() { x = 1u; }\n")
		for i := range private.GlobalVariables {
			if private.GlobalVariables[i].Name == "x" {
				private.GlobalVariables[i].MemoryDecorations = ir.MemoryCoherent
			}
		}
		expectRejected(t, private, caps, "only valid for variables in the `storage` address space")
	})
}
