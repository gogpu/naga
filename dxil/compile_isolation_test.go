// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil_test

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// Local variables inside nested blocks. Compile's passes rewrite nested
// blocks of its own copy of the module: mem2reg drops the stores of a
// variable used only inside one block, SROA splits a struct local and
// inserts emits next to its loads.
const nestedLocalsShader = `
struct Pair {
    a: f32,
    b: f32,
}

@group(0) @binding(0) var<storage, read_write> data: array<f32>;

@compute @workgroup_size(1)
fn cs() {
    var acc = 0.0;
    for (var i = 0u; i < 4u; i++) {
        if (i > 1u) {
            var t = data[i];
            t = t * 2.0;
            acc = acc + t;
        } else {
            var p = Pair(data[i], 1.0);
            p.b = p.a + p.b;
            acc = acc + p.b;
        }
    }
    data[0] = acc;
}
`

// The same shape with helper calls, which Compile inlines first.
const nestedHelperCallsShader = `
struct Pair {
    a: f32,
    b: f32,
}

@group(0) @binding(0) var<storage, read_write> data: array<f32>;

fn square(x: f32) -> f32 {
    return x * x;
}

fn twice(x: f32) -> f32 {
    var y = x;
    y = y + x;
    return y;
}

@compute @workgroup_size(1)
fn cs() {
    var acc = 0.0;
    for (var i = 0u; i < 4u; i++) {
        if (i > 1u) {
            var t = data[i];
            t = t * 2.0;
            acc = acc + square(t);
        } else {
            var p = Pair(data[i], 1.0);
            p.b = p.a + p.b;
            acc = acc + twice(p.b);
        }
    }
    data[0] = acc;
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

// TestCompileDoesNotMutateModule: Compile documents that it never mutates
// the caller's IR (BUG-IR-CLONE-001). The caller's module must be unchanged
// and a second Compile of it must produce the same container.
func TestCompileDoesNotMutateModule(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"nested_locals", nestedLocalsShader},
		{"nested_helper_calls", nestedHelperCallsShader},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := lowerWGSL(t, tt.source)
			want := lowerWGSL(t, tt.source)

			first, err := dxil.Compile(module, dxil.DefaultOptions())
			if err != nil {
				t.Fatalf("first Compile: %v", err)
			}
			if !reflect.DeepEqual(module, want) {
				t.Error("Compile changed the caller's module")
			}

			second, err := dxil.Compile(module, dxil.DefaultOptions())
			if err != nil {
				t.Fatalf("second Compile: %v", err)
			}
			if !bytes.Equal(first, second) {
				t.Errorf("second Compile of the same module differs from the first (%d vs %d bytes)", len(first), len(second))
			}
		})
	}
}
