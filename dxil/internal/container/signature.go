// Package container — signature encoding for ISG1/OSG1 parts.
//
// Each signature part has the layout:
//
//	Header:
//	  ParamCount  uint32   // total number of signature elements
//	  ParamOffset uint32   // byte offset from part start to first element (always 8)
//	Elements:
//	  [ParamCount]SignatureElement  // each 32 bytes
//	StringTable:
//	  null-terminated semantic name strings, 4-byte aligned
//
// Reference: Mesa dxil_container.c dxil_container_add_io_signature(),
// Mesa dxil_signature.h struct dxil_signature_element.
package container

import (
	"encoding/binary"
)

// SystemValueKind identifies DXIL system-value semantics.
// Values match Mesa's enum dxil_semantic_kind.
type SystemValueKind uint32

const (
	SVArbitrary    SystemValueKind = 0  // User-defined (TEXCOORD, COLOR, etc.)
	SVVertexID     SystemValueKind = 1  // SV_VertexID
	SVInstanceID   SystemValueKind = 2  // SV_InstanceID
	SVPosition     SystemValueKind = 3  // SV_Position
	SVTarget       SystemValueKind = 16 // SV_Target
	SVDepth        SystemValueKind = 17 // SV_Depth
	SVSampleIndex  SystemValueKind = 12 // SV_SampleIndex
	SVIsFrontFace  SystemValueKind = 13 // SV_IsFrontFace
	SVClipDistance SystemValueKind = 6  // SV_ClipDistance
)

// ProgSigCompType identifies the component data type in a signature element.
// Values match Mesa's enum dxil_prog_sig_comp_type.
type ProgSigCompType uint32

const (
	CompTypeUnknown ProgSigCompType = 0
	CompTypeUint32  ProgSigCompType = 1
	CompTypeSint32  ProgSigCompType = 2
	CompTypeFloat32 ProgSigCompType = 3
)

// SignatureElement describes one element of an input or output signature.
type SignatureElement struct {
	SemanticName  string
	SemanticIndex uint32
	SystemValue   SystemValueKind
	CompType      ProgSigCompType
	Register      uint32
	Mask          uint8 // Component mask: x=1, y=2, z=4, w=8
	RWMask        uint8 // NeverWritesMask (output) or AlwaysReadsMask (input)
}

// signatureElementSize is the binary size of one dxil_signature_element.
// Layout (32 bytes total):
//
//	stream:               uint32 (offset 0)
//	semantic_name_offset: uint32 (offset 4)
//	semantic_index:       uint32 (offset 8)
//	system_value:         uint32 (offset 12)
//	comp_type:            uint32 (offset 16)
//	register:             uint32 (offset 20)
//	mask:                 uint8  (offset 24)
//	rw_mask:              uint8  (offset 25)
//	pad:                  uint16 (offset 26)
//	min_precision:        uint32 (offset 28)
const signatureElementSize = 32

// EncodeSignature serializes a list of SignatureElements into the ISG1/OSG1
// binary format. The result is the part data (not including the FourCC/size
// part envelope).
func EncodeSignature(elements []SignatureElement) []byte {
	if len(elements) == 0 {
		// Empty signature: header only.
		var hdr [8]byte
		// paramCount = 0, paramOffset = 8
		binary.LittleEndian.PutUint32(hdr[4:], 8)
		return hdr[:]
	}

	// Build string table with deduplication for SV_ names.
	headerSize := 8 // paramCount + paramOffset
	fixedSize := headerSize + signatureElementSize*len(elements)

	nameOffsets := make([]uint32, len(elements))
	svCache := make(map[string]uint32) // deduplicate SV_ names
	var stringTable []byte

	for i, elem := range elements {
		name := elem.SemanticName
		if len(name) >= 3 && name[:3] == "SV_" {
			if off, ok := svCache[name]; ok {
				nameOffsets[i] = off
				continue
			}
		}
		off := uint32(fixedSize + len(stringTable)) //nolint:gosec // bounded by signature size
		nameOffsets[i] = off
		if len(name) >= 3 && name[:3] == "SV_" {
			svCache[name] = off
		}
		stringTable = append(stringTable, []byte(name)...)
		stringTable = append(stringTable, 0) // null terminator
	}

	// 4-byte align the string table.
	for len(stringTable)%4 != 0 {
		stringTable = append(stringTable, 0)
	}

	totalSize := fixedSize + len(stringTable)
	out := make([]byte, totalSize)

	// Header.
	binary.LittleEndian.PutUint32(out[0:], uint32(len(elements))) //nolint:gosec // bounded
	binary.LittleEndian.PutUint32(out[4:], uint32(headerSize))

	// Elements.
	for i, elem := range elements {
		base := headerSize + i*signatureElementSize
		// stream = 0
		binary.LittleEndian.PutUint32(out[base+0:], 0)
		binary.LittleEndian.PutUint32(out[base+4:], nameOffsets[i])
		binary.LittleEndian.PutUint32(out[base+8:], elem.SemanticIndex)
		binary.LittleEndian.PutUint32(out[base+12:], uint32(elem.SystemValue))
		binary.LittleEndian.PutUint32(out[base+16:], uint32(elem.CompType))
		binary.LittleEndian.PutUint32(out[base+20:], elem.Register)
		out[base+24] = elem.Mask
		out[base+25] = elem.RWMask
		// pad (uint16) at offset 26 = 0
		// min_precision (uint32) at offset 28 = 0
	}

	// String table.
	copy(out[fixedSize:], stringTable)

	return out
}

// AddInputSignature adds an ISG1 part to the container.
func (c *Container) AddInputSignature(elements []SignatureElement) {
	data := EncodeSignature(elements)
	c.parts = append(c.parts, part{fourCC: FourCCISG1, data: data})
}

// AddOutputSignature adds an OSG1 part to the container.
func (c *Container) AddOutputSignature(elements []SignatureElement) {
	data := EncodeSignature(elements)
	c.parts = append(c.parts, part{fourCC: FourCCOSG1, data: data})
}
