package testcorpus

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTree creates files (slash-separated relative path → content) under a
// fresh temporary directory and returns it.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestGitBlobIDMatchesGit(t *testing.T) {
	// Values produced by `git hash-object --stdin`.
	tests := []struct {
		content string
		want    string
	}{
		{"", "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
		{"hello\n", "ce013625030ba8dba906f756967f9e9ca394464a"},
	}
	for _, tt := range tests {
		if got := GitBlobID([]byte(tt.content)); got != tt.want {
			t.Errorf("GitBlobID(%q) = %s, want %s", tt.content, got, tt.want)
		}
	}
}

func TestHashTreeIsSortedAndSkipsOwnFiles(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"out/b.txt":    "b",
		"in/a.wgsl":    "a",
		"LICENSE.MIT":  "mit",
		ManifestName:   "{}",
		ReadmeName:     "readme",
		"in/sub/c.txt": "c",
	})
	files, err := HashTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range files {
		got = append(got, f.Path)
	}
	want := []string{"LICENSE.MIT", "in/a.wgsl", "in/sub/c.txt", "out/b.txt"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("paths = %v, want %v", got, want)
	}
	if files[1].GitBlob != GitBlobID([]byte("a")) {
		t.Errorf("gitBlob of in/a.wgsl = %s", files[1].GitBlob)
	}
	if len(files[1].SHA256) != 64 {
		t.Errorf("sha256 of in/a.wgsl = %q, want 64 hex chars", files[1].SHA256)
	}
}

func TestWriteThenLoadRoundTripsByteForByte(t *testing.T) {
	dir := writeTree(t, map[string]string{"in/a.wgsl": "a", "out/b.txt": "b"})
	files, err := HashTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := &Manifest{
		Schema: SchemaVersion,
		Kind:   KindVendored,
		Source: Source{Repository: "https://github.com/gfx-rs/wgpu", Commit: "14328f49"},
		Files:  files,
	}
	if err := m.Write(dir); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(dir, ManifestName))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(first, []byte("\r")) || !bytes.HasSuffix(first, []byte("}\n")) {
		t.Errorf("manifest must use LF and end with a newline:\n%s", first)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := loaded.Write(dir); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(dir, ManifestName))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Errorf("Write(Load(x)) is not byte-identical:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestVerifyCleanTree(t *testing.T) {
	dir := writeTree(t, map[string]string{"in/a.wgsl": "a"})
	writeManifest(t, dir)
	problems, err := Verify(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Errorf("clean tree: problems = %v", problems)
	}
}

func TestVerifyReportsEveryKindOfDrift(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"in/kept.wgsl":    "kept",
		"in/edited.wgsl":  "line\n",
		"in/removed.wgsl": "removed",
	})
	writeManifest(t, dir)

	// A CRLF conversion is a modification: vendored files are byte-exact.
	if err := os.WriteFile(filepath.Join(dir, "in", "edited.wgsl"), []byte("line\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "in", "removed.wgsl")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "in", "added.wgsl"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}

	problems, err := Verify(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]ProblemKind{}
	for _, p := range problems {
		got[p.Path] = p.Kind
	}
	want := map[string]ProblemKind{
		"in/edited.wgsl":  ProblemModified,
		"in/removed.wgsl": ProblemMissing,
		"in/added.wgsl":   ProblemExtra,
	}
	if len(got) != len(want) {
		t.Fatalf("problems = %v, want %v", problems, want)
	}
	for path, kind := range want {
		if got[path] != kind {
			t.Errorf("%s: kind = %v, want %v", path, got[path], kind)
		}
	}
}

func TestLoadRejectsInvalidManifests(t *testing.T) {
	const sha = "0000000000000000000000000000000000000000000000000000000000000000"
	const blob = "0000000000000000000000000000000000000000"
	file := func(path string) string {
		return `{"path":"` + path + `","sha256":"` + sha + `","gitBlob":"` + blob + `"}`
	}
	manifest := func(schema, kind, files string) string {
		return `{"schema":` + schema + `,"kind":"` + kind + `","source":{"repository":"r"},"files":[` + files + `]}`
	}
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{"unknown schema", manifest("99", "vendored", file("a")), "schema"},
		{"unknown kind", manifest("1", "copied", file("a")), "kind"},
		{"unsorted paths", manifest("1", "vendored", file("b")+","+file("a")), "sorted"},
		{"duplicate path", manifest("1", "vendored", file("a")+","+file("a")), "sorted"},
		{"parent traversal", manifest("1", "vendored", file("../a")), "path"},
		{"absolute path", manifest("1", "vendored", file("/a")), "path"},
		{"backslash path", manifest("1", "vendored", file(`a\\b`)), "path"},
		{"manifest lists itself", manifest("1", "vendored", file(ManifestName)), "path"},
		{"bad sha256", strings.Replace(manifest("1", "vendored", file("a")), sha, "xyz", 1), "sha256"},
		{"bad gitBlob", strings.Replace(manifest("1", "vendored", file("a")), blob, "xyz", 1), "gitBlob"},
		{"unknown field", `{"schema":1,"kind":"vendored","source":{"repository":"r"},"files":[],"extra":1}`, "unknown field"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeTree(t, map[string]string{ManifestName: tt.content})
			_, err := Load(dir)
			if err == nil {
				t.Fatalf("Load accepted an invalid manifest")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to mention %q", err, tt.wantErr)
			}
		})
	}
}

func writeManifest(t *testing.T, dir string) {
	t.Helper()
	files, err := HashTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := &Manifest{Schema: SchemaVersion, Kind: KindVendored, Source: Source{Repository: "r"}, Files: files}
	if err := m.Write(dir); err != nil {
		t.Fatal(err)
	}
}
