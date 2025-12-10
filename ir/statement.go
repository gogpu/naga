package ir

// Statement represents a statement in the IR.
// Statements have side effects and structured control flow, but do not produce values.
// The function body is represented as a tree of statements, with references to expressions.
type Statement struct {
	Kind StatementKind
}

// StatementKind represents the different kinds of statements.
type StatementKind interface {
	statementKind()
}

// Block represents a sequence of statements executed in order.
// This is a simplified version without span tracking (spans will be added later if needed).
type Block []Statement

// Range represents a range of expression handles for Emit statements.
type Range struct {
	Start ExpressionHandle
	End   ExpressionHandle // Exclusive
}

// StmtEmit emits a range of expressions, making them visible to all statements that follow.
// This is used to mark when expressions should be evaluated in SSA form.
// See module-level IR documentation for details on expression evaluation timing.
type StmtEmit struct {
	Range Range
}

func (StmtEmit) statementKind() {}

// StmtBlock contains a sequence of statements to be executed in order.
type StmtBlock struct {
	Block Block
}

func (StmtBlock) statementKind() {}

// StmtIf conditionally executes one of two blocks based on the condition value.
// Naga IR does not have phi instructions. To use values computed in accept or reject
// blocks after the If statement, store them in a LocalVariable.
type StmtIf struct {
	Condition ExpressionHandle // Must be a bool expression
	Accept    Block
	Reject    Block
}

func (StmtIf) statementKind() {}

// StmtSwitch conditionally executes one of multiple blocks based on the selector value.
// Each case must have a distinct value, and exactly one must be Default.
// The Default may appear at any position and covers all values not explicitly listed.
type StmtSwitch struct {
	Selector ExpressionHandle
	Cases    []SwitchCase
}

func (StmtSwitch) statementKind() {}

// SwitchCase represents a case in a switch statement.
type SwitchCase struct {
	Value       SwitchValue
	Body        Block
	FallThrough bool // If true, execution continues to next case
}

// SwitchValue represents the value that triggers a switch case.
type SwitchValue interface {
	switchValue()
}

// SwitchValueI32 represents a signed 32-bit integer switch value.
type SwitchValueI32 int32

func (SwitchValueI32) switchValue() {}

// SwitchValueU32 represents an unsigned 32-bit integer switch value.
type SwitchValueU32 uint32

func (SwitchValueU32) switchValue() {}

// SwitchValueDefault represents the default case in a switch statement.
type SwitchValueDefault struct{}

func (SwitchValueDefault) switchValue() {}

// StmtLoop executes a block repeatedly.
// Each iteration executes the Body block, followed by the Continuing block.
// The Continuing block is used for loop increment expressions (like C for-loop's third expression).
// Break, Return, or Kill statements exit the loop.
// Continue statements in Body jump to the Continuing block.
type StmtLoop struct {
	Body       Block
	Continuing Block
	BreakIf    *ExpressionHandle // Optional break-if expression evaluated after continuing
}

func (StmtLoop) statementKind() {}

// StmtBreak exits the innermost enclosing Loop or Switch statement.
// May not break out of a Loop from within its continuing block.
type StmtBreak struct{}

func (StmtBreak) statementKind() {}

// StmtContinue skips to the continuing block of the innermost enclosing Loop.
// May only appear within the body block of a Loop (not in the continuing block).
type StmtContinue struct{}

func (StmtContinue) statementKind() {}

// StmtReturn returns from the function, possibly with a value.
// Forbidden within the continuing block of a Loop statement.
type StmtReturn struct {
	Value *ExpressionHandle
}

func (StmtReturn) statementKind() {}

// StmtKill aborts the current shader execution (fragment shader discard).
// Forbidden within the continuing block of a Loop statement.
type StmtKill struct{}

func (StmtKill) statementKind() {}

// StmtBarrier synchronizes invocations within the work group.
// The Barrier flags control which memory accesses should be synchronized.
// If empty, this becomes purely an execution barrier.
type StmtBarrier struct {
	Flags BarrierFlags
}

func (StmtBarrier) statementKind() {}

// BarrierFlags represents memory barrier flags using bitflags pattern.
type BarrierFlags uint32

const (
	// BarrierStorage affects all Storage address space accesses.
	BarrierStorage BarrierFlags = 1 << 0
	// BarrierWorkGroup affects all WorkGroup address space accesses.
	BarrierWorkGroup BarrierFlags = 1 << 1
	// BarrierSubGroup synchronizes execution across invocations within a subgroup.
	BarrierSubGroup BarrierFlags = 1 << 2
	// BarrierTexture synchronizes texture memory accesses in a workgroup.
	BarrierTexture BarrierFlags = 1 << 3
)

// StmtStore stores a value at an address through a pointer.
// For Atomic types, the value must be a corresponding scalar.
// For other types behind pointer<T>, the value is T.
// This acts as a barrier for operations on the underlying variable.
type StmtStore struct {
	Pointer ExpressionHandle
	Value   ExpressionHandle
}

func (StmtStore) statementKind() {}

// StmtImageStore stores a texel value to an image.
// Storing into multisampled images or images with mipmaps is not supported.
// This acts as a barrier for operations on the image GlobalVariable.
type StmtImageStore struct {
	Image      ExpressionHandle
	Coordinate ExpressionHandle
	ArrayIndex *ExpressionHandle
	Value      ExpressionHandle
}

func (StmtImageStore) statementKind() {}

// StmtAtomic performs an atomic operation on a value.
// The pointer must point to an Atomic type with scalar type I32, U32, I64, U64, or F32.
// Support for I64/U64/F32 depends on enabled capabilities.
type StmtAtomic struct {
	Pointer ExpressionHandle
	Fun     AtomicFunction
	Value   ExpressionHandle
	Result  *ExpressionHandle // AtomicResult expression, required for some operations
}

func (StmtAtomic) statementKind() {}

// AtomicFunction represents atomic operations.
type AtomicFunction interface {
	atomicFunction()
}

// AtomicAdd performs atomic addition.
type AtomicAdd struct{}

func (AtomicAdd) atomicFunction() {}

// AtomicSubtract performs atomic subtraction.
type AtomicSubtract struct{}

func (AtomicSubtract) atomicFunction() {}

// AtomicAnd performs atomic bitwise AND.
type AtomicAnd struct{}

func (AtomicAnd) atomicFunction() {}

// AtomicExclusiveOr performs atomic bitwise XOR.
type AtomicExclusiveOr struct{}

func (AtomicExclusiveOr) atomicFunction() {}

// AtomicInclusiveOr performs atomic bitwise OR.
type AtomicInclusiveOr struct{}

func (AtomicInclusiveOr) atomicFunction() {}

// AtomicMin performs atomic minimum.
type AtomicMin struct{}

func (AtomicMin) atomicFunction() {}

// AtomicMax performs atomic maximum.
type AtomicMax struct{}

func (AtomicMax) atomicFunction() {}

// AtomicExchange performs atomic exchange.
// If Compare is set, performs compare-and-exchange operation.
type AtomicExchange struct {
	Compare *ExpressionHandle
}

func (AtomicExchange) atomicFunction() {}

// StmtWorkGroupUniformLoad loads uniformly from a uniform pointer in workgroup address space.
// Corresponds to WGSL workgroupUniformLoad built-in function with barrier semantics.
type StmtWorkGroupUniformLoad struct {
	Pointer ExpressionHandle // Must be Pointer in WorkGroup address space
	Result  ExpressionHandle // WorkGroupUniformLoadResult expression
}

func (StmtWorkGroupUniformLoad) statementKind() {}

// StmtCall calls a function.
// If Result is set, it must be a CallResult expression.
// The Call statement acts as a barrier for operations on the result expression.
type StmtCall struct {
	Function  FunctionHandle
	Arguments []ExpressionHandle
	Result    *ExpressionHandle
}

func (StmtCall) statementKind() {}

// StmtRayQuery performs a ray tracing query operation.
type StmtRayQuery struct {
	Query ExpressionHandle // Must be a RayQuery type
	Fun   RayQueryFunction
}

func (StmtRayQuery) statementKind() {}

// RayQueryFunction represents ray query operations.
type RayQueryFunction interface {
	rayQueryFunction()
}

// RayQueryInitialize initializes a RayQuery object.
type RayQueryInitialize struct {
	AccelerationStructure ExpressionHandle
	Descriptor            ExpressionHandle // Ray descriptor struct
}

func (RayQueryInitialize) rayQueryFunction() {}

// RayQueryProceed starts or continues a ray query.
// After execution, Result is a Bool indicating if there are more intersection candidates.
type RayQueryProceed struct {
	Result ExpressionHandle // RayQueryProceedResult expression
}

func (RayQueryProceed) rayQueryFunction() {}

// RayQueryTerminate terminates a ray query.
type RayQueryTerminate struct{}

func (RayQueryTerminate) rayQueryFunction() {}
