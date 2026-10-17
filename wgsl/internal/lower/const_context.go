package lower

import (
	"errors"
	"fmt"
	"math"

	"github.com/gogpu/naga/internal/consteval"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl/internal/parser"
)

// The module const context is Rust naga's ExpressionContextType::Constant(None):
// a module-scope const-expression is lowered by the same lowerExpression as a
// function body, but its expressions go to Module.GlobalExpressions and every
// one of them must evaluate (consteval.BehaviorConst).
//
// The arena is a function value whose Expressions alias
// Module.GlobalExpressions, the form ir.ResolveExpressionType takes for any
// arena. It lives as long as the lowerer, like Rust's const_typifier and
// global_expression_kind_tracker, so the types and evaluation stages of the
// global expressions are computed once.

// moduleArena is Module.GlobalExpressions as the evaluator's arena while the
// lowerer is in the module const context (the lowerer's current function is
// then moduleConstArena).
type moduleArena struct{ fnArena }

func (moduleArena) IsGlobal() bool { return true }

// functionState is the per-function lowering state that the module const
// context replaces and restores.
type functionState struct {
	currentFunc       *ir.Function
	currentFuncIdx    ir.FunctionHandle
	currentExprIdx    ir.ExpressionHandle
	locals            map[string]ir.ExpressionHandle
	localDecls        map[string]parser.Span
	usedLocals        map[string]bool
	localConsts       map[string]bool
	localIsVar        map[string]bool
	localIsPtr        map[string]bool
	localAbstractASTs map[string]parser.Expr
	scopeStack        []scopeFrame
	isInsideLoop      bool
	isStatement       bool
	emitStateStart    *ir.ExpressionHandle
	currentEmitTarget *[]ir.Statement
	globalExprCache   map[ir.GlobalVariableHandle]ir.ExpressionHandle
}

func (l *Lowerer) saveFunctionState() functionState {
	return functionState{
		currentFunc:       l.currentFunc,
		currentFuncIdx:    l.currentFuncIdx,
		currentExprIdx:    l.currentExprIdx,
		locals:            l.locals,
		localDecls:        l.localDecls,
		usedLocals:        l.usedLocals,
		localConsts:       l.localConsts,
		localIsVar:        l.localIsVar,
		localIsPtr:        l.localIsPtr,
		localAbstractASTs: l.localAbstractASTs,
		scopeStack:        l.scopeStack,
		isInsideLoop:      l.isInsideLoop,
		isStatement:       l.isStatement,
		emitStateStart:    l.emitStateStart,
		currentEmitTarget: l.currentEmitTarget,
		globalExprCache:   l.globalExprCache,
	}
}

func (l *Lowerer) restoreFunctionState(s functionState) {
	l.currentFunc = s.currentFunc
	l.currentFuncIdx = s.currentFuncIdx
	l.currentExprIdx = s.currentExprIdx
	l.locals = s.locals
	l.localDecls = s.localDecls
	l.usedLocals = s.usedLocals
	l.localConsts = s.localConsts
	l.localIsVar = s.localIsVar
	l.localIsPtr = s.localIsPtr
	l.localAbstractASTs = s.localAbstractASTs
	l.scopeStack = s.scopeStack
	l.isInsideLoop = s.isInsideLoop
	l.isStatement = s.isStatement
	l.emitStateStart = s.emitStateStart
	l.currentEmitTarget = s.currentEmitTarget
	l.globalExprCache = s.globalExprCache
}

// enterModuleConstContext makes Module.GlobalExpressions the arena that
// expression lowering appends to, with no locals and no emitter, and returns
// the function state to restore.
func (l *Lowerer) enterModuleConstContext() functionState {
	saved := l.saveFunctionState()
	if l.moduleConstArena == nil {
		l.moduleConstArena = &ir.Function{NamedExpressions: map[ir.ExpressionHandle]string{}}
		arena := moduleArena{fnArena{l}}
		l.moduleEval = &consteval.Evaluator{Module: l.module, Arena: arena, Types: arena, Behavior: consteval.BehaviorConst}
	}
	a := l.moduleConstArena
	// Global variable and override initializers are still appended to
	// Module.GlobalExpressions outside this context (TECH-GE-002): type the
	// expressions added since the last visit.
	a.Expressions = l.module.GlobalExpressions
	for h := len(a.ExpressionTypes); h < len(a.Expressions); h++ {
		ty, err := ir.ResolveExpressionType(l.module, a, index32[ir.ExpressionHandle](h))
		if err != nil {
			ty = ir.TypeResolution{}
		}
		a.ExpressionTypes = append(a.ExpressionTypes, ty)
	}

	l.currentFunc = a
	l.currentExprIdx = index32[ir.ExpressionHandle](len(a.Expressions))
	l.locals = map[string]ir.ExpressionHandle{}
	l.localDecls = map[string]parser.Span{}
	l.usedLocals = map[string]bool{}
	l.localConsts = map[string]bool{}
	l.localIsVar = map[string]bool{}
	l.localIsPtr = map[string]bool{}
	l.localAbstractASTs = map[string]parser.Expr{}
	l.scopeStack = nil
	l.isInsideLoop = false
	l.isStatement = false
	l.emitStateStart = nil
	l.currentEmitTarget = nil
	l.globalExprCache = map[ir.GlobalVariableHandle]ir.ExpressionHandle{}
	return saved
}

func (l *Lowerer) leaveModuleConstContext(saved functionState) {
	l.module.GlobalExpressions = l.moduleConstArena.Expressions
	l.restoreFunctionState(saved)
}

// inModuleConstContext reports whether expressions go to
// Module.GlobalExpressions.
func (l *Lowerer) inModuleConstContext() bool {
	return l.moduleConstArena != nil && l.currentFunc == l.moduleConstArena
}

// inRuntimeContext is Rust's ExpressionContext::is_runtime: a function body
// outside a const-expression.
func (l *Lowerer) inRuntimeContext() bool {
	return l.currentFunc != nil && !l.inModuleConstContext() && l.eval().Behavior == consteval.BehaviorRuntime
}

// lowerConstant lowers a module-scope `const` like Rust naga's
// GlobalDeclKind::Const: the explicit type is resolved first, the
// initializer is lowered and evaluated in the module const context and
// converted to the explicit type, or keeps its own type, abstract or not
// (AbstractRule::Allow). An abstract constant is a constant like any other;
// compaction removes it once lowering is done.
func (l *Lowerer) lowerConstant(c *parser.ConstDecl) error {
	if c.Init == nil {
		return fmt.Errorf("module constant '%s' must have initializer", c.Name)
	}
	var explicit *ir.TypeHandle
	if c.Type != nil {
		th, err := l.resolveType(c.Type)
		if err != nil {
			return fmt.Errorf("module constant '%s': %w", c.Name, err)
		}
		explicit = &th
	}

	saved := l.enterModuleConstContext()
	ty, init, err := l.lowerModuleConstInit(c, explicit)
	l.leaveModuleConstContext(saved)
	if err != nil {
		return fmt.Errorf("module constant '%s': %w", c.Name, err)
	}

	ch := index32[ir.ConstantHandle](len(l.module.Constants))
	l.module.Constants = append(l.module.Constants, ir.Constant{
		Name:  c.Name,
		Type:  ty,
		Init:  init,
		Value: constantScalarValue(l.module.GlobalExpressions[init].Kind),
	})
	l.moduleConstants[c.Name] = ch
	l.markConstInlineInit(ch)
	return nil
}

func (l *Lowerer) lowerModuleConstInit(c *parser.ConstDecl, explicit *ir.TypeHandle) (ir.TypeHandle, ir.ExpressionHandle, error) {
	h, err := l.lowerConstExpr(c.Init, nil)
	if err != nil {
		return 0, 0, err
	}
	return l.typeAndInit(c.Name, h, explicit, allowAbstract)
}

// lowerGlobalVarInit returns a global variable's type and initializer, like
// Rust naga's GlobalDeclKind::Var: type_and_init with AbstractRule::Concretize
// in the override context, where an override-expression is kept for
// pipeline creation and everything else must evaluate.
func (l *Lowerer) lowerGlobalVarInit(v *parser.VarDecl, explicit *ir.TypeHandle) (ir.TypeHandle, *ir.ExpressionHandle, error) {
	return l.lowerOverrideContextInit(v.Name, v.Init, explicit)
}

// lowerOverrideContextInit lowers the initializer of a var<private> or an
// override declaration in the module const context with override behavior
// and concretizes it (Rust: GlobalDeclKind::Var / Override, type_and_init
// with AbstractRule::Concretize in ctx.as_override()). The initializer is
// appended to GlobalExpressions when the declaration is lowered. Without an
// initializer the explicit type is required.
func (l *Lowerer) lowerOverrideContextInit(name string, init parser.Expr, explicit *ir.TypeHandle) (ir.TypeHandle, *ir.ExpressionHandle, error) {
	if init == nil {
		if explicit == nil {
			return 0, nil, fmt.Errorf("type annotation required without initializer")
		}
		return *explicit, nil, nil
	}
	saved := l.enterModuleConstContext()
	l.moduleEval.Behavior = consteval.BehaviorOverride
	ty, h, err := l.lowerOverrideContextInitExpr(name, init, explicit)
	l.moduleEval.Behavior = consteval.BehaviorConst
	l.leaveModuleConstContext(saved)
	if err != nil {
		return 0, nil, err
	}
	return ty, &h, nil
}

func (l *Lowerer) lowerOverrideContextInitExpr(name string, init parser.Expr, explicit *ir.TypeHandle) (ir.TypeHandle, ir.ExpressionHandle, error) {
	h, err := l.lowerConstExpr(init, nil)
	if err != nil {
		return 0, 0, err
	}
	return l.typeAndInit(name, h, explicit, concretizeAbstract)
}

// constLiteral lowers e as a const-expression where the lowerer is and
// returns its scalar value, concretized (Rust as_const + expression +
// get_const_val): in a function body the expression goes to the function's
// arena, evaluated in const behavior; anywhere else to
// Module.GlobalExpressions. Only the value is used; compaction removes the
// expression.
func (l *Lowerer) constLiteral(e parser.Expr) (ir.LiteralValue, error) {
	if l.currentFunc == nil {
		return l.moduleConstLiteral(e)
	}
	var lit ir.LiteralValue
	_, err := l.inConstContext(func() (ir.ExpressionHandle, error) {
		h, v, err := l.lowerConstLiteral(e, l.currentEmitTarget)
		lit = v
		return h, err
	})
	return lit, err
}

// moduleConstLiteral is constLiteral in the module const context, for
// module-scope expressions met while a function is lowered (entry point
// attributes, Rust ctx.as_global()).
func (l *Lowerer) moduleConstLiteral(e parser.Expr) (ir.LiteralValue, error) {
	saved := l.enterModuleConstContext()
	defer l.leaveModuleConstContext(saved)
	_, lit, err := l.lowerConstLiteral(e, nil)
	return lit, err
}

func (l *Lowerer) lowerConstLiteral(e parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, ir.LiteralValue, error) {
	h, err := l.lowerConstExpr(e, target)
	if err != nil {
		return 0, nil, err
	}
	if h, err = l.concretize(h); err != nil {
		return 0, nil, err
	}
	lit, ok := l.eval().Literal(h)
	if !ok {
		return 0, nil, errNotConstScalar
	}
	return h, lit, nil
}

// errNotConstScalar: the expression evaluated, but not to a scalar.
var errNotConstScalar = errors.New("must be a const-expression that resolves to a scalar")

// lowerConstExpr lowers a const-expression. A const-expression has no
// statements; target receives the emitter's flushes in a function body
// (nil at module scope), anything else lowering produces is an error.
func (l *Lowerer) lowerConstExpr(e parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	var stmts []ir.Statement
	h, err := l.lowerExpression(e, &stmts)
	if err != nil {
		return 0, err
	}
	for _, s := range stmts {
		if _, ok := s.Kind.(ir.StmtEmit); !ok {
			return 0, errors.New("a const-expression cannot call a function")
		}
		if target != nil {
			*target = append(*target, s)
		}
	}
	return h, nil
}

// constU32 is Rust's const_u32: a concrete integer const-expression that is
// not negative.
func (l *Lowerer) constU32(e parser.Expr) (uint32, error) {
	return literalU32(l.constLiteral(e))
}

// moduleConstU32 is constU32 in the module const context.
func (l *Lowerer) moduleConstU32(e parser.Expr) (uint32, error) {
	return literalU32(l.moduleConstLiteral(e))
}

func literalU32(lit ir.LiteralValue, err error) (uint32, error) {
	if err != nil {
		return 0, err
	}
	switch v := lit.(type) {
	case ir.LiteralU32:
		return uint32(v), nil
	case ir.LiteralI32:
		if v < 0 {
			return 0, fmt.Errorf("must be non-negative (>= 0)")
		}
		return uint32(v), nil
	}
	return 0, fmt.Errorf("must be a const-expression that resolves to a concrete integer scalar (`u32` or `i32`)")
}

// constantScalarValue is the Constant.Value of a constant whose initializer
// is a concrete scalar literal; nil otherwise. Backends take the value of
// every constant from Constant.Init; Value only serves the readers that look
// up a known scalar (index bounds, pipeline constants).
func constantScalarValue(init ir.ExpressionKind) ir.ConstantValue {
	lit, ok := init.(ir.Literal)
	if !ok {
		return nil
	}
	switch v := lit.Value.(type) {
	case ir.LiteralBool:
		if v {
			return ir.ScalarValue{Bits: 1, Kind: ir.ScalarBool}
		}
		return ir.ScalarValue{Bits: 0, Kind: ir.ScalarBool}
	case ir.LiteralI32:
		return ir.ScalarValue{Bits: sintBits(int64(v)), Kind: ir.ScalarSint}
	case ir.LiteralI64:
		return ir.ScalarValue{Bits: sintBits(int64(v)), Kind: ir.ScalarSint}
	case ir.LiteralU32:
		return ir.ScalarValue{Bits: uint64(v), Kind: ir.ScalarUint}
	case ir.LiteralU64:
		return ir.ScalarValue{Bits: uint64(v), Kind: ir.ScalarUint}
	case ir.LiteralF32:
		return ir.ScalarValue{Bits: uint64(math.Float32bits(float32(v))), Kind: ir.ScalarFloat}
	case ir.LiteralF64:
		return ir.ScalarValue{Bits: math.Float64bits(float64(v)), Kind: ir.ScalarFloat}
	}
	return nil
}

// index32 converts an arena length or index to a 32-bit IR handle or count.
// The IR addresses every arena with uint32 handles, so a valid index always
// fits.
func index32[T ~uint32](n int) T {
	return T(n)
}

// sintBits is the two's-complement bit pattern of a signed integer, the
// representation ScalarValue stores for ScalarSint.
func sintBits(v int64) uint64 {
	return uint64(v) //nolint:gosec // two's-complement reinterpretation is the stored representation
}
