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

// TestSubgroupElect pins subgroupElect: a SubgroupElect statement whose result
// is a bool operation-result expression — the shape ADR-014 gives the form
// (glslang emits OpGroupNonUniformElect, probed 2026-11-16).
func TestSubgroupElect(t *testing.T) {
	const extensions = "#extension GL_KHR_shader_subgroup_basic : require\n"
	mod := parseSubgroup(t, subgroupComputeSource(extensions,
		"void main() { if (subgroupElect()) { o.v = 1u; } }\n"), ir.StageCompute)
	fn := mainFunction(t, mod)

	var elect *ir.StmtSubgroupElect
	for _, stmt := range subgroupStatements(fn) {
		if kind, ok := stmt.Kind.(ir.StmtSubgroupElect); ok {
			elect = &kind
		}
	}
	if elect == nil {
		t.Fatal("no StmtSubgroupElect in the lowered function")
	}
	assertSubgroupBoolResult(t, mod, fn, elect.Result, "elect")

	// subgroupElect takes no arguments; a call with one is not the builtin.
	parseErr(t, subgroupComputeSource(extensions,
		"void main() { if (subgroupElect(true)) { o.v = 1u; } }\n"),
		Options{Stage: ir.StageCompute}, "Unknown function 'subgroupElect'")
}

// TestSubgroupAllEqual pins subgroupAllEqual: a collective operation with
// SubgroupOperationAllEqual, a reduce, and a bool result whatever the
// argument's type is. The argument may be any scalar or vector (GLSL's
// genType, genIType, genUType, genBType, genDType); glslang accepts uint,
// float, uvec2 and bool operands (probed 2026-11-16).
func TestSubgroupAllEqual(t *testing.T) {
	const extensions = "#extension GL_KHR_shader_subgroup_vote : require\n"
	cases := map[string]string{
		"uint":  "gl_GlobalInvocationID.x",
		"float": "1.5",
		"uvec2": "uvec2(1u, 2u)",
		"bool":  "true",
		"ivec4": "ivec4(1, 2, 3, 4)",
	}
	for name, argument := range cases {
		t.Run(name, func(t *testing.T) {
			mod := parseSubgroup(t, subgroupComputeSource(extensions,
				"void main() { if (subgroupAllEqual("+argument+")) { o.v = 1u; } }\n"), ir.StageCompute)
			fn := mainFunction(t, mod)

			var op *ir.StmtSubgroupCollectiveOperation
			for _, stmt := range subgroupStatements(fn) {
				if kind, ok := stmt.Kind.(ir.StmtSubgroupCollectiveOperation); ok {
					op = &kind
				}
			}
			if op == nil {
				t.Fatal("no StmtSubgroupCollectiveOperation in the lowered function")
			}
			if op.Op != ir.SubgroupOperationAllEqual {
				t.Fatalf("operation is %d, want SubgroupOperationAllEqual", op.Op)
			}
			if op.CollectiveOp != ir.CollectiveReduce {
				t.Fatalf("collective operation is %d, want Reduce (all-equal has no scan form)", op.CollectiveOp)
			}
			assertSubgroupBoolResult(t, mod, fn, op.Result, "all-equal")
		})
	}
}

// assertSubgroupBoolResult checks that a subgroup statement's result is an
// operation-result expression of the bool scalar type.
func assertSubgroupBoolResult(t *testing.T, mod *ir.Module, fn ir.Function, result ir.ExpressionHandle, form string) {
	t.Helper()
	res, ok := fn.Expressions[result].Kind.(ir.ExprSubgroupOperationResult)
	if !ok {
		t.Fatalf("%s result is %T, want ExprSubgroupOperationResult", form, fn.Expressions[result].Kind)
	}
	scalar, ok := mod.Types[res.Type].Inner.(ir.ScalarType)
	if !ok || scalar.Kind != ir.ScalarBool || scalar.Width != 1 {
		t.Fatalf("%s result type is %+v, want a bool scalar", form, mod.Types[res.Type].Inner)
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

// TestSubgroupBallotBitForms pins the seven ballot bit builtins: each lowers
// to ir.StmtSubgroupBallotBit with its operation over the ballot mask, and a
// bool result for extract and inverse ballot, u32 for the counts and finds.
// glslang emits OpGroupNonUniformBallotBitExtract, BitCount with Reduce /
// InclusiveScan / ExclusiveScan, FindLSB, FindMSB and InverseBallot for them
// (probed 2026-11-17; card P2-IR-039, ADR-014 D3).
func TestSubgroupBallotBitForms(t *testing.T) {
	const extensions = "#extension GL_KHR_shader_subgroup_ballot : require\n"
	cases := []struct {
		name       string
		call       string
		op         ir.SubgroupBallotBitOperation
		boolResult bool
	}{
		{"extract", "subgroupBallotBitExtract(subgroupBallot(true), 1u) ? 1u : 0u", ir.SubgroupBallotBitExtract, true},
		{"count", "subgroupBallotBitCount(subgroupBallot(true))", ir.SubgroupBallotBitCount, false},
		{"inclusive count", "subgroupBallotInclusiveBitCount(subgroupBallot(true))", ir.SubgroupBallotBitInclusiveCount, false},
		{"exclusive count", "subgroupBallotExclusiveBitCount(subgroupBallot(true))", ir.SubgroupBallotBitExclusiveCount, false},
		{"find lsb", "subgroupBallotFindLSB(subgroupBallot(true))", ir.SubgroupBallotFindLSB, false},
		{"find msb", "subgroupBallotFindMSB(subgroupBallot(true))", ir.SubgroupBallotFindMSB, false},
		{"inverse ballot", "subgroupInverseBallot(subgroupBallot(true)) ? 1u : 0u", ir.SubgroupInverseBallot, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mod := parseSubgroup(t, subgroupComputeSource(extensions,
				"void main() { o.v = "+tc.call+"; }\n"), ir.StageCompute)
			fn := mainFunction(t, mod)
			var bit *ir.StmtSubgroupBallotBit
			for _, stmt := range subgroupStatements(fn) {
				if kind, ok := stmt.Kind.(ir.StmtSubgroupBallotBit); ok {
					bit = &kind
				}
			}
			if bit == nil {
				t.Fatal("no StmtSubgroupBallotBit in the lowered function")
			}
			if bit.Op != tc.op {
				t.Fatalf("operation is %d, want %d", bit.Op, tc.op)
			}
			if _, ok := fn.Expressions[bit.Mask].Kind.(ir.ExprSubgroupBallotResult); !ok {
				t.Fatalf("mask expression is %T, want the ballot result", fn.Expressions[bit.Mask].Kind)
			}
			res, ok := fn.Expressions[bit.Result].Kind.(ir.ExprSubgroupOperationResult)
			if !ok {
				t.Fatalf("result expression is %T, want ExprSubgroupOperationResult", fn.Expressions[bit.Result].Kind)
			}
			scalar, ok := mod.Types[res.Type].Inner.(ir.ScalarType)
			wantKind := ir.ScalarUint
			if tc.boolResult {
				wantKind = ir.ScalarBool
			}
			if !ok || scalar.Kind != wantKind {
				t.Fatalf("result type is %+v, want scalar kind %d", mod.Types[res.Type].Inner, wantKind)
			}
			if tc.op == ir.SubgroupBallotBitExtract {
				lit, ok := fn.Expressions[bit.Index].Kind.(ir.Literal)
				if !ok || lit.Value != ir.LiteralU32(1) {
					t.Fatalf("index expression is %T, want the u32 literal 1", fn.Expressions[bit.Index].Kind)
				}
			}
		})
	}
}

// TestSubgroupBallotBitConversions: glslang implicitly converts an ivec4 mask
// and an int extract index to uint (OpBitcast %v4uint before the operation,
// probed 2026-11-17); the frontend converts rather than refuses, and the
// module validates.
func TestSubgroupBallotBitConversions(t *testing.T) {
	const extensions = "#extension GL_KHR_shader_subgroup_ballot : require\n"
	mod := parseSubgroup(t, subgroupComputeSource(extensions,
		"void main() { ivec4 sm = ivec4(subgroupBallot(true));"+
			" o.v = subgroupBallotBitCount(sm);"+
			" o.v = subgroupBallotBitExtract(sm, 1) ? 1u : 0u; }\n"), ir.StageCompute)
	fn := mainFunction(t, mod)
	var count, extract *ir.StmtSubgroupBallotBit
	for _, stmt := range subgroupStatements(fn) {
		if kind, ok := stmt.Kind.(ir.StmtSubgroupBallotBit); ok {
			switch kind.Op {
			case ir.SubgroupBallotBitCount:
				count = &kind
			case ir.SubgroupBallotBitExtract:
				extract = &kind
			}
		}
	}
	if count == nil || extract == nil {
		t.Fatal("the count and extract statements are not both lowered")
	}
	// The ivec4 masks convert to uint; a runtime value cannot fold away.
	for _, h := range []ir.ExpressionHandle{count.Mask, extract.Mask} {
		as, ok := fn.Expressions[h].Kind.(ir.ExprAs)
		if !ok || as.Kind != ir.ScalarUint {
			t.Fatalf("mask operand is %T, want an ExprAs to uint", fn.Expressions[h].Kind)
		}
	}
	// The int index converts to uint, or folds to a u32 literal.
	switch idx := fn.Expressions[extract.Index].Kind.(type) {
	case ir.ExprAs:
		if idx.Kind != ir.ScalarUint {
			t.Fatalf("index converts to scalar kind %d, want uint", idx.Kind)
		}
	case ir.Literal:
		if idx.Value != ir.LiteralU32(1) {
			t.Fatalf("index literal is %v, want u32(1)", idx.Value)
		}
	default:
		t.Fatalf("index operand is %T, want an ExprAs to uint or a u32 literal", fn.Expressions[extract.Index].Kind)
	}
}

// TestSubgroupBallotBitOperandTypes: the mask is a 32-bit integer vec4 and
// the extract index a 32-bit integer scalar; anything else is not the
// builtin (glslang: "no matching overloaded function found").
func TestSubgroupBallotBitOperandTypes(t *testing.T) {
	const extensions = "#extension GL_KHR_shader_subgroup_ballot : require\n"
	cases := []struct{ name, use string }{
		{"scalar mask", "o.v = subgroupBallotBitCount(1u);"},
		{"vec3 mask", "o.v = subgroupBallotBitCount(uvec3(1u));"},
		{"float mask", "o.v = subgroupBallotBitCount(vec4(1.0));"},
		{"bool mask", "o.v = subgroupBallotBitCount(bvec4(true));"},
		{"float index", "o.v = subgroupBallotBitExtract(subgroupBallot(true), 1.0) ? 1u : 0u;"},
		{"vector index", "o.v = subgroupBallotBitExtract(subgroupBallot(true), uvec2(1u)) ? 1u : 0u;"},
		{"extract without index", "o.v = subgroupBallotBitExtract(subgroupBallot(true)) ? 1u : 0u;"},
		{"count with two args", "o.v = subgroupBallotBitCount(subgroupBallot(true), 1u);"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parseErr(t, subgroupComputeSource(extensions, "void main() { "+tc.use+" }\n"),
				Options{Stage: ir.StageCompute}, "Unknown function")
		})
	}
}

// TestSubgroupClusteredForms pins the seven clustered builtins
// (GL_KHR_shader_subgroup §3.8): each lowers to
// ir.StmtSubgroupCollectiveOperation with ClusteredReduce, the builtin's
// operation, and the cluster size the second argument names as a constant.
// glslang emits OpGroupNonUniform*<op> ClusteredReduce with the cluster size
// as the optional operand (probed 2026-11-18; card P2-IR-040, ADR-014 D4).
func TestSubgroupClusteredForms(t *testing.T) {
	const extensions = "#extension GL_KHR_shader_subgroup_clustered : require\n"
	cases := []struct {
		name string
		call string
		op   ir.SubgroupOperation
	}{
		{"add", "subgroupClusteredAdd(gl_GlobalInvocationID.x, 4u)", ir.SubgroupOperationAdd},
		{"mul", "subgroupClusteredMul(gl_GlobalInvocationID.x + 1u, 4u)", ir.SubgroupOperationMul},
		{"min", "subgroupClusteredMin(gl_GlobalInvocationID.x, 4u)", ir.SubgroupOperationMin},
		{"max", "subgroupClusteredMax(gl_GlobalInvocationID.x, 4u)", ir.SubgroupOperationMax},
		{"and", "subgroupClusteredAnd(gl_GlobalInvocationID.x, 4u)", ir.SubgroupOperationAnd},
		{"or", "subgroupClusteredOr(gl_GlobalInvocationID.x, 4u)", ir.SubgroupOperationOr},
		{"xor", "subgroupClusteredXor(gl_GlobalInvocationID.x, 4u)", ir.SubgroupOperationXor},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mod := parseSubgroup(t, subgroupComputeSource(extensions,
				"void main() { o.v = "+tc.call+"; }\n"), ir.StageCompute)
			op := firstSubgroupCollective(t, mod)
			if op.Op != tc.op {
				t.Fatalf("operation is %d, want %d", op.Op, tc.op)
			}
			if op.CollectiveOp != ir.CollectiveOperationClusteredReduce {
				t.Fatalf("collective is %d, want a clustered reduce", op.CollectiveOp)
			}
			if op.ClusterSize != 4 {
				t.Fatalf("cluster size is %d, want 4", op.ClusterSize)
			}
			res, ok := mainFunction(t, mod).Expressions[op.Result].Kind.(ir.ExprSubgroupOperationResult)
			if !ok {
				t.Fatalf("result expression is %T, want ExprSubgroupOperationResult", mainFunction(t, mod).Expressions[op.Result].Kind)
			}
			scalar, isScalar := mod.Types[res.Type].Inner.(ir.ScalarType)
			if !isScalar || scalar.Kind != ir.ScalarUint {
				t.Fatalf("result type is %v, want the argument's uint", mod.Types[res.Type].Inner)
			}
		})
	}
}

// TestSubgroupClusteredOperands: the value types of the clustered family —
// the arithmetic four take int, uint, float and double, the bitwise three add
// bool (the extension's genBType overloads) — the cluster size accepts an int
// literal (glslang converts it to uint, probed 2026-11-18) and a
// const-qualified local, and its errors are glslang's texts.
func TestSubgroupClusteredOperands(t *testing.T) {
	const extensions = "#extension GL_KHR_shader_subgroup_clustered : require\n"

	t.Run("float add", func(t *testing.T) {
		mod := parseSubgroup(t, subgroupComputeSource(extensions,
			"void main() { o.f = subgroupClusteredAdd(float(gl_GlobalInvocationID.x), 4u); }\n"), ir.StageCompute)
		if op := firstSubgroupCollective(t, mod); op.Op != ir.SubgroupOperationAdd {
			t.Fatalf("operation is %d, want add", op.Op)
		}
	})
	t.Run("bool and", func(t *testing.T) {
		mod := parseSubgroup(t, subgroupComputeSource(extensions,
			"void main() { o.v = subgroupClusteredAnd(gl_GlobalInvocationID.x > 3u, 4u) ? 1u : 0u; }\n"), ir.StageCompute)
		if op := firstSubgroupCollective(t, mod); op.Op != ir.SubgroupOperationAnd {
			t.Fatalf("operation is %d, want and", op.Op)
		}
	})
	t.Run("int cluster size", func(t *testing.T) {
		mod := parseSubgroup(t, subgroupComputeSource(extensions,
			"void main() { o.v = subgroupClusteredAdd(gl_GlobalInvocationID.x, 4); }\n"), ir.StageCompute)
		if op := firstSubgroupCollective(t, mod); op.ClusterSize != 4 {
			t.Fatalf("cluster size is %d, want 4", op.ClusterSize)
		}
	})
	t.Run("const local cluster size", func(t *testing.T) {
		mod := parseSubgroup(t, subgroupComputeSource(extensions,
			"const uint cluster = 8u;\nvoid main() { o.v = subgroupClusteredAdd(gl_GlobalInvocationID.x, cluster); }\n"), ir.StageCompute)
		if op := firstSubgroupCollective(t, mod); op.ClusterSize != 8 {
			t.Fatalf("cluster size is %d, want 8", op.ClusterSize)
		}
	})

	// The cluster-size errors are glslang's, including the ones its semantic
	// layer reports: a runtime value, zero or negative, not a power of 2.
	cases := []struct{ name, body, want string }{
		{"non-constant", "o.v = subgroupClusteredAdd(gl_GlobalInvocationID.x, gl_GlobalInvocationID.x);",
			"'cluster size' : argument must be compile-time constant"},
		{"zero", "o.v = subgroupClusteredAdd(gl_GlobalInvocationID.x, 0u);",
			"'cluster size' : argument must be at least 1"},
		{"negative", "o.v = subgroupClusteredAdd(gl_GlobalInvocationID.x, -4);",
			"'cluster size' : argument must be at least 1"},
		{"not a power of 2", "o.v = subgroupClusteredAdd(gl_GlobalInvocationID.x, 3u);",
			"'cluster size' : argument must be a power of 2"},
		// A float is not the overload's uint parameter: glslang reports no
		// matching overload, which the frontend spells as the unknown function.
		{"float cluster size", "o.v = subgroupClusteredAdd(gl_GlobalInvocationID.x, 4.0);",
			"Unknown function 'subgroupClusteredAdd'"},
		// The arithmetic four take no bool value.
		{"bool value of add", "o.v = subgroupClusteredAdd(gl_GlobalInvocationID.x > 3u, 4u) ? 1u : 0u;",
			"Unknown function 'subgroupClusteredAdd'"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parseErr(t, subgroupComputeSource(extensions, "void main() { "+tc.body+" }\n"),
				Options{Stage: ir.StageCompute}, tc.want)
		})
	}
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

// TestSubgroupVectorOperand: GLSL takes a vector operand for the collective
// families and the IR carries it — the result is the argument's own vector type
// (SPIR-V's subgroup operation on a vector is per component; the MSL writer
// expands such an operand into one scalar simd call per component, and a
// vector row runs through the snapshot matrix, P2-GLSLIN-028).
func TestSubgroupVectorOperand(t *testing.T) {
	const extensions = "#extension GL_KHR_shader_subgroup_arithmetic : require\n"
	cases := []struct {
		name   string
		call   string
		size   ir.VectorSize
		scalar ir.ScalarKind
	}{
		{"float vec4 reduce", "subgroupAdd(vec4(gl_GlobalInvocationID.x))", ir.Vec4, ir.ScalarFloat},
		{"uint uvec2 scan", "subgroupInclusiveAdd(uvec2(gl_GlobalInvocationID.xy))", ir.Vec2, ir.ScalarUint},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := "void main() { vec4 sink = vec4(" + tc.call + "); o.f = sink.x; }\n"
			if tc.scalar == ir.ScalarUint {
				body = "void main() { uvec2 sink = " + tc.call + "; o.f = float(sink.x); }\n"
			}
			mod := parseSubgroup(t, subgroupComputeSource(extensions, body), ir.StageCompute)
			op := firstSubgroupCollective(t, mod)
			result := mainFunction(t, mod).Expressions[op.Result].Kind.(ir.ExprSubgroupOperationResult)
			vector, isVector := mod.Types[result.Type].Inner.(ir.VectorType)
			if !isVector || vector.Size != tc.size || vector.Scalar.Kind != tc.scalar {
				t.Fatalf("result type is %v, want a %d-vector of %v", mod.Types[result.Type].Inner, tc.size, tc.scalar)
			}
		})
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
		{"ballot bit without extension", "", "uvec4 m = uvec4(0u); o.v = subgroupBallotBitCount(m);",
			"'subgroupBallotBitCount' : required extension not requested: GL_KHR_shader_subgroup_ballot"},
		{"inverse ballot needs the ballot extension",
			"#extension GL_KHR_shader_subgroup_basic : require\n",
			"uvec4 m = uvec4(0u); o.v = subgroupInverseBallot(m) ? 1u : 0u;",
			"'subgroupInverseBallot' : required extension not requested: GL_KHR_shader_subgroup_ballot"},
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
		"#extension GL_KHR_shader_subgroup_arithmetic : require\n"
	cases := []struct {
		name    string
		use     string
		refusal string
	}{
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

// TestSubgroupBuiltinVariables: the four scalar variables of
// GL_KHR_shader_subgroup_basic (GLSL 4.60 §8.18) are wired through the entry
// point as glslang compiles them — Input variables decorated SubgroupSize,
// SubgroupLocalInvocationId, SubgroupId and NumSubgroups (probed 2026-11-16) —
// and each is a private u32 global bound as an entry argument.
func TestSubgroupBuiltinVariables(t *testing.T) {
	mod := parseSubgroup(t, subgroupComputeSource(
		"#extension GL_KHR_shader_subgroup_basic : require\n",
		"void main() { o.v = gl_SubgroupSize + gl_SubgroupInvocationID + gl_SubgroupID + gl_NumSubgroups; }\n"),
		ir.StageCompute)

	want := map[string]ir.BuiltinValue{
		"gl_SubgroupSize":         ir.BuiltinSubgroupSize,
		"gl_SubgroupInvocationID": ir.BuiltinSubgroupInvocationID,
		"gl_SubgroupID":           ir.BuiltinSubgroupID,
		"gl_NumSubgroups":         ir.BuiltinNumSubgroups,
	}
	bound := map[ir.BuiltinValue]bool{}
	for _, arg := range mod.EntryPoints[0].Function.Arguments {
		if arg.Binding == nil {
			continue
		}
		if b, ok := (*arg.Binding).(ir.BuiltinBinding); ok {
			bound[b.Builtin] = true
		}
	}
	for name, value := range want {
		g := findGlobal(t, mod, name)
		if g.Space != ir.SpacePrivate {
			t.Errorf("%s: space = %v, want private", name, g.Space)
		}
		scalar, ok := mod.Types[g.Type].Inner.(ir.ScalarType)
		if !ok || scalar.Kind != ir.ScalarUint || scalar.Width != 4 {
			t.Errorf("%s: type = %v, want uint", name, mod.Types[g.Type].Inner)
		}
		if !bound[value] {
			t.Errorf("%s is not bound as an entry argument", name)
		}
	}
}

// TestSubgroupMaskBuiltinVariables: the five ballot-mask variables of
// GL_KHR_shader_subgroup_ballot (§3.8) lower to globals of type vec4<u32>
// bound as entry arguments, as glslang compiles them (Input variables
// decorated SubgroupEqMask .. SubgroupLtMask; card P2-IR-041, ADR-014 D5).
func TestSubgroupMaskBuiltinVariables(t *testing.T) {
	mod := parseSubgroup(t, subgroupComputeSource(
		"#extension GL_KHR_shader_subgroup_ballot : require\n",
		"void main() { o.v = gl_SubgroupEqMask.x + gl_SubgroupGeMask.x + gl_SubgroupGtMask.x"+
			" + gl_SubgroupLeMask.x + gl_SubgroupLtMask.x; }\n"),
		ir.StageCompute)

	want := map[string]ir.BuiltinValue{
		"gl_SubgroupEqMask": ir.BuiltinSubgroupEqMask,
		"gl_SubgroupGeMask": ir.BuiltinSubgroupGeMask,
		"gl_SubgroupGtMask": ir.BuiltinSubgroupGtMask,
		"gl_SubgroupLeMask": ir.BuiltinSubgroupLeMask,
		"gl_SubgroupLtMask": ir.BuiltinSubgroupLtMask,
	}
	bound := map[ir.BuiltinValue]bool{}
	for _, arg := range mod.EntryPoints[0].Function.Arguments {
		if arg.Binding == nil {
			continue
		}
		if b, ok := (*arg.Binding).(ir.BuiltinBinding); ok {
			bound[b.Builtin] = true
		}
	}
	for name, value := range want {
		var found bool
		for _, g := range mod.GlobalVariables {
			if g.Name != name {
				continue
			}
			found = true
			vec, ok := mod.Types[g.Type].Inner.(ir.VectorType)
			if !ok || vec.Size != ir.Vec4 || vec.Scalar.Kind != ir.ScalarUint || vec.Scalar.Width != 4 {
				t.Errorf("%s: type = %v, want vec4<u32>", name, mod.Types[g.Type].Inner)
			}
		}
		if !found {
			t.Errorf("%s has no global", name)
		}
		if !bound[value] {
			t.Errorf("%s is not bound as an entry argument", name)
		}
	}
}

// TestSubgroupBuiltinVariableAvailability: the variables follow the availability
// rules of the subgroup builtins — the family's extension is required (glslang:
// "'gl_SubgroupSize' : required extension not requested:
// GL_KHR_shader_subgroup_basic", probed 2026-11-16; the ballot-mask variables
// need the ballot family's).
func TestSubgroupBuiltinVariableAvailability(t *testing.T) {
	t.Run("the basic extension is required", func(t *testing.T) {
		parseErr(t, subgroupComputeSource("", "void main() { o.v = gl_SubgroupSize; }\n"),
			Options{Stage: ir.StageCompute},
			"'gl_SubgroupSize' : required extension not requested: GL_KHR_shader_subgroup_basic")
	})
	t.Run("the ballot extension is required for a mask", func(t *testing.T) {
		parseErr(t, subgroupComputeSource("#extension GL_KHR_shader_subgroup_basic : require\n",
			"void main() { uvec4 m = gl_SubgroupEqMask; o.v = m.x; }\n"),
			Options{Stage: ir.StageCompute},
			"'gl_SubgroupEqMask' : required extension not requested: GL_KHR_shader_subgroup_ballot")
	})
}
