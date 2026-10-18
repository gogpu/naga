// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package ir_test

import (
	"reflect"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// cloneIsolationShader puts override uses where a pass rewrites
// handles through nested data: blocks inside loops and ifs, the Compare
// operand of a compare-exchange, a call result and its arguments, the array
// index of a sample and a return value.
const cloneIsolationShader = `
override o: i32;
override scale: f32 = 2.0;
var<workgroup> a: atomic<u32>;
@group(0) @binding(0) var t: texture_2d_array<f32>;
@group(0) @binding(1) var s: sampler;

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

@fragment
fn fs(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    var c = vec4<f32>(0.0);
    if (uv.x > scale) {
        c = textureSample(t, s, uv, o);
    }
    return c * scale;
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

// TestCloneThenMutateLeavesSourceUnchanged is the contract the passes rely
// on (BUG-IR-CLONE-001; naga.ProcessOverrides rebuilds the arenas of a
// clone): changing a clone in depth must not change the module it was cloned
// from. Lowering is deterministic, so a second lowering of the same source is
// an independent copy to compare with.
func TestCloneThenMutateLeavesSourceUnchanged(t *testing.T) {
	source := lowerWGSL(t, cloneIsolationShader)
	want := lowerWGSL(t, cloneIsolationShader)
	if !reflect.DeepEqual(source, want) {
		t.Fatal("lowering is not deterministic; the comparison below would be meaningless")
	}

	clone := source.Clone()
	mutateInDepth(clone)

	if reflect.DeepEqual(clone, want) {
		t.Fatal("the mutation changed nothing; the shader does not exercise it")
	}
	if !reflect.DeepEqual(source, want) {
		for i := range want.EntryPoints {
			if !reflect.DeepEqual(source.EntryPoints[i], want.EntryPoints[i]) {
				t.Errorf("entry point %q changed in the source module", want.EntryPoints[i].Name)
			}
		}
		for i := range want.Functions {
			if !reflect.DeepEqual(source.Functions[i], want.Functions[i]) {
				t.Errorf("function %q changed in the source module", want.Functions[i].Name)
			}
		}
		t.Error("source module changed after a mutation of its clone")
	}
}

func TestCloneNil(t *testing.T) {
	var m *ir.Module
	if m.Clone() != nil {
		t.Error("Clone of a nil module is not nil")
	}
}

func BenchmarkModuleClone(b *testing.B) {
	tokens, err := wgsl.NewLexer(cloneIsolationShader).Tokenize()
	if err != nil {
		b.Fatal(err)
	}
	ast, err := wgsl.NewParser(tokens).Parse()
	if err != nil {
		b.Fatal(err)
	}
	module, err := wgsl.LowerWithSource(ast, cloneIsolationShader)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		_ = module.Clone()
	}
}

// mutateInDepth changes every part of m a pass may rebuild: global and
// function arenas, statement handles in nested blocks, named expressions,
// local initializers, overrides, constants and types.
func mutateInDepth(m *ir.Module) {
	ir.CompactConstants(m, ir.KeepUnusedNo)
	m.Types = append(m.Types, ir.Type{Name: "added"})
	for i := range m.GlobalExpressions {
		m.GlobalExpressions[i].Kind = ir.Literal{Value: ir.LiteralU32(uint32(i))}
	}
	for i := range m.Overrides {
		m.Overrides[i].Name += "_changed"
	}
	fns := make([]*ir.Function, 0, len(m.Functions)+len(m.EntryPoints))
	for i := range m.Functions {
		fns = append(fns, &m.Functions[i])
	}
	for i := range m.EntryPoints {
		m.EntryPoints[i].Workgroup[0]++
		fns = append(fns, &m.EntryPoints[i].Function)
	}
	for _, fn := range fns {
		shift := make([]ir.ExpressionHandle, len(fn.Expressions))
		for i := range shift {
			shift[i] = ir.ExpressionHandle(i + 1)
		}
		ir.RemapStatementExpressionHandles(fn.Body, shift)
		for i := range fn.Expressions {
			fn.Expressions[i].Kind = ir.RemapExpressionHandles(fn.Expressions[i].Kind, shift)
		}
		for h := range fn.NamedExpressions {
			fn.NamedExpressions[h] += "_changed"
		}
		for i := range fn.LocalVars {
			if fn.LocalVars[i].Init != nil {
				*fn.LocalVars[i].Init++
			}
		}
	}
}
