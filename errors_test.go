package naga

import (
	"errors"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
)

// TestRefusalsAreErrUnsupported: every backend's refusal of what its target
// cannot express matches backend.ErrUnsupported (ADR-010 N9), whatever its
// own error type, and other errors do not.
func TestRefusalsAreErrUnsupported(t *testing.T) {
	drawIndex := lowerShader(t, drawIndexShader)
	bary := lowerShader(t, barycentricsShader)
	for _, tc := range []struct {
		name string
		err  func() error
	}{
		{"spirv option", func() error {
			_, _, err := spirv.Compile(drawIndex, spirv.Options{ZeroInitializeWorkgroupMemory: 99})
			return err
		}},
		{"msl built-in", func() error { _, _, err := msl.Compile(drawIndex, msl.Options{}); return err }},
		{"glsl version (MissingFeaturesError)", func() error { _, _, err := glsl.Compile(drawIndex, glsl.Options{}); return err }},
		{"hlsl built-in", func() error { _, _, err := hlsl.Compile(drawIndex, hlsl.Options{}); return err }},
		{"hlsl shader model (ShaderModelError)", func() error { _, _, err := hlsl.Compile(bary, hlsl.Options{}); return err }},
		{"dxil built-in", func() error { _, err := dxilBlob(drawIndex, dxil.Options{}); return err }},
	} {
		err := tc.err()
		if !errors.Is(err, backend.ErrUnsupported) {
			t.Errorf("%s: error %v (%T) does not match backend.ErrUnsupported", tc.name, err, err)
		}
	}

	partial := map[ir.ResourceBinding]spirv.BindingInfo{{Group: 0, Binding: 0}: {DescriptorSet: 0, Binding: 0}}
	_, err := spirvBinary(bindingModule(t), spirv.Options{Version: spirv.Version1_3, BindingMap: partial})
	if !errors.Is(err, backend.ErrMissingBinding) || errors.Is(err, backend.ErrUnsupported) {
		t.Errorf("missing binding: error %v, want ErrMissingBinding and not ErrUnsupported", err)
	}
}
