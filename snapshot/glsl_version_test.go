package snapshot_test

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/glsl"
)

// naga::back::glsl::Version is Desktop(u16) | Embedded { version, is_webgl };
// the corpus TOML spells it four ways. Every spelling must be read, and an
// unrecognized one must be an error rather than the ES 310 default.
func TestParseRustGLSLVersion(t *testing.T) {
	desktop := func(v uint8, minor uint8) glsl.Version { return glsl.Version{Major: v, Minor: minor} }
	es := func(v uint8, minor uint8) glsl.Version { return glsl.Version{Major: v, Minor: minor, ES: true} }
	for _, tc := range []struct {
		name, toml string
		want       glsl.Version
		webgl      bool
	}{
		{"dotted desktop", "[glsl]\nversion.Desktop = 430\n", desktop(4, 30), false},
		{"inline desktop", "[glsl]\nversion = { Desktop = 430 }\n", desktop(4, 30), false},
		{"dotted embedded table", "[glsl]\nversion.Embedded = { is_webgl = true, version = 300 }\n", es(3, 0), true},
		{"dotted embedded keys", "[glsl]\nversion.Embedded.is_webgl = false\nversion.Embedded.version = 320\n", es(3, 20), false},
	} {
		got, webgl, ok, err := parseRustGLSLVersion(tc.toml)
		if err != nil || !ok || got != tc.want || webgl != tc.webgl {
			t.Errorf("%s: got %+v webgl=%v ok=%v err=%v; want %+v webgl=%v", tc.name, got, webgl, ok, err, tc.want, tc.webgl)
		}
	}

	if _, _, ok, err := parseRustGLSLVersion("[glsl]\nwriter_flags = \"\"\n"); ok || err != nil {
		t.Errorf("no version: ok=%v err=%v, want not set and no error", ok, err)
	}
	for _, bad := range []string{
		"[glsl]\nversion = { Desktop = \"430\" }\n",
		"[glsl]\nversion = { Mobile = 300 }\n",
		"[glsl]\nversion.Embedded.version = 320\n", // is_webgl is required
	} {
		if _, _, _, err := parseRustGLSLVersion(bad); err == nil || !strings.Contains(err.Error(), "version") {
			t.Errorf("%q: error = %v, want a version error", bad, err)
		}
	}
}
