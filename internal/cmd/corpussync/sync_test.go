package main

import (
	"bytes"
	"io"
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
		// SPIR-V input corpus: the assembly, its parameters and its goldens.
		"naga/tests/in/spv/s.spvasm":                 "; spv input\n",
		"naga/tests/in/spv/s.toml":                   "targets = \"MSL\"\n",
		"naga/tests/in/spv/s.slang":                  "// source\n",
		"naga/tests/out/msl/spv-s.metal":             "// spv msl\n",
		"naga/tests/out/hlsl/spv-s.hlsl":             "// spv hlsl\n",
		"naga/tests/out/glsl/spv-s.main.Vertex.glsl": "// spv glsl\n",
		"naga/tests/out/ir/spv-s.ron":                "(spv ir)\n",
		// Not selected: the SPIR-V backend's own output for an spv input, the
		// WGSL round-trip, analysis info, non-corpus sources and code.
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
		"in/spv/s.spvasm":                   "; spv input\n",
		"in/spv/s.toml":                     "targets = \"MSL\"\n",
		"out/msl/spv-s.metal":               "// spv msl\n",
		"out/hlsl/spv-s.hlsl":               "// spv hlsl\n",
		"out/glsl/spv-s.main.Vertex.glsl":   "// spv glsl\n",
		"out/ir/spv-s.ron":                  "(spv ir)\n",
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

// TestManifestKeepsExistingSource pins the provenance rule: rewriting a
// manifest without the source flags keeps the tree's recorded commit,
// generator and note; an explicit flag overrides its own field only.
func TestManifestKeepsExistingSource(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.spv"), []byte("spv"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	full := []string{"manifest", "-dir", dir, "-kind", "derived",
		"-repo", upstreamRepository, "-commit", "14328f49", "-commit-date", "2026-10-14",
		"-generator", "spirv-as", "-note", "keep me"}
	if err := run(full, &out); err != nil {
		t.Fatalf("manifest: %v", err)
	}

	if err := run([]string{"manifest", "-dir", dir, "-kind", "derived"}, &out); err != nil {
		t.Fatalf("manifest without source flags: %v", err)
	}
	m, err := testcorpus.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Source.Repository != upstreamRepository || m.Source.Commit != "14328f49" ||
		m.Source.CommitDate != "2026-10-14" || m.Source.Generator != "spirv-as" ||
		m.Source.Note != "keep me" {
		t.Errorf("source = %+v, want the existing provenance kept", m.Source)
	}

	if err := run([]string{"manifest", "-dir", dir, "-kind", "derived", "-note", "new note"}, &out); err != nil {
		t.Fatalf("manifest with -note: %v", err)
	}
	m, err = testcorpus.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Source.Note != "new note" || m.Source.Generator != "spirv-as" || m.Source.Commit != "14328f49" {
		t.Errorf("source = %+v, want only the note overridden", m.Source)
	}
}

// TestManifestRefusesVendoredTree pins the provenance boundary: a hand-edited
// vendored file cannot be re-blessed while the upstream commit claim stays the
// same. The edit stays visible to the integrity check after the refusal.
func TestManifestRefusesVendoredTree(t *testing.T) {
	upstream := fakeUpstream(t, upstreamFiles())
	dir := t.TempDir()
	if _, err := syncCorpus(upstream, dir); err != nil {
		t.Fatal(err)
	}
	writeFiles(t, dir, map[string]string{"in/spv/s.spvasm": "; edited by hand\n"})

	var out bytes.Buffer
	err := run([]string{"manifest", "-dir", dir, "-kind", "vendored", "-repo", upstreamRepository}, &out)
	if err == nil || !strings.Contains(err.Error(), "vendored") {
		t.Fatalf("err = %v, want a refusal of -kind vendored", err)
	}
	problems, verifyErr := testcorpus.Verify(dir)
	if verifyErr != nil || len(problems) != 1 || problems[0].Path != "in/spv/s.spvasm" {
		t.Errorf("Verify after the refused rewrite = %v %v, want the hand edit still detected", problems, verifyErr)
	}
}

// TestManifestVendoredExtensionAndRefresh: at the recorded commit a file may
// be added (an extension of the tree) but a changed one is refused; an
// explicit new commit is a refresh and may rewrite files.
func TestManifestVendoredExtensionAndRefresh(t *testing.T) {
	upstream := fakeUpstream(t, upstreamFiles())
	dir := t.TempDir()
	if _, err := syncCorpus(upstream, dir); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer

	writeFiles(t, dir, map[string]string{"extinst.glsl.std.450.grammar.json": "{}\n"})
	if err := run([]string{"manifest", "-dir", dir, "-kind", "vendored"}, &out); err != nil {
		t.Fatalf("manifest after adding a file at the recorded commit: %v", err)
	}
	m, err := testcorpus.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Files) != 17 || m.Source.Commit == "" {
		t.Errorf("manifest = %d files from %q, want the extension recorded", len(m.Files), m.Source.Commit)
	}

	writeFiles(t, dir, map[string]string{"in/spv/s.spvasm": "; edited by hand\n"})
	err = run([]string{"manifest", "-dir", dir, "-kind", "vendored"}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "in/spv/s.spvasm changed") {
		t.Fatalf("manifest over an edited vendored file = %v, want the hand edit refused", err)
	}

	if err := run([]string{"manifest", "-dir", dir, "-kind", "vendored",
		"-commit", "0123456789abcdef", "-commit-date", "2026-02-02", "-note", "refresh"}, &out); err != nil {
		t.Fatalf("manifest after a commit refresh: %v", err)
	}
	m, err = testcorpus.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Source.Commit != "0123456789abcdef" || m.Source.CommitDate != "2026-02-02" || m.Source.Note != "refresh" {
		t.Errorf("source = %+v, want the new commit's provenance", m.Source)
	}
	if problems, err := testcorpus.Verify(dir); err != nil || len(problems) != 0 {
		t.Errorf("Verify after the refresh = %v %v, want a tree matching its manifest", problems, err)
	}
}

// TestManifestRefusesKindMismatch: a tree recorded as one kind is never
// rewritten as the other.
func TestManifestRefusesKindMismatch(t *testing.T) {
	upstream := fakeUpstream(t, upstreamFiles())
	dir := t.TempDir()
	if _, err := syncCorpus(upstream, dir); err != nil {
		t.Fatal(err)
	}
	err := run([]string{"manifest", "-dir", dir, "-kind", "derived", "-repo", upstreamRepository}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "recorded as vendored") {
		t.Fatalf("err = %v, want a refusal naming the recorded kind", err)
	}
}

// TestManifestChangedCommitNeedsNewProvenance: an explicit -commit that
// differs from the recorded one must not inherit the date and note that
// described the old commit; naming the recorded commit again inherits as
// usual.
func TestManifestChangedCommitNeedsNewProvenance(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"a.spv": "spv"})
	var out bytes.Buffer
	base := []string{"manifest", "-dir", dir, "-kind", "derived", "-repo", upstreamRepository,
		"-commit", "old", "-commit-date", "2026-01-01", "-note", "old note", "-generator", "spirv-as"}
	if err := run(base, &out); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"manifest", "-dir", dir, "-kind", "derived", "-commit", "new"},
		{"manifest", "-dir", dir, "-kind", "derived", "-commit", "new", "-commit-date", "2026-02-02"},
	} {
		err := run(args, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "-commit-date") || !strings.Contains(err.Error(), "-note") {
			t.Errorf("run(%q) = %v, want a refusal naming -commit-date and -note", args, err)
		}
	}

	if err := run([]string{"manifest", "-dir", dir, "-kind", "derived",
		"-commit", "new", "-commit-date", "2026-02-02", "-note", "new note"}, &out); err != nil {
		t.Fatalf("manifest with the new provenance: %v", err)
	}
	m, err := testcorpus.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Source.Commit != "new" || m.Source.CommitDate != "2026-02-02" || m.Source.Note != "new note" ||
		m.Source.Repository != upstreamRepository || m.Source.Generator != "spirv-as" {
		t.Errorf("source = %+v, want the new commit's date and note with the kept repository and generator", m.Source)
	}

	// Naming the recorded commit again is not a move: the date and note stay.
	if err := run([]string{"manifest", "-dir", dir, "-kind", "derived", "-commit", "new"}, &out); err != nil {
		t.Fatalf("manifest with the recorded commit: %v", err)
	}
	m, err = testcorpus.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Source.CommitDate != "2026-02-02" || m.Source.Note != "new note" {
		t.Errorf("source = %+v, want the recorded date and note kept", m.Source)
	}
}

// TestRunErrors covers the command's refusal paths: usage, unknown command,
// missing flags, and an unreadable manifest that must not be overwritten.
func TestRunErrors(t *testing.T) {
	if err := run(nil, io.Discard); err == nil {
		t.Error("run with no arguments succeeded, want a usage error")
	}
	if err := run([]string{"bogus"}, io.Discard); err == nil {
		t.Error("run with an unknown command succeeded, want an error")
	}
	if err := run([]string{"sync"}, io.Discard); err == nil {
		t.Error("sync without -wgpu/-out succeeded, want an error")
	}
	dir := t.TempDir()
	if err := run([]string{"manifest", "-dir", dir}, io.Discard); err == nil {
		t.Error("manifest without -kind succeeded, want an error")
	}
	if err := run([]string{"manifest", "-dir", dir, "-kind", "derived"}, io.Discard); err == nil {
		t.Error("manifest without a repository anywhere succeeded, want an error")
	}
	if err := os.WriteFile(filepath.Join(dir, testcorpus.ManifestName), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"manifest", "-dir", dir, "-kind", "derived", "-repo", "https://example.com/x"}, io.Discard); err == nil {
		t.Error("manifest over an invalid existing manifest succeeded, want an error")
	}
}

// TestSyncRequiresBothCorpora pins the shape check: a checkout missing either
// input set is refused instead of vendoring an empty half.
func TestSyncRequiresBothCorpora(t *testing.T) {
	t.Run("no SPIR-V inputs", func(t *testing.T) {
		files := upstreamFiles()
		delete(files, "naga/tests/in/spv/s.spvasm")
		delete(files, "naga/tests/in/spv/s.toml")
		upstream := fakeUpstream(t, files)
		_, err := syncCorpus(upstream, t.TempDir())
		if err == nil || !strings.Contains(err.Error(), "no .spvasm inputs") {
			t.Fatalf("err = %v, want a refusal mentioning the missing SPIR-V inputs", err)
		}
	})
	t.Run("no WGSL inputs", func(t *testing.T) {
		files := upstreamFiles()
		delete(files, "naga/tests/in/wgsl/a.wgsl")
		delete(files, "naga/tests/in/wgsl/a.toml")
		upstream := fakeUpstream(t, files)
		_, err := syncCorpus(upstream, t.TempDir())
		if err == nil || !strings.Contains(err.Error(), "no .wgsl inputs") {
			t.Fatalf("err = %v, want a refusal mentioning the missing WGSL inputs", err)
		}
	})
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
