// bitcheck.go — top-level entry point.
//
// Check(blob) is the second defensive layer in the dxcvalidator
// wrapper. It runs AFTER PreCheckContainer's fixed-offset structural
// checks and BEFORE the blob is handed to IDxcValidator::Validate.
//
// The pipeline is:
//
//	extractBitcode  — DXBC container → DxilProgramHeader → LLVM bitcode
//	NewReader       — bit-level primitives
//	NewBlockReader  — block / record dispatch with abbrev tables
//	walkMetadata    — METADATA_BLOCK walker, declaration-order index
//	verifyEntryPoints — null-check !dx.entryPoints tuples
//
// Every layer returns a typed Go error on malformed input. No panics
// on malformed input. See doc.go for the AV class this guards against.

package bitcheck

import (
	"errors"
	"fmt"
	"os"
)

// Top-level block IDs we recognize.
const (
	blockIDModule   = 8
	blockIDMetadata = 15
)

// Check scans a DXBC container blob for the null-function-reference
// pattern that makes IDxcValidator AV at dxil.dll+0xe9da. On success
// returns nil. On failure returns a typed error from the set:
//
//	ErrNoBitcode               — no DXIL part / bad program header / bad magic
//	ErrMalformedBitstream      — structural violation inside the bitstream
//	ErrMissingEntryPoints      — no dx.entryPoints named metadata
//	ErrNullEntryPointFunction  — tuple operand 0 is null
//	ErrEmptyEntryPointTuple    — tuple has zero operands
//
// All errors are wrapped via fmt.Errorf for context; use errors.Is to
// switch on the sentinel.
func Check(blob []byte) error {
	err := checkInternal(blob)
	if err == nil {
		return nil
	}
	// Translate bitstream-level sentinels (ErrUnexpectedEOF, ErrVBRTooWide,
	// ErrInvalidWidth) into the public ErrMalformedBitstream so callers can
	// switch on a single high-level sentinel without touching the internal
	// bit-reader error taxonomy. Higher-level sentinels (ErrNoBitcode,
	// ErrMissingEntryPoints, ErrNullEntryPointFunction, ErrEmptyEntryPointTuple)
	// pass through unchanged.
	switch {
	case errors.Is(err, ErrMalformedBitstream),
		errors.Is(err, ErrNoBitcode),
		errors.Is(err, ErrMissingEntryPoints),
		errors.Is(err, ErrNullEntryPointFunction),
		errors.Is(err, ErrEmptyEntryPointTuple):
		return err
	case errors.Is(err, ErrUnexpectedEOF),
		errors.Is(err, ErrVBRTooWide),
		errors.Is(err, ErrInvalidWidth):
		return fmt.Errorf("%w: %w", ErrMalformedBitstream, err)
	default:
		return err
	}
}

// checkInternal runs the full bitcheck pipeline without translating
// bitstream-level errors. Check() wraps this to expose a stable sentinel.
func checkInternal(blob []byte) error {
	bitcode, err := extractBitcode(blob)
	if err != nil {
		return err
	}
	// The LLVM bitstream begins with the 4-byte magic, which the
	// writer emits via WriteBits (not as raw bytes), so the reader
	// must consume it from the bitstream too. Starting abbrev width
	// at the top level is 2 (mirrors bitcode.NewWriter(2)).
	r := NewReader(bitcode, 2)
	if _, err := r.ReadFixed(32); err != nil {
		return fmt.Errorf("bitcheck: read bitstream magic: %w", err)
	}
	br := NewBlockReader(r)
	return walkTopLevel(br)
}

// walkTopLevel walks the top-level bitstream scope looking for the
// MODULE_BLOCK. If found, it recurses into walkModule. Any other
// top-level block is skipped via SkipBlock.
func walkTopLevel(br *BlockReader) error {
	for {
		e, err := br.Next()
		if err != nil {
			return fmt.Errorf("bitcheck: top-level next: %w", err)
		}
		switch e.Kind {
		case entryEOF:
			return fmt.Errorf("bitcheck: no MODULE_BLOCK found: %w", ErrNoBitcode)
		case entryEnd:
			// END at top level shouldn't happen — there's no enclosing
			// block. Treat as malformed.
			return fmt.Errorf("bitcheck: unexpected END at top level: %w",
				ErrMalformedBitstream)
		case entryDefineAbbrev:
			if err := br.ReadDefineAbbrev(); err != nil {
				return fmt.Errorf("bitcheck: top-level define-abbrev: %w", err)
			}
			continue
		case entryRecord:
			if _, err := br.ReadRecord(e.AbbrevID); err != nil {
				return fmt.Errorf("bitcheck: top-level record: %w", err)
			}
			continue
		case entrySubBlock:
			if e.BlockID != blockIDModule {
				if err := br.SkipBlock(); err != nil {
					return fmt.Errorf("bitcheck: skip top-level block %d: %w",
						e.BlockID, err)
				}
				continue
			}
			if err := br.EnterBlock(); err != nil {
				return fmt.Errorf("bitcheck: enter MODULE_BLOCK: %w", err)
			}
			return walkModule(br)
		}
	}
}

// walkModule walks the MODULE_BLOCK body looking for METADATA_BLOCK.
// Every non-metadata sub-block is skipped via SkipBlock. Records are
// consumed and discarded (they are not needed for the entry-point
// null check).
//
// DXC 1.8 emits TWO METADATA_BLOCKs in MODULE_BLOCK: the first carries
// the named metadata nodes (dx.entryPoints, dx.version, ...) and the
// second is the per-function attachment metadata (often empty when no
// debug info is present). To accept canonical DXC output we must
// consider "dx.entryPoints present in ANY block" as success rather than
// insisting that the first-seen block carries it. BUG-DXIL-024.
func walkModule(br *BlockReader) error {
	state := &metadataState{}
	for {
		e, err := br.Next()
		if err != nil {
			return fmt.Errorf("bitcheck: module next: %w", err)
		}
		done, err := handleModuleEntry(br, e, state)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
	}
}

// metadataState tracks cross-block metadata findings inside a single
// MODULE_BLOCK walk. handleMetadataBlock reports each METADATA_BLOCK's
// dx.entryPoints findings into state. At MODULE_BLOCK end, walkModule
// surfaces the merged result (or ErrMissingEntryPoints when no block
// declared the named node).
type metadataState struct {
	foundAnyBlock    bool  // at least one METADATA_BLOCK was encountered
	foundEntryPoints bool  // some METADATA_BLOCK declared dx.entryPoints
	firstErr         error // verification error from the first block that had entry points
}

// handleModuleEntry dispatches a single BlockReader entry inside
// MODULE_BLOCK. Returns done=true only when the block has been fully
// consumed (entryEnd). On metadata sub-block, walks metadata and merges
// entry-point findings into state.
func handleModuleEntry(br *BlockReader, e Entry, state *metadataState) (bool, error) {
	switch e.Kind {
	case entryEnd:
		if err := br.ExitBlock(); err != nil {
			return false, fmt.Errorf("bitcheck: exit MODULE_BLOCK: %w", err)
		}
		if !state.foundAnyBlock {
			return true, ErrMissingEntryPoints
		}
		if !state.foundEntryPoints {
			return true, ErrMissingEntryPoints
		}
		if state.firstErr != nil {
			return true, state.firstErr
		}
		return true, nil
	case entryEOF:
		return false, fmt.Errorf("bitcheck: unexpected EOF inside MODULE_BLOCK: %w",
			ErrMalformedBitstream)
	case entryDefineAbbrev:
		if err := br.ReadDefineAbbrev(); err != nil {
			return false, fmt.Errorf("bitcheck: module define-abbrev: %w", err)
		}
		return false, nil
	case entryRecord:
		if _, err := br.ReadRecord(e.AbbrevID); err != nil {
			return false, fmt.Errorf("bitcheck: module record: %w", err)
		}
		return false, nil
	case entrySubBlock:
		if e.BlockID != blockIDMetadata {
			if err := br.SkipBlock(); err != nil {
				return false, fmt.Errorf("bitcheck: skip sub-block %d: %w",
					e.BlockID, err)
			}
			return false, nil
		}
		if err := handleMetadataBlock(br, state); err != nil {
			return false, err
		}
		state.foundAnyBlock = true
		return false, nil
	}
	return false, nil
}

// handleMetadataBlock enters a METADATA_BLOCK, walks it, exits cleanly,
// then merges its entry-point verification result into state. A canonical
// DXC output carries dx.entryPoints in the first (named) METADATA_BLOCK
// and has an empty second (function-attachment) block; we must not let
// the empty block overwrite the success from the first. BUG-DXIL-024.
func handleMetadataBlock(br *BlockReader, state *metadataState) error {
	if err := br.EnterBlock(); err != nil {
		return fmt.Errorf("bitcheck: enter METADATA_BLOCK: %w", err)
	}
	tbl, err := walkMetadata(br)
	if err != nil {
		return fmt.Errorf("bitcheck: walk metadata: %w", err)
	}
	if err := br.ExitBlock(); err != nil {
		return fmt.Errorf("bitcheck: exit METADATA_BLOCK: %w", err)
	}
	if bitcheckTrace {
		fmt.Fprintf(os.Stderr, "[bitcheck] metadata block done: %d nodes, %d named entries: ",
			len(tbl.nodes), len(tbl.namedEntries))
		for k := range tbl.namedEntries {
			fmt.Fprintf(os.Stderr, "%q ", k)
		}
		fmt.Fprintln(os.Stderr)
	}
	if _, ok := tbl.namedEntries[dxEntryPointsName]; !ok {
		// This block does not declare dx.entryPoints. For the canonical
		// DXC layout the second METADATA_BLOCK is empty; accept it
		// silently. The outer walker will still report
		// ErrMissingEntryPoints if NO block ever declared it.
		return nil
	}
	state.foundEntryPoints = true
	if err := tbl.verifyEntryPoints(); err != nil && state.firstErr == nil {
		state.firstErr = err
	}
	return nil
}
