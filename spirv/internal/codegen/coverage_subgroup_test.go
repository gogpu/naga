package codegen

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// ---------------------------------------------------------------------------
// Subgroup operations — all 0% coverage.
// These IR patterns are not reachable from the WGSL parser so we construct IR
// modules directly and compile them through the SPIR-V backend.
// ---------------------------------------------------------------------------

// buildSubgroupModule constructs a minimal compute module with a single
// subgroup statement emitted inside the entry point body.
// argTypeHandle indexes into module.Types and must resolve to a valid type.
func buildSubgroupModule(
	types []ir.Type,
	expressions []ir.Expression,
	body []ir.Statement,
) *ir.Module {
	return &ir.Module{
		Types: types,
		EntryPoints: []ir.EntryPoint{
			{
				Name:      "main",
				Stage:     ir.StageCompute,
				Workgroup: [3]uint32{1, 1, 1},
				Function: ir.Function{
					Expressions: expressions,
					Body:        body,
				},
			},
		},
	}
}

// TestSubgroupBallotNoPredicate covers emitSubgroupBallot with no predicate
// (default OpConstantTrue path) and emitSubgroupResultRef.
func TestSubgroupBallotNoPredicate(t *testing.T) {
	u32Type := ir.TypeHandle(0)
	_ = u32Type

	mod := buildSubgroupModule(
		[]ir.Type{
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		},
		[]ir.Expression{
			// expr 0: SubgroupBallotResult (vec4<u32>)
			{Kind: ir.ExprSubgroupBallotResult{}},
		},
		[]ir.Statement{
			{Kind: ir.StmtSubgroupBallot{
				Result:    0,
				Predicate: nil, // no predicate — exercises OpConstantTrue path
			}},
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
		},
	)

	backend := NewBackend(DefaultOptions())
	spvBytes, err := backend.Compile(mod)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	assertValidSPIRV(t, spvBytes)
	instrs := decodeSPIRVInstructions(spvBytes)

	if !hasOpcodeInInstrs(instrs, OpGroupNonUniformBallot) {
		t.Error("expected OpGroupNonUniformBallot")
	}
	if !hasCapability(instrs, CapabilityGroupNonUniformBallot) {
		t.Error("expected CapabilityGroupNonUniformBallot")
	}
	if !hasOpcodeInInstrs(instrs, OpConstantTrue) {
		t.Error("expected OpConstantTrue for default predicate")
	}
}

// TestSubgroupBallotWithPredicate covers emitSubgroupBallot with an explicit
// boolean predicate expression.
func TestSubgroupBallotWithPredicate(t *testing.T) {
	predExpr := ir.ExpressionHandle(0)

	mod := buildSubgroupModule(
		[]ir.Type{
			{Name: "bool", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
		},
		[]ir.Expression{
			// expr 0: literal true
			{Kind: ir.Literal{Value: ir.LiteralBool(true)}},
			// expr 1: SubgroupBallotResult
			{Kind: ir.ExprSubgroupBallotResult{}},
		},
		[]ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
			{Kind: ir.StmtSubgroupBallot{
				Result:    1,
				Predicate: &predExpr,
			}},
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 2}}},
		},
	)

	backend := NewBackend(DefaultOptions())
	spvBytes, err := backend.Compile(mod)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	assertValidSPIRV(t, spvBytes)
	instrs := decodeSPIRVInstructions(spvBytes)

	if !hasOpcodeInInstrs(instrs, OpGroupNonUniformBallot) {
		t.Error("expected OpGroupNonUniformBallot")
	}
}

// TestSubgroupCollectiveAdd covers emitSubgroupCollectiveOperation with
// SubgroupOperationAdd + CollectiveReduce on u32 (integer path).
func TestSubgroupCollectiveAdd(t *testing.T) {
	u32Handle := ir.TypeHandle(0)

	mod := buildSubgroupModule(
		[]ir.Type{
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		},
		[]ir.Expression{
			// expr 0: argument = 1u
			{Kind: ir.Literal{Value: ir.LiteralU32(1)}},
			// expr 1: subgroup operation result
			{Kind: ir.ExprSubgroupOperationResult{Type: u32Handle}},
		},
		[]ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
			{Kind: ir.StmtSubgroupCollectiveOperation{
				Op:           ir.SubgroupOperationAdd,
				CollectiveOp: ir.CollectiveReduce,
				Argument:     0,
				Result:       1,
			}},
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 2}}},
		},
	)

	backend := NewBackend(DefaultOptions())
	spvBytes, err := backend.Compile(mod)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	assertValidSPIRV(t, spvBytes)
	instrs := decodeSPIRVInstructions(spvBytes)

	if !hasOpcodeInInstrs(instrs, OpGroupNonUniformIAdd) {
		t.Error("expected OpGroupNonUniformIAdd for integer add reduce")
	}
	if !hasCapability(instrs, CapabilityGroupNonUniformArithmetic) {
		t.Error("expected CapabilityGroupNonUniformArithmetic")
	}
}

// TestSubgroupCollectiveAllOps covers multiple subgroup operations to exercise
// the opcode selection switch and resolveSubgroupScalarKind.
func TestSubgroupCollectiveAllOps(t *testing.T) {
	tests := []struct {
		name    string
		op      ir.SubgroupOperation
		colOp   ir.CollectiveOperation
		scalar  ir.ScalarType
		litVal  ir.LiteralValue
		wantOp  OpCode
		wantCap Capability
		isVote  bool
		isBool  bool
	}{
		{
			name: "All/vote", op: ir.SubgroupOperationAll, colOp: ir.CollectiveReduce,
			scalar: ir.ScalarType{Kind: ir.ScalarBool, Width: 1},
			litVal: ir.LiteralBool(true), wantOp: OpGroupNonUniformAll,
			wantCap: CapabilityGroupNonUniformVote, isVote: true, isBool: true,
		},
		{
			name: "Any/vote", op: ir.SubgroupOperationAny, colOp: ir.CollectiveReduce,
			scalar: ir.ScalarType{Kind: ir.ScalarBool, Width: 1},
			litVal: ir.LiteralBool(false), wantOp: OpGroupNonUniformAny,
			wantCap: CapabilityGroupNonUniformVote, isVote: true, isBool: true,
		},
		{
			name: "Add/float", op: ir.SubgroupOperationAdd, colOp: ir.CollectiveInclusiveScan,
			scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
			litVal: ir.LiteralF32(1.0), wantOp: OpGroupNonUniformFAdd,
			wantCap: CapabilityGroupNonUniformArithmetic,
		},
		{
			name: "Mul/float", op: ir.SubgroupOperationMul, colOp: ir.CollectiveExclusiveScan,
			scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
			litVal: ir.LiteralF32(2.0), wantOp: OpGroupNonUniformFMul,
			wantCap: CapabilityGroupNonUniformArithmetic,
		},
		{
			name: "Mul/int", op: ir.SubgroupOperationMul, colOp: ir.CollectiveReduce,
			scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4},
			litVal: ir.LiteralI32(3), wantOp: OpGroupNonUniformIMul,
			wantCap: CapabilityGroupNonUniformArithmetic,
		},
		{
			name: "Min/float", op: ir.SubgroupOperationMin, colOp: ir.CollectiveReduce,
			scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
			litVal: ir.LiteralF32(0.0), wantOp: OpGroupNonUniformFMin,
			wantCap: CapabilityGroupNonUniformArithmetic,
		},
		{
			name: "Min/uint", op: ir.SubgroupOperationMin, colOp: ir.CollectiveReduce,
			scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4},
			litVal: ir.LiteralU32(0), wantOp: OpGroupNonUniformUMin,
			wantCap: CapabilityGroupNonUniformArithmetic,
		},
		{
			name: "Min/sint", op: ir.SubgroupOperationMin, colOp: ir.CollectiveReduce,
			scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4},
			litVal: ir.LiteralI32(0), wantOp: OpGroupNonUniformSMin,
			wantCap: CapabilityGroupNonUniformArithmetic,
		},
		{
			name: "Max/float", op: ir.SubgroupOperationMax, colOp: ir.CollectiveReduce,
			scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
			litVal: ir.LiteralF32(0.0), wantOp: OpGroupNonUniformFMax,
			wantCap: CapabilityGroupNonUniformArithmetic,
		},
		{
			name: "Max/uint", op: ir.SubgroupOperationMax, colOp: ir.CollectiveReduce,
			scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4},
			litVal: ir.LiteralU32(0), wantOp: OpGroupNonUniformUMax,
			wantCap: CapabilityGroupNonUniformArithmetic,
		},
		{
			name: "Max/sint", op: ir.SubgroupOperationMax, colOp: ir.CollectiveReduce,
			scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4},
			litVal: ir.LiteralI32(0), wantOp: OpGroupNonUniformSMax,
			wantCap: CapabilityGroupNonUniformArithmetic,
		},
		{
			name: "And/bool", op: ir.SubgroupOperationAnd, colOp: ir.CollectiveReduce,
			scalar: ir.ScalarType{Kind: ir.ScalarBool, Width: 1},
			litVal: ir.LiteralBool(true), wantOp: OpGroupNonUniformLogicalAnd,
			wantCap: CapabilityGroupNonUniformArithmetic, isBool: true,
		},
		{
			name: "And/uint", op: ir.SubgroupOperationAnd, colOp: ir.CollectiveReduce,
			scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4},
			litVal: ir.LiteralU32(0xFF), wantOp: OpGroupNonUniformBitwiseAnd,
			wantCap: CapabilityGroupNonUniformArithmetic,
		},
		{
			name: "Or/bool", op: ir.SubgroupOperationOr, colOp: ir.CollectiveReduce,
			scalar: ir.ScalarType{Kind: ir.ScalarBool, Width: 1},
			litVal: ir.LiteralBool(false), wantOp: OpGroupNonUniformLogicalOr,
			wantCap: CapabilityGroupNonUniformArithmetic, isBool: true,
		},
		{
			name: "Or/uint", op: ir.SubgroupOperationOr, colOp: ir.CollectiveReduce,
			scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4},
			litVal: ir.LiteralU32(0x0F), wantOp: OpGroupNonUniformBitwiseOr,
			wantCap: CapabilityGroupNonUniformArithmetic,
		},
		{
			name: "Xor/bool", op: ir.SubgroupOperationXor, colOp: ir.CollectiveReduce,
			scalar: ir.ScalarType{Kind: ir.ScalarBool, Width: 1},
			litVal: ir.LiteralBool(true), wantOp: OpGroupNonUniformLogicalXor,
			wantCap: CapabilityGroupNonUniformArithmetic, isBool: true,
		},
		{
			name: "Xor/uint", op: ir.SubgroupOperationXor, colOp: ir.CollectiveReduce,
			scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4},
			litVal: ir.LiteralU32(0xAA), wantOp: OpGroupNonUniformBitwiseXor,
			wantCap: CapabilityGroupNonUniformArithmetic,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typeHandle := ir.TypeHandle(0)

			mod := buildSubgroupModule(
				[]ir.Type{
					{Name: "t", Inner: tt.scalar},
				},
				[]ir.Expression{
					{Kind: ir.Literal{Value: tt.litVal}},
					{Kind: ir.ExprSubgroupOperationResult{Type: typeHandle}},
				},
				[]ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
					{Kind: ir.StmtSubgroupCollectiveOperation{
						Op:           tt.op,
						CollectiveOp: tt.colOp,
						Argument:     0,
						Result:       1,
					}},
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 2}}},
				},
			)

			backend := NewBackend(DefaultOptions())
			spvBytes, err := backend.Compile(mod)
			if err != nil {
				t.Fatalf("Compile failed: %v", err)
			}
			assertValidSPIRV(t, spvBytes)
			instrs := decodeSPIRVInstructions(spvBytes)

			if !hasOpcodeInInstrs(instrs, tt.wantOp) {
				t.Errorf("expected opcode %v", tt.wantOp)
			}
			if !hasCapability(instrs, tt.wantCap) {
				t.Errorf("expected capability %v", tt.wantCap)
			}
		})
	}
}

// TestSubgroupGatherAllModes covers all 8 GatherMode variants in emitSubgroupGather.
func TestSubgroupGatherAllModes(t *testing.T) {
	tests := []struct {
		name    string
		mode    ir.GatherMode
		wantOp  OpCode
		wantCap Capability
		// extra expressions needed for the mode (e.g. index/delta/mask)
		extraExprs []ir.Expression
	}{
		{
			name:    "BroadcastFirst",
			mode:    ir.GatherBroadcastFirst{},
			wantOp:  OpGroupNonUniformBroadcastFirst,
			wantCap: CapabilityGroupNonUniformBallot,
		},
		{
			name:    "Broadcast",
			mode:    ir.GatherBroadcast{Index: 2},
			wantOp:  OpGroupNonUniformBroadcast,
			wantCap: CapabilityGroupNonUniformBallot,
			extraExprs: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralU32(0)}}, // expr 2: index
			},
		},
		{
			name:    "Shuffle",
			mode:    ir.GatherShuffle{Index: 2},
			wantOp:  OpGroupNonUniformShuffle,
			wantCap: CapabilityGroupNonUniformShuffle,
			extraExprs: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralU32(1)}},
			},
		},
		{
			name:    "ShuffleDown",
			mode:    ir.GatherShuffleDown{Delta: 2},
			wantOp:  OpGroupNonUniformShuffleDown,
			wantCap: CapabilityGroupNonUniformShuffleRel,
			extraExprs: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralU32(1)}},
			},
		},
		{
			name:    "ShuffleUp",
			mode:    ir.GatherShuffleUp{Delta: 2},
			wantOp:  OpGroupNonUniformShuffleUp,
			wantCap: CapabilityGroupNonUniformShuffleRel,
			extraExprs: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralU32(1)}},
			},
		},
		{
			name:    "ShuffleXor",
			mode:    ir.GatherShuffleXor{Mask: 2},
			wantOp:  OpGroupNonUniformShuffleXor,
			wantCap: CapabilityGroupNonUniformShuffle,
			extraExprs: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralU32(3)}},
			},
		},
		{
			name:    "QuadBroadcast",
			mode:    ir.GatherQuadBroadcast{Index: 2},
			wantOp:  OpGroupNonUniformQuadBroadcast,
			wantCap: CapabilityGroupNonUniformQuad,
			extraExprs: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralU32(2)}},
			},
		},
		{
			name:    "QuadSwap/X",
			mode:    ir.GatherQuadSwap{Direction: ir.QuadDirectionX},
			wantOp:  OpGroupNonUniformQuadSwap,
			wantCap: CapabilityGroupNonUniformQuad,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typeHandle := ir.TypeHandle(0)

			// Base expressions: [0] = argument literal, [1] = subgroup result
			exprs := []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralU32(42)}},
				{Kind: ir.ExprSubgroupOperationResult{Type: typeHandle}},
			}
			exprs = append(exprs, tt.extraExprs...)

			// Build statements: emit the argument only (not the result expr),
			// then the gather statement stores the result in exprIDs.
			var stmts []ir.Statement
			// Emit arg literal + any extra expressions (indexes, deltas, masks)
			if len(tt.extraExprs) > 0 {
				stmts = append(stmts, ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{
					Start: 0, End: 1, // emit arg literal
				}}})
				stmts = append(stmts, ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{
					Start: 2, End: ir.ExpressionHandle(2 + len(tt.extraExprs)), // emit extra exprs
				}}})
			} else {
				stmts = append(stmts, ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{
					Start: 0, End: 1,
				}}})
			}
			stmts = append(stmts, ir.Statement{Kind: ir.StmtSubgroupGather{
				Mode:     tt.mode,
				Argument: 0,
				Result:   1,
			}})
			// Emit the result expression AFTER the gather has stored it
			stmts = append(stmts, ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{
				Start: 1, End: 2,
			}}})

			mod := buildSubgroupModule(
				[]ir.Type{
					{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
				},
				exprs,
				stmts,
			)

			backend := NewBackend(DefaultOptions())
			spvBytes, err := backend.Compile(mod)
			if err != nil {
				t.Fatalf("Compile failed: %v", err)
			}
			assertValidSPIRV(t, spvBytes)
			instrs := decodeSPIRVInstructions(spvBytes)

			if !hasOpcodeInInstrs(instrs, tt.wantOp) {
				t.Errorf("expected opcode %v for %s gather", tt.wantOp, tt.name)
			}
			if !hasCapability(instrs, tt.wantCap) {
				t.Errorf("expected capability %v for %s gather", tt.wantCap, tt.name)
			}
		})
	}
}

// TestSubgroupCollectiveWithVector covers resolveSubgroupScalarKind with
// vector argument type (exercises the VectorType branch).
func TestSubgroupCollectiveWithVector(t *testing.T) {
	vec4f32Handle := ir.TypeHandle(0)

	mod := buildSubgroupModule(
		[]ir.Type{
			{Name: "vec4f32", Inner: ir.VectorType{
				Size:   4,
				Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
			}},
		},
		[]ir.Expression{
			// expr 0: GlobalVariable pointing at nothing -- use a literal vec4 instead
			// Actually we need an argument expression. Use a ZeroValue.
			{Kind: ir.ExprZeroValue{Type: vec4f32Handle}},
			// expr 1: subgroup result
			{Kind: ir.ExprSubgroupOperationResult{Type: vec4f32Handle}},
		},
		[]ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
			{Kind: ir.StmtSubgroupCollectiveOperation{
				Op:           ir.SubgroupOperationAdd,
				CollectiveOp: ir.CollectiveReduce,
				Argument:     0,
				Result:       1,
			}},
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 2}}},
		},
	)

	backend := NewBackend(DefaultOptions())
	spvBytes, err := backend.Compile(mod)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	assertValidSPIRV(t, spvBytes)
	instrs := decodeSPIRVInstructions(spvBytes)

	// Float vector add → OpGroupNonUniformFAdd
	if !hasOpcodeInInstrs(instrs, OpGroupNonUniformFAdd) {
		t.Error("expected OpGroupNonUniformFAdd for float vector reduce")
	}
}
