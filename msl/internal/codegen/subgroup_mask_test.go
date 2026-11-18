package codegen

// The five ballot-mask built-ins (card P2-IR-042, ADR-014 D5). Metal has no
// such value, so the writer synthesizes each mask into a kernel local from the
// lane index and the simdgroup size: the mask spans every invocation of the
// simdgroup, and simd_ballot(true) holds only the active lanes.
// internal/subgroupmask is the semantic oracle.

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/internal/subgroupmask"
	"github.com/gogpu/naga/ir"
)

// TestMSL_SubgroupMaskBuiltins: each mask argument becomes a kernel local
// calling __naga_subgroup_mask with its kind over the synthetic lane and size
// parameters, and the synthesis functions are written once.
func TestMSL_SubgroupMaskBuiltins(t *testing.T) {
	src, _, err := Compile(subgroupMaskModule(
		ir.BuiltinSubgroupEqMask, ir.BuiltinSubgroupGeMask, ir.BuiltinSubgroupGtMask,
		ir.BuiltinSubgroupLeMask, ir.BuiltinSubgroupLtMask,
	), DefaultOptions())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	for kind := 0; kind <= 4; kind++ {
		call := "metal::__naga_subgroup_mask(" + string(rune('0'+kind)) + "u, __subgroup_invocation_id, __subgroup_size)"
		if !strings.Contains(src, call) {
			t.Errorf("kind %d: output lacks %q", kind, call)
		}
	}
	if got := strings.Count(src, "metal::uint __naga_subgroup_mask_word("); got != 1 {
		t.Errorf("__naga_subgroup_mask_word defined %d times, want 1", got)
	}
	if got := strings.Count(src, "[[threads_per_simdgroup]]"); got != 1 {
		t.Errorf("threads_per_simdgroup declared %d times, want 1", got)
	}
	if got := strings.Count(src, "[[thread_index_in_simdgroup]]"); got != 1 {
		t.Errorf("thread_index_in_simdgroup declared %d times, want 1", got)
	}
}

// TestMSL_SubgroupMaskWordsMatchTheOracle: the Go transcription of the
// emitted __naga_subgroup_mask_word formula equals internal/subgroupmask's
// spec-equation evaluator for every power-of-two size up to 128 and every
// lane, including the word boundaries and the partial last word — the check
// the emitted MSL itself cannot be run through locally.
func TestMSL_SubgroupMaskWordsMatchTheOracle(t *testing.T) {
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

// nagaSubgroupMaskWord transcribes the emitted MSL __naga_subgroup_mask_word
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

// TestMSL_SubgroupMaskStructMemberRefused: a mask as a struct member has no
// attribute a member could take, so the writer refuses it with the limitation
// named (only a direct mask argument is synthesized).
func TestMSL_SubgroupMaskStructMemberRefused(t *testing.T) {
	mod := subgroupMaskModule(ir.BuiltinSubgroupEqMask)
	// Move the mask from a direct argument into a struct member.
	maskBinding := *mod.EntryPoints[0].Function.Arguments[0].Binding
	mod.Types = append(mod.Types, ir.Type{Name: "", Inner: ir.StructType{Members: []ir.StructMember{
		{Name: "mask", Type: 1, Binding: &maskBinding},
	}}})
	structType := ir.TypeHandle(len(mod.Types) - 1)
	mod.EntryPoints[0].Function.Arguments = []ir.FunctionArgument{{Name: "io", Type: structType}}
	_, _, err := Compile(mod, DefaultOptions())
	if err == nil {
		t.Fatal("a struct-member mask compiled, want a refusal")
	}
	if !strings.Contains(err.Error(), "ballot-mask") || !strings.Contains(err.Error(), "struct member") {
		t.Errorf("error = %v, want the struct-member mask limitation", err)
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
