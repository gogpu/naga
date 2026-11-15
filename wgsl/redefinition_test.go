package wgsl

import (
	"strings"
	"testing"
)

// TestParseRejectsRedefinition pins WGSL's one namespace for module-scope
// declarations and for a struct's members: a repeated name is a shader-creation
// error, Rust naga's Error::Redefinition from its index pass
// (front/wgsl/index.rs:24-39). The validator no longer checks function names,
// because the GLSL frontend's overloads share one (valid/validate.go).
func TestParseRejectsRedefinition(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"function", "fn f() {}\nfn f() {}\n", "redefinition of `f`"},
		{"struct member", "struct S { a: f32, a: i32, }\n", "redefinition of `a`"},
		{"variable and function", "var<private> x: f32;\nfn x() {}\n", "redefinition of `x`"},
		{"const and struct", "struct T { v: f32, }\nconst T: f32 = 1.0;\n", "redefinition of `T`"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := Parse(tt.src, ParseOptions{})
			if err == nil {
				t.Fatalf("Parse accepted a redefinition: %q", tt.src)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want it to contain %q", err.Error(), tt.want)
			}
		})
	}
}

// TestParseAcceptsDistinctNames keeps the check honest: different names in one
// scope and the same member name in two structs are legal.
func TestParseAcceptsDistinctNames(t *testing.T) {
	src := `struct S { a: f32, }
struct T { a: f32, }
fn f() {}
fn g() {}
var<private> x: f32;
`
	if _, _, err := Parse(src, ParseOptions{}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
}
