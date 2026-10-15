package snapshot_test

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseRustParams(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    rustParams
	}{
		{
			name:    "absent targets use Rust's WGSL default",
			content: "[msl]\nlang_version = [2, 4]\n",
			want:    rustParams{targets: rustTargetsWGSLDefault},
		},
		{
			name:    "pipe-separated targets",
			content: `targets = "SPIRV | METAL | HLSL | WGSL"`,
			want:    rustParams{targets: rustTargetSPIRV | rustTargetMETAL | rustTargetHLSL | rustTargetWGSL},
		},
		{
			name:    "every flag",
			content: `targets = "IR | ANALYSIS | SPIRV | METAL | GLSL | DOT | HLSL | WGSL | NO_VALIDATION"`,
			want: rustParams{targets: rustTargetIR | rustTargetANALYSIS | rustTargetSPIRV | rustTargetMETAL |
				rustTargetGLSL | rustTargetDOT | rustTargetHLSL | rustTargetWGSL | rustTargetNoValidation},
		},
		{
			name:    "comment is not a declaration",
			content: "# targets = \"METAL\"\ntargets = \"GLSL\" # trailing comment\n",
			want:    rustParams{targets: rustTargetGLSL},
		},
		{
			name: "section keys do not leak to top level",
			content: "targets = \"SPIRV\"\n" +
				"[spv]\nseparate_entry_points = true\nversion = [1, 0]\n" +
				"[msl]\ntargets = \"METAL\"\n",
			want: rustParams{targets: rustTargetSPIRV, separateEntryPoints: true},
		},
		{
			name: "glsl exclude list and multi-line values",
			content: "pipeline_constants = { o = 2.0 }\n" +
				"glsl_exclude_list = [\"depth_load\", \"depth_no_comparison\"]\n" +
				"targets = \"GLSL\"\n" +
				"[hlsl]\nbinding_map = [\n  { bind_target = { register = 0 }, resource_binding = { group = 0, binding = 0 } },\n]\n" +
				"[spv]\nseparate_entry_points = false\n",
			want: rustParams{
				targets:         rustTargetGLSL,
				glslExcludeList: map[string]bool{"depth_load": true, "depth_no_comparison": true},
			},
		},
		{
			// naga-test parses and validates such a shader but writes no output.
			name:    "empty targets means no outputs",
			content: `targets = ""`,
			want:    rustParams{targets: 0},
		},
		{
			name: "arrays of tables are sections",
			content: "targets = \"HLSL\"\n" +
				"[[hlsl.binding_map]]\nbind_target = { register = 0 }\n" +
				"[[hlsl.binding_map]]\nbind_target = { register = 1 }\n" +
				"[spv]\nseparate_entry_points = true\n",
			want: rustParams{targets: rustTargetHLSL, separateEntryPoints: true},
		},
		{
			name:    "brackets inside strings do not open a value",
			content: "label = \"a [ b { c\"\ntargets = \"HLSL\"\n",
			want:    rustParams{targets: rustTargetHLSL},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRustParams(tt.content)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseRustParams() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseRustParamsRejectsMalformedInput(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{"unknown target flag", `targets = "SPIRV | VULKAN"`, "VULKAN"},
		{"empty target flag", `targets = "SPIRV | "`, "empty"},
		{"targets not a string", `targets = 3`, "targets"},
		{"duplicate key", "targets = \"SPIRV\"\ntargets = \"METAL\"\n", "duplicate"},
		{"separate_entry_points not a bool", "[spv]\nseparate_entry_points = \"yes\"\n", "separate_entry_points"},
		{"exclude list not strings", "glsl_exclude_list = [1, 2]\n", "glsl_exclude_list"},
		{"unterminated array", "glsl_exclude_list = [\"a\",\n", "unterminated"},
		{"line without key", "targets\n", "key = value"},
		{"unterminated section", "[spv\n", "section"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseRustParams(tt.content)
			if err == nil {
				t.Fatal("parseRustParams accepted malformed input")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to mention %q", err, tt.wantErr)
			}
		})
	}
}

func TestRustTargetsString(t *testing.T) {
	got := (rustTargetSPIRV | rustTargetMETAL | rustTargetGLSL).String()
	if got != "SPIRV | METAL | GLSL" {
		t.Errorf("String() = %q", got)
	}
}

// TestRustParamsOfEveryCorpusShader parses the parameters of every vendored
// shader: a malformed or unknown value must fail here, not silently change
// what the reference tests expect.
func TestRustParamsOfEveryCorpusShader(t *testing.T) {
	for _, shader := range loadCorpus(t) {
		if shader.origin != originRust {
			continue
		}
		if _, err := loadRustParams(shader.name); err != nil {
			t.Errorf("%s: %v", shader.name, err)
		}
	}
}
