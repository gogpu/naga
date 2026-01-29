package wgsl

import (
	"fmt"
	"strconv"

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
	globals   map[string]ir.GlobalVariableHandle
	locals    map[string]ir.ExpressionHandle
	globalIdx uint32

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
		module:     &ir.Module{},
		source:     source,
		registry:   ir.NewTypeRegistry(),
		types:      make(map[string]ir.TypeHandle),
		globals:    make(map[string]ir.GlobalVariableHandle),
		locals:     make(map[string]ir.ExpressionHandle),
		functions:  make(map[string]ir.FunctionHandle),
		localDecls: make(map[string]Span),
		usedLocals: make(map[string]bool),
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
		l.functions[f.Name] = ir.FunctionHandle(i) //nolint:gosec // G115: i is bounded by function count
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

// registerBuiltinTypes registers WGSL built-in scalar and sampler types.
func (l *Lowerer) registerBuiltinTypes() {
	// Scalars
	l.registerType("f32", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	l.registerType("f16", ir.ScalarType{Kind: ir.ScalarFloat, Width: 2})
	l.registerType("i32", ir.ScalarType{Kind: ir.ScalarSint, Width: 4})
	l.registerType("u32", ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	l.registerType("bool", ir.ScalarType{Kind: ir.ScalarBool, Width: 1})

	// Samplers (opaque types for texture sampling)
	l.registerType("sampler", ir.SamplerType{Comparison: false})
	l.registerType("sampler_comparison", ir.SamplerType{Comparison: true})
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

		members[i] = ir.StructMember{
			Name:    m.Name,
			Type:    typeHandle,
			Binding: binding,
			Offset:  offset,
		}
		// Simplified: assume each member is 16-byte aligned (actual layout is more complex)
		offset += 16
	}
	l.registerType(s.Name, ir.StructType{Members: members, Span: offset})
	return nil
}

// lowerGlobalVar converts a global variable declaration to IR.
func (l *Lowerer) lowerGlobalVar(v *VarDecl) error {
	typeHandle, err := l.resolveType(v.Type)
	if err != nil {
		return fmt.Errorf("global var %s: %w", v.Name, err)
	}

	space := l.addressSpace(v.AddressSpace)
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
	typeHandle, err := l.resolveType(c.Type)
	if err != nil {
		return fmt.Errorf("constant %s: %w", c.Name, err)
	}

	// TODO: Evaluate constant expression
	_ = typeHandle
	return nil
}

// lowerFunction converts a function declaration to IR.
func (l *Lowerer) lowerFunction(f *FunctionDecl) error {
	// Reset local context
	l.locals = make(map[string]ir.ExpressionHandle)
	l.localDecls = make(map[string]Span)
	l.usedLocals = make(map[string]bool)
	l.currentExprIdx = 0

	fn := &ir.Function{
		Name:            f.Name,
		Arguments:       make([]ir.FunctionArgument, len(f.Params)),
		LocalVars:       []ir.LocalVariable{},
		Expressions:     []ir.Expression{},
		ExpressionTypes: []ir.TypeResolution{},
		Body:            []ir.Statement{},
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
			Kind: ir.ExprFunctionArgument{Index: uint32(i)}, //nolint:gosec // i is bounded by function params length
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

	localIdx := uint32(len(l.currentFunc.LocalVars)) //nolint:gosec // local vars length is bounded
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
		v, _ := strconv.ParseInt(lit.Value, 0, 32)
		value = ir.LiteralI32(v)
	case TokenFloatLiteral:
		v, _ := strconv.ParseFloat(lit.Value, 32)
		value = ir.LiteralF32(v)
	case TokenTrue:
		value = ir.LiteralBool(true)
	case TokenFalse:
		value = ir.LiteralBool(false)
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

	// Check if this is a math function
	if mathFunc, ok := l.getMathFunction(funcName); ok {
		return l.lowerMathCall(mathFunc, call.Args, target)
	}

	// Check if this is a texture function
	if l.isTextureFunction(funcName) {
		return l.lowerTextureCall(funcName, call.Args, target)
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
		return l.registerType("", ir.ArrayType{Base: base, Size: size}), nil
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

	// Texture types: texture_2d<f32>
	if len(t.Name) >= 7 && t.Name[:7] == "texture" {
		dim := l.textureDim(t.Name)
		return l.registerType("", ir.ImageType{
			Dim:   dim,
			Class: ir.ImageClassSampled,
		}), nil
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
		constSize := uint32(len(args)) //nolint:gosec // args length is bounded
		arraySize := ir.ArraySize{
			Constant: &constSize,
		}
		typeHandle = l.registerType("", ir.ArrayType{
			Base: elemType,
			Size: arraySize,
		})
	}

	return l.addExpression(ir.Expression{
		Kind: ir.ExprCompose{Type: typeHandle, Components: components},
	}), nil
}

func (l *Lowerer) getMathFunction(name string) (ir.MathFunction, bool) {
	mathFuncs := map[string]ir.MathFunction{
		"abs":       ir.MathAbs,
		"min":       ir.MathMin,
		"max":       ir.MathMax,
		"clamp":     ir.MathClamp,
		"sin":       ir.MathSin,
		"cos":       ir.MathCos,
		"tan":       ir.MathTan,
		"sqrt":      ir.MathSqrt,
		"length":    ir.MathLength,
		"normalize": ir.MathNormalize,
		"dot":       ir.MathDot,
		"cross":     ir.MathCross,
	}
	fn, ok := mathFuncs[name]
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

func (l *Lowerer) builtin(name string) ir.BuiltinValue {
	builtins := map[string]ir.BuiltinValue{
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
	if b, ok := builtins[name]; ok {
		return b
	}
	return ir.BuiltinPosition // Default
}

func (l *Lowerer) addressSpace(space string) ir.AddressSpace {
	spaces := map[string]ir.AddressSpace{
		"function":      ir.SpaceFunction,
		"private":       ir.SpacePrivate,
		"workgroup":     ir.SpaceWorkGroup,
		"uniform":       ir.SpaceUniform,
		"storage":       ir.SpaceStorage,
		"push_constant": ir.SpacePushConstant,
		"handle":        ir.SpaceHandle,
	}
	if s, ok := spaces[space]; ok {
		return s
	}
	return ir.SpaceFunction // Default
}

func (l *Lowerer) textureDim(name string) ir.ImageDimension {
	if len(name) >= 9 && name == "texture_1d" {
		return ir.Dim1D
	}
	if len(name) >= 9 && name == "texture_2d" {
		return ir.Dim2D
	}
	if len(name) >= 9 && name == "texture_3d" {
		return ir.Dim3D
	}
	if len(name) >= 12 && name == "texture_cube" {
		return ir.DimCube
	}
	return ir.Dim2D // Default
}

func (l *Lowerer) tokenToBinaryOp(tok TokenKind) ir.BinaryOperator {
	ops := map[TokenKind]ir.BinaryOperator{
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
	if op, ok := ops[tok]; ok {
		return op
	}
	return ir.BinaryAdd // Default
}

func (l *Lowerer) tokenToUnaryOp(tok TokenKind) ir.UnaryOperator {
	ops := map[TokenKind]ir.UnaryOperator{
		TokenMinus: ir.UnaryNegate,
		TokenBang:  ir.UnaryLogicalNot,
		TokenTilde: ir.UnaryBitwiseNot,
	}
	if op, ok := ops[tok]; ok {
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

func (l *Lowerer) assignOpToBinary(tok TokenKind) ir.BinaryOperator {
	ops := map[TokenKind]ir.BinaryOperator{
		TokenPlusEqual:  ir.BinaryAdd,
		TokenMinusEqual: ir.BinarySubtract,
		TokenStarEqual:  ir.BinaryMultiply,
		TokenSlashEqual: ir.BinaryDivide,
	}
	if op, ok := ops[tok]; ok {
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
	// args: texture, sampler, coordinate
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

	return l.addExpression(ir.Expression{
		Kind: ir.ExprImageSample{
			Image:      image,
			Sampler:    sampler,
			Coordinate: coord,
			Level:      level,
		},
	}), nil
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
