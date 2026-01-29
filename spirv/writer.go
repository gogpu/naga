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

// InstructionBuilder builds SPIR-V instructions.
type InstructionBuilder struct {
	words []uint32
}

// NewInstructionBuilder creates a new instruction builder.
func NewInstructionBuilder() *InstructionBuilder {
	return &InstructionBuilder{
		words: make([]uint32, 0, 8),
	}
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
func (b *InstructionBuilder) Build(opcode OpCode) Instruction {
	return Instruction{
		Opcode: opcode,
		Words:  b.words,
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
}

// NewModuleBuilder creates a new SPIR-V module builder.
func NewModuleBuilder(version Version) *ModuleBuilder {
	return &ModuleBuilder{
		version:        version,
		generator:      GeneratorID,
		schema:         0,
		capabilities:   make([]Instruction, 0),
		extensions:     make([]Instruction, 0),
		extInstImports: make([]Instruction, 0),
		entryPoints:    make([]Instruction, 0),
		executionModes: make([]Instruction, 0),
		debugStrings:   make([]Instruction, 0),
		debugNames:     make([]Instruction, 0),
		annotations:    make([]Instruction, 0),
		types:          make([]Instruction, 0),
		globalVars:     make([]Instruction, 0),
		functions:      make([]Instruction, 0),
		nextID:         1,
	}
}

// AllocID allocates a new SPIR-V ID.
func (b *ModuleBuilder) AllocID() uint32 {
	id := b.nextID
	b.nextID++
	return id
}

// AddCapability adds a capability.
func (b *ModuleBuilder) AddCapability(capability Capability) {
	builder := NewInstructionBuilder()
	builder.AddWord(uint32(capability))
	b.capabilities = append(b.capabilities, builder.Build(OpCapability))
}

// AddExtension adds an extension.
func (b *ModuleBuilder) AddExtension(name string) {
	builder := NewInstructionBuilder()
	builder.AddString(name)
	b.extensions = append(b.extensions, builder.Build(OpExtension))
}

// AddExtInstImport imports an extended instruction set.
func (b *ModuleBuilder) AddExtInstImport(name string) uint32 {
	id := b.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(id)
	builder.AddString(name)
	b.extInstImports = append(b.extInstImports, builder.Build(OpExtInstImport))
	return id
}

// SetMemoryModel sets the memory model.
func (b *ModuleBuilder) SetMemoryModel(addressing AddressingModel, memory MemoryModel) {
	builder := NewInstructionBuilder()
	builder.AddWord(uint32(addressing))
	builder.AddWord(uint32(memory))
	inst := builder.Build(OpMemoryModel)
	b.memoryModel = &inst
}

// AddEntryPoint adds an entry point.
func (b *ModuleBuilder) AddEntryPoint(execModel ExecutionModel, funcID uint32, name string, interfaces []uint32) {
	builder := NewInstructionBuilder()
	builder.AddWord(uint32(execModel))
	builder.AddWord(funcID)
	builder.AddString(name)
	for _, iface := range interfaces {
		builder.AddWord(iface)
	}
	b.entryPoints = append(b.entryPoints, builder.Build(OpEntryPoint))
}

// AddExecutionMode adds an execution mode.
func (b *ModuleBuilder) AddExecutionMode(entryPoint uint32, mode ExecutionMode, params ...uint32) {
	builder := NewInstructionBuilder()
	builder.AddWord(entryPoint)
	builder.AddWord(uint32(mode))
	for _, param := range params {
		builder.AddWord(param)
	}
	b.executionModes = append(b.executionModes, builder.Build(OpExecutionMode))
}

// AddString adds a debug string.
func (b *ModuleBuilder) AddString(text string) uint32 {
	id := b.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(id)
	builder.AddString(text)
	b.debugStrings = append(b.debugStrings, builder.Build(OpString))
	return id
}

// AddName adds a debug name.
func (b *ModuleBuilder) AddName(id uint32, name string) {
	builder := NewInstructionBuilder()
	builder.AddWord(id)
	builder.AddString(name)
	b.debugNames = append(b.debugNames, builder.Build(OpName))
}

// AddMemberName adds a debug member name.
func (b *ModuleBuilder) AddMemberName(structID, member uint32, name string) {
	builder := NewInstructionBuilder()
	builder.AddWord(structID)
	builder.AddWord(member)
	builder.AddString(name)
	b.debugNames = append(b.debugNames, builder.Build(OpMemberName))
}

// AddDecorate adds a decoration.
func (b *ModuleBuilder) AddDecorate(id uint32, decoration Decoration, params ...uint32) {
	builder := NewInstructionBuilder()
	builder.AddWord(id)
	builder.AddWord(uint32(decoration))
	for _, param := range params {
		builder.AddWord(param)
	}
	b.annotations = append(b.annotations, builder.Build(OpDecorate))
}

// AddMemberDecorate adds a member decoration.
func (b *ModuleBuilder) AddMemberDecorate(structID, member uint32, decoration Decoration, params ...uint32) {
	builder := NewInstructionBuilder()
	builder.AddWord(structID)
	builder.AddWord(member)
	builder.AddWord(uint32(decoration))
	for _, param := range params {
		builder.AddWord(param)
	}
	b.annotations = append(b.annotations, builder.Build(OpMemberDecorate))
}

// AddTypeVoid adds OpTypeVoid.
func (b *ModuleBuilder) AddTypeVoid() uint32 {
	id := b.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(id)
	b.types = append(b.types, builder.Build(OpTypeVoid))
	return id
}

// AddTypeBool adds OpTypeBool.
func (b *ModuleBuilder) AddTypeBool() uint32 {
	id := b.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(id)
	b.types = append(b.types, builder.Build(OpTypeBool))
	return id
}

// AddTypeFloat adds OpTypeFloat.
func (b *ModuleBuilder) AddTypeFloat(width uint32) uint32 {
	id := b.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(id)
	builder.AddWord(width)
	b.types = append(b.types, builder.Build(OpTypeFloat))
	return id
}

// AddTypeInt adds OpTypeInt.
func (b *ModuleBuilder) AddTypeInt(width uint32, signed bool) uint32 {
	id := b.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(id)
	builder.AddWord(width)
	if signed {
		builder.AddWord(1)
	} else {
		builder.AddWord(0)
	}
	b.types = append(b.types, builder.Build(OpTypeInt))
	return id
}

// AddTypeVector adds OpTypeVector.
func (b *ModuleBuilder) AddTypeVector(componentType uint32, count uint32) uint32 {
	id := b.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(id)
	builder.AddWord(componentType)
	builder.AddWord(count)
	b.types = append(b.types, builder.Build(OpTypeVector))
	return id
}

// AddTypeMatrix adds OpTypeMatrix.
func (b *ModuleBuilder) AddTypeMatrix(columnType uint32, columnCount uint32) uint32 {
	id := b.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(id)
	builder.AddWord(columnType)
	builder.AddWord(columnCount)
	b.types = append(b.types, builder.Build(OpTypeMatrix))
	return id
}

// AddTypeArray adds OpTypeArray.
func (b *ModuleBuilder) AddTypeArray(elementType uint32, length uint32) uint32 {
	id := b.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(id)
	builder.AddWord(elementType)
	builder.AddWord(length) // length is a constant ID
	b.types = append(b.types, builder.Build(OpTypeArray))
	return id
}

// AddTypePointer adds OpTypePointer.
func (b *ModuleBuilder) AddTypePointer(storageClass StorageClass, baseType uint32) uint32 {
	id := b.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(id)
	builder.AddWord(uint32(storageClass))
	builder.AddWord(baseType)
	b.types = append(b.types, builder.Build(OpTypePointer))
	return id
}

// AddTypeFunction adds OpTypeFunction.
func (b *ModuleBuilder) AddTypeFunction(returnType uint32, paramTypes ...uint32) uint32 {
	id := b.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(id)
	builder.AddWord(returnType)
	for _, paramType := range paramTypes {
		builder.AddWord(paramType)
	}
	b.types = append(b.types, builder.Build(OpTypeFunction))
	return id
}

// AddTypeStruct adds OpTypeStruct.
func (b *ModuleBuilder) AddTypeStruct(memberTypes ...uint32) uint32 {
	id := b.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(id)
	for _, memberType := range memberTypes {
		builder.AddWord(memberType)
	}
	b.types = append(b.types, builder.Build(OpTypeStruct))
	return id
}

// AddConstant adds OpConstant.
func (b *ModuleBuilder) AddConstant(typeID uint32, values ...uint32) uint32 {
	id := b.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(typeID)
	builder.AddWord(id)
	for _, value := range values {
		builder.AddWord(value)
	}
	b.types = append(b.types, builder.Build(OpConstant))
	return id
}

// AddConstantFloat32 adds a 32-bit float constant.
func (b *ModuleBuilder) AddConstantFloat32(typeID uint32, value float32) uint32 {
	bits := math.Float32bits(value)
	return b.AddConstant(typeID, bits)
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
	builder := NewInstructionBuilder()
	builder.AddWord(typeID)
	builder.AddWord(id)
	for _, constituent := range constituents {
		builder.AddWord(constituent)
	}
	b.types = append(b.types, builder.Build(OpConstantComposite))
	return id
}

// AddVariable adds OpVariable.
func (b *ModuleBuilder) AddVariable(pointerType uint32, storageClass StorageClass) uint32 {
	id := b.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(pointerType)
	builder.AddWord(id)
	builder.AddWord(uint32(storageClass))
	b.globalVars = append(b.globalVars, builder.Build(OpVariable))
	return id
}

// AddVariableWithInit adds OpVariable with initializer.
func (b *ModuleBuilder) AddVariableWithInit(pointerType uint32, storageClass StorageClass, initID uint32) uint32 {
	id := b.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(pointerType)
	builder.AddWord(id)
	builder.AddWord(uint32(storageClass))
	builder.AddWord(initID)
	b.globalVars = append(b.globalVars, builder.Build(OpVariable))
	return id
}

// AddFunction adds a function definition.
func (b *ModuleBuilder) AddFunction(funcType uint32, returnType uint32, control FunctionControl) uint32 {
	id := b.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(returnType)
	builder.AddWord(id)
	builder.AddWord(uint32(control))
	builder.AddWord(funcType)
	b.functions = append(b.functions, builder.Build(OpFunction))
	return id
}

// AddFunctionParameter adds a function parameter.
func (b *ModuleBuilder) AddFunctionParameter(typeID uint32) uint32 {
	id := b.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(typeID)
	builder.AddWord(id)
	b.functions = append(b.functions, builder.Build(OpFunctionParameter))
	return id
}

// AddLabel adds a label.
func (b *ModuleBuilder) AddLabel() uint32 {
	id := b.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(id)
	b.functions = append(b.functions, builder.Build(OpLabel))
	return id
}

// AddReturn adds OpReturn.
func (b *ModuleBuilder) AddReturn() {
	builder := NewInstructionBuilder()
	b.functions = append(b.functions, builder.Build(OpReturn))
}

// AddReturnValue adds OpReturnValue.
func (b *ModuleBuilder) AddReturnValue(valueID uint32) {
	builder := NewInstructionBuilder()
	builder.AddWord(valueID)
	b.functions = append(b.functions, builder.Build(OpReturnValue))
}

// AddFunctionEnd adds OpFunctionEnd.
func (b *ModuleBuilder) AddFunctionEnd() {
	builder := NewInstructionBuilder()
	b.functions = append(b.functions, builder.Build(OpFunctionEnd))
}

// AddBinaryOp adds a binary operation instruction.
func (b *ModuleBuilder) AddBinaryOp(opcode OpCode, resultType uint32, left uint32, right uint32) uint32 {
	resultID := b.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(resultType)
	builder.AddWord(resultID)
	builder.AddWord(left)
	builder.AddWord(right)
	b.functions = append(b.functions, builder.Build(opcode))
	return resultID
}

// AddUnaryOp adds a unary operation instruction.
func (b *ModuleBuilder) AddUnaryOp(opcode OpCode, resultType uint32, operand uint32) uint32 {
	resultID := b.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(resultType)
	builder.AddWord(resultID)
	builder.AddWord(operand)
	b.functions = append(b.functions, builder.Build(opcode))
	return resultID
}

// AddLoad adds OpLoad.
func (b *ModuleBuilder) AddLoad(resultType uint32, pointer uint32) uint32 {
	resultID := b.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(resultType)
	builder.AddWord(resultID)
	builder.AddWord(pointer)
	b.functions = append(b.functions, builder.Build(OpLoad))
	return resultID
}

// AddStore adds OpStore.
func (b *ModuleBuilder) AddStore(pointer uint32, value uint32) {
	builder := NewInstructionBuilder()
	builder.AddWord(pointer)
	builder.AddWord(value)
	b.functions = append(b.functions, builder.Build(OpStore))
}

// AddAccessChain adds OpAccessChain.
func (b *ModuleBuilder) AddAccessChain(resultType uint32, base uint32, indices ...uint32) uint32 {
	resultID := b.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(resultType)
	builder.AddWord(resultID)
	builder.AddWord(base)
	for _, index := range indices {
		builder.AddWord(index)
	}
	b.functions = append(b.functions, builder.Build(OpAccessChain))
	return resultID
}

// AddCompositeConstruct adds OpCompositeConstruct.
func (b *ModuleBuilder) AddCompositeConstruct(resultType uint32, constituents ...uint32) uint32 {
	resultID := b.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(resultType)
	builder.AddWord(resultID)
	for _, constituent := range constituents {
		builder.AddWord(constituent)
	}
	b.functions = append(b.functions, builder.Build(OpCompositeConstruct))
	return resultID
}

// AddVectorShuffle adds OpVectorShuffle for vector swizzle operations.
func (b *ModuleBuilder) AddVectorShuffle(resultType uint32, vec1 uint32, vec2 uint32, components []uint32) uint32 {
	resultID := b.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(resultType)
	builder.AddWord(resultID)
	builder.AddWord(vec1)
	builder.AddWord(vec2)
	for _, component := range components {
		builder.AddWord(component)
	}
	b.functions = append(b.functions, builder.Build(OpVectorShuffle))
	return resultID
}

// AddSelect adds OpSelect.
func (b *ModuleBuilder) AddSelect(resultType uint32, condition uint32, accept uint32, reject uint32) uint32 {
	resultID := b.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(resultType)
	builder.AddWord(resultID)
	builder.AddWord(condition)
	builder.AddWord(accept)
	builder.AddWord(reject)
	b.functions = append(b.functions, builder.Build(OpSelect))
	return resultID
}

// AddSelectionMerge adds OpSelectionMerge.
func (b *ModuleBuilder) AddSelectionMerge(mergeLabel uint32, control SelectionControl) {
	builder := NewInstructionBuilder()
	builder.AddWord(mergeLabel)
	builder.AddWord(uint32(control))
	b.functions = append(b.functions, builder.Build(OpSelectionMerge))
}

// AddLoopMerge adds OpLoopMerge.
func (b *ModuleBuilder) AddLoopMerge(mergeLabel uint32, continueLabel uint32, control LoopControl) {
	builder := NewInstructionBuilder()
	builder.AddWord(mergeLabel)
	builder.AddWord(continueLabel)
	builder.AddWord(uint32(control))
	b.functions = append(b.functions, builder.Build(OpLoopMerge))
}

// AddBranchConditional adds OpBranchConditional.
func (b *ModuleBuilder) AddBranchConditional(condition uint32, trueLabel uint32, falseLabel uint32) {
	builder := NewInstructionBuilder()
	builder.AddWord(condition)
	builder.AddWord(trueLabel)
	builder.AddWord(falseLabel)
	b.functions = append(b.functions, builder.Build(OpBranchConditional))
}

// AddKill adds OpKill (fragment shader discard).
func (b *ModuleBuilder) AddKill() {
	builder := NewInstructionBuilder()
	b.functions = append(b.functions, builder.Build(OpKill))
}

// AddExtInst adds OpExtInst (extended instruction).
func (b *ModuleBuilder) AddExtInst(resultType uint32, extSet uint32, instruction uint32, operands ...uint32) uint32 {
	resultID := b.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(resultType)
	builder.AddWord(resultID)
	builder.AddWord(extSet)
	builder.AddWord(instruction)
	for _, operand := range operands {
		builder.AddWord(operand)
	}
	b.functions = append(b.functions, builder.Build(OpExtInst))
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
		totalWords += len(b.memoryModel.Encode())
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
		count += len(inst.Encode())
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
	words := inst.Encode()
	for _, word := range words {
		binary.LittleEndian.PutUint32(buffer[offset:], word)
		offset += 4
	}
	return offset
}

// versionToWord converts Version to SPIR-V word format.
func versionToWord(v Version) uint32 {
	return (uint32(v.Major) << 16) | (uint32(v.Minor) << 8)
}
