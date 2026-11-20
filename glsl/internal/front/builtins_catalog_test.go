package front

// This file pins the builtin overload catalog: which names GLSL declares, with
// which parameter types and which expansion, Rust naga's builtins.rs. The
// catalog is a contract, not an implementation detail — a missing or wrong
// overload either rejects a valid shader ("Unknown function") or silently
// resolves it to a conversion the GLSL spec does not allow.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// vecOf returns the vector type of size over scalar.
func vecOf(size ir.VectorSize, scalar ir.ScalarType) ir.TypeInner {
	return ir.VectorType{Size: size, Scalar: scalar}
}

// matOf returns the matrix type of rows by columns over scalar; Rust spells
// the fields columns then rows (builtins.rs:1318-1326), so callers keep them
// apart explicitly.
func matOf(rows, columns ir.VectorSize, scalar ir.ScalarType) ir.TypeInner {
	return ir.MatrixType{Columns: columns, Rows: rows, Scalar: scalar}
}

// sampledImage is a plain sampled image of the given shape and texel kind.
func sampledImage(dim ir.ImageDimension, arrayed, multi bool, kind ir.ScalarKind) ir.TypeInner {
	return ir.ImageType{Dim: dim, Arrayed: arrayed, Multisampled: multi, Class: ir.ImageClassSampled, SampledKind: kind}
}

// depthImage is a shadow sampler's depth image, builtins.rs:1600-1605.
func depthImage(dim ir.ImageDimension, arrayed bool) ir.TypeInner {
	return ir.ImageType{Dim: dim, Arrayed: arrayed, Class: ir.ImageClassDepth, SampledKind: ir.ScalarFloat}
}

// storageImage is a storage image whose format matches kind, the placeholder
// the declaration's layout format replaces (builtins.rs:17-25).
func storageImage(dim ir.ImageDimension, arrayed bool, kind ir.ScalarKind, access ir.StorageAccess) ir.TypeInner {
	return ir.ImageType{
		Dim:           dim,
		Arrayed:       arrayed,
		Class:         ir.ImageClassStorage,
		SampledKind:   kind,
		StorageFormat: dummyStorageFormat(kind),
		StorageAccess: access,
	}
}

// builtinCatalogCase is one expected overload of one builtin: its name, the
// parameter types, and the expansion family (and, optionally, the exact
// operator or math function).
type builtinCatalogCase struct {
	name   string
	args   []ir.TypeInner
	kind   macroKind
	math   *ir.MathFunction
	unary  *ir.UnaryOperator
	binary *ir.BinaryOperator
	rel    *ir.RelationalFunction
	cast   *ir.ScalarKind
	axis   *ir.DerivativeAxis
}

func mathPtr(fun ir.MathFunction) *ir.MathFunction { return &fun }

// builtinName returns the builtin a catalog label names. A label may carry a
// disambiguating suffix after a space ("abs f32"); the GLSL builtin name never
// does.
func builtinName(label string) string {
	if i := strings.IndexByte(label, ' '); i >= 0 {
		return label[:i]
	}
	return label
}

// checkBuiltinCatalog injects each case's name in the standard variation.
func checkBuiltinCatalog(t *testing.T, cases []builtinCatalogCase) {
	t.Helper()
	checkBuiltinCatalogVariation(t, variationStandard, cases)
}

// checkBuiltinCatalogVariation injects each case's name with the given builtin
// variations on a fresh frontend and asserts that the overload with the given
// parameter types exists and expands the way the GLSL spec and Rust naga's
// builtins.rs say.
func checkBuiltinCatalogVariation(t *testing.T, variation builtinVariations, cases []builtinCatalogCase) {
	t.Helper()
	for index, tc := range cases {
		t.Run(fmt.Sprintf("%s #%d", tc.name, index), func(t *testing.T) {
			f, _ := newTestContext(t)
			decl := &functionDeclaration{}
			f.injectBuiltin(decl, builtinName(tc.name), variation)
			ov := builtinOverload(t, f, decl, tc.args...)
			if ov.kind != overloadMacro {
				t.Fatalf("overload kind is %d, want a macro overload", ov.kind)
			}
			if ov.macro == nil {
				t.Fatal("the overload carries no expansion")
			}
			if ov.macro.kind != tc.kind {
				t.Fatalf("expansion kind is %d, want %d", ov.macro.kind, tc.kind)
			}
			if tc.math != nil && ov.macro.math != *tc.math {
				t.Fatalf("math is %d, want %d", ov.macro.math, *tc.math)
			}
			if tc.unary != nil && ov.macro.unary != *tc.unary {
				t.Fatalf("unary operator is %d, want %d", ov.macro.unary, *tc.unary)
			}
			if tc.binary != nil && ov.macro.binary != *tc.binary {
				t.Fatalf("binary operator is %d, want %d", ov.macro.binary, *tc.binary)
			}
			if tc.rel != nil && ov.macro.relational != *tc.rel {
				t.Fatalf("relational function is %d, want %d", ov.macro.relational, *tc.rel)
			}
			if tc.cast != nil && ov.macro.cast != *tc.cast {
				t.Fatalf("cast kind is %d, want %d", ov.macro.cast, *tc.cast)
			}
			if tc.axis != nil && ov.macro.axis != *tc.axis {
				t.Fatalf("derivative axis is %d, want %d", ov.macro.axis, *tc.axis)
			}
		})
	}
}

// TestBuiltinCatalogOneArgFloat covers the single-argument float family for
// every size it declares: the trigonometric, exponential and angle functions
// (macroMath), the float bit casts (macroBitCast) and the derivatives
// (macroDerivate), Rust's inject_one_arg_float_builtin (builtins.rs:550-603).
func TestBuiltinCatalogOneArgFloat(t *testing.T) {
	ptr := func(f ir.MathFunction) *ir.MathFunction { return &f }
	cases := []builtinCatalogCase{
		{"sin", []ir.TypeInner{scalarF32}, macroMath, ptr(ir.MathSin), nil, nil, nil, nil, nil},
		{"exp", []ir.TypeInner{scalarF32}, macroMath, ptr(ir.MathExp), nil, nil, nil, nil, nil},
		{"exp2", []ir.TypeInner{scalarF32}, macroMath, ptr(ir.MathExp2), nil, nil, nil, nil, nil},
		{"sinh", []ir.TypeInner{scalarF32}, macroMath, ptr(ir.MathSinh), nil, nil, nil, nil, nil},
		{"cos", []ir.TypeInner{scalarF32}, macroMath, ptr(ir.MathCos), nil, nil, nil, nil, nil},
		{"cosh", []ir.TypeInner{scalarF32}, macroMath, ptr(ir.MathCosh), nil, nil, nil, nil, nil},
		{"tan", []ir.TypeInner{scalarF32}, macroMath, ptr(ir.MathTan), nil, nil, nil, nil, nil},
		{"tanh", []ir.TypeInner{scalarF32}, macroMath, ptr(ir.MathTanh), nil, nil, nil, nil, nil},
		{"acos", []ir.TypeInner{scalarF32}, macroMath, ptr(ir.MathAcos), nil, nil, nil, nil, nil},
		{"asin", []ir.TypeInner{scalarF32}, macroMath, ptr(ir.MathAsin), nil, nil, nil, nil, nil},
		{"log", []ir.TypeInner{scalarF32}, macroMath, ptr(ir.MathLog), nil, nil, nil, nil, nil},
		{"log2", []ir.TypeInner{scalarF32}, macroMath, ptr(ir.MathLog2), nil, nil, nil, nil, nil},
		{"asinh", []ir.TypeInner{scalarF32}, macroMath, ptr(ir.MathAsinh), nil, nil, nil, nil, nil},
		{"acosh", []ir.TypeInner{scalarF32}, macroMath, ptr(ir.MathAcosh), nil, nil, nil, nil, nil},
		{"atanh", []ir.TypeInner{scalarF32}, macroMath, ptr(ir.MathAtanh), nil, nil, nil, nil, nil},
		{"radians", []ir.TypeInner{scalarF32}, macroMath, ptr(ir.MathRadians), nil, nil, nil, nil, nil},
		{"degrees", []ir.TypeInner{scalarF32}, macroMath, ptr(ir.MathDegrees), nil, nil, nil, nil, nil},
		// A vector argument carries the same expansion: the family covers
		// every size (builtins.rs:550-568).
		{"sin vec4", []ir.TypeInner{vecOf(ir.Vec4, scalarF32)}, macroMath, ptr(ir.MathSin), nil, nil, nil, nil, nil},
	}
	castSint, castUint, castFloat := ir.ScalarSint, ir.ScalarUint, ir.ScalarFloat
	cases = append(cases,
		builtinCatalogCase{"floatBitsToInt", []ir.TypeInner{vecOf(ir.Vec2, scalarF32)}, macroBitCast, nil, nil, nil, nil, &castSint, nil},
		builtinCatalogCase{"floatBitsToUint", []ir.TypeInner{scalarF32}, macroBitCast, nil, nil, nil, nil, &castUint, nil},
		builtinCatalogCase{"intBitsToFloat", []ir.TypeInner{vecOf(ir.Vec4, scalarI32)}, macroBitCast, nil, nil, nil, nil, &castFloat, nil},
		builtinCatalogCase{"uintBitsToFloat", []ir.TypeInner{scalarU32}, macroBitCast, nil, nil, nil, nil, &castFloat, nil},
	)
	axisX, axisY, axisW := ir.DerivativeX, ir.DerivativeY, ir.DerivativeWidth
	for _, name := range []string{"dFdx", "dFdxFine", "dFdxCoarse"} {
		cases = append(cases, builtinCatalogCase{name, []ir.TypeInner{scalarF32}, macroDerivate, nil, nil, nil, nil, nil, &axisX})
	}
	for _, name := range []string{"dFdy", "dFdyFine", "dFdyCoarse"} {
		cases = append(cases, builtinCatalogCase{name, []ir.TypeInner{scalarF32}, macroDerivate, nil, nil, nil, nil, nil, &axisY})
	}
	for _, name := range []string{"fwidth", "fwidthFine", "fwidthCoarse"} {
		cases = append(cases, builtinCatalogCase{name, []ir.TypeInner{vecOf(ir.Vec3, scalarF32)}, macroDerivate, nil, nil, nil, nil, nil, &axisW})
	}
	checkBuiltinCatalog(t, cases)
}

// TestBuiltinCatalogStandard covers the rest of Rust's
// inject_standard_builtins (builtins.rs:462-1044): pow, abs/sign, the bit
// family, the packing builtins, atan, the logical and comparison builtins,
// min/max, mix, clamp, barrier, and the common float family. Each entry names
// the expansion Rust naga gives it.
func TestBuiltinCatalogStandard(t *testing.T) {
	unaryNot := ir.UnaryLogicalNot
	relAll, relAny, relInf, relNan := ir.RelationalAll, ir.RelationalAny, ir.RelationalIsInf, ir.RelationalIsNan
	binLess, binGreater, binLessEq, binGreaterEq := ir.BinaryLess, ir.BinaryGreater, ir.BinaryLessEqual, ir.BinaryGreaterEqual
	binEqual, binNotEqual := ir.BinaryEqual, ir.BinaryNotEqual

	cases := []builtinCatalogCase{
		// pow: two arguments of the same shape (builtins.rs:628-649).
		{"pow", []ir.TypeInner{scalarF32, scalarF32}, macroMath, mathPtr(ir.MathPow), nil, nil, nil, nil, nil},
		{"pow vec3", []ir.TypeInner{vecOf(ir.Vec3, scalarF32), vecOf(ir.Vec3, scalarF32)}, macroMath, mathPtr(ir.MathPow), nil, nil, nil, nil, nil},
		// abs and sign cover f32 and i32 in the standard variation
		// (builtins.rs:650-680).
		{"abs f32", []ir.TypeInner{scalarF32}, macroMath, mathPtr(ir.MathAbs), nil, nil, nil, nil, nil},
		{"abs i32", []ir.TypeInner{scalarI32}, macroMath, mathPtr(ir.MathAbs), nil, nil, nil, nil, nil},
		{"sign vec2", []ir.TypeInner{vecOf(ir.Vec2, scalarF32)}, macroMath, mathPtr(ir.MathSign), nil, nil, nil, nil, nil},
		// The bit family (builtins.rs:681-751).
		{"bitCount i32", []ir.TypeInner{scalarI32}, macroMath, mathPtr(ir.MathCountOneBits), nil, nil, nil, nil, nil},
		{"bitCount u32", []ir.TypeInner{scalarU32}, macroMath, mathPtr(ir.MathCountOneBits), nil, nil, nil, nil, nil},
		{"bitfieldReverse u32", []ir.TypeInner{scalarU32}, macroMath, mathPtr(ir.MathReverseBits), nil, nil, nil, nil, nil},
		{"bitfieldExtract", []ir.TypeInner{scalarI32, scalarI32, scalarI32}, macroBitfieldExtract, nil, nil, nil, nil, nil, nil},
		{"bitfieldInsert", []ir.TypeInner{scalarU32, scalarU32, scalarI32, scalarI32}, macroBitfieldInsert, nil, nil, nil, nil, nil, nil},
		{"findLSB i32", []ir.TypeInner{scalarI32}, macroMath, mathPtr(ir.MathFirstTrailingBit), nil, nil, nil, nil, nil},
		{"findLSB u32", []ir.TypeInner{scalarU32}, macroFindLsbUint, nil, nil, nil, nil, nil, nil},
		{"findMSB i32", []ir.TypeInner{scalarI32}, macroMath, mathPtr(ir.MathFirstLeadingBit), nil, nil, nil, nil, nil},
		{"findMSB u32", []ir.TypeInner{scalarU32}, macroFindMsbUint, nil, nil, nil, nil, nil, nil},
		// Packing: 4x8 takes a vec4, 2x16 a vec2 (builtins.rs:752-798).
		{"packSnorm4x8", []ir.TypeInner{vecOf(ir.Vec4, scalarF32)}, macroMath, mathPtr(ir.MathPack4x8snorm), nil, nil, nil, nil, nil},
		{"packUnorm4x8", []ir.TypeInner{vecOf(ir.Vec4, scalarF32)}, macroMath, mathPtr(ir.MathPack4x8unorm), nil, nil, nil, nil, nil},
		{"packSnorm2x16", []ir.TypeInner{vecOf(ir.Vec2, scalarF32)}, macroMath, mathPtr(ir.MathPack2x16snorm), nil, nil, nil, nil, nil},
		{"packUnorm2x16", []ir.TypeInner{vecOf(ir.Vec2, scalarF32)}, macroMath, mathPtr(ir.MathPack2x16unorm), nil, nil, nil, nil, nil},
		{"packHalf2x16", []ir.TypeInner{vecOf(ir.Vec2, scalarF32)}, macroMath, mathPtr(ir.MathPack2x16float), nil, nil, nil, nil, nil},
		{"unpackSnorm4x8", []ir.TypeInner{scalarU32}, macroMath, mathPtr(ir.MathUnpack4x8snorm), nil, nil, nil, nil, nil},
		{"unpackUnorm4x8", []ir.TypeInner{scalarU32}, macroMath, mathPtr(ir.MathUnpack4x8unorm), nil, nil, nil, nil, nil},
		{"unpackSnorm2x16", []ir.TypeInner{scalarU32}, macroMath, mathPtr(ir.MathUnpack2x16snorm), nil, nil, nil, nil, nil},
		{"unpackUnorm2x16", []ir.TypeInner{scalarU32}, macroMath, mathPtr(ir.MathUnpack2x16unorm), nil, nil, nil, nil, nil},
		{"unpackHalf2x16", []ir.TypeInner{scalarU32}, macroMath, mathPtr(ir.MathUnpack2x16float), nil, nil, nil, nil, nil},
		// atan and atan2 share one generator (builtins.rs:799-830).
		{"atan", []ir.TypeInner{scalarF32}, macroMath, mathPtr(ir.MathAtan), nil, nil, nil, nil, nil},
		{"atan two args", []ir.TypeInner{scalarF32, scalarF32}, macroMath, mathPtr(ir.MathAtan2), nil, nil, nil, nil, nil},
		// Logical builtins over boolean vectors (builtins.rs:831-855).
		{"all", []ir.TypeInner{vecOf(ir.Vec2, scalarBool)}, macroRelational, nil, nil, nil, &relAll, nil, nil},
		{"any", []ir.TypeInner{vecOf(ir.Vec4, scalarBool)}, macroRelational, nil, nil, nil, &relAny, nil, nil},
		{"not", []ir.TypeInner{vecOf(ir.Vec3, scalarBool)}, macroUnary, nil, &unaryNot, nil, nil, nil, nil},
		// Vector comparisons (builtins.rs:856-912).
		{"lessThan", []ir.TypeInner{vecOf(ir.Vec2, scalarF32), vecOf(ir.Vec2, scalarF32)}, macroBinary, nil, nil, &binLess, nil, nil, nil},
		{"greaterThan", []ir.TypeInner{vecOf(ir.Vec3, scalarI32), vecOf(ir.Vec3, scalarI32)}, macroBinary, nil, nil, &binGreater, nil, nil, nil},
		{"lessThanEqual", []ir.TypeInner{vecOf(ir.Vec4, scalarU32), vecOf(ir.Vec4, scalarU32)}, macroBinary, nil, nil, &binLessEq, nil, nil, nil},
		{"greaterThanEqual", []ir.TypeInner{vecOf(ir.Vec2, scalarF32), vecOf(ir.Vec2, scalarF32)}, macroBinary, nil, nil, &binGreaterEq, nil, nil, nil},
		{"equal f32", []ir.TypeInner{vecOf(ir.Vec2, scalarF32), vecOf(ir.Vec2, scalarF32)}, macroBinary, nil, nil, &binEqual, nil, nil, nil},
		{"equal bool", []ir.TypeInner{vecOf(ir.Vec3, scalarBool), vecOf(ir.Vec3, scalarBool)}, macroBinary, nil, nil, &binEqual, nil, nil, nil},
		{"notEqual", []ir.TypeInner{vecOf(ir.Vec4, scalarF32), vecOf(ir.Vec4, scalarF32)}, macroBinary, nil, nil, &binNotEqual, nil, nil, nil},
		// min/max: the splat shapes (builtins.rs:913-953).
		{"min", []ir.TypeInner{scalarF32, scalarF32}, macroSplatted, mathPtr(ir.MathMin), nil, nil, nil, nil, nil},
		{"max vec4 scalar", []ir.TypeInner{vecOf(ir.Vec4, scalarF32), scalarF32}, macroSplatted, mathPtr(ir.MathMax), nil, nil, nil, nil, nil},
		{"min vec2 vec2", []ir.TypeInner{vecOf(ir.Vec2, scalarI32), vecOf(ir.Vec2, scalarI32)}, macroSplatted, mathPtr(ir.MathMin), nil, nil, nil, nil, nil},
		// mix: the boolean select, the scalar weight and the vector weight
		// (builtins.rs:954-998).
		{"mix scalar", []ir.TypeInner{scalarF32, scalarF32, scalarF32}, macroSplatted, mathPtr(ir.MathMix), nil, nil, nil, nil, nil},
		{"mix boolean", []ir.TypeInner{vecOf(ir.Vec2, scalarF32), vecOf(ir.Vec2, scalarF32), vecOf(ir.Vec2, scalarBool)}, macroMixBoolean, nil, nil, nil, nil, nil, nil},
		{"mix splatted weight", []ir.TypeInner{vecOf(ir.Vec3, scalarF32), vecOf(ir.Vec3, scalarF32), scalarF32}, macroSplatted, mathPtr(ir.MathMix), nil, nil, nil, nil, nil},
		// clamp: vector bounds and splatted bounds (builtins.rs:999-1037).
		{"clamp scalar", []ir.TypeInner{scalarF32, scalarF32, scalarF32}, macroClamp, nil, nil, nil, nil, nil, nil},
		{"clamp vector", []ir.TypeInner{vecOf(ir.Vec4, scalarF32), vecOf(ir.Vec4, scalarF32), vecOf(ir.Vec4, scalarF32)}, macroClamp, nil, nil, nil, nil, nil, nil},
		{"clamp splatted uint", []ir.TypeInner{vecOf(ir.Vec2, scalarU32), scalarU32, scalarU32}, macroClamp, nil, nil, nil, nil, nil, nil},
		// barrier is the void statement builtin (builtins.rs:2039).
		{"barrier", nil, macroBarrier, nil, nil, nil, nil, nil, nil},
		// The common single-argument float family (builtins.rs:1231-1268).
		{"ceil", []ir.TypeInner{scalarF32}, macroMath, mathPtr(ir.MathCeil), nil, nil, nil, nil, nil},
		{"round", []ir.TypeInner{scalarF32}, macroMath, mathPtr(ir.MathRound), nil, nil, nil, nil, nil},
		{"roundEven", []ir.TypeInner{vecOf(ir.Vec2, scalarF32)}, macroMath, mathPtr(ir.MathRound), nil, nil, nil, nil, nil},
		{"floor", []ir.TypeInner{scalarF32}, macroMath, mathPtr(ir.MathFloor), nil, nil, nil, nil, nil},
		{"fract", []ir.TypeInner{scalarF32}, macroMath, mathPtr(ir.MathFract), nil, nil, nil, nil, nil},
		{"trunc", []ir.TypeInner{scalarF32}, macroMath, mathPtr(ir.MathTrunc), nil, nil, nil, nil, nil},
		{"sqrt", []ir.TypeInner{scalarF32}, macroMath, mathPtr(ir.MathSqrt), nil, nil, nil, nil, nil},
		{"inversesqrt", []ir.TypeInner{scalarF32}, macroMath, mathPtr(ir.MathInverseSqrt), nil, nil, nil, nil, nil},
		{"normalize", []ir.TypeInner{vecOf(ir.Vec3, scalarF32)}, macroMath, mathPtr(ir.MathNormalize), nil, nil, nil, nil, nil},
		{"length", []ir.TypeInner{vecOf(ir.Vec2, scalarF32)}, macroMath, mathPtr(ir.MathLength), nil, nil, nil, nil, nil},
		{"isinf", []ir.TypeInner{scalarF32}, macroRelational, nil, nil, nil, &relInf, nil, nil},
		{"isnan", []ir.TypeInner{scalarF32}, macroRelational, nil, nil, nil, &relNan, nil, nil},
		// dot, reflect, distance and ldexp: ldexp's second argument is an int
		// vector (builtins.rs:1269-1301).
		{"dot", []ir.TypeInner{vecOf(ir.Vec2, scalarF32), vecOf(ir.Vec2, scalarF32)}, macroMath, mathPtr(ir.MathDot), nil, nil, nil, nil, nil},
		{"reflect", []ir.TypeInner{vecOf(ir.Vec3, scalarF32), vecOf(ir.Vec3, scalarF32)}, macroMath, mathPtr(ir.MathReflect), nil, nil, nil, nil, nil},
		{"distance", []ir.TypeInner{vecOf(ir.Vec4, scalarF32), vecOf(ir.Vec4, scalarF32)}, macroMath, mathPtr(ir.MathDistance), nil, nil, nil, nil, nil},
		{"ldexp", []ir.TypeInner{vecOf(ir.Vec2, scalarF32), vecOf(ir.Vec2, scalarI32)}, macroMath, mathPtr(ir.MathLdexp), nil, nil, nil, nil, nil},
		// The matrix builtins (builtins.rs:1302-1353).
		{"transpose", []ir.TypeInner{matOf(ir.Vec2, ir.Vec3, scalarF32)}, macroMath, mathPtr(ir.MathTranspose), nil, nil, nil, nil, nil},
		{"inverse", []ir.TypeInner{matOf(ir.Vec3, ir.Vec3, scalarF32)}, macroMath, mathPtr(ir.MathInverse), nil, nil, nil, nil, nil},
		{"determinant", []ir.TypeInner{matOf(ir.Vec4, ir.Vec4, scalarF32)}, macroMath, mathPtr(ir.MathDeterminant), nil, nil, nil, nil, nil},
		// mod expands to the floor form, step splats its edge
		// (builtins.rs:1354-1394).
		{"mod", []ir.TypeInner{scalarF32, scalarF32}, macroMod, nil, nil, nil, nil, nil, nil},
		{"step", []ir.TypeInner{scalarF32, scalarF32}, macroSplatted, mathPtr(ir.MathStep), nil, nil, nil, nil, nil},
		// cross, outerProduct, faceforward, fma, refract, smoothstep
		// (builtins.rs:1395-1530).
		{"cross", []ir.TypeInner{vecOf(ir.Vec3, scalarF32), vecOf(ir.Vec3, scalarF32)}, macroMath, mathPtr(ir.MathCross), nil, nil, nil, nil, nil},
		{"outerProduct", []ir.TypeInner{vecOf(ir.Vec2, scalarF32), vecOf(ir.Vec3, scalarF32)}, macroMath, mathPtr(ir.MathOuter), nil, nil, nil, nil, nil},
		{"faceforward", []ir.TypeInner{scalarF32, scalarF32, scalarF32}, macroMath, mathPtr(ir.MathFaceForward), nil, nil, nil, nil, nil},
		{"fma", []ir.TypeInner{vecOf(ir.Vec4, scalarF32), vecOf(ir.Vec4, scalarF32), vecOf(ir.Vec4, scalarF32)}, macroMath, mathPtr(ir.MathFma), nil, nil, nil, nil, nil},
		{"refract", []ir.TypeInner{vecOf(ir.Vec3, scalarF32), vecOf(ir.Vec3, scalarF32), scalarF32}, macroMath, mathPtr(ir.MathRefract), nil, nil, nil, nil, nil},
		{"smoothstep", []ir.TypeInner{scalarF32, scalarF32, scalarF32}, macroSmoothStep, nil, nil, nil, nil, nil, nil},
		{"smoothstep vector", []ir.TypeInner{vecOf(ir.Vec4, scalarF32), vecOf(ir.Vec4, scalarF32), vecOf(ir.Vec4, scalarF32)}, macroSmoothStep, nil, nil, nil, nil, nil, nil},
	}
	checkBuiltinCatalog(t, cases)
}

// TestBuiltinCatalogSamplers covers the sampler type constructors
// (builtins.rs:467-548): every shape maps to its image dimension and arrayed
// bit, the kind prefix selects the texel type, and a shadow sampler also
// declares its depth image.
func TestBuiltinCatalogSamplers(t *testing.T) {
	sampler := ir.TypeInner(ir.SamplerType{})
	cases := []builtinCatalogCase{
		{"sampler1D", []ir.TypeInner{sampledImage(ir.Dim1D, false, false, ir.ScalarFloat), sampler}, macroSampler, nil, nil, nil, nil, nil, nil},
		{"sampler1DArray", []ir.TypeInner{sampledImage(ir.Dim1D, true, false, ir.ScalarFloat), sampler}, macroSampler, nil, nil, nil, nil, nil, nil},
		{"sampler2D", []ir.TypeInner{sampledImage(ir.Dim2D, false, false, ir.ScalarFloat), sampler}, macroSampler, nil, nil, nil, nil, nil, nil},
		{"sampler2DArray", []ir.TypeInner{sampledImage(ir.Dim2D, true, false, ir.ScalarFloat), sampler}, macroSampler, nil, nil, nil, nil, nil, nil},
		{"sampler2DMS", []ir.TypeInner{sampledImage(ir.Dim2D, false, true, ir.ScalarFloat), sampler}, macroSampler, nil, nil, nil, nil, nil, nil},
		{"sampler2DMSArray", []ir.TypeInner{sampledImage(ir.Dim2D, true, true, ir.ScalarFloat), sampler}, macroSampler, nil, nil, nil, nil, nil, nil},
		{"sampler3D", []ir.TypeInner{sampledImage(ir.Dim3D, false, false, ir.ScalarFloat), sampler}, macroSampler, nil, nil, nil, nil, nil, nil},
		{"samplerCube", []ir.TypeInner{sampledImage(ir.DimCube, false, false, ir.ScalarFloat), sampler}, macroSampler, nil, nil, nil, nil, nil, nil},
		{"samplerCubeArray", []ir.TypeInner{sampledImage(ir.DimCube, true, false, ir.ScalarFloat), sampler}, macroSampler, nil, nil, nil, nil, nil, nil},
		// The kind prefix selects the texel type (builtins.rs:467-476).
		{"usampler2D", []ir.TypeInner{sampledImage(ir.Dim2D, false, false, ir.ScalarUint), sampler}, macroSampler, nil, nil, nil, nil, nil, nil},
		{"isampler2D", []ir.TypeInner{sampledImage(ir.Dim2D, false, false, ir.ScalarSint), sampler}, macroSampler, nil, nil, nil, nil, nil, nil},
		// Shadow samplers are usable as both the sampled and the depth image,
		// so the first declared overload matches the sampled one
		// (builtins.rs:531-548).
		{"sampler1DShadow", []ir.TypeInner{sampledImage(ir.Dim1D, false, false, ir.ScalarFloat), ir.SamplerType{Comparison: true}}, macroSamplerShadow, nil, nil, nil, nil, nil, nil},
		{"sampler1DArrayShadow", []ir.TypeInner{sampledImage(ir.Dim1D, true, false, ir.ScalarFloat), ir.SamplerType{Comparison: true}}, macroSamplerShadow, nil, nil, nil, nil, nil, nil},
		{"sampler2DShadow", []ir.TypeInner{sampledImage(ir.Dim2D, false, false, ir.ScalarFloat), ir.SamplerType{Comparison: true}}, macroSamplerShadow, nil, nil, nil, nil, nil, nil},
		{"sampler2DArrayShadow", []ir.TypeInner{sampledImage(ir.Dim2D, true, false, ir.ScalarFloat), ir.SamplerType{Comparison: true}}, macroSamplerShadow, nil, nil, nil, nil, nil, nil},
		{"samplerCubeShadow", []ir.TypeInner{sampledImage(ir.DimCube, false, false, ir.ScalarFloat), ir.SamplerType{Comparison: true}}, macroSamplerShadow, nil, nil, nil, nil, nil, nil},
		{"samplerCubeArrayShadow", []ir.TypeInner{sampledImage(ir.DimCube, true, false, ir.ScalarFloat), ir.SamplerType{Comparison: true}}, macroSamplerShadow, nil, nil, nil, nil, nil, nil},
	}
	checkBuiltinCatalog(t, cases)

	// The depth overload of a shadow sampler matches the depth image.
	f, _ := newTestContext(t)
	decl := &functionDeclaration{}
	f.injectBuiltin(decl, "sampler2DShadow", variationStandard)
	ov := builtinOverload(t, f, decl, depthImage(ir.Dim2D, false), ir.SamplerType{Comparison: true})
	if ov.macro.kind != macroSamplerShadow {
		t.Fatalf("the depth overload's kind is %d, want macroSamplerShadow", ov.macro.kind)
	}
}

// TestBuiltinCatalogTextureFamily covers the texture, image and texture-query
// overloads (builtins.rs:102-458): the coordinate, level, gradient, offset and
// bias parameters of each form, and the storage-image format placeholder.
func TestBuiltinCatalogTextureFamily(t *testing.T) {
	tex2D := sampledImage(ir.Dim2D, false, false, ir.ScalarFloat)
	tex2DArray := sampledImage(ir.Dim2D, true, false, ir.ScalarFloat)
	sampledKind := ir.StorageAccessLoad | ir.StorageAccessStore
	cases := []builtinCatalogCase{
		// texture() and its level, gradient, offset and projection forms.
		{"texture", []ir.TypeInner{tex2D, vecOf(ir.Vec2, scalarF32)}, macroTexture, nil, nil, nil, nil, nil, nil},
		{"textureGrad", []ir.TypeInner{tex2D, vecOf(ir.Vec2, scalarF32), vecOf(ir.Vec2, scalarF32), vecOf(ir.Vec2, scalarF32)}, macroTexture, nil, nil, nil, nil, nil, nil},
		{"textureGradOffset", []ir.TypeInner{tex2D, vecOf(ir.Vec2, scalarF32), vecOf(ir.Vec2, scalarF32), vecOf(ir.Vec2, scalarF32), vecOf(ir.Vec2, scalarI32)}, macroTexture, nil, nil, nil, nil, nil, nil},
		{"textureLod", []ir.TypeInner{tex2D, vecOf(ir.Vec2, scalarF32), scalarF32}, macroTexture, nil, nil, nil, nil, nil, nil},
		{"textureLodOffset", []ir.TypeInner{tex2D, vecOf(ir.Vec2, scalarF32), scalarF32, vecOf(ir.Vec2, scalarI32)}, macroTexture, nil, nil, nil, nil, nil, nil},
		{"textureOffset", []ir.TypeInner{tex2D, vecOf(ir.Vec2, scalarF32), vecOf(ir.Vec2, scalarI32)}, macroTexture, nil, nil, nil, nil, nil, nil},
		{"textureProj", []ir.TypeInner{tex2D, vecOf(ir.Vec3, scalarF32)}, macroTexture, nil, nil, nil, nil, nil, nil},
		{"textureProjGrad", []ir.TypeInner{tex2D, vecOf(ir.Vec3, scalarF32), vecOf(ir.Vec2, scalarF32), vecOf(ir.Vec2, scalarF32)}, macroTexture, nil, nil, nil, nil, nil, nil},
		{"textureProjLod", []ir.TypeInner{tex2D, vecOf(ir.Vec3, scalarF32), scalarF32}, macroTexture, nil, nil, nil, nil, nil, nil},
		{"textureProjOffset", []ir.TypeInner{tex2D, vecOf(ir.Vec3, scalarF32), vecOf(ir.Vec2, scalarI32)}, macroTexture, nil, nil, nil, nil, nil, nil},
		// textureSize: the mip level is an int, and a multisampled image has
		// none (builtins.rs:268-296).
		{"textureSize", []ir.TypeInner{tex2D, scalarI32}, macroTextureSize, nil, nil, nil, nil, nil, nil},
		{"textureSize multisampled", []ir.TypeInner{sampledImage(ir.Dim2D, false, true, ir.ScalarFloat)}, macroTextureSize, nil, nil, nil, nil, nil, nil},
		// textureQueryLevels takes the image alone (builtins.rs:297-316).
		{"textureQueryLevels", []ir.TypeInner{tex2D}, macroTextureQueryLevels, nil, nil, nil, nil, nil, nil},
		// texelFetch: a signed integer coordinate and a level or sample index
		// (builtins.rs:317-347).
		{"texelFetch", []ir.TypeInner{tex2D, vecOf(ir.Vec2, scalarI32), scalarI32}, macroImageLoad, nil, nil, nil, nil, nil, nil},
		{"texelFetch arrayed coord", []ir.TypeInner{tex2DArray, vecOf(ir.Vec3, scalarI32), scalarI32}, macroImageLoad, nil, nil, nil, nil, nil, nil},
		{"texelFetchOffset", []ir.TypeInner{tex2D, vecOf(ir.Vec2, scalarI32), scalarI32, vecOf(ir.Vec2, scalarI32)}, macroImageLoad, nil, nil, nil, nil, nil, nil},
		// imageSize, imageLoad and imageStore are storage images; the format
		// is a placeholder for the declaration's own (builtins.rs:348-456).
		{"imageSize", []ir.TypeInner{storageImage(ir.Dim2D, false, ir.ScalarFloat, sampledKind)}, macroTextureSize, nil, nil, nil, nil, nil, nil},
		{"imageLoad", []ir.TypeInner{storageImage(ir.Dim2D, false, ir.ScalarFloat, ir.StorageAccessLoad), vecOf(ir.Vec2, scalarI32)}, macroImageLoad, nil, nil, nil, nil, nil, nil},
		{"imageStore", []ir.TypeInner{storageImage(ir.Dim2D, false, ir.ScalarUint, ir.StorageAccessStore), vecOf(ir.Vec2, scalarI32), vecOf(ir.Vec4, scalarU32)}, macroImageStore, nil, nil, nil, nil, nil, nil},
	}
	checkBuiltinCatalog(t, cases)

	// imageStore is the one void builtin: it yields no value.
	f, _ := newTestContext(t)
	decl := &functionDeclaration{}
	f.injectBuiltin(decl, "imageStore", variationStandard)
	ov := builtinOverload(t, f, decl, storageImage(ir.Dim2D, false, ir.ScalarFloat, ir.StorageAccessStore), vecOf(ir.Vec2, scalarI32), vecOf(ir.Vec4, scalarF32))
	if !ov.void {
		t.Fatal("imageStore is not marked void")
	}

	// textureSamples is the sample-count query of a multisampled sampler
	// (GLSL 4.50 §8.9): one parameter, the sampler, and no other shape. The
	// multisampled 2D array is the shape the standard variation does not
	// request on its own (builtins.rs:2313-2365).
	checkBuiltinCatalogVariation(t, variationStandard|variationD2MultiTexturesArray, []builtinCatalogCase{
		{"textureSamples 2DMS", []ir.TypeInner{sampledImage(ir.Dim2D, false, true, ir.ScalarFloat)}, macroTextureSamples, nil, nil, nil, nil, nil, nil},
		{"textureSamples 2DMSArray", []ir.TypeInner{sampledImage(ir.Dim2D, true, true, ir.ScalarFloat)}, macroTextureSamples, nil, nil, nil, nil, nil, nil},
		{"textureSamples usampler2DMS", []ir.TypeInner{sampledImage(ir.Dim2D, false, true, ir.ScalarUint)}, macroTextureSamples, nil, nil, nil, nil, nil, nil},
	})
}

// TestBuiltinCatalogDoubleVariation covers Rust's inject_double_builtin
// (builtins.rs:1047-1217): the double overloads of abs/sign, min/max, mix,
// clamp, the comparisons, and every family of the common builtins at width 8.
func TestBuiltinCatalogDoubleVariation(t *testing.T) {
	ptr := func(f ir.MathFunction) *ir.MathFunction { return &f }
	binLess, binEqual := ir.BinaryLess, ir.BinaryEqual
	cases := []builtinCatalogCase{
		{"abs f64", []ir.TypeInner{scalarF64}, macroMath, ptr(ir.MathAbs), nil, nil, nil, nil, nil},
		{"sign f64", []ir.TypeInner{vecOf(ir.Vec2, scalarF64)}, macroMath, ptr(ir.MathSign), nil, nil, nil, nil, nil},
		{"min f64", []ir.TypeInner{vecOf(ir.Vec2, scalarF64), scalarF64}, macroSplatted, ptr(ir.MathMin), nil, nil, nil, nil, nil},
		{"max f64", []ir.TypeInner{scalarF64, scalarF64}, macroSplatted, ptr(ir.MathMax), nil, nil, nil, nil, nil},
		{"mix f64", []ir.TypeInner{vecOf(ir.Vec4, scalarF64), vecOf(ir.Vec4, scalarF64), vecOf(ir.Vec4, scalarF64)}, macroSplatted, ptr(ir.MathMix), nil, nil, nil, nil, nil},
		{"mix f64 boolean", []ir.TypeInner{vecOf(ir.Vec4, scalarF64), vecOf(ir.Vec4, scalarF64), vecOf(ir.Vec4, scalarBool)}, macroMixBoolean, nil, nil, nil, nil, nil, nil},
		{"clamp f64", []ir.TypeInner{vecOf(ir.Vec2, scalarF64), scalarF64, scalarF64}, macroClamp, nil, nil, nil, nil, nil, nil},
		{"lessThan f64", []ir.TypeInner{vecOf(ir.Vec3, scalarF64), vecOf(ir.Vec3, scalarF64)}, macroBinary, nil, nil, &binLess, nil, nil, nil},
		{"equal f64", []ir.TypeInner{vecOf(ir.Vec2, scalarF64), vecOf(ir.Vec2, scalarF64)}, macroBinary, nil, nil, &binEqual, nil, nil, nil},
		// The common builtins at width 8: one name per family.
		{"ceil f64", []ir.TypeInner{scalarF64}, macroMath, ptr(ir.MathCeil), nil, nil, nil, nil, nil},
		{"dot f64", []ir.TypeInner{vecOf(ir.Vec3, scalarF64), vecOf(ir.Vec3, scalarF64)}, macroMath, ptr(ir.MathDot), nil, nil, nil, nil, nil},
		{"transpose f64", []ir.TypeInner{matOf(ir.Vec2, ir.Vec4, scalarF64)}, macroMath, ptr(ir.MathTranspose), nil, nil, nil, nil, nil},
		{"inverse f64", []ir.TypeInner{matOf(ir.Vec3, ir.Vec3, scalarF64)}, macroMath, ptr(ir.MathInverse), nil, nil, nil, nil, nil},
		{"determinant f64", []ir.TypeInner{matOf(ir.Vec4, ir.Vec4, scalarF64)}, macroMath, ptr(ir.MathDeterminant), nil, nil, nil, nil, nil},
		{"mod f64", []ir.TypeInner{scalarF64, scalarF64}, macroMod, nil, nil, nil, nil, nil, nil},
		{"step f64", []ir.TypeInner{scalarF64, scalarF64}, macroSplatted, ptr(ir.MathStep), nil, nil, nil, nil, nil},
		{"cross f64", []ir.TypeInner{vecOf(ir.Vec3, scalarF64), vecOf(ir.Vec3, scalarF64)}, macroMath, ptr(ir.MathCross), nil, nil, nil, nil, nil},
		{"outerProduct f64", []ir.TypeInner{vecOf(ir.Vec2, scalarF64), vecOf(ir.Vec2, scalarF64)}, macroMath, ptr(ir.MathOuter), nil, nil, nil, nil, nil},
		{"faceforward f64", []ir.TypeInner{scalarF64, scalarF64, scalarF64}, macroMath, ptr(ir.MathFaceForward), nil, nil, nil, nil, nil},
		{"fma f64", []ir.TypeInner{scalarF64, scalarF64, scalarF64}, macroMath, ptr(ir.MathFma), nil, nil, nil, nil, nil},
		{"refract f64", []ir.TypeInner{scalarF64, scalarF64, scalarF32}, macroMath, ptr(ir.MathRefract), nil, nil, nil, nil, nil},
		{"smoothstep f64", []ir.TypeInner{scalarF64, scalarF64, scalarF64}, macroSmoothStep, nil, nil, nil, nil, nil, nil},
	}
	for i := range cases {
		// The double table is injected with both variations (a fresh
		// declaration always asks for the standard set too).
		tc := cases[i]
		t.Run(tc.name, func(t *testing.T) {
			f, _ := newTestContext(t)
			decl := &functionDeclaration{}
			f.injectBuiltin(decl, builtinName(tc.name), variationDouble)
			ov := builtinOverload(t, f, decl, tc.args...)
			if ov.kind != overloadMacro {
				t.Fatalf("overload kind is %d, want a macro overload", ov.kind)
			}
			if ov.macro.kind != tc.kind {
				t.Fatalf("expansion kind is %d, want %d", ov.macro.kind, tc.kind)
			}
			if tc.math != nil && ov.macro.math != *tc.math {
				t.Fatalf("math is %d, want %d", ov.macro.math, *tc.math)
			}
		})
	}
}

// TestBuiltinTextureSampleShapeNames pins textureSampleShape's table
// (builtins.rs:120-146): which texture() family members carry a projection
// divisor, take an offset, and where their level of detail comes from.
func TestBuiltinTextureSampleShapeNames(t *testing.T) {
	tests := []struct {
		name   string
		proj   bool
		offset bool
		level  textureLevelType
	}{
		{"texture", false, false, textureLevelNone},
		{"textureGrad", false, false, textureLevelGrad},
		{"textureGradOffset", false, true, textureLevelGrad},
		{"textureLod", false, false, textureLevelLod},
		{"textureLodOffset", false, true, textureLevelLod},
		{"textureOffset", false, true, textureLevelNone},
		{"textureProj", true, false, textureLevelNone},
		{"textureProjGrad", true, false, textureLevelGrad},
		{"textureProjGradOffset", true, true, textureLevelGrad},
		{"textureProjLod", true, false, textureLevelLod},
		{"textureProjLodOffset", true, true, textureLevelLod},
		{"textureProjOffset", true, true, textureLevelNone},
		// A name outside the family carries no shape of its own.
		{"textureSize", false, false, textureLevelNone},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			proj, offset, level := textureSampleShape(test.name)
			if proj != test.proj || offset != test.offset || level != test.level {
				t.Fatalf("shape is (%v, %v, %d), want (%v, %v, %d)", proj, offset, level, test.proj, test.offset, test.level)
			}
		})
	}
}

// TestBuiltinComparisonOperatorNames pins comparisonOperator's table
// (builtins.rs:873-879, 904-908): the four ordered and two equality
// comparisons, with notEqual as the fallback.
func TestBuiltinComparisonOperatorNames(t *testing.T) {
	tests := []struct {
		name string
		want ir.BinaryOperator
	}{
		{"lessThan", ir.BinaryLess},
		{"greaterThan", ir.BinaryGreater},
		{"lessThanEqual", ir.BinaryLessEqual},
		{"greaterThanEqual", ir.BinaryGreaterEqual},
		{"equal", ir.BinaryEqual},
		{"notEqual", ir.BinaryNotEqual},
		{"somethingElse", ir.BinaryNotEqual},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := comparisonOperator(test.name); got != test.want {
				t.Fatalf("operator is %d, want %d", got, test.want)
			}
		})
	}
}

// TestBuiltinAnykindSamplerNames pins anykindSampler (builtins.rs:467-476): the
// kind prefix selects the texel kind and is stripped from the shape name.
func TestBuiltinAnykindSamplerNames(t *testing.T) {
	tests := []struct {
		name    string
		sampler string
		kind    ir.ScalarKind
		ok      bool
	}{
		{"sampler2D", "sampler2D", ir.ScalarFloat, true},
		{"usampler2D", "sampler2D", ir.ScalarUint, true},
		{"isampler3D", "sampler3D", ir.ScalarSint, true},
		{"image2D", "", 0, false},
		{"vec4", "", 0, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sampler, kind, ok := anykindSampler(test.name)
			if sampler != test.sampler || kind != test.kind || ok != test.ok {
				t.Fatalf("anykindSampler = (%q, %d, %v), want (%q, %d, %v)", sampler, kind, ok, test.sampler, test.kind, test.ok)
			}
		})
	}
}

// TestBuiltinAnykindSamplerRejectsUnknownShape covers the default arm of
// inject_anykind_sampler (builtins.rs:478-511): a name with the sampler prefix
// that is not a GLSL sampler shape declares nothing.
func TestBuiltinAnykindSamplerRejectsUnknownShape(t *testing.T) {
	f, _ := newTestContext(t)
	decl := &functionDeclaration{}
	f.injectBuiltin(decl, "samplerBogus", variationStandard)
	if len(decl.overloads) != 0 {
		t.Fatalf("injected %d overloads for an unknown sampler shape", len(decl.overloads))
	}
}

// TestBuiltinImageMultisampled pins imageMultisampled (builtins.rs:1615): only
// the sampled and depth classes can carry multiple samples.
func TestBuiltinImageMultisampled(t *testing.T) {
	tests := []struct {
		name  string
		image ir.ImageType
		want  bool
	}{
		{"sampled multisampled", ir.ImageType{Class: ir.ImageClassSampled, Multisampled: true}, true},
		{"depth multisampled", ir.ImageType{Class: ir.ImageClassDepth, Multisampled: true}, true},
		{"sampled single", ir.ImageType{Class: ir.ImageClassSampled}, false},
		{"storage", ir.ImageType{Class: ir.ImageClassStorage, Multisampled: true}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := imageMultisampled(test.image); got != test.want {
				t.Fatalf("imageMultisampled = %v, want %v", got, test.want)
			}
		})
	}
}

// TestTextureArgsGeneratorOptions pins textureArgsGenerator
// (builtins.rs:2313-2365): the standard set alone declares no cube arrays and
// no multisampled 2D arrays, and each extra option adds exactly its shapes.
func TestTextureArgsGeneratorOptions(t *testing.T) {
	collect := func(options textureArgsOptions) map[string]bool {
		f, _ := newTestContext(t)
		shapes := map[string]bool{}
		f.textureArgsGenerator(options, func(kind ir.ScalarKind, dim ir.ImageDimension, arrayed, multi, shadow bool) {
			key := shapeKey(kind, dim, arrayed, multi, shadow)
			shapes[key] = true
		})
		return shapes
	}

	standard := collect(textureArgsStandard | textureArgsShadow)
	if standard[shapeKey(ir.ScalarFloat, ir.DimCube, true, false, false)] {
		t.Fatal("the standard set declared a cube array image")
	}
	if standard[shapeKey(ir.ScalarFloat, ir.Dim2D, true, true, false)] {
		t.Fatal("the standard set declared a multisampled 2D array image")
	}
	if !standard[shapeKey(ir.ScalarFloat, ir.Dim2D, false, false, false)] {
		t.Fatal("the standard set is missing the plain 2D image")
	}
	if !standard[shapeKey(ir.ScalarFloat, ir.Dim2D, false, false, true)] {
		t.Fatal("the shadow option is missing the 2D depth image")
	}
	if standard[shapeKey(ir.ScalarUint, ir.Dim2D, false, false, true)] {
		t.Fatal("a depth image was declared for a non-float kind")
	}

	withCubeArray := collect(textureArgsStandard | textureArgsCubeArray)
	if !withCubeArray[shapeKey(ir.ScalarFloat, ir.DimCube, true, false, false)] {
		t.Fatal("the cube array option is missing the cube array image")
	}

	withMulti := collect(textureArgsStandard | textureArgsMulti)
	if !withMulti[shapeKey(ir.ScalarFloat, ir.Dim2D, false, true, false)] {
		t.Fatal("the multi option is missing the multisampled 2D image")
	}

	withD2MultiArray := collect(textureArgsStandard | textureArgsMulti | textureArgsD2MultiArray)
	if !withD2MultiArray[shapeKey(ir.ScalarFloat, ir.Dim2D, true, true, false)] {
		t.Fatal("the 2D multisampled array option is missing its image")
	}
}

// shapeKey renders one generated image shape as a comparable key.
func shapeKey(kind ir.ScalarKind, dim ir.ImageDimension, arrayed, multi, shadow bool) string {
	key := ""
	switch kind {
	case ir.ScalarFloat:
		key += "f"
	case ir.ScalarUint:
		key += "u"
	default:
		key += "i"
	}
	key += string(rune('0' + int(dim)))
	if arrayed {
		key += "a"
	}
	if multi {
		key += "m"
	}
	if shadow {
		key += "s"
	}
	return key
}
