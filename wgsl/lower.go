package wgsl

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/gogpu/naga/ir"
)

// Warning represents a compiler warning (not an error).
type Warning struct {
	Message string
	Span    Span
}

// Lowerer converts WGSL AST to Naga IR.
type Lowerer struct {
	module *ir.Module
	source string // Original source code for error messages

	// Type resolution
	registry *ir.TypeRegistry         // Deduplicates types
	types    map[string]ir.TypeHandle // Named type lookup

	// Variable resolution
	globals         map[string]ir.GlobalVariableHandle
	locals          map[string]ir.ExpressionHandle
	moduleConstants map[string]ir.ConstantHandle
	globalIdx       uint32

	// Function resolution
	functions map[string]ir.FunctionHandle // Named function lookup

	// Variable usage tracking for unused variable warnings
	localDecls map[string]Span // Where each local variable was declared
	usedLocals map[string]bool // Which local variables have been used

	// Current function context
	currentFunc    *ir.Function
	currentFuncIdx ir.FunctionHandle
	currentExprIdx ir.ExpressionHandle

	// Errors and warnings
	errors   SourceErrors
	warnings []Warning
}

// LowerResult contains the result of lowering, including any warnings.
type LowerResult struct {
	Module   *ir.Module
	Warnings []Warning
}

// Lower converts a WGSL AST module to Naga IR.
func Lower(ast *Module) (*ir.Module, error) {
	return LowerWithSource(ast, "")
}

// LowerWithSource converts a WGSL AST module to Naga IR, keeping source for error messages.
func LowerWithSource(ast *Module, source string) (*ir.Module, error) {
	result, err := LowerWithWarnings(ast, source)
	if err != nil {
		return nil, err
	}
	return result.Module, nil
}

// LowerWithWarnings converts a WGSL AST module to Naga IR, returning warnings.
func LowerWithWarnings(ast *Module, source string) (*LowerResult, error) {
	l := &Lowerer{
		module:          &ir.Module{},
		source:          source,
		registry:        ir.NewTypeRegistry(),
		types:           make(map[string]ir.TypeHandle, 16),
		globals:         make(map[string]ir.GlobalVariableHandle, 8),
		locals:          make(map[string]ir.ExpressionHandle, 16),
		moduleConstants: make(map[string]ir.ConstantHandle, 16),
		functions:       make(map[string]ir.FunctionHandle, len(ast.Functions)),
		localDecls:      make(map[string]Span, 16),
		usedLocals:      make(map[string]bool, 16),
	}

	// Register built-in types
	l.registerBuiltinTypes()

	// Lower structs
	for _, s := range ast.Structs {
		if err := l.lowerStruct(s); err != nil {
			l.addError(err.Error(), s.Span)
		}
	}

	// Lower global variables
	for _, v := range ast.GlobalVars {
		if err := l.lowerGlobalVar(v); err != nil {
			l.addError(err.Error(), v.Span)
		}
	}

	// Lower constants
	for _, c := range ast.Constants {
		if err := l.lowerConstant(c); err != nil {
			l.addError(err.Error(), c.Span)
		}
	}

	// Pre-register all function names to support forward references
	for i, f := range ast.Functions {
		l.functions[f.Name] = ir.FunctionHandle(i)
	}

	// Lower functions and identify entry points
	for _, f := range ast.Functions {
		if err := l.lowerFunction(f); err != nil {
			l.addError(err.Error(), f.Span)
		}
	}

	if l.errors.HasErrors() {
		return nil, &l.errors
	}

	// Copy deduplicated types from registry to module
	l.module.Types = l.registry.GetTypes()

	return &LowerResult{
		Module:   l.module,
		Warnings: l.warnings,
	}, nil
}

// addError adds an error with source location.
func (l *Lowerer) addError(message string, span Span) {
	l.errors.Add(NewSourceError(message, span, l.source))
}

// registerBuiltinTypes registers WGSL built-in scalar types.
// Note: Sampler types are NOT pre-registered because they should only be emitted
// when actually used in the shader. Pre-registering them causes SPIR-V to contain
// OpTypeSampler instructions even in shaders without textures, which can cause
// Vulkan validation errors or crashes on some drivers.
func (l *Lowerer) registerBuiltinTypes() {
	// Scalars - these are needed for literals and basic expressions
	l.registerType("f32", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	// Note: f16 is NOT pre-registered. Per WGSL spec, f16 requires an explicit
	// "enable f16;" directive. Pre-registering causes SPIR-V to contain
	// OpCapability Float16 even in shaders that don't use it, which triggers
	// Vulkan validation errors on devices without shaderFloat16 support.
	l.registerType("i32", ir.ScalarType{Kind: ir.ScalarSint, Width: 4})
	l.registerType("u32", ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	l.registerType("bool", ir.ScalarType{Kind: ir.ScalarBool, Width: 1})

	// Samplers are NOT registered here. They will be created on-demand when
	// the shader actually uses sampler or sampler_comparison types.
	// See resolveNamedType() for lazy sampler type registration.
}

// registerType adds a type to the registry with deduplication and maps its name.
func (l *Lowerer) registerType(name string, inner ir.TypeInner) ir.TypeHandle {
	// Use registry for deduplication
	handle := l.registry.GetOrCreate(name, inner)

	// Map named types for lookup
	if name != "" {
		l.types[name] = handle
	}
	// Keep module types in sync so type resolution works during lowering.
	l.module.Types = l.registry.GetTypes()

	return handle
}

// lowerStruct converts a struct declaration to IR.
func (l *Lowerer) lowerStruct(s *StructDecl) error {
	members := make([]ir.StructMember, len(s.Members))
	var offset uint32
	var maxAlign uint32 = 1
	for i, m := range s.Members {
		typeHandle, err := l.resolveType(m.Type)
		if err != nil {
			return fmt.Errorf("struct %s member %s: %w", s.Name, m.Name, err)
		}

		// Extract binding from member attributes (@builtin, @location, etc.)
		var binding *ir.Binding
		for _, attr := range m.Attributes {
			if b := l.memberBinding(&attr); b != nil {
				binding = b
				break
			}
		}

		// Calculate proper alignment and size for WGSL uniform buffer layout
		align, size := l.typeAlignmentAndSize(typeHandle)
		if align > maxAlign {
			maxAlign = align
		}

		// Align offset to the type's alignment requirement
		offset = (offset + align - 1) &^ (align - 1)

		members[i] = ir.StructMember{
			Name:    m.Name,
			Type:    typeHandle,
			Binding: binding,
			Offset:  offset,
		}
		offset += size
	}
	// Round struct size up to alignment of largest member
	structSize := (offset + maxAlign - 1) &^ (maxAlign - 1)
	l.registerType(s.Name, ir.StructType{Members: members, Span: structSize})
	return nil
}

// typeAlignmentAndSize returns the alignment and size of a type for uniform buffer layout.
// Follows WGSL/WebGPU alignment rules (similar to std140 but with some differences).
func (l *Lowerer) typeAlignmentAndSize(handle ir.TypeHandle) (align, size uint32) {
	typ := l.module.Types[handle]

	switch t := typ.Inner.(type) {
	case ir.ScalarType:
		// f32, i32, u32: 4 bytes each
		return 4, 4

	case ir.VectorType:
		// vec2: align 8, size 8
		// vec3: align 16, size 12
		// vec4: align 16, size 16
		scalarSize := uint32(4) // Assume f32/i32/u32
		switch t.Size {
		case ir.Vec2:
			return 8, scalarSize * 2
		case ir.Vec3:
			return 16, scalarSize * 3
		case ir.Vec4:
			return 16, scalarSize * 4
		}

	case ir.MatrixType:
		// Matrix layout: column-major, each column is a vec with alignment
		// mat2x2: align 8, size 16 (2 vec2 columns)
		// mat3x3: align 16, size 48 (3 vec3 columns, each padded to 16)
		// mat4x4: align 16, size 64 (4 vec4 columns)
		colAlign, colSize := l.vectorAlignmentAndSize(uint8(t.Rows))
		return colAlign, colSize * uint32(t.Columns)

	case ir.ArrayType:
		// Array elements are aligned to 16 bytes (rounded up)
		elemAlign, elemSize := l.typeAlignmentAndSize(t.Base)
		// Each array element is rounded up to vec4 alignment (16 bytes)
		stride := (elemSize + 15) &^ 15
		if elemAlign < 16 {
			elemAlign = 16
		}
		if t.Size.Constant != nil {
			return elemAlign, stride * *t.Size.Constant
		}
		// Runtime-sized array
		return elemAlign, stride

	case ir.StructType:
		// Struct alignment is the max of its members, size is pre-calculated
		var maxMemberAlign uint32 = 1
		for _, member := range t.Members {
			memberAlign, _ := l.typeAlignmentAndSize(member.Type)
			if memberAlign > maxMemberAlign {
				maxMemberAlign = memberAlign
			}
		}
		return maxMemberAlign, t.Span
	}

	// Default fallback
	return 4, 4
}

// vectorAlignmentAndSize returns alignment and size for a vector of given component count.
func (l *Lowerer) vectorAlignmentAndSize(components uint8) (align, size uint32) {
	scalarSize := uint32(4) // f32/i32/u32
	switch components {
	case 2:
		return 8, scalarSize * 2
	case 3:
		return 16, scalarSize * 3
	case 4:
		return 16, scalarSize * 4
	default:
		return 4, scalarSize
	}
}

// lowerGlobalVar converts a global variable declaration to IR.
func (l *Lowerer) lowerGlobalVar(v *VarDecl) error {
	typeHandle, err := l.resolveType(v.Type)
	if err != nil {
		return fmt.Errorf("global var %s: %w", v.Name, err)
	}

	space := l.addressSpace(v.AddressSpace)

	// Samplers and textures must use SpaceHandle (maps to UniformConstant in SPIR-V)
	// This is required by Vulkan: "Variables identified with the UniformConstant
	// storage class are used only as handles to refer to opaque resources.
	// Such variables must be typed as OpTypeImage, OpTypeSampler, OpTypeSampledImage"
	if l.isOpaqueResourceType(typeHandle) {
		space = ir.SpaceHandle
	}

	var binding *ir.ResourceBinding

	// Parse @group and @binding attributes
	for _, attr := range v.Attributes {
		if attr.Name == "group" && len(attr.Args) > 0 {
			if lit, ok := attr.Args[0].(*Literal); ok {
				group, _ := strconv.ParseUint(lit.Value, 10, 32)
				if binding == nil {
					binding = &ir.ResourceBinding{}
				}
				binding.Group = uint32(group)
			}
		}
		if attr.Name == "binding" && len(attr.Args) > 0 {
			if lit, ok := attr.Args[0].(*Literal); ok {
				bind, _ := strconv.ParseUint(lit.Value, 10, 32)
				if binding == nil {
					binding = &ir.ResourceBinding{}
				}
				binding.Binding = uint32(bind)
			}
		}
	}

	handle := ir.GlobalVariableHandle(l.globalIdx)
	l.globalIdx++
	l.module.GlobalVariables = append(l.module.GlobalVariables, ir.GlobalVariable{
		Name:    v.Name,
		Space:   space,
		Binding: binding,
		Type:    typeHandle,
		Init:    nil, // TODO: handle initializers
	})
	l.globals[v.Name] = handle
	return nil
}

// lowerConstant converts a constant declaration to IR.
func (l *Lowerer) lowerConstant(c *ConstDecl) error {
	if c.Init == nil {
		return fmt.Errorf("module constant '%s' must have initializer", c.Name)
	}

	// Evaluate the constant initializer to a scalar value.
	lit, ok := c.Init.(*Literal)
	if !ok {
		return fmt.Errorf("module constant '%s': only literal initializers supported", c.Name)
	}

	var scalarKind ir.ScalarKind
	var bits uint64

	switch lit.Kind {
	case TokenIntLiteral:
		text := lit.Value
		isUnsigned := false
		if len(text) > 0 && text[len(text)-1] == 'u' {
			text = text[:len(text)-1]
			isUnsigned = true
		} else if len(text) > 0 && text[len(text)-1] == 'i' {
			text = text[:len(text)-1]
		}
		if isUnsigned {
			v, _ := strconv.ParseUint(text, 0, 32)
			bits = v
			scalarKind = ir.ScalarUint
		} else {
			v, _ := strconv.ParseInt(text, 0, 32)
			bits = uint64(v)
			scalarKind = ir.ScalarSint
		}
	case TokenFloatLiteral:
		text := lit.Value
		if len(text) > 0 && (text[len(text)-1] == 'f' || text[len(text)-1] == 'h') {
			text = text[:len(text)-1]
		}
		v, _ := strconv.ParseFloat(text, 32)
		bits = uint64(math.Float32bits(float32(v)))
		scalarKind = ir.ScalarFloat
	case TokenTrue, TokenBoolLiteral:
		if lit.Value == "true" {
			bits = 1
		}
		scalarKind = ir.ScalarBool
	case TokenFalse:
		scalarKind = ir.ScalarBool
	default:
		return fmt.Errorf("module constant '%s': unsupported literal kind %v", c.Name, lit.Kind)
	}

	// Resolve the type. If no explicit type, infer from the literal.
	var typeHandle ir.TypeHandle
	if c.Type != nil {
		var err error
		typeHandle, err = l.resolveType(c.Type)
		if err != nil {
			return fmt.Errorf("constant %s: %w", c.Name, err)
		}
	} else {
		// Infer type from literal
		width := uint8(4)
		typeHandle = l.registerType("", ir.ScalarType{Kind: scalarKind, Width: width})
	}

	handle := ir.ConstantHandle(len(l.module.Constants))
	l.module.Constants = append(l.module.Constants, ir.Constant{
		Name:  c.Name,
		Type:  typeHandle,
		Value: ir.ScalarValue{Bits: bits, Kind: scalarKind},
	})
	l.moduleConstants[c.Name] = handle
	return nil
}

// lowerFunction converts a function declaration to IR.
func (l *Lowerer) lowerFunction(f *FunctionDecl) error {
	// Reset local context by clearing maps instead of reallocating.
	for k := range l.locals {
		delete(l.locals, k)
	}
	for k := range l.localDecls {
		delete(l.localDecls, k)
	}
	for k := range l.usedLocals {
		delete(l.usedLocals, k)
	}
	l.currentExprIdx = 0

	// Estimate sizes based on function complexity.
	var bodySize int
	if f.Body != nil {
		bodySize = len(f.Body.Statements)
	}
	estExprs := bodySize * 3 // rough: ~3 expressions per statement
	if estExprs < 8 {
		estExprs = 8
	}

	fn := &ir.Function{
		Name:            f.Name,
		Arguments:       make([]ir.FunctionArgument, len(f.Params)),
		LocalVars:       make([]ir.LocalVariable, 0, 4),
		Expressions:     make([]ir.Expression, 0, estExprs),
		ExpressionTypes: make([]ir.TypeResolution, 0, estExprs),
		Body:            make([]ir.Statement, 0, bodySize),
	}
	l.currentFunc = fn

	// Lower parameters
	for i, p := range f.Params {
		typeHandle, err := l.resolveType(p.Type)
		if err != nil {
			return fmt.Errorf("function %s param %s: %w", f.Name, p.Name, err)
		}

		binding := l.paramBinding(p.Attributes)
		fn.Arguments[i] = ir.FunctionArgument{
			Name:    p.Name,
			Type:    typeHandle,
			Binding: binding,
		}

		// Register parameter as local expression (FunctionArgument)
		exprHandle := l.addExpression(ir.Expression{
			Kind: ir.ExprFunctionArgument{Index: uint32(i)},
		})
		l.locals[p.Name] = exprHandle
	}

	// Lower return type
	if f.ReturnType != nil {
		typeHandle, err := l.resolveType(f.ReturnType)
		if err != nil {
			return fmt.Errorf("function %s return type: %w", f.Name, err)
		}
		fn.Result = &ir.FunctionResult{
			Type:    typeHandle,
			Binding: l.returnBinding(f.ReturnAttrs),
		}
	}

	// Lower function body
	if f.Body != nil {
		if err := l.lowerBlock(f.Body, &fn.Body); err != nil {
			return fmt.Errorf("function %s body: %w", f.Name, err)
		}
	}

	// Check for unused local variables
	l.checkUnusedVariables(f.Name)

	// Add function to module (handle was pre-registered for forward references)
	funcHandle := l.functions[f.Name]
	l.module.Functions = append(l.module.Functions, *fn)
	l.currentFuncIdx = funcHandle

	// Check if this is an entry point
	stage := l.entryPointStage(f.Attributes)
	if stage != nil {
		ep := ir.EntryPoint{
			Name:     f.Name,
			Stage:    *stage,
			Function: funcHandle,
		}
		// Extract workgroup_size for compute shaders
		if *stage == ir.StageCompute {
			ep.Workgroup = l.extractWorkgroupSize(f.Attributes)
		}
		l.module.EntryPoints = append(l.module.EntryPoints, ep)
	}

	return nil
}

// lowerBlock converts a block statement to IR statements.
func (l *Lowerer) lowerBlock(block *BlockStmt, target *[]ir.Statement) error {
	for _, stmt := range block.Statements {
		if err := l.lowerStatement(stmt, target); err != nil {
			return err
		}
	}
	return nil
}

// lowerStatement converts a statement to IR.
func (l *Lowerer) lowerStatement(stmt Stmt, target *[]ir.Statement) error {
	switch s := stmt.(type) {
	case *ReturnStmt:
		return l.lowerReturn(s, target)
	case *VarDecl:
		return l.lowerLocalVar(s, target)
	case *AssignStmt:
		return l.lowerAssign(s, target)
	case *IfStmt:
		return l.lowerIf(s, target)
	case *ForStmt:
		return l.lowerFor(s, target)
	case *WhileStmt:
		return l.lowerWhile(s, target)
	case *LoopStmt:
		return l.lowerLoop(s, target)
	case *SwitchStmt:
		return l.lowerSwitch(s, target)
	case *ConstDecl:
		// Local const is treated like let (named expression)
		return l.lowerLocalConst(s, target)
	case *BreakStmt:
		*target = append(*target, ir.Statement{Kind: ir.StmtBreak{}})
		return nil
	case *ContinueStmt:
		*target = append(*target, ir.Statement{Kind: ir.StmtContinue{}})
		return nil
	case *DiscardStmt:
		*target = append(*target, ir.Statement{Kind: ir.StmtKill{}})
		return nil
	case *ExprStmt:
		// Evaluate expression for side effects
		_, err := l.lowerExpression(s.Expr, target)
		return err
	case *BlockStmt:
		var body []ir.Statement
		if err := l.lowerBlock(s, &body); err != nil {
			return err
		}
		*target = append(*target, ir.Statement{Kind: ir.StmtBlock{Block: body}})
		return nil
	default:
		return fmt.Errorf("unsupported statement type: %T", stmt)
	}
}

// lowerReturn converts a return statement to IR.
func (l *Lowerer) lowerReturn(ret *ReturnStmt, target *[]ir.Statement) error {
	var valueHandle *ir.ExpressionHandle
	if ret.Value != nil {
		handle, err := l.lowerExpression(ret.Value, target)
		if err != nil {
			return err
		}
		valueHandle = &handle
	}
	*target = append(*target, ir.Statement{
		Kind: ir.StmtReturn{Value: valueHandle},
	})
	return nil
}

// lowerLocalVar converts a local variable declaration to IR.
func (l *Lowerer) lowerLocalVar(v *VarDecl, target *[]ir.Statement) error {
	var typeHandle ir.TypeHandle
	var initHandle *ir.ExpressionHandle

	// Lower initializer first (needed for type inference)
	if v.Init != nil {
		init, err := l.lowerExpression(v.Init, target)
		if err != nil {
			return err
		}
		initHandle = &init
	}

	// Resolve type: explicit or inferred from initializer
	//nolint:nestif // Type resolution logic requires explicit type vs inference branching
	if v.Type != nil {
		// Explicit type annotation
		var err error
		typeHandle, err = l.resolveType(v.Type)
		if err != nil {
			return fmt.Errorf("local var %s: %w", v.Name, err)
		}
	} else if initHandle != nil {
		// Infer type from initializer expression
		var err error
		typeHandle, err = l.inferTypeFromExpression(*initHandle)
		if err != nil {
			return fmt.Errorf("local var %s type inference: %w", v.Name, err)
		}
	} else {
		return fmt.Errorf("local var %s: type required without initializer", v.Name)
	}

	localIdx := uint32(len(l.currentFunc.LocalVars))
	l.currentFunc.LocalVars = append(l.currentFunc.LocalVars, ir.LocalVariable{
		Name: v.Name,
		Type: typeHandle,
		Init: initHandle,
	})

	// Create local variable expression
	exprHandle := l.addExpression(ir.Expression{
		Kind: ir.ExprLocalVariable{Variable: localIdx},
	})
	l.locals[v.Name] = exprHandle

	// Track declaration for unused variable warnings
	l.localDecls[v.Name] = v.Span

	return nil
}

// lowerAssign converts an assignment statement to IR.
func (l *Lowerer) lowerAssign(assign *AssignStmt, target *[]ir.Statement) error {
	pointer, err := l.lowerExpression(assign.Left, target)
	if err != nil {
		return err
	}

	value, err := l.lowerExpression(assign.Right, target)
	if err != nil {
		return err
	}

	// Handle compound assignments (+=, -=, etc.)
	if assign.Op != TokenEqual {
		// Load current value
		loadHandle := l.addExpression(ir.Expression{
			Kind: ir.ExprLoad{Pointer: pointer},
		})

		// Apply operation
		op := l.assignOpToBinary(assign.Op)
		value = l.addExpression(ir.Expression{
			Kind: ir.ExprBinary{
				Op:    op,
				Left:  loadHandle,
				Right: value,
			},
		})
	}

	*target = append(*target, ir.Statement{
		Kind: ir.StmtStore{Pointer: pointer, Value: value},
	})
	return nil
}

// lowerIf converts an if statement to IR.
func (l *Lowerer) lowerIf(ifStmt *IfStmt, target *[]ir.Statement) error {
	condition, err := l.lowerExpression(ifStmt.Condition, target)
	if err != nil {
		return err
	}

	var accept, reject []ir.Statement
	if err := l.lowerBlock(ifStmt.Body, &accept); err != nil {
		return err
	}

	if ifStmt.Else != nil {
		if err := l.lowerStatement(ifStmt.Else, &reject); err != nil {
			return err
		}
	}

	*target = append(*target, ir.Statement{
		Kind: ir.StmtIf{
			Condition: condition,
			Accept:    accept,
			Reject:    reject,
		},
	})
	return nil
}

// lowerFor converts a for loop to IR.
func (l *Lowerer) lowerFor(forStmt *ForStmt, target *[]ir.Statement) error {
	// For loops become: init; loop { if !condition { break }; body; update }
	if forStmt.Init != nil {
		if err := l.lowerStatement(forStmt.Init, target); err != nil {
			return err
		}
	}

	var body, continuing []ir.Statement

	// Add condition check at start of body
	if forStmt.Condition != nil {
		condition, err := l.lowerExpression(forStmt.Condition, &body)
		if err != nil {
			return err
		}
		// Negate condition and break if false
		notCond := l.addExpression(ir.Expression{
			Kind: ir.ExprUnary{Op: ir.UnaryLogicalNot, Expr: condition},
		})
		body = append(body, ir.Statement{
			Kind: ir.StmtIf{
				Condition: notCond,
				Accept:    []ir.Statement{{Kind: ir.StmtBreak{}}},
				Reject:    []ir.Statement{},
			},
		})
	}

	// Add loop body
	if err := l.lowerBlock(forStmt.Body, &body); err != nil {
		return err
	}

	// Add update in continuing block
	if forStmt.Update != nil {
		if err := l.lowerStatement(forStmt.Update, &continuing); err != nil {
			return err
		}
	}

	*target = append(*target, ir.Statement{
		Kind: ir.StmtLoop{
			Body:       body,
			Continuing: continuing,
		},
	})
	return nil
}

// lowerWhile converts a while loop to IR.
func (l *Lowerer) lowerWhile(whileStmt *WhileStmt, target *[]ir.Statement) error {
	var body []ir.Statement //nolint:prealloc // Size varies based on loop content

	// Check condition at start
	condition, err := l.lowerExpression(whileStmt.Condition, &body)
	if err != nil {
		return err
	}

	notCond := l.addExpression(ir.Expression{
		Kind: ir.ExprUnary{Op: ir.UnaryLogicalNot, Expr: condition},
	})
	body = append(body, ir.Statement{
		Kind: ir.StmtIf{
			Condition: notCond,
			Accept:    []ir.Statement{{Kind: ir.StmtBreak{}}},
			Reject:    []ir.Statement{},
		},
	})

	if err := l.lowerBlock(whileStmt.Body, &body); err != nil {
		return err
	}

	*target = append(*target, ir.Statement{
		Kind: ir.StmtLoop{
			Body:       body,
			Continuing: []ir.Statement{},
		},
	})
	return nil
}

// lowerLoop converts a loop statement to IR.
func (l *Lowerer) lowerLoop(loopStmt *LoopStmt, target *[]ir.Statement) error {
	var body, continuing []ir.Statement

	if err := l.lowerBlock(loopStmt.Body, &body); err != nil {
		return err
	}

	if loopStmt.Continuing != nil {
		if err := l.lowerBlock(loopStmt.Continuing, &continuing); err != nil {
			return err
		}
	}

	*target = append(*target, ir.Statement{
		Kind: ir.StmtLoop{
			Body:       body,
			Continuing: continuing,
		},
	})
	return nil
}

// lowerSwitch converts a switch statement to IR.
func (l *Lowerer) lowerSwitch(switchStmt *SwitchStmt, target *[]ir.Statement) error {
	// Lower selector expression
	selector, err := l.lowerExpression(switchStmt.Selector, target)
	if err != nil {
		return fmt.Errorf("switch selector: %w", err)
	}

	var cases []ir.SwitchCase
	for i, clause := range switchStmt.Cases {
		var caseBody []ir.Statement
		if err := l.lowerBlock(clause.Body, &caseBody); err != nil {
			return fmt.Errorf("switch case %d body: %w", i, err)
		}

		if clause.IsDefault {
			cases = append(cases, ir.SwitchCase{
				Value: ir.SwitchValueDefault{},
				Body:  caseBody,
			})
		} else {
			// For each selector, create a case
			for _, sel := range clause.Selectors {
				value, err := l.lowerSwitchCaseValue(sel)
				if err != nil {
					return fmt.Errorf("switch case %d selector: %w", i, err)
				}
				cases = append(cases, ir.SwitchCase{
					Value: value,
					Body:  caseBody,
				})
			}
		}
	}

	*target = append(*target, ir.Statement{
		Kind: ir.StmtSwitch{
			Selector: selector,
			Cases:    cases,
		},
	})
	return nil
}

// lowerSwitchCaseValue converts a switch case selector to IR.
func (l *Lowerer) lowerSwitchCaseValue(expr Expr) (ir.SwitchValue, error) {
	switch e := expr.(type) {
	case *Literal:
		return l.literalToSwitchValue(e)
	case *Ident:
		// Resolve module-level constant to its literal value
		constHandle, ok := l.moduleConstants[e.Name]
		if !ok {
			return nil, fmt.Errorf("switch case selector '%s': not a known constant", e.Name)
		}
		constant := &l.module.Constants[constHandle]
		sv, ok := constant.Value.(ir.ScalarValue)
		if !ok {
			return nil, fmt.Errorf("switch case selector '%s': not a scalar constant", e.Name)
		}
		switch sv.Kind {
		case ir.ScalarUint:
			return ir.SwitchValueU32(uint32(sv.Bits)), nil
		case ir.ScalarSint:
			return ir.SwitchValueI32(int32(sv.Bits)), nil
		default:
			return nil, fmt.Errorf("switch case selector '%s': must be integer", e.Name)
		}
	default:
		return nil, fmt.Errorf("switch case selector must be a literal or constant, got %T", expr)
	}
}

// literalToSwitchValue converts a literal expression to a switch value.
func (l *Lowerer) literalToSwitchValue(lit *Literal) (ir.SwitchValue, error) {
	switch lit.Kind {
	case TokenIntLiteral:
		val, suffix := parseIntLiteral(lit.Value)
		if suffix == "u" {
			// #nosec G115 -- WGSL switch case values are always 32-bit
			return ir.SwitchValueU32(uint32(val & 0xFFFFFFFF)), nil
		}
		// #nosec G115 -- WGSL switch case values are always 32-bit
		return ir.SwitchValueI32(int32(val & 0xFFFFFFFF)), nil
	default:
		return nil, fmt.Errorf("switch case selector must be an integer literal")
	}
}

// parseIntLiteral parses an integer literal and returns the value and suffix.
func parseIntLiteral(s string) (int64, string) {
	suffix := ""
	if len(s) > 0 && (s[len(s)-1] == 'u' || s[len(s)-1] == 'i') {
		suffix = string(s[len(s)-1])
		s = s[:len(s)-1]
	}
	val, _ := strconv.ParseInt(s, 0, 64)
	return val, suffix
}

// lowerLocalConst converts a local const declaration to IR.
// Local const is treated as a named expression (similar to let).
func (l *Lowerer) lowerLocalConst(decl *ConstDecl, target *[]ir.Statement) error {
	if decl.Init == nil {
		return fmt.Errorf("local const '%s' must have initializer", decl.Name)
	}

	// Lower the initializer expression
	initHandle, err := l.lowerExpression(decl.Init, target)
	if err != nil {
		return fmt.Errorf("const '%s' initializer: %w", decl.Name, err)
	}

	// Register as a named expression (like let)
	l.locals[decl.Name] = initHandle

	// Emit the expression at the declaration point to ensure correct SSA dominance.
	// Without this, the expression would be lazily emitted at its first use site,
	// which may be inside a branch block. If the same expression is also used in
	// another branch, the SPIR-V ID would not dominate its use.
	*target = append(*target, ir.Statement{Kind: ir.StmtEmit{
		Range: ir.Range{Start: initHandle, End: initHandle + 1},
	}})

	return nil
}

// lowerExpression converts an expression to IR.
func (l *Lowerer) lowerExpression(expr Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	switch e := expr.(type) {
	case *Literal:
		return l.lowerLiteral(e)
	case *Ident:
		return l.resolveIdentifier(e.Name)
	case *BinaryExpr:
		return l.lowerBinary(e, target)
	case *UnaryExpr:
		return l.lowerUnary(e, target)
	case *CallExpr:
		return l.lowerCall(e, target)
	case *ConstructExpr:
		return l.lowerConstruct(e, target)
	case *MemberExpr:
		return l.lowerMember(e, target)
	case *IndexExpr:
		return l.lowerIndex(e, target)
	default:
		return 0, fmt.Errorf("unsupported expression type: %T", expr)
	}
}

// lowerLiteral converts a literal to IR.
func (l *Lowerer) lowerLiteral(lit *Literal) (ir.ExpressionHandle, error) {
	var value ir.LiteralValue

	switch lit.Kind {
	case TokenIntLiteral:
		text := lit.Value
		isUnsigned := false
		if len(text) > 0 && text[len(text)-1] == 'u' {
			text = text[:len(text)-1]
			isUnsigned = true
		} else if len(text) > 0 && text[len(text)-1] == 'i' {
			text = text[:len(text)-1]
		}
		if isUnsigned {
			v, _ := strconv.ParseUint(text, 0, 32)
			value = ir.LiteralU32(v)
		} else {
			v, _ := strconv.ParseInt(text, 0, 32)
			value = ir.LiteralI32(v)
		}
	case TokenFloatLiteral:
		v, _ := strconv.ParseFloat(lit.Value, 32)
		value = ir.LiteralF32(v)
	case TokenTrue:
		value = ir.LiteralBool(true)
	case TokenFalse:
		value = ir.LiteralBool(false)
	case TokenBoolLiteral:
		// Parser normalizes TokenTrue/TokenFalse to TokenBoolLiteral
		value = ir.LiteralBool(lit.Value == "true")
	default:
		return 0, fmt.Errorf("unsupported literal kind: %v", lit.Kind)
	}

	return l.addExpression(ir.Expression{
		Kind: ir.Literal{Value: value},
	}), nil
}

// lowerBinary converts a binary expression to IR.
func (l *Lowerer) lowerBinary(bin *BinaryExpr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	left, err := l.lowerExpression(bin.Left, target)
	if err != nil {
		return 0, err
	}

	right, err := l.lowerExpression(bin.Right, target)
	if err != nil {
		return 0, err
	}

	op := l.tokenToBinaryOp(bin.Op)
	return l.addExpression(ir.Expression{
		Kind: ir.ExprBinary{Op: op, Left: left, Right: right},
	}), nil
}

// lowerUnary converts a unary expression to IR.
func (l *Lowerer) lowerUnary(un *UnaryExpr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	// Handle address-of operator (&)
	// For global variables in non-handle address spaces, the variable expression
	// already produces a pointer, so & is effectively a no-op
	if un.Op == TokenAmpersand {
		return l.lowerExpression(un.Operand, target)
	}

	// Handle dereference operator (*)
	if un.Op == TokenStar {
		pointer, err := l.lowerExpression(un.Operand, target)
		if err != nil {
			return 0, err
		}
		return l.addExpression(ir.Expression{
			Kind: ir.ExprLoad{Pointer: pointer},
		}), nil
	}

	operand, err := l.lowerExpression(un.Operand, target)
	if err != nil {
		return 0, err
	}

	op := l.tokenToUnaryOp(un.Op)
	return l.addExpression(ir.Expression{
		Kind: ir.ExprUnary{Op: op, Expr: operand},
	}), nil
}

// lowerCall converts a call expression to IR.
func (l *Lowerer) lowerCall(call *CallExpr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	funcName := call.Func.Name

	// Check if this is a built-in function (vec4, vec3, etc.)
	if l.isBuiltinConstructor(funcName) {
		return l.lowerBuiltinConstructor(funcName, call.Args, target)
	}

	// Check if this is the select() built-in (uses ExprSelect, not ExprMath)
	if funcName == "select" {
		return l.lowerSelectCall(call.Args, target)
	}

	// Check if this is a derivative function (dpdx, dpdy, fwidth + coarse/fine)
	if deriv, ok := l.getDerivativeFunction(funcName); ok {
		return l.lowerDerivativeCall(deriv, call.Args, target)
	}

	// Check if this is a relational function (all, any)
	if relFun, ok := l.getRelationalFunction(funcName); ok {
		return l.lowerRelationalCall(relFun, call.Args, target)
	}

	// Check if this is arrayLength
	if funcName == "arrayLength" {
		return l.lowerArrayLengthCall(call.Args, target)
	}

	// Check if this is a math function
	if mathFunc, ok := l.getMathFunction(funcName); ok {
		return l.lowerMathCall(mathFunc, call.Args, target)
	}

	// Check if this is a texture function
	if l.isTextureFunction(funcName) {
		return l.lowerTextureCall(funcName, call.Args, target)
	}

	// Check if this is atomicStore (special case - no result)
	if funcName == "atomicStore" {
		return l.lowerAtomicStore(call.Args, target)
	}

	// Check if this is atomicLoad (special case - 1 arg, returns value)
	if funcName == "atomicLoad" {
		return l.lowerAtomicLoad(call.Args, target)
	}

	// Check if this is an atomic function
	if atomicFunc := l.getAtomicFunction(funcName); atomicFunc != nil {
		return l.lowerAtomicCall(atomicFunc, call.Args, target)
	}

	// Check if this is atomicCompareExchangeWeak (special case - 3 args)
	if funcName == "atomicCompareExchangeWeak" {
		return l.lowerAtomicCompareExchange(call.Args, target)
	}

	// Check if this is a barrier function
	if barrierFlags := l.getBarrierFlags(funcName); barrierFlags != 0 {
		*target = append(*target, ir.Statement{
			Kind: ir.StmtBarrier{Flags: barrierFlags},
		})
		return 0, nil // Barriers don't return a value
	}

	// Regular function call - look up function handle
	funcHandle, ok := l.functions[funcName]
	if !ok {
		return 0, fmt.Errorf("unknown function: %s", funcName)
	}

	args := make([]ir.ExpressionHandle, len(call.Args))
	for i, arg := range call.Args {
		handle, err := l.lowerExpression(arg, target)
		if err != nil {
			return 0, err
		}
		args[i] = handle
	}

	// Create a call result expression
	resultHandle := l.addExpression(ir.Expression{
		Kind: ir.ExprCallResult{Function: funcHandle},
	})

	*target = append(*target, ir.Statement{
		Kind: ir.StmtCall{
			Function:  funcHandle,
			Arguments: args,
			Result:    &resultHandle,
		},
	})

	return resultHandle, nil
}

// lowerConstruct converts a type constructor to IR.
func (l *Lowerer) lowerConstruct(cons *ConstructExpr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	typeHandle, err := l.resolveType(cons.Type)
	if err != nil {
		return 0, err
	}

	components := make([]ir.ExpressionHandle, len(cons.Args))
	for i, arg := range cons.Args {
		handle, err := l.lowerExpression(arg, target)
		if err != nil {
			return 0, err
		}
		components[i] = handle
	}

	// For scalar type constructors with a single argument (e.g., f32(x), u32(y)),
	// generate ExprAs (type conversion) instead of ExprCompose.
	if len(components) == 1 {
		targetType := l.module.Types[typeHandle]
		if scalar, ok := targetType.Inner.(ir.ScalarType); ok {
			width := scalar.Width
			return l.addExpression(ir.Expression{
				Kind: ir.ExprAs{
					Expr:    components[0],
					Kind:    scalar.Kind,
					Convert: &width,
				},
			}), nil
		}
	}

	// WGSL vector type conversion: vec2<i32>(vec2<f32>) -> ExprAs conversion.
	// When constructing a vector from a single vector argument of different element type,
	// this is a type conversion, not a composition.
	targetType := l.module.Types[typeHandle]
	if vec, ok := targetType.Inner.(ir.VectorType); ok && len(components) == 1 {
		argType, err := ir.ResolveExpressionType(l.module, l.currentFunc, components[0])
		if err == nil {
			argInner := ir.TypeResInner(l.module, argType)
			if argVec, ok := argInner.(ir.VectorType); ok && argVec.Size == vec.Size {
				// Vector-to-vector conversion (e.g., vec2<i32>(vec2<f32>))
				width := vec.Scalar.Width
				return l.addExpression(ir.Expression{
					Kind: ir.ExprAs{
						Expr:    components[0],
						Kind:    vec.Scalar.Kind,
						Convert: &width,
					},
				}), nil
			}
		}

		// WGSL splat constructor: vec2(scalar) -> vec2(scalar, scalar), etc.
		// When constructing a vector from a single scalar, replicate to all components.
		needed := int(vec.Size)
		splatted := make([]ir.ExpressionHandle, needed)
		for i := 0; i < needed; i++ {
			splatted[i] = components[0]
		}
		components = splatted
	}

	return l.addExpression(ir.Expression{
		Kind: ir.ExprCompose{Type: typeHandle, Components: components},
	}), nil
}

// lowerMember converts a member access to IR.
func (l *Lowerer) lowerMember(mem *MemberExpr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	base, err := l.lowerExpression(mem.Expr, target)
	if err != nil {
		return 0, err
	}

	baseType, err := ir.ResolveExpressionType(l.module, l.currentFunc, base)
	if err != nil {
		return 0, fmt.Errorf("member access base type: %w", err)
	}

	if index, ok, err := l.structMemberIndex(baseType, mem.Member); err != nil {
		return 0, err
	} else if ok {
		return l.addExpression(ir.Expression{
			Kind: ir.ExprAccessIndex{Base: base, Index: index},
		}), nil
	}

	vec, ok, err := l.vectorType(baseType)
	if err != nil {
		return 0, err
	}

	if !ok {
		return 0, fmt.Errorf("unsupported member access %q", mem.Member)
	}

	if len(mem.Member) == 1 {
		index, err := l.swizzleIndex(mem.Member, vec.Size)
		if err != nil {
			return 0, err
		}
		return l.addExpression(ir.Expression{
			Kind: ir.ExprAccessIndex{Base: base, Index: index},
		}), nil
	}
	size, pattern, err := l.swizzlePattern(mem.Member, vec.Size)
	if err != nil {
		return 0, err
	}
	return l.addExpression(ir.Expression{
		Kind: ir.ExprSwizzle{Size: size, Vector: base, Pattern: pattern},
	}), nil
}

// lowerIndex converts an index expression to IR.
func (l *Lowerer) lowerIndex(idx *IndexExpr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	base, err := l.lowerExpression(idx.Expr, target)
	if err != nil {
		return 0, err
	}

	index, err := l.lowerExpression(idx.Index, target)
	if err != nil {
		return 0, err
	}

	return l.addExpression(ir.Expression{
		Kind: ir.ExprAccess{Base: base, Index: index},
	}), nil
}

// Helper methods

func (l *Lowerer) addExpression(expr ir.Expression) ir.ExpressionHandle {
	handle := l.currentExprIdx
	l.currentExprIdx++
	l.currentFunc.Expressions = append(l.currentFunc.Expressions, expr)

	// Resolve and store the expression type
	exprType, err := ir.ResolveExpressionType(l.module, l.currentFunc, handle)
	if err != nil {
		// Store an empty type resolution on error - validation will catch this later
		exprType = ir.TypeResolution{}
	}
	l.currentFunc.ExpressionTypes = append(l.currentFunc.ExpressionTypes, exprType)

	return handle
}

func (l *Lowerer) resolveIdentifier(name string) (ir.ExpressionHandle, error) {
	// Check locals first
	if handle, ok := l.locals[name]; ok {
		// Mark as used for unused variable warnings
		l.usedLocals[name] = true
		return handle, nil
	}

	// Check module-level constants
	if handle, ok := l.moduleConstants[name]; ok {
		return l.addExpression(ir.Expression{
			Kind: ir.ExprConstant{Constant: handle},
		}), nil
	}

	// Check globals
	if handle, ok := l.globals[name]; ok {
		return l.addExpression(ir.Expression{
			Kind: ir.ExprGlobalVariable{Variable: handle},
		}), nil
	}

	return 0, fmt.Errorf("unresolved identifier: %s", name)
}

// resolveType converts a WGSL type to an IR type handle.
func (l *Lowerer) resolveType(typ Type) (ir.TypeHandle, error) {
	switch t := typ.(type) {
	case *NamedType:
		return l.resolveNamedType(t)
	case *ArrayType:
		base, err := l.resolveType(t.Element)
		if err != nil {
			return 0, err
		}
		// Parse size expression if present
		var size ir.ArraySize
		if t.Size != nil {
			if lit, ok := t.Size.(*Literal); ok && lit.Kind == TokenIntLiteral {
				n, _ := strconv.ParseUint(lit.Value, 0, 32)
				constSize := uint32(n)
				size.Constant = &constSize
			}
		}
		// Compute element stride for SPIR-V ArrayStride decoration.
		// Runtime arrays are always in storage buffers (std430 layout),
		// so stride = roundUp(elemAlign, elemSize).
		elemAlign, elemSize := l.typeAlignmentAndSize(base)
		stride := (elemSize + elemAlign - 1) &^ (elemAlign - 1)
		return l.registerType("", ir.ArrayType{Base: base, Size: size, Stride: stride}), nil
	case *PtrType:
		pointee, err := l.resolveType(t.PointeeType)
		if err != nil {
			return 0, err
		}
		space := l.addressSpace(t.AddressSpace)
		return l.registerType("", ir.PointerType{Base: pointee, Space: space}), nil
	default:
		return 0, fmt.Errorf("unsupported type: %T", typ)
	}
}

func (l *Lowerer) resolveNamedType(t *NamedType) (ir.TypeHandle, error) {
	// Check for built-in vector types
	if len(t.TypeParams) > 0 {
		return l.resolveParameterizedType(t)
	}

	// Look up simple named type
	if handle, ok := l.types[t.Name]; ok {
		return handle, nil
	}

	// Lazy registration of sampler types (only when actually used in shader)
	// This prevents OpTypeSampler from appearing in shaders without textures
	switch t.Name {
	case "sampler":
		return l.registerType("sampler", ir.SamplerType{Comparison: false}), nil
	case "sampler_comparison":
		return l.registerType("sampler_comparison", ir.SamplerType{Comparison: true}), nil
	}

	return 0, fmt.Errorf("unknown type: %s", t.Name)
}

func (l *Lowerer) resolveParameterizedType(t *NamedType) (ir.TypeHandle, error) {
	// Vector types: vec2<f32>, vec3<T>, vec4<T>
	if len(t.Name) == 4 && t.Name[:3] == "vec" {
		size := t.Name[3] - '0'
		scalarType, err := l.resolveType(t.TypeParams[0])
		if err != nil {
			return 0, err
		}
		// Get scalar from registry
		typ, ok := l.registry.Lookup(scalarType)
		if !ok {
			return 0, fmt.Errorf("scalar type handle %d not found in registry", scalarType)
		}
		scalar := typ.Inner.(ir.ScalarType)
		return l.registerType("", ir.VectorType{
			Size:   ir.VectorSize(size),
			Scalar: scalar,
		}), nil
	}

	// Matrix types: mat2x2<f32>, mat4x4<f32>
	if len(t.Name) >= 3 && t.Name[:3] == "mat" {
		// Simple parsing: mat4x4 -> 4 columns, 4 rows
		cols := t.Name[3] - '0'
		rows := t.Name[5] - '0'
		scalarType, err := l.resolveType(t.TypeParams[0])
		if err != nil {
			return 0, err
		}
		// Get scalar from registry
		typ, ok := l.registry.Lookup(scalarType)
		if !ok {
			return 0, fmt.Errorf("scalar type handle %d not found in registry", scalarType)
		}
		scalar := typ.Inner.(ir.ScalarType)
		return l.registerType("", ir.MatrixType{
			Columns: ir.VectorSize(cols),
			Rows:    ir.VectorSize(rows),
			Scalar:  scalar,
		}), nil
	}

	// Texture types: texture_2d<f32>, texture_storage_2d<rgba8unorm, write>, etc.
	if len(t.Name) >= 7 && t.Name[:7] == "texture" {
		imgType := l.parseTextureType(t)
		return l.registerType("", imgType), nil
	}

	// Atomic types: atomic<u32>, atomic<i32>
	if t.Name == "atomic" {
		if len(t.TypeParams) != 1 {
			return 0, fmt.Errorf("atomic type requires exactly one type parameter")
		}
		scalarType, err := l.resolveType(t.TypeParams[0])
		if err != nil {
			return 0, err
		}
		typ, ok := l.registry.Lookup(scalarType)
		if !ok {
			return 0, fmt.Errorf("scalar type handle %d not found in registry", scalarType)
		}
		scalar := typ.Inner.(ir.ScalarType)
		return l.registerType("", ir.AtomicType{
			Scalar: scalar,
		}), nil
	}

	return 0, fmt.Errorf("unsupported parameterized type: %s", t.Name)
}

func (l *Lowerer) isBuiltinConstructor(name string) bool {
	return (len(name) == 4 && name[:3] == "vec") ||
		(len(name) >= 3 && name[:3] == "mat") ||
		name == "array"
}

func (l *Lowerer) lowerBuiltinConstructor(name string, args []Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	// Lower all arguments first
	components := make([]ir.ExpressionHandle, len(args))
	for i, arg := range args {
		handle, err := l.lowerExpression(arg, target)
		if err != nil {
			return 0, err
		}
		components[i] = handle
	}

	// Infer type from constructor name
	var typeHandle ir.TypeHandle

	switch {
	case len(name) == 4 && name[:3] == "vec":
		// vec2, vec3, vec4
		size := name[3] - '0'
		// Assume f32 for now
		scalar := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
		typeHandle = l.registerType("", ir.VectorType{
			Size:   ir.VectorSize(size),
			Scalar: scalar,
		})

	case name == "array":
		// array(...) with inferred element type and size
		if len(args) == 0 {
			return 0, fmt.Errorf("array constructor requires at least one element")
		}

		// Infer element type from first argument
		elemType, err := l.inferTypeFromExpression(components[0])
		if err != nil {
			return 0, fmt.Errorf("cannot infer array element type: %w", err)
		}

		// Create array type with fixed size
		constSize := uint32(len(args))
		arraySize := ir.ArraySize{
			Constant: &constSize,
		}
		elemAlign, elemSize := l.typeAlignmentAndSize(elemType)
		arrStride := (elemSize + elemAlign - 1) &^ (elemAlign - 1)
		typeHandle = l.registerType("", ir.ArrayType{
			Base:   elemType,
			Size:   arraySize,
			Stride: arrStride,
		})
	}

	return l.addExpression(ir.Expression{
		Kind: ir.ExprCompose{Type: typeHandle, Components: components},
	}), nil
}

// mathFuncTable is a package-level lookup table for WGSL math function names to IR math functions.
// Declared at package scope to avoid reallocating the map on every call to getMathFunction.
var mathFuncTable = map[string]ir.MathFunction{
	// Comparison functions
	"abs":      ir.MathAbs,
	"min":      ir.MathMin,
	"max":      ir.MathMax,
	"clamp":    ir.MathClamp,
	"saturate": ir.MathSaturate,

	// Trigonometric functions
	"cos":   ir.MathCos,
	"cosh":  ir.MathCosh,
	"sin":   ir.MathSin,
	"sinh":  ir.MathSinh,
	"tan":   ir.MathTan,
	"tanh":  ir.MathTanh,
	"acos":  ir.MathAcos,
	"asin":  ir.MathAsin,
	"atan":  ir.MathAtan,
	"atan2": ir.MathAtan2,
	"asinh": ir.MathAsinh,
	"acosh": ir.MathAcosh,
	"atanh": ir.MathAtanh,

	// Angle conversion
	"radians": ir.MathRadians,
	"degrees": ir.MathDegrees,

	// Decomposition functions
	"ceil":  ir.MathCeil,
	"floor": ir.MathFloor,
	"round": ir.MathRound,
	"fract": ir.MathFract,
	"trunc": ir.MathTrunc,

	// Exponential functions
	"exp":  ir.MathExp,
	"exp2": ir.MathExp2,
	"log":  ir.MathLog,
	"log2": ir.MathLog2,
	"pow":  ir.MathPow,

	// Geometric functions
	"dot":         ir.MathDot,
	"cross":       ir.MathCross,
	"distance":    ir.MathDistance,
	"length":      ir.MathLength,
	"normalize":   ir.MathNormalize,
	"faceForward": ir.MathFaceForward,
	"reflect":     ir.MathReflect,
	"refract":     ir.MathRefract,

	// Computational functions
	"sign":        ir.MathSign,
	"fma":         ir.MathFma,
	"mix":         ir.MathMix,
	"step":        ir.MathStep,
	"smoothstep":  ir.MathSmoothStep,
	"sqrt":        ir.MathSqrt,
	"inverseSqrt": ir.MathInverseSqrt,

	// Matrix functions
	"transpose":   ir.MathTranspose,
	"determinant": ir.MathDeterminant,

	// Bit manipulation functions
	"countTrailingZeros": ir.MathCountTrailingZeros,
	"countLeadingZeros":  ir.MathCountLeadingZeros,
	"countOneBits":       ir.MathCountOneBits,
	"reverseBits":        ir.MathReverseBits,
	"extractBits":        ir.MathExtractBits,
	"insertBits":         ir.MathInsertBits,
	"firstTrailingBit":   ir.MathFirstTrailingBit,
	"firstLeadingBit":    ir.MathFirstLeadingBit,

	// Data packing functions
	"pack4x8snorm":  ir.MathPack4x8snorm,
	"pack4x8unorm":  ir.MathPack4x8unorm,
	"pack2x16snorm": ir.MathPack2x16snorm,
	"pack2x16unorm": ir.MathPack2x16unorm,
	"pack2x16float": ir.MathPack2x16float,

	// Data unpacking functions
	"unpack4x8snorm":  ir.MathUnpack4x8snorm,
	"unpack4x8unorm":  ir.MathUnpack4x8unorm,
	"unpack2x16snorm": ir.MathUnpack2x16snorm,
	"unpack2x16unorm": ir.MathUnpack2x16unorm,
	"unpack2x16float": ir.MathUnpack2x16float,
	"unpack4xI8":      ir.MathUnpack4xI8,
	"unpack4xU8":      ir.MathUnpack4xU8,
	"pack4xI8":        ir.MathPack4xI8,
	"pack4xU8":        ir.MathPack4xU8,
	"pack4xI8Clamp":   ir.MathPack4xI8Clamp,
	"pack4xU8Clamp":   ir.MathPack4xU8Clamp,

	// Decomposition functions (struct return)
	"modf":  ir.MathModf,
	"frexp": ir.MathFrexp,
	"ldexp": ir.MathLdexp,

	// Matrix functions
	"inverse": ir.MathInverse,

	// Precision
	"quantizeToF16": ir.MathQuantizeF16,

	// Vector/matrix operations
	"outerProduct": ir.MathOuter,
}

func (l *Lowerer) getMathFunction(name string) (ir.MathFunction, bool) {
	fn, ok := mathFuncTable[name]
	return fn, ok
}

func (l *Lowerer) lowerMathCall(mathFunc ir.MathFunction, args []Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) == 0 {
		return 0, fmt.Errorf("math function requires at least one argument")
	}

	arg0, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	var arg1, arg2, arg3 *ir.ExpressionHandle
	if len(args) > 1 {
		a, err := l.lowerExpression(args[1], target)
		if err != nil {
			return 0, err
		}
		arg1 = &a
	}
	if len(args) > 2 {
		a, err := l.lowerExpression(args[2], target)
		if err != nil {
			return 0, err
		}
		arg2 = &a
	}
	if len(args) > 3 {
		a, err := l.lowerExpression(args[3], target)
		if err != nil {
			return 0, err
		}
		arg3 = &a
	}

	return l.addExpression(ir.Expression{
		Kind: ir.ExprMath{
			Fun:  mathFunc,
			Arg:  arg0,
			Arg1: arg1,
			Arg2: arg2,
			Arg3: arg3,
		},
	}), nil
}

// lowerSelectCall converts select(falseVal, trueVal, condition) to IR ExprSelect.
// WGSL select() has signature: select(f, t, cond) -- returns t if cond is true, f otherwise.
func (l *Lowerer) lowerSelectCall(args []Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) != 3 {
		return 0, fmt.Errorf("select() requires exactly 3 arguments, got %d", len(args))
	}

	// WGSL: select(falseVal, trueVal, condition)
	falseVal, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	trueVal, err := l.lowerExpression(args[1], target)
	if err != nil {
		return 0, err
	}

	condition, err := l.lowerExpression(args[2], target)
	if err != nil {
		return 0, err
	}

	return l.addExpression(ir.Expression{
		Kind: ir.ExprSelect{
			Condition: condition,
			Accept:    trueVal,
			Reject:    falseVal,
		},
	}), nil
}

// getDerivativeFunction maps WGSL derivative function names to IR derivative parameters.
func (l *Lowerer) getDerivativeFunction(name string) (ir.ExprDerivative, bool) {
	switch name {
	case "dpdx":
		return ir.ExprDerivative{Axis: ir.DerivativeX, Control: ir.DerivativeNone}, true
	case "dpdy":
		return ir.ExprDerivative{Axis: ir.DerivativeY, Control: ir.DerivativeNone}, true
	case "fwidth":
		return ir.ExprDerivative{Axis: ir.DerivativeWidth, Control: ir.DerivativeNone}, true
	case "dpdxCoarse":
		return ir.ExprDerivative{Axis: ir.DerivativeX, Control: ir.DerivativeCoarse}, true
	case "dpdyCoarse":
		return ir.ExprDerivative{Axis: ir.DerivativeY, Control: ir.DerivativeCoarse}, true
	case "fwidthCoarse":
		return ir.ExprDerivative{Axis: ir.DerivativeWidth, Control: ir.DerivativeCoarse}, true
	case "dpdxFine":
		return ir.ExprDerivative{Axis: ir.DerivativeX, Control: ir.DerivativeFine}, true
	case "dpdyFine":
		return ir.ExprDerivative{Axis: ir.DerivativeY, Control: ir.DerivativeFine}, true
	case "fwidthFine":
		return ir.ExprDerivative{Axis: ir.DerivativeWidth, Control: ir.DerivativeFine}, true
	default:
		return ir.ExprDerivative{}, false
	}
}

func (l *Lowerer) lowerDerivativeCall(deriv ir.ExprDerivative, args []Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) != 1 {
		return 0, fmt.Errorf("derivative function requires exactly 1 argument, got %d", len(args))
	}
	expr, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}
	deriv.Expr = expr
	return l.addExpression(ir.Expression{Kind: deriv}), nil
}

// getRelationalFunction maps WGSL relational function names to IR.
func (l *Lowerer) getRelationalFunction(name string) (ir.RelationalFunction, bool) {
	switch name {
	case "all":
		return ir.RelationalAll, true
	case "any":
		return ir.RelationalAny, true
	case "isnan":
		return ir.RelationalIsNan, true
	case "isinf":
		return ir.RelationalIsInf, true
	default:
		return 0, false
	}
}

func (l *Lowerer) lowerRelationalCall(fun ir.RelationalFunction, args []Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) != 1 {
		return 0, fmt.Errorf("relational function requires exactly 1 argument, got %d", len(args))
	}
	arg, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}
	return l.addExpression(ir.Expression{
		Kind: ir.ExprRelational{Fun: fun, Argument: arg},
	}), nil
}

func (l *Lowerer) lowerArrayLengthCall(args []Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) != 1 {
		return 0, fmt.Errorf("arrayLength requires exactly 1 argument, got %d", len(args))
	}
	arg, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}
	return l.addExpression(ir.Expression{
		Kind: ir.ExprArrayLength{Array: arg},
	}), nil
}

// Attribute parsing

func (l *Lowerer) paramBinding(attrs []Attribute) *ir.Binding {
	for _, attr := range attrs {
		switch attr.Name {
		case "builtin":
			if len(attr.Args) > 0 {
				if id, ok := attr.Args[0].(*Ident); ok {
					var binding ir.Binding = ir.BuiltinBinding{Builtin: l.builtin(id.Name)}
					return &binding
				}
			}
		case "location":
			if len(attr.Args) > 0 {
				if lit, ok := attr.Args[0].(*Literal); ok {
					loc, _ := strconv.ParseUint(lit.Value, 10, 32)
					var binding ir.Binding = ir.LocationBinding{Location: uint32(loc)}
					return &binding
				}
			}
		}
	}
	return nil
}

func (l *Lowerer) returnBinding(attrs []Attribute) *ir.Binding {
	return l.paramBinding(attrs) // Same logic for return bindings
}

// memberBinding extracts binding from a single struct member attribute.
func (l *Lowerer) memberBinding(attr *Attribute) *ir.Binding {
	switch attr.Name {
	case "builtin":
		if len(attr.Args) > 0 {
			if id, ok := attr.Args[0].(*Ident); ok {
				var binding ir.Binding = ir.BuiltinBinding{Builtin: l.builtin(id.Name)}
				return &binding
			}
		}
	case "location":
		if len(attr.Args) > 0 {
			if lit, ok := attr.Args[0].(*Literal); ok {
				loc, _ := strconv.ParseUint(lit.Value, 10, 32)
				var binding ir.Binding = ir.LocationBinding{Location: uint32(loc)}
				return &binding
			}
		}
	}
	return nil
}

func (l *Lowerer) entryPointStage(attrs []Attribute) *ir.ShaderStage {
	for _, attr := range attrs {
		switch attr.Name {
		case "vertex":
			stage := ir.StageVertex
			return &stage
		case "fragment":
			stage := ir.StageFragment
			return &stage
		case "compute":
			stage := ir.StageCompute
			return &stage
		}
	}
	return nil
}

// extractWorkgroupSize extracts workgroup_size from attributes.
// Returns [x, y, z] where defaults are 1.
func (l *Lowerer) extractWorkgroupSize(attrs []Attribute) [3]uint32 {
	result := [3]uint32{1, 1, 1}
	for _, attr := range attrs {
		if attr.Name != "workgroup_size" {
			continue
		}
		for i, arg := range attr.Args {
			if i >= 3 {
				break
			}
			if lit, ok := arg.(*Literal); ok {
				if val, err := strconv.ParseUint(lit.Value, 10, 32); err == nil {
					result[i] = uint32(val)
				}
			}
		}
		break
	}
	return result
}

// builtinTable maps WGSL builtin names to IR builtin values.
var builtinTable = map[string]ir.BuiltinValue{
	"position":               ir.BuiltinPosition,
	"vertex_index":           ir.BuiltinVertexIndex,
	"instance_index":         ir.BuiltinInstanceIndex,
	"front_facing":           ir.BuiltinFrontFacing,
	"frag_depth":             ir.BuiltinFragDepth,
	"local_invocation_id":    ir.BuiltinLocalInvocationID,
	"local_invocation_index": ir.BuiltinLocalInvocationIndex,
	"global_invocation_id":   ir.BuiltinGlobalInvocationID,
	"workgroup_id":           ir.BuiltinWorkGroupID,
	"num_workgroups":         ir.BuiltinNumWorkGroups,
}

func (l *Lowerer) builtin(name string) ir.BuiltinValue {
	if b, ok := builtinTable[name]; ok {
		return b
	}
	return ir.BuiltinPosition // Default
}

// addressSpaceTable maps WGSL address space names to IR address spaces.
var addressSpaceTable = map[string]ir.AddressSpace{
	"function":      ir.SpaceFunction,
	"private":       ir.SpacePrivate,
	"workgroup":     ir.SpaceWorkGroup,
	"uniform":       ir.SpaceUniform,
	"storage":       ir.SpaceStorage,
	"push_constant": ir.SpacePushConstant,
	"handle":        ir.SpaceHandle,
}

func (l *Lowerer) addressSpace(space string) ir.AddressSpace {
	if s, ok := addressSpaceTable[space]; ok {
		return s
	}
	return ir.SpaceFunction // Default
}

// isOpaqueResourceType checks if a type is an opaque resource (sampler or image/texture).
// These types require SpaceHandle address space (UniformConstant in SPIR-V).
func (l *Lowerer) isOpaqueResourceType(handle ir.TypeHandle) bool {
	if int(handle) >= len(l.module.Types) {
		return false
	}
	typ := l.module.Types[handle]
	switch typ.Inner.(type) {
	case ir.SamplerType, ir.ImageType:
		return true
	default:
		return false
	}
}

// parseTextureType parses a texture type specification and returns an ImageType.
// Handles: texture_2d<f32>, texture_storage_2d<rgba8unorm, write>, texture_depth_2d, etc.
func (l *Lowerer) parseTextureType(t *NamedType) ir.ImageType {
	name := t.Name
	img := ir.ImageType{
		Dim:   ir.Dim2D,
		Class: ir.ImageClassSampled,
	}

	// Check for storage textures: texture_storage_1d, texture_storage_2d, etc.
	if strings.HasPrefix(name, "texture_storage_") {
		img.Class = ir.ImageClassStorage
		suffix := name[16:] // After "texture_storage_"
		img.Dim = l.parseTextureDimSuffix(suffix)
		img.Arrayed = strings.Contains(suffix, "_array")

		// Parse format and access from type params: <rgba8unorm, write>
		if len(t.TypeParams) >= 1 {
			img.StorageFormat = l.parseStorageFormat(t.TypeParams[0])
		}
		if len(t.TypeParams) >= 2 {
			img.StorageAccess = l.parseStorageAccess(t.TypeParams[1])
		}
		return img
	}

	// Check for depth textures: texture_depth_2d, texture_depth_cube, etc.
	if strings.HasPrefix(name, "texture_depth_") {
		img.Class = ir.ImageClassDepth
		suffix := name[14:] // After "texture_depth_"
		img.Dim = l.parseTextureDimSuffix(suffix)
		img.Arrayed = strings.Contains(suffix, "_array")
		img.Multisampled = strings.Contains(suffix, "multisampled")
		return img
	}

	// Check for multisampled textures: texture_multisampled_2d
	if strings.HasPrefix(name, "texture_multisampled_") {
		img.Multisampled = true
		suffix := name[21:] // After "texture_multisampled_"
		img.Dim = l.parseTextureDimSuffix(suffix)
		return img
	}

	// Regular sampled textures: texture_1d, texture_2d, texture_3d, texture_cube, etc.
	suffix := name[8:] // After "texture_"
	img.Dim = l.parseTextureDimSuffix(suffix)
	img.Arrayed = strings.Contains(suffix, "_array")

	return img
}

// parseTextureDimSuffix parses dimension from suffix like "1d", "2d", "3d", "cube", "2d_array".
func (l *Lowerer) parseTextureDimSuffix(suffix string) ir.ImageDimension {
	if strings.HasPrefix(suffix, "1d") {
		return ir.Dim1D
	}
	if strings.HasPrefix(suffix, "2d") {
		return ir.Dim2D
	}
	if strings.HasPrefix(suffix, "3d") {
		return ir.Dim3D
	}
	if strings.HasPrefix(suffix, "cube") {
		return ir.DimCube
	}
	return ir.Dim2D
}

// parseStorageFormat parses a storage texture format from a type parameter.
// storageFormatTable maps WGSL storage format names to IR storage formats.
var storageFormatTable = map[string]ir.StorageFormat{
	// 8-bit formats
	"r8unorm": ir.StorageFormatR8Unorm,
	"r8snorm": ir.StorageFormatR8Snorm,
	"r8uint":  ir.StorageFormatR8Uint,
	"r8sint":  ir.StorageFormatR8Sint,

	// 16-bit formats
	"r16uint":  ir.StorageFormatR16Uint,
	"r16sint":  ir.StorageFormatR16Sint,
	"r16float": ir.StorageFormatR16Float,
	"rg8unorm": ir.StorageFormatRg8Unorm,
	"rg8snorm": ir.StorageFormatRg8Snorm,
	"rg8uint":  ir.StorageFormatRg8Uint,
	"rg8sint":  ir.StorageFormatRg8Sint,

	// 32-bit formats
	"r32uint":    ir.StorageFormatR32Uint,
	"r32sint":    ir.StorageFormatR32Sint,
	"r32float":   ir.StorageFormatR32Float,
	"rg16uint":   ir.StorageFormatRg16Uint,
	"rg16sint":   ir.StorageFormatRg16Sint,
	"rg16float":  ir.StorageFormatRg16Float,
	"rgba8unorm": ir.StorageFormatRgba8Unorm,
	"rgba8snorm": ir.StorageFormatRgba8Snorm,
	"rgba8uint":  ir.StorageFormatRgba8Uint,
	"rgba8sint":  ir.StorageFormatRgba8Sint,
	"bgra8unorm": ir.StorageFormatBgra8Unorm,

	// Packed 32-bit formats
	"rgb10a2uint":  ir.StorageFormatRgb10a2Uint,
	"rgb10a2unorm": ir.StorageFormatRgb10a2Unorm,
	"rg11b10float": ir.StorageFormatRg11b10Ufloat,

	// 64-bit formats
	"rg32uint":    ir.StorageFormatRg32Uint,
	"rg32sint":    ir.StorageFormatRg32Sint,
	"rg32float":   ir.StorageFormatRg32Float,
	"rgba16uint":  ir.StorageFormatRgba16Uint,
	"rgba16sint":  ir.StorageFormatRgba16Sint,
	"rgba16float": ir.StorageFormatRgba16Float,

	// 128-bit formats
	"rgba32uint":  ir.StorageFormatRgba32Uint,
	"rgba32sint":  ir.StorageFormatRgba32Sint,
	"rgba32float": ir.StorageFormatRgba32Float,

	// Normalized 16-bit per channel formats
	"r16unorm":    ir.StorageFormatR16Unorm,
	"r16snorm":    ir.StorageFormatR16Snorm,
	"rg16unorm":   ir.StorageFormatRg16Unorm,
	"rg16snorm":   ir.StorageFormatRg16Snorm,
	"rgba16unorm": ir.StorageFormatRgba16Unorm,
	"rgba16snorm": ir.StorageFormatRgba16Snorm,
}

func (l *Lowerer) parseStorageFormat(param Type) ir.StorageFormat {
	// The format is typically an identifier like "rgba8unorm"
	namedType, ok := param.(*NamedType)
	if !ok {
		return ir.StorageFormatUnknown
	}
	if format, ok := storageFormatTable[namedType.Name]; ok {
		return format
	}
	return ir.StorageFormatUnknown
}

// parseStorageAccess parses a storage texture access mode from a type parameter.
func (l *Lowerer) parseStorageAccess(param Type) ir.StorageAccess {
	namedType, ok := param.(*NamedType)
	if !ok {
		return ir.StorageAccessWrite
	}
	name := namedType.Name
	switch name {
	case "read":
		return ir.StorageAccessRead
	case "write":
		return ir.StorageAccessWrite
	case "read_write":
		return ir.StorageAccessReadWrite
	default:
		return ir.StorageAccessWrite // Default to write
	}
}

// binaryOpTable maps token kinds to binary operators.
var binaryOpTable = map[TokenKind]ir.BinaryOperator{
	TokenPlus:           ir.BinaryAdd,
	TokenMinus:          ir.BinarySubtract,
	TokenStar:           ir.BinaryMultiply,
	TokenSlash:          ir.BinaryDivide,
	TokenPercent:        ir.BinaryModulo,
	TokenEqualEqual:     ir.BinaryEqual,
	TokenBangEqual:      ir.BinaryNotEqual,
	TokenLess:           ir.BinaryLess,
	TokenLessEqual:      ir.BinaryLessEqual,
	TokenGreater:        ir.BinaryGreater,
	TokenGreaterEqual:   ir.BinaryGreaterEqual,
	TokenAmpAmp:         ir.BinaryLogicalAnd,
	TokenPipePipe:       ir.BinaryLogicalOr,
	TokenAmpersand:      ir.BinaryAnd,
	TokenPipe:           ir.BinaryInclusiveOr,
	TokenCaret:          ir.BinaryExclusiveOr,
	TokenLessLess:       ir.BinaryShiftLeft,
	TokenGreaterGreater: ir.BinaryShiftRight,
}

// unaryOpTable maps token kinds to unary operators.
var unaryOpTable = map[TokenKind]ir.UnaryOperator{
	TokenMinus: ir.UnaryNegate,
	TokenBang:  ir.UnaryLogicalNot,
	TokenTilde: ir.UnaryBitwiseNot,
}

func (l *Lowerer) tokenToBinaryOp(tok TokenKind) ir.BinaryOperator {
	if op, ok := binaryOpTable[tok]; ok {
		return op
	}
	return ir.BinaryAdd // Default
}

func (l *Lowerer) tokenToUnaryOp(tok TokenKind) ir.UnaryOperator {
	if op, ok := unaryOpTable[tok]; ok {
		return op
	}
	return ir.UnaryNegate // Default
}

// checkUnusedVariables reports warnings for local variables that are declared but never used.
func (l *Lowerer) checkUnusedVariables(funcName string) {
	for name, span := range l.localDecls {
		if !l.usedLocals[name] {
			// Variables starting with _ are intentionally unused
			if len(name) > 0 && name[0] == '_' {
				continue
			}
			l.warnings = append(l.warnings, Warning{
				Message: fmt.Sprintf("unused variable '%s' in function '%s'", name, funcName),
				Span:    span,
			})
		}
	}
}

// assignOpTable maps compound assignment token kinds to binary operators.
var assignOpTable = map[TokenKind]ir.BinaryOperator{
	TokenPlusEqual:  ir.BinaryAdd,
	TokenMinusEqual: ir.BinarySubtract,
	TokenStarEqual:  ir.BinaryMultiply,
	TokenSlashEqual: ir.BinaryDivide,
}

func (l *Lowerer) assignOpToBinary(tok TokenKind) ir.BinaryOperator {
	if op, ok := assignOpTable[tok]; ok {
		return op
	}
	return ir.BinaryAdd // Default
}

func (l *Lowerer) structMemberIndex(base ir.TypeResolution, name string) (uint32, bool, error) {
	inner, ok, err := l.resolveTypeInner(base)
	if err != nil {
		return 0, false, err
	}
	if !ok {
		return 0, false, nil
	}
	st, isStruct := inner.(ir.StructType)
	if !isStruct {
		return 0, false, nil
	}
	var idx uint32
	for _, member := range st.Members {
		if member.Name == name {
			return idx, true, nil
		}
		idx++
	}
	return 0, false, fmt.Errorf("struct has no member %q", name)
}

func (l *Lowerer) vectorType(base ir.TypeResolution) (ir.VectorType, bool, error) {
	inner, ok, err := l.resolveTypeInner(base)
	if err != nil {
		return ir.VectorType{}, false, err
	}
	if !ok {
		return ir.VectorType{}, false, nil
	}
	if vec, isVec := inner.(ir.VectorType); isVec {
		return vec, true, nil
	}
	return ir.VectorType{}, false, nil
}

func (l *Lowerer) resolveTypeInner(base ir.TypeResolution) (ir.TypeInner, bool, error) {
	resolvePointer := func(inner ir.TypeInner) (ir.TypeInner, error) {
		pt, ok := inner.(ir.PointerType)
		if !ok {
			return inner, nil
		}
		baseType, ok := l.registry.Lookup(pt.Base)
		if !ok {
			return nil, fmt.Errorf("pointer base type %d out of range", pt.Base)
		}
		return baseType.Inner, nil
	}

	if base.Handle != nil {
		handle := *base.Handle
		typ, ok := l.registry.Lookup(handle)
		if !ok {
			return nil, false, fmt.Errorf("type handle %d out of range", handle)
		}
		inner, err := resolvePointer(typ.Inner)
		if err != nil {
			return nil, false, err
		}
		return inner, true, nil
	}
	if base.Value != nil {
		inner, err := resolvePointer(base.Value)
		if err != nil {
			return nil, false, err
		}
		return inner, true, nil
	}
	return nil, false, nil
}

func (l *Lowerer) swizzleIndex(member string, vecSize ir.VectorSize) (uint32, error) {
	if len(member) != 1 {
		return 0, fmt.Errorf("invalid swizzle %q", member)
	}
	comp, ok := swizzleComponent(member[0])
	if !ok {
		return 0, fmt.Errorf("invalid swizzle component %q", member)
	}
	if uint8(comp) >= uint8(vecSize) {
		return 0, fmt.Errorf("swizzle component %q out of range for vec%v", member, vecSize)
	}
	return uint32(comp), nil
}

func (l *Lowerer) swizzlePattern(member string, vecSize ir.VectorSize) (ir.VectorSize, [4]ir.SwizzleComponent, error) {
	if len(member) < 2 || len(member) > 4 {
		return 0, [4]ir.SwizzleComponent{}, fmt.Errorf("invalid swizzle %q", member)
	}
	var pattern [4]ir.SwizzleComponent
	for i := 0; i < len(member); i++ {
		comp, ok := swizzleComponent(member[i])
		if !ok {
			return 0, [4]ir.SwizzleComponent{}, fmt.Errorf("invalid swizzle component %q", member)
		}
		if uint8(comp) >= uint8(vecSize) {
			return 0, [4]ir.SwizzleComponent{}, fmt.Errorf("swizzle component %q out of range for vec%v", member, vecSize)
		}
		pattern[i] = comp
	}
	var size ir.VectorSize
	switch len(member) {
	case 2:
		size = ir.Vec2
	case 3:
		size = ir.Vec3
	case 4:
		size = ir.Vec4
	default:
		return 0, [4]ir.SwizzleComponent{}, fmt.Errorf("invalid swizzle %q", member)
	}
	return size, pattern, nil
}

func swizzleComponent(c byte) (ir.SwizzleComponent, bool) {
	switch c {
	case 'x', 'r', 's':
		return ir.SwizzleX, true
	case 'y', 'g', 't':
		return ir.SwizzleY, true
	case 'z', 'b', 'p':
		return ir.SwizzleZ, true
	case 'w', 'a', 'q':
		return ir.SwizzleW, true
	default:
		return 0, false
	}
}

// inferTypeFromExpression infers a type handle from an expression's resolved type.
// This is used for `let` bindings without explicit type annotations.
func (l *Lowerer) inferTypeFromExpression(handle ir.ExpressionHandle) (ir.TypeHandle, error) {
	if int(handle) >= len(l.currentFunc.ExpressionTypes) {
		return 0, fmt.Errorf("expression %d has no resolved type", handle)
	}

	resolution := l.currentFunc.ExpressionTypes[handle]

	// If it's already a handle, return it
	if resolution.Handle != nil {
		return *resolution.Handle, nil
	}

	// If it's an inline type, register it in the registry
	if resolution.Value != nil {
		return l.registerType("", resolution.Value), nil
	}

	return 0, fmt.Errorf("expression %d has empty type resolution", handle)
}

// isTextureFunction checks if a function name is a texture sampling/loading function.
func (l *Lowerer) isTextureFunction(name string) bool {
	switch name {
	case "textureSample", "textureSampleBias", "textureSampleLevel", "textureSampleGrad",
		"textureSampleCompare", "textureSampleCompareLevel",
		"textureLoad", "textureStore",
		"textureDimensions", "textureNumLevels", "textureNumLayers", "textureNumSamples":
		return true
	}
	return false
}

// lowerTextureCall converts a texture function call to IR.
func (l *Lowerer) lowerTextureCall(name string, args []Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) < 2 {
		return 0, fmt.Errorf("%s requires at least 2 arguments", name)
	}

	switch name {
	case "textureSample":
		// textureSample(t, s, coord) or textureSample(t, s, coord, offset)
		return l.lowerTextureSample(args, target, ir.SampleLevelAuto{})

	case "textureSampleBias":
		// textureSampleBias(t, s, coord, bias)
		if len(args) < 4 {
			return 0, fmt.Errorf("textureSampleBias requires 4 arguments")
		}
		bias, err := l.lowerExpression(args[3], target)
		if err != nil {
			return 0, err
		}
		return l.lowerTextureSample(args[:3], target, ir.SampleLevelBias{Bias: bias})

	case "textureSampleLevel":
		// textureSampleLevel(t, s, coord, level)
		if len(args) < 4 {
			return 0, fmt.Errorf("textureSampleLevel requires 4 arguments")
		}
		level, err := l.lowerExpression(args[3], target)
		if err != nil {
			return 0, err
		}
		return l.lowerTextureSample(args[:3], target, ir.SampleLevelExact{Level: level})

	case "textureSampleGrad":
		// textureSampleGrad(t, s, coord, ddx, ddy)
		if len(args) < 5 {
			return 0, fmt.Errorf("textureSampleGrad requires 5 arguments")
		}
		ddx, err := l.lowerExpression(args[3], target)
		if err != nil {
			return 0, err
		}
		ddy, err := l.lowerExpression(args[4], target)
		if err != nil {
			return 0, err
		}
		return l.lowerTextureSample(args[:3], target, ir.SampleLevelGradient{X: ddx, Y: ddy})

	case "textureLoad":
		// textureLoad(t, coord, level) or textureLoad(t, coord) for storage textures
		return l.lowerTextureLoad(args, target)

	case "textureStore":
		// textureStore(t, coord, value)
		return l.lowerTextureStore(args, target)

	case "textureDimensions":
		// textureDimensions(t) or textureDimensions(t, level)
		return l.lowerTextureQuery(args, target, ir.ImageQuerySize{})

	case "textureNumLevels":
		// textureNumLevels(t)
		return l.lowerTextureQuery(args, target, ir.ImageQueryNumLevels{})

	case "textureNumLayers":
		// textureNumLayers(t)
		return l.lowerTextureQuery(args, target, ir.ImageQueryNumLayers{})

	case "textureNumSamples":
		// textureNumSamples(t)
		return l.lowerTextureQuery(args, target, ir.ImageQueryNumSamples{})

	default:
		return 0, fmt.Errorf("unknown texture function: %s", name)
	}
}

// lowerTextureSample converts a texture sampling call to IR.
func (l *Lowerer) lowerTextureSample(args []Expr, target *[]ir.Statement, level ir.SampleLevel) (ir.ExpressionHandle, error) {
	// args: texture, sampler, coordinate [, array_index_or_offset] [, offset]
	image, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	sampler, err := l.lowerExpression(args[1], target)
	if err != nil {
		return 0, err
	}

	coord, err := l.lowerExpression(args[2], target)
	if err != nil {
		return 0, err
	}

	// Check if texture is arrayed to determine how to interpret extra arguments
	var arrayIndex *ir.ExpressionHandle
	if len(args) > 3 && l.isTextureArrayed(args[0]) {
		ai, aiErr := l.lowerExpression(args[3], target)
		if aiErr != nil {
			return 0, aiErr
		}
		arrayIndex = &ai
	}

	return l.addExpression(ir.Expression{
		Kind: ir.ExprImageSample{
			Image:      image,
			Sampler:    sampler,
			Coordinate: coord,
			ArrayIndex: arrayIndex,
			Level:      level,
		},
	}), nil
}

// isTextureArrayed checks if a texture expression refers to an arrayed image type.
func (l *Lowerer) isTextureArrayed(expr Expr) bool {
	ident, ok := expr.(*Ident)
	if !ok {
		return false
	}
	for _, gv := range l.module.GlobalVariables {
		if gv.Name == ident.Name {
			if int(gv.Type) < len(l.module.Types) {
				if img, ok := l.module.Types[gv.Type].Inner.(ir.ImageType); ok {
					return img.Arrayed
				}
			}
			return false
		}
	}
	return false
}

// lowerTextureLoad converts a texture load call to IR.
func (l *Lowerer) lowerTextureLoad(args []Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	// args: texture, coordinate [, level]
	image, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	coord, err := l.lowerExpression(args[1], target)
	if err != nil {
		return 0, err
	}

	var level *ir.ExpressionHandle
	if len(args) > 2 {
		lv, err := l.lowerExpression(args[2], target)
		if err != nil {
			return 0, err
		}
		level = &lv
	}

	return l.addExpression(ir.Expression{
		Kind: ir.ExprImageLoad{
			Image:      image,
			Coordinate: coord,
			Level:      level,
		},
	}), nil
}

// lowerTextureStore converts a texture store call to IR.
func (l *Lowerer) lowerTextureStore(args []Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	// args: texture, coordinate, value
	if len(args) < 3 {
		return 0, fmt.Errorf("textureStore requires 3 arguments")
	}

	image, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	coord, err := l.lowerExpression(args[1], target)
	if err != nil {
		return 0, err
	}

	value, err := l.lowerExpression(args[2], target)
	if err != nil {
		return 0, err
	}

	// textureStore is a statement, not an expression
	// Add a store statement and return a zero value
	*target = append(*target, ir.Statement{
		Kind: ir.StmtImageStore{
			Image:      image,
			Coordinate: coord,
			Value:      value,
		},
	})

	// Return a zero value expression since textureStore doesn't return anything useful
	return l.addExpression(ir.Expression{
		Kind: ir.ExprZeroValue{Type: 0}, // void
	}), nil
}

// lowerTextureQuery converts a texture query call to IR.
func (l *Lowerer) lowerTextureQuery(args []Expr, target *[]ir.Statement, query ir.ImageQuery) (ir.ExpressionHandle, error) {
	image, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	// For textureDimensions with level argument
	if len(args) > 1 {
		if sizeQuery, ok := query.(ir.ImageQuerySize); ok {
			level, err := l.lowerExpression(args[1], target)
			if err != nil {
				return 0, err
			}
			sizeQuery.Level = &level
			query = sizeQuery
		}
	}

	return l.addExpression(ir.Expression{
		Kind: ir.ExprImageQuery{
			Image: image,
			Query: query,
		},
	}), nil
}

// getBarrierFlags returns barrier flags for a given function name, or 0 if not a barrier.
func (l *Lowerer) getBarrierFlags(name string) ir.BarrierFlags {
	switch name {
	case "workgroupBarrier":
		return ir.BarrierWorkGroup
	case "storageBarrier":
		return ir.BarrierStorage
	case "textureBarrier":
		return ir.BarrierTexture
	}
	return 0
}

// getAtomicFunction returns the atomic function for a given name, or nil if not an atomic.
func (l *Lowerer) getAtomicFunction(name string) ir.AtomicFunction {
	switch name {
	case "atomicAdd":
		return ir.AtomicAdd{}
	case "atomicSub":
		return ir.AtomicSubtract{}
	case "atomicAnd":
		return ir.AtomicAnd{}
	case "atomicOr":
		return ir.AtomicInclusiveOr{}
	case "atomicXor":
		return ir.AtomicExclusiveOr{}
	case "atomicMin":
		return ir.AtomicMin{}
	case "atomicMax":
		return ir.AtomicMax{}
	case "atomicExchange":
		return ir.AtomicExchange{}
	}
	return nil
}

// lowerAtomicCall converts an atomic function call to IR.
// Atomic functions have the form: atomicOp(&ptr, value) -> old_value
func (l *Lowerer) lowerAtomicCall(atomicFunc ir.AtomicFunction, args []Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) < 2 {
		return 0, fmt.Errorf("atomic function requires at least 2 arguments")
	}

	// First argument is a pointer (passed with &)
	pointer, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	// Second argument is the value
	value, err := l.lowerExpression(args[1], target)
	if err != nil {
		return 0, err
	}

	// Create atomic result expression
	resultHandle := l.addExpression(ir.Expression{
		Kind: ir.ExprAtomicResult{},
	})

	*target = append(*target, ir.Statement{
		Kind: ir.StmtAtomic{
			Pointer: pointer,
			Fun:     atomicFunc,
			Value:   value,
			Result:  &resultHandle,
		},
	})

	return resultHandle, nil
}

// lowerAtomicStore converts atomicStore(&ptr, value) to IR.
// atomicStore is a statement - it has no return value.
func (l *Lowerer) lowerAtomicStore(args []Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) < 2 {
		return 0, fmt.Errorf("atomicStore requires 2 arguments")
	}

	pointer, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	value, err := l.lowerExpression(args[1], target)
	if err != nil {
		return 0, err
	}

	*target = append(*target, ir.Statement{
		Kind: ir.StmtAtomic{
			Pointer: pointer,
			Fun:     ir.AtomicStore{},
			Value:   value,
			Result:  nil, // atomicStore has no result
		},
	})

	return 0, nil // No return value
}

// lowerAtomicLoad converts atomicLoad(&ptr) to IR.
// Returns the loaded atomic value.
func (l *Lowerer) lowerAtomicLoad(args []Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) < 1 {
		return 0, fmt.Errorf("atomicLoad requires 1 argument")
	}

	pointer, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	resultHandle := l.addExpression(ir.Expression{
		Kind: ir.ExprAtomicResult{},
	})

	*target = append(*target, ir.Statement{
		Kind: ir.StmtAtomic{
			Pointer: pointer,
			Fun:     ir.AtomicLoad{},
			Value:   pointer, // Not used by backend for AtomicLoad
			Result:  &resultHandle,
		},
	})

	return resultHandle, nil
}

// lowerAtomicCompareExchange converts atomicCompareExchangeWeak to IR.
// atomicCompareExchangeWeak(ptr, compare, value) -> __atomic_compare_exchange_result<T>
// Note: Returns the old value; the exchanged bool would need struct support.
func (l *Lowerer) lowerAtomicCompareExchange(args []Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) < 3 {
		return 0, fmt.Errorf("atomicCompareExchangeWeak requires 3 arguments")
	}

	// First argument is a pointer
	pointer, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	// Second argument is the compare value
	compare, err := l.lowerExpression(args[1], target)
	if err != nil {
		return 0, err
	}

	// Third argument is the new value
	value, err := l.lowerExpression(args[2], target)
	if err != nil {
		return 0, err
	}

	// Create atomic result expression
	resultHandle := l.addExpression(ir.Expression{
		Kind: ir.ExprAtomicResult{},
	})

	*target = append(*target, ir.Statement{
		Kind: ir.StmtAtomic{
			Pointer: pointer,
			Fun:     ir.AtomicExchange{Compare: &compare},
			Value:   value,
			Result:  &resultHandle,
		},
	})

	return resultHandle, nil
}
