// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl

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
		case 4:
			return hlslInt
		case 8:
			return "int64_t"
		default:
			return hlslInt // Fallback for 1, 2 byte ints
		}
	case ir.ScalarUint:
		switch s.Width {
		case 4:
			return "uint"
		case 8:
			return "uint64_t"
		default:
			return "uint" // Fallback for 1, 2 byte uints
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

// BuiltInToSemantic returns the HLSL semantic for a built-in value.
// Ref: https://docs.microsoft.com/en-us/windows/win32/direct3dhlsl/dx-graphics-hlsl-semantics
func BuiltInToSemantic(b ir.BuiltinValue) string {
	switch b {
	// Vertex shader
	case ir.BuiltinPosition:
		return "SV_Position"
	case ir.BuiltinVertexIndex:
		return "SV_VertexID"
	case ir.BuiltinInstanceIndex:
		return "SV_InstanceID"
	// Fragment shader
	case ir.BuiltinFrontFacing:
		return "SV_IsFrontFace"
	case ir.BuiltinFragDepth:
		return "SV_Depth"
	case ir.BuiltinSampleIndex:
		return "SV_SampleIndex"
	case ir.BuiltinSampleMask:
		return "SV_Coverage"
	// Compute shader
	case ir.BuiltinGlobalInvocationID:
		return "SV_DispatchThreadID"
	case ir.BuiltinLocalInvocationID:
		return "SV_GroupThreadID"
	case ir.BuiltinLocalInvocationIndex:
		return "SV_GroupIndex"
	case ir.BuiltinWorkGroupID:
		return "SV_GroupID"
	case ir.BuiltinNumWorkGroups:
		// NumWorkGroups requires a helper constant buffer
		return "SV_GroupID" // Placeholder - will be replaced
	default:
		return "SV_Position" // Safe fallback
	}
}

// InterpolationToHLSL returns the HLSL interpolation modifier.
// Returns empty string for the default perspective interpolation.
func InterpolationToHLSL(k ir.InterpolationKind) string {
	switch k {
	case ir.InterpolationFlat:
		return "nointerpolation"
	case ir.InterpolationLinear:
		return "noperspective"
	case ir.InterpolationPerspective:
		return "" // Default in SM4+
	default:
		return ""
	}
}

// SamplingToHLSL returns the HLSL auxiliary sampling qualifier.
// Returns empty string for default center sampling.
func SamplingToHLSL(s ir.InterpolationSampling) string {
	switch s {
	case SamplingCenter:
		return "" // Default
	case SamplingCentroid:
		return "centroid"
	case SamplingSample:
		return "sample"
	default:
		return ""
	}
}

// Sampling constants matching ir.InterpolationSampling
const (
	SamplingCenter   ir.InterpolationSampling = ir.SamplingCenter
	SamplingCentroid ir.InterpolationSampling = ir.SamplingCentroid
	SamplingSample   ir.InterpolationSampling = ir.SamplingSample
)

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
