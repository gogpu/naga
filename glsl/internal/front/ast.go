package front

import "github.com/gogpu/naga/ir"

// This file defines the AST the GLSL parser builds: the HIR expression kinds of
// Rust naga's front/glsl/ast.rs (HirExprKind, ast.rs:118-178), the qualifier
// and declaration forms of the language, and the statement tree.
//
// Rust naga lowers every expression immediately while it parses it, so its HIR
// nodes already point at IR expressions. Here the parser is one pass and
// lowering is a second one over the finished statements: the HIR kinds and the
// per-statement arena are Rust's, but a hirVariable names a parsed symbol
// instead of an IR handle, and a call keeps its arguments as HIR nodes.
// Lowering walks the statements in source order, which is the order Rust emits
// them in, so the resulting IR has Rust's shape.

// subsumeSpan extends s to cover other, as Rust naga's Span::subsume
// (span.rs:39-52): an undefined span takes the other one.
func subsumeSpan(s, other ir.Span) ir.Span {
	switch {
	case !s.IsDefined():
		return other
	case !other.IsDefined():
		return s
	default:
		return ir.Span{Start: min(s.Start, other.Start), End: max(s.End, other.End)}
	}
}

// hirHandle is an expression handle into the HIR arena of one statement.
type hirHandle uint32

// hirNode is one expression with the source span it was written at.
type hirNode struct {
	Kind hirExpr
	Span ir.Span
}

// hirExpr is the kind of one HIR expression: the syntax of a GLSL expression
// before lowering decides its type, the implicit conversions and the IR
// expressions it becomes.
type hirExpr interface {
	hirExprKind()
}

// hirArena holds the HIR expressions of one statement.
type hirArena struct {
	Nodes []hirNode
}

// add appends a node to the arena and returns its handle.
func (a *hirArena) add(kind hirExpr, span ir.Span) hirHandle {
	a.Nodes = append(a.Nodes, hirNode{Kind: kind, Span: span})
	return hirHandle(len(a.Nodes) - 1)
}

// get returns the node with the given handle.
func (a *hirArena) get(h hirHandle) *hirNode { return &a.Nodes[h] }

// exprs is the expression arena of one statement and the expression the
// statement evaluates. Rust naga keeps exactly one HIR arena per statement
// (StmtContext in context.rs), which this mirrors.
type exprs struct {
	Arena hirArena
	Root  hirHandle
}

// hirSequence evaluates its expressions in order and yields the value of the
// last one; this is the comma operator.
type hirSequence struct {
	Exprs []hirHandle
}

func (hirSequence) hirExprKind() {}

// hirAccess indexes a base expression: an array element, a vector component or
// a matrix column.
type hirAccess struct {
	Base  hirHandle
	Index hirHandle
}

func (hirAccess) hirExprKind() {}

// hirSelect selects a struct field or a vector or matrix swizzle.
type hirSelect struct {
	Base  hirHandle
	Field string
}

func (hirSelect) hirExprKind() {}

// hirLiteral is a literal constant.
type hirLiteral struct {
	Value ir.LiteralValue
}

func (hirLiteral) hirExprKind() {}

// hirBinary applies a binary operator to two operands.
type hirBinary struct {
	Left  hirHandle
	Op    ir.BinaryOperator
	Right hirHandle
}

func (hirBinary) hirExprKind() {}

// hirUnary applies a unary operator.
type hirUnary struct {
	Op   ir.UnaryOperator
	Expr hirHandle
}

func (hirUnary) hirExprKind() {}

// hirVariable is a reference to a declared name: a local, a parameter, a global
// or a builtin variable.
type hirVariable struct {
	Sym *glslSymbol
}

func (hirVariable) hirExprKind() {}

// calleeKind tells what a call invokes or constructs.
type calleeKind uint8

const (
	// calleeFunction calls a function by name; overload resolution picks the
	// overload during lowering.
	calleeFunction calleeKind = iota
	// calleeTypeName constructs a value of a builtin type, as in vec4(...).
	calleeTypeName
	// calleeStruct constructs a value of a struct type declared in the shader.
	calleeStruct
)

// callee is the target of a call expression.
type callee struct {
	Kind calleeKind
	Name string    // the function name, when Kind is calleeFunction
	Typ  *typeSpec // the constructed type, otherwise
}

// hirCall calls a function or constructs a value of a type.
type hirCall struct {
	Callee callee
	Args   []hirHandle
}

func (hirCall) hirExprKind() {}

// hirConditional is the ternary operator: condition ? accept : reject.
type hirConditional struct {
	Condition hirHandle
	Accept    hirHandle
	Reject    hirHandle
}

func (hirConditional) hirExprKind() {}

// hirAssign stores value into tgt. GLSL assignments are expressions whose
// value is the stored one, which lowering yields as the value expression.
type hirAssign struct {
	Tgt   hirHandle
	Value hirHandle
}

func (hirAssign) hirExprKind() {}

// hirPrePostfix is the prefix or postfix increment and decrement. The operator
// is BinaryAdd or BinarySubtract, as in Rust naga.
type hirPrePostfix struct {
	Op      ir.BinaryOperator
	Postfix bool
	Expr    hirHandle
}

func (hirPrePostfix) hirExprKind() {}

// hirMethod is a method call, as in a.length().
type hirMethod struct {
	Expr hirHandle
	Name string
	Args []hirHandle
}

func (hirMethod) hirExprKind() {}

// symbolKind tells what a name declared during parsing denotes.
type symbolKind uint8

const (
	symLocal      symbolKind = iota // a function-local variable or constant
	symParam                        // a function parameter
	symGlobal                       // a module-scope variable, constant or block instance
	symBuiltinVar                   // a GLSL builtin variable, such as gl_Position
)

// glslSymbol is one name declared while parsing. Lowering attaches the IR
// declaration to it.
type glslSymbol struct {
	Kind symbolKind
	Name string
}

// symbolTable resolves names during parsing: a stack of scopes whose root holds
// the module-scope names. It is Rust naga's front::SymbolTable without the
// lookup-scope trick lowering uses when it splits branches (front/mod.rs:216).
type symbolTable struct {
	Scopes []map[string]*glslSymbol
}

// newSymbolTable returns a symbol table with its root scope.
func newSymbolTable() *symbolTable {
	return &symbolTable{Scopes: []map[string]*glslSymbol{{}}}
}

// pushScope opens a scope for a compound statement or a for statement.
func (s *symbolTable) pushScope() {
	s.Scopes = append(s.Scopes, map[string]*glslSymbol{})
}

// popScope closes the innermost scope; the root scope stays.
func (s *symbolTable) popScope() {
	if len(s.Scopes) > 1 {
		s.Scopes = s.Scopes[:len(s.Scopes)-1]
	}
}

// declare inserts sym into the innermost scope and returns the symbol it
// replaced there, or nil. Replacing is Rust naga's SymbolTable::add
// (front/mod.rs): the caller decides whether the replacement is an error — a
// duplicate local is (variables.rs:694-701), a repeated module-scope name is
// not (context.rs:232 discards the replaced symbol silently).
func (s *symbolTable) declare(sym *glslSymbol) *glslSymbol {
	scope := s.Scopes[len(s.Scopes)-1]
	prev := scope[sym.Name]
	scope[sym.Name] = sym
	return prev
}

// declareGlobal inserts sym into the root scope and returns the symbol it
// replaced there, or nil. Module-scope names live in the root whatever scope
// the parser is in when it meets them: an interface block inside a function
// still declares module-scope names, as Rust does.
func (s *symbolTable) declareGlobal(sym *glslSymbol) *glslSymbol {
	root := s.Scopes[0]
	prev := root[sym.Name]
	root[sym.Name] = sym
	return prev
}

// lookup resolves a name through the scopes, innermost first.
func (s *symbolTable) lookup(name string) *glslSymbol {
	for i := len(s.Scopes) - 1; i >= 0; i-- {
		if sym, ok := s.Scopes[i][name]; ok {
			return sym
		}
	}
	return nil
}

// storageQualifier is the storage class a declaration names. A declaration
// without one is storageNone, which Rust naga spells
// AddressSpace(Function) (ast.rs:323-335).
type storageQualifier uint8

const (
	storageNone storageQualifier = iota
	storageConst
	storageInput
	storageOutput
	storageUniform
	storageShared
	storageBuffer
)

// interpolationQualifier is the interpolation of an in or out variable.
type interpolationQualifier uint8

const (
	interpolationNone interpolationQualifier = iota
	interpolationFlat
	interpolationSmooth
	interpolationNoPerspective
)

// samplingQualifier is the per-sample or centroid sampling of an in variable.
type samplingQualifier uint8

const (
	samplingNone samplingQualifier = iota
	samplingCentroid
	samplingSample
)

// precisionQualifier is a lowp, mediump or highp precision hint.
type precisionQualifier uint8

const (
	precisionNone precisionQualifier = iota
	precisionLow
	precisionMedium
	precisionHigh
)

// structLayout is the layout applied to a struct or an interface block.
type structLayout uint8

const (
	structLayoutNone structLayout = iota
	structLayoutStd140
	structLayoutStd430
)

// layoutKind is the kind of one layout qualifier entry, Rust naga's
// QualifierKey (ast.rs:180-189).
type layoutKind uint8

const (
	// layoutNamed is an identifier entry, with an optional constant value:
	// local_size_x, local_size_x = 8, early_fragment_tests.
	layoutNamed layoutKind = iota
	// layoutStructure is std140 or std430.
	layoutStructure
	// layoutFormat is a storage image format such as rgba8.
	layoutFormat
	// layoutIndex is index = <constant>.
	layoutIndex
)

// layoutQualifier is one entry of a layout(...) list. Entries keep the order
// they were written in; a repeated key is kept as written, and lowering applies
// Rust's last-wins rule when it reads them (Rust stores them in a map, where an
// insert of the same key replaces the earlier value, parser/types.rs:380).
type layoutQualifier struct {
	Kind     layoutKind
	Name     string           // the entry's spelling; empty for std140 and std430
	HasValue bool             // an `= constant_expression` was written
	Value    *exprs           // the value expression, when HasValue
	Format   ir.StorageFormat // Kind == layoutFormat
	Struct   structLayout     // Kind == layoutStructure
	Span     ir.Span
}

// typeQualifiers is the set of qualifiers written before a declaration, Rust
// naga's TypeQualifiers (ast.rs:199-211). A value of its zero enum means the
// qualifier was not written.
type typeQualifiers struct {
	Span ir.Span

	Storage     storageQualifier
	StorageSpan ir.Span

	Invariant     bool
	InvariantSpan ir.Span

	Interpolation     interpolationQualifier
	InterpolationSpan ir.Span

	Sampling     samplingQualifier
	SamplingSpan ir.Span

	Precision     precisionQualifier
	PrecisionSpan ir.Span

	// StorageAccess is the access the memory qualifiers allow; it is only
	// meaningful when HasStorageAccess is set.
	StorageAccess     ir.StorageAccess
	HasStorageAccess  bool
	StorageAccessSpan ir.Span

	Layout []layoutQualifier
}

// typeSpec is a type as written. A struct definition is the only type GLSL
// writes inline; a named type is a struct the shader declared earlier. GLSL
// allows array specifiers both after the type and after a declarator's name
// (`float[2] a[3]`), and the two nest around each other, so each lives where it
// was written.
type typeSpec struct {
	Builtin   *typeName  // a builtin spelling, when non-nil
	Name      string     // the struct name, when Builtin is nil
	StructDef *structDef // the inline definition, when the type is a written struct
	Arrays    []arraySpec
	Span      ir.Span
}

// arraySpec is one array dimension of a declarator. A nil Size is an unsized
// dimension, `[]`.
type arraySpec struct {
	Size *exprs
	Span ir.Span
}

// initNode is one initializer: a single expression or a braced list of nested
// initializers, which GLSL allows for structs and arrays.
type initNode struct {
	Expr  hirHandle  // set when the node is one expression
	Items []initNode // set when the node is a braced list
	Span  ir.Span
}

// initializer is a declarator's initializer and the arena its expressions live
// in; a braced list is one arena shared by all its expressions.
type initializer struct {
	Arena hirArena
	Root  initNode
}

// declarator is one name of an init_declarator_list with its dimensions and its
// initializer. Sym is the symbol the parser declared for the name; lowering
// attaches the IR declaration to it, and every reference the parser built
// points at it.
type declarator struct {
	Name   string
	Sym    *glslSymbol
	Arrays []arraySpec
	Init   *initializer // nil when the declarator has no initializer
	Span   ir.Span
}

// declaration is one declaration: a variable declaration list, a function
// prototype or definition, an interface block definition, or one of the
// declarations that only carry qualifiers or a precision.
type declaration interface {
	declarationKind()
}

// declVarList declares one or more variables of a common type. A declaration of
// a struct and nothing else, as in `struct S { ... };`, is one of these with no
// declarators and the definition inside Typ.
type declVarList struct {
	Qualifiers  typeQualifiers
	Typ         typeSpec
	Declarators []declarator
}

func (declVarList) declarationKind() {}

// param is one parameter of a function prototype or definition.
type param struct {
	Qualifier parameterQualifier
	Typ       typeSpec
	Name      string // empty for an unnamed parameter, which #5388 makes legal here
	Sym       *glslSymbol
	Arrays    []arraySpec
	Span      ir.Span
}

// parameterQualifier is the direction of a parameter.
type parameterQualifier uint8

const (
	paramIn parameterQualifier = iota
	paramOut
	paramInOut
	paramConst
)

// isLHS reports whether a parameter is passed as a pointer, which out and
// inout parameters are, Rust naga's ParameterQualifier::is_lhs.
func (q parameterQualifier) isLHS() bool { return q == paramOut || q == paramInOut }

// declFunc declares a function: a prototype when Body is nil, a definition
// otherwise. A void return type is a nil Return.
type declFunc struct {
	Qualifiers typeQualifiers
	Return     *typeSpec
	Name       string
	Params     []param
	Body       []astStmt
	Span       ir.Span
}

func (declFunc) declarationKind() {}

// structDef is a struct type definition.
type structDef struct {
	Name    string
	Members []structMember
	Span    ir.Span
}

// structMember is one member of a struct or interface block. Sym is set for
// the members of an interface block, which declare module-scope names; a
// struct definition's members declare nothing.
type structMember struct {
	Typ    typeSpec
	Name   string
	Sym    *glslSymbol
	Arrays []arraySpec
	Span   ir.Span
}

// declBlock declares an interface block: a block type with an optional instance
// name, as in `uniform Blk { vec4 color; } blk;` or `uniform Blk { ... };`.
type declBlock struct {
	Qualifiers  typeQualifiers
	Name        string // the block type name
	Members     []structMember
	Instance    string      // the instance name; empty when the block has none
	InstanceSym *glslSymbol // the instance name's symbol, nil without one
	Arrays      []arraySpec // dimensions of the instance name
	Span        ir.Span
}

func (declBlock) declarationKind() {}

// declQualifiers declares qualifiers without a type, as in
// `layout(local_size_x = 8) in;` or `layout(early_fragment_tests) in;`.
type declQualifiers struct {
	Qualifiers typeQualifiers
}

func (declQualifiers) declarationKind() {}

// declInvariant redeclares a variable as invariant: `invariant gl_Position;`.
type declInvariant struct {
	Name string
	Sym  *glslSymbol
}

func (declInvariant) declarationKind() {}

// declPrecision sets the default precision of a type: `precision highp float;`.
type declPrecision struct {
	Precision precisionQualifier
	Typ       typeSpec
}

func (declPrecision) declarationKind() {}

// declInvalid is a declaration the parser rejected with a diagnostic and
// recovered from by consuming its tokens, as Rust naga returns after reporting
// the error (parser/declarations.rs:400-416). Lowering ignores it; the node
// exists so the parse loop keeps its place instead of re-reading the tokens.
type declInvalid struct{}

func (declInvalid) declarationKind() {}

// astDecl is one declaration with its span.
type astDecl struct {
	Kind declaration
	Span ir.Span
}

// astStmt is one statement with its span.
type astStmt struct {
	Kind astStmtKind
	Span ir.Span
}

// astStmtKind is the kind of one statement.
type astStmtKind interface {
	astStmtKind()
}

// stmtEmpty is an empty statement: a lone semicolon.
type stmtEmpty struct{}

func (stmtEmpty) astStmtKind() {}

// stmtBlock is a compound statement: a block with its own scope.
type stmtBlock struct {
	Body []astStmt
}

func (stmtBlock) astStmtKind() {}

// stmtDecl is a declaration used as a statement.
type stmtDecl struct {
	Decl astDecl
}

func (stmtDecl) astStmtKind() {}

// stmtExpr is an expression statement.
type stmtExpr struct {
	Expr exprs
}

func (stmtExpr) astStmtKind() {}

// stmtIf is an if statement; Reject is nil when there is no else branch.
type stmtIf struct {
	Cond   exprs
	Accept *astStmt
	Reject *astStmt
}

func (stmtIf) astStmtKind() {}

// switchCase is one case or default label of a switch with its statements.
// Label is nil for the default label.
type switchCase struct {
	Label *exprs
	Body  []astStmt
	Span  ir.Span
}

// stmtSwitch is a switch statement.
type stmtSwitch struct {
	Selector exprs
	Cases    []switchCase
}

func (stmtSwitch) astStmtKind() {}

// stmtWhile is a while statement.
type stmtWhile struct {
	Cond exprs
	Body astStmt
}

func (stmtWhile) astStmtKind() {}

// stmtDo is a do-while statement.
type stmtDo struct {
	Body astStmt
	Cond exprs
}

func (stmtDo) astStmtKind() {}

// stmtFor is a for statement. The init clause is a declaration, an expression
// or absent (both InitDecl and InitExpr nil); the condition and the update
// clause are expressions or absent.
type stmtFor struct {
	InitDecl *astDecl
	InitExpr *exprs
	Cond     *exprs
	Update   *exprs
	Body     astStmt
}

func (stmtFor) astStmtKind() {}

// stmtReturn is a return statement; Value is nil for a bare return.
type stmtReturn struct {
	Value *exprs
}

func (stmtReturn) astStmtKind() {}

// stmtBreak is a break statement.
type stmtBreak struct{}

func (stmtBreak) astStmtKind() {}

// stmtContinue is a continue statement.
type stmtContinue struct{}

func (stmtContinue) astStmtKind() {}

// stmtDiscard is a discard statement.
type stmtDiscard struct{}

func (stmtDiscard) astStmtKind() {}

// parsedFile is the AST of one source: its external declarations in source
// order and the semantic errors found while parsing. A syntax error aborts the
// parse and is the last error in the list, as Rust naga returns the fatal error
// after the collected ones (mod.rs:207-219).
type parsedFile struct {
	Decls  []astDecl
	Errors []frontError
}
