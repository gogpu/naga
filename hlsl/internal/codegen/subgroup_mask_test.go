package codegen

// The five ballot-mask built-ins (card P2-IR-042, ADR-014 D5). HLSL has no
// such value, so the writer synthesizes it from the lane index and the wave
// size: the mask spans every invocation of the wave, and WaveActiveBallot(true)
// holds only the active lanes. internal/subgroupmask is the semantic oracle.

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/internal/subgroupmask"
	"github.com/gogpu/naga/ir"
)

// TestHLSL_SubgroupMaskBuiltins: each mask argument initializes from
// naga_subgroup_mask with its kind, the synthesis functions are written once
// per module, and they read the lane index and the wave size.
func TestHLSL_SubgroupMaskBuiltins(t *testing.T) {
	code, _, err := Compile(subgroupMaskModule(
		ir.BuiltinSubgroupEqMask, ir.BuiltinSubgroupGeMask, ir.BuiltinSubgroupGtMask,
		ir.BuiltinSubgroupLeMask, ir.BuiltinSubgroupLtMask,
	), DefaultOptions())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	for kind, call := range map[int]string{
		0: "naga_subgroup_mask(0u)", 1: "naga_subgroup_mask(1u)", 2: "naga_subgroup_mask(2u)",
		3: "naga_subgroup_mask(3u)", 4: "naga_subgroup_mask(4u)",
	} {
		if !strings.Contains(code, call) {
			t.Errorf("kind %d: output lacks %q", kind, call)
		}
	}
	if got := strings.Count(code, "uint naga_subgroup_mask_word("); got != 1 {
		t.Errorf("naga_subgroup_mask_word defined %d times, want 1", got)
	}
	if got := strings.Count(code, "uint4 naga_subgroup_mask("); got != 1 {
		t.Errorf("naga_subgroup_mask defined %d times, want 1", got)
	}
	for _, want := range []string{"WaveGetLaneIndex()", "WaveGetLaneCount()"} {
		if !strings.Contains(code, want) {
			t.Errorf("output lacks %q", want)
		}
	}
}

// TestHLSL_SubgroupMaskWordsMatchTheOracle: the Go transcription of the
// emitted naga_subgroup_mask_word formula equals internal/subgroupmask's
// spec-equation evaluator for every power-of-two size up to 128 and every
// lane, including the word boundaries and the partial last word — the check
// the emitted HLSL itself cannot be run through locally.
func TestHLSL_SubgroupMaskWordsMatchTheOracle(t *testing.T) {
	for size := uint32(1); size <= 128; size <<= 1 {
		for lane := uint32(0); lane < size; lane++ {
			for kind := subgroupmask.Kind(0); kind <= subgroupmask.Lt; kind++ {
				var got [4]uint32
				for word := uint32(0); word < 4; word++ {
					got[word] = nagaSubgroupMaskWord(kind, word, lane, size)
				}
				if want := subgroupmask.Mask(kind, lane, size); got != want {
					t.Fatalf("size %d lane %d kind %d: %08x, want %08x", size, lane, kind, got, want)
				}
			}
		}
	}
}

// nagaSubgroupMaskWord transcribes the emitted HLSL naga_subgroup_mask_word
// line for line; a change to the emitted formula must change this mirror.
func nagaSubgroupMaskWord(kind subgroupmask.Kind, word, lane, size uint32) uint32 {
	base := word << 5
	var lo uint32
	if size > base {
		lo = size - base
		if lo > 32 {
			lo = 32
		}
	}
	var full uint32
	if lo >= 32 {
		full = 0xFFFFFFFF
	} else {
		full = (1 << lo) - 1
	}
	if kind == 0 { // Eq
		if lane>>5 == word {
			return 1 << (lane & 31)
		}
		return 0
	}
	if kind <= 2 { // Ge, Gt
		start := lane
		if kind == 2 {
			start = lane + 1
		}
		if start <= base {
			return full
		}
		delta := start - base
		if delta < 32 {
			return full &^ ((1 << delta) - 1)
		}
		return 0
	}
	if lane < base {
		return 0
	}
	var count uint32
	if kind == 3 { // Le
		count = lane - base + 1
	} else { // Lt
		count = lane - base
	}
	if count > lo {
		count = lo
	}
	if count >= 32 {
		return 0xFFFFFFFF
	}
	return (1 << count) - 1
}

// TestBuiltInToSemanticSubgroupMasks: the five masks are wave-intrinsic
// handled (no semantic), not refused.
func TestBuiltInToSemanticSubgroupMasks(t *testing.T) {
	for _, builtin := range []ir.BuiltinValue{
		ir.BuiltinSubgroupEqMask, ir.BuiltinSubgroupGeMask, ir.BuiltinSubgroupGtMask,
		ir.BuiltinSubgroupLeMask, ir.BuiltinSubgroupLtMask,
	} {
		if semantic, supported := BuiltInToSemantic(builtin); !supported || semantic != "" {
			t.Errorf("BuiltInToSemantic(%s) = %q, %v; want supported with no semantic", builtin, semantic, supported)
		}
	}
}

// subgroupMaskModule builds a compute entry point whose arguments are the
// given ballot-mask built-ins, each of type vec4<u32>.
func subgroupMaskModule(builtins ...ir.BuiltinValue) *ir.Module {
	args := make([]ir.FunctionArgument, len(builtins))
	for i, builtin := range builtins {
		binding := ir.Binding(ir.BuiltinBinding{Builtin: builtin})
		args[i] = ir.FunctionArgument{Name: "mask", Type: 1, Binding: &binding}
	}
	return &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
		},
		EntryPoints: []ir.EntryPoint{{
			Name:      "main",
			Stage:     ir.StageCompute,
			Workgroup: [3]uint32{64, 1, 1},
			Function: ir.Function{
				Name:      "main",
				Arguments: args,
				Body:      []ir.Statement{{Kind: ir.StmtReturn{}}},
			},
		}},
	}
}
