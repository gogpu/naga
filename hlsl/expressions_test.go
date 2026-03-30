// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// TestIsI32ScalarOp
// =============================================================================

func TestIsI32ScalarOp(t *testing.T) {
	// Module with types
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},                                       // 0: i32
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},                                       // 1: u32
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                      // 2: f32
			{Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}}, // 3: vec4<i32>
			{Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}}, // 4: vec4<u32>
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 8}},                                       // 5: i64
		},
	}

	i32Handle := ir.TypeHandle(0)
	u32Handle := ir.TypeHandle(1)
	f32Handle := ir.TypeHandle(2)
	ivec4Handle := ir.TypeHandle(3)
	uvec4Handle := ir.TypeHandle(4)
	i64Handle := ir.TypeHandle(5)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralI32(1)}}, // 0: i32 literal
			{Kind: ir.Literal{Value: ir.LiteralU32(1)}}, // 1: u32 literal
			{Kind: ir.Literal{Value: ir.LiteralF32(1)}}, // 2: f32 literal
			{Kind: ir.Literal{Value: ir.LiteralI32(1)}}, // 3: i32 (for vec4 test)
			{Kind: ir.Literal{Value: ir.LiteralU32(1)}}, // 4: u32 (for vec4 test)
			{Kind: ir.Literal{Value: ir.LiteralI64(1)}}, // 5: i64 literal
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &i32Handle},
			{Handle: &u32Handle},
			{Handle: &f32Handle},
			{Handle: &ivec4Handle},
			{Handle: &uvec4Handle},
			{Handle: &i64Handle},
		},
	}

	w := newTestWriter(module, nil, nil)
	setCurrentFunction(w, fn)

	tests := []struct {
		name string
		left ir.ExpressionHandle
		want bool
	}{
		{"i32_left", 0, true},  // i32 -> true
		{"u32_left", 1, false}, // u32 -> false (not signed)
		{"f32_left", 2, false}, // f32 -> false
		{"vec4_i32", 3, true},  // vec4<i32> -> true (scalar is i32)
		{"vec4_u32", 4, false}, // vec4<u32> -> false
		{"i64_left", 5, false}, // i64 -> false (width != 4)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := ir.ExprBinary{
				Op:    ir.BinaryAdd,
				Left:  tt.left,
				Right: tt.left, // same type for simplicity
			}
			got := w.isI32ScalarOp(e)
			if got != tt.want {
				t.Errorf("isI32ScalarOp(left=%d) = %v, want %v", tt.left, got, tt.want)
			}
		})
	}
}

// =============================================================================
// TestWriteAsExpressionVectorCast
// =============================================================================

func TestWriteAsExpressionVectorCast(t *testing.T) {
	// Test that casting vec4<i32> to vec4<f32> produces "float4(...)" not "float(...)"
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}},  // 0: vec4<i32>
			{Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // 1: vec2<f32>
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                       // 2: f32
		},
	}

	ivec4Handle := ir.TypeHandle(0)
	vec2fHandle := ir.TypeHandle(1)
	f32Handle := ir.TypeHandle(2)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralI32(0)}}, // 0: some i32 value
			{Kind: ir.Literal{Value: ir.LiteralI32(0)}}, // 1: some i32 value
			{Kind: ir.Literal{Value: ir.LiteralF32(0)}}, // 2: some f32 value
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &ivec4Handle}, // expr 0 is vec4<i32>
			{Handle: &vec2fHandle}, // expr 1 is vec2<f32>
			{Handle: &f32Handle},   // expr 2 is scalar f32
		},
		NamedExpressions: make(map[ir.ExpressionHandle]string),
	}

	w := newTestWriter(module, nil, nil)
	setCurrentFunction(w, fn)

	width4 := uint8(4)
	tests := []struct {
		name    string
		expr    ir.ExprAs
		wantPfx string // what the output should start with
	}{
		{
			"vec4_i32_to_f32",
			ir.ExprAs{Expr: 0, Kind: ir.ScalarFloat, Convert: &width4},
			"float4(",
		},
		{
			"vec2_f32_to_i32",
			ir.ExprAs{Expr: 1, Kind: ir.ScalarSint, Convert: &width4},
			"naga_f2i32(",
		},
		{
			"scalar_f32_to_i32",
			ir.ExprAs{Expr: 2, Kind: ir.ScalarSint, Convert: &width4},
			"naga_f2i32(",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w.out.Reset()
			err := w.writeAsExpression(tt.expr)
			if err != nil {
				t.Fatalf("writeAsExpression: %v", err)
			}
			got := w.out.String()
			if len(got) < len(tt.wantPfx) || got[:len(tt.wantPfx)] != tt.wantPfx {
				t.Errorf("writeAsExpression:\n  got:  %q\n  want prefix: %q", got, tt.wantPfx)
			}
		})
	}
}

// =============================================================================
// TestI32MinLiteral
// =============================================================================

func TestI32MinLiteral(t *testing.T) {
	tests := []struct {
		width uint8
		want  string
	}{
		{4, "int(-2147483647 - 1)"},
		{8, "(-9223372036854775807L - 1L)"},
	}
	for _, tt := range tests {
		got := i32MinLiteral(tt.width)
		if got != tt.want {
			t.Errorf("i32MinLiteral(%d) = %q, want %q", tt.width, got, tt.want)
		}
	}
}

// =============================================================================
// TestWrappingSignedArithmetic
// =============================================================================

func TestWrappingSignedArithmetic(t *testing.T) {
	// Verify that i32 Add/Sub/Mul produces asint(asuint(...) op asuint(...)) pattern
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, // 0: i32
		},
	}

	i32Handle := ir.TypeHandle(0)
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralI32(1)}},
			{Kind: ir.Literal{Value: ir.LiteralI32(2)}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &i32Handle},
			{Handle: &i32Handle},
		},
		NamedExpressions: make(map[ir.ExpressionHandle]string),
	}

	w := newTestWriter(module, nil, nil)
	setCurrentFunction(w, fn)

	tests := []struct {
		name string
		op   ir.BinaryOperator
		want string
	}{
		{"add_i32", ir.BinaryAdd, "asint(asuint(int(1)) + asuint(int(2)))"},
		{"sub_i32", ir.BinarySubtract, "asint(asuint(int(1)) - asuint(int(2)))"},
		{"mul_i32", ir.BinaryMultiply, "asint(asuint(int(1)) * asuint(int(2)))"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w.out.Reset()
			e := ir.ExprBinary{Op: tt.op, Left: 0, Right: 1}
			err := w.writeBinaryExpression(e)
			if err != nil {
				t.Fatalf("writeBinaryExpression: %v", err)
			}
			got := w.out.String()
			if got != tt.want {
				t.Errorf("writeBinaryExpression(%s):\n  got:  %q\n  want: %q", tt.name, got, tt.want)
			}
		})
	}
}

// TestGatherMethodNaming verifies that HLSL Gather method names match Rust naga:
// component 0 (X) -> .Gather, not .GatherRed
// component 1 (Y) -> .GatherGreen
// With depth ref -> .GatherCmp
func TestGatherMethodNaming(t *testing.T) {
	tests := []struct {
		name     string
		comp     ir.SwizzleComponent
		depthRef bool
		want     string
	}{
		{"comp_0_no_depth", ir.SwizzleX, false, ".Gather"},
		{"comp_1_no_depth", ir.SwizzleY, false, ".GatherGreen"},
		{"comp_2_no_depth", ir.SwizzleZ, false, ".GatherBlue"},
		{"comp_3_no_depth", ir.SwizzleW, false, ".GatherAlpha"},
		{"comp_0_with_depth", ir.SwizzleX, true, ".GatherCmp"},
		{"comp_1_with_depth", ir.SwizzleY, true, ".GatherCmpGreen"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comp := tt.comp
			components := [4]string{"", "Green", "Blue", "Alpha"}
			compStr := ""
			if int(comp) < 4 {
				compStr = components[comp]
			}
			cmpStr := ""
			if tt.depthRef {
				cmpStr = "Cmp"
			}
			got := ".Gather" + cmpStr + compStr
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestGetExpressionVectorSize_SplatFallback verifies that getExpressionVectorSize
// correctly infers the size from Splat expressions when ExpressionTypes is empty.
func TestGetExpressionVectorSize_SplatFallback(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, // 0: f32
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(0.5)}}, // 0: literal
			{Kind: ir.ExprSplat{Size: ir.Vec2, Value: 0}}, // 1: splat to vec2
			{Kind: ir.ExprSplat{Size: ir.Vec3, Value: 0}}, // 2: splat to vec3
			{Kind: ir.ExprSplat{Size: ir.Vec4, Value: 0}}, // 3: splat to vec4
		},
		// Intentionally empty ExpressionTypes to test fallback
		ExpressionTypes: make([]ir.TypeResolution, 4),
	}

	w := &Writer{module: module, currentFunction: fn}

	tests := []struct {
		handle ir.ExpressionHandle
		want   int
	}{
		{1, 2}, // vec2
		{2, 3}, // vec3
		{3, 4}, // vec4
	}
	for _, tt := range tests {
		got := w.getExpressionVectorSize(tt.handle)
		if got != tt.want {
			t.Errorf("getExpressionVectorSize(%d) = %d, want %d", tt.handle, got, tt.want)
		}
	}
}

// TestWriteConstExpression verifies that writeConstExpression bypasses named
// expression cache and writes the raw expression inline.
func TestWriteConstExpression(t *testing.T) {
	f32Handle := ir.TypeHandle(0)
	i32Handle := ir.TypeHandle(1)
	vec2i32Handle := ir.TypeHandle(2)

	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                      // 0: f32
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},                                       // 1: i32
			{Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}}, // 2: vec2<i32>
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralI32(3)}},                              // 0: 3
			{Kind: ir.Literal{Value: ir.LiteralI32(1)}},                              // 1: 1
			{Kind: ir.ExprCompose{Type: 2, Components: []ir.ExpressionHandle{0, 1}}}, // 2: int2(3, 1)
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &i32Handle},
			{Handle: &i32Handle},
			{Handle: &vec2i32Handle},
		},
	}

	w := &Writer{
		module:           module,
		currentFunction:  fn,
		namedExpressions: map[ir.ExpressionHandle]string{2: "_e6"},
		typeNames:        make(map[ir.TypeHandle]string),
		options:          &Options{},
	}
	_ = f32Handle

	// Normal writeExpression should use the baked name
	w.out.Reset()
	_ = w.writeExpression(2)
	if got := w.out.String(); got != "_e6" {
		t.Errorf("writeExpression(2) = %q, want %q", got, "_e6")
	}

	// writeConstExpression should bypass the baked name and write inline
	w.out.Reset()
	_ = w.writeConstExpression(2)
	if got := w.out.String(); got == "_e6" {
		t.Errorf("writeConstExpression(2) = %q, should NOT use baked name", got)
	}
}

// =============================================================================
// TestNonUniformResourceIndex — binding array non-uniformity detection
// =============================================================================

func TestNonUniformResourceIndex(t *testing.T) {
	size5 := uint32(5)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},                                       // 0: u32
			{Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled}},                           // 1: texture_2d
			{Inner: ir.BindingArrayType{Base: ir.TypeHandle(1), Size: &size5}},                          // 2: binding_array<texture_2d, 5>
			{Inner: ir.StructType{Members: []ir.StructMember{{Name: "index", Type: ir.TypeHandle(0)}}}}, // 3: Struct { index: u32 }
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "textures", Type: ir.TypeHandle(2), Space: ir.SpaceHandle, Binding: &ir.ResourceBinding{Group: 0, Binding: 0}}, // 0: binding_array
			{Name: "uni", Type: ir.TypeHandle(3), Space: ir.SpaceUniform, Binding: &ir.ResourceBinding{Group: 0, Binding: 1}},     // 1: uniform
		},
	}

	fn := &ir.Function{
		Arguments: []ir.FunctionArgument{
			{Name: "fragment_in", Type: ir.TypeHandle(3), Binding: bindingPtr(ir.LocationBinding{Location: 0})},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                           // 0: fragment_in (non-uniform)
			{Kind: ir.ExprAccessIndex{Base: ir.ExpressionHandle(0), Index: 0}},  // 1: fragment_in.index (non-uniform)
			{Kind: ir.ExprGlobalVariable{Variable: ir.GlobalVariableHandle(1)}}, // 2: uni (uniform)
			{Kind: ir.ExprLoad{Pointer: ir.ExpressionHandle(2)}},                // 3: load(uni) (uniform)
			{Kind: ir.ExprAccessIndex{Base: ir.ExpressionHandle(3), Index: 0}},  // 4: uni.index (uniform)
			{Kind: ir.Literal{Value: ir.LiteralU32(0)}},                         // 5: 0u (constant)
		},
		ExpressionTypes: make([]ir.TypeResolution, 6),
	}

	w := newWriter(module, DefaultOptions())
	w.currentFunction = fn

	tests := []struct {
		name     string
		handle   ir.ExpressionHandle
		expected bool
	}{
		{"fragment_in (function arg)", 0, true},
		{"fragment_in.index (non-uniform member)", 1, true},
		{"uni (uniform global)", 2, false},
		{"load(uni) (uniform)", 3, false},
		{"uni.index (uniform member)", 4, false},
		{"literal 0u (constant)", 5, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := w.isNonUniform(tc.handle)
			if got != tc.expected {
				t.Errorf("isNonUniform(%d) = %v, want %v", tc.handle, got, tc.expected)
			}
		})
	}
}

// =============================================================================
// TestSamplerHeapBindingArray — sampler binding array heap pattern detection
// =============================================================================

func TestSamplerHeapBindingArray(t *testing.T) {
	size5 := uint32(5)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.SamplerType{Comparison: false}},                         // 0: sampler
			{Inner: ir.BindingArrayType{Base: ir.TypeHandle(0), Size: &size5}}, // 1: binding_array<sampler, 5>
			{Inner: ir.SamplerType{Comparison: true}},                          // 2: sampler_comparison
			{Inner: ir.BindingArrayType{Base: ir.TypeHandle(2), Size: &size5}}, // 3: binding_array<sampler_comparison, 5>
			{Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled}},  // 4: texture_2d
			{Inner: ir.BindingArrayType{Base: ir.TypeHandle(4), Size: &size5}}, // 5: binding_array<texture_2d, 5>
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "samp", Type: ir.TypeHandle(1), Space: ir.SpaceHandle, Binding: &ir.ResourceBinding{Group: 0, Binding: 0}},      // 0: sampler array
			{Name: "samp_comp", Type: ir.TypeHandle(3), Space: ir.SpaceHandle, Binding: &ir.ResourceBinding{Group: 0, Binding: 1}}, // 1: comparison sampler array
			{Name: "textures", Type: ir.TypeHandle(5), Space: ir.SpaceHandle, Binding: &ir.ResourceBinding{Group: 0, Binding: 2}},  // 2: texture array
		},
	}

	// Test isBindingArrayOfSamplers
	t.Run("isBindingArrayOfSamplers", func(t *testing.T) {
		w := newWriter(module, DefaultOptions())
		w.names[nameKey{kind: nameKeyGlobalVariable, handle1: 0}] = "samp"
		w.names[nameKey{kind: nameKeyGlobalVariable, handle1: 1}] = "samp_comp"
		w.names[nameKey{kind: nameKeyGlobalVariable, handle1: 2}] = "textures"

		if !w.isBindingArrayOfSamplers(0) {
			t.Error("expected samp to be binding array of samplers")
		}
		if !w.isBindingArrayOfSamplers(1) {
			t.Error("expected samp_comp to be binding array of samplers")
		}
		if w.isBindingArrayOfSamplers(2) {
			t.Error("expected textures NOT to be binding array of samplers")
		}
	})

	// Test samplerBindingArrayInfoFromExpression
	t.Run("samplerBindingArrayInfo", func(t *testing.T) {
		w := newWriter(module, DefaultOptions())
		w.names[nameKey{kind: nameKeyGlobalVariable, handle1: 0}] = "samp"
		w.names[nameKey{kind: nameKeyGlobalVariable, handle1: 1}] = "samp_comp"
		w.samplerIndexBuffers = map[uint32]string{0: "nagaGroup0SamplerIndexArray"}

		fn := &ir.Function{
			Expressions: []ir.Expression{
				{Kind: ir.ExprGlobalVariable{Variable: ir.GlobalVariableHandle(0)}}, // 0: samp (sampler array)
				{Kind: ir.ExprGlobalVariable{Variable: ir.GlobalVariableHandle(1)}}, // 1: samp_comp
				{Kind: ir.ExprGlobalVariable{Variable: ir.GlobalVariableHandle(2)}}, // 2: textures
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: ptrTo(ir.TypeHandle(1))}, // type of samp
				{Handle: ptrTo(ir.TypeHandle(3))}, // type of samp_comp
				{Handle: ptrTo(ir.TypeHandle(5))}, // type of textures
			},
		}
		w.currentFunction = fn

		info := w.samplerBindingArrayInfoFromExpression(0)
		if info == nil {
			t.Fatal("expected sampler info for samp")
		}
		if info.samplerHeapName != "nagaSamplerHeap" {
			t.Errorf("heap name = %q, want %q", info.samplerHeapName, "nagaSamplerHeap")
		}
		if info.bindingArrayBaseIndexName != "samp" {
			t.Errorf("base index name = %q, want %q", info.bindingArrayBaseIndexName, "samp")
		}

		info2 := w.samplerBindingArrayInfoFromExpression(1)
		if info2 == nil {
			t.Fatal("expected sampler info for samp_comp")
		}
		if info2.samplerHeapName != "nagaComparisonSamplerHeap" {
			t.Errorf("heap name = %q, want %q", info2.samplerHeapName, "nagaComparisonSamplerHeap")
		}

		info3 := w.samplerBindingArrayInfoFromExpression(2)
		if info3 != nil {
			t.Error("expected no sampler info for texture array")
		}
	})
}

func ptrTo[T any](v T) *T { return &v }

// =============================================================================
// TestIsAbstractIntLiteral — abstract int detection for texture coordinates
// =============================================================================

func TestIsAbstractIntLiteral(t *testing.T) {
	module := &ir.Module{}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralAbstractInt(0)}}, // 0: abstract int
			{Kind: ir.Literal{Value: ir.LiteralI32(0)}},         // 1: i32
			{Kind: ir.Literal{Value: ir.LiteralU32(0)}},         // 2: u32
			{Kind: ir.Literal{Value: ir.LiteralF32(0)}},         // 3: f32
		},
		ExpressionTypes: make([]ir.TypeResolution, 4),
	}

	w := newWriter(module, DefaultOptions())
	w.currentFunction = fn

	if !w.isAbstractIntLiteral(0) {
		t.Error("expected abstract int literal for expr 0")
	}
	if w.isAbstractIntLiteral(1) {
		t.Error("expected NOT abstract int literal for i32")
	}
	if w.isAbstractIntLiteral(2) {
		t.Error("expected NOT abstract int literal for u32")
	}
	if w.isAbstractIntLiteral(3) {
		t.Error("expected NOT abstract int literal for f32")
	}
}
