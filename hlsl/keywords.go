// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

// Package hlsl provides HLSL (High-Level Shader Language) code generation.
package hlsl

import "strings"

// UnnamedIdentifier is the default name for empty identifiers.
const UnnamedIdentifier = "_unnamed"

// Naga helper function names (reserved to avoid conflicts with generated code).
const (
	NagaModfFunction               = "_naga_modf"
	NagaFrexpFunction              = "_naga_frexp"
	NagaExtractBitsFunction        = "_naga_extract_bits"
	NagaInsertBitsFunction         = "_naga_insert_bits"
	SamplerHeapVar                 = "_naga_sampler_heap"
	ComparisonSamplerHeapVar       = "_naga_comparison_sampler_heap"
	SampleExternalTextureFunction  = "_naga_sample_external_texture"
	NagaAbsFunction                = "_naga_abs"
	NagaDivFunction                = "_naga_div"
	NagaModFunction                = "_naga_mod"
	NagaNegFunction                = "_naga_neg"
	NagaF2I32Function              = "_naga_f2i32"
	NagaF2U32Function              = "_naga_f2u32"
	NagaF2I64Function              = "_naga_f2i64"
	NagaF2U64Function              = "_naga_f2u64"
	ImageLoadExternalFunction      = "_naga_image_load_external"
	ImageSampleBaseClampToEdgeFunc = "_naga_image_sample_base_clamp_to_edge"
	DynamicBufferOffsetsPrefix     = "__dynamic_buffer_offsets"
	ImageStorageLoadScalarWrapper  = "_naga_image_storage_load_scalar"
)

// reservedKeywords contains all HLSL reserved keywords.
// This includes FXC keywords, DXC keywords, intrinsics, and type names.
// Based on Microsoft HLSL documentation and Rust naga implementation.
var reservedKeywords = map[string]struct{}{
	// =========================================================================
	// FXC Keywords (~125 keywords)
	// =========================================================================
	"AppendStructuredBuffer":  {},
	"asm":                     {},
	"asm_fragment":            {},
	"BlendState":              {},
	"bool":                    {},
	"break":                   {},
	"Buffer":                  {},
	"ByteAddressBuffer":       {},
	"case":                    {},
	"cbuffer":                 {},
	"centroid":                {},
	"class":                   {},
	"column_major":            {},
	"compile":                 {},
	"compile_fragment":        {},
	"CompileShader":           {},
	"const":                   {},
	"continue":                {},
	"ComputeShader":           {},
	"ConsumeStructuredBuffer": {},
	"default":                 {},
	"DepthStencilState":       {},
	"DepthStencilView":        {},
	"discard":                 {},
	"do":                      {},
	"double":                  {},
	"DomainShader":            {},
	"dword":                   {},
	"else":                    {},
	"export":                  {},
	"extern":                  {},
	"false":                   {},
	"float":                   {},
	"for":                     {},
	"fxgroup":                 {},
	"GeometryShader":          {},
	"groupshared":             {},
	"half":                    {},
	"Hullshader":              {},
	"if":                      {},
	"in":                      {},
	"inline":                  {},
	"inout":                   {},
	"InputPatch":              {},
	"int":                     {},
	"interface":               {},
	"line":                    {},
	"lineadj":                 {},
	"linear":                  {},
	"LineStream":              {},
	"matrix":                  {},
	"min10float":              {},
	"min12int":                {},
	"min16float":              {},
	"min16int":                {},
	"min16uint":               {},
	"namespace":               {},
	"nointerpolation":         {},
	"noperspective":           {},
	"NULL":                    {},
	"out":                     {},
	"OutputPatch":             {},
	"packoffset":              {},
	"pass":                    {},
	"pixelfragment":           {},
	"PixelShader":             {},
	"point":                   {},
	"PointStream":             {},
	"precise":                 {},
	"RasterizerState":         {},
	"RenderTargetView":        {},
	"return":                  {},
	"register":                {},
	"row_major":               {},
	"RWBuffer":                {},
	"RWByteAddressBuffer":     {},
	"RWStructuredBuffer":      {},
	"RWTexture1D":             {},
	"RWTexture1DArray":        {},
	"RWTexture2D":             {},
	"RWTexture2DArray":        {},
	"RWTexture3D":             {},
	"sample":                  {},
	"sampler":                 {},
	"SamplerState":            {},
	"SamplerComparisonState":  {},
	"shared":                  {},
	"snorm":                   {},
	"stateblock":              {},
	"stateblock_state":        {},
	"static":                  {},
	"string":                  {},
	"struct":                  {},
	"switch":                  {},
	"StructuredBuffer":        {},
	"tbuffer":                 {},
	"technique":               {},
	"technique10":             {},
	"technique11":             {},
	"texture":                 {},
	"Texture1D":               {},
	"Texture1DArray":          {},
	"Texture2D":               {},
	"Texture2DArray":          {},
	"Texture2DMS":             {},
	"Texture2DMSArray":        {},
	"Texture3D":               {},
	"TextureCube":             {},
	"TextureCubeArray":        {},
	"true":                    {},
	"typedef":                 {},
	"triangle":                {},
	"triangleadj":             {},
	"TriangleStream":          {},
	"uint":                    {},
	"uniform":                 {},
	"unorm":                   {},
	"unsigned":                {},
	"vector":                  {},
	"vertexfragment":          {},
	"VertexShader":            {},
	"void":                    {},
	"volatile":                {},
	"while":                   {},

	// =========================================================================
	// FXC Reserved Words (~35 words)
	// =========================================================================
	"auto":             {},
	"catch":            {},
	"char":             {},
	"const_cast":       {},
	"delete":           {},
	"dynamic_cast":     {},
	"enum":             {},
	"explicit":         {},
	"friend":           {},
	"goto":             {},
	"long":             {},
	"mutable":          {},
	"new":              {},
	"operator":         {},
	"private":          {},
	"protected":        {},
	"public":           {},
	"reinterpret_cast": {},
	"short":            {},
	"signed":           {},
	"sizeof":           {},
	"static_cast":      {},
	"template":         {},
	"this":             {},
	"throw":            {},
	"try":              {},
	"typename":         {},
	"union":            {},
	"using":            {},
	"virtual":          {},

	// =========================================================================
	// FXC Intrinsics (~135 intrinsics)
	// =========================================================================
	"abort":                            {},
	"abs":                              {},
	"acos":                             {},
	"all":                              {},
	"AllMemoryBarrier":                 {},
	"AllMemoryBarrierWithGroupSync":    {},
	"any":                              {},
	"asdouble":                         {},
	"asfloat":                          {},
	"asin":                             {},
	"asint":                            {},
	"asuint":                           {},
	"atan":                             {},
	"atan2":                            {},
	"ceil":                             {},
	"CheckAccessFullyMapped":           {},
	"clamp":                            {},
	"clip":                             {},
	"cos":                              {},
	"cosh":                             {},
	"countbits":                        {},
	"cross":                            {},
	"D3DCOLORtoUBYTE4":                 {},
	"ddx":                              {},
	"ddx_coarse":                       {},
	"ddx_fine":                         {},
	"ddy":                              {},
	"ddy_coarse":                       {},
	"ddy_fine":                         {},
	"degrees":                          {},
	"determinant":                      {},
	"DeviceMemoryBarrier":              {},
	"DeviceMemoryBarrierWithGroupSync": {},
	"distance":                         {},
	"dot":                              {},
	"dst":                              {},
	"errorf":                           {},
	"EvaluateAttributeAtSample":        {},
	"EvaluateAttributeCentroid":        {},
	"EvaluateAttributeSnapped":         {},
	"exp":                              {},
	"exp2":                             {},
	"f16tof32":                         {},
	"f32tof16":                         {},
	"faceforward":                      {},
	"firstbithigh":                     {},
	"firstbitlow":                      {},
	"floor":                            {},
	"fma":                              {},
	"fmod":                             {},
	"frac":                             {},
	"frexp":                            {},
	"fwidth":                           {},
	"GetRenderTargetSampleCount":       {},
	"GetRenderTargetSamplePosition":    {},
	"GroupMemoryBarrier":               {},
	"GroupMemoryBarrierWithGroupSync":  {},
	"InterlockedAdd":                   {},
	"InterlockedAnd":                   {},
	"InterlockedCompareExchange":       {},
	"InterlockedCompareStore":          {},
	"InterlockedExchange":              {},
	"InterlockedMax":                   {},
	"InterlockedMin":                   {},
	"InterlockedOr":                    {},
	"InterlockedXor":                   {},
	"isfinite":                         {},
	"isinf":                            {},
	"isnan":                            {},
	"ldexp":                            {},
	"length":                           {},
	"lerp":                             {},
	"lit":                              {},
	"log":                              {},
	"log10":                            {},
	"log2":                             {},
	"mad":                              {},
	"max":                              {},
	"min":                              {},
	"modf":                             {},
	"msad4":                            {},
	"mul":                              {},
	"noise":                            {},
	"normalize":                        {},
	"pow":                              {},
	"printf":                           {},
	"Process2DQuadTessFactorsAvg":      {},
	"Process2DQuadTessFactorsMax":      {},
	"Process2DQuadTessFactorsMin":      {},
	"ProcessIsolineTessFactors":        {},
	"ProcessQuadTessFactorsAvg":        {},
	"ProcessQuadTessFactorsMax":        {},
	"ProcessQuadTessFactorsMin":        {},
	"ProcessTriTessFactorsAvg":         {},
	"ProcessTriTessFactorsMax":         {},
	"ProcessTriTessFactorsMin":         {},
	"radians":                          {},
	"rcp":                              {},
	"reflect":                          {},
	"refract":                          {},
	"reversebits":                      {},
	"round":                            {},
	"rsqrt":                            {},
	"saturate":                         {},
	"sign":                             {},
	"sin":                              {},
	"sincos":                           {},
	"sinh":                             {},
	"smoothstep":                       {},
	"sqrt":                             {},
	"step":                             {},
	"tan":                              {},
	"tanh":                             {},
	"tex1D":                            {},
	"tex1Dbias":                        {},
	"tex1Dgrad":                        {},
	"tex1Dlod":                         {},
	"tex1Dproj":                        {},
	"tex2D":                            {},
	"tex2Dbias":                        {},
	"tex2Dgrad":                        {},
	"tex2Dlod":                         {},
	"tex2Dproj":                        {},
	"tex3D":                            {},
	"tex3Dbias":                        {},
	"tex3Dgrad":                        {},
	"tex3Dlod":                         {},
	"tex3Dproj":                        {},
	"texCUBE":                          {},
	"texCUBEbias":                      {},
	"texCUBEgrad":                      {},
	"texCUBElod":                       {},
	"texCUBEproj":                      {},
	"transpose":                        {},
	"trunc":                            {},

	// =========================================================================
	// DXC Keywords (~230 keywords - C11 and compiler-specific)
	// =========================================================================
	"_Alignas":                       {},
	"_Alignof":                       {},
	"_Atomic":                        {},
	"_Bool":                          {},
	"_Complex":                       {},
	"_Generic":                       {},
	"_Imaginary":                     {},
	"_Noreturn":                      {},
	"_Static_assert":                 {},
	"_Thread_local":                  {},
	"__func__":                       {},
	"__objc_yes":                     {},
	"__objc_no":                      {},
	"wchar_t":                        {},
	"_Decimal32":                     {},
	"_Decimal64":                     {},
	"_Decimal128":                    {},
	"__null":                         {},
	"__alignof":                      {},
	"__attribute":                    {},
	"__builtin_choose_expr":          {},
	"__builtin_offsetof":             {},
	"__builtin_va_arg":               {},
	"__extension__":                  {},
	"__imag":                         {},
	"__int128":                       {},
	"__label__":                      {},
	"__real":                         {},
	"__thread":                       {},
	"__FUNCTION__":                   {},
	"__PRETTY_FUNCTION__":            {},
	"__auto_type":                    {},
	"typeof":                         {},
	"__FUNCDNAME__":                  {},
	"__FUNCSIG__":                    {},
	"L__FUNCTION__":                  {},
	"__is_interface_class":           {},
	"__is_sealed":                    {},
	"__is_destructible":              {},
	"__is_trivially_destructible":    {},
	"__is_nothrow_destructible":      {},
	"__is_nothrow_assignable":        {},
	"__is_constructible":             {},
	"__is_nothrow_constructible":     {},
	"__is_assignable":                {},
	"__has_nothrow_move_assign":      {},
	"__has_trivial_move_assign":      {},
	"__has_trivial_move_constructor": {},
	"__has_nothrow_move_constructor": {},
	"__is_trivially_constructible":   {},
	"__is_trivially_assignable":      {},
	"__is_trivially_copyable":        {},
	"__underlying_type":              {},
	"__is_final":                     {},
	"__is_aggregate":                 {},
	"__declspec":                     {},
	"__cdecl":                        {},
	"__clrcall":                      {},
	"__stdcall":                      {},
	"__fastcall":                     {},
	"__thiscall":                     {},
	"__vectorcall":                   {},
	"__forceinline":                  {},
	"__unaligned":                    {},
	"__super":                        {},
	"__int8":                         {},
	"__int16":                        {},
	"__int32":                        {},
	"__int64":                        {},
	"__if_exists":                    {},
	"__if_not_exists":                {},
	"__single_inheritance":           {},
	"__multiple_inheritance":         {},
	"__virtual_inheritance":          {},
	"__uuidof":                       {},
	"__w64":                          {},
	"__m64":                          {},
	"__m128":                         {},
	"__m128i":                        {},
	"__m128d":                        {},
	"__ptr64":                        {},
	"__ptr32":                        {},
	"__sptr":                         {},
	"__uptr":                         {},
	"__noop":                         {},
	"__assume":                       {},
	"__identifier":                   {},
	"__restrict":                     {},
	"__inline":                       {},
	"__asm":                          {},
	"_asm":                           {},
	"__asm__":                        {},
	"__based":                        {},
	"__except":                       {},
	"__event":                        {},
	"__hook":                         {},
	"__unhook":                       {},
	"__raise":                        {},
	"__try":                          {},
	"__finally":                      {},
	"__leave":                        {},
	"typeid":                         {},
	"__abstract":                     {},
	"__box":                          {},
	"__delegate":                     {},
	"__gc":                           {},
	"__nogc":                         {},
	"__pin":                          {},
	"__property":                     {},
	"__sealed":                       {},
	"__try_cast":                     {},
	"__typeof":                       {},
	"__value":                        {},
	"__interface":                    {},
	"__wchar_t":                      {},
	"nullptr":                        {},
	"constexpr":                      {},
	"decltype":                       {},
	"noexcept":                       {},
	"static_assert":                  {},
	"thread_local":                   {},
	"alignas":                        {},
	"alignof":                        {},
	"char16_t":                       {},
	"char32_t":                       {},
	"co_await":                       {},
	"co_return":                      {},
	"co_yield":                       {},
	"concept":                        {},
	"requires":                       {},
	"char8_t":                        {},
	"consteval":                      {},
	"constinit":                      {},

	// =========================================================================
	// DXC Intrinsics - Wave Operations
	// =========================================================================
	"WaveIsFirstLane":          {},
	"WaveGetLaneIndex":         {},
	"WaveGetLaneCount":         {},
	"WaveActiveAnyTrue":        {},
	"WaveActiveAllTrue":        {},
	"WaveActiveAllEqual":       {},
	"WaveActiveBallot":         {},
	"WaveReadLaneAt":           {},
	"WaveReadLaneFirst":        {},
	"WaveActiveCountBits":      {},
	"WaveActiveSum":            {},
	"WaveActiveProduct":        {},
	"WaveActiveBitAnd":         {},
	"WaveActiveBitOr":          {},
	"WaveActiveBitXor":         {},
	"WaveActiveMin":            {},
	"WaveActiveMax":            {},
	"WavePrefixCountBits":      {},
	"WavePrefixSum":            {},
	"WavePrefixProduct":        {},
	"WaveMatch":                {},
	"WaveMultiPrefixBitAnd":    {},
	"WaveMultiPrefixBitOr":     {},
	"WaveMultiPrefixBitXor":    {},
	"WaveMultiPrefixCountBits": {},
	"WaveMultiPrefixProduct":   {},
	"WaveMultiPrefixSum":       {},
	"QuadReadLaneAt":           {},
	"QuadReadAcrossX":          {},
	"QuadReadAcrossY":          {},
	"QuadReadAcrossDiagonal":   {},
	"QuadAny":                  {},
	"QuadAll":                  {},

	// =========================================================================
	// DXC Intrinsics - Ray Tracing
	// =========================================================================
	"TraceRay":               {},
	"ReportHit":              {},
	"CallShader":             {},
	"IgnoreHit":              {},
	"AcceptHitAndEndSearch":  {},
	"DispatchRaysIndex":      {},
	"DispatchRaysDimensions": {},
	"WorldRayOrigin":         {},
	"WorldRayDirection":      {},
	"ObjectRayOrigin":        {},
	"ObjectRayDirection":     {},
	"RayTMin":                {},
	"RayTCurrent":            {},
	"PrimitiveIndex":         {},
	"InstanceID":             {},
	"InstanceIndex":          {},
	"GeometryIndex":          {},
	"HitKind":                {},
	"RayFlags":               {},
	"ObjectToWorld":          {},
	"ObjectToWorld3x4":       {},
	"ObjectToWorld4x3":       {},
	"WorldToObject":          {},
	"WorldToObject3x4":       {},
	"WorldToObject4x3":       {},

	// =========================================================================
	// DXC Intrinsics - Mesh Shaders
	// =========================================================================
	"SetMeshOutputCounts":    {},
	"DispatchMesh":           {},
	"IsHelperLane":           {},
	"AllocateRayQuery":       {},
	"CreateResourceFromHeap": {},

	// =========================================================================
	// DXC Resource Types (~50 types)
	// =========================================================================
	"RWTexture2DMS":                      {},
	"RWTexture2DMSArray":                 {},
	"RWTextureCube":                      {},
	"RWTextureCubeArray":                 {},
	"FeedbackTexture2D":                  {},
	"FeedbackTexture2DArray":             {},
	"RasterizerOrderedTexture1D":         {},
	"RasterizerOrderedTexture2D":         {},
	"RasterizerOrderedTexture3D":         {},
	"RasterizerOrderedTexture1DArray":    {},
	"RasterizerOrderedTexture2DArray":    {},
	"RasterizerOrderedBuffer":            {},
	"RasterizerOrderedByteAddressBuffer": {},
	"RasterizerOrderedStructuredBuffer":  {},
	"ConstantBuffer":                     {},
	"TextureBuffer":                      {},
	"RaytracingAccelerationStructure":    {},
	"RayQuery":                           {},
	"RayDesc":                            {},

	// =========================================================================
	// Additional DXC Keywords
	// =========================================================================
	"globallycoherent": {},
	"indices":          {},
	"vertices":         {},
	"primitives":       {},
	"payload":          {},
	"attributes":       {},

	// =========================================================================
	// Semantic Names (reserved)
	// =========================================================================
	"SV_Position":               {},
	"SV_Target":                 {},
	"SV_Depth":                  {},
	"SV_VertexID":               {},
	"SV_InstanceID":             {},
	"SV_PrimitiveID":            {},
	"SV_IsFrontFace":            {},
	"SV_SampleIndex":            {},
	"SV_Coverage":               {},
	"SV_ClipDistance":           {},
	"SV_CullDistance":           {},
	"SV_DispatchThreadID":       {},
	"SV_GroupID":                {},
	"SV_GroupIndex":             {},
	"SV_GroupThreadID":          {},
	"SV_GSInstanceID":           {},
	"SV_InsideTessFactor":       {},
	"SV_OutputControlPointID":   {},
	"SV_RenderTargetArrayIndex": {},
	"SV_TessFactor":             {},
	"SV_ViewportArrayIndex":     {},
	"SV_StencilRef":             {},
	"SV_Barycentrics":           {},
	"SV_ShadingRate":            {},
	"SV_CullPrimitive":          {},

	// =========================================================================
	// Naga Helper Names (reserved to avoid conflicts)
	// =========================================================================
	"_naga_modf":                            {},
	"_naga_frexp":                           {},
	"_naga_extract_bits":                    {},
	"_naga_insert_bits":                     {},
	"_naga_sampler_heap":                    {},
	"_naga_comparison_sampler_heap":         {},
	"_naga_sample_external_texture":         {},
	"_naga_abs":                             {},
	"_naga_div":                             {},
	"_naga_mod":                             {},
	"_naga_neg":                             {},
	"_naga_f2i32":                           {},
	"_naga_f2u32":                           {},
	"_naga_f2i64":                           {},
	"_naga_f2u64":                           {},
	"_naga_image_load_external":             {},
	"_naga_image_sample_base_clamp_to_edge": {},
	"__dynamic_buffer_offsets":              {},
	"_naga_image_storage_load_scalar":       {},
}

// caseInsensitiveKeywords contains keywords that are case-insensitive in HLSL.
// These need special handling to avoid conflicts.
var caseInsensitiveKeywords = map[string]struct{}{
	"asm":         {},
	"decl":        {},
	"pass":        {},
	"technique":   {},
	"texture1d":   {},
	"texture2d":   {},
	"texture3d":   {},
	"texturecube": {},
}

// typeShorthands contains all scalar, vector, and matrix type shorthands.
// Generated programmatically from base types.
var typeShorthands = func() map[string]struct{} {
	result := make(map[string]struct{})

	// Base scalar types
	bases := []string{
		"bool", "int", "uint", "dword", "half", "float", "double",
		"min10float", "min16float", "min12int", "min16int", "min16uint",
		"int16_t", "int32_t", "int64_t", "uint16_t", "uint32_t", "uint64_t",
		"float16_t", "float32_t", "float64_t", "int8_t4_packed", "uint8_t4_packed",
	}

	// Add scalar types
	for _, base := range bases {
		result[base] = struct{}{}
	}

	// Vector-supporting types (subset of bases)
	vectorBases := []string{
		"bool", "int", "uint", "dword", "half", "float", "double",
		"min10float", "min16float", "min12int", "min16int", "min16uint",
		"int16_t", "int32_t", "int64_t", "uint16_t", "uint32_t", "uint64_t",
		"float16_t", "float32_t", "float64_t",
	}

	// Generate vector types: base1, base2, base3, base4
	for _, base := range vectorBases {
		for i := 1; i <= 4; i++ {
			result[base+string(rune('0'+i))] = struct{}{}
		}
	}

	// Matrix-supporting types
	matrixBases := []string{
		"bool", "int", "uint", "half", "float", "double",
		"min10float", "min16float", "min12int", "min16int", "min16uint",
		"float16_t", "float32_t", "float64_t",
	}

	// Generate matrix types: baseRxC where R,C in {1,2,3,4}
	for _, base := range matrixBases {
		for r := 1; r <= 4; r++ {
			for c := 1; c <= 4; c++ {
				result[base+string(rune('0'+r))+"x"+string(rune('0'+c))] = struct{}{}
			}
		}
	}

	return result
}()

// IsReserved checks if a name is an HLSL reserved keyword.
func IsReserved(name string) bool {
	if _, ok := reservedKeywords[name]; ok {
		return true
	}
	if _, ok := typeShorthands[name]; ok {
		return true
	}
	return false
}

// IsCaseInsensitiveReserved checks if a name conflicts with case-insensitive keywords.
// HLSL has some keywords that are case-insensitive (legacy behavior).
func IsCaseInsensitiveReserved(name string) bool {
	lower := strings.ToLower(name)
	_, ok := caseInsensitiveKeywords[lower]
	return ok
}

// Escape returns a safe identifier name.
// If the name is reserved or empty, it's prefixed with underscore.
func Escape(name string) string {
	if name == "" {
		return UnnamedIdentifier
	}
	if IsReserved(name) || IsCaseInsensitiveReserved(name) {
		return "_" + name
	}
	return name
}
