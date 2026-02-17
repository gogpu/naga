package spirv

import (
	"encoding/binary"
	"math"
)

// Instruction represents a SPIR-V instruction.
type Instruction struct {
	Opcode OpCode
	Words  []uint32 // result type ID, result ID, operands
}

// WordCount returns the total word count including the opcode word.
func (i Instruction) WordCount() int {
	return len(i.Words) + 1
}

// WriteTo writes the instruction directly to a byte buffer at the given offset.
// Returns the new offset after writing.
func (i Instruction) WriteTo(buffer []byte, offset int) int {
	wordCount := uint32(len(i.Words) + 1)
	binary.LittleEndian.PutUint32(buffer[offset:], (wordCount<<16)|uint32(i.Opcode))
	offset += 4
	for _, word := range i.Words {
		binary.LittleEndian.PutUint32(buffer[offset:], word)
		offset += 4
	}
	return offset
}

// InstructionBuilder builds SPIR-V instructions.
type InstructionBuilder struct {
	words []uint32
	arena *wordArena // if set, Build allocates from arena instead of heap
}

// NewInstructionBuilder creates a new instruction builder.
func NewInstructionBuilder() *InstructionBuilder {
	return &InstructionBuilder{
		words: make([]uint32, 0, 8),
	}
}

// Reset clears the builder for reuse without allocating.
func (b *InstructionBuilder) Reset() {
	b.words = b.words[:0]
}

// AddWord adds a word to the instruction.
func (b *InstructionBuilder) AddWord(word uint32) {
	b.words = append(b.words, word)
}

// AddString adds a null-terminated UTF-8 string.
func (b *InstructionBuilder) AddString(s string) {
	bytes := []byte(s)
	// Add null terminator if not present
	if len(bytes) == 0 || bytes[len(bytes)-1] != 0 {
		bytes = append(bytes, 0)
	}

	// Pad to word boundary
	for len(bytes)%4 != 0 {
		bytes = append(bytes, 0)
	}

	// Convert to words
	for i := 0; i < len(bytes); i += 4 {
		word := uint32(bytes[i]) |
			uint32(bytes[i+1])<<8 |
			uint32(bytes[i+2])<<16 |
			uint32(bytes[i+3])<<24
		b.words = append(b.words, word)
	}
}

// Build builds the instruction with the given opcode.
// It copies the words to a new slice so the builder can be safely reused.
// If the builder has an arena, words are allocated from the arena (amortized O(1)).
func (b *InstructionBuilder) Build(opcode OpCode) Instruction {
	var words []uint32
	if b.arena != nil {
		words = b.arena.alloc(len(b.words))
	} else {
		words = make([]uint32, len(b.words))
	}
	copy(words, b.words)
	return Instruction{
		Opcode: opcode,
		Words:  words,
	}
}

// Encode encodes the instruction to binary.
func (i Instruction) Encode() []uint32 {
	wordCount := uint32(len(i.Words) + 1) // +1 for opcode word
	result := make([]uint32, 0, wordCount)
	result = append(result, (wordCount<<16)|uint32(i.Opcode))
	result = append(result, i.Words...)
	return result
}

// wordArena is a bulk allocator for instruction word slices.
// Instead of allocating a separate []uint32 per instruction, we allocate
// from a single large backing array, reducing GC pressure.
type wordArena struct {
	buf []uint32
	pos int
}

// newWordArena creates an arena with the given initial capacity in words.
func newWordArena(capacity int) wordArena {
	return wordArena{
		buf: make([]uint32, capacity),
	}
}

// alloc returns a slice of n words from the arena.
// If the arena is full, it allocates a new backing array (doubling).
func (a *wordArena) alloc(n int) []uint32 {
	if a.pos+n > len(a.buf) {
		// Grow: at least double, or enough for this request
		newSize := len(a.buf) * 2
		if newSize < a.pos+n {
			newSize = a.pos + n
		}
		a.buf = make([]uint32, newSize)
		a.pos = 0
	}
	s := a.buf[a.pos : a.pos+n : a.pos+n]
	a.pos += n
	return s
}

// ModuleBuilder builds complete SPIR-V modules.
type ModuleBuilder struct {
	// Header
	version   Version
	generator uint32
	bound     uint32 // max ID + 1
	schema    uint32

	// Sections (ordered per SPIR-V spec)
	capabilities   []Instruction
	extensions     []Instruction
	extInstImports []Instruction
	memoryModel    *Instruction
	entryPoints    []Instruction
	executionModes []Instruction
	debugStrings   []Instruction // OpString
	debugNames     []Instruction // OpName, OpMemberName
	annotations    []Instruction // OpDecorate, OpMemberDecorate
	types          []Instruction // OpType*, OpConstant*
	globalVars     []Instruction // OpVariable (global)
	functions      []Instruction // OpFunction...OpFunctionEnd

	// ID allocation
	nextID uint32

	// Shared instruction builder to avoid per-instruction allocation.
	ib InstructionBuilder

	// Word arena for bulk allocation of instruction word slices.
	arena wordArena
}

// NewModuleBuilder creates a new SPIR-V module builder.
func NewModuleBuilder(version Version) *ModuleBuilder {
	mb := &ModuleBuilder{
		version:        version,
		generator:      GeneratorID,
		schema:         0,
		capabilities:   make([]Instruction, 0, 2),
		extensions:     make([]Instruction, 0),
		extInstImports: make([]Instruction, 0, 1),
		entryPoints:    make([]Instruction, 0, 2),
		executionModes: make([]Instruction, 0, 4),
		debugStrings:   make([]Instruction, 0),
		debugNames:     make([]Instruction, 0, 8),
		annotations:    make([]Instruction, 0, 16),
		types:          make([]Instruction, 0, 32),
		globalVars:     make([]Instruction, 0, 4),
		functions:      make([]Instruction, 0, 64),
		nextID:         1,
		arena:          newWordArena(2048), // pre-allocate ~2K words for all instructions
	}
	// Initialize shared instruction builder with arena reference.
	// The builder's scratch space (words) grows once to max needed size and is reused.
	// The arena provides O(1) bulk allocation for instruction word slices in Build().
	mb.ib = InstructionBuilder{
		words: make([]uint32, 0, 16),
		arena: &mb.arena,
	}
	return mb
}

// AllocID allocates a new SPIR-V ID.
func (b *ModuleBuilder) AllocID() uint32 {
	id := b.nextID
	b.nextID++
	return id
}

// AddCapability adds a capability.
func (b *ModuleBuilder) AddCapability(capability Capability) {
	b.ib.Reset()
	b.ib.AddWord(uint32(capability))
	b.capabilities = append(b.capabilities, b.ib.Build(OpCapability))
}

// AddExtension adds an extension.
func (b *ModuleBuilder) AddExtension(name string) {
	b.ib.Reset()
	b.ib.AddString(name)
	b.extensions = append(b.extensions, b.ib.Build(OpExtension))
}

// AddExtInstImport imports an extended instruction set.
func (b *ModuleBuilder) AddExtInstImport(name string) uint32 {
	id := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(id)
	b.ib.AddString(name)
	b.extInstImports = append(b.extInstImports, b.ib.Build(OpExtInstImport))
	return id
}

// SetMemoryModel sets the memory model.
func (b *ModuleBuilder) SetMemoryModel(addressing AddressingModel, memory MemoryModel) {
	b.ib.Reset()
	b.ib.AddWord(uint32(addressing))
	b.ib.AddWord(uint32(memory))
	inst := b.ib.Build(OpMemoryModel)
	b.memoryModel = &inst
}

// AddEntryPoint adds an entry point.
func (b *ModuleBuilder) AddEntryPoint(execModel ExecutionModel, funcID uint32, name string, interfaces []uint32) {
	b.ib.Reset()
	b.ib.AddWord(uint32(execModel))
	b.ib.AddWord(funcID)
	b.ib.AddString(name)
	for _, iface := range interfaces {
		b.ib.AddWord(iface)
	}
	b.entryPoints = append(b.entryPoints, b.ib.Build(OpEntryPoint))
}

// AddExecutionMode adds an execution mode.
func (b *ModuleBuilder) AddExecutionMode(entryPoint uint32, mode ExecutionMode, params ...uint32) {
	b.ib.Reset()
	b.ib.AddWord(entryPoint)
	b.ib.AddWord(uint32(mode))
	for _, param := range params {
		b.ib.AddWord(param)
	}
	b.executionModes = append(b.executionModes, b.ib.Build(OpExecutionMode))
}

// AddString adds a debug string.
func (b *ModuleBuilder) AddString(text string) uint32 {
	id := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(id)
	b.ib.AddString(text)
	b.debugStrings = append(b.debugStrings, b.ib.Build(OpString))
	return id
}

// AddName adds a debug name.
func (b *ModuleBuilder) AddName(id uint32, name string) {
	b.ib.Reset()
	b.ib.AddWord(id)
	b.ib.AddString(name)
	b.debugNames = append(b.debugNames, b.ib.Build(OpName))
}

// AddMemberName adds a debug member name.
func (b *ModuleBuilder) AddMemberName(structID, member uint32, name string) {
	b.ib.Reset()
	b.ib.AddWord(structID)
	b.ib.AddWord(member)
	b.ib.AddString(name)
	b.debugNames = append(b.debugNames, b.ib.Build(OpMemberName))
}

// AddDecorate adds a decoration.
func (b *ModuleBuilder) AddDecorate(id uint32, decoration Decoration, params ...uint32) {
	b.ib.Reset()
	b.ib.AddWord(id)
	b.ib.AddWord(uint32(decoration))
	for _, param := range params {
		b.ib.AddWord(param)
	}
	b.annotations = append(b.annotations, b.ib.Build(OpDecorate))
}

// AddMemberDecorate adds a member decoration.
func (b *ModuleBuilder) AddMemberDecorate(structID, member uint32, decoration Decoration, params ...uint32) {
	b.ib.Reset()
	b.ib.AddWord(structID)
	b.ib.AddWord(member)
	b.ib.AddWord(uint32(decoration))
	for _, param := range params {
		b.ib.AddWord(param)
	}
	b.annotations = append(b.annotations, b.ib.Build(OpMemberDecorate))
}

// AddTypeVoid adds OpTypeVoid.
func (b *ModuleBuilder) AddTypeVoid() uint32 {
	id := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(id)
	b.types = append(b.types, b.ib.Build(OpTypeVoid))
	return id
}

// AddTypeBool adds OpTypeBool.
func (b *ModuleBuilder) AddTypeBool() uint32 {
	id := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(id)
	b.types = append(b.types, b.ib.Build(OpTypeBool))
	return id
}

// AddTypeSampler adds OpTypeSampler.
func (b *ModuleBuilder) AddTypeSampler() uint32 {
	id := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(id)
	b.types = append(b.types, b.ib.Build(OpTypeSampler))
	return id
}

// AddTypeFloat adds OpTypeFloat.
func (b *ModuleBuilder) AddTypeFloat(width uint32) uint32 {
	id := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(id)
	b.ib.AddWord(width)
	b.types = append(b.types, b.ib.Build(OpTypeFloat))
	return id
}

// AddTypeInt adds OpTypeInt.
func (b *ModuleBuilder) AddTypeInt(width uint32, signed bool) uint32 {
	id := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(id)
	b.ib.AddWord(width)
	if signed {
		b.ib.AddWord(1)
	} else {
		b.ib.AddWord(0)
	}
	b.types = append(b.types, b.ib.Build(OpTypeInt))
	return id
}

// AddTypeVector adds OpTypeVector.
func (b *ModuleBuilder) AddTypeVector(componentType uint32, count uint32) uint32 {
	id := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(id)
	b.ib.AddWord(componentType)
	b.ib.AddWord(count)
	b.types = append(b.types, b.ib.Build(OpTypeVector))
	return id
}

// AddTypeMatrix adds OpTypeMatrix.
func (b *ModuleBuilder) AddTypeMatrix(columnType uint32, columnCount uint32) uint32 {
	id := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(id)
	b.ib.AddWord(columnType)
	b.ib.AddWord(columnCount)
	b.types = append(b.types, b.ib.Build(OpTypeMatrix))
	return id
}

// AddTypeArray adds OpTypeArray.
func (b *ModuleBuilder) AddTypeArray(elementType uint32, length uint32) uint32 {
	id := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(id)
	b.ib.AddWord(elementType)
	b.ib.AddWord(length) // length is a constant ID
	b.types = append(b.types, b.ib.Build(OpTypeArray))
	return id
}

// AddTypeRuntimeArray adds OpTypeRuntimeArray for storage buffer runtime-sized arrays.
func (b *ModuleBuilder) AddTypeRuntimeArray(elementType uint32) uint32 {
	id := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(id)
	b.ib.AddWord(elementType)
	b.types = append(b.types, b.ib.Build(OpTypeRuntimeArray))
	return id
}

// AddTypePointer adds OpTypePointer.
func (b *ModuleBuilder) AddTypePointer(storageClass StorageClass, baseType uint32) uint32 {
	id := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(id)
	b.ib.AddWord(uint32(storageClass))
	b.ib.AddWord(baseType)
	b.types = append(b.types, b.ib.Build(OpTypePointer))
	return id
}

// AddTypeFunction adds OpTypeFunction.
func (b *ModuleBuilder) AddTypeFunction(returnType uint32, paramTypes ...uint32) uint32 {
	id := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(id)
	b.ib.AddWord(returnType)
	for _, paramType := range paramTypes {
		b.ib.AddWord(paramType)
	}
	b.types = append(b.types, b.ib.Build(OpTypeFunction))
	return id
}

// AddTypeStruct adds OpTypeStruct.
func (b *ModuleBuilder) AddTypeStruct(memberTypes ...uint32) uint32 {
	id := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(id)
	for _, memberType := range memberTypes {
		b.ib.AddWord(memberType)
	}
	b.types = append(b.types, b.ib.Build(OpTypeStruct))
	return id
}

// AddConstant adds OpConstant.
func (b *ModuleBuilder) AddConstant(typeID uint32, values ...uint32) uint32 {
	id := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(typeID)
	b.ib.AddWord(id)
	for _, value := range values {
		b.ib.AddWord(value)
	}
	b.types = append(b.types, b.ib.Build(OpConstant))
	return id
}

// AddConstantFloat32 adds a 32-bit float constant.
func (b *ModuleBuilder) AddConstantFloat32(typeID uint32, value float32) uint32 {
	bits := math.Float32bits(value)
	return b.AddConstant(typeID, bits)
}

// AddConstantNull adds OpConstantNull for a given type (all zeros/false/null).
func (b *ModuleBuilder) AddConstantNull(typeID uint32) uint32 {
	id := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(typeID)
	b.ib.AddWord(id)
	b.types = append(b.types, b.ib.Build(OpConstantNull))
	return id
}

// AddConstantFloat64 adds a 64-bit float constant.
func (b *ModuleBuilder) AddConstantFloat64(typeID uint32, value float64) uint32 {
	bits := math.Float64bits(value)
	lowBits := uint32(bits & 0xFFFFFFFF)
	highBits := uint32(bits >> 32)
	return b.AddConstant(typeID, lowBits, highBits)
}

// AddConstantComposite adds OpConstantComposite.
func (b *ModuleBuilder) AddConstantComposite(typeID uint32, constituents ...uint32) uint32 {
	id := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(typeID)
	b.ib.AddWord(id)
	for _, constituent := range constituents {
		b.ib.AddWord(constituent)
	}
	b.types = append(b.types, b.ib.Build(OpConstantComposite))
	return id
}

// AddVariable adds OpVariable.
func (b *ModuleBuilder) AddVariable(pointerType uint32, storageClass StorageClass) uint32 {
	id := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(pointerType)
	b.ib.AddWord(id)
	b.ib.AddWord(uint32(storageClass))
	b.globalVars = append(b.globalVars, b.ib.Build(OpVariable))
	return id
}

// AddVariableWithInit adds OpVariable with initializer.
func (b *ModuleBuilder) AddVariableWithInit(pointerType uint32, storageClass StorageClass, initID uint32) uint32 {
	id := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(pointerType)
	b.ib.AddWord(id)
	b.ib.AddWord(uint32(storageClass))
	b.ib.AddWord(initID)
	b.globalVars = append(b.globalVars, b.ib.Build(OpVariable))
	return id
}

// AddFunction adds a function definition.
func (b *ModuleBuilder) AddFunction(funcType uint32, returnType uint32, control FunctionControl) uint32 {
	id := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(returnType)
	b.ib.AddWord(id)
	b.ib.AddWord(uint32(control))
	b.ib.AddWord(funcType)
	b.functions = append(b.functions, b.ib.Build(OpFunction))
	return id
}

// AddFunctionParameter adds a function parameter.
func (b *ModuleBuilder) AddFunctionParameter(typeID uint32) uint32 {
	id := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(typeID)
	b.ib.AddWord(id)
	b.functions = append(b.functions, b.ib.Build(OpFunctionParameter))
	return id
}

// AddLabel adds a label.
func (b *ModuleBuilder) AddLabel() uint32 {
	id := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(id)
	b.functions = append(b.functions, b.ib.Build(OpLabel))
	return id
}

// AddLabelWithID adds a label with a pre-allocated ID.
func (b *ModuleBuilder) AddLabelWithID(id uint32) {
	b.ib.Reset()
	b.ib.AddWord(id)
	b.functions = append(b.functions, b.ib.Build(OpLabel))
}

// AddReturn adds OpReturn.
func (b *ModuleBuilder) AddReturn() {
	b.ib.Reset()
	b.functions = append(b.functions, b.ib.Build(OpReturn))
}

// AddUnreachable adds OpUnreachable (terminator for unreachable basic blocks).
func (b *ModuleBuilder) AddUnreachable() {
	b.ib.Reset()
	b.functions = append(b.functions, b.ib.Build(OpUnreachable))
}

// AddReturnValue adds OpReturnValue.
func (b *ModuleBuilder) AddReturnValue(valueID uint32) {
	b.ib.Reset()
	b.ib.AddWord(valueID)
	b.functions = append(b.functions, b.ib.Build(OpReturnValue))
}

// AddFunctionEnd adds OpFunctionEnd.
func (b *ModuleBuilder) AddFunctionEnd() {
	b.ib.Reset()
	b.functions = append(b.functions, b.ib.Build(OpFunctionEnd))
}

// AddBinaryOp adds a binary operation instruction.
func (b *ModuleBuilder) AddBinaryOp(opcode OpCode, resultType uint32, left uint32, right uint32) uint32 {
	resultID := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(resultType)
	b.ib.AddWord(resultID)
	b.ib.AddWord(left)
	b.ib.AddWord(right)
	b.functions = append(b.functions, b.ib.Build(opcode))
	return resultID
}

// AddUnaryOp adds a unary operation instruction.
func (b *ModuleBuilder) AddUnaryOp(opcode OpCode, resultType uint32, operand uint32) uint32 {
	resultID := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(resultType)
	b.ib.AddWord(resultID)
	b.ib.AddWord(operand)
	b.functions = append(b.functions, b.ib.Build(opcode))
	return resultID
}

// AddLoad adds OpLoad.
func (b *ModuleBuilder) AddLoad(resultType uint32, pointer uint32) uint32 {
	resultID := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(resultType)
	b.ib.AddWord(resultID)
	b.ib.AddWord(pointer)
	b.functions = append(b.functions, b.ib.Build(OpLoad))
	return resultID
}

// AddStore adds OpStore.
func (b *ModuleBuilder) AddStore(pointer uint32, value uint32) {
	b.ib.Reset()
	b.ib.AddWord(pointer)
	b.ib.AddWord(value)
	b.functions = append(b.functions, b.ib.Build(OpStore))
}

// AddAccessChain adds OpAccessChain.
func (b *ModuleBuilder) AddAccessChain(resultType uint32, base uint32, indices ...uint32) uint32 {
	resultID := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(resultType)
	b.ib.AddWord(resultID)
	b.ib.AddWord(base)
	for _, index := range indices {
		b.ib.AddWord(index)
	}
	b.functions = append(b.functions, b.ib.Build(OpAccessChain))
	return resultID
}

// AddCompositeConstruct adds OpCompositeConstruct.
func (b *ModuleBuilder) AddCompositeConstruct(resultType uint32, constituents ...uint32) uint32 {
	resultID := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(resultType)
	b.ib.AddWord(resultID)
	for _, constituent := range constituents {
		b.ib.AddWord(constituent)
	}
	b.functions = append(b.functions, b.ib.Build(OpCompositeConstruct))
	return resultID
}

// AddCompositeExtract adds OpCompositeExtract to extract a member from a composite value.
// Unlike OpAccessChain, this operates on values (not pointers) and indexes are literals.
func (b *ModuleBuilder) AddCompositeExtract(resultType uint32, composite uint32, indexes ...uint32) uint32 {
	resultID := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(resultType)
	b.ib.AddWord(resultID)
	b.ib.AddWord(composite)
	for _, idx := range indexes {
		b.ib.AddWord(idx)
	}
	b.functions = append(b.functions, b.ib.Build(OpCompositeExtract))
	return resultID
}

// AddVectorExtractDynamic adds OpVectorExtractDynamic to extract an element from a vector
// using a dynamic (runtime) index. Unlike OpCompositeExtract, the index is an ID not a literal.
func (b *ModuleBuilder) AddVectorExtractDynamic(resultType uint32, vector uint32, index uint32) uint32 {
	resultID := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(resultType)
	b.ib.AddWord(resultID)
	b.ib.AddWord(vector)
	b.ib.AddWord(index)
	b.functions = append(b.functions, b.ib.Build(OpVectorExtractDynamic))
	return resultID
}

// AddVectorShuffle adds OpVectorShuffle for vector swizzle operations.
func (b *ModuleBuilder) AddVectorShuffle(resultType uint32, vec1 uint32, vec2 uint32, components []uint32) uint32 {
	resultID := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(resultType)
	b.ib.AddWord(resultID)
	b.ib.AddWord(vec1)
	b.ib.AddWord(vec2)
	for _, component := range components {
		b.ib.AddWord(component)
	}
	b.functions = append(b.functions, b.ib.Build(OpVectorShuffle))
	return resultID
}

// AddSelect adds OpSelect.
func (b *ModuleBuilder) AddSelect(resultType uint32, condition uint32, accept uint32, reject uint32) uint32 {
	resultID := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(resultType)
	b.ib.AddWord(resultID)
	b.ib.AddWord(condition)
	b.ib.AddWord(accept)
	b.ib.AddWord(reject)
	b.functions = append(b.functions, b.ib.Build(OpSelect))
	return resultID
}

// AddSelectionMerge adds OpSelectionMerge.
func (b *ModuleBuilder) AddSelectionMerge(mergeLabel uint32, control SelectionControl) {
	b.ib.Reset()
	b.ib.AddWord(mergeLabel)
	b.ib.AddWord(uint32(control))
	b.functions = append(b.functions, b.ib.Build(OpSelectionMerge))
}

// AddLoopMerge adds OpLoopMerge.
func (b *ModuleBuilder) AddLoopMerge(mergeLabel uint32, continueLabel uint32, control LoopControl) {
	b.ib.Reset()
	b.ib.AddWord(mergeLabel)
	b.ib.AddWord(continueLabel)
	b.ib.AddWord(uint32(control))
	b.functions = append(b.functions, b.ib.Build(OpLoopMerge))
}

// AddBranchConditional adds OpBranchConditional.
func (b *ModuleBuilder) AddBranchConditional(condition uint32, trueLabel uint32, falseLabel uint32) {
	b.ib.Reset()
	b.ib.AddWord(condition)
	b.ib.AddWord(trueLabel)
	b.ib.AddWord(falseLabel)
	b.functions = append(b.functions, b.ib.Build(OpBranchConditional))
}

// AddKill adds OpKill (fragment shader discard).
func (b *ModuleBuilder) AddKill() {
	b.ib.Reset()
	b.functions = append(b.functions, b.ib.Build(OpKill))
}

// AddExtInst adds OpExtInst (extended instruction).
func (b *ModuleBuilder) AddExtInst(resultType uint32, extSet uint32, instruction uint32, operands ...uint32) uint32 {
	resultID := b.AllocID()
	b.ib.Reset()
	b.ib.AddWord(resultType)
	b.ib.AddWord(resultID)
	b.ib.AddWord(extSet)
	b.ib.AddWord(instruction)
	for _, operand := range operands {
		b.ib.AddWord(operand)
	}
	b.functions = append(b.functions, b.ib.Build(OpExtInst))
	return resultID
}

// Build generates the final SPIR-V binary.
func (b *ModuleBuilder) Build() []byte {
	// Update bound to max ID
	b.bound = b.nextID

	// Calculate total size
	totalWords := 5 // header
	totalWords += countWords(b.capabilities)
	totalWords += countWords(b.extensions)
	totalWords += countWords(b.extInstImports)
	if b.memoryModel != nil {
		totalWords += b.memoryModel.WordCount()
	}
	totalWords += countWords(b.entryPoints)
	totalWords += countWords(b.executionModes)
	totalWords += countWords(b.debugStrings)
	totalWords += countWords(b.debugNames)
	totalWords += countWords(b.annotations)
	totalWords += countWords(b.types)
	totalWords += countWords(b.globalVars)
	totalWords += countWords(b.functions)

	// Allocate buffer
	buffer := make([]byte, totalWords*4)
	offset := 0

	// Write header
	binary.LittleEndian.PutUint32(buffer[offset:], MagicNumber)
	offset += 4
	binary.LittleEndian.PutUint32(buffer[offset:], versionToWord(b.version))
	offset += 4
	binary.LittleEndian.PutUint32(buffer[offset:], b.generator)
	offset += 4
	binary.LittleEndian.PutUint32(buffer[offset:], b.bound)
	offset += 4
	binary.LittleEndian.PutUint32(buffer[offset:], b.schema)
	offset += 4

	// Write sections in order
	offset = writeInstructions(buffer, offset, b.capabilities)
	offset = writeInstructions(buffer, offset, b.extensions)
	offset = writeInstructions(buffer, offset, b.extInstImports)
	if b.memoryModel != nil {
		offset = writeInstruction(buffer, offset, *b.memoryModel)
	}
	offset = writeInstructions(buffer, offset, b.entryPoints)
	offset = writeInstructions(buffer, offset, b.executionModes)
	offset = writeInstructions(buffer, offset, b.debugStrings)
	offset = writeInstructions(buffer, offset, b.debugNames)
	offset = writeInstructions(buffer, offset, b.annotations)
	offset = writeInstructions(buffer, offset, b.types)
	offset = writeInstructions(buffer, offset, b.globalVars)
	_ = writeInstructions(buffer, offset, b.functions)

	return buffer
}

// countWords counts total words in instructions.
func countWords(instructions []Instruction) int {
	count := 0
	for _, inst := range instructions {
		count += inst.WordCount()
	}
	return count
}

// writeInstructions writes instructions to buffer.
func writeInstructions(buffer []byte, offset int, instructions []Instruction) int {
	for _, inst := range instructions {
		offset = writeInstruction(buffer, offset, inst)
	}
	return offset
}

// writeInstruction writes a single instruction to buffer.
func writeInstruction(buffer []byte, offset int, inst Instruction) int {
	return inst.WriteTo(buffer, offset)
}

// versionToWord converts Version to SPIR-V word format.
func versionToWord(v Version) uint32 {
	return (uint32(v.Major) << 16) | (uint32(v.Minor) << 8)
}
