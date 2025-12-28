// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl

import "testing"

func TestIsReserved(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// FXC keywords
		{"fxc_keyword_bool", "bool", true},
		{"fxc_keyword_float", "float", true},
		{"fxc_keyword_struct", "struct", true},
		{"fxc_keyword_cbuffer", "cbuffer", true},
		{"fxc_keyword_texture2d", "Texture2D", true},
		{"fxc_keyword_buffer", "Buffer", true},

		// FXC reserved words
		{"fxc_reserved_auto", "auto", true},
		{"fxc_reserved_class", "class", true},
		{"fxc_reserved_delete", "delete", true},

		// FXC intrinsics
		{"fxc_intrinsic_abs", "abs", true},
		{"fxc_intrinsic_sin", "sin", true},
		{"fxc_intrinsic_cos", "cos", true},
		{"fxc_intrinsic_dot", "dot", true},
		{"fxc_intrinsic_cross", "cross", true},
		{"fxc_intrinsic_lerp", "lerp", true},
		{"fxc_intrinsic_saturate", "saturate", true},

		// DXC keywords
		{"dxc_keyword_constexpr", "constexpr", true},
		{"dxc_keyword_nullptr", "nullptr", true},
		{"dxc_keyword_alignas", "alignas", true},

		// DXC wave operations
		{"dxc_wave_isFirstLane", "WaveIsFirstLane", true},
		{"dxc_wave_getIndex", "WaveGetLaneIndex", true},
		{"dxc_wave_activeSum", "WaveActiveSum", true},

		// DXC ray tracing
		{"dxc_ray_traceRay", "TraceRay", true},
		{"dxc_ray_reportHit", "ReportHit", true},
		{"dxc_ray_worldRayOrigin", "WorldRayOrigin", true},

		// DXC mesh shaders
		{"dxc_mesh_setOutputCounts", "SetMeshOutputCounts", true},
		{"dxc_mesh_dispatchMesh", "DispatchMesh", true},

		// DXC resource types
		{"dxc_resource_rwTexture2dms", "RWTexture2DMS", true},
		{"dxc_resource_feedbackTex", "FeedbackTexture2D", true},
		{"dxc_resource_rayQuery", "RayQuery", true},

		// Semantic names
		{"semantic_position", "SV_Position", true},
		{"semantic_target", "SV_Target", true},
		{"semantic_dispatchThread", "SV_DispatchThreadID", true},

		// Naga helper names
		{"naga_modf", "_naga_modf", true},
		{"naga_div", "_naga_div", true},
		{"naga_sampler_heap", "_naga_sampler_heap", true},

		// Type shorthands - scalars
		{"type_bool", "bool", true},
		{"type_int", "int", true},
		{"type_uint", "uint", true},
		{"type_float", "float", true},
		{"type_double", "double", true},
		{"type_half", "half", true},

		// Type shorthands - vectors
		{"type_float2", "float2", true},
		{"type_float3", "float3", true},
		{"type_float4", "float4", true},
		{"type_int4", "int4", true},
		{"type_uint3", "uint3", true},
		{"type_bool2", "bool2", true},

		// Type shorthands - matrices
		{"type_float4x4", "float4x4", true},
		{"type_float3x3", "float3x3", true},
		{"type_float2x2", "float2x2", true},
		{"type_int4x4", "int4x4", true},
		{"type_half3x4", "half3x4", true},

		// Non-reserved names
		{"non_reserved_myVar", "myVar", false},
		{"non_reserved_custom", "customFunction", false},
		{"non_reserved_position", "position", false},
		{"non_reserved_color", "color", false},
		{"non_reserved_normal", "normal", false},
		{"non_reserved_underscore", "_myPrivateVar", false},
		{"non_reserved_camelCase", "myShaderVariable", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsReserved(tt.input)
			if got != tt.expected {
				t.Errorf("IsReserved(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsCaseInsensitiveReserved(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Case-insensitive keywords in various cases
		{"asm_lower", "asm", true},
		{"asm_upper", "ASM", true},
		{"asm_mixed", "Asm", true},
		{"decl_lower", "decl", true},
		{"decl_upper", "DECL", true},
		{"pass_lower", "pass", true},
		{"pass_upper", "PASS", true},
		{"technique_lower", "technique", true},
		{"technique_mixed", "Technique", true},
		{"texture1d_lower", "texture1d", true},
		{"texture1d_upper", "TEXTURE1D", true},
		{"texture2d_lower", "texture2d", true},
		{"texture2d_mixed", "Texture2D", true},
		{"texture3d_lower", "texture3d", true},
		{"texturecube_lower", "texturecube", true},
		{"texturecube_mixed", "TextureCube", true},

		// Non-case-insensitive keywords
		{"float_not_case_insensitive", "float", false},
		{"struct_not_case_insensitive", "struct", false},
		{"buffer_not_case_insensitive", "buffer", false},
		{"random_word", "myVariable", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsCaseInsensitiveReserved(tt.input)
			if got != tt.expected {
				t.Errorf("IsCaseInsensitiveReserved(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestEscape(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Empty string
		{"empty", "", UnnamedIdentifier},

		// Reserved keywords get prefixed
		{"escape_float", "float", "_float"},
		{"escape_int", "int", "_int"},
		{"escape_struct", "struct", "_struct"},
		{"escape_cbuffer", "cbuffer", "_cbuffer"},
		{"escape_bool", "bool", "_bool"},
		{"escape_void", "void", "_void"},

		// Intrinsics get prefixed
		{"escape_abs", "abs", "_abs"},
		{"escape_sin", "sin", "_sin"},
		{"escape_cos", "cos", "_cos"},
		{"escape_dot", "dot", "_dot"},

		// Type shorthands get prefixed
		{"escape_float4", "float4", "_float4"},
		{"escape_int3", "int3", "_int3"},
		{"escape_float4x4", "float4x4", "_float4x4"},

		// Case-insensitive keywords get prefixed
		{"escape_asm_lower", "asm", "_asm"},
		{"escape_ASM_upper", "ASM", "_ASM"},
		{"escape_Technique", "Technique", "_Technique"},

		// Non-reserved names pass through
		{"pass_myVar", "myVar", "myVar"},
		{"pass_custom", "customFunction", "customFunction"},
		{"pass_position", "position", "position"},
		{"pass_color", "color", "color"},
		{"pass_underscore", "_myPrivate", "_myPrivate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Escape(tt.input)
			if got != tt.expected {
				t.Errorf("Escape(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestTypeShorthandsGeneration(t *testing.T) {
	// Test that type shorthands are properly generated

	// Vector types for common bases
	vectorTests := []struct {
		base   string
		suffix int
	}{
		{"float", 1}, {"float", 2}, {"float", 3}, {"float", 4},
		{"int", 1}, {"int", 2}, {"int", 3}, {"int", 4},
		{"uint", 1}, {"uint", 2}, {"uint", 3}, {"uint", 4},
		{"bool", 1}, {"bool", 2}, {"bool", 3}, {"bool", 4},
		{"half", 1}, {"half", 2}, {"half", 3}, {"half", 4},
		{"double", 1}, {"double", 2}, {"double", 3}, {"double", 4},
	}

	for _, tt := range vectorTests {
		name := tt.base + string(rune('0'+tt.suffix))
		t.Run("vector_"+name, func(t *testing.T) {
			if !IsReserved(name) {
				t.Errorf("Expected %q to be reserved", name)
			}
		})
	}

	// Matrix types for common bases
	matrixTests := []struct {
		base string
		rows int
		cols int
	}{
		{"float", 2, 2}, {"float", 3, 3}, {"float", 4, 4},
		{"float", 2, 3}, {"float", 3, 4}, {"float", 4, 3},
		{"int", 2, 2}, {"int", 4, 4},
		{"half", 2, 2}, {"half", 4, 4},
	}

	for _, tt := range matrixTests {
		name := tt.base + string(rune('0'+tt.rows)) + "x" + string(rune('0'+tt.cols))
		t.Run("matrix_"+name, func(t *testing.T) {
			if !IsReserved(name) {
				t.Errorf("Expected %q to be reserved", name)
			}
		})
	}
}

func TestNagaHelperConstants(t *testing.T) {
	// Verify that Naga helper constants have expected values
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"NagaModfFunction", NagaModfFunction, "_naga_modf"},
		{"NagaFrexpFunction", NagaFrexpFunction, "_naga_frexp"},
		{"NagaExtractBitsFunction", NagaExtractBitsFunction, "_naga_extract_bits"},
		{"NagaInsertBitsFunction", NagaInsertBitsFunction, "_naga_insert_bits"},
		{"SamplerHeapVar", SamplerHeapVar, "_naga_sampler_heap"},
		{"ComparisonSamplerHeapVar", ComparisonSamplerHeapVar, "_naga_comparison_sampler_heap"},
		{"SampleExternalTextureFunction", SampleExternalTextureFunction, "_naga_sample_external_texture"},
		{"NagaAbsFunction", NagaAbsFunction, "_naga_abs"},
		{"NagaDivFunction", NagaDivFunction, "_naga_div"},
		{"NagaModFunction", NagaModFunction, "_naga_mod"},
		{"NagaNegFunction", NagaNegFunction, "_naga_neg"},
		{"NagaF2I32Function", NagaF2I32Function, "_naga_f2i32"},
		{"NagaF2U32Function", NagaF2U32Function, "_naga_f2u32"},
		{"NagaF2I64Function", NagaF2I64Function, "_naga_f2i64"},
		{"NagaF2U64Function", NagaF2U64Function, "_naga_f2u64"},
		{"ImageLoadExternalFunction", ImageLoadExternalFunction, "_naga_image_load_external"},
		{"ImageSampleBaseClampToEdgeFunc", ImageSampleBaseClampToEdgeFunc, "_naga_image_sample_base_clamp_to_edge"},
		{"DynamicBufferOffsetsPrefix", DynamicBufferOffsetsPrefix, "__dynamic_buffer_offsets"},
		{"ImageStorageLoadScalarWrapper", ImageStorageLoadScalarWrapper, "_naga_image_storage_load_scalar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.constant, tt.expected)
			}
		})
	}
}

func TestReservedKeywordsCoverage(t *testing.T) {
	// Test that the keyword map contains expected categories

	// FXC Keywords samples
	fxcKeywords := []string{
		"AppendStructuredBuffer", "BlendState", "Buffer", "ByteAddressBuffer",
		"cbuffer", "centroid", "column_major", "compile", "ConsumeStructuredBuffer",
		"DepthStencilState", "discard", "DomainShader", "GeometryShader",
		"groupshared", "Hullshader", "InputPatch", "interface", "lineadj",
		"LineStream", "matrix", "nointerpolation", "noperspective",
		"OutputPatch", "packoffset", "pixelfragment", "PixelShader",
		"PointStream", "precise", "RasterizerState", "RenderTargetView",
		"register", "row_major", "RWBuffer", "RWByteAddressBuffer",
		"RWStructuredBuffer", "SamplerState", "SamplerComparisonState",
		"shared", "snorm", "stateblock", "StructuredBuffer", "tbuffer",
		"technique10", "technique11", "texture", "Texture1D", "Texture1DArray",
		"Texture2D", "Texture2DArray", "Texture2DMS", "Texture2DMSArray",
		"Texture3D", "TextureCube", "TextureCubeArray", "triangle",
		"triangleadj", "TriangleStream", "uniform", "unorm", "vector",
		"vertexfragment", "VertexShader", "volatile",
	}

	for _, kw := range fxcKeywords {
		t.Run("fxc_"+kw, func(t *testing.T) {
			if _, ok := reservedKeywords[kw]; !ok {
				t.Errorf("FXC keyword %q not found in reservedKeywords", kw)
			}
		})
	}

	// DXC Wave operations
	waveOps := []string{
		"WaveIsFirstLane", "WaveGetLaneIndex", "WaveGetLaneCount",
		"WaveActiveAnyTrue", "WaveActiveAllTrue", "WaveActiveAllEqual",
		"WaveActiveBallot", "WaveReadLaneAt", "WaveReadLaneFirst",
		"WaveActiveCountBits", "WaveActiveSum", "WaveActiveProduct",
		"WaveActiveBitAnd", "WaveActiveBitOr", "WaveActiveBitXor",
		"WaveActiveMin", "WaveActiveMax", "WavePrefixCountBits",
		"WavePrefixSum", "WavePrefixProduct", "WaveMatch",
		"WaveMultiPrefixBitAnd", "WaveMultiPrefixBitOr",
		"WaveMultiPrefixBitXor", "WaveMultiPrefixCountBits",
		"WaveMultiPrefixProduct", "WaveMultiPrefixSum",
		"QuadReadLaneAt", "QuadReadAcrossX", "QuadReadAcrossY",
		"QuadReadAcrossDiagonal", "QuadAny", "QuadAll",
	}

	for _, op := range waveOps {
		t.Run("wave_"+op, func(t *testing.T) {
			if _, ok := reservedKeywords[op]; !ok {
				t.Errorf("Wave operation %q not found in reservedKeywords", op)
			}
		})
	}

	// Ray tracing intrinsics
	rayOps := []string{
		"TraceRay", "ReportHit", "CallShader", "IgnoreHit",
		"AcceptHitAndEndSearch", "DispatchRaysIndex", "DispatchRaysDimensions",
		"WorldRayOrigin", "WorldRayDirection", "ObjectRayOrigin",
		"ObjectRayDirection", "RayTMin", "RayTCurrent", "PrimitiveIndex",
		"InstanceID", "InstanceIndex", "GeometryIndex", "HitKind",
		"RayFlags", "ObjectToWorld", "WorldToObject",
	}

	for _, op := range rayOps {
		t.Run("ray_"+op, func(t *testing.T) {
			if _, ok := reservedKeywords[op]; !ok {
				t.Errorf("Ray tracing operation %q not found in reservedKeywords", op)
			}
		})
	}
}

func BenchmarkIsReserved(b *testing.B) {
	testCases := []string{
		"float",         // Reserved keyword
		"float4",        // Type shorthand
		"myVariable",    // Not reserved
		"WaveActiveSum", // DXC intrinsic
		"_naga_modf",    // Naga helper
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range testCases {
			_ = IsReserved(tc)
		}
	}
}

func BenchmarkIsCaseInsensitiveReserved(b *testing.B) {
	testCases := []string{
		"asm",
		"ASM",
		"Texture2D",
		"TECHNIQUE",
		"myVariable",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range testCases {
			_ = IsCaseInsensitiveReserved(tc)
		}
	}
}

func BenchmarkEscape(b *testing.B) {
	testCases := []string{
		"",         // Empty
		"float",    // Reserved
		"myVar",    // Not reserved
		"ASM",      // Case-insensitive
		"float4x4", // Type shorthand
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range testCases {
			_ = Escape(tc)
		}
	}
}
