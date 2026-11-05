// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"os"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/internal/typifier"
	"github.com/gogpu/naga/ir"
)

// manyScalarOutputsVS has nine output elements that pack into three rows:
// the position, then eight f32 locations, four to a row.
const manyScalarOutputsVS = `
struct Out {
    @builtin(position) pos: vec4<f32>,
    @location(0) a: f32,
    @location(1) b: f32,
    @location(2) c: f32,
    @location(3) d: f32,
    @location(4) e: f32,
    @location(5) f: f32,
    @location(6) g: f32,
    @location(7) h: f32,
}

@vertex
fn main(@location(0) p: vec2<f32>, @location(1) q: f32) -> Out {
    var o: Out;
    o.pos = vec4(p, q, 1.0);
    o.a = q; o.b = p.x; o.c = p.y; o.d = q;
    o.e = q; o.f = p.x; o.g = p.y; o.h = q;
    return o;
}
`

// manyScalarInputsFS has nine input elements that pack into three rows.
const manyScalarInputsFS = `
struct In {
    @builtin(position) pos: vec4<f32>,
    @location(0) a: f32,
    @location(1) b: f32,
    @location(2) c: f32,
    @location(3) d: f32,
    @location(4) e: f32,
    @location(5) f: f32,
    @location(6) g: f32,
    @location(7) h: f32,
}

@fragment
fn main(i: In) -> @location(0) vec4<f32> {
    return vec4(i.a + i.b, i.c + i.d, i.e + i.f, i.g + i.h);
}
`

// TestPSVViewIDTableUsesSignatureRows (P1-DXIL-089): the PSV0 input-to-output
// table is laid out by the signatures' packed row counts, the
// SigInputVectors and SigOutputVectors of the PSV0 header, as DXC's
// StoreViewIDStateToPSV reads it (DxilContainerAssembler.cpp:541-561,
// PSVComputeMaskDwordsFromVectors of the output rows, one mask per input
// scalar). The analyzer counted signature elements instead: nine output
// elements in three rows gave two mask dwords per input where the header
// says one, so every input's bits landed at twice its index, and nine input
// elements in three rows gave a table three times the header's size.
// IDxcValidator rejected both with 0x80aa0013 (ViewIDState mismatch between
// PSV0 and the module); gg's sdf_render vs_main is the production case.
func TestPSVViewIDTableUsesSignatureRows(t *testing.T) {
	ggSDF, err := os.ReadFile("../testdata/upstream/github.com/gogpu/gg/internal/gpu/shaders/sdf_render.wgsl")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, src, entry string
	}{
		{"nine outputs in three rows", manyScalarOutputsVS, "main"},
		{"nine inputs in three rows", manyScalarInputsFS, "main"},
		{"gg sdf_render vs_main", string(ggSDF), "vs_main"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := lowerSource(t, c.src)
			ep := findEntryPoint(t, m, c.entry)

			// The table's shape follows the header's row counts.
			sigs := buildGraphicsPSVSigs(m, ep)
			types, err := typifier.Function(m, &ep.Function)
			if err != nil {
				t.Fatal(err)
			}
			table := buildInputToOutputTable(m, ep, types, sigs)
			maskDwords := (uint32(sigs.SigOutputVectorsStream0) + 7) >> 3
			want := maskDwords * uint32(sigs.SigInputVectors) * 4
			if uint32(len(table)) != want {
				t.Errorf("table: %d dwords, want %d (%d input rows, %d output rows)",
					len(table), want, sigs.SigInputVectors, sigs.SigOutputVectorsStream0)
			}

			opts := DefaultOptions()
			opts.EntryPoint = &backend.EntryPointSelector{Stage: ep.Stage, Name: ep.Name}
			blob, _, err := Compile(m, opts)
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			if err := Validate(blob, ValidateFull); err != nil {
				t.Fatalf("Validate: %v", err)
			}
		})
	}
}

func findEntryPoint(t *testing.T, m *ir.Module, name string) *ir.EntryPoint {
	t.Helper()
	for i := range m.EntryPoints {
		if m.EntryPoints[i].Name == name {
			return &m.EntryPoints[i]
		}
	}
	t.Fatalf("no entry point %q", name)
	return nil
}
