package bitcheck

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// buildMinimalContainer wraps the given bitcode bytes in the minimum
// DXBC + DxilProgramHeader envelope needed to satisfy extractBitcode.
// This produces a blob that is NOT a real DXIL file (no PSV0, no
// signatures) but is structurally valid for the bitcheck pipeline.
func buildMinimalContainer(t *testing.T, bitcode []byte) []byte {
	t.Helper()

	// --- DxilProgramHeader (24 B) ---
	var progHdr [dxilProgHdrSize]byte
	binary.LittleEndian.PutUint32(progHdr[0:], 0)                      // programVersion (vs 6.0 style, irrelevant here)
	binary.LittleEndian.PutUint32(progHdr[4:], uint32(len(bitcode)/4)) // programSize in words, approximate
	binary.LittleEndian.PutUint32(progHdr[8:], ccDXIL)                 // DxilMagic
	binary.LittleEndian.PutUint32(progHdr[12:], 0x100)                 // dxilVersion
	binary.LittleEndian.PutUint32(progHdr[16:], 16)                    // bitcode offset (from dxilMagic)
	binary.LittleEndian.PutUint32(progHdr[20:], uint32(len(bitcode)))  // bitcode size

	partData := make([]byte, 0, len(progHdr)+len(bitcode))
	partData = append(partData, progHdr[:]...)
	partData = append(partData, bitcode...)

	// --- Part header (fourCC + size) ---
	const numParts = 1
	offsetTableSize := 4 * numParts
	partHdrOff := dxbcHdrSize + offsetTableSize
	partDataOff := partHdrOff + partHdrSize
	total := partDataOff + len(partData)

	out := make([]byte, total)
	// DXBC header.
	binary.LittleEndian.PutUint32(out[0:], ccDXBC)
	// digest bytes [4:20] left zero.
	binary.LittleEndian.PutUint16(out[20:], 1) // major
	binary.LittleEndian.PutUint16(out[22:], 0) // minor
	binary.LittleEndian.PutUint32(out[24:], uint32(total))
	binary.LittleEndian.PutUint32(out[28:], numParts)

	// Part offset table.
	binary.LittleEndian.PutUint32(out[32:], uint32(partHdrOff))

	// Part header.
	binary.LittleEndian.PutUint32(out[partHdrOff:], ccDXIL)
	binary.LittleEndian.PutUint32(out[partHdrOff+4:], uint32(len(partData)))

	// Part payload.
	copy(out[partDataOff:], partData)
	return out
}

// buildBitcodeWithEntryPoints assembles a LLVM bitstream with just the
// magic + a MODULE_BLOCK containing a METADATA_BLOCK that describes a
// single valid entry point. op0, typ, val let us break each case.
func buildBitcodeWithEntryPoints(_ *testing.T, op0, typ, val uint64, includeEntryPoints bool, emptyTuple bool) []byte {
	w := newBlockWriter(2)
	// 'BC\xC0\xDE' magic.
	w.writeFixed('B', 8)
	w.writeFixed('C', 8)
	w.writeFixed(0xC0, 8)
	w.writeFixed(0xDE, 8)
	// MODULE_BLOCK.
	w.enterBlock(blockIDModule, 3)
	// METADATA_BLOCK.
	w.enterBlock(blockIDMetadata, 3)
	// Node 0: METADATA_VALUE(typ, val).
	w.emitRecord(mdCodeValue, []uint64{typ, val})
	// Node 1: METADATA_STRING "main".
	w.emitRecord(mdCodeString, []uint64{'m', 'a', 'i', 'n'})
	// Node 2: METADATA_NODE with op0 + name ref.
	if emptyTuple {
		w.emitRecord(mdCodeNode, []uint64{})
	} else {
		w.emitRecord(mdCodeNode, []uint64{op0, 2})
	}
	if includeEntryPoints {
		w.emitNamedMD(dxEntryPointsName, []uint64{2})
	} else {
		w.emitNamedMD("llvm.ident", []uint64{1})
	}
	w.exitBlock() // METADATA_BLOCK
	w.exitBlock() // MODULE_BLOCK
	return w.bytes()
}

func TestCheck_ValidEntryPoint(t *testing.T) {
	bc := buildBitcodeWithEntryPoints(t, 1, 1, 1, true, false)
	container := buildMinimalContainer(t, bc)
	if err := Check(container); err != nil {
		t.Fatalf("Check valid container: %v", err)
	}
}

func TestCheck_NullEntryPointFunction(t *testing.T) {
	bc := buildBitcodeWithEntryPoints(t, 0, 1, 1, true, false)
	container := buildMinimalContainer(t, bc)
	if err := Check(container); !errors.Is(err, ErrNullEntryPointFunction) {
		t.Fatalf("Check null op0 err = %v, want ErrNullEntryPointFunction", err)
	}
}

func TestCheck_NullMetadataValue(t *testing.T) {
	bc := buildBitcodeWithEntryPoints(t, 1, 0, 0, true, false)
	container := buildMinimalContainer(t, bc)
	if err := Check(container); !errors.Is(err, ErrNullEntryPointFunction) {
		t.Fatalf("Check MD(0,0) err = %v, want ErrNullEntryPointFunction", err)
	}
}

func TestCheck_MissingEntryPoints(t *testing.T) {
	bc := buildBitcodeWithEntryPoints(t, 1, 1, 1, false, false)
	container := buildMinimalContainer(t, bc)
	if err := Check(container); !errors.Is(err, ErrMissingEntryPoints) {
		t.Fatalf("Check missing entry points err = %v, want ErrMissingEntryPoints", err)
	}
}

func TestCheck_EmptyEntryPointTuple(t *testing.T) {
	bc := buildBitcodeWithEntryPoints(t, 1, 1, 1, true, true)
	container := buildMinimalContainer(t, bc)
	if err := Check(container); !errors.Is(err, ErrEmptyEntryPointTuple) {
		t.Fatalf("Check empty tuple err = %v, want ErrEmptyEntryPointTuple", err)
	}
}

func TestCheck_NoBitcode(t *testing.T) {
	// Empty blob.
	if err := Check(nil); !errors.Is(err, ErrNoBitcode) {
		t.Fatalf("Check nil err = %v, want ErrNoBitcode", err)
	}
	// DXBC header but no parts.
	var hdr [dxbcHdrSize]byte
	binary.LittleEndian.PutUint32(hdr[0:], ccDXBC)
	binary.LittleEndian.PutUint32(hdr[28:], 0)
	if err := Check(hdr[:]); !errors.Is(err, ErrNoBitcode) {
		t.Fatalf("Check no-parts err = %v, want ErrNoBitcode", err)
	}
	// Bad magic.
	var bad [dxbcHdrSize]byte
	copy(bad[0:4], "XYZW")
	if err := Check(bad[:]); !errors.Is(err, ErrNoBitcode) {
		t.Fatalf("Check bad magic err = %v, want ErrNoBitcode", err)
	}
}

func TestCheck_MalformedTruncatedBitcode(t *testing.T) {
	// Build a valid container, then chop half the bitcode bytes.
	bc := buildBitcodeWithEntryPoints(t, 1, 1, 1, true, false)
	// Keep only the first 8 bytes of bitcode (magic + a few bits of
	// MODULE_BLOCK).
	bc = bc[:8]
	container := buildMinimalContainer(t, bc)
	err := Check(container)
	if err == nil {
		t.Fatalf("Check truncated bc err = nil, want error")
	}
	if !errors.Is(err, ErrMalformedBitstream) && !errors.Is(err, ErrNoBitcode) {
		t.Fatalf("Check truncated bc err = %v, want malformed or no-bitcode", err)
	}
}

// TestCheck_NagaFixture runs Check against the first-ever S_OK naga
// output. It must return nil.
func TestCheck_NagaFixture(t *testing.T) {
	path := filepath.Join("..", "..", "..", "tmp", "min1_final.dxil")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("skipping: cannot read %s: %v", path, err)
	}
	if err := Check(data); err != nil {
		t.Fatalf("Check naga fixture: %v", err)
	}
}

// TestCheck_GoldenDXCBlobs runs Check against the canonical DXC 1.8
// golden blobs staged under testdata/golden-dxc/. These files are real
// dxc.exe output for a trivial triangle VS + FS (see tmp/gogpu_triangle.hlsl)
// and exercise the dual-METADATA_BLOCK layout that DXC emits:
// one named-metadata block followed by an empty function-attachment
// block. The pre-BUG-DXIL-024 walker would incorrectly surface the
// second (empty) block's missing dx.entryPoints as a bitcheck failure.
func TestCheck_GoldenDXCBlobs(t *testing.T) {
	tests := []struct {
		name, path string
	}{
		{"triangle VS", filepath.Join("testdata", "golden-dxc", "triangle_vs_1.8.dxil")},
		{"triangle FS", filepath.Join("testdata", "golden-dxc", "triangle_fs_1.8.dxil")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatalf("read %s: %v", tc.path, err)
			}
			if err := Check(data); err != nil {
				t.Fatalf("Check %s: %v (expected nil — DXC output is valid)", tc.name, err)
			}
		})
	}
}

// TestCheck_DXCFixture runs Check against a DXC reference output so
// we exercise the abbreviation-decoder path against third-party
// bitcode. Skipped if the fixture is absent OR if the fixture itself
// lacks dx.entryPoints (our current tmp/dxc_trivial_vs.dxil is a
// partial / experimental artifact from a prior session and does not
// contain that named metadata — see FEAT-VALIDATOR-BITCHECK-002).
//
// The abbreviation-decoder code path IS implemented in blocks.go and
// IS exercised by bitstream round-trip + hand-crafted tests; this test
// is specifically the real-world-DXC-output integration gate and will
// be enabled once we have a proper minimal DXC fixture compiled from
// HLSL via dxc.exe.
func TestCheck_DXCFixture(t *testing.T) {
	path := filepath.Join("..", "..", "..", "tmp", "dxc_trivial_vs.dxil")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("skipping: cannot read %s: %v", path, err)
	}
	// Sanity: the blob must contain dx.entryPoints as a raw byte sequence
	// for this test to be meaningful. If it does not, the fixture is
	// incomplete and the abbreviation-decoder path cannot be exercised
	// through it. Skip rather than false-fail on a known-bad fixture.
	if !bytes.Contains(data, []byte("dx.entryPoints")) {
		t.Skipf("fixture %s does not contain raw 'dx.entryPoints' name — "+
			"known-incomplete artifact, tracked under FEAT-VALIDATOR-BITCHECK-002", path)
	}
	if err := Check(data); err != nil {
		t.Fatalf("Check DXC fixture: %v", err)
	}
}

// TestCheck_CorpusSmoke runs Check against every *.dxil blob it can
// find in the repo's tmp/ directory. It reports counts but does NOT
// hard-fail on individual errors — the corpus contains known-bad
// entries (wrong PSV0 stage, etc.) that precheck + bitcheck will
// legitimately reject. The goal is to ensure Check never panics and
// every error surface is a typed sentinel.
func TestCheck_CorpusSmoke(t *testing.T) {
	pattern := filepath.Join("..", "..", "..", "tmp", "*.dxil")
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Skipf("no tmp/*.dxil fixtures present")
	}
	var (
		okCount    int
		missing    int
		nullFn     int
		emptyTuple int
		malformed  int
		noBitcode  int
		unknown    int
	)
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Errorf("read %s: %v", f, err)
			continue
		}
		err = Check(data)
		switch {
		case err == nil:
			okCount++
		case errors.Is(err, ErrMissingEntryPoints):
			missing++
		case errors.Is(err, ErrNullEntryPointFunction):
			nullFn++
		case errors.Is(err, ErrEmptyEntryPointTuple):
			emptyTuple++
		case errors.Is(err, ErrMalformedBitstream):
			malformed++
		case errors.Is(err, ErrNoBitcode):
			noBitcode++
		default:
			unknown++
			t.Errorf("unexpected error kind for %s: %v", f, err)
		}
	}
	t.Logf("corpus %d files: ok=%d missing=%d nullFn=%d emptyTuple=%d malformed=%d noBitcode=%d unknown=%d",
		len(files), okCount, missing, nullFn, emptyTuple, malformed, noBitcode, unknown)
	if unknown > 0 {
		t.Fatalf("%d files returned an unknown error kind — every bitcheck error must be a typed sentinel", unknown)
	}
}
