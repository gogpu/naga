package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/gogpu/naga/internal/testcorpus"
)

// upstreamRepository is recorded as the source of the vendored corpus.
const upstreamRepository = "https://github.com/gfx-rs/wgpu"

// outputBackends are the naga/tests/out subdirectories vendored for WGSL
// inputs (files named wgsl-*). Other outputs (wgsl round-trip, dot, analysis)
// and outputs of non-WGSL inputs are not compared by our tests.
var outputBackends = []string{"spv", "msl", "hlsl", "glsl", "ir"}

// licenseFiles are copied to the tree root: the vendored files are
// MIT OR Apache-2.0 licensed.
var licenseFiles = []string{"LICENSE.APACHE", "LICENSE.MIT"}

const (
	upstreamInWGSL = "naga/tests/in/wgsl/"
	upstreamOut    = "naga/tests/out/"
)

// vendoredPath maps an upstream repository path to its path in the vendored
// tree, or reports false if the file is not part of the corpus.
func vendoredPath(upstreamPath string) (string, bool) {
	for _, name := range licenseFiles {
		if upstreamPath == name {
			return name, true
		}
	}
	if rel, ok := strings.CutPrefix(upstreamPath, upstreamInWGSL); ok {
		return "in/wgsl/" + rel, true
	}
	for _, backend := range outputBackends {
		rel, ok := strings.CutPrefix(upstreamPath, upstreamOut+backend+"/")
		if ok && strings.HasPrefix(path.Base(rel), "wgsl-") {
			return "out/" + backend + "/" + rel, true
		}
	}
	return "", false
}

// Report summarizes a sync against the previous manifest.
type Report struct {
	Commit     string
	CommitDate string
	Files      int
	Added      []string
	Removed    []string
	Changed    []string
}

// syncCorpus replaces the vendored tree at outDir with the corpus files of the
// HEAD commit of the gfx-rs/wgpu clone at upstreamDir and writes its manifest.
// The tree's README.md is preserved.
func syncCorpus(upstreamDir, outDir string) (Report, error) {
	repo := gitRepo{dir: upstreamDir}
	if st, err := repo.status(); err != nil {
		return Report{}, err
	} else if st != "" {
		return Report{}, fmt.Errorf("%s has uncommitted changes; sync only from a clean checkout:\n%s", upstreamDir, st)
	}
	commit, date, err := repo.head()
	if err != nil {
		return Report{}, err
	}

	files, err := readCorpus(repo)
	if err != nil {
		return Report{}, err
	}
	previous, err := checkOutputTree(outDir)
	if err != nil {
		return Report{}, err
	}
	if err := replaceTree(outDir, previous, files); err != nil {
		return Report{}, err
	}

	hashed, err := testcorpus.HashTree(outDir)
	if err != nil {
		return Report{}, err
	}
	m := &testcorpus.Manifest{
		Schema: testcorpus.SchemaVersion,
		Kind:   testcorpus.KindVendored,
		Source: testcorpus.Source{Repository: upstreamRepository, Commit: commit, CommitDate: date},
		Files:  hashed,
	}
	if err := m.Write(outDir); err != nil {
		return Report{}, err
	}

	report := diffManifests(previous, hashed)
	report.Commit, report.CommitDate, report.Files = commit, date, len(hashed)
	return report, nil
}

// readCorpus returns the selected files of HEAD keyed by vendored path. Every
// blob is checked against its id, so the copy provably equals upstream.
func readCorpus(repo gitRepo) (map[string][]byte, error) {
	pathspecs := append([]string{upstreamInWGSL}, licenseFiles...)
	for _, backend := range outputBackends {
		pathspecs = append(pathspecs, upstreamOut+backend+"/")
	}
	entries, err := repo.lsTree(pathspecs...)
	if err != nil {
		return nil, err
	}

	var selected []treeEntry
	var dests []string
	for _, e := range entries {
		if dest, ok := vendoredPath(e.path); ok {
			selected = append(selected, e)
			dests = append(dests, dest)
		}
	}
	ids := make([]string, len(selected))
	for i, e := range selected {
		ids[i] = e.blob
	}
	contents, err := repo.readBlobs(ids)
	if err != nil {
		return nil, err
	}

	files := make(map[string][]byte, len(selected))
	for i, e := range selected {
		if got := testcorpus.GitBlobID(contents[i]); got != e.blob {
			return nil, fmt.Errorf("%s: content hashes to %s, git lists %s", e.path, got, e.blob)
		}
		files[dests[i]] = contents[i]
	}
	if err := requireCorpusShape(files); err != nil {
		return nil, err
	}
	return files, nil
}

// requireCorpusShape fails on a checkout that is not gfx-rs/wgpu (or whose
// layout moved), instead of silently vendoring an empty corpus.
func requireCorpusShape(files map[string][]byte) error {
	for _, name := range licenseFiles {
		if _, ok := files[name]; !ok {
			return fmt.Errorf("upstream has no %s at the repository root", name)
		}
	}
	for p := range files {
		if strings.HasPrefix(p, "in/wgsl/") && strings.HasSuffix(p, ".wgsl") {
			return nil
		}
	}
	return fmt.Errorf("upstream has no WGSL inputs under %s", upstreamInWGSL)
}

// checkOutputTree returns the current manifest of outDir (nil for a new or
// empty directory). It refuses a non-empty directory without a manifest, or a
// tree holding files its manifest does not list: sync must never delete files
// it does not own.
func checkOutputTree(outDir string) (*testcorpus.Manifest, error) {
	entries, err := os.ReadDir(outDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil //nolint:nilnil // a missing tree has no previous manifest
	}
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil //nolint:nilnil // an empty tree has no previous manifest
	}
	previous, err := testcorpus.Load(outDir)
	if err != nil {
		return nil, fmt.Errorf("%s is not empty and has no valid %s; refusing to overwrite it: %w",
			outDir, testcorpus.ManifestName, err)
	}
	problems, err := testcorpus.Verify(outDir)
	if err != nil {
		return nil, err
	}
	for _, p := range problems {
		if p.Kind == testcorpus.ProblemExtra {
			return nil, fmt.Errorf("%s/%s is not listed in %s; refusing to delete it", outDir, p.Path, testcorpus.ManifestName)
		}
	}
	return previous, nil
}

// replaceTree removes the files of the previous manifest and writes files.
func replaceTree(outDir string, previous *testcorpus.Manifest, files map[string][]byte) error {
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return err
	}
	root, err := os.OpenRoot(outDir)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()

	if previous != nil {
		for _, f := range previous.Files {
			if err := root.Remove(f.Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
		}
		if err := removeEmptyDirs(root, "."); err != nil {
			return err
		}
	}
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		if dir := path.Dir(p); dir != "." {
			if err := root.MkdirAll(dir, 0o750); err != nil {
				return err
			}
		}
		if err := root.WriteFile(p, files[p], 0o644); err != nil {
			return err
		}
	}
	return root.Close()
}

// removeEmptyDirs removes empty directories below dir (not dir itself).
func removeEmptyDirs(root *os.Root, dir string) error {
	entries, err := fs.ReadDir(root.FS(), dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		child := path.Join(dir, e.Name())
		if err := removeEmptyDirs(root, child); err != nil {
			return err
		}
		if rest, err := fs.ReadDir(root.FS(), child); err != nil {
			return err
		} else if len(rest) == 0 {
			if err := root.Remove(child); err != nil {
				return err
			}
		}
	}
	return nil
}

// diffManifests lists paths added, removed and changed since previous.
func diffManifests(previous *testcorpus.Manifest, current []testcorpus.File) Report {
	var r Report
	old := map[string]string{}
	if previous != nil {
		for _, f := range previous.Files {
			old[f.Path] = f.SHA256
		}
	}
	for _, f := range current {
		sum, ok := old[f.Path]
		switch {
		case !ok:
			r.Added = append(r.Added, f.Path)
		case sum != f.SHA256:
			r.Changed = append(r.Changed, f.Path)
		}
		delete(old, f.Path)
	}
	for p := range old {
		r.Removed = append(r.Removed, p)
	}
	sort.Strings(r.Removed)
	return r
}
