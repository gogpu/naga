// Command corpussync maintains the Rust naga test corpus vendored under
// snapshot/testdata (ADR-006).
//
// Usage:
//
//	go run ./internal/cmd/corpussync sync -wgpu <gfx-rs/wgpu checkout> -out snapshot/testdata/rust
//	go run ./internal/cmd/corpussync manifest -dir <tree> -kind vendored|derived -repo <url> [flags]
//
// sync replaces the vendored tree with the WGSL inputs, their TOML parameters,
// the SPIR-V/MSL/HLSL/GLSL/IR outputs and the license files of the checkout's
// HEAD commit, read from git objects, and writes MANIFEST.json. The checkout
// must be clean.
//
// manifest (re)writes MANIFEST.json for an existing derived tree from explicit
// source flags, and records a manually vendored tree (SPIRV-Headers, license
// corpora) after an extension or a refresh. For a vendored tree it refuses to
// re-bless a changed or missing file while the recorded commit stays the same:
// that is the hand edit sync would have caught. A new -commit is a refresh and
// may rewrite every file.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gogpu/naga/internal/testcorpus"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "corpussync:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: corpussync sync|manifest [flags]")
	}
	switch args[0] {
	case "sync":
		return runSync(args[1:], stdout)
	case "manifest":
		return runManifest(args[1:], stdout)
	default:
		return fmt.Errorf("unknown command %q (want sync or manifest)", args[0])
	}
}

func runSync(args []string, stdout io.Writer) error {
	fl := flag.NewFlagSet("sync", flag.ContinueOnError)
	upstream := fl.String("wgpu", "", "path to a clean gfx-rs/wgpu checkout")
	out := fl.String("out", "", "vendored tree to replace (snapshot/testdata/rust)")
	if err := fl.Parse(args); err != nil {
		return err
	}
	if *upstream == "" || *out == "" {
		return errors.New("sync: -wgpu and -out are required")
	}

	report, err := syncCorpus(*upstream, *out)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "synced %d files from %s @ %s (%s)\n%s",
		report.Files, upstreamRepository, report.Commit, report.CommitDate, formatChanges(report))
	return err
}

func formatChanges(r Report) string {
	var b strings.Builder
	for _, group := range []struct {
		label string
		paths []string
	}{{"added", r.Added}, {"removed", r.Removed}, {"changed", r.Changed}} {
		fmt.Fprintf(&b, "%s: %d\n", group.label, len(group.paths))
		for _, p := range group.paths {
			fmt.Fprintf(&b, "  %s\n", p)
		}
	}
	return b.String()
}

// inheritSource fills the source fields a flag did not set from the manifest
// already recorded in the tree, so a regeneration (usually run with only -dir
// and -kind) cannot erase the provenance of derived files. It returns the
// recorded manifest, or nil when the tree has none yet. An invalid manifest is
// an error, and so is a tree recorded as another kind.
//
// The commit, date and note follow applyCommitMove: an explicit -commit that
// differs from the recorded one moves the tree to a new upstream commit and
// inherits none of the old facts about it.
func inheritSource(dir string, fl *flag.FlagSet, kind testcorpus.Kind, src *testcorpus.Source) (*testcorpus.Manifest, error) {
	manifestPath := filepath.Join(dir, testcorpus.ManifestName)
	if _, err := os.Stat(manifestPath); errors.Is(err, fs.ErrNotExist) {
		// A tree without a manifest is the initial vendoring: nil prev is a
		// valid result ("no recorded source"), never an error.
		return nil, nil //nolint:nilnil // the caller checks the error first
	} else if err != nil {
		return nil, err
	}
	prev, err := testcorpus.Load(dir)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", manifestPath, err)
	}
	if prev.Kind != kind {
		return nil, fmt.Errorf("%s is recorded as %s, not %s", dir, prev.Kind, kind)
	}
	set := make(map[string]bool)
	fl.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if err := applyCommitMove(set, &prev.Source, src); err != nil {
		return nil, err
	}
	if !set["repo"] {
		src.Repository = prev.Source.Repository
	}
	if !set["generator"] {
		src.Generator = prev.Source.Generator
	}
	return prev, nil
}

// applyCommitMove fills the commit, date and note fields from the recorded
// source, except when -commit explicitly names a different commit: the old
// date and note described the old commit, so the caller must supply both for
// the new one and nothing is inherited.
func applyCommitMove(set map[string]bool, prev *testcorpus.Source, src *testcorpus.Source) error {
	if set["commit"] && src.Commit != prev.Commit {
		if !set["commit-date"] || !set["note"] || src.CommitDate == "" || src.Note == "" {
			return fmt.Errorf("the commit changes from %q to %q: pass -commit-date and -note for the new commit; they are not inherited from the old one",
				prev.Commit, src.Commit)
		}
		return nil
	}
	if !set["commit"] {
		src.Commit = prev.Commit
	}
	if !set["commit-date"] {
		src.CommitDate = prev.CommitDate
	}
	if !set["note"] {
		src.Note = prev.Note
	}
	return nil
}

// checkVendoredRefresh refuses to re-bless a vendored file that changed or
// disappeared while the recorded upstream commit stays the same. Rewriting a
// vendored manifest at one commit is legitimate as an extension — a file added
// from that commit — but a changed file is a hand edit that sync would have
// caught. An explicit new commit is a refresh and may rewrite everything, and
// a tree recorded without a commit has no commit claim to protect.
func checkVendoredRefresh(prev *testcorpus.Manifest, src *testcorpus.Source, files []testcorpus.File) error {
	if prev == nil || prev.Source.Commit == "" || src.Commit != prev.Source.Commit {
		return nil
	}
	current := make(map[string]string, len(files))
	for _, f := range files {
		current[f.Path] = f.SHA256
	}
	var problems []string
	for _, f := range prev.Files {
		got, ok := current[f.Path]
		switch {
		case !ok:
			problems = append(problems, f.Path+" is missing")
		case got != f.SHA256:
			problems = append(problems, f.Path+" changed")
		}
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("refusing to re-bless a vendored tree: %s at the recorded commit %q; pass the new -commit (with -commit-date and -note) when the upstream tree moves",
		strings.Join(problems, ", "), prev.Source.Commit)
}

func runManifest(args []string, stdout io.Writer) error {
	fl := flag.NewFlagSet("manifest", flag.ContinueOnError)
	dir := fl.String("dir", "", "corpus tree")
	kind := fl.String("kind", "", "vendored or derived")
	src := testcorpus.Source{}
	fl.StringVar(&src.Repository, "repo", "", "upstream repository URL")
	fl.StringVar(&src.Commit, "commit", "", "upstream commit, if a single one is known")
	fl.StringVar(&src.CommitDate, "commit-date", "", "committer date of -commit (YYYY-MM-DD)")
	fl.StringVar(&src.Generator, "generator", "", "tool that produced derived files")
	fl.StringVar(&src.Note, "note", "", "provenance note for readers")
	if err := fl.Parse(args); err != nil {
		return err
	}
	if *dir == "" || *kind == "" {
		return errors.New("manifest: -dir and -kind are required")
	}
	switch testcorpus.Kind(*kind) {
	case testcorpus.KindDerived, testcorpus.KindVendored:
	default:
		return fmt.Errorf("manifest: unknown kind %q (want vendored or derived)", *kind)
	}
	prev, err := inheritSource(*dir, fl, testcorpus.Kind(*kind), &src)
	if err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	if src.Repository == "" {
		return errors.New("manifest: -repo is required (or an existing manifest with a repository)")
	}

	files, err := testcorpus.HashTree(*dir)
	if err != nil {
		return err
	}
	if testcorpus.Kind(*kind) == testcorpus.KindVendored {
		if err := checkVendoredRefresh(prev, &src, files); err != nil {
			return fmt.Errorf("manifest: %w", err)
		}
	}
	m := &testcorpus.Manifest{
		Schema: testcorpus.SchemaVersion,
		Kind:   testcorpus.Kind(*kind),
		Source: src,
		Files:  files,
	}
	if err := m.Write(*dir); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "wrote %s/%s: %d files\n", *dir, testcorpus.ManifestName, len(files))
	return err
}
