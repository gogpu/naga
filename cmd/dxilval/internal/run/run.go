// Package run implements the dxilval CLI logic. main.go is a 3-line entry
// point; this package owns flag parsing, mode dispatch, and exit codes so
// the logic is testable without spawning a subprocess.
package run

import (
	"flag"
	"fmt"
	"io"
)

// Exit codes.
const (
	ExitOK         = 0
	ExitUsage      = 2
	ExitFail       = 1
	ExitValidation = 3 // validator returned INVALID for at least one input
)

// Main parses args and dispatches to the appropriate mode. Returns a process
// exit code. stdout receives normal output (results, summaries); stderr
// receives errors and progress.
func Main(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("dxilval", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		wgsl   string
		corpus string
		entry  string
		save   string
		dump   string
		quiet  bool
	)
	fs.StringVar(&wgsl, "wgsl", "", "compile a WGSL file through naga and validate every entry point")
	fs.StringVar(&corpus, "corpus", "", "walk a directory of *.wgsl files, validate every entry point, print summary")
	fs.StringVar(&entry, "entry", "", "with --wgsl: validate only this entry point (default: all)")
	fs.StringVar(&save, "save", "", "with --corpus: also write the summary to this file")
	fs.StringVar(&dump, "dump", "", "with --wgsl: write the compiled DXIL blob(s) to this path/prefix instead of validating; survives dxil.dll AV crashes and is the canonical way to inspect output via dxc -dumpbin")
	fs.BoolVar(&quiet, "quiet", false, "with --corpus: omit per-shader detail, summary only")

	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: dxilval [flags] <input.dxil>\n\n")
		fmt.Fprintf(stderr, "Modes:\n")
		fmt.Fprintf(stderr, "  dxilval shader.dxil                  validate a single DXIL container\n")
		fmt.Fprintf(stderr, "  dxilval --wgsl shader.wgsl           compile through naga, then validate\n")
		fmt.Fprintf(stderr, "  dxilval --corpus dir/                walk directory, summarize\n\n")
		fmt.Fprintf(stderr, "Flags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		// flag already printed the error to stderr.
		return ExitUsage
	}

	// Mutually exclusive mode flags.
	modeCount := 0
	if wgsl != "" {
		modeCount++
	}
	if corpus != "" {
		modeCount++
	}
	if fs.NArg() > 0 {
		modeCount++
	}
	if modeCount == 0 {
		fs.Usage()
		return ExitUsage
	}
	if modeCount > 1 {
		fmt.Fprintln(stderr, "dxilval: --wgsl, --corpus, and a positional .dxil file are mutually exclusive")
		return ExitUsage
	}

	switch {
	case corpus != "":
		return runCorpus(corpus, save, quiet, stdout, stderr)
	case wgsl != "" && dump != "":
		return runDump(wgsl, entry, dump, stdout, stderr)
	case wgsl != "":
		return runWGSL(wgsl, entry, stdout, stderr)
	default:
		return runSingle(fs.Arg(0), stdout, stderr)
	}
}
