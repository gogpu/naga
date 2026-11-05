package wgsl

import (
	"errors"
	"strings"
	"testing"
)

// TestAttributeOnWrongDeclaration: an attribute on a module-scope
// declaration it does not apply to is a shader-creation error (WGSL spec,
// "Attributes": each attribute names the objects it applies to). The error
// labels the attribute's name.
//
// Rust naga (front/wgsl/parse/mod.rs, Parser::global_decl) parses every
// global attribute first and afterwards rejects only @must_use on a
// non-function (Error::FunctionMustUseOnNonFunction, whose text naga uses
// for every function attribute) and @group/@binding without a `var`
// (Error::Internal). It ACCEPTS a stage attribute, @workgroup_size or
// @early_depth_test on a struct, alias, const, override, var or
// const_assert, @group/@binding on an override, and @id on anything but an
// override, silently dropping the attribute. naga follows the spec.
func TestAttributeOnWrongDeclaration(t *testing.T) {
	const (
		onFunction = "is only valid on function declarations"
		onVar      = "is only valid on variable declarations"
		onOverride = "is only valid on override declarations"
	)
	tests := []struct {
		name, src, attr, want string
	}{
		{"vertex on struct", "@vertex struct S { a: f32 }", "vertex", onFunction},
		{"fragment on alias", "@fragment alias A = f32;", "fragment", onFunction},
		{"compute on const", "@compute const c = 1;", "compute", onFunction},
		{"workgroup_size on const", "@workgroup_size(1) const c = 1;", "workgroup_size", onFunction},
		{"early_depth_test on var", "@early_depth_test(force) var<private> v: f32;", "early_depth_test", onFunction},
		{"must_use on struct", "@must_use struct S { a: f32 }", "must_use", onFunction},
		{"vertex on const_assert", "@vertex const_assert true;", "vertex", onFunction},
		{"compute on override", "@compute override o: u32 = 1;", "compute", onFunction},
		{"group on fn", "@group(0) @binding(0) fn f() {}", "group", onVar},
		{"binding on struct", "@binding(0) @group(0) struct S { a: f32 }", "binding", onVar},
		{"group on override", "@group(0) @binding(0) override o: u32 = 1;", "group", onVar},
		{"group on const", "@group(0) @binding(0) const c = 1;", "group", onVar},
		{"id on var", "@id(0) var<private> v: u32;", "id", onOverride},
		{"id on const", "@id(0) const c = 1;", "id", onOverride},
		{"id on fn", "@id(0) fn f() {}", "id", onOverride},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := Parse(tt.src, ParseOptions{})
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("Parse error = %v, want a *ParseError", err)
			}
			want := "attribute `@" + tt.attr + "` " + tt.want
			if pe.Message != want {
				t.Fatalf("message = %q, want %q", pe.Message, want)
			}
			if len(pe.Labels) == 0 {
				t.Fatal("no label")
			}
			at := strings.Index(tt.src, "@"+tt.attr) + 1
			span := pe.Labels[0].Span
			if got := tt.src[span.Start:span.End]; int(span.Start) != at || got != tt.attr {
				t.Fatalf("label %d..%d (%q), want the attribute name at %d", span.Start, span.End, got, at)
			}
		})
	}

	// Each attribute stays valid on the declaration it applies to.
	for _, src := range []string{
		"@vertex fn v() -> @builtin(position) vec4<f32> { return vec4<f32>(); }",
		"@compute @workgroup_size(1) fn c() {}",
		"@fragment @early_depth_test(force) fn f() {}",
		"@must_use fn m() -> u32 { return 1u; }",
		"@group(0) @binding(0) var<uniform> u: vec4<f32>;",
		"@id(3) override o: u32 = 1;",
	} {
		if _, _, err := Parse(src, ParseOptions{}); err != nil {
			t.Errorf("Parse(%q) = %v, want no error", src, err)
		}
	}
}
