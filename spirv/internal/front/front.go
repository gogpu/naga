// Package front decodes a SPIR-V binary and lowers it to naga IR, as Rust
// naga's front::spv does. The public entry point is [spirv.Parse]; the package
// is internal because the SPIR-V enum set it reads lives beside it and no
// caller should depend on the decoder.
package front

import (
	"encoding/binary"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/gogpu/naga/frontend"
	"github.com/gogpu/naga/internal/registry"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// magicWord is the first word of every SPIR-V module.
const magicWord = 0x07230203

// Options configures Parse. Its fields are the public spirv.ParseOptions, so
// the wrapper converts field by field.
type Options struct {
	KeepCoordinateSpace      bool
	AllowUnknownCapabilities bool
	CombinedImageSampler     frontend.CombinedImageSamplerPolicy
	SamplerBindings          frontend.SamplerBindings
}

// Info is what Parse reports besides the module; spirv.ParseInfo mirrors it.
type Info struct {
	Warnings         []frontend.Diagnostic
	CombinedSamplers []frontend.CombinedSampler
}

// Parse decodes a SPIR-V binary and lowers it to IR. The module is not
// validated: the caller runs valid.Validate.
func Parse(data []byte, options Options) (*ir.Module, Info, error) {
	if err := checkModuleSize(int64(len(data))); err != nil {
		return nil, Info{}, err
	}
	if len(data)%4 != 0 {
		return nil, Info{}, errIncompleteData
	}
	return newParser(data, options).parse()
}

// newParser returns a parser for one module, with its lookup tables ready.
func newParser(data []byte, options Options) *parser {
	return &parser{
		data:                     data,
		options:                  options,
		types:                    registry.NewTypeRegistry(),
		entryPoints:              make(map[uint32]*entryPointInfo),
		decorations:              make(map[uint32]*decoration),
		memberDecorations:        make(map[memberKey]*decoration),
		lookupType:               make(map[uint32]lookupType),
		lookupStorageBufferTypes: make(map[ir.TypeHandle]ir.StorageAccess),
		lookupMember:             make(map[lookupMemberKey]lookupMember),
		lookupFunctionType:       make(map[uint32]lookupFunctionType),
		lookupConstant:           make(map[uint32]lookupConstant),
		lookupVariable:           make(map[uint32]lookupVariable),
		lookupFunction:           make(map[uint32]lookupFunction),
		glPerVertexBuiltinAccess: make(map[ir.BuiltinValue]struct{}),
	}
}

// parser holds the state of one module being parsed: the input words, the
// section reached (SPIR-V requires sections in a fixed order), and the
// SPIR-V id to IR handle tables the instructions are translated through.
type parser struct {
	data   []byte
	offset int // byte offset of the next word

	// instOffset is the byte offset of the instruction being handled; errors
	// point at it.
	instOffset int

	state   moduleState
	options Options

	module *ir.Module
	info   Info

	// extGLSLID is the result id of the OpExtInstImport "GLSL.std.450", and
	// extNonSemanticID of the NonSemantic.Shader.DebugInfo.100 set, whose
	// instructions are ignorable.
	extGLSLID         uint32
	hasExtGLSLID      bool
	extNonSemanticID  uint32
	hasExtNonSemantic bool

	// The SPIR-V id to IR handle tables (Rust naga's lookups).
	types    *registry.TypeRegistry
	layouter ir.Layouter
	// typeDepths holds the nesting depth of every type in the arena, by
	// handle, bounded by maxTypeDepth.
	typeDepths               []int
	lookupType               map[uint32]lookupType
	voidTypeID               uint32
	hasVoidType              bool
	lookupStorageBufferTypes map[ir.TypeHandle]ir.StorageAccess
	lookupMember             map[lookupMemberKey]lookupMember
	lookupFunctionType       map[uint32]lookupFunctionType
	lookupConstant           map[uint32]lookupConstant
	lookupVariable           map[uint32]lookupVariable
	lookupFunction           map[uint32]lookupFunction
	// glPerVertexBuiltinAccess holds the built-ins an OpAccessChain reaches
	// through a struct member, as Rust naga's set of the same name. glslang
	// declares the gl_PerVertex built-ins a shader never writes, and the
	// wrapper drops the binding of those in its result.
	glPerVertexBuiltinAccess map[ir.BuiltinValue]struct{}
	// lookupExpression is the table of the function being parsed; it is
	// cleared when the next function starts.
	lookupExpression map[uint32]lookupExpression
	// storageExpressions counts the global and constant expressions every
	// function parsed so far got, bounded by maxStorageExpressions.
	storageExpressions int

	// entryPoints holds the OpEntryPoint declarations by function id.
	entryPoints map[uint32]*entryPointInfo
	// deferredEntryPoints lists the entry point function ids in OpEntryPoint
	// order: their wrappers are built after every function is parsed.
	deferredEntryPoints []uint32

	// deferredCalls holds every call site, indexed by the placeholder
	// function handle its IR refers to until all functions are parsed, and
	// callGraph the calls between functions, which orders Module.Functions.
	deferredCalls []deferredCall
	callGraph     callGraph

	// decorations and memberDecorations hold the values of OpDecorate,
	// OpMemberDecorate, OpName and OpMemberName for ids that may not be
	// defined yet; a definition consumes the entry.
	decorations       map[uint32]*decoration
	memberDecorations map[memberKey]*decoration
}

// entryPointInfo is one OpEntryPoint instruction: the stage, the name, the
// execution modes and the interface variables, until the function is parsed
// and the entry point is assembled.
type entryPointInfo struct {
	stage          ir.ShaderStage
	name           string
	earlyDepthTest ir.EarlyDepthTest
	workgroup      [3]uint32
	variableIDs    []uint32
}

// memberKey identifies a struct member for OpMemberDecorate and OpMemberName.
type memberKey struct {
	typeID uint32
	member uint32
}

// moduleState is the section of the module being read, in the order SPIR-V
// requires. Rust naga's ModuleState also has ExtInstImport and ModuleProcessed
// states, which no handler there ever switches to; they are left out here.
type moduleState uint8

const (
	stateEmpty moduleState = iota
	stateCapability
	stateExtension
	stateMemoryModel
	stateEntryPoint
	stateExecutionMode
	stateSource
	stateName
	stateAnnotation
	stateType
	stateFunction
)

// String returns the state's name as Rust naga's Debug prints it.
func (s moduleState) String() string {
	switch s {
	case stateEmpty:
		return "Empty"
	case stateCapability:
		return "Capability"
	case stateExtension:
		return "Extension"
	case stateMemoryModel:
		return "MemoryModel"
	case stateEntryPoint:
		return "EntryPoint"
	case stateExecutionMode:
		return "ExecutionMode"
	case stateSource:
		return "Source"
	case stateName:
		return "Name"
	case stateAnnotation:
		return "Annotation"
	case stateType:
		return "Type"
	case stateFunction:
		return "Function"
	}
	return fmt.Sprintf("moduleState(%d)", uint8(s))
}

// instruction is one decoded instruction word: its opcode and its word count,
// which includes the word itself.
type instruction struct {
	op spvenums.Op
	wc uint16
}

// expect checks the instruction's word count, as Rust naga's
// Instruction::expect.
func (inst instruction) expect(wc uint16) error {
	if inst.wc != wc {
		return errInvalidOperandCount(inst.op, inst.wc)
	}
	return nil
}

// expectAtLeast returns the number of operand words beyond count, as Rust
// naga's Instruction::expect_at_least.
func (inst instruction) expectAtLeast(count uint16) (uint16, error) {
	if inst.wc < count {
		return 0, errInvalidOperandCount(inst.op, inst.wc)
	}
	return inst.wc - count, nil
}

// parse reads the header and then every instruction of the module.
func (p *parser) parse() (*ir.Module, Info, error) {
	p.module = &ir.Module{}
	if err := p.parseHeader(); err != nil {
		return nil, Info{}, p.at(err)
	}
	for {
		inst, err := p.nextInstruction()
		if errors.Is(err, errIncompleteData) {
			break
		}
		if err != nil {
			return nil, Info{}, p.at(err)
		}
		if err := p.handle(inst); err != nil {
			return nil, Info{}, p.at(err)
		}
	}

	// The entry point wrappers are built after every function is parsed, so
	// that their calls all have a target.
	for _, functionID := range p.deferredEntryPoints {
		if err := p.processEntryPoint(functionID); err != nil {
			return nil, Info{}, err
		}
	}

	// Every call target is known now: order the functions callees first and
	// replace the placeholder handles.
	if err := p.orderFunctions(); err != nil {
		return nil, Info{}, err
	}
	if err := p.checkCalls(); err != nil {
		return nil, Info{}, err
	}
	p.patchCalls()
	return p.module, p.info, nil
}

// parseHeader checks the magic number and the version and skips the words
// nothing reads: the generator, the bound (never used for preallocation) and
// the schema.
func (p *parser) parseHeader() error {
	// The magic word precedes any instruction: an offset for its errors would
	// collide with "no offset", so they carry none.
	p.instOffset = 0
	magic, err := p.next()
	if err != nil {
		return err
	}
	if magic != magicWord {
		return errInvalidHeader
	}
	version, err := p.next()
	if err != nil {
		return err
	}
	// The version is the major number in the high byte, the minor in the next
	// one; this package reads SPIR-V 1.0 through 1.6.
	major, minor := (version>>16)&0xff, (version>>8)&0xff
	if major != 1 || minor > 6 {
		return errInvalidHeader
	}
	if _, err := p.next(); err != nil {
		return err
	}
	if _, err := p.next(); err != nil {
		return err
	}
	if _, err := p.next(); err != nil {
		return err
	}
	return nil
}

// next reads one word.
func (p *parser) next() (uint32, error) {
	if p.offset+4 > len(p.data) {
		return 0, errIncompleteData
	}
	word := binary.LittleEndian.Uint32(p.data[p.offset:])
	p.offset += 4
	return word, nil
}

// nextInstruction decodes the instruction word: the word count in the high 16
// bits, the opcode in the low 16.
func (p *parser) nextInstruction() (instruction, error) {
	p.instOffset = p.offset
	word, err := p.next()
	if err != nil {
		return instruction{}, err
	}
	wc := uint16(word >> 16)
	if wc == 0 {
		return instruction{}, errInvalidWordCount
	}
	opcode := uint16(word)
	op := spvenums.Op(opcode)
	if !op.Known() {
		return instruction{}, errUnknownInstruction(opcode)
	}
	return instruction{op: op, wc: wc}, nil
}

// nextString reads a NUL-terminated UTF-8 string that occupies count words,
// as Rust naga's Frontend::next_string. It returns the string and the number
// of words left after the one that held the NUL.
func (p *parser) nextString(count uint16) (string, uint16, error) {
	var buf []byte
	for {
		if count == 0 {
			return "", 0, errBadString
		}
		count--
		word, err := p.next()
		if err != nil {
			return "", 0, err
		}
		chunk := [4]byte{byte(word), byte(word >> 8), byte(word >> 16), byte(word >> 24)}
		pos := -1
		for i, b := range chunk {
			if b == 0 {
				pos = i
				break
			}
		}
		if pos < 0 {
			buf = append(buf, chunk[:]...)
			continue
		}
		buf = append(buf, chunk[:pos]...)
		break
	}
	if !utf8.Valid(buf) {
		return "", 0, errBadString
	}
	return string(buf), count, nil
}

// parseModuleExtInst skips a module-level OpExtInst of the non-semantic debug
// set, whose instructions are ignorable; any other extended instruction there
// is unsupported, as in Rust naga.
func (p *parser) parseModuleExtInst(inst instruction) error {
	if _, err := inst.expectAtLeast(4); err != nil {
		return err
	}
	if _, err := p.next(); err != nil { // result type
		return err
	}
	if _, err := p.next(); err != nil { // result id
		return err
	}
	setID, err := p.next()
	if err != nil {
		return err
	}
	if !p.hasExtNonSemantic || setID != p.extNonSemanticID {
		return errUnsupportedInstruction(p.state, inst.op)
	}
	for range inst.wc - 4 {
		if _, err := p.next(); err != nil {
			return err
		}
	}
	return nil
}

// switchTo enforces SPIR-V's section order: a state before the one reached is
// an unsupported instruction, as Rust naga's Frontend::switch.
func (p *parser) switchTo(state moduleState, op spvenums.Op) error {
	if state < p.state {
		return errUnsupportedInstruction(p.state, op)
	}
	p.state = state
	return nil
}

// at stamps the current instruction's offset on err when it has none, so a
// caller can point into the disassembly.
func (p *parser) at(err error) error {
	var parseErr *Error
	if !errors.As(err, &parseErr) {
		return err
	}
	if parseErr.Offset != 0 || p.instOffset == 0 {
		return parseErr
	}
	return &Error{Message: parseErr.Message, Offset: uint32(p.instOffset)}
}

// warn records a diagnostic that does not stop parsing, at the current
// instruction.
func (p *parser) warnf(format string, args ...any) {
	p.info.Warnings = append(p.info.Warnings, frontend.Diagnostic{
		Message: fmt.Sprintf(format, args...),
		Span:    ir.Span{Start: uint32(p.instOffset), End: uint32(p.offset)},
	})
}

// handle dispatches one module-level instruction. Instructions inside a
// function are parsed by parseFunction, which owns its loop.
func (p *parser) handle(inst instruction) error {
	switch inst.op {
	case spvenums.OpCapability:
		return p.parseCapability(inst)
	case spvenums.OpExtension:
		return p.parseExtension(inst)
	case spvenums.OpExtInstImport:
		return p.parseExtInstImport(inst)
	case spvenums.OpMemoryModel:
		return p.parseMemoryModel(inst)
	case spvenums.OpEntryPoint:
		return p.parseEntryPoint(inst)
	case spvenums.OpExecutionMode:
		return p.parseExecutionMode(inst)
	case spvenums.OpString:
		return p.parseString(inst)
	case spvenums.OpSource:
		return p.parseSource(inst)
	case spvenums.OpSourceExtension:
		return p.parseSourceExtension(inst)
	case spvenums.OpName:
		return p.parseName(inst)
	case spvenums.OpMemberName:
		return p.parseMemberName(inst)
	case spvenums.OpModuleProcessed:
		return p.parseModuleProcessed(inst)
	case spvenums.OpDecorate:
		return p.parseDecorate(inst)
	case spvenums.OpMemberDecorate:
		return p.parseMemberDecorate(inst)
	case spvenums.OpTypeVoid:
		return p.parseTypeVoid(inst)
	case spvenums.OpTypeBool:
		return p.parseTypeBool(inst)
	case spvenums.OpTypeInt:
		return p.parseTypeInt(inst)
	case spvenums.OpTypeFloat:
		return p.parseTypeFloat(inst)
	case spvenums.OpTypeVector:
		return p.parseTypeVector(inst)
	case spvenums.OpTypeMatrix:
		return p.parseTypeMatrix(inst)
	case spvenums.OpTypeFunction:
		return p.parseTypeFunction(inst)
	case spvenums.OpTypePointer:
		return p.parseTypePointer(inst)
	case spvenums.OpTypeArray:
		return p.parseTypeArray(inst)
	case spvenums.OpTypeRuntimeArray:
		return p.parseTypeRuntimeArray(inst)
	case spvenums.OpTypeStruct:
		return p.parseTypeStruct(inst)
	case spvenums.OpTypeImage:
		return p.parseTypeImage(inst)
	case spvenums.OpTypeSampledImage:
		return p.parseTypeSampledImage(inst)
	case spvenums.OpTypeSampler:
		return p.parseTypeSampler(inst)
	case spvenums.OpConstant, spvenums.OpSpecConstant:
		return p.parseConstant(inst)
	case spvenums.OpConstantComposite, spvenums.OpSpecConstantComposite:
		return p.parseCompositeConstant(inst)
	case spvenums.OpConstantNull, spvenums.OpUndef:
		return p.parseNullConstant(inst)
	case spvenums.OpConstantTrue, spvenums.OpSpecConstantTrue:
		return p.parseBoolConstant(inst, true)
	case spvenums.OpConstantFalse, spvenums.OpSpecConstantFalse:
		return p.parseBoolConstant(inst, false)
	case spvenums.OpVariable:
		return p.parseGlobalVariable(inst)
	case spvenums.OpExtInst:
		return p.parseModuleExtInst(inst)
	case spvenums.OpFunction:
		if err := p.switchTo(stateFunction, inst.op); err != nil {
			return err
		}
		if err := inst.expect(5); err != nil {
			return err
		}
		return p.parseFunction()
	}
	return errUnsupportedInstruction(p.state, inst.op)
}

// parseCapability records one OpCapability. A capability the grammar does not
// define is an invalid module; one it defines but this package does not
// support is refused unless AllowUnknownCapabilities asks for a warning and
// lets the validator decide, as Rust naga does.
func (p *parser) parseCapability(inst instruction) error {
	if err := p.switchTo(stateCapability, inst.op); err != nil {
		return err
	}
	if err := inst.expect(2); err != nil {
		return err
	}
	raw, err := p.next()
	if err != nil {
		return err
	}
	capability := spvenums.Capability(raw)
	if !capability.Known() {
		return errUnknownCapability(raw)
	}
	if !supportedCapability(capability) {
		if !p.options.AllowUnknownCapabilities {
			return errUnsupportedCapability(capability)
		}
		p.warnf("Unknown capability %s", bareName(capability.String(), "Capability"))
	}
	return nil
}

// parseExtension accepts one of the SPIR-V extensions Rust naga supports. An
// extension is always an error otherwise: unlike a capability, nothing in the
// module can be read without knowing what it changes.
func (p *parser) parseExtension(inst instruction) error {
	if err := p.switchTo(stateExtension, inst.op); err != nil {
		return err
	}
	if _, err := inst.expectAtLeast(2); err != nil {
		return err
	}
	name, left, err := p.nextString(inst.wc - 1)
	if err != nil {
		return err
	}
	if left != 0 {
		return errInvalidOperand
	}
	if !supportedExtension(name) {
		return errUnsupportedExtension(name)
	}
	return nil
}

// parseExtInstImport records the only extended instruction sets this package
// reads: GLSL.std.450 and the non-semantic debug info set, whose instructions
// are ignorable.
func (p *parser) parseExtInstImport(inst instruction) error {
	if err := p.switchTo(stateExtension, inst.op); err != nil {
		return err
	}
	if _, err := inst.expectAtLeast(3); err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	name, left, err := p.nextString(inst.wc - 2)
	if err != nil {
		return err
	}
	if left != 0 {
		return errInvalidOperand
	}
	switch name {
	case "GLSL.std.450":
		p.extGLSLID, p.hasExtGLSLID = resultID, true
	case "NonSemantic.Shader.DebugInfo.100":
		p.extNonSemanticID, p.hasExtNonSemantic = resultID, true
	default:
		return errUnsupportedExtSet(name)
	}
	return nil
}

// parseMemoryModel reads and ignores OpMemoryModel: the addressing model and
// the memory model say nothing the IR keeps.
func (p *parser) parseMemoryModel(inst instruction) error {
	if err := p.switchTo(stateMemoryModel, inst.op); err != nil {
		return err
	}
	if err := inst.expect(3); err != nil {
		return err
	}
	if _, err := p.next(); err != nil {
		return err
	}
	if _, err := p.next(); err != nil {
		return err
	}
	return nil
}

// parseEntryPoint records one OpEntryPoint: the stage, the name, and the
// interface variable ids the wrapper is built from.
func (p *parser) parseEntryPoint(inst instruction) error {
	if err := p.switchTo(stateEntryPoint, inst.op); err != nil {
		return err
	}
	if _, err := inst.expectAtLeast(4); err != nil {
		return err
	}
	rawExec, err := p.next()
	if err != nil {
		return err
	}
	var stage ir.ShaderStage
	switch spvenums.ExecutionModel(rawExec) {
	case spvenums.ExecutionModelVertex:
		stage = ir.StageVertex
	case spvenums.ExecutionModelFragment:
		stage = ir.StageFragment
	case spvenums.ExecutionModelGLCompute:
		stage = ir.StageCompute
	case spvenums.ExecutionModelTaskEXT:
		stage = ir.StageTask
	case spvenums.ExecutionModelMeshEXT:
		stage = ir.StageMesh
	default:
		return errUnsupportedExecutionModel(rawExec)
	}
	functionID, err := p.next()
	if err != nil {
		return err
	}
	name, left, err := p.nextString(inst.wc - 3)
	if err != nil {
		return err
	}
	ep := &entryPointInfo{stage: stage, name: name}
	for range left {
		variableID, err := p.next()
		if err != nil {
			return err
		}
		ep.variableIDs = append(ep.variableIDs, variableID)
	}
	p.entryPoints[functionID] = ep
	return nil
}

// parseExecutionMode applies one OpExecutionMode to its entry point. The
// modes the IR cannot express are an error; the ones the IR deduces are
// ignored, as Rust naga does.
func (p *parser) parseExecutionMode(inst instruction) error {
	if err := p.switchTo(stateExecutionMode, inst.op); err != nil {
		return err
	}
	if _, err := inst.expectAtLeast(3); err != nil {
		return err
	}
	epID, err := p.next()
	if err != nil {
		return err
	}
	modeRaw, err := p.next()
	if err != nil {
		return err
	}
	args := make([]uint32, 0, inst.wc-3)
	for range inst.wc - 3 {
		arg, err := p.next()
		if err != nil {
			return err
		}
		args = append(args, arg)
	}
	ep, ok := p.entryPoints[epID]
	if !ok {
		return errInvalidID(epID)
	}
	setConservative := func(depth ir.ConservativeDepth) {
		switch current := ep.earlyDepthTest.(type) {
		case nil:
			ep.earlyDepthTest = ir.EarlyDepthAllow{Conservative: depth}
		case ir.EarlyDepthAllow:
			current.Conservative = depth
			ep.earlyDepthTest = current
		}
	}
	switch spvenums.ExecutionMode(modeRaw) {
	case spvenums.ExecutionModeEarlyFragmentTests:
		ep.earlyDepthTest = ir.EarlyDepthForce{}
	case spvenums.ExecutionModeDepthUnchanged:
		setConservative(ir.ConservativeDepthUnchanged)
	case spvenums.ExecutionModeDepthGreater:
		setConservative(ir.ConservativeDepthGreaterEqual)
	case spvenums.ExecutionModeDepthLess:
		setConservative(ir.ConservativeDepthLessEqual)
	case spvenums.ExecutionModeDepthReplacing:
		// Ignored: the IR deduces it from the shader writing its depth.
	case spvenums.ExecutionModeOriginUpperLeft:
		// Ignored: the other option, OriginLowerLeft, is not valid in Vulkan.
	case spvenums.ExecutionModeLocalSize:
		if len(args) < 3 {
			return errInvalidOperandCount(inst.op, inst.wc)
		}
		ep.workgroup = [3]uint32{args[0], args[1], args[2]}
	default:
		return errUnsupportedExecutionMode(modeRaw)
	}
	return nil
}

// parseString reads and ignores OpString, a debug name for an OpSource.
func (p *parser) parseString(inst instruction) error {
	if err := p.switchTo(stateSource, inst.op); err != nil {
		return err
	}
	if _, err := inst.expectAtLeast(3); err != nil {
		return err
	}
	if _, err := p.next(); err != nil {
		return err
	}
	if _, _, err := p.nextString(inst.wc - 2); err != nil {
		return err
	}
	return nil
}

// parseSource reads and ignores OpSource.
func (p *parser) parseSource(inst instruction) error {
	if err := p.switchTo(stateSource, inst.op); err != nil {
		return err
	}
	for range inst.wc - 1 {
		if _, err := p.next(); err != nil {
			return err
		}
	}
	return nil
}

// parseSourceExtension reads and ignores OpSourceExtension.
func (p *parser) parseSourceExtension(inst instruction) error {
	if err := p.switchTo(stateSource, inst.op); err != nil {
		return err
	}
	if _, err := inst.expectAtLeast(2); err != nil {
		return err
	}
	if _, _, err := p.nextString(inst.wc - 1); err != nil {
		return err
	}
	return nil
}

// parseName records OpName on its id, which may not be defined yet.
func (p *parser) parseName(inst instruction) error {
	if err := p.switchTo(stateName, inst.op); err != nil {
		return err
	}
	if _, err := inst.expectAtLeast(3); err != nil {
		return err
	}
	id, err := p.next()
	if err != nil {
		return err
	}
	name, left, err := p.nextString(inst.wc - 2)
	if err != nil {
		return err
	}
	if left != 0 {
		return errInvalidOperand
	}
	p.decorationOf(id).name = name
	return nil
}

// parseMemberName records OpMemberName on its struct member, which may not be
// defined yet.
func (p *parser) parseMemberName(inst instruction) error {
	if err := p.switchTo(stateName, inst.op); err != nil {
		return err
	}
	if _, err := inst.expectAtLeast(4); err != nil {
		return err
	}
	id, err := p.next()
	if err != nil {
		return err
	}
	member, err := p.next()
	if err != nil {
		return err
	}
	name, left, err := p.nextString(inst.wc - 3)
	if err != nil {
		return err
	}
	if left != 0 {
		return errInvalidOperand
	}
	p.memberDecorationOf(id, member).name = name
	return nil
}

// parseModuleProcessed reads and ignores OpModuleProcessed.
func (p *parser) parseModuleProcessed(inst instruction) error {
	if err := p.switchTo(stateName, inst.op); err != nil {
		return err
	}
	if _, err := inst.expectAtLeast(2); err != nil {
		return err
	}
	if _, left, err := p.nextString(inst.wc - 1); err != nil {
		return err
	} else if left != 0 {
		return errInvalidOperand
	}
	return nil
}

// parseDecorate records one OpDecorate on its id, which may not be defined
// yet.
func (p *parser) parseDecorate(inst instruction) error {
	if err := p.switchTo(stateAnnotation, inst.op); err != nil {
		return err
	}
	if _, err := inst.expectAtLeast(3); err != nil {
		return err
	}
	id, err := p.next()
	if err != nil {
		return err
	}
	return p.nextDecoration(inst, 2, p.decorationOf(id))
}

// parseMemberDecorate records one OpMemberDecorate on its struct member, which
// may not be defined yet.
func (p *parser) parseMemberDecorate(inst instruction) error {
	if err := p.switchTo(stateAnnotation, inst.op); err != nil {
		return err
	}
	if _, err := inst.expectAtLeast(4); err != nil {
		return err
	}
	id, err := p.next()
	if err != nil {
		return err
	}
	member, err := p.next()
	if err != nil {
		return err
	}
	return p.nextDecoration(inst, 3, p.memberDecorationOf(id, member))
}

// decorationOf returns the decoration record of id, creating it if this is
// its first mention.
func (p *parser) decorationOf(id uint32) *decoration {
	dec, ok := p.decorations[id]
	if !ok {
		dec = &decoration{}
		p.decorations[id] = dec
	}
	return dec
}

// memberDecorationOf returns the decoration record of a struct member,
// creating it if this is its first mention.
func (p *parser) memberDecorationOf(id, member uint32) *decoration {
	key := memberKey{typeID: id, member: member}
	dec, ok := p.memberDecorations[key]
	if !ok {
		dec = &decoration{}
		p.memberDecorations[key] = dec
	}
	return dec
}
