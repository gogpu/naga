// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

// Package subgroupmask computes the GL_KHR_shader_subgroup ballot-mask
// built-in variables from the invocation's lane id and the subgroup size: the
// four words of the vec4<u32> each of gl_SubgroupEqMask, gl_SubgroupGeMask,
// gl_SubgroupGtMask, gl_SubgroupLeMask and gl_SubgroupLtMask holds.
//
// It is the semantic oracle for the MSL, HLSL and DXIL writers, which
// synthesize the masks because those backends have no such built-in (card
// P2-IR-042, ADR-014 D5). The equations are the extension's own (GL_KHR_
// shader_subgroup §3.8): the mask is "a bitmask of all invocations, with one
// bit per invocation", bit index i set when
//
//	Eq: i == lane        Ge: i >= lane        Gt: i >  lane
//	Le: i <= lane        Lt: i <  lane
//
// and bits above size-1 stay zero. size is a power of two, at most 128, and
// lane < size. Note that the mask spans every invocation of the subgroup, not
// the active ones — it is not the ballot of true, which holds only the active
// lanes.
package subgroupmask

// Kind is one of the five ballot-mask built-ins.
type Kind uint8

// The five mask kinds, in the order GLSL and SPIR-V list the built-ins.
const (
	Eq Kind = iota
	Ge
	Gt
	Le
	Lt
)

// Mask returns the four 32-bit words of a mask: word i holds the bits for
// invocations 32i to 32i+31, bit 0 of word 0 the invocation with lane id 0.
// A lane at or above size, or a kind outside the enum, yields the zero mask.
func Mask(kind Kind, lane, size uint32) [4]uint32 {
	var mask [4]uint32
	if lane >= size {
		return mask
	}
	for i := uint32(0); i < size && i < 128; i++ {
		var set bool
		switch kind {
		case Eq:
			set = i == lane
		case Ge:
			set = i >= lane
		case Gt:
			set = i > lane
		case Le:
			set = i <= lane
		case Lt:
			set = i < lane
		}
		if set {
			mask[i/32] |= 1 << (i % 32)
		}
	}
	return mask
}
