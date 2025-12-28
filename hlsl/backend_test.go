// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl

import (
	"errors"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts == nil {
		t.Fatal("DefaultOptions() returned nil")
	}

	if opts.ShaderModel != ShaderModel5_1 {
		t.Errorf("ShaderModel = %v, want ShaderModel5_1", opts.ShaderModel)
	}

	if !opts.FakeMissingBindings {
		t.Error("FakeMissingBindings should be true by default")
	}

	if !opts.ZeroInitializeWorkgroupMemory {
		t.Error("ZeroInitializeWorkgroupMemory should be true by default")
	}

	if opts.RestrictIndexing {
		t.Error("RestrictIndexing should be false by default")
	}

	if opts.ForceLoopBounding {
		t.Error("ForceLoopBounding should be false by default")
	}

	if opts.BindingMap == nil {
		t.Error("BindingMap should not be nil")
	}
}

func TestFeatureFlags_Has(t *testing.T) {
	tests := []struct {
		name   string
		flags  FeatureFlags
		check  FeatureFlags
		expect bool
	}{
		{"none has none", FeatureNone, FeatureNone, false},
		{"wave has wave", FeatureWaveOps, FeatureWaveOps, true},
		{"wave has none", FeatureWaveOps, FeatureNone, false},
		{"combined has wave", FeatureWaveOps | FeatureRayTracing, FeatureWaveOps, true},
		{"combined has ray", FeatureWaveOps | FeatureRayTracing, FeatureRayTracing, true},
		{"combined no mesh", FeatureWaveOps | FeatureRayTracing, FeatureMeshShaders, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.flags.Has(tt.check)
			if got != tt.expect {
				t.Errorf("Has() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestFeatureFlags_String(t *testing.T) {
	tests := []struct {
		name  string
		flags FeatureFlags
		want  string
	}{
		{"none", FeatureNone, "none"},
		{"wave ops", FeatureWaveOps, "WaveOps"},
		{"ray tracing", FeatureRayTracing, "RayTracing"},
		{"mesh shaders", FeatureMeshShaders, "MeshShaders"},
		{"64-bit ints", Feature64BitIntegers, "64BitIntegers"},
		{"combined", FeatureWaveOps | FeatureRayTracing, "WaveOps, RayTracing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.flags.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCompile_NilModule(t *testing.T) {
	_, _, err := Compile(nil, nil)
	if err == nil {
		t.Error("expected error for nil module")
		return
	}

	var hlslErr *Error
	if !errors.As(err, &hlslErr) {
		t.Errorf("expected *Error, got %T", err)
		return
	}

	if hlslErr.Kind != ErrInternalError {
		t.Errorf("error kind = %v, want ErrInternalError", hlslErr.Kind)
	}
}

func TestCompile_EmptyModule(t *testing.T) {
	module := &ir.Module{}

	code, info, err := Compile(module, nil)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if code == "" {
		t.Error("expected non-empty output")
	}

	if info == nil {
		t.Fatal("expected non-nil TranslationInfo")
	}

	if info.RequiredShaderModel != ShaderModel5_1 {
		t.Errorf("RequiredShaderModel = %v, want ShaderModel5_1", info.RequiredShaderModel)
	}
}

func TestCompile_SimpleModule(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{
				Name: "float",
				Inner: ir.ScalarType{
					Kind:  ir.ScalarFloat,
					Width: 4,
				},
			},
		},
		Constants: []ir.Constant{
			{
				Name: "PI",
				Type: 0,
				Value: ir.ScalarValue{
					Bits: 0x40490fdb, // ~3.14159
					Kind: ir.ScalarFloat,
				},
			},
		},
	}

	opts := DefaultOptions()
	opts.ShaderModel = ShaderModel6_0

	code, info, err := Compile(module, opts)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if code == "" {
		t.Error("expected non-empty output")
	}

	// Check that output contains expected content
	if !containsSubstring(code, "SM 6.0") {
		t.Error("expected output to contain shader model comment")
	}

	if !containsSubstring(code, "static const") {
		t.Error("expected output to contain constant declaration")
	}

	if info == nil {
		t.Fatal("expected non-nil TranslationInfo")
	}
}

func TestCompile_WithEntryPoint(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{
				Name:  "void",
				Inner: nil, // void has no inner type
			},
		},
		Functions: []ir.Function{
			{
				Name: "compute_main",
			},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:      "main",
				Stage:     ir.StageCompute,
				Function:  0,
				Workgroup: [3]uint32{64, 1, 1},
			},
		},
	}

	opts := DefaultOptions()
	code, info, err := Compile(module, opts)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	// Check for numthreads attribute
	if !containsSubstring(code, "[numthreads(64, 1, 1)]") {
		t.Error("expected output to contain numthreads attribute")
	}

	if info.EntryPointNames == nil {
		t.Error("expected non-nil EntryPointNames")
	}
}

func TestCompile_BindingMap(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{
				Name: "float4",
				Inner: ir.VectorType{
					Size: ir.Vec4,
					Scalar: ir.ScalarType{
						Kind:  ir.ScalarFloat,
						Width: 4,
					},
				},
			},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:  "uniforms",
				Space: ir.SpaceUniform,
				Type:  0,
				Binding: &ir.ResourceBinding{
					Group:   0,
					Binding: 0,
				},
			},
		},
	}

	opts := DefaultOptions()
	opts.BindingMap = map[ResourceBinding]BindTarget{
		{Group: 0, Binding: 0}: {Space: 1, Register: 5},
	}

	code, info, err := Compile(module, opts)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	// Check that binding map was used
	if !containsSubstring(code, "register(b5, space1)") {
		t.Error("expected output to use custom binding")
	}

	if info.RegisterBindings == nil {
		t.Error("expected non-nil RegisterBindings")
	}
}

// containsSubstring checks if s contains substr.
func containsSubstring(s, substr string) bool {
	return strings.Contains(s, substr)
}
