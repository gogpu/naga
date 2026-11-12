package front

import (
	"errors"
	"strings"
	"testing"

	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// metadataCase is one module-level metadata instruction with its operands and
// the section it belongs to: an instruction of the capability or extension
// section goes before the memory model.
type metadataCase struct {
	name     string
	op       spvenums.Op
	operands []uint32
	early    bool
}

// metadataCases lists every module-level metadata handler with valid operands.
func metadataCases() []metadataCase {
	withString := func(words []uint32, s string) []uint32 {
		return append(append([]uint32{}, words...), stringWords(s)...)
	}
	return []metadataCase{
		{"OpCapability", spvenums.OpCapability, []uint32{uint32(spvenums.CapabilityShader)}, true},
		{"OpExtension", spvenums.OpExtension, withString(nil, "SPV_KHR_storage_buffer_storage_class"), true},
		{"OpExtInstImport", spvenums.OpExtInstImport, withString([]uint32{40}, "GLSL.std.450"), true},
		{"OpEntryPoint", spvenums.OpEntryPoint, withString([]uint32{uint32(spvenums.ExecutionModelGLCompute), 1}, "main"), false},
		{"OpString", spvenums.OpString, withString([]uint32{5}, "a.wgsl"), false},
		{"OpSource", spvenums.OpSource, []uint32{2, 450}, false},
		{"OpSourceExtension", spvenums.OpSourceExtension, withString(nil, "ext"), false},
		{"OpName", spvenums.OpName, withString([]uint32{7}, "x"), false},
		{"OpMemberName", spvenums.OpMemberName, withString([]uint32{7, 0}, "x"), false},
		{"OpModuleProcessed", spvenums.OpModuleProcessed, withString(nil, "done"), false},
		{"OpDecorate", spvenums.OpDecorate, []uint32{7, uint32(spvenums.DecorationLocation), 0}, false},
		{"OpMemberDecorate", spvenums.OpMemberDecorate, []uint32{7, 0, uint32(spvenums.DecorationOffset), 0}, false},
	}
}

// TestMetadataOperandReadsAreGuarded cuts every metadata instruction after
// each of its operand words: the handler must refuse it, never read past the
// input.
func TestMetadataOperandReadsAreGuarded(t *testing.T) {
	for _, tc := range metadataCases() {
		for cut := 0; cut < len(tc.operands); cut++ {
			words := []uint32{uint32(len(tc.operands)+1)<<16 | uint32(tc.op)}
			words = append(words, tc.operands[:cut]...)
			t.Run(tc.name+"/cut"+itoaTest(uint32(cut)), func(t *testing.T) {
				groups := [][]uint32{
					headerWords(),
					instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
				}
				if !tc.early {
					groups = append(groups, instWords(spvenums.OpMemoryModel, 0, 1))
				}
				groups = append(groups, words)
				_, _, err := parseWords(t, Options{}, groups...)
				var parseErr *Error
				if !errors.As(err, &parseErr) {
					t.Fatalf("error %v (%T), want a *Error", err, err)
				}
			})
		}
	}
}

// TestMetadataSectionOrder places every metadata instruction after the type
// section: SPIR-V's fixed section order makes each one an unsupported
// instruction there, as Rust naga's Frontend::switch reports.
func TestMetadataSectionOrder(t *testing.T) {
	cases := append(metadataCases(),
		metadataCase{"OpMemoryModel", spvenums.OpMemoryModel, []uint32{0, 1}, true},
		metadataCase{"OpExecutionMode", spvenums.OpExecutionMode, []uint32{1, uint32(spvenums.ExecutionModeLocalSize), 1, 1, 1}, false},
	)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := parseWords(t, Options{},
				headerWords(),
				instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
				instWords(spvenums.OpMemoryModel, 0, 1),
				instWords(spvenums.OpTypeVoid, 3),
				instWords(tc.op, tc.operands...),
			)
			want := "unsupported instruction " + strings.TrimPrefix(tc.op.String(), "Op") + " at Type"
			if got := requireError(t, err).Message; got != want {
				t.Errorf("message = %q, want %q", got, want)
			}
		})
	}
}

// TestModuleStateNames pins the state names the section-order error prints,
// Rust naga's Debug spelling of ModuleState.
func TestModuleStateNames(t *testing.T) {
	want := map[moduleState]string{
		stateEmpty:         "Empty",
		stateCapability:    "Capability",
		stateExtension:     "Extension",
		stateMemoryModel:   "MemoryModel",
		stateEntryPoint:    "EntryPoint",
		stateExecutionMode: "ExecutionMode",
		stateSource:        "Source",
		stateName:          "Name",
		stateAnnotation:    "Annotation",
		stateType:          "Type",
		stateFunction:      "Function",
	}
	for state, name := range want {
		if got := state.String(); got != name {
			t.Errorf("state %d = %q, want %q", state, got, name)
		}
	}
	if got := moduleState(200).String(); got != "moduleState(200)" {
		t.Errorf("unknown state = %q, want moduleState(200)", got)
	}
}

// TestModuleExtInstGuards covers the module-level OpExtInst skip's own
// refusals: too few words, and every truncation before the set id.
func TestModuleExtInstGuards(t *testing.T) {
	prelude := [][]uint32{
		headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWithString(spvenums.OpExtInstImport, []uint32{40}, "NonSemantic.Shader.DebugInfo.100"),
		instWords(spvenums.OpMemoryModel, 0, 1),
	}
	t.Run("too few words", func(t *testing.T) {
		groups := append(append([][]uint32{}, prelude...), instWords(spvenums.OpExtInst, 1, 2))
		_, _, err := parseWords(t, Options{}, groups...)
		if got := requireError(t, err).Message; got != "invalid operand count 3 for ExtInst" {
			t.Errorf("message = %q", got)
		}
	})
	for _, operands := range [][]uint32{{}, {1}, {1, 2}, {1, 2, 40}} {
		words := append([]uint32{uint32(6)<<16 | uint32(spvenums.OpExtInst)}, operands...)
		t.Run("cut"+itoaTest(uint32(len(operands))), func(t *testing.T) {
			groups := append(append([][]uint32{}, prelude...), words)
			_, _, err := parseWords(t, Options{}, groups...)
			if got := requireError(t, err).Message; got != "incomplete data" {
				t.Errorf("message = %q, want incomplete data", got)
			}
		})
	}
}

// TestHeaderTruncation cuts the header after each of its five words.
func TestHeaderTruncation(t *testing.T) {
	header := headerWords()
	for cut := 1; cut < len(header); cut++ {
		t.Run("words"+itoaTest(uint32(cut)), func(t *testing.T) {
			_, _, err := parseWords(t, Options{}, header[:cut])
			if got := requireError(t, err).Message; got != "incomplete data" {
				t.Errorf("message = %q, want incomplete data", got)
			}
		})
	}
	t.Run("empty", func(t *testing.T) {
		_, _, err := Parse(nil, Options{})
		if got := requireError(t, err).Message; got != "incomplete data" {
			t.Errorf("message = %q, want incomplete data", got)
		}
	})
}
