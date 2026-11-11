package gen

import (
	"strings"
	"testing"
)

// grammarJSON builds a one-capability-kind grammar with the given enumerants.
func grammarJSON(enumerants string) string {
	return `{"operand_kinds":[{"kind":"Capability","enumerants":[` + enumerants + `]}]}`
}

// headerEnum builds a spirv.h carrying the given enum lines.
func headerEnum(lines string) string {
	return "typedef enum SpvCapability_ {\n" + lines + "} SpvCapability;\n"
}

// TestGenerateAgrees covers the agreeing case: the returned source is
// formatted Go with the entries in number order, and the report counts what
// the files hold.
func TestGenerateAgrees(t *testing.T) {
	grammar := grammarJSON(`{"enumerant":"Shader","value":1},
		{"enumerant":"Matrix","value":0,"aliases":["Zeta","Alpha"]}`)
	header := headerEnum("\tSpvCapabilityMatrix = 0,\n\tSpvCapabilityAlpha = 0,\n\tSpvCapabilityZeta = 0,\n\tSpvCapabilityShader = 1,\n\tSpvCapabilityMax = 0x7fffffff\n")
	src, report, err := Generate(strings.NewReader(header), strings.NewReader(grammar))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if want := (Report{Capabilities: 2, Aliases: 2, HeaderNames: 4}); report != want {
		t.Errorf("report = %+v, want %+v", report, want)
	}
	for _, want := range []string{
		`{value: 0, name: "Matrix", aliases: []string{"Alpha", "Zeta"}},`,
		`{value: 1, name: "Shader"},`,
		HeadersRevision,
	} {
		if !strings.Contains(string(src), want) {
			t.Errorf("source does not contain %q:\n%s", want, src)
		}
	}
	if strings.Index(string(src), `name: "Matrix"`) > strings.Index(string(src), `name: "Shader"`) {
		t.Error("capabilities are not in number order")
	}
}

// TestGenerateRejectsBadGrammars covers the grammar errors: a document that
// is not JSON, a value that is not a number, and a grammar without the
// Capability operand kind.
func TestGenerateRejectsBadGrammars(t *testing.T) {
	for _, tt := range []struct {
		name    string
		grammar string
		want    string
	}{
		{"not json", `{`, "grammar:"},
		{"value not a number", grammarJSON(`{"enumerant":"Shader","value":"one"}`), `capability "Shader" value`},
		{"no capability kind", `{"operand_kinds":[{"kind":"MemoryModel","enumerants":[]}]}`, "no Capability operand kind"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := Generate(strings.NewReader(headerEnum("SpvCapabilityMax = 0\n")), strings.NewReader(tt.grammar))
			if err == nil {
				t.Fatal("Generate accepted the grammar")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not contain %q", err, tt.want)
			}
		})
	}
}

// TestGenerateRejectsBadHeaders covers the spirv.h errors: a file without the
// SpvCapability enum and an enum value too large for a capability number.
func TestGenerateRejectsBadHeaders(t *testing.T) {
	grammar := grammarJSON(`{"enumerant":"Shader","value":1}`)
	for _, tt := range []struct {
		name   string
		header string
		want   string
	}{
		{"no enum", "typedef enum SpvOther_ {\n} SpvOther;\n", "no SpvCapability enum"},
		{"value overflows", headerEnum("SpvCapabilityShader = 0x1FFFFFFFF\n"), "spirv.h:2"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := Generate(strings.NewReader(tt.header), strings.NewReader(grammar))
			if err == nil {
				t.Fatal("Generate accepted the header")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not contain %q", err, tt.want)
			}
		})
	}
}

// TestGenerateRejectsDisagreeingFiles covers every cross-check of check():
// a number with two capabilities, a repeated name, a capability missing from
// spirv.h, a number mismatch, an alias missing from spirv.h, an alias at
// another number, a repeated alias, and a spirv.h name the grammar lacks.
func TestGenerateRejectsDisagreeingFiles(t *testing.T) {
	for _, tt := range []struct {
		name    string
		grammar string
		header  string
		want    string
	}{
		{
			name:    "two capabilities share a number",
			grammar: grammarJSON(`{"enumerant":"A","value":1},{"enumerant":"B","value":1}`),
			header:  headerEnum("SpvCapabilityA = 1,\nSpvCapabilityB = 1,\n"),
			want:    "1 has two capabilities: A and B",
		},
		{
			name:    "repeated capability name",
			grammar: grammarJSON(`{"enumerant":"A","value":1},{"enumerant":"A","value":2}`),
			header:  headerEnum("SpvCapabilityA = 1,\n"),
			want:    "capability name A appears twice",
		},
		{
			name:    "capability missing from spirv.h",
			grammar: grammarJSON(`{"enumerant":"A","value":1}`),
			header:  headerEnum("SpvCapabilityMax = 1,\n"),
			want:    "A (1) is not in spirv.h",
		},
		{
			name:    "number mismatch",
			grammar: grammarJSON(`{"enumerant":"A","value":1}`),
			header:  headerEnum("SpvCapabilityA = 2,\n"),
			want:    "A: grammar 1, spirv.h 2",
		},
		{
			name:    "alias missing from spirv.h",
			grammar: grammarJSON(`{"enumerant":"A","value":1,"aliases":["B"]}`),
			header:  headerEnum("SpvCapabilityA = 1,\n"),
			want:    "alias B of A is not in spirv.h",
		},
		{
			name:    "alias at another number",
			grammar: grammarJSON(`{"enumerant":"A","value":1,"aliases":["B"]}`),
			header:  headerEnum("SpvCapabilityA = 1,\nSpvCapabilityB = 2,\n"),
			want:    "alias B: grammar 1, spirv.h 2",
		},
		{
			name:    "repeated alias",
			grammar: grammarJSON(`{"enumerant":"A","value":1},{"enumerant":"C","value":2,"aliases":["A"]}`),
			header:  headerEnum("SpvCapabilityA = 2,\nSpvCapabilityC = 2,\n"),
			want:    "alias A of C appears twice",
		},
		{
			name:    "spirv.h name missing from the grammar",
			grammar: grammarJSON(`{"enumerant":"A","value":1}`),
			header:  headerEnum("SpvCapabilityA = 1,\nSpvCapabilityC = 3,\n"),
			want:    "C (3) of spirv.h is not in the grammar",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := Generate(strings.NewReader(tt.header), strings.NewReader(tt.grammar))
			if err == nil {
				t.Fatal("Generate accepted disagreeing files")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not contain %q", err, tt.want)
			}
		})
	}
}
