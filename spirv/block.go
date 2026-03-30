package spirv

// Block represents a SPIR-V basic block under construction.
// Instructions are appended via Push; the block is not yet terminated.
type Block struct {
	LabelID uint32
	Body    []Instruction
}

// NewBlock creates a new block with the given label ID and an empty body.
func NewBlock(labelID uint32) Block {
	return Block{LabelID: labelID}
}

// Push appends an instruction to the block body.
func (b *Block) Push(inst Instruction) {
	b.Body = append(b.Body, inst)
}

// TerminatedBlock is a finalized basic block whose Body includes the
// terminator as its last instruction.
type TerminatedBlock struct {
	LabelID uint32
	Body    []Instruction // includes terminator as last element
}

// FunctionBuilder collects terminated blocks for a single SPIR-V function.
// It mirrors the Rust naga Function struct: blocks own their instructions,
// and ToInstructions serializes them into a flat list.
type FunctionBuilder struct {
	Blocks     []TerminatedBlock
	Variables  []Instruction // OpVariable instructions emitted in the first block only
	Signature  Instruction   // OpFunction instruction
	Parameters []Instruction // OpFunctionParameter instructions
}

// Consume finalizes a block by appending the given terminator instruction,
// then adds the resulting TerminatedBlock to the function.
func (f *FunctionBuilder) Consume(block Block, terminator Instruction) {
	block.Body = append(block.Body, terminator)
	f.Blocks = append(f.Blocks, TerminatedBlock(block))
}

// ToInstructions serializes all blocks into a flat instruction list suitable
// for SPIR-V binary encoding. OpLabel is emitted from block.LabelID (NOT
// stored in Body). Local variables are emitted only in the first block.
// This matches Rust naga's Function::to_words() layout:
//
//	OpFunction
//	OpFunctionParameter...
//	OpLabel (block 0)
//	OpVariable... (locals)
//	body instructions...
//	OpLabel (block 1)
//	body instructions...
//	...
//	OpFunctionEnd
func (f *FunctionBuilder) ToInstructions() []Instruction {
	// Pre-calculate capacity: signature + params + per-block(label+body) + variables + functionEnd.
	capacity := 1 + len(f.Parameters) + len(f.Variables) + 1 // signature + params + vars + funcEnd
	for _, block := range f.Blocks {
		capacity += 1 + len(block.Body) // label + body
	}

	result := make([]Instruction, 0, capacity)
	result = append(result, f.Signature)
	result = append(result, f.Parameters...)

	for i, block := range f.Blocks {
		result = append(result, makeLabelInstruction(block.LabelID))
		if i == 0 {
			result = append(result, f.Variables...)
		}
		result = append(result, block.Body...)
	}

	result = append(result, makeFunctionEndInstruction())
	return result
}

// makeLabelInstruction creates an OpLabel instruction with the given ID.
// Unlike ModuleBuilder.AddLabelWithID, this returns the instruction without
// appending it to any list.
func makeLabelInstruction(labelID uint32) Instruction {
	return Instruction{
		Opcode: OpLabel,
		Words:  []uint32{labelID},
	}
}

// makeFunctionEndInstruction creates an OpFunctionEnd instruction.
// Unlike ModuleBuilder.AddFunctionEnd, this returns the instruction without
// appending it to any list.
func makeFunctionEndInstruction() Instruction {
	return Instruction{
		Opcode: OpFunctionEnd,
		Words:  nil,
	}
}

// BlockExitKind specifies how a block should be terminated.
type BlockExitKind int

const (
	// BlockExitReturn terminates with OpReturn or OpReturnValue.
	BlockExitReturn BlockExitKind = iota
	// BlockExitBranch terminates with OpBranch to a target label.
	BlockExitBranch
	// BlockExitBreakIf terminates with OpBranchConditional (break-if pattern).
	BlockExitBreakIf
)

// BlockExit specifies how a block should end.
type BlockExit struct {
	Kind       BlockExitKind
	Target     uint32 // branch target label ID (for BlockExitBranch)
	Condition  uint32 // SPIR-V ID of the boolean condition (for BlockExitBreakIf)
	PreambleID uint32 // branch-back target (for BlockExitBreakIf)
}

// BlockExitDisposition indicates whether writeBlock consumed the provided exit.
type BlockExitDisposition int

const (
	// ExitUsed means the exit was applied normally.
	ExitUsed BlockExitDisposition = iota
	// ExitDiscarded means the block ended early (break/continue/return/kill)
	// and the provided exit was not emitted.
	ExitDiscarded
)

// LoopContext provides break/continue targets for loop bodies.
// Passed by VALUE to ensure nested loops get isolated contexts,
// matching Rust's Copy semantics for the equivalent struct.
type LoopContext struct {
	ContinuingID uint32 // 0 = not in a continuing block
	BreakID      uint32 // 0 = not in a loop/switch
}
