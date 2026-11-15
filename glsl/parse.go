package glsl

import (
	"fmt"

	"github.com/gogpu/naga/frontend"
	"github.com/gogpu/naga/glsl/internal/front"
	"github.com/gogpu/naga/internal/errfmt"
	"github.com/gogpu/naga/ir"
)

// ParseOptions configures [Parse]. The zero value parses a vertex shader with
// no defines, splitting combined image samplers as design §11 describes.
type ParseOptions struct {
	// Stage is the stage of the source. GLSL does not name its stage, so the
	// caller supplies it, as Rust naga's front::glsl::Options does. The zero
	// value is [ir.StageVertex]. The task and mesh stages are refused.
	Stage ir.ShaderStage

	// Defines are preprocessor definitions, as #define name value.
	Defines map[string]string

	// CombinedImageSampler selects the treatment of combined image samplers
	// (uniform sampler2D). The zero value splits each one into an image global
	// on its declared binding and a sampler global on a binding of its own;
	// CombinedImageSamplerReject refuses the declaration (naga's documented
	// refusal: Rust naga refuses it too, as an unimplemented variable
	// qualifier, gfx-rs/wgpu#5994).
	CombinedImageSampler frontend.CombinedImageSamplerPolicy

	// SamplerBindings places the sampler of each split combined image sampler;
	// see [frontend.SamplerBindings] for the per-module resolution and the
	// multi-module caveat.
	SamplerBindings frontend.SamplerBindings

	// ResourceBindings names the resource binding of a module-scope resource
	// that the source cannot bind itself (GL 3.30 core and ES 3.00 have no
	// layout(binding)): a uniform block, a storage block, an image or a split
	// sampler, by source name. Ignored where the source declares the binding
	// itself (Vulkan GLSL).
	//
	// Not implemented yet, and neither is the rest of the GL/ES binding model:
	// a resource without layout(binding) is a *ParseError today, so GL 3.30 and
	// ES 3.00 shaders that declare one do not parse (design §11; Phase 4).
	// Parse reports a *ParseError when this map is not empty rather than
	// ignoring it.
	ResourceBindings map[string]ir.ResourceBinding
}

// ParseInfo is what [Parse] reports besides the module.
type ParseInfo struct {
	// Warnings are diagnostics that do not stop parsing: an ignored pragma, an
	// extension enabled but not required, a location assigned implicitly.
	Warnings []frontend.Diagnostic

	// CombinedSamplers lists the splits the parse applied, in source order. A
	// caller that binds descriptors needs it to know where each sampler went.
	CombinedSamplers []frontend.CombinedSampler
}

// ParseError is the error [Parse] returns, in the WGSL frontend's shape: a
// message, the spans it is about (the first is primary) and notes. Every
// independent error the parse collected after the first becomes a note.
type ParseError struct {
	Message string
	Labels  []frontend.Label
	Notes   []string
}

// Error returns the message.
func (e *ParseError) Error() string { return e.Message }

// Location returns the line and column of the primary label in source, and
// false when the error has no label, as Rust's ParseError::location.
func (e *ParseError) Location(source string) (ir.SourceLocation, bool) {
	return errfmt.Location(e.Labels, source)
}

// Format renders the error against source, attributed to path, in the layout
// of Rust naga's emit_to_string_with_path (codespan). See [wgsl.ParseError]
// for the shape; both frontends render through internal/errfmt.
func (e *ParseError) Format(source, path string) string {
	return errfmt.Format(e.Message, e.Labels, e.Notes, source, path)
}

// Parse preprocesses, parses and lowers GLSL source into an IR module, as Rust
// naga's front::glsl::Frontend::parse. The module is not validated: call
// valid.Validate. A syntax or semantic error is a *ParseError.
//
// The module is not compacted, as Rust's is not: validation and every backend
// accept it as parsed.
func Parse(source string, options ParseOptions) (*ir.Module, ParseInfo, error) {
	module, info, parseErr := front.Parse(source, front.Options{
		Stage:                options.Stage,
		Defines:              options.Defines,
		CombinedImageSampler: options.CombinedImageSampler,
		SamplerBindings:      options.SamplerBindings,
		ResourceBindings:     options.ResourceBindings,
	})
	if parseErr != nil {
		return nil, ParseInfo{}, publicError(parseErr, source)
	}
	return module, ParseInfo{
		Warnings:         info.Warnings,
		CombinedSamplers: info.CombinedSamplers,
	}, nil
}

// publicError maps the frontend's error list to the public shape (design §3,
// R1): the first error is the message and its label, every later independent
// error becomes a note as line:col: message. The line index is built once:
// Span.Location scans the source prefix per call, which turns quadratic when
// a source carries one diagnostic per line (deep validation, 2026-11-15).
func publicError(parseErr *front.ParseError, source string) *ParseError {
	pub := &ParseError{}
	if len(parseErr.Errors) == 0 {
		pub.Message = "GLSL parse failed"
		return pub
	}
	lines := errfmt.NewSourceLines(source)
	first := parseErr.Errors[0]
	pub.Message = first.Message
	if first.Span.IsDefined() {
		pub.Labels = []frontend.Label{{Span: first.Span}}
	}
	for _, extra := range parseErr.Errors[1:] {
		if !extra.Span.IsDefined() {
			pub.Notes = append(pub.Notes, extra.Message)
			continue
		}
		loc := lines.Location(source, extra.Span)
		pub.Notes = append(pub.Notes, fmt.Sprintf("%d:%d: %s", loc.Line, loc.Column, extra.Message))
	}
	return pub
}
