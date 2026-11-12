// Package container — naga IR binding to DXIL semantic mapping.
//
// Maps naga IR bindings (BuiltinBinding, LocationBinding) to DXIL
// semantic names and system value kinds used in ISG1/OSG1 signatures.
//
// Reference: Mesa dxil_signature.c fill_io_signature().
package container

import (
	"github.com/gogpu/naga/internal/d3d"
	"github.com/gogpu/naga/ir"
)

// DXIL system-value semantic name constants.
const (
	semPosition = "SV_Position"
	semTarget   = "SV_Target"
)

// SemanticMapping maps a naga IR binding to its DXIL semantic representation.
type SemanticMapping struct {
	SemanticName  string
	SemanticIndex uint32
	SystemValue   SystemValueKind
}

// MapBuiltinToSemantic converts a naga BuiltinValue to a DXIL semantic. ok is
// false for a built-in with no signature system value (the compute thread-id
// family, the virtual mesh built-ins) and for one the emitter cannot express.
// The default never invents an SV_Unknown name: callers decide what ok=false
// means, and the entry-point check refuses the built-ins DXIL cannot write.
func MapBuiltinToSemantic(builtin ir.BuiltinValue) (SemanticMapping, bool) {
	switch builtin {
	case ir.BuiltinPosition:
		return SemanticMapping{semPosition, 0, SVPosition}, true
	case ir.BuiltinVertexIndex:
		return SemanticMapping{"SV_VertexID", 0, SVVertexID}, true
	case ir.BuiltinInstanceIndex:
		return SemanticMapping{"SV_InstanceID", 0, SVInstanceID}, true
	case ir.BuiltinFrontFacing:
		return SemanticMapping{"SV_IsFrontFace", 0, SVIsFrontFace}, true
	case ir.BuiltinFragDepth:
		return SemanticMapping{"SV_Depth", 0, SVDepth}, true
	case ir.BuiltinSampleIndex:
		return SemanticMapping{"SV_SampleIndex", 0, SVSampleIndex}, true
	case ir.BuiltinSampleMask:
		// SV_Coverage is a pixel output only; on a pixel input it is not a
		// signature element (callers leave it out).
		return SemanticMapping{"SV_Coverage", 0, SVCoverage}, true
	case ir.BuiltinClipDistances:
		return SemanticMapping{"SV_ClipDistance", 0, SVClipDistance}, true
	case ir.BuiltinCullDistance:
		return SemanticMapping{"SV_CullDistance", 0, SVCullDistance}, true
	case ir.BuiltinPrimitiveIndex:
		return SemanticMapping{"SV_PrimitiveID", 0, SVPrimitiveID}, true
	case ir.BuiltinCullPrimitive:
		return SemanticMapping{"SV_CullPrimitive", 0, SVCullPrimitive}, true
	default:
		return SemanticMapping{}, false
	}
}

// MapLocationToInputSemantic converts a location binding to an input semantic.
// User-defined @location(N) inputs use LocationSemantic — must match
// wgpu/hal/dx12's D3D12_INPUT_ELEMENT_DESC.SemanticName (BUG-DXIL-028).
func MapLocationToInputSemantic(loc uint32) SemanticMapping {
	return SemanticMapping{d3d.LocationSemantic, loc, SVArbitrary}
}

// MapLocationToOutputSemantic converts a location binding to an output semantic.
// Fragment outputs map to SV_Target; non-fragment (VS / DS / HS / GS)
// outputs use LocationSemantic so VS→FS wiring stays consistent with
// the input side.
func MapLocationToOutputSemantic(loc uint32, isFragment bool) SemanticMapping {
	if isFragment {
		return SemanticMapping{semTarget, loc, SVTarget}
	}
	return SemanticMapping{d3d.LocationSemantic, loc, SVArbitrary}
}

// MapBindingToSemantic converts any naga IR binding to a DXIL semantic.
// isOutput and isFragment control the semantic mapping for LocationBindings.
// ok is false only for a built-in MapBuiltinToSemantic does not map.
func MapBindingToSemantic(binding ir.Binding, isOutput bool, isFragment bool) (SemanticMapping, bool) {
	switch b := binding.(type) {
	case ir.BuiltinBinding:
		return MapBuiltinToSemantic(b.Builtin)
	case ir.LocationBinding:
		if isOutput {
			return MapLocationToOutputSemantic(b.Location, isFragment), true
		}
		return MapLocationToInputSemantic(b.Location), true
	default:
		return SemanticMapping{}, false
	}
}
