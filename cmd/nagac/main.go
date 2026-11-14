// Command nagac is the naga shader compiler CLI.
//
// Usage:
//
//	nagac [options] <input>
//
// The input is WGSL, or a SPIR-V binary with -from-spirv. Options must come
// before the input file: Go's flag package stops parsing at the first non-flag
// argument, so nagac rejects anything that follows the input file instead of
// silently ignoring it.
//
// Examples:
//
//	nagac shader.wgsl                          # Compile to SPIR-V on stdout (not to a terminal)
//	nagac -o shader.spv shader.wgsl            # Compile to SPIR-V in a file
//	nagac -backend msl shader.wgsl             # Translate WGSL to Metal Shading Language
//	nagac -from-spirv -backend msl shader.spv  # Translate a SPIR-V binary to MSL
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
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/valid"
	"github.com/gogpu/naga/wgsl"
)

// Exit statuses. A command line the tool cannot interpret is a usage error
// (2, as the flag package and go vet use); a failure while working is 1.
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

// The output backends -backend accepts. The zero value of the flag is
// backendSPIRV, the tool's historical output.
const (
	backendSPIRV = "spirv"
	backendMSL   = "msl"
	backendHLSL  = "hlsl"
	backendGLSL  = "glsl"
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
	debugFlag := fs.Bool("debug", false, "include debug info in the SPIR-V output")
	validate := fs.Bool("validate", true, "validate IR")
	versionFlag := fs.Bool("version", false, "print version")
	fromSPIRV := fs.Bool("from-spirv", false, "read the input as a SPIR-V binary instead of WGSL")
	backendName := fs.String("backend", backendSPIRV, "output backend: spirv, msl, hlsl or glsl")

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

	if !knownBackend(*backendName) {
		sayf(stderr, "nagac: unknown backend %q; use spirv, msl, hlsl or glsl\n", *backendName)
		return exitUsage
	}
	// The SPIR-V output is a binary; a terminal would garble it. A text
	// backend may write to one.
	if *backendName == backendSPIRV && *output == "" && stdoutIsTerminal {
		sayf(stderr, "nagac: refusing to write binary SPIR-V to a terminal; use -o <file> or redirect stdout\n")
		return exitUsage
	}

	source, err := os.ReadFile(inputPath)
	if err != nil {
		sayf(stderr, "Error reading file: %v\n", err)
		return exitFailure
	}

	module, err := parseInput(source, *fromSPIRV)
	if err != nil {
		sayf(stderr, "Compilation error: %v\n", err)
		return exitFailure
	}
	var info valid.ModuleInfo
	if *validate {
		info, err = valid.Validate(module, valid.Options{})
		if err != nil {
			sayf(stderr, "Compilation error: validation failed: %v\n", err)
			return exitFailure
		}
	}
	module, _, err = naga.ProcessOverrides(module, info, naga.OverrideOptions{})
	if err != nil {
		sayf(stderr, "Compilation error: pipeline constants: %v\n", err)
		return exitFailure
	}

	data, err := emit(module, *backendName, *debugFlag)
	if err != nil {
		sayf(stderr, "Compilation error: %v\n", err)
		return exitFailure
	}

	return writeOutput(inputPath, *output, data, stdout, stderr)
}

// writeOutput writes the compiled data to the output file, or to stdout when
// no file was given.
func writeOutput(inputPath, outputPath string, data []byte, stdout, stderr io.Writer) int {
	if outputPath != "" {
		if err := os.WriteFile(outputPath, data, 0o644); err != nil {
			sayf(stderr, "Error writing output: %v\n", err)
			return exitFailure
		}
		sayf(stdout, "Successfully compiled %s to %s (%d bytes)\n", inputPath, outputPath, len(data))
		return exitOK
	}
	if _, err := stdout.Write(data); err != nil {
		sayf(stderr, "Error writing output: %v\n", err)
		return exitFailure
	}
	return exitOK
}

// parseInput reads the input with the selected frontend: the SPIR-V binary
// parser, or the WGSL parser, whose errors name the source location.
func parseInput(source []byte, fromSPIRV bool) (*ir.Module, error) {
	if fromSPIRV {
		module, _, err := spirv.Parse(source, spirv.ParseOptions{})
		if err != nil {
			return nil, fmt.Errorf("spirv: %w", err)
		}
		return module, nil
	}
	module, _, err := wgsl.Parse(string(source), wgsl.ParseOptions{})
	if err != nil {
		var parseErr *wgsl.ParseError
		if errors.As(err, &parseErr) {
			if loc, ok := parseErr.Location(string(source)); ok {
				return nil, fmt.Errorf("wgsl:%d:%d: %w", loc.Line, loc.Column, err)
			}
		}
		return nil, fmt.Errorf("wgsl: %w", err)
	}
	return module, nil
}

// knownBackend reports whether name is one of the output backends.
func knownBackend(name string) bool {
	switch name {
	case backendSPIRV, backendMSL, backendHLSL, backendGLSL:
		return true
	}
	return false
}

// emit translates the module with the selected backend. The zero value of
// every text backend's options is the default the library documents; the
// SPIR-V output keeps the version nagac has always written.
func emit(module *ir.Module, backendName string, emitDebug bool) ([]byte, error) {
	switch backendName {
	case backendSPIRV:
		data, _, err := spirv.Compile(module, spirv.Options{Version: spirv.Version1_3, Debug: emitDebug})
		if err != nil {
			return nil, fmt.Errorf("SPIR-V generation error: %w", err)
		}
		return data, nil
	case backendMSL:
		text, _, err := msl.Compile(module, msl.Options{})
		if err != nil {
			return nil, fmt.Errorf("MSL generation error: %w", err)
		}
		return []byte(text), nil
	case backendHLSL:
		text, _, err := hlsl.Compile(module, hlsl.Options{})
		if err != nil {
			return nil, fmt.Errorf("HLSL generation error: %w", err)
		}
		return []byte(text), nil
	case backendGLSL:
		text, _, err := glsl.Compile(module, glsl.Options{})
		if err != nil {
			return nil, fmt.Errorf("GLSL generation error: %w", err)
		}
		return []byte(text), nil
	}
	return nil, fmt.Errorf("unknown backend %q", backendName)
}

func usage(fs *flag.FlagSet, w io.Writer) {
	sayf(w, "Usage: nagac [options] <input>\n\n")
	sayf(w, "The input is WGSL, or a SPIR-V binary with -from-spirv.\n\n")
	sayf(w, "Options come before the input file.\n\n")
	sayf(w, "Options:\n")
	fs.PrintDefaults()
	sayf(w, "\nExamples:\n")
	sayf(w, "  nagac shader.wgsl                          Compile to stdout (redirected)\n")
	sayf(w, "  nagac -o shader.spv shader.wgsl            Compile to file\n")
	sayf(w, "  nagac -backend msl shader.wgsl             Translate WGSL to MSL\n")
	sayf(w, "  nagac -from-spirv -backend msl shader.spv  Translate a SPIR-V binary to MSL\n")
}

// sayf writes a diagnostic. A failed write to stdout or stderr has nowhere
// left to be reported, so the error is dropped on purpose.
func sayf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...) // best effort, see above
}
