// Command nagac is the naga shader compiler CLI.
//
// Usage:
//
//	nagac [options] <input>
//
// Options must come before the input file: Go's flag package stops parsing at
// the first non-flag argument, so nagac rejects anything that follows the input
// file instead of silently ignoring it.
//
// Examples:
//
//	nagac shader.wgsl                    # Compile to SPIR-V on stdout (not to a terminal)
//	nagac -o shader.spv shader.wgsl      # Compile to SPIR-V in a file
//	nagac -debug -o shader.spv shader.wgsl  # Compile with debug info
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"github.com/gogpu/naga"
	"github.com/gogpu/naga/spirv"
)

// Exit statuses. A command line the tool cannot interpret is a usage error
// (2, as the flag package and go vet use); a failure while working is 1.
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

// version returns the module version from build info.
func version() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
	}
	return "dev"
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, isTerminal(os.Stdout)))
}

// isTerminal reports whether f is an interactive terminal. It uses the file
// mode, not golang.org/x/term, to stay dependency free. /dev/null (NUL on
// Windows) is a character device too, but discarding output there is a
// legitimate use, so it does not count as a terminal.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	null, err := os.Stat(os.DevNull)
	if err == nil && os.SameFile(info, null) {
		return false
	}
	return true
}

// run executes nagac with the given arguments and returns the exit status.
// stdoutIsTerminal tells whether stdout is an interactive terminal; the binary
// SPIR-V output is never written there.
func run(args []string, stdout, stderr io.Writer, stdoutIsTerminal bool) int {
	fs := flag.NewFlagSet("nagac", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { usage(fs, stderr) }
	output := fs.String("o", "", "output file (default: stdout)")
	debugFlag := fs.Bool("debug", false, "include debug info")
	validate := fs.Bool("validate", true, "validate IR")
	versionFlag := fs.Bool("version", false, "print version")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage // the flag package has already printed the reason
	}

	if *versionFlag {
		sayf(stdout, "nagac version %s\n", version())
		return exitOK
	}

	rest := fs.Args()
	if len(rest) < 1 {
		sayf(stderr, "Error: no input file specified\n")
		usage(fs, stderr)
		return exitFailure
	}
	if len(rest) > 1 {
		sayf(stderr, "nagac: unexpected arguments after the input file %s: %s\n", rest[0], strings.Join(rest[1:], " "))
		sayf(stderr, "nagac: options must come before the input file, for example: nagac -o out.spv %s\n", rest[0])
		return exitUsage
	}
	inputPath := rest[0]

	if *output == "" && stdoutIsTerminal {
		sayf(stderr, "nagac: refusing to write binary SPIR-V to a terminal; use -o <file> or redirect stdout\n")
		return exitUsage
	}

	source, err := os.ReadFile(inputPath)
	if err != nil {
		sayf(stderr, "Error reading file: %v\n", err)
		return exitFailure
	}

	opts := naga.CompileOptions{
		SPIRV:          spirv.Options{Version: spirv.Version1_3, Debug: *debugFlag},
		SkipValidation: !*validate,
	}
	spirvBytes, err := naga.CompileWithOptions(string(source), opts)
	if err != nil {
		sayf(stderr, "Compilation error: %v\n", err)
		return exitFailure
	}

	if *output != "" {
		if err := os.WriteFile(*output, spirvBytes, 0o644); err != nil {
			sayf(stderr, "Error writing output: %v\n", err)
			return exitFailure
		}
		sayf(stdout, "Successfully compiled %s to %s (%d bytes)\n", inputPath, *output, len(spirvBytes))
		return exitOK
	}
	if _, err := stdout.Write(spirvBytes); err != nil {
		sayf(stderr, "Error writing output: %v\n", err)
		return exitFailure
	}
	return exitOK
}

func usage(fs *flag.FlagSet, w io.Writer) {
	sayf(w, "Usage: nagac [options] <input.wgsl>\n\n")
	sayf(w, "Options come before the input file.\n\n")
	sayf(w, "Options:\n")
	fs.PrintDefaults()
	sayf(w, "\nExamples:\n")
	sayf(w, "  nagac shader.wgsl > shader.spv        Compile to stdout (redirected)\n")
	sayf(w, "  nagac -o shader.spv shader.wgsl       Compile to file\n")
	sayf(w, "  nagac -debug -o shader.spv shader.wgsl  Include debug info\n")
}

// sayf writes a diagnostic. A failed write to stdout or stderr has nowhere
// left to be reported, so the error is dropped on purpose.
func sayf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...) // best effort, see above
}
