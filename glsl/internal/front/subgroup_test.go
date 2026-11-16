package front

// This file covers the GLSL subgroup builtins: GLSL 4.60 §8.18 and the
// GL_KHR_shader_subgroup_* extensions. Rust naga's GLSL frontend has none
// (upstream #7571), so every expectation here was measured with
// glslangValidator 11:16.2.0 (`-V --target-env vulkan1.1`, the SPIR-V 1.3 the
// subgroup operations need) and read back with spirv-dis. The tests pin the IR
// the frontend builds and the availability rules; the end-to-end matrix in
// snapshot/glsl_subgroup_matrix_test.go runs each form through glslang,
// validation, SPIR-V + spirv-val and the text backends.

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
)

// parseSubgroup parses and validates a shader source, failing on either step.
func parseSubgroup(t *testing.T, source string, stage ir.ShaderStage) *ir.Module {
	t.Helper()
	mod, _, err := Parse(source, Options{Stage: stage})
	if err != nil {
		t.Fatalf("Parse: %v\n\nsource:\n%s", err, source)
	}
	if _, verr := valid.Validate(mod, valid.Options{Capabilities: valid.CapAll}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v\n\nsource:\n%s", verr, source)
	}
	return mod
}

// subgroupStatements returns every statement of fn, walking the nested blocks a
// test source may use.
func subgroupStatements(fn ir.Function) []ir.Statement {
	var out []ir.Statement
	var walk func(block ir.Block)
	walk = func(block ir.Block) {
		out = append(out, block...)
		for _, stmt := range block {
			switch kind := stmt.Kind.(type) {
			case ir.StmtBlock:
				walk(kind.Block)
			case ir.StmtIf:
				walk(kind.Accept)
				walk(kind.Reject)
			case ir.StmtLoop:
				walk(kind.Body)
				walk(kind.Continuing)
			case ir.StmtSwitch:
				for _, c := range kind.Cases {
					walk(c.Body)
				}
			}
		}
	}
	walk(fn.Body)
	return out
}

// subgroupComputeSource is the header every compute-stage test source shares.
func subgroupComputeSource(extensions, body string) string {
	return "#version 450\n" + extensions +
		"layout(local_size_x = 64) in;\n" +
		"layout(set = 0, binding = 0) buffer Out { uint v; float f; } o;\n" +
		body
}

// TestSubgroupBarrierStatements pins the two barrier shapes: subgroupBarrier is
// a control barrier at subgroup scope, and the subgroupMemoryBarrier family is a
// memory barrier at subgroup scope. glslang emits OpControlBarrier / an
// OpMemoryBarrier with ScopeSubgroup for both (measured).
func TestSubgroupBarrierStatements(t *testing.T) {
	const extensions = "#extension GL_KHR_shader_subgroup_basic : require\n"

	t.Run("control barrier", func(t *testing.T) {
		mod := parseSubgroup(t, subgroupComputeSource(extensions,
			"shared uint counter;\nvoid main() { subgroupBarrier(); counter = 1u; }\n"), ir.StageCompute)
		for _, stmt := range subgroupStatements(mainFunction(t, mod)) {
			if barrier, isBarrier := stmt.Kind.(ir.StmtControlBarrier); isBarrier {
				if barrier.Flags != ir.BarrierSubGroup {
					t.Fatalf("barrier flags are %d, want BarrierSubGroup", barrier.Flags)
				}
				return
			}
		}
		t.Fatal("no StmtControlBarrier in the lowered function")
	})

	for _, call := range []string{
		"subgroupMemoryBarrier", "subgroupMemoryBarrierBuffer",
		"subgroupMemoryBarrierShared", "subgroupMemoryBarrierImage",
	} {
		t.Run(call, func(t *testing.T) {
			mod := parseSubgroup(t, subgroupComputeSource(extensions,
				"shared uint counter;\nvoid main() { "+call+"(); counter = 1u; }\n"), ir.StageCompute)
			for _, stmt := range subgroupStatements(mainFunction(t, mod)) {
				if barrier, isBarrier := stmt.Kind.(ir.StmtMemoryBarrier); isBarrier {
					if barrier.Flags != ir.BarrierSubGroup {
						t.Fatalf("barrier flags are %d, want BarrierSubGroup", barrier.Flags)
					}
					return
				}
			}
			t.Fatal("no StmtMemoryBarrier in the lowered function")
		})
	}
}

// TestSubgroupVote pins subgroupAll and subgroupAny: a collective operation with
// the vote operation, a reduce and the argument's own bool type as result.
func TestSubgroupVote(t *testing.T) {
	const extensions = "#extension GL_KHR_shader_subgroup_vote : require\n"
	cases := map[string]ir.SubgroupOperation{
		"subgroupAll": ir.SubgroupOperationAll,
		"subgroupAny": ir.SubgroupOperationAny,
	}
	for call, want := range cases {
		t.Run(call, func(t *testing.T) {
			mod := parseSubgroup(t, subgroupComputeSource(extensions,
				"void main() { if ("+call+"(gl_GlobalInvocationID.x > 3u)) { o.v = 1u; } }\n"), ir.StageCompute)
			op := firstSubgroupCollective(t, mod)
			if op.Op != want {
				t.Fatalf("operation is %d, want %d", op.Op, want)
			}
			if op.CollectiveOp != ir.CollectiveReduce {
				t.Fatalf("collective is %d, want a reduce", op.CollectiveOp)
			}
			resultType := mainFunction(t, mod).Expressions[op.Result].Kind
			result, isResult := resultType.(ir.ExprSubgroupOperationResult)
			if !isResult {
				t.Fatalf("result expression is %T, want ExprSubgroupOperationResult", resultType)
			}
			scalar, isScalar := mod.Types[result.Type].Inner.(ir.ScalarType)
			if !isScalar || scalar.Kind != ir.ScalarBool {
				t.Fatalf("result type is %v, want bool", mod.Types[result.Type].Inner)
			}
		})
	}
}

// firstSubgroupCollective returns the first StmtSubgroupCollectiveOperation of
// the main function.
func firstSubgroupCollective(t *testing.T, mod *ir.Module) ir.StmtSubgroupCollectiveOperation {
	t.Helper()
	for _, stmt := range subgroupStatements(mainFunction(t, mod)) {
		if op, ok := stmt.Kind.(ir.StmtSubgroupCollectiveOperation); ok {
			return op
		}
	}
	t.Fatal("no StmtSubgroupCollectiveOperation in the lowered function")
	return ir.StmtSubgroupCollectiveOperation{}
}

// firstSubgroupGather returns the first StmtSubgroupGather of the main
// function.
func firstSubgroupGather(t *testing.T, mod *ir.Module) ir.StmtSubgroupGather {
	t.Helper()
	for _, stmt := range subgroupStatements(mainFunction(t, mod)) {
		if gather, ok := stmt.Kind.(ir.StmtSubgroupGather); ok {
			return gather
		}
	}
	t.Fatal("no StmtSubgroupGather in the lowered function")
	return ir.StmtSubgroupGather{}
}

// TestSubgroupBallot pins subgroupBallot: an ir.StmtSubgroupBallot whose
// predicate is the argument and whose result is typed vec4<u32>.
func TestSubgroupBallot(t *testing.T) {
	const extensions = "#extension GL_KHR_shader_subgroup_ballot : require\n"
	mod := parseSubgroup(t, subgroupComputeSource(extensions,
		"void main() { uvec4 mask = subgroupBallot(gl_GlobalInvocationID.x > 3u); o.v = mask.x; }\n"), ir.StageCompute)
	for _, stmt := range subgroupStatements(mainFunction(t, mod)) {
		ballot, ok := stmt.Kind.(ir.StmtSubgroupBallot)
		if !ok {
			continue
		}
		if ballot.Predicate == nil {
			t.Fatal("the ballot has no predicate; GLSL always passes one")
		}
		kind := mainFunction(t, mod).Expressions[ballot.Result].Kind
		if _, isResult := kind.(ir.ExprSubgroupBallotResult); !isResult {
			t.Fatalf("result expression is %T, want ExprSubgroupBallotResult", kind)
		}
		return
	}
	t.Fatal("no StmtSubgroupBallot in the lowered function")
}

// TestSubgroupGather pins the gather modes of the broadcast, shuffle and quad
// families: each builtin maps to the ir.GatherMode glslang's SPIR-V opcode
// names (OpGroupNonUniformBroadcast, ...ShuffleXor, ...QuadSwap with
// direction 0/1/2).
func TestSubgroupGather(t *testing.T) {
	const ballotExt = "#extension GL_KHR_shader_subgroup_ballot : require\n"
	const shuffleExt = "#extension GL_KHR_shader_subgroup_shuffle : require\n" +
		"#extension GL_KHR_shader_subgroup_shuffle_relative : require\n"
	const quadExt = "#extension GL_KHR_shader_subgroup_quad : require\n"

	cases := []struct {
		name       string
		extensions string
		call       string
		mode       ir.GatherMode
	}{
		{"broadcast", ballotExt, "subgroupBroadcast(1u, 2u)", ir.GatherBroadcast{}},
		{"broadcast first", ballotExt, "subgroupBroadcastFirst(1u)", ir.GatherBroadcastFirst{}},
		{"shuffle", shuffleExt, "subgroupShuffle(1u, 2u)", ir.GatherShuffle{}},
		{"shuffle xor", shuffleExt, "subgroupShuffleXor(1u, 2u)", ir.GatherShuffleXor{}},
		{"shuffle up", shuffleExt, "subgroupShuffleUp(1u, 2u)", ir.GatherShuffleUp{}},
		{"shuffle down", shuffleExt, "subgroupShuffleDown(1u, 2u)", ir.GatherShuffleDown{}},
		{"quad broadcast", quadExt, "subgroupQuadBroadcast(1u, 2u)", ir.GatherQuadBroadcast{}},
		{"quad swap horizontal", quadExt, "subgroupQuadSwapHorizontal(1u)", ir.GatherQuadSwap{Direction: ir.QuadDirectionX}},
		{"quad swap vertical", quadExt, "subgroupQuadSwapVertical(1u)", ir.GatherQuadSwap{Direction: ir.QuadDirectionY}},
		{"quad swap diagonal", quadExt, "subgroupQuadSwapDiagonal(1u)", ir.GatherQuadSwap{Direction: ir.QuadDirectionDiagonal}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mod := parseSubgroup(t, subgroupComputeSource(tc.extensions,
				"void main() { o.v = "+tc.call+"; }\n"), ir.StageCompute)
			gather := firstSubgroupGather(t, mod)
			if got, want := gatherModeName(gather.Mode), gatherModeName(tc.mode); got != want {
				t.Fatalf("gather mode is %s, want %s", got, want)
			}
			switch got := gather.Mode.(type) {
			case ir.GatherQuadSwap:
				want := tc.mode.(ir.GatherQuadSwap)
				if got.Direction != want.Direction {
					t.Fatalf("quad direction is %d, want %d", got.Direction, want.Direction)
				}
			}
		})
	}
}

// gatherModeName is a readable name for a gather mode, for failure messages.
func gatherModeName(mode ir.GatherMode) string {
	switch mode.(type) {
	case ir.GatherBroadcast:
		return "GatherBroadcast"
	case ir.GatherBroadcastFirst:
		return "GatherBroadcastFirst"
	case ir.GatherShuffle:
		return "GatherShuffle"
	case ir.GatherShuffleXor:
		return "GatherShuffleXor"
	case ir.GatherShuffleUp:
		return "GatherShuffleUp"
	case ir.GatherShuffleDown:
		return "GatherShuffleDown"
	case ir.GatherQuadBroadcast:
		return "GatherQuadBroadcast"
	case ir.GatherQuadSwap:
		return "GatherQuadSwap"
	}
	return "unknown"
}

// TestSubgroupArithmetic pins the reductions and the Add/Mul scans: each GLSL
// builtin maps to the IR operation and collective glslang's SPIR-V opcode
// carries (OpGroupNonUniformIAdd / FAdd ... with Reduce, InclusiveScan or
// ExclusiveScan).
func TestSubgroupArithmetic(t *testing.T) {
	const extensions = "#extension GL_KHR_shader_subgroup_arithmetic : require\n"
	cases := []struct {
		call string
		op   ir.SubgroupOperation
		cop  ir.CollectiveOperation
	}{
		{"subgroupAdd", ir.SubgroupOperationAdd, ir.CollectiveReduce},
		{"subgroupMul", ir.SubgroupOperationMul, ir.CollectiveReduce},
		{"subgroupMin", ir.SubgroupOperationMin, ir.CollectiveReduce},
		{"subgroupMax", ir.SubgroupOperationMax, ir.CollectiveReduce},
		{"subgroupAnd", ir.SubgroupOperationAnd, ir.CollectiveReduce},
		{"subgroupOr", ir.SubgroupOperationOr, ir.CollectiveReduce},
		{"subgroupXor", ir.SubgroupOperationXor, ir.CollectiveReduce},
		{"subgroupInclusiveAdd", ir.SubgroupOperationAdd, ir.CollectiveInclusiveScan},
		{"subgroupExclusiveAdd", ir.SubgroupOperationAdd, ir.CollectiveExclusiveScan},
		{"subgroupInclusiveMul", ir.SubgroupOperationMul, ir.CollectiveInclusiveScan},
		{"subgroupExclusiveMul", ir.SubgroupOperationMul, ir.CollectiveExclusiveScan},
	}
	for _, tc := range cases {
		t.Run(tc.call, func(t *testing.T) {
			mod := parseSubgroup(t, subgroupComputeSource(extensions,
				"void main() { o.v = "+tc.call+"(gl_GlobalInvocationID.x); }\n"), ir.StageCompute)
			op := firstSubgroupCollective(t, mod)
			if op.Op != tc.op || op.CollectiveOp != tc.cop {
				t.Fatalf("operation is %d/%d, want %d/%d", op.Op, op.CollectiveOp, tc.op, tc.cop)
			}
		})
	}
}

// TestSubgroupArithmeticFloat checks a float reduction: the result keeps the
// argument's float type.
func TestSubgroupArithmeticFloat(t *testing.T) {
	const extensions = "#extension GL_KHR_shader_subgroup_arithmetic : require\n"
	mod := parseSubgroup(t, subgroupComputeSource(extensions,
		"void main() { o.f = subgroupAdd(1.5); }\n"), ir.StageCompute)
	op := firstSubgroupCollective(t, mod)
	if op.Op != ir.SubgroupOperationAdd {
		t.Fatalf("operation is %d, want add", op.Op)
	}
	result := mainFunction(t, mod).Expressions[op.Result].Kind.(ir.ExprSubgroupOperationResult)
	scalar, isScalar := mod.Types[result.Type].Inner.(ir.ScalarType)
	if !isScalar || scalar.Kind != ir.ScalarFloat || scalar.Width != 4 {
		t.Fatalf("result type is %v, want f32", mod.Types[result.Type].Inner)
	}
}

// TestSubgroupExtensionGate pins the availability rule: each family needs its
// own extension, the basic family is enabled by any of them (each of the seven
// requires GL_KHR_shader_subgroup_basic), and the diagnostic is glslang's
// "required extension not requested: NAME".
func TestSubgroupExtensionGate(t *testing.T) {
	cases := []struct {
		name    string
		ext     string // the directive the source writes, if any
		use     string
		refusal string
	}{
		{"arithmetic without extension", "", "o.v = subgroupAdd(1u);",
			"'subgroupAdd' : required extension not requested: GL_KHR_shader_subgroup_arithmetic"},
		{"vote without extension", "", "if (subgroupAll(true)) { o.v = 1u; }",
			"'subgroupAll' : required extension not requested: GL_KHR_shader_subgroup_vote"},
		{"ballot without extension", "", "uvec4 m = subgroupBallot(true); o.v = m.x;",
			"'subgroupBallot' : required extension not requested: GL_KHR_shader_subgroup_ballot"},
		{"barrier without any extension", "", "subgroupBarrier();",
			"'subgroupBarrier' : required extension not requested: GL_KHR_shader_subgroup_basic"},
		{"shuffle relative needs its own extension",
			"#extension GL_KHR_shader_subgroup_shuffle : require\n", "o.v = subgroupShuffleUp(1u, 1u);",
			"'subgroupShuffleUp' : required extension not requested: GL_KHR_shader_subgroup_shuffle_relative"},
		{"quad needs its own extension",
			"#extension GL_KHR_shader_subgroup_ballot : require\n", "o.v = subgroupQuadBroadcast(1u, 0u);",
			"'subgroupQuadBroadcast' : required extension not requested: GL_KHR_shader_subgroup_quad"},
		{"clustered needs its own extension",
			"#extension GL_KHR_shader_subgroup_arithmetic : require\n", "o.v = subgroupClusteredAdd(1u, 4u);",
			"'subgroupClusteredAdd' : required extension not requested: GL_KHR_shader_subgroup_clustered"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parseErr(t, subgroupComputeSource(tc.ext, "void main() { "+tc.use+" }\n"),
				Options{Stage: ir.StageCompute}, tc.refusal)
		})
	}

	t.Run("any family enables the basic builtins", func(t *testing.T) {
		// glslang accepts subgroupBarrier with only GL_KHR_shader_subgroup_arithmetic
		// requested, because every family extension requires the basic one.
		parseSubgroup(t, subgroupComputeSource(
			"#extension GL_KHR_shader_subgroup_arithmetic : require\n",
			"shared uint counter;\nvoid main() { subgroupBarrier(); subgroupMemoryBarrierShared(); counter = 1u; }\n"),
			ir.StageCompute)
	})

	t.Run("disable turns a family off", func(t *testing.T) {
		parseErr(t, subgroupComputeSource(
			"#extension GL_KHR_shader_subgroup_arithmetic : require\n"+
				"#extension GL_KHR_shader_subgroup_arithmetic : disable\n",
			"void main() { o.v = subgroupAdd(1u); }\n"),
			Options{Stage: ir.StageCompute},
			"'subgroupAdd' : required extension not requested: GL_KHR_shader_subgroup_arithmetic")
	})
}

// TestSubgroupGL330WithExtension: the GL 3.30 core dialect gets the subgroup
// builtins through the extensions, as design §15's matrix and glslang say
// (glslangValidator: `#version 330` plus the arithmetic directive compiles
// subgroupAdd, and the same source without the directive reports "required
// extension not requested").
func TestSubgroupGL330WithExtension(t *testing.T) {
	const source = `#version 330
#extension GL_KHR_shader_subgroup_arithmetic : require
layout(location = 0) out vec4 color;
void main() { color = vec4(float(subgroupAdd(1u))); }
`
	mod := parseSubgroup(t, source, ir.StageFragment)
	if op := firstSubgroupCollective(t, mod); op.Op != ir.SubgroupOperationAdd {
		t.Fatalf("operation is %d, want add", op.Op)
	}

	parseErr(t, `#version 330
layout(location = 0) out vec4 color;
void main() { color = vec4(float(subgroupAdd(1u))); }
`, Options{Stage: ir.StageFragment},
		"'subgroupAdd' : required extension not requested: GL_KHR_shader_subgroup_arithmetic")
}

// TestSubgroupES300Refused: GLSL ES has no subgroup operations before 3.10, so
// a 300 es source is refused by the version rule even with the extension
// requested (glslang: "'subgroupAdd' : no matching overloaded function found").
func TestSubgroupES300Refused(t *testing.T) {
	parseErr(t, `#version 300 es
#extension GL_KHR_shader_subgroup_arithmetic : require
precision highp float;
out vec4 color;
void main() { color = vec4(float(subgroupAdd(1u))); }
`, Options{Stage: ir.StageFragment},
		"'subgroupAdd' : not supported by GLSL ES 3.00 (subgroup operations require GLSL ES 3.10)")
}

// TestSubgroupGapRefusals pins the forms the frontend recognizes but cannot
// lower: each names the missing IR or backend capability rather than an
// unknown-function text.
func TestSubgroupGapRefusals(t *testing.T) {
	all := "#extension GL_KHR_shader_subgroup_basic : require\n" +
		"#extension GL_KHR_shader_subgroup_vote : require\n" +
		"#extension GL_KHR_shader_subgroup_ballot : require\n" +
		"#extension GL_KHR_shader_subgroup_arithmetic : require\n" +
		"#extension GL_KHR_shader_subgroup_clustered : require\n"
	cases := []struct {
		name    string
		use     string
		refusal string
	}{
		{"elect", "if (subgroupElect()) { o.v = 1u; }", "no subgroup elect statement"},
		{"all equal", "if (subgroupAllEqual(gl_GlobalInvocationID.x)) { o.v = 1u; }",
			"ir.SubgroupOperation has All and Any but no AllEqual"},
		{"ballot bit extract", "o.v = subgroupBallotBitExtract(subgroupBallot(true), 1u) ? 1u : 0u;",
			"no ballot bit extract, bit count or find LSB/MSB"},
		{"ballot bit count", "o.v = subgroupBallotBitCount(subgroupBallot(true));",
			"no ballot bit extract, bit count or find LSB/MSB"},
		{"ballot inclusive bit count", "o.v = subgroupBallotInclusiveBitCount(subgroupBallot(true));",
			"no ballot bit extract, bit count or find LSB/MSB"},
		{"ballot exclusive bit count", "o.v = subgroupBallotExclusiveBitCount(subgroupBallot(true));",
			"no ballot bit extract, bit count or find LSB/MSB"},
		{"ballot find lsb", "o.v = subgroupBallotFindLSB(subgroupBallot(true));",
			"no ballot bit extract, bit count or find LSB/MSB"},
		{"ballot find msb", "o.v = subgroupBallotFindMSB(subgroupBallot(true));",
			"no ballot bit extract, bit count or find LSB/MSB"},
		{"clustered add", "o.v = subgroupClusteredAdd(1u, 4u);", "no ClusteredReduce"},
		{"clustered mul", "o.v = subgroupClusteredMul(1u, 4u);", "no ClusteredReduce"},
		{"clustered min", "o.v = subgroupClusteredMin(1u, 4u);", "no ClusteredReduce"},
		{"clustered max", "o.v = subgroupClusteredMax(1u, 4u);", "no ClusteredReduce"},
		{"clustered and", "o.v = subgroupClusteredAnd(1u, 4u);", "no ClusteredReduce"},
		{"clustered or", "o.v = subgroupClusteredOr(1u, 4u);", "no ClusteredReduce"},
		{"clustered xor", "o.v = subgroupClusteredXor(1u, 4u);", "no ClusteredReduce"},
		{"inclusive min", "o.v = subgroupInclusiveMin(1u);", "the Min/Max/And/Or/Xor scans"},
		{"inclusive max", "o.v = subgroupInclusiveMax(1u);", "the Min/Max/And/Or/Xor scans"},
		{"inclusive and", "o.v = subgroupInclusiveAnd(1u);", "the Min/Max/And/Or/Xor scans"},
		{"inclusive or", "o.v = subgroupInclusiveOr(1u);", "the Min/Max/And/Or/Xor scans"},
		{"inclusive xor", "o.v = subgroupInclusiveXor(1u);", "the Min/Max/And/Or/Xor scans"},
		{"exclusive min", "o.v = subgroupExclusiveMin(1u);", "the Min/Max/And/Or/Xor scans"},
		{"exclusive max", "o.v = subgroupExclusiveMax(1u);", "the Min/Max/And/Or/Xor scans"},
		{"exclusive and", "o.v = subgroupExclusiveAnd(1u);", "the Min/Max/And/Or/Xor scans"},
		{"exclusive or", "o.v = subgroupExclusiveOr(1u);", "the Min/Max/And/Or/Xor scans"},
		{"exclusive xor", "o.v = subgroupExclusiveXor(1u);", "the Min/Max/And/Or/Xor scans"},
		{"atomic counter barrier", "subgroupMemoryBarrierAtomicCounter();", "atomic counters do not exist in Vulkan GLSL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parseErr(t, subgroupComputeSource(all, "void main() { "+tc.use+" }\n"),
				Options{Stage: ir.StageCompute}, tc.refusal)
		})
	}
}

// TestSubgroupOperandTypes pins the operand checks: a vote takes a bool scalar,
// a ballot a bool predicate, an arithmetic collective a numeric value, a gather
// a 32-bit integer index, and a wrong count matches no overload.
func TestSubgroupOperandTypes(t *testing.T) {
	const extensions = "#extension GL_KHR_shader_subgroup_basic : require\n" +
		"#extension GL_KHR_shader_subgroup_vote : require\n" +
		"#extension GL_KHR_shader_subgroup_ballot : require\n" +
		"#extension GL_KHR_shader_subgroup_shuffle : require\n" +
		"#extension GL_KHR_shader_subgroup_arithmetic : require\n"
	cases := []struct {
		name string
		use  string
	}{
		{"all takes a bool scalar", "if (subgroupAll(1u)) { o.v = 1u; }"},
		{"ballot takes a bool predicate", "uvec4 m = subgroupBallot(1u); o.v = m.x;"},
		{"add takes a numeric value", "o.v = subgroupAdd(true);"},
		{"shuffle index is an integer", "o.v = subgroupShuffle(1u, 1.0);"},
		{"shuffle takes two arguments", "o.v = subgroupShuffle(1u);"},
		{"broadcast takes two arguments", "o.v = subgroupBroadcast(1u);"},
		{"barrier takes no argument", "subgroupBarrier(1);"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parseErr(t, subgroupComputeSource(extensions, "void main() { "+tc.use+" }\n"),
				Options{Stage: ir.StageCompute}, "Unknown function '")
		})
	}
}

// TestSubgroupDeclaredFunctionShadows: a function the shader declares under a
// subgroup builtin's name is called instead of the builtin, as the atomic path
// treats a declared name.
func TestSubgroupDeclaredFunctionShadows(t *testing.T) {
	const extensions = "#extension GL_KHR_shader_subgroup_arithmetic : require\n"
	mod := parseSubgroup(t, subgroupComputeSource(extensions,
		"uint subgroupAdd(uint x) { return x + 1u; }\nvoid main() { o.v = subgroupAdd(1u); }\n"), ir.StageCompute)
	for _, stmt := range subgroupStatements(mainFunction(t, mod)) {
		if _, isSubgroup := stmt.Kind.(ir.StmtSubgroupCollectiveOperation); isSubgroup {
			t.Fatal("the declared function was not called; the builtin was lowered instead")
		}
	}
}

// TestSubgroupVariableRefusal: the subgroup built-in variables are not wired
// into the entry point yet, and their use is refused with a message naming the
// limitation rather than an unknown-name text.
func TestSubgroupVariableRefusal(t *testing.T) {
	for _, name := range []string{"gl_SubgroupSize", "gl_SubgroupInvocationID", "gl_SubgroupID", "gl_NumSubgroups"} {
		t.Run(name, func(t *testing.T) {
			_, _, err := Parse(subgroupComputeSource(
				"#extension GL_KHR_shader_subgroup_basic : require\n",
				"void main() { o.v = "+name+"; }\n"), Options{Stage: ir.StageCompute})
			if err == nil {
				t.Fatalf("parsed, want a refusal for %s", name)
			}
			if !strings.Contains(err.Error(), "subgroup built-in variables of GL_KHR_shader_subgroup_basic are not implemented") {
				t.Fatalf("refusal is %q, want it to name the missing entry-point wiring", err.Error())
			}
		})
	}
}
