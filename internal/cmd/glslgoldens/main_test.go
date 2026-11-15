package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestStageOf(t *testing.T) {
	tests := []struct {
		name  string
		stage string
		ok    bool
	}{
		{"a.vert", "vert", true},
		{"a.frag", "frag", true},
		{"a.comp", "comp", true},
		{"a.VERT", "vert", true},
		{"a.toml", "", false},
		{"a.frag.txt", "", false},
		{"a", "", false},
	}
	for _, tt := range tests {
		stage, ok := stageOf(tt.name)
		if stage != tt.stage || ok != tt.ok {
			t.Errorf("stageOf(%q) = %q, %v; want %q, %v", tt.name, stage, ok, tt.stage, tt.ok)
		}
	}
}

func TestFirstError(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
		want   string
	}{
		{"empty", "", "(no message on stderr)"},
		{"blank lines", "\n  \n", "(no message on stderr)"},
		{"first line", "error: bad\nmore\n", "error: bad"},
		{"ansi", "\x1b[0m\x1b[1m\x1b[38;5;9merror\x1b[0m: bad\n", "error: bad"},
		{"leading blank", "\n\n  error: later  \n", "error: later"},
	}
	for _, tt := range tests {
		if got := firstError(tt.stderr); got != tt.want {
			t.Errorf("%s: firstError(%q) = %q, want %q", tt.name, tt.stderr, got, tt.want)
		}
	}
}

func TestFirstErrorTruncates(t *testing.T) {
	got := firstError(strings.Repeat("x", 300))
	if utf8.RuneCountInString(got) != 201 || !strings.HasSuffix(got, "…") ||
		!strings.HasPrefix(got, strings.Repeat("x", 200)) {
		t.Errorf("firstError(long) = %d runes ending %q, want 200 runes and an ellipsis", utf8.RuneCountInString(got), got[len(got)-1:])
	}
}

func TestGLSLInputs(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"b.frag":  "void main() {}",
		"a.vert":  "void main() {}",
		"c.comp":  "void main() {}",
		"c.toml":  "targets = \"MSL\"",
		"note.md": "not a shader",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "sub.frag"), 0o750); err != nil {
		t.Fatal(err)
	}

	inputs, err := glslInputs(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, in := range inputs {
		names = append(names, filepath.Base(in))
	}
	if got := strings.Join(names, ","); got != "a.vert,b.frag,c.comp" {
		t.Errorf("glslInputs = %s, want the three sources sorted, without toml/md/directories", got)
	}
}

func TestWriteSkipped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SKIPPED.md")
	skips := []skip{
		{Shader: "z.frag", Backend: "msl", Error: "no | support"},
		{Shader: "a.vert", Backend: "hlsl", Error: "bad thing"},
	}
	if err := writeSkipped(path, skips); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	a := strings.Index(text, "`a.vert`")
	z := strings.Index(text, "`z.frag`")
	if a < 0 || z < 0 || a > z {
		t.Errorf("SKIPPED.md does not list a.vert before z.frag:\n%s", text)
	}
	if !strings.Contains(text, `no \| support`) {
		t.Errorf("SKIPPED.md does not escape the table pipe:\n%s", text)
	}

	if err := writeSkipped(path, nil); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "No skips") || strings.Contains(string(data), "| Shader |") {
		t.Errorf("SKIPPED.md without skips:\n%s", data)
	}
}

func TestRunRequiresFlags(t *testing.T) {
	if err := run(nil, os.Stdout); err == nil {
		t.Error("run without flags succeeded, want an error")
	}
	if err := run([]string{"-naga", "naga", "-in", "in"}, os.Stdout); err == nil {
		t.Error("run without -out succeeded, want an error")
	}
}
