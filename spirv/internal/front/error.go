package front

import (
	"fmt"

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
	return &Error{Message: fmt.Sprintf("invalid operand count %d for %s", wc, op)}
}

func errUnsupportedInstruction(state moduleState, op spvenums.Op) *Error {
	return &Error{Message: fmt.Sprintf("unsupported instruction %s at %s", op, state)}
}

func errUnknownCapability(capability uint32) *Error {
	return &Error{Message: fmt.Sprintf("unknown capability %%%d", capability)}
}

func errUnsupportedCapability(capability spvenums.Capability) *Error {
	return &Error{Message: fmt.Sprintf("unsupported capability %s", capability)}
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
