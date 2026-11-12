package front

import (
	"encoding/binary"
	"errors"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// headerWords is the five-word header of a SPIR-V 1.0 module.
func headerWords() []uint32 {
	return []uint32{magicWord, 0x00010000, 0, 0, 0}
}

// instWords encodes one instruction: the word count, which includes the word
// itself, in the high 16 bits and the opcode in the low 16.
func instWords(op spvenums.Op, operands ...uint32) []uint32 {
	wc := uint16(len(operands) + 1)
	return append([]uint32{uint32(wc)<<16 | uint32(op)}, operands...)
}

// instWithString encodes an instruction whose last operand is a string, after
// the given word operands.
func instWithString(op spvenums.Op, words []uint32, s string) []uint32 {
	operands := make([]uint32, 0, len(words)+4)
	operands = append(operands, words...)
	operands = append(operands, stringWords(s)...)
	return instWords(op, operands...)
}

// stringWords encodes s as the NUL-terminated words SPIR-V stores strings in.
// The NUL takes a whole word when the string ends on a word boundary, as the
// specification requires.
func stringWords(s string) []uint32 {
	data := append([]byte(s), 0)
	for len(data)%4 != 0 {
		data = append(data, 0)
	}
	words := make([]uint32, len(data)/4)
	for i := range words {
		words[i] = binary.LittleEndian.Uint32(data[i*4:])
	}
	return words
}

// spvWords concatenates word groups into one binary.
func spvWords(groups ...[]uint32) []byte {
	var words []uint32
	for _, group := range groups {
		words = append(words, group...)
	}
	data := make([]byte, len(words)*4)
	for i, word := range words {
		binary.LittleEndian.PutUint32(data[i*4:], word)
	}
	return data
}

// parseWords parses a binary assembled from word groups.
func parseWords(t *testing.T, options Options, groups ...[]uint32) (*ir.Module, Info, error) {
	t.Helper()
	return Parse(spvWords(groups...), options)
}

// requireError returns the *Error a parse failed with.
func requireError(t *testing.T, err error) *Error {
	t.Helper()
	var parseErr *Error
	if !errors.As(err, &parseErr) {
		t.Fatalf("error %v (%T), want a *Error", err, err)
	}
	return parseErr
}

func TestParseHeader(t *testing.T) {
	t.Run("header only", func(t *testing.T) {
		module, _, err := parseWords(t, Options{}, headerWords())
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if module == nil {
			t.Fatal("module is nil")
		}
	})

	t.Run("wrong magic", func(t *testing.T) {
		words := headerWords()
		words[0] = 0xdeadbeef
		_, _, err := parseWords(t, Options{}, words)
		if got := requireError(t, err).Message; got != "invalid header" {
			t.Errorf("message = %q, want %q", got, "invalid header")
		}
	})

	t.Run("version 1.6", func(t *testing.T) {
		words := headerWords()
		words[1] = 0x00010600
		if _, _, err := parseWords(t, Options{}, words); err != nil {
			t.Errorf("Parse: %v", err)
		}
	})

	t.Run("version 1.7", func(t *testing.T) {
		words := headerWords()
		words[1] = 0x00010700
		_, _, err := parseWords(t, Options{}, words)
		if got := requireError(t, err).Message; got != "invalid header" {
			t.Errorf("message = %q, want %q", got, "invalid header")
		}
	})

	t.Run("truncated header", func(t *testing.T) {
		words := headerWords()[:3]
		_, _, err := parseWords(t, Options{}, words)
		if got := requireError(t, err).Message; got != "incomplete data" {
			t.Errorf("message = %q, want %q", got, "incomplete data")
		}
	})

	t.Run("length not a multiple of four", func(t *testing.T) {
		_, _, err := Parse([]byte{0x03, 0x02, 0x23}, Options{})
		if got := requireError(t, err).Message; got != "incomplete data" {
			t.Errorf("message = %q, want %q", got, "incomplete data")
		}
	})
}

func TestParseCapability(t *testing.T) {
	t.Run("supported", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
		)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
	})

	t.Run("unknown", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, 0xffff),
		)
		if got := requireError(t, err).Message; got != "unknown capability %65535" {
			t.Errorf("message = %q, want %q", got, "unknown capability %65535")
		}
	})

	t.Run("unsupported", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityLinkage)),
		)
		if got := requireError(t, err).Message; got != "unsupported capability CapabilityLinkage" {
			t.Errorf("message = %q, want %q", got, "unsupported capability CapabilityLinkage")
		}
	})

	t.Run("unknown allowed", func(t *testing.T) {
		_, info, err := parseWords(t, Options{AllowUnknownCapabilities: true},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityLinkage)),
		)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if len(info.Warnings) != 1 {
			t.Fatalf("warnings = %d, want 1", len(info.Warnings))
		}
		if want := "Unknown capability CapabilityLinkage"; info.Warnings[0].Message != want {
			t.Errorf("warning = %q, want %q", info.Warnings[0].Message, want)
		}
	})
}

func TestParseSectionOrder(t *testing.T) {
	_, _, err := parseWords(t, Options{},
		headerWords(),
		instWords(spvenums.OpMemoryModel, 0, 1),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
	)
	want := "unsupported instruction OpCapability at MemoryModel"
	if got := requireError(t, err).Message; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

func TestInstructionDecoding(t *testing.T) {
	t.Run("zero word count", func(t *testing.T) {
		_, _, err := parseWords(t, Options{}, headerWords(), []uint32{0})
		if got := requireError(t, err).Message; got != "invalid word count" {
			t.Errorf("message = %q, want %q", got, "invalid word count")
		}
	})

	t.Run("unknown opcode", func(t *testing.T) {
		word := uint32(1)<<16 | 0xffff
		_, _, err := parseWords(t, Options{}, headerWords(), []uint32{word})
		if got := requireError(t, err).Message; got != "unknown instruction 65535" {
			t.Errorf("message = %q, want %q", got, "unknown instruction 65535")
		}
	})

	t.Run("truncated operand", func(t *testing.T) {
		// OpCapability claims two words but its operand is missing.
		_, _, err := parseWords(t, Options{},
			headerWords(),
			[]uint32{uint32(2)<<16 | uint32(spvenums.OpCapability)},
		)
		parseErr := requireError(t, err)
		if parseErr.Message != "incomplete data" {
			t.Errorf("message = %q, want %q", parseErr.Message, "incomplete data")
		}
		// The header is five words, so the instruction starts at byte 20.
		if parseErr.Offset != 20 {
			t.Errorf("offset = %d, want 20", parseErr.Offset)
		}
	})

	t.Run("unsupported instruction", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpNop),
		)
		want := "unsupported instruction OpNop at Empty"
		if got := requireError(t, err).Message; got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
	})
}

func TestParseExtension(t *testing.T) {
	t.Run("supported", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWithString(spvenums.OpExtension, nil, "SPV_KHR_storage_buffer_storage_class"),
		)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
	})

	t.Run("unsupported", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWithString(spvenums.OpExtension, nil, "SPV_KHR_shader_clock"),
		)
		want := "unsupported extension SPV_KHR_shader_clock"
		if got := requireError(t, err).Message; got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
	})
}

func TestParseExtInstImport(t *testing.T) {
	t.Run("GLSL.std.450", func(t *testing.T) {
		p := newParser(spvWords(headerWords(), instWithString(spvenums.OpExtInstImport, []uint32{3}, "GLSL.std.450")), Options{})
		if _, _, err := p.parse(); err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if !p.hasExtGLSLID || p.extGLSLID != 3 {
			t.Errorf("extGLSLID = %d (set %v), want 3 (set)", p.extGLSLID, p.hasExtGLSLID)
		}
	})

	t.Run("unsupported set", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWithString(spvenums.OpExtInstImport, []uint32{3}, "OpenCL.std"),
		)
		want := "unsupported extension set OpenCL.std"
		if got := requireError(t, err).Message; got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
	})
}

func TestParseEntryPoint(t *testing.T) {
	t.Run("compute with LocalSize", func(t *testing.T) {
		p := newParser(spvWords(headerWords(),
			instWithString(spvenums.OpEntryPoint, []uint32{uint32(spvenums.ExecutionModelGLCompute), 1}, "main"),
			instWords(spvenums.OpExecutionMode, 1, uint32(spvenums.ExecutionModeLocalSize), 64, 1, 1),
		), Options{})
		if _, _, err := p.parse(); err != nil {
			t.Fatalf("Parse: %v", err)
		}
		ep, ok := p.entryPoints[1]
		if !ok {
			t.Fatal("entry point 1 not recorded")
		}
		if ep.stage != ir.StageCompute || ep.name != "main" {
			t.Errorf("stage %v name %q, want compute main", ep.stage, ep.name)
		}
		if ep.workgroup != [3]uint32{64, 1, 1} {
			t.Errorf("workgroup = %v, want [64 1 1]", ep.workgroup)
		}
		// The wrapper is deferred until the entry point's function is parsed.
		if len(p.deferredEntryPoints) != 0 {
			t.Errorf("deferred = %v, want none before the function is parsed", p.deferredEntryPoints)
		}
	})

	t.Run("fragment with DepthGreater", func(t *testing.T) {
		p := newParser(spvWords(headerWords(),
			instWithString(spvenums.OpEntryPoint, []uint32{uint32(spvenums.ExecutionModelFragment), 1}, "main"),
			instWords(spvenums.OpExecutionMode, 1, uint32(spvenums.ExecutionModeDepthGreater)),
		), Options{})
		if _, _, err := p.parse(); err != nil {
			t.Fatalf("Parse: %v", err)
		}
		allow, ok := p.entryPoints[1].earlyDepthTest.(ir.EarlyDepthAllow)
		if !ok {
			t.Fatalf("early depth test %T, want ir.EarlyDepthAllow", p.entryPoints[1].earlyDepthTest)
		}
		if allow.Conservative != ir.ConservativeDepthGreaterEqual {
			t.Errorf("conservative = %v, want GreaterEqual", allow.Conservative)
		}
	})

	t.Run("unsupported execution model", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWithString(spvenums.OpEntryPoint, []uint32{uint32(spvenums.ExecutionModelRayGenerationKHR), 1}, "main"),
		)
		if got := requireError(t, err).Message; !strings.HasPrefix(got, "unsupported execution model %") {
			t.Errorf("message = %q, want %q", got, "unsupported execution model %...")
		}
	})

	t.Run("unsupported execution mode", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWithString(spvenums.OpEntryPoint, []uint32{uint32(spvenums.ExecutionModelGLCompute), 1}, "main"),
			instWords(spvenums.OpExecutionMode, 1, uint32(spvenums.ExecutionModeInvocations), 1),
		)
		want := "unsupported execution mode %0"
		if got := requireError(t, err).Message; got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
	})

	t.Run("mode for an unknown entry point", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWithString(spvenums.OpEntryPoint, []uint32{uint32(spvenums.ExecutionModelGLCompute), 1}, "main"),
			instWords(spvenums.OpExecutionMode, 99, uint32(spvenums.ExecutionModeLocalSize), 1, 1, 1),
		)
		if got := requireError(t, err).Message; got != "invalid id %99" {
			t.Errorf("message = %q, want %q", got, "invalid id %99")
		}
	})

	t.Run("LocalSize without its literals", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWithString(spvenums.OpEntryPoint, []uint32{uint32(spvenums.ExecutionModelGLCompute), 1}, "main"),
			instWords(spvenums.OpExecutionMode, 1, uint32(spvenums.ExecutionModeLocalSize)),
		)
		want := "invalid operand count 3 for OpExecutionMode"
		if got := requireError(t, err).Message; got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
	})
}

func TestParseNamesAndDecorations(t *testing.T) {
	t.Run("empty global name", func(t *testing.T) {
		p := newParser(spvWords(headerWords(),
			instWithString(spvenums.OpEntryPoint, []uint32{uint32(spvenums.ExecutionModelGLCompute), 1}, "main"),
			instWithString(spvenums.OpName, []uint32{2}, ""),
		), Options{})
		if _, _, err := p.parse(); err != nil {
			t.Fatalf("Parse: %v", err)
		}
		dec, ok := p.decorations[2]
		if !ok || dec.name != "" {
			t.Errorf("decoration of 2 = %#v, want an empty name", dec)
		}
	})

	t.Run("string without a NUL", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpName, 2, 0x44434241), // "ABCD", no terminator
		)
		if got := requireError(t, err).Message; got != "bad string" {
			t.Errorf("message = %q, want %q", got, "bad string")
		}
	})

	t.Run("built-in decoration before the id's definition", func(t *testing.T) {
		p := newParser(spvWords(headerWords(),
			instWords(spvenums.OpDecorate, 2, uint32(spvenums.DecorationBuiltIn), 0),
		), Options{})
		if _, _, err := p.parse(); err != nil {
			t.Fatalf("Parse: %v", err)
		}
		dec, ok := p.decorations[2]
		if !ok || dec.builtIn == nil || *dec.builtIn != 0 {
			t.Errorf("decoration of 2 = %#v, want builtin 0", dec)
		}
	})

	t.Run("unknown decoration is a warning", func(t *testing.T) {
		_, info, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpDecorate, 2, uint32(spvenums.DecorationAliased)),
		)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if len(info.Warnings) != 1 || info.Warnings[0].Message != "Unknown decoration DecorationAliased" {
			t.Errorf("warnings = %#v, want the Aliased decoration", info.Warnings)
		}
	})

	t.Run("invalid decoration", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpDecorate, 2, 9999),
		)
		if got := requireError(t, err).Message; got != "invalid decoration %9999" {
			t.Errorf("message = %q, want %q", got, "invalid decoration %9999")
		}
	})
}

func TestCheckModuleSize(t *testing.T) {
	if err := checkModuleSize(4); err != nil {
		t.Errorf("checkModuleSize(4) = %v, want nil", err)
	}
	if err := checkModuleSize(maxModuleBytes + 1); !errors.Is(err, errModuleTooLarge) {
		t.Errorf("checkModuleSize(max+1) = %v, want errModuleTooLarge", err)
	}
}
