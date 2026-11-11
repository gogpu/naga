// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// HLSL type name constants.
const (
	hlslInt     = "int"
	hlslTexture = "Texture"
)

// ScalarToHLSL returns the HLSL type name for a scalar type.
// Ref: https://docs.microsoft.com/en-us/windows/win32/direct3dhlsl/dx-graphics-hlsl-scalar
func ScalarToHLSL(s ir.ScalarType) string {
	switch s.Kind {
	case ir.ScalarBool:
		return "bool"
	case ir.ScalarSint:
		switch s.Width {
		case 2:
			return hlslInt16
		case 4:
			return hlslInt
		case 8:
			return hlslInt64
		default:
			return hlslInt // Fallback for 1-byte ints
		}
	case ir.ScalarUint:
		switch s.Width {
		case 2:
			return hlslUint16
		case 4:
			return "uint"
		case 8:
			return hlslUint64
		default:
			return "uint" // Fallback for 1-byte uints
		}
	case ir.ScalarFloat:
		switch s.Width {
		case 2:
			return "half"
		case 4:
			return "float"
		case 8:
			return "double"
		default:
			return "float"
		}
	default:
		return hlslInt // Safe fallback
	}
}

// VectorToHLSL returns the HLSL type name for a vector type.
// HLSL uses TypeN syntax (e.g., float4, int3).
func VectorToHLSL(v ir.VectorType) string {
	size := v.Size
	if size < 2 || size > 4 {
		size = 4 // Clamp to valid range
	}
	return fmt.Sprintf("%s%d", ScalarToHLSL(v.Scalar), size)
}

// MatrixToHLSL returns the HLSL type name for a matrix type.
// HLSL uses TypeRxC syntax (e.g., float4x4, half3x3).
func MatrixToHLSL(m ir.MatrixType) string {
	cols := m.Columns
	rows := m.Rows

	if cols < 2 || cols > 4 {
		cols = 4
	}
	if rows < 2 || rows > 4 {
		rows = 4
	}

	return fmt.Sprintf("%s%dx%d", ScalarToHLSL(m.Scalar), cols, rows)
}

// ScalarCast returns the HLSL cast function for a scalar kind.
// Used for reinterpreting bits (asfloat, asint, asuint).
func ScalarCast(k ir.ScalarKind) string {
	switch k {
	case ir.ScalarFloat:
		return "asfloat"
	case ir.ScalarSint:
		return "asint"
	case ir.ScalarUint:
		return "asuint"
	default:
		return "asfloat"
	}
}

// BuiltInToSemantic returns the HLSL semantic for a built-in value and
// whether HLSL can express it at all: Rust naga's BuiltIn::to_hlsl_str
// (back/hlsl/conv.rs). supported is false for the built-ins Rust refuses
// with an error. semantic is empty for the ones Rust maps to None, the
// "virtual" mesh-shader built-ins HLSL has no semantic for.
// Ref: https://docs.microsoft.com/en-us/windows/win32/direct3dhlsl/dx-graphics-hlsl-semantics
func BuiltInToSemantic(b ir.BuiltinValue) (semantic string, supported bool) {
	switch b {
	// Vertex shader
	case ir.BuiltinPosition:
		return semanticSVPosition, true
	case ir.BuiltinVertexIndex:
		return "SV_VertexID", true
	case ir.BuiltinInstanceIndex:
		return "SV_InstanceID", true
	case ir.BuiltinClipDistances:
		return "SV_ClipDistance", true
	// Fragment shader
	case ir.BuiltinFrontFacing:
		return "SV_IsFrontFace", true
	case ir.BuiltinFragDepth:
		return "SV_Depth", true
	case ir.BuiltinSampleIndex:
		return "SV_SampleIndex", true
	case ir.BuiltinSampleMask:
		return "SV_Coverage", true
	// Compute shader
	case ir.BuiltinGlobalInvocationID:
		return "SV_DispatchThreadID", true
	case ir.BuiltinLocalInvocationID:
		return "SV_GroupThreadID", true
	case ir.BuiltinLocalInvocationIndex:
		return "SV_GroupIndex", true
	case ir.BuiltinWorkGroupID:
		return "SV_GroupID", true
	case ir.BuiltinNumWorkGroups:
		// NumWorkGroups requires a helper constant buffer
		return "SV_GroupID", true // Placeholder - will be replaced
	case ir.BuiltinViewIndex:
		return "SV_ViewID", true
	case ir.BuiltinBarycentric, ir.BuiltinBarycentricNoPerspective:
		return "SV_Barycentrics", true
	case ir.BuiltinPrimitiveIndex:
		return "SV_PrimitiveID", true
	// Wave intrinsics are emitted as functions, never as a semantic.
	case ir.BuiltinNumSubgroups, ir.BuiltinSubgroupID, ir.BuiltinSubgroupSize, ir.BuiltinSubgroupInvocationID:
		return "", true
	case ir.BuiltinCullPrimitive:
		return "SV_CullPrimitive", true
	case ir.BuiltinMeshTaskSize, ir.BuiltinPointIndex, ir.BuiltinLineIndices,
		ir.BuiltinTriangleIndices, ir.BuiltinVertexCount, ir.BuiltinVertices, ir.BuiltinPrimitiveCount,
		ir.BuiltinPrimitives:
		// Virtual mesh-shader built-ins: HLSL has no semantic for them
		// (Rust to_hlsl_str's Ok(None)). Their values reach HLSL through the
		// mesh entry point's indices/vertices/primitives parameters.
		return "", true
	default:
		// HLSL cannot express the built-in at all (point_size, draw_index:
		// Rust to_hlsl_str's Unsupported builtin error); checkBuiltins
		// refuses the entry point.
		return "", false
	}
}

// InterpolationToHLSL returns the HLSL interpolation modifier: Rust naga's
// Interpolation::to_hlsl_str (back/hlsl/conv.rs). Perspective, the default
// in SM4 and up, has none; a per-vertex input is nointerpolation, read with
// GetAttributeAtVertex.
func InterpolationToHLSL(i ir.Interpolation) string {
	switch i {
	case ir.InterpolationLinear:
		return "noperspective"
	case ir.InterpolationFlat, ir.InterpolationPerVertex:
		return "nointerpolation"
	default:
		return ""
	}
}

// SamplingToHLSL returns the HLSL auxiliary sampling qualifier: Rust naga's
// Sampling::to_hlsl_str. Center, first and either have none.
func SamplingToHLSL(s ir.Sampling) string {
	switch s {
	case ir.SamplingCentroid:
		return "centroid"
	case ir.SamplingSample:
		return "sample"
	default:
		return ""
	}
}

// interpolationModifiers is Rust naga's write_modifier for a location
// binding: the interpolation modifier, then the sampling one, each followed
// by a space; "" when the binding has neither.
func interpolationModifiers(loc ir.LocationBinding) string {
	var s string
	if loc.Interpolation != nil {
		if m := InterpolationToHLSL(*loc.Interpolation); m != "" {
			s += m + " "
		}
	}
	if loc.Sampling != nil {
		if m := SamplingToHLSL(*loc.Sampling); m != "" {
			s += m + " "
		}
	}
	return s
}

// AtomicOpToHLSL returns the HLSL Interlocked function suffix.
func AtomicOpToHLSL(op string) string {
	switch op {
	case "add", "sub":
		return "Add"
	case "and":
		return "And"
	case "or":
		return "Or"
	case "xor":
		return "Xor"
	case "min":
		return "Min"
	case "max":
		return "Max"
	case "exchange":
		return "Exchange"
	case "compare_exchange":
		return "CompareExchange"
	default:
		return "Exchange"
	}
}

// AddressSpaceToHLSL returns the HLSL address space qualifier.
func AddressSpaceToHLSL(space ir.AddressSpace) string {
	switch space {
	case ir.SpaceWorkGroup:
		return "groupshared"
	case ir.SpaceUniform:
		return "uniform"
	case ir.SpaceStorage:
		return "globallycoherent" // For RW resources
	default:
		return ""
	}
}

// ImageDimToHLSL returns the HLSL texture dimension suffix.
func ImageDimToHLSL(dim ir.ImageDimension, arrayed bool) string {
	var suffix string
	switch dim {
	case ir.Dim1D:
		suffix = "1D"
	case ir.Dim2D:
		suffix = "2D"
	case ir.Dim3D:
		suffix = "3D"
	case ir.DimCube:
		suffix = "Cube"
	default:
		suffix = "2D"
	}

	if arrayed && dim != ir.Dim3D { // 3D textures can't be arrays
		suffix += "Array"
	}

	return suffix
}

// ImageClassToHLSL returns the HLSL texture prefix for an image class.
func ImageClassToHLSL(class ir.ImageClass, readWrite bool) string {
	switch class {
	case ir.ImageClassSampled:
		return hlslTexture
	case ir.ImageClassDepth:
		return hlslTexture
	case ir.ImageClassStorage:
		if readWrite {
			return "RW" + hlslTexture
		}
		return hlslTexture
	default:
		return hlslTexture
	}
}

// ImageToHLSL returns the full HLSL texture type name.
func ImageToHLSL(img ir.ImageType, readWrite bool) string {
	prefix := ImageClassToHLSL(img.Class, readWrite)

	// Get base dimension (without Array suffix)
	baseDim := ImageDimToHLSL(img.Dim, false)

	if img.Multisampled {
		// Multisampled: Texture2DMS, Texture2DMSArray
		if img.Arrayed {
			return fmt.Sprintf("%s%sMSArray", prefix, baseDim)
		}
		return fmt.Sprintf("%s%sMS", prefix, baseDim)
	}

	// Non-multisampled: use full dim with Array if needed
	dim := ImageDimToHLSL(img.Dim, img.Arrayed)
	return prefix + dim
}

// SamplerToHLSL returns the HLSL sampler type name.
func SamplerToHLSL(comparison bool) string {
	if comparison {
		return "SamplerComparisonState"
	}
	return "SamplerState"
}

// ShaderStageToHLSL returns the HLSL profile suffix for a shader stage.
func ShaderStageToHLSL(stage ir.ShaderStage) string {
	switch stage {
	case ir.StageVertex:
		return "vs"
	case ir.StageFragment:
		return "ps" // Pixel shader in HLSL terminology
	case ir.StageCompute:
		return "cs"
	default:
		return "vs"
	}
}

// ShaderProfile returns the HLSL shader profile string.
// Example: "vs_5_1", "ps_6_0", "cs_6_6"
func ShaderProfile(stage ir.ShaderStage, major, minor uint8) string {
	return fmt.Sprintf("%s_%d_%d", ShaderStageToHLSL(stage), major, minor)
}
