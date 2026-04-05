// Package container implements the DXBC container format used to wrap
// DXIL shader bitcode.
//
// A DXBC container consists of a header followed by a series of parts,
// each identified by a FourCC code. The DXIL bitcode is stored in the
// DXIL part, and the container hash is stored in the HASH part.
//
// Reference implementation: Mesa's dxil_container.c
package container

import (
	"encoding/binary"
)

// FourCC codes for container parts.
var (
	FourCCDXBC = fourCC('D', 'X', 'B', 'C')
	FourCCDXIL = fourCC('D', 'X', 'I', 'L')
	FourCCSFI0 = fourCC('S', 'F', 'I', '0')
	FourCCHASH = fourCC('H', 'A', 'S', 'H')
	FourCCISG1 = fourCC('I', 'S', 'G', '1')
	FourCCOSG1 = fourCC('O', 'S', 'G', '1')
	FourCCPSV0 = fourCC('P', 'S', 'V', '0')
)

func fourCC(a, b, c, d byte) uint32 {
	return uint32(a) | uint32(b)<<8 | uint32(c)<<16 | uint32(d)<<24
}

// MaxParts is the maximum number of parts in a container.
const MaxParts = 8

// Container represents a DXBC container holding shader parts.
type Container struct {
	parts []part
}

type part struct {
	fourCC uint32
	data   []byte
}

// New creates an empty DXBC container.
func New() *Container {
	return &Container{}
}

// AddDXILPart adds the DXIL bitcode part to the container.
//
// The DXIL part has a program header before the bitcode:
//
//	ProgramVersion: (shaderKind << 16) | (majorVersion << 4) | minorVersion
//	ProgramSize:    total size in 32-bit words (header + bitcode)
//	DXILMagic:      0x4C495844 ("DXIL" in LE)
//	DXILVersion:    0x100 (DXIL bitcode version)
//	BitcodeOffset:  16 (bytes from DXIL magic to bitcode start)
//	BitcodeSize:    size of bitcode in bytes
func (c *Container) AddDXILPart(shaderKind uint32, majorVer, minorVer uint32, bitcodeData []byte) {
	// Program header: 6 uint32s = 24 bytes.
	version := (shaderKind << 16) | (majorVer << 4) | minorVer
	totalSize := 6*4 + uint32(len(bitcodeData)) //nolint:gosec // shader bytecode always fits in uint32
	wordSize := totalSize / 4

	var hdr [24]byte
	binary.LittleEndian.PutUint32(hdr[0:], version)
	binary.LittleEndian.PutUint32(hdr[4:], wordSize)
	binary.LittleEndian.PutUint32(hdr[8:], 0x4C495844)                // DXIL magic
	binary.LittleEndian.PutUint32(hdr[12:], 0x100)                    // DXIL version
	binary.LittleEndian.PutUint32(hdr[16:], 16)                       // bitcode offset
	binary.LittleEndian.PutUint32(hdr[20:], uint32(len(bitcodeData))) //nolint:gosec // same as totalSize check above

	data := make([]byte, len(hdr)+len(bitcodeData))
	copy(data, hdr[:])
	copy(data[len(hdr):], bitcodeData)

	c.parts = append(c.parts, part{fourCC: FourCCDXIL, data: data})
}

// AddFeaturesPart adds the SFI0 (Shader Feature Info) part.
// The features are encoded as a 64-bit bitmask.
func (c *Container) AddFeaturesPart(features uint64) {
	var data [8]byte
	binary.LittleEndian.PutUint64(data[:], features)
	c.parts = append(c.parts, part{fourCC: FourCCSFI0, data: data[:]})
}

// AddHashPart adds the HASH part with the BYPASS sentinel.
func (c *Container) AddHashPart() {
	// BYPASS sentinel: 01010101...01010101 (16 bytes).
	data := make([]byte, 16)
	for i := range data {
		data[i] = 0x01
	}
	c.parts = append(c.parts, part{fourCC: FourCCHASH, data: data})
}

// AddRawPart adds an arbitrary part with the given FourCC and data.
func (c *Container) AddRawPart(fc uint32, data []byte) {
	c.parts = append(c.parts, part{fourCC: fc, data: data})
}

// Bytes serializes the container to the DXBC binary format.
//
// Layout:
//
//	[Header: 28 bytes]
//	  Magic:      "DXBC" (0x44584243)
//	  Digest:     16 bytes (zeros = unsigned, or BYPASS sentinel)
//	  Version:    uint16 major=1, uint16 minor=0
//	  FileSize:   uint32 total size
//	  PartCount:  uint32
//
//	[PartOffsets: 4 * PartCount bytes]
//	  uint32 offsets from start of file to each part
//
//	[Parts:]
//	  For each part:
//	    FourCC:   uint32
//	    PartSize: uint32
//	    Data:     PartSize bytes
func (c *Container) Bytes() []byte {
	numParts := len(c.parts)
	headerSize := 32 + 4*numParts // header (32) + part offset table

	// Calculate total size.
	totalSize := headerSize
	for _, p := range c.parts {
		totalSize += 8 + len(p.data) // fourCC(4) + size(4) + data
	}

	out := make([]byte, totalSize)
	pos := 0

	// Offset 0: Magic "DXBC"
	binary.LittleEndian.PutUint32(out[pos:], FourCCDXBC)
	pos += 4

	// Offset 4: Digest (16 zero bytes = unsigned).
	pos += 16

	// Offset 20: Version major=1, minor=0.
	binary.LittleEndian.PutUint16(out[pos:], 1)
	pos += 2
	binary.LittleEndian.PutUint16(out[pos:], 0)
	pos += 2

	// Offset 24: Container total size.
	binary.LittleEndian.PutUint32(out[pos:], uint32(totalSize)) //nolint:gosec // container < 4GB
	pos += 4

	// Offset 28: Part count.
	binary.LittleEndian.PutUint32(out[pos:], uint32(numParts)) //nolint:gosec // max 8 parts
	pos += 4

	// Offset 32: Part offset table (4 bytes per part).
	offsetTableStart := pos
	pos += 4 * numParts

	// Parts data follows the offset table.
	for i, p := range c.parts {
		// Record the offset of this part (from file start).
		binary.LittleEndian.PutUint32(out[offsetTableStart+4*i:], uint32(pos)) //nolint:gosec // pos < totalSize < 4GB

		// Part header: FourCC + size.
		binary.LittleEndian.PutUint32(out[pos:], p.fourCC)
		pos += 4
		binary.LittleEndian.PutUint32(out[pos:], uint32(len(p.data))) //nolint:gosec // part data < 4GB
		pos += 4

		// Part data.
		copy(out[pos:], p.data)
		pos += len(p.data)
	}

	return out
}
