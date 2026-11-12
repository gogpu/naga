package front

import (
	"runtime"
	"strings"
	"testing"

	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// storageModule assembles a module of constants u32 constants and functions
// empty void functions. Each function's arena starts with one expression per
// constant, so the module asks for constants × functions storage expressions.
func storageModule(constants, functions int) []byte {
	groups := [][]uint32{
		headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpMemoryModel, 0, 1),
		instWords(spvenums.OpTypeVoid, 1),
		instWords(spvenums.OpTypeFunction, 2, 1),
		instWords(spvenums.OpTypeInt, 3, 32, 0),
	}
	id := uint32(10)
	for i := range constants {
		groups = append(groups, instWords(spvenums.OpConstant, 3, id, uint32(i)))
		id++
	}
	for range functions {
		groups = append(groups,
			instWords(spvenums.OpFunction, 1, id, 0, 2),
			instWords(spvenums.OpLabel, id+1),
			instWords(spvenums.OpReturn),
			instWords(spvenums.OpFunctionEnd),
		)
		id += 2
	}
	return spvWords(groups...)
}

// TestStorageExpressionLimit: the global and constant expressions all
// functions get together are bounded by maxStorageExpressions. Rust naga has
// no such bound (front/spv/function.rs, make_expression_storage), so 4,000
// constants and 4,000 functions in a 208 KB module built 16M expressions.
func TestStorageExpressionLimit(t *testing.T) {
	const constants = 1024
	atLimit := maxStorageExpressions / constants

	t.Run("at the limit", func(t *testing.T) {
		if _, _, err := Parse(storageModule(constants, atLimit), Options{}); err != nil {
			t.Fatalf("Parse at the limit: %v", err)
		}
	})

	t.Run("just over the limit", func(t *testing.T) {
		_, _, err := Parse(storageModule(constants, atLimit+1), Options{})
		if err == nil {
			t.Fatal("Parse accepted a module over the storage expression limit")
		}
		if got := requireError(t, err).Message; !strings.Contains(got, "more than the limit") {
			t.Errorf("message = %q, want the storage expression limit", got)
		}
	})
}

// TestStorageExpressionLimitCostIndependentOfFunctions: a module over the
// limit is refused at the function that crosses it, so a module that claims a
// hundred times more functions allocates no more. The bound is absolute, as in
// the WGSL frontend's early-error test: TotalAlloc is process-wide.
func TestStorageExpressionLimitCostIndependentOfFunctions(t *testing.T) {
	const constants = 1024
	overLimit := maxStorageExpressions/constants + 1
	measure := func(functions int) uint64 {
		data := storageModule(constants, functions)
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		_, _, err := Parse(data, Options{})
		runtime.ReadMemStats(&after)
		if err == nil {
			t.Fatalf("Parse of %d functions: expected the storage expression limit", functions)
		}
		allocs := after.TotalAlloc - before.TotalAlloc
		t.Logf("%d functions (%d bytes): %d bytes allocated", functions, len(data), allocs)
		return allocs
	}
	small := measure(overLimit)
	large := measure(100 * overLimit)
	const noise = 1 << 20
	if large > small+noise {
		t.Fatalf("refusing %d functions allocated %d bytes, %d functions %d: the cost grows with the input",
			100*overLimit, large, overLimit, small)
	}
}
