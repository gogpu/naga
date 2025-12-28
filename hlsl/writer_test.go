// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

func TestWriter_Indentation(t *testing.T) {
	module := &ir.Module{}
	opts := DefaultOptions()
	w := newWriter(module, opts)

	// Initial state
	if w.indent != 0 {
		t.Errorf("initial indent = %d, want 0", w.indent)
	}

	// Push indent
	w.pushIndent()
	if w.indent != 1 {
		t.Errorf("after pushIndent, indent = %d, want 1", w.indent)
	}

	// Push again
	w.pushIndent()
	if w.indent != 2 {
		t.Errorf("after second pushIndent, indent = %d, want 2", w.indent)
	}

	// Pop indent
	w.popIndent()
	if w.indent != 1 {
		t.Errorf("after popIndent, indent = %d, want 1", w.indent)
	}

	// Pop to zero
	w.popIndent()
	if w.indent != 0 {
		t.Errorf("after second popIndent, indent = %d, want 0", w.indent)
	}

	// Pop below zero should stay at 0
	w.popIndent()
	if w.indent != 0 {
		t.Errorf("popIndent below zero should stay at 0, got %d", w.indent)
	}
}

func TestWriter_WriteLine(t *testing.T) {
	module := &ir.Module{}
	opts := DefaultOptions()
	w := newWriter(module, opts)

	w.writeLine("test line")
	w.writeLine("second line")

	output := w.String()
	lines := strings.Split(output, "\n")

	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}

	if lines[0] != "test line" {
		t.Errorf("first line = %q, want \"test line\"", lines[0])
	}
	if lines[1] != "second line" {
		t.Errorf("second line = %q, want \"second line\"", lines[1])
	}
}

func TestWriter_WriteLineWithFormat(t *testing.T) {
	module := &ir.Module{}
	opts := DefaultOptions()
	w := newWriter(module, opts)

	w.writeLine("value = %d", 42)
	w.writeLine("name = %s", "test")

	output := w.String()

	if !strings.Contains(output, "value = 42") {
		t.Error("expected output to contain \"value = 42\"")
	}
	if !strings.Contains(output, "name = test") {
		t.Error("expected output to contain \"name = test\"")
	}
}

func TestWriter_IndentedOutput(t *testing.T) {
	module := &ir.Module{}
	opts := DefaultOptions()
	w := newWriter(module, opts)

	w.writeLine("level 0")
	w.pushIndent()
	w.writeLine("level 1")
	w.pushIndent()
	w.writeLine("level 2")
	w.popIndent()
	w.writeLine("back to 1")
	w.popIndent()
	w.writeLine("back to 0")

	output := w.String()
	lines := strings.Split(output, "\n")

	// Check indentation (4 spaces per level)
	expectations := []struct {
		lineNum int
		prefix  string
	}{
		{0, "level 0"},
		{1, "    level 1"},
		{2, "        level 2"},
		{3, "    back to 1"},
		{4, "back to 0"},
	}

	for _, exp := range expectations {
		if exp.lineNum >= len(lines) {
			t.Errorf("line %d not found", exp.lineNum)
			continue
		}
		if lines[exp.lineNum] != exp.prefix {
			t.Errorf("line %d = %q, want %q", exp.lineNum, lines[exp.lineNum], exp.prefix)
		}
	}
}

func TestWriter_GetTypeName_Scalar(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
		},
	}
	opts := DefaultOptions()
	w := newWriter(module, opts)

	tests := []struct {
		handle ir.TypeHandle
		want   string
	}{
		{0, "float"},
		{1, "int"},
		{2, "uint"},
		{3, "bool"},
	}

	for _, tt := range tests {
		got := w.getTypeName(tt.handle)
		if got != tt.want {
			t.Errorf("getTypeName(%d) = %q, want %q", tt.handle, got, tt.want)
		}
	}
}

func TestWriter_GetTypeName_Vector(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Inner: ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}},
		},
	}
	opts := DefaultOptions()
	w := newWriter(module, opts)

	tests := []struct {
		handle ir.TypeHandle
		want   string
	}{
		{0, "float2"},
		{1, "float3"},
		{2, "float4"},
		{3, "int4"},
	}

	for _, tt := range tests {
		got := w.getTypeName(tt.handle)
		if got != tt.want {
			t.Errorf("getTypeName(%d) = %q, want %q", tt.handle, got, tt.want)
		}
	}
}

func TestWriter_GetTypeName_Matrix(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.MatrixType{
				Columns: ir.Vec4,
				Rows:    ir.Vec4,
				Scalar:  ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
			}},
			{Inner: ir.MatrixType{
				Columns: ir.Vec3,
				Rows:    ir.Vec3,
				Scalar:  ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
			}},
		},
	}
	opts := DefaultOptions()
	w := newWriter(module, opts)

	tests := []struct {
		handle ir.TypeHandle
		want   string
	}{
		{0, "float4x4"},
		{1, "float3x3"},
	}

	for _, tt := range tests {
		got := w.getTypeName(tt.handle)
		if got != tt.want {
			t.Errorf("getTypeName(%d) = %q, want %q", tt.handle, got, tt.want)
		}
	}
}

func TestWriter_GetTypeName_Struct(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{
				Name: "MyStruct",
				Inner: ir.StructType{
					Members: []ir.StructMember{
						{Name: "position", Type: 0},
					},
				},
			},
		},
	}
	opts := DefaultOptions()
	w := newWriter(module, opts)

	got := w.getTypeName(0)
	if got != "MyStruct" {
		t.Errorf("getTypeName(0) = %q, want \"MyStruct\"", got)
	}
}

func TestWriter_GetTypeName_InvalidHandle(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	opts := DefaultOptions()
	w := newWriter(module, opts)

	// Invalid handle
	got := w.getTypeName(999)
	if !strings.Contains(got, "999") {
		t.Errorf("invalid handle should contain handle number, got %q", got)
	}
}

func TestWriter_WriteModule_Header(t *testing.T) {
	module := &ir.Module{}
	opts := DefaultOptions()
	opts.ShaderModel = ShaderModel6_5

	w := newWriter(module, opts)
	err := w.writeModule()
	if err != nil {
		t.Fatalf("writeModule() error = %v", err)
	}

	output := w.String()

	// Check header
	if !strings.Contains(output, "Generated by naga") {
		t.Error("expected header comment")
	}
	if !strings.Contains(output, "SM 6.5") {
		t.Error("expected shader model in header")
	}
}

func TestWriter_WriteTypes_Struct(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{
				Name: "VertexInput",
				Inner: ir.StructType{
					Members: []ir.StructMember{
						{Name: "position", Type: 0},
						{Name: "normal", Type: 0},
					},
				},
			},
		},
	}
	opts := DefaultOptions()

	w := newWriter(module, opts)
	err := w.writeModule()
	if err != nil {
		t.Fatalf("writeModule() error = %v", err)
	}

	output := w.String()

	// Check struct definition
	if !strings.Contains(output, "struct VertexInput") {
		t.Error("expected struct definition")
	}
	if !strings.Contains(output, "position") {
		t.Error("expected position member")
	}
	if !strings.Contains(output, "normal") {
		t.Error("expected normal member")
	}
}

func TestWriter_WriteConstants(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		Constants: []ir.Constant{
			{
				Name:  "PI",
				Type:  0,
				Value: ir.ScalarValue{Bits: 0x40490fdb, Kind: ir.ScalarFloat},
			},
			{
				Name:  "MAX_LIGHTS",
				Type:  1,
				Value: ir.ScalarValue{Bits: 8, Kind: ir.ScalarSint},
			},
		},
	}
	opts := DefaultOptions()

	w := newWriter(module, opts)
	err := w.writeModule()
	if err != nil {
		t.Fatalf("writeModule() error = %v", err)
	}

	output := w.String()

	if !strings.Contains(output, "static const") {
		t.Error("expected static const declaration")
	}
	if !strings.Contains(output, "PI") {
		t.Error("expected PI constant")
	}
	if !strings.Contains(output, "MAX_LIGHTS") {
		t.Error("expected MAX_LIGHTS constant")
	}
}

func TestWriter_ScalarValue(t *testing.T) {
	module := &ir.Module{}
	opts := DefaultOptions()
	w := newWriter(module, opts)

	tests := []struct {
		name  string
		value ir.ScalarValue
		want  string
	}{
		{"bool true", ir.ScalarValue{Bits: 1, Kind: ir.ScalarBool}, "true"},
		{"bool false", ir.ScalarValue{Bits: 0, Kind: ir.ScalarBool}, "false"},
		{"int positive", ir.ScalarValue{Bits: 42, Kind: ir.ScalarSint}, "42"},
		{"int negative", ir.ScalarValue{Bits: 0xFFFFFFFF, Kind: ir.ScalarSint}, "-1"},
		{"uint", ir.ScalarValue{Bits: 100, Kind: ir.ScalarUint}, "100u"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.writeScalarValue(tt.value)
			if got != tt.want {
				t.Errorf("writeScalarValue() = %q, want %q", got, tt.want)
			}
		})
	}
}
