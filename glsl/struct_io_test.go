// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// Helper: build common types used across struct IO tests
// =============================================================================

// testTypes creates a common type set for struct IO tests.
// Returns the module types slice and named type indices.
//
// Type indices:
//
//	0: f32
//	1: vec2<f32>
//	2: vec4<f32>
//	3: VertexInput struct (9 location-bound members)
//	4: VertexOutput struct (position builtin + location-bound members)
func testTypesVertexIO() ([]ir.Type, map[string]ir.TypeHandle) {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}

	locBinding := func(loc uint32) *ir.Binding {
		b := ir.Binding(ir.LocationBinding{Location: loc})
		return &b
	}
	positionBinding := func() *ir.Binding {
		b := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
		return &b
	}

	types := []ir.Type{
		{Name: "", Inner: f32}, // 0: f32
		{Name: "", Inner: ir.VectorType{Size: ir.Vec2, Scalar: f32}}, // 1: vec2<f32>
		{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: f32}}, // 2: vec4<f32>
		{Name: "VertexInput", Inner: ir.StructType{ // 3: VertexInput
			Members: []ir.StructMember{
				{Name: "position", Type: 1, Binding: locBinding(0), Offset: 0},    // @location(0) position: vec2<f32>
				{Name: "local", Type: 1, Binding: locBinding(1), Offset: 8},       // @location(1) local: vec2<f32>
				{Name: "shape_kind", Type: 0, Binding: locBinding(2), Offset: 16}, // @location(2) shape_kind: f32
				{Name: "center", Type: 1, Binding: locBinding(3), Offset: 20},     // @location(3) center: vec2<f32>
				{Name: "size", Type: 1, Binding: locBinding(4), Offset: 28},       // @location(4) size: vec2<f32>
				{Name: "params", Type: 2, Binding: locBinding(5), Offset: 36},     // @location(5) params: vec4<f32>
				{Name: "stroke", Type: 1, Binding: locBinding(6), Offset: 52},     // @location(6) stroke: vec2<f32>
				{Name: "aa_width", Type: 0, Binding: locBinding(7), Offset: 60},   // @location(7) aa_width: f32
				{Name: "color", Type: 2, Binding: locBinding(8), Offset: 64},      // @location(8) color: vec4<f32>
			},
			Span: 80,
		}},
		{Name: "VertexOutput", Inner: ir.StructType{ // 4: VertexOutput
			Members: []ir.StructMember{
				{Name: "clip_position", Type: 2, Binding: positionBinding(), Offset: 0}, // @builtin(position) clip_position: vec4<f32>
				{Name: "local", Type: 1, Binding: locBinding(0), Offset: 16},            // @location(0) local: vec2<f32>
				{Name: "shape_kind", Type: 0, Binding: locBinding(1), Offset: 24},       // @location(1) shape_kind: f32
				{Name: "color", Type: 2, Binding: locBinding(2), Offset: 28},            // @location(2) color: vec4<f32>
			},
			Span: 44,
		}},
	}

	handles := map[string]ir.TypeHandle{
		"f32":          0,
		"vec2":         1,
		"vec4":         2,
		"VertexInput":  3,
		"VertexOutput": 4,
	}

	return types, handles
}

// =============================================================================
// Test: Vertex shader with struct input (VertexInput)
// =============================================================================

func TestCompile_StructVertexInput(t *testing.T) {
	types, handles := testTypesVertexIO()

	// Build a minimal vertex shader:
	//   @vertex fn vs_main(in: VertexInput) -> @builtin(position) vec4<f32> {
	//     return vec4<f32>(in.position.x, in.position.y, 0.0, 1.0);
	//   }
	posBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})

	module := &ir.Module{
		Types: types,
		Functions: []ir.Function{
			{
				Name: "vs_main",
				Arguments: []ir.FunctionArgument{
					{Name: "in", Type: handles["VertexInput"], Binding: nil}, // struct arg, no direct binding
				},
				Result: &ir.FunctionResult{
					Type:    handles["vec4"],
					Binding: &posBinding,
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 0}},                                                    // [0] = in
					{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},                                                // [1] = in.position (vec2)
					{Kind: ir.ExprSwizzle{Vector: 1, Size: 1, Pattern: [4]ir.SwizzleComponent{0}}},               // [2] = in.position.x
					{Kind: ir.ExprSwizzle{Vector: 1, Size: 1, Pattern: [4]ir.SwizzleComponent{1}}},               // [3] = in.position.y
					{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                                                // [4] = 0.0
					{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                                                // [5] = 1.0
					{Kind: ir.ExprCompose{Type: handles["vec4"], Components: []ir.ExpressionHandle{2, 3, 4, 5}}}, // [6] = vec4(...)
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 7}}},
					{Kind: ir.StmtReturn{Value: ptrExpr(6)}},
				},
			},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "vs_main", Stage: ir.StageVertex, Function: 0},
		},
	}

	source, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	t.Logf("Generated GLSL:\n%s", source)

	// Verify struct input is flattened into individual layout declarations
	mustContain(t, source, "layout(location = 0) in vec2 position;")
	mustContain(t, source, "layout(location = 1) in vec2 local;")
	mustContain(t, source, "layout(location = 2) in float shape_kind;")
	mustContain(t, source, "layout(location = 3) in vec2 center;")
	mustContain(t, source, "layout(location = 4) in vec2 size;")
	mustContain(t, source, "layout(location = 5) in vec4 params;")
	mustContain(t, source, "layout(location = 6) in vec2 stroke;")
	mustContain(t, source, "layout(location = 7) in float aa_width;")
	mustContain(t, source, "layout(location = 8) in vec4 color;")

	// Verify no _in[N] references (the bug)
	mustNotContain(t, source, "_in[")
	mustNotContain(t, source, "_in.")

	// Verify position member is accessed as the variable name directly
	mustContain(t, source, "position.x")

	// Verify gl_Position assignment
	mustContain(t, source, "gl_Position =")
}

// =============================================================================
// Test: Vertex shader with struct output (VertexOutput)
// =============================================================================

func TestCompile_StructVertexOutput(t *testing.T) {
	types, handles := testTypesVertexIO()

	// Build a vertex shader that returns a VertexOutput struct:
	//   @vertex fn vs_main(in: VertexInput) -> VertexOutput {
	//     var out: VertexOutput;
	//     out.clip_position = vec4<f32>(in.position, 0.0, 1.0);
	//     out.local = in.local;
	//     out.shape_kind = in.shape_kind;
	//     out.color = in.color;
	//     return out;
	//   }
	//
	// The IR for return typically uses ExprCompose to construct the struct.

	module := &ir.Module{
		Types: types,
		Functions: []ir.Function{
			{
				Name: "vs_main",
				Arguments: []ir.FunctionArgument{
					{Name: "in", Type: handles["VertexInput"], Binding: nil},
				},
				Result: &ir.FunctionResult{
					Type:    handles["VertexOutput"],
					Binding: nil, // struct result has no direct binding
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 0}},     // [0] = in
					{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // [1] = in.position (vec2)
					{Kind: ir.ExprAccessIndex{Base: 0, Index: 1}}, // [2] = in.local (vec2)
					{Kind: ir.ExprAccessIndex{Base: 0, Index: 2}}, // [3] = in.shape_kind (f32)
					{Kind: ir.ExprAccessIndex{Base: 0, Index: 8}}, // [4] = in.color (vec4)
					{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}}, // [5] = 0.0
					{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}}, // [6] = 1.0
					// clip_position = vec4(in.position, 0.0, 1.0)
					{Kind: ir.ExprCompose{Type: handles["vec4"], Components: []ir.ExpressionHandle{1, 5, 6}}}, // [7]
					// Compose the VertexOutput struct
					{Kind: ir.ExprCompose{Type: handles["VertexOutput"], Components: []ir.ExpressionHandle{7, 2, 3, 4}}}, // [8]
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 9}}},
					{Kind: ir.StmtReturn{Value: ptrExpr(8)}},
				},
			},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "vs_main", Stage: ir.StageVertex, Function: 0},
		},
	}

	source, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	t.Logf("Generated GLSL:\n%s", source)

	// Verify struct input is flattened
	mustContain(t, source, "layout(location = 0) in vec2 position;")
	mustContain(t, source, "layout(location = 1) in vec2 local;")

	// Verify struct output is flattened with v_ prefix (avoids name collision with inputs)
	mustContain(t, source, "layout(location = 0) out vec2 v_local;")
	mustContain(t, source, "layout(location = 1) out float v_shape_kind;")
	mustContain(t, source, "layout(location = 2) out vec4 v_color;")

	// Verify gl_Position is assigned (builtin member)
	mustContain(t, source, "gl_Position =")

	// Verify no _in[N] references
	mustNotContain(t, source, "_in[")
}

// =============================================================================
// Test: Fragment shader with struct input
// =============================================================================

func TestCompile_StructFragmentInput(t *testing.T) {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	locBinding := func(loc uint32) *ir.Binding {
		b := ir.Binding(ir.LocationBinding{Location: loc})
		return &b
	}

	types := []ir.Type{
		{Name: "", Inner: f32},
		{Name: "", Inner: ir.VectorType{Size: ir.Vec2, Scalar: f32}},
		{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: f32}},
		{Name: "FragInput", Inner: ir.StructType{
			Members: []ir.StructMember{
				{Name: "uv", Type: 1, Binding: locBinding(0), Offset: 0},
				{Name: "color", Type: 2, Binding: locBinding(1), Offset: 8},
			},
			Span: 24,
		}},
	}

	// Fragment shader: fn fs_main(in: FragInput) -> @location(0) vec4<f32> { return in.color; }
	outBinding := ir.Binding(ir.LocationBinding{Location: 0})
	module := &ir.Module{
		Types: types,
		Functions: []ir.Function{
			{
				Name: "fs_main",
				Arguments: []ir.FunctionArgument{
					{Name: "in", Type: 3, Binding: nil},
				},
				Result: &ir.FunctionResult{
					Type:    2, // vec4
					Binding: &outBinding,
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 0}},     // [0] = in
					{Kind: ir.ExprAccessIndex{Base: 0, Index: 1}}, // [1] = in.color (vec4)
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
					{Kind: ir.StmtReturn{Value: ptrExpr(1)}},
				},
			},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: 0},
		},
	}

	source, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	t.Logf("Generated GLSL:\n%s", source)

	// Verify struct input is flattened into individual in declarations
	mustContain(t, source, "layout(location = 0) in vec2 uv;")
	mustContain(t, source, "layout(location = 1) in vec4 color;")

	// Verify output
	mustContain(t, source, "layout(location = 0) out vec4 fragColor;")

	// Verify the return assigns to fragColor
	mustContain(t, source, "fragColor = color;")

	// No _in[N] references
	mustNotContain(t, source, "_in[")
}

// =============================================================================
// Test: Mixed struct and direct binding arguments
// =============================================================================

func TestCompile_MixedStructAndDirectArgs(t *testing.T) {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	locBinding := func(loc uint32) *ir.Binding {
		b := ir.Binding(ir.LocationBinding{Location: loc})
		return &b
	}
	vertexIDBinding := func() *ir.Binding {
		b := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinVertexIndex})
		return &b
	}
	posBinding := func() *ir.Binding {
		b := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
		return &b
	}

	types := []ir.Type{
		{Name: "", Inner: f32},
		{Name: "", Inner: ir.VectorType{Size: ir.Vec2, Scalar: f32}},
		{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: f32}},
		{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, // 3: u32
		{Name: "VertexData", Inner: ir.StructType{
			Members: []ir.StructMember{
				{Name: "pos", Type: 1, Binding: locBinding(0), Offset: 0},
				{Name: "uv", Type: 1, Binding: locBinding(1), Offset: 8},
			},
			Span: 16,
		}}, // 4: VertexData
	}

	// Vertex shader with mixed args:
	//   fn vs_main(data: VertexData, @builtin(vertex_index) vid: u32) -> @builtin(position) vec4<f32>
	posB := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
	module := &ir.Module{
		Types: types,
		Functions: []ir.Function{
			{
				Name: "vs_main",
				Arguments: []ir.FunctionArgument{
					{Name: "data", Type: 4, Binding: nil},              // struct arg
					{Name: "vid", Type: 3, Binding: vertexIDBinding()}, // builtin arg
				},
				Result: &ir.FunctionResult{
					Type:    2,
					Binding: &posB,
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 0}},                                   // [0] = data
					{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},                               // [1] = data.pos (vec2)
					{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                               // [2] = 0.0
					{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                               // [3] = 1.0
					{Kind: ir.ExprCompose{Type: 2, Components: []ir.ExpressionHandle{1, 2, 3}}}, // [4] = vec4(pos, 0.0, 1.0)
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 5}}},
					{Kind: ir.StmtReturn{Value: ptrExpr(4)}},
				},
			},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "vs_main", Stage: ir.StageVertex, Function: 0},
		},
	}

	_ = posBinding() // use the helper to avoid unused warning

	source, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	t.Logf("Generated GLSL:\n%s", source)

	// Struct arg should be flattened
	mustContain(t, source, "layout(location = 0) in vec2 pos;")
	mustContain(t, source, "layout(location = 1) in vec2 uv;")

	// Builtin arg (vertex_index) should NOT produce a layout(location) in declaration.
	// There should be exactly TWO layout(location) lines: one for pos and one for uv.
	count := strings.Count(source, "layout(location")
	if count != 2 {
		t.Errorf("Expected exactly 2 layout(location) declarations, got %d", count)
	}

	// The builtin vertex_index arg should use gl_VertexID natively, not a variable.
	mustNotContain(t, source, "in uint vid")
}

// =============================================================================
// Test: Vertex shader with struct input containing builtin member
// =============================================================================

func TestCompile_StructInputWithBuiltin(t *testing.T) {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	locBinding := func(loc uint32) *ir.Binding {
		b := ir.Binding(ir.LocationBinding{Location: loc})
		return &b
	}
	vertexIDBinding := func() *ir.Binding {
		b := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinVertexIndex})
		return &b
	}
	posBinding := func() *ir.Binding {
		b := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
		return &b
	}

	types := []ir.Type{
		{Name: "", Inner: f32},
		{Name: "", Inner: ir.VectorType{Size: ir.Vec2, Scalar: f32}},
		{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: f32}},
		{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, // 3: u32
		{Name: "VertexInput", Inner: ir.StructType{
			Members: []ir.StructMember{
				{Name: "position", Type: 1, Binding: locBinding(0), Offset: 0},
				{Name: "vertex_index", Type: 3, Binding: vertexIDBinding(), Offset: 8},
			},
			Span: 12,
		}}, // 4: VertexInput with builtin member
	}

	posB := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
	module := &ir.Module{
		Types: types,
		Functions: []ir.Function{
			{
				Name: "vs_main",
				Arguments: []ir.FunctionArgument{
					{Name: "in", Type: 4, Binding: nil},
				},
				Result: &ir.FunctionResult{
					Type:    2,
					Binding: &posB,
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 0}},                                   // [0] = in
					{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},                               // [1] = in.position (vec2)
					{Kind: ir.ExprAccessIndex{Base: 0, Index: 1}},                               // [2] = in.vertex_index -> gl_VertexID
					{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                               // [3] = 0.0
					{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                               // [4] = 1.0
					{Kind: ir.ExprCompose{Type: 2, Components: []ir.ExpressionHandle{1, 3, 4}}}, // [5] = vec4(...)
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 6}}},
					{Kind: ir.StmtReturn{Value: ptrExpr(5)}},
				},
			},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "vs_main", Stage: ir.StageVertex, Function: 0},
		},
	}

	_ = posBinding() // use the helper

	source, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	t.Logf("Generated GLSL:\n%s", source)

	// Location-bound member should produce in declaration
	mustContain(t, source, "layout(location = 0) in vec2 position;")

	// Builtin member should NOT produce a layout(location) in declaration.
	// There should be exactly ONE "layout(location" line (for position).
	count := strings.Count(source, "layout(location")
	if count != 1 {
		t.Errorf("Expected exactly 1 layout(location) declaration (for position), got %d", count)
	}

	// The builtin member (vertex_index) should NOT generate "in uint vertex_index"
	mustNotContain(t, source, "in uint vertex_index")
}

// =============================================================================
// Test: Fragment shader with struct output
// =============================================================================

func TestCompile_StructFragmentOutput(t *testing.T) {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	locBinding := func(loc uint32) *ir.Binding {
		b := ir.Binding(ir.LocationBinding{Location: loc})
		return &b
	}

	types := []ir.Type{
		{Name: "", Inner: f32},
		{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: f32}},
		{Name: "FragOutput", Inner: ir.StructType{
			Members: []ir.StructMember{
				{Name: "color", Type: 1, Binding: locBinding(0), Offset: 0},
				{Name: "bloom", Type: 1, Binding: locBinding(1), Offset: 16},
			},
			Span: 32,
		}}, // 2: FragOutput
	}

	// Fragment shader with struct output:
	//   fn fs_main() -> FragOutput {
	//     return FragOutput(vec4(1.0), vec4(0.5));
	//   }
	module := &ir.Module{
		Types: types,
		Functions: []ir.Function{
			{
				Name:      "fs_main",
				Arguments: nil,
				Result: &ir.FunctionResult{
					Type:    2, // FragOutput
					Binding: nil,
				},
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                                  // [0]
					{Kind: ir.ExprCompose{Type: 1, Components: []ir.ExpressionHandle{0, 0, 0, 0}}}, // [1] = vec4(1.0,1.0,1.0,1.0)
					{Kind: ir.Literal{Value: ir.LiteralF32(0.5)}},                                  // [2]
					{Kind: ir.ExprCompose{Type: 1, Components: []ir.ExpressionHandle{2, 2, 2, 2}}}, // [3] = vec4(0.5,...)
					{Kind: ir.ExprCompose{Type: 2, Components: []ir.ExpressionHandle{1, 3}}},       // [4] = FragOutput(color, bloom)
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 5}}},
					{Kind: ir.StmtReturn{Value: ptrExpr(4)}},
				},
			},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: 0},
		},
	}

	source, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	t.Logf("Generated GLSL:\n%s", source)

	// Verify struct output is flattened into individual out declarations
	mustContain(t, source, "layout(location = 0) out vec4 color;")
	mustContain(t, source, "layout(location = 1) out vec4 bloom;")

	// Verify return is expanded into individual assignments
	mustContain(t, source, "color =")
	mustContain(t, source, "bloom =")
}

// =============================================================================
// Test: Backward compatibility — direct binding args still work
// =============================================================================

func TestCompile_DirectBindingArgsStillWork(t *testing.T) {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}

	locBinding := func(loc uint32) *ir.Binding {
		b := ir.Binding(ir.LocationBinding{Location: loc})
		return &b
	}
	posBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})

	types := []ir.Type{
		{Name: "", Inner: f32},
		{Name: "", Inner: ir.VectorType{Size: ir.Vec2, Scalar: f32}},
		{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: f32}},
	}

	// Simple vertex shader with direct-binding args (no structs):
	//   fn vs_main(@location(0) pos: vec2<f32>) -> @builtin(position) vec4<f32>
	module := &ir.Module{
		Types: types,
		Functions: []ir.Function{
			{
				Name: "vs_main",
				Arguments: []ir.FunctionArgument{
					{Name: "pos", Type: 1, Binding: locBinding(0)},
				},
				Result: &ir.FunctionResult{
					Type:    2,
					Binding: &posBinding,
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 0}},                                   // [0] = pos
					{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                               // [1] = 0.0
					{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                               // [2] = 1.0
					{Kind: ir.ExprCompose{Type: 2, Components: []ir.ExpressionHandle{0, 1, 2}}}, // [3] = vec4(pos, 0, 1)
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
					{Kind: ir.StmtReturn{Value: ptrExpr(3)}},
				},
			},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "vs_main", Stage: ir.StageVertex, Function: 0},
		},
	}

	source, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	t.Logf("Generated GLSL:\n%s", source)

	// Direct binding should still work as before
	mustContain(t, source, "layout(location = 0) in vec2 pos;")
	mustContain(t, source, "gl_Position =")
}

// =============================================================================
// Helpers
// =============================================================================

// ptrExpr returns a pointer to an ExpressionHandle (for StmtReturn.Value).
func ptrExpr(h ir.ExpressionHandle) *ir.ExpressionHandle {
	return &h
}

// mustContain asserts that the source contains the expected substring.
func mustContain(t *testing.T, source, expected string) {
	t.Helper()
	if !strings.Contains(source, expected) {
		t.Errorf("Expected source to contain %q, but it was not found.\nSource:\n%s", expected, source)
	}
}

// mustNotContain asserts that the source does NOT contain the given substring.
func mustNotContain(t *testing.T, source, forbidden string) {
	t.Helper()
	if strings.Contains(source, forbidden) {
		t.Errorf("Source should NOT contain %q, but it was found.\nSource:\n%s", forbidden, source)
	}
}
