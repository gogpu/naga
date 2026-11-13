package front

import (
	"errors"
	"reflect"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// subgroupModuleGroups assembles a compute module for the subgroup
// instructions. Ids: 1 the function, 10 void, 11 fn void(), 12 bool, 13 u32,
// 14 vec4<u32>, 15 the u32 constant 4 (the subgroup scope), 16 the u32
// constant 1, 17 the constant true, 18 the u32 constant 0.
func subgroupModuleGroups(body ...[]uint32) [][]uint32 {
	return subgroupVariantGroups(nil, body...)
}

// subgroupVariantGroups is subgroupModuleGroups with extra groups before the
// function.
func subgroupVariantGroups(extra [][]uint32, body ...[]uint32) [][]uint32 {
	groups := [][]uint32{
		headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityGroupNonUniform)),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityGroupNonUniformBallot)),
		instWords(spvenums.OpMemoryModel, 0, 1),
		entryPointOp(spvenums.ExecutionModelGLCompute),
		instWords(spvenums.OpExecutionMode, 1, uint32(spvenums.ExecutionModeLocalSize), 1, 1, 1),
		instWords(spvenums.OpTypeVoid, 10),
		instWords(spvenums.OpTypeFunction, 11, 10),
		instWords(spvenums.OpTypeBool, 12),
		instWords(spvenums.OpTypeInt, 13, 32, 0),
		instWords(spvenums.OpTypeVector, 14, 13, 4),
		instWords(spvenums.OpConstant, 13, 15, uint32(spvenums.ScopeSubgroup)),
		instWords(spvenums.OpConstant, 13, 16, 1),
		instWords(spvenums.OpConstantTrue, 12, 17),
		instWords(spvenums.OpConstant, 13, 18, 0),
	}
	groups = append(groups, extra...)
	groups = append(groups,
		voidFunctionBody(),
		instWords(spvenums.OpLabel, 2),
	)
	groups = append(groups, body...)
	return append(groups,
		instWords(spvenums.OpReturn),
		instWords(spvenums.OpFunctionEnd),
	)
}

// TestSubgroupBallot: a ballot with the constant true predicate has none in
// the IR (every invocation votes yes), and any other predicate is kept.
func TestSubgroupBallot(t *testing.T) {
	t.Run("true predicate", func(t *testing.T) {
		module, _, err := parseWords(t, Options{}, subgroupModuleGroups(
			instWords(spvenums.OpGroupNonUniformBallot, 14, 20, 15, 17),
		)...)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		ballot, ok := findStatement(&module.Functions[0], func(k ir.StatementKind) bool {
			_, ok := k.(ir.StmtSubgroupBallot)
			return ok
		})
		if !ok {
			t.Fatal("no SubgroupBallot statement")
		}
		if ballot.(ir.StmtSubgroupBallot).Predicate != nil {
			t.Error("the ballot kept the constant true predicate")
		}
	})

	t.Run("dynamic predicate", func(t *testing.T) {
		module, _, err := parseWords(t, Options{}, subgroupModuleGroups(
			instWords(spvenums.OpGroupNonUniformBallot, 14, 20, 15, 16),
		)...)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		ballot, ok := findStatement(&module.Functions[0], func(k ir.StatementKind) bool {
			_, ok := k.(ir.StmtSubgroupBallot)
			return ok
		})
		if !ok {
			t.Fatal("no SubgroupBallot statement")
		}
		if ballot.(ir.StmtSubgroupBallot).Predicate == nil {
			t.Error("the ballot dropped a non-constant predicate")
		}
	})
}

// TestSubgroupCollective: OpGroupNonUniformIAdd with the reduce group
// operation, and a scan.
func TestSubgroupCollective(t *testing.T) {
	for _, tc := range []struct {
		name       string
		groupOp    uint32
		collective ir.CollectiveOperation
	}{
		{"reduce", uint32(spvenums.GroupOperationReduce), ir.CollectiveReduce},
		{"inclusive scan", uint32(spvenums.GroupOperationInclusiveScan), ir.CollectiveInclusiveScan},
		{"exclusive scan", uint32(spvenums.GroupOperationExclusiveScan), ir.CollectiveExclusiveScan},
	} {
		t.Run(tc.name, func(t *testing.T) {
			module, _, err := parseWords(t, Options{}, subgroupModuleGroups(
				instWords(spvenums.OpGroupNonUniformIAdd, 13, 20, 15, tc.groupOp, 16),
			)...)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			statement, ok := findStatement(&module.Functions[0], func(k ir.StatementKind) bool {
				_, ok := k.(ir.StmtSubgroupCollectiveOperation)
				return ok
			})
			if !ok {
				t.Fatal("no SubgroupCollectiveOperation statement")
			}
			operation := statement.(ir.StmtSubgroupCollectiveOperation)
			if operation.Op != ir.SubgroupOperationAdd {
				t.Errorf("operation = %v, want Add", operation.Op)
			}
			if operation.CollectiveOp != tc.collective {
				t.Errorf("collective operation = %v, want %v", operation.CollectiveOp, tc.collective)
			}
		})
	}
}

// TestSubgroupQuadSwap: the direction operand selects the quad direction.
func TestSubgroupQuadSwap(t *testing.T) {
	module, _, err := parseWords(t, Options{}, subgroupModuleGroups(
		instWords(spvenums.OpGroupNonUniformQuadSwap, 14, 20, 15, 16, 18),
	)...)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	statement, ok := findStatement(&module.Functions[0], func(k ir.StatementKind) bool {
		_, ok := k.(ir.StmtSubgroupGather)
		return ok
	})
	if !ok {
		t.Fatal("no SubgroupGather statement")
	}
	swap, ok := statement.(ir.StmtSubgroupGather).Mode.(ir.GatherQuadSwap)
	if !ok || swap.Direction != ir.QuadDirectionX {
		t.Errorf("mode = %#v, want a quad swap along X", statement.(ir.StmtSubgroupGather).Mode)
	}
}

// TestSubgroupScope: an execution scope that is not subgroup is refused.
func TestSubgroupScope(t *testing.T) {
	_, _, err := parseWords(t, Options{}, subgroupModuleGroups(
		instWords(spvenums.OpGroupNonUniformIAdd, 13, 20, 16, 0, 16),
	)...)
	if err == nil {
		t.Fatal("Parse accepted a workgroup scope for a subgroup operation")
	}
	var parseErr *Error
	if !errors.As(err, &parseErr) || parseErr.Message != errInvalidBarrierScope(16).Message {
		t.Fatalf("error = %v, want %q", err, errInvalidBarrierScope(16).Message)
	}
}

// TestSubgroupScopeNotConstant: a scope operand that is no constant is the
// id error the lookup reports, not a wrong-scope error.
func TestSubgroupScopeNotConstant(t *testing.T) {
	_, _, err := parseWords(t, Options{}, subgroupModuleGroups(
		instWords(spvenums.OpGroupNonUniformIAdd, 13, 20, 99, 0, 16),
	)...)
	if err == nil {
		t.Fatal("Parse accepted a scope id that is no constant")
	}
	if got := requireError(t, err).Message; got != errInvalidID(99).Message {
		t.Errorf("message = %q, want %q", got, errInvalidID(99).Message)
	}
}

// TestSubgroupGatherModes: the broadcast and shuffle family and the quad
// operations select their IR gather mode by opcode, and a quad swap by the
// direction operand.
func TestSubgroupGatherModes(t *testing.T) {
	// The u32 constant 2 is the diagonal quad direction.
	extra := [][]uint32{instWords(spvenums.OpConstant, 13, 19, 2)}
	parse := func(t *testing.T, body ...[]uint32) ir.GatherMode {
		t.Helper()
		module, _, err := parseWords(t, Options{}, subgroupVariantGroups(extra, body...)...)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		statement, ok := findStatement(&module.Functions[0], func(k ir.StatementKind) bool {
			_, ok := k.(ir.StmtSubgroupGather)
			return ok
		})
		if !ok {
			t.Fatal("no SubgroupGather statement")
		}
		return statement.(ir.StmtSubgroupGather).Mode
	}

	for _, tc := range []struct {
		name string
		op   spvenums.Op
		want any // the concrete ir.GatherMode type
	}{
		{"broadcast first", spvenums.OpGroupNonUniformBroadcastFirst, ir.GatherBroadcastFirst{}},
		{"broadcast", spvenums.OpGroupNonUniformBroadcast, ir.GatherBroadcast{}},
		{"shuffle", spvenums.OpGroupNonUniformShuffle, ir.GatherShuffle{}},
		{"shuffle down", spvenums.OpGroupNonUniformShuffleDown, ir.GatherShuffleDown{}},
		{"shuffle up", spvenums.OpGroupNonUniformShuffleUp, ir.GatherShuffleUp{}},
		{"shuffle xor", spvenums.OpGroupNonUniformShuffleXor, ir.GatherShuffleXor{}},
		{"quad broadcast", spvenums.OpGroupNonUniformQuadBroadcast, ir.GatherQuadBroadcast{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			operands := []uint32{14, 20, 15, 16}
			if tc.op != spvenums.OpGroupNonUniformBroadcastFirst {
				// The shuffle family takes the index as a fifth operand.
				operands = append(operands, 18)
			}
			mode := parse(t, instWords(tc.op, operands...))
			if reflect.TypeOf(mode) != reflect.TypeOf(tc.want) {
				t.Errorf("mode = %T, want %T", mode, tc.want)
			}
		})
	}

	for _, tc := range []struct {
		name      string
		direction uint32
		want      ir.QuadDirection
	}{
		{"quad swap x", 18, ir.QuadDirectionX},
		{"quad swap y", 16, ir.QuadDirectionY},
		{"quad swap diagonal", 19, ir.QuadDirectionDiagonal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mode := parse(t, instWords(spvenums.OpGroupNonUniformQuadSwap, 14, 20, 15, 16, tc.direction))
			swap, ok := mode.(ir.GatherQuadSwap)
			if !ok || swap.Direction != tc.want {
				t.Errorf("mode = %#v, want a quad swap along %v", mode, tc.want)
			}
		})
	}

	t.Run("quad swap direction refused", func(t *testing.T) {
		// The u32 constant 3 is no quad direction.
		_, _, err := parseWords(t, Options{}, subgroupVariantGroups(
			[][]uint32{instWords(spvenums.OpConstant, 13, 19, 3)},
			instWords(spvenums.OpGroupNonUniformQuadSwap, 14, 20, 15, 16, 19),
		)...)
		if err == nil {
			t.Fatal("Parse accepted a quad swap direction of 3")
		}
		if got := requireError(t, err).Message; got != errInvalidOperand.Message {
			t.Errorf("message = %q, want %q", got, errInvalidOperand.Message)
		}
	})
}

// TestSubgroupAllAny: OpGroupNonUniformAll and OpGroupNonUniformAny have no
// group operation operand; they are the all and any reductions.
func TestSubgroupAllAny(t *testing.T) {
	for _, tc := range []struct {
		name string
		op   spvenums.Op
		want ir.SubgroupOperation
	}{
		{"all", spvenums.OpGroupNonUniformAll, ir.SubgroupOperationAll},
		{"any", spvenums.OpGroupNonUniformAny, ir.SubgroupOperationAny},
	} {
		t.Run(tc.name, func(t *testing.T) {
			module, _, err := parseWords(t, Options{}, subgroupModuleGroups(
				instWords(tc.op, 12, 20, 15, 17),
			)...)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			statement, ok := findStatement(&module.Functions[0], func(k ir.StatementKind) bool {
				_, ok := k.(ir.StmtSubgroupCollectiveOperation)
				return ok
			})
			if !ok {
				t.Fatal("no SubgroupCollectiveOperation statement")
			}
			operation := statement.(ir.StmtSubgroupCollectiveOperation)
			if operation.Op != tc.want {
				t.Errorf("operation = %v, want %v", operation.Op, tc.want)
			}
			if operation.CollectiveOp != ir.CollectiveReduce {
				t.Errorf("collective operation = %v, want reduce", operation.CollectiveOp)
			}
		})
	}
}

// TestSubgroupUnsupportedGroupOperation: a group operation the IR has no
// collective for is refused.
func TestSubgroupUnsupportedGroupOperation(t *testing.T) {
	// 5 is ClusteredReduce, which the IR cannot express.
	_, _, err := parseWords(t, Options{}, subgroupModuleGroups(
		instWords(spvenums.OpGroupNonUniformIAdd, 13, 20, 15, 5, 16),
	)...)
	if err == nil {
		t.Fatal("Parse accepted a clustered reduce")
	}
	if got := requireError(t, err).Message; got != errUnsupportedGroupOperation(5).Message {
		t.Errorf("message = %q, want %q", got, errUnsupportedGroupOperation(5).Message)
	}
}
