package front

import (
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// supportedCapability reports whether this package implements the capability,
// as Rust naga's SUPPORTED_CAPABILITIES.
func supportedCapability(c spvenums.Capability) bool {
	switch c {
	case spvenums.CapabilityShader,
		spvenums.CapabilityVulkanMemoryModel,
		spvenums.CapabilityClipDistance,
		spvenums.CapabilityCullDistance,
		spvenums.CapabilitySampleRateShading,
		spvenums.CapabilityDerivativeControl,
		spvenums.CapabilityMatrix,
		spvenums.CapabilityImageQuery,
		spvenums.CapabilitySampled1D,
		spvenums.CapabilityImage1D,
		spvenums.CapabilitySampledCubeArray,
		spvenums.CapabilityImageCubeArray,
		spvenums.CapabilityStorageImageExtendedFormats,
		spvenums.CapabilityInt8,
		spvenums.CapabilityInt16,
		spvenums.CapabilityInt64,
		spvenums.CapabilityInt64Atomics,
		spvenums.CapabilityFloat16,
		spvenums.CapabilityAtomicFloat32AddEXT,
		spvenums.CapabilityFloat64,
		spvenums.CapabilityGeometry,
		spvenums.CapabilityMultiView,
		spvenums.CapabilityStorageBuffer16BitAccess,
		spvenums.CapabilityUniformAndStorageBuffer16BitAccess,
		spvenums.CapabilityGroupNonUniform,
		spvenums.CapabilityGroupNonUniformVote,
		spvenums.CapabilityGroupNonUniformArithmetic,
		spvenums.CapabilityGroupNonUniformBallot,
		spvenums.CapabilityGroupNonUniformShuffle,
		spvenums.CapabilityGroupNonUniformShuffleRelative,
		spvenums.CapabilityRuntimeDescriptorArray,
		spvenums.CapabilityStorageImageMultisample,
		spvenums.CapabilityFragmentBarycentricKHR,
		// The two array-indexing capabilities look like they should be
		// required, but they are "tricky ones" in Rust naga: the indexing is
		// read from the module's types and the capability is not enforced.
		spvenums.CapabilityUniformBufferArrayDynamicIndexing,
		spvenums.CapabilityStorageBufferArrayDynamicIndexing:
		return true
	}
	return false
}

// mapBinaryOperator maps the opcode of a two-operand instruction to an IR
// binary operator, as Rust naga's map_binary_operator.
func mapBinaryOperator(op spvenums.Op) (ir.BinaryOperator, error) {
	switch op {
	case spvenums.OpIAdd, spvenums.OpFAdd:
		return ir.BinaryAdd, nil
	case spvenums.OpISub, spvenums.OpFSub:
		return ir.BinarySubtract, nil
	case spvenums.OpIMul, spvenums.OpFMul,
		spvenums.OpVectorTimesScalar, spvenums.OpVectorTimesMatrix,
		spvenums.OpMatrixTimesScalar, spvenums.OpMatrixTimesVector, spvenums.OpMatrixTimesMatrix:
		return ir.BinaryMultiply, nil
	case spvenums.OpUDiv, spvenums.OpSDiv, spvenums.OpFDiv:
		return ir.BinaryDivide, nil
	case spvenums.OpSRem, spvenums.OpUMod, spvenums.OpFRem:
		return ir.BinaryModulo, nil
	case spvenums.OpIEqual, spvenums.OpFOrdEqual, spvenums.OpFUnordEqual, spvenums.OpLogicalEqual:
		return ir.BinaryEqual, nil
	case spvenums.OpINotEqual, spvenums.OpFOrdNotEqual, spvenums.OpFUnordNotEqual, spvenums.OpLogicalNotEqual:
		return ir.BinaryNotEqual, nil
	case spvenums.OpULessThan, spvenums.OpSLessThan, spvenums.OpFOrdLessThan, spvenums.OpFUnordLessThan:
		return ir.BinaryLess, nil
	case spvenums.OpULessThanEqual, spvenums.OpSLessThanEqual,
		spvenums.OpFOrdLessThanEqual, spvenums.OpFUnordLessThanEqual:
		return ir.BinaryLessEqual, nil
	case spvenums.OpUGreaterThan, spvenums.OpSGreaterThan,
		spvenums.OpFOrdGreaterThan, spvenums.OpFUnordGreaterThan:
		return ir.BinaryGreater, nil
	case spvenums.OpUGreaterThanEqual, spvenums.OpSGreaterThanEqual,
		spvenums.OpFOrdGreaterThanEqual, spvenums.OpFUnordGreaterThanEqual:
		return ir.BinaryGreaterEqual, nil
	case spvenums.OpBitwiseOr:
		return ir.BinaryInclusiveOr, nil
	case spvenums.OpBitwiseXor:
		return ir.BinaryExclusiveOr, nil
	case spvenums.OpBitwiseAnd:
		return ir.BinaryAnd, nil
	case spvenums.OpLogicalOr:
		return ir.BinaryLogicalOr, nil
	case spvenums.OpLogicalAnd:
		return ir.BinaryLogicalAnd, nil
	case spvenums.OpShiftLeftLogical:
		return ir.BinaryShiftLeft, nil
	case spvenums.OpShiftRightLogical, spvenums.OpShiftRightArithmetic:
		return ir.BinaryShiftRight, nil
	}
	return 0, errUnknownBinaryOperator(op)
}

// mapRelationalFunction maps OpAny, OpAll, OpIsNan and OpIsInf.
func mapRelationalFunction(op spvenums.Op) (ir.RelationalFunction, error) {
	switch op {
	case spvenums.OpAll:
		return ir.RelationalAll, nil
	case spvenums.OpAny:
		return ir.RelationalAny, nil
	case spvenums.OpIsNan:
		return ir.RelationalIsNan, nil
	case spvenums.OpIsInf:
		return ir.RelationalIsInf, nil
	}
	return 0, errUnknownRelationalFunction(op)
}

// mapGlslStd450 maps a GLSL.std.450 instruction to an IR math function, as
// Rust naga's ExtInst handling.
func mapGlslStd450(op spvenums.GlslStd450) (ir.MathFunction, error) {
	switch op {
	case spvenums.GlslStd450Round, spvenums.GlslStd450RoundEven:
		return ir.MathRound, nil
	case spvenums.GlslStd450Trunc:
		return ir.MathTrunc, nil
	case spvenums.GlslStd450FAbs, spvenums.GlslStd450SAbs:
		return ir.MathAbs, nil
	case spvenums.GlslStd450FSign, spvenums.GlslStd450SSign:
		return ir.MathSign, nil
	case spvenums.GlslStd450Floor:
		return ir.MathFloor, nil
	case spvenums.GlslStd450Ceil:
		return ir.MathCeil, nil
	case spvenums.GlslStd450Fract:
		return ir.MathFract, nil
	case spvenums.GlslStd450Sin:
		return ir.MathSin, nil
	case spvenums.GlslStd450Cos:
		return ir.MathCos, nil
	case spvenums.GlslStd450Tan:
		return ir.MathTan, nil
	case spvenums.GlslStd450Asin:
		return ir.MathAsin, nil
	case spvenums.GlslStd450Acos:
		return ir.MathAcos, nil
	case spvenums.GlslStd450Atan:
		return ir.MathAtan, nil
	case spvenums.GlslStd450Sinh:
		return ir.MathSinh, nil
	case spvenums.GlslStd450Cosh:
		return ir.MathCosh, nil
	case spvenums.GlslStd450Tanh:
		return ir.MathTanh, nil
	case spvenums.GlslStd450Atan2:
		return ir.MathAtan2, nil
	case spvenums.GlslStd450Asinh:
		return ir.MathAsinh, nil
	case spvenums.GlslStd450Acosh:
		return ir.MathAcosh, nil
	case spvenums.GlslStd450Atanh:
		return ir.MathAtanh, nil
	case spvenums.GlslStd450Radians:
		return ir.MathRadians, nil
	case spvenums.GlslStd450Degrees:
		return ir.MathDegrees, nil
	case spvenums.GlslStd450Pow:
		return ir.MathPow, nil
	case spvenums.GlslStd450Exp:
		return ir.MathExp, nil
	case spvenums.GlslStd450Log:
		return ir.MathLog, nil
	case spvenums.GlslStd450Exp2:
		return ir.MathExp2, nil
	case spvenums.GlslStd450Log2:
		return ir.MathLog2, nil
	case spvenums.GlslStd450Sqrt:
		return ir.MathSqrt, nil
	case spvenums.GlslStd450InverseSqrt:
		return ir.MathInverseSqrt, nil
	case spvenums.GlslStd450MatrixInverse:
		return ir.MathInverse, nil
	case spvenums.GlslStd450Determinant:
		return ir.MathDeterminant, nil
	case spvenums.GlslStd450ModfStruct:
		return ir.MathModf, nil
	case spvenums.GlslStd450FMin, spvenums.GlslStd450UMin,
		spvenums.GlslStd450SMin, spvenums.GlslStd450NMin:
		return ir.MathMin, nil
	case spvenums.GlslStd450FMax, spvenums.GlslStd450UMax,
		spvenums.GlslStd450SMax, spvenums.GlslStd450NMax:
		return ir.MathMax, nil
	case spvenums.GlslStd450FClamp, spvenums.GlslStd450UClamp,
		spvenums.GlslStd450SClamp, spvenums.GlslStd450NClamp:
		return ir.MathClamp, nil
	case spvenums.GlslStd450FMix:
		return ir.MathMix, nil
	case spvenums.GlslStd450Step:
		return ir.MathStep, nil
	case spvenums.GlslStd450SmoothStep:
		return ir.MathSmoothStep, nil
	case spvenums.GlslStd450Fma:
		return ir.MathFma, nil
	case spvenums.GlslStd450FrexpStruct:
		return ir.MathFrexp, nil
	case spvenums.GlslStd450Ldexp:
		return ir.MathLdexp, nil
	case spvenums.GlslStd450Length:
		return ir.MathLength, nil
	case spvenums.GlslStd450Distance:
		return ir.MathDistance, nil
	case spvenums.GlslStd450Cross:
		return ir.MathCross, nil
	case spvenums.GlslStd450Normalize:
		return ir.MathNormalize, nil
	case spvenums.GlslStd450FaceForward:
		return ir.MathFaceForward, nil
	case spvenums.GlslStd450Reflect:
		return ir.MathReflect, nil
	case spvenums.GlslStd450Refract:
		return ir.MathRefract, nil
	case spvenums.GlslStd450PackUnorm4x8:
		return ir.MathPack4x8unorm, nil
	case spvenums.GlslStd450PackSnorm4x8:
		return ir.MathPack4x8snorm, nil
	case spvenums.GlslStd450PackHalf2x16:
		return ir.MathPack2x16float, nil
	case spvenums.GlslStd450PackUnorm2x16:
		return ir.MathPack2x16unorm, nil
	case spvenums.GlslStd450PackSnorm2x16:
		return ir.MathPack2x16snorm, nil
	case spvenums.GlslStd450UnpackUnorm4x8:
		return ir.MathUnpack4x8unorm, nil
	case spvenums.GlslStd450UnpackSnorm4x8:
		return ir.MathUnpack4x8snorm, nil
	case spvenums.GlslStd450UnpackHalf2x16:
		return ir.MathUnpack2x16float, nil
	case spvenums.GlslStd450UnpackUnorm2x16:
		return ir.MathUnpack2x16unorm, nil
	case spvenums.GlslStd450UnpackSnorm2x16:
		return ir.MathUnpack2x16snorm, nil
	case spvenums.GlslStd450FindILsb:
		return ir.MathFirstTrailingBit, nil
	case spvenums.GlslStd450FindUMsb, spvenums.GlslStd450FindSMsb:
		return ir.MathFirstLeadingBit, nil
	}
	return 0, errUnsupportedExtInst(uint32(op))
}

// mathArgumentCount returns the number of operands a math function takes, as
// Rust naga's MathFunction::argument_count.
func mathArgumentCount(fun ir.MathFunction) uint8 {
	switch fun {
	case ir.MathMin, ir.MathMax,
		ir.MathAtan2, ir.MathLdexp, ir.MathPow,
		ir.MathDot, ir.MathDot4I8Packed, ir.MathDot4U8Packed, ir.MathOuter,
		ir.MathCross, ir.MathDistance, ir.MathReflect, ir.MathStep:
		return 2
	case ir.MathClamp, ir.MathFaceForward, ir.MathRefract, ir.MathFma,
		ir.MathMix, ir.MathSmoothStep, ir.MathExtractBits:
		return 3
	case ir.MathInsertBits:
		return 4
	}
	return 1
}

// supportedExtension reports whether this package implements the extension,
// as Rust naga's SUPPORTED_EXTENSIONS.
func supportedExtension(name string) bool {
	switch name {
	case "SPV_KHR_storage_buffer_storage_class",
		"SPV_KHR_vulkan_memory_model",
		"SPV_KHR_multiview",
		"SPV_EXT_descriptor_indexing",
		"SPV_EXT_shader_atomic_float_add",
		"SPV_KHR_16bit_storage",
		"SPV_KHR_non_semantic_info",
		"SPV_KHR_fragment_shader_barycentric":
		return true
	}
	return false
}
