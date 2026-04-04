// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestHLSL_ZeroValueWrapperFunction verifies ZeroValue wrapper generation.
func TestHLSL_ZeroValueWrapperFunction(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}},
		},
		GlobalExpressions: []ir.Expression{
			{Kind: ir.ExprZeroValue{Type: 0}},
		},
	}

	w := newWriter(module, &Options{FakeMissingBindings: true})
	w.writeZeroValueWrapperFunctions(module.GlobalExpressions)
	output := w.out.String()

	if !strings.Contains(output, "int4 ZeroValueint4()") {
		t.Errorf("expected ZeroValueint4 wrapper function, got:\n%s", output)
	}
	if !strings.Contains(output, "return (int4)0;") {
		t.Errorf("expected (int4)0 return, got:\n%s", output)
	}
}

// TestHLSL_ZeroValueExpression verifies ZeroValue renders as function call.
func TestHLSL_ZeroValueExpression(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}},
		},
	}

	w := newWriter(module, &Options{FakeMissingBindings: true})
	err := w.writeZeroValueExpression(ir.ExprZeroValue{Type: 0})
	if err != nil {
		t.Fatal(err)
	}
	got := w.out.String()
	if got != "ZeroValueint4()" {
		t.Errorf("writeZeroValueExpression = %q, want %q", got, "ZeroValueint4()")
	}
}

// TestHLSL_EmptyComposeExpandsZeros verifies empty Compose gets explicit zero args.
func TestHLSL_EmptyComposeExpandsZeros(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	w := newWriter(module, &Options{FakeMissingBindings: true})
	err := w.writeGlobalComposeExpression(ir.ExprCompose{Type: 0, Components: nil})
	if err != nil {
		t.Fatal(err)
	}
	got := w.out.String()
	if got != "float2(0.0, 0.0)" {
		t.Errorf("empty compose = %q, want %q", got, "float2(0.0, 0.0)")
	}
}

// TestHLSL_ArrayConstructorTypedef verifies array constructor typedef generation.
func TestHLSL_ArrayConstructorTypedef(t *testing.T) {
	size4 := uint32(4)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &size4}, Stride: 4}},
		},
	}

	w := newWriter(module, &Options{FakeMissingBindings: true})
	w.writeArrayConstructor(1)
	output := w.out.String()

	if !strings.Contains(output, "typedef float ret_Constructarray4_float_[4];") {
		t.Errorf("expected typedef, got:\n%s", output)
	}
	if !strings.Contains(output, "ret_Constructarray4_float_ Constructarray4_float_(") {
		t.Errorf("expected constructor function, got:\n%s", output)
	}
}

// TestHLSL_StructPadding verifies padding fields in struct definitions.
func TestHLSL_StructPadding(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "Padded", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "a", Type: 0, Offset: 0},
					{Name: "b", Type: 0, Offset: 16}, // 12 bytes of padding
				},
				Span: 32, // 12 bytes of end padding
			}},
		},
	}

	w := newWriter(module, &Options{FakeMissingBindings: true})
	_ = w.registerNames()
	st := module.Types[1].Inner.(ir.StructType)
	err := w.writeStructDefinition(1, "Padded", st)
	if err != nil {
		t.Fatal(err)
	}
	output := w.out.String()

	// Should have padding between a (offset 0, size 4) and b (offset 16)
	if !strings.Contains(output, "_pad1_") {
		t.Errorf("expected padding before member 1, got:\n%s", output)
	}
	// Should have end padding (span 32, last member at offset 16 + 4 = 20)
	if !strings.Contains(output, "_end_pad_") {
		t.Errorf("expected end padding, got:\n%s", output)
	}
}

// TestHLSL_PreciseModifier verifies `precise` on invariant SV_Position.
func TestHLSL_PreciseModifier(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "Output", Inner: ir.StructType{
				Members: []ir.StructMember{
					{
						Name:    "position",
						Type:    0,
						Binding: ptrBinding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition, Invariant: true}),
					},
				},
				Span: 16,
			}},
		},
	}

	w := newWriter(module, &Options{FakeMissingBindings: true})
	_ = w.registerNames()
	st := module.Types[1].Inner.(ir.StructType)
	err := w.writeStructDefinition(1, "Output", st)
	if err != nil {
		t.Fatal(err)
	}
	output := w.out.String()

	if !strings.Contains(output, "precise float4") {
		t.Errorf("expected 'precise float4', got:\n%s", output)
	}
}

func ptrBinding(b ir.Binding) *ir.Binding {
	return &b
}

// TestHLSL_FunctionalCast verifies As expression uses functional cast type(x).
func TestHLSL_FunctionalCast(t *testing.T) {
	width := uint8(4)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		},
		Functions: []ir.Function{
			{
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 0}},
					{Kind: ir.ExprAs{Expr: 0, Kind: ir.ScalarFloat, Convert: &width}},
				},
			},
		},
	}

	w := newWriter(module, &Options{FakeMissingBindings: true})
	w.currentFunction = &module.Functions[0]
	w.currentFuncHandle = 0

	err := w.writeAsExpression(ir.ExprAs{Expr: 0, Kind: ir.ScalarFloat, Convert: &width})
	if err != nil {
		t.Fatal(err)
	}
	got := w.out.String()
	// Should use functional cast: float(...)  NOT (float)(...)
	if strings.Contains(got, "(float)(") {
		t.Errorf("should use functional cast, got C-style: %q", got)
	}
	if !strings.Contains(got, "float(") {
		t.Errorf("expected functional cast float(...), got: %q", got)
	}
}

// TestHLSL_InoutPointerArgs verifies pointer arguments get inout prefix.
func TestHLSL_InoutPointerArgs(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Inner: ir.PointerType{Base: 0, Space: ir.SpaceFunction}},
		},
		Functions: []ir.Function{
			{
				Name: "takes_ptr",
				Arguments: []ir.FunctionArgument{
					{Name: "p", Type: 1},
				},
			},
		},
	}

	opts := &Options{FakeMissingBindings: true}
	w := newWriter(module, opts)
	_ = w.registerNames()
	w.writePerFunctionWrappedHelpers(&module.Functions[0])
	err := w.writeFunction(0, &module.Functions[0])
	if err != nil {
		t.Fatal(err)
	}
	output := w.out.String()

	if !strings.Contains(output, "inout int") {
		t.Errorf("expected 'inout int', got:\n%s", output)
	}
}

// TestHLSL_WriteSpecialConstants verifies NagaConstants struct generation.
func TestHLSL_WriteSpecialConstants(t *testing.T) {
	module := &ir.Module{}
	names := map[nameKey]string{}
	w := newTestWriter(module, names, map[ir.TypeHandle]string{})
	w.options.SpecialConstantsBinding = &BindTarget{Register: 0, Space: 1}

	w.writeSpecialConstants()
	output := w.out.String()

	if !strings.Contains(output, "struct NagaConstants {") {
		t.Error("expected NagaConstants struct")
	}
	if !strings.Contains(output, "int first_vertex;") {
		t.Error("expected first_vertex field")
	}
	if !strings.Contains(output, "int first_instance;") {
		t.Error("expected first_instance field")
	}
	if !strings.Contains(output, "uint other;") {
		t.Error("expected other field")
	}
	if !strings.Contains(output, "ConstantBuffer<NagaConstants> _NagaConstants: register(b0, space1)") {
		t.Errorf("expected correct register binding, got:\n%s", output)
	}
}

// TestHLSL_WorkgroupZeroInitArrayLoop verifies that workgroup array variables
// use per-element loops instead of (Type[N])0, which causes FXC to hang.
func TestHLSL_WorkgroupZeroInitArrayLoop(t *testing.T) {
	size4 := uint32(4)
	size256 := uint32(256)

	t.Run("scalar_no_loop", func(t *testing.T) {
		module := &ir.Module{
			Types: []ir.Type{
				{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, // 0: uint
			},
			GlobalVariables: []ir.GlobalVariable{
				{Name: "wg_counter", Space: ir.SpaceWorkGroup, Type: 0},
			},
		}
		names := map[nameKey]string{
			{kind: nameKeyGlobalVariable, handle1: 0}: "wg_counter",
		}
		w := newTestWriter(module, names, map[ir.TypeHandle]string{0: "uint"})
		w.options.ZeroInitializeWorkgroupMemory = true
		w.writeWorkgroupInit()
		output := w.out.String()
		if !strings.Contains(output, "wg_counter = (uint)0;") {
			t.Errorf("scalar should use (Type)0, got:\n%s", output)
		}
		if strings.Contains(output, "for (uint") {
			t.Errorf("scalar should NOT use loop, got:\n%s", output)
		}
	})

	t.Run("array_uses_loop", func(t *testing.T) {
		module := &ir.Module{
			Types: []ir.Type{
				{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},                  // 0: uint
				{Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &size256}}}, // 1: array<uint, 256>
			},
			GlobalVariables: []ir.GlobalVariable{
				{Name: "wg_data", Space: ir.SpaceWorkGroup, Type: 1},
			},
		}
		names := map[nameKey]string{
			{kind: nameKeyGlobalVariable, handle1: 0}: "wg_data",
		}
		w := newTestWriter(module, names, map[ir.TypeHandle]string{0: "uint"})
		w.options.ZeroInitializeWorkgroupMemory = true
		w.writeWorkgroupInit()
		output := w.out.String()
		if !strings.Contains(output, "for (uint _naga_zi_0 = 0u; _naga_zi_0 < 256u; _naga_zi_0++)") {
			t.Errorf("array should use loop, got:\n%s", output)
		}
		if !strings.Contains(output, "wg_data[_naga_zi_0] = (uint)0;") {
			t.Errorf("array element should use (ElementType)0, got:\n%s", output)
		}
	})

	t.Run("nested_array_nested_loops", func(t *testing.T) {
		module := &ir.Module{
			Types: []ir.Type{
				{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                 // 0: float
				{Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &size4}}},   // 1: array<float, 4>
				{Inner: ir.ArrayType{Base: 1, Size: ir.ArraySize{Constant: &size256}}}, // 2: array<array<float, 4>, 256>
			},
			GlobalVariables: []ir.GlobalVariable{
				{Name: "wg_matrix", Space: ir.SpaceWorkGroup, Type: 2},
			},
		}
		names := map[nameKey]string{
			{kind: nameKeyGlobalVariable, handle1: 0}: "wg_matrix",
		}
		w := newTestWriter(module, names, map[ir.TypeHandle]string{0: "float"})
		w.options.ZeroInitializeWorkgroupMemory = true
		w.writeWorkgroupInit()
		output := w.out.String()
		if !strings.Contains(output, "for (uint _naga_zi_0 = 0u; _naga_zi_0 < 256u; _naga_zi_0++)") {
			t.Errorf("outer loop missing, got:\n%s", output)
		}
		// Inner array (size 4) is below threshold — uses bulk assign, not loop
		if !strings.Contains(output, "wg_matrix[_naga_zi_0] = (float[4])0;") {
			t.Errorf("inner bulk assign missing, got:\n%s", output)
		}
	})

	t.Run("struct_no_loop", func(t *testing.T) {
		module := &ir.Module{
			Types: []ir.Type{
				{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, // 0: uint
				{Name: "MyStruct", Inner: ir.StructType{
					Members: []ir.StructMember{{Name: "x", Type: 0, Offset: 0}},
				}}, // 1: struct
			},
			GlobalVariables: []ir.GlobalVariable{
				{Name: "wg_struct", Space: ir.SpaceWorkGroup, Type: 1},
			},
		}
		names := map[nameKey]string{
			{kind: nameKeyGlobalVariable, handle1: 0}: "wg_struct",
		}
		w := newTestWriter(module, names, map[ir.TypeHandle]string{1: "MyStruct"})
		w.options.ZeroInitializeWorkgroupMemory = true
		w.writeWorkgroupInit()
		output := w.out.String()
		if !strings.Contains(output, "wg_struct = (MyStruct)0;") {
			t.Errorf("struct should use (Type)0, got:\n%s", output)
		}
		if strings.Contains(output, "for (uint") {
			t.Errorf("struct should NOT use loop, got:\n%s", output)
		}
	})

	t.Run("array_of_structs_uses_loop", func(t *testing.T) {
		module := &ir.Module{
			Types: []ir.Type{
				{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, // 0: uint
				{Name: "PathMonoid", Inner: ir.StructType{
					Members: []ir.StructMember{{Name: "x", Type: 0, Offset: 0}},
				}}, // 1: struct PathMonoid
				{Inner: ir.ArrayType{Base: 1, Size: ir.ArraySize{Constant: &size256}}}, // 2: array<PathMonoid, 256>
			},
			GlobalVariables: []ir.GlobalVariable{
				{Name: "sh_scratch", Space: ir.SpaceWorkGroup, Type: 2},
			},
		}
		names := map[nameKey]string{
			{kind: nameKeyGlobalVariable, handle1: 0}: "sh_scratch",
		}
		w := newTestWriter(module, names, map[ir.TypeHandle]string{1: "PathMonoid"})
		w.options.ZeroInitializeWorkgroupMemory = true
		w.writeWorkgroupInit()
		output := w.out.String()
		if !strings.Contains(output, "for (uint _naga_zi_0 = 0u; _naga_zi_0 < 256u; _naga_zi_0++)") {
			t.Errorf("array of structs should use loop, got:\n%s", output)
		}
		if !strings.Contains(output, "sh_scratch[_naga_zi_0] = (PathMonoid)0;") {
			t.Errorf("element should use (StructType)0, got:\n%s", output)
		}
	})
}

// TestHLSL_NeedWorkgroupInit verifies workgroup init detection.
func TestHLSL_NeedWorkgroupInit(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, // 0: uint
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "wg_counter", Space: ir.SpaceWorkGroup, Type: 0},
		},
	}

	names := map[nameKey]string{
		{kind: nameKeyGlobalVariable, handle1: 0}: "wg_counter",
	}
	w := newTestWriter(module, names, map[ir.TypeHandle]string{0: "uint"})
	w.options.ZeroInitializeWorkgroupMemory = true

	// Compute stage with workgroup var should need init
	ep := &ir.EntryPoint{Stage: ir.StageCompute}
	if !w.needWorkgroupInit(ep) {
		t.Error("compute EP with workgroup var should need init")
	}

	// Vertex stage should not need init
	ep2 := &ir.EntryPoint{Stage: ir.StageVertex}
	if w.needWorkgroupInit(ep2) {
		t.Error("vertex EP should not need workgroup init")
	}

	// Disabled option should not need init
	w.options.ZeroInitializeWorkgroupMemory = false
	if w.needWorkgroupInit(ep) {
		t.Error("disabled ZeroInitializeWorkgroupMemory should not need init")
	}
}
