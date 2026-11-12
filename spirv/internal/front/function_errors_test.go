package front

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/spirv/internal/spvenums"
)

func TestParseFunctionErrors(t *testing.T) {
	t.Run("wrong result type", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpTypeVoid, 10),
			instWords(spvenums.OpTypeFunction, 11, 10),
			instWords(spvenums.OpTypeInt, 12, 32, 1),
			instWords(spvenums.OpFunction, 12, 1, 0, 11),
			instWords(spvenums.OpLabel, 2),
			instWords(spvenums.OpReturn),
			instWords(spvenums.OpFunctionEnd),
		)
		want := "wrong function result type %12"
		if got := requireError(t, err).Message; got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
	})

	t.Run("wrong argument type", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpTypeVoid, 10),
			instWords(spvenums.OpTypeInt, 12, 32, 1),
			instWords(spvenums.OpTypeFloat, 13, 32),
			instWords(spvenums.OpTypeFunction, 11, 10, 12),
			instWords(spvenums.OpFunction, 10, 1, 0, 11),
			instWords(spvenums.OpFunctionParameter, 13, 20),
			instWords(spvenums.OpLabel, 2),
			instWords(spvenums.OpReturn),
			instWords(spvenums.OpFunctionEnd),
		)
		want := "wrong function argument type %13"
		if got := requireError(t, err).Message; got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
	})

	t.Run("parameter list holds no label", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpTypeVoid, 10),
			instWords(spvenums.OpTypeInt, 12, 32, 1),
			instWords(spvenums.OpTypeFunction, 11, 10, 12),
			instWords(spvenums.OpFunction, 10, 1, 0, 11),
			instWords(spvenums.OpLabel, 2),
			instWords(spvenums.OpReturn),
			instWords(spvenums.OpFunctionEnd),
		)
		want := "invalid parameter Label"
		if got := requireError(t, err).Message; got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
	})

	t.Run("unsupported instruction at function level", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpTypeVoid, 10),
			instWords(spvenums.OpTypeFunction, 11, 10),
			instWords(spvenums.OpFunction, 10, 1, 0, 11),
			instWords(spvenums.OpNop),
			instWords(spvenums.OpFunctionEnd),
		)
		want := "unsupported instruction Nop at Function"
		if got := requireError(t, err).Message; got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
	})

	t.Run("non-semantic instruction at function level is skipped", func(t *testing.T) {
		p := newParser(spvWords(headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWithString(spvenums.OpExtInstImport, []uint32{40}, "NonSemantic.Shader.DebugInfo.100"),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpTypeVoid, 10),
			instWords(spvenums.OpTypeFunction, 11, 10),
			instWords(spvenums.OpFunction, 10, 1, 0, 11),
			instWords(spvenums.OpExtInst, 10, 30, 40, 1, 2, 3),
			instWords(spvenums.OpLabel, 2),
			instWords(spvenums.OpReturn),
			instWords(spvenums.OpFunctionEnd),
		), Options{})
		if _, _, err := p.parse(); err != nil {
			t.Fatalf("Parse: %v", err)
		}
	})

	t.Run("semantic instruction at function level fails", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWithString(spvenums.OpExtInstImport, []uint32{40}, "GLSL.std.450"),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpTypeVoid, 10),
			instWords(spvenums.OpTypeFunction, 11, 10),
			instWords(spvenums.OpFunction, 10, 1, 0, 11),
			instWords(spvenums.OpExtInst, 10, 30, 40, 1),
			instWords(spvenums.OpFunctionEnd),
		)
		want := "unsupported instruction ExtInst at Function"
		if got := requireError(t, err).Message; got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
	})
}

func TestParseConstantAndConversionMistakes(t *testing.T) {
	t.Run("constant of a struct type", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpTypeInt, 1, 32, 0),
			instWords(spvenums.OpTypeStruct, 2, 1),
			instWords(spvenums.OpConstant, 2, 3, 0),
		)
		if got := requireError(t, err).Message; !strings.HasPrefix(got, "unsupported type [") {
			t.Errorf("message = %q, want an unsupported type", got)
		}
	})

	t.Run("constant of a 16-bit integer", func(t *testing.T) {
		for _, signedness := range []uint32{0, 1} {
			_, _, err := parseWords(t, Options{},
				headerWords(),
				instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
				instWords(spvenums.OpMemoryModel, 0, 1),
				instWords(spvenums.OpTypeInt, 1, 16, signedness),
				instWords(spvenums.OpConstant, 1, 2, 0),
			)
			want := "invalid type width %2"
			if got := requireError(t, err).Message; got != want {
				t.Errorf("signedness %d: message = %q, want %q", signedness, got, want)
			}
		}
	})

	t.Run("constant of an 8-bit float", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpTypeFloat, 1, 8),
			instWords(spvenums.OpConstant, 1, 2, 0),
		)
		// The width in the message is in bytes, as Rust's Bytes type is.
		want := "invalid type width %1"
		if got := requireError(t, err).Message; got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
	})

	t.Run("sampled image of an unknown type", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpTypeSampledImage, 1, 99),
		)
		want := "invalid id %99"
		if got := requireError(t, err).Message; got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
	})

	t.Run("bitcast to a struct", func(t *testing.T) {
		fn := newBodyModule().parseError(t,
			instWords(spvenums.OpUndef, 14, 200),
			instWords(spvenums.OpBitcast, 14, 201, 20),
		)
		if !strings.HasPrefix(fn.Message, "invalid as type [") {
			t.Errorf("message = %q, want an invalid as type", fn.Message)
		}
	})

	t.Run("access chain into a scalar", func(t *testing.T) {
		m := newBodyModule()
		got := m.parseError(t,
			instWords(spvenums.OpVariable, 11, 200, uint32(spvenums.StorageClassFunction)),
			instWords(spvenums.OpAccessChain, 11, 201, 200, 20),
		)
		if !strings.HasPrefix(got.Message, "invalid access type %") {
			t.Errorf("message = %q, want an invalid access type", got.Message)
		}
	})

	t.Run("binding array without a resource binding", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpTypeFloat, 1, 32),
			instWords(spvenums.OpTypeImage, 2, 1, uint32(spvenums.Dim2D), 0, 0, 0, 1, 0),
			instWords(spvenums.OpTypeInt, 3, 32, 0),
			instWords(spvenums.OpConstant, 3, 4, 4),
			instWords(spvenums.OpTypeArray, 5, 2, 4),
			instWords(spvenums.OpTypePointer, 6, uint32(spvenums.StorageClassUniformConstant), 5),
			instWords(spvenums.OpVariable, 6, 7, uint32(spvenums.StorageClassUniformConstant)),
		)
		if got := requireError(t, err).Message; got != errNonBindingArrayOfImageOrSamplers.Message {
			t.Errorf("message = %q", got)
		}
	})
}
