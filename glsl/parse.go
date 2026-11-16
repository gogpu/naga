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
	// multi-module caveat. For a GL 3.30 or ES 3.00 source it is the more
	// specific placement: the image's binding is fixed first (by
	// [ParseOptions.ResourceBindings] or the automatic counter), then this map
	// may place the split sampler; an entry wins over the next free slot.
	SamplerBindings frontend.SamplerBindings

	// ResourceBindings names the resource binding of a module-scope resource
	// that the source cannot bind itself (GL 3.30 core and ES 3.00 have no
	// layout(binding)), by source name: a uniform block (its type name when the
	// block has no instance name), a storage block, an image, a plain sampler,
	// the image of a combined image sampler, or the block the plain uniforms of
	// the module are collected into ([frontend.DefaultBlockName], see
	// [ParseInfo.DefaultBlock]). A supplied slot is reserved
	// before any declaration is lowered, so automatic placement skips it; two
	// names claiming one slot is a *ParseError. An entry that names no resource
	// needing a binding is a warning in [ParseInfo.Warnings], as design §11
	// (R5) requires.
	//
	// A Vulkan GLSL source (440/450/460) declares its own bindings and this map
	// is not consulted there at all: layout(binding) wins, an entry can neither
	// claim a slot nor move an automatic placement, and no diagnostic is
	// produced for it. The split sampler of a combined image sampler is placed
	// by [ParseOptions.SamplerBindings], not by this map.
	ResourceBindings map[string]ir.ResourceBinding
}

// ParseInfo is what [Parse] reports besides the module.
type ParseInfo struct {
	// Warnings are diagnostics that do not stop parsing. The producers are: an
	// unmatched ResourceBindings entry (design §11 R5), and, in the GL 3.30
	// core and ES 3.00 dialects, a #extension directive naming an extension the
	// frontend cannot place — such a directive is accepted as a no-op with a
	// warning (a deliberate deviation from GLSL 4.60 §3.3, card P2-GLSLIN-022).
	// Vulkan GLSL produces no #extension diagnostics.
	Warnings []frontend.Diagnostic

	// CombinedSamplers lists the splits the parse applied, in source order. A
	// caller that binds descriptors needs it to know where each sampler went.
	CombinedSamplers []frontend.CombinedSampler

	// DefaultBlock reports the uniform block the parse synthesized for the
	// plain (non-block) uniforms of a GL 3.30 core or ES 3.00 source, as
	// [frontend.DefaultBlockInfo] describes: the block's name and binding, its
	// std140 size and every member's offset. A caller allocates that many bytes,
	// writes each member at its reported offset and binds the buffer at the
	// reported binding; it must use the reported offsets rather than re-derive
	// the layout.
	//
	// It is nil when the module declares no plain uniform — a module with no
	// plain uniform consumes no binding for one — and always nil for a Vulkan
	// GLSL source, whose plain uniforms are refused: the Vulkan profile has no
	// default uniform block. To place the block explicitly, name
	// [frontend.DefaultBlockName] in [ParseOptions.ResourceBindings].
	DefaultBlock *frontend.DefaultBlockInfo
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
		DefaultBlock:     info.DefaultBlock,
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
