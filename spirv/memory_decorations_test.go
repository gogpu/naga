package spirv

import (
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// TestMemoryDecorationsRoundTrip: the Coherent and Volatile decorations of a
// storage variable survive SPIR-V write and read (card P2-SPVIN-001: they used
// to be dropped because the IR had no field for them).
func TestMemoryDecorationsRoundTrip(t *testing.T) {
	const source = "@group(0) @binding(0) var<storage, read_write> data: array<u32>;\n" +
		"@compute @workgroup_size(1) fn main() { data[0] = 1u; }\n"
	module, _, err := wgsl.Parse(source, wgsl.ParseOptions{})
	if err != nil {
		t.Fatalf("parse WGSL: %v", err)
	}
	if len(module.GlobalVariables) != 1 {
		t.Fatalf("globals = %d, want 1", len(module.GlobalVariables))
	}
	module.GlobalVariables[0].MemoryDecorations = ir.MemoryCoherent | ir.MemoryVolatile

	binary, _, err := Compile(module, Options{})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	parsed, _, err := Parse(binary, ParseOptions{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(parsed.GlobalVariables) != 1 {
		t.Fatalf("parsed globals = %d, want 1", len(parsed.GlobalVariables))
	}
	if got := parsed.GlobalVariables[0].MemoryDecorations; got != ir.MemoryCoherent|ir.MemoryVolatile {
		t.Errorf("memory decorations = %b, want coherent|volatile", got)
	}
}
