package snapshot_test

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// TestIRDump parses ALL WGSL shaders in testdata/in/ and dumps the resulting
// ir.Module to a human-readable text format suitable for diffing against Rust
// naga's IR.
//
// Usage:
//
//	go test -run TestIRDump -v ./snapshot/
//	go test -run TestIRDump/globals -v ./snapshot/   # single shader
//
// Each dump is saved to tmp/go_ir_<shader>.txt when the tmp/ directory exists.
// A combined dump is saved to tmp/go_ir_all.txt.
func TestIRDump(t *testing.T) {
	entries, err := filepath.Glob(filepath.Join("testdata", "in", "*.wgsl"))
	if err != nil {
		t.Fatalf("glob wgsl files: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no .wgsl files found in testdata/in/")
	}

	sort.Strings(entries)

	outDir := filepath.Join("..", "tmp")
	hasOutDir := false
	if info, statErr := os.Stat(outDir); statErr == nil && info.IsDir() {
		hasOutDir = true
	}

	var passed, failed int
	var failedNames []string
	var combined strings.Builder

	for _, entry := range entries {
		base := filepath.Base(entry)
		name := strings.TrimSuffix(base, ".wgsl")
		t.Run(name, func(t *testing.T) {
			dumpShaderIR(t, name)
		})

		// Also build a combined dump (best-effort, skip failures)
		if hasOutDir {
			src, readErr := os.ReadFile(entry)
			if readErr != nil {
				failed++
				failedNames = append(failedNames, name+" (read)")
				continue
			}
			module, compileErr := tryCompileToIR(string(src))
			if compileErr != nil {
				failed++
				failedNames = append(failedNames, name+" ("+compileErr.Error()+")")
				continue
			}
			var sb strings.Builder
			dumpModule(&sb, module)
			passed++
			fmt.Fprintf(&combined, "=== SHADER: %s ===\n", name)
			combined.WriteString(sb.String())
			combined.WriteString("\n")
		}
	}

	if hasOutDir {
		combinedPath := filepath.Join(outDir, "go_ir_all.txt")
		if writeErr := os.WriteFile(combinedPath, []byte(combined.String()), 0o644); writeErr != nil {
			t.Errorf("write combined dump: %v", writeErr)
		}
		t.Logf("IR dump combined: %d passed, %d failed out of %d total", passed, failed, len(entries))
		if len(failedNames) > 0 {
			t.Logf("Failed shaders:\n  %s", strings.Join(failedNames, "\n  "))
		}
	}
}

// tryCompileToIR attempts to compile WGSL source to IR, returning an error
// instead of calling t.Fatal on failure.
func tryCompileToIR(source string) (*ir.Module, error) {
	lexer := wgsl.NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		return nil, fmt.Errorf("tokenize: %w", err)
	}

	parser := wgsl.NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	module, err := wgsl.LowerWithSource(ast, source)
	if err != nil {
		return nil, fmt.Errorf("lower: %w", err)
	}

	return module, nil
}

// dumpShaderIR is the reusable core: parse a single WGSL shader by name and
// dump the full ir.Module in a diffable text format.
func dumpShaderIR(t *testing.T, shaderName string) {
	t.Helper()

	src, err := os.ReadFile(filepath.Join("testdata", "in", shaderName+".wgsl"))
	if err != nil {
		t.Fatalf("read shader %q: %v", shaderName, err)
	}

	module := compileToIR(t, shaderName, string(src))

	var sb strings.Builder
	dumpModule(&sb, module)
	dump := sb.String()

	// Save to file for easy diffing (dump is NOT logged to avoid flooding CI output).
	outDir := filepath.Join("..", "tmp")
	if info, statErr := os.Stat(outDir); statErr == nil && info.IsDir() {
		outPath := filepath.Join(outDir, "go_ir_"+shaderName+".txt")
		if writeErr := os.WriteFile(outPath, []byte(dump), 0o644); writeErr != nil {
			t.Errorf("write dump file: %v", writeErr)
		} else {
			t.Logf("dump saved to %s", outPath)
		}
	}
}

// ---------------------------------------------------------------------------
// Module dump
// ---------------------------------------------------------------------------

func dumpModule(sb *strings.Builder, m *ir.Module) {
	dumpTypes(sb, m)
	dumpConstants(sb, m)
	dumpOverrides(sb, m)
	dumpGlobals(sb, m)
	dumpFunctions(sb, m)
	dumpEntryPoints(sb, m)
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

func dumpTypes(sb *strings.Builder, m *ir.Module) {
	sb.WriteString("=== TYPES ===\n")
	for i := range m.Types {
		ty := &m.Types[i]
		fmt.Fprintf(sb, "[%d] ", i)
		dumpTypeInner(sb, ty.Inner, ty.Name)
		sb.WriteByte('\n')
	}
	sb.WriteByte('\n')
}

func dumpTypeInner(sb *strings.Builder, inner ir.TypeInner, name string) {
	switch t := inner.(type) {
	case ir.ScalarType:
		fmt.Fprintf(sb, "Scalar(%s, %d)", scalarKindStr(t.Kind), t.Width)
	case ir.VectorType:
		fmt.Fprintf(sb, "Vector { size: Vec%d, scalar: %s(%d) }", t.Size, scalarKindStr(t.Scalar.Kind), t.Scalar.Width)
	case ir.MatrixType:
		fmt.Fprintf(sb, "Matrix { columns: Vec%d, rows: Vec%d, scalar: %s(%d) }",
			t.Columns, t.Rows, scalarKindStr(t.Scalar.Kind), t.Scalar.Width)
	case ir.ArrayType:
		sb.WriteString("Array { base: ")
		fmt.Fprintf(sb, "[%d]", t.Base)
		sb.WriteString(", size: ")
		if t.Size.Constant != nil {
			fmt.Fprintf(sb, "Constant(%d)", *t.Size.Constant)
		} else {
			sb.WriteString("Dynamic")
		}
		fmt.Fprintf(sb, ", stride: %d }", t.Stride)
	case ir.StructType:
		fmt.Fprintf(sb, "Struct %q { members: [", name)
		for k := range t.Members {
			mem := &t.Members[k]
			if k > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(sb, "{ name: %q, type: [%d], offset: %d", mem.Name, mem.Type, mem.Offset)
			if mem.Binding != nil {
				fmt.Fprintf(sb, ", binding: %s", formatBinding(*mem.Binding))
			}
			sb.WriteByte('}')
		}
		fmt.Fprintf(sb, "], span: %d }", t.Span)
	case ir.PointerType:
		fmt.Fprintf(sb, "Pointer { base: [%d], space: %s }", t.Base, addressSpaceStr(t.Space))
	case ir.AtomicType:
		fmt.Fprintf(sb, "Atomic { base: %s(%d) }", scalarKindStr(t.Scalar.Kind), t.Scalar.Width)
	case ir.SamplerType:
		if t.Comparison {
			sb.WriteString("Sampler(Comparison)")
		} else {
			sb.WriteString("Sampler(Filtering)")
		}
	case ir.ImageType:
		fmt.Fprintf(sb, "Image { dim: %s, arrayed: %v, class: %s", imageDimStr(t.Dim), t.Arrayed, imageClassStr(t.Class))
		if t.Multisampled {
			sb.WriteString(", multisampled: true")
		}
		sb.WriteByte('}')
	case ir.BindingArrayType:
		fmt.Fprintf(sb, "BindingArray { base: [%d]", t.Base)
		if t.Size != nil {
			fmt.Fprintf(sb, ", size: %d", *t.Size)
		}
		sb.WriteByte('}')
	case ir.AccelerationStructureType:
		sb.WriteString("AccelerationStructure")
	case ir.RayQueryType:
		sb.WriteString("RayQuery")
	default:
		fmt.Fprintf(sb, "Unknown(%T)", inner)
	}
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

func dumpConstants(sb *strings.Builder, m *ir.Module) {
	sb.WriteString("=== CONSTANTS ===\n")
	if len(m.Constants) == 0 {
		sb.WriteString("(none)\n")
	}
	for i := range m.Constants {
		c := &m.Constants[i]
		name := c.Name
		if name == "" {
			name = "_"
		}
		fmt.Fprintf(sb, "[%d] %q type=[%d] init=%s\n", i, name, c.Type, formatConstantValue(c.Value))
	}
	sb.WriteByte('\n')
}

func formatConstantValue(v ir.ConstantValue) string {
	switch cv := v.(type) {
	case ir.ScalarValue:
		return fmt.Sprintf("Literal(%s)", formatScalarLiteral(cv))
	case ir.CompositeValue:
		parts := make([]string, len(cv.Components))
		for i, h := range cv.Components {
			parts[i] = fmt.Sprintf("[%d]", h)
		}
		return fmt.Sprintf("Composite(%s)", strings.Join(parts, ", "))
	default:
		return fmt.Sprintf("Unknown(%T)", v)
	}
}

func formatScalarLiteral(sv ir.ScalarValue) string {
	switch sv.Kind {
	case ir.ScalarBool:
		if sv.Bits != 0 {
			return "Bool(true)"
		}
		return "Bool(false)"
	case ir.ScalarFloat:
		f := math.Float64frombits(sv.Bits)
		return fmt.Sprintf("Float(%g)", f)
	case ir.ScalarSint:
		return fmt.Sprintf("Sint(%d)", int64(sv.Bits))
	case ir.ScalarUint:
		return fmt.Sprintf("Uint(%d)", sv.Bits)
	default:
		return fmt.Sprintf("Unknown(bits=%d)", sv.Bits)
	}
}

// ---------------------------------------------------------------------------
// Overrides
// ---------------------------------------------------------------------------

func dumpOverrides(sb *strings.Builder, m *ir.Module) {
	sb.WriteString("=== OVERRIDES ===\n")
	if len(m.Overrides) == 0 {
		sb.WriteString("(none)\n\n")
		return
	}
	for i, o := range m.Overrides {
		idStr := "None"
		if o.ID != nil {
			idStr = fmt.Sprintf("Some(%d)", *o.ID)
		}
		initStr := "None"
		if o.Init != nil {
			initStr = fmt.Sprintf("Some(%d)", *o.Init)
		}
		fmt.Fprintf(sb, "  [%d] name=%q id=%s ty=%d init=%s\n", i, o.Name, idStr, o.Ty, initStr)
	}
	sb.WriteString("\n")
}

// ---------------------------------------------------------------------------
// Globals
// ---------------------------------------------------------------------------

func dumpGlobals(sb *strings.Builder, m *ir.Module) {
	sb.WriteString("=== GLOBALS ===\n")
	if len(m.GlobalVariables) == 0 {
		sb.WriteString("(none)\n")
	}
	for i := range m.GlobalVariables {
		g := &m.GlobalVariables[i]
		spaceStr := addressSpaceStr(g.Space)
		if g.Space == ir.SpaceStorage {
			switch g.Access {
			case ir.StorageRead:
				spaceStr = "Storage(LOAD)"
			case ir.StorageReadWrite:
				spaceStr = "Storage(LOAD|STORE)"
			}
		}
		fmt.Fprintf(sb, "[%d] %q type=[%d] space=%s", i, g.Name, g.Type, spaceStr)
		if g.Binding != nil {
			fmt.Fprintf(sb, " binding=(%d,%d)", g.Binding.Group, g.Binding.Binding)
		} else {
			sb.WriteString(" binding=None")
		}
		if g.Init != nil {
			fmt.Fprintf(sb, " init=[%d]", *g.Init)
		}
		sb.WriteByte('\n')
	}
	sb.WriteByte('\n')
}

// ---------------------------------------------------------------------------
// Functions
// ---------------------------------------------------------------------------

func dumpFunctions(sb *strings.Builder, m *ir.Module) {
	sb.WriteString("=== FUNCTIONS ===\n")
	for i := range m.Functions {
		fn := &m.Functions[i]
		fmt.Fprintf(sb, "[%d] %q\n", i, fn.Name)

		// Arguments
		sb.WriteString("  args: [")
		for j := range fn.Arguments {
			arg := &fn.Arguments[j]
			if j > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(sb, "{ name: %q, type: [%d]", arg.Name, arg.Type)
			if arg.Binding != nil {
				fmt.Fprintf(sb, ", binding: %s", formatBinding(*arg.Binding))
			} else {
				sb.WriteString(", binding: None")
			}
			sb.WriteByte('}')
		}
		sb.WriteString("]\n")

		// Result
		if fn.Result != nil {
			fmt.Fprintf(sb, "  result: type=[%d]", fn.Result.Type)
			if fn.Result.Binding != nil {
				fmt.Fprintf(sb, " binding=%s", formatBinding(*fn.Result.Binding))
			}
			sb.WriteByte('\n')
		} else {
			sb.WriteString("  result: None\n")
		}

		// Locals
		fmt.Fprintf(sb, "  locals: %d\n", len(fn.LocalVars))
		for j := range fn.LocalVars {
			lv := &fn.LocalVars[j]
			fmt.Fprintf(sb, "    [%d] %q type=[%d]", j, lv.Name, lv.Type)
			if lv.Init != nil {
				fmt.Fprintf(sb, " init=[%d]", *lv.Init)
			}
			sb.WriteByte('\n')
		}

		// Expressions
		fmt.Fprintf(sb, "  expressions: %d\n", len(fn.Expressions))
		for j := range fn.Expressions {
			expr := &fn.Expressions[j]
			fmt.Fprintf(sb, "    [%d] %s\n", j, formatExpression(expr.Kind))
		}

		// Named expressions
		if len(fn.NamedExpressions) > 0 {
			sb.WriteString("  named_expressions:\n")
			// Sort by handle for deterministic output
			handles := make([]ir.ExpressionHandle, 0, len(fn.NamedExpressions))
			for h := range fn.NamedExpressions {
				handles = append(handles, h)
			}
			sort.Slice(handles, func(a, b int) bool { return handles[a] < handles[b] })
			for _, h := range handles {
				fmt.Fprintf(sb, "    [%d] = %q\n", h, fn.NamedExpressions[h])
			}
		} else {
			sb.WriteString("  named_expressions: {}\n")
		}

		// Body
		fmt.Fprintf(sb, "  body: %d statements\n", len(fn.Body))
		dumpStatements(sb, fn.Body, 4)
		sb.WriteByte('\n')
	}
}

// ---------------------------------------------------------------------------
// Expressions
// ---------------------------------------------------------------------------

func formatExpression(kind ir.ExpressionKind) string {
	switch e := kind.(type) {
	case ir.Literal:
		return fmt.Sprintf("Literal(%s)", formatLiteralValue(e.Value))
	case ir.ExprConstant:
		return fmt.Sprintf("Constant([%d])", e.Constant)
	case ir.ExprZeroValue:
		return fmt.Sprintf("ZeroValue(type=[%d])", e.Type)
	case ir.ExprCompose:
		parts := make([]string, len(e.Components))
		for i, h := range e.Components {
			parts[i] = fmt.Sprintf("[%d]", h)
		}
		return fmt.Sprintf("Compose { type: [%d], components: [%s] }", e.Type, strings.Join(parts, ", "))
	case ir.ExprAccess:
		return fmt.Sprintf("Access { base: [%d], index: [%d] }", e.Base, e.Index)
	case ir.ExprAccessIndex:
		return fmt.Sprintf("AccessIndex { base: [%d], index: %d }", e.Base, e.Index)
	case ir.ExprSplat:
		return fmt.Sprintf("Splat { size: Vec%d, value: [%d] }", e.Size, e.Value)
	case ir.ExprSwizzle:
		return fmt.Sprintf("Swizzle { size: Vec%d, vector: [%d], pattern: [%d, %d, %d, %d] }",
			e.Size, e.Vector, e.Pattern[0], e.Pattern[1], e.Pattern[2], e.Pattern[3])
	case ir.ExprFunctionArgument:
		return fmt.Sprintf("FunctionArgument(%d)", e.Index)
	case ir.ExprGlobalVariable:
		return fmt.Sprintf("GlobalVariable([%d])", e.Variable)
	case ir.ExprLocalVariable:
		return fmt.Sprintf("LocalVariable([%d])", e.Variable)
	case ir.ExprLoad:
		return fmt.Sprintf("Load { pointer: [%d] }", e.Pointer)
	case ir.ExprBinary:
		return fmt.Sprintf("Binary { op: %s, left: [%d], right: [%d] }", binaryOpStr(e.Op), e.Left, e.Right)
	case ir.ExprUnary:
		return fmt.Sprintf("Unary { op: %s, expr: [%d] }", unaryOpStr(e.Op), e.Expr)
	case ir.ExprSelect:
		return fmt.Sprintf("Select { condition: [%d], accept: [%d], reject: [%d] }", e.Condition, e.Accept, e.Reject)
	case ir.ExprMath:
		args := fmt.Sprintf("[%d]", e.Arg)
		if e.Arg1 != nil {
			args += fmt.Sprintf(", [%d]", *e.Arg1)
		}
		if e.Arg2 != nil {
			args += fmt.Sprintf(", [%d]", *e.Arg2)
		}
		if e.Arg3 != nil {
			args += fmt.Sprintf(", [%d]", *e.Arg3)
		}
		return fmt.Sprintf("Math { fun: %s, args: %s }", mathFuncStr(e.Fun), args)
	case ir.ExprAs:
		if e.Convert != nil {
			return fmt.Sprintf("As { expr: [%d], kind: %s, convert: %d }", e.Expr, scalarKindStr(e.Kind), *e.Convert)
		}
		return fmt.Sprintf("As { expr: [%d], kind: %s, convert: None }", e.Expr, scalarKindStr(e.Kind))
	case ir.ExprCallResult:
		return fmt.Sprintf("CallResult(function=[%d])", e.Function)
	case ir.ExprArrayLength:
		return fmt.Sprintf("ArrayLength(expr=[%d])", e.Array)
	case ir.ExprAtomicResult:
		return "AtomicResult"
	case ir.ExprWorkGroupUniformLoadResult:
		return "WorkGroupUniformLoadResult"
	case ir.ExprImageSample:
		return fmt.Sprintf("ImageSample { image: [%d], sampler: [%d], coordinate: [%d] }", e.Image, e.Sampler, e.Coordinate)
	case ir.ExprImageLoad:
		return fmt.Sprintf("ImageLoad { image: [%d], coordinate: [%d] }", e.Image, e.Coordinate)
	case ir.ExprImageQuery:
		return fmt.Sprintf("ImageQuery { image: [%d], query: %T }", e.Image, e.Query)
	case ir.ExprDerivative:
		return fmt.Sprintf("Derivative { axis: %d, control: %d, expr: [%d] }", e.Axis, e.Control, e.Expr)
	case ir.ExprRelational:
		return fmt.Sprintf("Relational { fun: %d, arg: [%d] }", e.Fun, e.Argument)
	case ir.ExprRayQueryGetIntersection:
		return fmt.Sprintf("RayQueryGetIntersection { query: [%d], committed: %v }", e.Query, e.Committed)
	case ir.ExprRayQueryProceedResult:
		return "RayQueryProceedResult"
	case ir.ExprSubgroupBallotResult:
		return "SubgroupBallotResult"
	case ir.ExprSubgroupOperationResult:
		return fmt.Sprintf("SubgroupOperationResult { type: [%d] }", e.Type)
	default:
		return fmt.Sprintf("Unknown(%T)", kind)
	}
}

func formatLiteralValue(v ir.LiteralValue) string {
	switch lv := v.(type) {
	case ir.LiteralBool:
		return fmt.Sprintf("Bool(%v)", bool(lv))
	case ir.LiteralF32:
		return fmt.Sprintf("F32(%v)", float32(lv))
	case ir.LiteralF64:
		return fmt.Sprintf("F64(%v)", float64(lv))
	case ir.LiteralI32:
		return fmt.Sprintf("I32(%d)", int32(lv))
	case ir.LiteralU32:
		return fmt.Sprintf("U32(%d)", uint32(lv))
	case ir.LiteralI64:
		return fmt.Sprintf("I64(%d)", int64(lv))
	case ir.LiteralU64:
		return fmt.Sprintf("U64(%d)", uint64(lv))
	case ir.LiteralAbstractInt:
		return fmt.Sprintf("AbstractInt(%d)", int64(lv))
	case ir.LiteralAbstractFloat:
		return fmt.Sprintf("AbstractFloat(%v)", float64(lv))
	default:
		return fmt.Sprintf("Unknown(%T)", v)
	}
}

// ---------------------------------------------------------------------------
// Statements
// ---------------------------------------------------------------------------

func dumpStatements(sb *strings.Builder, stmts []ir.Statement, indent int) {
	pad := strings.Repeat(" ", indent)
	for i := range stmts {
		dumpStatement(sb, stmts[i].Kind, pad, indent)
	}
}

func dumpStatement(sb *strings.Builder, kind ir.StatementKind, pad string, indent int) {
	switch s := kind.(type) {
	case ir.StmtEmit:
		fmt.Fprintf(sb, "%sEmit(%d..%d)\n", pad, s.Range.Start, s.Range.End)
	case ir.StmtStore:
		fmt.Fprintf(sb, "%sStore { pointer: [%d], value: [%d] }\n", pad, s.Pointer, s.Value)
	case ir.StmtCall:
		args := make([]string, len(s.Arguments))
		for j, a := range s.Arguments {
			args[j] = fmt.Sprintf("[%d]", a)
		}
		result := "None"
		if s.Result != nil {
			result = fmt.Sprintf("[%d]", *s.Result)
		}
		fmt.Fprintf(sb, "%sCall { function: [%d], args: [%s], result: %s }\n", pad, s.Function, strings.Join(args, ", "), result)
	case ir.StmtIf:
		fmt.Fprintf(sb, "%sIf { condition: [%d] }\n", pad, s.Condition)
		fmt.Fprintf(sb, "%s  accept:\n", pad)
		dumpStatements(sb, s.Accept, indent+4)
		fmt.Fprintf(sb, "%s  reject:\n", pad)
		dumpStatements(sb, s.Reject, indent+4)
	case ir.StmtSwitch:
		fmt.Fprintf(sb, "%sSwitch { selector: [%d] }\n", pad, s.Selector)
		for ci := range s.Cases {
			c := &s.Cases[ci]
			fmt.Fprintf(sb, "%s  case %s:\n", pad, formatSwitchValue(c.Value))
			dumpStatements(sb, c.Body, indent+4)
		}
	case ir.StmtLoop:
		sb.WriteString(pad + "Loop\n")
		fmt.Fprintf(sb, "%s  body:\n", pad)
		dumpStatements(sb, s.Body, indent+4)
		if len(s.Continuing) > 0 {
			fmt.Fprintf(sb, "%s  continuing:\n", pad)
			dumpStatements(sb, s.Continuing, indent+4)
		}
		if s.BreakIf != nil {
			fmt.Fprintf(sb, "%s  break_if: [%d]\n", pad, *s.BreakIf)
		}
	case ir.StmtReturn:
		if s.Value != nil {
			fmt.Fprintf(sb, "%sReturn { value: [%d] }\n", pad, *s.Value)
		} else {
			fmt.Fprintf(sb, "%sReturn\n", pad)
		}
	case ir.StmtBreak:
		fmt.Fprintf(sb, "%sBreak\n", pad)
	case ir.StmtContinue:
		fmt.Fprintf(sb, "%sContinue\n", pad)
	case ir.StmtKill:
		fmt.Fprintf(sb, "%sKill\n", pad)
	case ir.StmtBarrier:
		fmt.Fprintf(sb, "%sBarrier(flags=%d)\n", pad, s.Flags)
	case ir.StmtAtomic:
		result := "None"
		if s.Result != nil {
			result = fmt.Sprintf("[%d]", *s.Result)
		}
		fmt.Fprintf(sb, "%sAtomic { pointer: [%d], fun: %s, value: [%d], result: %s }\n",
			pad, s.Pointer, formatAtomicFun(s.Fun), s.Value, result)
	case ir.StmtImageStore:
		fmt.Fprintf(sb, "%sImageStore { image: [%d], coordinate: [%d], value: [%d] }\n",
			pad, s.Image, s.Coordinate, s.Value)
	case ir.StmtImageAtomic:
		fmt.Fprintf(sb, "%sImageAtomic { image: [%d], coordinate: [%d], value: [%d] }\n",
			pad, s.Image, s.Coordinate, s.Value)
	case ir.StmtBlock:
		fmt.Fprintf(sb, "%sBlock\n", pad)
		dumpStatements(sb, s.Block, indent+2)
	case ir.StmtRayQuery:
		fmt.Fprintf(sb, "%sRayQuery { query: [%d], fun: %T }\n", pad, s.Query, s.Fun)
	case ir.StmtWorkGroupUniformLoad:
		fmt.Fprintf(sb, "%sWorkGroupUniformLoad { pointer: [%d], result: [%d] }\n", pad, s.Pointer, s.Result)
	case ir.StmtSubgroupBallot:
		fmt.Fprintf(sb, "%sSubgroupBallot { result: [%d] }\n", pad, s.Result)
	case ir.StmtSubgroupCollectiveOperation:
		fmt.Fprintf(sb, "%sSubgroupCollectiveOperation { op: %d, collective_op: %d, arg: [%d], result: [%d] }\n",
			pad, s.Op, s.CollectiveOp, s.Argument, s.Result)
	case ir.StmtSubgroupGather:
		fmt.Fprintf(sb, "%sSubgroupGather { mode: %T, arg: [%d], result: [%d] }\n",
			pad, s.Mode, s.Argument, s.Result)
	default:
		fmt.Fprintf(sb, "%sUnknown(%T)\n", pad, kind)
	}
}

// ---------------------------------------------------------------------------
// Entry Points
// ---------------------------------------------------------------------------

func dumpEntryPoints(sb *strings.Builder, m *ir.Module) {
	sb.WriteString("=== ENTRY POINTS ===\n")
	for i := range m.EntryPoints {
		ep := &m.EntryPoints[i]
		fn := &ep.Function
		fmt.Fprintf(sb, "[%d] %q stage=%s function={name=%q, args=%d, exprs=%d, stmts=%d, locals=%d}",
			i, ep.Name, shaderStageStr(ep.Stage),
			fn.Name, len(fn.Arguments), len(fn.Expressions), len(fn.Body), len(fn.LocalVars))
		if ep.Stage == ir.StageCompute {
			fmt.Fprintf(sb, " workgroup_size=(%d,%d,%d)", ep.Workgroup[0], ep.Workgroup[1], ep.Workgroup[2])
		}
		sb.WriteByte('\n')
	}
}

// ---------------------------------------------------------------------------
// String helpers
// ---------------------------------------------------------------------------

func scalarKindStr(k ir.ScalarKind) string {
	switch k {
	case ir.ScalarSint:
		return "Sint"
	case ir.ScalarUint:
		return "Uint"
	case ir.ScalarFloat:
		return "Float"
	case ir.ScalarBool:
		return "Bool"
	default:
		return fmt.Sprintf("ScalarKind(%d)", k)
	}
}

func addressSpaceStr(s ir.AddressSpace) string {
	switch s {
	case ir.SpaceFunction:
		return "Function"
	case ir.SpacePrivate:
		return "Private"
	case ir.SpaceWorkGroup:
		return "WorkGroup"
	case ir.SpaceUniform:
		return "Uniform"
	case ir.SpaceStorage:
		return "Storage"
	case ir.SpacePushConstant:
		return "PushConstant"
	case ir.SpaceHandle:
		return "Handle"
	default:
		return fmt.Sprintf("AddressSpace(%d)", s)
	}
}

func shaderStageStr(s ir.ShaderStage) string {
	switch s {
	case ir.StageVertex:
		return "Vertex"
	case ir.StageTask:
		return "Task"
	case ir.StageMesh:
		return "Mesh"
	case ir.StageFragment:
		return "Fragment"
	case ir.StageCompute:
		return "Compute"
	default:
		return fmt.Sprintf("Stage(%d)", s)
	}
}

func binaryOpStr(op ir.BinaryOperator) string {
	switch op {
	case ir.BinaryAdd:
		return "Add"
	case ir.BinarySubtract:
		return "Subtract"
	case ir.BinaryMultiply:
		return "Multiply"
	case ir.BinaryDivide:
		return "Divide"
	case ir.BinaryModulo:
		return "Modulo"
	case ir.BinaryEqual:
		return "Equal"
	case ir.BinaryNotEqual:
		return "NotEqual"
	case ir.BinaryLess:
		return "Less"
	case ir.BinaryLessEqual:
		return "LessEqual"
	case ir.BinaryGreater:
		return "Greater"
	case ir.BinaryGreaterEqual:
		return "GreaterEqual"
	case ir.BinaryAnd:
		return "And"
	case ir.BinaryExclusiveOr:
		return "ExclusiveOr"
	case ir.BinaryInclusiveOr:
		return "InclusiveOr"
	case ir.BinaryLogicalAnd:
		return "LogicalAnd"
	case ir.BinaryLogicalOr:
		return "LogicalOr"
	case ir.BinaryShiftLeft:
		return "ShiftLeft"
	case ir.BinaryShiftRight:
		return "ShiftRight"
	default:
		return fmt.Sprintf("BinaryOp(%d)", op)
	}
}

func unaryOpStr(op ir.UnaryOperator) string {
	switch op {
	case ir.UnaryNegate:
		return "Negate"
	case ir.UnaryLogicalNot:
		return "LogicalNot"
	case ir.UnaryBitwiseNot:
		return "BitwiseNot"
	default:
		return fmt.Sprintf("UnaryOp(%d)", op)
	}
}

func mathFuncStr(f ir.MathFunction) string {
	names := map[ir.MathFunction]string{
		ir.MathAbs: "Abs", ir.MathMin: "Min", ir.MathMax: "Max",
		ir.MathClamp: "Clamp", ir.MathSaturate: "Saturate",
		ir.MathCos: "Cos", ir.MathCosh: "Cosh", ir.MathSin: "Sin",
		ir.MathSinh: "Sinh", ir.MathTan: "Tan", ir.MathTanh: "Tanh",
		ir.MathAcos: "Acos", ir.MathAsin: "Asin", ir.MathAtan: "Atan",
		ir.MathAtan2: "Atan2", ir.MathAsinh: "Asinh", ir.MathAcosh: "Acosh",
		ir.MathAtanh: "Atanh", ir.MathRadians: "Radians", ir.MathDegrees: "Degrees",
		ir.MathCeil: "Ceil", ir.MathFloor: "Floor", ir.MathRound: "Round",
		ir.MathFract: "Fract", ir.MathTrunc: "Trunc", ir.MathModf: "Modf",
		ir.MathFrexp: "Frexp", ir.MathLdexp: "Ldexp",
		ir.MathExp: "Exp", ir.MathExp2: "Exp2", ir.MathLog: "Log",
		ir.MathLog2: "Log2", ir.MathPow: "Pow",
		ir.MathDot: "Dot", ir.MathOuter: "Outer", ir.MathCross: "Cross",
		ir.MathDistance: "Distance", ir.MathLength: "Length",
		ir.MathNormalize: "Normalize", ir.MathFaceForward: "FaceForward",
		ir.MathReflect: "Reflect", ir.MathRefract: "Refract",
		ir.MathSign: "Sign", ir.MathFma: "Fma", ir.MathMix: "Mix",
		ir.MathStep: "Step", ir.MathSmoothStep: "SmoothStep",
		ir.MathSqrt: "Sqrt", ir.MathInverseSqrt: "InverseSqrt",
		ir.MathInverse: "Inverse", ir.MathTranspose: "Transpose",
		ir.MathDeterminant: "Determinant", ir.MathQuantizeF16: "QuantizeToF16",
		ir.MathCountTrailingZeros: "CountTrailingZeros",
		ir.MathCountLeadingZeros:  "CountLeadingZeros",
		ir.MathCountOneBits:       "CountOneBits", ir.MathReverseBits: "ReverseBits",
		ir.MathExtractBits: "ExtractBits", ir.MathInsertBits: "InsertBits",
		ir.MathFirstTrailingBit: "FirstTrailingBit",
		ir.MathFirstLeadingBit:  "FirstLeadingBit",
		ir.MathPack4x8snorm:     "Pack4x8snorm", ir.MathPack4x8unorm: "Pack4x8unorm",
		ir.MathPack2x16snorm: "Pack2x16snorm", ir.MathPack2x16unorm: "Pack2x16unorm",
		ir.MathPack2x16float:  "Pack2x16float",
		ir.MathUnpack4x8snorm: "Unpack4x8snorm", ir.MathUnpack4x8unorm: "Unpack4x8unorm",
		ir.MathUnpack2x16snorm: "Unpack2x16snorm", ir.MathUnpack2x16unorm: "Unpack2x16unorm",
		ir.MathUnpack2x16float: "Unpack2x16float",
		ir.MathDot4I8Packed:    "Dot4I8Packed", ir.MathDot4U8Packed: "Dot4U8Packed",
		ir.MathPack4xI8: "Pack4xI8", ir.MathPack4xU8: "Pack4xU8",
		ir.MathPack4xI8Clamp: "Pack4xI8Clamp", ir.MathPack4xU8Clamp: "Pack4xU8Clamp",
		ir.MathUnpack4xI8: "Unpack4xI8", ir.MathUnpack4xU8: "Unpack4xU8",
	}
	if name, ok := names[f]; ok {
		return name
	}
	return fmt.Sprintf("MathFunction(%d)", f)
}

func imageDimStr(d ir.ImageDimension) string {
	switch d {
	case ir.Dim1D:
		return "D1"
	case ir.Dim2D:
		return "D2"
	case ir.Dim3D:
		return "D3"
	case ir.DimCube:
		return "Cube"
	default:
		return fmt.Sprintf("Dim(%d)", d)
	}
}

func imageClassStr(c ir.ImageClass) string {
	switch c {
	case ir.ImageClassSampled:
		return "Sampled"
	case ir.ImageClassDepth:
		return "Depth"
	case ir.ImageClassStorage:
		return "Storage"
	default:
		return fmt.Sprintf("ImageClass(%d)", c)
	}
}

func formatBinding(b ir.Binding) string {
	switch v := b.(type) {
	case ir.BuiltinBinding:
		return fmt.Sprintf("Builtin(%d)", v.Builtin)
	case ir.LocationBinding:
		s := fmt.Sprintf("Location(%d)", v.Location)
		if v.Interpolation != nil {
			s += fmt.Sprintf(" interp=%d/%d", v.Interpolation.Kind, v.Interpolation.Sampling)
		}
		return s
	default:
		return fmt.Sprintf("Binding(%T)", b)
	}
}

func formatSwitchValue(v ir.SwitchValue) string {
	switch sv := v.(type) {
	case ir.SwitchValueI32:
		return fmt.Sprintf("I32(%d)", int32(sv))
	case ir.SwitchValueU32:
		return fmt.Sprintf("U32(%d)", uint32(sv))
	case ir.SwitchValueDefault:
		return "Default"
	default:
		return fmt.Sprintf("SwitchValue(%T)", v)
	}
}

func formatAtomicFun(f ir.AtomicFunction) string {
	switch f.(type) {
	case ir.AtomicAdd:
		return "Add"
	case ir.AtomicSubtract:
		return "Subtract"
	case ir.AtomicAnd:
		return "And"
	case ir.AtomicExclusiveOr:
		return "ExclusiveOr"
	case ir.AtomicInclusiveOr:
		return "InclusiveOr"
	case ir.AtomicMin:
		return "Min"
	case ir.AtomicMax:
		return "Max"
	case ir.AtomicExchange:
		return "Exchange"
	case ir.AtomicStore:
		return "Store"
	case ir.AtomicLoad:
		return "Load"
	default:
		return fmt.Sprintf("AtomicFun(%T)", f)
	}
}
