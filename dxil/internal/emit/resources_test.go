// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package emit

import (
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/ir"
)

// TestCBVStructTypeMat4x4 verifies that a bare mat4x4<f32> uniform produces
// the correct DXC-matching type: %hostlayout.mvp_matrix = type { [4 x <4 x float>] }
func TestCBVStructTypeMat4x4(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			// 0: Matrix { columns: Vec4, rows: Vec4, scalar: Float(4) }
			{Name: "", Inner: ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:    "mvp_matrix",
				Space:   ir.SpaceUniform,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
				Type:    0,
			},
		},
	}

	e := &Emitter{
		ir:              mod,
		mod:             module.NewModule(module.VertexShader),
		resourceHandles: make(map[ir.GlobalVariableHandle]int),
		opts:            EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0},
	}
	e.analyzeResources()

	if len(e.resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(e.resources))
	}

	ty := e.getCBVStructType(&e.resources[0])

	// Outer: hostlayout.mvp_matrix = { inner }
	if ty.Kind != module.TypeStruct {
		t.Fatalf("outer: expected struct, got kind %d", ty.Kind)
	}
	if ty.StructName != "hostlayout.mvp_matrix" {
		t.Errorf("outer: name: want %q, got %q", "hostlayout.mvp_matrix", ty.StructName)
	}
	if len(ty.StructElems) != 1 {
		t.Fatalf("outer: expected 1 member, got %d", len(ty.StructElems))
	}

	// Inner member: [4 x <4 x float>]
	inner := ty.StructElems[0]
	if inner.Kind != module.TypeArray {
		t.Fatalf("inner: expected array, got kind %d", inner.Kind)
	}
	if inner.ElemCount != 4 {
		t.Errorf("inner: array count: want 4, got %d", inner.ElemCount)
	}

	// Element: <4 x float>
	vecTy := inner.ElemType
	if vecTy.Kind != module.TypeVector {
		t.Fatalf("vec: expected vector, got kind %d", vecTy.Kind)
	}
	if vecTy.ElemCount != 4 {
		t.Errorf("vec: count: want 4, got %d", vecTy.ElemCount)
	}
	if vecTy.ElemType.Kind != module.TypeFloat || vecTy.ElemType.FloatBits != 32 {
		t.Errorf("vec elem: expected float32, got kind=%d bits=%d", vecTy.ElemType.Kind, vecTy.ElemType.FloatBits)
	}
}

// TestCBVStructTypeStructWithMatAndVec verifies that a struct with mat4x4 +
// vec4 fields produces DXC-matching typed members:
// %hostlayout.struct.Entity = type { [4 x <4 x float>], <4 x float> }
func TestCBVStructTypeStructWithMatAndVec(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			// 0: f32
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			// 1: Matrix { columns: Vec4, rows: Vec4, scalar: Float(4) }
			{Name: "", Inner: ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			// 2: vec4<f32>
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			// 3: Struct "Entity" { world: mat4x4, color: vec4<f32> }
			{Name: "Entity", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "world", Type: 1, Offset: 0},
					{Name: "color", Type: 2, Offset: 64},
				},
				Span: 80,
			}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:    "u_entity",
				Space:   ir.SpaceUniform,
				Binding: &ir.ResourceBinding{Group: 1, Binding: 0},
				Type:    3,
			},
		},
	}

	e := &Emitter{
		ir:              mod,
		mod:             module.NewModule(module.VertexShader),
		resourceHandles: make(map[ir.GlobalVariableHandle]int),
		opts:            EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0},
	}
	e.analyzeResources()

	if len(e.resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(e.resources))
	}

	ty := e.getCBVStructType(&e.resources[0])

	// Outer: hostlayout.u_entity = { hostlayout.struct.Entity }
	if ty.StructName != "hostlayout.u_entity" {
		t.Errorf("outer: name: want %q, got %q", "hostlayout.u_entity", ty.StructName)
	}
	if len(ty.StructElems) != 1 {
		t.Fatalf("outer: expected 1 elem (inner struct), got %d", len(ty.StructElems))
	}

	// Inner struct: hostlayout.struct.Entity = { [4 x <4 x float>], <4 x float> }
	innerSt := ty.StructElems[0]
	if innerSt.Kind != module.TypeStruct {
		t.Fatalf("inner: expected struct, got kind %d", innerSt.Kind)
	}
	if innerSt.StructName != "hostlayout.struct.Entity" {
		t.Errorf("inner: name: want %q, got %q", "hostlayout.struct.Entity", innerSt.StructName)
	}
	if len(innerSt.StructElems) != 2 {
		t.Fatalf("inner: expected 2 members, got %d", len(innerSt.StructElems))
	}

	// Member 0: [4 x <4 x float>] for mat4x4
	m0 := innerSt.StructElems[0]
	if m0.Kind != module.TypeArray || m0.ElemCount != 4 {
		t.Errorf("member[0]: want [4 x ...], got kind=%d count=%d", m0.Kind, m0.ElemCount)
	}
	if m0.ElemType.Kind != module.TypeVector || m0.ElemType.ElemCount != 4 {
		t.Errorf("member[0] elem: want <4 x float>, got kind=%d count=%d", m0.ElemType.Kind, m0.ElemType.ElemCount)
	}

	// Member 1: <4 x float> for vec4<f32>
	m1 := innerSt.StructElems[1]
	if m1.Kind != module.TypeVector || m1.ElemCount != 4 {
		t.Errorf("member[1]: want <4 x float>, got kind=%d count=%d", m1.Kind, m1.ElemCount)
	}
	if m1.ElemType.Kind != module.TypeFloat || m1.ElemType.FloatBits != 32 {
		t.Errorf("member[1] elem: want float32, got kind=%d bits=%d", m1.ElemType.Kind, m1.ElemType.FloatBits)
	}
}

// TestCBVStructTypeMatAndVecU32 verifies mat4x4<f32> + vec4<u32> produces:
// %hostlayout.struct.Globals = type { [4 x <4 x float>], <4 x i32> }
func TestCBVStructTypeMatAndVecU32(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			// 0: u32
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			// 1: Matrix { columns: Vec4, rows: Vec4, scalar: Float(4) }
			{Name: "", Inner: ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			// 2: vec4<u32>
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
			// 3: Struct "Globals" { view_proj: mat4x4, num_lights: vec4<u32> }
			{Name: "Globals", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "view_proj", Type: 1, Offset: 0},
					{Name: "num_lights", Type: 2, Offset: 64},
				},
				Span: 80,
			}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:    "u_globals",
				Space:   ir.SpaceUniform,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
				Type:    3,
			},
		},
	}

	e := &Emitter{
		ir:              mod,
		mod:             module.NewModule(module.VertexShader),
		resourceHandles: make(map[ir.GlobalVariableHandle]int),
		opts:            EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0},
	}
	e.analyzeResources()

	if len(e.resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(e.resources))
	}

	ty := e.getCBVStructType(&e.resources[0])

	// Navigate to inner struct
	innerSt := ty.StructElems[0]
	if innerSt.StructName != "hostlayout.struct.Globals" {
		t.Errorf("inner: name: want %q, got %q", "hostlayout.struct.Globals", innerSt.StructName)
	}
	if len(innerSt.StructElems) != 2 {
		t.Fatalf("inner: expected 2 members, got %d", len(innerSt.StructElems))
	}

	// Member 1: <4 x i32> for vec4<u32>
	m1 := innerSt.StructElems[1]
	if m1.Kind != module.TypeVector || m1.ElemCount != 4 {
		t.Errorf("member[1]: want <4 x i32>, got kind=%d count=%d", m1.Kind, m1.ElemCount)
	}
	if m1.ElemType.Kind != module.TypeInteger || m1.ElemType.IntBits != 32 {
		t.Errorf("member[1] elem: want i32, got kind=%d bits=%d", m1.ElemType.Kind, m1.ElemType.IntBits)
	}
}

// TestCBVStructTypeAllScalars verifies that a struct with only scalar fields
// produces individual scalar members without hostlayout prefix:
// %struct.SimParams = type { float, float, float, float, float, float, float }
func TestCBVStructTypeAllScalars(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			// 0: f32
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			// 1: Struct "SimParams" { 7 float fields }
			{Name: "SimParams", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "deltaT", Type: 0, Offset: 0},
					{Name: "rule1Distance", Type: 0, Offset: 4},
					{Name: "rule2Distance", Type: 0, Offset: 8},
					{Name: "rule3Distance", Type: 0, Offset: 12},
					{Name: "rule1Scale", Type: 0, Offset: 16},
					{Name: "rule2Scale", Type: 0, Offset: 20},
					{Name: "rule3Scale", Type: 0, Offset: 24},
				},
				Span: 28,
			}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:    "params",
				Space:   ir.SpaceUniform,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
				Type:    1,
			},
		},
	}

	e := &Emitter{
		ir:              mod,
		mod:             module.NewModule(module.ComputeShader),
		resourceHandles: make(map[ir.GlobalVariableHandle]int),
		opts:            EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0},
	}
	e.analyzeResources()

	if len(e.resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(e.resources))
	}

	ty := e.getCBVStructType(&e.resources[0])

	// Outer: params = { struct.SimParams }
	if ty.StructName != "params" {
		t.Errorf("outer: name: want %q, got %q", "params", ty.StructName)
	}
	if len(ty.StructElems) != 1 {
		t.Fatalf("outer: expected 1 elem, got %d", len(ty.StructElems))
	}

	// Inner: struct.SimParams (no hostlayout prefix)
	innerSt := ty.StructElems[0]
	if innerSt.StructName != "struct.SimParams" {
		t.Errorf("inner: name: want %q, got %q", "struct.SimParams", innerSt.StructName)
	}
	if len(innerSt.StructElems) != 7 {
		t.Fatalf("inner: expected 7 scalar members, got %d", len(innerSt.StructElems))
	}

	// All members should be float32
	for i, m := range innerSt.StructElems {
		if m.Kind != module.TypeFloat || m.FloatBits != 32 {
			t.Errorf("member[%d]: want float32, got kind=%d bits=%d", i, m.Kind, m.FloatBits)
		}
	}
}

// TestCBVStructTypeMat4x3 verifies mat4x3<f32> produces [4 x <3 x float>],
// matching DXC golden for the padding shader.
func TestCBVStructTypeMat4x3(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			// 0: f32
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			// 1: Matrix { columns: Vec4, rows: Vec3, scalar: Float(4) } = mat4x3
			{Name: "", Inner: ir.MatrixType{Columns: 4, Rows: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			// 2: Struct "Test3" { a: mat4x3, b: f32 }
			{Name: "Test3", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "a", Type: 1, Offset: 0},
					{Name: "b", Type: 0, Offset: 64},
				},
				Span: 68,
			}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:    "input3",
				Space:   ir.SpaceUniform,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 2},
				Type:    2,
			},
		},
	}

	e := &Emitter{
		ir:              mod,
		mod:             module.NewModule(module.VertexShader),
		resourceHandles: make(map[ir.GlobalVariableHandle]int),
		opts:            EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0},
	}
	e.analyzeResources()

	if len(e.resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(e.resources))
	}

	ty := e.getCBVStructType(&e.resources[0])

	// Navigate: hostlayout.input3 -> hostlayout.struct.Test3
	innerSt := ty.StructElems[0]
	if innerSt.StructName != "hostlayout.struct.Test3" {
		t.Errorf("inner: name: want %q, got %q", "hostlayout.struct.Test3", innerSt.StructName)
	}
	if len(innerSt.StructElems) != 2 {
		t.Fatalf("inner: expected 2 members, got %d", len(innerSt.StructElems))
	}

	// Member 0: [4 x <3 x float>] for mat4x3
	m0 := innerSt.StructElems[0]
	if m0.Kind != module.TypeArray {
		t.Fatalf("member[0]: expected array, got kind %d", m0.Kind)
	}
	if m0.ElemCount != 4 {
		t.Errorf("member[0]: array count: want 4, got %d", m0.ElemCount)
	}
	if m0.ElemType.Kind != module.TypeVector || m0.ElemType.ElemCount != 3 {
		t.Errorf("member[0] elem: want <3 x float>, got kind=%d count=%d", m0.ElemType.Kind, m0.ElemType.ElemCount)
	}

	// Member 1: float for scalar f32
	m1 := innerSt.StructElems[1]
	if m1.Kind != module.TypeFloat || m1.FloatBits != 32 {
		t.Errorf("member[1]: want float32, got kind=%d bits=%d", m1.Kind, m1.FloatBits)
	}
}

// TestCBVStructTypeThreeMatrices verifies a struct with three mat4x4<f32>
// produces the correct DXC-matching type:
// %hostlayout.struct.Uniforms = type { [4 x <4 x float>], [4 x <4 x float>], [4 x <4 x float>] }
func TestCBVStructTypeThreeMatrices(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			// 0: Matrix { columns: Vec4, rows: Vec4, scalar: Float(4) }
			{Name: "", Inner: ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			// 1: Struct "Uniforms" { model, view, projection }
			{Name: "Uniforms", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "model", Type: 0, Offset: 0},
					{Name: "view", Type: 0, Offset: 64},
					{Name: "projection", Type: 0, Offset: 128},
				},
				Span: 192,
			}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:    "uniforms",
				Space:   ir.SpaceUniform,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
				Type:    1,
			},
		},
	}

	e := &Emitter{
		ir:              mod,
		mod:             module.NewModule(module.VertexShader),
		resourceHandles: make(map[ir.GlobalVariableHandle]int),
		opts:            EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0},
	}
	e.analyzeResources()

	if len(e.resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(e.resources))
	}

	ty := e.getCBVStructType(&e.resources[0])

	// Navigate: hostlayout.uniforms -> hostlayout.struct.Uniforms
	if ty.StructName != "hostlayout.uniforms" {
		t.Errorf("outer: name: want %q, got %q", "hostlayout.uniforms", ty.StructName)
	}

	innerSt := ty.StructElems[0]
	if innerSt.StructName != "hostlayout.struct.Uniforms" {
		t.Errorf("inner: name: want %q, got %q", "hostlayout.struct.Uniforms", innerSt.StructName)
	}
	if len(innerSt.StructElems) != 3 {
		t.Fatalf("inner: expected 3 members, got %d", len(innerSt.StructElems))
	}

	// All three members: [4 x <4 x float>]
	for i, m := range innerSt.StructElems {
		if m.Kind != module.TypeArray || m.ElemCount != 4 {
			t.Errorf("member[%d]: want [4 x ...], got kind=%d count=%d", i, m.Kind, m.ElemCount)
		}
		if m.ElemType.Kind != module.TypeVector || m.ElemType.ElemCount != 4 {
			t.Errorf("member[%d] elem: want <4 x float>, got kind=%d count=%d", i, m.ElemType.Kind, m.ElemType.ElemCount)
		}
		if m.ElemType.ElemType.Kind != module.TypeFloat || m.ElemType.ElemType.FloatBits != 32 {
			t.Errorf("member[%d] scalar: want float32, got kind=%d", i, m.ElemType.ElemType.Kind)
		}
	}
}

// TestAnalyzeResourcesBindingMapSingleSRV verifies that analyzeResources
// applies a caller-supplied BindingMap to a single SRV resource: the raw
// WGSL @group(0) @binding(5) is remapped to (space=0, register=0), which
// is what wgpu's per-class monotonic register scheme expects.
func TestAnalyzeResourcesBindingMapSingleSRV(t *testing.T) {
	// IR module with a single SRV (sampled texture) at @group(0) @binding(5).
	imgHandle := ir.TypeHandle(0)
	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:    "tex",
				Space:   ir.SpaceHandle,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 5},
				Type:    imgHandle,
			},
		},
	}

	e := &Emitter{
		ir:              mod,
		mod:             module.NewModule(module.PixelShader),
		resourceHandles: make(map[ir.GlobalVariableHandle]int),
		opts: EmitOptions{
			ShaderModelMajor: 6,
			ShaderModelMinor: 0,
			BindingMap: BindingMap{
				BindingLocation{Group: 0, Binding: 5}: BindTarget{Space: 0, Register: 0},
			},
		},
	}
	e.analyzeResources()

	if len(e.resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(e.resources))
	}

	res := &e.resources[0]
	if res.class != resourceClassSRV {
		t.Errorf("expected SRV (class %d), got class %d", resourceClassSRV, res.class)
	}
	if res.group != 0 {
		t.Errorf("expected remapped group (space) 0, got %d", res.group)
	}
	if res.binding != 0 {
		t.Errorf("expected remapped binding (register) 0, got %d", res.binding)
	}
	if res.rangeID != 0 {
		t.Errorf("expected rangeID 0, got %d", res.rangeID)
	}
}

// TestAnalyzeResourcesBindingMapMultiClass verifies that a particles-style
// shader with an SRV, a UAV, and a CBV in the same bind group (raw WGSL
// bindings 0/1/2) can be remapped via BindingMap to (t0, u0, b0) — the
// per-class monotonic scheme wgpu/hal/dx12 uses. Each resource must end
// up with register 0 in its own class, and rangeID must remain monotonic
// per class (all three are the first in their class, so all 0).
func TestAnalyzeResourcesBindingMapMultiClass(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			// 0: f32
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			// 1: vec4<f32>
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			// 2: array<vec4<f32>> (runtime-sized)
			{Name: "", Inner: ir.ArrayType{Base: 1, Size: ir.ArraySize{Constant: nil}}},
			// 3: Params { x: f32 }
			{Name: "Params", Inner: ir.StructType{
				Members: []ir.StructMember{{Name: "x", Type: 0, Offset: 0}},
				Span:    4,
			}},
		},
		GlobalVariables: []ir.GlobalVariable{
			// SRV: read-only storage buffer (arrays of vec4)
			{
				Name:    "pin",
				Space:   ir.SpaceStorage,
				Access:  ir.StorageRead,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
				Type:    2,
			},
			// UAV: read-write storage buffer
			{
				Name:    "pout",
				Space:   ir.SpaceStorage,
				Access:  ir.StorageReadWrite,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 1},
				Type:    2,
			},
			// CBV: uniform buffer
			{
				Name:    "params",
				Space:   ir.SpaceUniform,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 2},
				Type:    3,
			},
		},
	}

	e := &Emitter{
		ir:              mod,
		mod:             module.NewModule(module.ComputeShader),
		resourceHandles: make(map[ir.GlobalVariableHandle]int),
		opts: EmitOptions{
			ShaderModelMajor: 6,
			ShaderModelMinor: 0,
			BindingMap: BindingMap{
				// All three collapse to register 0 in their respective
				// classes, matching wgpu's monotonic per-class scheme.
				BindingLocation{Group: 0, Binding: 0}: BindTarget{Space: 0, Register: 0}, // t0
				BindingLocation{Group: 0, Binding: 1}: BindTarget{Space: 0, Register: 0}, // u0
				BindingLocation{Group: 0, Binding: 2}: BindTarget{Space: 0, Register: 0}, // b0
			},
		},
	}
	e.analyzeResources()

	if len(e.resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(e.resources))
	}

	// After FEAT-DXIL-003: classifyGlobalVariable distinguishes read-only
	// (StorageRead → SRV / t-register) from read-write (StorageReadWrite →
	// UAV / u-register) storage buffers, matching the HLSL backend's
	// classifier. Per-class rangeID counters are independent, so pin and
	// pout both land at rangeID=0 — the first (and only) resource in
	// their respective classes.
	//
	// Expected:
	//   resources[0] = pin    : SRV, rangeID=0, (space=0, register=0) → t0
	//   resources[1] = pout   : UAV, rangeID=0, (space=0, register=0) → u0
	//   resources[2] = params : CBV, rangeID=0, (space=0, register=0) → b0

	type expect struct {
		name    string
		class   uint8
		rangeID int
		space   uint32
		reg     uint32
	}
	want := []expect{
		{"pin", resourceClassSRV, 0, 0, 0},
		{"pout", resourceClassUAV, 0, 0, 0},
		{"params", resourceClassCBV, 0, 0, 0},
	}

	for i, w := range want {
		res := &e.resources[i]
		if res.name != w.name {
			t.Errorf("resource[%d]: name: want %q, got %q", i, w.name, res.name)
		}
		if res.class != w.class {
			t.Errorf("resource[%d] (%s): class: want %d, got %d", i, w.name, w.class, res.class)
		}
		if res.rangeID != w.rangeID {
			t.Errorf("resource[%d] (%s): rangeID: want %d, got %d", i, w.name, w.rangeID, res.rangeID)
		}
		if res.group != w.space {
			t.Errorf("resource[%d] (%s): space (group): want %d, got %d", i, w.name, w.space, res.group)
		}
		if res.binding != w.reg {
			t.Errorf("resource[%d] (%s): register (binding): want %d, got %d", i, w.name, w.reg, res.binding)
		}
	}
}

// TestClassifyStorageBufferAccessMode verifies FEAT-DXIL-003: read-only
// storage buffers (var<storage, read>) classify as SRV (t-register), while
// read-write storage buffers (var<storage, read_write>) classify as UAV
// (u-register). Per-class rangeID counters are tracked independently, so
// the first SRV and the first UAV both get rangeID=0.
//
// This matches the HLSL backend's getRegisterTypeForAddressSpace in
// hlsl/storage.go. Without this, two storage buffers in the same bind
// group collide on the same UAV register in the DXIL root signature and
// D3D12 CreateComputePipelineState fails with E_INVALIDARG (see
// gogpu/examples/particles reproduction in FEAT-DXIL-003 task).
func TestClassifyStorageBufferAccessMode(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			// 0: f32
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			// 1: vec4<f32>
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			// 2: array<vec4<f32>> (runtime-sized)
			{Name: "", Inner: ir.ArrayType{Base: 1, Size: ir.ArraySize{Constant: nil}}},
		},
		GlobalVariables: []ir.GlobalVariable{
			// SRV: read-only storage buffer
			{
				Name:    "ro_buf",
				Space:   ir.SpaceStorage,
				Access:  ir.StorageRead,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
				Type:    2,
			},
			// UAV: read-write storage buffer
			{
				Name:    "rw_buf",
				Space:   ir.SpaceStorage,
				Access:  ir.StorageReadWrite,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 1},
				Type:    2,
			},
		},
	}

	e := &Emitter{
		ir:              mod,
		mod:             module.NewModule(module.ComputeShader),
		resourceHandles: make(map[ir.GlobalVariableHandle]int),
		opts: EmitOptions{
			ShaderModelMajor: 6,
			ShaderModelMinor: 0,
		},
	}
	e.analyzeResources()

	if len(e.resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(e.resources))
	}

	// ro_buf: SRV at t0 (first in SRV class → rangeID=0)
	ro := &e.resources[0]
	if ro.name != "ro_buf" {
		t.Errorf("resources[0].name: want %q, got %q", "ro_buf", ro.name)
	}
	if ro.class != resourceClassSRV {
		t.Errorf("ro_buf: class: want SRV (%d), got %d", resourceClassSRV, ro.class)
	}
	if ro.rangeID != 0 {
		t.Errorf("ro_buf: rangeID: want 0, got %d", ro.rangeID)
	}

	// rw_buf: UAV at u0 (first in UAV class → rangeID=0, independent of SRV counter)
	rw := &e.resources[1]
	if rw.name != "rw_buf" {
		t.Errorf("resources[1].name: want %q, got %q", "rw_buf", rw.name)
	}
	if rw.class != resourceClassUAV {
		t.Errorf("rw_buf: class: want UAV (%d), got %d", resourceClassUAV, rw.class)
	}
	if rw.rangeID != 0 {
		t.Errorf("rw_buf: rangeID: want 0 (independent per-class counter), got %d", rw.rangeID)
	}
}

// TestAnalyzeResourcesBindingMapNilBackwardCompat verifies that when
// BindingMap is nil, analyzeResources preserves the raw WGSL @group and
// @binding numbers — i.e. the backend continues to behave exactly as it
// did before FEAT-DXIL-002.
func TestAnalyzeResourcesBindingMapNilBackwardCompat(t *testing.T) {
	mod := buildCBVFragmentShader() // @group(0) @binding(0) uniforms CBV

	e := &Emitter{
		ir:              mod,
		mod:             module.NewModule(module.PixelShader),
		resourceHandles: make(map[ir.GlobalVariableHandle]int),
		opts: EmitOptions{
			ShaderModelMajor: 6,
			ShaderModelMinor: 0,
			// BindingMap: nil — explicitly no remap
		},
	}
	e.analyzeResources()

	if len(e.resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(e.resources))
	}
	res := &e.resources[0]
	if res.group != 0 || res.binding != 0 {
		t.Errorf("nil BindingMap: expected raw (0,0), got (%d,%d)", res.group, res.binding)
	}
}

// TestAnalyzeResourcesBindingMapPartialMiss verifies that when BindingMap
// is non-nil but does not contain an entry for a particular binding, that
// binding retains its raw WGSL numbers.
func TestAnalyzeResourcesBindingMapPartialMiss(t *testing.T) {
	imgHandle := ir.TypeHandle(0)
	samplerHandle := ir.TypeHandle(1)
	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}},
			{Name: "", Inner: ir.SamplerType{Comparison: false}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "tex", Space: ir.SpaceHandle, Binding: &ir.ResourceBinding{Group: 0, Binding: 3}, Type: imgHandle},
			{Name: "samp", Space: ir.SpaceHandle, Binding: &ir.ResourceBinding{Group: 0, Binding: 7}, Type: samplerHandle},
		},
	}

	e := &Emitter{
		ir:              mod,
		mod:             module.NewModule(module.PixelShader),
		resourceHandles: make(map[ir.GlobalVariableHandle]int),
		opts: EmitOptions{
			BindingMap: BindingMap{
				// Only remap the texture.
				BindingLocation{Group: 0, Binding: 3}: BindTarget{Space: 0, Register: 0},
			},
		},
	}
	e.analyzeResources()

	// Sampler-heap mode rewrites samp into a per-group index buffer SRV
	// (inserted at the sampler's position) plus a SamplerHeap entry appended
	// at the end. With the one texture (tex) before the sampler, the list is:
	//   [0] SRV tex                      — remapped from (0,3) → (0,0)
	//   [1] nagaGroup0SamplerIndexArray  — synthetic StructuredBuffer SRV (inserted at samp position)
	//   [2] nagaSamplerHeap              — synthetic SamplerHeap at (s0, space0)
	if len(e.resources) != 3 {
		t.Fatalf("expected 3 resources (SRV + IndexBuffer + SamplerHeap), got %d", len(e.resources))
	}
	// SRV (tex): remapped to (0,0).
	if e.resources[0].group != 0 || e.resources[0].binding != 0 {
		t.Errorf("tex: expected remapped (0,0), got (%d,%d)",
			e.resources[0].group, e.resources[0].binding)
	}
	// Sampler index buffer (synthesized SRV — inserted at sampler position).
	if e.resources[1].class != resourceClassSRV {
		t.Errorf("resources[1]: expected SRV index buffer, got class %d", e.resources[1].class)
	}
	if e.resources[1].name != "nagaGroup0SamplerIndexArray" {
		t.Errorf("resources[1]: expected name 'nagaGroup0SamplerIndexArray', got %q", e.resources[1].name)
	}
	// SamplerHeap (synthesized — replaces direct `samp` binding).
	if e.resources[2].class != resourceClassSampler {
		t.Errorf("resources[2]: expected SamplerHeap, got class %d", e.resources[2].class)
	}
	if e.resources[2].name != "nagaSamplerHeap" {
		t.Errorf("resources[2]: expected name 'nagaSamplerHeap', got %q", e.resources[2].name)
	}
}

// TestEmitBindingMapEndToEndCreateHandleIndex verifies that when a
// BindingMap is supplied to Emit(), the remapped register flows all the
// way through to the dx.op.createHandle "index" argument in the final
// DXIL module. This is the end-to-end guarantee: the index argument to
// createHandle must equal the remapped register (which will also be the
// DXIL metadata lowerBound — they must match, per DXIL spec).
//
// Setup: CBV "uniforms" at raw @group(0) @binding(0). BindingMap remaps
// (0,0) -> (space=0, register=7). Expected: createHandle is called with
// index=7 (as an i32 constant operand).
func TestEmitBindingMapEndToEndCreateHandleIndex(t *testing.T) {
	mod := buildCBVFragmentShader()

	result, err := Emit(mod, EmitOptions{
		ShaderModelMajor: 6,
		ShaderModelMinor: 0,
		BindingMap: BindingMap{
			BindingLocation{Group: 0, Binding: 0}: BindTarget{Space: 0, Register: 7},
		},
	})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	mainFn := findMainFunction(result)
	if mainFn == nil {
		t.Fatal("main function not found in emitted module")
	}

	// Find the dx.op.createHandle call and inspect its operand 3 (index).
	// createHandle signature: (i32 opcode, i8 class, i32 rangeID, i32 index, i1 nonUniform)
	var createHandleInstr *module.Instruction
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrCall && instr.CalledFunc != nil &&
				instr.CalledFunc.Name == "dx.op.createHandle" {
				createHandleInstr = instr
				break
			}
		}
		if createHandleInstr != nil {
			break
		}
	}
	if createHandleInstr == nil {
		t.Fatal("dx.op.createHandle call not found in main function")
	}

	// Operand layout: [0]=opcode, [1]=class, [2]=rangeID, [3]=index, [4]=nonUniform
	if len(createHandleInstr.Operands) != 5 {
		t.Fatalf("createHandle: expected 5 operands, got %d", len(createHandleInstr.Operands))
	}

	// The index operand is a value ID that points to an i32 constant.
	// Look it up in module.Constants (finalize() has assigned final
	// ValueIDs by the time Emit returns).
	indexValueID := createHandleInstr.Operands[3]

	var found bool
	var gotIndex int64
	for _, c := range result.Constants {
		if c == nil || c.IsUndef || c.IsAggregate {
			continue
		}
		if c.ValueID == indexValueID {
			found = true
			gotIndex = c.IntValue
			break
		}
	}
	if !found {
		t.Fatalf("could not locate constant for createHandle index value ID %d", indexValueID)
	}
	if gotIndex != 7 {
		t.Errorf("createHandle index: want 7 (remapped register), got %d", gotIndex)
	}
}

// TestUAVStoreStructElemFlattenedComponentCount locks the contract that
// emitUAVStore relies on for BUG-DXIL-005: when a storage buffer's element
// type is a struct (e.g. Particle { pos: vec2<f32>, vel: vec2<f32> }), the
// flattened scalar count must equal the sum of the members' scalar counts
// — NOT 1 (the previous componentCount default-branch result).
//
// Without this contract, `pout[i] = p` was lowered to a single dx.op.bufferStore
// call with write-mask 0x1, dropping 3 of 4 floats. cbvComponentCount must
// recurse into struct members so the store path emits a single 4-float
// bufferStore with mask 0xF (or, for >4-component aggregates, multiple
// batched bufferStore calls covering all components).
//
// Reference: D:/projects/gogpu/naga/dxil/internal/emit/resources.go
// emitUAVStore() — uses cbvComponentCount unconditionally to compute numComps.
func TestUAVStoreStructElemFlattenedComponentCount(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			// 0: f32
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			// 1: vec2<f32>
			{Name: "", Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			// 2: Particle { pos: vec2<f32>, vel: vec2<f32> }
			{Name: "Particle", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "pos", Type: 1, Offset: 0},
					{Name: "vel", Type: 1, Offset: 8},
				},
				Span: 16,
			}},
			// 3: Foo { a: f32, b: vec2<f32>, c: f32 } — 4 scalars total, mixed members
			{Name: "Foo", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "a", Type: 0, Offset: 0},
					{Name: "b", Type: 1, Offset: 4},
					{Name: "c", Type: 0, Offset: 12},
				},
				Span: 16,
			}},
			// 4: scalar f32 — sanity case
			// (already type 0; reuse)
		},
	}

	tests := []struct {
		name      string
		typeIdx   ir.TypeHandle
		wantComps int
	}{
		{"scalar f32", 0, 1},
		{"vec2<f32>", 1, 2},
		{"Particle struct {vec2,vec2}", 2, 4},
		{"mixed struct {f32,vec2,f32}", 3, 4},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := cbvComponentCount(mod, mod.Types[tc.typeIdx].Inner)
			if got != tc.wantComps {
				t.Errorf("cbvComponentCount(%s) = %d, want %d", tc.name, got, tc.wantComps)
			}
		})
	}
}

// TestGetResourceComponentType locks in the depth-texture path of
// getResourceComponentType. SampledKind is documented as valid only
// when Class == ImageClassSampled (ir.go:380); our IR parser leaves
// it at the zero value (ScalarSint) for depth textures. Without the
// explicit ImageClassDepth case added alongside this test, the lookup
// returned I32 and the validator trips 'sample_* instructions require
// resource to be declared to return UNORM, SNORM or FLOAT'.
//
// Mirror regression of the imageOverload fix (commit 21466c5).
func TestGetResourceComponentType(t *testing.T) {
	// Build minimal IR module with each interesting image variant.
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}},
			{Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarUint}},
			{Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarSint}},
			// Depth texture leaves SampledKind at zero (ScalarSint).
			// The buggy pre-21466c5 classification returned I32 here
			// which tripped the rule above; the fix below returns F32.
			{Inner: ir.ImageType{Dim: ir.DimCube, Arrayed: true, Class: ir.ImageClassDepth}},
			{Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassDepth}},
			// Storage image — SampledKind is not used at all; caller
			// goes through a different path (storage format), but
			// getResourceComponentType should still not crash or
			// misclassify.
			{Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassStorage}},
		},
	}
	e := &Emitter{ir: mod, mod: module.NewModule(module.PixelShader)}

	cases := []struct {
		name     string
		typeIdx  ir.TypeHandle
		wantComp int
	}{
		{"sampled f32 → F32", 0, dxilCompTypeF32},
		{"sampled u32 → U32", 1, dxilCompTypeU32},
		{"sampled i32 → I32", 2, dxilCompTypeI32},
		{"depth cube array → F32 regardless of zero SampledKind", 3, dxilCompTypeF32},
		{"depth 2D → F32 regardless of zero SampledKind", 4, dxilCompTypeF32},
		{"storage 2D → F32 default (storage comp type goes elsewhere)", 5, dxilCompTypeF32},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := &resourceInfo{typeHandle: tc.typeIdx}
			got := e.getResourceComponentType(res)
			if got != tc.wantComp {
				t.Errorf("getResourceComponentType = %d, want %d", got, tc.wantComp)
			}
		})
	}
}

// TestSamplerHeapBothKinds verifies BUG-DXIL-035: when a module has both
// a standard and a comparison sampler in the same bind group, the
// analysis produces ONE SamplerHeap entry + ONE ComparisonSamplerHeap
// entry + ONE shared per-group index buffer, with deterministic rangeID
// ordering (standard before comparison, textures before index buffer).
func TestSamplerHeapBothKinds(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			// 0: sampled 2D texture
			{Name: "", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}},
			// 1: depth 2D texture
			{Name: "", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassDepth, SampledKind: ir.ScalarFloat}},
			// 2: standard sampler
			{Name: "", Inner: ir.SamplerType{Comparison: false}},
			// 3: comparison sampler
			{Name: "", Inner: ir.SamplerType{Comparison: true}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "tex", Space: ir.SpaceHandle, Binding: &ir.ResourceBinding{Group: 0, Binding: 0}, Type: 0},
			{Name: "depth", Space: ir.SpaceHandle, Binding: &ir.ResourceBinding{Group: 0, Binding: 1}, Type: 1},
			{Name: "samp", Space: ir.SpaceHandle, Binding: &ir.ResourceBinding{Group: 0, Binding: 2}, Type: 2},
			{Name: "cmp", Space: ir.SpaceHandle, Binding: &ir.ResourceBinding{Group: 0, Binding: 3}, Type: 3},
		},
	}

	e := &Emitter{
		ir:              mod,
		mod:             module.NewModule(module.PixelShader),
		resourceHandles: make(map[ir.GlobalVariableHandle]int),
	}
	e.analyzeResources()

	// Expected after sampler-heap rewrite — index buffer SRV is inserted at
	// the position of the first sampler (global[2] "samp"), matching the HLSL
	// backend's writeSamplerIndexBuffer call order:
	//   [0] tex                          SRV     rangeID 0 (t0)
	//   [1] depth                        SRV     rangeID 1 (t1)
	//   [2] nagaGroup0SamplerIndexArray  SRV     rangeID 2 (t0, space255) - StructuredBuffer
	//   [3] nagaSamplerHeap              Sampler rangeID 0 (s0, space0, 2048 slots)
	//   [4] nagaComparisonSamplerHeap    Sampler rangeID 1 (s0, space1, 2048 slots)
	if len(e.resources) != 5 {
		t.Fatalf("expected 5 resources (2 SRV + 1 IndexBuffer + 2 SamplerHeap), got %d", len(e.resources))
	}
	if e.resources[2].name != "nagaGroup0SamplerIndexArray" {
		t.Errorf("resources[2]: expected nagaGroup0SamplerIndexArray, got %q", e.resources[2].name)
	}
	if e.resources[2].kindOverride != 12 { // StructuredBuffer
		t.Errorf("resources[2]: expected kindOverride=12 (StructuredBuffer), got %d", e.resources[2].kindOverride)
	}
	if e.resources[3].name != "nagaSamplerHeap" {
		t.Errorf("resources[3]: expected nagaSamplerHeap, got %q", e.resources[3].name)
	}
	if e.resources[3].comparisonSampler {
		t.Errorf("resources[3]: standard heap should NOT be comparisonSampler")
	}
	if e.resources[4].name != "nagaComparisonSamplerHeap" {
		t.Errorf("resources[4]: expected nagaComparisonSamplerHeap, got %q", e.resources[4].name)
	}
	if !e.resources[4].comparisonSampler {
		t.Errorf("resources[4]: comparison heap should be comparisonSampler")
	}

	// Each WGSL sampler global should resolve through samplerHeap state —
	// they are NOT in e.resourceHandles directly yet (emit phase does that).
	if e.samplerHeap == nil {
		t.Fatal("samplerHeap state should be populated")
	}
	if !e.samplerHeap.hasStandard || !e.samplerHeap.hasComparison {
		t.Errorf("hasStandard=%v hasComparison=%v, want both true",
			e.samplerHeap.hasStandard, e.samplerHeap.hasComparison)
	}
	if len(e.samplerHeap.samplerWGSLBinding) != 2 {
		t.Errorf("expected 2 WGSL sampler bindings tracked, got %d",
			len(e.samplerHeap.samplerWGSLBinding))
	}
}

// TestSamplerHeapDisabledByBindingArray verifies that a binding_array
// wrapping a sampler keeps the direct-binding path (heap rewrite bypassed)
// — the existing emitDynamicCreateHandle flow handles it.
func TestSamplerHeapDisabledByBindingArray(t *testing.T) {
	arrSize := uint32(5)
	mod := &ir.Module{
		Types: []ir.Type{
			// 0: sampler
			{Name: "", Inner: ir.SamplerType{Comparison: false}},
			// 1: binding_array<sampler, 5>
			{Name: "", Inner: ir.BindingArrayType{Base: 0, Size: &arrSize}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "samp_array", Space: ir.SpaceHandle, Binding: &ir.ResourceBinding{Group: 0, Binding: 0}, Type: 1},
		},
	}

	e := &Emitter{
		ir:              mod,
		mod:             module.NewModule(module.PixelShader),
		resourceHandles: make(map[ir.GlobalVariableHandle]int),
	}
	e.analyzeResources()

	// binding_array<sampler, N> must remain on the direct-binding path —
	// one resource, class=Sampler, isBindingArray=true. No heap
	// synthesis (e.samplerHeap == nil).
	if e.samplerHeap != nil {
		t.Errorf("samplerHeap should be nil when only binding_array<sampler> is present, got %+v", e.samplerHeap)
	}
	if len(e.resources) != 1 {
		t.Fatalf("expected 1 resource (binding_array sampler), got %d", len(e.resources))
	}
	if e.resources[0].class != resourceClassSampler {
		t.Errorf("resources[0]: expected Sampler class, got %d", e.resources[0].class)
	}
	if !e.resources[0].isBindingArray {
		t.Errorf("resources[0]: expected isBindingArray=true")
	}
	if e.resources[0].arraySize != 5 {
		t.Errorf("resources[0]: expected arraySize=5, got %d", e.resources[0].arraySize)
	}
}

// TestClassifyDxOpAttrReadOnly verifies that createHandle,
// createHandleFromBinding, annotateHandle, bufferLoad, sample, and other
// memory-reading intrinsics are classified as AttrSetReadOnly (nounwind
// readonly), matching DXC's per-intrinsic attribute classification.
func TestClassifyDxOpAttrReadOnly(t *testing.T) {
	readonlyCases := []string{
		"dx.op.createHandle",
		"dx.op.createHandleFromBinding",
		"dx.op.annotateHandle",
		"dx.op.bufferLoad.i32",
		"dx.op.bufferLoad.f32",
		"dx.op.rawBufferLoad.i32",
		"dx.op.cbufferLoadLegacy.f32",
		"dx.op.cbufferLoadLegacy.i32",
		"dx.op.sample.f32",
		"dx.op.sampleLevel.f32",
		"dx.op.sampleCmp.f32",
		"dx.op.sampleCmpLevelZero.f32",
		"dx.op.textureGather.f32",
		"dx.op.textureGatherCmp.f32",
		"dx.op.textureLoad.f32",
		"dx.op.getDimensions",
	}
	for _, name := range readonlyCases {
		got := classifyDxOpAttr(name)
		if got != module.AttrSetReadOnly {
			t.Errorf("classifyDxOpAttr(%q) = %d, want AttrSetReadOnly(%d)", name, got, module.AttrSetReadOnly)
		}
	}
}

// TestClassifyDxOpAttrReadNone verifies that pure (no memory effect)
// intrinsics are classified as AttrSetReadNone (nounwind readnone).
func TestClassifyDxOpAttrReadNone(t *testing.T) {
	readnoneCases := []string{
		"dx.op.loadInput.f32",
		"dx.op.unary.f32",
		"dx.op.binary.f32",
		"dx.op.binary.i32",
		"dx.op.tertiary.f32",
		"dx.op.dot3.f32",
		"dx.op.threadId.i32",
	}
	for _, name := range readnoneCases {
		got := classifyDxOpAttr(name)
		if got != module.AttrSetReadNone {
			t.Errorf("classifyDxOpAttr(%q) = %d, want AttrSetReadNone(%d)", name, got, module.AttrSetReadNone)
		}
	}
}

// TestClassifyDxOpAttrNounwind verifies that impure intrinsics (stores,
// atomics) are classified as AttrSetNounwind (nounwind only).
func TestClassifyDxOpAttrNounwind(t *testing.T) {
	nounwindCases := []string{
		"dx.op.storeOutput.f32",
		"dx.op.bufferStore.f32",
		"dx.op.bufferStore.i32",
		"dx.op.rawBufferStore.i32",
		"dx.op.textureStore.f32",
	}
	for _, name := range nounwindCases {
		got := classifyDxOpAttr(name)
		if got != module.AttrSetNounwind {
			t.Errorf("classifyDxOpAttr(%q) = %d, want AttrSetNounwind(%d)", name, got, module.AttrSetNounwind)
		}
	}
}

// TestSamplerIndexBufferInsertionOrder verifies that the sampler index
// buffer SRV is inserted at the position of the first sampler in the
// globals list — not prepended or appended — matching DXC's declaration
// order semantics.
func TestSamplerIndexBufferInsertionOrder(t *testing.T) {
	// Test case: globals order is [storage_buf, texture, sampler]
	// Expected: index buffer SRV gets rangeID AFTER the texture (position
	// where the sampler triggers the insertion).
	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                       // 0: f32 scalar
			{Name: "", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassDepth, SampledKind: ir.ScalarFloat}}, // 1: depth texture
			{Name: "", Inner: ir.SamplerType{Comparison: true}},                                                    // 2: comparison sampler
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "buf", Space: ir.SpaceStorage, Access: ir.StorageRead,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0}, Type: 0},
			{Name: "tex", Space: ir.SpaceHandle,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 1}, Type: 1},
			{Name: "samp", Space: ir.SpaceHandle,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 2}, Type: 2},
		},
	}

	e := &Emitter{
		ir:              mod,
		mod:             module.NewModule(module.PixelShader),
		resourceHandles: make(map[ir.GlobalVariableHandle]int),
	}
	e.analyzeResources()

	// Expected order:
	// [0] buf              SRV rangeID=0 (the storage buffer)
	// [1] tex              SRV rangeID=1 (the texture)
	// [2] nagaGroup0...    SRV rangeID=2 (index buffer, inserted at sampler position)
	// [3] nagaComparison...  Sampler rangeID=0
	if len(e.resources) != 4 {
		t.Fatalf("expected 4 resources, got %d", len(e.resources))
	}
	if e.resources[0].name != "buf" || e.resources[0].rangeID != 0 {
		t.Errorf("resources[0]: expected buf rangeID=0, got %q rangeID=%d",
			e.resources[0].name, e.resources[0].rangeID)
	}
	if e.resources[1].name != "tex" || e.resources[1].rangeID != 1 {
		t.Errorf("resources[1]: expected tex rangeID=1, got %q rangeID=%d",
			e.resources[1].name, e.resources[1].rangeID)
	}
	if e.resources[2].name != "nagaGroup0SamplerIndexArray" || e.resources[2].rangeID != 2 {
		t.Errorf("resources[2]: expected nagaGroup0SamplerIndexArray rangeID=2, got %q rangeID=%d",
			e.resources[2].name, e.resources[2].rangeID)
	}
	if e.resources[3].name != "nagaComparisonSamplerHeap" {
		t.Errorf("resources[3]: expected nagaComparisonSamplerHeap, got %q", e.resources[3].name)
	}
}

// TestTryResolveConstInt verifies that tryResolveConstInt follows alias
// chains and resolves to constant integer literals. This helper enables
// constant folding of byte-offset calculations after mem2reg promotes
// inlined function arguments.
func TestTryResolveConstInt(t *testing.T) {
	irMod := &ir.Module{}
	mod := module.NewModule(module.ComputeShader)
	e := &Emitter{ir: irMod, mod: mod}

	fn := &ir.Function{}

	// Literal I32(42)
	lit := ir.ExpressionHandle(len(fn.Expressions))
	fn.Expressions = append(fn.Expressions, ir.Expression{Kind: ir.Literal{Value: ir.LiteralI32(42)}})

	// Literal U32(7)
	litU := ir.ExpressionHandle(len(fn.Expressions))
	fn.Expressions = append(fn.Expressions, ir.Expression{Kind: ir.Literal{Value: ir.LiteralU32(7)}})

	// Alias chain: alias1 -> alias2 -> lit
	alias2 := ir.ExpressionHandle(len(fn.Expressions))
	fn.Expressions = append(fn.Expressions, ir.Expression{Kind: ir.ExprAlias{Source: lit}})

	alias1 := ir.ExpressionHandle(len(fn.Expressions))
	fn.Expressions = append(fn.Expressions, ir.Expression{Kind: ir.ExprAlias{Source: alias2}})

	// Non-constant expression
	nonConst := ir.ExpressionHandle(len(fn.Expressions))
	fn.Expressions = append(fn.Expressions, ir.Expression{Kind: ir.ExprZeroValue{}})

	// Float literal (not integer)
	litF := ir.ExpressionHandle(len(fn.Expressions))
	fn.Expressions = append(fn.Expressions, ir.Expression{Kind: ir.Literal{Value: ir.LiteralF32(3.14)}})

	tests := []struct {
		name    string
		handle  ir.ExpressionHandle
		wantOK  bool
		wantVal uint64
	}{
		{"direct i32 literal", lit, true, 42},
		{"direct u32 literal", litU, true, 7},
		{"alias chain to i32", alias1, true, 42},
		{"non-constant expression", nonConst, false, 0},
		{"float literal", litF, false, 0},
		{"out of range handle", ir.ExpressionHandle(999), false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := e.tryResolveConstInt(fn, tt.handle)
			if ok != tt.wantOK {
				t.Fatalf("tryResolveConstInt(%d): ok=%v, want %v", tt.handle, ok, tt.wantOK)
			}
			if ok && val != tt.wantVal {
				t.Fatalf("tryResolveConstInt(%d): val=%d, want %d", tt.handle, val, tt.wantVal)
			}
		})
	}
}

// TestBuildHandleEmitOrder verifies that createHandle calls are emitted in
// DXC convention order: UAV first (descending rangeID), then SRV (descending
// rangeID), then CBV (descending rangeID), then Sampler. Binding arrays and
// virtual entries are excluded from the emission order.
func TestBuildHandleEmitOrder(t *testing.T) {
	tests := []struct {
		name      string
		resources []resourceInfo
		// wantOrder is the expected sequence of (class, rangeID) pairs.
		wantOrder [][2]int
	}{
		{
			name: "single_class_UAV_descending",
			resources: []resourceInfo{
				{class: resourceClassUAV, rangeID: 0, name: "buf0"},
				{class: resourceClassUAV, rangeID: 1, name: "buf1"},
				{class: resourceClassUAV, rangeID: 2, name: "buf2"},
			},
			wantOrder: [][2]int{
				{int(resourceClassUAV), 2},
				{int(resourceClassUAV), 1},
				{int(resourceClassUAV), 0},
			},
		},
		{
			// Mirrors boids.wgsl: CBV(b0), SRV(t1), UAV(u2).
			// DXC emits: UAV, SRV, CBV.
			name: "mixed_UAV_SRV_CBV_boids_pattern",
			resources: []resourceInfo{
				{class: resourceClassCBV, rangeID: 0, name: "params", binding: 0},
				{class: resourceClassSRV, rangeID: 0, name: "particlesSrc", binding: 1},
				{class: resourceClassUAV, rangeID: 0, name: "particlesDst", binding: 2},
			},
			wantOrder: [][2]int{
				{int(resourceClassUAV), 0},
				{int(resourceClassSRV), 0},
				{int(resourceClassCBV), 0},
			},
		},
		{
			// Mirrors bounds-check-dynamic-buffer: 5 UAVs interleaved with 1 CBV.
			// DXC emits: UAV(4,3,2,1,0), CBV(0).
			name: "interleaved_UAV_CBV",
			resources: []resourceInfo{
				{class: resourceClassSRV, rangeID: 0, name: "src0", binding: 0},
				{class: resourceClassUAV, rangeID: 0, name: "dst0", binding: 0},
				{class: resourceClassCBV, rangeID: 0, name: "uniforms", binding: 0},
				{class: resourceClassUAV, rangeID: 1, name: "dst1", binding: 1},
				{class: resourceClassUAV, rangeID: 2, name: "dst2", binding: 2},
				{class: resourceClassUAV, rangeID: 3, name: "dst3", binding: 3},
				{class: resourceClassUAV, rangeID: 4, name: "dst4", binding: 4},
			},
			wantOrder: [][2]int{
				{int(resourceClassUAV), 4},
				{int(resourceClassUAV), 3},
				{int(resourceClassUAV), 2},
				{int(resourceClassUAV), 1},
				{int(resourceClassUAV), 0},
				{int(resourceClassSRV), 0},
				{int(resourceClassCBV), 0},
			},
		},
		{
			// Mirrors shadow shader: SRVs, CBVs, Sampler — no UAV.
			// DXC emits: SRV(1,0), CBV(2,1,0), Sampler(0).
			name: "SRV_CBV_Sampler_shadow_pattern",
			resources: []resourceInfo{
				{class: resourceClassSRV, rangeID: 0, name: "tex0"},
				{class: resourceClassSRV, rangeID: 1, name: "tex1"},
				{class: resourceClassCBV, rangeID: 0, name: "cb0"},
				{class: resourceClassCBV, rangeID: 1, name: "cb1"},
				{class: resourceClassCBV, rangeID: 2, name: "cb2"},
				{class: resourceClassSampler, rangeID: 0, name: "samp0"},
			},
			wantOrder: [][2]int{
				{int(resourceClassSRV), 1},
				{int(resourceClassSRV), 0},
				{int(resourceClassCBV), 2},
				{int(resourceClassCBV), 1},
				{int(resourceClassCBV), 0},
				{int(resourceClassSampler), 0},
			},
		},
		{
			// Binding arrays and virtual entries are excluded.
			name: "skip_binding_arrays_and_virtual",
			resources: []resourceInfo{
				{class: resourceClassUAV, rangeID: 0, name: "buf0"},
				{class: resourceClassSRV, rangeID: 0, name: "ba", isBindingArray: true},
				{class: resourceClassSampler, rangeID: 0, name: "virt", virtual: true},
				{class: resourceClassCBV, rangeID: 0, name: "cb0"},
			},
			wantOrder: [][2]int{
				{int(resourceClassUAV), 0},
				{int(resourceClassCBV), 0},
			},
		},
		{
			// Empty resources list.
			name:      "empty",
			resources: nil,
			wantOrder: [][2]int{},
		},
		{
			// All four classes present with multiple rangeIDs.
			name: "all_four_classes",
			resources: []resourceInfo{
				{class: resourceClassCBV, rangeID: 0, name: "cb0"},
				{class: resourceClassSampler, rangeID: 0, name: "samp0"},
				{class: resourceClassSRV, rangeID: 0, name: "srv0"},
				{class: resourceClassSRV, rangeID: 1, name: "srv1"},
				{class: resourceClassUAV, rangeID: 0, name: "uav0"},
				{class: resourceClassUAV, rangeID: 1, name: "uav1"},
			},
			wantOrder: [][2]int{
				{int(resourceClassUAV), 1},
				{int(resourceClassUAV), 0},
				{int(resourceClassSRV), 1},
				{int(resourceClassSRV), 0},
				{int(resourceClassCBV), 0},
				{int(resourceClassSampler), 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Emitter{
				resources: tt.resources,
			}
			order := e.buildHandleEmitOrder()
			if len(order) != len(tt.wantOrder) {
				t.Fatalf("len(order)=%d, want %d", len(order), len(tt.wantOrder))
			}
			for i, idx := range order {
				res := &e.resources[idx]
				gotClass := int(res.class)
				gotRange := res.rangeID
				wantClass := tt.wantOrder[i][0]
				wantRange := tt.wantOrder[i][1]
				if gotClass != wantClass || gotRange != wantRange {
					t.Errorf("order[%d]: got (class=%d, rangeID=%d), want (class=%d, rangeID=%d)",
						i, gotClass, gotRange, wantClass, wantRange)
				}
			}
		})
	}
}

// TestCreateHandleLegacyName verifies that getDxOpCreateHandleFunc returns
// the correct function name (dx.op.createHandle, opcode 57). DXC uses this
// for all shader models; the SM 6.6+ createHandleFromBinding path has been
// removed since DXC's default compilation mode doesn't use it.
func TestCreateHandleLegacyName(t *testing.T) {
	e := &Emitter{
		ir:        &ir.Module{},
		mod:       module.NewModule(module.ComputeShader),
		dxOpFuncs: make(map[dxOpKey]*module.Function),
		opts:      EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 6},
	}
	fn := e.getDxOpCreateHandleFunc()
	if fn.Name != "dx.op.createHandle" {
		t.Errorf("got function name %q, want %q", fn.Name, "dx.op.createHandle")
	}
}
