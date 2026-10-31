package readback

import (
	"strings"
	"testing"
)

// bindShader declares one buffer of every kind; bindSpec matches it.
const bindShader = `
struct P { k: u32, pad: vec3<u32> }
@group(0) @binding(0) var<storage, read> input: array<u32>;
@group(0) @binding(1) var<storage, read_write> out: array<u32, 4>;
@group(0) @binding(2) var<uniform> params: P;
@compute @workgroup_size(1) fn main() { out[0] = input[0] + params.k; }
`

const bindSpec = `{
  "format": 1, "desc": "d", "entry": "main", "workgroups": [1, 1, 1],
  "bindings": [
    {"binding": 0, "kind": "storage-read", "words": [1]},
    {"binding": 1, "kind": "storage-rw", "len": 4, "expect": [[3, 0, 0, 0]]},
    {"binding": 2, "kind": "uniform", "words": [2, 0, 0, 0, 0, 0, 0, 0]}
  ]
}`

func check(t *testing.T, shader, spec string) error {
	t.Helper()
	s, err := ParseSpec([]byte(spec))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	c := &Case{Name: "test/case", WGSL: shader, Spec: s}
	m, err := Module(c)
	if err != nil {
		return err
	}
	return CheckModule(&c.Spec, m)
}

func TestCheckModule(t *testing.T) {
	if err := check(t, bindShader, bindSpec); err != nil {
		t.Fatalf("CheckModule: %v", err)
	}
}

func TestCheckModuleRejects(t *testing.T) {
	tests := []struct {
		name         string
		shader, spec string
		want         string
	}{
		{
			name:   "global without JSON binding",
			shader: bindShader,
			spec: strings.Replace(bindSpec, `,
    {"binding": 2, "kind": "uniform", "words": [2, 0, 0, 0, 0, 0, 0, 0]}`, ``, 1),
			want: "binding 2 (params): no JSON binding",
		},
		{
			name:   "JSON binding not in the shader",
			shader: bindShader,
			spec:   strings.Replace(bindSpec, `"words": [2, 0, 0, 0, 0, 0, 0, 0]}`, `"words": [2, 0, 0, 0, 0, 0, 0, 0]}, {"binding": 5, "kind": "storage-read", "words": [0]}`, 1),
			want:   "binding 5: no global of entry point main",
		},
		{
			name:   "kind mismatch",
			shader: bindShader,
			spec:   strings.Replace(bindSpec, `{"binding": 0, "kind": "storage-read"`, `{"binding": 0, "kind": "storage-rw"`, 1),
			want:   `binding 0 (input): kind "storage-rw", the shader declares "storage-read"`,
		},
		{
			name:   "buffer smaller than the type",
			shader: bindShader,
			spec:   strings.Replace(bindSpec, `"len": 4, "expect": [[3, 0, 0, 0]]`, `"len": 3, "expect": [[3, 0, 0]]`, 1),
			want:   "binding 1 (out): 12 bytes, the type needs 16",
		},
		{
			name:   "uniform smaller than the struct",
			shader: bindShader,
			spec:   strings.Replace(bindSpec, `"words": [2, 0, 0, 0, 0, 0, 0, 0]}`, `"words": [2]}`, 1),
			want:   "binding 2 (params): 4 bytes, the type needs 32",
		},
		{
			name:   "unused global",
			shader: strings.Replace(bindShader, "input[0] + params.k", "params.k", 1),
			spec:   bindSpec,
			want:   "binding 0: no global of entry point main",
		},
		{
			name: "other group",
			shader: "@group(1) @binding(0) var<storage, read_write> out: array<u32>;\n" +
				"@compute @workgroup_size(1) fn main() { out[0] = 1u; }",
			spec: `{"format": 1, "desc": "d", "entry": "main", "workgroups": [1, 1, 1],
  "bindings": [{"binding": 0, "kind": "storage-rw", "len": 1, "expect": [[1]]}]}`,
			want: "group 1: the corpus binds @group(0) only",
		},
		{
			name: "texture",
			shader: "@group(0) @binding(0) var<storage, read_write> out: array<f32>;\n" +
				"@group(0) @binding(1) var tex: texture_2d<f32>;\n" +
				"@compute @workgroup_size(1) fn main() { out[0] = textureLoad(tex, vec2(0), 0).x; }",
			spec: `{"format": 1, "desc": "d", "entry": "main", "workgroups": [1, 1, 1],
  "bindings": [{"binding": 0, "kind": "storage-rw", "len": 1, "expect": [[1]]}]}`,
			want: "binding 1 (tex): not a buffer",
		},
		{
			name:   "entry point missing",
			shader: bindShader,
			spec:   strings.Replace(bindSpec, `"entry": "main"`, `"entry": "other"`, 1),
			want:   "entry point",
		},
		{
			name: "entry point not compute",
			shader: "@group(0) @binding(0) var<storage, read_write> out: array<u32>;\n" +
				"@vertex fn main() -> @builtin(position) vec4<f32> { return vec4(f32(out[0])); }",
			spec: `{"format": 1, "desc": "d", "entry": "main", "workgroups": [1, 1, 1],
  "bindings": [{"binding": 0, "kind": "storage-rw", "len": 1, "expect": [[1]]}]}`,
			want: `entry point not found: "main" (compute stage)`,
		},
		{
			name:   "requires f16 without using it",
			shader: bindShader,
			spec:   strings.Replace(bindSpec, `"bindings"`, `"requires": ["shader-f16"], "bindings"`, 1),
			want:   "requires shader-f16, the shader uses no f16",
		},
		{
			name:   "f16 without requires",
			shader: f16Shader,
			spec:   `{"format": 1, "desc": "d", "entry": "main", "workgroups": [1, 1, 1], ` + f16Bindings,
			want:   "f16",
		},
		{
			name: "unknown pipeline constant",
			shader: "override k: u32 = 1u;\n@group(0) @binding(0) var<storage, read_write> out: array<u32>;\n" +
				"@compute @workgroup_size(1) fn main() { out[0] = k; }",
			spec: `{"format": 1, "desc": "d", "entry": "main", "workgroups": [1, 1, 1], "constants": {"kk": 2},
  "bindings": [{"binding": 0, "kind": "storage-rw", "len": 1, "expect": [[2]]}]}`,
			want: "kk",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.spec == bindSpec && tt.shader == bindShader {
				t.Fatal("test case changes nothing")
			}
			err := check(t, tt.shader, tt.spec)
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to contain %q", err, tt.want)
			}
		})
	}
}

// f16Shader uses f16 at run time: a folded `let h = 1.5h` leaves no f16 type
// in the module.
const f16Shader = "enable f16;\n" +
	"@group(0) @binding(0) var<storage, read> input: array<u32>;\n" +
	"@group(0) @binding(1) var<storage, read_write> out: array<u32>;\n" +
	"@compute @workgroup_size(1) fn main() { let h = f16(input[0]) + 0.5h; out[0] = u32(h * 2.0h); }"

const f16Bindings = `"bindings": [{"binding": 0, "kind": "storage-read", "words": [1]},
  {"binding": 1, "kind": "storage-rw", "len": 1, "expect": [[3]]}]}`

func TestModuleF16(t *testing.T) {
	spec := `{"format": 1, "desc": "d", "entry": "main", "workgroups": [1, 1, 1], "requires": ["shader-f16"], ` + f16Bindings
	if err := check(t, f16Shader, spec); err != nil {
		t.Fatalf("f16 case with requires: %v", err)
	}
}
