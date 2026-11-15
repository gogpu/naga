package front

// Contracts of the split combined-sampler binding allocation, design §11: the
// sampler of a split combined image sampler takes a binding of its own, and
// the choice is made once every module-scope declaration is known, the way the
// SPIR-V frontend resolves it after its parse
// (spirv/internal/front/combined_sampler.go:78-111).

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
)

// globalBinding returns the binding of the global variable of a name.
func globalBinding(t *testing.T, mod *ir.Module, name string) ir.ResourceBinding {
	t.Helper()
	global := findGlobal(t, mod, name)
	if global.Binding == nil {
		t.Fatalf("global %q has no binding", name)
	}
	return *global.Binding
}

// twoSamplers is two combined samplers at bindings 0 and 1 of one group, each
// sampled: the smallest shader that makes the split samplers compete with the
// second declaration for a slot.
const twoSamplers = `#version 450
layout(set = 0, binding = 0) uniform sampler2D texA;
layout(set = 0, binding = 1) uniform sampler2D texB;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 o;
void main() {
    o = texture(texA, uv) + texture(texB, uv);
}
`

// TestCombinedSamplerBindingsDoNotCollide is the two-sampler case
// (P0-GLSLIN-003): the second image declaration takes binding 1, which the
// first sampler must not have taken, so the module validates.
func TestCombinedSamplerBindingsDoNotCollide(t *testing.T) {
	mod, info, err := Parse(twoSamplers, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}
	if got := globalBinding(t, mod, "texA"); got != (ir.ResourceBinding{Group: 0, Binding: 0}) {
		t.Fatalf("texA binding = %+v, want the declared (0, 0)", got)
	}
	if got := globalBinding(t, mod, "texB"); got != (ir.ResourceBinding{Group: 0, Binding: 1}) {
		t.Fatalf("texB binding = %+v, want the declared (0, 1)", got)
	}

	// The samplers take the next free slots after both images, in source
	// order.
	samplerA := globalBinding(t, mod, "texA_sampler")
	samplerB := globalBinding(t, mod, "texB_sampler")
	wantA := ir.ResourceBinding{Group: 0, Binding: 2}
	wantB := ir.ResourceBinding{Group: 0, Binding: 3}
	if samplerA != wantA || samplerB != wantB {
		t.Fatalf("sampler bindings = %+v and %+v, want %+v and %+v", samplerA, samplerB, wantA, wantB)
	}
	if len(info.CombinedSamplers) != 2 {
		t.Fatalf("ParseInfo.CombinedSamplers = %+v, want two splits", info.CombinedSamplers)
	}
	if info.CombinedSamplers[0].Split != wantA || info.CombinedSamplers[1].Split != wantB {
		t.Fatalf("reported splits = %+v, want %+v and %+v", info.CombinedSamplers, wantA, wantB)
	}
}

// TestCombinedSamplerSuppliedBindingIsReserved is the SamplerBindings arm: a
// supplied slot belongs to the sampler it was given to, so the automatic
// allocation of the next split sampler skips it.
func TestCombinedSamplerSuppliedBindingIsReserved(t *testing.T) {
	mod, info, err := Parse(twoSamplers, Options{
		Stage: ir.StageFragment,
		SamplerBindings: map[ir.ResourceBinding]ir.ResourceBinding{
			{Group: 0, Binding: 0}: {Group: 0, Binding: 2},
		},
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}
	if got := globalBinding(t, mod, "texA_sampler"); got != (ir.ResourceBinding{Group: 0, Binding: 2}) {
		t.Fatalf("texA_sampler binding = %+v, want the supplied (0, 2)", got)
	}
	if got := globalBinding(t, mod, "texB_sampler"); got != (ir.ResourceBinding{Group: 0, Binding: 3}) {
		t.Fatalf("texB_sampler binding = %+v, want the next free (0, 3)", got)
	}
	if got := info.CombinedSamplers[1].Split; got != (ir.ResourceBinding{Group: 0, Binding: 3}) {
		t.Fatalf("reported second split = %+v, want (0, 3)", got)
	}
}

// TestCombinedSamplerSuppliedBindingCollisionIsAnError is the other arm: a
// supplied slot another resource of the module already holds is refused
// instead of producing a module validation rejects.
func TestCombinedSamplerSuppliedBindingCollisionIsAnError(t *testing.T) {
	_, _, err := Parse(twoSamplers, Options{
		Stage: ir.StageFragment,
		SamplerBindings: map[ir.ResourceBinding]ir.ResourceBinding{
			{Group: 0, Binding: 0}: {Group: 0, Binding: 1},
		},
	})
	if err == nil {
		t.Fatal("Parse succeeded, want the supplied binding (0, 1) refused: texB holds it")
	}
	if want := "already uses"; !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want it to name the taken binding", err.Error())
	}
}
