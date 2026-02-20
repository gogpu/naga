package wgsl

// Module represents a WGSL module (translation unit).
type Module struct {
	Enables     []Enable
	Diagnostics []Diagnostic
	Structs     []*StructDecl
	Functions   []*FunctionDecl
	GlobalVars  []*VarDecl
	Aliases     []*AliasDecl
	Constants   []*ConstDecl
}

// Enable represents an enable directive.
type Enable struct {
	Extensions []string
	Span       Span
}

// Diagnostic represents a diagnostic directive.
type Diagnostic struct {
	Severity string
	Rule     string
	Span     Span
}

// Node is the base interface for all AST nodes.
type Node interface {
	Pos() Span
}

// Decl is the interface for declarations.
type Decl interface {
	Node
	declNode()
}

// Stmt is the interface for statements.
type Stmt interface {
	Node
	stmtNode()
}

// Expr is the interface for expressions.
type Expr interface {
	Node
	exprNode()
}

// StructDecl represents a struct declaration.
type StructDecl struct {
	Name    string
	Members []*StructMember
	Span    Span
}

func (s *StructDecl) Pos() Span { return s.Span }
func (s *StructDecl) declNode() {}

// StructMember represents a struct member.
type StructMember struct {
	Name       string
	Type       Type
	Attributes []Attribute
	Span       Span
}

// FunctionDecl represents a function declaration.
type FunctionDecl struct {
	Name        string
	Params      []*Parameter
	ReturnType  Type
	ReturnAttrs []Attribute // Attributes on return type (e.g., @builtin(position), @location(0))
	Attributes  []Attribute
	Body        *BlockStmt
	Span        Span
}

func (f *FunctionDecl) Pos() Span { return f.Span }
func (f *FunctionDecl) declNode() {}

// Parameter represents a function parameter.
type Parameter struct {
	Name       string
	Type       Type
	Attributes []Attribute
	Span       Span
}

// VarDecl represents a variable declaration.
type VarDecl struct {
	Name         string
	Type         Type
	Init         Expr
	AddressSpace string // function, private, workgroup, uniform, storage
	AccessMode   string // read, write, read_write
	Attributes   []Attribute
	Span         Span
}

func (v *VarDecl) Pos() Span { return v.Span }
func (v *VarDecl) declNode() {}
func (v *VarDecl) stmtNode() {}

// ConstDecl represents a const declaration.
type ConstDecl struct {
	Name string
	Type Type
	Init Expr
	Span Span
}

func (c *ConstDecl) Pos() Span { return c.Span }
func (c *ConstDecl) declNode() {}
func (c *ConstDecl) stmtNode() {} // Allow const as local statement

// AliasDecl represents a type alias declaration.
type AliasDecl struct {
	Name string
	Type Type
	Span Span
}

func (a *AliasDecl) Pos() Span { return a.Span }
func (a *AliasDecl) declNode() {}

// Attribute represents an attribute (e.g., @location(0)).
type Attribute struct {
	Name string
	Args []Expr
	Span Span
}

// Type represents a type.
type Type interface {
	Node
	typeNode()
}

// NamedType represents a named type (e.g., f32, vec3<f32>).
type NamedType struct {
	Name       string
	TypeParams []Type
	Span       Span
}

func (n *NamedType) Pos() Span { return n.Span }
func (n *NamedType) typeNode() {}

// ArrayType represents an array type.
type ArrayType struct {
	Element Type
	Size    Expr // nil for runtime-sized arrays
	Span    Span
}

func (a *ArrayType) Pos() Span { return a.Span }
func (a *ArrayType) typeNode() {}

// BindingArrayType represents a binding array type (binding_array<T, N>).
type BindingArrayType struct {
	Element Type
	Size    Expr // nil for unbounded
	Span    Span
}

func (b *BindingArrayType) Pos() Span { return b.Span }
func (b *BindingArrayType) typeNode() {}

// PtrType represents a pointer type.
type PtrType struct {
	AddressSpace string
	PointeeType  Type
	AccessMode   string
	Span         Span
}

func (p *PtrType) Pos() Span { return p.Span }
func (p *PtrType) typeNode() {}

// Statements

// BlockStmt represents a block statement.
type BlockStmt struct {
	Statements []Stmt
	Span       Span
}

func (b *BlockStmt) Pos() Span { return b.Span }
func (b *BlockStmt) stmtNode() {}

// ReturnStmt represents a return statement.
type ReturnStmt struct {
	Value Expr
	Span  Span
}

func (r *ReturnStmt) Pos() Span { return r.Span }
func (r *ReturnStmt) stmtNode() {}

// IfStmt represents an if statement.
type IfStmt struct {
	Condition Expr
	Body      *BlockStmt
	Else      Stmt // *BlockStmt or *IfStmt
	Span      Span
}

func (i *IfStmt) Pos() Span { return i.Span }
func (i *IfStmt) stmtNode() {}

// ForStmt represents a for loop.
type ForStmt struct {
	Init      Stmt
	Condition Expr
	Update    Stmt
	Body      *BlockStmt
	Span      Span
}

func (f *ForStmt) Pos() Span { return f.Span }
func (f *ForStmt) stmtNode() {}

// WhileStmt represents a while loop.
type WhileStmt struct {
	Condition Expr
	Body      *BlockStmt
	Span      Span
}

func (w *WhileStmt) Pos() Span { return w.Span }
func (w *WhileStmt) stmtNode() {}

// LoopStmt represents a loop statement.
type LoopStmt struct {
	Body       *BlockStmt
	Continuing *BlockStmt
	Span       Span
}

func (l *LoopStmt) Pos() Span { return l.Span }
func (l *LoopStmt) stmtNode() {}

// BreakStmt represents a break statement.
type BreakStmt struct {
	Span Span
}

func (b *BreakStmt) Pos() Span { return b.Span }
func (b *BreakStmt) stmtNode() {}

// ContinueStmt represents a continue statement.
type ContinueStmt struct {
	Span Span
}

func (c *ContinueStmt) Pos() Span { return c.Span }
func (c *ContinueStmt) stmtNode() {}

// DiscardStmt represents a discard statement.
type DiscardStmt struct {
	Span Span
}

func (d *DiscardStmt) Pos() Span { return d.Span }
func (d *DiscardStmt) stmtNode() {}

// AssignStmt represents an assignment statement.
type AssignStmt struct {
	Left  Expr
	Op    TokenKind // =, +=, -=, etc.
	Right Expr
	Span  Span
}

func (a *AssignStmt) Pos() Span { return a.Span }
func (a *AssignStmt) stmtNode() {}

// ExprStmt represents an expression statement.
type ExprStmt struct {
	Expr Expr
	Span Span
}

func (e *ExprStmt) Pos() Span { return e.Span }
func (e *ExprStmt) stmtNode() {}

// SwitchStmt represents a switch statement.
type SwitchStmt struct {
	Selector Expr
	Cases    []*SwitchCaseClause
	Span     Span
}

func (s *SwitchStmt) Pos() Span { return s.Span }
func (s *SwitchStmt) stmtNode() {}

// SwitchCaseClause represents a case clause in a switch statement.
type SwitchCaseClause struct {
	Selectors []Expr     // Case selectors (nil or empty for default)
	IsDefault bool       // True for default case
	Body      *BlockStmt // Case body
	Span      Span
}

// Expressions

// Ident represents an identifier.
type Ident struct {
	Name string
	Span Span
}

func (i *Ident) Pos() Span { return i.Span }
func (i *Ident) exprNode() {}

// Literal represents a literal value.
type Literal struct {
	Kind  TokenKind // IntLiteral, FloatLiteral, BoolLiteral
	Value string
	Span  Span
}

func (l *Literal) Pos() Span { return l.Span }
func (l *Literal) exprNode() {}

// BinaryExpr represents a binary expression.
type BinaryExpr struct {
	Left  Expr
	Op    TokenKind
	Right Expr
	Span  Span
}

func (b *BinaryExpr) Pos() Span { return b.Span }
func (b *BinaryExpr) exprNode() {}

// UnaryExpr represents a unary expression.
type UnaryExpr struct {
	Op      TokenKind
	Operand Expr
	Span    Span
}

func (u *UnaryExpr) Pos() Span { return u.Span }
func (u *UnaryExpr) exprNode() {}

// CallExpr represents a function call.
type CallExpr struct {
	Func *Ident
	Args []Expr
	Span Span
}

func (c *CallExpr) Pos() Span { return c.Span }
func (c *CallExpr) exprNode() {}

// IndexExpr represents an index expression.
type IndexExpr struct {
	Expr  Expr
	Index Expr
	Span  Span
}

func (i *IndexExpr) Pos() Span { return i.Span }
func (i *IndexExpr) exprNode() {}

// MemberExpr represents a member access expression.
type MemberExpr struct {
	Expr   Expr
	Member string
	Span   Span
}

func (m *MemberExpr) Pos() Span { return m.Span }
func (m *MemberExpr) exprNode() {}

// ConstructExpr represents a type constructor expression.
type ConstructExpr struct {
	Type Type
	Args []Expr
	Span Span
}

func (c *ConstructExpr) Pos() Span { return c.Span }
func (c *ConstructExpr) exprNode() {}

// BitcastExpr represents a bitcast expression: bitcast<TargetType>(expr).
type BitcastExpr struct {
	Type Type // Target type
	Expr Expr // Source expression
	Span Span
}

func (b *BitcastExpr) Pos() Span { return b.Span }
func (b *BitcastExpr) exprNode() {}
