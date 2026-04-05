package container

import (
	"encoding/binary"
	"testing"
)

func TestNew(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New() returned nil")
	}
	data := c.Bytes()
	// Minimum header: magic(4) + digest(16) + version(4) + size(4) + partCount(4) = 32
	if len(data) < 32 {
		t.Fatalf("empty container too small: %d bytes", len(data))
	}
	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != FourCCDXBC {
		t.Errorf("magic: got 0x%08X, want 0x%08X", magic, FourCCDXBC)
	}
	partCount := binary.LittleEndian.Uint32(data[28:32])
	if partCount != 0 {
		t.Errorf("empty container part count: got %d, want 0", partCount)
	}
}

func TestFourCC(t *testing.T) {
	tests := []struct {
		name string
		got  uint32
		want uint32
	}{
		{"DXBC", FourCCDXBC, 0x43425844},
		{"DXIL", FourCCDXIL, 0x4C495844},
		{"SFI0", FourCCSFI0, 0x30494653},
		{"HASH", FourCCHASH, 0x48534148},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("FourCC %s: got 0x%08X, want 0x%08X", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestAddFeaturesPart(t *testing.T) {
	c := New()
	c.AddFeaturesPart(0)
	data := c.Bytes()

	partCount := binary.LittleEndian.Uint32(data[28:32])
	if partCount != 1 {
		t.Fatalf("part count: got %d, want 1", partCount)
	}

	// Part offset is at position 32.
	offset := binary.LittleEndian.Uint32(data[32:36])
	fc := binary.LittleEndian.Uint32(data[offset:])
	if fc != FourCCSFI0 {
		t.Errorf("part FourCC: got 0x%08X, want SFI0 0x%08X", fc, FourCCSFI0)
	}

	partSize := binary.LittleEndian.Uint32(data[offset+4:])
	if partSize != 8 {
		t.Errorf("features part size: got %d, want 8", partSize)
	}
}

func TestAddDXILPart(t *testing.T) {
	c := New()
	bitcode := []byte{0x42, 0x43, 0xC0, 0xDE, 0, 0, 0, 0} // minimal bitcode
	c.AddDXILPart(1, 1, 0, bitcode)
	data := c.Bytes()

	partCount := binary.LittleEndian.Uint32(data[28:32])
	if partCount != 1 {
		t.Fatalf("part count: got %d, want 1", partCount)
	}

	offset := binary.LittleEndian.Uint32(data[32:36])
	fc := binary.LittleEndian.Uint32(data[offset:])
	if fc != FourCCDXIL {
		t.Errorf("part FourCC: got 0x%08X, want DXIL 0x%08X", fc, FourCCDXIL)
	}

	// Part size = program header (24) + bitcode (8) = 32.
	partSize := binary.LittleEndian.Uint32(data[offset+4:])
	if partSize != 32 {
		t.Errorf("DXIL part size: got %d, want 32", partSize)
	}

	// Verify program header fields.
	hdrOff := int(offset) + 8
	version := binary.LittleEndian.Uint32(data[hdrOff:])
	// shaderKind=1(VS) << 16 | major=1 << 4 | minor=0 = 0x10010
	wantVer := uint32(1<<16 | 1<<4)
	if version != wantVer {
		t.Errorf("program version: got 0x%08X, want 0x%08X", version, wantVer)
	}

	dxilMagic := binary.LittleEndian.Uint32(data[hdrOff+8:])
	if dxilMagic != 0x4C495844 {
		t.Errorf("DXIL magic: got 0x%08X, want 0x4C495844", dxilMagic)
	}
}

func TestContainerFileSize(t *testing.T) {
	c := New()
	c.AddFeaturesPart(0)
	c.AddDXILPart(1, 1, 0, make([]byte, 16))
	c.AddHashPart()
	data := c.Bytes()

	fileSize := binary.LittleEndian.Uint32(data[24:28])
	if fileSize != uint32(len(data)) {
		t.Errorf("file size field: got %d, actual %d", fileSize, len(data))
	}
}

func TestSetBypassHash_Applied(t *testing.T) {
	data := make([]byte, 32)
	binary.LittleEndian.PutUint32(data[0:4], FourCCDXBC)
	SetBypassHash(data)

	for i := 4; i < 20; i++ {
		if data[i] != 0x01 {
			t.Errorf("byte %d: got 0x%02X, want 0x01", i, data[i])
		}
	}
}

func TestSetBypassHash_TooSmall(t *testing.T) {
	// Should not panic on small input.
	data := make([]byte, 10)
	SetBypassHash(data) // no-op, should not panic
}

func TestComputeRetailHash(t *testing.T) {
	// Create a minimal valid container.
	c := New()
	c.AddFeaturesPart(0)
	data := c.Bytes()

	// Compute retail hash (modified MD5).
	ComputeRetailHash(data)

	// The digest (bytes 4-19) should NOT be all zeros after hashing.
	allZero := true
	for i := 4; i < 20; i++ {
		if data[i] != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("retail hash produced all-zero digest")
	}

	// It should also NOT be the BYPASS sentinel.
	isBypass := true
	for i := 4; i < 20; i++ {
		if data[i] != 0x01 {
			isBypass = false
			break
		}
	}
	if isBypass {
		t.Error("retail hash produced BYPASS sentinel (impossible for real MD5)")
	}
}

func TestAddRawPart(t *testing.T) {
	c := New()
	c.AddRawPart(FourCCISG1, []byte{1, 2, 3, 4})
	data := c.Bytes()

	partCount := binary.LittleEndian.Uint32(data[28:32])
	if partCount != 1 {
		t.Fatalf("part count: got %d, want 1", partCount)
	}

	offset := binary.LittleEndian.Uint32(data[32:36])
	fc := binary.LittleEndian.Uint32(data[offset:])
	if fc != FourCCISG1 {
		t.Errorf("raw part FourCC: got 0x%08X, want ISG1 0x%08X", fc, FourCCISG1)
	}

	partSize := binary.LittleEndian.Uint32(data[offset+4:])
	if partSize != 4 {
		t.Errorf("raw part size: got %d, want 4", partSize)
	}
}
