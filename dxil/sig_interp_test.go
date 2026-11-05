// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"os"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/dxil/internal/container"
)

// fragmentSystemValuesFS reads every fragment system value that sits in the
// input signature: SV_Position, SV_IsFrontFace, SV_PrimitiveID and
// SV_SampleIndex (the last one unallocated).
const fragmentSystemValuesFS = `
enable primitive_index;

@fragment
fn main(
    @builtin(position) pos: vec4<f32>,
    @builtin(front_facing) ff: bool,
    @builtin(primitive_index) prim: u32,
    @builtin(sample_index) si: u32,
) -> @location(0) vec4<f32> {
    return vec4(pos.x, select(0.0, 1.0, ff), f32(prim), f32(si));
}
`

// PSV0 semantic kinds (DxilConstants.h SemanticKind) used below.
const (
	psvKindPosition    = 3
	psvKindPrimitiveID = 10
	psvKindSampleIndex = 12
	psvKindIsFrontFace = 13
	psvKindCoverage    = 14
	psvKindDepth       = 17
)

// TestFragmentSystemValueInterpolation (P1-DXIL-083): a fragment input
// system value that is not a float gets InterpolationMode Constant
// (nointerpolation), as DXC assigns it. HLSignatureLower.cpp:266-272 gives a
// signature point that needs an interpolation mode (PackingKind::Vertex:
// vertex output, pixel input) Linear for float elements and Constant for
// everything else, and UpdateSemanticAndInterpMode (:49-60) turns
// SV_Position's Linear into LinearNoperspective. The DXC golden for
// interface/fragment shows SV_IsFrontFace and SV_SampleIndex as
// nointerpolation. We wrote Undefined for every system value except
// SV_Position and SV_ClipDistance, in PSV0 and in !dx.entryPoints alike,
// so the validator saw no mismatch while Intel read a wrong
// SV_PrimitiveID for the second triangle.
func TestFragmentSystemValueInterpolation(t *testing.T) {
	m := lowerSource(t, fragmentSystemValuesFS)
	ep := findEntryPoint(t, m, "main")

	want := map[uint8]uint8{
		psvKindPosition:    4, // LinearNoperspective
		psvKindIsFrontFace: 1, // Constant
		psvKindPrimitiveID: 1,
		psvKindSampleIndex: 1,
	}
	sigs := buildGraphicsPSVSigs(m, ep)
	seen := 0
	for _, e := range sigs.PSVInputs {
		w, ok := want[e.SemanticKind]
		if !ok {
			continue
		}
		seen++
		if e.InterpolationMode != w {
			t.Errorf("PSV0 input kind %d: InterpolationMode %d, want %d", e.SemanticKind, e.InterpolationMode, w)
		}
	}
	if seen != len(want) {
		t.Errorf("PSV0 inputs: found %d of the %d system values", seen, len(want))
	}

	// The module's !dx.entryPoints side has to agree with PSV0, or the
	// validator rejects the container.
	opts := DefaultOptions()
	opts.EntryPoint = &backend.EntryPointSelector{Stage: ep.Stage, Name: ep.Name}
	blob, _, err := Compile(m, opts)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if err := Validate(blob, ValidateFull); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

// TestUnallocatedSignatureElementsValidate (P1-DXIL-083): system values that
// get no register (SV_Depth, SV_Coverage on output, SV_SampleIndex on input)
// carry StartRow 0 and a clear allocated bit in PSV0, which is what
// InitPSVSignatureElement writes (DxilPipelineStateValidation.cpp:92-112: the
// element is zeroed and StartRow is set only when the element is allocated);
// the validator rebuilds the element that way and compares the bytes
// (DxilContainerValidation.cpp:307-350). OSG1 names SV_Coverage with
// D3D_NAME_COVERAGE. We wrote StartRow 0xFF, and SV_Coverage as an
// arbitrary "SV_Unknown" element, so both shaders were rejected.
func TestUnallocatedSignatureElementsValidate(t *testing.T) {
	cases := []struct{ file, entry string }{
		{"early-depth-test-conservative.wgsl", "main"},
		{"interface.wgsl", "fragment"},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			src, err := os.ReadFile("../snapshot/testdata/rust/in/wgsl/" + c.file)
			if err != nil {
				t.Fatal(err)
			}
			m := lowerSource(t, string(src))
			ep := findEntryPoint(t, m, c.entry)

			sigs := buildGraphicsPSVSigs(m, ep)
			for _, list := range [][]container.PSVSignatureElement{sigs.PSVInputs, sigs.PSVOutputs} {
				for _, e := range list {
					switch e.SemanticKind {
					case psvKindDepth, psvKindCoverage, psvKindSampleIndex:
						if e.StartRow != 0 || e.ColsAndStart&0x40 != 0 {
							t.Errorf("PSV0 kind %d: StartRow %d ColsAndStart %#x, want 0 and no allocated bit",
								e.SemanticKind, e.StartRow, e.ColsAndStart)
						}
					}
				}
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
