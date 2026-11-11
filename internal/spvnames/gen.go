//go:build ignore

// gen writes table.go from the vendored SPIR-V headers:
//
//	go generate ./internal/spvnames
//
// or, with explicit files:
//
//	go run gen.go -header <spirv.h> -grammar <spirv.core.grammar.json> -out table.go
//
// The generation and the cross-check live in the gen package, so the drift
// test (TestTableMatchesVendoredHeaders) runs exactly the same code on the
// vendored copy. A capability the table does not know prints as a bare number,
// and its name is rejected as unknown; the generated header names the headers
// revision it came from.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/gogpu/naga/internal/spvnames/gen"
)

func main() {
	headerPath := flag.String("header", "", "path to spirv.h of the SPIR-V headers")
	grammarPath := flag.String("grammar", "", "path to spirv.core.grammar.json of the SPIR-V headers")
	outPath := flag.String("out", "table.go", "Go file to write")
	flag.Parse()
	if *headerPath == "" || *grammarPath == "" {
		fmt.Fprintln(os.Stderr, "gen: -header and -grammar are required")
		os.Exit(2)
	}
	if err := run(*headerPath, *grammarPath, *outPath); err != nil {
		fmt.Fprintf(os.Stderr, "gen: %v\n", err)
		os.Exit(1)
	}
}

// run generates the table from the two header files and writes it to outPath.
func run(headerPath, grammarPath, outPath string) error {
	header, err := os.Open(headerPath)
	if err != nil {
		return err
	}
	defer func() { _ = header.Close() }() // read-only
	grammar, err := os.Open(grammarPath)
	if err != nil {
		return err
	}
	defer func() { _ = grammar.Close() }() // read-only

	src, report, err := gen.Generate(header, grammar)
	if err != nil {
		return err
	}
	fmt.Printf("grammar: %d capabilities, %d aliases; spirv.h: %d names; mismatches: 0\n",
		report.Capabilities, report.Aliases, report.HeaderNames)
	return os.WriteFile(outPath, src, 0o644)
}
