package front

import (
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
	"github.com/gogpu/naga/valid"
)

// validateModule parses words and validates what they produce. The wrapper's
// Emit ranges and the pre-emit expressions are only correct if the validator
// accepts them: the frontend's contract is that Parse's output validates.
func validateModule(t *testing.T, options Options, groups ...[]uint32) *ir.Module {
	t.Helper()
	module, _, err := parseWords(t, options, groups...)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, err := valid.Validate(module, valid.Options{Capabilities: valid.CapAll}); err != nil {
		t.Fatalf("valid.Validate: %v", err)
	}
	return module
}

// TestWrapperIntegerInputValidates: a signed-integer built-in input is copied
// through a convert expression; the argument itself is pre-emit and must stay
// outside every Emit range.
func TestWrapperIntegerInputValidates(t *testing.T) {
	validateModule(t, Options{},
		headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpMemoryModel, 0, 1),
		entryPointOp(spvenums.ExecutionModelVertex, 3, 4),
		instWords(spvenums.OpDecorate, 3, uint32(spvenums.DecorationBuiltIn), uint32(spvenums.BuiltInVertexIndex)),
		instWords(spvenums.OpDecorate, 4, uint32(spvenums.DecorationBuiltIn), uint32(spvenums.BuiltInPosition)),
		instWords(spvenums.OpTypeVoid, 10),
		instWords(spvenums.OpTypeFunction, 11, 10),
		instWords(spvenums.OpTypeInt, 12, 32, 1),
		instWords(spvenums.OpTypeFloat, 13, 32),
		instWords(spvenums.OpTypeVector, 14, 13, 4),
		instWords(spvenums.OpTypePointer, 15, uint32(spvenums.StorageClassInput), 12),
		instWords(spvenums.OpTypePointer, 16, uint32(spvenums.StorageClassOutput), 14),
		instWords(spvenums.OpVariable, 15, 3, uint32(spvenums.StorageClassInput)),
		instWords(spvenums.OpVariable, 16, 4, uint32(spvenums.StorageClassOutput)),
		voidFunctionBody(),
		instWords(spvenums.OpLabel, 2),
		instWords(spvenums.OpReturn),
		instWords(spvenums.OpFunctionEnd),
	)
}

// TestUndefInBodyValidates: OpUndef yields a pre-emit zero value, which must
// not be introduced by an Emit.
func TestUndefInBodyValidates(t *testing.T) {
	m := newBodyModule()
	module, _, err := Parse(m.build(instWords(spvenums.OpUndef, 3, 200)), Options{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, err := valid.Validate(module, valid.Options{Capabilities: valid.CapAll}); err != nil {
		t.Fatalf("valid.Validate: %v", err)
	}
}

// TestMatrixStrideAlignment: the expected stride uses the row alignment, so a
// vec3 row aligns like a vec4. A mat2x3<f16> member has a stride of 8.
func TestMatrixStrideAlignment(t *testing.T) {
	t.Run("mat2x3 of f16", func(t *testing.T) {
		p := newParser(spvWords(headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpMemberDecorate, 4, 0, uint32(spvenums.DecorationMatrixStride), 8),
			instWords(spvenums.OpMemberDecorate, 4, 0, uint32(spvenums.DecorationOffset), 0),
			instWords(spvenums.OpTypeFloat, 1, 16),
			instWords(spvenums.OpTypeVector, 2, 1, 3),
			instWords(spvenums.OpTypeMatrix, 3, 2, 2),
			instWords(spvenums.OpTypeStruct, 4, 3),
		), Options{})
		if _, _, err := p.parse(); err != nil {
			t.Fatalf("Parse: %v", err)
		}
	})

	t.Run("mat2x3 of f32", func(t *testing.T) {
		p := newParser(spvWords(headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpMemberDecorate, 4, 0, uint32(spvenums.DecorationMatrixStride), 16),
			instWords(spvenums.OpMemberDecorate, 4, 0, uint32(spvenums.DecorationOffset), 0),
			instWords(spvenums.OpTypeFloat, 1, 32),
			instWords(spvenums.OpTypeVector, 2, 1, 3),
			instWords(spvenums.OpTypeMatrix, 3, 2, 2),
			instWords(spvenums.OpTypeStruct, 4, 3),
		), Options{})
		if _, _, err := p.parse(); err != nil {
			t.Fatalf("Parse: %v", err)
		}
	})

	t.Run("mat2x3 with a tight stride", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpMemberDecorate, 4, 0, uint32(spvenums.DecorationMatrixStride), 12),
			instWords(spvenums.OpTypeFloat, 1, 32),
			instWords(spvenums.OpTypeVector, 2, 1, 3),
			instWords(spvenums.OpTypeMatrix, 3, 2, 2),
			instWords(spvenums.OpTypeStruct, 4, 3),
		)
		want := "unsupported matrix stride 12 for a 2x3 matrix with scalar width=4"
		if got := requireError(t, err).Message; got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
	})
}

// TestModuleExtInst: a module-level OpExtInst is ignored only for the
// non-semantic debug set.
func TestModuleExtInst(t *testing.T) {
	t.Run("non-semantic is skipped", func(t *testing.T) {
		p := newParser(spvWords(headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWithString(spvenums.OpExtInstImport, []uint32{40}, "NonSemantic.Shader.DebugInfo.100"),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpTypeInt, 1, 32, 0),
			instWords(spvenums.OpExtInst, 1, 2, 40, 1, 2, 3),
		), Options{})
		if _, _, err := p.parse(); err != nil {
			t.Fatalf("Parse: %v", err)
		}
	})

	t.Run("another set is unsupported", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWithString(spvenums.OpExtInstImport, []uint32{40}, "GLSL.std.450"),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpTypeInt, 1, 32, 0),
			instWords(spvenums.OpExtInst, 1, 2, 40, 1),
		)
		want := "unsupported instruction ExtInst at Type"
		if got := requireError(t, err).Message; got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
	})
}

// TestDecorationWordCountGuard: a decoration instruction with too few words is
// refused before the decoration is read, so the unknown-decoration skip cannot
// count backwards.
func TestDecorationWordCountGuard(t *testing.T) {
	_, _, err := parseWords(t, Options{},
		headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpMemoryModel, 0, 1),
		instWords(spvenums.OpDecorate, 1),
	)
	want := "invalid operand count 2 for Decorate"
	if got := requireError(t, err).Message; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}
