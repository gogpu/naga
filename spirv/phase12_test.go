package spirv

import (
	"encoding/binary"
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// Helper: compile IR module to SPIR-V with options
// =============================================================================

func compileModuleWithOptions(t *testing.T, module *ir.Module, opts Options) []byte {
	t.Helper()
	backend := NewBackend(opts)
	spvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return spvBytes
}

func compileModule(t *testing.T, module *ir.Module) []byte {
	t.Helper()
	return compileModuleWithOptions(t, module, DefaultOptions())
}

// =============================================================================
// Helper: extract OpEntryPoint instructions from SPIR-V binary
// =============================================================================

type entryPointInfo struct {
	ExecutionModel uint32
	FuncID         uint32
	Name           string
	InterfaceIDs   []uint32
}

func extractEntryPointsInfo(spvBytes []byte) []entryPointInfo {
	var result []entryPointInfo
	if len(spvBytes) < 20 {
		return result
	}

	offset := 20
	for offset+4 <= len(spvBytes) {
		word := binary.LittleEndian.Uint32(spvBytes[offset:])
		opcode := word & 0xFFFF
		wordCount := int(word >> 16)

		if wordCount == 0 || offset+wordCount*4 > len(spvBytes) {
			break
		}

		if opcode == uint32(OpEntryPoint) && wordCount >= 4 {
			ep := entryPointInfo{
				ExecutionModel: binary.LittleEndian.Uint32(spvBytes[offset+4:]),
				FuncID:         binary.LittleEndian.Uint32(spvBytes[offset+8:]),
			}

			nameStartByte := offset + 12
			nameBytes := spvBytes[nameStartByte : offset+wordCount*4]
			nameEnd := 0
			for i, b := range nameBytes {
				if b == 0 {
					nameEnd = i
					break
				}
			}
			ep.Name = string(nameBytes[:nameEnd])

			nameWords := (nameEnd + 1 + 3) / 4
			ifaceStart := 3 + nameWords
			for i := ifaceStart; i < wordCount; i++ {
				id := binary.LittleEndian.Uint32(spvBytes[offset+i*4:])
				ep.InterfaceIDs = append(ep.InterfaceIDs, id)
			}

			result = append(result, ep)
		}

		offset += wordCount * 4
	}
	return result
}

// =============================================================================
// Helper: extract OpDecorate and OpVariable instructions
// =============================================================================

type decorationEntry struct {
	TargetID   uint32
	Decoration uint32
	Operands   []uint32
}

func extractAllDecorations(spvBytes []byte) []decorationEntry {
	var result []decorationEntry
	if len(spvBytes) < 20 {
		return result
	}

	offset := 20
	for offset+4 <= len(spvBytes) {
		word := binary.LittleEndian.Uint32(spvBytes[offset:])
		opcode := word & 0xFFFF
		wordCount := int(word >> 16)

		if wordCount == 0 || offset+wordCount*4 > len(spvBytes) {
			break
		}

		if opcode == uint32(OpDecorate) && wordCount >= 3 {
			dec := decorationEntry{
				TargetID:   binary.LittleEndian.Uint32(spvBytes[offset+4:]),
				Decoration: binary.LittleEndian.Uint32(spvBytes[offset+8:]),
			}
			for i := 3; i < wordCount; i++ {
				dec.Operands = append(dec.Operands, binary.LittleEndian.Uint32(spvBytes[offset+i*4:]))
			}
			result = append(result, dec)
		}

		offset += wordCount * 4
	}
	return result
}

type variableEntry struct {
	ResultTypeID uint32
	ResultID     uint32
	StorageClass uint32
}

func extractAllVariables(spvBytes []byte) []variableEntry {
	var result []variableEntry
	if len(spvBytes) < 20 {
		return result
	}

	offset := 20
	for offset+4 <= len(spvBytes) {
		word := binary.LittleEndian.Uint32(spvBytes[offset:])
		opcode := word & 0xFFFF
		wordCount := int(word >> 16)

		if wordCount == 0 || offset+wordCount*4 > len(spvBytes) {
			break
		}

		if opcode == uint32(OpVariable) && wordCount >= 4 {
			v := variableEntry{
				ResultTypeID: binary.LittleEndian.Uint32(spvBytes[offset+4:]),
				ResultID:     binary.LittleEndian.Uint32(spvBytes[offset+8:]),
				StorageClass: binary.LittleEndian.Uint32(spvBytes[offset+12:]),
			}
			result = append(result, v)
		}

		offset += wordCount * 4
	}
	return result
}

// hasOpcode scans SPIR-V binary for presence of a given opcode.
func hasOpcode(spvBytes []byte, op OpCode) bool {
	if len(spvBytes) < 20 {
		return false
	}
	offset := 20
	for offset+4 <= len(spvBytes) {
		word := binary.LittleEndian.Uint32(spvBytes[offset:])
		opcode := word & 0xFFFF
		wordCount := int(word >> 16)
		if wordCount == 0 || offset+wordCount*4 > len(spvBytes) {
			break
		}
		if opcode == uint32(op) {
			return true
		}
		offset += wordCount * 4
	}
	return false
}

// findBuiltInDecorations returns target ID -> BuiltIn value for all BuiltIn decorations.
func findBuiltInDecorations(spvBytes []byte) map[uint32]uint32 {
	result := make(map[uint32]uint32)
	for _, dec := range extractAllDecorations(spvBytes) {
		if Decoration(dec.Decoration) == DecorationBuiltIn && len(dec.Operands) > 0 {
			result[dec.TargetID] = dec.Operands[0]
		}
	}
	return result
}

// ptrUint32 returns a pointer to a uint32 value.
func ptrUint32(v uint32) *uint32 {
	return &v
}

// makeLocBinding creates a LocationBinding and returns it as *ir.Binding (pointer to interface).
func makeLocBinding(loc uint32) *ir.Binding {
	var b ir.Binding = ir.LocationBinding{Location: loc}
	return &b
}

// makeBuiltinBinding creates a BuiltinBinding and returns it as *ir.Binding.
func makeBuiltinBinding(builtin ir.BuiltinValue) *ir.Binding {
	var b ir.Binding = ir.BuiltinBinding{Builtin: builtin}
	return &b
}

// =============================================================================
// SECTION 1: Capability detection tests
// =============================================================================

// TestCapability_BuiltinClipDistance verifies that using BuiltinClipDistance
// in a struct member triggers CapabilityClipDistance.
func TestCapability_BuiltinClipDistance(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			// 0: f32
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			// 1: array<f32, 1>
			{Name: "array_f32_1", Inner: ir.ArrayType{
				Base:   0,
				Size:   ir.ArraySize{Constant: ptrUint32(1)},
				Stride: 4,
			}},
			// 2: vec4<f32>
			{Name: "vec4f", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			// 3: output struct
			{Name: "VertexOutput", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "position", Type: 2, Offset: 0, Binding: makeBuiltinBinding(ir.BuiltinPosition)},
					{Name: "clip_distance", Type: 1, Offset: 16, Binding: makeBuiltinBinding(ir.BuiltinClipDistance)},
				},
				Span: 20,
			}},
		},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageVertex,
				Function: ir.Function{
					Name: "main",
					Result: &ir.FunctionResult{
						Type: 3,
					},
					Expressions: []ir.Expression{
						{Kind: ir.ExprZeroValue{Type: 3}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
						{Kind: ir.StmtReturn{Value: ptrExprHandle(0)}},
					},
				},
			},
		},
	}

	spvBytes := compileModule(t, module)
	caps := extractCapabilities(spvBytes)
	assertCapability(t, caps, CapabilityClipDistance)
}

// TestCapability_BuiltinPrimitiveIndex verifies CapabilityGeometry is triggered.
func TestCapability_BuiltinPrimitiveIndex(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			// 0: u32
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			// 1: vec4<f32>
			{Name: "vec4f", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageFragment,
				Function: ir.Function{
					Name: "main",
					Arguments: []ir.FunctionArgument{
						{Name: "prim_idx", Type: 0, Binding: makeBuiltinBinding(ir.BuiltinPrimitiveIndex)},
					},
					Result: &ir.FunctionResult{
						Type:    1,
						Binding: makeLocBinding(0),
					},
					Expressions: []ir.Expression{
						{Kind: ir.ExprFunctionArgument{Index: 0}},
						{Kind: ir.ExprZeroValue{Type: 1}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
						{Kind: ir.StmtReturn{Value: ptrExprHandle(1)}},
					},
				},
			},
		},
	}

	spvBytes := compileModule(t, module)
	caps := extractCapabilities(spvBytes)
	assertCapability(t, caps, CapabilityGeometry)
}

// TestCapability_SubgroupBuiltins verifies CapabilityGroupNonUniform is triggered
// by subgroup-related builtins.
func TestCapability_SubgroupBuiltins(t *testing.T) {
	tests := []struct {
		name    string
		builtin ir.BuiltinValue
	}{
		{"NumSubgroups", ir.BuiltinNumSubgroups},
		{"SubgroupID", ir.BuiltinSubgroupID},
		{"SubgroupSize", ir.BuiltinSubgroupSize},
		{"SubgroupInvocationID", ir.BuiltinSubgroupInvocationID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &ir.Module{
				Types: []ir.Type{
					{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
				},
				Constants:       []ir.Constant{},
				GlobalVariables: []ir.GlobalVariable{},
				Functions:       []ir.Function{},
				EntryPoints: []ir.EntryPoint{
					{
						Name:      "main",
						Stage:     ir.StageCompute,
						Workgroup: [3]uint32{1, 1, 1},
						Function: ir.Function{
							Name: "main",
							Arguments: []ir.FunctionArgument{
								{Name: "sg", Type: 0, Binding: makeBuiltinBinding(tt.builtin)},
							},
							Expressions: []ir.Expression{
								{Kind: ir.ExprFunctionArgument{Index: 0}},
							},
							Body: []ir.Statement{
								{Kind: ir.StmtReturn{}},
							},
						},
					},
				},
			}

			spvBytes := compileModule(t, module)
			caps := extractCapabilities(spvBytes)
			assertCapability(t, caps, CapabilityGroupNonUniform)
		})
	}
}

// TestCapability_Float16_StorageBuffer16Bit verifies that f16 scalar types
// trigger StorageBuffer16BitAccess and UniformAndStorageBuffer16BitAccess.
func TestCapability_Float16_StorageBuffer16Bit(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f16", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}},
		},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints:     []ir.EntryPoint{},
	}

	spvBytes := compileModule(t, module)
	caps := extractCapabilities(spvBytes)

	assertCapability(t, caps, CapabilityFloat16)
	assertCapability(t, caps, CapabilityStorageBuffer16BitAccess)
	assertCapability(t, caps, CapabilityUniformAndStorageBuffer16BitAccess)
}

// TestCapability_Float16_StorageInputOutput16 verifies that when
// UseStorageInputOutput16 is true, StorageInputOutput16 is also emitted for f16.
func TestCapability_Float16_StorageInputOutput16(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f16", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}},
		},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints:     []ir.EntryPoint{},
	}

	t.Run("enabled", func(t *testing.T) {
		opts := DefaultOptions()
		opts.UseStorageInputOutput16 = true
		spvBytes := compileModuleWithOptions(t, module, opts)
		caps := extractCapabilities(spvBytes)
		assertCapability(t, caps, CapabilityStorageInputOutput16)
	})

	t.Run("disabled", func(t *testing.T) {
		opts := DefaultOptions()
		opts.UseStorageInputOutput16 = false
		spvBytes := compileModuleWithOptions(t, module, opts)
		caps := extractCapabilities(spvBytes)
		assertNoCapability(t, caps, CapabilityStorageInputOutput16)
	})
}

// TestCapability_AtomicFloat32 verifies that AtomicType{Float, 4} triggers
// CapabilityAtomicFloat32AddEXT and the SPV_EXT_shader_atomic_float_add extension.
func TestCapability_AtomicFloat32(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "atomic_f32", Inner: ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints:     []ir.EntryPoint{},
	}

	spvBytes := compileModule(t, module)
	caps := extractCapabilities(spvBytes)
	assertCapability(t, caps, CapabilityAtomicFloat32AddEXT)

	exts := extractExtensions(spvBytes)
	found := false
	for _, ext := range exts {
		if ext == "SPV_EXT_shader_atomic_float_add" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected extension SPV_EXT_shader_atomic_float_add, got: %v", exts)
	}
}

// TestCapability_StorageImageExtendedFormats verifies that non-basic storage
// image formats trigger StorageImageExtendedFormats capability.
func TestCapability_StorageImageExtendedFormats(t *testing.T) {
	tests := []struct {
		name     string
		format   ir.StorageFormat
		extended bool
	}{
		{"Rgba32Float_basic", ir.StorageFormatRgba32Float, false},
		{"Rgba16Float_basic", ir.StorageFormatRgba16Float, false},
		{"R32Float_basic", ir.StorageFormatR32Float, false},
		{"Rgba8Unorm_basic", ir.StorageFormatRgba8Unorm, false},
		{"Rg32Float_extended", ir.StorageFormatRg32Float, true},
		{"R16Float_extended", ir.StorageFormatR16Float, true},
		{"Rg8Unorm_extended", ir.StorageFormatRg8Unorm, true},
		{"R8Unorm_extended", ir.StorageFormatR8Unorm, true},
		{"Bgra8Unorm_extended", ir.StorageFormatBgra8Unorm, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &ir.Module{
				Types: []ir.Type{
					{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
					{Name: "img", Inner: ir.ImageType{
						Dim:           ir.Dim2D,
						Arrayed:       false,
						Class:         ir.ImageClassStorage,
						StorageFormat: tt.format,
					}},
				},
				Constants:       []ir.Constant{},
				GlobalVariables: []ir.GlobalVariable{},
				Functions:       []ir.Function{},
				EntryPoints:     []ir.EntryPoint{},
			}

			spvBytes := compileModule(t, module)
			caps := extractCapabilities(spvBytes)

			if tt.extended {
				assertCapability(t, caps, CapabilityStorageImageExtendedFormats)
			} else {
				assertNoCapability(t, caps, CapabilityStorageImageExtendedFormats)
			}
		})
	}
}

// TestCapability_Linkage verifies that modules with no entry points
// get CapabilityLinkage.
func TestCapability_Linkage(t *testing.T) {
	t.Run("no_entry_points", func(t *testing.T) {
		module := &ir.Module{
			Types:           []ir.Type{},
			Constants:       []ir.Constant{},
			GlobalVariables: []ir.GlobalVariable{},
			Functions:       []ir.Function{},
			EntryPoints:     []ir.EntryPoint{},
		}
		spvBytes := compileModule(t, module)
		caps := extractCapabilities(spvBytes)
		assertCapability(t, caps, CapabilityLinkage)
	})

	t.Run("with_entry_point", func(t *testing.T) {
		module := &ir.Module{
			Types:           []ir.Type{},
			Constants:       []ir.Constant{},
			GlobalVariables: []ir.GlobalVariable{},
			Functions:       []ir.Function{},
			EntryPoints: []ir.EntryPoint{
				{
					Name:      "main",
					Stage:     ir.StageCompute,
					Workgroup: [3]uint32{1, 1, 1},
					Function: ir.Function{
						Name: "main",
						Body: []ir.Statement{
							{Kind: ir.StmtReturn{}},
						},
					},
				},
			},
		}
		spvBytes := compileModule(t, module)
		caps := extractCapabilities(spvBytes)
		assertNoCapability(t, caps, CapabilityLinkage)
	})
}

// =============================================================================
// SECTION 2: Entry point interface variable tests
// =============================================================================

// TestEntryPointInterface_SPIRV14_IncludesGlobals verifies that with SPIR-V 1.4+,
// all used global variables (Uniform, StorageBuffer, etc.) appear in OpEntryPoint.
func TestEntryPointInterface_SPIRV14_IncludesGlobals(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			// 0: f32
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			// 1: struct { value: f32 }
			{Name: "Uniforms", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "value", Type: 0, Offset: 0},
				},
				Span: 4,
			}},
		},
		Constants: []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:    "uniforms",
				Space:   ir.SpaceUniform,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
				Type:    1,
			},
		},
		Functions: []ir.Function{},
		EntryPoints: []ir.EntryPoint{
			{
				Name:      "main",
				Stage:     ir.StageCompute,
				Workgroup: [3]uint32{1, 1, 1},
				Function: ir.Function{
					Name: "main",
					Expressions: []ir.Expression{
						{Kind: ir.ExprGlobalVariable{Variable: 0}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtReturn{}},
					},
				},
			},
		},
	}

	opts := DefaultOptions()
	opts.Version = Version1_4
	spvBytes := compileModuleWithOptions(t, module, opts)

	eps := extractEntryPointsInfo(spvBytes)
	if len(eps) != 1 {
		t.Fatalf("expected 1 entry point, got %d", len(eps))
	}

	if len(eps[0].InterfaceIDs) == 0 {
		t.Errorf("SPIR-V 1.4: expected interface variables in OpEntryPoint, got none")
	}
}

// TestEntryPointInterface_SPIRV11_NoExtraGlobals verifies that with SPIR-V 1.1,
// only Input/Output variables appear in OpEntryPoint (not Uniform/StorageBuffer).
func TestEntryPointInterface_SPIRV11_NoExtraGlobals(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "Uniforms", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "value", Type: 0, Offset: 0},
				},
				Span: 4,
			}},
		},
		Constants: []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:    "uniforms",
				Space:   ir.SpaceUniform,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
				Type:    1,
			},
		},
		Functions: []ir.Function{},
		EntryPoints: []ir.EntryPoint{
			{
				Name:      "main",
				Stage:     ir.StageCompute,
				Workgroup: [3]uint32{1, 1, 1},
				Function: ir.Function{
					Name: "main",
					Expressions: []ir.Expression{
						{Kind: ir.ExprGlobalVariable{Variable: 0}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtReturn{}},
					},
				},
			},
		},
	}

	opts := DefaultOptions()
	opts.Version = Version1_1
	spvBytes := compileModuleWithOptions(t, module, opts)

	eps := extractEntryPointsInfo(spvBytes)
	if len(eps) != 1 {
		t.Fatalf("expected 1 entry point, got %d", len(eps))
	}

	if len(eps[0].InterfaceIDs) != 0 {
		t.Errorf("SPIR-V 1.1: expected no interface variables for compute shader with only uniform globals, got %d", len(eps[0].InterfaceIDs))
	}
}

// =============================================================================
// SECTION 3: ForcePointSize tests
// =============================================================================

// TestForcePointSize_VertexShader verifies that ForcePointSize=true adds
// a BuiltIn PointSize Output variable to vertex shaders.
func TestForcePointSize_VertexShader(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "vec4f", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageVertex,
				Function: ir.Function{
					Name: "main",
					Result: &ir.FunctionResult{
						Type:    0,
						Binding: makeBuiltinBinding(ir.BuiltinPosition),
					},
					Expressions: []ir.Expression{
						{Kind: ir.ExprZeroValue{Type: 0}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
						{Kind: ir.StmtReturn{Value: ptrExprHandle(0)}},
					},
				},
			},
		},
	}

	opts := DefaultOptions()
	opts.ForcePointSize = true
	spvBytes := compileModuleWithOptions(t, module, opts)

	builtinDecs := findBuiltInDecorations(spvBytes)

	foundPointSize := false
	var pointSizeVarID uint32
	for varID, builtinVal := range builtinDecs {
		if BuiltIn(builtinVal) == BuiltInPointSize {
			foundPointSize = true
			pointSizeVarID = varID
			break
		}
	}
	if !foundPointSize {
		t.Fatalf("ForcePointSize=true: expected BuiltIn PointSize decoration")
	}

	// Verify the PointSize variable is Output storage class
	vars := extractAllVariables(spvBytes)
	foundOutputVar := false
	for _, v := range vars {
		if v.ResultID == pointSizeVarID {
			if StorageClass(v.StorageClass) != StorageClassOutput {
				t.Errorf("PointSize variable storage class: got %d, want %d (Output)", v.StorageClass, StorageClassOutput)
			}
			foundOutputVar = true
			break
		}
	}
	if !foundOutputVar {
		t.Errorf("PointSize variable ID %d not found in OpVariable instructions", pointSizeVarID)
	}

	// Verify PointSize variable appears in OpEntryPoint interface
	eps := extractEntryPointsInfo(spvBytes)
	if len(eps) != 1 {
		t.Fatalf("expected 1 entry point, got %d", len(eps))
	}
	foundInInterface := false
	for _, id := range eps[0].InterfaceIDs {
		if id == pointSizeVarID {
			foundInInterface = true
			break
		}
	}
	if !foundInInterface {
		t.Errorf("PointSize variable not found in OpEntryPoint interface list")
	}
}

// TestForcePointSize_FragmentShader verifies that fragment shaders do NOT
// get a PointSize variable even when ForcePointSize=true.
func TestForcePointSize_FragmentShader(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "vec4f", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageFragment,
				Function: ir.Function{
					Name: "main",
					Result: &ir.FunctionResult{
						Type:    0,
						Binding: makeLocBinding(0),
					},
					Expressions: []ir.Expression{
						{Kind: ir.ExprZeroValue{Type: 0}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
						{Kind: ir.StmtReturn{Value: ptrExprHandle(0)}},
					},
				},
			},
		},
	}

	opts := DefaultOptions()
	opts.ForcePointSize = true
	spvBytes := compileModuleWithOptions(t, module, opts)

	builtinDecs := findBuiltInDecorations(spvBytes)
	for _, builtinVal := range builtinDecs {
		if BuiltIn(builtinVal) == BuiltInPointSize {
			t.Errorf("ForcePointSize=true: fragment shader should NOT have PointSize")
		}
	}
}

// TestForcePointSize_Disabled verifies that ForcePointSize=false does NOT add
// PointSize to vertex shaders.
func TestForcePointSize_Disabled(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "vec4f", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageVertex,
				Function: ir.Function{
					Name: "main",
					Result: &ir.FunctionResult{
						Type:    0,
						Binding: makeBuiltinBinding(ir.BuiltinPosition),
					},
					Expressions: []ir.Expression{
						{Kind: ir.ExprZeroValue{Type: 0}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
						{Kind: ir.StmtReturn{Value: ptrExprHandle(0)}},
					},
				},
			},
		},
	}

	opts := DefaultOptions()
	opts.ForcePointSize = false
	spvBytes := compileModuleWithOptions(t, module, opts)

	builtinDecs := findBuiltInDecorations(spvBytes)
	for _, builtinVal := range builtinDecs {
		if BuiltIn(builtinVal) == BuiltInPointSize {
			t.Errorf("ForcePointSize=false: should NOT have PointSize")
		}
	}
}

// TestForcePointSize_NoDuplicate_StructMember verifies that if a vertex shader
// has PointSize in a struct member, ForcePointSize does NOT add a duplicate.
// The backend decomposes struct outputs into individual variables, and the
// existing PointSize member should prevent ForcePointSize from adding another.
func TestForcePointSize_NoDuplicate_StructMember(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			// 0: f32
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			// 1: vec4<f32>
			{Name: "vec4f", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			// 2: output struct with position + point_size
			{Name: "VertexOutput", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "position", Type: 1, Offset: 0, Binding: makeBuiltinBinding(ir.BuiltinPosition)},
					{Name: "point_size", Type: 0, Offset: 16, Binding: makeBuiltinBinding(ir.BuiltinPointSize)},
				},
				Span: 20,
			}},
		},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageVertex,
				Function: ir.Function{
					Name: "main",
					Result: &ir.FunctionResult{
						Type: 2,
					},
					Expressions: []ir.Expression{
						{Kind: ir.ExprZeroValue{Type: 2}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
						{Kind: ir.StmtReturn{Value: ptrExprHandle(0)}},
					},
				},
			},
		},
	}

	opts := DefaultOptions()
	opts.ForcePointSize = true
	spvBytes := compileModuleWithOptions(t, module, opts)

	// Count PointSize BuiltIn decorations (both OpDecorate and the fact
	// that no EXTRA force_point_size variable was created).
	// The struct member PointSize is detected by hasPointSize check,
	// so the backend should NOT create an additional Output variable.
	builtinDecs := findBuiltInDecorations(spvBytes)
	forcePointSizeCount := 0
	for _, builtinVal := range builtinDecs {
		if BuiltIn(builtinVal) == BuiltInPointSize {
			forcePointSizeCount++
		}
	}
	// At most 1 (from struct decomposition). The key assertion is that
	// ForcePointSize did NOT create an additional one beyond what the
	// struct decomposition would create.
	// With struct decomposition, there should be exactly 1 PointSize decoration.
	// If ForcePointSize incorrectly added another, there would be 2.
	if forcePointSizeCount > 1 {
		t.Errorf("expected at most 1 PointSize decoration (struct has it), got %d -- ForcePointSize created a duplicate", forcePointSizeCount)
	}
}

// =============================================================================
// SECTION 4: Workgroup initialization polyfill tests
// =============================================================================

// TestWorkgroupInit_ComputeWithWorkgroupVar verifies that compute shaders
// with workgroup variables get zero-initialization code.
func TestWorkgroupInit_ComputeWithWorkgroupVar(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		},
		Constants: []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "shared_val", Space: ir.SpaceWorkGroup, Type: 0},
		},
		Functions: []ir.Function{},
		EntryPoints: []ir.EntryPoint{
			{
				Name:      "main",
				Stage:     ir.StageCompute,
				Workgroup: [3]uint32{64, 1, 1},
				Function: ir.Function{
					Name: "main",
					Expressions: []ir.Expression{
						{Kind: ir.ExprGlobalVariable{Variable: 0}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtReturn{}},
					},
				},
			},
		},
	}

	spvBytes := compileModule(t, module)

	if !hasOpcode(spvBytes, OpControlBarrier) {
		t.Errorf("expected OpControlBarrier in compute shader with workgroup variable")
	}

	if !hasOpcode(spvBytes, OpStore) {
		t.Errorf("expected OpStore for zero-initialization of workgroup variable")
	}

	builtinDecs := findBuiltInDecorations(spvBytes)
	foundLocalInvocID := false
	var localInvocVarID uint32
	for varID, builtinVal := range builtinDecs {
		if BuiltIn(builtinVal) == BuiltInLocalInvocationID {
			foundLocalInvocID = true
			localInvocVarID = varID
			break
		}
	}
	if !foundLocalInvocID {
		t.Fatalf("expected BuiltIn LocalInvocationId variable for workgroup init")
	}

	vars := extractAllVariables(spvBytes)
	for _, v := range vars {
		if v.ResultID == localInvocVarID {
			if StorageClass(v.StorageClass) != StorageClassInput {
				t.Errorf("LocalInvocationId storage class: got %d, want %d (Input)", v.StorageClass, StorageClassInput)
			}
			break
		}
	}

	eps := extractEntryPointsInfo(spvBytes)
	if len(eps) != 1 {
		t.Fatalf("expected 1 entry point, got %d", len(eps))
	}
	foundInInterface := false
	for _, id := range eps[0].InterfaceIDs {
		if id == localInvocVarID {
			foundInInterface = true
			break
		}
	}
	if !foundInInterface {
		t.Errorf("LocalInvocationId variable not found in OpEntryPoint interface")
	}
}

// TestWorkgroupInit_ComputeWithoutWorkgroupVar verifies that compute shaders
// WITHOUT workgroup variables do NOT get the polyfill.
func TestWorkgroupInit_ComputeWithoutWorkgroupVar(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints: []ir.EntryPoint{
			{
				Name:      "main",
				Stage:     ir.StageCompute,
				Workgroup: [3]uint32{1, 1, 1},
				Function: ir.Function{
					Name: "main",
					Body: []ir.Statement{
						{Kind: ir.StmtReturn{}},
					},
				},
			},
		},
	}

	spvBytes := compileModule(t, module)

	if hasOpcode(spvBytes, OpControlBarrier) {
		t.Errorf("compute shader without workgroup variables should not have OpControlBarrier")
	}
}

// TestWorkgroupInit_VertexShader verifies that non-compute shaders do NOT get
// the workgroup init polyfill.
func TestWorkgroupInit_VertexShader(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "vec4f", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageVertex,
				Function: ir.Function{
					Name: "main",
					Result: &ir.FunctionResult{
						Type:    0,
						Binding: makeBuiltinBinding(ir.BuiltinPosition),
					},
					Expressions: []ir.Expression{
						{Kind: ir.ExprZeroValue{Type: 0}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
						{Kind: ir.StmtReturn{Value: ptrExprHandle(0)}},
					},
				},
			},
		},
	}

	spvBytes := compileModule(t, module)

	if hasOpcode(spvBytes, OpControlBarrier) {
		t.Errorf("vertex shader should not have OpControlBarrier")
	}
}

// =============================================================================
// SECTION 5: collectUsedGlobalVars tests
// =============================================================================

// TestCollectUsedGlobalVars_Direct verifies that direct ExprGlobalVariable
// references in an entry point function are collected.
func TestCollectUsedGlobalVars_Direct(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Name: "Buf", Inner: ir.StructType{
				Members: []ir.StructMember{{Name: "x", Type: 0, Offset: 0}},
				Span:    4,
			}},
		},
		Constants: []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "buf0", Space: ir.SpaceStorage, Binding: &ir.ResourceBinding{Group: 0, Binding: 0}, Type: 1},
			{Name: "buf1", Space: ir.SpaceStorage, Binding: &ir.ResourceBinding{Group: 0, Binding: 1}, Type: 1},
			{Name: "buf2", Space: ir.SpaceStorage, Binding: &ir.ResourceBinding{Group: 0, Binding: 2}, Type: 1},
		},
		Functions: []ir.Function{},
		EntryPoints: []ir.EntryPoint{
			{
				Name:      "main",
				Stage:     ir.StageCompute,
				Workgroup: [3]uint32{1, 1, 1},
				Function: ir.Function{
					Name: "main",
					Expressions: []ir.Expression{
						{Kind: ir.ExprGlobalVariable{Variable: 0}}, // buf0
						{Kind: ir.ExprGlobalVariable{Variable: 2}}, // buf2 (skip buf1)
					},
					Body: []ir.Statement{
						{Kind: ir.StmtReturn{}},
					},
				},
			},
		},
	}

	opts := DefaultOptions()
	opts.Version = Version1_4
	spvBytes := compileModuleWithOptions(t, module, opts)

	eps := extractEntryPointsInfo(spvBytes)
	if len(eps) != 1 {
		t.Fatalf("expected 1 entry point, got %d", len(eps))
	}

	if len(eps[0].InterfaceIDs) < 2 {
		t.Errorf("expected at least 2 interface variables (buf0, buf2), got %d", len(eps[0].InterfaceIDs))
	}
}

// TestCollectUsedGlobalVars_Transitive verifies that global variables referenced
// in called functions are transitively collected for the entry point.
func TestCollectUsedGlobalVars_Transitive(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Name: "Buf", Inner: ir.StructType{
				Members: []ir.StructMember{{Name: "x", Type: 0, Offset: 0}},
				Span:    4,
			}},
		},
		Constants: []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "direct_buf", Space: ir.SpaceStorage, Binding: &ir.ResourceBinding{Group: 0, Binding: 0}, Type: 1},
			{Name: "indirect_buf", Space: ir.SpaceStorage, Binding: &ir.ResourceBinding{Group: 0, Binding: 1}, Type: 1},
		},
		Functions: []ir.Function{
			// Function 0: helper() -- references indirect_buf
			{
				Name: "helper",
				Expressions: []ir.Expression{
					{Kind: ir.ExprGlobalVariable{Variable: 1}},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{}},
				},
			},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:      "main",
				Stage:     ir.StageCompute,
				Workgroup: [3]uint32{1, 1, 1},
				Function: ir.Function{
					Name: "main",
					Expressions: []ir.Expression{
						{Kind: ir.ExprGlobalVariable{Variable: 0}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtCall{Function: 0, Arguments: []ir.ExpressionHandle{}}},
						{Kind: ir.StmtReturn{}},
					},
				},
			},
		},
	}

	opts := DefaultOptions()
	opts.Version = Version1_4
	spvBytes := compileModuleWithOptions(t, module, opts)

	eps := extractEntryPointsInfo(spvBytes)
	if len(eps) != 1 {
		t.Fatalf("expected 1 entry point, got %d", len(eps))
	}

	if len(eps[0].InterfaceIDs) < 2 {
		t.Errorf("expected at least 2 interface variables (direct + transitive), got %d", len(eps[0].InterfaceIDs))
	}
}

// TestCollectUsedGlobalVars_NoDuplicates verifies that the same global variable
// referenced multiple times does not create duplicate entries.
func TestCollectUsedGlobalVars_NoDuplicates(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Name: "Buf", Inner: ir.StructType{
				Members: []ir.StructMember{{Name: "x", Type: 0, Offset: 0}},
				Span:    4,
			}},
		},
		Constants: []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "buf", Space: ir.SpaceStorage, Binding: &ir.ResourceBinding{Group: 0, Binding: 0}, Type: 1},
		},
		Functions: []ir.Function{},
		EntryPoints: []ir.EntryPoint{
			{
				Name:      "main",
				Stage:     ir.StageCompute,
				Workgroup: [3]uint32{1, 1, 1},
				Function: ir.Function{
					Name: "main",
					Expressions: []ir.Expression{
						{Kind: ir.ExprGlobalVariable{Variable: 0}},
						{Kind: ir.ExprGlobalVariable{Variable: 0}}, // duplicate
					},
					Body: []ir.Statement{
						{Kind: ir.StmtReturn{}},
					},
				},
			},
		},
	}

	opts := DefaultOptions()
	opts.Version = Version1_4
	spvBytes := compileModuleWithOptions(t, module, opts)

	eps := extractEntryPointsInfo(spvBytes)
	if len(eps) != 1 {
		t.Fatalf("expected 1 entry point, got %d", len(eps))
	}

	idCounts := make(map[uint32]int)
	for _, id := range eps[0].InterfaceIDs {
		idCounts[id]++
	}

	for id, count := range idCounts {
		if count > 1 {
			t.Errorf("interface variable ID %d appears %d times (should be unique)", id, count)
		}
	}
}

// TestNonUniformBindingArrayDecoration verifies that accessing a binding array
// with a non-uniform index (fragment shader input) produces:
// - ShaderNonUniform capability
// - SPV_EXT_descriptor_indexing extension
// - NonUniform decorations on AccessChain and Load results
//
// Matches Rust naga behavior: decorate_non_uniform_binding_array_access.
func TestNonUniformBindingArrayDecoration(t *testing.T) {
	source := `
@group(0) @binding(0) var textures: binding_array<texture_2d<f32>, 4>;

struct FragIn {
    @location(0) idx: u32,
};

@fragment
fn main(fin: FragIn) -> @location(0) vec4<f32> {
    let non_uniform = fin.idx;
    let dims = textureDimensions(textures[non_uniform]);
    return vec4<f32>(f32(dims.x), f32(dims.y), 0.0, 1.0);
}
`
	spvBytes := compileWGSLForCapabilityTest(t, source)

	// 1. Check ShaderNonUniform capability is present
	caps := extractCapabilities(spvBytes)
	assertCapability(t, caps, CapabilityShaderNonUniform)

	// 2. Check SPV_EXT_descriptor_indexing extension is present
	exts := extractExtensions(spvBytes)
	found := false
	for _, ext := range exts {
		if ext == "SPV_EXT_descriptor_indexing" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected extension SPV_EXT_descriptor_indexing, got %v", exts)
	}

	// 3. Check NonUniform decorations exist
	decorations := extractAllDecorations(spvBytes)
	nonUniformCount := 0
	for _, d := range decorations {
		if d.Decoration == uint32(DecorationNonUniform) {
			nonUniformCount++
		}
	}
	// At minimum, we expect 2 NonUniform decorations for the binding array access:
	// one for OpAccessChain result, one for OpLoad result
	if nonUniformCount < 2 {
		t.Errorf("expected at least 2 NonUniform decorations, got %d", nonUniformCount)
	}
}

// TestNoNonUniformForUniformIndex verifies that accessing a binding array
// with a uniform index (from a uniform buffer) does NOT produce NonUniform
// decorations. Only non-uniform indices (fragment inputs) should trigger them.
func TestNoNonUniformForUniformIndex(t *testing.T) {
	source := `
struct Uniforms {
    idx: u32,
};

@group(0) @binding(0) var textures: binding_array<texture_2d<f32>, 4>;
@group(0) @binding(1) var<uniform> uni: Uniforms;

@fragment
fn main() -> @location(0) vec4<f32> {
    let uniform_index = uni.idx;
    let dims = textureDimensions(textures[uniform_index]);
    return vec4<f32>(f32(dims.x), f32(dims.y), 0.0, 1.0);
}
`
	spvBytes := compileWGSLForCapabilityTest(t, source)

	// Uniform buffer index should NOT trigger NonUniform decorations
	caps := extractCapabilities(spvBytes)
	if caps[uint32(CapabilityShaderNonUniform)] {
		t.Errorf("ShaderNonUniform capability should NOT be present for uniform index access")
	}

	decorations := extractAllDecorations(spvBytes)
	for _, d := range decorations {
		if d.Decoration == uint32(DecorationNonUniform) {
			t.Errorf("NonUniform decoration should NOT be present for uniform index access")
			break
		}
	}
}
