package frontend

import "github.com/gogpu/naga/ir"

// CombinedImageSamplerPolicy selects the treatment of combined image samplers.
type CombinedImageSamplerPolicy uint8

const (
	// CombinedImageSamplerSplit (zero value) splits a combined image sampler
	// into an image global and a sampler global.
	CombinedImageSamplerSplit CombinedImageSamplerPolicy = iota
	// CombinedImageSamplerReject refuses a combined image sampler, as Rust
	// naga does.
	CombinedImageSamplerReject
)

// SamplerBindings maps the declared (group, binding) of a combined image
// sampler to the binding of the sampler global it is split into.
//
// Resolution is per module and deterministic, in source order: without an
// entry a sampler takes the next free binding in the same group within that
// module. Two modules parsed separately (a vertex and a fragment stage) can
// therefore give different samplers the same "next free" binding and collide
// in one pipeline layout; a caller that builds such a layout supplies the map,
// or reconciles the ParseInfo reports before creating the layout.
type SamplerBindings map[ir.ResourceBinding]ir.ResourceBinding

// CombinedSampler is one resolved split, in source order.
type CombinedSampler struct {
	Sampler ir.ResourceBinding // where the combined image sampler was declared
	Image   ir.ResourceBinding // the image global's binding (unchanged)
	Split   ir.ResourceBinding // the sampler global's binding
}
