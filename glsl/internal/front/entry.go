package front

// This file assembles a parsed file into a module and builds the entry point
// around GLSL's main, Rust naga's add_entry_point (functions.rs:1188-1386) and
// the declaration loop of ParsingContext::parse (parser.rs:168-201).

import (
	"strconv"

	"github.com/gogpu/naga/ir"
)

// resetMeta sets the per-shader metadata defaults, Rust naga's
// ShaderMetadata::reset (mod.rs:107-112): the compute workgroup size starts at
// one and every other stage at zero, where zero means "no workgroup".
func (f *Frontend) resetMeta(stage ir.ShaderStage) {
	f.meta.stage = stage
	f.meta.workgroupSize = [3]uint32{0, 0, 0}
	if stage == ir.StageCompute {
		f.meta.workgroupSize = [3]uint32{1, 1, 1}
	}
	f.meta.earlyFragmentTest = false
}

// lowerModule lowers a parsed file into f.mod: the module-scope declarations
// in source order, then the entry point around main. It returns the context
// that became the entry-point function's body, which a caller may read for
// its expressions.
func (f *Frontend) lowerModule(file *parsedFile) *frontError {
	// The module-level context carries the global initializers and becomes
	// the entry-point function, as Rust's ParsingContext::parse does
	// (parser.rs:168-178).
	c, err := newContext(f, false)
	if err != nil {
		return err
	}
	f.reserveExplicitIOLocations(file.Decls)
	for i := range file.Decls {
		if derr := f.lowerDeclaration(c, &file.Decls[i]); derr != nil {
			return derr
		}
	}
	// A split combined sampler's binding depends on every bound resource of
	// the module, so it is resolved once the declarations are all lowered.
	if derr := f.resolveCombinedSamplerBindings(file.Decls); derr != nil {
		return derr
	}
	// Every resource that needed a binding has it now, so a supplied entry no
	// declaration used is reported (design §11, R5: a warning, never a silent
	// fallback). A Vulkan GLSL source binds itself and consults no map, so its
	// entries all land here.
	for _, name := range f.bindings.unusedNames() {
		f.warn(ir.Span{}, "the ResourceBindings entry for "+strconv.Quote(name)+
			" did not match a resource that needs a binding")
	}
	// GLSL forbids recursion; Rust naga rejects a call cycle in validation.
	// Refused here, before the module reaches a backend.
	if rerr := f.rejectRecursiveCalls(); rerr != nil {
		return rerr
	}

	mainDecl, ok := f.functions["main"]
	if ok {
		for _, decl := range mainDecl.overloads {
			if decl.kind != overloadCall || !decl.defined || len(decl.parameters) != 0 {
				continue
			}
			return f.addEntryPoint(c, decl.function)
		}
	}
	return errMessage(ir.Span{}, "Missing entry point")
}

// reserveExplicitIOLocations records the locations the module-scope in and out
// variables declare explicitly, so an unlocated variable of the same class
// takes a location none of them uses (design §10: Rust gives every unlocated
// variable 0, which collides). The values are constant expressions, the same
// kind an array size has.
//
// The reservation is best effort: a location whose constant expression cannot
// be evaluated here is left unreserved and the declaration reports the error
// when it lowers. Evaluating locations before any declaration also appends
// their literals to the module's global expressions ahead of the first
// constant's initializer, which is the one place our global-expression order
// differs from Rust's; every handle stays consistent within the module (review
// finding P2-10).
func (f *Frontend) reserveExplicitIOLocations(decls []astDecl) {
	for i := range decls {
		d, ok := decls[i].Kind.(declVarList)
		if !ok {
			continue
		}
		if d.Qualifiers.Storage != storageInput && d.Qualifiers.Storage != storageOutput {
			continue
		}
		class := ioClassOf(d.Qualifiers.Storage)
		for _, entry := range d.Qualifiers.Layout {
			if entry.Kind != layoutNamed || entry.Name != "location" || !entry.HasValue || entry.Value == nil {
				continue
			}
			location, err := f.constU32(entry.Value)
			if err != nil {
				continue
			}
			if f.ioLocations[class] == nil {
				f.ioLocations[class] = map[uint32]bool{}
			}
			f.ioLocations[class][location] = true
		}
	}
}

// lowerDeclaration lowers one external declaration.
func (f *Frontend) lowerDeclaration(c *Context, d *astDecl) *frontError {
	switch kind := d.Kind.(type) {
	case declVarList:
		return f.lowerGlobalVarList(c, &kind)
	case declBlock:
		return f.lowerBlockDeclaration(c, &kind)
	case declFunc:
		return f.lowerFunctionDeclaration(&kind)
	case declQualifiers:
		f.lowerQualifierDeclaration(&kind)
		return nil
	case declPrecision:
		f.lowerPrecisionDeclaration(&kind)
		return nil
	case declInvariant:
		if kind.Sym == nil {
			// Rust's lookup_variable returns None for a name it knows neither
			// as a variable nor as a builtin, and the redeclaration does
			// nothing (variables.rs:232-241).
			return nil
		}
		return f.makeVariableInvariant(c, kind.Sym, d.Span)
	case declInvalid:
		// The parser reported the declaration and consumed its tokens.
		return nil
	default:
		return errInternal("unknown declaration kind", d.Span)
	}
}

// addEntryPoint synthesizes the IR entry point that calls the function main,
// Rust naga's add_entry_point (functions.rs:1218-1386): every input variable
// becomes an argument stored into its private global, the global initializers
// run, main is called, and every output variable is loaded into the result
// struct.
func (f *Frontend) addEntryPoint(c *Context, mainHandle ir.FunctionHandle) *frontError {
	var arguments []ir.FunctionArgument

	// The statements accumulated for module-scope initializers move into the
	// entry point before the call.
	globalInit := c.body
	c.body = ir.Block{}

	for _, arg := range f.entryArgs {
		if arg.storage != storageInput {
			continue
		}
		pointer := c.appendExpr(ir.ExprGlobalVariable{Variable: arg.handle})
		ty := f.mod.GlobalVariables[arg.handle].Type
		werr := f.argTypeWalker(c, arg.name, arg.binding, pointer, ty,
			func(c *Context, name string, pointer ir.ExpressionHandle, ty ir.TypeHandle, binding ir.Binding) *frontError {
				idx := uint32(len(arguments))
				arguments = append(arguments, ir.FunctionArgument{Name: name, Type: ty, Binding: &binding})
				value := c.appendExpr(ir.ExprFunctionArgument{Index: idx})
				c.body = append(c.body, ir.Statement{Kind: ir.StmtStore{Pointer: pointer, Value: value}})
				return nil
			})
		if werr != nil {
			return werr
		}
	}

	c.body = append(c.body, globalInit...)
	c.body = append(c.body, ir.Statement{Kind: ir.StmtCall{Function: mainHandle}})

	var (
		members    []ir.StructMember
		components []ir.ExpressionHandle
		span       uint32
	)
	for _, arg := range f.entryArgs {
		if arg.storage != storageOutput {
			continue
		}
		pointer := c.appendExpr(ir.ExprGlobalVariable{Variable: arg.handle})
		ty := f.mod.GlobalVariables[arg.handle].Type
		werr := f.argTypeWalker(c, arg.name, arg.binding, pointer, ty,
			func(c *Context, name string, pointer ir.ExpressionHandle, ty ir.TypeHandle, binding ir.Binding) *frontError {
				members = append(members, ir.StructMember{Name: name, Type: ty, Binding: &binding, Offset: span})
				span += f.typeSize(ty)

				load := c.appendExpr(ir.ExprLoad{Pointer: pointer})
				c.body = append(c.body, ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: load, End: load + 1}}})
				components = append(components, load)
				return nil
			})
		if werr != nil {
			return werr
		}
	}

	var resultTy *ir.TypeHandle
	var resultValue *ir.ExpressionHandle
	if len(components) != 0 {
		ty := f.registerType(ir.StructType{Members: members, Span: span})
		compose := c.appendExpr(ir.ExprCompose{Type: ty, Components: components})
		c.body = append(c.body, ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: compose, End: compose + 1}}})
		resultTy, resultValue = &ty, &compose
	}

	ret := ir.StmtReturn{Value: resultValue}
	c.body = append(c.body, ir.Statement{Kind: ret})

	var functionResult *ir.FunctionResult
	if resultTy != nil {
		functionResult = &ir.FunctionResult{Type: *resultTy}
	}

	var early ir.EarlyDepthTest
	if f.meta.earlyFragmentTest {
		early = ir.EarlyDepthForce{}
	}

	f.mod.EntryPoints = append(f.mod.EntryPoints, ir.EntryPoint{
		Name:      "main",
		Stage:     f.meta.stage,
		Workgroup: f.meta.workgroupSize,
		Function: ir.Function{
			Name:        "main",
			Arguments:   arguments,
			Expressions: c.expressions,
			Body:        c.body,
			Result:      functionResult,
		},
		EarlyDepthTest: early,
	})
	return nil
}

// argTypeWalker walks a variable's type and calls fn for every leaf that
// needs its own binding, Rust naga's arg_type_walker (functions.rs:1401-1502):
// an array's elements take consecutive locations, a struct's members take
// their own binding or the next location, and a builtin binding is not walked.
func (f *Frontend) argTypeWalker(c *Context, name string, binding ir.Binding, pointer ir.ExpressionHandle, ty ir.TypeHandle, fn func(*Context, string, ir.ExpressionHandle, ir.TypeHandle, ir.Binding) *frontError) *frontError {
	switch inner := f.mod.Types[ty].Inner.(type) {
	case ir.ArrayType:
		location, ok := locationOf(binding)
		if !ok || inner.Size.Constant == nil {
			// Only a constant-sized array of locations is walked. Anything
			// else — a builtin binding, a dynamically sized array such as
			// gl_ClipDistance — is a leaf, as Rust's match falls through to
			// `_ => f(...)` (functions.rs:1419-1430, 1498).
			return fn(c, name, pointer, ty, binding)
		}
		var interpolation *ir.Interpolation
		if scalar, ok := scalarComponents(f.mod.Types[inner.Base].Inner); ok {
			value := ir.InterpolationFlat
			if scalar.Kind == ir.ScalarFloat {
				value = ir.InterpolationPerspective
			}
			interpolation = &value
		}
		for index := uint32(0); index < *inner.Size.Constant; index++ {
			memberPointer, err := c.addExpression(ir.ExprAccessIndex{Base: pointer, Index: index}, ir.Span{})
			if err != nil {
				return err
			}
			elementBinding := ir.LocationBinding{Location: location + index, Interpolation: interpolation}
			if werr := f.argTypeWalker(c, name, elementBinding, memberPointer, inner.Base, fn); werr != nil {
				return werr
			}
		}
		return nil

	case ir.StructType:
		location, ok := locationOf(binding)
		if !ok {
			return fn(c, name, pointer, ty, binding)
		}
		for i, member := range inner.Members {
			memberPointer, err := c.addExpression(ir.ExprAccessIndex{Base: pointer, Index: uint32(i)}, ir.Span{})
			if err != nil {
				return err
			}
			var memberBinding ir.Binding
			if member.Binding != nil {
				memberBinding = *member.Binding
			} else {
				var interpolation *ir.Interpolation
				if scalar, ok := scalarComponents(f.mod.Types[member.Type].Inner); ok {
					value := ir.InterpolationFlat
					if scalar.Kind == ir.ScalarFloat {
						value = ir.InterpolationPerspective
					}
					interpolation = &value
				}
				memberBinding = ir.LocationBinding{Location: location, Interpolation: interpolation}
				location++
			}
			if werr := f.argTypeWalker(c, member.Name, memberBinding, memberPointer, member.Type, fn); werr != nil {
				return werr
			}
		}
		return nil

	default:
		return fn(c, name, pointer, ty, binding)
	}
}

// locationOf returns the location of a location binding, or false for any
// other binding.
func locationOf(binding ir.Binding) (uint32, bool) {
	location, ok := binding.(ir.LocationBinding)
	return location.Location, ok
}

// typeSize is the byte size of a type as Rust naga's TypeInner::size computes
// it, without the module: the result struct's members are laid end to end with
// it.
func (f *Frontend) typeSize(ty ir.TypeHandle) uint32 {
	switch inner := f.mod.Types[ty].Inner.(type) {
	case ir.ScalarType:
		return uint32(inner.Width)
	case ir.VectorType:
		return uint32(inner.Size) * uint32(inner.Scalar.Width)
	case ir.MatrixType:
		stride := alignmentOf(inner.Rows) * uint32(inner.Scalar.Width)
		return stride * uint32(inner.Columns)
	case ir.ArrayType:
		count := uint32(1)
		if inner.Size.Constant != nil {
			count = *inner.Size.Constant
		}
		return count * inner.Stride
	case ir.StructType:
		return inner.Span
	default:
		return 0
	}
}
