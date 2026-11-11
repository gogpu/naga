package spvnames

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogpu/naga/internal/spvnames/gen"
)

// TestTableMatchesVendoredHeaders regenerates the capability table from the
// vendored SPIR-V headers and compares it with the committed table.go. The
// test is hermetic — the vendored copy travels with the module, so CI on every
// OS and every contributor runs it — and it is what makes a stale or
// hand-edited table a failure instead of a silent drift from the
// specification. It also runs the grammar/spirv.h cross-check of gen.Generate,
// so headers whose two files disagree fail here too.
func TestTableMatchesVendoredHeaders(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "upstream", "github.com", "KhronosGroup", "SPIRV-Headers")
	header, err := os.Open(filepath.Join(dir, "spirv.h"))
	if err != nil {
		t.Fatalf("vendored spirv.h: %v", err)
	}
	defer func() { _ = header.Close() }() // read-only
	grammar, err := os.Open(filepath.Join(dir, "spirv.core.grammar.json"))
	if err != nil {
		t.Fatalf("vendored grammar: %v", err)
	}
	defer func() { _ = grammar.Close() }() // read-only

	got, report, err := gen.Generate(header, grammar)
	if err != nil {
		t.Fatalf("generate from the vendored headers: %v", err)
	}
	if report.Capabilities != len(capabilityTable) {
		t.Errorf("grammar has %d capabilities, table.go lists %d", report.Capabilities, len(capabilityTable))
	}
	aliases := 0
	for _, c := range capabilityTable {
		aliases += len(c.aliases)
	}
	if report.Aliases != aliases {
		t.Errorf("grammar has %d aliases, table.go lists %d", report.Aliases, aliases)
	}

	want, err := os.ReadFile("table.go")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(got, want) {
		return
	}
	gotLines := bytes.Split(got, []byte("\n"))
	wantLines := bytes.Split(want, []byte("\n"))
	for i := 0; i < len(gotLines) || i < len(wantLines); i++ {
		var g, w string
		if i < len(gotLines) {
			g = string(gotLines[i])
		}
		if i < len(wantLines) {
			w = string(wantLines[i])
		}
		if g != w {
			t.Fatalf("table.go is stale at line %d:\n  generated: %s\n  committed: %s\nrun go generate ./internal/spvnames", i+1, g, w)
		}
	}
	t.Fatal("table.go differs from the generated source")
}

// TestGenerateRejectsDisagreeingHeaders keeps the cross-check the generator
// documents: a grammar capability that spirv.h does not have (or numbers
// differently) must fail generation.
func TestGenerateRejectsDisagreeingHeaders(t *testing.T) {
	const grammar = `{"operand_kinds":[{"kind":"Capability","enumerants":[
		{"enumerant":"Matrix","value":0},
		{"enumerant":"Shader","value":1}]}]}`
	const header = `
typedef enum SpvCapability_ {
    SpvCapabilityMatrix = 0,
    SpvCapabilityShader = 2,
    SpvCapabilityMax = 0x7fffffff
} SpvCapability;
`
	_, _, err := gen.Generate(bytes.NewBufferString(header), bytes.NewBufferString(grammar))
	if err == nil {
		t.Fatal("Generate accepted headers that disagree")
	}
	if want := "Shader: grammar 1, spirv.h 2"; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not name the mismatch (%q)", err, want)
	}
}
