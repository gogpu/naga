package wgsl

import (
	"errors"
	"strings"
	"testing"
)

// TestParseRejectsOutOfRangeIntegerLiterals: a literal not representable by
// its type is a shader-creation error, not a clamp. Rust naga's
// NumberError::NotRepresentable has the same message.
func TestParseRejectsOutOfRangeIntegerLiterals(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{"i32 high", `const x = 3000000000i;`},
		{"i32 low", `const x = -2147483649i;`},
		// The literal is 2147483648 before the minus applies, so it is not
		// representable as i32; Rust naga rejects it the same way.
		{"i32 minimum written literally", `const x = -2147483648i;`},
		{"u32", `const x = 4294967296u;`},
		{"i64", `const x = 9223372036854775808li;`},
		{"u64", `const x = 18446744073709551616lu;`},
		{"abstract", `const x = 99999999999999999999;`},
		{"abstract in expression", `const x = 99999999999999999999 % 1000;`},
		{"hex i32", `const x = 0x80000000i;`},
		{"group attribute", "@group(4294967297) @binding(0) var<uniform> u: f32;"},
		{"location attribute", "struct S { @location(4294967296) a: f32 }"},
		{"blend_src attribute", "enable dual_source_blending;\nstruct S { @location(0) @blend_src(4294967296) a: f32 }"},
		{"binding array size", "enable wgpu_binding_array;\n@group(0) @binding(0) var t: binding_array<f32, 4294967296>;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := Parse(tc.source, ParseOptions{})
			var parseErr *ParseError
			if !errors.As(err, &parseErr) {
				t.Fatalf("error %v (%T), want a *ParseError", err, err)
			}
			const want = "numeric literal not representable by target type"
			if !strings.Contains(parseErr.Message, want) {
				t.Errorf("message = %q, want it to contain %q", parseErr.Message, want)
			}
		})
	}
}

// TestParseKeepsInRangeIntegerLiterals: the neighbors of the rejected values
// still compile, so the check is not broader than the type.
func TestParseKeepsInRangeIntegerLiterals(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{"i32 max", `const x = 2147483647i;`},
		{"u32 max", `const x = 4294967295u;`},
		{"i64 max", `const x = 9223372036854775807li;`},
		{"u64 max", `const x = 18446744073709551615lu;`},
		{"abstract", `const x = 9223372036854775807;`},
		{"hex i32 max", `const x = 0x7fffffffi;`},
		// The two spellings of i32's minimum that stay valid: an abstract
		// literal converted to i32, and an expression of i32 literals.
		{"i32 min through an abstract literal", `const x = i32(-2147483648);`},
		{"i32 min as an expression", `const x = -2147483647i - 1;`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := Parse(tc.source, ParseOptions{}); err != nil {
				t.Errorf("Parse: %v", err)
			}
		})
	}
}
