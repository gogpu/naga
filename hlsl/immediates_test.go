package hlsl_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/hlsl"
)

const immediatesShader = `
struct Im { scale: f32 }
var<immediate> im: Im;
@fragment fn main() -> @location(0) vec4<f32> { return vec4<f32>(im.scale); }`

// TestImmediatesTarget: the immediate data is the constant buffer at
// ImmediatesTarget (Rust naga's immediates_target). Without a target it is
// an error, FakeMissingBindings or not: the data has no WGSL binding to fall
// back on, and the old fallback, register(b0), could take the caller's own b0.
func TestImmediatesTarget(t *testing.T) {
	m := lowerBoundsSource(t, immediatesShader)
	for _, fake := range []bool{false, true} {
		_, _, err := hlsl.Compile(m, hlsl.Options{FakeMissingBindings: fake})
		if !errors.Is(err, backend.ErrMissingBinding) {
			t.Errorf("FakeMissingBindings %v, no target: err = %v, want backend.ErrMissingBinding", fake, err)
		}
	}
	src, _, err := hlsl.Compile(m, hlsl.Options{ImmediatesTarget: &hlsl.BindTarget{Register: 3, Space: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if want := "ConstantBuffer<Im> im: register(b3, space2);"; !strings.Contains(src, want) {
		t.Errorf("output lacks %q:\n%s", want, src)
	}
	// A module without immediate data needs no target.
	if _, _, err := hlsl.Compile(lowerBoundsSource(t, `@fragment fn main() {}`), hlsl.Options{}); err != nil {
		t.Errorf("no immediate data: %v", err)
	}
}
