package container

import (
	"encoding/binary"
	"testing"
)

func TestEncodeSignature_Empty(t *testing.T) {
	data := EncodeSignature(nil)
	if len(data) != 8 {
		t.Fatalf("empty signature size: got %d, want 8", len(data))
	}
	paramCount := binary.LittleEndian.Uint32(data[0:4])
	if paramCount != 0 {
		t.Errorf("paramCount: got %d, want 0", paramCount)
	}
	paramOffset := binary.LittleEndian.Uint32(data[4:8])
	if paramOffset != 8 {
		t.Errorf("paramOffset: got %d, want 8", paramOffset)
	}
}

func TestEncodeInputSignature(t *testing.T) {
	// Vertex shader with 2 inputs: POSITION (float4) + TEXCOORD0 (float2).
	elements := []SignatureElement{
		{
			SemanticName:  "POSITION",
			SemanticIndex: 0,
			SystemValue:   SVArbitrary,
			CompType:      CompTypeFloat32,
			Register:      0,
			Mask:          0x0F, // xyzw
			RWMask:        0x0F,
		},
		{
			SemanticName:  "TEXCOORD",
			SemanticIndex: 0,
			SystemValue:   SVArbitrary,
			CompType:      CompTypeFloat32,
			Register:      1,
			Mask:          0x03, // xy
			RWMask:        0x03,
		},
	}

	data := EncodeSignature(elements)

	// Header.
	paramCount := binary.LittleEndian.Uint32(data[0:4])
	if paramCount != 2 {
		t.Errorf("paramCount: got %d, want 2", paramCount)
	}
	paramOffset := binary.LittleEndian.Uint32(data[4:8])
	if paramOffset != 8 {
		t.Errorf("paramOffset: got %d, want 8", paramOffset)
	}

	// Element 0: POSITION.
	e0Base := 8
	stream := binary.LittleEndian.Uint32(data[e0Base:])
	if stream != 0 {
		t.Errorf("elem0 stream: got %d, want 0", stream)
	}
	semIdx := binary.LittleEndian.Uint32(data[e0Base+8:])
	if semIdx != 0 {
		t.Errorf("elem0 semantic_index: got %d, want 0", semIdx)
	}
	sysVal := binary.LittleEndian.Uint32(data[e0Base+12:])
	if sysVal != uint32(SVArbitrary) {
		t.Errorf("elem0 system_value: got %d, want %d", sysVal, SVArbitrary)
	}
	compType := binary.LittleEndian.Uint32(data[e0Base+16:])
	if compType != uint32(CompTypeFloat32) {
		t.Errorf("elem0 comp_type: got %d, want %d", compType, CompTypeFloat32)
	}
	reg := binary.LittleEndian.Uint32(data[e0Base+20:])
	if reg != 0 {
		t.Errorf("elem0 register: got %d, want 0", reg)
	}
	if data[e0Base+24] != 0x0F {
		t.Errorf("elem0 mask: got 0x%02X, want 0x0F", data[e0Base+24])
	}

	// Element 1: TEXCOORD at register 1.
	e1Base := 8 + signatureElementSize
	reg1 := binary.LittleEndian.Uint32(data[e1Base+20:])
	if reg1 != 1 {
		t.Errorf("elem1 register: got %d, want 1", reg1)
	}
	if data[e1Base+24] != 0x03 {
		t.Errorf("elem1 mask: got 0x%02X, want 0x03", data[e1Base+24])
	}

	// Verify semantic name offsets point to valid strings.
	fixedSize := 8 + signatureElementSize*2
	nameOff0 := binary.LittleEndian.Uint32(data[e0Base+4:])
	if int(nameOff0) < fixedSize || int(nameOff0) >= len(data) {
		t.Errorf("elem0 name offset %d out of range [%d, %d)", nameOff0, fixedSize, len(data))
	}

	// Read string at offset.
	name0 := readNullTermString(data, int(nameOff0))
	if name0 != "POSITION" {
		t.Errorf("elem0 semantic name: got %q, want %q", name0, "POSITION")
	}

	nameOff1 := binary.LittleEndian.Uint32(data[e1Base+4:])
	name1 := readNullTermString(data, int(nameOff1))
	if name1 != "TEXCOORD" {
		t.Errorf("elem1 semantic name: got %q, want %q", name1, "TEXCOORD")
	}

	// Verify total data is 4-byte aligned.
	if len(data)%4 != 0 {
		t.Errorf("signature data not 4-byte aligned: %d bytes", len(data))
	}
}

func TestEncodeOutputSignature(t *testing.T) {
	// Vertex shader output: SV_Position + TEXCOORD0.
	elements := []SignatureElement{
		{
			SemanticName:  "SV_Position",
			SemanticIndex: 0,
			SystemValue:   SVPosition,
			CompType:      CompTypeFloat32,
			Register:      0,
			Mask:          0x0F,
			RWMask:        0x00, // neverWritesMask = none written for outputs
		},
		{
			SemanticName:  "TEXCOORD",
			SemanticIndex: 0,
			SystemValue:   SVArbitrary,
			CompType:      CompTypeFloat32,
			Register:      1,
			Mask:          0x03,
			RWMask:        0x00,
		},
	}

	data := EncodeSignature(elements)

	// Check SV_Position has correct system value.
	e0Base := 8
	sysVal := binary.LittleEndian.Uint32(data[e0Base+12:])
	if sysVal != uint32(SVPosition) {
		t.Errorf("SV_Position system_value: got %d, want %d", sysVal, SVPosition)
	}
}

func TestFragmentSignatures(t *testing.T) {
	// Fragment shader output: SV_Target0 (float4).
	elements := []SignatureElement{
		{
			SemanticName:  "SV_Target",
			SemanticIndex: 0,
			SystemValue:   SVTarget,
			CompType:      CompTypeFloat32,
			Register:      0,
			Mask:          0x0F,
			RWMask:        0x00,
		},
	}

	data := EncodeSignature(elements)

	paramCount := binary.LittleEndian.Uint32(data[0:4])
	if paramCount != 1 {
		t.Errorf("paramCount: got %d, want 1", paramCount)
	}

	e0Base := 8
	sysVal := binary.LittleEndian.Uint32(data[e0Base+12:])
	if sysVal != uint32(SVTarget) {
		t.Errorf("SV_Target system_value: got %d, want %d", sysVal, SVTarget)
	}

	name := readNullTermString(data, int(binary.LittleEndian.Uint32(data[e0Base+4:])))
	if name != "SV_Target" {
		t.Errorf("semantic name: got %q, want %q", name, "SV_Target")
	}
}

func TestSVNameDeduplication(t *testing.T) {
	// Two SV_Target elements should share the same name offset.
	elements := []SignatureElement{
		{
			SemanticName:  "SV_Target",
			SemanticIndex: 0,
			SystemValue:   SVTarget,
			CompType:      CompTypeFloat32,
			Register:      0,
			Mask:          0x0F,
		},
		{
			SemanticName:  "SV_Target",
			SemanticIndex: 1,
			SystemValue:   SVTarget,
			CompType:      CompTypeFloat32,
			Register:      1,
			Mask:          0x0F,
		},
	}

	data := EncodeSignature(elements)

	e0Base := 8
	e1Base := 8 + signatureElementSize

	nameOff0 := binary.LittleEndian.Uint32(data[e0Base+4:])
	nameOff1 := binary.LittleEndian.Uint32(data[e1Base+4:])

	if nameOff0 != nameOff1 {
		t.Errorf("SV_ name offsets should be deduplicated: got %d and %d", nameOff0, nameOff1)
	}

	// Semantic indices should differ.
	si0 := binary.LittleEndian.Uint32(data[e0Base+8:])
	si1 := binary.LittleEndian.Uint32(data[e1Base+8:])
	if si0 != 0 || si1 != 1 {
		t.Errorf("semantic indices: got %d,%d want 0,1", si0, si1)
	}
}

func TestContainerWithSignatures(t *testing.T) {
	c := New()
	c.AddFeaturesPart(0)
	c.AddInputSignature([]SignatureElement{
		{
			SemanticName:  "POSITION",
			SemanticIndex: 0,
			SystemValue:   SVArbitrary,
			CompType:      CompTypeFloat32,
			Register:      0,
			Mask:          0x0F,
			RWMask:        0x0F,
		},
	})
	c.AddOutputSignature([]SignatureElement{
		{
			SemanticName:  "SV_Position",
			SemanticIndex: 0,
			SystemValue:   SVPosition,
			CompType:      CompTypeFloat32,
			Register:      0,
			Mask:          0x0F,
		},
	})
	c.AddDXILPart(1, 1, 0, make([]byte, 16))
	c.AddHashPart()
	data := c.Bytes()

	// Verify part count = 5 (SFI0, ISG1, OSG1, DXIL, HASH).
	partCount := binary.LittleEndian.Uint32(data[28:32])
	if partCount != 5 {
		t.Fatalf("part count: got %d, want 5", partCount)
	}

	// Verify file size matches.
	fileSize := binary.LittleEndian.Uint32(data[24:28])
	if fileSize != uint32(len(data)) {
		t.Errorf("file size: got %d, actual %d", fileSize, len(data))
	}

	// Find ISG1 and OSG1 parts.
	foundISG1, foundOSG1 := false, false
	offsetTableStart := 32
	for i := 0; i < int(partCount); i++ {
		offset := binary.LittleEndian.Uint32(data[offsetTableStart+4*i:])
		fc := binary.LittleEndian.Uint32(data[offset:])
		switch fc {
		case FourCCISG1:
			foundISG1 = true
		case FourCCOSG1:
			foundOSG1 = true
		}
	}
	if !foundISG1 {
		t.Error("ISG1 part not found in container")
	}
	if !foundOSG1 {
		t.Error("OSG1 part not found in container")
	}
}

// readNullTermString reads a null-terminated string from data at the given offset.
func readNullTermString(data []byte, offset int) string {
	end := offset
	for end < len(data) && data[end] != 0 {
		end++
	}
	return string(data[offset:end])
}
