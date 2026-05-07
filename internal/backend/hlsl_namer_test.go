package backend

import (
	"strings"
	"testing"
)

// TestNeedsTrailingUnderscore_DXILResourceNames verifies that resource variable
// names used in real naga shaders produce the correct trailing-underscore
// decision. The DXIL backend (dxil/internal/emit/resources.go:195) calls this
// to match DXC's metadata resource naming. Getting this wrong means DXIL
// container validation fails.
//
// Test cases sourced from actual shader golden files and the Rust naga namer:
//   - t1, t2, atomic_i32 etc. end with digits -> underscore
//   - "in", "out", "float" etc. are HLSL keywords -> underscore
//   - "params", "camera" etc. are plain identifiers -> no underscore
func TestNeedsTrailingUnderscore_DXILResourceNames(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		// Real DXIL shader resource names that end with digits.
		// The Rust namer appends SEPARATOR ('_') when base.ends_with(char::is_numeric).
		{"t1", true},
		{"t2", true},
		{"atomic_i32", true},
		{"atomic_u32", true},
		{"input1", true},
		{"input2", true},
		{"input3", true},
		{"image_2d_i32", true},
		{"in_data_storage_g0_b3", true},

		// HLSL case-sensitive keywords — Rust namer: self.keywords.contains(base).
		{"in", true},
		{"out", true},
		{"float", true},
		{"int", true},
		{"struct", true},
		{"texture", true},
		{"indices", true},
		{"vertices", true},
		{"register", true},
		{"cbuffer", true},
		{"void", true},
		{"return", true},

		// DXC-specific reserved keywords.
		{"nullptr", true},
		{"constexpr", true},
		{"alignas", true},

		// Wave/ray intrinsics — these are keywords too.
		{"WaveGetLaneIndex", true},
		{"TraceRay", true},

		// Naga helper names — reserved to avoid conflicts with generated code.
		{"naga_modf", true},
		{"naga_frexp", true},
		{"naga_div", true},
		{"_naga_abs", true},

		// SV_* semantic names — reserved.
		{"SV_Position", true},
		{"SV_Target", true},
		{"SV_DispatchThreadID", true},

		// Normal names that should NOT get underscore.
		{"params", false},
		{"uniformOne", false},
		{"camera", false},
		{"globals", false},
		{"output", false},
		{"result", false},
		{"nagaSamplerHeap", false},
		{"myBuffer", false},

		// Empty string.
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NeedsTrailingUnderscore(tt.name); got != tt.want {
				t.Errorf("NeedsTrailingUnderscore(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// TestNeedsTrailingUnderscore_CaseInsensitive verifies the case-insensitive
// keyword matching that mirrors Rust naga's CaseInsensitiveKeywordSet.
//
// FXC (without strict mode) treats these keywords as case-insensitive.
// FXC strict mode: "error X3086: alternate cases for 'pass' are deprecated".
// Rust stores them in AsciiUniCase (eq_ignore_ascii_case); our Go code uses
// strings.ToLower + exact map lookup. Both must produce the same result.
//
// The canonical list from Rust hlsl/keywords.rs RESERVED_CASE_INSENSITIVE:
//
//	asm, decl, pass, technique, Texture1D, Texture2D, Texture3D, TextureCube
func TestNeedsTrailingUnderscore_CaseInsensitive(t *testing.T) {
	// Every case variant of each case-insensitive keyword must trigger.
	caseInsensitiveKeywords := []string{
		"asm", "decl", "pass", "technique",
		"Texture1D", "Texture2D", "Texture3D", "TextureCube",
	}
	for _, kw := range caseInsensitiveKeywords {
		// Test lowercase, UPPERCASE, and TitleCase.
		variants := []string{
			strings.ToLower(kw),
			strings.ToUpper(kw),
			strings.ToUpper(kw[:1]) + strings.ToLower(kw[1:]),
		}
		for _, v := range variants {
			t.Run(v, func(t *testing.T) {
				if !NeedsTrailingUnderscore(v) {
					t.Errorf("NeedsTrailingUnderscore(%q) = false, want true (case-insensitive keyword)", v)
				}
			})
		}
	}

	// Verify that case matters for NON-case-insensitive keywords.
	// "float" is a case-sensitive keyword: "float"->true, "FLOAT"->false.
	if NeedsTrailingUnderscore("FLOAT") {
		t.Error("FLOAT should NOT match: not in case-insensitive set, not in case-sensitive set")
	}
	if NeedsTrailingUnderscore("Return") {
		t.Error("Return should NOT match: case-sensitive set has 'return' not 'Return'")
	}
}

// TestNeedsTrailingUnderscore_EdgeCases verifies boundary conditions.
func TestNeedsTrailingUnderscore_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		// Single characters at boundary.
		{"single zero", "0", true},
		{"single nine", "9", true},
		{"single letter", "x", false},

		// Digit in middle but not at end.
		{"digit middle", "a1b", false},
		{"digit then underscore", "foo1_", false},

		// Multi-byte rune at end — not an ASCII digit.
		{"unicode suffix", "var\u00e9", false},

		// Close to a keyword but not one.
		{"float_x prefix", "float_x", false},
		{"xfloat suffix", "xfloat", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NeedsTrailingUnderscore(tt.input); got != tt.want {
				t.Errorf("NeedsTrailingUnderscore(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestEndsWithDigit verifies ASCII digit detection at end of string,
// matching Rust's base.ends_with(char::is_numeric).
func TestEndsWithDigit(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"", false},
		{"abc0", true},
		{"abc9", true},
		{"abc1x", false},
		{"foo_", false},
		{"123", true},
		{"a", false},
		{"5", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := EndsWithDigit(tt.input); got != tt.want {
				t.Errorf("EndsWithDigit(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestIsASCIIAlphanumeric verifies the Go implementation matches
// Rust's char::is_ascii_alphanumeric(), which the Rust namer uses
// in sanitize() to filter identifier characters.
func TestIsASCIIAlphanumeric(t *testing.T) {
	// Verify the exact boundaries: a-z, A-Z, 0-9 are true.
	// Everything else including underscore, unicode, control chars is false.
	boundaries := []struct {
		r    rune
		want bool
	}{
		// Inclusive boundaries.
		{'a', true}, {'z', true}, {'A', true}, {'Z', true},
		{'0', true}, {'9', true},
		// Just outside boundaries.
		{'a' - 1, false}, {'z' + 1, false},
		{'A' - 1, false}, {'Z' + 1, false},
		{'0' - 1, false}, {'9' + 1, false},
		// Underscore — NOT alphanumeric (critical: Rust namer checks separately).
		{'_', false},
		// Unicode — NOT ASCII alphanumeric.
		{'\u03b8', false}, // theta
		{'\u00e9', false}, // e-acute
	}
	for _, tt := range boundaries {
		if got := IsASCIIAlphanumeric(tt.r); got != tt.want {
			t.Errorf("IsASCIIAlphanumeric(%q / U+%04X) = %v, want %v", tt.r, tt.r, got, tt.want)
		}
	}
}

// TestHLSLKeywordMaps_RustParity verifies that our keyword maps contain
// all categories from the Rust naga hlsl/keywords.rs source.
func TestHLSLKeywordMaps_RustParity(t *testing.T) {
	// Spot-check from each Rust category.
	mustExistCaseSensitive := map[string]string{
		// FXC keywords
		"float": "FXC keyword",
		"void":  "FXC keyword",
		"break": "FXC keyword",
		// FXC reserved words
		"catch":    "FXC reserved",
		"delete":   "FXC reserved",
		"typename": "FXC reserved",
		// FXC intrinsics
		"abs":       "FXC intrinsic",
		"cos":       "FXC intrinsic",
		"normalize": "FXC intrinsic",
		"transpose": "FXC intrinsic",
		// DXC C11 keywords
		"_Atomic":    "DXC C11",
		"constexpr":  "DXC C++",
		"nullptr":    "DXC C++",
		"__declspec": "DXC compiler",
		// DXC wave ops
		"WaveGetLaneIndex":   "DXC wave op",
		"WaveActiveBallot":   "DXC wave op",
		"QuadReadAcrossX":    "DXC quad op",
		"WaveActiveAnyTrue":  "DXC wave op",
		"WaveActiveAllEqual": "DXC wave op",
		// DXC ray tracing
		"TraceRay":      "DXC ray tracing",
		"DispatchMesh":  "DXC mesh shader",
		"IsHelperLane":  "DXC helper",
		"ReportHit":     "DXC ray tracing",
		"AcceptHitAndEndSearch": "DXC ray tracing",
		// DXC resource types
		"RasterizerOrderedBuffer": "DXC ROV",
		"RayQuery":                "DXC ray query",
		"ConstantBuffer":          "DXC resource",
		// SV_* semantics
		"SV_Position":          "semantic",
		"SV_Target":            "semantic",
		"SV_Depth":             "semantic",
		"SV_VertexID":          "semantic",
		"SV_InstanceID":        "semantic",
		"SV_DispatchThreadID":  "semantic",
		"SV_GroupID":           "semantic",
		"SV_GroupThreadID":     "semantic",
		"SV_GroupIndex":        "semantic",
		"SV_IsFrontFace":      "semantic",
		"SV_SampleIndex":      "semantic",
		"SV_Coverage":         "semantic",
		"SV_PrimitiveID":      "semantic",
		"SV_ClipDistance":      "semantic",
		"SV_CullDistance":      "semantic",
		"SV_RenderTargetArrayIndex": "semantic",
		"SV_ViewportArrayIndex":     "semantic",
		"SV_StencilRef":        "semantic",
		"SV_Barycentrics":      "semantic",
		"SV_ShadingRate":       "semantic",
		"SV_CullPrimitive":     "semantic",
		// Naga helpers
		"naga_modf":          "naga helper",
		"naga_frexp":         "naga helper",
		"naga_extractBits":   "naga helper",
		"naga_insertBits":    "naga helper",
		"naga_div":           "naga helper",
		"_naga_abs":          "naga helper",
		"_naga_neg":          "naga helper",
	}
	for kw, category := range mustExistCaseSensitive {
		if _, ok := HLSLReservedKeywords[kw]; !ok {
			t.Errorf("HLSLReservedKeywords missing %q (%s)", kw, category)
		}
	}

	// Case-insensitive: must contain all 8 entries from Rust RESERVED_CASE_INSENSITIVE,
	// stored as lowercase keys.
	mustExistCaseInsensitive := []string{
		"asm", "decl", "pass", "technique",
		"texture1d", "texture2d", "texture3d", "texturecube",
	}
	if len(HLSLCaseInsensitiveKeywords) != len(mustExistCaseInsensitive) {
		t.Errorf("HLSLCaseInsensitiveKeywords has %d entries, want %d (Rust parity)",
			len(HLSLCaseInsensitiveKeywords), len(mustExistCaseInsensitive))
	}
	for _, kw := range mustExistCaseInsensitive {
		if _, ok := HLSLCaseInsensitiveKeywords[kw]; !ok {
			t.Errorf("HLSLCaseInsensitiveKeywords missing %q", kw)
		}
	}
}

// TestLocationSemantic_Value verifies the cross-repo semantic name constant.
// Five consumers must agree on this exact value (see semantic.go doc).
// Changing it breaks D3D12 CreateGraphicsPipelineState (E_INVALIDARG).
func TestLocationSemantic_Value(t *testing.T) {
	if LocationSemantic != "LOC" {
		t.Errorf("LocationSemantic = %q, want %q — changing this breaks D3D12 pipelines", LocationSemantic, "LOC")
	}
}
