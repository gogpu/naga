package container

import (
	"encoding/binary"
	"testing"
)

// TestPSVShaderKindMatchesMicrosoftABI locks the PSVShaderKind enum to the
// Microsoft D3D12 runtime ABI. If these values ever drift again, this test
// fails at build time instead of silently breaking CreateGraphicsPipelineState
// at runtime (which is how BUG-DXIL-007 originally escaped our test gates).
//
// Sources of truth:
//   - Microsoft DXC: reference/dxil/dxc/include/dxc/DXIL/DxilConstants.h:244
//     (enum class ShaderKind, "Must match D3D11_SHADER_VERSION_TYPE")
//   - Mesa DXIL:     reference/dxil/mesa/src/microsoft/compiler/dxil_module.h:44
//     (enum dxil_shader_kind)
func TestPSVShaderKindMatchesMicrosoftABI(t *testing.T) {
	cases := []struct {
		name     string
		kind     PSVShaderKind
		expected uint8
	}{
		{"Pixel", PSVPixel, 0},
		{"Vertex", PSVVertex, 1},
		{"Geometry", PSVGeometry, 2},
		{"Hull", PSVHull, 3},
		{"Domain", PSVDomain, 4},
		{"Compute", PSVCompute, 5},
		{"Library", PSVLibrary, 6},
		{"RayGeneration", PSVRayGeneration, 7},
		{"Intersection", PSVIntersection, 8},
		{"AnyHit", PSVAnyHit, 9},
		{"ClosestHit", PSVClosestHit, 10},
		{"Miss", PSVMiss, 11},
		{"Callable", PSVCallable, 12},
		{"Mesh", PSVMesh, 13},
		{"Amplification", PSVAmplification, 14},
		{"Node", PSVNode, 15},
		{"Invalid", PSVInvalid, 16},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if uint8(c.kind) != c.expected {
				t.Fatalf("%s = %d, want %d (Microsoft D3D12 ABI — see DxilConstants.h:244)",
					c.name, uint8(c.kind), c.expected)
			}
		})
	}
}

// TestEncodePSV0ShaderStageByte verifies EncodePSV0 writes the ShaderStage
// byte at the correct PSVRuntimeInfo1 offset with the correct ABI value.
// Offset: 4 (psv_size field) + 24 (PSVRuntimeInfo0 size) = 28 in the output.
func TestEncodePSV0ShaderStageByte(t *testing.T) {
	stages := []struct {
		stage    PSVShaderKind
		wantByte uint8
	}{
		{PSVPixel, 0},
		{PSVVertex, 1},
		{PSVGeometry, 2},
		{PSVHull, 3},
		{PSVDomain, 4},
		{PSVCompute, 5},
	}
	const shaderStageOffset = 4 + 24 // psv_size(4) + PSVRuntimeInfo0(24)
	for _, tc := range stages {
		info := PSVInfo{
			ShaderStage:      tc.stage,
			MinWaveLaneCount: 0,
			MaxWaveLaneCount: 0xFFFFFFFF,
		}
		data := EncodePSV0(info)
		got := data[shaderStageOffset]
		if got != tc.wantByte {
			t.Errorf("stage %d: encoded byte at offset %d = %d, want %d (Microsoft D3D12 ABI)",
				tc.stage, shaderStageOffset, got, tc.wantByte)
		}
	}
}

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
	// Updated by BUG-DXIL-009: PSV0 now always emitted at Info3 (52 bytes)
	// regardless of stage. Unused NumThreads/EntryFunctionName fields are
	// zero-padded so the payload is safe for validator 1.6+ to parse.
	psvSize := binary.LittleEndian.Uint32(data[0:4])
	if psvSize != runtimeInfo3Size {
		t.Errorf("psv size: got %d, want %d", psvSize, runtimeInfo3Size)
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
	// Updated by BUG-DXIL-009: when PSVSigInputs/PSVSigOutputs are empty
	// (no matching PSV signature elements will be appended), the RTI1
	// sig-element counts are force-cleared to keep the PSV0 payload
	// self-consistent for the validator. Proper per-stage PSV signature
	// emission for VS/PS/HS/DS/GS is a follow-up task.
	sigInputElems := data[4+28]
	if sigInputElems != 0 {
		t.Errorf("sig_input_elements (clamped): got %d, want 0", sigInputElems)
	}
	sigOutputElems := data[4+29]
	if sigOutputElems != 0 {
		t.Errorf("sig_output_elements (clamped): got %d, want 0", sigOutputElems)
	}

	// NumThreadsX/Y/Z zero-padded for VS (PSVRuntimeInfo2 extension area).
	for i, off := range []int{36, 40, 44} {
		if v := binary.LittleEndian.Uint32(data[4+off:]); v != 0 {
			t.Errorf("num_threads[%d] (offset %d): got %d, want 0", i, off, v)
		}
	}
	// EntryFunctionName zero-padded for VS with empty StringTable
	// (PSVRuntimeInfo3 extension area).
	if entry := binary.LittleEndian.Uint32(data[4+48:]); entry != 0 {
		t.Errorf("entry_function_name: got %d, want 0", entry)
	}

	// Resource count should be 0.
	resourceCount := binary.LittleEndian.Uint32(data[4+runtimeInfo3Size:])
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
	// Updated by BUG-DXIL-009: runtime info is now 52 bytes (Info3), not 36.
	pos := 4 + runtimeInfo3Size + 4 // psv_size(4) + runtime(52) + resource_count(4)

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

// TestEncodePSV0SizeIsAlwaysInfo3 is the regression test for BUG-DXIL-009.
// Every shader stage — VS, PS, CS, MS, AS — must write psv_size = 52
// (PSVRuntimeInfo3) in the first 4 bytes of the PSV0 payload. Modern
// dxil.dll validators interpret this header as the struct layout version
// and assume Info3 fields are present; emitting 36 (Info1) causes an AV.
func TestEncodePSV0SizeIsAlwaysInfo3(t *testing.T) {
	stages := []struct {
		name  string
		stage PSVShaderKind
	}{
		{"vertex", PSVVertex},
		{"pixel", PSVPixel},
		{"compute", PSVCompute},
		{"mesh", PSVMesh},
		{"amplification", PSVAmplification},
	}
	for _, tc := range stages {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			data := EncodePSV0(PSVInfo{
				ShaderStage:      tc.stage,
				MinWaveLaneCount: 0,
				MaxWaveLaneCount: 0xFFFFFFFF,
			})
			psvSize := binary.LittleEndian.Uint32(data[0:4])
			if psvSize != runtimeInfo3Size {
				t.Errorf("stage %s: psv_size = %d, want %d (PSVRuntimeInfo3)",
					tc.name, psvSize, runtimeInfo3Size)
			}
			// Payload must contain at least 4 (header) + 52 (runtime info)
			// + 4 (resource count) + 4 (string table size) + 4 (semantic
			// index count) = 68 bytes even with empty optional tables.
			if len(data) < 4+runtimeInfo3Size+12 {
				t.Errorf("stage %s: payload too short: got %d bytes, want >= %d",
					tc.name, len(data), 4+runtimeInfo3Size+12)
			}
		})
	}
}

// TestEncodePSV0ZeroPadsUnusedFields verifies that non-compute stages emit
// zero bytes in the NumThreadsX/Y/Z and EntryFunctionName slots when the
// caller does not populate them. This is the contract that unlocks VS/PS
// under modern dxil.dll: the validator reads past Info1 but finds valid
// zero bytes instead of garbage.
func TestEncodePSV0ZeroPadsUnusedFields(t *testing.T) {
	data := EncodePSV0(PSVInfo{
		ShaderStage:      PSVVertex,
		MinWaveLaneCount: 0,
		MaxWaveLaneCount: 0xFFFFFFFF,
	})
	// Offsets inside the runtime info block (add 4 for psv_size header).
	const rtOff = 4
	for _, tc := range []struct {
		name string
		off  int
	}{
		{"NumThreadsX", 36},
		{"NumThreadsY", 40},
		{"NumThreadsZ", 44},
		{"EntryFunctionName", 48},
	} {
		got := binary.LittleEndian.Uint32(data[rtOff+tc.off:])
		if got != 0 {
			t.Errorf("%s at runtime offset %d: got %d, want 0", tc.name, tc.off, got)
		}
	}
}

// TestEncodePSV0EntryFunctionNameInStringTable verifies that the string
// table layout contains the entry function name and that the
// EntryFunctionName field is a valid offset pointing at it. This is the
// contract that the caller (dxil.go:buildPSV) cooperates with.
func TestEncodePSV0EntryFunctionNameInStringTable(t *testing.T) {
	table, off := BuildPSVStringTable("main")
	// Sanity: helper must return a 4-byte aligned slice containing "main".
	if len(table)%4 != 0 {
		t.Fatalf("string table not 4-byte aligned: len=%d", len(table))
	}
	if off != 1 {
		t.Errorf("entry offset: got %d, want 1 (after leading \\0)", off)
	}
	name := readNullTermString(table, int(off))
	if name != "main" {
		t.Errorf("entry name round-trip: got %q, want %q", name, "main")
	}

	data := EncodePSV0(PSVInfo{
		ShaderStage:       PSVVertex,
		MinWaveLaneCount:  0,
		MaxWaveLaneCount:  0xFFFFFFFF,
		StringTable:       table,
		EntryFunctionName: off,
	})
	// EntryFunctionName field inside the runtime info, read back.
	gotOff := binary.LittleEndian.Uint32(data[4+48:])
	if gotOff != off {
		t.Errorf("EntryFunctionName field: got %d, want %d", gotOff, off)
	}
	// String table sits after runtime info + resource count.
	stPos := 4 + runtimeInfo3Size + 4
	stSize := binary.LittleEndian.Uint32(data[stPos:])
	if stSize != uint32(len(table)) {
		t.Errorf("string table size: got %d, want %d", stSize, len(table))
	}
	// Read the entry name back from the encoded string table.
	roundTrip := readNullTermString(data, stPos+4+int(off))
	if roundTrip != "main" {
		t.Errorf("string table entry round-trip: got %q, want %q", roundTrip, "main")
	}
}

// TestEncodePSV0ClampsSigCountsWhenNoElements verifies that a caller which
// declares SigInputElements/SigOutputElements counts but provides no
// PSVSigInputs/PSVSigOutputs entries ends up with those counts forcibly
// cleared in the emitted RTI1 header. This is the self-consistency contract
// that stops the validator reading past our payload into adjacent memory.
func TestEncodePSV0ClampsSigCountsWhenNoElements(t *testing.T) {
	data := EncodePSV0(PSVInfo{
		ShaderStage:                 PSVVertex,
		MinWaveLaneCount:            0,
		MaxWaveLaneCount:            0xFFFFFFFF,
		SigInputElements:            3,
		SigOutputElements:           5,
		SigPatchConstOrPrimElements: 2,
		SigInputVectors:             7,
		SigOutputVectors:            11,
		// Intentionally no PSVSigInputs / PSVSigOutputs.
	})
	for _, tc := range []struct {
		name string
		off  int
	}{
		{"sig_input_elements", 28},
		{"sig_output_elements", 29},
		{"sig_patch_elements", 30},
		{"sig_input_vectors", 31},
		{"sig_output_vectors[0]", 32},
	} {
		if v := data[4+tc.off]; v != 0 {
			t.Errorf("%s: got %d, want 0 (clamped)", tc.name, v)
		}
	}
}

// TestEncodePSV0KeepsSigCountsWhenElementsProvided verifies the clamp is
// conditional — once the caller actually supplies PSVSigInputs/Outputs we
// honor the declared counts verbatim so mesh (and future per-stage
// signature emission) remains unaffected.
func TestEncodePSV0KeepsSigCountsWhenElementsProvided(t *testing.T) {
	data := EncodePSV0(PSVInfo{
		ShaderStage:       PSVMesh,
		MinWaveLaneCount:  0,
		MaxWaveLaneCount:  0xFFFFFFFF,
		SigInputElements:  0,
		SigOutputElements: 2,
		SigInputVectors:   0,
		SigOutputVectors:  2,
		PSVSigOutputs: []PSVSignatureElement{
			{Rows: 1, ColsAndStart: 0x04},
			{Rows: 1, ColsAndStart: 0x04},
		},
	})
	if got := data[4+29]; got != 2 {
		t.Errorf("sig_output_elements: got %d, want 2 (not clamped when elements provided)", got)
	}
	if got := data[4+32]; got != 2 {
		t.Errorf("sig_output_vectors[0]: got %d, want 2 (not clamped when elements provided)", got)
	}
}

// TestBuildPSVStringTableEmpty verifies the empty-name edge case: the helper
// must still return a 4-byte aligned non-nil slice with offset 0.
func TestBuildPSVStringTableEmpty(t *testing.T) {
	table, off := BuildPSVStringTable("")
	if off != 0 {
		t.Errorf("empty entry offset: got %d, want 0", off)
	}
	if len(table) == 0 || len(table)%4 != 0 {
		t.Errorf("empty string table: len=%d, want non-zero aligned to 4", len(table))
	}
	for i, b := range table {
		if b != 0 {
			t.Errorf("empty string table byte %d: got %d, want 0", i, b)
		}
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
