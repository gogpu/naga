// precheck_test.go — unit tests for PreCheckContainer.
//
// Tests build malformed fixtures by patching specific bytes on a
// known-good DXIL blob (tmp/min1_final.dxil, the first-ever naga S_OK
// fixture). This avoids hand-crafting a DXBC container from scratch
// and guarantees tests break only when real invariants are violated.

package dxcvalidator

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// goldenBlob loads tmp/min1_final.dxil relative to the module root.
// Returns a FRESH copy per call so tests can mutate freely.
func goldenBlob(tb testing.TB) []byte {
	tb.Helper()
	// precheck_test.go sits in internal/dxcvalidator; module root is two up.
	path := filepath.Join("..", "..", "tmp", "min1_final.dxil")
	data, err := os.ReadFile(path)
	if err != nil {
		tb.Skipf("golden fixture not available (%v) — skipping mutation-based tests", err)
	}
	out := make([]byte, len(data))
	copy(out, data)
	return out
}

// findPart walks a known-good container and returns the start offset and
// length of the first part matching fcc. Used to target specific parts
// for mutation. Caller must already trust the container shape.
func findPart(tb testing.TB, blob []byte, fcc uint32) (partOffset, partSize int) {
	tb.Helper()
	if len(blob) < dxbcHeaderSize {
		tb.Fatalf("findPart: blob too short")
	}
	partCount := binary.LittleEndian.Uint32(blob[28:32])
	for i := uint32(0); i < partCount; i++ {
		off := binary.LittleEndian.Uint32(blob[dxbcHeaderSize+4*i:])
		if binary.LittleEndian.Uint32(blob[off:]) == fcc {
			size := binary.LittleEndian.Uint32(blob[off+4:])
			return int(off), int(size)
		}
	}
	tb.Fatalf("findPart: no part with fourCC %08x", fcc)
	return 0, 0
}

func TestPreCheck_GoldenBlobPasses(t *testing.T) {
	blob := goldenBlob(t)
	if err := PreCheckContainer(blob); err != nil {
		t.Fatalf("golden blob must pass pre-check, got: %v", err)
	}
}

func TestPreCheck_EmptyBlob(t *testing.T) {
	err := PreCheckContainer(nil)
	if !errors.Is(err, ErrTruncatedContainer) {
		t.Fatalf("want ErrTruncatedContainer, got %v", err)
	}
}

func TestPreCheck_ShortBlob(t *testing.T) {
	err := PreCheckContainer(make([]byte, 31))
	if !errors.Is(err, ErrTruncatedContainer) {
		t.Fatalf("want ErrTruncatedContainer, got %v", err)
	}
}

func TestPreCheck_BadMagic(t *testing.T) {
	b := make([]byte, 64)
	copy(b, "NOPE")
	err := PreCheckContainer(b)
	if !errors.Is(err, ErrBadMagic) {
		t.Fatalf("want ErrBadMagic, got %v", err)
	}
}

func TestPreCheck_BadPartCount_TooMany(t *testing.T) {
	blob := goldenBlob(t)
	// Overwrite part count with 65.
	binary.LittleEndian.PutUint32(blob[28:32], 65)
	err := PreCheckContainer(blob)
	if !errors.Is(err, ErrBadPartCount) {
		t.Fatalf("want ErrBadPartCount, got %v", err)
	}
}

func TestPreCheck_BadPartCount_Zero(t *testing.T) {
	blob := goldenBlob(t)
	binary.LittleEndian.PutUint32(blob[28:32], 0)
	err := PreCheckContainer(blob)
	if !errors.Is(err, ErrBadPartCount) {
		t.Fatalf("want ErrBadPartCount, got %v", err)
	}
}

func TestPreCheck_MalformedPartHeader(t *testing.T) {
	blob := goldenBlob(t)
	// Overwrite part[0] offset with 0xFFFFFFFF.
	binary.LittleEndian.PutUint32(blob[dxbcHeaderSize:dxbcHeaderSize+4], 0xFFFFFFFF)
	err := PreCheckContainer(blob)
	if !errors.Is(err, ErrMalformedPartHeader) {
		t.Fatalf("want ErrMalformedPartHeader, got %v", err)
	}
}

func TestPreCheck_MalformedPartHeader_SizeOverrun(t *testing.T) {
	blob := goldenBlob(t)
	// Pick the first part and inflate its declared size past the blob.
	partOff := binary.LittleEndian.Uint32(blob[dxbcHeaderSize : dxbcHeaderSize+4])
	binary.LittleEndian.PutUint32(blob[partOff+4:partOff+8], 0xFFFFFFFF)
	err := PreCheckContainer(blob)
	if !errors.Is(err, ErrMalformedPartHeader) {
		t.Fatalf("want ErrMalformedPartHeader, got %v", err)
	}
}

func TestPreCheck_MissingDXILPart(t *testing.T) {
	blob := goldenBlob(t)
	// Corrupt the DXIL part's fourCC to something junk so the walker
	// cannot find it. Keep the PSV0 / ISG1 / OSG1 intact — we want the
	// error to be "no DXIL", not any downstream check.
	dxilOff, _ := findPart(t, blob, fccDXIL)
	copy(blob[dxilOff:dxilOff+4], "JUNK")
	err := PreCheckContainer(blob)
	if !errors.Is(err, ErrMissingDXILPart) {
		t.Fatalf("want ErrMissingDXILPart, got %v", err)
	}
}

func TestPreCheck_MissingPSV0(t *testing.T) {
	blob := goldenBlob(t)
	psv0Off, _ := findPart(t, blob, fccPSV0)
	copy(blob[psv0Off:psv0Off+4], "JUNK")
	err := PreCheckContainer(blob)
	if !errors.Is(err, ErrMissingPSV0) {
		t.Fatalf("want ErrMissingPSV0, got %v", err)
	}
}

func TestPreCheck_EmptyEntryName(t *testing.T) {
	blob := goldenBlob(t)
	psv0Off, _ := findPart(t, blob, fccPSV0)
	psv0Data := blob[psv0Off+partHeaderSize:]
	// EntryFunctionName = PSV0 payload offset 4 + 48 = 52.
	// Overwrite with 0 so the string at offset 0 (a leading '\0') is used.
	binary.LittleEndian.PutUint32(psv0Data[52:56], 0)
	err := PreCheckContainer(blob)
	if !errors.Is(err, ErrEmptyEntryName) {
		t.Fatalf("want ErrEmptyEntryName, got %v", err)
	}
}

func TestPreCheck_InvalidStageByte(t *testing.T) {
	blob := goldenBlob(t)
	psv0Off, _ := findPart(t, blob, fccPSV0)
	psv0Data := blob[psv0Off+partHeaderSize:]
	// Stage byte = PSV0 payload offset 4 + 24 = 28.
	psv0Data[28] = 0xFF
	err := PreCheckContainer(blob)
	if !errors.Is(err, ErrInvalidStageByte) {
		t.Fatalf("want ErrInvalidStageByte, got %v", err)
	}
}

func TestPreCheck_MalformedPSV0_TooShort(t *testing.T) {
	// Build a minimal container with just an DXIL + PSV0 where PSV0 is
	// only 8 bytes long. Patching the golden blob in-place would move all
	// the following parts; simpler to fabricate.
	blob := goldenBlob(t)
	psv0Off, psv0Size := findPart(t, blob, fccPSV0)
	// Shrink declared size on the part header to 8. The on-disk bytes
	// after the first 8 PSV0 bytes will then be interpreted as belonging
	// to the following part — so we ALSO patch the part size to a value
	// that clearly under-runs PSVRuntimeInfo3.
	_ = psv0Size
	binary.LittleEndian.PutUint32(blob[psv0Off+4:psv0Off+8], 8)
	err := PreCheckContainer(blob)
	if !errors.Is(err, ErrMalformedPSV0) {
		t.Fatalf("want ErrMalformedPSV0, got %v", err)
	}
}

func TestPreCheck_MissingISGOSG_GraphicsStage(t *testing.T) {
	blob := goldenBlob(t)
	// Golden blob is a vertex shader (stage=1), so it has ISG1/OSG1.
	// Corrupt BOTH so the graphics check fires.
	isg1Off, _ := findPart(t, blob, fccISG1)
	copy(blob[isg1Off:isg1Off+4], "JUNK")
	osg1Off, _ := findPart(t, blob, fccOSG1)
	copy(blob[osg1Off:osg1Off+4], "JUNK")
	err := PreCheckContainer(blob)
	if !errors.Is(err, ErrMissingISGOSG) {
		t.Fatalf("want ErrMissingISGOSG, got %v", err)
	}
}

// TestPreCheck_CorpusSmoke runs PreCheckContainer against every DXIL
// fixture currently cached in tmp/. Not every fixture is structurally
// valid (some are intentionally broken by previous investigations),
// so we only assert that the pre-check NEVER panics and that every
// error returned is a typed error.
func TestPreCheck_CorpusSmoke(t *testing.T) {
	entries, err := filepath.Glob(filepath.Join("..", "..", "tmp", "*.dxil"))
	if err != nil || len(entries) == 0 {
		t.Skip("no tmp/*.dxil fixtures available")
	}
	for _, p := range entries {
		p := p
		t.Run(filepath.Base(p), func(t *testing.T) {
			data, readErr := os.ReadFile(p)
			if readErr != nil {
				t.Skipf("read %s: %v", p, readErr)
			}
			// We only care that the check completes without panicking.
			// Any error must be one of our typed errors.
			if err := PreCheckContainer(data); err != nil {
				for _, sentinel := range []error{
					ErrTruncatedContainer,
					ErrBadMagic,
					ErrBadPartCount,
					ErrMalformedPartHeader,
					ErrMissingDXILPart,
					ErrMissingPSV0,
					ErrMissingISGOSG,
					ErrEmptyEntryName,
					ErrInvalidStageByte,
					ErrMalformedPSV0,
				} {
					if errors.Is(err, sentinel) {
						return
					}
				}
				t.Fatalf("%s: untyped error: %v", p, err)
			}
		})
	}
}
