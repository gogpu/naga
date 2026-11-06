package snapshot_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gogpu/naga/valid"
	"github.com/gogpu/naga/wgsl"
)

// TestDeepestAcceptedShadersCompile compiles, with every backend, shaders at
// the deepest nesting and the longest chains the WGSL parser accepts. The
// parser's limits (recursion 200 and brace nesting 127 as in Rust naga;
// expression depth 512 and statement chaining 127 as in Tint) are only a
// defense if every later pass, all of them recursive over the expression
// and statement trees, also survives what the parser lets through. Without
// the chain limits, a long operator chain parsed and then killed the process
// with Go's fatal "stack overflow" in a backend writer.
func TestDeepestAcceptedShadersCompile(t *testing.T) {
	r := strings.Repeat
	// The deep statement shapes store to the local `v` and copy it to storage
	// at the top: the HLSL writer refuses a storage store more than 20 indent
	// levels deep, a separate limit of its own (Rust has none).
	const prelude = "@group(0) @binding(0) var<storage, read_write> o: array<i32>;\n" +
		"@compute @workgroup_size(1) fn main() {\n  let a = o[0];\n  var v = a;\n"
	elseIf := func(n int) string {
		var b strings.Builder
		b.WriteString("  if a == 0 { v = 0; }")
		for i := 1; i <= n; i++ {
			fmt.Fprintf(&b, " else if a == %d { v = %d; }", i, i)
		}
		b.WriteString(" else { v = -1; }\n  o[1] = v;\n")
		return b.String()
	}
	cases := []struct{ name, body string }{
		// 512 operators: the tree height Tint's expression depth allows.
		{"binary chain", "  o[1] = a" + r(" + a", 512) + ";\n"},
		{"unary chain", "  o[1] = " + r("- ", 512) + "a;\n"},
		{"mixed chain", "  o[1] = a" + r(" * (a + (a - a))", 170) + ";\n"},
		// 125 else-ifs: the function body, the first if and 125 chained
		// bodies are the 127 statement levels.
		{"else-if chain", elseIf(125)},
		// 126 nested blocks inside the function body: brace level 127.
		{"nested blocks", "  " + r("{ v += 1; ", 126) + r("} ", 126) + "\n  o[1] = v;\n"},
		{"nested ifs", "  " + r("if a > 0 { ", 126) + "v = 1;" + r(" }", 126) + "\n  o[1] = v;\n"},
		{"nested loops", "  " + r("loop { ", 126) + "v += 1; break;" + r(" }", 126) + "\n  o[1] = v;\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			module, _, err := wgsl.Parse(prelude+tc.body+"}\n", wgsl.ParseOptions{})
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if _, err := valid.Validate(module, valid.Options{Capabilities: valid.CapAll}); err != nil {
				t.Fatalf("validate: %v", err)
			}
			for _, c := range determinismCompilers(module) {
				if _, err := c.compile(module); err != nil {
					t.Errorf("%s: %v", c.name, err)
				}
			}
		})
	}
}
