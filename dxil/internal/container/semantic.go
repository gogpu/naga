// Package container — naga IR binding to DXIL semantic mapping.
//
// Maps naga IR bindings (BuiltinBinding, LocationBinding) to DXIL
// semantic names and system value kinds used in ISG1/OSG1 signatures.
//
// Reference: Mesa dxil_signature.c fill_io_signature().
package container

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// SemanticMapping maps a naga IR binding to its DXIL semantic representation.
type SemanticMapping struct {
	SemanticName  string
	SemanticIndex uint32
	SystemValue   SystemValueKind
}

// MapBuiltinToSemantic converts a naga BuiltinValue to a DXIL semantic.
func MapBuiltinToSemantic(builtin ir.BuiltinValue) SemanticMapping {
	switch builtin {
	case ir.BuiltinPosition:
		return SemanticMapping{"SV_Position", 0, SVPosition}
	case ir.BuiltinVertexIndex:
		return SemanticMapping{"SV_VertexID", 0, SVVertexID}
	case ir.BuiltinInstanceIndex:
		return SemanticMapping{"SV_InstanceID", 0, SVInstanceID}
	case ir.BuiltinFrontFacing:
		return SemanticMapping{"SV_IsFrontFace", 0, SVIsFrontFace}
	case ir.BuiltinFragDepth:
		return SemanticMapping{"SV_Depth", 0, SVDepth}
	case ir.BuiltinSampleIndex:
		return SemanticMapping{"SV_SampleIndex", 0, SVSampleIndex}
	case ir.BuiltinClipDistance:
		return SemanticMapping{"SV_ClipDistance", 0, SVClipDistance}
	default:
		return SemanticMapping{fmt.Sprintf("SV_Unknown%d", builtin), 0, SVArbitrary}
	}
}

// MapLocationToInputSemantic converts a location binding to an input semantic.
// Input locations use TEXCOORD semantics by convention.
func MapLocationToInputSemantic(loc uint32) SemanticMapping {
	return SemanticMapping{"TEXCOORD", loc, SVArbitrary}
}

// MapLocationToOutputSemantic converts a location binding to an output semantic.
// For fragment shader outputs, locations map to SV_Target.
// For vertex shader outputs, locations use TEXCOORD.
func MapLocationToOutputSemantic(loc uint32, isFragment bool) SemanticMapping {
	if isFragment {
		return SemanticMapping{"SV_Target", loc, SVTarget}
	}
	return SemanticMapping{"TEXCOORD", loc, SVArbitrary}
}

// MapBindingToSemantic converts any naga IR binding to a DXIL semantic.
// isOutput and isFragment control the semantic mapping for LocationBindings.
func MapBindingToSemantic(binding ir.Binding, isOutput bool, isFragment bool) SemanticMapping {
	switch b := binding.(type) {
	case ir.BuiltinBinding:
		return MapBuiltinToSemantic(b.Builtin)
	case ir.LocationBinding:
		if isOutput {
			return MapLocationToOutputSemantic(b.Location, isFragment)
		}
		return MapLocationToInputSemantic(b.Location)
	default:
		return SemanticMapping{"UNKNOWN", 0, SVArbitrary}
	}
}
