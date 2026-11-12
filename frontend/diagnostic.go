// Package frontend holds the types shared by the source frontends (WGSL,
// SPIR-V, GLSL) and the callers of their Parse functions: the diagnostic
// vocabulary and the combined image sampler policy with its binding remap.
package frontend

import "github.com/gogpu/naga/ir"

// Diagnostic is a message about a span: a range of source text, or of the
// binary module for a binary frontend.
type Diagnostic struct {
	Message string
	Span    ir.Span
}

// Label marks a span in a parse error. Text says what the span is.
type Label struct {
	Span ir.Span
	Text string
}
