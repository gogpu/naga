package spvenums

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogpu/naga/spirv/internal/spvenums/gen"
)

// TestEnumsMatchVendoredGrammar regenerates enums.go from the vendored
// grammars and compares it with the committed file. The test is hermetic —
// the vendored copies travel with the module, so CI on every OS runs it — and
// it is what makes a stale or hand-edited file a failure instead of a silent
// drift from the specification.
func TestEnumsMatchVendoredGrammar(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "testdata", "upstream", "github.com", "KhronosGroup", "SPIRV-Headers")
	core, err := os.Open(filepath.Join(dir, "spirv.core.grammar.json"))
	if err != nil {
		t.Fatalf("vendored grammar: %v", err)
	}
	defer func() { _ = core.Close() }() // read-only
	extinst, err := os.Open(filepath.Join(dir, "extinst.glsl.std.450.grammar.json"))
	if err != nil {
		t.Fatalf("vendored GLSL.std.450 grammar: %v", err)
	}
	defer func() { _ = extinst.Close() }() // read-only

	got, report, err := gen.Generate(core, extinst)
	if err != nil {
		t.Fatalf("generate from the vendored grammar: %v", err)
	}
	if report.Opcodes == 0 || report.Enumerants == 0 || report.GlslStd450 == 0 {
		t.Fatalf("generator read %+v, want all three grammars parsed", report)
	}

	want, err := os.ReadFile("enums.go")
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
			t.Fatalf("enums.go is stale at line %d:\n  generated: %s\n  committed: %s\nrun go generate ./spirv/internal/spvenums", i+1, g, w)
		}
	}
	t.Fatal("enums.go differs from the generated source")
}

// TestEnumPins pins a few numbers and spellings the backend and the frontend
// depend on, the alias resolution, and the String fallback.
func TestEnumPins(t *testing.T) {
	if OpTypeVoid != 19 || OpEntryPoint != 15 || OpTypePointer != 32 {
		t.Errorf("opcodes = %d, %d, %d, want 19, 15, 32", OpTypeVoid, OpEntryPoint, OpTypePointer)
	}
	if CapabilityShader != 1 || CapabilityInt64 != 11 {
		t.Errorf("capabilities = %d, %d, want 1, 11", CapabilityShader, CapabilityInt64)
	}
	if BuiltInPosition != 0 || DecorationBuiltIn != 11 {
		t.Errorf("built-in %d, decoration %d, want 0, 11", BuiltInPosition, DecorationBuiltIn)
	}
	if GlslStd450Round != 1 || GlslStd450Fma != 50 {
		t.Errorf("GLSL.std.450 = %d, %d, want 1, 50", GlslStd450Round, GlslStd450Fma)
	}
	if got := OpTypeVoid.String(); got != "OpTypeVoid" {
		t.Errorf("OpTypeVoid.String() = %q", got)
	}
	if got := Op(9999).String(); got != "Op(9999)" {
		t.Errorf("Op(9999).String() = %q", got)
	}
}

// TestAliasResolves pins that an alias names the same value as its canonical
// enumerant, which is how the specification spells the alternatives.
func TestAliasResolves(t *testing.T) {
	if MemorySemanticsNone != MemorySemanticsRelaxed {
		t.Errorf("MemorySemanticsNone = %d, MemorySemanticsRelaxed = %d, want one value",
			MemorySemanticsNone, MemorySemanticsRelaxed)
	}
	if got := MemorySemanticsRelaxed.String(); got != "MemorySemanticsRelaxed" {
		t.Errorf("MemorySemanticsRelaxed.String() = %q, want the canonical name", got)
	}
}
