// Command nagac is the naga shader compiler CLI.
//
// Usage:
//
//	nagac [options] <input>
//
// Examples:
//
//	nagac shader.wgsl                    # Parse and validate
//	nagac -o shader.spv shader.wgsl      # Compile to SPIR-V
//	nagac -debug shader.wgsl             # Compile with debug info
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/gogpu/naga"
)

var (
	output   = flag.String("o", "", "output file (default: stdout)")
	debug    = flag.Bool("debug", false, "include debug info")
	validate = flag.Bool("validate", true, "validate IR")
	version  = flag.Bool("version", false, "print version")
)

const nagaVersion = "0.1.0-dev"

func main() {
	flag.Usage = usage
	flag.Parse()

	if *version {
		fmt.Printf("nagac version %s\n", nagaVersion)
		return
	}

	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: no input file specified")
		usage()
		os.Exit(1)
	}

	inputPath := args[0]

	// Read input file
	source, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	// Compile WGSL to SPIR-V
	opts := naga.CompileOptions{
		Debug:    *debug,
		Validate: *validate,
	}
	spirvBytes, err := naga.CompileWithOptions(string(source), opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Compilation error: %v\n", err)
		os.Exit(1)
	}

	// Write output
	if *output != "" {
		err = os.WriteFile(*output, spirvBytes, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Successfully compiled %s to %s (%d bytes)\n", inputPath, *output, len(spirvBytes))
	} else {
		_, err = os.Stdout.Write(spirvBytes)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
			os.Exit(1)
		}
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: nagac [options] <input.wgsl>\n\n")
	fmt.Fprintf(os.Stderr, "Options:\n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nExamples:\n")
	fmt.Fprintf(os.Stderr, "  nagac shader.wgsl               Compile to stdout\n")
	fmt.Fprintf(os.Stderr, "  nagac -o shader.spv shader.wgsl Compile to file\n")
	fmt.Fprintf(os.Stderr, "  nagac -debug shader.wgsl        Include debug info\n")
}
