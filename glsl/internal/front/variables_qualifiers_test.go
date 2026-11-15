package front

// Contracts of the interface qualifier rules of front/glsl/variables.rs:
// interpolation and sampling qualifiers (586-615), the output index binding
// (ast.rs QualifierKey::Index), the binding requirement of a block (759) and
// the address spaces of image and sampler globals (variables.rs:704-753).

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestInterpolationAndSamplingQualifiers is the qualifier table of
// variables.rs:586-615: noperspective is Linear, smooth is Perspective, flat is
// Flat, and centroid/sample select the sampling point. A sampling qualifier
// without an interpolation one is replaced by the default (Rust's
// apply_default_interpolation sets both when interpolation is absent,
// front/interpolator.rs:39-53).
func TestInterpolationAndSamplingQualifiers(t *testing.T) {
	source := `#version 450
noperspective in vec3 a;
smooth in vec3 b;
flat in vec3 c;
centroid smooth in vec3 d;
sample smooth in vec3 e;
centroid in vec3 f;
layout(location = 0) out vec4 o;
void main() {
    o = vec4(a + b + c + d + e + f, 1.0);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := map[string]struct {
		interpolation ir.Interpolation
		sampling      *ir.Sampling
	}{
		"a": {ir.InterpolationLinear, nil},
		"b": {ir.InterpolationPerspective, nil},
		"c": {ir.InterpolationFlat, nil},
		"d": {ir.InterpolationPerspective, samplingPtr(ir.SamplingCentroid)},
		"e": {ir.InterpolationPerspective, samplingPtr(ir.SamplingSample)},
		// No interpolation qualifier: Rust replaces the written sampling with
		// the default center one alongside the default interpolation.
		"f": {ir.InterpolationPerspective, samplingPtr(ir.SamplingCenter)},
	}
	args := onlyEntry(t, mod).Function.Arguments
	if len(args) != len(want) {
		t.Fatalf("entry point has %d arguments, want %d", len(args), len(want))
	}
	for _, arg := range args {
		expect, ok := want[arg.Name]
		if !ok {
			t.Fatalf("unexpected argument %q", arg.Name)
		}
		binding := locationBinding(t, "the input", arg.Binding)
		if binding.Interpolation == nil || *binding.Interpolation != expect.interpolation {
			t.Fatalf("%q interpolation = %v, want %v", arg.Name, binding.Interpolation, expect.interpolation)
		}
		if expect.sampling == nil {
			if binding.Sampling != nil {
				t.Fatalf("%q sampling = %v, want none", arg.Name, *binding.Sampling)
			}
			continue
		}
		if binding.Sampling == nil || *binding.Sampling != *expect.sampling {
			t.Fatalf("%q sampling = %v, want %v", arg.Name, binding.Sampling, *expect.sampling)
		}
	}
}

// samplingPtr returns a pointer to a sampling value.
func samplingPtr(s ir.Sampling) *ir.Sampling { return &s }

// TestOutputIndexBecomesBlendSrc is the index layout entry (ast.rs
// QualifierKey::Index): the output's blend source reaches the member's binding.
func TestOutputIndexBecomesBlendSrc(t *testing.T) {
	source := `#version 450
layout(location = 0, index = 1) out vec4 o;
void main() { o = vec4(1.0); }
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	entry := onlyEntry(t, mod)
	result := mod.Types[entry.Function.Result.Type].Inner.(ir.StructType)
	binding := locationBinding(t, "the output", result.Members[0].Binding)
	if binding.BlendSrc == nil || *binding.BlendSrc != 1 {
		t.Fatalf("blend source = %v, want the declared index 1", binding.BlendSrc)
	}
}

// TestBlockRequiresBinding is variables.rs:755-763: a uniform or buffer block
// must name its binding.
func TestBlockRequiresBinding(t *testing.T) {
	_, _, err := Parse(`#version 450
layout(set = 0) uniform U { float x; } u;
layout(location = 0) out vec4 o;
void main() { o = vec4(u.x); }
`, Options{Stage: ir.StageFragment})
	if err == nil || !strings.Contains(err.Error(), "uniform/buffer blocks require layout(binding=X)") {
		t.Fatalf("error = %v, want the missing-binding error", err)
	}
}

// TestStd430StorageBlockIsRestricted is the read-only storage block: the
// declared layout is kept (std430) and the memory qualifier narrows the
// global's access.
func TestStd430StorageBlockIsRestricted(t *testing.T) {
	source := `#version 450
layout(std430, set = 0, binding = 0) readonly buffer B { float data[4]; } b;
layout(location = 0) out vec4 o;
void main() { o = vec4(b.data[0]); }
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	block := findGlobal(t, mod, "b")
	if block.Access != ir.StorageAccessLoad {
		t.Fatalf("block access = %v, want the readonly Load", block.Access)
	}
	structTy := mod.Types[block.Type].Inner.(ir.StructType)
	array, ok := mod.Types[structTy.Members[0].Type].Inner.(ir.ArrayType)
	if !ok || array.Stride != 4 {
		t.Fatalf("member array = %v, want the std430 stride 4", mod.Types[structTy.Members[0].Type].Inner)
	}
}

// TestSamplerGlobalsAreHandleSpace is variables.rs:713-743: a sampled image and
// a sampler live in the handle address space, unlike a block.
func TestSamplerGlobalsAreHandleSpace(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) uniform texture2D t;
layout(set = 0, binding = 1) uniform sampler s;
layout(location = 0) out vec4 o;
void main() { o = texture(sampler2D(t, s), vec2(0.5)); }
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	for _, name := range []string{"t", "s"} {
		global := findGlobal(t, mod, name)
		if global.Space != ir.SpaceHandle {
			t.Fatalf("%s space = %v, want handle", name, global.Space)
		}
	}
}
