package front

import (
	"github.com/gogpu/naga/ir"
)

// processEntryPoint builds the wrapper of one entry point, as Rust naga's
// process_entry_point: the wrapper copies the entry point's inputs from its
// arguments into the private globals the parsed function reads, calls that
// function, and copies the outputs back into the result.
func (p *parser) processEntryPoint(functionID uint32) error {
	ep, ok := p.entryPoints[functionID]
	if !ok {
		return errInvalidID(functionID)
	}
	if _, ok := p.lookupFunction[functionID]; !ok {
		return errInvalidID(functionID)
	}

	function := ir.Function{Name: ep.name + "_wrap"}
	var consumer emitter

	// 1. Copy the inputs from the arguments into their privates. The argument
	// and the pointer are pre-emit expressions: they stay outside every Emit
	// range, as Rust naga's wrapper leaves them.
	for _, variableID := range ep.variableIDs {
		variable, ok := p.lookupVariable[variableID]
		if !ok {
			return errInvalidID(variableID)
		}
		if variable.class != classInput {
			continue
		}
		global := p.module.GlobalVariables[variable.handle]
		argument := appendExpression(&function, ir.ExprFunctionArgument{
			Index: uint32(len(function.Arguments)),
		})
		value := argument
		if variable.argument.Type != global.Type {
			// The only difference is an integer built-in the IR reads as
			// unsigned and this global declares as signed.
			consumer.startWith(function.Expressions)
			width := uint8(4)
			value = appendExpression(&function, ir.ExprAs{
				Expr:    argument,
				Kind:    ir.ScalarSint,
				Convert: &width,
			})
			if statement, ok := consumer.finish(function.Expressions); ok {
				function.Body = append(function.Body, statement)
			}
		}
		pointer := appendExpression(&function, ir.ExprGlobalVariable{Variable: variable.handle})
		function.Body = append(function.Body, ir.Statement{Kind: ir.StmtStore{
			Pointer: pointer,
			Value:   value,
		}})

		argumentCopy := variable.argument
		if ep.stage == ir.StageFragment && argumentCopy.Binding != nil {
			argumentCopy.Binding = withDefaultInterpolation(argumentCopy.Binding, p.module.Types[argumentCopy.Type].Inner)
		}
		function.Arguments = append(function.Arguments, argumentCopy)
	}

	// 2. Call the parsed function. The wrapper is a node of the call graph
	// too, after every function, and the call holds a placeholder handle
	// until the functions are ordered.
	inner := p.addCall(callNode{id: uint32(len(p.module.EntryPoints)), wrapper: true}, deferredCall{
		callee:  functionID,
		wrapper: true,
	})
	function.Body = append(function.Body, ir.Statement{Kind: ir.StmtCall{Function: inner}})

	// 3. Copy the outputs from their privates into the result.
	if err := p.updateLayouter(); err != nil {
		return err
	}
	var members []ir.StructMember
	var components []ir.ExpressionHandle
	var nextOffset, structAlignment uint32 = 0, 1
	for _, variableID := range ep.variableIDs {
		variable, ok := p.lookupVariable[variableID]
		if !ok {
			return errInvalidID(variableID)
		}
		if variable.class != classOutput {
			continue
		}
		result := variable.result
		expression := appendExpression(&function, ir.ExprGlobalVariable{Variable: variable.handle})

		if structType, ok := p.module.Types[result.Type].Inner.(ir.StructType); ok {
			structType = p.cullUnusedBuiltIns(result.Type, structType)
			for index, member := range structType.Members {
				if member.Binding == nil {
					continue
				}
				if ep.stage == ir.StageVertex {
					member.Binding = withDefaultInterpolation(member.Binding, p.module.Types[member.Type].Inner)
				}
				layout := p.layouter.Layout(member.Type)
				nextOffset = roundUp(nextOffset, layout.Alignment)
				member.Offset = nextOffset
				structAlignment = max(structAlignment, layout.Alignment)
				nextOffset += layout.Size
				members = append(members, member)
				components = append(components, appendExpression(&function, ir.ExprAccessIndex{
					Base:  expression,
					Index: uint32(index),
				}))
			}
			continue
		}

		binding := result.Binding
		if binding != nil && ep.stage == ir.StageVertex {
			binding = withDefaultInterpolation(binding, p.module.Types[result.Type].Inner)
		}
		layout := p.layouter.Layout(result.Type)
		nextOffset = roundUp(nextOffset, layout.Alignment)
		members = append(members, ir.StructMember{
			Type:    result.Type,
			Binding: binding,
			Offset:  nextOffset,
		})
		structAlignment = max(structAlignment, layout.Alignment)
		nextOffset += layout.Size
		components = append(components, expression)
	}

	// Negate the Y of a Position output unless the caller keeps SPIR-V's
	// coordinate space.
	if !p.options.KeepCoordinateSpace {
		for index, member := range members {
			if member.Binding == nil {
				continue
			}
			builtin, ok := (*member.Binding).(ir.BuiltinBinding)
			if !ok || builtin.Builtin != ir.BuiltinPosition {
				continue
			}
			consumer.startWith(function.Expressions)
			access := appendExpression(&function, ir.ExprAccessIndex{Base: components[index], Index: 1})
			load := appendExpression(&function, ir.ExprLoad{Pointer: access})
			negated := appendExpression(&function, ir.ExprUnary{Op: ir.UnaryNegate, Expr: load})
			if statement, ok := consumer.finish(function.Expressions); ok {
				function.Body = append(function.Body, statement)
			}
			function.Body = append(function.Body, ir.Statement{Kind: ir.StmtStore{Pointer: access, Value: negated}})
		}
	}

	consumer.startWith(function.Expressions)
	for i, component := range components {
		components[i] = appendExpression(&function, ir.ExprLoad{Pointer: component})
	}

	switch len(members) {
	case 0:
	case 1:
		if statement, ok := consumer.finish(function.Expressions); ok {
			function.Body = append(function.Body, statement)
		}
		value := components[0]
		function.Body = append(function.Body, ir.Statement{Kind: ir.StmtReturn{Value: &value}})
		function.Result = &ir.FunctionResult{Type: members[0].Type, Binding: members[0].Binding}
	default:
		resultType := p.insertType("", ir.StructType{
			Members: members,
			Span:    roundUp(nextOffset, structAlignment),
		})
		composed := appendExpression(&function, ir.ExprCompose{Type: resultType, Components: components})
		if statement, ok := consumer.finish(function.Expressions); ok {
			function.Body = append(function.Body, statement)
		}
		function.Body = append(function.Body, ir.Statement{Kind: ir.StmtReturn{Value: &composed}})
		function.Result = &ir.FunctionResult{Type: resultType}
	}

	p.module.EntryPoints = append(p.module.EntryPoints, ir.EntryPoint{
		Name:           ep.name,
		Stage:          ep.stage,
		Function:       function,
		Workgroup:      ep.workgroup,
		EarlyDepthTest: ep.earlyDepthTest,
	})
	return nil
}

// cullUnusedBuiltIns drops the binding of every built-in member of an output
// struct that no access chain reached, as Rust naga does for gl_PerVertex
// (front/spv/function.rs, process_entry_point): glslang declares PointSize,
// ClipDistance and CullDistance whether or not the shader writes them, and
// their presence in the result can make a backend refuse the module.
//
// The type is rewritten in place, keeping its handle: the output global and
// its default initializer refer to the same type, and Rust naga's
// UniqueArena::replace keeps the handle too.
func (p *parser) cullUnusedBuiltIns(handle ir.TypeHandle, structType ir.StructType) ir.StructType {
	var culled []ir.StructMember
	for index, member := range structType.Members {
		if member.Binding == nil {
			continue
		}
		builtin, ok := (*member.Binding).(ir.BuiltinBinding)
		if !ok {
			continue
		}
		if _, accessed := p.glPerVertexBuiltinAccess[builtin.Builtin]; accessed {
			continue
		}
		if culled == nil {
			culled = append([]ir.StructMember(nil), structType.Members...)
		}
		culled[index].Binding = nil
	}
	if culled == nil {
		return structType
	}
	name := p.module.Types[handle].Name
	structType.Members = culled
	p.types.Replace(handle, name, structType)
	p.module.Types = p.types.GetTypes()
	return structType
}

// withDefaultInterpolation returns a copy of binding with the default
// interpolation applied. The binding a variable or a struct type holds is
// shared by every entry point that lists it, so it is never written through;
// Rust naga clones it the same way (front/spv/function.rs, the wrapper).
func withDefaultInterpolation(binding *ir.Binding, inner ir.TypeInner) *ir.Binding {
	copied := *binding
	applyDefaultInterpolation(&copied, inner)
	return &copied
}

// applyDefaultInterpolation gives a float location binding the interpolation
// Rust naga's frontends apply: perspective-correct, at the pixel center. An
// integer location keeps none, which the validator requires to be flat.
func applyDefaultInterpolation(binding *ir.Binding, inner ir.TypeInner) {
	location, ok := (*binding).(ir.LocationBinding)
	if !ok || location.Interpolation != nil {
		return
	}
	switch t := inner.(type) {
	case ir.ScalarType:
		if t.Kind != ir.ScalarFloat {
			return
		}
	case ir.VectorType:
		if t.Scalar.Kind != ir.ScalarFloat {
			return
		}
	default:
		return
	}
	interpolation := ir.InterpolationPerspective
	sampling := ir.SamplingCenter
	location.Interpolation = &interpolation
	location.Sampling = &sampling
	*binding = location
}
