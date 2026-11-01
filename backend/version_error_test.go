package backend_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gogpu/naga/backend"
)

func TestVersionSpaceString(t *testing.T) {
	for _, tc := range []struct {
		space backend.VersionSpace
		want  string
	}{
		{backend.VersionSPIRV, "SPIR-V"},
		{backend.VersionShaderModel, "shader model"},
		{backend.VersionMSL, "MSL"},
		{0, "VersionSpace(0)"},
		{9, "VersionSpace(9)"},
	} {
		if got := tc.space.String(); got != tc.want {
			t.Errorf("VersionSpace(%d).String() = %q, want %q", uint8(tc.space), got, tc.want)
		}
	}
}

func TestVersionString(t *testing.T) {
	for _, tc := range []struct {
		v    backend.Version
		want string
	}{
		{backend.Version{}, "0.0"},
		{backend.Version{Major: 6, Minor: 5}, "6.5"},
		{backend.Version{Major: 1, Minor: 3}, "1.3"},
		{backend.Version{Major: 10, Minor: 12}, "10.12"},
	} {
		if got := tc.v.String(); got != tc.want {
			t.Errorf("%#v.String() = %q, want %q", tc.v, got, tc.want)
		}
	}
}

// One message shape for every backend, naming the option the caller sets:
// Options.Version (spirv), Options.ShaderModel (hlsl, dxil) and
// Options.LangVersion (msl).
func TestVersionErrorText(t *testing.T) {
	for _, tc := range []struct {
		err  backend.VersionError
		want string
	}{
		{
			backend.VersionError{Space: backend.VersionSPIRV, Feature: "a subgroup operation",
				Required: backend.Version{Major: 1, Minor: 3}, Requested: backend.Version{Major: 1, Minor: 0}},
			"a subgroup operation needs SPIR-V 1.3, Options.Version is 1.0",
		},
		{
			backend.VersionError{Space: backend.VersionShaderModel, Feature: "a mesh or task shader",
				Required: backend.Version{Major: 6, Minor: 5}, Requested: backend.Version{Major: 6, Minor: 0}},
			"a mesh or task shader needs shader model 6.5, Options.ShaderModel is 6.0",
		},
		{
			backend.VersionError{Space: backend.VersionMSL, Feature: "a ray query",
				Required: backend.Version{Major: 2, Minor: 4}, Requested: backend.Version{Major: 2, Minor: 1}},
			"a ray query needs MSL 2.4, Options.LangVersion is 2.1",
		},
	} {
		if got := tc.err.Error(); got != tc.want {
			t.Errorf("Error() = %q, want %q", got, tc.want)
		}
	}
}

// A VersionError is an ErrUnsupported, and errors.As finds it through the
// backends' "<backend>: %w" wrapping.
func TestVersionErrorUnwrap(t *testing.T) {
	inner := &backend.VersionError{Space: backend.VersionMSL, Feature: "@invariant",
		Required: backend.Version{Major: 2, Minor: 1}, Requested: backend.Version{Major: 1, Minor: 2}}
	err := fmt.Errorf("msl: %w", inner)
	if !errors.Is(err, backend.ErrUnsupported) {
		t.Errorf("errors.Is(%v, ErrUnsupported) = false, want true", err)
	}
	var vErr *backend.VersionError
	if !errors.As(err, &vErr) {
		t.Fatalf("errors.As(%v, *VersionError) = false, want true", err)
	}
	if vErr != inner {
		t.Errorf("errors.As found %p, want %p", vErr, inner)
	}
	if errors.Is(err, backend.ErrEntryPointNotFound) || errors.Is(err, backend.ErrMissingBinding) {
		t.Errorf("%v matches another error category", err)
	}
}
