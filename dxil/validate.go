package dxil

import (
	"errors"
	"fmt"

	"github.com/gogpu/naga/internal/dxcvalidator"
	"github.com/gogpu/naga/internal/dxcvalidator/bitcheck"
)

// ValidationLevel selects how deeply Validate inspects a DXIL container.
type ValidationLevel int

const (
	// ValidateStructural runs PreCheckContainer only — DXBC header,
	// part list, PSV0/ISG1/OSG1 presence, PSV0 stage byte sanity. Fast,
	// cross-platform, catches the big structural bugs that dxil.dll
	// would reject with HRESULT 0x80aa0004..6 family.
	ValidateStructural ValidationLevel = iota

	// ValidateBitcode extends ValidateStructural with a defensive
	// LLVM 3.7 bitstream walk (bitcheck.Check) that catches null
	// dx.entryPoints function pointers and malformed METADATA_BLOCK
	// records before IDxcValidator access-violates on them.
	ValidateBitcode

	// ValidateFull extends ValidateBitcode with a real IDxcValidator
	// call (Windows-only; on non-Windows platforms degrades to
	// ValidateBitcode silently). Returns the exact HRESULT + error
	// string that dxil.dll would report, matching what D3D12
	// CreateGraphicsPipelineState sees internally.
	ValidateFull
)

// Validate inspects a DXBC container produced by Compile and returns a
// non-nil error if any layer rejects it. Intended to be called by
// consumers (e.g. wgpu hal/dx12) right after Compile so that bogus
// bytecode surfaces a readable diagnostic *before* the D3D12 runtime
// folds it into an opaque E_INVALIDARG at pipeline creation time.
//
// The three levels form an inclusion chain; picking Full on Linux or
// macOS is safe — it simply stops one layer short.
func Validate(blob []byte, level ValidationLevel) error {
	if err := dxcvalidator.PreCheckContainer(blob); err != nil {
		return err
	}
	if level >= ValidateBitcode {
		if err := bitcheck.Check(blob); err != nil {
			return err
		}
	}
	if level >= ValidateFull {
		return runDxcValidation(blob)
	}
	return nil
}

// runDxcValidation invokes IDxcValidator and returns its verdict.
// On non-Windows platforms ErrUnsupported is silently ignored.
func runDxcValidation(blob []byte) error {
	var reject error
	runErr := dxcvalidator.Run(func(v *dxcvalidator.Validator) error {
		res, err := v.Validate(blob)
		if err != nil {
			return err
		}
		if !res.OK {
			reject = fmt.Errorf("IDxcValidator: HRESULT 0x%08x: %s", res.Status, res.Error)
		}
		return nil
	})
	if runErr != nil {
		if errors.Is(runErr, dxcvalidator.ErrUnsupported) {
			return nil
		}
		return runErr
	}
	return reject
}
