package snapshot_test

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
	"testing"
)

// spvModule assembles a SPIR-V binary from raw instructions; each instruction
// is its opcode followed by its operand words.
func spvModule(instrs ...[]uint32) []byte {
	words := []uint32{0x07230203, 0x00010300, 0, 1 << 24, 0}
	for _, in := range instrs {
		words = append(words, uint32(len(in))<<16|in[0])
		words = append(words, in[1:]...)
	}
	out := make([]byte, 4*len(words))
	for i, w := range words {
		binary.LittleEndian.PutUint32(out[4*i:], w)
	}
	return out
}

// spvStringOpcodes carry a literal string operand, which the printer decodes
// as text rather than words; their operands are covered by spvPrinterCases.
var spvStringOpcodes = map[uint16]bool{
	3:   true, // OpSource (optional file text)
	4:   true, // OpSourceExtension
	5:   true, // OpName
	6:   true, // OpMemberName
	7:   true, // OpString
	10:  true, // OpExtension
	11:  true, // OpExtInstImport
	15:  true, // OpEntryPoint
	330: true, // OpModuleProcessed
}

// TestDisassembleSPIRVPrintsEveryOperand pins BUG-TEST-SPVFMT-001: golden
// SPIR-V text must show every operand word of every instruction. The printer
// formats many opcodes by hand, and a hand-written case that stops early hid
// OpVariable initializers from every golden. For each opcode the printer
// knows, an instruction with ten distinct marker operands must print all ten.
func TestDisassembleSPIRVPrintsEveryOperand(t *testing.T) {
	opcodes := make([]int, 0, len(spvOpcodeNames))
	for op := range spvOpcodeNames {
		if !spvStringOpcodes[op] {
			opcodes = append(opcodes, int(op))
		}
	}
	sort.Ints(opcodes)

	for _, op := range opcodes {
		in := []uint32{uint32(op)}
		var markers []string
		for i := range 10 {
			m := uint32(7100000 + 11*i)
			in = append(in, m)
			markers = append(markers, fmt.Sprint(m))
		}
		text := disassembleSPIRV(spvModule(in))
		var missing []string
		for _, m := range markers {
			if !strings.Contains(text, m) {
				missing = append(missing, m)
			}
		}
		if len(missing) > 0 {
			line := strings.TrimSpace(text[strings.LastIndex(strings.TrimRight(text, "\n"), "\n")+1:])
			t.Errorf("%s (%d): operands %v not printed: %q", spvOpcodeNames[uint16(op)], op, missing, line)
		}
	}
}

// spvPrinterCases pins the text of instructions whose optional operands the
// printer dropped, in the golden notation.
var spvPrinterCases = []struct {
	name  string
	instr []uint32
	want  string
}{
	{"OpVariable with initializer", []uint32{59, 3, 5, 6, 4}, "%_5 = OpVariable %_3 Private %_4"},
	{"OpVariable without initializer", []uint32{59, 3, 5, 6}, "%_5 = OpVariable %_3 Private"},
	{"OpLoad Aligned 4", []uint32{61, 2, 6, 5, 2, 4}, "%_6 = OpLoad %_2 %_5 2 4"},
	{"OpStore Volatile", []uint32{62, 5, 4, 1}, "OpStore %_5 %_4 1"},
	{"OpFunction DontInline", []uint32{54, 2, 7, 2, 8}, "%_7 = OpFunction %_2 DontInline %_8"},
	{"OpFunction None", []uint32{54, 2, 7, 0, 8}, "%_7 = OpFunction %_2 None %_8"},
	{"OpTypeImage sampled", []uint32{25, 9, 2, 1, 0, 0, 0, 1, 0}, "%_9 = OpTypeImage %_2 2D 0 0 0 1 Unknown"},
	{"OpTypeImage storage", []uint32{25, 9, 2, 1, 0, 0, 0, 2, 4}, "%_9 = OpTypeImage %_2 2D 0 0 0 2 Rgba8"},
	{"OpTypeImage access", []uint32{25, 9, 2, 1, 0, 0, 0, 2, 1, 0}, "%_9 = OpTypeImage %_2 2D 0 0 0 2 Rgba32f ReadOnly"},
	{"OpImageRead Sample", []uint32{98, 2, 10, 9, 3, 0x40, 4}, "%_10 = OpImageRead %_2 %_9 %_3 %_64 %_4"},
	{"OpImageWrite Sample", []uint32{99, 9, 3, 4, 0x40, 5}, "OpImageWrite %_9 %_3 %_4 %_64 %_5"},
	{"OpLoopMerge with parameter", []uint32{246, 11, 12, 8, 4}, "OpLoopMerge %_11 %_12 8 4"},
	{"OpBranchConditional with weights", []uint32{250, 13, 14, 15, 3, 1}, "OpBranchConditional %_13 %_14 %_15 3 1"},
}

func TestDisassembleSPIRVOptionalOperands(t *testing.T) {
	for _, tc := range spvPrinterCases {
		t.Run(tc.name, func(t *testing.T) {
			text := disassembleSPIRV(spvModule(tc.instr))
			got := strings.TrimSpace(text[strings.LastIndex(strings.TrimRight(text, "\n"), "\n")+1:])
			if got != tc.want {
				t.Errorf("got  %q\nwant %q", got, tc.want)
			}
		})
	}
}
