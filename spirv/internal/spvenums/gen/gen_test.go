package gen

import (
	"encoding/json"
	"strings"
	"testing"
)

func enumerantOf(name, value string, aliases ...string) enumerant {
	return enumerant{Enumerant: name, Value: json.RawMessage(value), Aliases: aliases}
}

// TestCollectKindRejectsDuplicateValue: two enumerants sharing a value is the
// ambiguity the generator refuses; the specification shares values only
// through aliases.
func TestCollectKindRejectsDuplicateValue(t *testing.T) {
	_, err := collectKind(operandKind{Category: "ValueEnum", Kind: "Dim", Enumerants: []enumerant{
		enumerantOf("2D", "1"),
		enumerantOf("3D", "1"),
	}})
	if err == nil || !strings.Contains(err.Error(), "share value 1") {
		t.Fatalf("err = %v, want a refusal naming the shared value", err)
	}
}

// TestCollectKindRecordsAliases: an alias is a second name for one value, and
// the canonical name stays the one String prints.
func TestCollectKindRecordsAliases(t *testing.T) {
	et, err := collectKind(operandKind{Category: "ValueEnum", Kind: "MemorySemantics", Enumerants: []enumerant{
		enumerantOf("Relaxed", `"0x0000"`, "None"),
		enumerantOf("Acquire", `"0x0002"`),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(et.Constants) != 3 {
		t.Errorf("constants = %d, want 3 (canonical Relaxed, alias None, canonical Acquire)", len(et.Constants))
	}
	if got := et.Names[0]; got != "MemorySemanticsRelaxed" {
		t.Errorf("names[0] = %q, want the canonical name", got)
	}
	found := false
	for _, c := range et.Constants {
		if c.Name == "MemorySemanticsNone" && c.Value == 0 {
			found = true
		}
	}
	if !found {
		t.Error("the alias MemorySemanticsNone is not recorded at value 0")
	}
}

// TestCollectKindRejectsBadInput: an unknown category, a name that is not a Go
// identifier and a value beyond 32 bits are all refused, never skipped.
func TestCollectKindRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		kind operandKind
		want string
	}{
		{
			name: "unknown category",
			kind: operandKind{Category: "Enum", Kind: "Dim", Enumerants: []enumerant{enumerantOf("2D", "1")}},
			want: "unsupported category",
		},
		{
			name: "invalid identifier",
			kind: operandKind{Category: "ValueEnum", Kind: "Dim", Enumerants: []enumerant{enumerantOf("Bad-Name", "1")}},
			want: "not a Go identifier",
		},
		{
			name: "value out of range",
			kind: operandKind{Category: "ValueEnum", Kind: "Dim", Enumerants: []enumerant{enumerantOf("TooBig", `"0x100000000"`)}},
			want: "does not fit 32 bits",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := collectKind(tt.kind)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want a refusal mentioning %q", err, tt.want)
			}
		})
	}
}

// TestGenerateRejectsExtInstVersion: the GLSL.std.450 grammar carries its own
// version; a different file must not be mistaken for the pinned one.
func TestGenerateRejectsExtInstVersion(t *testing.T) {
	// Every generated kind must be present before the extinst grammar is
	// read, so the fake core carries them all.
	var core strings.Builder
	core.WriteString(`{"instructions":[{"opname":"OpNop","opcode":0}],"operand_kinds":[`)
	for i, kind := range kinds {
		if i > 0 {
			core.WriteString(",")
		}
		core.WriteString(`{"category":"ValueEnum","kind":"` + kind + `","enumerants":[]}`)
	}
	core.WriteString(`]}`)
	extinst := `{"version":99,"instructions":[]}`
	_, _, err := Generate(strings.NewReader(core.String()), strings.NewReader(extinst))
	if err == nil || !strings.Contains(err.Error(), "version 99") {
		t.Fatalf("err = %v, want a refusal naming the extinst version", err)
	}
}
