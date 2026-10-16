// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl_test

import (
	"reflect"
	"testing"

	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// Override uses sit where override processing rewrites handles through
// nested data: blocks inside loops and ifs, the Compare operand of a
// compare-exchange, a call result and its arguments, a return value.
const overridesInNestedBlocksShader = `
override o: i32;
override scale: f32 = 2.0;
var<workgroup> a: atomic<u32>;

fn helper(x: f32) -> f32 {
    return x * scale;
}

@compute @workgroup_size(1)
fn cs() {
    for (var i = 0; i < o; i++) {
        if (i > 0) {
            atomicCompareExchangeWeak(&a, u32(o), 1u);
            let h = helper(f32(i) + scale);
            atomicStore(&a, u32(h));
        }
    }
}
`

func lowerWGSL(t *testing.T, source string) *ir.Module {
	t.Helper()
	tokens, err := wgsl.NewLexer(source).Tokenize()
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	ast, err := wgsl.NewParser(tokens).Parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	module, err := wgsl.LowerWithSource(ast, source)
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	return module
}

// TestCompileWithPipelineConstantsDoesNotMutateModule: one lowered module is
// compiled for several pipelines (BUG-IR-CLONE-001). Each Compile must see
// the module as lowered, so the same options give the same GLSL every time
// and the caller's module is left as it was.
func TestCompileWithPipelineConstantsDoesNotMutateModule(t *testing.T) {
	module := lowerWGSL(t, overridesInNestedBlocksShader)
	want := lowerWGSL(t, overridesInNestedBlocksShader)

	opts := glsl.Options{
		LangVersion:       glsl.VersionES310,
		EntryPoint:        "cs",
		PipelineConstants: ir.PipelineConstants{"o": 3},
	}
	first, _, err := glsl.Compile(module, opts)
	if err != nil {
		t.Fatalf("first Compile: %v", err)
	}
	if !reflect.DeepEqual(module, want) {
		t.Error("Compile changed the caller's module")
	}

	second, _, err := glsl.Compile(module, opts)
	if err != nil {
		t.Fatalf("second Compile: %v", err)
	}
	if second != first {
		t.Errorf("second Compile of the same module differs from the first\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}
