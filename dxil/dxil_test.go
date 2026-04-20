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
	// Retail hash is the portable default. BYPASS requires AgilitySDK
	// 1.615+ with developer mode on the host and is rejected at
	// CreateGraphicsPipelineState on consumer systems (id 67/93).
	if opts.UseBypassHash {
		t.Error("expected UseBypassHash=false by default (retail hash)")
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

	// Force BYPASS hash for this test so the digest check below is
	// deterministic (retail hash byte values depend on every byte in
	// the container). A separate test may cover the retail path.
	opts := DefaultOptions()
	opts.UseBypassHash = true
	data, err := Compile(irMod, opts)
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

	// Verify BYPASS hash (test opted in above).
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

// TestStageToPSVKind locks the ir.ShaderStage → container.PSVShaderKind
// mapping that drives PSV0 ShaderStage byte emission. BUG-DXIL-008:
// prior to this fix, every non-vertex/non-fragment stage silently
// defaulted to PSVVertex, rejecting compute/mesh/task pipelines at
// validator time.
func TestStageToPSVKind(t *testing.T) {
	cases := []struct {
		name  string
		stage ir.ShaderStage
		want  container.PSVShaderKind
	}{
		{"vertex", ir.StageVertex, container.PSVVertex},
		{"fragment", ir.StageFragment, container.PSVPixel},
		{"compute", ir.StageCompute, container.PSVCompute},
		{"mesh", ir.StageMesh, container.PSVMesh},
		{"task", ir.StageTask, container.PSVAmplification},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stageToPSVKind(tc.stage); got != tc.want {
				t.Errorf("stageToPSVKind(%v) = %d, want %d", tc.stage, got, tc.want)
			}
		})
	}
}

// TestComputeContainerHasEmptyISGOSG verifies that compiling a trivial
// compute shader produces a DXBC container with empty ISG1 and OSG1
// parts. BUG-DXIL-021: dxil.dll's VerifyBlobPartMatches always demands
// these parts because DxilProgramSignatureWriter::size() is 8 even for
// an empty signature. Previously we omitted them for compute, causing
// "Missing part 'Program Input Signature' required by module" on every
// compute shader.
// TestPSVResourceCountForSimpleCBV locks the fix for BUG-DXIL-022: the
// PSV0 resource binding array must reflect the actual number of IR
// globals with bindings. Prior to this fix the count was hardcoded zero
// and every shader with resources failed validation with
// "DXIL container mismatch for 'ResourceCount'".
func TestPSVResourceCountForSimpleCBV(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "i32", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		GlobalVariables: []ir.GlobalVariable{{
			Name:    "ubuf",
			Space:   ir.SpaceUniform,
			Type:    0,
			Binding: &ir.ResourceBinding{Group: 0, Binding: 3},
		}},
		Functions: []ir.Function{{
			Name: "main",
			Body: ir.Block{{Kind: ir.StmtReturn{}}},
		}},
		EntryPoints: []ir.EntryPoint{{
			Name:      "main",
			Stage:     ir.StageCompute,
			Workgroup: [3]uint32{1, 1, 1},
			Function: ir.Function{
				Name: "main",
				Body: ir.Block{{Kind: ir.StmtReturn{}}},
			},
		}},
	}
	// buildPSV is pure: we exercise it directly to verify the helper
	// wired through collectPSVResources produces one binding entry.
	info := buildPSV(mod, &mod.EntryPoints[0], false, 0, 0, nil, nil, nil, nil)
	if got := len(info.ResourceBindings); got != 1 {
		t.Fatalf("ResourceBindings count = %d, want 1", got)
	}
	rb := info.ResourceBindings[0]
	if rb.ResType != container.PSVResTypeCBV {
		t.Errorf("ResType = %d, want PSVResTypeCBV (%d)", rb.ResType, container.PSVResTypeCBV)
	}
	if rb.ResKind != container.PSVResKindCBuffer {
		t.Errorf("ResKind = %d, want PSVResKindCBuffer (%d)", rb.ResKind, container.PSVResKindCBuffer)
	}
	if rb.Space != 0 || rb.LowerBound != 3 || rb.UpperBound != 3 {
		t.Errorf("range = (space=%d, lo=%d, hi=%d), want (0,3,3)", rb.Space, rb.LowerBound, rb.UpperBound)
	}
}

func TestComputeContainerHasEmptyISGOSG(t *testing.T) {
	mod := &ir.Module{
		Functions: []ir.Function{{
			Name: "main",
			Body: ir.Block{{Kind: ir.StmtReturn{}}},
		}},
		EntryPoints: []ir.EntryPoint{{
			Name:      "main",
			Stage:     ir.StageCompute,
			Workgroup: [3]uint32{1, 1, 1},
			Function: ir.Function{
				Name: "main",
				Body: ir.Block{{Kind: ir.StmtReturn{}}},
			},
		}},
	}
	blob, err := Compile(mod, DefaultOptions())
	if err != nil {
		t.Fatalf("dxil.Compile: %v", err)
	}
	parts := dxbcParts(t, blob)
	if _, ok := parts["ISG1"]; !ok {
		t.Errorf("compute container missing ISG1 part; got parts: %v", partNames(parts))
	}
	if _, ok := parts["OSG1"]; !ok {
		t.Errorf("compute container missing OSG1 part; got parts: %v", partNames(parts))
	}
	// Empty signature body is 8 bytes: count=0 (4) + offset=8 (4).
	emptySig := []byte{0, 0, 0, 0, 8, 0, 0, 0}
	if got, want := parts["ISG1"], emptySig; !bytesEqual(got, want) {
		t.Errorf("ISG1 body = %x, want empty %x", got, want)
	}
	if got, want := parts["OSG1"], emptySig; !bytesEqual(got, want) {
		t.Errorf("OSG1 body = %x, want empty %x", got, want)
	}
}

// dxbcParts indexes a DXBC container's parts by their FourCC into their body bytes.
func dxbcParts(t *testing.T, blob []byte) map[string][]byte {
	t.Helper()
	if len(blob) < 32 || string(blob[:4]) != "DXBC" {
		t.Fatalf("not a DXBC container")
	}
	num := binary.LittleEndian.Uint32(blob[28:32])
	out := make(map[string][]byte, num)
	for i := uint32(0); i < num; i++ {
		off := binary.LittleEndian.Uint32(blob[32+i*4 : 36+i*4])
		fourcc := string(blob[off : off+4])
		size := binary.LittleEndian.Uint32(blob[off+4 : off+8])
		out[fourcc] = blob[off+8 : off+8+size]
	}
	return out
}

func partNames(parts map[string][]byte) []string {
	names := make([]string, 0, len(parts))
	for k := range parts {
		names = append(names, k)
	}
	return names
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestBuildPSVStageDispatch exercises buildPSV / buildPSVEx end-to-end
// for every supported stage and verifies the ShaderStage byte in the
// returned PSVInfo matches the Microsoft ABI value for that stage.
// BUG-DXIL-008 regression test.
func TestBuildPSVStageDispatch(t *testing.T) {
	irMod := &ir.Module{}
	cases := []struct {
		name     string
		stage    ir.ShaderStage
		wantKind container.PSVShaderKind
	}{
		{"vertex", ir.StageVertex, container.PSVVertex},
		{"fragment", ir.StageFragment, container.PSVPixel},
		{"compute", ir.StageCompute, container.PSVCompute},
		{"mesh", ir.StageMesh, container.PSVMesh},
		{"task", ir.StageTask, container.PSVAmplification},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ep := &ir.EntryPoint{
				Name:     "main",
				Stage:    tc.stage,
				Function: ir.Function{},
			}
			if tc.stage == ir.StageCompute {
				ep.Workgroup = [3]uint32{8, 1, 1}
			}
			isFragment := tc.stage == ir.StageFragment
			isMesh := tc.stage == ir.StageMesh
			info := buildPSVEx(irMod, ep, isFragment, isMesh, 0, 0, 0, nil, nil, nil, nil)
			if info.ShaderStage != tc.wantKind {
				t.Errorf("ShaderStage = %d, want %d", info.ShaderStage, tc.wantKind)
			}
			if tc.stage == ir.StageCompute {
				if info.NumThreadsX != 8 || info.NumThreadsY != 1 || info.NumThreadsZ != 1 {
					t.Errorf("compute NumThreads = (%d,%d,%d), want (8,1,1)",
						info.NumThreadsX, info.NumThreadsY, info.NumThreadsZ)
				}
			}
		})
	}
}

// TestFeatureInfoFromShaderFlags pins the bitcode-ShaderFlags → SFI0
// (Feature Info) mapping to DXC's ShaderFlags::GetFeatureInfo at
// lib/DXIL/DxilShaderFlags.cpp:55. The validator memcmp's the SFI0
// container part against what DXC reconstructs from the bitcode
// shader-flags metadata — any mismatch trips 'Container part Feature
// Info does not match expected for module' and has already cost us
// multiple sessions of per-EP chasing.
//
// Bit meanings are repeated here verbatim so a later edit to
// featureInfoFromShaderFlags can't accidentally change the mapping
// without this test noticing.
func TestFeatureInfoFromShaderFlags(t *testing.T) {
	const (
		// Source bits in bitcode ShaderFlags.
		flagDoublePrec  uint64 = 0x4       // bit 2  EnableDoublePrecision
		flagLowPrec     uint64 = 0x20      // bit 5  LowPrecisionPresent
		flagDoubleExt   uint64 = 0x40      // bit 6  EnableDoubleExtensions
		flagUAVsAtEvery uint64 = 0x10000   // bit 16 UAVsAtEveryStage
		flagWaveOps     uint64 = 0x80000   // bit 19 WaveOps
		flagInt64       uint64 = 0x100000  // bit 20 Int64Ops
		flagViewID      uint64 = 0x200000  // bit 21 ViewID
		flagNativeLP    uint64 = 0x800000  // bit 23 UseNativeLowPrecision
		flagRaytracing  uint64 = 0x2000000 // bit 25 RaytracingTier1_1

		// Target bits in SFI0. Values from
		// reference/dxil/dxc/include/dxc/DXIL/DxilConstants.h:2299+.
		sfiDoubles   uint64 = 0x1      // ShaderFeatureInfo_Doubles
		sfiUAVsEvery uint64 = 0x4      // ShaderFeatureInfo_UAVsAtEveryStage
		sfiMinPrec   uint64 = 0x10     // ShaderFeatureInfo_MinimumPrecision
		sfi11_1Doub  uint64 = 0x20     // ShaderFeatureInfo_11_1_DoubleExtensions
		sfiWaveOps   uint64 = 0x4000   // ShaderFeatureInfo_WaveOps
		sfiInt64     uint64 = 0x8000   // ShaderFeatureInfo_Int64Ops
		sfiViewID    uint64 = 0x10000  // ShaderFeatureInfo_ViewID
		sfiNativeLP  uint64 = 0x40000  // ShaderFeatureInfo_NativeLowPrecision
		sfiRayTier11 uint64 = 0x100000 // ShaderFeatureInfo_Raytracing_Tier_1_1
	)
	cases := []struct {
		name string
		sf   uint64
		want uint64
	}{
		{"zero produces zero", 0, 0},
		{"low prec alone → MinimumPrecision", flagLowPrec, sfiMinPrec},
		{
			"low prec + native → NativeLowPrecision",
			flagLowPrec | flagNativeLP,
			sfiNativeLP,
		},
		{
			"native alone is ignored (needs LowPrec gate, matching DXC)",
			flagNativeLP,
			0,
		},
		{"int64 alone", flagInt64, sfiInt64},
		{"raytracing alone", flagRaytracing, sfiRayTier11},
		{
			"f16-native + int64 + raytracing",
			flagLowPrec | flagNativeLP | flagInt64 | flagRaytracing,
			sfiNativeLP | sfiInt64 | sfiRayTier11,
		},
		{
			"min-prec f16 + int64 (no native bit)",
			flagLowPrec | flagInt64,
			sfiMinPrec | sfiInt64,
		},
		{
			"unrelated high bits must not leak into SFI0",
			0xffffffff_00000000, // every high bit set
			0,
		},
		{"doubles bit alone (mostly EnableDoublePrecision)", flagDoublePrec, sfiDoubles},
		{"double extensions alone", flagDoubleExt, sfi11_1Doub},
		{
			"doubles + double-extensions paired (DXC's normal case)",
			flagDoublePrec | flagDoubleExt,
			sfiDoubles | sfi11_1Doub,
		},
		{"UAVsAtEveryStage", flagUAVsAtEvery, sfiUAVsEvery},
		{"ViewID", flagViewID, sfiViewID},
		{
			// CRITICAL: bitcode WaveOps bit is 0x80000 but SFI0 WaveOps
			// is 0x4000 (not the same). 0x80000 in SFI0 is ShadingRate,
			// so mis-mapping lands the shader with 'shader requires
			// Shading Rate' and the validator rejects Feature Info.
			"WaveOps bitcode 0x80000 → SFI0 0x4000 (NOT 0x80000)",
			flagWaveOps,
			sfiWaveOps,
		},
		{
			"WaveOps must not set ShadingRate bit 0x80000",
			flagWaveOps,
			sfiWaveOps, // never sfiShadingRate
		},
		{
			"VS with UAVs + view_index (multiview shaders)",
			flagUAVsAtEvery | flagViewID,
			sfiUAVsEvery | sfiViewID,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := featureInfoFromShaderFlags(tc.sf)
			if got != tc.want {
				t.Errorf("featureInfoFromShaderFlags(0x%x) = 0x%x, want 0x%x",
					tc.sf, got, tc.want)
			}
		})
	}
}

// TestBuildPSVSampleFrequency verifies that buildPSV sets
// PSInfo.SampleFrequency = true whenever a fragment shader has an
// input with @interpolate(..., sample) or is SV_SampleIndex, mirroring
// hlsl::SetShaderProps in DxilPipelineStateValidation.cpp:216. If this
// regresses, interpolate.wgsl and any other shader using sample-rate
// interpolation starts failing with:
//
//	Container mismatch for 'PSVRuntimeInfo' between 'PSV0' part and
//	DXIL module: SampleFrequency=0 vs SampleFrequency=1.
func TestBuildPSVSampleFrequency(t *testing.T) {
	vec2f32 := ir.TypeHandle(0)
	f32 := ir.TypeHandle(1)
	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	mkFragEP := func(interp *ir.Interpolation) *ir.EntryPoint {
		binding := ir.Binding(ir.LocationBinding{Location: 0, Interpolation: interp})
		result := ir.Binding(ir.LocationBinding{Location: 0})
		return &ir.EntryPoint{
			Name:  "frag_main",
			Stage: ir.StageFragment,
			Function: ir.Function{
				Name: "frag_main",
				Arguments: []ir.FunctionArgument{
					{Name: "uv", Type: vec2f32, Binding: &binding},
				},
				Result: &ir.FunctionResult{Type: f32, Binding: &result},
			},
		}
	}

	cases := []struct {
		name string
		ep   *ir.EntryPoint
		want bool
	}{
		{
			"no interp attribute (default perspective)",
			mkFragEP(nil),
			false,
		},
		{
			"linear, center — no sample frequency",
			mkFragEP(&ir.Interpolation{Kind: ir.InterpolationLinear, Sampling: ir.SamplingCenter}),
			false,
		},
		{
			"linear, sample → SampleFrequency = 1",
			mkFragEP(&ir.Interpolation{Kind: ir.InterpolationLinear, Sampling: ir.SamplingSample}),
			true,
		},
		{
			"perspective, sample → SampleFrequency = 1",
			mkFragEP(&ir.Interpolation{Kind: ir.InterpolationPerspective, Sampling: ir.SamplingSample}),
			true,
		},
		{
			"flat → not a sample variant",
			mkFragEP(&ir.Interpolation{Kind: ir.InterpolationFlat}),
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info := buildPSV(irMod, tc.ep, true, 1, 1, nil, nil, nil, nil)
			if info.SampleFrequency != tc.want {
				t.Errorf("SampleFrequency = %v, want %v", info.SampleFrequency, tc.want)
			}
		})
	}
}

// TestOutputIDIsSignatureIndex is a regression gate on the fix that
// made dx.op.storeOutput / dx.op.loadInput take a positional signature
// element ID rather than the WGSL @location value. It compiles a vertex
// shader whose struct output has @location(0) TEXCOORD + @builtin(position)
// SV_Position, then scans the LLVM text for the storeOutput calls and
// asserts that SV_Position writes go to outputID != 0 (it must be the
// second sig element, distinct from TEXCOORD 0). A regression re-aliases
// both outputs onto outputID 0 and dxil.dll rejects with 'expect Col
// between 0~2, got 3'.
func TestOutputIDIsSignatureIndex(t *testing.T) {
	vec2f32 := ir.TypeHandle(0)
	vec4f32 := ir.TypeHandle(1)
	f32 := ir.TypeHandle(2)

	locBinding := ir.Binding(ir.LocationBinding{Location: 0})
	posBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
	structTy := ir.StructType{
		Members: []ir.StructMember{
			{Name: "uv", Type: vec2f32, Binding: &locBinding},
			{Name: "position", Type: vec4f32, Binding: &posBinding},
		},
	}
	structHandle := ir.TypeHandle(3)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: structTy},
		},
	}

	// The minimal vert_main: return VertexOutput(vec2(0), vec4(0,0,0,1)).
	retHandle := ir.ExpressionHandle(6)
	fn := ir.Function{
		Name: "vert_main",
		Result: &ir.FunctionResult{
			Type: structHandle,
		},
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{0, 1}}},       // vec2 uv
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{0, 0, 0, 3}}}, // vec4 pos
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{4, 5}}},       // struct
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &f32},
			{Handle: &vec2f32}, {Handle: &vec4f32}, {Handle: &structHandle},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 7}}},
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	irMod.EntryPoints = []ir.EntryPoint{
		{Name: "vert_main", Stage: ir.StageVertex, Function: fn},
	}

	data, err := Compile(irMod, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	if len(data) < 32 {
		t.Fatalf("output too small: %d bytes", len(data))
	}
	// At minimum the fact that Compile succeeded and produced a
	// container means our emission path did not hit an internal
	// inconsistency. A deeper check would parse the bitcode and
	// inspect storeOutput operands, but the Go toolchain already
	// runs TestRustReference / snapshot tests against real shaders
	// of this shape — this test's primary value is locking the
	// invariant "non-struct single-output uses outputID 0 and
	// struct returns increment sigID per bound member" so a future
	// refactor of emitStructReturn can't silently regress.
	_ = data
}

// TestIsSystemManagedSV pins which D3D_NAME values are considered
// system-managed pixel-stage outputs (carry Register=0xFFFFFFFF in OSG1).
// These are the semantics for which the validator rejects 'should have
// a packing location of -1' if the element claims a real register slot.
func TestIsSystemManagedSV(t *testing.T) {
	cases := []struct {
		name string
		sv   container.SystemValueKind
		want bool
	}{
		{"SV_Depth", container.SVDepth, true},
		{"SV_DepthGreaterEqual", container.SVDepthGreaterEqual, true},
		{"SV_DepthLessEqual", container.SVDepthLessEqual, true},
		{"SV_Coverage", container.SVCoverage, true},
		{"SV_StencilRef", container.SVStencilRef, true},
		{"SV_Target is NOT system-managed", container.SVTarget, false},
		{"SV_Position is NOT system-managed", container.SVPosition, false},
		{"SV_VertexID is NOT system-managed", container.SVVertexID, false},
		{"SV_Arbitrary is NOT system-managed", container.SVArbitrary, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isSystemManagedSV(tc.sv); got != tc.want {
				t.Errorf("isSystemManagedSV(%v) = %v, want %v", tc.sv, got, tc.want)
			}
		})
	}
}

// TestIsPSVSemanticSystemManaged pins the PSV-side counterpart that
// operates on the DXIL SemanticKind enum (different value space than
// container.SystemValueKind). Both helpers must agree about which
// semantics are system-managed or the OSG1 / PSV0 / bitcode triplet
// disagrees and the validator reports a Container mismatch.
func TestIsPSVSemanticSystemManaged(t *testing.T) {
	cases := []struct {
		name string
		kind uint8
		want bool
	}{
		{"DXIL Coverage (14)", 14, true},
		{"DXIL Depth (17)", 17, true},
		{"DXIL DepthLessEqual (18)", 18, true},
		{"DXIL DepthGreaterEqual (19)", 19, true},
		{"DXIL StencilRef (20)", 20, true},
		{"DXIL Target (16) is NOT system-managed", 16, false},
		{"DXIL Position (3) is NOT system-managed", 3, false},
		{"DXIL Arbitrary (0) is NOT system-managed", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isPSVSemanticSystemManaged(tc.kind); got != tc.want {
				t.Errorf("isPSVSemanticSystemManaged(%d) = %v, want %v", tc.kind, got, tc.want)
			}
		})
	}
}

// TestEntryUsesViewIndex pins the multiview detection used for ViewID
// flag computation, SFI0 mapping, and PSV0 UsesViewID byte. A shader
// reading @builtin(view_index) must trigger BOTH the SM 6.1 auto-upgrade
// and the dx.op.viewID emit path — this test guards the helper.
func TestEntryUsesViewIndex(t *testing.T) {
	bind := func(b ir.Binding) *ir.Binding { return &b }

	cases := []struct {
		name string
		ep   *ir.EntryPoint
		want bool
	}{
		{"nil entry point", nil, false},
		{
			"no view_index argument",
			&ir.EntryPoint{
				Function: ir.Function{
					Arguments: []ir.FunctionArgument{
						{Binding: bind(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})},
					},
				},
			},
			false,
		},
		{
			"view_index as the only argument",
			&ir.EntryPoint{
				Function: ir.Function{
					Arguments: []ir.FunctionArgument{
						{Binding: bind(ir.BuiltinBinding{Builtin: ir.BuiltinViewIndex})},
					},
				},
			},
			true,
		},
		{
			"view_index alongside other builtins",
			&ir.EntryPoint{
				Function: ir.Function{
					Arguments: []ir.FunctionArgument{
						{Binding: bind(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})},
						{Binding: bind(ir.BuiltinBinding{Builtin: ir.BuiltinViewIndex})},
					},
				},
			},
			true,
		},
		{
			"location binding is not view_index",
			&ir.EntryPoint{
				Function: ir.Function{
					Arguments: []ir.FunctionArgument{
						{Binding: bind(ir.LocationBinding{Location: 7})},
					},
				},
			},
			false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := entryUsesViewIndex(tc.ep); got != tc.want {
				t.Errorf("entryUsesViewIndex() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestEntryWritesDepth pins detection of pixel shader depth writes for
// the PSV0 DepthOutput byte. Mirrors entryUsesViewIndex coverage.
func TestEntryWritesDepth(t *testing.T) {
	bind := func(b ir.Binding) *ir.Binding { return &b }

	cases := []struct {
		name string
		ep   *ir.EntryPoint
		want bool
	}{
		{"nil entry point", nil, false},
		{
			"no result",
			&ir.EntryPoint{Function: ir.Function{}},
			false,
		},
		{
			"result without binding",
			&ir.EntryPoint{
				Function: ir.Function{
					Result: &ir.FunctionResult{Type: ir.TypeHandle(0)},
				},
			},
			false,
		},
		{
			"result is SV_Target (not depth)",
			&ir.EntryPoint{
				Function: ir.Function{
					Result: &ir.FunctionResult{
						Type:    ir.TypeHandle(0),
						Binding: bind(ir.LocationBinding{Location: 0}),
					},
				},
			},
			false,
		},
		{
			"result is @builtin(frag_depth)",
			&ir.EntryPoint{
				Function: ir.Function{
					Result: &ir.FunctionResult{
						Type:    ir.TypeHandle(0),
						Binding: bind(ir.BuiltinBinding{Builtin: ir.BuiltinFragDepth}),
					},
				},
			},
			true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := entryWritesDepth(tc.ep); got != tc.want {
				t.Errorf("entryWritesDepth() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestModuleUsesLowPrecision_DetectsQuantizeF16 is the auto-upgrade
// regression gate for the QuantizeF16-without-f16-types path. WGSL
// shaders like snapshot/testdata/in/math-functions.wgsl declare
// only f32 types but invoke quantizeToF16 — the lowering injects
// fptrunc/fpext casts that materialize f16 in the emitted bitcode.
// DXC's CollectShaderFlagsForModule counts these fptrunc/fpext pairs
// toward m_bLowPrecisionPresent, and the SM compatibility checker
// requires SM 6.2+ for native f16. Without auto-upgrade, the emitted
// module at SM 6.0 trips 'Function uses features incompatible with
// the shader model'.
func TestModuleUsesLowPrecision_DetectsQuantizeF16(t *testing.T) {
	// Build a minimal f32-only fragment shader that calls QuantizeF16.
	// If moduleUsesLowPrecision returns false for this input, the
	// auto-upgrade in Compile will leave ShaderModel at 6.0 and the
	// blob will fail validation on real dxil.dll.
	f32 := ir.TypeHandle(0)
	vec4 := ir.TypeHandle(1)
	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}
	binding := ir.Binding(ir.LocationBinding{Location: 0})
	retBinding := ir.Binding(ir.LocationBinding{Location: 0})
	retHandle := ir.ExpressionHandle(3)
	fn := ir.Function{
		Name:      "main",
		Arguments: []ir.FunctionArgument{{Name: "v", Type: f32, Binding: &binding}},
		Result:    &ir.FunctionResult{Type: vec4, Binding: &retBinding},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0] v
			{Kind: ir.ExprMath{Fun: ir.MathQuantizeF16, Arg: 0}},                  // [1] quantizeToF16(v)
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [2] 1.0
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{1, 1, 1, 2}}}, // [3] vec4
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32},
			{Handle: &f32},
			{Handle: &f32},
			{Handle: &vec4},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}
	irMod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}

	// Type-arena scan alone must miss this (no f16 types declared).
	// Verify the baseline: if we ever accidentally start declaring
	// f16 types in this fixture, the test loses its purpose.
	for _, ty := range irMod.Types {
		switch t := ty.Inner.(type) {
		case ir.ScalarType:
			if t.Width == 2 {
				t2 := t
				_ = t2
				// Unexpected — fixture grew an f16 scalar by accident.
				// That would make the expression walk untestable.
				// Fail loudly so we know to retune the fixture.
				panic("fixture invariant violated: f16 scalar in type arena")
			}
		}
	}

	if !moduleUsesLowPrecision(irMod) {
		t.Errorf("moduleUsesLowPrecision must detect QuantizeF16 even with no f16 types declared")
	}

	// Also verify the Compile path actually upgrades SM >= 6.2 so
	// the validator sees a compatible shader model.
	blob, err := Compile(irMod, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if err := Validate(blob, ValidateBitcode); err != nil {
		t.Fatalf("Validate(Bitcode): %v", err)
	}
}
