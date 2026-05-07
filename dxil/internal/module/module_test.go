package module

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestNewModule(t *testing.T) {
	mod := NewModule(VertexShader)
	if mod.ShaderKind != VertexShader {
		t.Errorf("ShaderKind: got %d, want %d", mod.ShaderKind, VertexShader)
	}
	if mod.TargetTriple != "dxil-ms-dx" {
		t.Errorf("TargetTriple: got %q, want %q", mod.TargetTriple, "dxil-ms-dx")
	}
	if mod.MajorVersion != 1 || mod.MinorVersion != 0 {
		t.Errorf("version: got %d.%d, want 1.0", mod.MajorVersion, mod.MinorVersion)
	}
}

func TestShaderKindConstants(t *testing.T) {
	// Verify shader kind values match DXIL spec.
	tests := []struct {
		name string
		kind ShaderKind
		want ShaderKind
	}{
		{"Pixel", PixelShader, 0},
		{"Vertex", VertexShader, 1},
		{"Geometry", GeometryShader, 2},
		{"Hull", HullShader, 3},
		{"Domain", DomainShader, 4},
		{"Compute", ComputeShader, 5},
	}
	for _, tt := range tests {
		if tt.kind != tt.want {
			t.Errorf("%s: got %d, want %d", tt.name, tt.kind, tt.want)
		}
	}
}

func TestGetVoidType_Deduplication(t *testing.T) {
	mod := NewModule(VertexShader)
	v1 := mod.GetVoidType()
	v2 := mod.GetVoidType()
	if v1 != v2 {
		t.Error("GetVoidType returned different pointers")
	}
	if v1.Kind != TypeVoid {
		t.Errorf("void type kind: got %d, want %d", v1.Kind, TypeVoid)
	}
}

func TestGetIntType_Deduplication(t *testing.T) {
	mod := NewModule(VertexShader)
	for _, bits := range []uint{1, 8, 16, 32, 64} {
		t1 := mod.GetIntType(bits)
		t2 := mod.GetIntType(bits)
		if t1 != t2 {
			t.Errorf("GetIntType(%d) not deduplicated", bits)
		}
		if t1.IntBits != bits {
			t.Errorf("GetIntType(%d).IntBits = %d", bits, t1.IntBits)
		}
	}
}

func TestGetFloatType_Deduplication(t *testing.T) {
	mod := NewModule(VertexShader)
	for _, bits := range []uint{16, 32, 64} {
		t1 := mod.GetFloatType(bits)
		t2 := mod.GetFloatType(bits)
		if t1 != t2 {
			t.Errorf("GetFloatType(%d) not deduplicated", bits)
		}
		if t1.FloatBits != bits {
			t.Errorf("GetFloatType(%d).FloatBits = %d", bits, t1.FloatBits)
		}
	}
}

func TestGetPointerType_Deduplication(t *testing.T) {
	mod := NewModule(VertexShader)
	i32 := mod.GetIntType(32)
	p1 := mod.GetPointerType(i32)
	p2 := mod.GetPointerType(i32)
	if p1 != p2 {
		t.Error("GetPointerType not deduplicated for same element type")
	}
	if p1.PointerElem != i32 {
		t.Error("pointer element type mismatch")
	}
}

func TestGetFunctionType(t *testing.T) {
	mod := NewModule(VertexShader)
	voidTy := mod.GetVoidType()
	i32Ty := mod.GetIntType(32)
	fty := mod.GetFunctionType(voidTy, []*Type{i32Ty, i32Ty})
	if fty.Kind != TypeFunction {
		t.Errorf("function type kind: got %d, want %d", fty.Kind, TypeFunction)
	}
	if fty.RetType != voidTy {
		t.Error("function return type mismatch")
	}
	if len(fty.ParamTypes) != 2 {
		t.Errorf("function param count: got %d, want 2", len(fty.ParamTypes))
	}
}

func TestGetStructType(t *testing.T) {
	mod := NewModule(VertexShader)
	i32 := mod.GetIntType(32)
	st := mod.GetStructType("test_struct", []*Type{i32, i32})
	if st.Kind != TypeStruct {
		t.Errorf("struct kind: got %d, want %d", st.Kind, TypeStruct)
	}
	if st.StructName != "test_struct" {
		t.Errorf("struct name: got %q, want %q", st.StructName, "test_struct")
	}
	if len(st.StructElems) != 2 {
		t.Errorf("struct elem count: got %d, want 2", len(st.StructElems))
	}
}

func TestGetArrayType(t *testing.T) {
	mod := NewModule(VertexShader)
	f32 := mod.GetFloatType(32)
	arr := mod.GetArrayType(f32, 4)
	if arr.Kind != TypeArray {
		t.Errorf("array kind: got %d, want %d", arr.Kind, TypeArray)
	}
	if arr.ElemCount != 4 {
		t.Errorf("array count: got %d, want 4", arr.ElemCount)
	}
}

func TestAddFunction(t *testing.T) {
	mod := NewModule(VertexShader)
	voidTy := mod.GetVoidType()
	funcTy := mod.GetFunctionType(voidTy, nil)
	fn := mod.AddFunction("main", funcTy, false)
	if fn.Name != "main" {
		t.Errorf("function name: got %q, want %q", fn.Name, "main")
	}
	if fn.IsDeclaration {
		t.Error("expected non-declaration function")
	}
	if len(mod.Functions) != 1 {
		t.Errorf("function count: got %d, want 1", len(mod.Functions))
	}
}

func TestAddBasicBlock(t *testing.T) {
	mod := NewModule(VertexShader)
	voidTy := mod.GetVoidType()
	funcTy := mod.GetFunctionType(voidTy, nil)
	fn := mod.AddFunction("main", funcTy, false)
	bb := fn.AddBasicBlock("entry")
	if bb.Name != "entry" {
		t.Errorf("bb name: got %q, want %q", bb.Name, "entry")
	}
	if len(fn.BasicBlocks) != 1 {
		t.Errorf("bb count: got %d, want 1", len(fn.BasicBlocks))
	}
}

func TestNewRetVoidInstr(t *testing.T) {
	instr := NewRetVoidInstr()
	if instr.Kind != InstrRet {
		t.Errorf("kind: got %d, want %d", instr.Kind, InstrRet)
	}
	if instr.HasValue {
		t.Error("void return should not have value")
	}
	if instr.ReturnValue != -1 {
		t.Errorf("return value: got %d, want -1", instr.ReturnValue)
	}
}

func TestAddIntConst(t *testing.T) {
	mod := NewModule(VertexShader)
	i32 := mod.GetIntType(32)
	c := mod.AddIntConst(i32, 42)
	if c.IntValue != 42 {
		t.Errorf("const value: got %d, want 42", c.IntValue)
	}
	if c.ConstType != i32 {
		t.Error("const type mismatch")
	}
}

func TestMetadata(t *testing.T) {
	mod := NewModule(VertexShader)
	i32 := mod.GetIntType(32)
	c := mod.AddIntConst(i32, 1)

	mdStr := mod.AddMetadataString("hello")
	if mdStr.Kind != MDString {
		t.Errorf("md string kind: got %d, want %d", mdStr.Kind, MDString)
	}
	if mdStr.StringValue != "hello" {
		t.Errorf("md string value: got %q, want %q", mdStr.StringValue, "hello")
	}

	mdVal := mod.AddMetadataValue(i32, c)
	if mdVal.Kind != MDValue {
		t.Errorf("md value kind: got %d, want %d", mdVal.Kind, MDValue)
	}

	mdTuple := mod.AddMetadataTuple([]*MetadataNode{mdStr, mdVal, nil})
	if mdTuple.Kind != MDTuple {
		t.Errorf("md tuple kind: got %d, want %d", mdTuple.Kind, MDTuple)
	}
	if len(mdTuple.SubNodes) != 3 {
		t.Errorf("md tuple sub-nodes: got %d, want 3", len(mdTuple.SubNodes))
	}
	if mdTuple.SubNodes[2] != nil {
		t.Error("expected nil sub-node at index 2")
	}
}

func TestNamedMetadata(t *testing.T) {
	mod := NewModule(VertexShader)
	mdStr := mod.AddMetadataString("test")
	mod.AddNamedMetadata("dx.version", []*MetadataNode{mdStr})

	if len(mod.NamedMetadata) != 1 {
		t.Fatalf("named metadata count: got %d, want 1", len(mod.NamedMetadata))
	}
	if mod.NamedMetadata[0].Name != "dx.version" {
		t.Errorf("named md name: got %q, want %q", mod.NamedMetadata[0].Name, "dx.version")
	}
}

func TestTypeIDAssignment(t *testing.T) {
	mod := NewModule(VertexShader)
	v := mod.GetVoidType()
	i32 := mod.GetIntType(32)
	f32 := mod.GetFloatType(32)

	if v.ID != 0 {
		t.Errorf("void ID: got %d, want 0", v.ID)
	}
	if i32.ID != 1 {
		t.Errorf("i32 ID: got %d, want 1", i32.ID)
	}
	if f32.ID != 2 {
		t.Errorf("f32 ID: got %d, want 2", f32.ID)
	}
}

func TestSerialize_MinimalModule(t *testing.T) {
	mod := NewModule(VertexShader)
	voidTy := mod.GetVoidType()
	funcTy := mod.GetFunctionType(voidTy, nil)
	fn := mod.AddFunction("main", funcTy, false)
	bb := fn.AddBasicBlock("entry")
	bb.AddInstruction(NewRetVoidInstr())

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization produced empty output")
	}

	// Verify magic.
	if data[0] != 'B' || data[1] != 'C' || data[2] != 0xC0 || data[3] != 0xDE {
		t.Errorf("bad magic: %02X %02X %02X %02X", data[0], data[1], data[2], data[3])
	}

	// Verify 32-bit alignment.
	if len(data)%4 != 0 {
		t.Errorf("output not 32-bit aligned: %d bytes", len(data))
	}

	// Should be bigger than just magic (there's a module block with content).
	if len(data) < 16 {
		t.Errorf("output too small: %d bytes", len(data))
	}
}

func TestSerialize_WithMetadata(t *testing.T) {
	mod := NewModule(VertexShader)
	voidTy := mod.GetVoidType()
	i32 := mod.GetIntType(32)
	funcTy := mod.GetFunctionType(voidTy, nil)
	fn := mod.AddFunction("main", funcTy, false)
	bb := fn.AddBasicBlock("entry")
	bb.AddInstruction(NewRetVoidInstr())

	c0 := mod.AddIntConst(i32, 0)
	c1 := mod.AddIntConst(i32, 1)

	md0 := mod.AddMetadataValue(i32, c0)
	md1 := mod.AddMetadataValue(i32, c1)
	mdTuple := mod.AddMetadataTuple([]*MetadataNode{md1, md0})
	mod.AddNamedMetadata("dx.version", []*MetadataNode{mdTuple})

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with metadata produced empty output")
	}
	if len(data)%4 != 0 {
		t.Errorf("output not 32-bit aligned: %d bytes", len(data))
	}
}

func TestEncodeSignRotated(t *testing.T) {
	// LLVM 3.7 sign-rotated VBR encoding for signed integers:
	//   non-negative N → N << 1
	//   negative N     → ((-N) << 1) | 1
	//
	// Pinned against Mesa dxil_module.c:2590 encode_signed. The previous
	// test expected -1→1 / -2→3 / -100→199, matching the (buggy)
	// '(-N << 1) - 1' form. That form produced encodings that decoded
	// to WRONG values: dxc bitcode reader takes X=1 and decodes
	// -(1>>1) = -0 = 0, not -1. Verified by !{i32 -1} round-trip in
	// dxc -dumpbin: with the new encoding (X=3), dxc displays i32 -1.
	tests := []struct {
		input int64
		want  uint64
	}{
		{0, 0},
		{1, 2},
		{2, 4},
		{-1, 3},
		{-2, 5},
		{100, 200},
		{-100, 201},
	}
	for _, tt := range tests {
		got := encodeSignRotated(tt.input)
		if got != tt.want {
			t.Errorf("encodeSignRotated(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestSerialize_EmptyModule(t *testing.T) {
	// Module with no functions, constants, or metadata.
	mod := NewModule(ComputeShader)
	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("empty module serialization produced nothing")
	}
	if data[0] != 'B' || data[1] != 'C' {
		t.Error("bad magic for empty module")
	}

	// Verify the first 32-bit word after magic is the ENTER_SUBBLOCK encoding.
	if len(data) < 8 {
		t.Fatalf("too small: %d bytes", len(data))
	}
	// After magic (4 bytes), the MODULE_BLOCK enters. We just verify alignment.
	if len(data)%4 != 0 {
		t.Errorf("not 32-bit aligned: %d bytes", len(data))
	}
}

func TestSerialize_Deterministic(t *testing.T) {
	// Verify that serializing the same module twice produces identical output.
	build := func() []byte {
		mod := NewModule(VertexShader)
		voidTy := mod.GetVoidType()
		i32 := mod.GetIntType(32)
		funcTy := mod.GetFunctionType(voidTy, nil)
		fn := mod.AddFunction("main", funcTy, false)
		bb := fn.AddBasicBlock("entry")
		bb.AddInstruction(NewRetVoidInstr())
		c := mod.AddIntConst(i32, 42)
		mdV := mod.AddMetadataValue(i32, c)
		mdT := mod.AddMetadataTuple([]*MetadataNode{mdV})
		mod.AddNamedMetadata("dx.test", []*MetadataNode{mdT})
		return Serialize(mod)
	}

	data1 := build()
	data2 := build()

	if len(data1) != len(data2) {
		t.Fatalf("different lengths: %d vs %d", len(data1), len(data2))
	}

	for i := range data1 {
		if data1[i] != data2[i] {
			// Find dword containing the difference.
			dw := i / 4
			d1 := binary.LittleEndian.Uint32(data1[dw*4:])
			d2 := binary.LittleEndian.Uint32(data2[dw*4:])
			t.Errorf("difference at byte %d (dword %d): 0x%08X vs 0x%08X", i, dw, d1, d2)
			break
		}
	}
}

// TestLog2Uint verifies the log2Uint helper used for alignment encoding.
func TestLog2Uint(t *testing.T) {
	cases := []struct {
		in   uint32
		want uint32
	}{
		{0, 0}, // special case: log2(0) = 0
		{1, 0}, // log2(1) = 0
		{2, 1}, // log2(2) = 1
		{4, 2}, // log2(4) = 2
		{8, 3}, // log2(8) = 3
		{16, 4},
		{3, 1}, // floor(log2(3)) = 1
		{7, 2}, // floor(log2(7)) = 2
	}
	for _, tc := range cases {
		got := log2Uint(tc.in)
		if got != tc.want {
			t.Errorf("log2Uint(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Type constructors (module.go): GetVectorType, GetLabelType, GetMetadataType
// ---------------------------------------------------------------------------

func TestGetVectorType(t *testing.T) {
	mod := NewModule(VertexShader)
	f32 := mod.GetFloatType(32)

	v1 := mod.GetVectorType(f32, 4)
	if v1.Kind != TypeVector {
		t.Errorf("vector kind: got %d, want %d", v1.Kind, TypeVector)
	}
	if v1.ElemType != f32 {
		t.Error("vector element type mismatch")
	}
	if v1.ElemCount != 4 {
		t.Errorf("vector count: got %d, want 4", v1.ElemCount)
	}

	// Deduplication: same (elem, count) must return same pointer.
	v2 := mod.GetVectorType(f32, 4)
	if v1 != v2 {
		t.Error("GetVectorType not deduplicated for same (elem, count)")
	}

	// Different count must return a new type.
	v3 := mod.GetVectorType(f32, 2)
	if v3 == v1 {
		t.Error("GetVectorType returned same pointer for different count")
	}
}

func TestGetLabelType(t *testing.T) {
	mod := NewModule(VertexShader)
	lbl := mod.GetLabelType()
	if lbl.Kind != TypeLabel {
		t.Errorf("label kind: got %d, want %d", lbl.Kind, TypeLabel)
	}
}

func TestGetMetadataType(t *testing.T) {
	mod := NewModule(VertexShader)
	md := mod.GetMetadataType()
	if md.Kind != TypeMetadata {
		t.Errorf("metadata type kind: got %d, want %d", md.Kind, TypeMetadata)
	}
}

// ---------------------------------------------------------------------------
// Non-standard integer/float bit widths (module.go default switch branches)
// ---------------------------------------------------------------------------

func TestGetIntType_NonStandardWidth(t *testing.T) {
	mod := NewModule(VertexShader)
	// 24-bit integer hits the default branch, not cached.
	t1 := mod.GetIntType(24)
	if t1.IntBits != 24 {
		t.Errorf("IntBits: got %d, want 24", t1.IntBits)
	}
	// Not deduplicated for non-standard widths.
	t2 := mod.GetIntType(24)
	if t1 == t2 {
		t.Error("non-standard width should not be deduplicated via cache")
	}
}

func TestGetFloatType_NonStandardWidth(t *testing.T) {
	mod := NewModule(VertexShader)
	// 80-bit float hits the default branch.
	t1 := mod.GetFloatType(80)
	if t1.FloatBits != 80 {
		t.Errorf("FloatBits: got %d, want 80", t1.FloatBits)
	}
}

// ---------------------------------------------------------------------------
// BasicBlock instructions (module.go): PrependInstruction
// ---------------------------------------------------------------------------

func TestPrependInstruction(t *testing.T) {
	mod := NewModule(VertexShader)
	voidTy := mod.GetVoidType()
	funcTy := mod.GetFunctionType(voidTy, nil)
	fn := mod.AddFunction("main", funcTy, false)
	bb := fn.AddBasicBlock("entry")

	retInstr := NewRetVoidInstr()
	bb.AddInstruction(retInstr)

	// Prepend an alloca-like instruction at the front.
	allocaInstr := &Instruction{
		Kind:     InstrAlloca,
		HasValue: true,
	}
	bb.PrependInstruction(allocaInstr)

	if len(bb.Instructions) != 2 {
		t.Fatalf("instruction count: got %d, want 2", len(bb.Instructions))
	}
	if bb.Instructions[0] != allocaInstr {
		t.Error("prepended instruction should be first")
	}
	if bb.Instructions[1] != retInstr {
		t.Error("original instruction should be second")
	}
}

// ---------------------------------------------------------------------------
// Branch instructions (module.go): NewBrInstr, NewBrCondInstr
// ---------------------------------------------------------------------------

func TestNewBrInstr(t *testing.T) {
	instr := NewBrInstr(2)
	if instr.Kind != InstrBr {
		t.Errorf("kind: got %d, want %d", instr.Kind, InstrBr)
	}
	if instr.HasValue {
		t.Error("branch should not produce a value")
	}
	if len(instr.Operands) != 1 || instr.Operands[0] != 2 {
		t.Errorf("operands: got %v, want [2]", instr.Operands)
	}
}

func TestNewBrCondInstr(t *testing.T) {
	instr := NewBrCondInstr(1, 3, 5)
	if instr.Kind != InstrBr {
		t.Errorf("kind: got %d, want %d", instr.Kind, InstrBr)
	}
	if instr.HasValue {
		t.Error("conditional branch should not produce a value")
	}
	if len(instr.Operands) != 3 {
		t.Fatalf("operands length: got %d, want 3", len(instr.Operands))
	}
	if instr.Operands[0] != 1 || instr.Operands[1] != 3 || instr.Operands[2] != 5 {
		t.Errorf("operands: got %v, want [1,3,5]", instr.Operands)
	}
}

// ---------------------------------------------------------------------------
// Phi instruction (module.go): NewPhiInstr
// ---------------------------------------------------------------------------

func TestNewPhiInstr(t *testing.T) {
	mod := NewModule(VertexShader)
	i32 := mod.GetIntType(32)

	incomings := []PhiIncoming{
		{ValueID: 10, BBIndex: 0},
		{ValueID: 20, BBIndex: 1},
	}
	instr := NewPhiInstr(i32, incomings)
	if instr.Kind != InstrPhi {
		t.Errorf("kind: got %d, want %d", instr.Kind, InstrPhi)
	}
	if !instr.HasValue {
		t.Error("phi should produce a value")
	}
	if instr.ResultType != i32 {
		t.Error("phi result type mismatch")
	}
	if len(instr.PhiIncomings) != 2 {
		t.Fatalf("phi incomings: got %d, want 2", len(instr.PhiIncomings))
	}
	if instr.PhiIncomings[0].ValueID != 10 || instr.PhiIncomings[0].BBIndex != 0 {
		t.Errorf("incoming[0]: got %+v", instr.PhiIncomings[0])
	}
}

// ---------------------------------------------------------------------------
// Constant constructors (module.go): AddAggregateConst, AddDataArrayConst,
// AddUndefConst
// ---------------------------------------------------------------------------

func TestAddAggregateConst(t *testing.T) {
	mod := NewModule(VertexShader)
	i32 := mod.GetIntType(32)
	arrTy := mod.GetArrayType(i32, 3)

	e0 := mod.AddIntConst(i32, 10)
	e1 := mod.AddIntConst(i32, 20)
	e2 := mod.AddIntConst(i32, 30)

	agg := mod.AddAggregateConst(arrTy, []*Constant{e0, e1, e2})
	if !agg.IsAggregate {
		t.Error("expected IsAggregate = true")
	}
	if len(agg.Elements) != 3 {
		t.Errorf("elements: got %d, want 3", len(agg.Elements))
	}
	if agg.ConstType != arrTy {
		t.Error("aggregate type mismatch")
	}
}

func TestAddDataArrayConst(t *testing.T) {
	mod := NewModule(VertexShader)
	i32 := mod.GetIntType(32)
	arrTy := mod.GetArrayType(i32, 3)

	dac := mod.AddDataArrayConst(arrTy, []uint64{1, 2, 3})
	if !dac.IsDataArray {
		t.Error("expected IsDataArray = true")
	}
	if len(dac.DataValues) != 3 {
		t.Errorf("data values: got %d, want 3", len(dac.DataValues))
	}
	if dac.DataValues[0] != 1 || dac.DataValues[1] != 2 || dac.DataValues[2] != 3 {
		t.Errorf("data values: got %v, want [1,2,3]", dac.DataValues)
	}
}

func TestAddUndefConst(t *testing.T) {
	mod := NewModule(VertexShader)
	i32 := mod.GetIntType(32)

	u := mod.AddUndefConst(i32)
	if !u.IsUndef {
		t.Error("expected IsUndef = true")
	}
	if u.ConstType != i32 {
		t.Error("undef type mismatch")
	}
}

// ---------------------------------------------------------------------------
// Global variables (module.go): AddGlobalVar
// ---------------------------------------------------------------------------

func TestAddGlobalVar(t *testing.T) {
	mod := NewModule(ComputeShader)
	i32 := mod.GetIntType(32)

	gv := mod.AddGlobalVar("g_counter", i32, 0)
	if gv.Name != "g_counter" {
		t.Errorf("name: got %q, want %q", gv.Name, "g_counter")
	}
	if gv.VarType != i32 {
		t.Error("global var type mismatch")
	}
	if gv.AddrSpace != 0 {
		t.Errorf("addrspace: got %d, want 0", gv.AddrSpace)
	}
	if gv.Linkage != -1 {
		t.Errorf("default linkage: got %d, want -1", gv.Linkage)
	}
	if len(mod.GlobalVars) != 1 {
		t.Errorf("global var count: got %d, want 1", len(mod.GlobalVars))
	}
}

func TestAddGlobalVar_Workgroup(t *testing.T) {
	mod := NewModule(ComputeShader)
	i32 := mod.GetIntType(32)

	gv := mod.AddGlobalVar("g_shared", i32, 3)
	if gv.AddrSpace != 3 {
		t.Errorf("addrspace: got %d, want 3 (workgroup)", gv.AddrSpace)
	}
}

// ---------------------------------------------------------------------------
// Function attribute sets (module.go): AddFunction declaration vs definition
// ---------------------------------------------------------------------------

func TestAddFunction_DeclarationAttrSet(t *testing.T) {
	mod := NewModule(VertexShader)
	voidTy := mod.GetVoidType()
	funcTy := mod.GetFunctionType(voidTy, nil)

	// Declaration should get AttrSetNounwind.
	decl := mod.AddFunction("dx.op.loadInput.f32", funcTy, true)
	if decl.AttrSetID != AttrSetNounwind {
		t.Errorf("declaration AttrSetID: got %d, want %d (AttrSetNounwind)", decl.AttrSetID, AttrSetNounwind)
	}

	// Definition should get AttrSetNone.
	def := mod.AddFunction("main", funcTy, false)
	if def.AttrSetID != AttrSetNone {
		t.Errorf("definition AttrSetID: got %d, want %d (AttrSetNone)", def.AttrSetID, AttrSetNone)
	}
}

// ---------------------------------------------------------------------------
// Metadata function reference (metadata.go): AddMetadataFunc
// ---------------------------------------------------------------------------

func TestAddMetadataFunc(t *testing.T) {
	mod := NewModule(VertexShader)
	voidTy := mod.GetVoidType()
	funcTy := mod.GetFunctionType(voidTy, nil)
	fn := mod.AddFunction("main", funcTy, false)

	mdFn := mod.AddMetadataFunc(fn)
	if mdFn.Kind != MDValue {
		t.Errorf("kind: got %d, want %d (MDValue)", mdFn.Kind, MDValue)
	}
	if mdFn.ValueFunc != fn {
		t.Error("ValueFunc mismatch")
	}
	if mdFn.ValueType == nil {
		t.Fatal("ValueType should be pointer-to-function type, got nil")
	}
	if mdFn.ValueType.Kind != TypePointer {
		t.Errorf("ValueType kind: got %d, want %d (TypePointer)", mdFn.ValueType.Kind, TypePointer)
	}
	if mdFn.ValueType.PointerElem != funcTy {
		t.Error("ValueType.PointerElem should be function type")
	}
}

// ---------------------------------------------------------------------------
// Array type deduplication (module.go): GetArrayType second call path
// ---------------------------------------------------------------------------

func TestGetArrayType_Deduplication(t *testing.T) {
	mod := NewModule(VertexShader)
	i32 := mod.GetIntType(32)

	a1 := mod.GetArrayType(i32, 10)
	a2 := mod.GetArrayType(i32, 10)
	if a1 != a2 {
		t.Error("GetArrayType not deduplicated for same (elem, count)")
	}

	a3 := mod.GetArrayType(i32, 5)
	if a3 == a1 {
		t.Error("GetArrayType returned same pointer for different count")
	}
}

// ---------------------------------------------------------------------------
// Pointer type with address space (module.go): GetPointerTypeAS
// ---------------------------------------------------------------------------

func TestGetPointerTypeAS(t *testing.T) {
	mod := NewModule(ComputeShader)
	i32 := mod.GetIntType(32)

	p0 := mod.GetPointerTypeAS(i32, 0)
	p3 := mod.GetPointerTypeAS(i32, 3)
	if p0 == p3 {
		t.Error("different address spaces should produce different types")
	}
	if p3.PointerAddrSpace != 3 {
		t.Errorf("addrspace: got %d, want 3", p3.PointerAddrSpace)
	}

	// Same addrspace should be deduplicated.
	p3b := mod.GetPointerTypeAS(i32, 3)
	if p3 != p3b {
		t.Error("GetPointerTypeAS not deduplicated for same (elem, addrspace)")
	}
}

// ---------------------------------------------------------------------------
// Serialization: float constants (serialize.go): floatBits, float32ToF16Bits
// ---------------------------------------------------------------------------

func TestSerialize_FloatConstants(t *testing.T) {
	mod := NewModule(VertexShader)
	voidTy := mod.GetVoidType()
	f16 := mod.GetFloatType(16)
	f32 := mod.GetFloatType(32)
	f64 := mod.GetFloatType(64)
	funcTy := mod.GetFunctionType(voidTy, nil)
	fn := mod.AddFunction("main", funcTy, false)
	bb := fn.AddBasicBlock("entry")
	bb.AddInstruction(NewRetVoidInstr())

	// Add float constants of all three precisions to exercise floatBits paths.
	mod.Constants = append(mod.Constants, &Constant{ConstType: f16, FloatValue: 1.0})
	mod.Constants = append(mod.Constants, &Constant{ConstType: f32, FloatValue: 3.14})
	mod.Constants = append(mod.Constants, &Constant{ConstType: f64, FloatValue: 2.718281828})

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization produced empty output")
	}
	if len(data)%4 != 0 {
		t.Errorf("output not 32-bit aligned: %d bytes", len(data))
	}
}

func TestFloat32ToF16Bits(t *testing.T) {
	tests := []struct {
		name string
		in   float32
		want uint16
	}{
		{"positive zero", 0.0, 0x0000},
		{"negative zero", float32(math.Copysign(0, -1)), 0x8000},
		{"one", 1.0, 0x3C00},
		{"neg one", -1.0, 0xBC00},
		{"positive infinity", float32(math.Inf(1)), 0x7C00},
		{"negative infinity", float32(math.Inf(-1)), 0xFC00},
		{"NaN", float32(math.NaN()), 0x7E00},
		{"overflow to inf", 100000.0, 0x7C00},
		{"small subnormal", float32(math.Ldexp(1, -24)), 0x0001},
		{"underflow to zero", float32(math.Ldexp(1, -30)), 0x0000},
		// Rounding carry: frac rounds up causing frac overflow (0x3FF->0x400),
		// which bumps the exponent. Value just below 2.0 with fraction all 1s.
		// f32 bits: sign=0, exp=127 (biased), frac=0x7FFFFF (max fraction)
		// In f16 normal range (exp=0). f16Frac = 0x7FFFFF>>13 = 0x3FF,
		// remainder = 0x7FFFFF & 0x1FFF = 0x1FFF > 0x1000, so f16Frac++
		// becomes 0x400 >= 0x400, so f16Frac=0, exp++ (0->1).
		// Result: sign=0, exp=1+15=16<<10=0x4000, frac=0 → 0x4000 (=2.0)
		{"rounding carry to exp", math.Float32frombits(0x3FFFFFFF), 0x4000},
		// Rounding carry that overflows f16 exp (exp=15 + carry → >15 → inf).
		// f32 value just below 65536: f32 with exp=142(biased)=15(unbiased),
		// frac=0x7FFFFF. f16Frac=0x3FF, remainder=0x1FFF → carry → 0x400,
		// exp becomes 16 > 15 → infinity.
		{"rounding carry to inf", math.Float32frombits(0x477FFFFF), 0x7C00},
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
// Serialization: undef, aggregate, dataarray constants
// ---------------------------------------------------------------------------

func TestSerialize_UndefConstant(t *testing.T) {
	mod := NewModule(VertexShader)
	voidTy := mod.GetVoidType()
	i32 := mod.GetIntType(32)
	funcTy := mod.GetFunctionType(voidTy, nil)
	fn := mod.AddFunction("main", funcTy, false)
	bb := fn.AddBasicBlock("entry")
	bb.AddInstruction(NewRetVoidInstr())

	mod.AddUndefConst(i32)

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with undef produced empty output")
	}
}

func TestSerialize_AggregateConstant(t *testing.T) {
	mod := NewModule(VertexShader)
	voidTy := mod.GetVoidType()
	i32 := mod.GetIntType(32)
	arrTy := mod.GetArrayType(i32, 2)
	funcTy := mod.GetFunctionType(voidTy, nil)
	fn := mod.AddFunction("main", funcTy, false)
	bb := fn.AddBasicBlock("entry")
	bb.AddInstruction(NewRetVoidInstr())

	e0 := mod.AddIntConst(i32, 100)
	e1 := mod.AddIntConst(i32, 200)
	mod.AddAggregateConst(arrTy, []*Constant{e0, e1})

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with aggregate produced empty output")
	}
}

func TestSerialize_DataArrayConstant(t *testing.T) {
	mod := NewModule(VertexShader)
	voidTy := mod.GetVoidType()
	i32 := mod.GetIntType(32)
	arrTy := mod.GetArrayType(i32, 3)
	funcTy := mod.GetFunctionType(voidTy, nil)
	fn := mod.AddFunction("main", funcTy, false)
	bb := fn.AddBasicBlock("entry")
	bb.AddInstruction(NewRetVoidInstr())

	mod.AddDataArrayConst(arrTy, []uint64{0, 1, 2})

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with data array produced empty output")
	}
}

// ---------------------------------------------------------------------------
// Serialization: global variables (serialize.go): emitGlobalVarDecl
// ---------------------------------------------------------------------------

func TestSerialize_GlobalVar(t *testing.T) {
	mod := NewModule(ComputeShader)
	voidTy := mod.GetVoidType()
	i32 := mod.GetIntType(32)
	funcTy := mod.GetFunctionType(voidTy, nil)
	fn := mod.AddFunction("main", funcTy, false)
	bb := fn.AddBasicBlock("entry")
	bb.AddInstruction(NewRetVoidInstr())

	// Global without initializer (external linkage by default).
	mod.AddGlobalVar("g_counter", i32, 0)

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with global var produced empty output")
	}
	if len(data)%4 != 0 {
		t.Errorf("not 32-bit aligned: %d bytes", len(data))
	}
}

func TestSerialize_GlobalVarWithInitializer(t *testing.T) {
	mod := NewModule(ComputeShader)
	voidTy := mod.GetVoidType()
	i32 := mod.GetIntType(32)
	funcTy := mod.GetFunctionType(voidTy, nil)
	fn := mod.AddFunction("main", funcTy, false)
	bb := fn.AddBasicBlock("entry")
	bb.AddInstruction(NewRetVoidInstr())

	initVal := mod.AddIntConst(i32, 42)
	gv := mod.AddGlobalVar("g_init", i32, 0)
	gv.Initializer = initVal
	gv.IsConstant = true

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with initialized global produced empty output")
	}
}

func TestSerialize_GlobalVarExplicitLinkage(t *testing.T) {
	mod := NewModule(ComputeShader)
	voidTy := mod.GetVoidType()
	i32 := mod.GetIntType(32)
	funcTy := mod.GetFunctionType(voidTy, nil)
	fn := mod.AddFunction("main", funcTy, false)
	bb := fn.AddBasicBlock("entry")
	bb.AddInstruction(NewRetVoidInstr())

	// External linkage with undef initializer (decomposed groupshared).
	undef := mod.AddUndefConst(i32)
	gv := mod.AddGlobalVar("g_shared", i32, 3)
	gv.Initializer = undef
	gv.Linkage = 0 // explicit external
	gv.Alignment = 8

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with explicit-linkage global produced empty output")
	}
}

// ---------------------------------------------------------------------------
// Serialization: type coverage — label, metadata, vector, anon struct
// (serialize.go): emitType all branches
// ---------------------------------------------------------------------------

func TestSerialize_AllTypeKinds(t *testing.T) {
	mod := NewModule(VertexShader)
	voidTy := mod.GetVoidType()
	i32 := mod.GetIntType(32)
	f32 := mod.GetFloatType(32)
	f16 := mod.GetFloatType(16)
	f64 := mod.GetFloatType(64)
	funcTy := mod.GetFunctionType(voidTy, nil)

	// Exercise type kinds not covered by minimal tests.
	mod.GetLabelType()
	mod.GetMetadataType()
	mod.GetVectorType(f32, 4)
	mod.GetArrayType(i32, 8)
	mod.GetPointerType(i32)
	mod.GetPointerTypeAS(i32, 3)

	// Anonymous struct (empty StructName).
	mod.addType(&Type{Kind: TypeStruct, StructElems: []*Type{i32, f32}})

	// Named struct.
	mod.GetStructType("TestStruct", []*Type{f16, f64})

	fn := mod.AddFunction("main", funcTy, false)
	bb := fn.AddBasicBlock("entry")
	bb.AddInstruction(NewRetVoidInstr())

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with all type kinds produced empty output")
	}
	if len(data)%4 != 0 {
		t.Errorf("not 32-bit aligned: %d bytes", len(data))
	}
}

// ---------------------------------------------------------------------------
// Serialization: metadata function reference (serialize.go): emitMetadataValue
// with ValueFunc path
// ---------------------------------------------------------------------------

func TestSerialize_MetadataFunc(t *testing.T) {
	mod := NewModule(VertexShader)
	voidTy := mod.GetVoidType()
	funcTy := mod.GetFunctionType(voidTy, nil)
	fn := mod.AddFunction("main", funcTy, false)
	bb := fn.AddBasicBlock("entry")
	bb.AddInstruction(NewRetVoidInstr())

	mdFn := mod.AddMetadataFunc(fn)
	mdTuple := mod.AddMetadataTuple([]*MetadataNode{mdFn})
	mod.AddNamedMetadata("dx.entryPoints", []*MetadataNode{mdTuple})

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with metadata func produced empty output")
	}
}

// ---------------------------------------------------------------------------
// Serialization: metadata string directly
// ---------------------------------------------------------------------------

func TestSerialize_MetadataStringOnly(t *testing.T) {
	mod := NewModule(VertexShader)
	voidTy := mod.GetVoidType()
	funcTy := mod.GetFunctionType(voidTy, nil)
	fn := mod.AddFunction("main", funcTy, false)
	bb := fn.AddBasicBlock("entry")
	bb.AddInstruction(NewRetVoidInstr())

	mdStr := mod.AddMetadataString("vs_6_0")
	mod.AddNamedMetadata("dx.shaderModel", []*MetadataNode{mdStr})

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with metadata string produced empty output")
	}
}

// ---------------------------------------------------------------------------
// Serialization: instruction kinds (serialize.go): emitInstruction all branches
// ---------------------------------------------------------------------------

// buildInstrTestModule creates a module with a main function containing a
// single entry basic block, returns the module, function, and entry block.
func buildInstrTestModule() (*Module, *Function, *BasicBlock) {
	mod := NewModule(VertexShader)
	voidTy := mod.GetVoidType()
	i32 := mod.GetIntType(32)
	funcTy := mod.GetFunctionType(voidTy, []*Type{i32})
	fn := mod.AddFunction("main", funcTy, false)
	bb := fn.AddBasicBlock("entry")
	return mod, fn, bb
}

func TestSerialize_InstrBinOp(t *testing.T) {
	mod, _, bb := buildInstrTestModule()
	// BinOp with two operands and opcode (add = 0).
	bb.AddInstruction(&Instruction{
		Kind:     InstrBinOp,
		HasValue: true,
		Operands: []int{0, 0, 0}, // lhs, rhs, opcode
	})
	bb.AddInstruction(NewRetVoidInstr())

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with binop produced empty output")
	}
}

func TestSerialize_InstrBinOpWithFlags(t *testing.T) {
	mod, _, bb := buildInstrTestModule()
	// BinOp with fast-math flags.
	bb.AddInstruction(&Instruction{
		Kind:     InstrBinOp,
		HasValue: true,
		Operands: []int{0, 0, 13}, // lhs, rhs, fadd opcode
		Flags:    1,               // UnsafeAlgebra
	})
	bb.AddInstruction(NewRetVoidInstr())

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with binop+flags produced empty output")
	}
}

func TestSerialize_InstrCmp(t *testing.T) {
	mod, _, bb := buildInstrTestModule()
	bb.AddInstruction(&Instruction{
		Kind:     InstrCmp,
		HasValue: true,
		Operands: []int{0, 0, 32}, // lhs, rhs, icmp eq predicate
	})
	bb.AddInstruction(NewRetVoidInstr())

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with cmp produced empty output")
	}
}

func TestSerialize_InstrCall(t *testing.T) {
	mod, _, bb := buildInstrTestModule()
	voidTy := mod.GetVoidType()
	i32 := mod.GetIntType(32)
	intrTy := mod.GetFunctionType(voidTy, []*Type{i32})
	intrFn := mod.AddFunction("dx.op.storeOutput.f32", intrTy, true)

	bb.AddInstruction(&Instruction{
		Kind:       InstrCall,
		HasValue:   false,
		CalledFunc: intrFn,
		Operands:   []int{0}, // one argument
	})
	bb.AddInstruction(NewRetVoidInstr())

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with call produced empty output")
	}
}

func TestSerialize_InstrSelect(t *testing.T) {
	mod, _, bb := buildInstrTestModule()
	i32 := mod.GetIntType(32)
	bb.AddInstruction(&Instruction{
		Kind:       InstrSelect,
		HasValue:   true,
		ResultType: i32,
		Operands:   []int{0, 0, 0}, // cond, trueVal, falseVal
	})
	bb.AddInstruction(NewRetVoidInstr())

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with select produced empty output")
	}
}

func TestSerialize_InstrCast(t *testing.T) {
	mod, _, bb := buildInstrTestModule()
	f32 := mod.GetFloatType(32)
	bb.AddInstruction(&Instruction{
		Kind:       InstrCast,
		HasValue:   true,
		ResultType: f32,
		Operands:   []int{0, 8}, // src, SIToFP opcode
	})
	bb.AddInstruction(NewRetVoidInstr())

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with cast produced empty output")
	}
}

func TestSerialize_InstrBranch(t *testing.T) {
	mod, fn, bb := buildInstrTestModule()

	// Create two additional basic blocks as targets.
	bbTrue := fn.AddBasicBlock("if.true")
	bbFalse := fn.AddBasicBlock("if.false")

	// Conditional branch from entry.
	bb.AddInstruction(NewBrCondInstr(1, 2, 0)) // trueBB=1, falseBB=2, cond=param0

	// Unconditional branch in true block to false block.
	bbTrue.AddInstruction(NewBrInstr(2))

	// Return in false block.
	bbFalse.AddInstruction(NewRetVoidInstr())

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with branches produced empty output")
	}
}

func TestSerialize_InstrExtractVal(t *testing.T) {
	mod, _, bb := buildInstrTestModule()
	i32 := mod.GetIntType(32)
	bb.AddInstruction(&Instruction{
		Kind:       InstrExtractVal,
		HasValue:   true,
		ResultType: i32,
		Operands:   []int{0, 0}, // aggregate, index
	})
	bb.AddInstruction(NewRetVoidInstr())

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with extractval produced empty output")
	}
}

func TestSerialize_InstrInsertVal(t *testing.T) {
	mod, _, bb := buildInstrTestModule()
	i32 := mod.GetIntType(32)
	bb.AddInstruction(&Instruction{
		Kind:       InstrInsertVal,
		HasValue:   true,
		ResultType: i32,
		Operands:   []int{0, 0, 0}, // aggregate, value, index
	})
	bb.AddInstruction(NewRetVoidInstr())

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with insertval produced empty output")
	}
}

func TestSerialize_InstrAlloca(t *testing.T) {
	mod, _, bb := buildInstrTestModule()
	i32 := mod.GetIntType(32)
	bb.AddInstruction(&Instruction{
		Kind:       InstrAlloca,
		HasValue:   true,
		ResultType: mod.GetPointerType(i32),
		Operands:   []int{i32.ID, i32.ID, 0, 3}, // allocType, sizeType, sizeValue, alignFlags
	})
	bb.AddInstruction(NewRetVoidInstr())

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with alloca produced empty output")
	}
}

func TestSerialize_InstrLoadStore(t *testing.T) {
	mod, _, bb := buildInstrTestModule()
	i32 := mod.GetIntType(32)
	ptrI32 := mod.GetPointerType(i32)

	// Alloca producing a pointer.
	bb.AddInstruction(&Instruction{
		Kind:       InstrAlloca,
		HasValue:   true,
		ResultType: ptrI32,
		Operands:   []int{i32.ID, i32.ID, 0, 3},
	})
	// Load from the alloca.
	bb.AddInstruction(&Instruction{
		Kind:       InstrLoad,
		HasValue:   true,
		ResultType: i32,
		Operands:   []int{1, i32.ID, 2, 0}, // ptr(valID=1), typeID, align, volatile
	})
	// Store to the alloca.
	bb.AddInstruction(&Instruction{
		Kind:     InstrStore,
		HasValue: false,
		Operands: []int{1, 2, 2, 0}, // ptr, value, align, volatile
	})
	bb.AddInstruction(NewRetVoidInstr())

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with load/store produced empty output")
	}
}

func TestSerialize_InstrGEP(t *testing.T) {
	mod, _, bb := buildInstrTestModule()
	i32 := mod.GetIntType(32)
	bb.AddInstruction(&Instruction{
		Kind:       InstrGEP,
		HasValue:   true,
		ResultType: mod.GetPointerType(i32),
		Operands:   []int{1, i32.ID, 0, 0}, // inbounds=1, elemTypeID, ptr, idx
	})
	bb.AddInstruction(NewRetVoidInstr())

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with GEP produced empty output")
	}
}

func TestSerialize_InstrAtomicRMW(t *testing.T) {
	mod, _, bb := buildInstrTestModule()
	i32 := mod.GetIntType(32)
	bb.AddInstruction(&Instruction{
		Kind:       InstrAtomicRMW,
		HasValue:   true,
		ResultType: i32,
		// ptr, val, op(add=0), volatile=0, ordering=4(monotonic), synchscope=1
		Operands: []int{0, 0, 0, 0, 4, 1},
	})
	bb.AddInstruction(NewRetVoidInstr())

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with atomicrmw produced empty output")
	}
}

func TestSerialize_InstrCmpXchg(t *testing.T) {
	mod, _, bb := buildInstrTestModule()
	i32 := mod.GetIntType(32)
	bb.AddInstruction(&Instruction{
		Kind:       InstrCmpXchg,
		HasValue:   true,
		ResultType: i32,
		// ptr, cmp, new, volatile=0, ordering=4, synchscope=1
		Operands: []int{0, 0, 0, 0, 4, 1},
	})
	bb.AddInstruction(NewRetVoidInstr())

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with cmpxchg produced empty output")
	}
}

func TestSerialize_InstrPhi(t *testing.T) {
	mod := NewModule(VertexShader)
	voidTy := mod.GetVoidType()
	i32 := mod.GetIntType(32)
	funcTy := mod.GetFunctionType(voidTy, []*Type{i32})
	fn := mod.AddFunction("main", funcTy, false)

	bb0 := fn.AddBasicBlock("bb0")
	bb1 := fn.AddBasicBlock("bb1")
	bb2 := fn.AddBasicBlock("merge")

	// bb0 → bb2
	bb0.AddInstruction(NewBrInstr(2))
	// bb1 → bb2
	bb1.AddInstruction(NewBrInstr(2))

	// Phi in merge block.
	phi := NewPhiInstr(i32, []PhiIncoming{
		{ValueID: 0, BBIndex: 0},
		{ValueID: 0, BBIndex: 1},
	})
	bb2.AddInstruction(phi)
	bb2.AddInstruction(NewRetVoidInstr())

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with phi produced empty output")
	}
}

func TestSerialize_InstrRetWithValue(t *testing.T) {
	mod := NewModule(VertexShader)
	i32 := mod.GetIntType(32)
	funcTy := mod.GetFunctionType(i32, nil)
	fn := mod.AddFunction("main", funcTy, false)
	bb := fn.AddBasicBlock("entry")

	// Return with value (the function's first constant).
	c := mod.AddIntConst(i32, 0)
	bb.AddInstruction(&Instruction{
		Kind:        InstrRet,
		HasValue:    false,
		ReturnValue: c.ValueID, // will be assigned during serialize
	})

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with ret-value produced empty output")
	}
}

// ---------------------------------------------------------------------------
// Serialization: null constant (default branch in emitConstant)
// ---------------------------------------------------------------------------

func TestSerialize_NullConstant(t *testing.T) {
	mod := NewModule(VertexShader)
	voidTy := mod.GetVoidType()
	funcTy := mod.GetFunctionType(voidTy, nil)
	fn := mod.AddFunction("main", funcTy, false)
	bb := fn.AddBasicBlock("entry")
	bb.AddInstruction(NewRetVoidInstr())

	// A Constant with a pointer type and no special flags hits the default
	// case in emitConstant (constCodeNull).
	ptrTy := mod.GetPointerType(mod.GetIntType(32))
	mod.Constants = append(mod.Constants, &Constant{ConstType: ptrTy})

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with null constant produced empty output")
	}
}

// ---------------------------------------------------------------------------
// Serialization: value symtab with global vars
// ---------------------------------------------------------------------------

func TestSerialize_ValueSymtabGlobalVars(t *testing.T) {
	mod := NewModule(ComputeShader)
	voidTy := mod.GetVoidType()
	i32 := mod.GetIntType(32)
	funcTy := mod.GetFunctionType(voidTy, nil)
	fn := mod.AddFunction("main", funcTy, false)
	bb := fn.AddBasicBlock("entry")
	bb.AddInstruction(NewRetVoidInstr())

	mod.AddGlobalVar("g_a", i32, 0)
	mod.AddGlobalVar("g_b", i32, 3)

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with global vars in symtab produced empty output")
	}
}

// ---------------------------------------------------------------------------
// Serialization: function declaration attrs (emitFunctionDecl AttrSetID)
// ---------------------------------------------------------------------------

func TestSerialize_FunctionDeclWithAttrs(t *testing.T) {
	mod := NewModule(VertexShader)
	voidTy := mod.GetVoidType()
	i32 := mod.GetIntType(32)
	intrTy := mod.GetFunctionType(i32, []*Type{i32})
	funcTy := mod.GetFunctionType(voidTy, nil)

	// Declaration with readnone attrs.
	decl := mod.AddFunction("dx.op.threadId.i32", intrTy, true)
	decl.AttrSetID = AttrSetReadNone

	// Definition.
	fn := mod.AddFunction("main", funcTy, false)
	bb := fn.AddBasicBlock("entry")
	bb.AddInstruction(NewRetVoidInstr())

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with function decl attrs produced empty output")
	}
}

// ---------------------------------------------------------------------------
// Serialization: no data layout (emitModule branch)
// ---------------------------------------------------------------------------

func TestSerialize_NoDataLayout(t *testing.T) {
	mod := NewModule(VertexShader)
	mod.DataLayout = "" // explicitly empty
	voidTy := mod.GetVoidType()
	funcTy := mod.GetFunctionType(voidTy, nil)
	fn := mod.AddFunction("main", funcTy, false)
	bb := fn.AddBasicBlock("entry")
	bb.AddInstruction(NewRetVoidInstr())

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization without data layout produced empty output")
	}
}

// ---------------------------------------------------------------------------
// Serialization: complex module with all features exercises overall coverage
// ---------------------------------------------------------------------------

func TestSerialize_ComplexModule(t *testing.T) {
	mod := NewModule(ComputeShader)
	voidTy := mod.GetVoidType()
	i1 := mod.GetIntType(1)
	i32 := mod.GetIntType(32)
	f32 := mod.GetFloatType(32)
	f16 := mod.GetFloatType(16)
	f64 := mod.GetFloatType(64)
	ptrI32 := mod.GetPointerType(i32)

	// Vector type for texture structs.
	mod.GetVectorType(f32, 4)

	// Array type.
	arrTy := mod.GetArrayType(i32, 4)

	// Label and metadata types.
	mod.GetLabelType()
	mod.GetMetadataType()

	// Workgroup pointer.
	mod.GetPointerTypeAS(i32, 3)

	// Global variables.
	gv := mod.AddGlobalVar("g_shared", i32, 3)
	gv.Alignment = 4
	mod.AddGlobalVar("g_data", arrTy, 0)

	// Intrinsic declarations.
	loadIntrTy := mod.GetFunctionType(f32, []*Type{i32, i32, i32, i32, i32})
	loadDecl := mod.AddFunction("dx.op.loadInput.f32", loadIntrTy, true)
	loadDecl.AttrSetID = AttrSetReadNone

	storeIntrTy := mod.GetFunctionType(voidTy, []*Type{i32, i32, i32, i32, f32})
	storeDecl := mod.AddFunction("dx.op.storeOutput.f32", storeIntrTy, true)
	_ = storeDecl

	// Main function.
	mainTy := mod.GetFunctionType(voidTy, nil)
	fn := mod.AddFunction("main", mainTy, false)
	bb := fn.AddBasicBlock("entry")

	// Various constants.
	mod.AddIntConst(i32, 0)
	mod.AddIntConst(i32, -1)
	mod.AddIntConst(i1, 1)
	mod.Constants = append(mod.Constants, &Constant{ConstType: f32, FloatValue: 1.5})
	mod.Constants = append(mod.Constants, &Constant{ConstType: f16, FloatValue: 0.5})
	mod.Constants = append(mod.Constants, &Constant{ConstType: f64, FloatValue: 2.5})
	mod.AddUndefConst(ptrI32)

	// Instructions.
	bb.AddInstruction(&Instruction{
		Kind:       InstrCall,
		HasValue:   true,
		ResultType: f32,
		CalledFunc: loadDecl,
		Operands:   []int{3, 3, 3, 3, 3}, // constant operands
	})
	bb.AddInstruction(NewRetVoidInstr())

	// Metadata.
	mdStr := mod.AddMetadataString("cs_6_0")
	mdFn := mod.AddMetadataFunc(fn)
	mdI32 := mod.AddMetadataValue(i32, mod.Constants[0])
	mdTuple := mod.AddMetadataTuple([]*MetadataNode{mdStr, mdFn, mdI32, nil})
	mod.AddNamedMetadata("dx.shaderModel", []*MetadataNode{mdTuple})

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("complex module serialization produced empty output")
	}
	if len(data)%4 != 0 {
		t.Errorf("not 32-bit aligned: %d bytes", len(data))
	}

	// Verify magic.
	if data[0] != 'B' || data[1] != 'C' || data[2] != 0xC0 || data[3] != 0xDE {
		t.Errorf("bad magic: %02X %02X %02X %02X", data[0], data[1], data[2], data[3])
	}
}

// ---------------------------------------------------------------------------
// Serialization: function with parameters (emitFunctionBody param offset)
// ---------------------------------------------------------------------------

func TestSerialize_FunctionWithParams(t *testing.T) {
	mod := NewModule(VertexShader)
	voidTy := mod.GetVoidType()
	i32 := mod.GetIntType(32)
	funcTy := mod.GetFunctionType(voidTy, []*Type{i32, i32, i32})
	fn := mod.AddFunction("main", funcTy, false)
	bb := fn.AddBasicBlock("entry")
	bb.AddInstruction(NewRetVoidInstr())

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with params produced empty output")
	}
}

// ---------------------------------------------------------------------------
// Serialization: multiple constants with type changes (emitConstants SETTYPE)
// ---------------------------------------------------------------------------

func TestSerialize_ConstantsTypeChange(t *testing.T) {
	mod := NewModule(VertexShader)
	voidTy := mod.GetVoidType()
	i32 := mod.GetIntType(32)
	f32 := mod.GetFloatType(32)
	funcTy := mod.GetFunctionType(voidTy, nil)
	fn := mod.AddFunction("main", funcTy, false)
	bb := fn.AddBasicBlock("entry")
	bb.AddInstruction(NewRetVoidInstr())

	// Alternate types to force SETTYPE records.
	mod.AddIntConst(i32, 1)
	mod.Constants = append(mod.Constants, &Constant{ConstType: f32, FloatValue: 1.0})
	mod.AddIntConst(i32, 2)
	mod.Constants = append(mod.Constants, &Constant{ConstType: f32, FloatValue: 2.0})

	data := Serialize(mod)
	if len(data) == 0 {
		t.Fatal("serialization with type changes produced empty output")
	}
}
