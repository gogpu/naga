package spirv

// Diagnostic test: compiles the exact span() pattern from path_count.wgsl
// and disassembles the SPIR-V to inspect instruction ordering around
// function calls and deferred stores.

import (
	"fmt"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

const spanTestShader = `
fn span(a: f32, b: f32) -> u32 {
    return u32(max(ceil(max(a, b)) - floor(min(a, b)), 1.0));
}

@group(0) @binding(0) var<storage, read_write> output: array<u32>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let s0 = vec2<f32>(1.0, 1.0);
    let s1 = vec2<f32>(1.0, 3.0);
    var count_x = span(s0.x, s1.x) - 1u;
    var count = count_x + span(s0.y, s1.y);
    output[0] = count_x;
    output[1] = count;
}
`

// Same shader but with span() inlined for comparison.
const spanInlineShader = `
@group(0) @binding(0) var<storage, read_write> output: array<u32>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let s0 = vec2<f32>(1.0, 1.0);
    let s1 = vec2<f32>(1.0, 3.0);
    var count_x = u32(max(ceil(max(s0.x, s1.x)) - floor(min(s0.x, s1.x)), 1.0)) - 1u;
    var count = count_x + u32(max(ceil(max(s0.y, s1.y)) - floor(min(s0.y, s1.y)), 1.0));
    output[0] = count_x;
    output[1] = count;
}
`

func TestSpanFunctionCallSPIRV(t *testing.T) {
	// Compile function call version
	t.Log("=== Function Call Version ===")
	spirvFunc := compileWGSLToSPIRV(t, "SpanFuncCall", spanTestShader)
	t.Logf("Function call: %d bytes (%d words)", len(spirvFunc), len(spirvFunc)/4)

	// Compile inline version
	t.Log("\n=== Inline Version ===")
	spirvInline := compileWGSLToSPIRV(t, "SpanInline", spanInlineShader)
	t.Logf("Inline: %d bytes (%d words)", len(spirvInline), len(spirvInline)/4)

	// Disassemble both
	t.Log("\n" + "=" + "===========================================")
	t.Log("FUNCTION CALL VERSION DISASSEMBLY")
	t.Log("============================================")
	t.Log("\n" + disassembleSPIRV(spirvFunc))

	t.Log("\n" + "============================================")
	t.Log("INLINE VERSION DISASSEMBLY")
	t.Log("============================================")
	t.Log("\n" + disassembleSPIRV(spirvInline))

	// Focused analysis: find OpFunctionCall and surrounding stores
	t.Log("\n============================================")
	t.Log("FOCUSED: Function calls and deferred stores")
	t.Log("============================================")
	analyzeSpanCalls(t, spirvFunc)
}

// analyzeSpanCalls finds and reports OpFunctionCall instructions and
// surrounding OpStore/OpLoad instructions in the SPIR-V.
func analyzeSpanCalls(t *testing.T, data []byte) {
	t.Helper()

	instrs := decodeSPIRVInstructions(data)
	if instrs == nil {
		t.Fatal("Failed to decode SPIR-V")
	}

	// Collect names
	names := make(map[uint32]string)
	for _, inst := range instrs {
		if inst.opcode == OpName && inst.wordCount >= 3 {
			names[inst.words[1]] = decodeString(inst.words[2:])
		}
	}

	// Find OpFunction, OpFunctionCall, OpStore, OpLoad, OpVariable in the main function
	inMain := false
	for i, inst := range instrs {
		if inst.opcode == OpFunction {
			if name, ok := names[inst.words[2]]; ok && name == "main" {
				inMain = true
				t.Logf("\n--- Entering main() ---")
			} else if inMain {
				break // Exited main
			}
		}
		if inst.opcode == OpFunctionEnd && inMain {
			t.Logf("--- End main() ---")
			inMain = false
		}

		if !inMain {
			continue
		}

		// Log interesting instructions
		switch inst.opcode {
		case OpVariable:
			if inst.wordCount >= 4 {
				t.Logf("[%d] OpVariable %%%d (type=%%%d, storage=%d) %s",
					i, inst.words[2], inst.words[1], inst.words[3],
					nameOrEmpty(inst.words[2], names))
			}
		case OpStore:
			if inst.wordCount >= 3 {
				t.Logf("[%d] OpStore %%%d <- %%%d   %s <- %s",
					i, inst.words[1], inst.words[2],
					nameOrEmpty(inst.words[1], names), nameOrEmpty(inst.words[2], names))
			}
		case OpLoad:
			if inst.wordCount >= 4 {
				t.Logf("[%d] OpLoad %%%d = load(%%%d)  %s",
					i, inst.words[3], inst.words[3],
					nameOrEmpty(inst.words[3], names))
			}
		case OpFunctionCall:
			if inst.wordCount >= 4 {
				args := ""
				for j := 4; j < inst.wordCount; j++ {
					if j > 4 {
						args += ", "
					}
					args += idStr(inst.words[j], names)
				}
				t.Logf("[%d] *** OpFunctionCall %%%d = call %%%d(%s)  %s",
					i, inst.words[2], inst.words[3], args,
					nameOrEmpty(inst.words[3], names))
			}
		case OpLabel:
			t.Logf("[%d] OpLabel %%%d", i, inst.words[1])
		case OpReturn, OpReturnValue:
			t.Logf("[%d] OpReturn", i)
		case OpBranch:
			t.Logf("[%d] OpBranch -> %%%d", i, inst.words[1])
		case OpBranchConditional:
			t.Logf("[%d] OpBranchConditional %%%d -> %%%d / %%%d",
				i, inst.words[1], inst.words[2], inst.words[3])
		case OpISub:
			t.Logf("[%d] OpISub %%%d = %%%d - %%%d", i, inst.words[2], inst.words[3], inst.words[4])
		case OpIAdd:
			t.Logf("[%d] OpIAdd %%%d = %%%d + %%%d", i, inst.words[2], inst.words[3], inst.words[4])
		case OpConvertFToU:
			t.Logf("[%d] OpConvertFToU %%%d = u32(%%%d)", i, inst.words[2], inst.words[3])
		}
	}

	// Also show the span function
	t.Logf("\n--- span() function body ---")
	inSpan := false
	for i, inst := range instrs {
		if inst.opcode == OpFunction {
			if name, ok := names[inst.words[2]]; ok && name == "span" {
				inSpan = true
				t.Logf("[%d] OpFunction span %%%d", i, inst.words[2])
			}
		}
		if inst.opcode == OpFunctionEnd && inSpan {
			t.Logf("[%d] OpFunctionEnd", i)
			inSpan = false
		}
		if !inSpan {
			continue
		}
		memberNames := make(map[uint32]map[uint32]string)
		line := formatInstruction(inst.opcode, inst.words, names, memberNames)
		t.Logf("  [%d] %s", i, line)
	}
}

func nameOrEmpty(id uint32, names map[uint32]string) string {
	if name, ok := names[id]; ok {
		return "(" + name + ")"
	}
	return ""
}

// TestSpanIRDump dumps the naga IR for the span shader to see expression handles.
func TestSpanIRDump(t *testing.T) {
	lexer := wgsl.NewLexer(spanTestShader)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	parser := wgsl.NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower: %v", err)
	}

	// Dump functions
	for i := range module.Functions {
		fn := &module.Functions[i]
		t.Logf("Function[%d]: %s (args=%d, locals=%d, exprs=%d, stmts=%d)",
			i, fn.Name, len(fn.Arguments), len(fn.LocalVars),
			len(fn.Expressions), len(fn.Body))

		t.Logf("  Arguments:")
		for j, arg := range fn.Arguments {
			t.Logf("    [%d] %s", j, arg.Name)
		}

		t.Logf("  LocalVars:")
		for j, lv := range fn.LocalVars {
			initStr := "nil"
			if lv.Init != nil {
				initStr = dumpExprTree(fn.Expressions, *lv.Init, 0)
			}
			t.Logf("    [%d] %s init=%s", j, lv.Name, initStr)
		}

		t.Logf("  Statements:")
		dumpStatements(t, fn.Expressions, fn.Body, 2)
	}

	// Dump entry points
	for i, ep := range module.EntryPoints {
		fn := &module.Functions[ep.Function]
		t.Logf("EntryPoint[%d]: %s (func=%d, args=%d, locals=%d, exprs=%d, stmts=%d)",
			i, ep.Name, ep.Function, len(fn.Arguments), len(fn.LocalVars),
			len(fn.Expressions), len(fn.Body))

		t.Logf("  LocalVars:")
		for j, lv := range fn.LocalVars {
			initStr := "nil"
			if lv.Init != nil {
				initStr = dumpExprTree(fn.Expressions, *lv.Init, 0)
			}
			t.Logf("    [%d] %s init=%s", j, lv.Name, initStr)
		}

		t.Logf("  Statements (body):")
		dumpStatements(t, fn.Expressions, fn.Body, 2)
	}
}

func dumpExprTree(exprs []ir.Expression, handle ir.ExpressionHandle, depth int) string {
	if depth > 10 {
		return "..."
	}
	if int(handle) >= len(exprs) {
		return "INVALID"
	}
	expr := exprs[handle]
	prefix := ""
	for i := 0; i < depth; i++ {
		prefix += "  "
	}

	switch k := expr.Kind.(type) {
	case ir.Literal:
		return formatLiteral(k.Value)
	case ir.ExprLocalVariable:
		return fmt.Sprintf("LocalVar(%d)", k.Variable)
	case ir.ExprFunctionArgument:
		return fmt.Sprintf("FuncArg(%d)", k.Index)
	case ir.ExprCallResult:
		return fmt.Sprintf("CallResult(func=%d)", k.Function)
	case ir.ExprBinary:
		left := dumpExprTree(exprs, k.Left, depth+1)
		right := dumpExprTree(exprs, k.Right, depth+1)
		return fmt.Sprintf("Binary(%v, %s, %s)", k.Op, left, right)
	case ir.ExprAccessIndex:
		base := dumpExprTree(exprs, k.Base, depth+1)
		return fmt.Sprintf("AccessIndex(%s, %d)", base, k.Index)
	case ir.ExprMath:
		args := dumpExprTree(exprs, k.Arg, depth+1)
		if k.Arg1 != nil {
			args += ", " + dumpExprTree(exprs, *k.Arg1, depth+1)
		}
		if k.Arg2 != nil {
			args += ", " + dumpExprTree(exprs, *k.Arg2, depth+1)
		}
		return fmt.Sprintf("Math(%v, %s)", k.Fun, args)
	case ir.ExprAs:
		inner := dumpExprTree(exprs, k.Expr, depth+1)
		return fmt.Sprintf("As(%v, %s)", k.Kind, inner)
	case ir.ExprCompose:
		var parts []string
		for _, c := range k.Components {
			parts = append(parts, dumpExprTree(exprs, c, depth+1))
		}
		return fmt.Sprintf("Compose(%v)", parts)
	case ir.ExprSelect:
		cond := dumpExprTree(exprs, k.Condition, depth+1)
		accept := dumpExprTree(exprs, k.Accept, depth+1)
		reject := dumpExprTree(exprs, k.Reject, depth+1)
		return fmt.Sprintf("Select(%s, %s, %s)", cond, accept, reject)
	case ir.ExprGlobalVariable:
		return fmt.Sprintf("GlobalVar(%d)", k.Variable)
	case ir.ExprLoad:
		ptr := dumpExprTree(exprs, k.Pointer, depth+1)
		return fmt.Sprintf("Load(%s)", ptr)
	default:
		return fmt.Sprintf("<%T>(handle=%d)", k, handle)
	}
}

func formatLiteral(v ir.LiteralValue) string {
	switch val := v.(type) {
	case ir.LiteralF32:
		return fmt.Sprintf("%.4g", float32(val))
	case ir.LiteralU32:
		return fmt.Sprintf("%du", uint32(val))
	case ir.LiteralI32:
		return fmt.Sprintf("%di", int32(val))
	case ir.LiteralBool:
		return fmt.Sprintf("%v", bool(val))
	default:
		return fmt.Sprintf("%v", v)
	}
}

func dumpStatements(t *testing.T, exprs []ir.Expression, stmts []ir.Statement, indent int) {
	prefix := ""
	for i := 0; i < indent; i++ {
		prefix += "  "
	}
	for _, stmt := range stmts {
		switch s := stmt.Kind.(type) {
		case ir.StmtCall:
			args := ""
			for j, a := range s.Arguments {
				if j > 0 {
					args += ", "
				}
				args += fmt.Sprintf("expr[%d]=%s", a, dumpExprTree(exprs, a, 0))
			}
			result := "void"
			if s.Result != nil {
				result = fmt.Sprintf("expr[%d]", *s.Result)
			}
			t.Logf("%sCall func[%d](%s) -> %s", prefix, s.Function, args, result)
		case ir.StmtStore:
			ptr := dumpExprTree(exprs, s.Pointer, 0)
			val := dumpExprTree(exprs, s.Value, 0)
			t.Logf("%sStore %s = %s", prefix, ptr, val)
		case ir.StmtReturn:
			if s.Value != nil {
				t.Logf("%sReturn %s", prefix, dumpExprTree(exprs, *s.Value, 0))
			} else {
				t.Logf("%sReturn", prefix)
			}
		case ir.StmtEmit:
			for h := s.Range.Start; h < s.Range.End; h++ {
				t.Logf("%sEmit expr[%d] = %s", prefix, h, dumpExprTree(exprs, h, 0))
			}
		default:
			t.Logf("%s%T", prefix, s)
		}
	}
}
