package dxil

import (
	"errors"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/dxil/internal/container"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

const immediatesShader = `
struct Im { scale: f32 }
var<immediate> im: Im;
@fragment fn main() -> @location(0) vec4<f32> { return vec4<f32>(im.scale); }`

func lowerImmediatesShader(t *testing.T) *ir.Module {
	t.Helper()
	tokens, err := wgsl.NewLexer(immediatesShader).Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	ast, err := wgsl.NewParser(tokens).Parse()
	if err != nil {
		t.Fatal(err)
	}
	m, err := wgsl.LowerWithSource(ast, immediatesShader)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// TestImmediatesTarget: the immediate data is the constant buffer at
// ImmediatesTarget. Without a target it is an error, FakeMissingBindings or
// not; before, it took a synthetic binding the PSV0 resource table left out.
func TestImmediatesTarget(t *testing.T) {
	m := lowerImmediatesShader(t)
	for _, fake := range []bool{false, true} {
		_, err := Compile(m, Options{FakeMissingBindings: fake})
		if !errors.Is(err, backend.ErrMissingBinding) {
			t.Errorf("FakeMissingBindings %v, no target: err = %v, want backend.ErrMissingBinding", fake, err)
		}
	}
	if _, err := Compile(m, Options{ImmediatesTarget: &BindTarget{Space: 2, Register: 3}}); err != nil {
		t.Fatal(err)
	}
}

// TestImmediatesPSV: the PSV0 resource table has the immediate data's
// constant buffer at ImmediatesTarget, as the emitter's metadata does.
func TestImmediatesPSV(t *testing.T) {
	m := lowerImmediatesShader(t)
	got := collectPSVResources(m, psvTargets{immediates: &BindTarget{Space: 2, Register: 3}})
	if len(got) != 1 {
		t.Fatalf("%d PSV0 resources, want 1: %+v", len(got), got)
	}
	if rb := got[0]; rb.ResType != container.PSVResTypeCBV || rb.Space != 2 || rb.LowerBound != 3 || rb.UpperBound != 3 {
		t.Errorf("PSV0 resource %+v, want a CBV at b3 in space 2", rb)
	}
}
