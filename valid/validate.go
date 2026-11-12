package valid

import (
	"errors"
	"fmt"

	"github.com/gogpu/naga/internal/typifier"
	"github.com/gogpu/naga/ir"
)

// Error is a validation error (Rust naga's ValidationError).
type Error struct {
	Message string
	// Optional context
	Function   string
	Expression *ir.ExpressionHandle
	Statement  int
}

// Error implements the error interface.
func (e Error) Error() string {
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
	module       *ir.Module
	capabilities Capabilities
	// missing collects every capability a check found the set lacking
	// (has, typeCapability), for MissingCapabilities.
	missing            Capabilities
	subgroupStages     ShaderStages
	subgroupOperations SubgroupOperationSet
	// functionStages are the stages each module function may run in, by
	// FunctionHandle, as far as validated (Rust's available_stages).
	functionStages []ShaderStages
	// functionSubgroupStages is the part of functionStages that comes from the
	// subgroup stage set alone (MissingCapabilities needs to tell a stage the
	// capabilities forbid from one an operation forbids).
	functionSubgroupStages []ShaderStages
	// validatedFunctions is how many module functions have their
	// functionStages.
	validatedFunctions int
	errors             []Error
	context            validationContext
}

// validationContext holds current validation context.
type validationContext struct {
	function       *ir.Function
	types          *typifier.Typifier // function's expression types; nil when it could not be typed
	functionName   string
	flow           controlFlowAbility
	expressionUsed map[ir.ExpressionHandle]bool
	// stages are the stages the function may run in: Rust naga's
	// FunctionInfo::available_stages, narrowed so far by the subgroup
	// operations and the calls.
	stages ShaderStages
	// subgroupStages is stages narrowed by the subgroup stage set only.
	subgroupStages ShaderStages
	// inScope holds, by handle, the expressions a statement may use at the
	// statement being validated (Rust naga's valid_expression_set);
	// scopeList is the order they came into scope, so that a block can
	// drop its own (valid_expression_list).
	inScope   []bool
	scopeList []ir.ExpressionHandle
}

// allStages is Rust's ShaderStages::all(): a function that does nothing
// stage-specific runs in any stage.
const allStages = ^ShaderStages(0)

// maxTypeNesting bounds the walks through a type's members and elements, so a
// malformed module with a cycle in its type arena cannot recurse forever.
const maxTypeNesting = 32

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

// Options configures Validate. The zero value is WebGPU core: no
// optional capability.
type Options struct {
	// Capabilities are the optional features the module may use; a type or
	// operation that needs one outside the set is a validation error. This
	// is the single gate for capabilities: neither the frontends nor the
	// backends check them.
	Capabilities Capabilities

	// SubgroupStages are the stages a subgroup operation may run in (Rust
	// naga's Validator::subgroup_stages). Zero derives them from
	// Capabilities, as Rust's Validator::new: CapSubgroup gives the fragment
	// and compute-like stages, CapSubgroupVertexStage adds the vertex stage.
	// A non-zero set replaces the derived one. An entry point whose stage
	// is outside the set and that reaches a subgroup operation is an error.
	SubgroupStages ShaderStages

	// SubgroupOperations are the subgroup operations the module may use
	// (Rust naga's Validator::subgroup_operations). Zero derives them from
	// Capabilities, as Rust's Validator::new: CapSubgroup gives every
	// operation, none without it. A non-zero set replaces the derived one.
	// Every subgroup operation still needs CapSubgroup.
	SubgroupOperations SubgroupOperationSet
}

// Errors is the error Validate returns: every problem it found,
// where Rust naga stops at the first. errors.As finds an *Error.
type Errors []Error

// Error reports the first error and how many follow.
func (e Errors) Error() string {
	switch len(e) {
	case 0:
		return "no validation errors"
	case 1:
		return e[0].Error()
	}
	return fmt.Sprintf("%s (and %d more)", e[0].Error(), len(e)-1)
}

// Unwrap returns each error, for errors.As and errors.Is.
func (e Errors) Unwrap() []error {
	errs := make([]error, len(e))
	for i := range e {
		errs[i] = &e[i]
	}
	return errs
}

// Validate checks the module for correctness (Rust naga's
// Validator::validate) and returns what a WebGPU implementation needs from
// it: per function and entry point, the globals it uses and how. A module
// with problems returns an Errors.
func Validate(module *ir.Module, opts Options) (ModuleInfo, error) {
	if module == nil {
		return ModuleInfo{}, fmt.Errorf("module is nil")
	}

	v := &validator{
		module:                 module,
		capabilities:           opts.Capabilities,
		functionStages:         make([]ShaderStages, len(module.Functions)),
		functionSubgroupStages: make([]ShaderStages, len(module.Functions)),
		errors:                 make([]Error, 0),
	}
	v.subgroupStages, v.subgroupOperations = subgroupSets(opts)

	v.validateModule()

	if len(v.errors) > 0 {
		return ModuleInfo{}, Errors(v.errors)
	}
	info := analyzeModule(module)
	v.validateEntryPointBindings(info)
	if len(v.errors) > 0 {
		return ModuleInfo{}, Errors(v.errors)
	}
	return info, nil
}

// has reports whether the capability set contains c, and remembers c as
// missing when it does not. Every check that gates a feature on a
// capability asks only when the module uses the feature, so the remembered
// set is what the module needs and the set lacks.
func (v *validator) has(c Capabilities) bool {
	if v.capabilities.Contains(c) {
		return true
	}
	v.missing |= c
	return false
}

// validateEntryPointBindings rejects two resources at one @group/@binding
// among the globals one entry point uses, callees included, as Rust naga's
// EntryPointError::BindingCollision (valid/interface.rs). Separate entry
// points may reuse a binding: each is its own pipeline.
func (v *validator) validateEntryPointBindings(info ModuleInfo) {
	for i := range v.module.EntryPoints {
		ep := &v.module.EntryPoints[i]
		seen := make(map[ir.ResourceBinding]bool)
		immediates := 0
		for h, use := range info.EntryPoints[i].GlobalUses {
			gv := &v.module.GlobalVariables[h]
			if use != 0 && gv.Space == ir.SpaceImmediate {
				// Rust's EntryPointError::MoreThanOneImmediateUsed, checked
				// before the bindings as Rust does.
				immediates++
				if immediates == 2 {
					v.entryPointError(ep, "More than 1 immediate data variable is used")
				}
			}
			if use == 0 || gv.Binding == nil {
				continue
			}
			if seen[*gv.Binding] {
				v.entryPointError(ep, fmt.Sprintf("Bindings for [%d] conflict with other resource", h))
			}
			seen[*gv.Binding] = true
		}
	}
}

// validateModule validates the complete module.
func (v *validator) validateModule() {
	// Validate types
	v.validateTypes()

	// Validate constants
	v.validateConstants()
	v.validateGlobalExpressionLiterals()

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
		v.validateType(ir.TypeHandle(i), &typ)
	}
}

// checkScalarCapability validates that the given scalar type is allowed by the
// current capabilities. Mirrors Rust naga valid::type.rs check_width().
//
// Width=8 (64-bit) requires CapFloat64 (for f64) or CapShaderInt64 (for i64/u64).
// Width=2 (16-bit) requires CapShaderFloat16 (for f16).
// Width=4 and width=1 (bool) are always allowed.
func (v *validator) checkScalarCapability(handle ir.TypeHandle, scalar ir.ScalarType, context string) {
	switch {
	case scalar.Width == 8 && scalar.Kind == ir.ScalarFloat:
		if !v.has(CapFloat64) {
			v.addError(fmt.Sprintf("type [%d]%s: f64 requires the FLOAT64 capability", handle, context))
		}
	case scalar.Width == 8 && (scalar.Kind == ir.ScalarSint || scalar.Kind == ir.ScalarUint):
		name := "i64"
		if scalar.Kind == ir.ScalarUint {
			name = "u64"
		}
		if !v.has(CapShaderInt64) {
			v.addError(fmt.Sprintf("type [%d]%s: %s requires the SHADER_INT64 capability", handle, context, name))
		}
	case scalar.Width == 2 && scalar.Kind == ir.ScalarFloat:
		if !v.has(CapShaderFloat16) {
			v.addError(fmt.Sprintf("type [%d]%s: f16 requires the SHADER_FLOAT16 capability", handle, context))
		}
	case scalar.Width == 2 && (scalar.Kind == ir.ScalarSint || scalar.Kind == ir.ScalarUint):
		name := "i16"
		if scalar.Kind == ir.ScalarUint {
			name = "u16"
		}
		if !v.has(CapShaderInt16) {
			v.addError(fmt.Sprintf("type [%d]%s: %s requires the SHADER_INT16 capability", handle, context, name))
		}
	}
}

// typeCapability reports a type that needs a capability outside the set:
// Rust naga's TypeError::MissingCapability (valid/type.rs) under
// ValidationError::Type.
func (v *validator) typeCapability(handle ir.TypeHandle, c Capabilities) {
	v.missing |= c
	v.addError(fmt.Sprintf("Type [%d] '%s' is invalid: Capability %s is required",
		handle, v.module.Types[handle].Name, c.rustDebug()))
}

// checkTypeCapabilities checks the capabilities a type needs beyond its
// scalar width (valid/type.rs validate_type): 64-bit and f32 atomics, ray
// queries and acceleration structures, @blend_src members.
func (v *validator) checkTypeCapabilities(handle ir.TypeHandle, inner ir.TypeInner) {
	switch t := inner.(type) {
	case ir.AtomicType:
		switch {
		case (t.Scalar.Kind == ir.ScalarSint || t.Scalar.Kind == ir.ScalarUint) && t.Scalar.Width == 8:
			if v.capabilities&(CapShaderInt64AtomicAllOps|CapShaderInt64AtomicMinMax) == 0 {
				v.typeCapability(handle, CapShaderInt64AtomicAllOps)
			}
		case t.Scalar.Kind == ir.ScalarFloat && t.Scalar.Width == 4:
			if !v.has(CapShaderFloat32Atomic) {
				v.typeCapability(handle, CapShaderFloat32Atomic)
			}
		}
	case ir.RayQueryType:
		if !v.has(CapRayQuery) {
			v.typeCapability(handle, CapRayQuery)
		}
	case ir.AccelerationStructureType:
		// RAY_TRACING_PIPELINE or RAY_QUERY; Rust reports the second.
		if !v.capabilities.Contains(capRayTracingPipeline) && !v.has(CapRayQuery) {
			v.typeCapability(handle, CapRayQuery)
		}
	case ir.StructType:
		for _, m := range t.Members {
			if m.Binding == nil {
				continue
			}
			if loc, ok := (*m.Binding).(ir.LocationBinding); ok && loc.BlendSrc != nil &&
				!v.has(CapDualSourceBlending) {
				v.typeCapability(handle, CapDualSourceBlending)
				return
			}
		}
	}
}

// validateType validates a single type.
func (v *validator) validateType(handle ir.TypeHandle, typ *ir.Type) {
	if typ.Inner == nil {
		v.addError(fmt.Sprintf("type %d has nil inner type", handle))
		return
	}
	v.checkTypeCapabilities(handle, typ.Inner)

	switch inner := typ.Inner.(type) {
	case ir.ScalarType:
		// Scalar types are always valid
		if inner.Width != 1 && inner.Width != 2 && inner.Width != 4 && inner.Width != 8 {
			v.addError(fmt.Sprintf("type %d: scalar width must be 1, 2, 4, or 8 bytes, got %d", handle, inner.Width))
		}
		v.checkScalarCapability(handle, inner, "")

	case ir.VectorType:
		// Vector size must be 2, 3, or 4
		if inner.Size != ir.Vec2 && inner.Size != ir.Vec3 && inner.Size != ir.Vec4 {
			v.addError(fmt.Sprintf("type %d: vector size must be 2, 3, or 4, got %d", handle, inner.Size))
		}
		// Scalar width validation
		if inner.Scalar.Width != 1 && inner.Scalar.Width != 2 && inner.Scalar.Width != 4 && inner.Scalar.Width != 8 {
			v.addError(fmt.Sprintf("type %d: vector scalar width must be 1, 2, 4, or 8 bytes, got %d", handle, inner.Scalar.Width))
		}
		v.checkScalarCapability(handle, inner.Scalar, "")

	case ir.MatrixType:
		// Matrix dimensions must be 2, 3, or 4
		if inner.Columns != ir.Vec2 && inner.Columns != ir.Vec3 && inner.Columns != ir.Vec4 {
			v.addError(fmt.Sprintf("type %d: matrix columns must be 2, 3, or 4, got %d", handle, inner.Columns))
		}
		if inner.Rows != ir.Vec2 && inner.Rows != ir.Vec3 && inner.Rows != ir.Vec4 {
			v.addError(fmt.Sprintf("type %d: matrix rows must be 2, 3, or 4, got %d", handle, inner.Rows))
		}
		// Matrix scalar must be float
		if inner.Scalar.Kind != ir.ScalarFloat {
			v.addError(fmt.Sprintf("type %d: matrix scalar must be float, got %v", handle, inner.Scalar.Kind))
		}
		v.checkScalarCapability(handle, inner.Scalar, "")

	case ir.ArrayType:
		// Base type must exist
		if !v.isValidTypeHandle(inner.Base) {
			v.addError(fmt.Sprintf("type %d: array base type %d does not exist", handle, inner.Base))
		}
		// Check for potential circular reference (simplified check)
		if inner.Base == handle {
			v.addError(fmt.Sprintf("type %d: array has circular reference to itself", handle))
		}

	case ir.StructType:
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

	case ir.PointerType:
		// Base type must exist
		if !v.isValidTypeHandle(inner.Base) {
			v.addError(fmt.Sprintf("type %d: pointer base type %d does not exist", handle, inner.Base))
		}

	case ir.AtomicType:
		// Atomic types embed a scalar directly — check capability.
		v.checkScalarCapability(handle, inner.Scalar, "")

	case ir.SamplerType:
		// Sampler types are always valid

	case ir.ImageType:
		if inner.Class == ir.ImageClassStorage && inner.StorageFormat == ir.StorageFormatUndefined {
			v.addError(fmt.Sprintf("type %d: storage image has no format (StorageFormatUndefined)", handle))
		}
		if inner.Class == ir.ImageClassStorage && inner.StorageAccess == 0 {
			v.addError(fmt.Sprintf("type %d: storage image has no access (empty StorageAccess)", handle))
		}
	}
}

// validateConstants checks all constants: the type and the init expression,
// the constant's only value, exist (Rust naga's handle validation of
// Constant { ty, init }).
func (v *validator) validateConstants() {
	for i, c := range v.module.Constants {
		if !v.isValidTypeHandle(c.Type) {
			v.addError(fmt.Sprintf("constant %d (%s): type %d does not exist", i, c.Name, c.Type))
		}
		if int(c.Init) >= len(v.module.GlobalExpressions) {
			v.addError(fmt.Sprintf("constant %d (%s): init expression %d does not exist", i, c.Name, c.Init))
		}
	}
}

// validateGlobalExpressionLiterals checks the width of every literal among
// the global expressions against the capabilities, as Rust naga's
// validate_const_expression does (ValidationError::ConstExpression).
func (v *validator) validateGlobalExpressionLiterals() {
	for i, e := range v.module.GlobalExpressions {
		if lit, ok := e.Kind.(ir.Literal); ok {
			if msg := v.literalWidthError(lit.Value); msg != "" {
				v.addError(fmt.Sprintf("Constant expression [%d] is invalid: %s", i, msg))
			}
		}
	}
}

// literalWidthError returns Rust naga's WidthError::MissingCapability text
// for a literal whose scalar needs a capability outside the set (Rust's
// validate_literal, check_width), or "".
func (v *validator) literalWidthError(lit ir.LiteralValue) string {
	var name, flag string
	var need Capabilities
	switch lit.(type) {
	case ir.LiteralF64:
		name, flag, need = "f64", "FLOAT64", CapFloat64
	case ir.LiteralF16:
		// Rust's text names the flag FLOAT16.
		name, flag, need = "f16", "FLOAT16", CapShaderFloat16
	case ir.LiteralI16:
		name, flag, need = "i16", "SHADER_INT16", CapShaderInt16
	case ir.LiteralU16:
		name, flag, need = "u16", "SHADER_INT16", CapShaderInt16
	case ir.LiteralI64:
		name, flag, need = "i64", "SHADER_INT64", CapShaderInt64
	case ir.LiteralU64:
		name, flag, need = "u64", "SHADER_INT64", CapShaderInt64
	default:
		return ""
	}
	if v.has(need) {
		return ""
	}
	// %#q backquotes, as Rust's text does: Using `u64` values ...
	return fmt.Sprintf("Using %#q values requires the %#q flag", name, "naga::valid::Capabilities::"+flag)
}

// validateGlobalVariables checks all global variables.
func (v *validator) validateGlobalVariables() {
	names := make(map[string]bool)

	for i, gv := range v.module.GlobalVariables {
		if gv.Name != "" {
			if names[gv.Name] {
				v.addError(fmt.Sprintf("duplicate global variable name %q", gv.Name))
			}
			names[gv.Name] = true
		}

		if c := v.globalVariableCapability(&v.module.GlobalVariables[i]); c != 0 {
			v.addError(fmt.Sprintf("Global variable [%d] '%s' is invalid: Capability %s is not supported", i, gv.Name, c.rustDebug()))
		}

		if !v.isValidTypeHandle(gv.Type) {
			v.addError(fmt.Sprintf("global variable %d (%s): type %d does not exist", i, gv.Name, gv.Type))
		} else if gv.Space != ir.SpaceWorkGroup && v.containsPendingArray(gv.Type, 0) {
			// Rust: the type lacks CREATION_RESOLVED, which every address
			// space but workgroup requires.
			v.addError(fmt.Sprintf("Global variable [%d] '%s' is invalid: an array sized by an override is allowed only in the workgroup address space", i, gv.Name))
		} else if gv.Space == ir.SpaceImmediate && !v.immediateCompatible(gv.Type, 0) {
			// Rust's GlobalVariableError::InvalidImmediateType text.
			v.addError(fmt.Sprintf("Global variable [%d] '%s' is invalid: Type is not valid for use as a immediate data", i, gv.Name))
		}

		// Rust keeps the access inside AddressSpace::Storage (ADR-010 E2).
		switch {
		case gv.Space == ir.SpaceStorage && gv.Access == 0:
			v.addError(fmt.Sprintf("global variable %q: storage variable has no access (empty StorageAccess)", gv.Name))
		case gv.Space == ir.SpaceStorage && !gv.Access.Contains(ir.StorageAccessLoad):
			// Rust's GlobalVariableError text.
			v.addError(fmt.Sprintf("global variable %q: Storage address space doesn't support write-only access", gv.Name))
		case gv.Space != ir.SpaceStorage && gv.Access != 0:
			v.addError(fmt.Sprintf("global variable %q: only a storage variable has an access", gv.Name))
		}

		if gv.Init != nil {
			if !v.isValidConstantHandle(*gv.Init) {
				v.addError(fmt.Sprintf("global variable %q: init constant %d does not exist", gv.Name, *gv.Init))
			}
		}
	}
}

// globalVariableCapability returns the capability a global variable needs
// and the set lacks, or 0: the capability part of Rust naga's
// validate_global_var (valid/interface.rs). A binding array needs the
// capability of its resource kind and is then checked as its element.
func (v *validator) globalVariableCapability(gv *ir.GlobalVariable) Capabilities {
	if !v.isValidTypeHandle(gv.Type) {
		return 0
	}
	missing := func(c Capabilities) Capabilities {
		if v.has(c) {
			return 0
		}
		return c
	}
	inner := gv.Type
	if ba, ok := v.module.Types[gv.Type].Inner.(ir.BindingArrayType); ok {
		if !v.isValidTypeHandle(ba.Base) {
			return 0
		}
		inner = ba.Base
		var need Capabilities
		switch gv.Space {
		case ir.SpaceStorage:
			need = CapStorageBufferBindingArray
		case ir.SpaceUniform:
			need = CapBufferBindingArray
		case ir.SpaceHandle:
			switch base := v.module.Types[ba.Base].Inner.(type) {
			case ir.ImageType:
				switch base.Class {
				case ir.ImageClassStorage:
					need = CapStorageTextureBindingArray
				case ir.ImageClassSampled, ir.ImageClassDepth:
					need = CapTextureAndSamplerBindingArray
				}
			case ir.SamplerType:
				need = CapTextureAndSamplerBindingArray
			case ir.AccelerationStructureType:
				need = CapAccelerationStructureBindingArray
			}
		}
		if c := missing(need); c != 0 {
			return c
		}
	}
	switch gv.Space {
	case ir.SpaceHandle:
		if img, ok := v.module.Types[inner].Inner.(ir.ImageType); ok && img.Class == ir.ImageClassStorage {
			switch img.StorageFormat {
			case ir.StorageFormatR16Unorm, ir.StorageFormatR16Snorm, ir.StorageFormatRg16Unorm,
				ir.StorageFormatRg16Snorm, ir.StorageFormatRgba16Unorm, ir.StorageFormatRgba16Snorm:
				return missing(CapStorageTexture16BitNormFormats)
			}
		}
	case ir.SpaceTaskPayload:
		return missing(CapMeshShader)
	}
	return 0
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
			expressionUsed: make(map[ir.ExpressionHandle]bool),
		}

		v.validateFunction(fn)
		v.functionStages[i] = v.context.stages
		v.functionSubgroupStages[i] = v.context.subgroupStages
		v.validatedFunctions = i + 1
	}
}

// validateFunction validates a single function.
//
// Its expressions are typed first, as Rust's analyzer types each one before
// the checks that read the types. A function that cannot be typed is
// reported as Rust reports it (FunctionError::Expression, "Type resolution
// failed"), unless the function has other errors: those (a handle out of
// range, say) are what keeps it from being typed, and they are what the
// validator has always reported.
func (v *validator) validateFunction(fn *ir.Function) {
	types, typeErr := typifier.Function(v.module, fn)
	v.context.types = types
	v.context.stages = allStages
	v.context.subgroupStages = allStages
	// The expressions evaluated before the function runs are in scope
	// everywhere; every other one comes into scope at its Emit or at the
	// statement that gives it its value (Rust's validate_function).
	v.context.inScope = make([]bool, len(fn.Expressions))
	v.context.scopeList = v.context.scopeList[:0]
	for i := range fn.Expressions {
		v.context.inScope[i] = fn.Expressions[i].Kind != nil && fn.Expressions[i].NeedsPreEmit()
	}
	errorsBefore := len(v.errors)

	// Validate arguments
	for i, arg := range fn.Arguments {
		if !v.isValidTypeHandle(arg.Type) {
			v.addErrorInFunction(fmt.Sprintf("argument %d (%s): type %d does not exist", i, arg.Name, arg.Type))
			continue
		}
		v.validateArgumentPointerSpace(i, &arg)
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
		v.validateExpression(ir.ExpressionHandle(i), &expr)
		v.context.stages &= v.expressionStages(expr.Kind)
		v.narrowSubgroupStages(expr.Kind)
	}

	// Validate body
	v.validateBlock(fn.Body)

	var te *typifier.Error
	if typeErr != nil && len(v.errors) == errorsBefore && errors.As(typeErr, &te) {
		v.addErrorInExpression(te.Expression, "Type resolution failed: "+te.Err.Error())
	}
}

// validateArgumentPointerSpace rejects a pointer argument outside the
// function and private address spaces, as Rust naga's validate_function
// (valid/function.rs, FunctionError::InvalidArgumentPointerSpace). The WGSL
// spec lifts the restriction only under the unrestricted_pointer_parameters
// language extension, which neither Rust naga nor naga implements, so the
// check is unconditional, as in Rust. Without it a storage or workgroup
// pointer reaches backends that cannot express it (the MSL runtime-array
// bound needs the originating global, P0-MSL-100).
func (v *validator) validateArgumentPointerSpace(index int, arg *ir.FunctionArgument) {
	var space ir.AddressSpace
	switch t := v.module.Types[arg.Type].Inner.(type) {
	case ir.PointerType:
		space = t.Space
	case ir.ValuePointerType:
		space = t.Space
	default:
		return
	}
	if space == ir.SpaceFunction || space == ir.SpacePrivate {
		return
	}
	v.addErrorInFunction(fmt.Sprintf("Argument '%s' at index %d is a pointer of space %s, which can't be passed into functions.",
		arg.Name, index, addressSpaceDebug(space)))
}

// validateExpression validates a single expression.
func (v *validator) validateExpression(handle ir.ExpressionHandle, expr *ir.Expression) {
	if expr.Kind == nil {
		v.addErrorInExpression(handle, "expression has nil kind")
		return
	}

	switch kind := expr.Kind.(type) {
	case ir.Literal:
		if msg := v.literalWidthError(kind.Value); msg != "" {
			v.addErrorInExpression(handle, msg)
		}

	case ir.ExprConstant:
		if !v.isValidConstantHandle(kind.Constant) {
			v.addErrorInExpression(handle, fmt.Sprintf("constant %d does not exist", kind.Constant))
		}

	case ir.ExprZeroValue:
		if !v.isValidTypeHandle(kind.Type) {
			v.addErrorInExpression(handle, fmt.Sprintf("type %d does not exist", kind.Type))
		}

	case ir.ExprCompose:
		if !v.isValidTypeHandle(kind.Type) {
			v.addErrorInExpression(handle, fmt.Sprintf("type %d does not exist", kind.Type))
		}
		for i, comp := range kind.Components {
			if !v.isValidExpressionHandle(comp) {
				v.addErrorInExpression(handle, fmt.Sprintf("component %d: expression %d does not exist", i, comp))
			}
		}

	case ir.ExprAccess:
		if !v.isValidExpressionHandle(kind.Base) {
			v.addErrorInExpression(handle, fmt.Sprintf("base expression %d does not exist", kind.Base))
		}
		if !v.isValidExpressionHandle(kind.Index) {
			v.addErrorInExpression(handle, fmt.Sprintf("index expression %d does not exist", kind.Index))
		}

	case ir.ExprAccessIndex:
		if !v.isValidExpressionHandle(kind.Base) {
			v.addErrorInExpression(handle, fmt.Sprintf("base expression %d does not exist", kind.Base))
		}

	case ir.ExprSplat:
		if kind.Size != ir.Vec2 && kind.Size != ir.Vec3 && kind.Size != ir.Vec4 {
			v.addErrorInExpression(handle, fmt.Sprintf("splat size must be 2, 3, or 4, got %d", kind.Size))
		}
		if !v.isValidExpressionHandle(kind.Value) {
			v.addErrorInExpression(handle, fmt.Sprintf("value expression %d does not exist", kind.Value))
		}

	case ir.ExprSwizzle:
		if kind.Size != ir.Vec2 && kind.Size != ir.Vec3 && kind.Size != ir.Vec4 {
			v.addErrorInExpression(handle, fmt.Sprintf("swizzle size must be 2, 3, or 4, got %d", kind.Size))
		}
		if !v.isValidExpressionHandle(kind.Vector) {
			v.addErrorInExpression(handle, fmt.Sprintf("vector expression %d does not exist", kind.Vector))
		}
		for i := 0; i < int(kind.Size); i++ {
			if kind.Pattern[i] > ir.SwizzleW {
				v.addErrorInExpression(handle, fmt.Sprintf("pattern[%d] invalid component %d", i, kind.Pattern[i]))
			}
		}

	case ir.ExprFunctionArgument:
		if v.context.function != nil {
			if int(kind.Index) >= len(v.context.function.Arguments) {
				v.addErrorInExpression(handle, fmt.Sprintf("argument index %d out of range (function has %d args)",
					kind.Index, len(v.context.function.Arguments)))
			}
		}

	case ir.ExprGlobalVariable:
		if !v.isValidGlobalVariableHandle(kind.Variable) {
			v.addErrorInExpression(handle, fmt.Sprintf("global variable %d does not exist", kind.Variable))
		}

	case ir.ExprLocalVariable:
		if v.context.function != nil {
			if int(kind.Variable) >= len(v.context.function.LocalVars) {
				v.addErrorInExpression(handle, fmt.Sprintf("local variable index %d out of range (function has %d vars)",
					kind.Variable, len(v.context.function.LocalVars)))
			}
		}

	case ir.ExprLoad:
		if !v.isValidExpressionHandle(kind.Pointer) {
			v.addErrorInExpression(handle, fmt.Sprintf("pointer expression %d does not exist", kind.Pointer))
		}

	case ir.ExprImageSample:
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

	case ir.ExprImageLoad:
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

	case ir.ExprImageQuery:
		if !v.isValidExpressionHandle(kind.Image) {
			v.addErrorInExpression(handle, fmt.Sprintf("image expression %d does not exist", kind.Image))
		}

	case ir.ExprUnary:
		if !v.isValidExpressionHandle(kind.Expr) {
			v.addErrorInExpression(handle, fmt.Sprintf("operand expression %d does not exist", kind.Expr))
		} else {
			v.checkUnaryOperand(handle, kind)
		}

	case ir.ExprBinary:
		if !v.isValidExpressionHandle(kind.Left) {
			v.addErrorInExpression(handle, fmt.Sprintf("left expression %d does not exist", kind.Left))
		}
		if !v.isValidExpressionHandle(kind.Right) {
			v.addErrorInExpression(handle, fmt.Sprintf("right expression %d does not exist", kind.Right))
		}
		if v.isValidExpressionHandle(kind.Left) && v.isValidExpressionHandle(kind.Right) {
			v.checkBinaryOperands(handle, kind)
		}

	case ir.ExprSelect:
		if !v.isValidExpressionHandle(kind.Condition) {
			v.addErrorInExpression(handle, fmt.Sprintf("condition expression %d does not exist", kind.Condition))
		}
		if !v.isValidExpressionHandle(kind.Accept) {
			v.addErrorInExpression(handle, fmt.Sprintf("accept expression %d does not exist", kind.Accept))
		}
		if !v.isValidExpressionHandle(kind.Reject) {
			v.addErrorInExpression(handle, fmt.Sprintf("reject expression %d does not exist", kind.Reject))
		}
		if v.isValidExpressionHandle(kind.Condition) && v.isValidExpressionHandle(kind.Accept) && v.isValidExpressionHandle(kind.Reject) {
			v.checkSelectTypes(handle, kind)
		}

	case ir.ExprDerivative:
		if !v.isValidExpressionHandle(kind.Expr) {
			v.addErrorInExpression(handle, fmt.Sprintf("expression %d does not exist", kind.Expr))
		}

	case ir.ExprRelational:
		if !v.isValidExpressionHandle(kind.Argument) {
			v.addErrorInExpression(handle, fmt.Sprintf("argument expression %d does not exist", kind.Argument))
		}

	case ir.ExprMath:
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

	case ir.ExprAs:
		if !v.isValidExpressionHandle(kind.Expr) {
			v.addErrorInExpression(handle, fmt.Sprintf("expression %d does not exist", kind.Expr))
		}

	case ir.ExprCallResult:
		if !v.isValidFunctionHandle(kind.Function) {
			v.addErrorInExpression(handle, fmt.Sprintf("function %d does not exist", kind.Function))
		}

	case ir.ExprArrayLength:
		if !v.isValidExpressionHandle(kind.Array) {
			v.addErrorInExpression(handle, fmt.Sprintf("array expression %d does not exist", kind.Array))
		} else if v.isSizedArrayPointer(kind.Array) {
			// Rust: arrayLength needs a runtime-sized array.
			v.addErrorInExpression(handle, fmt.Sprintf("Array length of [%d] can't be done", kind.Array))
		}

	case ir.ExprRayQueryProceedResult:
		// No fields to validate — result is produced by StmtRayQuery with Proceed function

	case ir.ExprRayQueryGetIntersection:
		if !v.isValidExpressionHandle(kind.Query) {
			v.addErrorInExpression(handle, fmt.Sprintf("ray query expression %d does not exist", kind.Query))
		}

	case ir.ExprSubgroupBallotResult:
		// No fields to validate — result is produced by StmtSubgroupBallot

	case ir.ExprSubgroupOperationResult:
		if !v.isValidTypeHandle(kind.Type) {
			v.addErrorInExpression(handle, fmt.Sprintf("type %d does not exist", kind.Type))
		}

	case ir.ExprOverride, ir.ExprAtomicResult, ir.ExprWorkGroupUniformLoadResult:
		// No operand expression; this switch checks nothing more for them.

	default:
		// A kind of no frontend, such as the DXIL backend's SSA kinds,
		// which exist only in that backend's own copy of a module.
		v.addErrorInExpression(handle, fmt.Sprintf("unknown expression kind %T", kind))
	}
}

// validateBlock validates a block of statements; the expressions it brings
// into scope leave scope at its end (Rust's validate_block).
func (v *validator) validateBlock(block ir.Block) {
	mark := len(v.context.scopeList)
	v.validateBlockImpl(block)
	v.dropScope(mark)
}

// validateBlockImpl validates a block of statements and leaves what it
// brought into scope in scope (Rust's validate_block_impl).
func (v *validator) validateBlockImpl(block ir.Block) {
	for i, stmt := range block {
		v.validateStatement(i, &stmt)
	}
}

// validateBlockWith validates a block whose statements may leave it as
// flow allows.
func (v *validator) validateBlockWith(block ir.Block, flow controlFlowAbility) {
	saved := v.context.flow
	v.context.flow = flow
	v.validateBlock(block)
	v.context.flow = saved
}

// dropScope takes out of scope what came into it after mark.
func (v *validator) dropScope(mark int) {
	for _, h := range v.context.scopeList[mark:] {
		v.context.inScope[h] = false
	}
	v.context.scopeList = v.context.scopeList[:mark]
}

// notInScopeText is Rust naga's ExpressionError::NotInScope.
const notInScopeText = "Used by a statement before it was introduced into the scope by any of the dominating blocks"

// inScope reports whether a statement may use h here. A handle out of range
// is reported by the handle checks, not as out of scope.
func (v *validator) inScope(h ir.ExpressionHandle) bool {
	return int(h) >= len(v.context.inScope) || v.context.inScope[h]
}

// useExpression checks that a statement's operand is in scope: an
// expression emitted before it in its block or a block around it, the
// result of a statement before it, or one evaluated before the function
// runs (Rust's FunctionError::Expression { source: NotInScope }).
func (v *validator) useExpression(h ir.ExpressionHandle) {
	if !v.inScope(h) {
		v.addErrorInExpression(h, notInScopeText)
	}
}

// introduce brings h into scope, at its Emit or at the statement that gives
// it its value; already in scope, it is Rust's
// FunctionError::ExpressionAlreadyInScope.
func (v *validator) introduce(h ir.ExpressionHandle) {
	if int(h) >= len(v.context.inScope) {
		return
	}
	if v.context.inScope[h] {
		v.addErrorInExpression(h, fmt.Sprintf("Expression [%d] can't be introduced - it's already in scope", h))
		return
	}
	v.context.inScope[h] = true
	v.context.scopeList = append(v.context.scopeList, h)
}

// validateStatement validates a single statement.
func (v *validator) validateStatement(index int, stmt *ir.Statement) {
	if stmt.Kind == nil {
		v.addErrorInStatement(index, "statement has nil kind")
		return
	}

	switch kind := stmt.Kind.(type) {
	case ir.StmtEmit:
		// Validate range
		if v.context.function != nil {
			exprCount := uint32(len(v.context.function.Expressions))
			if kind.Range.Start >= ir.ExpressionHandle(exprCount) {
				v.addErrorInStatement(index, fmt.Sprintf("emit range start %d out of range", kind.Range.Start))
			}
			if kind.Range.End > ir.ExpressionHandle(exprCount) {
				v.addErrorInStatement(index, fmt.Sprintf("emit range end %d out of range", kind.Range.End))
			}
			if kind.Range.Start >= kind.Range.End {
				v.addErrorInStatement(index, fmt.Sprintf("emit range start %d >= end %d", kind.Range.Start, kind.Range.End))
			}
			for h := kind.Range.Start; h < kind.Range.End && h < ir.ExpressionHandle(exprCount); h++ {
				v.introduce(h)
			}
		}

	case ir.StmtBlock:
		v.validateBlock(kind.Block)

	case ir.StmtIf:
		if !v.isValidExpressionHandle(kind.Condition) {
			v.addErrorInStatement(index, fmt.Sprintf("condition expression %d does not exist", kind.Condition))
		}
		v.useExpression(kind.Condition)
		v.validateBlock(kind.Accept)
		v.validateBlock(kind.Reject)

	case ir.StmtSwitch:
		if !v.isValidExpressionHandle(kind.Selector) {
			v.addErrorInStatement(index, fmt.Sprintf("selector expression %d does not exist", kind.Selector))
		}
		v.useExpression(kind.Selector)
		hasDefault := false
		for _, c := range kind.Cases {
			if _, ok := c.Value.(ir.SwitchValueDefault); ok {
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

	case ir.StmtLoop:
		// The continuing block and the break-if see the body's scope.
		mark := len(v.context.scopeList)
		saved := v.context.flow
		v.context.flow = saved&abilityReturn | abilityBreak | abilityContinue
		v.validateBlockImpl(kind.Body)
		v.context.flow = 0
		v.validateBlockImpl(kind.Continuing)
		v.context.flow = saved

		if kind.BreakIf != nil {
			if !v.isValidExpressionHandle(*kind.BreakIf) {
				v.addErrorInStatement(index, fmt.Sprintf("break-if expression %d does not exist", *kind.BreakIf))
			}
			v.useExpression(*kind.BreakIf)
		}
		v.dropScope(mark)

	// The messages are Rust naga's FunctionError texts.
	case ir.StmtBreak:
		if v.context.flow&abilityBreak == 0 {
			v.addErrorInStatement(index, "The `break` is used outside of a `loop` or `switch` context")
		}

	case ir.StmtContinue:
		if v.context.flow&abilityContinue == 0 {
			v.addErrorInStatement(index, "The `continue` is used outside of a `loop` context")
		}

	case ir.StmtReturn:
		if v.context.flow&abilityReturn == 0 {
			v.addErrorInStatement(index, "The `return` is called within a `continuing` block")
		}
		if kind.Value != nil {
			if !v.isValidExpressionHandle(*kind.Value) {
				v.addErrorInStatement(index, fmt.Sprintf("return value expression %d does not exist", *kind.Value))
			}
			v.useExpression(*kind.Value)
		}

	case ir.StmtKill:
		// Allowed anywhere in a fragment shader, a continuing block too
		// (Rust naga checks only the stage).
		v.context.stages &= ShaderStagesFragment

	case ir.StmtControlBarrier:
		v.context.stages &= ShaderStagesComputeLike
		v.checkBarrier(index, kind.Flags)

	case ir.StmtMemoryBarrier:
		v.context.stages &= ShaderStagesComputeLike
		v.checkBarrier(index, kind.Flags)

	case ir.StmtStore:
		if !v.isValidExpressionHandle(kind.Pointer) {
			v.addErrorInStatement(index, fmt.Sprintf("pointer expression %d does not exist", kind.Pointer))
		}
		if !v.isValidExpressionHandle(kind.Value) {
			v.addErrorInStatement(index, fmt.Sprintf("value expression %d does not exist", kind.Value))
		}
		// Rust checks the scope of the value only: the pointer is a chain
		// of accesses to a variable, walked without its types.
		v.useExpression(kind.Value)

	case ir.StmtImageAtomic:
		v.useExpression(kind.Coordinate)
		if kind.ArrayIndex != nil {
			v.useExpression(*kind.ArrayIndex)
		}
		v.checkImageAtomic(index, kind)
		v.useExpression(kind.Value)

	case ir.StmtSubgroupBallot, ir.StmtSubgroupCollectiveOperation, ir.StmtSubgroupGather:
		v.validateSubgroupStatement(index, kind)

	case ir.StmtImageStore:
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
		v.useExpression(kind.Coordinate)
		if kind.ArrayIndex != nil {
			v.useExpression(*kind.ArrayIndex)
		}
		v.useExpression(kind.Value)

	case ir.StmtAtomic:
		if !v.isValidExpressionHandle(kind.Pointer) {
			v.addErrorInStatement(index, fmt.Sprintf("pointer expression %d does not exist", kind.Pointer))
		}
		if !v.isValidExpressionHandle(kind.Value) {
			v.addErrorInStatement(index, fmt.Sprintf("value expression %d does not exist", kind.Value))
		}
		if kind.Result != nil && !v.isValidExpressionHandle(*kind.Result) {
			v.addErrorInStatement(index, fmt.Sprintf("result expression %d does not exist", *kind.Result))
		}
		v.useExpression(kind.Pointer)
		v.useExpression(kind.Value)
		v.checkAtomic(index, kind)
		if xchg, ok := kind.Fun.(ir.AtomicExchange); ok && xchg.Compare != nil {
			v.useExpression(*xchg.Compare)
		}
		if kind.Result != nil {
			v.introduce(*kind.Result)
		}

	case ir.StmtWorkGroupUniformLoad:
		v.context.stages &= ShaderStagesComputeLike
		if !v.isValidExpressionHandle(kind.Pointer) {
			v.addErrorInStatement(index, fmt.Sprintf("pointer expression %d does not exist", kind.Pointer))
		}
		if !v.isValidExpressionHandle(kind.Result) {
			v.addErrorInStatement(index, fmt.Sprintf("result expression %d does not exist", kind.Result))
		}
		v.useExpression(kind.Pointer)
		v.introduce(kind.Result)

	case ir.StmtCall:
		v.validateCall(index, kind)

	case ir.StmtRayQuery:
		if !v.isValidExpressionHandle(kind.Query) {
			v.addErrorInStatement(index, fmt.Sprintf("query expression %d does not exist", kind.Query))
		}
		switch fun := kind.Fun.(type) {
		case ir.RayQueryInitialize:
			v.useExpression(fun.AccelerationStructure)
			v.useExpression(fun.Descriptor)
		case ir.RayQueryProceed:
			v.introduce(fun.Result)
		case ir.RayQueryGenerateIntersection:
			v.useExpression(fun.HitT)
		}
	}
}

// validateCall checks a call: its handles, that its arguments are in
// scope, and its result comes into scope (Rust's validate_call). The
// caller can run only in the stages the callee can.
func (v *validator) validateCall(index int, call ir.StmtCall) {
	if !v.isValidFunctionHandle(call.Function) {
		v.addErrorInStatement(index, fmt.Sprintf("function %d does not exist", call.Function))
	}
	for i, arg := range call.Arguments {
		if !v.isValidExpressionHandle(arg) {
			v.addErrorInStatement(index, fmt.Sprintf("argument %d expression %d does not exist", i, arg))
		} else if !v.inScope(arg) {
			v.addErrorInExpression(arg, fmt.Sprintf("Call to [%d] is invalid: Argument %d expression is invalid: %s",
				call.Function, i, notInScopeText))
		}
	}
	if call.Result != nil {
		if !v.isValidExpressionHandle(*call.Result) {
			v.addErrorInStatement(index, fmt.Sprintf("result expression %d does not exist", *call.Result))
		} else if v.context.inScope[*call.Result] {
			v.addErrorInExpression(*call.Result, fmt.Sprintf("Call to [%d] is invalid: Result expression [%d] has already been introduced earlier",
				call.Function, *call.Result))
		} else {
			v.introduce(*call.Result)
		}
	}
	v.context.stages &= v.calleeStages(call.Function)
	v.narrowSubgroupStagesByCallee(call.Function)
}

// calleeStages are the stages a call of fn can run in. A function is
// validated before the functions after it, which are the only ones that
// may call it (Rust's handle order).
func (v *validator) calleeStages(fn ir.FunctionHandle) ShaderStages {
	if int(fn) < v.validatedFunctions {
		return v.functionStages[fn]
	}
	return allStages
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
		case ir.StageVertex:
			// Vertex shader must return @builtin(position)
			// Position can be either:
			// 1. Direct return: fn() -> @builtin(position) vec4<f32>
			// 2. Struct member: fn() -> VertexOutput { @builtin(position) pos: vec4<f32>, ... }
			if fn.Result == nil {
				v.addError(fmt.Sprintf("entry point %q (@vertex): must have a return value", ep.Name))
			} else if !v.hasPositionBuiltin(fn.Result) {
				v.addError(fmt.Sprintf("entry point %q (@vertex): must return @builtin(position)", ep.Name))
			}

		case ir.StageFragment:
			// Fragment shader can optionally return with location binding
			// No strict validation here as fragment can be void

		case ir.StageCompute:
			// Compute shader must have workgroup size
			if ep.Workgroup[0] == 0 || ep.Workgroup[1] == 0 || ep.Workgroup[2] == 0 {
				v.addError(fmt.Sprintf("entry point %q (@compute): workgroup size must be non-zero", ep.Name))
			}
		}

		v.checkEntryPointCapabilities(&v.module.EntryPoints[i])
		v.validateVaryings(&v.module.EntryPoints[i])

		// The body is validated like any function's (Rust validates entry
		// point functions with the module's functions).
		v.context = validationContext{
			function:       fn,
			functionName:   ep.Name,
			flow:           abilityReturn,
			expressionUsed: make(map[ir.ExpressionHandle]bool),
			scopeList:      v.context.scopeList,
		}
		v.validateFunction(fn)
		if ep.Stage == ir.StageVertex && v.context.subgroupStages&ShaderStagesVertex == 0 {
			// A subgroup operation in a vertex shader needs
			// SUBGROUP_VERTEX_STAGE; Rust reports it as a forbidden stage.
			v.missing |= CapSubgroupVertexStage
		}
		if v.context.stages&stageBit(ep.Stage) == 0 {
			v.entryPointError(&v.module.EntryPoints[i], "Uses operations forbidden at this stage")
		}
	}
}

// validateVaryings checks the interpolation of each location binding of an
// entry point's arguments and result, struct members included: the
// interpolation part of Rust naga's VaryingContext::validate_impl
// (valid/interface.rs). The messages are Rust's VaryingError texts.
func (v *validator) validateVaryings(ep *ir.EntryPoint) {
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
func (v *validator) validateVarying(ep *ir.EntryPoint, where string, ty ir.TypeHandle, binding *ir.Binding, output bool) {
	if binding == nil {
		if !v.isValidTypeHandle(ty) {
			return
		}
		if st, ok := v.module.Types[ty].Inner.(ir.StructType); ok {
			for _, m := range st.Members {
				if m.Binding != nil {
					v.validateVarying(ep, where, m.Type, m.Binding, output)
				}
			}
		}
		return
	}
	if b, ok := (*binding).(ir.BuiltinBinding); ok {
		if c := builtinCapability(b.Builtin); c != 0 && !v.has(c) {
			v.addError(fmt.Sprintf("entry point %q: %s: Capability %s is not supported", ep.Name, where, c.rustDebug()))
		}
		if msg := v.builtinVaryingError(ep.Stage, ty, b.Builtin, output); msg != "" {
			v.addError(fmt.Sprintf("entry point %q: %s: %s", ep.Name, where, msg))
		}
		return
	}
	loc, ok := (*binding).(ir.LocationBinding)
	if !ok {
		return
	}
	if msg := v.locationVaryingError(ep.Stage, ty, loc, output); msg != "" {
		v.addError(fmt.Sprintf("entry point %q: %s: %s", ep.Name, where, msg))
	}
}

// builtinVaryingError returns Rust's VaryingError text for a built-in whose
// stage, direction or type does not fit (valid/interface.rs
// VaryingContext::validate_impl), or "" for a built-in this check does not
// cover. It covers the built-ins the IR gained for the SPIR-V frontend; the
// rest of Rust's table is card P1-VALID-001.
func (v *validator) builtinVaryingError(stage ir.ShaderStage, ty ir.TypeHandle, builtin ir.BuiltinValue, output bool) string {
	if !v.isValidTypeHandle(ty) {
		return ""
	}
	inner := v.module.Types[ty].Inner
	computeLike := stage == ir.StageCompute || stage == ir.StageTask || stage == ir.StageMesh
	scalarU32 := func() bool {
		s, ok := inner.(ir.ScalarType)
		return ok && s.Kind == ir.ScalarUint && s.Width == 4
	}
	isVector := func(size ir.VectorSize, kind ir.ScalarKind, width uint8) bool {
		vec, ok := inner.(ir.VectorType)
		return ok && vec.Size == size && vec.Scalar.Kind == kind && vec.Scalar.Width == width
	}
	var visible, typeOK bool
	switch builtin {
	case ir.BuiltinBaseInstance, ir.BuiltinBaseVertex:
		// valid/interface.rs:292-295.
		visible = stage == ir.StageVertex && !output
		typeOK = scalarU32()
	case ir.BuiltinCullDistance:
		// valid/interface.rs:309-321: a vertex or mesh output, an array of f32
		// with a constant size up to eight.
		visible = (stage == ir.StageVertex || stage == ir.StageMesh) && output
		if arr, ok := inner.(ir.ArrayType); ok && int(arr.Base) < len(v.module.Types) {
			base, baseOK := v.module.Types[arr.Base].Inner.(ir.ScalarType)
			size := arr.Size.Constant
			typeOK = baseOK && base.Kind == ir.ScalarFloat && base.Width == 4 &&
				size != nil && *size > 0 && *size <= 8
		}
	case ir.BuiltinPointCoord:
		// valid/interface.rs:326-333.
		visible = stage == ir.StageFragment && !output
		typeOK = isVector(ir.Vec2, ir.ScalarFloat, 4)
	case ir.BuiltinWorkGroupSize:
		// valid/interface.rs:394-405, compute_like stages.
		visible = computeLike && !output
		typeOK = isVector(ir.Vec3, ir.ScalarUint, 4)
	default:
		return ""
	}
	if !visible {
		return fmt.Sprintf("Built-in %s is not available at this stage", builtinDebug(builtin))
	}
	if !typeOK {
		return fmt.Sprintf("Built-in type for %s is invalid. Found %s", builtinDebug(builtin), typeInnerDebug(inner))
	}
	return ""
}

// locationVaryingError returns Rust's VaryingError text for a location
// binding whose interpolation does not fit its stage, direction and type, or
// "" when it fits.
func (v *validator) locationVaryingError(stage ir.ShaderStage, ty ir.TypeHandle, loc ir.LocationBinding, output bool) string {
	if !v.isValidTypeHandle(ty) {
		return ""
	}
	if loc.PerPrimitive && !v.has(CapMeshShader) {
		return "Capability " + CapMeshShader.rustDebug() + " is not supported"
	}
	perVertex := loc.Interpolation != nil && *loc.Interpolation == ir.InterpolationPerVertex
	if perVertex {
		if stage != ir.StageFragment {
			return "Interpolation PerVertex is only valid for stage Fragment"
		}
		if !v.has(CapPerVertex) {
			return "Capability " + CapPerVertex.rustDebug() + " is not supported"
		}
		if loc.Sampling != nil && *loc.Sampling != ir.SamplingCenter {
			return "Per vertex can only have Center sampling or no sampling modifier"
		}
		// The checks below are on the element type of the array<T, 3>.
		arr, ok := v.module.Types[ty].Inner.(ir.ArrayType)
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
	if loc.PerPrimitive && (stage != ir.StageFragment || output) {
		return "The `@per_primitive` attribute can only be used in fragment shader inputs or mesh shader primitive outputs"
	}

	if loc.Interpolation != nil && loc.Sampling != nil {
		valid := false
		switch *loc.Interpolation {
		case ir.InterpolationPerspective, ir.InterpolationLinear:
			s := *loc.Sampling
			valid = s == ir.SamplingCenter || s == ir.SamplingCentroid || s == ir.SamplingSample
		case ir.InterpolationFlat:
			valid = *loc.Sampling == ir.SamplingFirst || *loc.Sampling == ir.SamplingEither
		case ir.InterpolationPerVertex:
			valid = true // Center only, checked above
		}
		if !valid {
			return fmt.Sprintf("Cannot combine %s interpolation with the %s sample type", *loc.Interpolation, *loc.Sampling)
		}
	}

	var needsInterpolation bool
	switch stage {
	case ir.StageVertex, ir.StageMesh:
		needsInterpolation = output
	case ir.StageFragment:
		needsInterpolation = !output && !loc.PerPrimitive
	}
	if perVertex || !needsInterpolation {
		return ""
	}
	var kind ir.ScalarKind
	switch t := v.module.Types[ty].Inner.(type) {
	case ir.ScalarType:
		kind = t.Kind
	case ir.VectorType:
		kind = t.Scalar.Kind
	case ir.MatrixType:
		kind = t.Scalar.Kind
	default:
		return ""
	}
	switch kind {
	case ir.ScalarFloat:
		// The frontend applies the default interpolation to floats.
		if loc.Interpolation == nil {
			return "Interpolation must be specified on vertex shader outputs and fragment shader inputs"
		}
	case ir.ScalarSint, ir.ScalarUint:
		// Integers have no default interpolation: flat must be explicit.
		if loc.Interpolation == nil || *loc.Interpolation != ir.InterpolationFlat {
			return "`@interpolate(flat)` must be explicitly specified for integer I/O"
		}
	}
	return ""
}

// hasPositionBuiltin checks if the function result contains @builtin(position).
// This can be either:
// 1. Direct binding on result: fn() -> @builtin(position) vec4<f32>
// 2. Struct member binding: fn() -> Struct { @builtin(position) pos: vec4<f32> }
func (v *validator) hasPositionBuiltin(result *ir.FunctionResult) bool {
	// Case 1: Direct binding on result
	if result.Binding != nil && isPositionBuiltin(*result.Binding) {
		return true
	}

	// Case 2: Check struct members for @builtin(position)
	return v.structHasPositionBuiltin(result.Type)
}

// isPositionBuiltin checks if a binding is @builtin(position).
func isPositionBuiltin(binding ir.Binding) bool {
	b, ok := binding.(ir.BuiltinBinding)
	return ok && b.Builtin == ir.BuiltinPosition
}

// structHasPositionBuiltin checks if a struct type has a member with @builtin(position).
func (v *validator) structHasPositionBuiltin(typeHandle ir.TypeHandle) bool {
	if int(typeHandle) >= len(v.module.Types) {
		return false
	}
	structType, ok := v.module.Types[typeHandle].Inner.(ir.StructType)
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

func (v *validator) isValidTypeHandle(handle ir.TypeHandle) bool {
	return int(handle) < len(v.module.Types)
}

func (v *validator) isValidConstantHandle(handle ir.ConstantHandle) bool {
	return int(handle) < len(v.module.Constants)
}

func (v *validator) isValidGlobalVariableHandle(handle ir.GlobalVariableHandle) bool {
	return int(handle) < len(v.module.GlobalVariables)
}

func (v *validator) isValidFunctionHandle(handle ir.FunctionHandle) bool {
	return int(handle) < len(v.module.Functions)
}

func (v *validator) isValidExpressionHandle(handle ir.ExpressionHandle) bool {
	if v.context.function == nil {
		return false
	}
	return int(handle) < len(v.context.function.Expressions)
}

func (v *validator) addError(msg string) {
	v.errors = append(v.errors, Error{
		Message:   msg,
		Statement: -1,
	})
}

func (v *validator) addErrorInFunction(msg string) {
	v.errors = append(v.errors, Error{
		Message:   msg,
		Function:  v.context.functionName,
		Statement: -1,
	})
}

func (v *validator) addErrorInExpression(handle ir.ExpressionHandle, msg string) {
	v.errors = append(v.errors, Error{
		Message:    msg,
		Function:   v.context.functionName,
		Expression: &handle,
		Statement:  -1,
	})
}

func (v *validator) addErrorInStatement(index int, msg string) {
	v.errors = append(v.errors, Error{
		Message:   msg,
		Function:  v.context.functionName,
		Statement: index,
	})
}

// containsPendingArray reports a type that is, or holds, an array sized by
// an override.
func (v *validator) containsPendingArray(h ir.TypeHandle, depth int) bool {
	if depth > maxTypeNesting || !v.isValidTypeHandle(h) {
		return false
	}
	switch t := v.module.Types[h].Inner.(type) {
	case ir.ArrayType:
		return t.Size.Pending != nil || v.containsPendingArray(t.Base, depth+1)
	case ir.StructType:
		for _, m := range t.Members {
			if v.containsPendingArray(m.Type, depth+1) {
				return true
			}
		}
	}
	return false
}

// immediateCompatible reports whether a var<immediate> may hold a value of
// type h, as Rust's TypeInfo::immediates_compatibility (valid/type.rs): no
// array anywhere in it (ImmediateError::InvalidArray, the immediate-data
// proposal's rule) and no 16-bit integer (InvalidScalar).
func (v *validator) immediateCompatible(h ir.TypeHandle, depth int) bool {
	if depth > maxTypeNesting || !v.isValidTypeHandle(h) {
		return true
	}
	switch t := v.module.Types[h].Inner.(type) {
	case ir.ScalarType:
		return immediateScalar(t)
	case ir.VectorType:
		return immediateScalar(t.Scalar)
	case ir.MatrixType:
		return immediateScalar(t.Scalar)
	case ir.AtomicType:
		return immediateScalar(t.Scalar)
	case ir.ArrayType:
		return false
	case ir.StructType:
		for _, m := range t.Members {
			if !v.immediateCompatible(m.Type, depth+1) {
				return false
			}
		}
	}
	return true
}

// immediateScalar reports whether s may be stored in immediate data: not a
// 16-bit integer (Rust's check_width marks i16 and u16 only).
func immediateScalar(s ir.ScalarType) bool {
	return !((s.Kind == ir.ScalarSint || s.Kind == ir.ScalarUint) && s.Width == 2)
}

// checkSelectTypes checks a select as Rust's validate_expression for
// Expression::Select: the accept and reject values have one type
// (SelectValuesTypeMismatch, checked first), and the condition is a bool
// with scalar or vector values or a bool vector of the values' size
// (SelectConditionNotABool), the WGSL spec's select overloads.
func (v *validator) checkSelectTypes(handle ir.ExpressionHandle, s ir.ExprSelect) {
	accept, okA := v.expressionInner(s.Accept)
	reject, okR := v.expressionInner(s.Reject)
	condition, okC := v.expressionInner(s.Condition)
	if !okA || !okR || !okC {
		return
	}
	if !sameTypeInner(accept, reject) {
		v.addErrorInExpression(handle, fmt.Sprintf(
			"Expected selection argument types to match, but reject value of type %s does not match accept value of value %s",
			typeInnerDebug(reject), typeInnerDebug(accept)))
		return
	}
	good := false
	switch c := condition.(type) {
	case ir.ScalarType:
		if c.Kind == ir.ScalarBool {
			switch accept.(type) {
			case ir.ScalarType, ir.VectorType:
				good = true
			}
		}
	case ir.VectorType:
		if c.Scalar.Kind == ir.ScalarBool {
			if av, ok := accept.(ir.VectorType); ok {
				good = av.Size == c.Size
			}
		}
	}
	if !good {
		v.addErrorInExpression(handle, "Expected selection condition to be a boolean value, got "+typeInnerDebug(condition))
	}
}

// expressionInner is the type of expression h of the function being
// validated, and whether it is known.
func (v *validator) expressionInner(h ir.ExpressionHandle) (ir.TypeInner, bool) {
	types := v.context.types
	if types == nil || int(h) >= types.Len() {
		return nil, false
	}
	res := types.Get(h)
	if res.Handle != nil && !v.isValidTypeHandle(*res.Handle) {
		return nil, false
	}
	return res.Inner(v.module.Types), true
}

// sameTypeInner reports whether a and b are one type, as Rust compares two
// TypeInner values. Scalars, vectors and matrices compare by value; the
// other kinds are module types, unique by handle in Rust's UniqueArena, and
// compare as their struct, array or pointer contents here.
func sameTypeInner(a, b ir.TypeInner) bool {
	switch at := a.(type) {
	case ir.ScalarType, ir.VectorType, ir.MatrixType, ir.AtomicType:
		return a == b
	case ir.ArrayType:
		bt, ok := b.(ir.ArrayType)
		return ok && at.Base == bt.Base && at.Stride == bt.Stride && sameArraySize(at.Size, bt.Size)
	case ir.StructType:
		bt, ok := b.(ir.StructType)
		if !ok || at.Span != bt.Span || len(at.Members) != len(bt.Members) {
			return false
		}
		for i := range at.Members {
			if at.Members[i].Type != bt.Members[i].Type || at.Members[i].Offset != bt.Members[i].Offset {
				return false
			}
		}
		return true
	}
	return false
}

// sameArraySize reports whether two array sizes are one: the same count,
// the same override, or both runtime-sized.
func sameArraySize(a, b ir.ArraySize) bool {
	switch {
	case a.Constant != nil || b.Constant != nil:
		return a.Constant != nil && b.Constant != nil && *a.Constant == *b.Constant
	case a.Pending != nil || b.Pending != nil:
		return a.Pending != nil && b.Pending != nil && *a.Pending == *b.Pending
	}
	return true
}

// checkUnaryOperand checks the operand type of a unary operator, as Rust's
// validate_expression for Expression::Unary: a scalar or vector, of a float
// or signed integer for negation, of bool for `!`, of an integer for `~`
// (the WGSL spec's unary operators). A matrix or a composite was compiled to
// whatever each backend made of it.
func (v *validator) checkUnaryOperand(handle ir.ExpressionHandle, u ir.ExprUnary) {
	types := v.context.types
	if types == nil || int(u.Expr) >= types.Len() {
		return
	}
	res := types.Get(u.Expr)
	if res.Handle != nil && !v.isValidTypeHandle(*res.Handle) {
		return
	}
	var kind ir.ScalarKind
	ok := false
	switch t := res.Inner(v.module.Types).(type) {
	case ir.ScalarType:
		kind, ok = t.Kind, true
	case ir.VectorType:
		kind, ok = t.Scalar.Kind, true
	}
	if ok {
		switch u.Op {
		case ir.UnaryNegate:
			ok = kind == ir.ScalarFloat || kind == ir.ScalarSint
		case ir.UnaryLogicalNot:
			ok = kind == ir.ScalarBool
		case ir.UnaryBitwiseNot:
			ok = kind == ir.ScalarSint || kind == ir.ScalarUint
		default:
			ok = false
		}
	}
	if !ok {
		v.addErrorInExpression(handle, fmt.Sprintf("Operation %s can't work with [%d]", unaryOperatorName(u.Op), u.Expr))
	}
}

// unaryOperatorNames are Rust naga's UnaryOperator variant names.
var unaryOperatorNames = [...]string{
	ir.UnaryNegate:     "Negate",
	ir.UnaryLogicalNot: "LogicalNot",
	ir.UnaryBitwiseNot: "BitwiseNot",
}

// unaryOperatorName is op as Rust's Debug prints it in an error message.
func unaryOperatorName(op ir.UnaryOperator) string {
	if int(op) < len(unaryOperatorNames) {
		return unaryOperatorNames[op]
	}
	return fmt.Sprintf("UnaryOperator(%d)", op)
}

// isSizedArrayPointer reports a pointer, in the function being validated,
// to an array that is not runtime-sized.
func (v *validator) isSizedArrayPointer(h ir.ExpressionHandle) bool {
	types := v.context.types
	if types == nil || int(h) >= types.Len() {
		return false
	}
	res := types.Get(h)
	if res.Handle != nil && !v.isValidTypeHandle(*res.Handle) {
		return false
	}
	ptr, ok := res.Inner(v.module.Types).(ir.PointerType)
	if !ok || !v.isValidTypeHandle(ptr.Base) {
		return false
	}
	at, ok := v.module.Types[ptr.Base].Inner.(ir.ArrayType)
	return ok && !at.Size.IsRuntimeSized()
}
