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
// manifest (re)writes MANIFEST.json for an existing tree from explicit source
// flags. Use it for derived trees after regenerating them; a vendored tree is
// written by sync.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
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
// and -kind) cannot erase the provenance of derived files. A missing manifest
// leaves src untouched; an invalid one is an error.
func inheritSource(dir string, fl *flag.FlagSet, src *testcorpus.Source) error {
	manifestPath := filepath.Join(dir, testcorpus.ManifestName)
	if _, err := os.Stat(manifestPath); errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	prev, err := testcorpus.Load(dir)
	if err != nil {
		return fmt.Errorf("%s: %w", manifestPath, err)
	}
	set := make(map[string]bool)
	fl.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if !set["repo"] {
		src.Repository = prev.Source.Repository
	}
	if !set["commit"] {
		src.Commit = prev.Source.Commit
	}
	if !set["commit-date"] {
		src.CommitDate = prev.Source.CommitDate
	}
	if !set["generator"] {
		src.Generator = prev.Source.Generator
	}
	if !set["note"] {
		src.Note = prev.Source.Note
	}
	return nil
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
	if err := inheritSource(*dir, fl, &src); err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	if src.Repository == "" {
		return errors.New("manifest: -repo is required (or an existing manifest with a repository)")
	}

	files, err := testcorpus.HashTree(*dir)
	if err != nil {
		return err
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
