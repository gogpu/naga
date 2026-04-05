package dxil

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogpu/naga/dxil/internal/container"
	"github.com/gogpu/naga/dxil/internal/module"
)

func TestCompile_NotImplemented(t *testing.T) {
	_, err := Compile(nil, DefaultOptions())
	if err == nil {
		t.Fatal("expected error from unimplemented Compile")
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
