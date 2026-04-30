// Package viewid computes input→output dataflow dependencies for graphics
// entry points, producing the data that populates DXIL's `dx.viewIdState`
// metadata (ComputeViewIdState.cpp format) and the PSV0 dependency table
// (PSVDependencyTable layout).
//
// BUG-DXIL-011 Phase 3 / BUG-DXIL-018 follow-up.
//
// The DXC reference lives at:
//
//	reference/dxil/dxc/lib/HLSL/ComputeViewIdStateBuilder.cpp
//
// That implementation is a full LLVM-based def-use walker with control-
// dependence and alloca reaching-def analysis. We implement a conservative
// IR-level pass-through analyzer that's sufficient for trivial
// pass-through shaders (triangle FS) and falls back to per-element
// "any-to-any" approximation for cases it can't trace precisely.
//
// # Scalar indexing
//
// Two DIFFERENT scalar indexing schemes are used:
//
//   - ViewIdState (`dx.viewIdState` metadata) uses PACKED scalar indices —
//     each signature element's scalars are consecutive, and the total
//     count is Σ numChannels. E.g. {vec4 position, vec3 color} → 7 total
//     scalars, with position=[0..4), color=[4..7).
//
//   - PSV0 dependency table uses COMPONENT-INDEXED layout — each signature
//     element occupies 4 components regardless of numChannels, with unused
//     trailing components reserved. E.g. {vec4 position, vec3 color} → 8
//     total components, with position=[0..4), color=[4..7) and comp 7
//     reserved/zero. The table row for input component i has length
//     MaskDwords(OutputVectors) dwords.
//
// Deps carries both layouts so the two serializers don't redo the
// analysis.
package viewid

import (
	"github.com/gogpu/naga/ir"
)

// Deps holds the computed dependency information for a graphics entry
// point. The ViewIdState metadata and PSV0 dependency table use the
// SAME scalar-indexed layout (confirmed via DXC DxilContainerAssembler
// CopyViewIDStateForOutputToPSV which memcpys the ViewIdState scalar
// table directly into PSV0's InputToOutputTable). The PSV allocated
// size is larger (MaskDwords * InputVectors * 4) but only the first
// MaskDwords * InputScalars dwords carry real data; the trailing
// component-padding slots stay zero.
type Deps struct {
	// NumInputScalars is Σ numChannels over input signature elements.
	// Matches ComputeViewIdState.cpp: m_NumInputSigScalars.
	NumInputScalars uint32

	// NumOutputScalars is Σ numChannels over output signature elements.
	// Non-GS shaders have a single stream; index 0 is the only one used.
	NumOutputScalars uint32

	// SigInputVectors is the number of input signature rows (each vector
	// occupies one row). Matches PSVRuntimeInfo1::SigInputVectors.
	SigInputVectors uint32

	// SigOutputVectors is the number of output signature rows.
	SigOutputVectors uint32

	// InputScalarToOutputs[inputScalar * outMaskDwords + w] is a bitmap
	// of output scalars that receive a contribution from this input
	// scalar. Layout matches ComputeViewIdState.cpp
	// SerializeInputsContributingToOutput:
	//
	//	pData[inputIdx * NumOutUINTs + w] |= (1u << b)
	//
	// where w = outputIdx / 32, b = outputIdx % 32. For shaders with
	// NumOutputScalars ≤ 32 (typical), NumOutUINTs = 1 and each entry is
	// a single uint32 bitmap.
	//
	// Size: NumInputScalars * MaskDwordsForScalars(NumOutputScalars).
	InputScalarToOutputs []uint32

	// InputCompToOutputComps[inputComp * outCompMaskDwords + w] is a
	// bitmap of output components that receive a contribution from this
	// input component. Layout matches PSVDependencyTable:
	//
	//	inputComp = inputVectorRow * 4 + inputCol
	//	outputComp = outputVectorRow * 4 + outputCol
	//	pData[inputComp * PSVMaskDwords(SigOutputVectors) + w] |= (1u << b)
	//
	// where w = outputComp / 32, b = outputComp % 32. Unused component
	// slots (e.g. TEXCOORD.w when the element is vec3) stay zero.
	//
	// Size: SigInputVectors * 4 * MaskDwordsForComponents(SigOutputVectors).
	InputCompToOutputComps []uint32
}

// MaskDwordsForScalars returns the number of uint32 dwords required to
// hold a per-scalar output bitmap with `n` output scalars. Mirrors DXC's
// RoundUpToUINT in ComputeViewIdState.cpp (line 208).
func MaskDwordsForScalars(n uint32) uint32 {
	return (n + 31) / 32
}

// MaskDwordsForComponents returns the number of uint32 dwords required to
// hold a per-component output bitmap with `outputVectors` output vectors
// (each vector = 4 components). Mirrors DXC's PSVComputeMaskDwordsFromVectors
// in DxilPipelineStateValidation.h:37.
func MaskDwordsForComponents(outputVectors uint32) uint32 {
	return (outputVectors + 7) >> 3
}

// SigElement describes a signature element's shape in the scalar/component
// index spaces. Both input and output elements are described the same way.
type SigElement struct {
	// ScalarStart is the starting scalar index in ViewIdState space
	// (cumulative sum of numChannels over preceding elements).
	ScalarStart uint32

	// NumChannels is the scalar width of this element (1..4).
	NumChannels uint32

	// VectorRow is the row index this element occupies in PSV0 space
	// (each row = 4 components). Preserved for PSV0 component indexing.
	VectorRow uint32

	// StartCol is the starting column within VectorRow. Used by
	// DetermineMaxPackedLocation to compute the linear scalar count:
	// max(StartRow*4 + StartCol + NumChannels) over all elements.
	StartCol uint32

	// SystemManaged is true for signature elements that do not occupy
	// a real row/col — e.g. SV_Depth on PS output, SV_SampleIndex on
	// PS input. These carry StartRow=-1, StartCol=-1 and the ViewIdState
	// analyzer treats their "scalars" as having no dataflow contribution.
	//
	// Currently unused in the IR-level analyzer but preserved for
	// future precision (DXC ComputeViewIdStateBuilder.cpp respects
	// this flag when walking stores).
	SystemManaged bool
}

// Analyze walks the entry point function body and computes dataflow
// dependencies between inputs and outputs.
//
// The analysis is conservative: when it cannot trace precisely (e.g.
// through local-variable flow with multiple stores, dynamic indexing,
// function calls), it reports "this input signature element contributes
// to every output scalar". For simple pass-through shaders the result is
// exact.
//
// The caller supplies pre-computed signature element descriptions — this
// decouples the IR-level walker from the DXIL emitter's sig collection
// machinery while still honoring the emit-order ↔ ViewIdState-scalar-
// order contract.
func Analyze(irMod *ir.Module, ep *ir.EntryPoint, inputs, outputs []SigElement) *Deps {
	var numInScalars, numOutScalars uint32
	var sigInVectors, sigOutVectors uint32

	// DXC's ComputeViewIdState uses PACKED LINEAR indexing for scalar
	// positions (GetLinearIndex = row*4 + col). NumInput/OutputSigScalars
	// is max(elem.VectorRow*4 + elem.NumChannels) over non-system-managed
	// elements, NOT sum of NumChannels. See DXC
	// ComputeViewIdStateBuilder.cpp DetermineMaxPackedLocation.
	for i := range inputs {
		if !inputs[i].SystemManaged {
			sigInVectors++
			end := inputs[i].VectorRow*4 + inputs[i].NumChannels
			if end > numInScalars {
				numInScalars = end
			}
		}
	}
	for i := range outputs {
		if !outputs[i].SystemManaged {
			sigOutVectors++
			end := outputs[i].VectorRow*4 + outputs[i].NumChannels
			if end > numOutScalars {
				numOutScalars = end
			}
		}
	}

	outMaskDwords := MaskDwordsForScalars(numOutScalars)
	outCompMaskDwords := MaskDwordsForComponents(sigOutVectors)

	deps := &Deps{
		NumInputScalars:        numInScalars,
		NumOutputScalars:       numOutScalars,
		SigInputVectors:        sigInVectors,
		SigOutputVectors:       sigOutVectors,
		InputScalarToOutputs:   make([]uint32, numInScalars*outMaskDwords),
		InputCompToOutputComps: make([]uint32, sigInVectors*4*outCompMaskDwords),
	}

	if numInScalars == 0 || numOutScalars == 0 {
		return deps
	}

	// Scalar -> input-component index map. Analyzer's internal taint uses
	// cumulative ScalarStart indexing, but DXC's ViewIdState layout uses
	// packed linear indexing (VectorRow*4 + StartCol + channelOffset).
	// StartCol is critical when multiple elements pack into the same
	// register row with different column offsets (e.g., two scalar
	// @location inputs sharing register 0 at columns 0 and 1).
	inScalarToComp := make([]uint32, 0)
	for i := range inputs {
		inp := &inputs[i]
		if inp.SystemManaged {
			continue
		}
		for c := uint32(0); c < inp.NumChannels; c++ {
			inScalarToComp = append(inScalarToComp, inp.VectorRow*4+inp.StartCol+c)
		}
	}

	// Run per-component taint propagation.
	state := newAnalysisState(irMod, ep, inputs, outputs)
	state.analyze()

	// Emit results in both layouts from the same taint result.
	emitTaintResults(deps, outputs, state, inScalarToComp, outMaskDwords, outCompMaskDwords)

	return deps
}

// emitTaintResults writes the per-component taint propagation results
// into both InputScalarToOutputs and InputCompToOutputComps bitmasks,
// remapping from the analyzer's cumulative indexing to DXC-style packed
// linear indexing (VectorRow*4 + col).
func emitTaintResults(
	deps *Deps,
	outputs []SigElement,
	state *analysisState,
	inScalarToComp []uint32,
	outMaskDwords, outCompMaskDwords uint32,
) {
	for outSigIdx, outSig := range outputs {
		if outSig.SystemManaged {
			continue
		}
		for outCol := uint32(0); outCol < outSig.NumChannels; outCol++ {
			outScalar := outSig.VectorRow*4 + outSig.StartCol + outCol // packed linear, matches DXC
			outComp := outSig.VectorRow*4 + outSig.StartCol + outCol
			taint := state.outputTaint[outSigIdx][outCol]
			for inScalar := range taint {
				// inScalar is the analyzer's cumulative index. Remap to
				// packed linear for the serialized mask.
				if int(inScalar) >= len(inScalarToComp) {
					continue
				}
				inPacked := inScalarToComp[inScalar]
				setBit(deps.InputScalarToOutputs, inPacked, outScalar, outMaskDwords)
				setBit(deps.InputCompToOutputComps, inPacked, outComp, outCompMaskDwords)
			}
		}
	}
}

// setBit sets a single bit at row `row`, column `col` in a per-row
// bitmap with `dwordsPerRow` dwords per row.
func setBit(buf []uint32, row, col, dwordsPerRow uint32) {
	idx := row*dwordsPerRow + col/32
	if idx >= uint32(len(buf)) { //nolint:gosec // len(buf) non-negative, fits uint32
		return
	}
	buf[idx] |= 1 << (col % 32)
}
