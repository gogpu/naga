package front

import (
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// TestParseMetadataInstructions covers the debug and annotation instructions
// a compiler emits, which the frontend reads and mostly ignores.
func TestParseMetadataInstructions(t *testing.T) {
	p := newParser(spvWords(headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpMemoryModel, 0, 1),
		instWithString(spvenums.OpString, []uint32{5}, "main.wgsl"),
		instWords(spvenums.OpSource, 2, 450),
		instWithString(spvenums.OpSourceExtension, nil, "GLSL.std.450"),
		instWithString(spvenums.OpName, []uint32{7}, "block"),
		instWithString(spvenums.OpMemberName, []uint32{7, 0}, "member"),
		instWithString(spvenums.OpModuleProcessed, nil, "compiled"),
		instWords(spvenums.OpTypeInt, 8, 32, 0),
		instWords(spvenums.OpTypeStruct, 7, 8),
	), Options{})
	module, _, err := p.parse()
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	structType, ok := typeDefinition(t, module, 7, p).Inner.(ir.StructType)
	if !ok || len(structType.Members) != 1 {
		t.Fatalf("struct = %#v", typeDefinition(t, module, 7, p).Inner)
	}
	if structType.Members[0].Name != "member" {
		t.Errorf("member name = %q, want member", structType.Members[0].Name)
	}
	if name := typeDefinition(t, module, 7, p).Name; name != "block" {
		t.Errorf("struct name = %q, want block", name)
	}
}

// TestParseDecorationKinds checks the decoration values the frontend keeps for
// the type and variable mapping.
func TestParseDecorationKinds(t *testing.T) {
	p := newParser(spvWords(headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpMemoryModel, 0, 1),
		instWords(spvenums.OpDecorate, 3, uint32(spvenums.DecorationFlat)),
		instWords(spvenums.OpDecorate, 4, uint32(spvenums.DecorationNoPerspective)),
		instWords(spvenums.OpDecorate, 5, uint32(spvenums.DecorationPerVertexKHR)),
		instWords(spvenums.OpDecorate, 6, uint32(spvenums.DecorationCentroid)),
		instWords(spvenums.OpDecorate, 7, uint32(spvenums.DecorationSample)),
		instWords(spvenums.OpDecorate, 8, uint32(spvenums.DecorationInvariant)),
		instWords(spvenums.OpDecorate, 9, uint32(spvenums.DecorationColMajor)),
		instWords(spvenums.OpDecorate, 10, uint32(spvenums.DecorationRowMajor)),
		instWords(spvenums.OpDecorate, 11, uint32(spvenums.DecorationCoherent)),
		instWords(spvenums.OpDecorate, 12, uint32(spvenums.DecorationVolatile)),
		instWords(spvenums.OpDecorate, 13, uint32(spvenums.DecorationNonReadable)),
		instWords(spvenums.OpDecorate, 14, uint32(spvenums.DecorationSpecId), 9),
		instWords(spvenums.OpDecorate, 15, uint32(spvenums.DecorationArrayStride), 0),
		instWords(spvenums.OpTypeInt, 1, 32, 0),
	), Options{})
	if _, _, err := p.parse(); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	flat := p.decorations[3]
	if flat.interpolation == nil || *flat.interpolation != ir.InterpolationFlat {
		t.Errorf("flat = %#v, want flat interpolation", flat)
	}
	noPerspective := p.decorations[4]
	if noPerspective.interpolation == nil || *noPerspective.interpolation != ir.InterpolationLinear {
		t.Errorf("noperspective = %#v, want linear interpolation", noPerspective)
	}
	perVertex := p.decorations[5]
	if perVertex.interpolation == nil || *perVertex.interpolation != ir.InterpolationPerVertex {
		t.Errorf("pervertex = %#v, want per-vertex interpolation", perVertex)
	}
	centroid := p.decorations[6]
	if centroid.sampling == nil || *centroid.sampling != ir.SamplingCentroid {
		t.Errorf("centroid = %#v, want centroid sampling", centroid)
	}
	sample := p.decorations[7]
	if sample.sampling == nil || *sample.sampling != ir.SamplingSample {
		t.Errorf("sample = %#v, want sample sampling", sample)
	}
	if !p.decorations[8].invariant {
		t.Error("invariant was not recorded")
	}
	if rowMajor := p.decorations[9].rowMajor; rowMajor == nil || *rowMajor {
		t.Errorf("colmajor = %v, want false", rowMajor)
	}
	if rowMajor := p.decorations[10].rowMajor; rowMajor == nil || !*rowMajor {
		t.Errorf("rowmajor = %v, want true", rowMajor)
	}
	if !p.decorations[11].flags.Has(decorationCoherent) {
		t.Error("coherent was not recorded")
	}
	if !p.decorations[12].flags.Has(decorationVolatile) {
		t.Error("volatile was not recorded")
	}
	if !p.decorations[13].flags.Has(decorationNonReadable) {
		t.Error("non-readable was not recorded")
	}
	if id := p.decorations[14].specID; id == nil || *id != 9 {
		t.Errorf("spec id = %v, want 9", id)
	}
	// A zero array stride is no stride.
	if p.decorations[15].hasArrayStride {
		t.Error("a zero ArrayStride was recorded")
	}
}

// Has reports whether every bit of flag is set.
func (f decorationFlags) Has(flag decorationFlags) bool { return f&flag == flag }

// trailingWord appends one operand word to an encoded instruction and fixes
// the word count to cover it.
func trailingWord(words []uint32) []uint32 {
	words = append(words, 7)
	words[0] = uint32(len(words))<<16 | (words[0] & 0xffff)
	return words
}

// TestParseMemberNameErrorPath checks the word-count guards of the metadata
// instructions.
func TestParseMetadataWordCounts(t *testing.T) {
	cases := []struct {
		name string
		inst []uint32
		want string
		// early instructions belong to the extension section, before the
		// memory model.
		early bool
	}{
		{"name without a string", instWords(spvenums.OpName, 3), "invalid operand count 2 for Name", false},
		{"member name without a string", instWords(spvenums.OpMemberName, 3, 0), "invalid operand count 3 for MemberName", false},
		{"module processed without a string", instWords(spvenums.OpModuleProcessed), "invalid operand count 1 for ModuleProcessed", false},
		{"extension without a name", instWords(spvenums.OpExtension), "invalid operand count 1 for Extension", true},
		{"source extension without a name", instWords(spvenums.OpSourceExtension), "invalid operand count 1 for SourceExtension", false},
		{"ext inst import without a name", instWords(spvenums.OpExtInstImport, 3), "invalid operand count 2 for ExtInstImport", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			groups := [][]uint32{
				headerWords(),
				instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			}
			if tc.early {
				groups = append(groups, tc.inst, instWords(spvenums.OpMemoryModel, 0, 1))
			} else {
				groups = append(groups, instWords(spvenums.OpMemoryModel, 0, 1), tc.inst)
			}
			_, _, err := parseWords(t, Options{}, groups...)
			if got := requireError(t, err).Message; got != tc.want {
				t.Errorf("message = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestParseExtraOperandsAfterStrings: a string operand that is followed by
// words is an invalid operand, as Rust naga reports.
func TestParseExtraOperandsAfterStrings(t *testing.T) {
	cases := []struct {
		name  string
		inst  []uint32
		early bool
	}{
		{"extension", instWords(spvenums.OpExtension, append(stringWords("SPV_KHR_storage_buffer_storage_class"), 7)...), true},
		{"name", trailingWord(append(instWords(spvenums.OpName, 3), stringWords("x")...)), false},
		{"member name", trailingWord(append(instWords(spvenums.OpMemberName, 3, 0), stringWords("x")...)), false},
		{"module processed", trailingWord(append(instWords(spvenums.OpModuleProcessed), stringWords("x")...)), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			groups := [][]uint32{
				headerWords(),
				instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			}
			if tc.early {
				groups = append(groups, tc.inst, instWords(spvenums.OpMemoryModel, 0, 1))
			} else {
				groups = append(groups, instWords(spvenums.OpMemoryModel, 0, 1), tc.inst)
			}
			_, _, err := parseWords(t, Options{}, groups...)
			if got := requireError(t, err).Message; got != "invalid operand" {
				t.Errorf("message = %q, want invalid operand", got)
			}
		})
	}
}
