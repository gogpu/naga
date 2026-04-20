// precheck.go — DXBC container structural pre-check.
//
// This file implements PreCheckContainer, a fast fixed-offset structural
// validator that runs BEFORE handing any blob to Microsoft's IDxcValidator
// (dxil.dll). It catches garbage containers at the container level without
// decoding LLVM bitcode — that is bitcheck.Check's job (the second layer).
//
// dxil.dll is brittle: truncated containers, malformed part tables, missing
// required parts, or PSV0 payloads with out-of-range stage bytes can trigger
// undefined behavior (AV, heap corruption, false S_OK with NULL result
// pointer). This layer rejects those shapes with a typed Go error so the
// dxcvalidator wrapper never hands dxil.dll a blob it will choke on.
//
// Scope is deliberately fixed-offset:
//
//   - DXBC header present, magic "DXBC", sane sizes
//   - Part table does not walk out of bounds
//   - At least one DXIL or ILDB part present
//   - PSV0 part present (required for graphics + compute)
//   - Graphics stages (PSVShaderKind 0..4, 13, 14) have ISG1 and OSG1
//   - PSV0 payload is at least PSVRuntimeInfo0 sized; PSVRuntimeInfo3
//     ShaderStage byte (offset 24 into the runtime-info struct) is a
//     documented PSVShaderKind value (0..14)
//   - PSV0 EntryFunctionName string offset resolves to a non-empty name
//     inside the string table
//
// Out of scope (handled elsewhere):
//
//   - Bitcode metadata walking → FEAT-VALIDATOR-BITCHECK-001
//   - Cross-part consistency (PSV0 counts vs ISG1/OSG1 element counts)
//   - Repairing malformed containers (this layer rejects; it does not fix)
//
// Reference: dxil/internal/container/container.go (emit path) and
// dxil/internal/container/psv.go (PSV0 layout).

package dxcvalidator

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Typed errors returned by PreCheckContainer. Callers use errors.Is to
// switch on the specific invariant that failed.
var (
	// ErrTruncatedContainer — blob shorter than the DXBC header (32 B) or
	// a later offset walks past the blob end.
	ErrTruncatedContainer = errors.New("dxcvalidator: precheck: container truncated")

	// ErrBadMagic — first 4 bytes are not "DXBC".
	ErrBadMagic = errors.New("dxcvalidator: precheck: bad DXBC magic")

	// ErrBadPartCount — part count is absurd (> 64) or the part offset
	// table itself does not fit in the blob.
	ErrBadPartCount = errors.New("dxcvalidator: precheck: bad part count")

	// ErrMalformedPartHeader — a part offset points past the end of the
	// blob, or the declared part data length walks past the end.
	ErrMalformedPartHeader = errors.New("dxcvalidator: precheck: malformed part header")

	// ErrMissingDXILPart — no "DXIL" or "ILDB" part present.
	ErrMissingDXILPart = errors.New("dxcvalidator: precheck: missing DXIL part")

	// ErrMissingPSV0 — no "PSV0" part present.
	ErrMissingPSV0 = errors.New("dxcvalidator: precheck: missing PSV0 part")

	// ErrMissingISGOSG — graphics stage missing "ISG1" or "OSG1".
	ErrMissingISGOSG = errors.New("dxcvalidator: precheck: missing ISG1/OSG1 for graphics stage")

	// ErrEmptyEntryName — PSV0 EntryFunctionName resolves to an empty
	// string.
	ErrEmptyEntryName = errors.New("dxcvalidator: precheck: empty PSV0 entry function name")

	// ErrInvalidStageByte — PSVRuntimeInfo ShaderStage byte is outside
	// the documented PSVShaderKind range [0..14].
	ErrInvalidStageByte = errors.New("dxcvalidator: precheck: invalid PSV0 shader stage byte")

	// ErrMalformedPSV0 — PSV0 payload is smaller than the PSVRuntimeInfo
	// header requires, or declared sizes walk past the end.
	ErrMalformedPSV0 = errors.New("dxcvalidator: precheck: malformed PSV0 part")
)

// Layout constants mirrored from dxil/internal/container.
const (
	dxbcHeaderSize     = 32 // magic(4) + digest(16) + major(2) + minor(2) + filesize(4) + partcount(4)
	dxbcMaxParts       = 64 // sanity bound; emit path writes at most ~8
	partHeaderSize     = 8  // fourCC(4) + size(4)
	psvRuntimeInfo3Min = 52 // PSVRuntimeInfo3 — matches container.runtimeInfo3Size
	psvStageByteOffset = 24 // ShaderStage is first byte of PSVRuntimeInfo1 extension
	psvMaxStageByte    = 14 // PSVAmplification = highest documented value
)

// FourCC codes, packed little-endian (matches container.fourCC).
const (
	fccDXBC uint32 = 'D' | 'X'<<8 | 'B'<<16 | 'C'<<24
	fccDXIL uint32 = 'D' | 'X'<<8 | 'I'<<16 | 'L'<<24
	fccILDB uint32 = 'I' | 'L'<<8 | 'D'<<16 | 'B'<<24
	fccPSV0 uint32 = 'P' | 'S'<<8 | 'V'<<16 | '0'<<24
	fccISG1 uint32 = 'I' | 'S'<<8 | 'G'<<16 | '1'<<24
	fccOSG1 uint32 = 'O' | 'S'<<8 | 'G'<<16 | '1'<<24
)

// precheckPart is a lightweight view over one DXBC part during pre-check.
type precheckPart struct {
	fourCC uint32
	data   []byte // subslice of the input blob; never copied
}

// PreCheckContainer runs a fast fixed-offset structural check over a DXBC
// container blob. It returns nil when the container is structurally sane
// enough to hand to IDxcValidator, or a wrapped typed error when any
// invariant is violated.
//
// PreCheckContainer never mutates the input and never allocates beyond
// small bounded slices for the part table view.
//
// Corpus impact note: when first enabled on the 237-entry naga test corpus,
// this check reshuffles ~113 entries from the "INVALID" bin (previously
// reported by dxil.dll) into the "VALIDATE_ERROR" bin (now reported here).
// The VALID count is unchanged (1/237 — the golden fixture), and the total
// failing count is unchanged. The reshuffled entries are genuinely broken:
// most are compute shaders whose PSV0 emitter still defaults ShaderStage=1
// (Vertex) instead of 5 (Compute), so this check correctly flags them as
// graphics stages missing ISG1/OSG1. Those are separate emitter bugs that
// will be filed and fixed independently — precheck is not false-positive,
// it is surfacing real underlying issues earlier in the pipeline.
func PreCheckContainer(blob []byte) error {
	parts, err := parseContainerParts(blob)
	if err != nil {
		return err
	}

	// Require a DXIL/ILDB part.
	var (
		havePSV0 bool
		haveISG1 bool
		haveOSG1 bool
		psv0     *precheckPart
		haveDXIL bool
	)
	for i := range parts {
		p := &parts[i]
		switch p.fourCC {
		case fccDXIL, fccILDB:
			haveDXIL = true
		case fccPSV0:
			havePSV0 = true
			psv0 = p
		case fccISG1:
			haveISG1 = true
		case fccOSG1:
			haveOSG1 = true
		}
	}
	if !haveDXIL {
		return fmt.Errorf("dxcvalidator: precheck: %w", ErrMissingDXILPart)
	}
	if !havePSV0 {
		return fmt.Errorf("dxcvalidator: precheck: %w", ErrMissingPSV0)
	}

	// PSV0 structural check. Stage byte decides whether ISG1/OSG1 are
	// required.
	stage, err := validatePSV0(psv0.data)
	if err != nil {
		return err
	}

	if isGraphicsStage(stage) {
		if !haveISG1 || !haveOSG1 {
			return fmt.Errorf("dxcvalidator: precheck: stage=%d: %w", stage, ErrMissingISGOSG)
		}
	}
	return nil
}

// parseContainerParts walks the DXBC header + part offset table and
// returns a view of each part's FourCC and payload slice. It performs
// full bounds checking on every offset and size.
func parseContainerParts(blob []byte) ([]precheckPart, error) {
	if len(blob) < dxbcHeaderSize {
		return nil, fmt.Errorf("dxcvalidator: precheck: need %d bytes, have %d: %w",
			dxbcHeaderSize, len(blob), ErrTruncatedContainer)
	}
	if binary.LittleEndian.Uint32(blob[0:4]) != fccDXBC {
		return nil, fmt.Errorf("dxcvalidator: precheck: %w", ErrBadMagic)
	}
	// Byte layout mirrors container.Bytes():
	//   0..3   : magic
	//   4..19  : digest
	//   20..21 : major
	//   22..23 : minor
	//   24..27 : total file size
	//   28..31 : part count
	declaredSize := binary.LittleEndian.Uint32(blob[24:28])
	if declaredSize != 0 && int(declaredSize) > len(blob) {
		return nil, fmt.Errorf("dxcvalidator: precheck: declared size %d > blob %d: %w",
			declaredSize, len(blob), ErrTruncatedContainer)
	}

	partCount := binary.LittleEndian.Uint32(blob[28:32])
	if partCount == 0 || partCount > dxbcMaxParts {
		return nil, fmt.Errorf("dxcvalidator: precheck: partCount=%d: %w", partCount, ErrBadPartCount)
	}

	// Part offset table: 4 bytes per part immediately after the header.
	offsetTableEnd := uint64(dxbcHeaderSize) + 4*uint64(partCount)
	if offsetTableEnd > uint64(len(blob)) {
		return nil, fmt.Errorf("dxcvalidator: precheck: part offset table end %d > blob %d: %w",
			offsetTableEnd, len(blob), ErrBadPartCount)
	}

	parts := make([]precheckPart, 0, partCount)
	for i := uint32(0); i < partCount; i++ {
		offsetPos := uint64(dxbcHeaderSize) + 4*uint64(i)
		partOffset := uint64(binary.LittleEndian.Uint32(blob[offsetPos : offsetPos+4]))

		// Part header must fit.
		if partOffset+uint64(partHeaderSize) > uint64(len(blob)) {
			return nil, fmt.Errorf("dxcvalidator: precheck: part[%d] offset=%d: %w",
				i, partOffset, ErrMalformedPartHeader)
		}
		fourCC := binary.LittleEndian.Uint32(blob[partOffset : partOffset+4])
		partSize := uint64(binary.LittleEndian.Uint32(blob[partOffset+4 : partOffset+8]))

		dataStart := partOffset + partHeaderSize
		dataEnd := dataStart + partSize
		if dataEnd > uint64(len(blob)) {
			return nil, fmt.Errorf("dxcvalidator: precheck: part[%d] size=%d walks past blob: %w",
				i, partSize, ErrMalformedPartHeader)
		}

		parts = append(parts, precheckPart{
			fourCC: fourCC,
			data:   blob[dataStart:dataEnd],
		})
	}
	return parts, nil
}

// validatePSV0 checks PSV0 structural invariants and returns the
// PSVShaderKind stage byte parsed from PSVRuntimeInfo1.
//
// Layout (see dxil/internal/container/psv.go:EncodePSV0):
//
//	[0..3]                  psv_size (uint32)
//	[4..4+psv_size]         PSVRuntimeInfo3 (52 B minimum for modern dxil.dll)
//	[...]                   resource_count (uint32) + bindings
//	[...]                   string_table_size (uint32) + data
//	[...]                   sem_index_count (uint32) + data
//	[...]                   optional PSVSignatureElement array
//
// Stage byte is PSVRuntimeInfo1 offset 0 = PSVRuntimeInfo0 offset 24 =
// data[4+24] = data[28].
//
// EntryFunctionName is PSVRuntimeInfo3 offset 24 (into RTI1..3) =
// PSVRuntimeInfo0 offset 48 = data[4+48] = data[52]. It is a u32 offset
// into the string table that follows the resource bindings.
func validatePSV0(data []byte) (uint8, error) {
	// Minimum payload: 4 (psv_size) + 52 (RTI3) + 4 (resource_count) +
	// 4 (string_table_size) + 4 (sem_index_count) = 68 bytes.
	const minPSV0 = 4 + psvRuntimeInfo3Min + 4 + 4 + 4
	if len(data) < minPSV0 {
		return 0, fmt.Errorf("dxcvalidator: precheck: PSV0 %d < %d: %w",
			len(data), minPSV0, ErrMalformedPSV0)
	}

	psvSize := binary.LittleEndian.Uint32(data[0:4])
	if psvSize < psvRuntimeInfo3Min {
		return 0, fmt.Errorf("dxcvalidator: precheck: psv_size=%d < %d: %w",
			psvSize, psvRuntimeInfo3Min, ErrMalformedPSV0)
	}
	if uint64(4)+uint64(psvSize) > uint64(len(data)) {
		return 0, fmt.Errorf("dxcvalidator: precheck: psv_size=%d walks past PSV0 payload %d: %w",
			psvSize, len(data), ErrMalformedPSV0)
	}

	// Stage byte.
	stage := data[4+psvStageByteOffset]
	if stage > psvMaxStageByte {
		return stage, fmt.Errorf("dxcvalidator: precheck: stage=%d: %w", stage, ErrInvalidStageByte)
	}

	// EntryFunctionName offset (PSVRuntimeInfo3, 4 bytes at offset 48).
	entryFuncOffset := binary.LittleEndian.Uint32(data[4+48 : 4+48+4])

	// Walk past the runtime info + resource_count + resource table to
	// reach the string table.
	pos := uint64(4) + uint64(psvSize)
	if pos+4 > uint64(len(data)) {
		return stage, fmt.Errorf("dxcvalidator: precheck: no room for resource_count: %w", ErrMalformedPSV0)
	}
	resourceCount := binary.LittleEndian.Uint32(data[pos : pos+4])
	pos += 4

	// If there are resource bindings, PSV0 encodes a per-entry size
	// followed by resourceCount entries. Our emit path never writes
	// resources (count == 0) so skip when zero; otherwise walk past the
	// resource table carefully.
	if resourceCount > 0 {
		// resource_binding_size(4) + resource_binding_size*count.
		if pos+4 > uint64(len(data)) {
			return stage, fmt.Errorf("dxcvalidator: precheck: no room for resource_binding_size: %w",
				ErrMalformedPSV0)
		}
		bindingSize := uint64(binary.LittleEndian.Uint32(data[pos : pos+4]))
		pos += 4
		total := bindingSize * uint64(resourceCount)
		if pos+total > uint64(len(data)) {
			return stage, fmt.Errorf("dxcvalidator: precheck: resource table walks past PSV0: %w",
				ErrMalformedPSV0)
		}
		pos += total
	}

	// String table: uint32 size + data.
	if pos+4 > uint64(len(data)) {
		return stage, fmt.Errorf("dxcvalidator: precheck: no room for string_table_size: %w",
			ErrMalformedPSV0)
	}
	stringTableSize := uint64(binary.LittleEndian.Uint32(data[pos : pos+4]))
	pos += 4
	stringTableStart := pos
	if pos+stringTableSize > uint64(len(data)) {
		return stage, fmt.Errorf("dxcvalidator: precheck: string_table_size=%d walks past PSV0: %w",
			stringTableSize, ErrMalformedPSV0)
	}

	// Validate the EntryFunctionName string.
	if uint64(entryFuncOffset) >= stringTableSize {
		return stage, fmt.Errorf("dxcvalidator: precheck: entry_func_offset=%d >= string_table_size=%d: %w",
			entryFuncOffset, stringTableSize, ErrEmptyEntryName)
	}
	stringTable := data[stringTableStart : stringTableStart+stringTableSize]
	// Entry name must be at least one non-null byte followed by a NUL terminator
	// somewhere inside the table.
	nameStart := uint64(entryFuncOffset)
	if nameStart >= uint64(len(stringTable)) || stringTable[nameStart] == 0 {
		return stage, fmt.Errorf("dxcvalidator: precheck: %w", ErrEmptyEntryName)
	}
	// Require a NUL terminator inside the table (defensive; the emit path
	// always writes one).
	foundTerminator := false
	for i := nameStart; i < uint64(len(stringTable)); i++ {
		if stringTable[i] == 0 {
			foundTerminator = true
			break
		}
	}
	if !foundTerminator {
		return stage, fmt.Errorf("dxcvalidator: precheck: entry name not NUL-terminated: %w",
			ErrMalformedPSV0)
	}

	return stage, nil
}

// isGraphicsStage reports whether the given PSVShaderKind stage byte
// corresponds to a stage that must carry ISG1 and OSG1 signature parts
// in the container (matches container.PSVShaderKind).
//
//	Pixel=0, Vertex=1, Geometry=2, Hull=3, Domain=4 — classic graphics
//	Mesh=13, Amplification=14 — modern mesh pipeline, both have I/O sigs
//
// Compute=5 and ray-tracing stages are excluded: they do not carry
// signature parts.
func isGraphicsStage(stage uint8) bool {
	switch stage {
	case 0, 1, 2, 3, 4, 13, 14:
		return true
	default:
		return false
	}
}
