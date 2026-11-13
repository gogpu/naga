package front

import (
	"runtime"
	"strings"
	"testing"

	"github.com/gogpu/naga/spirv/internal/spvenums"
	"github.com/gogpu/naga/valid"
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

// typeChainModule assembles a module whose types nest levels deep: i32 (one
// level) wrapped levels-1 times in a one-element array or a one-member struct,
// then tail more constants, which a refusal at the chain must never read.
func typeChainModule(levels int, structs bool, tail int) []byte {
	groups := [][]uint32{
		headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpMemoryModel, 0, 1),
		instWords(spvenums.OpTypeInt, 3, 32, 1),
		instWords(spvenums.OpTypeInt, 4, 32, 0),
		instWords(spvenums.OpConstant, 4, 5, 1),
	}
	id := uint32(10)
	base := uint32(3)
	for range levels - 1 {
		if structs {
			groups = append(groups, instWords(spvenums.OpTypeStruct, id, base))
		} else {
			groups = append(groups, instWords(spvenums.OpTypeArray, id, base, 5))
		}
		base = id
		id++
	}
	for i := range tail {
		groups = append(groups, instWords(spvenums.OpConstant, 4, id, uint32(i)))
		id++
	}
	return spvWords(groups...)
}

// TestTypeDepthLimit: types nest at most maxTypeDepth levels, so every
// recursive walk over the IR's types (the layouter, the validator, the
// backends' type writers) is bounded. SPIR-V lets a chain of OpTypeArray or
// OpTypeStruct grow with the module; Rust naga has no bound.
func TestTypeDepthLimit(t *testing.T) {
	for _, structs := range []bool{false, true} {
		name := map[bool]string{false: "arrays", true: "structs"}[structs]
		t.Run(name+" at the limit", func(t *testing.T) {
			module, _, err := Parse(typeChainModule(maxTypeDepth, structs, 0), Options{})
			if err != nil {
				t.Fatalf("Parse at the limit: %v", err)
			}
			if len(module.Types) < maxTypeDepth {
				t.Errorf("types = %d, want the whole chain", len(module.Types))
			}
			if _, err := valid.Validate(module, valid.Options{Capabilities: valid.CapAll}); err != nil {
				t.Errorf("valid.Validate: %v", err)
			}
		})
		t.Run(name+" over the limit", func(t *testing.T) {
			_, _, err := Parse(typeChainModule(maxTypeDepth+1, structs, 0), Options{})
			if err == nil {
				t.Fatal("Parse accepted a type nested over the limit")
			}
			want := "type is nested deeper than 64"
			if got := requireError(t, err).Message; got != want {
				t.Errorf("message = %q, want %q", got, want)
			}
		})
	}
}

// TestTypeDepthLimitCostIndependentOfInput: the type over the limit is
// refused where it is declared, so a module that goes on for a hundred times
// longer allocates no more.
func TestTypeDepthLimitCostIndependentOfInput(t *testing.T) {
	measure := func(tail int) uint64 {
		data := typeChainModule(maxTypeDepth+1, false, tail)
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		_, _, err := Parse(data, Options{})
		runtime.ReadMemStats(&after)
		if err == nil {
			t.Fatalf("Parse with a tail of %d: expected the type depth limit", tail)
		}
		return after.TotalAlloc - before.TotalAlloc
	}
	small := measure(1000)
	large := measure(100_000)
	const noise = 1 << 20
	if large > small+noise {
		t.Fatalf("refusing with a tail of 100000 allocated %d bytes, with 1000 %d: the cost grows with the input",
			large, small)
	}
}

// TestHeaderBoundIsIgnored: the header's id bound is never used to size
// anything (design §16; Rust naga ignores it too), so a module claiming 2^32
// ids parses with the allocations of a small one.
func TestHeaderBoundIsIgnored(t *testing.T) {
	module := func(bound uint32) []byte {
		header := headerWords()
		header[3] = bound
		return spvWords(header,
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpTypeInt, 3, 32, 1),
		)
	}
	measure := func(bound uint32) uint64 {
		data := module(bound)
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		if _, _, err := Parse(data, Options{}); err != nil {
			t.Fatalf("Parse with bound %d: %v", bound, err)
		}
		runtime.ReadMemStats(&after)
		return after.TotalAlloc - before.TotalAlloc
	}
	small := measure(4)
	large := measure(0xffffffff)
	const noise = 1 << 20
	if large > small+noise {
		t.Fatalf("a bound of 2^32-1 allocated %d bytes, a bound of 4 %d", large, small)
	}
}

// TestWorkgroupSizeIsKeptRaw: the LocalSize values go into the IR as read;
// the frontend never multiplies them (design §16), the validator and the
// device limits judge them.
func TestWorkgroupSizeIsKeptRaw(t *testing.T) {
	words := []uint32{uint32(spvenums.ExecutionModelGLCompute), 1}
	words = append(words, stringWords("main")...)
	module, _, err := Parse(spvWords(
		headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpMemoryModel, 0, 1),
		instWords(spvenums.OpEntryPoint, words...),
		instWords(spvenums.OpExecutionMode, 1, uint32(spvenums.ExecutionModeLocalSize), 0xffffffff, 0xffffffff, 0xffffffff),
		instWords(spvenums.OpTypeVoid, 10),
		instWords(spvenums.OpTypeFunction, 11, 10),
		instWords(spvenums.OpFunction, 10, 1, 0, 11),
		instWords(spvenums.OpLabel, 2),
		instWords(spvenums.OpReturn),
		instWords(spvenums.OpFunctionEnd),
	), Options{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := module.EntryPoints[0].Workgroup; got != [3]uint32{0xffffffff, 0xffffffff, 0xffffffff} {
		t.Errorf("workgroup = %v, want the raw values", got)
	}
}

// TestLongCallChain: the call graph is sorted and patched without recursion,
// so a chain of calls as long as the module allows cannot exhaust the stack.
// Each function calls the next one, which is declared after it.
func TestLongCallChain(t *testing.T) {
	const functions = 20_000
	var chain [][][]uint32
	for i := range functions {
		id := uint32(100 + 2*i)
		if i == functions-1 {
			chain = append(chain, [][]uint32{
				instWords(spvenums.OpFunction, 1, id, 0, 2),
				instWords(spvenums.OpLabel, id+1),
				instWords(spvenums.OpReturn),
				instWords(spvenums.OpFunctionEnd),
			})
			break
		}
		chain = append(chain, [][]uint32{
			instWords(spvenums.OpFunction, 1, id, 0, 2),
			instWords(spvenums.OpLabel, id+1),
			instWords(spvenums.OpFunctionCall, 1, 1_000_000+id, id+2),
			instWords(spvenums.OpReturn),
			instWords(spvenums.OpFunctionEnd),
		})
	}
	groups := [][]uint32{
		headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpMemoryModel, 0, 1),
		instWords(spvenums.OpTypeVoid, 1),
		instWords(spvenums.OpTypeFunction, 2, 1),
	}
	for _, function := range chain {
		groups = append(groups, function...)
	}
	module, _, err := Parse(spvWords(groups...), Options{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(module.Functions) != functions {
		t.Fatalf("functions = %d, want %d", len(module.Functions), functions)
	}
	// Callees first: the last function of the chain comes first.
	for i := 1; i < functions; i++ {
		calls := calls(module.Functions[i].Body)
		if len(calls) != 1 || int(calls[0].Function) != i-1 {
			t.Fatalf("function %d calls %v, want function %d", i, calls, i-1)
		}
	}
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
