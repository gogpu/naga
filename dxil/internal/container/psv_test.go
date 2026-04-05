package container

import (
	"encoding/binary"
	"testing"
)

func TestPSV0Basic_VertexShader(t *testing.T) {
	info := PSVInfo{
		ShaderStage:           PSVVertex,
		OutputPositionPresent: true,
		SigInputElements:      1,
		SigOutputElements:     1,
		SigInputVectors:       1,
		SigOutputVectors:      1,
		MinWaveLaneCount:      0,
		MaxWaveLaneCount:      0xFFFFFFFF,
	}

	data := EncodePSV0(info)

	// First 4 bytes = runtime info size.
	psvSize := binary.LittleEndian.Uint32(data[0:4])
	if psvSize != runtimeInfo1Size {
		t.Errorf("psv size: got %d, want %d", psvSize, runtimeInfo1Size)
	}

	// Output position present flag (first byte of union).
	if data[4] != 1 {
		t.Errorf("output_position_present: got %d, want 1", data[4])
	}

	// Min/max wave lane count.
	minWave := binary.LittleEndian.Uint32(data[4+16:])
	if minWave != 0 {
		t.Errorf("min_wave_lane_count: got %d, want 0", minWave)
	}
	maxWave := binary.LittleEndian.Uint32(data[4+20:])
	if maxWave != 0xFFFFFFFF {
		t.Errorf("max_wave_lane_count: got 0x%08X, want 0xFFFFFFFF", maxWave)
	}

	// Shader stage (byte at offset 24 in runtime info = offset 28 in data).
	stage := data[4+24]
	if stage != uint8(PSVVertex) {
		t.Errorf("shader_stage: got %d, want %d", stage, PSVVertex)
	}

	// Signature element counts.
	sigInputElems := data[4+28]
	if sigInputElems != 1 {
		t.Errorf("sig_input_elements: got %d, want 1", sigInputElems)
	}
	sigOutputElems := data[4+29]
	if sigOutputElems != 1 {
		t.Errorf("sig_output_elements: got %d, want 1", sigOutputElems)
	}

	// Resource count should be 0.
	resourceCount := binary.LittleEndian.Uint32(data[4+runtimeInfo1Size:])
	if resourceCount != 0 {
		t.Errorf("resource_count: got %d, want 0", resourceCount)
	}
}

func TestPSV0Basic_PixelShader(t *testing.T) {
	info := PSVInfo{
		ShaderStage:      PSVPixel,
		DepthOutput:      true,
		SampleFrequency:  false,
		MinWaveLaneCount: 0,
		MaxWaveLaneCount: 0xFFFFFFFF,
	}

	data := EncodePSV0(info)

	// Depth output flag.
	if data[4] != 1 {
		t.Errorf("depth_output: got %d, want 1", data[4])
	}

	// Sample frequency flag.
	if data[5] != 0 {
		t.Errorf("sample_frequency: got %d, want 0", data[5])
	}

	stage := data[4+24]
	if stage != uint8(PSVPixel) {
		t.Errorf("shader_stage: got %d, want %d", stage, PSVPixel)
	}
}

func TestPSV0WithStringTable(t *testing.T) {
	info := PSVInfo{
		ShaderStage:        PSVVertex,
		MinWaveLaneCount:   0,
		MaxWaveLaneCount:   0xFFFFFFFF,
		StringTable:        []byte("POSITION\x00TEXCOORD\x00"),
		SemanticIndexTable: []uint32{0, 0},
	}

	data := EncodePSV0(info)

	// Find string table after runtime info + resource count.
	pos := 4 + runtimeInfo1Size + 4 // psv_size(4) + runtime(36) + resource_count(4)

	// String table size (4-byte aligned).
	stringTableSize := binary.LittleEndian.Uint32(data[pos:])
	if stringTableSize != 20 { // "POSITION\0TEXCOORD\0" = 18 bytes, aligned to 20
		t.Errorf("string_table_size: got %d, want 20", stringTableSize)
	}
	pos += 4

	// Read POSITION string from table.
	name := readNullTermString(data, pos)
	if name != "POSITION" {
		t.Errorf("first string: got %q, want %q", name, "POSITION")
	}
	pos += int(stringTableSize)

	// Semantic index table count.
	semIdxCount := binary.LittleEndian.Uint32(data[pos:])
	if semIdxCount != 2 {
		t.Errorf("sem_index_count: got %d, want 2", semIdxCount)
	}
}

func TestContainerWithPSV0(t *testing.T) {
	c := New()
	c.AddFeaturesPart(0)
	c.AddPSV0(PSVInfo{
		ShaderStage:      PSVVertex,
		MinWaveLaneCount: 0,
		MaxWaveLaneCount: 0xFFFFFFFF,
	})
	c.AddDXILPart(1, 1, 0, make([]byte, 16))
	c.AddHashPart()
	data := c.Bytes()

	partCount := binary.LittleEndian.Uint32(data[28:32])
	if partCount != 4 {
		t.Fatalf("part count: got %d, want 4", partCount)
	}

	// Find PSV0 part.
	foundPSV0 := false
	offsetTableStart := 32
	for i := 0; i < int(partCount); i++ {
		offset := binary.LittleEndian.Uint32(data[offsetTableStart+4*i:])
		fc := binary.LittleEndian.Uint32(data[offset:])
		if fc == FourCCPSV0 {
			foundPSV0 = true
		}
	}
	if !foundPSV0 {
		t.Error("PSV0 part not found in container")
	}

	// Verify file size.
	fileSize := binary.LittleEndian.Uint32(data[24:28])
	if fileSize != uint32(len(data)) {
		t.Errorf("file size: got %d, actual %d", fileSize, len(data))
	}
}
