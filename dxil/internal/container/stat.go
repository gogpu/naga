// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package container

import "encoding/binary"

// FourCCSTAT is the DXBC container part tag for the "Shader Statistics"
// part. DXC calls this DFCC_ShaderStatistics. It is consumed by:
//
//   - the D3D12 runtime's CreateGraphicsPipelineState format validator
//     (empirically, as of 2026 the runtime rejects graphics containers
//     without STAT with STATE_CREATION error id 67 "Vertex Shader is
//     corrupt or in an unrecognized format" — even though the same
//     container passes IDxcValidator with S_OK)
//   - ID3D12ShaderReflection API consumers (tooling path, not the
//     pipeline creation path)
//
// IDxcValidator does NOT require STAT; only the runtime does. See
// BUG-DXIL-011 for the empirical proof (2026-04-14 session). Mesa's
// dxil_container.h defines the same FourCC but does not emit the
// part — Mesa-driven Vulkan-on-D3D12 (dzn/dozen) is almost entirely
// compute and slips under the runtime's format check. Our target is
// graphics pipelines, so STAT is mandatory.
var FourCCSTAT = fourCC('S', 'T', 'A', 'T')

// AddSTATPart emits a "Shader Statistics" part whose body wraps the
// same LLVM 3.7 bitcode as the main DXIL part under a DxilProgramHeader
// (the Option A shape from BUG-DXIL-011).
//
// DXC constructs STAT by cloning the LLVM module, stripping function
// bodies via StripAndCreateReflectionStream, and re-serializing. The
// resulting bitcode is ~96 bytes smaller than the main DXIL part for
// a trivial VS (function-body instructions removed) but structurally
// identical: same module header, same types, same metadata, same
// function declarations, and an empty function body list. D3D12
// runtime's reflection consumer does NOT execute the stripped
// bodies; it only reads the module-level metadata. Therefore a full
// (non-stripped) duplicate of the main DXIL bitcode is an accepted
// superset of what DXC emits:
//
//   - Same DxilProgramHeader wrapper (version, shader kind, bitcode
//     offset = 16, bitcode size).
//   - Same LLVM 3.7 bitcode (BC\xc0\xde ... module block).
//
// The only observable difference vs DXC is body presence inside
// function blocks. The runtime accepts this per experimental
// verification of the wgpu `GOGPU_DX12_DXIL_VALIDATE=1` hook.
//
// Option B (truncated reflection module with only !dx.version /
// !dx.shaderModel / !dx.entryPoints / empty function) is reserved
// for future compatibility if a future D3D12 runtime tightens the
// format check. As of validator 1.8 / AgilitySDK 1.615, Option A
// suffices.
//
// Reference:
//   - reference/dxil/dxc/lib/DxilContainer/DxilContainerAssembler.cpp
//     StripAndCreateReflectionStream (line 1873)
//     SerializeDxilContainerForModule STAT emission (line 2091)
//   - reference/dxil/mesa/src/microsoft/compiler/dxil_container.h:65
//     (DXIL_STAT fourCC definition; no emission site)
func (c *Container) AddSTATPart(shaderKind, majorVer, minorVer uint32, bitcodeData []byte) {
	// The DxilProgramHeader layout matches AddDXILPart exactly: 24
	// bytes of fixed header followed by the bitcode stream.
	version := (shaderKind << 16) | (majorVer << 4) | minorVer
	totalSize := 6*4 + uint32(len(bitcodeData)) //nolint:gosec // shader bytecode always fits in uint32
	wordSize := totalSize / 4

	var hdr [24]byte
	binary.LittleEndian.PutUint32(hdr[0:], version)
	binary.LittleEndian.PutUint32(hdr[4:], wordSize)
	binary.LittleEndian.PutUint32(hdr[8:], 0x4C495844)                // DXIL magic
	binary.LittleEndian.PutUint32(hdr[12:], uint32(0x100)|minorVer)   // DXIL version 1.N
	binary.LittleEndian.PutUint32(hdr[16:], 16)                       // bitcode offset
	binary.LittleEndian.PutUint32(hdr[20:], uint32(len(bitcodeData))) //nolint:gosec // same cap as wordSize

	data := make([]byte, len(hdr)+len(bitcodeData))
	copy(data, hdr[:])
	copy(data[len(hdr):], bitcodeData)

	c.parts = append(c.parts, part{fourCC: FourCCSTAT, data: data})
}
