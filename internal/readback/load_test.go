package readback

import (
	"strings"
	"testing"
	"testing/fstest"
)

const validShader = "@group(0) @binding(0) var<storage, read_write> out: array<u32>;\n" +
	"@compute @workgroup_size(1) fn main() { out[0] = 1u; }\n"

func file(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }

func TestLoad(t *testing.T) {
	fsys := fstest.MapFS{
		"README.md":          file("# corpus"),
		"loops/b-rt.wgsl":    file(validShader),
		"loops/b-rt.json":    file(validSpec),
		"loops/a.wgsl":       file(validShader),
		"loops/a.json":       file(validSpec),
		"atomics/count.wgsl": file(validShader),
		"atomics/count.json": file(validSpec),
	}
	cases, err := Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	var names []string
	for _, c := range cases {
		names = append(names, c.Name)
	}
	if got, want := strings.Join(names, " "), "atomics/count loops/a loops/b-rt"; got != want {
		t.Fatalf("cases = %s, want %s (sorted)", got, want)
	}
	if cases[1].WGSL != validShader || cases[1].Spec.Entry != "main" {
		t.Errorf("case loops/a = %+v", cases[1])
	}
}

func TestLoadRejects(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"empty corpus", map[string]string{"README.md": "x"}, "no cases"},
		{"json without shader", map[string]string{"loops/a.json": validSpec}, "loops/a: no loops/a.wgsl"},
		{"shader without json", map[string]string{"loops/a.wgsl": validShader}, "loops/a: no loops/a.json"},
		{"file at the root", map[string]string{"a.wgsl": validShader, "a.json": validSpec}, "a.json: not in an area directory"},
		{"too deep", map[string]string{"loops/x/a.wgsl": validShader, "loops/x/a.json": validSpec}, "loops/x/a.json: not in an area directory"},
		{"other file", map[string]string{"loops/a.wgsl": validShader, "loops/a.json": validSpec, "loops/notes.txt": "x"}, "loops/notes.txt: not a .wgsl or .json file"},
		{"bad name", map[string]string{"loops/Loop_A.wgsl": validShader, "loops/Loop_A.json": validSpec}, "name"},
		{"bad area", map[string]string{"Loops/a.wgsl": validShader, "Loops/a.json": validSpec}, "name"},
		{"bad spec", map[string]string{"loops/a.wgsl": validShader, "loops/a.json": `{"format": 2}`}, "loops/a.json: format 2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := fstest.MapFS{}
			for name, data := range tt.files {
				fsys[name] = file(data)
			}
			_, err := Load(fsys)
			if err == nil {
				t.Fatal("Load accepted the corpus")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to contain %q", err, tt.want)
			}
		})
	}
}
