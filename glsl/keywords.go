// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl

// glslKeywords contains all GLSL reserved words.
// This includes current keywords, future reserved words, and built-in names.
// Based on GLSL 4.60 and GLSL ES 3.20 specifications.
var glslKeywords = map[string]struct{}{
	// Basic types
	"void": {}, "bool": {}, "int": {}, "uint": {}, "float": {}, "double": {},

	// Vector types
	"vec2": {}, "vec3": {}, "vec4": {},
	"ivec2": {}, "ivec3": {}, "ivec4": {},
	"uvec2": {}, "uvec3": {}, "uvec4": {},
	"bvec2": {}, "bvec3": {}, "bvec4": {},
	"dvec2": {}, "dvec3": {}, "dvec4": {},

	// Matrix types
	"mat2": {}, "mat3": {}, "mat4": {},
	"mat2x2": {}, "mat2x3": {}, "mat2x4": {},
	"mat3x2": {}, "mat3x3": {}, "mat3x4": {},
	"mat4x2": {}, "mat4x3": {}, "mat4x4": {},
	"dmat2": {}, "dmat3": {}, "dmat4": {},
	"dmat2x2": {}, "dmat2x3": {}, "dmat2x4": {},
	"dmat3x2": {}, "dmat3x3": {}, "dmat3x4": {},
	"dmat4x2": {}, "dmat4x3": {}, "dmat4x4": {},

	// Sampler types
	"sampler": {}, "sampler1D": {}, "sampler2D": {}, "sampler3D": {},
	"samplerCube": {}, "sampler2DRect": {},
	"sampler1DShadow": {}, "sampler2DShadow": {}, "samplerCubeShadow": {}, "sampler2DRectShadow": {},
	"sampler1DArray": {}, "sampler2DArray": {},
	"sampler1DArrayShadow": {}, "sampler2DArrayShadow": {},
	"samplerCubeArray": {}, "samplerCubeArrayShadow": {},
	"samplerBuffer": {}, "sampler2DMS": {}, "sampler2DMSArray": {},

	// Integer sampler types
	"isampler1D": {}, "isampler2D": {}, "isampler3D": {},
	"isamplerCube": {}, "isampler2DRect": {},
	"isampler1DArray": {}, "isampler2DArray": {},
	"isamplerCubeArray": {},
	"isamplerBuffer":    {}, "isampler2DMS": {}, "isampler2DMSArray": {},

	// Unsigned integer sampler types
	"usampler1D": {}, "usampler2D": {}, "usampler3D": {},
	"usamplerCube": {}, "usampler2DRect": {},
	"usampler1DArray": {}, "usampler2DArray": {},
	"usamplerCubeArray": {},
	"usamplerBuffer":    {}, "usampler2DMS": {}, "usampler2DMSArray": {},

	// Image types
	"image1D": {}, "image2D": {}, "image3D": {},
	"imageCube": {}, "image2DRect": {},
	"image1DArray": {}, "image2DArray": {},
	"imageCubeArray": {},
	"imageBuffer":    {}, "image2DMS": {}, "image2DMSArray": {},
	"iimage1D": {}, "iimage2D": {}, "iimage3D": {},
	"iimageCube": {}, "iimage2DRect": {},
	"iimage1DArray": {}, "iimage2DArray": {},
	"iimageCubeArray": {},
	"iimageBuffer":    {}, "iimage2DMS": {}, "iimage2DMSArray": {},
	"uimage1D": {}, "uimage2D": {}, "uimage3D": {},
	"uimageCube": {}, "uimage2DRect": {},
	"uimage1DArray": {}, "uimage2DArray": {},
	"uimageCubeArray": {},
	"uimageBuffer":    {}, "uimage2DMS": {}, "uimage2DMSArray": {},

	// Atomic counter types
	"atomic_uint": {},

	// Keywords
	"attribute": {}, "const": {}, "uniform": {}, "varying": {},
	"buffer": {}, "shared": {}, "coherent": {}, "volatile": {}, "restrict": {}, "readonly": {}, "writeonly": {},
	"layout": {}, "centroid": {}, "flat": {}, "smooth": {}, "noperspective": {},
	"patch": {}, "sample": {},
	"break": {}, "continue": {}, "do": {}, "for": {}, "while": {}, "switch": {}, "case": {}, "default": {},
	"if": {}, "else": {},
	"subroutine": {},
	"in":         {}, "out": {}, "inout": {},
	"true": {}, "false": {},
	"invariant": {}, "precise": {},
	"discard": {}, "return": {},
	"struct": {},

	// Precision qualifiers
	"lowp": {}, "mediump": {}, "highp": {}, "precision": {},

	// Reserved for future use
	"common": {}, "partition": {}, "active": {},
	"asm": {}, "class": {}, "union": {}, "enum": {}, "typedef": {}, "template": {}, "this": {},
	"resource": {},
	"goto":     {},
	"inline":   {}, "noinline": {}, "public": {}, "static": {}, "extern": {}, "external": {}, "interface": {},
	"long": {}, "short": {}, "half": {}, "fixed": {}, "unsigned": {}, "superp": {},
	"input": {}, "output": {},
	"hvec2": {}, "hvec3": {}, "hvec4": {}, "fvec2": {}, "fvec3": {}, "fvec4": {},
	"sampler3DRect": {},
	"filter":        {},
	"sizeof":        {}, "cast": {},
	"namespace": {}, "using": {},

	// Built-in variables (vertex)
	"gl_VertexID": {}, "gl_InstanceID": {},
	"gl_Position": {}, "gl_PointSize": {}, "gl_ClipDistance": {}, "gl_CullDistance": {},
	"gl_PerVertex": {},

	// Built-in variables (fragment)
	"gl_FragCoord": {}, "gl_FrontFacing": {}, "gl_PointCoord": {},
	"gl_SampleID": {}, "gl_SamplePosition": {}, "gl_SampleMaskIn": {},
	"gl_FragDepth": {}, "gl_SampleMask": {},
	"gl_Layer": {}, "gl_ViewportIndex": {},
	"gl_HelperInvocation": {},

	// Built-in variables (compute)
	"gl_NumWorkGroups": {}, "gl_WorkGroupSize": {}, "gl_WorkGroupID": {},
	"gl_LocalInvocationID": {}, "gl_GlobalInvocationID": {}, "gl_LocalInvocationIndex": {},

	// Built-in variables (tessellation)
	"gl_PatchVerticesIn": {}, "gl_PrimitiveID": {}, "gl_InvocationID": {},
	"gl_TessLevelOuter": {}, "gl_TessLevelInner": {}, "gl_TessCoord": {},

	// Built-in variables (geometry)
	"gl_PrimitiveIDIn": {},

	// Built-in constants
	"gl_MaxVertexAttribs": {}, "gl_MaxVertexUniformVectors": {},
	"gl_MaxVaryingVectors": {}, "gl_MaxVertexTextureImageUnits": {},
	"gl_MaxCombinedTextureImageUnits": {}, "gl_MaxTextureImageUnits": {},
	"gl_MaxFragmentUniformVectors": {}, "gl_MaxDrawBuffers": {},
	"gl_MaxClipDistances": {}, "gl_MaxCullDistances": {},
	"gl_MaxComputeWorkGroupCount": {}, "gl_MaxComputeWorkGroupSize": {},
	"gl_MaxComputeUniformComponents": {}, "gl_MaxComputeTextureImageUnits": {},
	"gl_MaxComputeImageUniforms": {}, "gl_MaxComputeAtomicCounters": {},
	"gl_MaxComputeAtomicCounterBuffers": {},

	// Built-in functions (commonly used as identifiers)
	"main":    {},
	"radians": {}, "degrees": {}, "sin": {}, "cos": {}, "tan": {},
	"asin": {}, "acos": {}, "atan": {}, "sinh": {}, "cosh": {}, "tanh": {},
	"asinh": {}, "acosh": {}, "atanh": {},
	"pow": {}, "exp": {}, "log": {}, "exp2": {}, "log2": {}, "sqrt": {}, "inversesqrt": {},
	"abs": {}, "sign": {}, "floor": {}, "trunc": {}, "round": {}, "roundEven": {}, "ceil": {}, "fract": {},
	"mod": {}, "modf": {}, "min": {}, "max": {}, "clamp": {}, "mix": {}, "step": {}, "smoothstep": {},
	"isnan": {}, "isinf": {},
	"floatBitsToInt": {}, "floatBitsToUint": {}, "intBitsToFloat": {}, "uintBitsToFloat": {},
	"fma":   {},
	"frexp": {}, "ldexp": {},
	"packUnorm2x16": {}, "packSnorm2x16": {}, "packUnorm4x8": {}, "packSnorm4x8": {},
	"unpackUnorm2x16": {}, "unpackSnorm2x16": {}, "unpackUnorm4x8": {}, "unpackSnorm4x8": {},
	"packHalf2x16": {}, "unpackHalf2x16": {},
	"packDouble2x32": {}, "unpackDouble2x32": {},
	"length": {}, "distance": {}, "dot": {}, "cross": {}, "normalize": {}, "faceforward": {}, "reflect": {}, "refract": {},
	"matrixCompMult": {}, "outerProduct": {}, "transpose": {}, "determinant": {}, "inverse": {},
	"lessThan": {}, "lessThanEqual": {}, "greaterThan": {}, "greaterThanEqual": {}, "equal": {}, "notEqual": {},
	"any": {}, "all": {}, "not": {},
	"uaddCarry": {}, "usubBorrow": {}, "umulExtended": {}, "imulExtended": {},
	"bitfieldExtract": {}, "bitfieldInsert": {}, "bitfieldReverse": {}, "bitCount": {}, "findLSB": {}, "findMSB": {},
	"textureSize": {}, "textureQueryLod": {}, "textureQueryLevels": {}, "textureSamples": {},
	"texture": {}, "textureProj": {}, "textureLod": {}, "textureOffset": {},
	"texelFetch": {}, "texelFetchOffset": {},
	"textureProjLod": {}, "textureProjOffset": {}, "textureLodOffset": {}, "textureProjLodOffset": {},
	"textureGrad": {}, "textureGradOffset": {}, "textureProjGrad": {}, "textureProjGradOffset": {},
	"textureGather": {}, "textureGatherOffset": {}, "textureGatherOffsets": {},
	"dFdx": {}, "dFdy": {}, "dFdxFine": {}, "dFdyFine": {}, "dFdxCoarse": {}, "dFdyCoarse": {},
	"fwidth": {}, "fwidthFine": {}, "fwidthCoarse": {},
	"interpolateAtCentroid": {}, "interpolateAtSample": {}, "interpolateAtOffset": {},
	"noise1": {}, "noise2": {}, "noise3": {}, "noise4": {},
	"EmitStreamVertex": {}, "EndStreamPrimitive": {}, "EmitVertex": {}, "EndPrimitive": {},
	"barrier": {}, "memoryBarrier": {}, "memoryBarrierAtomicCounter": {}, "memoryBarrierBuffer": {},
	"memoryBarrierShared": {}, "memoryBarrierImage": {}, "groupMemoryBarrier": {},
	"imageLoad": {}, "imageStore": {}, "imageAtomicAdd": {}, "imageAtomicMin": {}, "imageAtomicMax": {},
	"imageAtomicAnd": {}, "imageAtomicOr": {}, "imageAtomicXor": {}, "imageAtomicExchange": {},
	"imageAtomicCompSwap": {}, "imageSize": {}, "imageSamples": {},
	"atomicCounterIncrement": {}, "atomicCounterDecrement": {}, "atomicCounter": {},
	"atomicCounterAdd": {}, "atomicCounterSubtract": {}, "atomicCounterMin": {}, "atomicCounterMax": {},
	"atomicCounterAnd": {}, "atomicCounterOr": {}, "atomicCounterXor": {}, "atomicCounterExchange": {},
	"atomicCounterCompSwap": {},
	"atomicAdd":             {}, "atomicMin": {}, "atomicMax": {}, "atomicAnd": {}, "atomicOr": {}, "atomicXor": {},
	"atomicExchange": {}, "atomicCompSwap": {},
	"subpassLoad": {},
}

// isKeyword checks if a name is a GLSL keyword or reserved word.
func isKeyword(name string) bool {
	_, ok := glslKeywords[name]
	return ok
}

// escapeKeyword escapes a name if it conflicts with GLSL keywords.
// Returns the name with underscore prefix if it's reserved.
func escapeKeyword(name string) string {
	if name == "" {
		return "_unnamed"
	}
	if isKeyword(name) {
		return "_" + name
	}
	// Also escape names starting with "gl_" (reserved prefix)
	if len(name) >= 3 && name[:3] == "gl_" {
		return "_" + name
	}
	return name
}
