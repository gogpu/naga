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

// D3D_NAME values for ISG1/OSG1 signature elements.
// These match the D3D_NAME enumeration from d3dcommon.h, NOT the DXIL semantic kind.
// Reference: D3D12 SDK d3dcommon.h enum D3D_NAME
const (
	SVArbitrary         SystemValueKind = 0  // D3D_NAME_UNDEFINED — user-defined (TEXCOORD, etc.)
	SVPosition          SystemValueKind = 1  // D3D_NAME_POSITION — SV_Position
	SVClipDistance      SystemValueKind = 2  // D3D_NAME_CLIP_DISTANCE — SV_ClipDistance
	SVCullDistance      SystemValueKind = 3  // D3D_NAME_CULL_DISTANCE — SV_CullDistance
	SVVertexID          SystemValueKind = 6  // D3D_NAME_VERTEX_ID — SV_VertexID
	SVPrimitiveID       SystemValueKind = 7  // D3D_NAME_PRIMITIVE_ID — SV_PrimitiveID
	SVInstanceID        SystemValueKind = 8  // D3D_NAME_INSTANCE_ID — SV_InstanceID
	SVIsFrontFace       SystemValueKind = 9  // D3D_NAME_IS_FRONT_FACE — SV_IsFrontFace
	SVSampleIndex       SystemValueKind = 10 // D3D_NAME_SAMPLE_INDEX — SV_SampleIndex
	SVTarget            SystemValueKind = 64 // D3D_NAME_TARGET — SV_Target
	SVDepth             SystemValueKind = 65 // D3D_NAME_DEPTH — SV_Depth
	SVCoverage          SystemValueKind = 66 // D3D_NAME_COVERAGE — SV_Coverage
	SVDepthGreaterEqual SystemValueKind = 67 // D3D_NAME_DEPTH_GREATER_EQUAL — SV_DepthGreaterEqual
	SVDepthLessEqual    SystemValueKind = 68 // D3D_NAME_DEPTH_LESS_EQUAL — SV_DepthLessEqual
	SVStencilRef        SystemValueKind = 69 // D3D_NAME_STENCIL_REF — SV_StencilRef
	SVCullPrimitive     SystemValueKind = 24 // SV_CullPrimitive (mesh shader)
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

	// Build string table with full deduplication. dxc reuses the same
	// string offset whenever two signature elements share a semantic
	// name (e.g. multiple TEXCOORD elements all point to the same
	// "TEXCOORD\0" entry). Without this, our ISG1 disagrees with
	// what NewProgramSignatureWriter regenerates from the bitcode
	// signature, and dxil.dll reports
	// "Container part 'Program Input/Output Signature' does not match
	// expected for module".
	headerSize := 8 // paramCount + paramOffset
	fixedSize := headerSize + signatureElementSize*len(elements)

	nameOffsets := make([]uint32, len(elements))
	nameCache := make(map[string]uint32)
	var stringTable []byte

	for i, elem := range elements {
		name := elem.SemanticName
		if off, ok := nameCache[name]; ok {
			nameOffsets[i] = off
			continue
		}
		off := uint32(fixedSize + len(stringTable)) //nolint:gosec // bounded by signature size
		nameOffsets[i] = off
		nameCache[name] = off
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

// AddPrimitiveSignature adds a PSG1 part to the container.
// PSG1 is used for mesh shader primitive output signatures.
func (c *Container) AddPrimitiveSignature(elements []SignatureElement) {
	data := EncodeSignature(elements)
	c.parts = append(c.parts, part{fourCC: FourCCPSG1, data: data})
}
