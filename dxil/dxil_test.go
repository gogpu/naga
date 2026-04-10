package dxil

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogpu/naga/dxil/internal/container"
	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/ir"
)

func TestCompile_NilModule(t *testing.T) {
	_, err := Compile(nil, DefaultOptions())
	if err == nil {
		t.Fatal("expected error for nil module")
	}
}

func TestCompile_NoEntryPoints(t *testing.T) {
	irMod := &ir.Module{}
	_, err := Compile(irMod, DefaultOptions())
	if err == nil {
		t.Fatal("expected error for module with no entry points")
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	if opts.ShaderModel != SM6_0 {
		t.Errorf("default shader model: got %v, want SM6_0", opts.ShaderModel)
	}
	if !opts.UseBypassHash {
		t.Error("expected UseBypassHash=true by default")
	}
}

func TestMinimalDXILModule(t *testing.T) {
	// Build a minimal DXIL module:
	// - target triple "dxil-ms-dx"
	// - one empty vertex shader function @main of type void()
	// - dx.version = {1, 0}
	// - dx.shaderModel = {vs, 6, 0}
	// - dx.entryPoints = {{@main, "main", null, null, null}}

	mod := module.NewModule(module.VertexShader)

	// Create types.
	voidTy := mod.GetVoidType()
	i32Ty := mod.GetIntType(32)
	funcTy := mod.GetFunctionType(voidTy, nil) // void()

	// Add main function with an empty body that just returns void.
	mainFn := mod.AddFunction("main", funcTy, false)
	bb := mainFn.AddBasicBlock("entry")
	bb.AddInstruction(module.NewRetVoidInstr())

	// Add constants for metadata values.
	const0 := mod.AddIntConst(i32Ty, 0)
	const1 := mod.AddIntConst(i32Ty, 1)
	const6 := mod.AddIntConst(i32Ty, 6)

	// Build metadata:
	// !dx.version = !{!N} where !N = !{i32 1, i32 0}
	mdVer1 := mod.AddMetadataValue(i32Ty, const1)
	mdVer0 := mod.AddMetadataValue(i32Ty, const0)
	mdVersionTuple := mod.AddMetadataTuple([]*module.MetadataNode{mdVer1, mdVer0})
	mod.AddNamedMetadata("dx.version", []*module.MetadataNode{mdVersionTuple})

	// !dx.shaderModel = !{!M} where !M = !{!"vs", i32 6, i32 0}
	mdVS := mod.AddMetadataString("vs")
	mdSM6 := mod.AddMetadataValue(i32Ty, const6)
	mdSM0 := mod.AddMetadataValue(i32Ty, const0)
	mdShaderModelTuple := mod.AddMetadataTuple([]*module.MetadataNode{mdVS, mdSM6, mdSM0})
	mod.AddNamedMetadata("dx.shaderModel", []*module.MetadataNode{mdShaderModelTuple})

	// !dx.entryPoints = !{!E} where !E = !{void()* @main, !"main", null, null, null}
	mdMainName := mod.AddMetadataString("main")
	mdEntryTuple := mod.AddMetadataTuple([]*module.MetadataNode{
		nil,        // function reference (simplified for Phase 0)
		mdMainName, // entry point name
		nil,        // signatures (null)
		nil,        // resources (null)
		nil,        // properties (null)
	})
	mod.AddNamedMetadata("dx.entryPoints", []*module.MetadataNode{mdEntryTuple})

	// Serialize to bitcode.
	bitcodeData := module.Serialize(mod)
	if len(bitcodeData) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}

	// Verify bitcode magic.
	if bitcodeData[0] != 'B' || bitcodeData[1] != 'C' ||
		bitcodeData[2] != 0xC0 || bitcodeData[3] != 0xDE {
		t.Errorf("invalid bitcode magic: %02X %02X %02X %02X",
			bitcodeData[0], bitcodeData[1], bitcodeData[2], bitcodeData[3])
	}

	// Verify 32-bit alignment.
	if len(bitcodeData)%4 != 0 {
		t.Errorf("bitcode not 32-bit aligned: %d bytes", len(bitcodeData))
	}

	// Wrap in DXBC container.
	c := container.New()
	c.AddFeaturesPart(0) // no special features
	c.AddDXILPart(uint32(module.VertexShader), 1, 0, bitcodeData)
	c.AddHashPart()
	containerData := c.Bytes()

	// Verify DXBC header.
	if len(containerData) < 28 {
		t.Fatalf("container too small: %d bytes", len(containerData))
	}
	magic := binary.LittleEndian.Uint32(containerData[0:4])
	if magic != container.FourCCDXBC {
		t.Errorf("container magic: got 0x%08X, want 0x%08X (DXBC)", magic, container.FourCCDXBC)
	}

	// Verify file size field (offset 24 in DXBC header).
	fileSize := binary.LittleEndian.Uint32(containerData[24:28])
	if fileSize != uint32(len(containerData)) {
		t.Errorf("file size: got %d, want %d", fileSize, len(containerData))
	}

	// Apply BYPASS hash.
	container.SetBypassHash(containerData)

	// Verify BYPASS hash was written.
	for i := 4; i < 20; i++ {
		if containerData[i] != 0x01 {
			t.Errorf("bypass hash byte %d: got 0x%02X, want 0x01", i, containerData[i])
		}
	}

	// Write to tmp/ for manual validation with D3D12.
	tmpDir := filepath.Join("..", "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Logf("warning: could not create tmp dir: %v", err)
	} else {
		outPath := filepath.Join(tmpDir, "test_minimal.dxil")
		if err := os.WriteFile(outPath, containerData, 0o644); err != nil {
			t.Logf("warning: could not write test file: %v", err)
		} else {
			t.Logf("wrote %d bytes to %s", len(containerData), outPath)
		}
	}

	t.Logf("bitcode size: %d bytes", len(bitcodeData))
	t.Logf("container size: %d bytes", len(containerData))
}

func TestContainerStructure(t *testing.T) {
	c := container.New()
	c.AddFeaturesPart(0)
	c.AddDXILPart(1, 1, 0, []byte{0x42, 0x43, 0xC0, 0xDE, 0, 0, 0, 0}) // minimal bitcode
	c.AddHashPart()
	data := c.Bytes()

	// Header checks.
	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != container.FourCCDXBC {
		t.Fatalf("wrong magic: 0x%08X", magic)
	}

	fileSize := binary.LittleEndian.Uint32(data[24:28])
	if fileSize != uint32(len(data)) {
		t.Errorf("file size mismatch: header says %d, actual %d", fileSize, len(data))
	}

	partCount := binary.LittleEndian.Uint32(data[28:32])
	if partCount != 3 {
		t.Errorf("expected 3 parts, got %d", partCount)
	}

	// Verify part offsets point to valid FourCCs.
	for i := uint32(0); i < partCount; i++ {
		off := binary.LittleEndian.Uint32(data[32+4*i:])
		if int(off) >= len(data)-4 {
			t.Errorf("part %d offset %d out of range", i, off)
			continue
		}
		fc := binary.LittleEndian.Uint32(data[off:])
		switch fc {
		case container.FourCCSFI0, container.FourCCDXIL, container.FourCCHASH:
			// OK
		default:
			t.Errorf("part %d: unexpected FourCC 0x%08X", i, fc)
		}
	}
}

func TestBypassHash(t *testing.T) {
	data := make([]byte, 28)
	binary.LittleEndian.PutUint32(data[0:4], container.FourCCDXBC)
	container.SetBypassHash(data)

	for i := 4; i < 20; i++ {
		if data[i] != 0x01 {
			t.Errorf("byte %d: got 0x%02X, want 0x01", i, data[i])
		}
	}
}

// TestCompile_PassthroughVertex tests the full compilation pipeline
// for a passthrough vertex shader.
func TestCompile_PassthroughVertex(t *testing.T) {
	vec4f32Handle := ir.TypeHandle(0)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	posBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
	resultBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
	retHandle := ir.ExpressionHandle(0)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "pos", Type: vec4f32Handle, Binding: &posBinding},
		},
		Result: &ir.FunctionResult{
			Type:    vec4f32Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &vec4f32Handle},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	irMod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageVertex, Function: fn},
	}

	data, err := Compile(irMod, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Verify DXBC container structure.
	if len(data) < 32 {
		t.Fatalf("output too small: %d bytes", len(data))
	}
	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != container.FourCCDXBC {
		t.Errorf("container magic: got 0x%08X, want DXBC", magic)
	}

	fileSize := binary.LittleEndian.Uint32(data[24:28])
	if fileSize != uint32(len(data)) {
		t.Errorf("file size: header=%d, actual=%d", fileSize, len(data))
	}

	// Verify BYPASS hash.
	for i := 4; i < 20; i++ {
		if data[i] != 0x01 {
			t.Errorf("bypass hash byte %d: got 0x%02X, want 0x01", i, data[i])
			break
		}
	}

	// Write to tmp/ for manual DXC validation.
	tmpDir := filepath.Join("..", "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Logf("warning: could not create tmp dir: %v", err)
	} else {
		outPath := filepath.Join(tmpDir, "test_passthrough_vs.dxil")
		if err := os.WriteFile(outPath, data, 0o644); err != nil {
			t.Logf("warning: could not write test file: %v", err)
		} else {
			t.Logf("wrote %d bytes to %s", len(data), outPath)
		}
	}

	t.Logf("passthrough vertex shader compiled: %d bytes", len(data))
}

// TestCompile_RetailHash tests compilation with retail (INF-0004) hash instead of BYPASS.
func TestCompile_RetailHash(t *testing.T) {
	vec4f32Handle := ir.TypeHandle(0)
	posBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
	resultBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
	retHandle := ir.ExpressionHandle(0)

	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		EntryPoints: []ir.EntryPoint{{
			Name:  "main",
			Stage: ir.StageVertex,
			Function: ir.Function{
				Name:      "main",
				Arguments: []ir.FunctionArgument{{Name: "pos", Type: vec4f32Handle, Binding: &posBinding}},
				Result:    &ir.FunctionResult{Type: vec4f32Handle, Binding: &resultBinding},
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 0}},
				},
				ExpressionTypes: []ir.TypeResolution{{Handle: &vec4f32Handle}},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
					{Kind: ir.StmtReturn{Value: &retHandle}},
				},
			},
		}},
	}

	opts := DefaultOptions()
	opts.UseBypassHash = false // Use retail hash

	data, err := Compile(mod, opts)
	if err != nil {
		t.Fatalf("Compile with retail hash failed: %v", err)
	}

	if len(data) < 20 {
		t.Fatalf("output too small: %d bytes", len(data))
	}

	// Digest must NOT be all zeros (unsigned).
	allZero := true
	for i := 4; i < 20; i++ {
		if data[i] != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("retail hash produced all-zero digest (unsigned)")
	}

	// Digest must NOT be BYPASS sentinel.
	allOne := true
	for i := 4; i < 20; i++ {
		if data[i] != 0x01 {
			allOne = false
			break
		}
	}
	if allOne {
		t.Error("retail hash produced BYPASS sentinel")
	}

	t.Logf("retail hash digest: %02x", data[4:20])

	// Write to tmp/ for testing with D3D12.
	tmpDir := filepath.Join("..", "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err == nil {
		outPath := filepath.Join(tmpDir, "test_retail_hash.dxil")
		if err := os.WriteFile(outPath, data, 0o644); err == nil {
			t.Logf("wrote %d bytes to %s", len(data), outPath)
		}
	}
}

// TestCompile_SimpleFragment tests compilation of a minimal fragment shader.
func TestCompile_SimpleFragment(t *testing.T) {
	vec4f32Handle := ir.TypeHandle(0)
	f32Handle := ir.TypeHandle(1)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}

	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})
	retHandle := ir.ExpressionHandle(4)

	fn := ir.Function{
		Name: "main",
		Result: &ir.FunctionResult{
			Type:    vec4f32Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{0, 1, 2, 3}}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle},
			{Handle: &f32Handle},
			{Handle: &f32Handle},
			{Handle: &f32Handle},
			{Handle: &vec4f32Handle},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 5}}},
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	irMod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}

	data, err := Compile(irMod, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if len(data) < 32 {
		t.Fatalf("output too small: %d bytes", len(data))
	}

	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != container.FourCCDXBC {
		t.Errorf("container magic: got 0x%08X, want DXBC", magic)
	}

	// Write to tmp/ for manual DXC validation.
	tmpDir := filepath.Join("..", "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Logf("warning: could not create tmp dir: %v", err)
	} else {
		outPath := filepath.Join(tmpDir, "test_simple_ps.dxil")
		if err := os.WriteFile(outPath, data, 0o644); err != nil {
			t.Logf("warning: could not write test file: %v", err)
		} else {
			t.Logf("wrote %d bytes to %s", len(data), outPath)
		}
	}

	t.Logf("simple fragment shader compiled: %d bytes", len(data))
}
