package naga

import (
	"fmt"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/valid"
	"github.com/gogpu/naga/wgsl"
)

const computeHead = "@compute @workgroup_size(1) fn main() {\n var x: i32 = 0;\n"

// elseIfChain is an if with links `else if` links whose final else runs last.
func elseIfChain(links int, last string) string {
	var b strings.Builder
	b.WriteString(" if x == 0 { x = 1; }")
	for i := 1; i <= links; i++ {
		fmt.Fprintf(&b, " else if x == %d { x = %d; }", i, i+1)
	}
	b.WriteString(" else { " + last + " }\n")
	return b.String()
}

// deepChainShader is a compute shader whose body is an if with links
// `else if` links (statement depth 2+links) and, in its final else, a unary
// chain of depth operators.
func deepChainShader(links, depth int) string {
	return computeHead + elseIfChain(links, "x = "+strings.Repeat("- ", depth)+"x;") + "}\n"
}

// compileEverywhere runs source through the parser, the validator, override
// processing and every backend.
func compileEverywhere(t *testing.T, source string) {
	t.Helper()
	module, _, err := wgsl.Parse(source, wgsl.ParseOptions{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info, err := valid.Validate(module, valid.Options{})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if module, _, err = ProcessOverrides(module, info, OverrideOptions{}); err != nil {
		t.Fatalf("process overrides: %v", err)
	}
	if _, _, err := spirv.Compile(module, spirv.Options{}); err != nil {
		t.Errorf("spirv: %v", err)
	}
	if _, _, err := msl.Compile(module, msl.Options{}); err != nil {
		t.Errorf("msl: %v", err)
	}
	if _, _, err := hlsl.Compile(module, hlsl.Options{}); err != nil {
		t.Errorf("hlsl: %v", err)
	}
	if _, _, err := glsl.Compile(module, glsl.Options{LangVersion: glsl.VersionES310}); err != nil {
		t.Errorf("glsl: %v", err)
	}
	if _, _, err := dxil.Compile(module, dxil.Options{}); err != nil {
		t.Errorf("dxil: %v", err)
	}
}

// TestRustFloorChainsCompile: Rust naga reads else-if and operator chains in
// a loop and bounds neither; its CLI (14328f49, 1 MiB main stack on Windows)
// compiles 650 else-if links and 1500 unary operators and overflows its
// stack beyond. Every such shader must compile here too, through every
// backend (naga's floor: no shader Rust accepts is refused).
func TestRustFloorChainsCompile(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles shaders with thousands of nested statements")
	}
	t.Run("650 else-if links", func(t *testing.T) { compileEverywhere(t, deepChainShader(650, 1)) })
	t.Run("1500 unary operators", func(t *testing.T) { compileEverywhere(t, deepChainShader(0, 1500)) })
}

// TestDeepestAcceptedShadersFitTheStack compiles the deepest shapes the
// parser accepts (statement depth 1024, expressions 4096 deep) through every
// pass with the goroutine stack capped at 64 MiB. The deepest needed 4 MiB
// when the limits were chosen (2026-11-08). A pass that starts to recurse
// deeper per level ends this test with Go's fatal stack overflow, before it
// can end a host process that runs on a smaller stack than Go's 1 GiB
// default.
func TestDeepestAcceptedShadersFitTheStack(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles shaders with a thousand nested statements")
	}
	defer debug.SetMaxStack(debug.SetMaxStack(64 << 20))
	r := strings.Repeat
	const ops = 4096 // maxExpressionDepth
	cases := []struct{ name, src string }{
		// Statement depth 2+1022 = 1024, with a 4088-deep expression inside.
		{"else-if chain with a unary chain inside", deepChainShader(1022, ops-8)},
		{"binary chain", computeHead + " x = x" + r(" + x", ops) + ";\n}\n"},
		{"logical not chain", computeHead + " var b = x == 0;\n b = " + r("!", ops) + "b;\n if b { x = 2; }\n}\n"},
		{"swizzle chain", computeHead + " let v = vec4<i32>(x);\n x = v" + r(".wzyx", ops-1) + ".x;\n}\n"},
		{"const initializer chain", "const c = 1" + r(" + 1", ops) + ";\n" + computeHead + " x = c;\n}\n"},
		{"override initializer chain", "override o: i32 = 1" + r(" + 1", ops) + ";\n" + computeHead + " x = o;\n}\n"},
		{
			// 60 nested loops take levels 2-61, the if body 62, link k 62+k.
			"else-if chain in nested loops in a helper",
			"fn helper(y: i32) -> i32 {\n var x = y;\n" + r(" loop {", 60) + elseIfChain(962, "x = -x;") +
				r(" break; }", 60) + "\n return x;\n}\n" + computeHead + " x = helper(x);\n}\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { compileEverywhere(t, tc.src) })
	}

	// One more link is refused by the parser, not by a crash.
	_, _, err := wgsl.Parse(deepChainShader(1023, 1), wgsl.ParseOptions{})
	if err == nil || !strings.Contains(err.Error(), "statement nesting depth / chaining length exceeds limit of 1024") {
		t.Fatalf("1023 links: want the statement depth error, got %v", err)
	}
}
