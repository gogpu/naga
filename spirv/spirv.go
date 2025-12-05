// Package spirv provides SPIR-V code generation from naga IR.
//
// SPIR-V is the standard intermediate language for GPU shaders,
// used by Vulkan, OpenCL, and other APIs.
package spirv

import (
	"github.com/gogpu/naga/ir"
)

// Version represents a SPIR-V version.
type Version struct {
	Major uint8
	Minor uint8
}

// Common SPIR-V versions
var (
	Version1_0 = Version{1, 0}
	Version1_3 = Version{1, 3}
	Version1_4 = Version{1, 4}
	Version1_5 = Version{1, 5}
	Version1_6 = Version{1, 6}
)

// Options configures SPIR-V generation.
type Options struct {
	// Version is the SPIR-V version to target
	Version Version

	// Capabilities are additional capabilities to declare
	Capabilities []Capability

	// Debug includes debug information
	Debug bool

	// Validation enables output validation
	Validation bool
}

// DefaultOptions returns sensible default options.
func DefaultOptions() Options {
	return Options{
		Version:    Version1_3,
		Debug:      false,
		Validation: true,
	}
}

// Capability represents a SPIR-V capability.
type Capability uint32

// Common capabilities
const (
	CapabilityShader Capability = 1
	CapabilityMatrix Capability = 0 // Implied by Shader
)

// Writer generates SPIR-V from IR.
type Writer struct {
	options Options
	module  *ir.Module

	// Internal state
	nextID      uint32
	typeIDs     map[uint32]uint32
	constantIDs map[uint32]uint32
}

// NewWriter creates a new SPIR-V writer.
func NewWriter(options Options) *Writer {
	return &Writer{
		options:     options,
		nextID:      1,
		typeIDs:     make(map[uint32]uint32),
		constantIDs: make(map[uint32]uint32),
	}
}

// Write generates SPIR-V binary from IR module.
func (w *Writer) Write(module *ir.Module) ([]byte, error) {
	w.module = module

	// TODO: Implement SPIR-V generation
	// This is a placeholder for future implementation

	// Basic structure:
	// 1. Write header (magic, version, generator, bound, schema)
	// 2. Write capabilities
	// 3. Write extensions
	// 4. Write ext inst imports
	// 5. Write memory model
	// 6. Write entry points
	// 7. Write execution modes
	// 8. Write debug info
	// 9. Write decorations
	// 10. Write types and constants
	// 11. Write global variables
	// 12. Write functions

	return nil, nil
}

// SPIR-V magic number and constants
const (
	MagicNumber = 0x07230203
	GeneratorID = 0x00000000 // Unregistered generator
)

// OpCode represents a SPIR-V opcode.
type OpCode uint16

// Common opcodes
const (
	OpNop              OpCode = 0
	OpSource           OpCode = 3
	OpName             OpCode = 5
	OpMemberName       OpCode = 6
	OpExtInstImport    OpCode = 11
	OpMemoryModel      OpCode = 14
	OpEntryPoint       OpCode = 15
	OpExecutionMode    OpCode = 16
	OpCapability       OpCode = 17
	OpTypeVoid         OpCode = 19
	OpTypeBool         OpCode = 20
	OpTypeInt          OpCode = 21
	OpTypeFloat        OpCode = 22
	OpTypeVector       OpCode = 23
	OpTypeMatrix       OpCode = 24
	OpTypeArray        OpCode = 28
	OpTypeStruct       OpCode = 30
	OpTypePointer      OpCode = 32
	OpTypeFunction     OpCode = 33
	OpConstant         OpCode = 43
	OpConstantComposite OpCode = 44
	OpFunction         OpCode = 54
	OpFunctionParameter OpCode = 55
	OpFunctionEnd      OpCode = 56
	OpVariable         OpCode = 59
	OpLoad             OpCode = 61
	OpStore            OpCode = 62
	OpAccessChain      OpCode = 65
	OpDecorate         OpCode = 71
	OpMemberDecorate   OpCode = 72
	OpLabel            OpCode = 248
	OpBranch           OpCode = 249
	OpReturn           OpCode = 253
	OpReturnValue      OpCode = 254
)

// Decoration represents a SPIR-V decoration.
type Decoration uint32

// Common decorations
const (
	DecorationBlock          Decoration = 2
	DecorationColMajor       Decoration = 5
	DecorationRowMajor       Decoration = 4
	DecorationArrayStride    Decoration = 6
	DecorationMatrixStride   Decoration = 7
	DecorationBuiltIn        Decoration = 11
	DecorationLocation       Decoration = 30
	DecorationBinding        Decoration = 33
	DecorationDescriptorSet  Decoration = 34
	DecorationOffset         Decoration = 35
)
