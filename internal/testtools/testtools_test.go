package testtools

import (
	"errors"
	"fmt"
	"testing"
)

// recorder captures how a helper ended a test.
type recorder struct{ fatal, skip, log []string }

func (r *recorder) Helper() {}
func (r *recorder) Fatalf(format string, args ...any) {
	r.fatal = append(r.fatal, fmt.Sprintf(format, args...))
}
func (r *recorder) Skipf(format string, args ...any) {
	r.skip = append(r.skip, fmt.Sprintf(format, args...))
}
func (r *recorder) Logf(format string, args ...any) {
	r.log = append(r.log, fmt.Sprintf(format, args...))
}

// withTools makes lookPath find exactly the given tools.
func withTools(t *testing.T, found ...string) {
	t.Helper()
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	lookPath = func(name string) (string, error) {
		for _, f := range found {
			if f == name {
				return "/usr/bin/" + name, nil
			}
		}
		return "", errors.New("not found")
	}
}

func TestRequired(t *testing.T) {
	tests := []struct {
		env, name string
		want      bool
	}{
		{"", "spirv-val", false},
		{"spirv-val", "spirv-val", true},
		{"spirv-dis, spirv-val", "spirv-val", true},
		{"spirv-dis", "spirv-val", false},
		{"all", "dxc", true},
		{"spirv-val-extra", "spirv-val", false},
	}
	for _, tt := range tests {
		t.Setenv(EnvRequired, tt.env)
		if got := Required(tt.name); got != tt.want {
			t.Errorf("%s=%q: Required(%q) = %v, want %v", EnvRequired, tt.env, tt.name, got, tt.want)
		}
	}
}

func TestLook(t *testing.T) {
	withTools(t, "spirv-val")

	t.Setenv(EnvRequired, "")
	var r recorder
	if got := Look(&r, "spirv-val", "install it"); got != "/usr/bin/spirv-val" || len(r.fatal)+len(r.skip) != 0 {
		t.Errorf("found tool: path %q, recorder %+v", got, r)
	}

	r = recorder{}
	Look(&r, "spirv-dis", "install it")
	if len(r.skip) != 1 || len(r.fatal) != 0 {
		t.Errorf("missing optional tool must skip: %+v", r)
	}

	t.Setenv(EnvRequired, "spirv-dis")
	r = recorder{}
	Look(&r, "spirv-dis", "install it")
	if len(r.fatal) != 1 || len(r.skip) != 0 {
		t.Errorf("missing required tool must fail: %+v", r)
	}
}

func TestOptional(t *testing.T) {
	withTools(t)

	t.Setenv(EnvRequired, "")
	var r recorder
	if _, ok := Optional(&r, "spirv-val", "install it"); ok || len(r.log) != 1 || len(r.fatal)+len(r.skip) != 0 {
		t.Errorf("missing optional tool must log and continue: ok=%v %+v", ok, r)
	}

	t.Setenv(EnvRequired, "all")
	r = recorder{}
	if _, ok := Optional(&r, "spirv-val", "install it"); ok || len(r.fatal) != 1 {
		t.Errorf("missing required tool must fail: ok=%v %+v", ok, r)
	}
}

func TestMissing(t *testing.T) {
	t.Setenv(EnvRequired, "")
	var r recorder
	Missing(&r, "dxc", "install the Windows SDK")
	if len(r.skip) != 1 {
		t.Errorf("Missing on an optional tool must skip: %+v", r)
	}
	t.Setenv(EnvRequired, "dxc")
	r = recorder{}
	Missing(&r, "dxc", "install the Windows SDK")
	if len(r.fatal) != 1 {
		t.Errorf("Missing on a required tool must fail: %+v", r)
	}
}
