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
	return atomicVariantGroups(instWords(spvenums.OpTypeStruct, 12, 14, 14), nil, nil, body...)
}

// atomicVariantGroups is atomicModuleGroups with another struct type, extra
// groups before it (the member types it names) and, when variable is not nil,
// another OpVariable instruction.
func atomicVariantGroups(structType []uint32, extra [][]uint32, variable []uint32, body ...[]uint32) [][]uint32 {
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
		instWords(spvenums.OpTypePointer, 15, uint32(spvenums.StorageClassStorageBuffer), 14),
		instWords(spvenums.OpConstant, 14, 16, 0),
		instWords(spvenums.OpConstant, 14, 17, 1),
	}
	groups = append(groups, extra...)
	groups = append(groups,
		structType,
		instWords(spvenums.OpTypePointer, 13, uint32(spvenums.StorageClassStorageBuffer), 12),
	)
	if variable == nil {
		variable = instWords(spvenums.OpVariable, 13, 18, uint32(spvenums.StorageClassStorageBuffer))
	}
	groups = append(groups,
		variable,
		voidFunctionBody(),
		instWords(spvenums.OpLabel, 2),
	)
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

// TestAtomicOpsMatrix: every atomic instruction lowers to the IR operation it
// means, the load and the store become plain ones, and the increment and the
// decrement carry the literal one of the value's type.
func TestAtomicOpsMatrix(t *testing.T) {
	module, _, err := parseWords(t, Options{}, atomicModuleGroups(
		instWords(spvenums.OpAccessChain, 15, 20, 18, 16),
		instWords(spvenums.OpAtomicLoad, 14, 21, 20, 16, 16),
		instWords(spvenums.OpAtomicStore, 20, 16, 16, 17),
		instWords(spvenums.OpAtomicExchange, 14, 22, 20, 16, 16, 17),
		instWords(spvenums.OpAtomicIAdd, 14, 23, 20, 16, 16, 17),
		instWords(spvenums.OpAtomicISub, 14, 24, 20, 16, 16, 17),
		instWords(spvenums.OpAtomicSMin, 14, 25, 20, 16, 16, 17),
		instWords(spvenums.OpAtomicUMax, 14, 26, 20, 16, 16, 17),
		instWords(spvenums.OpAtomicAnd, 14, 27, 20, 16, 16, 17),
		instWords(spvenums.OpAtomicOr, 14, 28, 20, 16, 16, 17),
		instWords(spvenums.OpAtomicXor, 14, 29, 20, 16, 16, 17),
		instWords(spvenums.OpAtomicIIncrement, 14, 30, 20, 16, 16),
		instWords(spvenums.OpAtomicIDecrement, 14, 31, 20, 16, 16),
	)...)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	fn := &module.Functions[0]
	var atomics []ir.StmtAtomic
	store := false
	for _, statement := range fn.Body {
		switch kind := statement.Kind.(type) {
		case ir.StmtStore:
			store = true
		case ir.StmtAtomic:
			atomics = append(atomics, kind)
		}
	}
	if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
		_, ok := k.(ir.ExprLoad)
		return ok
	}); !ok {
		t.Error("OpAtomicLoad became no load expression")
	}
	if !store {
		t.Error("OpAtomicStore became no store")
	}
	want := []ir.AtomicFunction{
		ir.AtomicExchange{}, ir.AtomicAdd{}, ir.AtomicSubtract{}, ir.AtomicMin{}, ir.AtomicMax{},
		ir.AtomicAnd{}, ir.AtomicInclusiveOr{}, ir.AtomicExclusiveOr{},
		ir.AtomicAdd{}, ir.AtomicSubtract{},
	}
	if len(atomics) != len(want) {
		t.Fatalf("atomics = %d, want %d", len(atomics), len(want))
	}
	for i := range want {
		if atomics[i].Fun != want[i] {
			t.Errorf("atomic %d = %#v, want %#v", i, atomics[i].Fun, want[i])
		}
		if atomics[i].Result == nil {
			t.Errorf("atomic %d has no result", i)
		}
	}
	for _, i := range []int{8, 9} {
		literal, ok := fn.Expressions[atomics[i].Value].Kind.(ir.Literal)
		if !ok {
			t.Fatalf("atomic %d value = %T, want the literal one", i, fn.Expressions[atomics[i].Value].Kind)
		}
		if literal.Value != ir.LiteralU32(1) {
			t.Errorf("atomic %d literal = %v, want u32 1", i, literal.Value)
		}
	}
}

// TestAtomicUpgradeArrayMember: an atomic access through an array member of a
// storage struct upgrades the array's element type, not the whole member.
func TestAtomicUpgradeArrayMember(t *testing.T) {
	arrayOfU32 := instWords(spvenums.OpTypeArray, 19, 14, 17) // array<u32, 1>
	module, _, err := parseWords(t, Options{},
		atomicVariantGroups(instWords(spvenums.OpTypeStruct, 12, 19, 14), [][]uint32{arrayOfU32}, nil,
			instWords(spvenums.OpAccessChain, 15, 20, 18, 16, 16),
			instWords(spvenums.OpAtomicIAdd, 14, 21, 20, 16, 16, 17),
		)...)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	global := module.GlobalVariables[0]
	structType, ok := module.Types[global.Type].Inner.(ir.StructType)
	if !ok {
		t.Fatalf("global type = %T, want a struct", module.Types[global.Type].Inner)
	}
	array, ok := module.Types[structType.Members[0].Type].Inner.(ir.ArrayType)
	if !ok {
		t.Fatalf("member 0 = %T, want an array", module.Types[structType.Members[0].Type].Inner)
	}
	if _, ok := module.Types[array.Base].Inner.(ir.AtomicType); !ok {
		t.Errorf("array base = %T, want atomic", module.Types[array.Base].Inner)
	}
	if _, ok := module.Types[structType.Members[1].Type].Inner.(ir.ScalarType); !ok {
		t.Errorf("member 1 = %T, want the untouched scalar", module.Types[structType.Members[1].Type].Inner)
	}
}

// TestAtomicPointerNotGlobal: an atomic instruction on a pointer that does not
// lead to a global variable is refused with the upgrade error.
func TestAtomicPointerNotGlobal(t *testing.T) {
	// A function-scope variable is a pointer the atomic's base type accepts,
	// but the upgrade only rewrites globals.
	localPointer := instWords(spvenums.OpTypePointer, 22, uint32(spvenums.StorageClassFunction), 14)
	_, _, err := parseWords(t, Options{},
		atomicVariantGroups(instWords(spvenums.OpTypeStruct, 12, 14, 14), [][]uint32{localPointer}, nil,
			instWords(spvenums.OpVariable, 22, 20, uint32(spvenums.StorageClassFunction)),
			instWords(spvenums.OpAtomicIAdd, 14, 21, 20, 16, 16, 17),
		)...)
	if err == nil {
		t.Fatal("Parse accepted an atomic on a function-scope pointer")
	}
	if got := requireError(t, err).Message; got != errAtomicGlobalVariableMissing.Message {
		t.Errorf("message = %q, want %q", got, errAtomicGlobalVariableMissing.Message)
	}
}

// TestAtomicGlobalInit: an atomically accessed global with an initializer is
// refused: the IR has no atomic global with an initializer (Rust naga's
// atomic_upgrade refuses it the same way).
func TestAtomicGlobalInit(t *testing.T) {
	variable := instWords(spvenums.OpVariable, 13, 18, uint32(spvenums.StorageClassStorageBuffer), 16)
	_, _, err := parseWords(t, Options{},
		atomicVariantGroups(instWords(spvenums.OpTypeStruct, 12, 14, 14), nil, variable,
			instWords(spvenums.OpAccessChain, 15, 20, 18, 16),
			instWords(spvenums.OpAtomicIAdd, 14, 21, 20, 16, 16, 17),
		)...)
	if err == nil {
		t.Fatal("Parse accepted an atomic global with an initializer")
	}
	if got := requireError(t, err).Message; got != errAtomicGlobalInitUnsupported.Message {
		t.Errorf("message = %q, want %q", got, errAtomicGlobalInitUnsupported.Message)
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
