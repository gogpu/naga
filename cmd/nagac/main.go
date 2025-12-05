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
//	nagac -emit-ast shader.wgsl          # Print AST
package main

import (
	"flag"
	"fmt"
	"os"
)

var (
	output   = flag.String("o", "", "output file path")
	emitAST  = flag.Bool("emit-ast", false, "print AST")
	emitIR   = flag.Bool("emit-ir", false, "print IR")
	validate = flag.Bool("validate", true, "validate shader")
	version  = flag.Bool("version", false, "print version")
)

const nagaVersion = "0.1.0-dev"

func main() {
	flag.Usage = usage
	flag.Parse()

	if *version {
		fmt.Printf("nagac version %s\n", nagaVersion)
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) < 1 {
		usage()
		os.Exit(1)
	}

	inputPath := args[0]

	// Read input file
	source, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", inputPath, err)
		os.Exit(1)
	}

	fmt.Printf("nagac: Compiling %s (%d bytes)\n", inputPath, len(source))
	fmt.Println("")
	fmt.Println("Note: Full compilation is not yet implemented.")
	fmt.Println("Currently available:")
	fmt.Println("  - Lexical analysis (tokenization)")
	fmt.Println("  - AST type definitions")
	fmt.Println("")
	fmt.Println("Coming soon:")
	fmt.Println("  - Full WGSL parsing")
	fmt.Println("  - Semantic analysis")
	fmt.Println("  - IR generation")
	fmt.Println("  - SPIR-V output")
	fmt.Println("")
	fmt.Printf("Options: -o=%q -emit-ast=%v -emit-ir=%v -validate=%v\n",
		*output, *emitAST, *emitIR, *validate)
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: nagac [options] <input>\n\n")
	fmt.Fprintf(os.Stderr, "Options:\n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nExamples:\n")
	fmt.Fprintf(os.Stderr, "  nagac shader.wgsl               Parse and validate\n")
	fmt.Fprintf(os.Stderr, "  nagac -o shader.spv shader.wgsl Compile to SPIR-V\n")
	fmt.Fprintf(os.Stderr, "  nagac -emit-ast shader.wgsl     Print AST\n")
}
