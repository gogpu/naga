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
		{"msl version (VersionError)", func() error { _, _, err := msl.Compile(bary, msl.Options{}); return err }},
		{"glsl version (MissingFeaturesError)", func() error { _, _, err := glsl.Compile(drawIndex, glsl.Options{}); return err }},
		{"hlsl built-in", func() error { _, _, err := hlsl.Compile(drawIndex, hlsl.Options{}); return err }},
		{"hlsl shader model (VersionError)", func() error { _, _, err := hlsl.Compile(bary, hlsl.Options{}); return err }},
		{"dxil built-in", func() error { _, err := dxilBlob(drawIndex, dxil.Options{}); return err }},
	} {
		err := tc.err()
		if !errors.Is(err, backend.ErrUnsupported) {
			t.Errorf("%s: error %v (%T) does not match backend.ErrUnsupported", tc.name, err, err)
		}
	}

	// msl.Compile refuses a feature above Options.LangVersion, whose zero
	// value is 2.1, with the shared version error.
	_, _, err := msl.Compile(bary, msl.Options{})
	var vErr *backend.VersionError
	if !errors.As(err, &vErr) || vErr.Space != backend.VersionMSL ||
		err.Error() != "msl: @builtin(barycentric) needs MSL 2.3, Options.LangVersion is 2.1" {
		t.Errorf("msl barycentrics at the zero version: error %v, want an MSL VersionError for 2.3", err)
	}

	partial := map[ir.ResourceBinding]spirv.BindingInfo{{Group: 0, Binding: 0}: {DescriptorSet: 0, Binding: 0}}
	_, err = spirvBinary(bindingModule(t), spirv.Options{Version: spirv.Version1_3, BindingMap: partial})
	if !errors.Is(err, backend.ErrMissingBinding) || errors.Is(err, backend.ErrUnsupported) {
		t.Errorf("missing binding: error %v, want ErrMissingBinding and not ErrUnsupported", err)
	}
}
