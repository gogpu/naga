package wgsl

import (
	"errors"
	"strings"
	"testing"
)

// TestParseRejectsRepeatedAttributes: WGSL allows each attribute once per
// declaration (except @diagnostic), and Rust naga reports RepeatedAttribute.
func TestParseRejectsRepeatedAttributes(t *testing.T) {
	for _, tc := range []struct {
		name      string
		source    string
		attribute string
	}{
		{"group", "@group(0) @group(1) @binding(0) var<uniform> u: f32;", "group"},
		{"binding", "@group(0) @binding(0) @binding(1) var<uniform> u: f32;", "binding"},
		{"location member", "struct S { @location(0) @location(1) a: f32 }", "location"},
		{"builtin result", "fn f() -> @builtin(position) @builtin(position) vec4<f32> { return vec4<f32>(0.0); }", "builtin"},
		{"vertex stage", "@vertex @vertex fn f() -> @builtin(position) vec4<f32> { return vec4<f32>(0.0); }", "vertex"},
		{"workgroup_size", "@compute @workgroup_size(1) @workgroup_size(2) fn f() {}", "workgroup_size"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := Parse(tc.source, ParseOptions{})
			var parseErr *ParseError
			if !errors.As(err, &parseErr) {
				t.Fatalf("error %v (%T), want a *ParseError", err, err)
			}
			const want = "repeated attribute"
			if !strings.Contains(parseErr.Message, want) || !strings.Contains(parseErr.Message, tc.attribute) {
				t.Errorf("message = %q, want %q naming %q", parseErr.Message, want, tc.attribute)
			}
		})
	}
}

// TestParseAllowsRepeatedDiagnosticAttributes: the spec allows @diagnostic
// more than once on a declaration.
func TestParseAllowsRepeatedDiagnosticAttributes(t *testing.T) {
	source := `@diagnostic(warning, derivative_uniformity)
@diagnostic(error, derivative_uniformity)
fn main() {}`
	if _, _, err := Parse(source, ParseOptions{}); err != nil {
		t.Errorf("Parse: %v", err)
	}
}

// TestRepeatedAttributeCostIsBounded: the rejection comes before the
// attributes are collected, so the number of allocations does not follow the
// number of repeated attributes in the source. Before the fix, 100,000
// attributes allocated about a hundred bytes each.
func TestRepeatedAttributeCostIsBounded(t *testing.T) {
	small := "@group(0) " + strings.Repeat("@group(1) ", 10)
	large := "@group(0) " + strings.Repeat("@group(1) ", 100_000)
	parse := func(source string) {
		_, _, _ = Parse(source, ParseOptions{})
	}
	smallAllocs := testing.AllocsPerRun(5, func() { parse(small) })
	largeAllocs := testing.AllocsPerRun(5, func() { parse(large) })
	// The generous bound absorbs the module and error setup; the point is that
	// a 10,000x input is not 10,000x the work.
	if largeAllocs > smallAllocs*4+100 {
		t.Errorf("allocations: %v for 10 attributes, %v for 100,000: the parse follows the input",
			smallAllocs, largeAllocs)
	}
}
