//go:build ignore

// gen writes enums.go from the vendored SPIR-V grammars:
//
//	go generate ./spirv/internal/spvenums
//
// or, with explicit files:
//
//	go run gen.go -core <spirv.core.grammar.json> -extinst <extinst.glsl.std.450.grammar.json> -out enums.go
//
// The generation lives in the gen package, so the drift test
// (TestEnumsMatchVendoredGrammar) runs exactly the same code on the vendored
// copies. The generated header names the headers revision it came from.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/gogpu/naga/spirv/internal/spvenums/gen"
)

func main() {
	corePath := flag.String("core", "", "path to spirv.core.grammar.json of the SPIR-V headers")
	extinstPath := flag.String("extinst", "", "path to extinst.glsl.std.450.grammar.json of the SPIR-V headers")
	outPath := flag.String("out", "enums.go", "Go file to write")
	flag.Parse()
	if *corePath == "" || *extinstPath == "" {
		fmt.Fprintln(os.Stderr, "gen: -core and -extinst are required")
		os.Exit(2)
	}
	if err := run(*corePath, *extinstPath, *outPath); err != nil {
		fmt.Fprintf(os.Stderr, "gen: %v\n", err)
		os.Exit(1)
	}
}

// run generates the enums from the two grammars and writes them to outPath.
func run(corePath, extinstPath, outPath string) error {
	core, err := os.Open(corePath)
	if err != nil {
		return err
	}
	defer func() { _ = core.Close() }() // read-only
	extinst, err := os.Open(extinstPath)
	if err != nil {
		return err
	}
	defer func() { _ = extinst.Close() }() // read-only

	src, report, err := gen.Generate(core, extinst)
	if err != nil {
		return err
	}
	fmt.Printf("grammar: %d opcodes, %d enumerants, %d aliases; GLSL.std.450: %d instructions\n",
		report.Opcodes, report.Enumerants, report.Aliases, report.GlslStd450)
	return os.WriteFile(outPath, src, 0o644)
}
