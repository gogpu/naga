// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"strings"

	"github.com/gogpu/naga/internal/backend"
)

// reservedKeywords is an alias to the authoritative keyword map in the
// backend package. Using a package-level alias keeps existing code
// (namer, Escape, IsReserved) unchanged while the data lives in one place.
var reservedKeywords = backend.HLSLReservedKeywords

// caseInsensitiveKeywords is an alias to the authoritative map in backend.
var caseInsensitiveKeywords = backend.HLSLCaseInsensitiveKeywords

// UnnamedIdentifier is the default name for empty identifiers.
const UnnamedIdentifier = "_unnamed"

// Naga helper function names (reserved to avoid conflicts with generated code).
const (
	NagaModfFunction               = "naga_modf"
	NagaFrexpFunction              = "naga_frexp"
	NagaExtractBitsFunction        = "naga_extractBits"
	NagaInsertBitsFunction         = "naga_insertBits"
	SamplerHeapVar                 = "_naga_sampler_heap"
	ComparisonSamplerHeapVar       = "_naga_comparison_sampler_heap"
	SampleExternalTextureFunction  = "_naga_sample_external_texture"
	NagaAbsFunction                = "_naga_abs"
	NagaDivFunction                = "naga_div"
	NagaModFunction                = "naga_mod"
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
// If the name is reserved or empty, it's suffixed with underscore (matches Rust naga).
func Escape(name string) string {
	if name == "" {
		return UnnamedIdentifier
	}
	if IsReserved(name) || IsCaseInsensitiveReserved(name) {
		return name + "_"
	}
	return name
}
