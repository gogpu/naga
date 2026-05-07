package backend

import "testing"

func TestEndsWithDigit(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"empty string", "", false},
		{"single letter", "a", false},
		{"single digit", "5", true},
		{"ends with zero", "abc0", true},
		{"ends with nine", "abc9", true},
		{"ends with letter", "abc1x", false},
		{"underscore at end", "foo_", false},
		{"digit then underscore", "foo1_", false},
		{"only digits", "123", true},
		{"mixed then digit", "a1b2c3", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EndsWithDigit(tt.input); got != tt.want {
				t.Errorf("EndsWithDigit(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsASCIIAlphanumeric(t *testing.T) {
	tests := []struct {
		name  string
		input rune
		want  bool
	}{
		{"lowercase a", 'a', true},
		{"lowercase z", 'z', true},
		{"uppercase A", 'A', true},
		{"uppercase Z", 'Z', true},
		{"digit 0", '0', true},
		{"digit 9", '9', true},
		{"underscore", '_', false},
		{"space", ' ', false},
		{"dash", '-', false},
		{"at sign", '@', false},
		{"curly brace", '{', false},
		{"unicode letter", '\u00e9', false}, // accented e
		{"null byte", 0, false},
		{"tilde", '~', false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsASCIIAlphanumeric(tt.input); got != tt.want {
				t.Errorf("IsASCIIAlphanumeric(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNeedsTrailingUnderscore(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		// Empty string.
		{"empty", "", false},

		// Ends with digit — always needs underscore.
		{"ends with digit", "var0", true},
		{"single digit", "7", true},
		{"digit in middle no", "a1b", false},

		// Case-sensitive HLSL reserved keywords (sample from the map).
		{"keyword float", "float", true},
		{"keyword void", "void", true},
		{"keyword return", "return", true},
		{"keyword struct", "struct", true},
		{"keyword abs", "abs", true},
		{"keyword cbuffer", "cbuffer", true},
		{"keyword register", "register", true},
		{"keyword WaveGetLaneIndex", "WaveGetLaneIndex", true},
		{"keyword SV_Position", "SV_Position", true},
		{"keyword naga_modf", "naga_modf", true},

		// Case-insensitive HLSL keywords.
		{"case insensitive asm", "asm", true},        // also in case-sensitive map
		{"case insensitive ASM", "ASM", true},        // matched via ToLower
		{"case insensitive Asm", "Asm", true},        // matched via ToLower
		{"case insensitive texture2d", "texture2d", true},
		{"case insensitive TEXTURE2D", "TEXTURE2D", true},
		{"case insensitive Texture2D", "Texture2D", true}, // also in case-sensitive map
		{"case insensitive pass", "pass", true},
		{"case insensitive PASS", "PASS", true},

		// Not a keyword, not ending in digit.
		{"normal identifier", "myVariable", false},
		{"normal with underscore", "my_var", false},
		{"normal camelCase", "fooBar", false},
		{"empty-like but not", "x", false},

		// Not a keyword but close — case matters for case-sensitive set.
		{"FLOAT uppercase", "FLOAT", false},  // not in case-sensitive, not in case-insensitive
		{"Return capitalized", "Return", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NeedsTrailingUnderscore(tt.input); got != tt.want {
				t.Errorf("NeedsTrailingUnderscore(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestHLSLReservedKeywords_NonEmpty verifies the keyword map is populated.
func TestHLSLReservedKeywords_NonEmpty(t *testing.T) {
	if len(HLSLReservedKeywords) == 0 {
		t.Fatal("HLSLReservedKeywords is empty")
	}
	// Spot-check a few from different categories.
	mustExist := []string{"float", "int", "void", "abs", "cos", "WaveGetLaneIndex", "SV_Position"}
	for _, kw := range mustExist {
		if _, ok := HLSLReservedKeywords[kw]; !ok {
			t.Errorf("HLSLReservedKeywords missing %q", kw)
		}
	}
}

// TestHLSLCaseInsensitiveKeywords_NonEmpty verifies the case-insensitive map.
func TestHLSLCaseInsensitiveKeywords_NonEmpty(t *testing.T) {
	if len(HLSLCaseInsensitiveKeywords) == 0 {
		t.Fatal("HLSLCaseInsensitiveKeywords is empty")
	}
	mustExist := []string{"asm", "pass", "texture2d", "texturecube"}
	for _, kw := range mustExist {
		if _, ok := HLSLCaseInsensitiveKeywords[kw]; !ok {
			t.Errorf("HLSLCaseInsensitiveKeywords missing %q", kw)
		}
	}
}
