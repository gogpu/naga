package front

import (
	"errors"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// subgroupModuleGroups assembles a compute module for the subgroup
// instructions. Ids: 1 the function, 10 void, 11 fn void(), 12 bool, 13 u32,
// 14 vec4<u32>, 15 the u32 constant 4 (the subgroup scope), 16 the u32
// constant 1, 17 the constant true, 18 the u32 constant 0.
func subgroupModuleGroups(body ...[]uint32) [][]uint32 {
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
		voidFunctionBody(),
		instWords(spvenums.OpLabel, 2),
	}
	for _, instruction := range body {
		groups = append(groups, instruction)
	}
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
