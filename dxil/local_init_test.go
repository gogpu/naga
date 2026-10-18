// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"testing"

	"github.com/gogpu/naga/dxil/internal/emit"
	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/wgsl"
)

// TestLocalInitializerStoredBeforeUse: a local var with an initializer that
// stays in memory (its value changes in a loop, so mem2reg leaves the alloca)
// must have the initializer stored before it is read. The emitter allocated
// such locals without storing the initializer, so a loop counter started from
// garbage: `for (var j = 0u; j < 4u; j++) { result[j] = ...; }` wrote nothing
// on the GPU while IDxcValidator accepted the module. Mesa lowers function
// temporaries' initializers to stores at the top of the function
// (nir_lower_variable_initializers); DXC stores them after the alloca.
func TestLocalInitializerStoredBeforeUse(t *testing.T) {
	for name, body := range map[string]string{
		"scalar loop counter": `
    for (var j = 0u; j < 4u; j++) {
        result[j] = j + 1u;
    }`,
		// The counter is initialized from a run-time value, so the
		// lowerer emits a store before the loop instead of an Init.
		// mem2reg's structured pass dropped that store and only then
		// found the counter stored in the loop and gave up on it.
		"run-time initializer": `
    let i = result[7];
    for (var j = i; j < 4u; j++) {
        result[j] = j + 1u;
    }`,
		"vector": `
    var v = vec2<u32>(3u, 4u);
    for (var i = 0u; i < 2u; i++) {
        v = v + vec2<u32>(1u);
        result[i] = v.x + v.y;
    }`,
		"array": `
    var a = array<u32, 2>(5u, 6u);
    for (var i = 0u; i < 2u; i++) {
        a[i] = a[i] + 1u;
        result[i] = a[i];
    }`,
		"struct": `
    var s = S(7u, 8u);
    for (var i = 0u; i < 2u; i++) {
        s.x = s.x + s.y;
        result[i] = s.x;
    }`,
	} {
		t.Run(name, func(t *testing.T) {
			src := "struct S { x: u32, y: u32 }\n" +
				"@group(0) @binding(0) var<storage, read_write> result: array<u32>;\n" +
				"@compute @workgroup_size(1)\nfn main() {" + body + "\n}\n"
			fn := emitMain(t, src)
			for ptr, first := range firstAccessToAllocas(fn) {
				if first != module.InstrStore {
					t.Errorf("alloca %%%d is read before anything is stored to it (first access: load)", ptr)
				}
			}
		})
	}
}

// emitMain runs the module through the Compile pipeline up to emission and
// returns the main function.
func emitMain(t *testing.T, src string) *module.Function {
	t.Helper()
	tokens, err := wgsl.NewLexer(src).Tokenize()
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	ast, err := wgsl.NewParser(tokens).Parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	irMod, err := wgsl.LowerWithSource(ast, src)
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	irMod, err = prepareModule(irMod)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if err := runOptPasses(irMod); err != nil {
		t.Fatalf("passes: %v", err)
	}
	mod, _, err := emit.EmitWithFlags(irMod, emit.EmitOptions{
		ShaderModelMajor: 6,
		ShaderModelMinor: 0,
		ReachableGlobals: reachableGlobalsForEntry(irMod, &irMod.EntryPoints[0]),
	})
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	for _, f := range mod.Functions {
		if f.Name == "main" {
			return f
		}
	}
	t.Fatal("main not found")
	return nil
}

// firstAccessToAllocas maps every alloca of fn to the kind of the first load
// or store whose pointer is the alloca or a GEP of it, in block order. The
// entry block, where the allocas and their initializers live, comes first.
func firstAccessToAllocas(fn *module.Function) map[int]module.InstrKind {
	root := map[int]int{} // value ID -> alloca it points into
	first := map[int]module.InstrKind{}
	for _, bb := range fn.BasicBlocks {
		for _, in := range bb.Instructions {
			switch in.Kind {
			case module.InstrAlloca:
				root[in.ValueID] = in.ValueID
			case module.InstrGEP:
				// Operands: inbounds, source element type, pointer, indices.
				if len(in.Operands) > 2 {
					if a, ok := root[in.Operands[2]]; ok {
						root[in.ValueID] = a
					}
				}
			case module.InstrLoad, module.InstrStore:
				if len(in.Operands) == 0 {
					continue
				}
				if a, ok := root[in.Operands[0]]; ok {
					if _, seen := first[a]; !seen {
						first[a] = in.Kind
					}
				}
			}
		}
	}
	return first
}

// TestLocalVectorDynamicIndex: a local vector is lowered to one alloca per
// component, so `v[i]` with a run-time i must reach every component. emitAccess
// fell back to the first component's alloca: `v[i] = v[i] + 1u` in a loop over
// i read and wrote v.x only (on the GPU v = (12, 20) instead of (11, 21)).
// Mesa lowers such indirect accesses to a select over the components
// (nir_lower_indirect_derefs).
func TestLocalVectorDynamicIndex(t *testing.T) {
	fn := emitMain(t, `
@group(0) @binding(0) var<storage, read_write> result: array<u32>;
@compute @workgroup_size(1)
fn main() {
    var v = vec3<u32>(10u, 20u, 30u);
    for (var i = 0u; i < 3u; i++) {
        v[i] = v[i] + 1u;
    }
    result[0] = v.x;
    result[1] = v.y;
    result[2] = v.z;
}
`)
	allocas := map[int]bool{}
	storedInLoop := map[int]bool{}
	loadedInLoop := map[int]bool{}
	for bi, bb := range fn.BasicBlocks {
		for _, in := range bb.Instructions {
			switch in.Kind {
			case module.InstrAlloca:
				allocas[in.ValueID] = true
			case module.InstrStore:
				if bi > 0 && allocas[in.Operands[0]] {
					storedInLoop[in.Operands[0]] = true
				}
			case module.InstrLoad:
				if bi > 0 && allocas[in.Operands[0]] {
					loadedInLoop[in.Operands[0]] = true
				}
			}
		}
	}
	// Four allocas: v.x, v.y, v.z and the counter i. Every one of them is
	// read and written after the entry block.
	if len(allocas) != 4 {
		t.Fatalf("allocas = %d, want 4 (3 vector components + counter)", len(allocas))
	}
	for a := range allocas {
		if !storedInLoop[a] {
			t.Errorf("alloca %%%d is never stored after the entry block; v[i] must reach every component", a)
		}
		if !loadedInLoop[a] {
			t.Errorf("alloca %%%d is never loaded after the entry block", a)
		}
	}
}
