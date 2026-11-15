package main

import (
	"bytes"
	"errors"
	"io/fs"
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
	if err := run([]string{"-not-a-flag"}, os.Stdout); err == nil {
		t.Error("run with an unknown flag succeeded, want the flag error")
	}
}

func TestGenerateMissingInputDir(t *testing.T) {
	err := generate("naga", filepath.Join(t.TempDir(), "absent"), t.TempDir(),
		func(_, _, _, _ string) (string, error) { return "", nil }, os.Stdout)
	if err == nil {
		t.Fatal("generate over a missing -in succeeded, want an error")
	}
}

func TestWriteSkippedError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent", "SKIPPED.md")
	if err := writeSkipped(path, nil); err == nil {
		t.Fatal("writeSkipped into a missing directory succeeded, want an error")
	}
}

func TestGenerate(t *testing.T) {
	in := t.TempDir()
	out := t.TempDir()
	for name, content := range map[string]string{
		"a.vert": "#version 450 core\n",
		"b.frag": "#version 450 core\n",
		"a.toml": "targets = \"MSL\"\n",
	} {
		if err := os.WriteFile(filepath.Join(in, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Output of an earlier run must not survive a regeneration.
	if err := os.MkdirAll(filepath.Join(out, "msl"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "msl", "glsl-old.vert.metal"), []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}

	var calls []string
	translate := func(_, stage, input, output string) (string, error) {
		base := filepath.Base(input)
		calls = append(calls, stage+":"+base)
		// One refusal, with a terminal color and a second line: SKIPPED.md
		// must keep the first line, without the escape.
		if base == "b.frag" && strings.HasSuffix(output, ".hlsl") {
			return "\x1b[0merror: nope\nmore\n", errors.New("exit 1")
		}
		return "", os.WriteFile(output, []byte("out "+base+" "+filepath.Ext(output)+"\n"), 0o600)
	}

	var stdout bytes.Buffer
	if err := generate("naga", in, out, translate, &stdout); err != nil {
		t.Fatal(err)
	}

	for rel, want := range map[string]string{
		"msl/glsl-a.vert.metal": "out a.vert .metal\n",
		"msl/glsl-b.frag.metal": "out b.frag .metal\n",
		"hlsl/glsl-a.vert.hlsl": "out a.vert .hlsl\n",
		"spv/glsl-a.vert.spv":   "out a.vert .spv\n",
		"spv/glsl-b.frag.spv":   "out b.frag .spv\n",
	} {
		got, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(rel)))
		if err != nil {
			t.Errorf("%s: %v", rel, err)
			continue
		}
		if string(got) != want {
			t.Errorf("%s = %q, want %q", rel, got, want)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "hlsl", "glsl-b.frag.hlsl")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("refused output exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "msl", "glsl-old.vert.metal")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("stale output survived regeneration: %v", err)
	}

	skipped, err := os.ReadFile(filepath.Join(out, "SKIPPED.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(skipped), "| `b.frag` | hlsl | error: nope |") {
		t.Errorf("SKIPPED.md does not record the refusal without its escape:\n%s", skipped)
	}
	if strings.Contains(string(skipped), "\x1b[") {
		t.Errorf("SKIPPED.md kept a terminal escape:\n%s", skipped)
	}

	if got := stdout.String(); !strings.Contains(got, "generated 5 outputs for 2 shaders; 1 skipped") {
		t.Errorf("summary = %q", got)
	}
	if len(calls) != 6 {
		t.Errorf("translator called %d times, want 2 shaders x 3 backends: %v", len(calls), calls)
	}
	for _, call := range calls {
		if !strings.HasPrefix(call, "vert:a.vert") && !strings.HasPrefix(call, "frag:b.frag") {
			t.Errorf("translator called with %q, want the stage of the source", call)
		}
	}
}

func TestGenerateWithoutInputs(t *testing.T) {
	err := generate("naga", t.TempDir(), t.TempDir(),
		func(_, _, _, _ string) (string, error) { return "", nil }, os.Stdout)
	if err == nil || !strings.Contains(err.Error(), "no .vert/.frag/.comp inputs") {
		t.Fatalf("err = %v, want a refusal mentioning the missing inputs", err)
	}
}

func TestGenerateFailsToCreateOutTree(t *testing.T) {
	in := t.TempDir()
	if err := os.WriteFile(filepath.Join(in, "a.vert"), []byte("void main() {}"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(out, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := generate("naga", in, out,
		func(_, _, _, _ string) (string, error) { return "", nil }, os.Stdout)
	if err == nil {
		t.Fatal("generate over a file as -out succeeded, want an error")
	}
}

func TestTranslateNaga(t *testing.T) {
	old := command
	defer func() { command = old }()

	var gotName string
	var gotArgs []string
	command = func(name string, args ...string) (string, error) {
		gotName, gotArgs = name, args
		return "\x1b[0merror: nope\nmore\n", errors.New("exit 1")
	}
	stderr, err := translateNaga("naga-cli", "frag", "in.frag", "out.metal")
	if err == nil || firstError(stderr) != "error: nope" {
		t.Fatalf("translateNaga = %q, %v; want the process error and its stderr", stderr, err)
	}
	if gotName != "naga-cli" {
		t.Errorf("command name = %q, want naga-cli", gotName)
	}
	want := []string{"--input-kind", "glsl", "--shader-stage", "frag", "--compact", "in.frag", "out.metal"}
	if strings.Join(gotArgs, " ") != strings.Join(want, " ") {
		t.Errorf("command args = %v, want %v", gotArgs, want)
	}

	command = func(_ string, _ ...string) (string, error) { return "", nil }
	if stderr, err := translateNaga("naga-cli", "vert", "in.vert", "out.metal"); err != nil || stderr != "" {
		t.Errorf("translateNaga on success = %q, %v; want no error and no stderr", stderr, err)
	}
}

func TestCommand(t *testing.T) {
	// The test binary is a real executable on every platform: given the
	// generator's arguments it rejects the unknown flags and exits non-zero,
	// which exercises the process launch itself.
	stderr, err := command(os.Args[0],
		"--input-kind", "glsl", "--shader-stage", "frag", "--compact", "in", "out")
	if err == nil {
		t.Fatal("the test binary accepted the generator's arguments, want a launch error")
	}
	if stderr == "" {
		t.Error("command returned no stderr for a failing process")
	}
}

func TestRunWithFakeTranslator(t *testing.T) {
	in := t.TempDir()
	out := t.TempDir()
	if err := os.WriteFile(filepath.Join(in, "a.vert"), []byte("void main() {}"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := translateImpl
	translateImpl = func(_, stage, _, output string) (string, error) {
		return "", os.WriteFile(output, []byte(stage), 0o600)
	}
	defer func() { translateImpl = old }()

	var stdout bytes.Buffer
	if err := run([]string{"-naga", "naga", "-in", in, "-out", out}, &stdout); err != nil {
		t.Fatal(err)
	}
	if got := stdout.String(); !strings.Contains(got, "generated 3 outputs for 1 shaders") {
		t.Errorf("summary = %q", got)
	}
	for _, rel := range []string{"msl/glsl-a.vert.metal", "hlsl/glsl-a.vert.hlsl", "spv/glsl-a.vert.spv"} {
		got, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(rel)))
		if err != nil || string(got) != "vert" {
			t.Errorf("%s = %q, %v; want the stage written by the fake translator", rel, got, err)
		}
	}
}
