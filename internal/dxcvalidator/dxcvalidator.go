// Package dxcvalidator wraps Microsoft's IDxcValidator (dxil.dll) so naga
// can run real DXIL validation against generated containers from Pure Go.
//
// The validator lives in dxil.dll, ships with the Windows 10 SDK, and is
// only available on Windows. On other platforms every call returns
// ErrUnsupported.
//
// dxil.dll initializes its allocator state via DllMain(DLL_THREAD_ATTACH)
// which only fires for OS threads created AFTER LoadLibrary returns. None
// of Go runtime's M's qualify. Run therefore spawns a fresh OS thread via
// kernel32!CreateThread, initializes the validator on that thread, and
// dispatches every Validate call back onto the same thread. A single Run
// invocation may issue many Validate calls.
package dxcvalidator

import "errors"

// Result is the outcome of a single Validate call.
type Result struct {
	// OK reports whether IDxcOperationResult::GetStatus returned S_OK.
	OK bool
	// Status is the raw HRESULT from GetStatus. Zero when OK.
	Status uint32
	// Error is the textual error blob returned by the validator. Empty
	// when OK.
	Error string
}

// ErrUnsupported is returned by Run when invoked on a non-Windows build.
var ErrUnsupported = errors.New("dxcvalidator: requires Windows")

// Validator wraps an IDxcValidator instance bound to a worker OS thread.
// Obtain one via Run. Validator is not safe for use outside the callback
// passed to Run.
type Validator struct {
	impl *validatorImpl
}

// Validate submits a DXBC container to IDxcValidator::Validate and returns
// the HRESULT plus any error blob. The blob is copied into the Windows
// process heap before the call because dxil.dll inspects buffer ownership
// via HeapSize.
func (v *Validator) Validate(dxil []byte) (Result, error) {
	return v.impl.validate(dxil)
}

// Run spawns a fresh OS thread, initializes IDxcValidator on it, invokes
// fn with a Validator handle bound to that thread, and tears the validator
// down on return. fn may call v.Validate any number of times; all calls
// run on the same thread.
//
// If fn returns an error, Run returns that error after teardown.
// On non-Windows: returns ErrUnsupported without invoking fn.
func Run(fn func(v *Validator) error) error {
	return runImpl(fn)
}
