package readback

import (
	"strings"
	"testing"
)

// validSpec is a minimal case that every rule accepts. The table below
// breaks it one field at a time.
const validSpec = `{
  "format": 1,
  "desc": "writes 1..4",
  "entry": "main",
  "workgroups": [1, 1, 1],
  "bindings": [
    {"binding": 0, "kind": "storage-read", "words": [3, "0x4"]},
    {"binding": 1, "kind": "storage-rw", "len": 3, "fill": "0xDEADBEEF",
     "expect": [[1, 2, "fill"], [1, 2, 3]]}
  ]
}`

func TestParseSpecValid(t *testing.T) {
	s, err := ParseSpec([]byte(validSpec))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	if s.Format != FormatVersion || s.Entry != "main" || len(s.Bindings) != 2 {
		t.Fatalf("spec = %+v", s)
	}
	in := s.Bindings[0]
	if got := in.Initial(); len(got) != 2 || got[0] != 3 || got[1] != 4 {
		t.Errorf("binding 0 initial = %v, want [3 4]", got)
	}
	out := s.Bindings[1]
	if got := out.Initial(); len(got) != 3 || got[0] != 0xDEADBEEF || got[2] != 0xDEADBEEF {
		t.Errorf("binding 1 initial = %#x, want three sentinels", got)
	}
	if out.Size() != 3 {
		t.Errorf("binding 1 size = %d, want 3", out.Size())
	}
	want := out.Expected()
	if len(want) != 2 || want[0][2] != 0xDEADBEEF || want[1][2] != 3 {
		t.Errorf("binding 1 expected = %#x", want)
	}
}

func TestParseSpecKnownFailures(t *testing.T) {
	src := strings.Replace(validSpec, `"bindings"`, `"requires": ["shader-f16"],
  "constants": {"k": 3, "7": 1.5},
  "knownFailures": [
    {"backend": "dx12-dxil", "card": "BUG-LOOP-VAR-REINIT-001", "reason": "loop-local var not reset"},
    {"backend": "gles", "adapter": "llvmpipe", "driver": "Mesa", "card": "gogpu/wgpu:P1-INFRA-001", "reason": "driver"}
  ],
  "bindings"`, 1)
	s, err := ParseSpec([]byte(src))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	if len(s.KnownFailures) != 2 || s.KnownFailures[1].Backend != BackendGLES || s.Constants["7"] != 1.5 {
		t.Fatalf("spec = %+v", s)
	}
}

// TestParseSpecRejects lists every rule of format 1: each source breaks one.
func TestParseSpecRejects(t *testing.T) {
	tests := []struct {
		name string
		old  string // replaced in validSpec
		new  string
		want string // substring of the error
	}{
		{"no format", `"format": 1,`, ``, "format 0"},
		{"newer format", `"format": 1,`, `"format": 2,`, "format 2"},
		{"unknown field", `"entry"`, `"entyr": "x", "entry"`, `unknown field "entyr"`},
		{"no desc", `"desc": "writes 1..4",`, ``, "desc"},
		{"no entry", `"entry": "main",`, ``, "entry"},
		{"two workgroup dims", `[1, 1, 1]`, `[1, 1]`, "workgroups"},
		{"zero workgroups", `[1, 1, 1]`, `[1, 0, 1]`, "workgroups"},
		{"words and len", `"words": [3, "0x4"]`, `"words": [3], "len": 1`, "both words and len"},
		{"neither words nor len", `"kind": "storage-read", "words": [3, "0x4"]`, `"kind": "storage-read"`, "words or len"},
		{"fill with words", `"words": [3, "0x4"]`, `"words": [3], "fill": 1`, "fill needs len"},
		{"expect on read-only", `"words": [3, "0x4"]`, `"words": [3], "expect": [[3]]`, "expect on storage-read"},
		{"expect length", `[1, 2, 3]]`, `[1, 2]]`, "2 words, binding has 3"},
		{"fill token without fill", `"len": 3, "fill": "0xDEADBEEF",`, `"len": 3,`, `"fill" without a fill`},
		{"word too large", `[3, "0x4"]`, `[4294967296]`, "word"},
		{"negative word", `[3, "0x4"]`, `[-1]`, "word"},
		{"fractional word", `[3, "0x4"]`, `[1.5]`, "word"},
		{"bad hex word", `[3, "0x4"]`, `["0x1G"]`, "word"},
		{"wide hex word", `[3, "0x4"]`, `["0x100000000"]`, "word"},
		{"fill token in words", `[3, "0x4"]`, `["fill"]`, "word"},
		{"duplicate binding", `{"binding": 1,`, `{"binding": 0,`, "binding 0 twice"},
		{"unknown kind", `"storage-read"`, `"storage"`, `kind "storage"`},
		{"bad type", `"kind": "storage-read",`, `"kind": "storage-read", "type": "u64",`, `type "u64"`},
		{"nothing checked", `,
     "expect": [[1, 2, "fill"], [1, 2, 3]]`, ``, "no binding has expect"},
		{"empty expect", `[[1, 2, "fill"], [1, 2, 3]]`, `[]`, "expect is empty"},
		{"unknown requirement", `"bindings"`, `"requires": ["subgroups"], "bindings"`, `requires "subgroups"`},
		{"empty constant key", `"bindings"`, `"constants": {"": 1}, "bindings"`, "constant name"},
		{"unknown backend", `"bindings"`, `"knownFailures": [{"backend": "d3d12", "card": "BUG-X-001", "reason": "r"}], "bindings"`, `backend "d3d12"`},
		{"no card", `"bindings"`, `"knownFailures": [{"backend": "gles", "reason": "r"}], "bindings"`, "card"},
		{"bad card", `"bindings"`, `"knownFailures": [{"backend": "gles", "card": "see wgpu#368", "reason": "r"}], "bindings"`, "card"},
		{"no reason", `"bindings"`, `"knownFailures": [{"backend": "gles", "card": "BUG-X-001"}], "bindings"`, "reason"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := strings.Replace(validSpec, tt.old, tt.new, 1)
			if src == validSpec {
				t.Fatalf("replacement %q did not apply", tt.old)
			}
			_, err := ParseSpec([]byte(src))
			if err == nil {
				t.Fatalf("ParseSpec accepted:\n%s", src)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestParseSpecNoBindings(t *testing.T) {
	_, err := ParseSpec([]byte(`{"format": 1, "desc": "d", "entry": "main", "workgroups": [1, 1, 1]}`))
	if err == nil || !strings.Contains(err.Error(), "no bindings") {
		t.Fatalf("error = %v, want \"no bindings\"", err)
	}
}

func TestParseSpecTrailingData(t *testing.T) {
	if _, err := ParseSpec([]byte(validSpec + `{}`)); err == nil {
		t.Fatal("ParseSpec accepted a second JSON value after the case")
	}
}

func TestKnownFailureAllAdapters(t *testing.T) {
	tests := []struct {
		kf   KnownFailure
		want bool
	}{
		{KnownFailure{Backend: BackendGLES}, true},
		{KnownFailure{Backend: BackendGLES, Adapter: "Intel"}, false},
		{KnownFailure{Backend: BackendGLES, Driver: "Mesa"}, false},
	}
	for _, tt := range tests {
		if got := tt.kf.AllAdapters(); got != tt.want {
			t.Errorf("%+v.AllAdapters() = %v, want %v", tt.kf, got, tt.want)
		}
	}
}
