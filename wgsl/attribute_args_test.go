package wgsl

import (
	"errors"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// The integer arguments of @group, @binding, @location, @blend_src, @id and
// the binding_array size are const-expressions of a concrete integer scalar,
// as Rust naga's const_u32 (front/wgsl/lower/mod.rs) evaluates them; the
// parser keeps them as expressions (front/wgsl/parse/mod.rs, BindingParser).
// Parsing the raw literal token refused a suffix or hex spelling and silently
// dropped a named constant.

// parseOK parses source and fails the test on an error.
func parseOK(t *testing.T, source string) *ir.Module {
	t.Helper()
	module, _, err := Parse(source, ParseOptions{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return module
}

// parseRefused parses source and returns the error's message.
func parseRefused(t *testing.T, source string) string {
	t.Helper()
	_, _, err := Parse(source, ParseOptions{})
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("error %v (%T), want a *ParseError", err, err)
	}
	return parseErr.Message
}

// resourceBinding returns the binding of the global named name.
func resourceBinding(t *testing.T, module *ir.Module, name string) ir.ResourceBinding {
	t.Helper()
	for _, global := range module.GlobalVariables {
		if global.Name == name {
			if global.Binding == nil {
				t.Fatalf("global %s has no binding", name)
			}
			return *global.Binding
		}
	}
	t.Fatalf("no global %s", name)
	return ir.ResourceBinding{}
}

func TestResourceBindingArguments(t *testing.T) {
	for _, tc := range []struct {
		name           string
		decl           string
		group, binding uint32
	}{
		{"u suffix", "@group(0u) @binding(1u)", 0, 1},
		{"i suffix", "@group(2i) @binding(3i)", 2, 3},
		{"hex", "@group(0x1) @binding(0x2)", 1, 2},
		{"hex with suffix", "@group(0x3u) @binding(0x4i)", 3, 4},
		{"named constants", "@group(A) @binding(B + 1u)", 1, 3},
		{"maximum", "@group(4294967295u) @binding(2147483647)", 4294967295, 2147483647},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "const A = 1;\nconst B = 2u;\n" + tc.decl + " var<uniform> u: f32;\n" +
				"@compute @workgroup_size(1) fn main() { _ = u; }\n"
			got := resourceBinding(t, parseOK(t, source), "u")
			if got.Group != tc.group || got.Binding != tc.binding {
				t.Errorf("binding = (%d, %d), want (%d, %d)", got.Group, got.Binding, tc.group, tc.binding)
			}
		})
	}
}

func TestLocationArguments(t *testing.T) {
	source := "const L = 3;\n" +
		"struct In { @location(L) b: f32, @location(0x4u) c: f32 }\n" +
		"@fragment fn main(@location(1u) a: f32, in: In) -> @location(0x2) vec4<f32> {\n" +
		"    return vec4<f32>(a + in.b + in.c);\n}\n"
	module := parseOK(t, source)
	if len(module.EntryPoints) != 1 {
		t.Fatalf("entry points = %d, want 1", len(module.EntryPoints))
	}
	fn := module.EntryPoints[0].Function
	location := func(binding *ir.Binding) uint32 {
		t.Helper()
		if binding == nil {
			t.Fatal("no binding")
		}
		loc, ok := (*binding).(ir.LocationBinding)
		if !ok {
			t.Fatalf("binding = %#v, want a location", *binding)
		}
		return loc.Location
	}
	if got := location(fn.Arguments[0].Binding); got != 1 {
		t.Errorf("@location(1u) = %d, want 1", got)
	}
	if got := location(fn.Result.Binding); got != 2 {
		t.Errorf("@location(0x2) = %d, want 2", got)
	}
	in, ok := module.Types[fn.Arguments[1].Type].Inner.(ir.StructType)
	if !ok || len(in.Members) != 2 {
		t.Fatalf("In = %#v, want a struct of two members", module.Types[fn.Arguments[1].Type].Inner)
	}
	if got := location(in.Members[0].Binding); got != 3 {
		t.Errorf("@location(L) = %d, want 3", got)
	}
	if got := location(in.Members[1].Binding); got != 4 {
		t.Errorf("@location(0x4u) = %d, want 4", got)
	}
}

func TestBlendSrcArgument(t *testing.T) {
	source := "enable dual_source_blending;\n" +
		"struct Out { @location(0) @blend_src(0u) a: vec4<f32>, @location(0) @blend_src(0x1) b: vec4<f32> }\n" +
		"@fragment fn main() -> Out { return Out(vec4<f32>(1.0), vec4<f32>(0.0)); }\n"
	module := parseOK(t, source)
	fn := module.EntryPoints[0].Function
	out, ok := module.Types[fn.Result.Type].Inner.(ir.StructType)
	if !ok || len(out.Members) != 2 {
		t.Fatalf("Out = %#v", module.Types[fn.Result.Type].Inner)
	}
	for i, want := range []uint32{0, 1} {
		loc, ok := (*out.Members[i].Binding).(ir.LocationBinding)
		if !ok || loc.BlendSrc == nil || *loc.BlendSrc != want {
			t.Errorf("member %d binding = %#v, want blend_src %d", i, *out.Members[i].Binding, want)
		}
	}
}

func TestBindingArraySizeArgument(t *testing.T) {
	for _, tc := range []struct {
		name, size string
		want       uint32
	}{
		{"u suffix", "4u", 4},
		{"hex", "0x5", 5},
		{"named constant", "N", 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "enable wgpu_binding_array;\nconst N = 6u;\n" +
				"@group(0) @binding(0) var t: binding_array<texture_2d<f32>, " + tc.size + ">;\n"
			module := parseOK(t, source)
			found := false
			for _, ty := range module.Types {
				array, ok := ty.Inner.(ir.BindingArrayType)
				if !ok {
					continue
				}
				found = true
				if array.Size == nil || *array.Size != tc.want {
					t.Errorf("size = %v, want %d", array.Size, tc.want)
				}
			}
			if !found {
				t.Fatal("no binding array type")
			}
		})
	}
}

func TestOverrideIDArgument(t *testing.T) {
	for _, tc := range []struct {
		name, id string
		want     uint16
	}{
		{"decimal", "7", 7},
		{"hex", "0x10", 16},
		{"u suffix", "5u", 5},
		{"named constant", "K", 9},
		{"maximum", "65535", 65535},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "const K = 9;\n@id(" + tc.id + ") override o: f32 = 1.0;\n"
			module := parseOK(t, source)
			if len(module.Overrides) != 1 {
				t.Fatalf("overrides = %d, want 1", len(module.Overrides))
			}
			id := module.Overrides[0].ID
			if id == nil || *id != tc.want {
				t.Errorf("id = %v, want %d", id, tc.want)
			}
		})
	}
}

// TestAttributeArgumentErrors: the arguments Rust's const_u32 refuses, with
// its messages.
func TestAttributeArgumentErrors(t *testing.T) {
	const (
		notConcreteInteger = "must be a const-expression that resolves to a concrete integer scalar (`u32` or `i32`)"
		negative           = "must be non-negative (>= 0)"
		idRange            = "pipeline constant ID must be between 0 and 65535 inclusive"
	)
	use := "\n@compute @workgroup_size(1) fn main() { _ = u; }\n"
	for _, tc := range []struct {
		name, source, want string
	}{
		// An abstract integer past i32 and u32 concretizes to f32 (Rust's
		// concretize tries i32, u32, f32), so const_u32 refuses its type.
		{"group past u32", "@group(4294967297) @binding(0) var<uniform> u: f32;" + use, notConcreteInteger},
		{"location past u32", "struct S { @location(4294967296) a: f32 }", notConcreteInteger},
		{"blend_src past u32", "enable dual_source_blending;\nstruct S { @location(0) @blend_src(4294967296) a: f32 }",
			notConcreteInteger},
		{"binding array size past u32",
			"enable wgpu_binding_array;\n@group(0) @binding(0) var t: binding_array<f32, 4294967296>;", notConcreteInteger},
		{"suffixed binding past u32", "@group(0) @binding(4294967296u) var<uniform> u: f32;" + use,
			"numeric literal not representable by target type"},
		{"negative group", "@group(-1) @binding(0) var<uniform> u: f32;" + use, negative},
		{"negative binding", "const N = -2;\n@group(0) @binding(N) var<uniform> u: f32;" + use, negative},
		{"float group", "@group(1.0) @binding(0) var<uniform> u: f32;" + use, notConcreteInteger},
		{"bool binding", "@group(0) @binding(true) var<uniform> u: f32;" + use, notConcreteInteger},
		{"override group", "override g: u32 = 0u;\n@group(g) @binding(0) var<uniform> u: f32;" + use, "override"},
		{"negative location", "@fragment fn main(@location(-1) a: f32) {}\n", negative},
		{"float location", "@fragment fn main(@location(1.5) a: f32) {}\n", notConcreteInteger},
		{"negative id", "@id(-1) override o: f32 = 1.0;\n", negative},
		{"id over u16", "@id(70000) override o: f32 = 1.0;\n", idRange},
		{"hex id over u16", "@id(0x10000u) override o: f32 = 1.0;\n", idRange},
		{"float id", "@id(1.0) override o: f32 = 1.0;\n", notConcreteInteger},
		{"zero binding array", "enable wgpu_binding_array;\n@group(0) @binding(0) var t: binding_array<texture_2d<f32>, 0u>;\n",
			"array element count must be positive (> 0)"},
		{"negative binding array", "enable wgpu_binding_array;\n@group(0) @binding(0) var t: binding_array<texture_2d<f32>, -1>;\n",
			"array element count must be positive (> 0)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseRefused(t, tc.source); !strings.Contains(got, tc.want) {
				t.Errorf("message = %q, want it to contain %q", got, tc.want)
			}
		})
	}
}
