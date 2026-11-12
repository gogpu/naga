package spirv

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// TestParseErrorShape: a malformed module is a *ParseError carrying the
// decoder's message and the byte offset of the instruction.
func TestParseErrorShape(t *testing.T) {
	_, _, err := Parse([]byte{0x03, 0x02, 0x23}, ParseOptions{})
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("error %v (%T), want a *ParseError", err, err)
	}
	if parseErr.Message != "incomplete data" {
		t.Errorf("message = %q, want incomplete data", parseErr.Message)
	}
	if got := parseErr.Error(); got != parseErr.Message {
		t.Errorf("Error() = %q, want the message", got)
	}
}

// TestParseOptionsAreCarried: KeepCoordinateSpace reaches the frontend; the
// wrapper it changes is what a caller sees.
func TestParseOptionsAreCarried(t *testing.T) {
	binary := positionModule()
	flipped, _, err := Parse(binary, ParseOptions{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	kept, _, err := Parse(binary, ParseOptions{KeepCoordinateSpace: true})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(flipped.EntryPoints) != 1 || len(kept.EntryPoints) != 1 {
		t.Fatalf("entry points = %d and %d, want 1", len(flipped.EntryPoints), len(kept.EntryPoints))
	}
	// The flip appends the negate and its store, so the wrapper is longer.
	if len(flipped.EntryPoints[0].Function.Expressions) <= len(kept.EntryPoints[0].Function.Expressions) {
		t.Errorf("the default wrapper has %d expressions, the keeping one %d: the flag did not reach the frontend",
			len(flipped.EntryPoints[0].Function.Expressions), len(kept.EntryPoints[0].Function.Expressions))
	}
}

// positionModule assembles a minimal vertex shader with a Position output.
// Ids: 1 function, 2 label, 3 output variable, 10 void, 11 function type,
// 12 float, 13 vec4<f32>, 14 pointer to vec4<f32> in the Output class.
func positionModule() []byte {
	inst := func(op spvenums.Op, operands ...uint32) []uint32 {
		wc := uint32(len(operands) + 1)
		return append([]uint32{wc<<16 | uint32(op)}, operands...)
	}
	name := func(s string) []uint32 {
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
	entryPoint := append([]uint32{uint32(spvenums.ExecutionModelVertex), 1}, name("main")...)
	entryPoint = append(entryPoint, 3)

	words := []uint32{0x07230203, 0x00010000, 0, 0, 0}
	words = append(words, inst(spvenums.OpCapability, uint32(spvenums.CapabilityShader))...)
	words = append(words, inst(spvenums.OpMemoryModel, 0, 1)...)
	words = append(words, inst(spvenums.OpEntryPoint, entryPoint...)...)
	words = append(words, inst(spvenums.OpDecorate, 3, uint32(spvenums.DecorationBuiltIn), uint32(spvenums.BuiltInPosition))...)
	words = append(words, inst(spvenums.OpTypeVoid, 10)...)
	words = append(words, inst(spvenums.OpTypeFunction, 11, 10)...)
	words = append(words, inst(spvenums.OpTypeFloat, 12, 32)...)
	words = append(words, inst(spvenums.OpTypeVector, 13, 12, 4)...)
	words = append(words, inst(spvenums.OpTypePointer, 14, uint32(spvenums.StorageClassOutput), 13)...)
	words = append(words, inst(spvenums.OpVariable, 14, 3, uint32(spvenums.StorageClassOutput))...)
	words = append(words, inst(spvenums.OpFunction, 10, 1, 0, 11)...)
	words = append(words, inst(spvenums.OpLabel, 2)...)
	words = append(words, inst(spvenums.OpReturn)...)
	words = append(words, inst(spvenums.OpFunctionEnd)...)

	data := make([]byte, len(words)*4)
	for i, word := range words {
		binary.LittleEndian.PutUint32(data[i*4:], word)
	}
	return data
}
