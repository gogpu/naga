package front

import (
	"github.com/gogpu/naga/frontend"
	"github.com/gogpu/naga/ir"
)

// combinedSamplerSplit is one combined image sampler the module declares: the
// image and sampler globals it was split into, and the binding it was
// declared with. The sampler's binding is resolved after every global is
// parsed, because the next free binding depends on the whole module.
type combinedSamplerSplit struct {
	image    ir.GlobalVariableHandle
	sampler  ir.GlobalVariableHandle
	declared ir.ResourceBinding
}

// splitCombinedSampler turns a combined image sampler variable into the image
// and sampler globals the IR needs, as Tint's ResolveBindingConflictsPass
// does (design §12): the image keeps the declared (set, binding), the sampler
// gets a binding of its own.
func (p *parser) splitCombinedSampler(id, typeID uint32, dec *decoration, imageType ir.TypeHandle) error {
	if p.options.CombinedImageSampler == frontend.CombinedImageSamplerReject {
		return errCombinedSamplerRejected(id)
	}
	declared := dec.resourceBinding()
	if declared == nil {
		return errCombinedSamplerWithoutBinding(id)
	}
	image := ir.GlobalVariable{
		Name:        dec.name,
		NamePresent: dec.namePresent(),
		Space:       ir.SpaceHandle,
		Binding:     declared,
		Type:        imageType,
	}
	sampler := ir.GlobalVariable{
		Space: ir.SpaceHandle,
		Type:  p.insertType("", false, ir.SamplerType{}),
	}
	imageHandle := ir.GlobalVariableHandle(len(p.module.GlobalVariables))
	p.module.GlobalVariables = append(p.module.GlobalVariables, image)
	samplerHandle := ir.GlobalVariableHandle(len(p.module.GlobalVariables))
	p.module.GlobalVariables = append(p.module.GlobalVariables, sampler)

	if p.canComparisonSample(imageType) {
		p.handleSampling[imageHandle] = 0
	}
	p.handleSampling[samplerHandle] = 0
	p.combinedSamplerSplits = append(p.combinedSamplerSplits, combinedSamplerSplit{
		image:    imageHandle,
		sampler:  samplerHandle,
		declared: *declared,
	})
	p.lookupVariable[id] = lookupVariable{
		class:   classGlobal,
		handle:  imageHandle,
		typeID:  typeID,
		sampler: &samplerHandle,
	}
	return nil
}

// resolveCombinedSamplerBindings gives every split sampler its binding: the
// one ParseOptions.SamplerBindings names for the declared binding, or the
// lowest binding of the same group that no global of the module uses yet. It
// fills ParseInfo.CombinedSamplers in source order. A mapping onto a binding
// another resource of the module already uses is an error: the caller asked
// for two globals to share a slot.
func (p *parser) resolveCombinedSamplerBindings() error {
	if len(p.combinedSamplerSplits) == 0 {
		return nil
	}
	used := make(map[uint32]map[uint32]struct{})
	mark := func(binding ir.ResourceBinding) {
		group, ok := used[binding.Group]
		if !ok {
			group = make(map[uint32]struct{})
			used[binding.Group] = group
		}
		group[binding.Binding] = struct{}{}
	}
	for _, global := range p.module.GlobalVariables {
		if global.Binding != nil {
			mark(*global.Binding)
		}
	}

	for _, split := range p.combinedSamplerSplits {
		var samplerBinding ir.ResourceBinding
		if mapped, ok := p.options.SamplerBindings[split.declared]; ok {
			if _, taken := used[mapped.Group][mapped.Binding]; taken {
				return errCombinedSamplerBindingTaken(split.declared, mapped)
			}
			samplerBinding = mapped
		} else {
			samplerBinding = ir.ResourceBinding{Group: split.declared.Group}
			for {
				if _, taken := used[samplerBinding.Group][samplerBinding.Binding]; !taken {
					break
				}
				samplerBinding.Binding++
			}
		}
		mark(samplerBinding)
		p.module.GlobalVariables[split.sampler].Binding = &samplerBinding
		p.info.CombinedSamplers = append(p.info.CombinedSamplers, frontend.CombinedSampler{
			Sampler: split.declared,
			Image:   split.declared,
			Split:   samplerBinding,
		})
	}
	return nil
}
