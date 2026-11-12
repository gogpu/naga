package wgsl

import (
	"fmt"
	"strings"
	"testing"
)

// TestParseRejectsUnknownAttributes: an attribute the parser does not know is
// a shader-creation error at the first one, Rust naga's UnknownAttribute
// (front/wgsl/parse/mod.rs, global_decl and BindingParser).
func TestParseRejectsUnknownAttributes(t *testing.T) {
	for _, tc := range []struct {
		name, source, attribute string
	}{
		{"function", "@a0 fn f() {}", "a0"},
		{"variable", "@group(0) @binding(0) @blah var<uniform> u: f32;", "blah"},
		{"parameter", "fn f(@location2(0) a: f32) {}", "location2"},
		{"result", "fn f() -> @what vec4<f32> { return vec4<f32>(0.0); }", "what"},
		{"struct member", "struct S { @offset(4) a: f32 }", "offset"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := parseRefused(t, tc.source)
			if want := fmt.Sprintf("unknown attribute: `%s`", tc.attribute); !strings.Contains(got, want) {
				t.Errorf("message = %q, want %q", got, want)
			}
		})
	}
}

// TestParseKeepsKnownAttributes: every attribute naga lowers still parses.
func TestParseKeepsKnownAttributes(t *testing.T) {
	for _, source := range []string{
		"@diagnostic(off, derivative_uniformity) @fragment fn f(@location(0) @interpolate(flat) a: u32, " +
			"@builtin(front_facing) b: bool) -> @location(0) vec4<f32> { return vec4<f32>(0.0); }",
		"@vertex fn f() -> @invariant @builtin(position) vec4<f32> { return vec4<f32>(0.0); }",
		"@compute @workgroup_size(1) @must_use fn f() -> i32 { return 1; }",
		"@fragment @early_depth_test(force) fn f() {}",
		"struct S { @size(16) @align(16) a: f32 }",
		"@id(0) override o: f32 = 1.0;",
		"@group(0) @binding(0) var<uniform> u: f32;",
		"enable dual_source_blending;\nstruct S { @location(0) @blend_src(0) a: vec4<f32> }",
	} {
		if _, _, err := Parse(source, ParseOptions{}); err != nil {
			t.Errorf("Parse(%q): %v", source, err)
		}
	}
}

// TestUnknownAttributeCostIsBounded: distinct unknown names are refused at
// the first, so 100,000 of them cost no more than 10. Before the fix the
// parser collected every one (1 GB for 8.9 MB of source).
func TestUnknownAttributeCostIsBounded(t *testing.T) {
	source := func(n int) string {
		var b strings.Builder
		for i := range n {
			fmt.Fprintf(&b, "@a%d ", i)
		}
		return b.String() + "fn f() {}"
	}
	small, large := source(10), source(100_000)
	parse := func(source string) {
		_, _, _ = Parse(source, ParseOptions{})
	}
	smallAllocs := testing.AllocsPerRun(5, func() { parse(small) })
	largeAllocs := testing.AllocsPerRun(5, func() { parse(large) })
	if largeAllocs > smallAllocs*4+100 {
		t.Errorf("allocations: %v for 10 unknown attributes, %v for 100,000: the parse follows the input",
			smallAllocs, largeAllocs)
	}
}
