package msl

import (
	"fmt"
	"strings"

	"github.com/gogpu/naga/ir"
)

// nameKey identifies an IR entity for name lookup.
type nameKey struct {
	kind    nameKeyKind
	handle1 uint32
	handle2 uint32
}

type nameKeyKind uint8

const (
	nameKeyType nameKeyKind = iota
	nameKeyStructMember
	nameKeyConstant
	nameKeyGlobalVariable
	nameKeyFunction
	nameKeyFunctionArgument
	nameKeyEntryPoint
	nameKeyLocal
)

// Writer generates MSL source code from IR.
type Writer struct {
	module   *ir.Module
	options  *Options
	pipeline *PipelineOptions

	// Output buffer
	out strings.Builder

	// Current indentation level
	indent int

	// Name management
	names      map[nameKey]string
	namer      *namer
	structPads map[nameKey]struct{} // Tracks struct members that need padding

	// Type tracking
	typeNames     map[ir.TypeHandle]string
	arrayWrappers map[ir.TypeHandle]string // Array types wrapped in structs

	// Function context (set during function writing)
	currentFunction    *ir.Function
	currentFuncHandle  ir.FunctionHandle
	localNames         map[uint32]string
	namedExpressions   map[ir.ExpressionHandle]string
	needBakeExpression map[ir.ExpressionHandle]struct{}

	// Output tracking
	entryPointNames  map[string]string
	needsSizesBuffer bool
	needsDivHelper   bool
	needsModHelper   bool

	entryPointOutputVar        string
	entryPointOutputType       ir.TypeHandle
	entryPointOutputTypeActive bool
	entryPointInputStructArg   int
}

// namer generates unique identifiers.
type namer struct {
	usedNames map[string]struct{}
	counter   uint32
}

func newNamer() *namer {
	return &namer{
		usedNames: make(map[string]struct{}),
	}
}

// call generates a unique name based on the given base.
func (n *namer) call(base string) string {
	// First try the base name directly
	escaped := escapeName(base)
	if _, used := n.usedNames[escaped]; !used {
		n.usedNames[escaped] = struct{}{}
		return escaped
	}

	// Add numeric suffix
	for {
		n.counter++
		candidate := fmt.Sprintf("%s_%d", escaped, n.counter)
		if _, used := n.usedNames[candidate]; !used {
			n.usedNames[candidate] = struct{}{}
			return candidate
		}
	}
}

// newWriter creates a new MSL writer.
func newWriter(module *ir.Module, options *Options, pipeline *PipelineOptions) *Writer {
	return &Writer{
		module:           module,
		options:          options,
		pipeline:         pipeline,
		names:            make(map[nameKey]string),
		namer:            newNamer(),
		structPads:       make(map[nameKey]struct{}),
		typeNames:        make(map[ir.TypeHandle]string),
		arrayWrappers:    make(map[ir.TypeHandle]string),
		entryPointNames:  make(map[string]string),
		namedExpressions: make(map[ir.ExpressionHandle]string),
		entryPointInputStructArg: -1,
	}
}

// String returns the generated MSL source code.
func (w *Writer) String() string {
	return w.out.String()
}

// writeModule generates MSL code for the entire module.
func (w *Writer) writeModule() error {
	// 1. Write header
	w.writeHeader()

	// 2. Register all names
	if err := w.registerNames(); err != nil {
		return err
	}

	// 3. Write type definitions
	if err := w.writeTypes(); err != nil {
		return err
	}

	// 4. Write constants
	if err := w.writeConstants(); err != nil {
		return err
	}

	// 5. Write helper functions if needed
	w.writeHelperFunctions()

	// 6. Write functions
	if err := w.writeFunctions(); err != nil {
		return err
	}

	// 7. Write entry points
	return w.writeEntryPoints()
}

// writeHeader writes the MSL file header.
func (w *Writer) writeHeader() {
	w.writeLine("#include <metal_stdlib>")
	w.writeLine("#include <simd/simd.h>")
	w.writeLine("")
	w.writeLine("using metal::uint;")
	w.writeLine("")
}

// registerNames assigns unique names to all IR entities.
//
//nolint:gocognit // Name registration requires handling all IR entity types
func (w *Writer) registerNames() error {
	entryPointNames := make(map[ir.FunctionHandle]string)
	for _, ep := range w.module.EntryPoints {
		if ep.Name != "" {
			entryPointNames[ep.Function] = ep.Name
		}
	}

	// Register type names
	for handle, typ := range w.module.Types {
		var baseName string
		if typ.Name != "" {
			baseName = typ.Name
		} else {
			baseName = fmt.Sprintf("type_%d", handle)
		}
		name := w.namer.call(baseName)
		w.names[nameKey{kind: nameKeyType, handle1: uint32(handle)}] = name //nolint:gosec // G115: handle is valid slice index
		w.typeNames[ir.TypeHandle(handle)] = name                           //nolint:gosec // G115: handle is valid slice index

		// Register struct member names
		if st, ok := typ.Inner.(ir.StructType); ok {
			for memberIdx, member := range st.Members {
				memberName := member.Name
				if memberName == "" {
					memberName = fmt.Sprintf("member_%d", memberIdx)
				}
				w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(handle), handle2: uint32(memberIdx)}] = escapeName(memberName) //nolint:gosec // G115: handle is valid slice index
			}
		}
	}

	// Register constant names
	for handle, constant := range w.module.Constants {
		var baseName string
		if constant.Name != "" {
			baseName = constant.Name
		} else {
			baseName = fmt.Sprintf("const_%d", handle)
		}
		name := w.namer.call(baseName)
		w.names[nameKey{kind: nameKeyConstant, handle1: uint32(handle)}] = name //nolint:gosec // G115: handle is valid slice index
	}

	// Register global variable names
	for handle, global := range w.module.GlobalVariables {
		var baseName string
		if global.Name != "" {
			baseName = global.Name
		} else {
			baseName = fmt.Sprintf("global_%d", handle)
		}
		name := w.namer.call(baseName)
		w.names[nameKey{kind: nameKeyGlobalVariable, handle1: uint32(handle)}] = name //nolint:gosec // G115: handle is valid slice index
	}

	// Register function names
	for handle := range w.module.Functions {
		fn := &w.module.Functions[handle]
		baseName := fn.Name
		if entryName, ok := entryPointNames[ir.FunctionHandle(handle)]; ok {
			baseName = entryName
		}
		if baseName == "" {
			baseName = fmt.Sprintf("function_%d", handle)
		}
		name := w.namer.call(baseName)
		w.names[nameKey{kind: nameKeyFunction, handle1: uint32(handle)}] = name //nolint:gosec // G115: handle is valid slice index

		// Register function argument names
		for argIdx, arg := range fn.Arguments {
			argName := arg.Name
			if argName == "" {
				argName = fmt.Sprintf("arg_%d", argIdx)
			}
			w.names[nameKey{kind: nameKeyFunctionArgument, handle1: uint32(handle), handle2: uint32(argIdx)}] = escapeName(argName) //nolint:gosec // G115: handle is valid slice index
		}
	}

	// Register entry point names
	for epIdx, ep := range w.module.EntryPoints {
		fnName, ok := w.names[nameKey{kind: nameKeyFunction, handle1: uint32(ep.Function)}]
		if !ok || fnName == "" {
			fnName = w.namer.call(ep.Name)
		}
		w.names[nameKey{kind: nameKeyEntryPoint, handle1: uint32(epIdx)}] = fnName //nolint:gosec // G115: epIdx is valid slice index
		w.entryPointNames[ep.Name] = fnName
	}

	return nil
}

// Output helpers

// write writes text to the output. If args are provided, uses fmt.Fprintf.
//
//nolint:goprintffuncname
func (w *Writer) write(format string, args ...any) {
	if len(args) == 0 {
		w.out.WriteString(format)
	} else {
		fmt.Fprintf(&w.out, format, args...)
	}
}

// writeLine writes a line with optional format args and a newline.
//
//nolint:goprintffuncname
func (w *Writer) writeLine(format string, args ...any) {
	w.writeIndent()
	if len(args) == 0 {
		w.out.WriteString(format)
	} else {
		fmt.Fprintf(&w.out, format, args...)
	}
	w.out.WriteByte('\n')
}

// writeIndent writes the current indentation.
func (w *Writer) writeIndent() {
	for i := 0; i < w.indent; i++ {
		w.out.WriteString("    ")
	}
}

// pushIndent increases indentation.
func (w *Writer) pushIndent() {
	w.indent++
}

// popIndent decreases indentation.
func (w *Writer) popIndent() {
	if w.indent > 0 {
		w.indent--
	}
}

// writeHelperFunctions writes any needed polyfill functions.
func (w *Writer) writeHelperFunctions() {
	// These will be set to true during expression/statement writing
	// For now, we write them unconditionally for simplicity

	// Safe integer division helper
	w.writeLine("// Safe division helper (handles zero divisor)")
	w.writeLine("template <typename T, typename D>")
	w.writeLine("T _naga_div(T lhs, D rhs) {")
	w.pushIndent()
	w.writeLine("D nz = D(rhs != D(0));")
	w.writeLine("return lhs / (nz * rhs + D(!nz));")
	w.popIndent()
	w.writeLine("}")
	w.writeLine("")

	// Safe integer modulo helper
	w.writeLine("// Safe modulo helper (handles zero divisor)")
	w.writeLine("template <typename T, typename D>")
	w.writeLine("T _naga_mod(T lhs, D rhs) {")
	w.pushIndent()
	w.writeLine("D nz = D(rhs != D(0));")
	w.writeLine("return lhs %s (nz * rhs + D(!nz));", "%")
	w.popIndent()
	w.writeLine("}")
	w.writeLine("")
}

// getTypeName returns the MSL type name for a type handle.
func (w *Writer) getTypeName(handle ir.TypeHandle) string {
	if name, ok := w.typeNames[handle]; ok {
		return name
	}
	return fmt.Sprintf("type_%d", handle)
}

// getName returns the registered name for a name key.
func (w *Writer) getName(key nameKey) string {
	if name, ok := w.names[key]; ok {
		return name
	}
	return fmt.Sprintf("unnamed_%d_%d", key.kind, key.handle1)
}
