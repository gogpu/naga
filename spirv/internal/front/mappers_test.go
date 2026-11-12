package front

import (
	"math"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

func TestMapBinaryOperator(t *testing.T) {
	cases := map[spvenums.Op]ir.BinaryOperator{
		spvenums.OpIAdd:                   ir.BinaryAdd,
		spvenums.OpFAdd:                   ir.BinaryAdd,
		spvenums.OpISub:                   ir.BinarySubtract,
		spvenums.OpFSub:                   ir.BinarySubtract,
		spvenums.OpIMul:                   ir.BinaryMultiply,
		spvenums.OpFMul:                   ir.BinaryMultiply,
		spvenums.OpVectorTimesScalar:      ir.BinaryMultiply,
		spvenums.OpVectorTimesMatrix:      ir.BinaryMultiply,
		spvenums.OpMatrixTimesScalar:      ir.BinaryMultiply,
		spvenums.OpMatrixTimesVector:      ir.BinaryMultiply,
		spvenums.OpMatrixTimesMatrix:      ir.BinaryMultiply,
		spvenums.OpUDiv:                   ir.BinaryDivide,
		spvenums.OpSDiv:                   ir.BinaryDivide,
		spvenums.OpFDiv:                   ir.BinaryDivide,
		spvenums.OpSRem:                   ir.BinaryModulo,
		spvenums.OpUMod:                   ir.BinaryModulo,
		spvenums.OpFRem:                   ir.BinaryModulo,
		spvenums.OpIEqual:                 ir.BinaryEqual,
		spvenums.OpFOrdEqual:              ir.BinaryEqual,
		spvenums.OpFUnordEqual:            ir.BinaryEqual,
		spvenums.OpLogicalEqual:           ir.BinaryEqual,
		spvenums.OpINotEqual:              ir.BinaryNotEqual,
		spvenums.OpFOrdNotEqual:           ir.BinaryNotEqual,
		spvenums.OpFUnordNotEqual:         ir.BinaryNotEqual,
		spvenums.OpLogicalNotEqual:        ir.BinaryNotEqual,
		spvenums.OpULessThan:              ir.BinaryLess,
		spvenums.OpSLessThan:              ir.BinaryLess,
		spvenums.OpFOrdLessThan:           ir.BinaryLess,
		spvenums.OpFUnordLessThan:         ir.BinaryLess,
		spvenums.OpULessThanEqual:         ir.BinaryLessEqual,
		spvenums.OpSLessThanEqual:         ir.BinaryLessEqual,
		spvenums.OpFOrdLessThanEqual:      ir.BinaryLessEqual,
		spvenums.OpFUnordLessThanEqual:    ir.BinaryLessEqual,
		spvenums.OpUGreaterThan:           ir.BinaryGreater,
		spvenums.OpSGreaterThan:           ir.BinaryGreater,
		spvenums.OpFOrdGreaterThan:        ir.BinaryGreater,
		spvenums.OpFUnordGreaterThan:      ir.BinaryGreater,
		spvenums.OpUGreaterThanEqual:      ir.BinaryGreaterEqual,
		spvenums.OpSGreaterThanEqual:      ir.BinaryGreaterEqual,
		spvenums.OpFOrdGreaterThanEqual:   ir.BinaryGreaterEqual,
		spvenums.OpFUnordGreaterThanEqual: ir.BinaryGreaterEqual,
		spvenums.OpBitwiseOr:              ir.BinaryInclusiveOr,
		spvenums.OpBitwiseXor:             ir.BinaryExclusiveOr,
		spvenums.OpBitwiseAnd:             ir.BinaryAnd,
		spvenums.OpLogicalOr:              ir.BinaryLogicalOr,
		spvenums.OpLogicalAnd:             ir.BinaryLogicalAnd,
		spvenums.OpShiftLeftLogical:       ir.BinaryShiftLeft,
		spvenums.OpShiftRightLogical:      ir.BinaryShiftRight,
		spvenums.OpShiftRightArithmetic:   ir.BinaryShiftRight,
	}
	for op, want := range cases {
		got, err := mapBinaryOperator(op)
		if err != nil || got != want {
			t.Errorf("mapBinaryOperator(%s) = %v, %v; want %v", op, got, err, want)
		}
	}
	if _, err := mapBinaryOperator(spvenums.OpNop); err == nil {
		t.Error("mapBinaryOperator(OpNop) did not fail")
	}
}

func TestMapRelationalFunction(t *testing.T) {
	cases := map[spvenums.Op]ir.RelationalFunction{
		spvenums.OpAll:   ir.RelationalAll,
		spvenums.OpAny:   ir.RelationalAny,
		spvenums.OpIsNan: ir.RelationalIsNan,
		spvenums.OpIsInf: ir.RelationalIsInf,
	}
	for op, want := range cases {
		got, err := mapRelationalFunction(op)
		if err != nil || got != want {
			t.Errorf("mapRelationalFunction(%s) = %v, %v; want %v", op, got, err, want)
		}
	}
	if _, err := mapRelationalFunction(spvenums.OpIsFinite); err == nil {
		t.Error("mapRelationalFunction(OpIsFinite) did not fail")
	}
}

func TestMapGlslStd450(t *testing.T) {
	cases := map[spvenums.GlslStd450]ir.MathFunction{
		spvenums.GlslStd450Round:           ir.MathRound,
		spvenums.GlslStd450RoundEven:       ir.MathRound,
		spvenums.GlslStd450Trunc:           ir.MathTrunc,
		spvenums.GlslStd450FAbs:            ir.MathAbs,
		spvenums.GlslStd450SAbs:            ir.MathAbs,
		spvenums.GlslStd450FSign:           ir.MathSign,
		spvenums.GlslStd450SSign:           ir.MathSign,
		spvenums.GlslStd450Floor:           ir.MathFloor,
		spvenums.GlslStd450Ceil:            ir.MathCeil,
		spvenums.GlslStd450Fract:           ir.MathFract,
		spvenums.GlslStd450Sin:             ir.MathSin,
		spvenums.GlslStd450Cos:             ir.MathCos,
		spvenums.GlslStd450Tan:             ir.MathTan,
		spvenums.GlslStd450Asin:            ir.MathAsin,
		spvenums.GlslStd450Acos:            ir.MathAcos,
		spvenums.GlslStd450Atan:            ir.MathAtan,
		spvenums.GlslStd450Sinh:            ir.MathSinh,
		spvenums.GlslStd450Cosh:            ir.MathCosh,
		spvenums.GlslStd450Tanh:            ir.MathTanh,
		spvenums.GlslStd450Atan2:           ir.MathAtan2,
		spvenums.GlslStd450Asinh:           ir.MathAsinh,
		spvenums.GlslStd450Acosh:           ir.MathAcosh,
		spvenums.GlslStd450Atanh:           ir.MathAtanh,
		spvenums.GlslStd450Radians:         ir.MathRadians,
		spvenums.GlslStd450Degrees:         ir.MathDegrees,
		spvenums.GlslStd450Pow:             ir.MathPow,
		spvenums.GlslStd450Exp:             ir.MathExp,
		spvenums.GlslStd450Log:             ir.MathLog,
		spvenums.GlslStd450Exp2:            ir.MathExp2,
		spvenums.GlslStd450Log2:            ir.MathLog2,
		spvenums.GlslStd450Sqrt:            ir.MathSqrt,
		spvenums.GlslStd450InverseSqrt:     ir.MathInverseSqrt,
		spvenums.GlslStd450MatrixInverse:   ir.MathInverse,
		spvenums.GlslStd450Determinant:     ir.MathDeterminant,
		spvenums.GlslStd450ModfStruct:      ir.MathModf,
		spvenums.GlslStd450FMin:            ir.MathMin,
		spvenums.GlslStd450UMin:            ir.MathMin,
		spvenums.GlslStd450SMin:            ir.MathMin,
		spvenums.GlslStd450NMin:            ir.MathMin,
		spvenums.GlslStd450FMax:            ir.MathMax,
		spvenums.GlslStd450UMax:            ir.MathMax,
		spvenums.GlslStd450SMax:            ir.MathMax,
		spvenums.GlslStd450NMax:            ir.MathMax,
		spvenums.GlslStd450FClamp:          ir.MathClamp,
		spvenums.GlslStd450UClamp:          ir.MathClamp,
		spvenums.GlslStd450SClamp:          ir.MathClamp,
		spvenums.GlslStd450NClamp:          ir.MathClamp,
		spvenums.GlslStd450FMix:            ir.MathMix,
		spvenums.GlslStd450Step:            ir.MathStep,
		spvenums.GlslStd450SmoothStep:      ir.MathSmoothStep,
		spvenums.GlslStd450Fma:             ir.MathFma,
		spvenums.GlslStd450FrexpStruct:     ir.MathFrexp,
		spvenums.GlslStd450Ldexp:           ir.MathLdexp,
		spvenums.GlslStd450Length:          ir.MathLength,
		spvenums.GlslStd450Distance:        ir.MathDistance,
		spvenums.GlslStd450Cross:           ir.MathCross,
		spvenums.GlslStd450Normalize:       ir.MathNormalize,
		spvenums.GlslStd450FaceForward:     ir.MathFaceForward,
		spvenums.GlslStd450Reflect:         ir.MathReflect,
		spvenums.GlslStd450Refract:         ir.MathRefract,
		spvenums.GlslStd450PackUnorm4x8:    ir.MathPack4x8unorm,
		spvenums.GlslStd450PackSnorm4x8:    ir.MathPack4x8snorm,
		spvenums.GlslStd450PackHalf2x16:    ir.MathPack2x16float,
		spvenums.GlslStd450PackUnorm2x16:   ir.MathPack2x16unorm,
		spvenums.GlslStd450PackSnorm2x16:   ir.MathPack2x16snorm,
		spvenums.GlslStd450UnpackUnorm4x8:  ir.MathUnpack4x8unorm,
		spvenums.GlslStd450UnpackSnorm4x8:  ir.MathUnpack4x8snorm,
		spvenums.GlslStd450UnpackHalf2x16:  ir.MathUnpack2x16float,
		spvenums.GlslStd450UnpackUnorm2x16: ir.MathUnpack2x16unorm,
		spvenums.GlslStd450UnpackSnorm2x16: ir.MathUnpack2x16snorm,
		spvenums.GlslStd450FindILsb:        ir.MathFirstTrailingBit,
		spvenums.GlslStd450FindUMsb:        ir.MathFirstLeadingBit,
		spvenums.GlslStd450FindSMsb:        ir.MathFirstLeadingBit,
	}
	for op, want := range cases {
		got, err := mapGlslStd450(op)
		if err != nil || got != want {
			t.Errorf("mapGlslStd450(%s) = %v, %v; want %v", op, got, err, want)
		}
	}
	for _, unsupported := range []spvenums.GlslStd450{
		spvenums.GlslStd450Modf,
		spvenums.GlslStd450Frexp,
		spvenums.GlslStd450IMix,
		spvenums.GlslStd450PackDouble2x32,
		spvenums.GlslStd450InterpolateAtCentroid,
	} {
		if _, err := mapGlslStd450(unsupported); err == nil {
			t.Errorf("mapGlslStd450(%s) did not fail", unsupported)
		}
	}
}

func TestMathArgumentCount(t *testing.T) {
	cases := map[ir.MathFunction]uint8{
		ir.MathAbs: 1, ir.MathMin: 2, ir.MathMax: 2, ir.MathClamp: 3,
		ir.MathAtan2: 2, ir.MathLdexp: 2, ir.MathPow: 2,
		ir.MathDot: 2, ir.MathCross: 2, ir.MathDistance: 2,
		ir.MathStep: 2, ir.MathReflect: 2,
		ir.MathFaceForward: 3, ir.MathRefract: 3, ir.MathFma: 3,
		ir.MathMix: 3, ir.MathSmoothStep: 3, ir.MathExtractBits: 3,
		ir.MathInsertBits: 4,
		ir.MathSqrt:       1, ir.MathInverse: 1, ir.MathDeterminant: 1,
		ir.MathLength: 1, ir.MathNormalize: 1,
		ir.MathPack4x8unorm: 1, ir.MathUnpack2x16float: 1,
	}
	for fun, want := range cases {
		if got := mathArgumentCount(fun); got != want {
			t.Errorf("mathArgumentCount(%v) = %d, want %d", fun, got, want)
		}
	}
}

func TestMapWidthAndVectorSize(t *testing.T) {
	for word, want := range map[uint32]uint8{8: 1, 16: 2, 24: 3, 32: 4, 64: 8, 0: 0} {
		got, err := mapWidth(word)
		if err != nil || got != want {
			t.Errorf("mapWidth(%d) = %d, %v; want %d", word, got, err, want)
		}
	}
	if _, err := mapWidth(1 << 12); err == nil {
		t.Error("mapWidth(4096) did not fail")
	}
	for word, want := range map[uint32]ir.VectorSize{2: ir.Vec2, 3: ir.Vec3, 4: ir.Vec4} {
		got, err := mapVectorSize(word)
		if err != nil || got != want {
			t.Errorf("mapVectorSize(%d) = %v, %v; want %v", word, got, err, want)
		}
	}
	if _, err := mapVectorSize(5); err == nil {
		t.Error("mapVectorSize(5) did not fail")
	}
}

func TestMapImageDimAndFormat(t *testing.T) {
	for word, want := range map[uint32]ir.ImageDimension{
		uint32(spvenums.Dim1D): ir.Dim1D, uint32(spvenums.Dim2D): ir.Dim2D,
		uint32(spvenums.Dim3D): ir.Dim3D, uint32(spvenums.DimCube): ir.DimCube,
	} {
		got, err := mapImageDim(word)
		if err != nil || got != want {
			t.Errorf("mapImageDim(%d) = %v, %v; want %v", word, got, err, want)
		}
	}
	if _, err := mapImageDim(uint32(spvenums.DimRect)); err == nil {
		t.Error("mapImageDim(DimRect) did not fail")
	}

	formats := map[spvenums.ImageFormat]ir.StorageFormat{
		spvenums.ImageFormatR8: ir.StorageFormatR8Unorm, spvenums.ImageFormatR8Snorm: ir.StorageFormatR8Snorm,
		spvenums.ImageFormatR8ui: ir.StorageFormatR8Uint, spvenums.ImageFormatR8i: ir.StorageFormatR8Sint,
		spvenums.ImageFormatR16: ir.StorageFormatR16Unorm, spvenums.ImageFormatR16Snorm: ir.StorageFormatR16Snorm,
		spvenums.ImageFormatR16ui: ir.StorageFormatR16Uint, spvenums.ImageFormatR16i: ir.StorageFormatR16Sint,
		spvenums.ImageFormatR16f: ir.StorageFormatR16Float,
		spvenums.ImageFormatRg8:  ir.StorageFormatRg8Unorm, spvenums.ImageFormatRg8Snorm: ir.StorageFormatRg8Snorm,
		spvenums.ImageFormatRg8ui: ir.StorageFormatRg8Uint, spvenums.ImageFormatRg8i: ir.StorageFormatRg8Sint,
		spvenums.ImageFormatR32ui: ir.StorageFormatR32Uint, spvenums.ImageFormatR32i: ir.StorageFormatR32Sint,
		spvenums.ImageFormatR32f: ir.StorageFormatR32Float,
		spvenums.ImageFormatRg16: ir.StorageFormatRg16Unorm, spvenums.ImageFormatRg16Snorm: ir.StorageFormatRg16Snorm,
		spvenums.ImageFormatRg16ui: ir.StorageFormatRg16Uint, spvenums.ImageFormatRg16i: ir.StorageFormatRg16Sint,
		spvenums.ImageFormatRg16f: ir.StorageFormatRg16Float,
		spvenums.ImageFormatRgba8: ir.StorageFormatRgba8Unorm, spvenums.ImageFormatRgba8Snorm: ir.StorageFormatRgba8Snorm,
		spvenums.ImageFormatRgba8ui: ir.StorageFormatRgba8Uint, spvenums.ImageFormatRgba8i: ir.StorageFormatRgba8Sint,
		spvenums.ImageFormatRgb10a2ui: ir.StorageFormatRgb10a2Uint, spvenums.ImageFormatRgb10A2: ir.StorageFormatRgb10a2Unorm,
		spvenums.ImageFormatR11fG11fB10f: ir.StorageFormatRg11b10Ufloat,
		spvenums.ImageFormatR64ui:        ir.StorageFormatR64Uint,
		spvenums.ImageFormatRg32ui:       ir.StorageFormatRg32Uint, spvenums.ImageFormatRg32i: ir.StorageFormatRg32Sint,
		spvenums.ImageFormatRg32f:  ir.StorageFormatRg32Float,
		spvenums.ImageFormatRgba16: ir.StorageFormatRgba16Unorm, spvenums.ImageFormatRgba16Snorm: ir.StorageFormatRgba16Snorm,
		spvenums.ImageFormatRgba16ui: ir.StorageFormatRgba16Uint, spvenums.ImageFormatRgba16i: ir.StorageFormatRgba16Sint,
		spvenums.ImageFormatRgba16f:  ir.StorageFormatRgba16Float,
		spvenums.ImageFormatRgba32ui: ir.StorageFormatRgba32Uint, spvenums.ImageFormatRgba32i: ir.StorageFormatRgba32Sint,
		spvenums.ImageFormatRgba32f: ir.StorageFormatRgba32Float,
	}
	for format, want := range formats {
		got, err := mapImageFormat(uint32(format))
		if err != nil || got != want {
			t.Errorf("mapImageFormat(%s) = %v, %v; want %v", format, got, err, want)
		}
	}
	if _, err := mapImageFormat(uint32(spvenums.ImageFormatUnknown)); err == nil {
		t.Error("mapImageFormat(Unknown) did not fail")
	}
}

func TestMapStorageClass(t *testing.T) {
	cases := map[spvenums.StorageClass]extendedClass{
		spvenums.StorageClassFunction:        {class: classGlobal, space: ir.SpaceFunction},
		spvenums.StorageClassInput:           {class: classInput},
		spvenums.StorageClassOutput:          {class: classOutput},
		spvenums.StorageClassPrivate:         {class: classGlobal, space: ir.SpacePrivate},
		spvenums.StorageClassUniformConstant: {class: classGlobal, space: ir.SpaceHandle},
		spvenums.StorageClassStorageBuffer:   {class: classGlobal, space: ir.SpaceStorage},
		spvenums.StorageClassUniform:         {class: classGlobal, space: ir.SpaceUniform},
		spvenums.StorageClassWorkgroup:       {class: classGlobal, space: ir.SpaceWorkGroup},
		spvenums.StorageClassPushConstant:    {class: classGlobal, space: ir.SpaceImmediate},
	}
	for class, want := range cases {
		got, err := mapStorageClass(uint32(class))
		if err != nil || got != want {
			t.Errorf("mapStorageClass(%s) = %#v, %v; want %#v", class, got, err, want)
		}
	}
	if _, err := mapStorageClass(uint32(spvenums.StorageClassAtomicCounter)); err == nil {
		t.Error("mapStorageClass(AtomicCounter) did not fail")
	}
}

func TestMapBuiltin(t *testing.T) {
	cases := map[spvenums.BuiltIn]ir.BuiltinValue{
		spvenums.BuiltInPosition:                  ir.BuiltinPosition,
		spvenums.BuiltInFragCoord:                 ir.BuiltinPosition,
		spvenums.BuiltInViewIndex:                 ir.BuiltinViewIndex,
		spvenums.BuiltInBaseInstance:              ir.BuiltinBaseInstance,
		spvenums.BuiltInBaseVertex:                ir.BuiltinBaseVertex,
		spvenums.BuiltInClipDistance:              ir.BuiltinClipDistances,
		spvenums.BuiltInCullDistance:              ir.BuiltinCullDistance,
		spvenums.BuiltInInstanceIndex:             ir.BuiltinInstanceIndex,
		spvenums.BuiltInPointSize:                 ir.BuiltinPointSize,
		spvenums.BuiltInVertexIndex:               ir.BuiltinVertexIndex,
		spvenums.BuiltInDrawIndex:                 ir.BuiltinDrawIndex,
		spvenums.BuiltInFragDepth:                 ir.BuiltinFragDepth,
		spvenums.BuiltInPointCoord:                ir.BuiltinPointCoord,
		spvenums.BuiltInFrontFacing:               ir.BuiltinFrontFacing,
		spvenums.BuiltInPrimitiveId:               ir.BuiltinPrimitiveIndex,
		spvenums.BuiltInBaryCoordKHR:              ir.BuiltinBarycentric,
		spvenums.BuiltInBaryCoordNoPerspKHR:       ir.BuiltinBarycentricNoPerspective,
		spvenums.BuiltInSampleId:                  ir.BuiltinSampleIndex,
		spvenums.BuiltInSampleMask:                ir.BuiltinSampleMask,
		spvenums.BuiltInGlobalInvocationId:        ir.BuiltinGlobalInvocationID,
		spvenums.BuiltInLocalInvocationId:         ir.BuiltinLocalInvocationID,
		spvenums.BuiltInLocalInvocationIndex:      ir.BuiltinLocalInvocationIndex,
		spvenums.BuiltInWorkgroupId:               ir.BuiltinWorkGroupID,
		spvenums.BuiltInWorkgroupSize:             ir.BuiltinWorkGroupSize,
		spvenums.BuiltInNumWorkgroups:             ir.BuiltinNumWorkGroups,
		spvenums.BuiltInNumSubgroups:              ir.BuiltinNumSubgroups,
		spvenums.BuiltInSubgroupId:                ir.BuiltinSubgroupID,
		spvenums.BuiltInSubgroupSize:              ir.BuiltinSubgroupSize,
		spvenums.BuiltInSubgroupLocalInvocationId: ir.BuiltinSubgroupInvocationID,
	}
	for builtin, want := range cases {
		got, err := mapBuiltin(uint32(builtin), false)
		if err != nil || got != want {
			t.Errorf("mapBuiltin(%s) = %v, %v; want %v", builtin, got, err, want)
		}
	}
	if _, err := mapBuiltin(uint32(spvenums.BuiltInTessLevelOuter), false); err == nil {
		t.Error("mapBuiltin(TessLevelOuter) did not fail")
	}
}

func TestDecorationHelpers(t *testing.T) {
	set := &decoration{descSet: ptrTo(uint32(2)), descIndex: ptrTo(uint32(5))}
	if got := set.resourceBinding(); got == nil || *got != (ir.ResourceBinding{Group: 2, Binding: 5}) {
		t.Errorf("resourceBinding = %v, want (2,5)", got)
	}
	if got := (&decoration{descSet: ptrTo(uint32(1))}).resourceBinding(); got != nil {
		t.Errorf("resourceBinding without a binding = %v, want nil", got)
	}

	cases := []struct {
		name  string
		flags decorationFlags
		want  ir.StorageAccess
	}{
		{"both", 0, ir.StorageAccessLoad | ir.StorageAccessStore},
		{"non-readable", decorationNonReadable, ir.StorageAccessStore},
		{"non-writable", decorationNonWritable, ir.StorageAccessLoad},
		{"neither", decorationNonReadable | decorationNonWritable, 0},
	}
	for _, tc := range cases {
		if got := (&decoration{flags: tc.flags}).storageAccess(); got != tc.want {
			t.Errorf("%s: storageAccess = %d, want %d", tc.name, got, tc.want)
		}
	}

	builtin, err := (&decoration{builtIn: ptrTo(uint32(spvenums.BuiltInPosition)), invariant: true}).ioBinding()
	if err != nil {
		t.Fatalf("ioBinding(builtin): %v", err)
	}
	if b, ok := builtin.(ir.BuiltinBinding); !ok || b.Builtin != ir.BuiltinPosition || !b.Invariant {
		t.Errorf("ioBinding(builtin) = %#v, want an invariant position", builtin)
	}

	blended, err := (&decoration{location: ptrTo(uint32(3)), index: ptrTo(uint32(1))}).ioBinding()
	if err != nil {
		t.Fatalf("ioBinding(blend src): %v", err)
	}
	if b, ok := blended.(ir.LocationBinding); !ok || b.Location != 3 || b.BlendSrc == nil || *b.BlendSrc != 1 {
		t.Errorf("ioBinding(blend src) = %#v, want location 3 blend 1", blended)
	}

	if _, err := (&decoration{builtIn: ptrTo(uint32(spvenums.BuiltInPosition)), location: ptrTo(uint32(0))}).ioBinding(); err == nil {
		t.Error("ioBinding with both a builtin and a location did not fail")
	}
	if _, err := (&decoration{}).ioBinding(); err == nil {
		t.Error("ioBinding without decorations did not fail")
	}
}

func TestFloat16FromBits(t *testing.T) {
	cases := map[uint16]float32{
		0x0000: 0,
		0x8000: float32(math.Copysign(0, -1)),
		0x3c00: 1,
		0xbc00: -1,
		0x3555: 0.333251953125,
		0x0001: float32(math.Pow(2, -24)), // smallest subnormal
		0x03ff: 0.000060975552,            // largest subnormal
		0x0400: float32(math.Pow(2, -14)), // smallest normal
		0x7bff: 65504,                     // largest finite
		0x7c00: float32(math.Inf(1)),
		0xfc00: float32(math.Inf(-1)),
		0x7e00: float32(math.NaN()),
	}
	for bits, want := range cases {
		got := float16FromBits(bits)
		if bits&0x7c00 == 0x7c00 && bits&0x03ff != 0 {
			if !math.IsNaN(float64(got)) {
				t.Errorf("float16FromBits(%#04x) = %v, want NaN", bits, got)
			}
			continue
		}
		if math.Abs(float64(got-want)) > math.Abs(float64(want))*1e-6 && !(got == 0 && want == 0) {
			t.Errorf("float16FromBits(%#04x) = %v, want %v", bits, got, want)
		}
		if math.Signbit(float64(got)) != math.Signbit(float64(want)) {
			t.Errorf("float16FromBits(%#04x) = %v, want the sign of %v", bits, got, want)
		}
	}
}

// ptrTo returns a pointer to a copy of v.
func ptrTo[T any](v T) *T { return &v }
