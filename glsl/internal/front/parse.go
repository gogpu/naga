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
	// itself (GL 3.30 and ES 3.00 have no layout(binding)). Not implemented
	// with the rest of the GL/ES binding model: Parse refuses a non-empty map
	// instead of ignoring it (design §11, Phase 4).
	ResourceBindings map[string]ir.ResourceBinding
}

// Info is what Parse reports besides the module, the public ParseInfo's source.
type Info struct {
	// Warnings are diagnostics that do not stop the parse.
	Warnings []frontend.Diagnostic
	// CombinedSamplers lists the splits the parse applied, in source order.
	CombinedSamplers []frontend.CombinedSampler
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

	if len(options.ResourceBindings) != 0 {
		return nil, Info{}, &ParseError{Errors: []frontError{*errMessage(ir.Span{},
			"Not implemented: ResourceBindings for a source without layout(binding)")}}
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

	tokens := lex(pp.Tokens)
	file := parse(tokens, pp.Version.Value)
	if len(file.Errors) != 0 {
		return nil, Info{}, &ParseError{Errors: file.Errors}
	}

	fe := newFrontend(&ir.Module{})
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

	info := Info{}
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
