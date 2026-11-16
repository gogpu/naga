package front

// This file is the frontend's entry point: preprocess, lex, parse, lower
// (design §5). The public glsl.Parse is a thin wrapper over it.

import (
	"github.com/gogpu/naga/frontend"
	"github.com/gogpu/naga/ir"
)

// Options is what the public ParseOptions carries into the frontend.
type Options struct {
	// Stage is the shader stage; GLSL does not name it in the source.
	Stage ir.ShaderStage
	// Defines are preprocessor definitions, as #define name value.
	Defines map[string]string
	// CombinedImageSampler selects the treatment of combined image samplers.
	CombinedImageSampler frontend.CombinedImageSamplerPolicy
	// SamplerBindings places the sampler of each split combined image
	// sampler; see frontend.SamplerBindings for the resolution rule.
	SamplerBindings frontend.SamplerBindings
	// ResourceBindings names the binding of a resource the source cannot bind
	// itself (GL 3.30 and ES 3.00 have no layout(binding)), by source name:
	// design §11 (R2), bindings.go. Vulkan GLSL declares its bindings and
	// consults no map. The name of the block the plain uniforms are collected
	// into (defaultBlockName) is one of the keys.
	ResourceBindings map[string]ir.ResourceBinding
}

// Info is what Parse reports besides the module, the public ParseInfo's source.
type Info struct {
	// Warnings are diagnostics that do not stop the parse.
	Warnings []frontend.Diagnostic
	// CombinedSamplers lists the splits the parse applied, in source order.
	CombinedSamplers []frontend.CombinedSampler
	// DefaultBlock reports the block a GL 3.30 core or ES 3.00 source's plain
	// uniforms were collected into, and is nil when the module declares none
	// and for every Vulkan GLSL source (ADR-013).
	DefaultBlock *frontend.DefaultBlockInfo
}

// ParseError is a parse or lowering failure with every error the run
// collected: the first is the primary one and the others become the public
// ParseError's notes (design §3, R1). The parse always returns one error value.
type ParseError struct {
	Errors []frontError
}

// Error returns the primary error's message.
func (e *ParseError) Error() string {
	if len(e.Errors) == 0 {
		return "GLSL parse failed"
	}
	return e.Errors[0].Message
}

// Parse preprocesses, parses and lowers GLSL source into an IR module. The
// module is not validated: call valid.Validate. A syntax or semantic error is
// a *ParseError.
func Parse(source string, options Options) (*ir.Module, Info, *ParseError) {
	// The stage is the caller's, and only the three stages GLSL has here are
	// accepted (design §14, R5).
	switch options.Stage {
	case ir.StageVertex, ir.StageFragment, ir.StageCompute:
	default:
		return nil, Info{}, &ParseError{Errors: []frontError{*errMessage(ir.Span{},
			"the GLSL frontend supports the vertex, fragment and compute stages, not "+
				options.Stage.String())}}
	}

	pp := preprocess(source, options.Defines)
	if len(pp.Errors) != 0 {
		return nil, Info{}, ppFailure(pp.Errors)
	}
	if !pp.HasVersion {
		return nil, Info{}, &ParseError{Errors: []frontError{*errMessage(ir.Span{},
			"Missing #version directive")}}
	}
	// The preprocessor has already checked the version and its profile against
	// the dialect matrix of design §15 (pp_directive.go versionDirective).
	d := dialect{version: pp.Version.Value, profile: pp.Version.Profile}

	// The compute stage is Vulkan GLSL only: glslang, for both 330 core and 300
	// es, "compute shaders require es profile with version 310 or above, or
	// non-es profile with version 420 or above".
	if options.Stage == ir.StageCompute && !d.supportsCompute() {
		return nil, Info{}, &ParseError{Errors: []frontError{*errMessage(pp.Version.Span,
			"compute shaders require es profile with version 310 or above, or non-es profile with version 420 or above")}}
	}

	// The #extension behavior rules need the directive list, which the
	// preprocessor records. Vulkan GLSL gets no extension diagnostics (Rust
	// naga never examines the directive); in the GL/ES dialects an unsupported
	// require is a warning, so parsing goes on.
	extensionErrors, extensionWarnings := checkExtensions(d, pp.Extensions)
	if len(extensionErrors) != 0 {
		return nil, Info{}, &ParseError{Errors: extensionErrors}
	}

	tokens := lex(pp.Tokens)
	file := parse(tokens, d, options.Stage, pp.Extensions)
	if len(file.Errors) != 0 {
		return nil, Info{}, &ParseError{Errors: file.Errors}
	}

	fe := newFrontendFor(&ir.Module{}, d, options.ResourceBindings)
	fe.resetMeta(options.Stage)
	fe.combinedPolicy = options.CombinedImageSampler
	for from, to := range options.SamplerBindings {
		fe.samplerBindings[resourceKey{group: from.Group, binding: from.Binding}] = to.Binding
	}

	if err := fe.lowerModule(file); err != nil {
		fe.addError(err)
	}
	if len(fe.errors) != 0 {
		return nil, Info{}, &ParseError{Errors: fe.errors}
	}

	info := Info{Warnings: append(extensionWarnings, fe.warnings...)}
	info.DefaultBlock = fe.defaultBlockReport
	for _, split := range fe.combinedSamplers {
		info.CombinedSamplers = append(info.CombinedSamplers, frontend.CombinedSampler{
			Sampler: split.imageBinding,
			Image:   split.imageBinding,
			Split:   split.samplerBinding,
		})
	}
	return fe.mod, info, nil
}

// ppFailure turns preprocessor errors into one parse error, Rust's
// PreprocessorError wording: the message stands alone.
func ppFailure(errs []ppError) *ParseError {
	out := make([]frontError, 0, len(errs))
	for _, err := range errs {
		out = append(out, frontError(err))
	}
	return &ParseError{Errors: out}
}
