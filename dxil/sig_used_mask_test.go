// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/dxil/internal/container"
)

// TestPackedInputUsedMaskShiftedToItsColumns (P1-DXIL-095): ISG1's
// AlwaysReads mask names register lanes, so a member packed at StartCol 2
// and read in its .x and .y has the mask zw (0xC), as
// DxilProgramSignatureWriter shifts the usage mask by the element's start
// column (DxilContainerAssembler.cpp:311-317). We masked the unshifted usage
// (xy) with the shifted declared mask (zw) and wrote 0, so the validator,
// which rebuilds the part from the loadInput calls, rejected the input
// signature (f16-native and f16-polyfill test_component_access).
func TestPackedInputUsedMaskShiftedToItsColumns(t *testing.T) {
	const src = `
struct IO {
    @location(0) a: f32,
    @location(1) b: f32,
    @location(2) v: vec2<f32>,
    @location(3) w: vec2<f32>,
}
@fragment
fn main(input: IO) -> IO {
    var output: IO;
    output.v.x = input.v.y;
    output.v.y = input.v.x;
    return output;
}
`
	m := lowerSource(t, src)
	inputs, _ := buildSignatures(m, &m.EntryPoints[0], true)
	var found bool
	for _, e := range inputs {
		if e.SemanticIndex != 2 {
			continue
		}
		found = true
		if e.Mask != 0xC || e.RWMask != 0xC {
			t.Errorf("LOC2: Mask %#x AlwaysReads %#x, want 0xc and 0xc", e.Mask, e.RWMask)
		}
	}
	if !found {
		t.Fatal("no LOC2 input")
	}
	compileValidateEach(t, src)
}

// f16IO mixes 16-bit and 32-bit inputs, as f16-native's F16IO does.
const f16IO = `
enable f16;
struct F16IO {
    @location(0) scalar_f16: f16,
    @location(1) scalar_f32: f32,
    @location(2) vec2_f16: vec2<f16>,
    @location(3) vec2_f32: vec2<f32>,
}
@fragment
fn main(input: F16IO) -> F16IO {
    var output: F16IO;
    output.vec2_f16.x = input.vec2_f16.y;
    output.vec2_f16.y = input.vec2_f16.x;
    output.scalar_f32 = input.scalar_f32;
    return output;
}
`

// TestSixteenBitSignatureElements (P1-DXIL-095): a 16-bit element is
// declared with its 16-bit component type (fp16 in ISG1/OSG1 and PSV0, F16 in
// !dx.entryPoints), and 16-bit and 32-bit elements never share a row: DXC
// (-enable-16bit-types, golden f16-native test_component_access) puts LOC0
// at x and LOC2 at yz of row 0, LOC1 at x and LOC3 at yz of row 1
// (DxilSignatureAllocator: elements of a different data width conflict;
// CompTypeToSigCompType, DxilContainerAssembler.cpp:138-170). We declared
// every float element Float32 and packed them together, so the loads read
// f16 values out of an element declared 32-bit.
func TestSixteenBitSignatureElements(t *testing.T) {
	m := lowerSource(t, f16IO)
	ep := &m.EntryPoints[0]
	inputs, outputs := buildSignatures(m, ep, true)
	type want struct {
		ct   container.ProgSigCompType
		reg  uint32
		mask uint8
	}
	wantIn := map[uint32]want{
		0: {container.CompTypeFloat16, 0, 0x1},
		2: {container.CompTypeFloat16, 0, 0x6},
		1: {container.CompTypeFloat32, 1, 0x1},
		3: {container.CompTypeFloat32, 1, 0x6},
	}
	for _, e := range inputs {
		w := wantIn[e.SemanticIndex]
		if e.CompType != w.ct || e.Register != w.reg || e.Mask != w.mask {
			t.Errorf("input LOC%d: comp %d row %d mask %#x, want comp %d row %d mask %#x",
				e.SemanticIndex, e.CompType, e.Register, e.Mask, w.ct, w.reg, w.mask)
		}
	}
	for _, e := range outputs {
		wantCT := container.CompTypeFloat32
		if e.SemanticIndex == 0 || e.SemanticIndex == 2 {
			wantCT = container.CompTypeFloat16
		}
		if e.CompType != wantCT {
			t.Errorf("output SV_Target%d: comp %d, want %d", e.SemanticIndex, e.CompType, wantCT)
		}
	}
	sigs := buildGraphicsPSVSigs(m, ep)
	for _, list := range [][]container.PSVSignatureElement{sigs.PSVInputs, sigs.PSVOutputs} {
		for _, e := range list {
			if e.Rows == 1 && e.ColsAndStart&0x0F != 0 && e.ComponentType != uint8(container.CompTypeFloat16) && e.ComponentType != uint8(container.CompTypeFloat32) {
				t.Errorf("PSV0 element: ComponentType %d", e.ComponentType)
			}
		}
	}
	opts := DefaultOptions()
	opts.ShaderModel = backend.ShaderModel{Major: 6, Minor: 2}
	blob, _, err := Compile(m, opts)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if err := Validate(blob, ValidateFull); err != nil {
		t.Errorf("Validate: %v", err)
	}
}
