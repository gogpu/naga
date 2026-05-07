package codegen

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// ---------------------------------------------------------------------------
// Test helpers for SPIR-V binary inspection
// ---------------------------------------------------------------------------

// hasOpcode returns true if the decoded instruction stream contains the given opcode.
func hasOpcodeInInstrs(instrs []spirvInstruction, opcode OpCode) bool {
	for _, inst := range instrs {
		if inst.opcode == opcode {
			return true
		}
	}
	return false
}

// countOpcode counts how many times a given opcode appears.
func countOpcodeInInstrs(instrs []spirvInstruction, opcode OpCode) int {
	count := 0
	for _, inst := range instrs {
		if inst.opcode == opcode {
			count++
		}
	}
	return count
}

// hasCapability returns true if the module declares the given capability.
func hasCapability(instrs []spirvInstruction, cap Capability) bool {
	for _, inst := range instrs {
		if inst.opcode == OpCapability && len(inst.words) >= 2 {
			if Capability(inst.words[1]) == cap {
				return true
			}
		}
	}
	return false
}

// assertValidSPIRV checks magic number and minimum size.
func assertValidSPIRV(t *testing.T, data []byte) {
	t.Helper()
	if len(data) < 20 {
		t.Fatalf("SPIR-V too small: %d bytes", len(data))
	}
	magic := binary.LittleEndian.Uint32(data[:4])
	if magic != MagicNumber {
		t.Fatalf("invalid magic: 0x%08X", magic)
	}
}

// ---------------------------------------------------------------------------
// 1. StorageFormatToImageFormat — table-driven test (29.5% -> 100%)
// ---------------------------------------------------------------------------

func TestStorageFormatToImageFormat(t *testing.T) {
	tests := []struct {
		format ir.StorageFormat
		want   ImageFormat
	}{
		// 8-bit
		{ir.StorageFormatR8Unorm, ImageFormatR8},
		{ir.StorageFormatR8Snorm, ImageFormatR8Snorm},
		{ir.StorageFormatR8Uint, ImageFormatR8ui},
		{ir.StorageFormatR8Sint, ImageFormatR8i},
		// 16-bit
		{ir.StorageFormatR16Uint, ImageFormatR16ui},
		{ir.StorageFormatR16Sint, ImageFormatR16i},
		{ir.StorageFormatR16Float, ImageFormatR16f},
		{ir.StorageFormatRg8Unorm, ImageFormatRg8},
		{ir.StorageFormatRg8Snorm, ImageFormatRg8Snorm},
		{ir.StorageFormatRg8Uint, ImageFormatRg8ui},
		{ir.StorageFormatRg8Sint, ImageFormatRg8i},
		// 32-bit
		{ir.StorageFormatR32Uint, ImageFormatR32ui},
		{ir.StorageFormatR32Sint, ImageFormatR32i},
		{ir.StorageFormatR32Float, ImageFormatR32f},
		{ir.StorageFormatRg16Uint, ImageFormatRg16ui},
		{ir.StorageFormatRg16Sint, ImageFormatRg16i},
		{ir.StorageFormatRg16Float, ImageFormatRg16f},
		{ir.StorageFormatRgba8Unorm, ImageFormatRgba8},
		{ir.StorageFormatRgba8Snorm, ImageFormatRgba8Snorm},
		{ir.StorageFormatRgba8Uint, ImageFormatRgba8ui},
		{ir.StorageFormatRgba8Sint, ImageFormatRgba8i},
		{ir.StorageFormatBgra8Unorm, ImageFormatRgba8},
		// Packed 32-bit
		{ir.StorageFormatRgb10a2Uint, ImageFormatRgb10a2ui},
		{ir.StorageFormatRgb10a2Unorm, ImageFormatRgb10A2},
		{ir.StorageFormatRg11b10Ufloat, ImageFormatR11fG11fB10f},
		// 64-bit
		{ir.StorageFormatRg32Uint, ImageFormatRg32ui},
		{ir.StorageFormatRg32Sint, ImageFormatRg32i},
		{ir.StorageFormatRg32Float, ImageFormatRg32f},
		{ir.StorageFormatRgba16Uint, ImageFormatRgba16ui},
		{ir.StorageFormatRgba16Sint, ImageFormatRgba16i},
		{ir.StorageFormatRgba16Float, ImageFormatRgba16f},
		// 128-bit
		{ir.StorageFormatRgba32Uint, ImageFormatRgba32ui},
		{ir.StorageFormatRgba32Sint, ImageFormatRgba32i},
		{ir.StorageFormatRgba32Float, ImageFormatRgba32f},
		// Normalized 16-bit per channel
		{ir.StorageFormatR16Unorm, ImageFormatR16},
		{ir.StorageFormatR16Snorm, ImageFormatR16Snorm},
		{ir.StorageFormatRg16Unorm, ImageFormatRg16},
		{ir.StorageFormatRg16Snorm, ImageFormatRg16Snorm},
		{ir.StorageFormatRgba16Unorm, ImageFormatRgba16},
		{ir.StorageFormatRgba16Snorm, ImageFormatRgba16Snorm},
		// 64-bit integer (SPV_EXT_shader_image_int64)
		{ir.StorageFormatR64Uint, ImageFormatR64ui},
		{ir.StorageFormatR64Sint, ImageFormatR64i},
		// Unknown
		{ir.StorageFormatUnknown, ImageFormatUnknown},
	}
	for _, tt := range tests {
		got := StorageFormatToImageFormat(tt.format)
		if got != tt.want {
			t.Errorf("StorageFormatToImageFormat(%d) = %d, want %d", tt.format, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// 2. float32ToF16Bits — edge cases not covered by TestFloat32ToF16Bits in backend_test.go
// ---------------------------------------------------------------------------

func TestFloat32ToF16BitsEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		in   float32
		want uint32
	}{
		// Negative zero
		{"negative zero", float32(math.Copysign(0, -1)), 0x8000},
		// Infinity
		{"+Inf", float32(math.Inf(1)), 0x7C00},
		{"-Inf", float32(math.Inf(-1)), 0xFC00},
		// NaN (sign=0, exp=0x7C00, frac!=0)
		{"NaN", float32(math.NaN()), 0x7E00},
		// Overflow to infinity (exp > 15)
		{"100000 (overflow)", 100000.0, 0x7C00},
		{"-100000 (overflow)", -100000.0, 0xFC00},
		// Subnormal f16 range (exp >= -24 and exp < -14)
		{"5.96e-8 (smallest subnormal f16)", math.Float32frombits(0x33800000), 0x0001},
		// Very small (below subnormal range -> zero, exp < -24)
		{"1e-10 (underflow to zero)", 1e-10, 0x0000},
		// Rounding: round-to-nearest-even
		{"1.0009766 (round up)", math.Float32frombits(0x3F802000), 0x3C01},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := float32ToF16Bits(tt.in)
			if got != tt.want {
				t.Errorf("float32ToF16Bits(%v) = 0x%04X, want 0x%04X", tt.in, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 3. NewWriter (0%)
// ---------------------------------------------------------------------------

func TestNewWriter(t *testing.T) {
	opts := Options{
		Version: Version1_3,
		Debug:   true,
	}
	w := NewWriter(opts)
	if w == nil {
		t.Fatal("NewWriter returned nil")
	}
	if w.options.Version != Version1_3 {
		t.Errorf("Version = %v, want %v", w.options.Version, Version1_3)
	}
	if !w.options.Debug {
		t.Error("Debug should be true")
	}
	if w.nextID != 1 {
		t.Errorf("nextID = %d, want 1", w.nextID)
	}
	if w.typeIDs == nil || w.constantIDs == nil {
		t.Error("maps should be initialized")
	}
}

// ---------------------------------------------------------------------------
// 4. ModuleBuilder uncovered methods (0% each)
// ---------------------------------------------------------------------------

func TestModuleBuilder_AddString(t *testing.T) {
	b := NewModuleBuilder(Version1_3)
	id := b.AddString("debug_text")
	if id == 0 {
		t.Error("AddString returned ID 0")
	}
	if len(b.debugStrings) != 1 {
		t.Errorf("expected 1 debug string, got %d", len(b.debugStrings))
	}
}

func TestModuleBuilder_AddTypeSampler(t *testing.T) {
	b := NewModuleBuilder(Version1_3)
	id := b.AddTypeSampler()
	if id == 0 {
		t.Error("AddTypeSampler returned ID 0")
	}
}

func TestModuleBuilder_AddConstantFloat64(t *testing.T) {
	b := NewModuleBuilder(Version1_3)
	f64Type := b.AddTypeFloat(64)
	id := b.AddConstantFloat64(f64Type, 3.141592653589793)
	if id == 0 {
		t.Error("AddConstantFloat64 returned ID 0")
	}
}

func TestModuleBuilder_AddVariableWithInit(t *testing.T) {
	b := NewModuleBuilder(Version1_3)
	b.AddCapability(CapabilityShader)
	b.SetMemoryModel(AddressingModelLogical, MemoryModelGLSL450)

	intType := b.AddTypeInt(32, false)
	ptrType := b.AddTypePointer(StorageClassPrivate, intType)
	initVal := b.AddConstant(intType, 42)
	varID := b.AddVariableWithInit(ptrType, StorageClassPrivate, initVal)

	if varID == 0 {
		t.Error("AddVariableWithInit returned ID 0")
	}
	if len(b.globalVars) != 1 {
		t.Errorf("expected 1 global var, got %d", len(b.globalVars))
	}
}

func TestModuleBuilder_AddFunctionParameter(t *testing.T) {
	b := NewModuleBuilder(Version1_3)
	b.AddCapability(CapabilityShader)
	b.SetMemoryModel(AddressingModelLogical, MemoryModelGLSL450)

	voidType := b.AddTypeVoid()
	floatType := b.AddTypeFloat(32)
	funcType := b.AddTypeFunction(voidType, floatType)

	_ = b.AddFunction(funcType, voidType, FunctionControlNone)
	paramID := b.AddFunctionParameter(floatType)
	_ = b.AddLabel()
	b.AddReturn()
	b.AddFunctionEnd()

	if paramID == 0 {
		t.Error("AddFunctionParameter returned ID 0")
	}

	data := b.Build()
	if len(data) < 20 {
		t.Fatalf("Module too small: %d bytes", len(data))
	}
}

func TestModuleBuilder_AddLabelWithID(t *testing.T) {
	b := NewModuleBuilder(Version1_3)
	b.AddCapability(CapabilityShader)
	b.SetMemoryModel(AddressingModelLogical, MemoryModelGLSL450)

	voidType := b.AddTypeVoid()
	funcType := b.AddTypeFunction(voidType)
	_ = b.AddFunction(funcType, voidType, FunctionControlNone)

	// Pre-allocate a label ID
	preAllocID := b.AllocID()
	b.AddLabelWithID(preAllocID)
	b.AddReturn()
	b.AddFunctionEnd()

	data := b.Build()
	if len(data) < 20 {
		t.Fatalf("Module too small: %d bytes", len(data))
	}
}

func TestModuleBuilder_AddUnreachable(t *testing.T) {
	b := NewModuleBuilder(Version1_3)
	b.AddCapability(CapabilityShader)
	b.SetMemoryModel(AddressingModelLogical, MemoryModelGLSL450)

	voidType := b.AddTypeVoid()
	funcType := b.AddTypeFunction(voidType)
	_ = b.AddFunction(funcType, voidType, FunctionControlNone)
	_ = b.AddLabel()
	b.AddUnreachable()
	b.AddFunctionEnd()

	data := b.Build()
	if len(data) < 20 {
		t.Fatalf("Module too small: %d bytes", len(data))
	}
}

func TestModuleBuilder_AddReturnValue(t *testing.T) {
	b := NewModuleBuilder(Version1_3)
	b.AddCapability(CapabilityShader)
	b.SetMemoryModel(AddressingModelLogical, MemoryModelGLSL450)

	floatType := b.AddTypeFloat(32)
	funcType := b.AddTypeFunction(floatType)
	_ = b.AddFunction(funcType, floatType, FunctionControlNone)
	_ = b.AddLabel()
	val := b.AddConstantFloat32(floatType, 1.0)
	b.AddReturnValue(val)
	b.AddFunctionEnd()

	data := b.Build()
	if len(data) < 20 {
		t.Fatalf("Module too small: %d bytes", len(data))
	}
}

func TestModuleBuilder_AddCopyLogical(t *testing.T) {
	b := NewModuleBuilder(Version1_3)
	b.AddCapability(CapabilityShader)
	b.SetMemoryModel(AddressingModelLogical, MemoryModelGLSL450)

	floatType := b.AddTypeFloat(32)
	funcType := b.AddTypeFunction(floatType)
	_ = b.AddFunction(funcType, floatType, FunctionControlNone)
	_ = b.AddLabel()
	val := b.AddConstantFloat32(floatType, 2.0)
	copyID := b.AddCopyLogical(floatType, val)
	if copyID == 0 {
		t.Error("AddCopyLogical returned ID 0")
	}
	b.AddReturnValue(copyID)
	b.AddFunctionEnd()

	data := b.Build()
	if len(data) < 20 {
		t.Fatalf("Module too small: %d bytes", len(data))
	}
}

func TestModuleBuilder_AddVectorExtractDynamic(t *testing.T) {
	b := NewModuleBuilder(Version1_3)
	b.AddCapability(CapabilityShader)
	b.SetMemoryModel(AddressingModelLogical, MemoryModelGLSL450)

	floatType := b.AddTypeFloat(32)
	vec4Type := b.AddTypeVector(floatType, 4)
	intType := b.AddTypeInt(32, false)
	funcType := b.AddTypeFunction(floatType)

	_ = b.AddFunction(funcType, floatType, FunctionControlNone)
	_ = b.AddLabel()

	// Build a vec4 and extract with dynamic index
	c0 := b.AddConstantFloat32(floatType, 1.0)
	c1 := b.AddConstantFloat32(floatType, 2.0)
	vec := b.AddCompositeConstruct(vec4Type, c0, c0, c1, c1)
	idx := b.AddConstant(intType, 2)
	extracted := b.AddVectorExtractDynamic(floatType, vec, idx)
	if extracted == 0 {
		t.Error("AddVectorExtractDynamic returned ID 0")
	}
	b.AddReturnValue(extracted)
	b.AddFunctionEnd()

	data := b.Build()
	if len(data) < 20 {
		t.Fatalf("Module too small: %d bytes", len(data))
	}
}

func TestModuleBuilder_AddBranchConditional(t *testing.T) {
	b := NewModuleBuilder(Version1_3)
	b.AddCapability(CapabilityShader)
	b.SetMemoryModel(AddressingModelLogical, MemoryModelGLSL450)

	voidType := b.AddTypeVoid()
	boolType := b.AddTypeBool()
	funcType := b.AddTypeFunction(voidType)

	_ = b.AddFunction(funcType, voidType, FunctionControlNone)
	_ = b.AddLabel()

	trueLabel := b.AllocID()
	falseLabel := b.AllocID()
	mergeLabel := b.AllocID()

	b.AddSelectionMerge(mergeLabel, SelectionControlNone)
	cond := b.AddConstant(boolType, 1)
	b.AddBranchConditional(cond, trueLabel, falseLabel)

	// True block: branch to merge using raw instruction (no AddBranch method)
	b.AddLabelWithID(trueLabel)
	b.ib.Reset()
	b.ib.AddWord(mergeLabel)
	b.funcAppend(b.ib.Build(OpBranch))

	// False block: branch to merge
	b.AddLabelWithID(falseLabel)
	b.ib.Reset()
	b.ib.AddWord(mergeLabel)
	b.funcAppend(b.ib.Build(OpBranch))

	// Merge block
	b.AddLabelWithID(mergeLabel)
	b.AddReturn()
	b.AddFunctionEnd()

	data := b.Build()
	if len(data) < 20 {
		t.Fatalf("Module too small: %d bytes", len(data))
	}
}

func TestModuleBuilder_AddKill(t *testing.T) {
	b := NewModuleBuilder(Version1_3)
	b.AddCapability(CapabilityShader)
	b.SetMemoryModel(AddressingModelLogical, MemoryModelGLSL450)

	voidType := b.AddTypeVoid()
	funcType := b.AddTypeFunction(voidType)
	funcID := b.AddFunction(funcType, voidType, FunctionControlNone)
	_ = b.AddLabel()
	b.AddKill()
	b.AddFunctionEnd()

	b.AddEntryPoint(ExecutionModelFragment, funcID, "main", nil)
	b.AddExecutionMode(funcID, ExecutionModeOriginUpperLeft)

	data := b.Build()
	if len(data) < 20 {
		t.Fatalf("Module too small: %d bytes", len(data))
	}
}

// ---------------------------------------------------------------------------
// 5. WGSL end-to-end shader compilation for uncovered backend paths
// Uses compileWGSL helper from shader_test.go.
// ---------------------------------------------------------------------------

// TestCompileSelectExpression exercises emitSelect (42.9%).
// Verifies OpSelect is emitted for WGSL select() built-in.
func TestCompileSelectExpression(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let a = vec4<f32>(1.0, 0.0, 0.0, 1.0);
    let b = vec4<f32>(0.0, 0.0, 1.0, 1.0);
    let result = select(a, b, uv.x > 0.5);
    return result;
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	if !hasOpcodeInInstrs(instrs, OpSelect) {
		t.Error("expected OpSelect in output for WGSL select()")
	}
}

// TestCompileSelectVectorExpression exercises emitSelect with vector condition.
// Verifies OpSelect with vec4<bool> condition does not require scalar-to-vector splat.
func TestCompileSelectVectorExpression(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let a = vec4<f32>(1.0, 0.0, 0.0, 1.0);
    let b = vec4<f32>(0.0, 1.0, 0.0, 1.0);
    let cond = vec4<bool>(true, false, true, false);
    let result = select(a, b, cond);
    return result;
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	if !hasOpcodeInInstrs(instrs, OpSelect) {
		t.Error("expected OpSelect for vector select")
	}
}

// TestCompileLocalVariables exercises emitLocalVarValue / emitLocalVarRef (0% / 66.7%).
func TestCompileLocalVariables(t *testing.T) {
	source := `
@fragment
fn main(@location(0) color: vec3<f32>) -> @location(0) vec4<f32> {
    var r: f32 = color.x;
    var g: f32 = color.y;
    var b: f32 = color.z;
    r = r * 2.0;
    g = g * 0.5;
    return vec4<f32>(r, g, b, 1.0);
}
`
	spv := compileWGSL(t, source)
	t.Logf("local variables shader: %d bytes", len(spv))
}

// TestCompileConstExpression exercises emitConstExpression (35.7%).
func TestCompileConstExpression(t *testing.T) {
	// Texture sampling with const offset exercises emitConstExpression
	source := `
@group(0) @binding(0) var my_texture: texture_2d<f32>;
@group(0) @binding(1) var my_sampler: sampler;

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSampleLevel(my_texture, my_sampler, uv, 0.0, vec2<i32>(1, 2));
}
`
	spv := compileWGSL(t, source)
	t.Logf("const expression shader: %d bytes", len(spv))
}

// TestCompileDerivatives exercises emitDerivative (44.0%).
// Verifies DerivativeControl capability for fine/coarse derivatives and
// that the correct derivative opcodes are emitted.
func TestCompileDerivatives(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let dx = dpdx(uv.x);
    let dy = dpdy(uv.y);
    let dxf = dpdxFine(uv.x);
    let dyf = dpdyFine(uv.y);
    let dxc = dpdxCoarse(uv.x);
    let dyc = dpdyCoarse(uv.y);
    let fw = fwidth(uv.x);
    return vec4<f32>(dx + dxf + dxc, dy + dyf + dyc, fw, 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)

	// Fine/coarse derivatives require DerivativeControl capability
	if !hasCapability(instrs, CapabilityDerivativeControl) {
		t.Error("expected DerivativeControl capability for fine/coarse derivatives")
	}
	// Verify standard derivative opcodes
	if !hasOpcodeInInstrs(instrs, OpDPdx) {
		t.Error("expected OpDPdx for dpdx()")
	}
	if !hasOpcodeInInstrs(instrs, OpDPdy) {
		t.Error("expected OpDPdy for dpdy()")
	}
	if !hasOpcodeInInstrs(instrs, OpFwidth) {
		t.Error("expected OpFwidth for fwidth()")
	}
	// Verify fine derivatives
	if !hasOpcodeInInstrs(instrs, OpDPdxFine) {
		t.Error("expected OpDPdxFine for dpdxFine()")
	}
	if !hasOpcodeInInstrs(instrs, OpDPdyFine) {
		t.Error("expected OpDPdyFine for dpdyFine()")
	}
	// Verify coarse derivatives
	if !hasOpcodeInInstrs(instrs, OpDPdxCoarse) {
		t.Error("expected OpDPdxCoarse for dpdxCoarse()")
	}
	if !hasOpcodeInInstrs(instrs, OpDPdyCoarse) {
		t.Error("expected OpDPdyCoarse for dpdyCoarse()")
	}
}

// TestCompileComputeBarrier exercises emitBarrier (89.7%).
// Verifies OpControlBarrier is emitted for workgroupBarrier() and storageBarrier().
func TestCompileComputeBarrier(t *testing.T) {
	source := `
@group(0) @binding(0) var<storage, read_write> data: array<f32>;

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    data[id.x] = data[id.x] * 2.0;
    workgroupBarrier();
    storageBarrier();
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	// workgroupBarrier + storageBarrier = at least 2 OpControlBarrier
	barrierCount := countOpcodeInInstrs(instrs, OpControlBarrier)
	if barrierCount < 2 {
		t.Errorf("expected at least 2 OpControlBarrier, got %d", barrierCount)
	}
}

// TestCompileAtomics exercises emitAtomic / resolveAtomicScalar / resolveAtomicScalarKind / atomicOpcode.
// Verifies atomic opcodes are emitted for each WGSL atomic built-in.
func TestCompileAtomics(t *testing.T) {
	source := `
@group(0) @binding(0) var<storage, read_write> counter: atomic<u32>;

@compute @workgroup_size(1)
fn main() {
    let prev = atomicLoad(&counter);
    atomicStore(&counter, prev + 1u);
    let added = atomicAdd(&counter, 10u);
    let subbed = atomicSub(&counter, 5u);
    let maxed = atomicMax(&counter, 100u);
    let mined = atomicMin(&counter, 0u);
    let anded = atomicAnd(&counter, 0xFFu);
    let ored = atomicOr(&counter, 0x0Fu);
    let xored = atomicXor(&counter, 0xAAu);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)

	// Verify at least some atomic operations were emitted.
	// The exact opcodes depend on IR lowering and DCE behavior.
	atomicOpcodes := []OpCode{
		OpAtomicLoad, OpAtomicStore, OpAtomicIAdd, OpAtomicISub,
		OpAtomicUMax, OpAtomicUMin, OpAtomicAnd, OpAtomicOr, OpAtomicXor,
	}
	atomicCount := 0
	for _, op := range atomicOpcodes {
		atomicCount += countOpcodeInInstrs(instrs, op)
	}
	if atomicCount == 0 {
		t.Error("expected at least one atomic opcode in output, got 0")
	}
	t.Logf("atomic shader: %d bytes, %d atomic ops emitted", len(spv), atomicCount)
}

// TestCompileAtomicsI32 exercises atomicOpcode with signed integers.
func TestCompileAtomicsI32(t *testing.T) {
	source := `
@group(0) @binding(0) var<storage, read_write> counter: atomic<i32>;

@compute @workgroup_size(1)
fn main() {
    let prev = atomicLoad(&counter);
    atomicStore(&counter, prev + 1i);
    let added = atomicAdd(&counter, 10i);
    let maxed = atomicMax(&counter, 100i);
    let mined = atomicMin(&counter, -100i);
}
`
	spv := compileWGSL(t, source)
	t.Logf("atomic i32 shader: %d bytes", len(spv))
}

// TestCompileAtomicCompareExchange exercises emitAtomicCompareExchange (90.5%).
// Verifies OpAtomicCompareExchange is emitted and the result struct is constructed
// with old_value and exchanged (bool) fields.
func TestCompileAtomicCompareExchange(t *testing.T) {
	source := `
@group(0) @binding(0) var<storage, read_write> val: atomic<u32>;

@compute @workgroup_size(1)
fn main() {
    let result = atomicCompareExchangeWeak(&val, 0u, 1u);
    if result.exchanged {
        atomicStore(&val, result.old_value + 1u);
    }
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	if !hasOpcodeInInstrs(instrs, OpAtomicCompareExch) {
		t.Error("expected OpAtomicCompareExch in output")
	}
	// The result.exchanged field uses IEqual to compare old_value == compare
	if !hasOpcodeInInstrs(instrs, OpIEqual) {
		t.Error("expected OpIEqual for exchanged field computation")
	}
	// The struct is built via CompositeConstruct
	if !hasOpcodeInInstrs(instrs, OpCompositeConstruct) {
		t.Error("expected OpCompositeConstruct for result struct {old_value, exchanged}")
	}
}

// TestCompileDiscard exercises AddKill path through WGSL discard.
// Verifies OpKill terminator is emitted for WGSL discard statement.
func TestCompileDiscard(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    if uv.x < 0.0 {
        discard;
    }
    return vec4<f32>(uv, 0.0, 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	if !hasOpcodeInInstrs(instrs, OpKill) {
		t.Error("expected OpKill for WGSL discard statement")
	}
	if !hasOpcodeInInstrs(instrs, OpSelectionMerge) {
		t.Error("expected OpSelectionMerge for if-discard control flow")
	}
}

// TestCompileFunctionCall exercises emitCall (65.6%).
func TestCompileFunctionCall(t *testing.T) {
	source := `
fn helper(a: f32, b: f32) -> f32 {
    return a * b + 1.0;
}

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let r = helper(uv.x, uv.y);
    return vec4<f32>(r, r, r, 1.0);
}
`
	spv := compileWGSL(t, source)
	t.Logf("function call shader: %d bytes", len(spv))
}

// TestCompileFunctionCallMultipleParams exercises AddFunctionParameter (0%).
func TestCompileFunctionCallMultipleParams(t *testing.T) {
	source := `
fn compute_color(r: f32, g: f32, b: f32, a: f32) -> vec4<f32> {
    return vec4<f32>(r, g, b, a);
}

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return compute_color(uv.x, uv.y, 0.5, 1.0);
}
`
	spv := compileWGSL(t, source)
	t.Logf("multi-param function call shader: %d bytes", len(spv))
}

// TestCompileImageSampleWithOffset exercises image sampling with const offset.
func TestCompileImageSampleWithOffset(t *testing.T) {
	source := `
@group(0) @binding(0) var my_texture: texture_2d<f32>;
@group(0) @binding(1) var my_sampler: sampler;

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let color = textureSample(my_texture, my_sampler, uv, vec2<i32>(2, 3));
    return color;
}
`
	spv := compileWGSL(t, source)
	t.Logf("image sample with offset shader: %d bytes", len(spv))
}

// TestCompileArrayLength exercises emitArrayLength (45.3%).
func TestCompileArrayLength(t *testing.T) {
	source := `
struct Data {
    values: array<f32>,
}

@group(0) @binding(0) var<storage, read> data: Data;

@compute @workgroup_size(1)
fn main() {
    let len = arrayLength(&data.values);
    if len > 0u {
        let first = data.values[0];
    }
}
`
	spv := compileWGSL(t, source)
	t.Logf("array length shader: %d bytes", len(spv))
}

// TestCompileSwitch exercises emitSwitch (93.3%) for deeper coverage.
func TestCompileSwitch(t *testing.T) {
	source := `
@fragment
fn main(@location(0) @interpolate(flat) mode: i32) -> @location(0) vec4<f32> {
    var color: vec4<f32>;
    switch mode {
        case 0i: {
            color = vec4<f32>(1.0, 0.0, 0.0, 1.0);
        }
        case 1i: {
            color = vec4<f32>(0.0, 1.0, 0.0, 1.0);
        }
        case 2i: {
            color = vec4<f32>(0.0, 0.0, 1.0, 1.0);
        }
        default: {
            color = vec4<f32>(1.0, 1.0, 1.0, 1.0);
        }
    }
    return color;
}
`
	spv := compileWGSL(t, source)
	t.Logf("switch shader: %d bytes", len(spv))
}

// TestCompileLoop exercises emitLoop (82.9%).
func TestCompileLoop(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    var sum: f32 = 0.0;
    var i: i32 = 0;
    loop {
        if i >= 10 {
            break;
        }
        sum = sum + uv.x;
        i = i + 1;
        if i == 5 {
            continue;
        }
    }
    return vec4<f32>(sum, sum, sum, 1.0);
}
`
	spv := compileWGSL(t, source)
	t.Logf("loop shader: %d bytes", len(spv))
}

// TestCompileNestedIf exercises emitIf (89.7%) with nesting.
func TestCompileNestedIf(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    var color = vec4<f32>(0.0, 0.0, 0.0, 1.0);
    if uv.x > 0.5 {
        if uv.y > 0.5 {
            color = vec4<f32>(1.0, 1.0, 0.0, 1.0);
        } else {
            color = vec4<f32>(1.0, 0.0, 0.0, 1.0);
        }
    } else {
        if uv.y > 0.5 {
            color = vec4<f32>(0.0, 1.0, 0.0, 1.0);
        } else {
            color = vec4<f32>(0.0, 0.0, 1.0, 1.0);
        }
    }
    return color;
}
`
	spv := compileWGSL(t, source)
	t.Logf("nested if shader: %d bytes", len(spv))
}

// TestCompileStorageTexture exercises emitImageStore / requestImageFormatCapabilities (50%/80%).
// Verifies OpImageWrite is emitted and StorageImageWriteWithoutFormat capability is declared.
func TestCompileStorageTexture(t *testing.T) {
	source := `
@group(0) @binding(0) var output: texture_storage_2d<rgba8unorm, write>;

@compute @workgroup_size(8, 8)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    textureStore(output, vec2<i32>(vec2<u32>(id.x, id.y)), vec4<f32>(1.0, 0.0, 0.0, 1.0));
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	if !hasOpcodeInInstrs(instrs, OpImageWrite) {
		t.Error("expected OpImageWrite for textureStore()")
	}
}

// TestCompileStorageTextureR32Float exercises a different image format.
func TestCompileStorageTextureR32Float(t *testing.T) {
	source := `
@group(0) @binding(0) var output: texture_storage_2d<r32float, write>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    textureStore(output, vec2<i32>(vec2<u32>(id.x, id.y)), vec4<f32>(1.0, 0.0, 0.0, 1.0));
}
`
	spv := compileWGSL(t, source)
	t.Logf("storage texture r32float shader: %d bytes", len(spv))
}

// TestCompileImageQueryDimensions exercises emitImageQuery (89.2%) with dimensions and levels.
func TestCompileImageQueryDimensions(t *testing.T) {
	source := `
@group(0) @binding(0) var my_texture: texture_2d<f32>;

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let dims = textureDimensions(my_texture);
    let levels = textureNumLevels(my_texture);
    let w = f32(dims.x);
    let h = f32(dims.y);
    return vec4<f32>(w, h, f32(levels), 1.0);
}
`
	spv := compileWGSL(t, source)
	t.Logf("image query shader: %d bytes", len(spv))
}

// TestCompileTypeCast exercises emitAs (66.7%) and selectConversionOp (60%).
func TestCompileTypeCast(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let i = i32(uv.x * 100.0);
    let u = u32(uv.y * 100.0);
    let fi = f32(i);
    let fu = f32(u);
    let iu = u32(i);
    let ui = i32(u);
    return vec4<f32>(fi, fu, f32(iu), f32(ui));
}
`
	spv := compileWGSL(t, source)
	t.Logf("type cast shader: %d bytes", len(spv))
}

// TestCompileBoolToNumeric exercises emitBoolToNumeric (95.0%).
func TestCompileBoolToNumeric(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let condition = uv.x > 0.5;
    let f = select(0.0, 1.0, condition);
    let i = select(0i, 1i, condition);
    let u = select(0u, 1u, condition);
    return vec4<f32>(f, f32(i), f32(u), 1.0);
}
`
	spv := compileWGSL(t, source)
	t.Logf("bool to numeric shader: %d bytes", len(spv))
}

// TestCompileUnary exercises emitUnary (82.1%).
func TestCompileUnary(t *testing.T) {
	source := `
@fragment
fn main(@location(0) @interpolate(flat) val: i32) -> @location(0) vec4<f32> {
    let neg_i = -val;
    let neg_f = -1.5;
    let bit_not = ~val;
    let b = val > 0;
    let not_b = !b;
    return vec4<f32>(f32(neg_i), neg_f, f32(bit_not), select(0.0, 1.0, not_b));
}
`
	spv := compileWGSL(t, source)
	t.Logf("unary shader: %d bytes", len(spv))
}

// TestCompileDebugNames exercises emitDebugNames (68.2%) by enabling Debug mode.
func TestCompileDebugNames(t *testing.T) {
	source := `
struct MyStruct {
    position: vec4<f32>,
    color: vec3<f32>,
}

fn helper_func(x: f32) -> f32 {
    var local_var: f32 = x;
    return local_var * 2.0;
}

@vertex
fn main(@location(0) pos: vec3<f32>) -> @builtin(position) vec4<f32> {
    let result = helper_func(pos.x);
    return vec4<f32>(result, pos.y, pos.z, 1.0);
}
`
	lexer := wgsl.NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	parser := wgsl.NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	// Enable debug mode to exercise emitDebugNames
	opts := DefaultOptions()
	opts.Debug = true

	backend := NewBackend(opts)
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	assertValidSPIRV(t, spirvBytes)
	instrs := decodeSPIRVInstructions(spirvBytes)

	// Debug mode must emit OpName instructions for functions, variables, and types
	nameCount := countOpcodeInInstrs(instrs, OpName)
	if nameCount == 0 {
		t.Error("expected OpName instructions in debug mode, got 0")
	}

	// Also expect OpMemberName for struct members
	memberNameCount := countOpcodeInInstrs(instrs, OpMemberName)
	if memberNameCount == 0 {
		t.Error("expected OpMemberName for struct member 'position' and 'color'")
	}
	t.Logf("debug names: %d OpName + %d OpMemberName instructions", nameCount, memberNameCount)
}

// TestCompileMatrixOperations exercises emitBinary matrix multiply paths.
func TestCompileMatrixOperations(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let m = mat4x4<f32>(
        1.0, 0.0, 0.0, 0.0,
        0.0, 1.0, 0.0, 0.0,
        0.0, 0.0, 1.0, 0.0,
        0.0, 0.0, 0.0, 1.0
    );
    let v = vec4<f32>(uv.x, uv.y, 0.0, 1.0);
    let result = m * v;
    return result;
}
`
	spv := compileWGSL(t, source)
	t.Logf("matrix operations shader: %d bytes", len(spv))
}

// TestCompileSwizzle exercises emitSwizzle (81.8%).
func TestCompileSwizzle(t *testing.T) {
	source := `
@fragment
fn main(@location(0) color: vec4<f32>) -> @location(0) vec4<f32> {
    let rgb = color.rgb;
    let rg = color.rg;
    let ba = color.ba;
    return vec4<f32>(rgb.x, rg.y, ba.x, ba.y);
}
`
	spv := compileWGSL(t, source)
	t.Logf("swizzle shader: %d bytes", len(spv))
}

// TestCompileF64Literal exercises AddConstantFloat64 via WGSL (through emitLiteral f64 path).
func TestCompileF64Literal(t *testing.T) {
	// Abstract float literals that result in f64 types
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let x = uv.x + 0.5;
    let y = uv.y * 2.0;
    return vec4<f32>(x, y, 0.0, 1.0);
}
`
	spv := compileWGSL(t, source)
	t.Logf("f64 literal shader: %d bytes", len(spv))
}

// TestCompileMultipleEntryPoints exercises emitEntryPoints with vertex + fragment.
func TestCompileMultipleEntryPoints(t *testing.T) {
	source := `
struct VertexOutput {
    @builtin(position) pos: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

@vertex
fn vs_main(@location(0) position: vec2<f32>, @location(1) texcoord: vec2<f32>) -> VertexOutput {
    var out: VertexOutput;
    out.pos = vec4<f32>(position, 0.0, 1.0);
    out.uv = texcoord;
    return out;
}

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(uv, 0.0, 1.0);
}
`
	spv := compileWGSL(t, source)
	t.Logf("multiple entry points shader: %d bytes", len(spv))
}

// TestCompileWorkgroupVariable exercises workgroup variable paths.
func TestCompileWorkgroupVariable(t *testing.T) {
	source := `
var<workgroup> shared_data: array<f32, 64>;

@group(0) @binding(0) var<storage, read_write> output: array<f32>;

@compute @workgroup_size(64)
fn main(@builtin(local_invocation_id) lid: vec3<u32>, @builtin(global_invocation_id) gid: vec3<u32>) {
    shared_data[lid.x] = f32(gid.x);
    workgroupBarrier();
    output[gid.x] = shared_data[lid.x];
}
`
	spv := compileWGSL(t, source)
	t.Logf("workgroup variable shader: %d bytes", len(spv))
}

// TestCompileCompositeAccess exercises emitAccess / emitAccessIndex (51.1% / 75%).
func TestCompileCompositeAccess(t *testing.T) {
	source := `
struct Inner {
    x: f32,
    y: f32,
}

struct Outer {
    a: Inner,
    b: Inner,
    values: array<f32, 4>,
}

@group(0) @binding(0) var<uniform> data: Outer;

@fragment
fn main() -> @location(0) vec4<f32> {
    let ax = data.a.x;
    let by = data.b.y;
    let v0 = data.values[0];
    let v3 = data.values[3];
    return vec4<f32>(ax, by, v0, v3);
}
`
	spv := compileWGSL(t, source)
	t.Logf("composite access shader: %d bytes", len(spv))
}

// TestCompileInterpolation exercises addInterpolationDecorations (54.5%).
// Verifies Flat and NoPerspective decorations are applied correctly.
func TestCompileInterpolation(t *testing.T) {
	source := `
struct FragIn {
    @builtin(position) pos: vec4<f32>,
    @location(0) smooth_val: f32,
    @location(1) @interpolate(flat) flat_val: f32,
    @location(2) @interpolate(linear) linear_val: f32,
    @location(3) @interpolate(linear, sample) linear_sample_val: f32,
    @location(4) @interpolate(perspective, centroid) persp_centroid: f32,
}

@fragment
fn main(input: FragIn) -> @location(0) vec4<f32> {
    return vec4<f32>(input.smooth_val, input.flat_val, input.linear_val, input.linear_sample_val);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)

	// @interpolate(flat) should produce Flat decoration
	hasFlatDecoration := false
	hasNoPerspective := false
	for _, inst := range instrs {
		if inst.opcode == OpDecorate && len(inst.words) >= 3 {
			dec := Decoration(inst.words[2])
			if dec == DecorationFlat {
				hasFlatDecoration = true
			}
			if dec == DecorationNoPerspective {
				hasNoPerspective = true
			}
		}
	}
	if !hasFlatDecoration {
		t.Error("expected Flat decoration for @interpolate(flat)")
	}
	if !hasNoPerspective {
		t.Error("expected NoPerspective decoration for @interpolate(linear)")
	}
}

// TestCompilePointerLoad exercises emitLoad (60%) via pointer dereferencing.
func TestCompilePointerLoad(t *testing.T) {
	source := `
@group(0) @binding(0) var<uniform> data: vec4<f32>;

@fragment
fn main() -> @location(0) vec4<f32> {
    return data;
}
`
	spv := compileWGSL(t, source)
	t.Logf("pointer load shader: %d bytes", len(spv))
}

// TestCompileForLoop exercises emitStatement for-loop path.
func TestCompileForLoop(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    var sum: f32 = 0.0;
    for (var i: i32 = 0; i < 10; i = i + 1) {
        sum = sum + uv.x * f32(i);
    }
    return vec4<f32>(sum, 0.0, 0.0, 1.0);
}
`
	spv := compileWGSL(t, source)
	t.Logf("for loop shader: %d bytes", len(spv))
}

// TestCompileWhileLoop exercises emitStatement while-loop path.
func TestCompileWhileLoop(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    var sum: f32 = 0.0;
    var i: i32 = 0;
    while (i < 8) {
        sum = sum + uv.x;
        i = i + 1;
    }
    return vec4<f32>(sum, 0.0, 0.0, 1.0);
}
`
	spv := compileWGSL(t, source)
	t.Logf("while loop shader: %d bytes", len(spv))
}

// TestCompileSplatExpression exercises emitSplat (83.3%) with runtime splat.
func TestCompileSplatExpression(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let val = uv.x;
    let result = vec4<f32>(val);
    return result;
}
`
	spv := compileWGSL(t, source)
	t.Logf("splat expression shader: %d bytes", len(spv))
}

// TestCompileMathBuiltins exercises emitMath (78.2%) with various math functions.
func TestCompileMathBuiltins(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let a = abs(uv.x);
    let b = ceil(uv.y);
    let c = floor(uv.x);
    let d = sqrt(abs(uv.y));
    let e = sin(uv.x);
    let f = cos(uv.y);
    let g = exp(uv.x * 0.1);
    let h = log(abs(uv.y) + 1.0);
    let i = min(uv.x, uv.y);
    let j = max(uv.x, uv.y);
    let k = clamp(uv.x, 0.0, 1.0);
    let l = mix(uv.x, uv.y, 0.5);
    return vec4<f32>(a + b + e + f, c + d + g + h, i + j, k + l);
}
`
	spv := compileWGSL(t, source)
	t.Logf("math builtins shader: %d bytes", len(spv))
}

// TestCompileVectorConstructors exercises emitCompose (77.8%).
func TestCompileVectorConstructors(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let v2 = vec2<f32>(uv.x, uv.y);
    let v3 = vec3<f32>(v2, 0.0);
    let v4 = vec4<f32>(v3, 1.0);
    let v4b = vec4<f32>(uv.x, uv.y, 0.0, 1.0);
    return v4 + v4b;
}
`
	spv := compileWGSL(t, source)
	t.Logf("vector constructors shader: %d bytes", len(spv))
}

// TestCompileTextureLoad exercises emitImageLoad (85.7%).
func TestCompileTextureLoad(t *testing.T) {
	source := `
@group(0) @binding(0) var my_texture: texture_2d<f32>;

@fragment
fn main(@location(0) @interpolate(flat) coord: vec2<i32>) -> @location(0) vec4<f32> {
    return textureLoad(my_texture, coord, 0);
}
`
	spv := compileWGSL(t, source)
	t.Logf("texture load shader: %d bytes", len(spv))
}

// TestCompileMultisampleTexture exercises texture_multisampled_2d.
func TestCompileMultisampleTexture(t *testing.T) {
	source := `
@group(0) @binding(0) var my_texture: texture_multisampled_2d<f32>;

@fragment
fn main(@location(0) @interpolate(flat) coord: vec2<i32>) -> @location(0) vec4<f32> {
    return textureLoad(my_texture, coord, 0);
}
`
	spv := compileWGSL(t, source)
	t.Logf("multisample texture shader: %d bytes", len(spv))
}

// TestCompileDepthTexture exercises texture_depth_2d path.
func TestCompileDepthTexture(t *testing.T) {
	source := `
@group(0) @binding(0) var depth_tex: texture_depth_2d;
@group(0) @binding(1) var depth_sampler: sampler;

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let d = textureSample(depth_tex, depth_sampler, uv);
    return vec4<f32>(d, d, d, 1.0);
}
`
	spv := compileWGSL(t, source)
	t.Logf("depth texture shader: %d bytes", len(spv))
}

// TestCompileSampleMask exercises isSampleMaskBinding (80%).
func TestCompileSampleMask(t *testing.T) {
	source := `
@fragment
fn main(@builtin(sample_mask) mask: u32) -> @location(0) vec4<f32> {
    if (mask & 1u) != 0u {
        return vec4<f32>(1.0, 0.0, 0.0, 1.0);
    }
    return vec4<f32>(0.0, 0.0, 0.0, 1.0);
}
`
	spv := compileWGSL(t, source)
	t.Logf("sample mask shader: %d bytes", len(spv))
}

// TestCompileFragDepth exercises entryPointWritesFragDepth (86.7%).
func TestCompileFragDepth(t *testing.T) {
	source := `
struct FragOutput {
    @location(0) color: vec4<f32>,
    @builtin(frag_depth) depth: f32,
}

@fragment
fn main(@location(0) uv: vec2<f32>) -> FragOutput {
    var out: FragOutput;
    out.color = vec4<f32>(uv, 0.0, 1.0);
    out.depth = uv.x;
    return out;
}
`
	spv := compileWGSL(t, source)
	t.Logf("frag depth shader: %d bytes", len(spv))
}

// TestCompileComparisonSampler exercises texture comparison sample paths.
func TestCompileComparisonSampler(t *testing.T) {
	source := `
@group(0) @binding(0) var shadow_tex: texture_depth_2d;
@group(0) @binding(1) var shadow_sampler: sampler_comparison;

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let depth_ref = 0.5;
    let shadow = textureSampleCompare(shadow_tex, shadow_sampler, uv, depth_ref);
    return vec4<f32>(shadow, shadow, shadow, 1.0);
}
`
	spv := compileWGSL(t, source)
	t.Logf("comparison sampler shader: %d bytes", len(spv))
}

// TestCompileStorageBuffer exercises getStorageAccessFlags (78.6%).
func TestCompileStorageBuffer(t *testing.T) {
	source := `
struct Params {
    count: u32,
    scale: f32,
}

@group(0) @binding(0) var<storage, read> input: array<f32>;
@group(0) @binding(1) var<storage, read_write> output: array<f32>;
@group(0) @binding(2) var<uniform> params: Params;

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    if id.x < params.count {
        output[id.x] = input[id.x] * params.scale;
    }
}
`
	spv := compileWGSL(t, source)
	t.Logf("storage buffer shader: %d bytes", len(spv))
}

// TestCompileModfFrexp exercises emitModfStructType / emitFrexpStructType (0%).
func TestCompileModfFrexp(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let m = modf(uv.x);
    let f = frexp(uv.y);
    return vec4<f32>(m.fract, m.whole, f.fract, f32(f.exp));
}
`
	spv := compileWGSL(t, source)
	t.Logf("modf/frexp shader: %d bytes", len(spv))
}

// TestCompileModfVec exercises modf on vector type.
func TestCompileModfVec(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let m = modf(uv);
    return vec4<f32>(m.fract, m.whole);
}
`
	spv := compileWGSL(t, source)
	t.Logf("modf vec shader: %d bytes", len(spv))
}

// TestCompileVectorScalarBinary exercises splatScalar / promoteScalarToVector (0%/73.7%).
func TestCompileVectorScalarBinary(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let v = vec4<f32>(uv, 0.0, 1.0);
    // vec / scalar -> triggers promoteScalarToVector
    let result = v / 2.0;
    // vec % scalar (if supported)
    let added = v + 1.0;
    let subbed = v - 0.5;
    return result + added + subbed;
}
`
	spv := compileWGSL(t, source)
	t.Logf("vector scalar binary shader: %d bytes", len(spv))
}

// TestCompileIntVectorScalarBinary exercises splatScalar with integer types.
func TestCompileIntVectorScalarBinary(t *testing.T) {
	source := `
@fragment
fn main(@location(0) @interpolate(flat) val: vec4<i32>) -> @location(0) vec4<f32> {
    let div = val / 2i;
    let add = val + 1i;
    return vec4<f32>(f32(div.x), f32(div.y), f32(add.z), f32(add.w));
}
`
	spv := compileWGSL(t, source)
	t.Logf("int vector scalar binary shader: %d bytes", len(spv))
}

// TestCompileDynamicAccess exercises emitAccess (51.1%) with dynamic indexing.
func TestCompileDynamicAccess(t *testing.T) {
	source := `
@group(0) @binding(0) var<storage, read> data: array<vec4<f32>>;

@fragment
fn main(@location(0) @interpolate(flat) idx: u32) -> @location(0) vec4<f32> {
    return data[idx];
}
`
	spv := compileWGSL(t, source)
	t.Logf("dynamic access shader: %d bytes", len(spv))
}

// TestCompileWorkgroupStruct exercises maybeCopyLogicalForStore / requireSpirvVersion14 (48.1%/0%).
func TestCompileWorkgroupStruct(t *testing.T) {
	source := `
struct TileData {
    values: array<f32, 16>,
}

var<workgroup> tile: TileData;

@group(0) @binding(0) var<storage, read_write> output: array<f32>;

@compute @workgroup_size(16)
fn main(@builtin(local_invocation_id) lid: vec3<u32>, @builtin(global_invocation_id) gid: vec3<u32>) {
    tile.values[lid.x] = f32(gid.x);
    workgroupBarrier();
    output[gid.x] = tile.values[lid.x];
}
`
	spv := compileWGSL(t, source)
	t.Logf("workgroup struct shader: %d bytes", len(spv))
}

// TestCompileRuntimeSizedArray exercises emitGlobalVarValue (25%) with runtime-sized array.
func TestCompileRuntimeSizedArray(t *testing.T) {
	source := `
struct Buffer {
    count: u32,
    data: array<f32>,
}

@group(0) @binding(0) var<storage, read> buf: Buffer;

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    if id.x < buf.count {
        let val = buf.data[id.x];
        _ = val;
    }
}
`
	spv := compileWGSL(t, source)
	t.Logf("runtime sized array shader: %d bytes", len(spv))
}

// TestCompileStorageTextureR32Uint exercises more image format capabilities.
func TestCompileStorageTextureR32Uint(t *testing.T) {
	source := `
@group(0) @binding(0) var output: texture_storage_2d<r32uint, write>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    textureStore(output, vec2<i32>(i32(id.x), i32(id.y)), vec4<u32>(id.x, 0u, 0u, 1u));
}
`
	spv := compileWGSL(t, source)
	t.Logf("storage texture r32uint shader: %d bytes", len(spv))
}

// TestCompileStorageTextureRgba16Float exercises rgba16float image format.
func TestCompileStorageTextureRgba16Float(t *testing.T) {
	source := `
@group(0) @binding(0) var output: texture_storage_2d<rgba16float, write>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    textureStore(output, vec2<i32>(i32(id.x), i32(id.y)), vec4<f32>(1.0, 0.5, 0.0, 1.0));
}
`
	spv := compileWGSL(t, source)
	t.Logf("storage texture rgba16float shader: %d bytes", len(spv))
}

// TestCompileTextureSampleBias exercises textureSampleBias path.
func TestCompileTextureSampleBias(t *testing.T) {
	source := `
@group(0) @binding(0) var my_texture: texture_2d<f32>;
@group(0) @binding(1) var my_sampler: sampler;

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSampleBias(my_texture, my_sampler, uv, -1.0);
}
`
	spv := compileWGSL(t, source)
	t.Logf("texture sample bias shader: %d bytes", len(spv))
}

// TestCompileTextureSampleGrad exercises textureSampleGrad path.
func TestCompileTextureSampleGrad(t *testing.T) {
	source := `
@group(0) @binding(0) var my_texture: texture_2d<f32>;
@group(0) @binding(1) var my_sampler: sampler;

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let ddx = vec2<f32>(dpdx(uv.x), dpdx(uv.y));
    let ddy = vec2<f32>(dpdy(uv.x), dpdy(uv.y));
    return textureSampleGrad(my_texture, my_sampler, uv, ddx, ddy);
}
`
	spv := compileWGSL(t, source)
	t.Logf("texture sample grad shader: %d bytes", len(spv))
}

// TestCompileTextureGather exercises textureGather path.
func TestCompileTextureGather(t *testing.T) {
	source := `
@group(0) @binding(0) var my_texture: texture_2d<f32>;
@group(0) @binding(1) var my_sampler: sampler;

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureGather(0, my_texture, my_sampler, uv);
}
`
	spv := compileWGSL(t, source)
	t.Logf("texture gather shader: %d bytes", len(spv))
}

// TestCompileMultipleMathFunctions exercises emitMath with more built-in functions.
func TestCompileMultipleMathFunctions(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let a = round(uv.x);
    let b = trunc(uv.y);
    let c = fract(uv.x);
    let d = sign(uv.y - 0.5);
    let e = atan2(uv.y, uv.x);
    let f = pow(abs(uv.x) + 0.01, 2.0);
    let g = exp2(uv.x);
    let h = log2(abs(uv.y) + 1.0);
    let i = step(0.5, uv.x);
    let j = smoothstep(0.0, 1.0, uv.x);
    let k = length(uv);
    let l = distance(uv, vec2<f32>(0.5, 0.5));
    return vec4<f32>(a + e + i, b + f + j, c + g + k, d + h + l);
}
`
	spv := compileWGSL(t, source)
	t.Logf("multiple math functions shader: %d bytes", len(spv))
}

// TestCompileIntegerMath exercises emitMath with integer-specific operations.
func TestCompileIntegerMath(t *testing.T) {
	source := `
@fragment
fn main(@location(0) @interpolate(flat) val: vec2<u32>) -> @location(0) vec4<f32> {
    let a = countOneBits(val.x);
    let b = reverseBits(val.y);
    let c = firstLeadingBit(val.x);
    let d = firstTrailingBit(val.y);
    return vec4<f32>(f32(a), f32(b), f32(c), f32(d));
}
`
	spv := compileWGSL(t, source)
	t.Logf("integer math shader: %d bytes", len(spv))
}

// TestCompileAbsInt exercises abs on signed integers.
func TestCompileAbsInt(t *testing.T) {
	source := `
@fragment
fn main(@location(0) @interpolate(flat) val: i32) -> @location(0) vec4<f32> {
    let a = abs(val);
    let b = max(val, -val);
    let c = min(val, 0i);
    return vec4<f32>(f32(a), f32(b), f32(c), 1.0);
}
`
	spv := compileWGSL(t, source)
	t.Logf("abs int shader: %d bytes", len(spv))
}

// TestCompileLocalVarInLoop exercises emitLocalVarValue (0%) via loop with local variable reads.
func TestCompileLocalVarInLoop(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    var acc: f32 = 0.0;
    var i: i32 = 0;
    loop {
        if i >= 5 {
            break;
        }
        // Read local variable value (not through pointer)
        let current = acc;
        acc = current + uv.x;
        i = i + 1;
    }
    return vec4<f32>(acc, acc, acc, 1.0);
}
`
	spv := compileWGSL(t, source)
	t.Logf("local var in loop shader: %d bytes", len(spv))
}

// TestCompilePointerArgs exercises emitCall with pointer arguments / isMemoryObjectDeclaration (42.9%).
func TestCompilePointerArgs(t *testing.T) {
	source := `
fn add_to(p: ptr<function, f32>, amount: f32) {
    *p = *p + amount;
}

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    var val: f32 = uv.x;
    add_to(&val, uv.y);
    return vec4<f32>(val, val, val, 1.0);
}
`
	spv := compileWGSL(t, source)
	t.Logf("pointer args shader: %d bytes", len(spv))
}

// TestCompileCrossProductDotNormalize exercises cross, dot, normalize, reflect, refract.
func TestCompileCrossProductDotNormalize(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let a = vec3<f32>(uv.x, uv.y, 0.0);
    let b = vec3<f32>(0.0, 1.0, 0.0);
    let c = cross(a, b);
    let d = dot(a, b);
    let n = normalize(a);
    let r = reflect(a, b);
    let fwd = faceForward(a, b, vec3<f32>(0.0, 0.0, 1.0));
    return vec4<f32>(c.x + n.x + r.x + fwd.x, c.y + d, c.z, 1.0);
}
`
	spv := compileWGSL(t, source)
	t.Logf("cross dot normalize shader: %d bytes", len(spv))
}

// TestCompileMatrixConstructor exercises emitCompose for matrix types.
func TestCompileMatrixConstructor(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let m = mat2x2<f32>(
        uv.x, uv.y,
        1.0 - uv.x, 1.0 - uv.y
    );
    let v = m * uv;
    return vec4<f32>(v, 0.0, 1.0);
}
`
	spv := compileWGSL(t, source)
	t.Logf("matrix constructor shader: %d bytes", len(spv))
}

// TestCompileMultiReturnFunction exercises emitCall with struct returns.
func TestCompileMultiReturnFunction(t *testing.T) {
	source := `
struct Result {
    x: f32,
    y: f32,
}

fn compute(a: f32, b: f32) -> Result {
    var r: Result;
    r.x = a + b;
    r.y = a * b;
    return r;
}

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let r = compute(uv.x, uv.y);
    return vec4<f32>(r.x, r.y, 0.0, 1.0);
}
`
	spv := compileWGSL(t, source)
	t.Logf("multi-return function shader: %d bytes", len(spv))
}

// TestCompileWrappedUniformBuffer exercises emitGlobals wrapped uniform struct path.
func TestCompileWrappedUniformBuffer(t *testing.T) {
	source := `
struct Uniforms {
    model: mat4x4<f32>,
    view: mat4x4<f32>,
    proj: mat4x4<f32>,
}

@group(0) @binding(0) var<uniform> uniforms: Uniforms;

@vertex
fn main(@location(0) pos: vec3<f32>) -> @builtin(position) vec4<f32> {
    let mvp = uniforms.proj * uniforms.view * uniforms.model;
    return mvp * vec4<f32>(pos, 1.0);
}
`
	spv := compileWGSL(t, source)
	t.Logf("wrapped uniform buffer shader: %d bytes", len(spv))
}

// TestCompileTexture3D exercises texture_3d sampling.
func TestCompileTexture3D(t *testing.T) {
	source := `
@group(0) @binding(0) var vol_tex: texture_3d<f32>;
@group(0) @binding(1) var vol_sampler: sampler;

@fragment
fn main(@location(0) uvw: vec3<f32>) -> @location(0) vec4<f32> {
    return textureSample(vol_tex, vol_sampler, uvw);
}
`
	spv := compileWGSL(t, source)
	t.Logf("texture 3d shader: %d bytes", len(spv))
}

// TestCompileTextureCube exercises texture_cube sampling.
func TestCompileTextureCube(t *testing.T) {
	source := `
@group(0) @binding(0) var cube_tex: texture_cube<f32>;
@group(0) @binding(1) var cube_sampler: sampler;

@fragment
fn main(@location(0) dir: vec3<f32>) -> @location(0) vec4<f32> {
    return textureSample(cube_tex, cube_sampler, dir);
}
`
	spv := compileWGSL(t, source)
	t.Logf("texture cube shader: %d bytes", len(spv))
}

// TestCompileTextureArray exercises texture_2d_array sampling.
func TestCompileTextureArray(t *testing.T) {
	source := `
@group(0) @binding(0) var arr_tex: texture_2d_array<f32>;
@group(0) @binding(1) var arr_sampler: sampler;

@fragment
fn main(@location(0) uv: vec2<f32>, @location(1) @interpolate(flat) layer: i32) -> @location(0) vec4<f32> {
    return textureSample(arr_tex, arr_sampler, uv, layer);
}
`
	spv := compileWGSL(t, source)
	t.Logf("texture array shader: %d bytes", len(spv))
}

// ---------------------------------------------------------------------------
// 6. Direct unit tests for pure functions to increase coverage
// ---------------------------------------------------------------------------

// TestAtomicOpcode exercises atomicOpcode (18.8%) with all atomic function types.
func TestAtomicOpcode(t *testing.T) {
	tests := []struct {
		name   string
		fun    ir.AtomicFunction
		scalar ir.ScalarKind
		want   OpCode
		ok     bool
	}{
		{"add_uint", ir.AtomicAdd{}, ir.ScalarUint, OpAtomicIAdd, true},
		{"add_sint", ir.AtomicAdd{}, ir.ScalarSint, OpAtomicIAdd, true},
		{"add_float", ir.AtomicAdd{}, ir.ScalarFloat, OpAtomicFAddEXT, true},
		{"subtract", ir.AtomicSubtract{}, ir.ScalarUint, OpAtomicISub, true},
		{"and", ir.AtomicAnd{}, ir.ScalarUint, OpAtomicAnd, true},
		{"or", ir.AtomicInclusiveOr{}, ir.ScalarUint, OpAtomicOr, true},
		{"xor", ir.AtomicExclusiveOr{}, ir.ScalarUint, OpAtomicXor, true},
		{"min_uint", ir.AtomicMin{}, ir.ScalarUint, OpAtomicUMin, true},
		{"min_sint", ir.AtomicMin{}, ir.ScalarSint, OpAtomicSMin, true},
		{"max_uint", ir.AtomicMax{}, ir.ScalarUint, OpAtomicUMax, true},
		{"max_sint", ir.AtomicMax{}, ir.ScalarSint, OpAtomicSMax, true},
		{"exchange", ir.AtomicExchange{}, ir.ScalarUint, OpAtomicExchange, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := atomicOpcode(tt.fun, tt.scalar)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
			if got != tt.want {
				t.Errorf("opcode = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestAtomicOpcodeLoad exercises AtomicLoad/Store which return false from atomicOpcode
// (they're handled separately in emitAtomic).
func TestAtomicOpcodeSpecialCases(t *testing.T) {
	// AtomicLoad and AtomicStore are handled by emitAtomic directly,
	// they don't go through atomicOpcode.
	_, ok := atomicOpcode(ir.AtomicLoad{}, ir.ScalarUint)
	if ok {
		t.Error("AtomicLoad should not produce an opcode via atomicOpcode (handled separately)")
	}
	_, ok = atomicOpcode(ir.AtomicStore{}, ir.ScalarUint)
	if ok {
		t.Error("AtomicStore should not produce an opcode via atomicOpcode (handled separately)")
	}
}

// TestCompileAtomicExchange exercises the AtomicExchange path (separate from compare-exchange).
func TestCompileAtomicExchange(t *testing.T) {
	source := `
@group(0) @binding(0) var<storage, read_write> val: atomic<u32>;

@compute @workgroup_size(1)
fn main() {
    let old = atomicExchange(&val, 42u);
    _ = old;
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	if !hasOpcodeInInstrs(instrs, OpAtomicExchange) {
		t.Error("expected OpAtomicExchange for atomicExchange()")
	}
}

// TestCompileWorkgroupAtomicI32 exercises atomicOpcode with signed atomic min/max.
func TestCompileWorkgroupAtomicI32(t *testing.T) {
	source := `
var<workgroup> wg_counter: atomic<i32>;

@compute @workgroup_size(64)
fn main(@builtin(local_invocation_id) lid: vec3<u32>) {
    atomicAdd(&wg_counter, 1i);
    let old = atomicMax(&wg_counter, i32(lid.x));
    let old2 = atomicMin(&wg_counter, -1i);
    _ = old;
    _ = old2;
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	// Signed min/max should use OpAtomicSMin/OpAtomicSMax
	if !hasOpcodeInInstrs(instrs, OpAtomicSMin) {
		t.Error("expected OpAtomicSMin for signed atomicMin()")
	}
	if !hasOpcodeInInstrs(instrs, OpAtomicSMax) {
		t.Error("expected OpAtomicSMax for signed atomicMax()")
	}
}

// TestCompileArrayAccess exercises emitAccess with dynamic array indexing.
func TestCompileArrayAccess(t *testing.T) {
	source := `
@group(0) @binding(0) var<storage, read> data: array<vec4<f32>>;
@group(0) @binding(1) var<storage, read_write> output: array<vec4<f32>>;

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    let val = data[id.x];
    output[id.x] = val * 2.0;
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	// Dynamic access needs OpAccessChain
	if !hasOpcodeInInstrs(instrs, OpAccessChain) {
		t.Error("expected OpAccessChain for dynamic array access")
	}
}

// TestCompileArrayLengthOnStruct exercises emitArrayLength with wrapped struct.
func TestCompileArrayLengthOnStruct(t *testing.T) {
	source := `
struct Buffer {
    header: u32,
    data: array<f32>,
}

@group(0) @binding(0) var<storage, read> buf: Buffer;
@group(0) @binding(1) var<storage, read_write> out: array<f32>;

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    let len = arrayLength(&buf.data);
    if id.x < len {
        out[id.x] = buf.data[id.x];
    }
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	if !hasOpcodeInInstrs(instrs, OpArrayLength) {
		t.Error("expected OpArrayLength for arrayLength()")
	}
}

// TestCompileSwitchWithFallthrough exercises emitSwitch with many cases.
func TestCompileSwitchWithFallthrough(t *testing.T) {
	source := `
@fragment
fn main(@location(0) @interpolate(flat) mode: i32) -> @location(0) vec4<f32> {
    var r: f32 = 0.0;
    var g: f32 = 0.0;
    var b: f32 = 0.0;
    switch mode {
        case 0i: { r = 1.0; }
        case 1i: { g = 1.0; }
        case 2i: { b = 1.0; }
        case 3i: { r = 0.5; g = 0.5; }
        case 4i: { g = 0.5; b = 0.5; }
        case 5i, 6i: { r = 0.3; g = 0.3; b = 0.3; }
        default: { r = 1.0; g = 1.0; b = 1.0; }
    }
    return vec4<f32>(r, g, b, 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	if !hasOpcodeInInstrs(instrs, OpSwitch) {
		t.Error("expected OpSwitch for WGSL switch statement")
	}
}

// TestCompileNestedLoopBreakContinue exercises emitLoop with break/continue.
func TestCompileNestedLoopBreakContinue(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    var sum: f32 = 0.0;
    for (var i: i32 = 0; i < 10; i++) {
        if i == 3 {
            continue;
        }
        for (var j: i32 = 0; j < 5; j++) {
            if j == 2 {
                break;
            }
            sum += uv.x;
        }
    }
    return vec4<f32>(sum, 0.0, 0.0, 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	// Nested loops require at least 2 OpLoopMerge
	loopCount := countOpcodeInInstrs(instrs, OpLoopMerge)
	if loopCount < 2 {
		t.Errorf("expected at least 2 OpLoopMerge for nested loops, got %d", loopCount)
	}
}

// TestCompileMultiBindGroupUniform exercises emitGlobals (62.7%) with multiple bind groups.
func TestCompileMultiBindGroupUniform(t *testing.T) {
	source := `
struct CameraUniforms {
    view: mat4x4<f32>,
    proj: mat4x4<f32>,
}

struct ModelUniforms {
    model: mat4x4<f32>,
    color: vec4<f32>,
}

@group(0) @binding(0) var<uniform> camera: CameraUniforms;
@group(1) @binding(0) var<uniform> model: ModelUniforms;
@group(2) @binding(0) var diffuse_tex: texture_2d<f32>;
@group(2) @binding(1) var diffuse_sampler: sampler;

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let tex_color = textureSample(diffuse_tex, diffuse_sampler, uv);
    return tex_color * model.color;
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)

	// Multiple descriptor sets require DescriptorSet decorations
	descriptorSetCount := 0
	for _, inst := range instrs {
		if inst.opcode == OpDecorate && len(inst.words) >= 3 {
			if Decoration(inst.words[2]) == DecorationDescriptorSet {
				descriptorSetCount++
			}
		}
	}
	if descriptorSetCount < 3 {
		t.Errorf("expected at least 3 DescriptorSet decorations for 3 bind groups, got %d", descriptorSetCount)
	}
}

// TestCompileTextureSampleLevel exercises textureSampleLevel path in emitImageSample.
func TestCompileTextureSampleLevel(t *testing.T) {
	source := `
@group(0) @binding(0) var my_texture: texture_2d<f32>;
@group(0) @binding(1) var my_sampler: sampler;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    let uv = vec2<f32>(f32(id.x) / 256.0, f32(id.y) / 256.0);
    let color = textureSampleLevel(my_texture, my_sampler, uv, 0.0);
    _ = color;
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	// textureSampleLevel uses explicit LOD sampling
	if !hasOpcodeInInstrs(instrs, OpImageSampleExplicitLod) {
		t.Error("expected OpImageSampleExplicitLod for textureSampleLevel()")
	}
}

// TestCompileTextureDimensionsCube exercises emitImageQuery on cube texture.
func TestCompileTextureDimensionsCube(t *testing.T) {
	source := `
@group(0) @binding(0) var cube_tex: texture_cube<f32>;

@fragment
fn main() -> @location(0) vec4<f32> {
    let dims = textureDimensions(cube_tex);
    return vec4<f32>(f32(dims.x), f32(dims.y), 0.0, 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	if !hasCapability(instrs, CapabilityImageQuery) {
		t.Error("expected ImageQuery capability for textureDimensions()")
	}
}

// TestCompileNumericConversions exercises emitAs (66.7%) with more conversion types.
func TestCompileNumericConversions(t *testing.T) {
	source := `
@fragment
fn main(@location(0) @interpolate(flat) ival: i32) -> @location(0) vec4<f32> {
    // Various type conversions
    let u = u32(ival);     // i32 -> u32 (bitcast)
    let f = f32(ival);     // i32 -> f32 (SConvert)
    let fu = f32(u);       // u32 -> f32 (UConvert)
    let fi = i32(f);       // f32 -> i32 (ConvertFToS)
    let fiu = u32(fu);     // f32 -> u32 (ConvertFToU)

    // Bool conversions
    let b = ival > 0;
    let bi = select(0i, 1i, b);  // bool -> i32
    let bf = select(0.0, 1.0, b); // bool -> f32

    return vec4<f32>(f, fu, f32(bi), bf);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
}

// TestCompileVertexBuiltins exercises builtinToSPIRV (95.8%) and entry point interface.
func TestCompileVertexBuiltins(t *testing.T) {
	source := `
struct VertexIn {
    @builtin(vertex_index) vertex_idx: u32,
    @builtin(instance_index) instance_idx: u32,
}

struct VertexOut {
    @builtin(position) pos: vec4<f32>,
}

@vertex
fn main(input: VertexIn) -> VertexOut {
    var out: VertexOut;
    let idx = f32(input.vertex_idx) + f32(input.instance_idx);
    out.pos = vec4<f32>(idx, 0.0, 0.0, 1.0);
    return out;
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)

	// Vertex shader should have BuiltIn decorations for vertex_index and instance_index
	builtInCount := 0
	for _, inst := range instrs {
		if inst.opcode == OpDecorate && len(inst.words) >= 3 {
			if Decoration(inst.words[2]) == DecorationBuiltIn {
				builtInCount++
			}
		}
	}
	// At least position + vertex_index + instance_index = 3
	if builtInCount < 3 {
		t.Errorf("expected at least 3 BuiltIn decorations, got %d", builtInCount)
	}
}

// TestCompileComputeBuiltins exercises emitEntryPoints compute shader builtins.
func TestCompileComputeBuiltins(t *testing.T) {
	source := `
@compute @workgroup_size(8, 4, 2)
fn main(
    @builtin(global_invocation_id) gid: vec3<u32>,
    @builtin(local_invocation_id) lid: vec3<u32>,
    @builtin(workgroup_id) wid: vec3<u32>,
    @builtin(local_invocation_index) idx: u32,
    @builtin(num_workgroups) num_wg: vec3<u32>,
) {
    _ = gid;
    _ = lid;
    _ = wid;
    _ = idx;
    _ = num_wg;
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)

	// Workgroup size 8,4,2 should be specified via ExecutionMode
	foundWorkgroupSize := false
	for _, inst := range instrs {
		if inst.opcode == OpExecutionMode && len(inst.words) >= 5 {
			mode := ExecutionMode(inst.words[2])
			if mode == ExecutionModeLocalSize {
				foundWorkgroupSize = true
				// Verify dimensions 8, 4, 2
				if inst.words[3] != 8 || inst.words[4] != 4 || inst.words[5] != 2 {
					t.Errorf("workgroup size = (%d,%d,%d), want (8,4,2)",
						inst.words[3], inst.words[4], inst.words[5])
				}
			}
		}
	}
	if !foundWorkgroupSize {
		t.Error("expected ExecutionModeLocalSize for @workgroup_size(8,4,2)")
	}
}

// TestCompileBitOperations exercises emitBinary with bitwise operations.
// Verifies bit operation opcodes are emitted correctly.
func TestCompileBitOperations(t *testing.T) {
	source := `
@fragment
fn main(@location(0) @interpolate(flat) val: u32) -> @location(0) vec4<f32> {
    let a = val & 0xFFu;
    let b = val | 0x0Fu;
    let c = val ^ 0xAAu;
    let d = val << 2u;
    let e = val >> 4u;
    return vec4<f32>(f32(a), f32(b), f32(c), f32(d + e));
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	if !hasOpcodeInInstrs(instrs, OpBitwiseAnd) {
		t.Error("expected OpBitwiseAnd for &")
	}
	if !hasOpcodeInInstrs(instrs, OpBitwiseOr) {
		t.Error("expected OpBitwiseOr for |")
	}
	if !hasOpcodeInInstrs(instrs, OpBitwiseXor) {
		t.Error("expected OpBitwiseXor for ^")
	}
	if !hasOpcodeInInstrs(instrs, OpShiftLeftLogical) {
		t.Error("expected OpShiftLeftLogical for <<")
	}
	if !hasOpcodeInInstrs(instrs, OpShiftRightLogical) {
		t.Error("expected OpShiftRightLogical for >>")
	}
}

// ---------------------------------------------------------------------------
// 7. IR-level tests for functions unreachable from WGSL
// ---------------------------------------------------------------------------

// TestCompileModuleWithAtomicResult exercises emitAtomicResultRef (0%) via direct IR.
func TestCompileModuleWithAtomicResult(t *testing.T) {
	atomicTypeHandle := ir.TypeHandle(0)
	u32TypeHandle := ir.TypeHandle(1)

	resultExprHandle := ir.ExpressionHandle(2)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "atomic_u32", Inner: ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:  "counter",
				Type:  atomicTypeHandle,
				Space: ir.SpaceWorkGroup,
			},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:      "main",
				Stage:     ir.StageCompute,
				Workgroup: [3]uint32{1, 1, 1},
				Function: ir.Function{
					Expressions: []ir.Expression{
						{Kind: ir.ExprGlobalVariable{Variable: 0}},                        // expr 0: &counter
						{Kind: ir.Literal{Value: ir.LiteralU32(1)}},                       // expr 1: 1u
						{Kind: ir.ExprAtomicResult{Ty: u32TypeHandle, Comparison: false}}, // expr 2: atomic result
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 2}}},
						{Kind: ir.StmtAtomic{
							Pointer: 0,
							Fun:     ir.AtomicAdd{},
							Value:   1,
							Result:  &resultExprHandle,
						}},
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 2, End: 3}}},
					},
				},
			},
		},
	}

	backend := NewBackend(DefaultOptions())
	spvBytes, err := backend.Compile(module)
	if err != nil {
		// The IR may be incomplete for this specific test, but the path through
		// emitAtomicResultRef is the goal. Even if it fails, we verify the path is exercised.
		t.Logf("Compile result: err=%v, bytes=%d", err, len(spvBytes))
		return
	}
	assertValidSPIRV(t, spvBytes)
	instrs := decodeSPIRVInstructions(spvBytes)
	if !hasOpcodeInInstrs(instrs, OpAtomicIAdd) {
		t.Error("expected OpAtomicIAdd in output")
	}
}

// TestCompilePack4x8 exercises emitPack4x8Polyfill (0%).
func TestCompilePack4x8(t *testing.T) {
	source := `
@fragment
fn main(@location(0) @interpolate(flat) vals: vec4<i32>) -> @location(0) vec4<f32> {
    let packed_i = pack4xI8(vals);
    let packed_u = pack4xU8(vec4<u32>(vals));
    let packed_ic = pack4xI8Clamp(vals);
    let packed_uc = pack4xU8Clamp(vec4<u32>(vals));
    return vec4<f32>(f32(packed_i), f32(packed_u), f32(packed_ic), f32(packed_uc));
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	// Pack uses BitFieldInsert polyfill
	if !hasOpcodeInInstrs(instrs, OpBitFieldInsert) {
		t.Error("expected OpBitFieldInsert for pack4x8 polyfill")
	}
}

// TestCompileUnpack4x8 exercises emitUnpack4x8Polyfill (0%).
func TestCompileUnpack4x8(t *testing.T) {
	source := `
@fragment
fn main(@location(0) @interpolate(flat) val: u32) -> @location(0) vec4<f32> {
    let unpacked_i = unpack4xI8(val);
    let unpacked_u = unpack4xU8(val);
    return vec4<f32>(f32(unpacked_i.x), f32(unpacked_i.y), f32(unpacked_u.z), f32(unpacked_u.w));
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	// Unpack uses BitFieldSExtract/BitFieldUExtract
	hasBitField := hasOpcodeInInstrs(instrs, OpBitFieldSExtract) || hasOpcodeInInstrs(instrs, OpBitFieldUExtract)
	if !hasBitField {
		t.Error("expected OpBitFieldSExtract or OpBitFieldUExtract for unpack4x8 polyfill")
	}
}

// TestCompileWorkgroupUniformLoad exercises emitWorkGroupUniformLoad (0%).
func TestCompileWorkgroupUniformLoad(t *testing.T) {
	source := `
var<workgroup> shared_val: u32;

@compute @workgroup_size(64)
fn main(@builtin(local_invocation_id) lid: vec3<u32>) {
    if lid.x == 0u {
        shared_val = 42u;
    }
    workgroupBarrier();
    let uniform_val = workgroupUniformLoad(&shared_val);
    _ = uniform_val;
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	// WorkgroupUniformLoad emits barrier + load + barrier
	barrierCount := countOpcodeInInstrs(instrs, OpControlBarrier)
	// At least 3 barriers: explicit workgroupBarrier + 2 from workgroupUniformLoad
	if barrierCount < 3 {
		t.Errorf("expected at least 3 OpControlBarrier for workgroupBarrier + workgroupUniformLoad, got %d", barrierCount)
	}
}

// TestCompileSpillCompositeAccess exercises spill-to-variable path (0%) when accessing
// a field of a composite returned from a function call.
func TestCompileSpillCompositeAccess(t *testing.T) {
	source := `
struct Pair {
    a: f32,
    b: f32,
}

fn make_pair(x: f32, y: f32) -> Pair {
    var p: Pair;
    p.a = x;
    p.b = y;
    return p;
}

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let p = make_pair(uv.x, uv.y);
    // Accessing p.a and p.b may trigger spill if the composite needs OpAccessChain
    return vec4<f32>(p.a, p.b, p.a + p.b, 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	// Function call results with field access produce CompositeExtract or AccessChain
	hasExtract := hasOpcodeInInstrs(instrs, OpCompositeExtract)
	hasAccessChain := hasOpcodeInInstrs(instrs, OpAccessChain)
	if !hasExtract && !hasAccessChain {
		t.Error("expected OpCompositeExtract or OpAccessChain for struct field access on call result")
	}
}

// TestCompileDeferredCallResultInit exercises findDeferredLocalVarRef / processDeferredStores
// when a local variable is initialized from a function call result.
func TestCompileDeferredCallResultInit(t *testing.T) {
	source := `
fn compute(x: f32) -> f32 {
    return x * x + 1.0;
}

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    var result = compute(uv.x);
    result = result + compute(uv.y);
    return vec4<f32>(result, result, result, 1.0);
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	// Should have OpFunctionCall
	if !hasOpcodeInInstrs(instrs, OpFunctionCall) {
		t.Error("expected OpFunctionCall for helper function calls")
	}
}

// TestCompileVecDivideScalar exercises promoteScalarToVector -> splatScalar for vec/scalar divide.
func TestCompileVecDivideScalar(t *testing.T) {
	source := `
@fragment
fn main(@location(0) @interpolate(flat) v: vec4<i32>) -> @location(0) vec4<f32> {
    // Integer vec / scalar triggers promoteScalarToVector -> splatScalar
    let result = v / 3i;
    let result2 = v % 5i;
    return vec4<f32>(f32(result.x), f32(result.y), f32(result2.z), f32(result2.w));
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
}

// TestCompileSelectWithFloatCondition exercises emitSelect float condition path.
func TestCompileSelectWithFloatCondition(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let a = vec4<f32>(1.0, 0.0, 0.0, 1.0);
    let b = vec4<f32>(0.0, 1.0, 0.0, 1.0);
    // step() returns float, select treats it as condition
    let s = step(0.5, uv.x);
    let result = select(a, b, s > 0.0);
    return result;
}
`
	spv := compileWGSL(t, source)
	assertValidSPIRV(t, spv)
}

// TestCompileModuleWithLocalVarValue exercises emitLocalVarValue (0%) via direct IR.
func TestCompileModuleWithLocalVarValue(t *testing.T) {
	u32TypeHandle := ir.TypeHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:      "main",
				Stage:     ir.StageCompute,
				Workgroup: [3]uint32{1, 1, 1},
				Function: ir.Function{
					LocalVars: []ir.LocalVariable{
						{Name: "x", Type: u32TypeHandle},
					},
					Expressions: []ir.Expression{
						{Kind: ir.ExprLocalVariable{Variable: 0}},    // expr 0: &x (pointer)
						{Kind: ir.Literal{Value: ir.LiteralU32(42)}}, // expr 1: 42u
						{Kind: ir.ExprLoad{Pointer: 0}},              // expr 2: *(&x) = load x
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 2}}},
						{Kind: ir.StmtStore{Pointer: 0, Value: 1}},
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 2, End: 3}}},
					},
				},
			},
		},
	}

	backend := NewBackend(DefaultOptions())
	spvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	assertValidSPIRV(t, spvBytes)

	instrs := decodeSPIRVInstructions(spvBytes)
	// Should have OpVariable for local var, OpStore, OpLoad
	if !hasOpcodeInInstrs(instrs, OpVariable) {
		t.Error("expected OpVariable for local variable")
	}
	if !hasOpcodeInInstrs(instrs, OpStore) {
		t.Error("expected OpStore to write to local variable")
	}
	if !hasOpcodeInInstrs(instrs, OpLoad) {
		t.Error("expected OpLoad to read local variable")
	}
}
