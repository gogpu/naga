// container.go — DXBC container + DxilProgramHeader extraction.
//
// Input: a raw DXBC shader container (what the user feeds to dxilval).
// Output: a slice pointing at the LLVM 3.7 bitstream body — starting at
// the 'BC\xC0\xDE' magic — inside the DXIL or ILDB part.
//
// Layout mirrors dxil/internal/container/container.go:
//
//	DXBC header (32 B):
//	  magic "DXBC"        : 4 B
//	  digest              : 16 B
//	  major / minor       : 4 B (2+2)
//	  totalFileSize       : 4 B
//	  partCount           : 4 B
//
//	Part offset table:
//	  partOffset[i]       : 4 B × partCount
//
//	Part header at each offset:
//	  fourCC              : 4 B
//	  partSize            : 4 B
//	  partData            : partSize B
//
// DXIL part data layout (DxilProgramHeader then bitcode):
//
//	programVersion      : 4 B  (shaderKind << 16 | major<<4 | minor)
//	programSize_words   : 4 B
//	dxilMagic "DXIL"    : 4 B
//	dxilVersion         : 4 B
//	bitcodeOffset       : 4 B  (from start of dxilMagic, usually 16)
//	bitcodeSize         : 4 B
//	bitcode[bitcodeSize]: starts with 'B','C',0xC0,0xDE

package bitcheck

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// ErrNoBitcode — the container does not contain a usable DXIL/ILDB
// part, or the DXIL part wrapper is malformed, or the bitcode body
// does not carry the LLVM bitstream magic.
var ErrNoBitcode = errors.New("bitcheck: no LLVM bitcode in DXIL part")

// DXBC / DXIL fourCC codes mirroring internal/dxcvalidator/precheck.go.
const (
	ccDXBC uint32 = 'D' | 'X'<<8 | 'B'<<16 | 'C'<<24
	ccDXIL uint32 = 'D' | 'X'<<8 | 'I'<<16 | 'L'<<24
	ccILDB uint32 = 'I' | 'L'<<8 | 'D'<<16 | 'B'<<24
)

const (
	dxbcHdrSize  = 32
	partHdrSize  = 8
	dxbcMaxParts = 64
	// DxilProgramHeader is 24 bytes; bitcodeOffset is relative to the
	// dxilMagic field at program-header offset 8. Default is 16 → the
	// bitcode body begins 8+16=24 bytes into the part data.
	dxilProgHdrSize   = 24
	dxilMagicOffset   = 8
	dxilBCOffsetField = 16
	dxilBCSizeField   = 20
)

// extractBitcode walks the DXBC container, finds the DXIL (or ILDB)
// part, unwraps the DxilProgramHeader, and returns a slice pointing at
// the LLVM 3.7 bitstream body starting with the 'BC\xC0\xDE' magic.
// Returns ErrNoBitcode on any structural problem. Never mutates the
// input slice.
func extractBitcode(blob []byte) ([]byte, error) {
	if len(blob) < dxbcHdrSize {
		return nil, fmt.Errorf("container %d < %d: %w",
			len(blob), dxbcHdrSize, ErrNoBitcode)
	}
	if binary.LittleEndian.Uint32(blob[0:4]) != ccDXBC {
		return nil, fmt.Errorf("missing DXBC magic: %w", ErrNoBitcode)
	}
	partCount := binary.LittleEndian.Uint32(blob[28:32])
	if partCount == 0 || partCount > dxbcMaxParts {
		return nil, fmt.Errorf("part count %d: %w", partCount, ErrNoBitcode)
	}
	offsetTableEnd := uint64(dxbcHdrSize) + 4*uint64(partCount)
	if offsetTableEnd > uint64(len(blob)) {
		return nil, fmt.Errorf("part offset table walks past blob: %w", ErrNoBitcode)
	}
	for i := uint32(0); i < partCount; i++ {
		offPos := uint64(dxbcHdrSize) + 4*uint64(i)
		partOff := uint64(binary.LittleEndian.Uint32(blob[offPos : offPos+4]))
		if partOff+partHdrSize > uint64(len(blob)) {
			return nil, fmt.Errorf("part[%d] header past blob: %w", i, ErrNoBitcode)
		}
		fourCC := binary.LittleEndian.Uint32(blob[partOff : partOff+4])
		partSize := uint64(binary.LittleEndian.Uint32(blob[partOff+4 : partOff+8]))
		dataStart := partOff + partHdrSize
		dataEnd := dataStart + partSize
		if dataEnd > uint64(len(blob)) {
			return nil, fmt.Errorf("part[%d] data past blob: %w", i, ErrNoBitcode)
		}
		if fourCC != ccDXIL && fourCC != ccILDB {
			continue
		}
		return unwrapDxilProgramHeader(blob[dataStart:dataEnd])
	}
	return nil, fmt.Errorf("no DXIL/ILDB part: %w", ErrNoBitcode)
}

// unwrapDxilProgramHeader verifies the 24-byte DxilProgramHeader and
// returns a slice pointing at the bitcode body.
func unwrapDxilProgramHeader(part []byte) ([]byte, error) {
	if len(part) < dxilProgHdrSize {
		return nil, fmt.Errorf("DXIL part %d < %d: %w",
			len(part), dxilProgHdrSize, ErrNoBitcode)
	}
	// DxilMagic at offset 8 must equal "DXIL" little-endian.
	if binary.LittleEndian.Uint32(part[dxilMagicOffset:dxilMagicOffset+4]) != ccDXIL {
		return nil, fmt.Errorf("missing DxilProgramHeader magic: %w", ErrNoBitcode)
	}
	// bitcodeOffset is from the start of dxilMagic.
	bcOff := uint64(binary.LittleEndian.Uint32(part[dxilBCOffsetField : dxilBCOffsetField+4]))
	bcSize := uint64(binary.LittleEndian.Uint32(part[dxilBCSizeField : dxilBCSizeField+4]))
	start := uint64(dxilMagicOffset) + bcOff
	end := start + bcSize
	if start > uint64(len(part)) || end > uint64(len(part)) {
		return nil, fmt.Errorf("bitcode range [%d,%d) past DXIL part %d: %w",
			start, end, len(part), ErrNoBitcode)
	}
	body := part[start:end]
	// Verify the LLVM bitstream magic 'BC\xC0\xDE'.
	if len(body) < 4 || body[0] != 'B' || body[1] != 'C' ||
		body[2] != 0xC0 || body[3] != 0xDE {
		return nil, fmt.Errorf("missing LLVM bitstream magic: %w", ErrNoBitcode)
	}
	return body, nil
}
