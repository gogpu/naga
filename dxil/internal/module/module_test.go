package module

import (
	"encoding/binary"
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
