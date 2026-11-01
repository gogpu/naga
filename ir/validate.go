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

// validator validates IR modules.
type validator struct {
	module       *Module
	capabilities Capabilities
	errors       []ValidationError
	context      validationContext
}

// validationContext holds current validation context.
type validationContext struct {
	function       *Function
	functionName   string
	flow           controlFlowAbility
	expressionUsed map[ExpressionHandle]bool
}

// controlFlowAbility is what a statement may do to leave the block it is
// in: Rust naga's ControlFlowAbility (valid/function.rs). A function body
// may return; a switch case may also break; a loop body may also break and
// continue; a continuing block may do none of them, and a loop inside it
// may break and continue again.
type controlFlowAbility uint8

const (
	abilityReturn controlFlowAbility = 1 << iota
	abilityBreak
	abilityContinue
)

// ValidateOptions configures Validate. The zero value is WebGPU core: no
// optional capability.
type ValidateOptions struct {
	// Capabilities are the optional features the module may use; a type or
	// operation that needs one outside the set is a validation error. This
	// is the single gate for capabilities: neither the frontends nor the
	// backends check them.
	Capabilities Capabilities
}

// ValidationErrors is the error Validate returns: every problem it found,
// where Rust naga stops at the first. errors.As finds a *ValidationError.
type ValidationErrors []ValidationError

// Error reports the first error and how many follow.
func (e ValidationErrors) Error() string {
	switch len(e) {
	case 0:
		return "no validation errors"
	case 1:
		return e[0].Error()
	}
	return fmt.Sprintf("%s (and %d more)", e[0].Error(), len(e)-1)
}

// Unwrap returns each error, for errors.As and errors.Is.
func (e ValidationErrors) Unwrap() []error {
	errs := make([]error, len(e))
	for i := range e {
		errs[i] = &e[i]
	}
	return errs
}

// Validate checks the module for correctness (Rust naga's
// Validator::validate) and returns what a WebGPU implementation needs from
// it: per function and entry point, the globals it uses and how. A module
// with problems returns a ValidationErrors.
func Validate(module *Module, opts ValidateOptions) (ModuleInfo, error) {
	if module == nil {
		return ModuleInfo{}, fmt.Errorf("module is nil")
	}

	v := &validator{
		module:       module,
		capabilities: opts.Capabilities,
		errors:       make([]ValidationError, 0),
	}

	v.validateModule()

	if len(v.errors) > 0 {
		return ModuleInfo{}, ValidationErrors(v.errors)
	}
	return analyzeModule(module), nil
}

// validateModule validates the complete module.
func (v *validator) validateModule() {
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
func (v *validator) validateTypes() {
	for i, typ := range v.module.Types {
		v.validateType(TypeHandle(i), &typ)
	}
}

// checkScalarCapability validates that the given scalar type is allowed by the
// current capabilities. Mirrors Rust naga valid::type.rs check_width().
//
// Width=8 (64-bit) requires CapFloat64 (for f64) or CapShaderInt64 (for i64/u64).
// Width=2 (16-bit) requires CapShaderFloat16 (for f16).
// Width=4 and width=1 (bool) are always allowed.
func (v *validator) checkScalarCapability(handle TypeHandle, scalar ScalarType, context string) {
	switch {
	case scalar.Width == 8 && scalar.Kind == ScalarFloat:
		if !v.capabilities.Contains(CapFloat64) {
			v.addError(fmt.Sprintf("type [%d]%s: f64 requires the FLOAT64 capability", handle, context))
		}
	case scalar.Width == 8 && (scalar.Kind == ScalarSint || scalar.Kind == ScalarUint):
		name := "i64"
		if scalar.Kind == ScalarUint {
			name = "u64"
		}
		if !v.capabilities.Contains(CapShaderInt64) {
			v.addError(fmt.Sprintf("type [%d]%s: %s requires the SHADER_INT64 capability", handle, context, name))
		}
	case scalar.Width == 2 && scalar.Kind == ScalarFloat:
		if !v.capabilities.Contains(CapShaderFloat16) {
			v.addError(fmt.Sprintf("type [%d]%s: f16 requires the SHADER_FLOAT16 capability", handle, context))
		}
	}
}

// validateType validates a single type.
func (v *validator) validateType(handle TypeHandle, typ *Type) {
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
		v.checkScalarCapability(handle, inner, "")

	case VectorType:
		// Vector size must be 2, 3, or 4
		if inner.Size != Vec2 && inner.Size != Vec3 && inner.Size != Vec4 {
			v.addError(fmt.Sprintf("type %d: vector size must be 2, 3, or 4, got %d", handle, inner.Size))
		}
		// Scalar width validation
		if inner.Scalar.Width != 1 && inner.Scalar.Width != 2 && inner.Scalar.Width != 4 && inner.Scalar.Width != 8 {
			v.addError(fmt.Sprintf("type %d: vector scalar width must be 1, 2, 4, or 8 bytes, got %d", handle, inner.Scalar.Width))
		}
		v.checkScalarCapability(handle, inner.Scalar, "")

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
		v.checkScalarCapability(handle, inner.Scalar, "")

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

	case AtomicType:
		// Atomic types embed a scalar directly — check capability.
		v.checkScalarCapability(handle, inner.Scalar, "")

	case SamplerType:
		// Sampler types are always valid

	case ImageType:
		if inner.Class == ImageClassStorage && inner.StorageFormat == StorageFormatUndefined {
			v.addError(fmt.Sprintf("type %d: storage image has no format (StorageFormatUndefined)", handle))
		}
		if inner.Class == ImageClassStorage && inner.StorageAccess == 0 {
			v.addError(fmt.Sprintf("type %d: storage image has no access (empty StorageAccess)", handle))
		}
	}
}

// validateConstants checks all constants.
func (v *validator) validateConstants() {
	for i, c := range v.module.Constants {
		if !v.isValidTypeHandle(c.Type) {
			v.addError(fmt.Sprintf("constant %d (%s): type %d does not exist", i, c.Name, c.Type))
		}
	}
}

// validateGlobalVariables checks all global variables.
func (v *validator) validateGlobalVariables() {
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
		} else if gv.Space != SpaceWorkGroup && v.containsPendingArray(gv.Type, 0) {
			// Rust: the type lacks CREATION_RESOLVED, which every address
			// space but workgroup requires.
			v.addError(fmt.Sprintf("Global variable [%d] '%s' is invalid: an array sized by an override is allowed only in the workgroup address space", i, gv.Name))
		}

		// Rust keeps the access inside AddressSpace::Storage (ADR-010 E2).
		switch {
		case gv.Space == SpaceStorage && gv.Access == 0:
			v.addError(fmt.Sprintf("global variable %q: storage variable has no access (empty StorageAccess)", gv.Name))
		case gv.Space == SpaceStorage && !gv.Access.Contains(StorageAccessLoad):
			// Rust's GlobalVariableError text.
			v.addError(fmt.Sprintf("global variable %q: Storage address space doesn't support write-only access", gv.Name))
		case gv.Space != SpaceStorage && gv.Access != 0:
			v.addError(fmt.Sprintf("global variable %q: only a storage variable has an access", gv.Name))
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
func (v *validator) validateFunctions() {
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
			flow:           abilityReturn,
			expressionUsed: make(map[ExpressionHandle]bool),
		}

		v.validateFunction(fn)
	}
}

// validateFunction validates a single function.
func (v *validator) validateFunction(fn *Function) {
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
		} else if v.containsPendingArray(lv.Type, 0) {
			// Rust: the type is not CONSTRUCTIBLE.
			v.addErrorInFunction(fmt.Sprintf("Local variable [%d] '%s' is invalid: an array sized by an override is not constructible", i, lv.Name))
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
func (v *validator) validateExpression(handle ExpressionHandle, expr *Expression) {
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
		} else if v.isSizedArrayPointer(kind.Array) {
			// Rust: arrayLength needs a runtime-sized array.
			v.addErrorInExpression(handle, fmt.Sprintf("Array length of [%d] can't be done", kind.Array))
		}

	case ExprRayQueryProceedResult:
		// No fields to validate — result is produced by StmtRayQuery with Proceed function

	case ExprRayQueryGetIntersection:
		if !v.isValidExpressionHandle(kind.Query) {
			v.addErrorInExpression(handle, fmt.Sprintf("ray query expression %d does not exist", kind.Query))
		}

	case ExprSubgroupBallotResult:
		// No fields to validate — result is produced by StmtSubgroupBallot

	case ExprSubgroupOperationResult:
		if !v.isValidTypeHandle(kind.Type) {
			v.addErrorInExpression(handle, fmt.Sprintf("type %d does not exist", kind.Type))
		}
	}
}

// validateBlock validates a block of statements.
func (v *validator) validateBlock(block Block) {
	for i, stmt := range block {
		v.validateStatement(i, &stmt)
	}
}

// validateBlockWith validates a block whose statements may leave it as
// flow allows.
func (v *validator) validateBlockWith(block Block, flow controlFlowAbility) {
	saved := v.context.flow
	v.context.flow = flow
	v.validateBlock(block)
	v.context.flow = saved
}

// validateStatement validates a single statement.
func (v *validator) validateStatement(index int, stmt *Statement) {
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
			v.validateBlockWith(c.Body, v.context.flow&(abilityReturn|abilityContinue)|abilityBreak)
		}
		if !hasDefault {
			v.addErrorInStatement(index, "switch missing default case")
		}

	case StmtLoop:
		v.validateBlockWith(kind.Body, v.context.flow&abilityReturn|abilityBreak|abilityContinue)
		v.validateBlockWith(kind.Continuing, 0)

		if kind.BreakIf != nil {
			if !v.isValidExpressionHandle(*kind.BreakIf) {
				v.addErrorInStatement(index, fmt.Sprintf("break-if expression %d does not exist", *kind.BreakIf))
			}
		}

	// The messages are Rust naga's FunctionError texts.
	case StmtBreak:
		if v.context.flow&abilityBreak == 0 {
			v.addErrorInStatement(index, "The `break` is used outside of a `loop` or `switch` context")
		}

	case StmtContinue:
		if v.context.flow&abilityContinue == 0 {
			v.addErrorInStatement(index, "The `continue` is used outside of a `loop` context")
		}

	case StmtReturn:
		if v.context.flow&abilityReturn == 0 {
			v.addErrorInStatement(index, "The `return` is called within a `continuing` block")
		}
		if kind.Value != nil {
			if !v.isValidExpressionHandle(*kind.Value) {
				v.addErrorInStatement(index, fmt.Sprintf("return value expression %d does not exist", *kind.Value))
			}
		}

	case StmtKill:
		// Allowed anywhere in a fragment shader, a continuing block too
		// (Rust naga checks only the stage).

	case StmtControlBarrier, StmtMemoryBarrier:
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
func (v *validator) validateEntryPoints() {
	names := make(map[string]bool)

	for i, ep := range v.module.EntryPoints {
		if ep.Name == "" {
			v.addError(fmt.Sprintf("entry point %d has empty name", i))
		}
		if names[ep.Name] {
			v.addError(fmt.Sprintf("duplicate entry point name %q", ep.Name))
		}
		names[ep.Name] = true

		// Entry point function is stored inline (not via handle).
		fn := &v.module.EntryPoints[i].Function

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

		v.validateVaryings(&v.module.EntryPoints[i])

		// The body is validated like any function's (Rust validates entry
		// point functions with the module's functions).
		v.context = validationContext{
			function:       fn,
			functionName:   ep.Name,
			flow:           abilityReturn,
			expressionUsed: make(map[ExpressionHandle]bool),
		}
		v.validateFunction(fn)
	}
}

// validateVaryings checks the interpolation of each location binding of an
// entry point's arguments and result, struct members included: the
// interpolation part of Rust naga's VaryingContext::validate_impl
// (valid/interface.rs). The messages are Rust's VaryingError texts.
func (v *validator) validateVaryings(ep *EntryPoint) {
	fn := &ep.Function
	for i := range fn.Arguments {
		arg := &fn.Arguments[i]
		v.validateVarying(ep, fmt.Sprintf("argument %d", i), arg.Type, arg.Binding, false)
	}
	if fn.Result != nil {
		v.validateVarying(ep, "result", fn.Result.Type, fn.Result.Binding, true)
	}
}

// validateVarying checks one entry point argument or result: its binding,
// or each struct member's.
func (v *validator) validateVarying(ep *EntryPoint, where string, ty TypeHandle, binding *Binding, output bool) {
	if binding == nil {
		if !v.isValidTypeHandle(ty) {
			return
		}
		if st, ok := v.module.Types[ty].Inner.(StructType); ok {
			for _, m := range st.Members {
				if m.Binding != nil {
					v.validateVarying(ep, where, m.Type, m.Binding, output)
				}
			}
		}
		return
	}
	loc, ok := (*binding).(LocationBinding)
	if !ok {
		return
	}
	if msg := v.locationVaryingError(ep.Stage, ty, loc, output); msg != "" {
		v.addError(fmt.Sprintf("entry point %q: %s: %s", ep.Name, where, msg))
	}
}

// locationVaryingError returns Rust's VaryingError text for a location
// binding whose interpolation does not fit its stage, direction and type, or
// "" when it fits.
func (v *validator) locationVaryingError(stage ShaderStage, ty TypeHandle, loc LocationBinding, output bool) string {
	if !v.isValidTypeHandle(ty) {
		return ""
	}
	perVertex := loc.Interpolation != nil && *loc.Interpolation == InterpolationPerVertex
	if perVertex {
		if stage != StageFragment {
			return "Interpolation PerVertex is only valid for stage Fragment"
		}
		if loc.Sampling != nil && *loc.Sampling != SamplingCenter {
			return "Per vertex can only have Center sampling or no sampling modifier"
		}
		// The checks below are on the element type of the array<T, 3>.
		arr, ok := v.module.Types[ty].Inner.(ArrayType)
		if !ok || arr.Size.Constant == nil || *arr.Size.Constant != 3 {
			return "Per vertex fragment inputs must be an array of length 3."
		}
		ty = arr.Base
		if !v.isValidTypeHandle(ty) {
			return ""
		}
	}

	// Only a fragment input reads a per-primitive value outside a mesh
	// shader's primitive outputs, which are not entry point results.
	if loc.PerPrimitive && (stage != StageFragment || output) {
		return "The `@per_primitive` attribute can only be used in fragment shader inputs or mesh shader primitive outputs"
	}

	if loc.Interpolation != nil && loc.Sampling != nil {
		valid := false
		switch *loc.Interpolation {
		case InterpolationPerspective, InterpolationLinear:
			s := *loc.Sampling
			valid = s == SamplingCenter || s == SamplingCentroid || s == SamplingSample
		case InterpolationFlat:
			valid = *loc.Sampling == SamplingFirst || *loc.Sampling == SamplingEither
		case InterpolationPerVertex:
			valid = true // Center only, checked above
		}
		if !valid {
			return fmt.Sprintf("Cannot combine %s interpolation with the %s sample type", *loc.Interpolation, *loc.Sampling)
		}
	}

	var needsInterpolation bool
	switch stage {
	case StageVertex, StageMesh:
		needsInterpolation = output
	case StageFragment:
		needsInterpolation = !output && !loc.PerPrimitive
	}
	if perVertex || !needsInterpolation {
		return ""
	}
	var kind ScalarKind
	switch t := v.module.Types[ty].Inner.(type) {
	case ScalarType:
		kind = t.Kind
	case VectorType:
		kind = t.Scalar.Kind
	case MatrixType:
		kind = t.Scalar.Kind
	default:
		return ""
	}
	switch kind {
	case ScalarFloat:
		// The frontend applies the default interpolation to floats.
		if loc.Interpolation == nil {
			return "Interpolation must be specified on vertex shader outputs and fragment shader inputs"
		}
	case ScalarSint, ScalarUint:
		// Integers have no default interpolation: flat must be explicit.
		if loc.Interpolation == nil || *loc.Interpolation != InterpolationFlat {
			return "`@interpolate(flat)` must be explicitly specified for integer I/O"
		}
	}
	return ""
}

// hasPositionBuiltin checks if the function result contains @builtin(position).
// This can be either:
// 1. Direct binding on result: fn() -> @builtin(position) vec4<f32>
// 2. Struct member binding: fn() -> Struct { @builtin(position) pos: vec4<f32> }
func (v *validator) hasPositionBuiltin(result *FunctionResult) bool {
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
func (v *validator) structHasPositionBuiltin(typeHandle TypeHandle) bool {
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

func (v *validator) isValidTypeHandle(handle TypeHandle) bool {
	return int(handle) < len(v.module.Types)
}

func (v *validator) isValidConstantHandle(handle ConstantHandle) bool {
	return int(handle) < len(v.module.Constants)
}

func (v *validator) isValidGlobalVariableHandle(handle GlobalVariableHandle) bool {
	return int(handle) < len(v.module.GlobalVariables)
}

func (v *validator) isValidFunctionHandle(handle FunctionHandle) bool {
	return int(handle) < len(v.module.Functions)
}

func (v *validator) isValidExpressionHandle(handle ExpressionHandle) bool {
	if v.context.function == nil {
		return false
	}
	return int(handle) < len(v.context.function.Expressions)
}

func (v *validator) addError(msg string) {
	v.errors = append(v.errors, ValidationError{
		Message:   msg,
		Statement: -1,
	})
}

func (v *validator) addErrorInFunction(msg string) {
	v.errors = append(v.errors, ValidationError{
		Message:   msg,
		Function:  v.context.functionName,
		Statement: -1,
	})
}

func (v *validator) addErrorInExpression(handle ExpressionHandle, msg string) {
	v.errors = append(v.errors, ValidationError{
		Message:    msg,
		Function:   v.context.functionName,
		Expression: &handle,
		Statement:  -1,
	})
}

func (v *validator) addErrorInStatement(index int, msg string) {
	v.errors = append(v.errors, ValidationError{
		Message:   msg,
		Function:  v.context.functionName,
		Statement: index,
	})
}

// containsPendingArray reports a type that is, or holds, an array sized by
// an override.
func (v *validator) containsPendingArray(h TypeHandle, depth int) bool {
	if depth > 32 || !v.isValidTypeHandle(h) {
		return false
	}
	switch t := v.module.Types[h].Inner.(type) {
	case ArrayType:
		return t.Size.Pending != nil || v.containsPendingArray(t.Base, depth+1)
	case StructType:
		for _, m := range t.Members {
			if v.containsPendingArray(m.Type, depth+1) {
				return true
			}
		}
	}
	return false
}

// isSizedArrayPointer reports a pointer, in the function being validated,
// to an array that is not runtime-sized.
func (v *validator) isSizedArrayPointer(h ExpressionHandle) bool {
	fn := v.context.function
	if fn == nil || int(h) >= len(fn.ExpressionTypes) {
		return false
	}
	ptr, ok := TypeResInner(v.module, fn.ExpressionTypes[h]).(PointerType)
	if !ok || !v.isValidTypeHandle(ptr.Base) {
		return false
	}
	at, ok := v.module.Types[ptr.Base].Inner.(ArrayType)
	return ok && !at.Size.IsRuntimeSized()
}
