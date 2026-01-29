package ir

import (
	"fmt"
)

// ValidationError represents a validation error.
type ValidationError struct {
	Message string
	// Optional context
	Function   string
	Expression *ExpressionHandle
	Statement  int
}

// Error implements the error interface.
func (e ValidationError) Error() string {
	if e.Function != "" {
		if e.Expression != nil {
			return fmt.Sprintf("in function %s, expression %d: %s", e.Function, *e.Expression, e.Message)
		}
		if e.Statement >= 0 {
			return fmt.Sprintf("in function %s, statement %d: %s", e.Function, e.Statement, e.Message)
		}
		return fmt.Sprintf("in function %s: %s", e.Function, e.Message)
	}
	return e.Message
}

// Validator validates IR modules.
type Validator struct {
	module  *Module
	errors  []ValidationError
	context validationContext
}

// validationContext holds current validation context.
type validationContext struct {
	function       *Function
	functionName   string
	loopDepth      int
	inContinuing   bool
	expressionUsed map[ExpressionHandle]bool
}

// Validate checks the IR module for correctness.
// Returns validation errors if any, or nil if module is valid.
func Validate(module *Module) ([]ValidationError, error) {
	if module == nil {
		return nil, fmt.Errorf("module is nil")
	}

	v := &Validator{
		module: module,
		errors: make([]ValidationError, 0),
	}

	v.ValidateModule()

	if len(v.errors) > 0 {
		return v.errors, nil
	}
	return nil, nil
}

// ValidateModule validates the complete module.
func (v *Validator) ValidateModule() {
	// Validate types
	v.validateTypes()

	// Validate constants
	v.validateConstants()

	// Validate global variables
	v.validateGlobalVariables()

	// Validate functions
	v.validateFunctions()

	// Validate entry points
	v.validateEntryPoints()
}

// validateTypes checks all type definitions.
func (v *Validator) validateTypes() {
	for i, typ := range v.module.Types {
		v.validateType(TypeHandle(i), &typ)
	}
}

// validateType validates a single type.
//
//nolint:gocognit,gocyclo,cyclop // Type validation requires checking many type variants
func (v *Validator) validateType(handle TypeHandle, typ *Type) {
	if typ.Inner == nil {
		v.addError(fmt.Sprintf("type %d has nil inner type", handle))
		return
	}

	switch inner := typ.Inner.(type) {
	case ScalarType:
		// Scalar types are always valid
		if inner.Width != 1 && inner.Width != 2 && inner.Width != 4 && inner.Width != 8 {
			v.addError(fmt.Sprintf("type %d: scalar width must be 1, 2, 4, or 8 bytes, got %d", handle, inner.Width))
		}

	case VectorType:
		// Vector size must be 2, 3, or 4
		if inner.Size != Vec2 && inner.Size != Vec3 && inner.Size != Vec4 {
			v.addError(fmt.Sprintf("type %d: vector size must be 2, 3, or 4, got %d", handle, inner.Size))
		}
		// Scalar width validation
		if inner.Scalar.Width != 1 && inner.Scalar.Width != 2 && inner.Scalar.Width != 4 && inner.Scalar.Width != 8 {
			v.addError(fmt.Sprintf("type %d: vector scalar width must be 1, 2, 4, or 8 bytes, got %d", handle, inner.Scalar.Width))
		}

	case MatrixType:
		// Matrix dimensions must be 2, 3, or 4
		if inner.Columns != Vec2 && inner.Columns != Vec3 && inner.Columns != Vec4 {
			v.addError(fmt.Sprintf("type %d: matrix columns must be 2, 3, or 4, got %d", handle, inner.Columns))
		}
		if inner.Rows != Vec2 && inner.Rows != Vec3 && inner.Rows != Vec4 {
			v.addError(fmt.Sprintf("type %d: matrix rows must be 2, 3, or 4, got %d", handle, inner.Rows))
		}
		// Matrix scalar must be float
		if inner.Scalar.Kind != ScalarFloat {
			v.addError(fmt.Sprintf("type %d: matrix scalar must be float, got %v", handle, inner.Scalar.Kind))
		}

	case ArrayType:
		// Base type must exist
		if !v.isValidTypeHandle(inner.Base) {
			v.addError(fmt.Sprintf("type %d: array base type %d does not exist", handle, inner.Base))
		}
		// Check for potential circular reference (simplified check)
		if inner.Base == handle {
			v.addError(fmt.Sprintf("type %d: array has circular reference to itself", handle))
		}

	case StructType:
		// Validate struct members
		memberNames := make(map[string]bool)
		for j, member := range inner.Members {
			if member.Name == "" {
				v.addError(fmt.Sprintf("type %d: struct member %d has empty name", handle, j))
			}
			if memberNames[member.Name] {
				v.addError(fmt.Sprintf("type %d: duplicate struct member name %q", handle, member.Name))
			}
			memberNames[member.Name] = true

			if !v.isValidTypeHandle(member.Type) {
				v.addError(fmt.Sprintf("type %d: struct member %q type %d does not exist", handle, member.Name, member.Type))
			}
			// Check for circular reference (simplified)
			if member.Type == handle {
				v.addError(fmt.Sprintf("type %d: struct member %q has circular reference", handle, member.Name))
			}
		}

	case PointerType:
		// Base type must exist
		if !v.isValidTypeHandle(inner.Base) {
			v.addError(fmt.Sprintf("type %d: pointer base type %d does not exist", handle, inner.Base))
		}

	case SamplerType:
		// Sampler types are always valid

	case ImageType:
		// Image types are always valid (dimension and class have enum constraints)
	}
}

// validateConstants checks all constants.
func (v *Validator) validateConstants() {
	for i, c := range v.module.Constants {
		if !v.isValidTypeHandle(c.Type) {
			v.addError(fmt.Sprintf("constant %d (%s): type %d does not exist", i, c.Name, c.Type))
		}
	}
}

// validateGlobalVariables checks all global variables.
func (v *Validator) validateGlobalVariables() {
	bindings := make(map[string]bool) // Track binding uniqueness (group:binding)
	names := make(map[string]bool)

	for i, gv := range v.module.GlobalVariables {
		if gv.Name != "" {
			if names[gv.Name] {
				v.addError(fmt.Sprintf("duplicate global variable name %q", gv.Name))
			}
			names[gv.Name] = true
		}

		if !v.isValidTypeHandle(gv.Type) {
			v.addError(fmt.Sprintf("global variable %d (%s): type %d does not exist", i, gv.Name, gv.Type))
		}

		if gv.Binding != nil {
			key := fmt.Sprintf("%d:%d", gv.Binding.Group, gv.Binding.Binding)
			if bindings[key] {
				v.addError(fmt.Sprintf("global variable %q: duplicate binding @group(%d) @binding(%d)",
					gv.Name, gv.Binding.Group, gv.Binding.Binding))
			}
			bindings[key] = true
		}

		if gv.Init != nil {
			if !v.isValidConstantHandle(*gv.Init) {
				v.addError(fmt.Sprintf("global variable %q: init constant %d does not exist", gv.Name, *gv.Init))
			}
		}
	}
}

// validateFunctions checks all functions.
func (v *Validator) validateFunctions() {
	names := make(map[string]bool)

	for i := range v.module.Functions {
		fn := &v.module.Functions[i]
		if fn.Name != "" {
			if names[fn.Name] {
				v.addError(fmt.Sprintf("duplicate function name %q", fn.Name))
			}
			names[fn.Name] = true
		}

		v.context = validationContext{
			function:       fn,
			functionName:   fn.Name,
			loopDepth:      0,
			inContinuing:   false,
			expressionUsed: make(map[ExpressionHandle]bool),
		}

		v.validateFunction(fn)
	}
}

// validateFunction validates a single function.
func (v *Validator) validateFunction(fn *Function) {
	// Validate arguments
	for i, arg := range fn.Arguments {
		if !v.isValidTypeHandle(arg.Type) {
			v.addErrorInFunction(fmt.Sprintf("argument %d (%s): type %d does not exist", i, arg.Name, arg.Type))
		}
	}

	// Validate result
	if fn.Result != nil {
		if !v.isValidTypeHandle(fn.Result.Type) {
			v.addErrorInFunction(fmt.Sprintf("result type %d does not exist", fn.Result.Type))
		}
	}

	// Validate local variables
	for i, lv := range fn.LocalVars {
		if !v.isValidTypeHandle(lv.Type) {
			v.addErrorInFunction(fmt.Sprintf("local variable %d (%s): type %d does not exist", i, lv.Name, lv.Type))
		}
		if lv.Init != nil {
			if !v.isValidExpressionHandle(*lv.Init) {
				v.addErrorInFunction(fmt.Sprintf("local variable %q: init expression %d does not exist", lv.Name, *lv.Init))
			}
		}
	}

	// Validate expressions
	for i, expr := range fn.Expressions {
		v.validateExpression(ExpressionHandle(i), &expr)
	}

	// Validate body
	v.validateBlock(fn.Body)
}

// validateExpression validates a single expression.
//
//nolint:gocognit,gocyclo,cyclop,funlen // Expression validation requires checking many expression variants
func (v *Validator) validateExpression(handle ExpressionHandle, expr *Expression) {
	if expr.Kind == nil {
		v.addErrorInExpression(handle, "expression has nil kind")
		return
	}

	switch kind := expr.Kind.(type) {
	case Literal:
		// Literals are always valid

	case ExprConstant:
		if !v.isValidConstantHandle(kind.Constant) {
			v.addErrorInExpression(handle, fmt.Sprintf("constant %d does not exist", kind.Constant))
		}

	case ExprZeroValue:
		if !v.isValidTypeHandle(kind.Type) {
			v.addErrorInExpression(handle, fmt.Sprintf("type %d does not exist", kind.Type))
		}

	case ExprCompose:
		if !v.isValidTypeHandle(kind.Type) {
			v.addErrorInExpression(handle, fmt.Sprintf("type %d does not exist", kind.Type))
		}
		for i, comp := range kind.Components {
			if !v.isValidExpressionHandle(comp) {
				v.addErrorInExpression(handle, fmt.Sprintf("component %d: expression %d does not exist", i, comp))
			}
		}

	case ExprAccess:
		if !v.isValidExpressionHandle(kind.Base) {
			v.addErrorInExpression(handle, fmt.Sprintf("base expression %d does not exist", kind.Base))
		}
		if !v.isValidExpressionHandle(kind.Index) {
			v.addErrorInExpression(handle, fmt.Sprintf("index expression %d does not exist", kind.Index))
		}

	case ExprAccessIndex:
		if !v.isValidExpressionHandle(kind.Base) {
			v.addErrorInExpression(handle, fmt.Sprintf("base expression %d does not exist", kind.Base))
		}

	case ExprSplat:
		if kind.Size != Vec2 && kind.Size != Vec3 && kind.Size != Vec4 {
			v.addErrorInExpression(handle, fmt.Sprintf("splat size must be 2, 3, or 4, got %d", kind.Size))
		}
		if !v.isValidExpressionHandle(kind.Value) {
			v.addErrorInExpression(handle, fmt.Sprintf("value expression %d does not exist", kind.Value))
		}

	case ExprSwizzle:
		if kind.Size != Vec2 && kind.Size != Vec3 && kind.Size != Vec4 {
			v.addErrorInExpression(handle, fmt.Sprintf("swizzle size must be 2, 3, or 4, got %d", kind.Size))
		}
		if !v.isValidExpressionHandle(kind.Vector) {
			v.addErrorInExpression(handle, fmt.Sprintf("vector expression %d does not exist", kind.Vector))
		}
		for i := 0; i < int(kind.Size); i++ {
			if kind.Pattern[i] > SwizzleW {
				v.addErrorInExpression(handle, fmt.Sprintf("pattern[%d] invalid component %d", i, kind.Pattern[i]))
			}
		}

	case ExprFunctionArgument:
		if v.context.function != nil {
			if int(kind.Index) >= len(v.context.function.Arguments) {
				v.addErrorInExpression(handle, fmt.Sprintf("argument index %d out of range (function has %d args)",
					kind.Index, len(v.context.function.Arguments)))
			}
		}

	case ExprGlobalVariable:
		if !v.isValidGlobalVariableHandle(kind.Variable) {
			v.addErrorInExpression(handle, fmt.Sprintf("global variable %d does not exist", kind.Variable))
		}

	case ExprLocalVariable:
		if v.context.function != nil {
			if int(kind.Variable) >= len(v.context.function.LocalVars) {
				v.addErrorInExpression(handle, fmt.Sprintf("local variable index %d out of range (function has %d vars)",
					kind.Variable, len(v.context.function.LocalVars)))
			}
		}

	case ExprLoad:
		if !v.isValidExpressionHandle(kind.Pointer) {
			v.addErrorInExpression(handle, fmt.Sprintf("pointer expression %d does not exist", kind.Pointer))
		}

	case ExprImageSample:
		if !v.isValidExpressionHandle(kind.Image) {
			v.addErrorInExpression(handle, fmt.Sprintf("image expression %d does not exist", kind.Image))
		}
		if !v.isValidExpressionHandle(kind.Sampler) {
			v.addErrorInExpression(handle, fmt.Sprintf("sampler expression %d does not exist", kind.Sampler))
		}
		if !v.isValidExpressionHandle(kind.Coordinate) {
			v.addErrorInExpression(handle, fmt.Sprintf("coordinate expression %d does not exist", kind.Coordinate))
		}
		if kind.ArrayIndex != nil && !v.isValidExpressionHandle(*kind.ArrayIndex) {
			v.addErrorInExpression(handle, fmt.Sprintf("array index expression %d does not exist", *kind.ArrayIndex))
		}
		if kind.Offset != nil && !v.isValidExpressionHandle(*kind.Offset) {
			v.addErrorInExpression(handle, fmt.Sprintf("offset expression %d does not exist", *kind.Offset))
		}
		if kind.DepthRef != nil && !v.isValidExpressionHandle(*kind.DepthRef) {
			v.addErrorInExpression(handle, fmt.Sprintf("depth ref expression %d does not exist", *kind.DepthRef))
		}

	case ExprImageLoad:
		if !v.isValidExpressionHandle(kind.Image) {
			v.addErrorInExpression(handle, fmt.Sprintf("image expression %d does not exist", kind.Image))
		}
		if !v.isValidExpressionHandle(kind.Coordinate) {
			v.addErrorInExpression(handle, fmt.Sprintf("coordinate expression %d does not exist", kind.Coordinate))
		}
		if kind.ArrayIndex != nil && !v.isValidExpressionHandle(*kind.ArrayIndex) {
			v.addErrorInExpression(handle, fmt.Sprintf("array index expression %d does not exist", *kind.ArrayIndex))
		}
		if kind.Sample != nil && !v.isValidExpressionHandle(*kind.Sample) {
			v.addErrorInExpression(handle, fmt.Sprintf("sample expression %d does not exist", *kind.Sample))
		}
		if kind.Level != nil && !v.isValidExpressionHandle(*kind.Level) {
			v.addErrorInExpression(handle, fmt.Sprintf("level expression %d does not exist", *kind.Level))
		}

	case ExprImageQuery:
		if !v.isValidExpressionHandle(kind.Image) {
			v.addErrorInExpression(handle, fmt.Sprintf("image expression %d does not exist", kind.Image))
		}

	case ExprUnary:
		if !v.isValidExpressionHandle(kind.Expr) {
			v.addErrorInExpression(handle, fmt.Sprintf("operand expression %d does not exist", kind.Expr))
		}

	case ExprBinary:
		if !v.isValidExpressionHandle(kind.Left) {
			v.addErrorInExpression(handle, fmt.Sprintf("left expression %d does not exist", kind.Left))
		}
		if !v.isValidExpressionHandle(kind.Right) {
			v.addErrorInExpression(handle, fmt.Sprintf("right expression %d does not exist", kind.Right))
		}

	case ExprSelect:
		if !v.isValidExpressionHandle(kind.Condition) {
			v.addErrorInExpression(handle, fmt.Sprintf("condition expression %d does not exist", kind.Condition))
		}
		if !v.isValidExpressionHandle(kind.Accept) {
			v.addErrorInExpression(handle, fmt.Sprintf("accept expression %d does not exist", kind.Accept))
		}
		if !v.isValidExpressionHandle(kind.Reject) {
			v.addErrorInExpression(handle, fmt.Sprintf("reject expression %d does not exist", kind.Reject))
		}

	case ExprDerivative:
		if !v.isValidExpressionHandle(kind.Expr) {
			v.addErrorInExpression(handle, fmt.Sprintf("expression %d does not exist", kind.Expr))
		}

	case ExprRelational:
		if !v.isValidExpressionHandle(kind.Argument) {
			v.addErrorInExpression(handle, fmt.Sprintf("argument expression %d does not exist", kind.Argument))
		}

	case ExprMath:
		if !v.isValidExpressionHandle(kind.Arg) {
			v.addErrorInExpression(handle, fmt.Sprintf("arg expression %d does not exist", kind.Arg))
		}
		if kind.Arg1 != nil && !v.isValidExpressionHandle(*kind.Arg1) {
			v.addErrorInExpression(handle, fmt.Sprintf("arg1 expression %d does not exist", *kind.Arg1))
		}
		if kind.Arg2 != nil && !v.isValidExpressionHandle(*kind.Arg2) {
			v.addErrorInExpression(handle, fmt.Sprintf("arg2 expression %d does not exist", *kind.Arg2))
		}
		if kind.Arg3 != nil && !v.isValidExpressionHandle(*kind.Arg3) {
			v.addErrorInExpression(handle, fmt.Sprintf("arg3 expression %d does not exist", *kind.Arg3))
		}

	case ExprAs:
		if !v.isValidExpressionHandle(kind.Expr) {
			v.addErrorInExpression(handle, fmt.Sprintf("expression %d does not exist", kind.Expr))
		}

	case ExprCallResult:
		if !v.isValidFunctionHandle(kind.Function) {
			v.addErrorInExpression(handle, fmt.Sprintf("function %d does not exist", kind.Function))
		}

	case ExprArrayLength:
		if !v.isValidExpressionHandle(kind.Array) {
			v.addErrorInExpression(handle, fmt.Sprintf("array expression %d does not exist", kind.Array))
		}
	}
}

// validateBlock validates a block of statements.
func (v *Validator) validateBlock(block Block) {
	for i, stmt := range block {
		v.validateStatement(i, &stmt)
	}
}

// validateStatement validates a single statement.
//
//nolint:gocognit,gocyclo,cyclop,funlen // Statement validation requires checking many statement variants
func (v *Validator) validateStatement(index int, stmt *Statement) {
	if stmt.Kind == nil {
		v.addErrorInStatement(index, "statement has nil kind")
		return
	}

	switch kind := stmt.Kind.(type) {
	case StmtEmit:
		// Validate range
		if v.context.function != nil {
			exprCount := uint32(len(v.context.function.Expressions))
			if kind.Range.Start >= ExpressionHandle(exprCount) {
				v.addErrorInStatement(index, fmt.Sprintf("emit range start %d out of range", kind.Range.Start))
			}
			if kind.Range.End > ExpressionHandle(exprCount) {
				v.addErrorInStatement(index, fmt.Sprintf("emit range end %d out of range", kind.Range.End))
			}
			if kind.Range.Start >= kind.Range.End {
				v.addErrorInStatement(index, fmt.Sprintf("emit range start %d >= end %d", kind.Range.Start, kind.Range.End))
			}
		}

	case StmtBlock:
		v.validateBlock(kind.Block)

	case StmtIf:
		if !v.isValidExpressionHandle(kind.Condition) {
			v.addErrorInStatement(index, fmt.Sprintf("condition expression %d does not exist", kind.Condition))
		}
		v.validateBlock(kind.Accept)
		v.validateBlock(kind.Reject)

	case StmtSwitch:
		if !v.isValidExpressionHandle(kind.Selector) {
			v.addErrorInStatement(index, fmt.Sprintf("selector expression %d does not exist", kind.Selector))
		}
		hasDefault := false
		for _, c := range kind.Cases {
			if _, ok := c.Value.(SwitchValueDefault); ok {
				if hasDefault {
					v.addErrorInStatement(index, "switch has multiple default cases")
				}
				hasDefault = true
			}
			v.validateBlock(c.Body)
		}
		if !hasDefault {
			v.addErrorInStatement(index, "switch missing default case")
		}

	case StmtLoop:
		oldDepth := v.context.loopDepth
		v.context.loopDepth++

		v.validateBlock(kind.Body)

		oldContinuing := v.context.inContinuing
		v.context.inContinuing = true
		v.validateBlock(kind.Continuing)
		v.context.inContinuing = oldContinuing

		if kind.BreakIf != nil {
			if !v.isValidExpressionHandle(*kind.BreakIf) {
				v.addErrorInStatement(index, fmt.Sprintf("break-if expression %d does not exist", *kind.BreakIf))
			}
		}

		v.context.loopDepth = oldDepth

	case StmtBreak:
		if v.context.loopDepth == 0 {
			v.addErrorInStatement(index, "break outside of loop")
		}
		if v.context.inContinuing {
			v.addErrorInStatement(index, "break in continuing block")
		}

	case StmtContinue:
		if v.context.loopDepth == 0 {
			v.addErrorInStatement(index, "continue outside of loop")
		}
		if v.context.inContinuing {
			v.addErrorInStatement(index, "continue in continuing block")
		}

	case StmtReturn:
		if v.context.inContinuing {
			v.addErrorInStatement(index, "return in continuing block")
		}
		if kind.Value != nil {
			if !v.isValidExpressionHandle(*kind.Value) {
				v.addErrorInStatement(index, fmt.Sprintf("return value expression %d does not exist", *kind.Value))
			}
		}

	case StmtKill:
		if v.context.inContinuing {
			v.addErrorInStatement(index, "kill in continuing block")
		}

	case StmtBarrier:
		// Barriers are always valid

	case StmtStore:
		if !v.isValidExpressionHandle(kind.Pointer) {
			v.addErrorInStatement(index, fmt.Sprintf("pointer expression %d does not exist", kind.Pointer))
		}
		if !v.isValidExpressionHandle(kind.Value) {
			v.addErrorInStatement(index, fmt.Sprintf("value expression %d does not exist", kind.Value))
		}

	case StmtImageStore:
		if !v.isValidExpressionHandle(kind.Image) {
			v.addErrorInStatement(index, fmt.Sprintf("image expression %d does not exist", kind.Image))
		}
		if !v.isValidExpressionHandle(kind.Coordinate) {
			v.addErrorInStatement(index, fmt.Sprintf("coordinate expression %d does not exist", kind.Coordinate))
		}
		if kind.ArrayIndex != nil && !v.isValidExpressionHandle(*kind.ArrayIndex) {
			v.addErrorInStatement(index, fmt.Sprintf("array index expression %d does not exist", *kind.ArrayIndex))
		}
		if !v.isValidExpressionHandle(kind.Value) {
			v.addErrorInStatement(index, fmt.Sprintf("value expression %d does not exist", kind.Value))
		}

	case StmtAtomic:
		if !v.isValidExpressionHandle(kind.Pointer) {
			v.addErrorInStatement(index, fmt.Sprintf("pointer expression %d does not exist", kind.Pointer))
		}
		if !v.isValidExpressionHandle(kind.Value) {
			v.addErrorInStatement(index, fmt.Sprintf("value expression %d does not exist", kind.Value))
		}
		if kind.Result != nil && !v.isValidExpressionHandle(*kind.Result) {
			v.addErrorInStatement(index, fmt.Sprintf("result expression %d does not exist", *kind.Result))
		}

	case StmtWorkGroupUniformLoad:
		if !v.isValidExpressionHandle(kind.Pointer) {
			v.addErrorInStatement(index, fmt.Sprintf("pointer expression %d does not exist", kind.Pointer))
		}
		if !v.isValidExpressionHandle(kind.Result) {
			v.addErrorInStatement(index, fmt.Sprintf("result expression %d does not exist", kind.Result))
		}

	case StmtCall:
		if !v.isValidFunctionHandle(kind.Function) {
			v.addErrorInStatement(index, fmt.Sprintf("function %d does not exist", kind.Function))
		}
		for i, arg := range kind.Arguments {
			if !v.isValidExpressionHandle(arg) {
				v.addErrorInStatement(index, fmt.Sprintf("argument %d expression %d does not exist", i, arg))
			}
		}
		if kind.Result != nil && !v.isValidExpressionHandle(*kind.Result) {
			v.addErrorInStatement(index, fmt.Sprintf("result expression %d does not exist", *kind.Result))
		}

	case StmtRayQuery:
		if !v.isValidExpressionHandle(kind.Query) {
			v.addErrorInStatement(index, fmt.Sprintf("query expression %d does not exist", kind.Query))
		}
	}
}

// validateEntryPoints checks all entry points.
func (v *Validator) validateEntryPoints() {
	names := make(map[string]bool)

	for i, ep := range v.module.EntryPoints {
		if ep.Name == "" {
			v.addError(fmt.Sprintf("entry point %d has empty name", i))
		}
		if names[ep.Name] {
			v.addError(fmt.Sprintf("duplicate entry point name %q", ep.Name))
		}
		names[ep.Name] = true

		if !v.isValidFunctionHandle(ep.Function) {
			v.addError(fmt.Sprintf("entry point %q: function %d does not exist", ep.Name, ep.Function))
			continue
		}

		fn := &v.module.Functions[ep.Function]

		// Validate stage-specific requirements
		switch ep.Stage {
		case StageVertex:
			// Vertex shader must return @builtin(position)
			// Position can be either:
			// 1. Direct return: fn() -> @builtin(position) vec4<f32>
			// 2. Struct member: fn() -> VertexOutput { @builtin(position) pos: vec4<f32>, ... }
			if fn.Result == nil {
				v.addError(fmt.Sprintf("entry point %q (@vertex): must have a return value", ep.Name))
			} else if !v.hasPositionBuiltin(fn.Result) {
				v.addError(fmt.Sprintf("entry point %q (@vertex): must return @builtin(position)", ep.Name))
			}

		case StageFragment:
			// Fragment shader can optionally return with location binding
			// No strict validation here as fragment can be void

		case StageCompute:
			// Compute shader must have workgroup size
			if ep.Workgroup[0] == 0 || ep.Workgroup[1] == 0 || ep.Workgroup[2] == 0 {
				v.addError(fmt.Sprintf("entry point %q (@compute): workgroup size must be non-zero", ep.Name))
			}
		}
	}
}

// hasPositionBuiltin checks if the function result contains @builtin(position).
// This can be either:
// 1. Direct binding on result: fn() -> @builtin(position) vec4<f32>
// 2. Struct member binding: fn() -> Struct { @builtin(position) pos: vec4<f32> }
func (v *Validator) hasPositionBuiltin(result *FunctionResult) bool {
	// Case 1: Direct binding on result
	if result.Binding != nil && isPositionBuiltin(*result.Binding) {
		return true
	}

	// Case 2: Check struct members for @builtin(position)
	return v.structHasPositionBuiltin(result.Type)
}

// isPositionBuiltin checks if a binding is @builtin(position).
func isPositionBuiltin(binding Binding) bool {
	b, ok := binding.(BuiltinBinding)
	return ok && b.Builtin == BuiltinPosition
}

// structHasPositionBuiltin checks if a struct type has a member with @builtin(position).
func (v *Validator) structHasPositionBuiltin(typeHandle TypeHandle) bool {
	if int(typeHandle) >= len(v.module.Types) {
		return false
	}
	structType, ok := v.module.Types[typeHandle].Inner.(StructType)
	if !ok {
		return false
	}
	for _, member := range structType.Members {
		if member.Binding != nil && isPositionBuiltin(*member.Binding) {
			return true
		}
	}
	return false
}

// Helper methods for validation

func (v *Validator) isValidTypeHandle(handle TypeHandle) bool {
	return int(handle) < len(v.module.Types)
}

func (v *Validator) isValidConstantHandle(handle ConstantHandle) bool {
	return int(handle) < len(v.module.Constants)
}

func (v *Validator) isValidGlobalVariableHandle(handle GlobalVariableHandle) bool {
	return int(handle) < len(v.module.GlobalVariables)
}

func (v *Validator) isValidFunctionHandle(handle FunctionHandle) bool {
	return int(handle) < len(v.module.Functions)
}

func (v *Validator) isValidExpressionHandle(handle ExpressionHandle) bool {
	if v.context.function == nil {
		return false
	}
	return int(handle) < len(v.context.function.Expressions)
}

func (v *Validator) addError(msg string) {
	v.errors = append(v.errors, ValidationError{
		Message:   msg,
		Statement: -1,
	})
}

func (v *Validator) addErrorInFunction(msg string) {
	v.errors = append(v.errors, ValidationError{
		Message:   msg,
		Function:  v.context.functionName,
		Statement: -1,
	})
}

func (v *Validator) addErrorInExpression(handle ExpressionHandle, msg string) {
	v.errors = append(v.errors, ValidationError{
		Message:    msg,
		Function:   v.context.functionName,
		Expression: &handle,
		Statement:  -1,
	})
}

func (v *Validator) addErrorInStatement(index int, msg string) {
	v.errors = append(v.errors, ValidationError{
		Message:   msg,
		Function:  v.context.functionName,
		Statement: index,
	})
}
