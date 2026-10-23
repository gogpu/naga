package snapshot_test

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/msl"
)

// Rust serializes naga_types::VertexFormat with serde(rename_all =
// "lowercase") plus three explicit renames; TOML configs use those names.
// An unknown name must be an error: silently falling back to a default
// format would compile the shader with the wrong vertex fetches.
func TestParseVertexFormat(t *testing.T) {
	for _, tc := range []struct {
		name string
		want msl.VertexFormat
	}{
		{"uint8", msl.VertexFormatUint8},
		{"uint8x2", msl.VertexFormatUint8x2},
		{"snorm16x4", msl.VertexFormatSnorm16x4},
		{"float16x2", msl.VertexFormatFloat16x2},
		{"float32", msl.VertexFormatFloat32},
		{"float32x3", msl.VertexFormatFloat32x3},
		{"sint32x4", msl.VertexFormatSint32x4},
		{"unorm10-10-10-2", msl.VertexFormatUnorm10_10_10_2},
		{"unorm8x4-bgra", msl.VertexFormatUnorm8x4Bgra},
		{"snorm10-10-10-2", msl.VertexFormatSnorm10_10_10_2},
	} {
		got, err := parseVertexFormat(tc.name)
		if err != nil || got != tc.want {
			t.Errorf("parseVertexFormat(%q) = %v, %v; want %v", tc.name, got, err, tc.want)
		}
	}

	for _, tc := range []struct{ name, errContains string }{
		{"Uint8x2", "unknown vertex format"}, // pre-14328f49 spelling, not Rust's
		{"float", "unknown vertex format"},
		{"float64x2", "not supported by the msl package"},
	} {
		_, err := parseVertexFormat(tc.name)
		if err == nil || !strings.Contains(err.Error(), tc.errContains) {
			t.Errorf("parseVertexFormat(%q) error = %v, want one containing %q", tc.name, err, tc.errContains)
		}
	}
}

func TestParseAttributeArrayRejectsUnknownFormat(t *testing.T) {
	attrs, err := parseAttributeArray(`[ { offset = 4, shader_location = 1, format = "uint8x2" } ]`)
	if err != nil || len(attrs) != 1 || attrs[0].Format != msl.VertexFormatUint8x2 ||
		attrs[0].Offset != 4 || attrs[0].ShaderLocation != 1 {
		t.Errorf("parseAttributeArray = %+v, %v", attrs, err)
	}
	if _, err := parseAttributeArray(`[ { offset = 0, shader_location = 0, format = "Uint8x2" } ]`); err == nil {
		t.Error("parseAttributeArray accepted an unknown format")
	}
}
