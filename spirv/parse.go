package spirv

import (
	"errors"

	"github.com/gogpu/naga/frontend"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/front"
)

// ParseOptions configures [Parse]. The zero value is Rust naga's
// front::spv::Options::default behavior plus the combined-sampler split.
type ParseOptions struct {
	// KeepCoordinateSpace disables the default: the Y of a Position output is
	// negated on the way in, so IR space is Y-up like every other frontend
	// (Rust's adjust_coordinate_space = false). The SPIR-V backend's
	// [Options].AdjustCoordinateSpace points the other way on purpose; each is
	// documented at its field.
	KeepCoordinateSpace bool

	// AllowUnknownCapabilities parses despite capabilities outside the
	// supported set (Rust's strict_capabilities = false). The module will
	// still be refused by valid.Validate unless the device supports them.
	AllowUnknownCapabilities bool

	// CombinedImageSampler selects the treatment of combined image samplers,
	// the form glslc emits for `uniform sampler2D`. The zero value splits
	// each such variable into an image global on its declared binding and a
	// sampler global on a binding of its own (design §12);
	// CombinedImageSamplerReject refuses the module, as Rust naga does.
	CombinedImageSampler frontend.CombinedImageSamplerPolicy

	// SamplerBindings places the sampler of each split combined image
	// sampler; see frontend.SamplerBindings for the per-module resolution and
	// the multi-module caveat. Ignored with CombinedImageSamplerReject.
	SamplerBindings frontend.SamplerBindings
}

// ParseInfo is what [Parse] reports besides the module.
type ParseInfo struct {
	// Warnings are diagnostics that do not stop parsing: an unknown
	// decoration, an ignored NonSemantic instruction block, an ignored image
	// operand.
	Warnings []frontend.Diagnostic

	// CombinedSamplers lists the splits the parse applied, in source order. A
	// caller that binds descriptors needs it to know where each sampler went.
	CombinedSamplers []frontend.CombinedSampler
}

// ParseError is the error [Parse] returns. Its labels are frontend.Labels;
// unlike the WGSL frontend's error its affordance is an instruction offset,
// not a source location.
type ParseError struct {
	Message string
	// Offset is the byte offset of the offending instruction, 0 if none.
	Offset uint32
	// Labels and Notes are reserved for diagnostics that point into the
	// binary; the decoder reports the offset and the message today and fills
	// neither.
	Labels []frontend.Label
	Notes  []string
}

// Error returns the message.
func (e *ParseError) Error() string { return e.Message }

// Parse decodes a SPIR-V binary and lowers it to IR, as Rust naga's
// front::spv::parse_u8_slice. The module is not validated: call
// valid.Validate. A malformed or unsupported module is a *ParseError.
//
// The module is not compacted, as Rust's is not: validation and every
// backend accept it as parsed.
func Parse(data []byte, options ParseOptions) (*ir.Module, ParseInfo, error) {
	module, info, err := front.Parse(data, front.Options{
		KeepCoordinateSpace:      options.KeepCoordinateSpace,
		AllowUnknownCapabilities: options.AllowUnknownCapabilities,
		CombinedImageSampler:     options.CombinedImageSampler,
		SamplerBindings:          options.SamplerBindings,
	})
	if err != nil {
		var parseErr *front.Error
		if errors.As(err, &parseErr) {
			return nil, ParseInfo{}, &ParseError{Message: parseErr.Message, Offset: parseErr.Offset}
		}
		return nil, ParseInfo{}, err
	}
	return module, ParseInfo{
		Warnings:         info.Warnings,
		CombinedSamplers: info.CombinedSamplers,
	}, nil
}
