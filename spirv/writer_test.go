package spirv

import (
	"encoding/binary"
	"testing"
)

func TestModuleBuilder_MinimalModule(t *testing.T) {
	builder := NewModuleBuilder(Version1_3)

	// Add basic capability
	builder.AddCapability(CapabilityShader)

	// Set memory model (required)
	builder.SetMemoryModel(AddressingModelLogical, MemoryModelGLSL450)

	// Build the module
	data := builder.Build()

	// Verify header (5 words = 20 bytes)
	if len(data) < 20 {
		t.Fatalf("Module too small: got %d bytes, want at least 20", len(data))
	}

	// Check magic number
	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != MagicNumber {
		t.Errorf("Invalid magic number: got 0x%08X, want 0x%08X", magic, MagicNumber)
	}

	// Check version
	version := binary.LittleEndian.Uint32(data[4:8])
	expectedVersion := uint32(1<<16 | 3<<8) // Version 1.3
	if version != expectedVersion {
		t.Errorf("Invalid version: got 0x%08X, want 0x%08X", version, expectedVersion)
	}

	// Check generator
	generator := binary.LittleEndian.Uint32(data[8:12])
	if generator != GeneratorID {
		t.Errorf("Invalid generator: got 0x%08X, want 0x%08X", generator, GeneratorID)
	}

	// Check bound (should be > 0)
	bound := binary.LittleEndian.Uint32(data[12:16])
	if bound == 0 {
		t.Error("Bound should be > 0")
	}

	// Check schema (reserved, must be 0)
	schema := binary.LittleEndian.Uint32(data[16:20])
	if schema != 0 {
		t.Errorf("Schema should be 0, got %d", schema)
	}

	t.Logf("Module size: %d bytes", len(data))
	t.Logf("Bound: %d", bound)
}

func TestModuleBuilder_WithTypes(t *testing.T) {
	builder := NewModuleBuilder(Version1_3)

	builder.AddCapability(CapabilityShader)
	builder.SetMemoryModel(AddressingModelLogical, MemoryModelGLSL450)

	// Add some types
	voidType := builder.AddTypeVoid()
	floatType := builder.AddTypeFloat(32)
	intType := builder.AddTypeInt(32, true)
	vec4Type := builder.AddTypeVector(floatType, 4)

	// Build
	data := builder.Build()

	if len(data) < 20 {
		t.Fatalf("Module too small: %d bytes", len(data))
	}

	// Verify IDs are unique and sequential
	if voidType == floatType || voidType == intType || voidType == vec4Type {
		t.Error("Type IDs should be unique")
	}

	if floatType == intType || floatType == vec4Type || intType == vec4Type {
		t.Error("Type IDs should be unique")
	}

	t.Logf("Type IDs: void=%d, float=%d, int=%d, vec4=%d", voidType, floatType, intType, vec4Type)
	t.Logf("Module size: %d bytes", len(data))
}

func TestModuleBuilder_WithEntryPoint(t *testing.T) {
	builder := NewModuleBuilder(Version1_3)

	builder.AddCapability(CapabilityShader)
	builder.SetMemoryModel(AddressingModelLogical, MemoryModelGLSL450)

	// Create function types
	voidType := builder.AddTypeVoid()
	funcType := builder.AddTypeFunction(voidType)

	// Create function
	funcID := builder.AddFunction(funcType, voidType, FunctionControlNone)
	labelID := builder.AddLabel()
	builder.AddReturn()
	builder.AddFunctionEnd()

	// Add entry point
	builder.AddEntryPoint(ExecutionModelFragment, funcID, "main", nil)
	builder.AddExecutionMode(funcID, ExecutionModeOriginUpperLeft)

	// Build
	data := builder.Build()

	if len(data) < 20 {
		t.Fatalf("Module too small: %d bytes", len(data))
	}

	// Check magic
	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != MagicNumber {
		t.Errorf("Invalid magic: 0x%08X", magic)
	}

	t.Logf("Function ID: %d, Label ID: %d", funcID, labelID)
	t.Logf("Module size: %d bytes", len(data))
}

func TestInstructionBuilder_String(t *testing.T) {
	builder := NewInstructionBuilder()
	builder.AddString("hello")

	inst := builder.Build(OpName)
	encoded := inst.Encode()

	// First word is opcode
	opcodeWord := encoded[0]
	wordCount := opcodeWord >> 16
	opcode := OpCode(opcodeWord & 0xFFFF)

	if opcode != OpName {
		t.Errorf("Wrong opcode: got %d, want %d", opcode, OpName)
	}

	// Word count includes opcode word
	if wordCount < 2 {
		t.Errorf("String should produce at least 2 words, got %d", wordCount)
	}

	t.Logf("String 'hello' encoded to %d words", wordCount)
}

func TestInstructionBuilder_Float32(t *testing.T) {
	builder := NewModuleBuilder(Version1_3)

	floatType := builder.AddTypeFloat(32)
	constID := builder.AddConstantFloat32(floatType, 3.14159)

	data := builder.Build()

	if len(data) < 20 {
		t.Fatalf("Module too small: %d bytes", len(data))
	}

	t.Logf("Float constant ID: %d", constID)
	t.Logf("Module size: %d bytes", len(data))
}

func TestModuleBuilder_IDAllocation(t *testing.T) {
	builder := NewModuleBuilder(Version1_3)

	id1 := builder.AllocID()
	id2 := builder.AllocID()
	id3 := builder.AllocID()

	if id1 >= id2 || id2 >= id3 {
		t.Error("IDs should be strictly increasing")
	}

	if id1 == 0 || id2 == 0 || id3 == 0 {
		t.Error("IDs should never be 0")
	}

	t.Logf("Allocated IDs: %d, %d, %d", id1, id2, id3)
}
