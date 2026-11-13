package front

import (
	"fmt"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
	"github.com/gogpu/naga/valid"
)

// The call modules share one preamble. Ids: 1 void, 2 fn void(), 3 i32,
// 4 fn i32(i32), 5 pointer to i32 in the Function space, 6 fn void(ptr i32),
// 7 fn i32(), 8 f32, 9 bool, 20 the i32 constant 5, 21 the f32 constant 1.0,
// 70 the bool constant true.
// Functions start at 100; the entry point, when there is one, is function
// 100 named "main".

// callModule assembles a module with the given functions. Each function is
// named fN after its id N, so the order of Module.Functions can be read back.
func callModule(entryPoint bool, functions ...[][]uint32) []byte {
	groups := [][]uint32{
		headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpMemoryModel, 0, 1),
	}
	if entryPoint {
		words := []uint32{uint32(spvenums.ExecutionModelGLCompute), 100}
		words = append(words, stringWords("main")...)
		groups = append(groups,
			instWords(spvenums.OpEntryPoint, words...),
			instWords(spvenums.OpExecutionMode, 100, uint32(spvenums.ExecutionModeLocalSize), 1, 1, 1),
		)
	}
	for _, function := range functions {
		id := function[0][2]
		groups = append(groups, instWithString(spvenums.OpName, []uint32{id}, fmt.Sprintf("f%d", id)))
	}
	groups = append(groups,
		instWords(spvenums.OpTypeVoid, 1),
		instWords(spvenums.OpTypeFunction, 2, 1),
		instWords(spvenums.OpTypeInt, 3, 32, 1),
		instWords(spvenums.OpTypeFunction, 4, 3, 3),
		instWords(spvenums.OpTypePointer, 5, uint32(spvenums.StorageClassFunction), 3),
		instWords(spvenums.OpTypeFunction, 6, 1, 5),
		instWords(spvenums.OpTypeFunction, 7, 3),
		instWords(spvenums.OpTypeFloat, 8, 32),
		instWords(spvenums.OpTypeBool, 9),
		instWords(spvenums.OpConstant, 3, 20, 5),
		instWords(spvenums.OpConstant, 8, 21, 0x3f800000),
		instWords(spvenums.OpConstantTrue, 9, 70),
	)
	for _, function := range functions {
		groups = append(groups, function...)
	}
	return spvWords(groups...)
}

// voidFunction is a function void() with one block: body, then OpReturn.
func voidFunction(id uint32, body ...[]uint32) [][]uint32 {
	function := [][]uint32{
		instWords(spvenums.OpFunction, 1, id, 0, 2),
		instWords(spvenums.OpLabel, id+1),
	}
	function = append(function, body...)
	return append(function, instWords(spvenums.OpReturn), instWords(spvenums.OpFunctionEnd))
}

// identityFunction is a function i32(i32) returning its parameter.
func identityFunction(id uint32) [][]uint32 {
	return [][]uint32{
		instWords(spvenums.OpFunction, 3, id, 0, 4),
		instWords(spvenums.OpFunctionParameter, 3, id+1),
		instWords(spvenums.OpLabel, id+2),
		instWords(spvenums.OpReturnValue, id+1),
		instWords(spvenums.OpFunctionEnd),
	}
}

// call is OpFunctionCall resultType result callee arguments...
func call(resultType, result, callee uint32, arguments ...uint32) []uint32 {
	return instWords(spvenums.OpFunctionCall, append([]uint32{resultType, result, callee}, arguments...)...)
}

// parseCalls parses a call module, validates it and returns it.
func parseCalls(t *testing.T, entryPoint bool, functions ...[][]uint32) *ir.Module {
	t.Helper()
	module, _, err := Parse(callModule(entryPoint, functions...), Options{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, err := valid.Validate(module, valid.Options{Capabilities: valid.CapAll}); err != nil {
		t.Fatalf("valid.Validate: %v", err)
	}
	return module
}

// callError parses a call module that must be refused.
func callError(t *testing.T, entryPoint bool, functions ...[][]uint32) *Error {
	t.Helper()
	_, _, err := Parse(callModule(entryPoint, functions...), Options{})
	if err == nil {
		t.Fatal("Parse accepted the module")
	}
	return requireError(t, err)
}

// functionOrder returns the names of Module.Functions in arena order.
func functionOrder(module *ir.Module) []string {
	names := make([]string, len(module.Functions))
	for i := range module.Functions {
		names[i] = module.Functions[i].Name
	}
	return names
}

// functionHandle returns the handle of the function named name.
func functionHandle(t *testing.T, module *ir.Module, name string) ir.FunctionHandle {
	t.Helper()
	for i := range module.Functions {
		if module.Functions[i].Name == name {
			return ir.FunctionHandle(i)
		}
	}
	t.Fatalf("no function %q in %v", name, functionOrder(module))
	return 0
}

// calls returns the call statements of a block, nested ones included.
func calls(block ir.Block) []ir.StmtCall {
	var out []ir.StmtCall
	for _, statement := range block {
		switch kind := statement.Kind.(type) {
		case ir.StmtCall:
			out = append(out, kind)
		case ir.StmtBlock:
			out = append(out, calls(kind.Block)...)
		case ir.StmtIf:
			out = append(out, calls(kind.Accept)...)
			out = append(out, calls(kind.Reject)...)
		case ir.StmtLoop:
			out = append(out, calls(kind.Body)...)
			out = append(out, calls(kind.Continuing)...)
		case ir.StmtSwitch:
			for _, c := range kind.Cases {
				out = append(out, calls(c.Body)...)
			}
		}
	}
	return out
}

func equalNames(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// TestCallToLaterFunction: the callee is parsed after its caller, so the call
// holds a placeholder until every function is parsed. Rust naga then orders
// Module.Functions callees first (the reversed toposort of its call graph).
func TestCallToLaterFunction(t *testing.T) {
	module := parseCalls(t, true,
		voidFunction(100, call(3, 50, 200, 20)),
		identityFunction(200),
	)
	if got, want := functionOrder(module), []string{"f200", "f100"}; !equalNames(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
	callee := functionHandle(t, module, "f200")
	caller := &module.Functions[functionHandle(t, module, "f100")]
	got := calls(caller.Body)
	if len(got) != 1 {
		t.Fatalf("calls = %d, want 1", len(got))
	}
	if got[0].Function != callee {
		t.Errorf("call target = %d, want %d", got[0].Function, callee)
	}
	if got[0].Result == nil {
		t.Fatal("the call has no result")
	}
	result, ok := caller.Expressions[*got[0].Result].Kind.(ir.ExprCallResult)
	if !ok || result.Function != callee {
		t.Errorf("result = %#v, want CallResult(%d)", caller.Expressions[*got[0].Result].Kind, callee)
	}
	if len(got[0].Arguments) != 1 {
		t.Fatalf("arguments = %d, want 1", len(got[0].Arguments))
	}
	wrapperCalls := calls(module.EntryPoints[0].Function.Body)
	if len(wrapperCalls) != 1 || wrapperCalls[0].Function != functionHandle(t, module, "f100") {
		t.Errorf("wrapper calls = %#v, want one call of f100", wrapperCalls)
	}
}

// TestCallToEarlierFunction: a callee parsed first keeps its place.
func TestCallToEarlierFunction(t *testing.T) {
	module := parseCalls(t, false,
		identityFunction(200),
		voidFunction(100, call(3, 50, 200, 20)),
	)
	if got, want := functionOrder(module), []string{"f200", "f100"}; !equalNames(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
	got := calls(module.Functions[1].Body)
	if len(got) != 1 || got[0].Function != 0 {
		t.Fatalf("calls = %#v, want one call of function 0", got)
	}
}

// TestCallOrderFollowsRustToposort pins the order petgraph's toposort gives:
// a caller with two later callees puts the second callee first, because the
// depth-first search finishes the most recently pushed neighbor first.
func TestCallOrderFollowsRustToposort(t *testing.T) {
	module := parseCalls(t, true,
		voidFunction(100, call(1, 50, 200), call(1, 51, 300)),
		voidFunction(200),
		voidFunction(300, call(1, 52, 400)),
		voidFunction(400),
	)
	// Nodes in insertion order: 100, 200, 300, 400, the wrapper. The search
	// from 100 pushes 200 and 300, visits 300 first, which reaches 400:
	// finished 400, 300, 200, 100, wrapper.
	if got, want := functionOrder(module), []string{"f400", "f300", "f200", "f100"}; !equalNames(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
	for i := range module.Functions {
		for _, c := range calls(module.Functions[i].Body) {
			if int(c.Function) >= i {
				t.Errorf("%s calls function %d, which is not before it", module.Functions[i].Name, c.Function)
			}
		}
	}
}

// TestCallWithPointerArgument is do-while's shape: a void function taking a
// pointer to a local, called with a local variable.
func TestCallWithPointerArgument(t *testing.T) {
	module := parseCalls(t, true,
		[][]uint32{
			instWords(spvenums.OpFunction, 1, 100, 0, 2),
			instWords(spvenums.OpLabel, 101),
			instWords(spvenums.OpVariable, 5, 60, uint32(spvenums.StorageClassFunction)),
			call(1, 61, 200, 60),
			instWords(spvenums.OpReturn),
			instWords(spvenums.OpFunctionEnd),
		},
		[][]uint32{
			instWords(spvenums.OpFunction, 1, 200, 0, 6),
			instWords(spvenums.OpFunctionParameter, 5, 201),
			instWords(spvenums.OpLabel, 202),
			instWords(spvenums.OpStore, 201, 20),
			instWords(spvenums.OpReturn),
			instWords(spvenums.OpFunctionEnd),
		},
	)
	got := calls(module.Functions[functionHandle(t, module, "f100")].Body)
	if len(got) != 1 || got[0].Result != nil {
		t.Fatalf("calls = %#v, want one call without a result", got)
	}
}

// TestCallInsideControlFlow: a call nested in a selection is patched too.
func TestCallInsideControlFlow(t *testing.T) {
	module := parseCalls(t, false,
		[][]uint32{
			instWords(spvenums.OpFunction, 1, 100, 0, 2),
			instWords(spvenums.OpLabel, 101),
			instWords(spvenums.OpSelectionMerge, 103, 0),
			instWords(spvenums.OpBranchConditional, 70, 102, 103),
			instWords(spvenums.OpLabel, 102),
			call(3, 50, 200, 20),
			instWords(spvenums.OpBranch, 103),
			instWords(spvenums.OpLabel, 103),
			instWords(spvenums.OpReturn),
			instWords(spvenums.OpFunctionEnd),
		},
		identityFunction(200),
	)
	got := calls(module.Functions[functionHandle(t, module, "f100")].Body)
	if len(got) != 1 || got[0].Function != functionHandle(t, module, "f200") {
		t.Fatalf("calls = %#v, want one call of f200", got)
	}
}

// TestCallErrors: Rust naga's FunctionCallCycle for recursion, and this
// package's own errors where Rust naga panics (an undefined callee) or builds
// IR its validator rejects (a call that does not match the callee's type).
func TestCallErrors(t *testing.T) {
	tests := []struct {
		name      string
		functions [][][]uint32
		want      string
	}{
		{
			name:      "direct recursion",
			functions: [][][]uint32{voidFunction(100, call(1, 50, 100))},
			want:      "recursive function call %100",
		},
		{
			// petgraph reports the second node its reversed search reaches.
			name: "mutual recursion",
			functions: [][][]uint32{
				voidFunction(100, call(1, 50, 200)),
				voidFunction(200, call(1, 51, 100)),
			},
			want: "recursive function call %200",
		},
		{
			name: "recursion through three functions",
			functions: [][][]uint32{
				voidFunction(100, call(1, 50, 200)),
				voidFunction(200, call(1, 51, 300)),
				voidFunction(300, call(1, 52, 100)),
			},
			want: "recursive function call %300",
		},
		{
			name:      "undefined callee",
			functions: [][][]uint32{voidFunction(100, call(1, 50, 999))},
			want:      "invalid id %999",
		},
		{
			name:      "callee is not a function",
			functions: [][][]uint32{voidFunction(100, call(1, 50, 20))},
			want:      "invalid id %20",
		},
		{
			name:      "undefined argument",
			functions: [][][]uint32{voidFunction(100, call(3, 50, 200, 77)), identityFunction(200)},
			want:      "invalid id %77",
		},
		{
			name:      "too few arguments",
			functions: [][][]uint32{voidFunction(100, call(3, 50, 200)), identityFunction(200)},
			want:      "function %200 takes 1 arguments, the call passes 0",
		},
		{
			name:      "too many arguments",
			functions: [][][]uint32{voidFunction(100, call(3, 50, 200, 20, 20)), identityFunction(200)},
			want:      "function %200 takes 1 arguments, the call passes 2",
		},
		{
			name:      "argument type",
			functions: [][][]uint32{voidFunction(100, call(3, 50, 200, 21)), identityFunction(200)},
			want:      "wrong function argument type %8",
		},
		{
			name:      "result type",
			functions: [][][]uint32{voidFunction(100, call(8, 50, 200, 20)), identityFunction(200)},
			want:      "wrong function result type %8",
		},
		{
			name:      "result of a void function",
			functions: [][][]uint32{voidFunction(100, call(3, 50, 200)), voidFunction(200)},
			want:      "wrong function result type %3",
		},
		{
			name:      "function defined twice",
			functions: [][][]uint32{voidFunction(100), voidFunction(100)},
			want:      "function %100 is defined twice",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := callError(t, false, tt.functions...)
			if err.Message != tt.want {
				t.Errorf("message = %q, want %q", err.Message, tt.want)
			}
		})
	}
}

// TestCallEntryPointWithParameters: the wrapper calls the entry point's
// function without arguments, so a function with parameters cannot be one.
func TestCallEntryPointWithParameters(t *testing.T) {
	err := callError(t, true, [][]uint32{
		instWords(spvenums.OpFunction, 3, 100, 0, 4),
		instWords(spvenums.OpFunctionParameter, 3, 101),
		instWords(spvenums.OpLabel, 102),
		instWords(spvenums.OpReturnValue, 101),
		instWords(spvenums.OpFunctionEnd),
	})
	if want := "function %100 takes 1 arguments, the call passes 0"; err.Message != want {
		t.Errorf("message = %q, want %q", err.Message, want)
	}
}

// TestCallMutations rewrites every operand word of a module with calls:
// each result is an *Error or IR that validates, never a panic.
func TestCallMutations(t *testing.T) {
	functions := [][][]uint32{
		voidFunction(100, call(3, 50, 200, 20), call(1, 51, 300, 60)),
		identityFunction(200),
		{
			instWords(spvenums.OpFunction, 1, 300, 0, 6),
			instWords(spvenums.OpFunctionParameter, 5, 301),
			instWords(spvenums.OpLabel, 302),
			call(3, 53, 200, 20),
			instWords(spvenums.OpStore, 301, 53),
			instWords(spvenums.OpReturn),
			instWords(spvenums.OpFunctionEnd),
		},
	}
	// The caller needs a local for the pointer argument.
	functions[0] = append([][]uint32{
		functions[0][0], functions[0][1],
		instWords(spvenums.OpVariable, 5, 60, uint32(spvenums.StorageClassFunction)),
	}, functions[0][2:]...)
	values := []uint32{0, 1, 3, 4, 5, 6, 8, 20, 21, 50, 60, 100, 200, 201, 300, 301, 0xffffffff}
	parses := 0
	for f := range functions {
		for i := range functions[f] {
			for w := 1; w < len(functions[f][i]); w++ {
				for _, v := range values {
					mutated := make([][][]uint32, len(functions))
					copy(mutated, functions)
					function := make([][]uint32, len(functions[f]))
					copy(function, functions[f])
					inst := append([]uint32(nil), functions[f][i]...)
					inst[w] = v
					function[i] = inst
					mutated[f] = function
					func() {
						defer func() {
							if r := recover(); r != nil {
								t.Fatalf("function %d, instruction %d word %d = %d: panic %v", f, i, w, v, r)
							}
						}()
						module, _, err := Parse(callModule(true, mutated...), Options{})
						if err != nil {
							requireError(t, err)
							return
						}
						parses++
						if _, err := valid.Validate(module, valid.Options{Capabilities: valid.CapAll}); err != nil {
							t.Errorf("function %d, instruction %d word %d = %d: parses into invalid IR: %v", f, i, w, v, err)
						}
					}()
				}
			}
		}
	}
	if parses == 0 {
		t.Error("no mutation parsed: the module itself is broken")
	}
}
