package front

import (
	"errors"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// atomicModuleGroups assembles a compute module over one storage buffer of
// struct { u32, u32 }: id 1 the function, 10 void, 11 fn void(), 12 the struct
// type, 13 its StorageBuffer pointer, 14 the u32 type, 15 its StorageBuffer
// pointer, 16 the zero constant, 17 the one constant, 18 the variable.
func atomicModuleGroups(body ...[]uint32) [][]uint32 {
	groups := [][]uint32{
		headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpMemoryModel, 0, 1),
		entryPointOp(spvenums.ExecutionModelGLCompute, 18),
		instWords(spvenums.OpExecutionMode, 1, uint32(spvenums.ExecutionModeLocalSize), 1, 1, 1),
		instWords(spvenums.OpDecorate, 12, uint32(spvenums.DecorationBlock)),
		instWords(spvenums.OpMemberDecorate, 12, 0, uint32(spvenums.DecorationOffset), 0),
		instWords(spvenums.OpMemberDecorate, 12, 1, uint32(spvenums.DecorationOffset), 4),
		instWords(spvenums.OpDecorate, 18, uint32(spvenums.DecorationDescriptorSet), 0),
		instWords(spvenums.OpDecorate, 18, uint32(spvenums.DecorationBinding), 0),
		instWords(spvenums.OpTypeVoid, 10),
		instWords(spvenums.OpTypeFunction, 11, 10),
		instWords(spvenums.OpTypeInt, 14, 32, 0),
		instWords(spvenums.OpTypeStruct, 12, 14, 14),
		instWords(spvenums.OpTypePointer, 13, uint32(spvenums.StorageClassStorageBuffer), 12),
		instWords(spvenums.OpTypePointer, 15, uint32(spvenums.StorageClassStorageBuffer), 14),
		instWords(spvenums.OpConstant, 14, 16, 0),
		instWords(spvenums.OpConstant, 14, 17, 1),
		instWords(spvenums.OpVariable, 13, 18, uint32(spvenums.StorageClassStorageBuffer)),
		voidFunctionBody(),
		instWords(spvenums.OpLabel, 2),
	}
	groups = append(groups, body...)
	return append(groups,
		instWords(spvenums.OpReturn),
		instWords(spvenums.OpFunctionEnd),
	)
}

// TestAtomicUpgradeField: an atomic access to one member of a storage buffer
// struct upgrades that member's type to Atomic and leaves the other alone.
func TestAtomicUpgradeField(t *testing.T) {
	module, _, err := parseWords(t, Options{}, atomicModuleGroups(
		instWords(spvenums.OpAccessChain, 15, 20, 18, 16),
		instWords(spvenums.OpAtomicIAdd, 14, 21, 20, 16, 16, 17),
	)...)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(module.GlobalVariables) != 1 {
		t.Fatalf("globals = %d, want 1", len(module.GlobalVariables))
	}
	global := module.GlobalVariables[0]
	structType, ok := module.Types[global.Type].Inner.(ir.StructType)
	if !ok {
		t.Fatalf("global type = %T, want a struct", module.Types[global.Type].Inner)
	}
	if _, ok := module.Types[structType.Members[0].Type].Inner.(ir.AtomicType); !ok {
		t.Errorf("member 0 = %T, want atomic", module.Types[structType.Members[0].Type].Inner)
	}
	if _, ok := module.Types[structType.Members[1].Type].Inner.(ir.ScalarType); !ok {
		t.Errorf("member 1 = %T, want the untouched scalar", module.Types[structType.Members[1].Type].Inner)
	}
	fn := &module.Functions[0]
	atomic, ok := findStatement(fn, func(k ir.StatementKind) bool {
		_, ok := k.(ir.StmtAtomic)
		return ok
	})
	if !ok {
		t.Fatal("no Atomic statement")
	}
	if _, ok := atomic.(ir.StmtAtomic).Fun.(ir.AtomicAdd); !ok {
		t.Errorf("atomic function = %T, want Add", atomic.(ir.StmtAtomic).Fun)
	}
}

// TestAtomicCompareExchangeShape: the statement's result is the predeclared
// compare-exchange struct, and the instruction's result is its old value.
func TestAtomicCompareExchangeShape(t *testing.T) {
	module, _, err := parseWords(t, Options{}, atomicModuleGroups(
		instWords(spvenums.OpAccessChain, 15, 20, 18, 16),
		instWords(spvenums.OpAtomicCompareExchange, 14, 21, 20, 16, 16, 16, 17, 17),
	)...)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	fn := &module.Functions[0]
	atomic, ok := findStatement(fn, func(k ir.StatementKind) bool {
		_, ok := k.(ir.StmtAtomic)
		return ok
	})
	if !ok {
		t.Fatal("no Atomic statement")
	}
	exchange, ok := atomic.(ir.StmtAtomic).Fun.(ir.AtomicExchange)
	if !ok || exchange.Compare == nil {
		t.Fatalf("atomic function = %T, want a compare-exchange", atomic.(ir.StmtAtomic).Fun)
	}
	result := atomic.(ir.StmtAtomic).Result
	if result == nil {
		t.Fatal("the atomic statement has no result")
	}
	kind, ok := fn.Expressions[*result].Kind.(ir.ExprAtomicResult)
	if !ok || !kind.Comparison {
		t.Fatalf("result expression = %T, want a comparison AtomicResult", fn.Expressions[*result].Kind)
	}
	if name := module.Types[kind.Ty].Name; name != "__atomic_compare_exchange_result<Uint,4>" {
		t.Errorf("result type name = %q", name)
	}
	if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
		access, ok := k.(ir.ExprAccessIndex)
		return ok && access.Base == *result && access.Index == 0
	}); !ok {
		t.Error("no access to the result's old value")
	}
}

// TestCompositeInsertBudget: a composite larger than the frontend accepts is
// refused instead of built.
func TestCompositeInsertBudget(t *testing.T) {
	const huge = maxCompositeInsertExpressions + 1
	_, _, err := parseWords(t, Options{},
		headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpMemoryModel, 0, 1),
		entryPointOp(spvenums.ExecutionModelGLCompute),
		instWords(spvenums.OpExecutionMode, 1, uint32(spvenums.ExecutionModeLocalSize), 1, 1, 1),
		instWords(spvenums.OpTypeVoid, 10),
		instWords(spvenums.OpTypeFunction, 11, 10),
		instWords(spvenums.OpTypeInt, 12, 32, 0),
		instWords(spvenums.OpConstant, 12, 13, uint32(huge)),
		instWords(spvenums.OpTypeArray, 14, 12, 13),
		instWords(spvenums.OpConstantNull, 14, 15),
		instWords(spvenums.OpConstant, 12, 16, 0),
		instWords(spvenums.OpConstantNull, 12, 17),
		voidFunctionBody(),
		instWords(spvenums.OpLabel, 2),
		instWords(spvenums.OpCompositeInsert, 14, 18, 17, 15, 16),
		instWords(spvenums.OpReturn),
		instWords(spvenums.OpFunctionEnd),
	)
	if err == nil {
		t.Fatal("Parse accepted a composite insert over a huge array")
	}
	var parseErr *Error
	if !errors.As(err, &parseErr) || parseErr.Message != errTooManyCompositeInsertComponents.Message {
		t.Fatalf("error = %v, want %q", err, errTooManyCompositeInsertComponents.Message)
	}
}

// findStatement returns the first statement the predicate accepts.
func findStatement(fn *ir.Function, accept func(ir.StatementKind) bool) (ir.StatementKind, bool) {
	var walk func(block ir.Block) (ir.StatementKind, bool)
	walk = func(block ir.Block) (ir.StatementKind, bool) {
		for _, statement := range block {
			if accept(statement.Kind) {
				return statement.Kind, true
			}
			switch kind := statement.Kind.(type) {
			case ir.StmtBlock:
				if found, ok := walk(kind.Block); ok {
					return found, true
				}
			case ir.StmtIf:
				if found, ok := walk(kind.Accept); ok {
					return found, true
				}
				if found, ok := walk(kind.Reject); ok {
					return found, true
				}
			case ir.StmtLoop:
				if found, ok := walk(kind.Body); ok {
					return found, true
				}
				if found, ok := walk(kind.Continuing); ok {
					return found, true
				}
			}
		}
		return nil, false
	}
	return walk(fn.Body)
}
