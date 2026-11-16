package emit

// The subgroup forms ADR-014 added to the IR: elect and all-equal, emitted as
// the DXIL wave operations of docs/DXIL.rst (WaveIsFirstLane 110,
// WaveActiveAllEqual 115).

import (
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/ir"
)

// TestEmitSubgroupElect: StmtSubgroupElect emits dx.op.waveIsFirstLane, whose
// signature is i1(i32 opcode) — no value operand.
func TestEmitSubgroupElect(t *testing.T) {
	irMod := dxilSubgroupModule(
		[]ir.Type{{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}}},
		[]ir.Expression{{Kind: ir.ExprSubgroupOperationResult{Type: 0}}},
		[]ir.Statement{
			{Kind: ir.StmtSubgroupElect{Result: 0}},
			{Kind: ir.StmtReturn{}},
		},
	)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}
	if !hasDxOpFunc(mod, "dx.op.waveIsFirstLane") {
		t.Error("dx.op.waveIsFirstLane function not found")
	}
	if !hasDxOpCall(mod, "dx.op.waveIsFirstLane") {
		t.Error("no call to dx.op.waveIsFirstLane in main")
	}
}

// TestEmitSubgroupAllEqual: StmtSubgroupCollectiveOperation with AllEqual
// emits dx.op.waveActiveAllEqual.T(i32 115, T value) with the argument's
// numeric overload; bool operands are widened to i32 because the DXIL overload
// table has no i1.
func TestEmitSubgroupAllEqual(t *testing.T) {
	irMod := dxilAllEqualModule(ir.Type{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		ir.Literal{Value: ir.LiteralU32(1)})
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}
	if !hasDxOpFunc(mod, "dx.op.waveActiveAllEqual.i32") {
		t.Error("dx.op.waveActiveAllEqual.i32 function not found")
	}
	if !hasDxOpCall(mod, "dx.op.waveActiveAllEqual.i32") {
		t.Error("no call to dx.op.waveActiveAllEqual.i32 in main")
	}
}

// TestEmitSubgroupAllEqualBool: a bool operand is zero-extended to i32 and
// passed through the i32 overload.
func TestEmitSubgroupAllEqualBool(t *testing.T) {
	irMod := dxilAllEqualModule(ir.Type{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
		ir.Literal{Value: ir.LiteralBool(true)})
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}
	if !hasDxOpCall(mod, "dx.op.waveActiveAllEqual.i32") {
		t.Error("no call to dx.op.waveActiveAllEqual.i32 in main")
	}
}

// TestEmitSubgroupAllEqualVectorRefused: the emitter declares scalar overloads
// only, so a vector all-equal is refused instead of written with a mismatched
// declaration.
func TestEmitSubgroupAllEqualVectorRefused(t *testing.T) {
	irMod := dxilAllEqualModule(
		ir.Type{Name: "", Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
		ir.ExprSplat{Size: ir.Vec2, Value: 0})
	if _, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0}); err == nil {
		t.Fatal("a vector all-equal emitted, want a refusal")
	}
}

// dxilSubgroupModule builds a compute entry point with the given expressions
// and body.
func dxilSubgroupModule(types []ir.Type, expressions []ir.Expression, body []ir.Statement) *ir.Module {
	return &ir.Module{
		Types: types,
		EntryPoints: []ir.EntryPoint{{
			Name:      "main",
			Stage:     ir.StageCompute,
			Workgroup: [3]uint32{64, 1, 1},
			Function: ir.Function{
				Name:        "main",
				Expressions: expressions,
				Body:        body,
			},
		}},
	}
}

// dxilAllEqualModule builds the all-equal over one argument of the given type:
// the argument expression 0, the bool result expression 1.
func dxilAllEqualModule(argumentType ir.Type, argument ir.ExpressionKind) *ir.Module {
	switch a := argument.(type) {
	case ir.ExprSplat:
		// A vector argument: the uint literal is expression 0, the splat
		// expression 1, the result expression 2.
		a.Value = 0
		return dxilSubgroupModule(
			[]ir.Type{
				{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
				{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
				argumentType,
			},
			[]ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralU32(1)}},
				{Kind: a},
				{Kind: ir.ExprSubgroupOperationResult{Type: 1}},
			},
			[]ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
				{Kind: ir.StmtSubgroupCollectiveOperation{
					Op:           ir.SubgroupOperationAllEqual,
					CollectiveOp: ir.CollectiveReduce,
					Argument:     1,
					Result:       2,
				}},
				{Kind: ir.StmtReturn{}},
			},
		)
	default:
		return dxilSubgroupModule(
			[]ir.Type{
				argumentType,
				{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
			},
			[]ir.Expression{
				{Kind: argument},
				{Kind: ir.ExprSubgroupOperationResult{Type: 1}},
			},
			[]ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
				{Kind: ir.StmtSubgroupCollectiveOperation{
					Op:           ir.SubgroupOperationAllEqual,
					CollectiveOp: ir.CollectiveReduce,
					Argument:     0,
					Result:       1,
				}},
				{Kind: ir.StmtReturn{}},
			},
		)
	}
}

// hasDxOpFunc reports whether the module declares a dx.op function by name.
func hasDxOpFunc(mod *module.Module, name string) bool {
	for _, fn := range mod.Functions {
		if fn.Name == name {
			return true
		}
	}
	return false
}

// hasDxOpCall reports whether the main function calls a dx.op function by name.
func hasDxOpCall(mod *module.Module, name string) bool {
	mainFn := findMainFunc(mod)
	if mainFn == nil {
		return false
	}
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrCall && instr.CalledFunc != nil && instr.CalledFunc.Name == name {
				return true
			}
		}
	}
	return false
}
