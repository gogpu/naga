package front

import (
	"fmt"
	"strings"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// Error is one SPIR-V parse error. The public spirv.ParseError mirrors it:
// Message is Rust naga's text, and Offset is the byte offset of the offending
// instruction, 0 when the error has none.
type Error struct {
	Message string
	Offset  uint32
}

// Error returns the message.
func (e *Error) Error() string { return e.Message }

// The error values that carry no instruction. Only errIncompleteData is
// compared by identity: a truncation at an instruction boundary ends the
// module, while the same error inside a handler is a real one.
var (
	errInvalidHeader    = &Error{Message: "invalid header"}
	errInvalidWordCount = &Error{Message: "invalid word count"}
	errIncompleteData   = &Error{Message: "incomplete data"}
	errBadString        = &Error{Message: "bad string"}
	errInvalidOperand   = &Error{Message: "invalid operand"}
	errModuleTooLarge   = &Error{Message: "module is longer than 4 GiB"}
)

// The error constructors below keep Rust naga's texts (error.rs):
// message parity is part of the port.

func errUnknownInstruction(opcode uint16) *Error {
	return &Error{Message: fmt.Sprintf("unknown instruction %d", opcode)}
}

func errInvalidOperandCount(op spvenums.Op, wc uint16) *Error {
	return &Error{Message: fmt.Sprintf("invalid operand count %d for %s", wc, bareName(op.String(), "Op"))}
}

// bareName is the name Rust naga's Debug prints for a SPIR-V value: the
// grammar's enumerant without the kind prefix the generated constant names
// carry ("TypeVoid", not "OpTypeVoid").
func bareName(name, kind string) string {
	return strings.TrimPrefix(name, kind)
}

func errUnsupportedInstruction(state moduleState, op spvenums.Op) *Error {
	return &Error{Message: fmt.Sprintf("unsupported instruction %s at %s", bareName(op.String(), "Op"), state)}
}

func errUnknownCapability(capability uint32) *Error {
	return &Error{Message: fmt.Sprintf("unknown capability %%%d", capability)}
}

func errUnsupportedCapability(capability spvenums.Capability) *Error {
	return &Error{Message: fmt.Sprintf("unsupported capability %s", bareName(capability.String(), "Capability"))}
}

func errUnsupportedExtension(name string) *Error {
	return &Error{Message: fmt.Sprintf("unsupported extension %s", name)}
}

func errUnsupportedExtSet(name string) *Error {
	return &Error{Message: fmt.Sprintf("unsupported extension set %s", name)}
}

func errUnsupportedExecutionModel(model uint32) *Error {
	return &Error{Message: fmt.Sprintf("unsupported execution model %%%d", model)}
}

func errUnsupportedExecutionMode(mode uint32) *Error {
	return &Error{Message: fmt.Sprintf("unsupported execution mode %%%d", mode)}
}

func errInvalidDecoration(decoration uint32) *Error {
	return &Error{Message: fmt.Sprintf("invalid decoration %%%d", decoration)}
}

func errInvalidID(id uint32) *Error {
	return &Error{Message: fmt.Sprintf("invalid id %%%d", id)}
}

func errInvalidSign(sign uint32) *Error {
	return &Error{Message: fmt.Sprintf("invalid sign %%%d", sign)}
}

func errInvalidInnerType(id uint32) *Error {
	return &Error{Message: fmt.Sprintf("invalid inner type %%%d", id)}
}

func errInvalidVectorSize(size uint32) *Error {
	return &Error{Message: fmt.Sprintf("invalid vector size %%%d", size)}
}

func errInvalidTypeWidth(width uint32) *Error {
	return &Error{Message: fmt.Sprintf("invalid type width %%%d", width)}
}

// errUnsupportedType names an IR type the way Rust's Debug prints a Handle.
func errUnsupportedType(ty ir.TypeHandle) *Error {
	return &Error{Message: fmt.Sprintf("unsupported type [%d]", ty)}
}

func errInvalidImageBaseType(ty ir.TypeHandle) *Error {
	return &Error{Message: fmt.Sprintf("invalid image base type [%d]", ty)}
}

// errInvalidGlobalVar and errInvalidImageExpression are Rust naga's
// InvalidGlobalVar and InvalidImageExpression without the expression dump:
// the IR has no Debug rendering, as with errInvalidAccess.
var (
	errInvalidGlobalVar       = &Error{Message: "invalid global var"}
	errInvalidImageExpression = &Error{Message: "invalid image/sampler expression"}
)

func errInvalidImage(ty ir.TypeHandle) *Error {
	return &Error{Message: fmt.Sprintf("invalid image [%d]", ty)}
}

// errInvalidImageCoordinate replaces a Rust naga panic (unreachable!) with an
// error: a texture coordinate of neither scalar nor vector type is untrusted
// input, not an internal invariant.
func errInvalidImageCoordinate(ty ir.TypeHandle) *Error {
	return &Error{Message: fmt.Sprintf("invalid image coordinate type [%d]", ty)}
}

func errInvalidVectorType(ty ir.TypeHandle) *Error {
	return &Error{Message: fmt.Sprintf("invalid vector type [%d]", ty)}
}

func errInconsistentComparisonSampling(handle ir.GlobalVariableHandle) *Error {
	return &Error{Message: fmt.Sprintf("inconsistent comparison sampling [%d]", handle)}
}

// The atomic upgrade errors keep Rust naga's atomic_upgrade::Error texts under
// their wrapper ("atomic upgrade error: ..."), as Rust's Display does.
var (
	errAtomicUnsupportedExpression      = &Error{Message: "atomic upgrade error: encountered an unsupported expression"}
	errAtomicGlobalVariableMissing      = &Error{Message: "atomic upgrade error: expected to find a global variable"}
	errAtomicGlobalInitUnsupported      = &Error{Message: "atomic upgrade error: encountered unsupported global initializer in an atomic variable"}
	errAtomicCompareExchangeNonScalar   = &Error{Message: "atomic upgrade error: atomic compare exchange requires a scalar base type"}
	errTooManyCompositeInsertComponents = &Error{Message: "the composite an OpCompositeInsert builds is larger than this frontend accepts"}
)

func errInvalidIndexType(id uint32) *Error {
	return &Error{Message: fmt.Sprintf("invalid index type %%%d", id)}
}

func errUnsupportedGroupOperation(op uint32) *Error {
	return &Error{Message: fmt.Sprintf("unsupported group operation %%%d", op)}
}

func errCombinedSamplerRejected(id uint32) *Error {
	return &Error{Message: fmt.Sprintf("combined image sampler %%%d: splitting is disabled by CombinedImageSamplerReject", id)}
}

func errCombinedSamplerWithoutBinding(id uint32) *Error {
	return &Error{Message: fmt.Sprintf("combined image sampler %%%d has no descriptor set and binding to split on", id)}
}

func errCombinedSamplerParameter(id uint32) *Error {
	return &Error{Message: fmt.Sprintf("function %%%d takes a combined image sampler parameter, which naga IR cannot express", id)}
}

func errCombinedSamplerBindingTaken(declared, mapped ir.ResourceBinding) *Error {
	return &Error{Message: fmt.Sprintf("the sampler of combined image sampler (group %d, binding %d) is mapped to (group %d, binding %d), which another resource of the module already uses",
		declared.Group, declared.Binding, mapped.Group, mapped.Binding)}
}

var (
	errInvalidImageDepthStorage            = &Error{Message: "cannot create a OpTypeImage as both a depth and storage image"}
	errInvalidStorageImageWithoutFormat    = &Error{Message: "image read/write without format is not currently supported. See https://github.com/gfx-rs/wgpu/issues/6797"}
	errUnsupportedRuntimeArrayStorageClass = &Error{Message: "Naga supports OpTypeRuntimeArray in the StorageBuffer storage class only"}
	errNonBindingArrayOfImageOrSamplers    = &Error{Message: "arrays of images / samplers are supported only through bindings for now (i.e. you can't create an array of images or samplers that doesn't come from a binding)"}
)

func errUnsupportedImageDim(dim uint32) *Error {
	return &Error{Message: fmt.Sprintf("unsupported image dimension %%%d", dim)}
}

func errUnsupportedImageFormat(format uint32) *Error {
	return &Error{Message: fmt.Sprintf("unsupported image format %%%d", format)}
}

func errUnsupportedStorageClass(class uint32) *Error {
	return &Error{Message: fmt.Sprintf("unsupported storage class %%%d", class)}
}

func errUnsupportedBuiltIn(builtin uint32) *Error {
	return &Error{Message: fmt.Sprintf("unsupported builtin %%%d", builtin)}
}

func errMissingDecoration(decoration spvenums.Decoration) *Error {
	return &Error{Message: fmt.Sprintf("missing decoration %s", bareName(decoration.String(), "Decoration"))}
}

func errInvalidArraySize(id uint32) *Error {
	return &Error{Message: fmt.Sprintf("invalid array size %%%d", id)}
}

func errArrayTooLarge(count uint32) *Error {
	return &Error{Message: fmt.Sprintf("array size %d exceeds the limit %d", count, maxArrayElements)}
}

func errTooManyStorageExpressions(total int) *Error {
	return &Error{Message: fmt.Sprintf(
		"the functions need %d expressions for the module's globals and constants, more than the limit %d",
		total, maxStorageExpressions)}
}

func errUnsupportedMatrixStride(stride uint32, columns, rows uint8, width uint8) *Error {
	return &Error{Message: fmt.Sprintf("unsupported matrix stride %d for a %dx%d matrix with scalar width=%d",
		stride, columns, rows, width)}
}

func errSpecIDTooHigh(id uint32) *Error {
	return &Error{Message: fmt.Sprintf("naga only supports specialization constant IDs up to 65535 but was given %d", id)}
}

var (
	// errInvalidAccess has no Rust counterpart's expression dump: the IR has
	// no debug rendering of an expression, so it names none.
	errInvalidAccess = &Error{Message: "invalid access"}
	// errUnsupportedRowMajorMatrix is this frontend's own limit: a row-major
	// matrix is refused until its transposed load lands, never read in the
	// wrong order.
	errUnsupportedRowMajorMatrix = &Error{Message: "row-major matrix access is not supported yet"}
)

// errInvalidTerminator is Rust naga's InvalidTerminator text. Here it is the
// error for a Break, Continue or Return that structured control flow puts
// where the IR cannot express it.
var errInvalidTerminator = &Error{Message: "invalid terminator"}

// The structured control flow errors below have no Rust naga counterpart:
// Rust naga panics or builds IR its validator rejects on such input. Each is
// invalid SPIR-V, refused while it is parsed.
var errNestingTooDeep = &Error{Message: fmt.Sprintf("control flow is nested deeper than %d", maxNesting)}

// errSwitchExitsLoop is valid SPIR-V the IR cannot express without a flag
// variable: a switch case that branches to the merge block of the loop around
// the switch. Rust naga turns the branch into a switch Break, so the loop
// never exits.
var errSwitchExitsLoop = &Error{Message: "a switch case that exits the enclosing loop is not supported"}

// The switch fall-through errors refuse shapes the IR cannot express by case
// order. spirv-val rejects each of them as well; Rust naga accepts them and
// loses or misplaces case bodies.

func errMultipleFallThrough(target uint32) *Error {
	return &Error{Message: fmt.Sprintf("more than one switch case falls through into %%%d, which is not supported", target)}
}

func errFallThroughCycle(target uint32) *Error {
	return &Error{Message: fmt.Sprintf("the fall-through of switch case %%%d forms a cycle, which is not supported", target)}
}

func errFallThroughFromOutside(target uint32) *Error {
	return &Error{Message: fmt.Sprintf("a branch into switch case %%%d from outside a case construct of its switch is not supported", target)}
}

func errFallThroughIntoTwoCases(first, second uint32) *Error {
	return &Error{Message: fmt.Sprintf("a conditional branch into two switch cases %%%d and %%%d is not supported", first, second)}
}

// errMergeAtCase refuses a selection, switch or loop inside a switch case
// whose merge block is another case of that switch: the branch to the merge
// would end the construct, and the case's fall-through into the next case
// would be lost. Its header does not dominate the case, so spirv-val rejects
// it too.
func errMergeAtCase(target uint32) *Error {
	return &Error{Message: fmt.Sprintf("a construct inside a switch case merging at switch case %%%d is not supported", target)}
}

func errSwitchDefaultAtContinue(target uint32) *Error {
	return &Error{Message: fmt.Sprintf("a switch default at the enclosing loop's continue target %%%d is not supported", target)}
}

// errLoopMergeBeforeContinue is valid SPIR-V too: a loop whose merge block
// comes before its continue construct, which then ends in a conditional back
// edge. Rust naga fails on it as well, in its validator.
func errLoopMergeBeforeContinue(mergeID uint32) *Error {
	return &Error{Message: fmt.Sprintf("loop merge block %%%d comes before the loop's continue construct, which is not supported", mergeID)}
}

// errTypeTooDeep has no Rust naga counterpart either: Rust naga has no bound
// on type nesting.
var errTypeTooDeep = &Error{Message: fmt.Sprintf("type is nested deeper than %d", maxTypeDepth)}

func errUndefinedLabel(id uint32) *Error {
	return &Error{Message: fmt.Sprintf("undefined label %%%d", id)}
}

func errDuplicateLabel(id uint32) *Error {
	return &Error{Message: fmt.Sprintf("label %%%d is defined twice", id)}
}

func errDuplicateMerge(id uint32) *Error {
	return &Error{Message: fmt.Sprintf("merge block %%%d is declared twice", id)}
}

func errInvalidPhiPredecessor(id uint32) *Error {
	return &Error{Message: fmt.Sprintf("phi predecessor %%%d is not a block of the function", id)}
}

func errInvalidBranchTarget(id uint32) *Error {
	return &Error{Message: fmt.Sprintf("invalid branch target %%%d", id)}
}

func errInvalidConditionType(ty ir.TypeHandle) *Error {
	return &Error{Message: fmt.Sprintf("invalid branch condition type [%d]", ty)}
}

func errRepeatedSwitchCase(value int32) *Error {
	return &Error{Message: fmt.Sprintf("switch case %d is repeated", value)}
}

func errWrongFunctionResultType(id uint32) *Error {
	return &Error{Message: fmt.Sprintf("wrong function result type %%%d", id)}
}

func errWrongFunctionArgumentType(id uint32) *Error {
	return &Error{Message: fmt.Sprintf("wrong function argument type %%%d", id)}
}

func errFunctionCallCycle(id uint32) *Error {
	return &Error{Message: fmt.Sprintf("recursive function call %%%d", id)}
}

// errWrongArgumentCount and errDuplicateFunction have no Rust naga
// counterpart: Rust naga builds IR its validator rejects for the first and
// drops the earlier function silently for the second.
func errWrongArgumentCount(id uint32, want, got int) *Error {
	return &Error{Message: fmt.Sprintf("function %%%d takes %d arguments, the call passes %d", id, want, got)}
}

func errDuplicateFunction(id uint32) *Error {
	return &Error{Message: fmt.Sprintf("function %%%d is defined twice", id)}
}

func errInvalidParameter(op spvenums.Op) *Error {
	return &Error{Message: fmt.Sprintf("invalid parameter %s", bareName(op.String(), "Op"))}
}

func errInvalidAccessType(id uint32) *Error {
	return &Error{Message: fmt.Sprintf("invalid access type %%%d", id)}
}

func errInvalidAccessIndex(index uint32) *Error {
	return &Error{Message: fmt.Sprintf("invalid access index %%%d", index)}
}

func errInvalidAsType(ty ir.TypeHandle) *Error {
	return &Error{Message: fmt.Sprintf("invalid as type [%d]", ty)}
}

func errUnsupportedExtInstSet(id uint32) *Error {
	return &Error{Message: fmt.Sprintf("unsupported extension instantiation set %%%d", id)}
}

func errUnsupportedExtInst(id uint32) *Error {
	return &Error{Message: fmt.Sprintf("unsupported extension instantiation %%%d", id)}
}

func errUnknownBinaryOperator(op spvenums.Op) *Error {
	return &Error{Message: fmt.Sprintf("unknown binary operator %s", bareName(op.String(), "Op"))}
}

func errUnknownRelationalFunction(op spvenums.Op) *Error {
	return &Error{Message: fmt.Sprintf("unknown relational function %s", bareName(op.String(), "Op"))}
}

func errInvalidBarrierScope(id uint32) *Error {
	return &Error{Message: fmt.Sprintf("invalid barrier scope %%%d", id)}
}

func errInvalidBarrierMemorySemantics(id uint32) *Error {
	return &Error{Message: fmt.Sprintf("invalid barrier memory semantics %%%d", id)}
}
