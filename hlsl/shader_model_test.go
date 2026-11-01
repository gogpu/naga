package hlsl_test

import (
	"errors"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/hlsl"
)

// TestShaderModelRefused: a feature above Options.ShaderModel is a
// *backend.VersionError naming the model it needs, as Rust naga's
// ShaderModelTooLow; the backend never raises the model. The view index
// (SV_ViewID) needs 6.1, and the zero model is 5.1.
func TestShaderModelRefused(t *testing.T) {
	m := lowerBoundsSource(t, `
@fragment fn main(@builtin(view_index) v: u32) -> @location(0) vec4<f32> { return vec4<f32>(f32(v)); }`)
	_, _, err := hlsl.Compile(m, hlsl.Options{})
	var vErr *backend.VersionError
	if !errors.As(err, &vErr) || vErr.Space != backend.VersionShaderModel ||
		vErr.Required != (backend.Version{Major: 6, Minor: 1}) || vErr.Requested != (backend.Version{Major: 5, Minor: 1}) {
		t.Fatalf("SM 5.1: err = %v, want a shader model VersionError requiring 6.1", err)
	}
	if want := "hlsl: the view index needs shader model 6.1, Options.ShaderModel is 5.1"; err.Error() != want {
		t.Errorf("SM 5.1: err = %q, want %q", err, want)
	}
	if _, _, err := hlsl.Compile(m, hlsl.Options{ShaderModel: backend.ShaderModel{Major: 6, Minor: 1}}); err != nil {
		t.Errorf("SM 6.1: %v", err)
	}
}
