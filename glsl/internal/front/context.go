package front

// This file is Rust naga's front/glsl/context.rs: the lowering context of one
// function body, or of one module-scope constant expression, with its
// expression and local-variable arenas, its symbol table, the implicit
// conversions and the Emit machinery. Expression lowering itself is in
// lower_expr.go.

import (
	"errors"

	"github.com/gogpu/naga/internal/consteval"
	"github.com/gogpu/naga/internal/irutil"
	"github.com/gogpu/naga/internal/typifier"
	"github.com/gogpu/naga/ir"
)

// exprPosKind is the kind of an exprPos.
type exprPosKind uint8

const (
	// posLHS lowers an expression to a pointer to assign through.
	posLHS exprPosKind = iota
	// posRHS lowers an expression to a value.
	posRHS
	// posAccessBase lowers an expression to the base of an access, which may
	// be a pointer (for a dynamically indexed access) or a value (for a
	// constant index).
	posAccessBase
)

// exprPos is the position an expression is lowered in, Rust naga's ExprPos
// (context.rs:19-45). It drives pointer-versus-value lowering.
type exprPos struct {
	kind          exprPosKind
	constantIndex bool
}

var (
	// lhsPos lowers an expression to a pointer, Rust's ExprPos::Lhs.
	lhsPos = exprPos{kind: posLHS}
	// rhsPos lowers an expression to a value, Rust's ExprPos::Rhs.
	rhsPos = exprPos{kind: posRHS}
)

// accessBasePos lowers an expression to the base of an access whose index is
// a constant (or not), Rust's ExprPos::AccessBase.
func accessBasePos(constantIndex bool) exprPos {
	return exprPos{kind: posAccessBase, constantIndex: constantIndex}
}

// maybeAccessBase returns the position a base of an access is lowered in: an
// lhs or a dynamic access base keeps its position, every other one becomes an
// access base, as Rust's ExprPos::maybe_access_base (context.rs:34-44).
func (p exprPos) maybeAccessBase(constantIndex bool) exprPos {
	if p.kind == posLHS || (p.kind == posAccessBase && !p.constantIndex) {
		return p
	}
	return accessBasePos(constantIndex)
}

// emitter records where the pending Emit range starts, Rust naga's
// proc::Emitter (proc/emitter.rs). Expressions appended while it runs form one
// Statement::Emit when it is closed, unless none were appended.
type emitter struct {
	running bool
	from    ir.ExpressionHandle
}

// start opens a range at expression index n.
func (e *emitter) start(n ir.ExpressionHandle) {
	e.running = true
	e.from = n
}

// finish closes the range at expression index n and reports it, or reports
// false when the range is empty.
func (e *emitter) finish(n ir.ExpressionHandle) (ir.Range, bool) {
	if !e.running {
		return ir.Range{}, false
	}
	e.running = false
	if e.from == n {
		return ir.Range{}, false
	}
	return ir.Range{Start: e.from, End: n}, true
}

// variableRef is how a declared name resolves during lowering, Rust naga's
// VariableReference (ast.rs:96-107).
type variableRef struct {
	// expr is the expression the name stands for: a pointer in the function
	// or global arena, or a value for a constant or an opaque handle.
	expr ir.ExpressionHandle
	// load reports whether the name must be loaded through expr to produce
	// its value.
	load bool
	// mutable reports whether an assignment may target the name.
	mutable bool
	// constant names the module constant a const name was declared as, with
	// the type it was declared with; dynamic indexing of such a name goes
	// through a proxy local (context.rs:1037-1061).
	constant *ir.ConstantHandle
	constTy  ir.TypeHandle
	// entryArg is the index of the name's entry-point argument, or -1.
	entryArg int
}

// Context is the lowering state of one function body or one module-scope
// constant expression, Rust naga's front/glsl/context.rs Context. It owns the
// arenas it fills; a finished function takes them over (intoFunction). A const
// context appends to the module's global expressions instead and has no body.
type Context struct {
	fe  *Frontend
	mod *ir.Module

	isConst bool

	expressions    []ir.Expression
	locals         []ir.LocalVariable
	arguments      []ir.FunctionArgument
	parameters     []ir.TypeHandle
	parametersInfo []parameterInfo

	body ir.Block

	typifier      typifier.Typifier
	constTypifier typifier.Typifier

	emit emitter

	eval *consteval.Evaluator

	// vars resolves the names the parser declared to their IR references:
	// the lowering side of the parser's symbol table. Keying on the symbol
	// rather than its name keeps a shadowing declaration apart from the one
	// it shadows.
	vars map[*glslSymbol]*variableRef

	// samplers maps a combined sampler's image expression to its sampler
	// expression, which the texture builtins read (builtins.rs:2071-2097).
	samplers map[ir.ExpressionHandle]ir.ExpressionHandle
}

// newContext returns a context over fe's module. isConst makes it lower
// module-scope constant expressions into the module's global expression
// arena. The module-scope names of the frontend are made visible, as Rust's
// Context::new does (context.rs:119-126).
func newContext(fe *Frontend, isConst bool) (*Context, *frontError) {
	c := &Context{
		fe:       fe,
		mod:      fe.mod,
		isConst:  isConst,
		vars:     map[*glslSymbol]*variableRef{},
		samplers: map[ir.ExpressionHandle]ir.ExpressionHandle{},
	}
	behavior := consteval.BehaviorRuntime
	if isConst {
		behavior = consteval.BehaviorConst
	}
	c.eval = &consteval.Evaluator{Module: c.mod, Arena: c, Types: c, Behavior: behavior}
	c.emitStart()
	// A function context makes the module-scope names visible here. A const
	// context does not: it resolves a constant or override name on demand
	// (Context.variable), so its module arena holds only the expressions the
	// constant really needs. Rust appends every reference to a throwaway
	// function arena instead (context.rs:96-126), which is also why its
	// constant index reads the wrong arena (design §8).
	if !isConst {
		for _, g := range fe.globalVars {
			if err := c.addGlobal(g); err != nil {
				return nil, err
			}
		}
	}
	return c, nil
}

// materializeConstRef builds the reference of a module-scope constant or
// override name on first use in a const context. The constant's own value is
// seen through by the evaluator (check_and_get returns the initializer's
// handle), so only the expression the constant really needs enters the module
// arena.
func (c *Context) materializeConstRef(sym *glslSymbol, span ir.Span) (*variableRef, *frontError) {
	g, ok := c.fe.globalLookup(sym)
	if !ok {
		return nil, nil
	}
	switch g.kind {
	case lookupConstant:
		expr, err := c.addExpression(ir.ExprConstant{Constant: g.constant}, span)
		if err != nil {
			return nil, err
		}
		constant := g.constant
		ref := &variableRef{expr: expr, constant: &constant, constTy: g.ty, entryArg: -1}
		c.vars[sym] = ref
		return ref, nil
	case lookupOverride:
		expr, err := c.addExpression(ir.ExprOverride{Override: g.override}, span)
		if err != nil {
			return nil, err
		}
		ref := &variableRef{expr: expr, entryArg: -1}
		c.vars[sym] = ref
		return ref, nil
	default:
		return nil, nil
	}
}

// intoFunction takes the context's arenas as a finished IR function.
func (c *Context) intoFunction(name string) ir.Function {
	return ir.Function{
		Name:        name,
		Arguments:   c.arguments,
		LocalVars:   c.locals,
		Expressions: c.expressions,
		Body:        c.body,
	}
}

// addGlobal makes a module-scope name visible in the context, Rust naga's
// Context::add_global (context.rs:160-235).
func (c *Context) addGlobal(g namedLookup) *frontError {
	var (
		expr    ir.ExpressionHandle
		load    bool
		constH  *ir.ConstantHandle
		constTy ir.TypeHandle
		err     *frontError
	)
	switch g.lookup.kind {
	case lookupVariable:
		expr, err = c.addGlobalExpression(ir.ExprGlobalVariable{Variable: g.lookup.variable}, ir.Span{})
		// A handle-space global (a texture or a sampler) is its own value;
		// every other one is a pointer to it.
		load = c.mod.GlobalVariables[g.lookup.variable].Space != ir.SpaceHandle
	case lookupBlockSelect:
		var base ir.ExpressionHandle
		base, err = c.addGlobalExpression(ir.ExprGlobalVariable{Variable: g.lookup.variable}, ir.Span{})
		if err == nil {
			expr, err = c.addGlobalExpression(ir.ExprAccessIndex{Base: base, Index: g.lookup.index}, ir.Span{})
		}
		load = c.blockSelectMemberLoads(g.lookup.variable, g.lookup.index)
	case lookupConstant:
		expr, err = c.addGlobalExpression(ir.ExprConstant{Constant: g.lookup.constant}, ir.Span{})
		h := g.lookup.constant
		constH = &h
		constTy = g.lookup.ty
	case lookupOverride:
		expr, err = c.addGlobalExpression(ir.ExprOverride{Override: g.lookup.override}, ir.Span{})
	}
	if err != nil {
		return err
	}
	c.vars[g.sym] = &variableRef{
		expr:     expr,
		load:     load,
		mutable:  g.lookup.mutable,
		constant: constH,
		constTy:  constTy,
		entryArg: g.lookup.entryArg,
	}

	// A combined sampler is one name and two globals: the context records
	// the pair in its own arena so a texture builtin finds the sampler
	// expression next to the image the name lowered to (design §11).
	if !c.isConst {
		if samplerHandle, ok := c.fe.combinedPairs[g.lookup.variable]; ok {
			samplerExpr, serr := c.addExpression(ir.ExprGlobalVariable{Variable: samplerHandle}, ir.Span{})
			if serr != nil {
				return serr
			}
			c.samplers[expr] = samplerExpr
		}
	}
	return nil
}

// addGlobalExpression appends the expression a module-scope name stands for.
// A function arena goes through the evaluator, as Rust's add_global does; a
// const context appends into the module's global arena directly, because the
// evaluator's const behavior would refuse the run-time reference a global
// variable is.
func (c *Context) addGlobalExpression(kind ir.ExpressionKind, span ir.Span) (ir.ExpressionHandle, *frontError) {
	if c.isConst {
		return c.appendExpr(kind), nil
	}
	return c.addExpression(kind, span)
}

// blockSelectMemberLoads reports whether a block member is read through a
// load: every member except a dynamically sized array, which is already the
// value the name denotes (context.rs:181-204).
func (c *Context) blockSelectMemberLoads(handle ir.GlobalVariableHandle, index uint32) bool {
	block, ok := c.mod.Types[c.mod.GlobalVariables[handle].Type].Inner.(ir.StructType)
	if !ok || int(index) >= len(block.Members) {
		return true
	}
	arr, ok := c.mod.Types[block.Members[index].Type].Inner.(ir.ArrayType)
	return !ok || arr.Size.Constant != nil || arr.Size.Pending != nil
}

// addLocalVar declares a local name in the current scope and returns the
// reference of the name it replaced, if any, Rust naga's Context::add_local_var
// (context.rs:298-313).
func (c *Context) addLocalVar(sym *glslSymbol, expr ir.ExpressionHandle, mutable bool) *variableRef {
	prev := c.vars[sym]
	c.vars[sym] = &variableRef{expr: expr, load: true, mutable: mutable, constTy: 0, entryArg: -1}
	return prev
}

// addFunctionArg adds one declared parameter to the function being built,
// Rust naga's Context::add_function_arg (context.rs:316-403). An out or inout
// parameter becomes a pointer argument; an in parameter the body may write is
// copied into a local at entry, so a write does not reach the caller. sym is
// the parameter's symbol, nil for an unnamed parameter.
func (c *Context) addFunctionArg(sym *glslSymbol, ty ir.TypeHandle, qualifier parameterQualifier, span ir.Span) *frontError {
	name := ""
	if sym != nil {
		name = sym.Name
	}
	index := len(c.arguments)
	arg := ir.FunctionArgument{Name: name, NamePresent: false, Type: ty}
	c.parameters = append(c.parameters, ty)

	opaque := false
	switch c.mod.Types[ty].Inner.(type) {
	case ir.ImageType, ir.SamplerType:
		opaque = true
	}

	if qualifier.isLHS() {
		arg.Type = c.fe.registerType(ir.PointerType{Base: arg.Type, Space: ir.SpaceFunction})
	}
	c.arguments = append(c.arguments, arg)
	c.parametersInfo = append(c.parametersInfo, parameterInfo{qualifier: qualifier})

	if sym == nil {
		return nil
	}
	expr, err := c.addExpression(ir.ExprFunctionArgument{Index: uint32(irutil.ArenaIndex(index))}, span)
	if err != nil {
		return err
	}
	mutable := qualifier != paramConst && !opaque
	load := qualifier.isLHS()

	ref := &variableRef{expr: expr, load: load, mutable: mutable, entryArg: -1}
	if mutable && !load {
		local := ir.LocalVariable{Name: name, Type: ty}
		h := ir.LocalVariableHandle(irutil.ArenaIndex(len(c.locals)))
		c.locals = append(c.locals, local)
		localExpr, err := c.addExpression(ir.ExprLocalVariable{Variable: h}, span)
		if err != nil {
			return err
		}
		c.emitRestart()
		c.body = append(c.body, ir.Statement{Kind: ir.StmtStore{Pointer: localExpr, Value: expr}})
		ref = &variableRef{expr: localExpr, load: true, mutable: mutable, entryArg: -1}
	}
	c.vars[sym] = ref
	return nil
}

// expressionsLen is the number of expressions of the arena the context
// appends to.
func (c *Context) expressionsLen() ir.ExpressionHandle {
	if c.isConst {
		return ir.ExpressionHandle(irutil.ArenaIndex(len(c.mod.GlobalExpressions)))
	}
	return ir.ExpressionHandle(irutil.ArenaIndex(len(c.expressions)))
}

// Expression implements consteval.Arena.
func (c *Context) Expression(h ir.ExpressionHandle) ir.ExpressionKind {
	if c.isConst {
		return c.mod.GlobalExpressions[h].Kind
	}
	return c.expressions[h].Kind
}

// Append implements consteval.Arena. In a function arena an expression that
// needs no Emit (a literal, a constant, a variable reference) closes the
// pending range before it and opens a new one after it, so no Emit statement
// covers it, as Rust's append_expr does (constant_evaluator.rs:3640-3669).
func (c *Context) Append(kind ir.ExpressionKind) ir.ExpressionHandle {
	expr := ir.Expression{Kind: kind}
	if c.isConst {
		h := ir.ExpressionHandle(irutil.ArenaIndex(len(c.mod.GlobalExpressions)))
		c.mod.GlobalExpressions = append(c.mod.GlobalExpressions, expr)
		return h
	}
	if c.emit.running && expr.NeedsPreEmit() {
		c.emitEnd()
		h := c.appendRaw(expr)
		c.emitStart()
		return h
	}
	return c.appendRaw(expr)
}

// appendRaw appends expr to the function arena without the Emit split.
func (c *Context) appendRaw(expr ir.Expression) ir.ExpressionHandle {
	h := ir.ExpressionHandle(irutil.ArenaIndex(len(c.expressions)))
	c.expressions = append(c.expressions, expr)
	return h
}

// appendExpr appends an expression to the arena without the constant
// evaluator, as Rust naga's entry-point wrapper does with a direct arena
// append (functions.rs:1241-1243, 1318-1320). The evaluator would close the
// pending Emit range here, which the wrapper's expressions do not belong to.
func (c *Context) appendExpr(kind ir.ExpressionKind) ir.ExpressionHandle {
	if c.isConst {
		h := ir.ExpressionHandle(irutil.ArenaIndex(len(c.mod.GlobalExpressions)))
		c.mod.GlobalExpressions = append(c.mod.GlobalExpressions, ir.Expression{Kind: kind})
		return h
	}
	return c.appendRaw(ir.Expression{Kind: kind})
}

// IsGlobal implements consteval.Arena and consteval.Types.
func (c *Context) IsGlobal() bool { return c.isConst }

// Type implements consteval.Types: it registers an anonymous result type.
func (c *Context) Type(inner ir.TypeInner) ir.TypeHandle { return c.fe.registerType(inner) }

// Array implements consteval.Types: it registers the array of base with the
// given size, striding elements by base's size rounded up to its alignment
// (Rust's Layouter::array_stride).
func (c *Context) Array(base ir.TypeHandle, size ir.ArraySize) ir.TypeHandle {
	if err := c.fe.layouter.Update(c.mod); err != nil {
		// The evaluator only arrays types that exist, so a cycle cannot come
		// from here; a zero stride keeps the lowering going and validation
		// reports the type.
		return c.fe.registerType(ir.ArrayType{Base: base, Size: size})
	}
	layout := c.fe.layouter.Layout(base)
	stride := layout.Size
	if layout.Alignment > 1 {
		stride = (layout.Size + layout.Alignment - 1) / layout.Alignment * layout.Alignment
	}
	return c.fe.registerType(ir.ArrayType{Base: base, Size: size, Stride: stride})
}

// emitStart opens a new Emit range at the end of the expression arena.
func (c *Context) emitStart() { c.emit.start(c.expressionsLen()) }

// emitEnd closes the pending Emit range into the current body, if it is not
// empty.
func (c *Context) emitEnd() {
	if r, ok := c.emit.finish(c.expressionsLen()); ok {
		c.body = append(c.body, ir.Statement{Kind: ir.StmtEmit{Range: r}})
	}
}

// emitRestart closes the pending Emit range and opens a new one, Rust naga's
// Context::emit_restart (context.rs:265-268).
func (c *Context) emitRestart() {
	c.emitEnd()
	c.emitStart()
}

// swapBody runs f with body as the current block, restarting the emitter on
// entry and exit, as Rust's Context::with_body does (context.rs:148-158).
func (c *Context) swapBody(body *ir.Block, f func() *frontError) *frontError {
	c.emitRestart()
	saved := c.body
	c.body = *body
	err := f()
	c.emitRestart()
	*body = c.body
	c.body = saved
	return err
}

// addExpression appends expr to the arena, evaluated when its operands are
// constant, Rust naga's Context::add_expression (context.rs:270-292).
func (c *Context) addExpression(kind ir.ExpressionKind, span ir.Span) (ir.ExpressionHandle, *frontError) {
	h, err := c.eval.TryEvalAndAppend(kind)
	if err != nil {
		return 0, errMessage(span, constantErrorText(err))
	}
	return h, nil
}

// constantErrorText is the message Rust naga shows for a constant evaluator
// error: the error's own text, which is ErrorKind::SemanticError's text
// (error.rs:138-142). The two structural errors that a GLSL expression meets
// carry Rust's spelling.
func constantErrorText(err error) string {
	switch {
	case errors.Is(err, consteval.ErrNotConst):
		return "Unexpected runtime-expression"
	case errors.Is(err, consteval.ErrOverrideInConst):
		return "Unexpected override-expression"
	default:
		return err.Error()
	}
}

// typifierGrow resolves the types of the arena's expressions, Rust naga's
// Context::typifier_grow (types.rs:233-254), which types the whole arena so
// that expr is always covered.
func (c *Context) typifierGrow(span ir.Span) *frontError {
	var err error
	if c.isConst {
		err = c.constTypifier.GrowArena(c.mod, c.mod.GlobalExpressions, nil, nil, c.expressionsLen())
	} else {
		err = c.typifier.GrowArena(c.mod, c.expressions, c.locals, c.arguments, c.expressionsLen())
	}
	if err != nil {
		return errSemantic(span, "Can't resolve type: %v", err)
	}
	return nil
}

// parameterTypes returns the inners of the declared parameters, which the
// builtin variation check reads.
func (c *Context) parameterTypes() []ir.TypeInner {
	types := make([]ir.TypeInner, len(c.parameters))
	for i, parameter := range c.parameters {
		types[i] = c.mod.Types[parameter].Inner
	}
	return types
}

// invalidateTypifier drops the cached expression types, Rust naga's
// Typifier::invalidate for a change that rewrites an expression's operands in
// place (a sampled image turned into a depth image). Re-typing the arena is a
// rare cost and keeps the cache honest.
func (c *Context) invalidateTypifier() {
	if c.isConst {
		c.constTypifier.Reset()
		return
	}
	c.typifier.Reset()
}

// getType returns the type of expr, which the typifier has resolved.
func (c *Context) getType(expr ir.ExpressionHandle) ir.TypeInner {
	t := &c.typifier
	if c.isConst {
		t = &c.constTypifier
	}
	return t.Get(expr).Inner(c.mod.Types)
}

// resolveType returns the type of expr, growing the typifier to it.
func (c *Context) resolveType(expr ir.ExpressionHandle, span ir.Span) (ir.TypeInner, *frontError) {
	if err := c.typifierGrow(span); err != nil {
		return nil, err
	}
	return c.getType(expr), nil
}

// resolveTypeHandle returns the handle of expr's type in the module's type
// arena, registering an anonymous type for a value resolution, Rust naga's
// Context::resolve_type_handle (types.rs:297-311).
func (c *Context) resolveTypeHandle(expr ir.ExpressionHandle, span ir.Span) (ir.TypeHandle, *frontError) {
	if err := c.typifierGrow(span); err != nil {
		return 0, err
	}
	t := &c.typifier
	if c.isConst {
		t = &c.constTypifier
	}
	res := t.Get(expr)
	if res.Handle != nil {
		return *res.Handle, nil
	}
	return c.fe.registerType(res.Value), nil
}

// exprScalarComponents returns the scalar type of expr, if it has one.
func (c *Context) exprScalarComponents(expr ir.ExpressionHandle, span ir.Span) (ir.ScalarType, bool, *frontError) {
	ty, err := c.resolveType(expr, span)
	if err != nil {
		return ir.ScalarType{}, false, err
	}
	s, ok := scalarComponents(ty)
	return s, ok, nil
}

// exprPower returns the implicit-conversion rank of expr's scalar type.
func (c *Context) exprPower(expr ir.ExpressionHandle, span ir.Span) (uint32, bool, *frontError) {
	s, ok, err := c.exprScalarComponents(expr, span)
	if err != nil || !ok {
		return 0, false, err
	}
	p, ok := typePower(s)
	return p, ok, nil
}

// conversion converts expr to scalar, whatever its current type, as Rust
// naga's Context::conversion (context.rs:1411-1427).
func (c *Context) conversion(expr ir.ExpressionHandle, span ir.Span, scalar ir.ScalarType) (ir.ExpressionHandle, *frontError) {
	width := scalar.Width
	return c.addExpression(ir.ExprAs{Expr: expr, Kind: scalar.Kind, Convert: &width}, span)
}

// implicitConversion converts expr to scalar when the target's conversion
// rank is higher, Rust naga's Context::implicit_conversion
// (context.rs:1429-1444).
func (c *Context) implicitConversion(expr ir.ExpressionHandle, span ir.Span, scalar ir.ScalarType) (ir.ExpressionHandle, *frontError) {
	tgtPower, ok := typePower(scalar)
	if !ok {
		return expr, nil
	}
	exprPower, ok, err := c.exprPower(expr, span)
	if err != nil {
		return expr, err
	}
	if !ok || tgtPower <= exprPower {
		return expr, nil
	}
	return c.conversion(expr, span, scalar)
}

// forcedConversion converts expr to scalar unless it already has that type,
// Rust naga's Context::forced_conversion (context.rs:1446-1459).
func (c *Context) forcedConversion(expr ir.ExpressionHandle, span ir.Span, scalar ir.ScalarType) (ir.ExpressionHandle, *frontError) {
	cur, ok, err := c.exprScalarComponents(expr, span)
	if err != nil {
		return expr, err
	}
	if !ok || cur == scalar {
		return expr, nil
	}
	return c.conversion(expr, span, scalar)
}

// binaryImplicitConversion converts the weaker side of a binary operation to
// the other side's scalar type, Rust naga's Context::binary_implicit_conversion
// (context.rs:1461-1487).
func (c *Context) binaryImplicitConversion(left ir.ExpressionHandle, leftSpan ir.Span, right ir.ExpressionHandle, rightSpan ir.Span) (ir.ExpressionHandle, ir.ExpressionHandle, *frontError) {
	leftScalar, leftOK, err := c.exprScalarComponents(left, leftSpan)
	if err != nil {
		return left, right, err
	}
	rightScalar, rightOK, err := c.exprScalarComponents(right, rightSpan)
	if err != nil {
		return left, right, err
	}
	leftPower, leftRanked := typePower(leftScalar)
	rightPower, rightRanked := typePower(rightScalar)
	if !leftOK || !rightOK || !leftRanked || !rightRanked {
		return left, right, nil
	}
	switch {
	case leftPower < rightPower:
		left, err = c.conversion(left, leftSpan, rightScalar)
	case leftPower > rightPower:
		right, err = c.conversion(right, rightSpan, leftScalar)
	}
	return left, right, err
}

// implicitSplat splats a scalar expr into a vector of the given size, when a
// size is given, Rust naga's Context::implicit_splat (context.rs:1489-1502).
func (c *Context) implicitSplat(expr ir.ExpressionHandle, span ir.Span, size *ir.VectorSize) (ir.ExpressionHandle, *frontError) {
	if size == nil {
		return expr, nil
	}
	ty, err := c.resolveType(expr, span)
	if err != nil {
		return expr, err
	}
	if _, ok := ty.(ir.ScalarType); !ok {
		return expr, nil
	}
	return c.addExpression(ir.ExprSplat{Size: *size, Value: expr}, span)
}

// vectorResize swizzles vector down to size, Rust naga's Context::vector_resize
// (context.rs:1504-1518).
func (c *Context) vectorResize(size ir.VectorSize, vector ir.ExpressionHandle, span ir.Span) (ir.ExpressionHandle, *frontError) {
	return c.addExpression(ir.ExprSwizzle{
		Size:    size,
		Vector:  vector,
		Pattern: [4]ir.SwizzleComponent{ir.SwizzleX, ir.SwizzleY, ir.SwizzleZ, ir.SwizzleW},
	}, span)
}

// lowerStore stores value through pointer, splitting a swizzled pointer into
// one store per component, Rust naga's Context::lower_store
// (context.rs:482-540).
func (c *Context) lowerStore(pointer, value ir.ExpressionHandle, span ir.Span) *frontError {
	if sw, ok := c.Expression(pointer).(ir.ExprSwizzle); ok {
		vector := sw.Vector
		if load, ok := c.Expression(vector).(ir.ExprLoad); ok {
			vector = load.Pointer
		}
		for i := 0; i < vectorSizeCount(sw.Size); i++ {
			dst, err := c.addExpression(ir.ExprAccessIndex{Base: vector, Index: uint32(sw.Pattern[i])}, span)
			if err != nil {
				return err
			}
			src, err := c.addExpression(ir.ExprAccessIndex{Base: value, Index: uint32(i)}, span)
			if err != nil {
				return err
			}
			c.emitRestart()
			c.body = append(c.body, ir.Statement{Kind: ir.StmtStore{Pointer: dst, Value: src}})
		}
		return nil
	}
	c.emitRestart()
	c.body = append(c.body, ir.Statement{Kind: ir.StmtStore{Pointer: pointer, Value: value}})
	return nil
}

// variable resolves the name sym denotes, or reports Rust naga's unknown
// variable error. A builtin variable name resolves on first use, as Rust's
// lookup_variable does (variables.rs:101-224).
func (c *Context) variable(sym *glslSymbol, span ir.Span) (*variableRef, *frontError) {
	if ref := c.vars[sym]; ref != nil {
		return ref, nil
	}
	if c.isConst {
		// A const context carries no global references: a constant or
		// override resolves on its first use (a variable cannot appear in a
		// constant expression at all).
		if ref, err := c.materializeConstRef(sym, span); err != nil || ref != nil {
			return ref, err
		}
	}
	if sym.Kind == symBuiltinVar {
		return c.fe.addBuiltin(c, sym, span)
	}
	return nil, errUnknownVariable(sym.Name, span)
}

// liftUpConstExpression copies a const-expression of the function arena to
// the module's global expression arena, Rust naga's
// Context::lift_up_const_expression (types.rs:335-371). It is how a
// module-scope initializer becomes a global's init: only a literal, a
// constant, a zero value or a compose or splat of those can be lifted, and
// anything else is an error the caller may ignore.
func (c *Context) liftUpConstExpression(expr ir.ExpressionHandle, span ir.Span) (ir.ExpressionHandle, *frontError) {
	appendGlobal := func(kind ir.ExpressionKind) ir.ExpressionHandle {
		h := ir.ExpressionHandle(irutil.ArenaIndex(len(c.mod.GlobalExpressions)))
		c.mod.GlobalExpressions = append(c.mod.GlobalExpressions, ir.Expression{Kind: kind})
		return h
	}
	switch kind := c.Expression(expr).(type) {
	case ir.Literal:
		return appendGlobal(kind), nil
	case ir.ExprConstant:
		return appendGlobal(kind), nil
	case ir.ExprZeroValue:
		return appendGlobal(kind), nil
	case ir.ExprCompose:
		components := make([]ir.ExpressionHandle, len(kind.Components))
		for i, component := range kind.Components {
			lifted, err := c.liftUpConstExpression(component, span)
			if err != nil {
				return 0, err
			}
			components[i] = lifted
		}
		return appendGlobal(ir.ExprCompose{Type: kind.Type, Components: components}), nil
	case ir.ExprSplat:
		lifted, err := c.liftUpConstExpression(kind.Value, span)
		if err != nil {
			return 0, err
		}
		return appendGlobal(ir.ExprSplat{Size: kind.Size, Value: lifted}), nil
	default:
		return 0, errMessage(span, "Expression is not const-expression")
	}
}

// scalarComponents returns the scalar type of a scalar, vector, matrix or
// value pointer, Rust naga's types::scalar_components (types.rs:199-207).
func scalarComponents(inner ir.TypeInner) (ir.ScalarType, bool) {
	switch t := inner.(type) {
	case ir.ScalarType:
		return t, true
	case ir.VectorType:
		return t.Scalar, true
	case ir.MatrixType:
		return t.Scalar, true
	case ir.ValuePointerType:
		return t.Scalar, true
	default:
		return ir.ScalarType{}, false
	}
}

// typePower is a scalar type's rank in GLSL's implicit conversions, Rust
// naga's types::type_power (types.rs:209-217): sint < uint < float32 < other
// floats. A bool has no rank.
func typePower(scalar ir.ScalarType) (uint32, bool) {
	switch scalar.Kind {
	case ir.ScalarSint:
		return 0, true
	case ir.ScalarUint:
		return 1, true
	case ir.ScalarFloat:
		if scalar.Width == 4 {
			return 2, true
		}
		return 3, true
	default:
		return 0, false
	}
}

// vectorSizeCount is the number of components of a vector size.
func vectorSizeCount(size ir.VectorSize) int {
	switch size {
	case ir.Vec2:
		return 2
	case ir.Vec3:
		return 3
	case ir.Vec4:
		return 4
	}
	return 0
}
