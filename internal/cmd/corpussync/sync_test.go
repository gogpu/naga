package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogpu/naga/internal/testcorpus"
	"github.com/gogpu/naga/internal/testtools"
)

// fakeUpstream builds a git repository shaped like gfx-rs/wgpu with the given
// files (slash-separated path → content), commits them and returns its path.
func fakeUpstream(t *testing.T, files map[string]string) string {
	t.Helper()
	testtools.Look(t, "git", "install git")
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	// Store content exactly as written, whatever the machine's settings.
	runGit(t, dir, "config", "core.autocrlf", "false")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "test")
	writeFiles(t, dir, files)
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "init")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func upstreamFiles() map[string]string {
	return map[string]string{
		"LICENSE.MIT":    "mit\n",
		"LICENSE.APACHE": "apache\n",
		// CRLF must survive byte for byte.
		"naga/tests/in/wgsl/a.wgsl":                    "fn a() {}\r\n",
		"naga/tests/in/wgsl/a.toml":                    "targets = \"SPIRV\"\n",
		"naga/tests/out/spv/wgsl-a.spvasm":             "; spv\n",
		"naga/tests/out/msl/wgsl-a.metal":              "// msl\n",
		"naga/tests/out/hlsl/wgsl-a.hlsl":              "// hlsl\n",
		"naga/tests/out/glsl/wgsl-a.main.Compute.glsl": "// glsl\n",
		"naga/tests/out/ir/wgsl-a.ron":                 "(ir)\n",
		"naga/tests/out/ir/wgsl-a.compact.ron":         "(compact)\n",
		// Not selected: non-WGSL sources, other output kinds, other inputs.
		"naga/tests/out/spv/spv-b.spvasm":         "; from spv input\n",
		"naga/tests/out/wgsl/wgsl-a.wgsl":         "// wgsl roundtrip\n",
		"naga/tests/out/analysis/wgsl-a.info.ron": "(info)\n",
		"naga/tests/in/glsl/c.vert":               "void main() {}\n",
		"naga/src/lib.rs":                         "// code\n",
	}
}

func TestSyncCopiesSelectedFilesByteForByte(t *testing.T) {
	upstream := fakeUpstream(t, upstreamFiles())
	out := t.TempDir()

	report, err := syncCorpus(upstream, out)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"LICENSE.MIT":                       "mit\n",
		"LICENSE.APACHE":                    "apache\n",
		"in/wgsl/a.wgsl":                    "fn a() {}\r\n",
		"in/wgsl/a.toml":                    "targets = \"SPIRV\"\n",
		"out/spv/wgsl-a.spvasm":             "; spv\n",
		"out/msl/wgsl-a.metal":              "// msl\n",
		"out/hlsl/wgsl-a.hlsl":              "// hlsl\n",
		"out/glsl/wgsl-a.main.Compute.glsl": "// glsl\n",
		"out/ir/wgsl-a.ron":                 "(ir)\n",
		"out/ir/wgsl-a.compact.ron":         "(compact)\n",
	}
	m, err := testcorpus.Load(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Files) != len(want) {
		var got []string
		for _, f := range m.Files {
			got = append(got, f.Path)
		}
		t.Fatalf("manifest lists %v, want %d files", got, len(want))
	}
	for rel, content := range want {
		got, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(rel)))
		if err != nil {
			t.Errorf("%s: %v", rel, err)
			continue
		}
		if !bytes.Equal(got, []byte(content)) {
			t.Errorf("%s = %q, want %q", rel, got, content)
		}
	}

	head := runGit(t, upstream, "rev-parse", "HEAD")
	if m.Kind != testcorpus.KindVendored || m.Source.Commit != head || m.Source.CommitDate == "" ||
		m.Source.Repository != upstreamRepository {
		t.Errorf("source = %+v kind = %s, want vendored from %s at %s", m.Source, m.Kind, upstreamRepository, head)
	}
	if len(report.Added) != len(want) || len(report.Removed) != 0 || len(report.Changed) != 0 {
		t.Errorf("report = %+v, want %d added", report, len(want))
	}
	if problems, err := testcorpus.Verify(out); err != nil || len(problems) != 0 {
		t.Errorf("Verify after sync: %v %v", problems, err)
	}
}

func TestSyncReplacesStaleFilesAndKeepsReadme(t *testing.T) {
	files := upstreamFiles()
	upstream := fakeUpstream(t, files)
	out := t.TempDir()
	if _, err := syncCorpus(upstream, out); err != nil {
		t.Fatal(err)
	}
	writeFiles(t, out, map[string]string{testcorpus.ReadmeName: "ours\n"})

	// Upstream: one output changes, one input is removed, one is added.
	writeFiles(t, upstream, map[string]string{
		"naga/tests/out/spv/wgsl-a.spvasm": "; spv v2\n",
		"naga/tests/in/wgsl/new.wgsl":      "fn new() {}\n",
	})
	runGit(t, upstream, "rm", "-q", "naga/tests/in/wgsl/a.toml")
	runGit(t, upstream, "add", "-A")
	runGit(t, upstream, "commit", "-q", "-m", "update")

	report, err := syncCorpus(upstream, out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(report.Added, ",") != "in/wgsl/new.wgsl" ||
		strings.Join(report.Removed, ",") != "in/wgsl/a.toml" ||
		strings.Join(report.Changed, ",") != "out/spv/wgsl-a.spvasm" {
		t.Errorf("report = %+v", report)
	}
	if _, err := os.Stat(filepath.Join(out, "in", "wgsl", "a.toml")); !os.IsNotExist(err) {
		t.Errorf("stale a.toml still present: %v", err)
	}
	readme, err := os.ReadFile(filepath.Join(out, testcorpus.ReadmeName))
	if err != nil || string(readme) != "ours\n" {
		t.Errorf("README.md not preserved: %q %v", readme, err)
	}
	if problems, err := testcorpus.Verify(out); err != nil || len(problems) != 0 {
		t.Errorf("Verify after re-sync: %v %v", problems, err)
	}
}

func TestSyncRefusesDirtyUpstream(t *testing.T) {
	upstream := fakeUpstream(t, upstreamFiles())
	writeFiles(t, upstream, map[string]string{"naga/tests/in/wgsl/a.wgsl": "edited\n"})

	out := t.TempDir()
	_, err := syncCorpus(upstream, out)
	if err == nil || !strings.Contains(err.Error(), "uncommitted") {
		t.Fatalf("err = %v, want refusal mentioning uncommitted changes", err)
	}
	entries, readErr := os.ReadDir(out)
	if readErr != nil || len(entries) != 0 {
		t.Errorf("output touched after refusal: %v %v", entries, readErr)
	}
}

func TestSyncRefusesNonCorpusFilesInOutput(t *testing.T) {
	upstream := fakeUpstream(t, upstreamFiles())
	out := t.TempDir()
	// A directory that is not a corpus tree must never be wiped.
	writeFiles(t, out, map[string]string{"main.go": "package main\n"})

	_, err := syncCorpus(upstream, out)
	if err == nil || !strings.Contains(err.Error(), testcorpus.ManifestName) {
		t.Fatalf("err = %v, want refusal: output has files but no manifest", err)
	}
	if _, statErr := os.Stat(filepath.Join(out, "main.go")); statErr != nil {
		t.Errorf("main.go removed: %v", statErr)
	}
}

func TestManifestCommandWritesExplicitSource(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"ir/wgsl-a.ron": "(ir)\n"})

	var stdout bytes.Buffer
	err := run([]string{
		"manifest", "-dir", dir, "-kind", "derived",
		"-repo", upstreamRepository, "-generator", "ir_dump", "-note", "test",
	}, &stdout)
	if err != nil {
		t.Fatal(err)
	}
	m, err := testcorpus.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Kind != testcorpus.KindDerived || m.Source.Generator != "ir_dump" || m.Source.Note != "test" ||
		len(m.Files) != 1 || m.Files[0].Path != "ir/wgsl-a.ron" {
		t.Errorf("manifest = %+v", m)
	}
}

func TestRunRejectsUsageErrors(t *testing.T) {
	tests := [][]string{
		nil,
		{"unknown"},
		{"sync"},                          // -wgpu and -out required
		{"manifest", "-dir", t.TempDir()}, // -kind and -repo required
		{"manifest", "-dir", t.TempDir(), "-kind", "copied", "-repo", "r"},
	}
	for _, args := range tests {
		var stdout bytes.Buffer
		if err := run(args, &stdout); err == nil {
			t.Errorf("run(%q) succeeded, want a usage error", args)
		}
	}
}
