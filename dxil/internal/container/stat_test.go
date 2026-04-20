// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package container

import (
	"encoding/binary"
	"testing"
)

// TestAddSTATPart pins the DXBC STAT part layout:
//
//   - FourCC "STAT" (DFCC_ShaderStatistics)
//   - 24-byte DxilProgramHeader wrapper identical to the main DXIL part
//     (program version, word size, DXIL magic, DXIL version, bitcode
//     offset=16, bitcode size)
//   - raw bitcode body (Option A: duplicate of the main DXIL bitcode)
//
// Required by the D3D12 runtime format validator for graphics
// pipelines (BUG-DXIL-011). IDxcValidator accepts containers without
// STAT (returns S_OK), but D3D12 CreateGraphicsPipelineState rejects
// them with STATE_CREATION error id 67 / 93. DXC always emits STAT;
// we mirror that.
func TestAddSTATPart(t *testing.T) {
	c := New()
	bitcode := []byte{0x42, 0x43, 0xC0, 0xDE, 0, 0, 0, 0} // minimal bitcode
	c.AddSTATPart(1, 6, 0, bitcode)
	data := c.Bytes()

	partCount := binary.LittleEndian.Uint32(data[28:32])
	if partCount != 1 {
		t.Fatalf("part count: got %d, want 1", partCount)
	}

	offset := binary.LittleEndian.Uint32(data[32:36])
	fc := binary.LittleEndian.Uint32(data[offset:])
	if fc != FourCCSTAT {
		t.Errorf("part FourCC: got 0x%08X, want STAT 0x%08X", fc, FourCCSTAT)
	}

	// Part size = program header (24) + bitcode (8) = 32.
	partSize := binary.LittleEndian.Uint32(data[offset+4:])
	if partSize != 32 {
		t.Errorf("STAT part size: got %d, want 32", partSize)
	}

	// Program header: VS (kind=1) SM 6.0 → (1<<16) | (6<<4) | 0 = 0x10060
	hdrOff := int(offset) + 8
	version := binary.LittleEndian.Uint32(data[hdrOff:])
	wantVer := uint32(1<<16 | 6<<4)
	if version != wantVer {
		t.Errorf("program version: got 0x%08X, want 0x%08X", version, wantVer)
	}

	dxilMagic := binary.LittleEndian.Uint32(data[hdrOff+8:])
	if dxilMagic != 0x4C495844 {
		t.Errorf("DXIL magic: got 0x%08X, want 0x4C495844", dxilMagic)
	}

	// Bitcode offset in the header must be 16 (immediately after the
	// program header's first 4 uint32s + the magic).
	bcOff := binary.LittleEndian.Uint32(data[hdrOff+16:])
	if bcOff != 16 {
		t.Errorf("bitcode offset: got %d, want 16", bcOff)
	}

	bcSize := binary.LittleEndian.Uint32(data[hdrOff+20:])
	if bcSize != uint32(len(bitcode)) {
		t.Errorf("bitcode size: got %d, want %d", bcSize, len(bitcode))
	}
}

// TestContainerPartOrder_STATBeforeHASH verifies STAT precedes HASH in
// the canonical DXC part order (SFI0, ISG1, OSG1, PSV0, STAT, HASH,
// DXIL). The order matters because the BYPASS / retail hash covers the
// entire container including STAT — inserting STAT after HASH would
// either leave it unhashed or require re-hashing.
func TestContainerPartOrder_STATBeforeHASH(t *testing.T) {
	c := New()
	c.AddFeaturesPart(0)
	c.AddInputSignature(nil)
	c.AddOutputSignature(nil)
	c.AddPSV0(PSVInfo{ShaderStage: 1})
	c.AddSTATPart(1, 6, 0, []byte{0x42, 0x43, 0xC0, 0xDE, 0, 0, 0, 0})
	c.AddHashPart()
	c.AddDXILPart(1, 6, 0, []byte{0x42, 0x43, 0xC0, 0xDE, 0, 0, 0, 0})

	data := c.Bytes()
	partCount := binary.LittleEndian.Uint32(data[28:32])
	if partCount != 7 {
		t.Fatalf("part count: got %d, want 7", partCount)
	}

	expect := []uint32{
		FourCCSFI0,
		FourCCISG1,
		FourCCOSG1,
		FourCCPSV0,
		FourCCSTAT,
		FourCCHASH,
		FourCCDXIL,
	}
	for i, want := range expect {
		off := binary.LittleEndian.Uint32(data[32+i*4:])
		got := binary.LittleEndian.Uint32(data[off:])
		if got != want {
			t.Errorf("part[%d]: got 0x%08X, want 0x%08X", i, got, want)
		}
	}

	// Additional invariant: STAT must appear BEFORE HASH in the offset
	// list, so the slot index of STAT is 4 and HASH is 5. If future
	// maintenance reorders the part list, the validator's hash will
	// no longer cover STAT and D3D12 will reject the pipeline.
	statOff := binary.LittleEndian.Uint32(data[32+4*4:])
	hashOff := binary.LittleEndian.Uint32(data[32+5*4:])
	if statOff >= hashOff {
		t.Errorf("STAT offset (0x%x) must precede HASH offset (0x%x)", statOff, hashOff)
	}
}
