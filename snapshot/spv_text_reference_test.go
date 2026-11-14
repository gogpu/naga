package snapshot_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/gogpu/naga"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
)

// spvInputDir holds the SPIR-V corpus sources, their parameters and the
// modules' Rust-derived binaries.
var spvInputDir = filepath.Join(rustCorpusDir, "in", "spv")

// TestSPVTextReference compares the MSL and HLSL text our backends write for
// the SPIR-V corpus with Rust naga's own outputs of the same modules
// (design §17.2). The IR is compared separately by TestSPVIRReference; this
// test proves the backends consume the frontend's IR the way Rust's consume
// its own, on every module Rust has a golden for.
func TestSPVTextReference(t *testing.T) {
	compared := 0
	for _, kind := range []string{"msl", "hlsl"} {
		goldens, err := filepath.Glob(filepath.Join(rustCorpusDir, "out", kind, "spv-*"))
		if err != nil {
			t.Fatal(err)
		}
		extension := map[string]string{"msl": ".metal", "hlsl": ".hlsl"}[kind]
		for _, golden := range goldens {
			if filepath.Ext(golden) != extension {
				continue // the .ron forms are IR snapshots, not text goldens
			}
			name := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(golden), "spv-"), extension)
			t.Run(name+"/"+kind, func(t *testing.T) {
				data, err := os.ReadFile(filepath.Join(rustDerivedDir, "spv", name+".spv"))
				if err != nil {
					t.Fatalf("read %s: %v", name, err)
				}
				// Rust's harness parses with adjust_coordinate_space off unless
				// the shader's TOML asks for it (naga-test's SpirvInParameters
				// defaults to false); our option is the inverse.
				params := filepath.Join(spvInputDir, name+".toml")
				adjust := false
				if content, err := os.ReadFile(params); err == nil {
					adjust = strings.Contains(string(content), "adjust_coordinate_space = true")
				} else if !os.IsNotExist(err) {
					t.Fatal(err)
				}
				module, _, err := spirv.Parse(data, spirv.ParseOptions{KeepCoordinateSpace: !adjust})
				if err != nil {
					t.Fatalf("Parse: %v", err)
				}
				// Rust naga's snapshot harness compacts the module before it
				// writes a text backend (tests/naga/snapshots.rs): the goldens
				// are compacted, and handle-based names follow the compacted
				// arenas.
				naga.Compact(module, naga.KeepUnusedNo)
				want, err := os.ReadFile(golden)
				if err != nil {
					t.Fatal(err)
				}
				var got string
				switch kind {
				case "msl":
					opts, err := readRustMSLConfigFrom(params)
					if err != nil {
						t.Fatalf("MSL options: %v", err)
					}
					got, _, err = msl.Compile(module, opts)
					if err != nil {
						t.Fatalf("msl.Compile: %v", err)
					}
				case "hlsl":
					opts, err := rustHLSLOptionsFrom(params)
					if err != nil {
						t.Fatalf("HLSL options: %v", err)
					}
					got, _, err = hlsl.Compile(module, opts)
					if err != nil {
						t.Fatalf("hlsl.Compile: %v", err)
					}
				}
				// The frontend's expression arena is in ascending id order where
				// Rust naga's is in FxHashMap order (the deliberate deviation
				// TestSPVIRReference pins), so the same expression carries a
				// different _eN number. Rename both sides by first use: the
				// latter is the order the text uses them, which a byte compare
				// keeps honest.
				wantText := normalizeExprNames(string(want))
				gotText := normalizeExprNames(got)
				if gotText == wantText {
					compared++
					return
				}
				t.Errorf("differs from Rust naga:\n%s", firstDiff(wantText, gotText))
			})
		}
	}
	t.Logf("SPIR-V text reference: %d goldens match Rust", compared)
}

// exprNamePattern matches the namer's expression identifiers.
var exprNamePattern = regexp.MustCompile(`\b_e[0-9]+\b`)

// normalizeExprNames renames every _eN identifier to _eK by the order the
// text first mentions it, so two texts that differ only in the numbering of
// the same expressions compare equal.
func normalizeExprNames(text string) string {
	mapping := make(map[string]string)
	return exprNamePattern.ReplaceAllStringFunc(text, func(name string) string {
		if replacement, ok := mapping[name]; ok {
			return replacement
		}
		replacement := "_e" + strconv.Itoa(len(mapping))
		mapping[name] = replacement
		return replacement
	})
}

// firstDiff returns the first lines where want and got differ, with the
// surrounding context, for a readable failure.
func firstDiff(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	var b strings.Builder
	shown := 0
	for i := 0; i < len(wantLines) && i < len(gotLines) && shown < 12; i++ {
		if wantLines[i] == gotLines[i] {
			continue
		}
		fmt.Fprintf(&b, "line %d:\n  rust: %s\n  ours: %s\n", i+1, wantLines[i], gotLines[i])
		shown++
	}
	if len(wantLines) != len(gotLines) {
		fmt.Fprintf(&b, "lines: rust=%d ours=%d\n", len(wantLines), len(gotLines))
	}
	return b.String()
}
