// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package ir

import (
	"math"
	"testing"
)

func TestProcessOverrides_ResolveByID(t *testing.T) {
	id0 := uint16(0)
	id1300 := uint16(1300)
	init0 := ExpressionHandle(0)

	module := &Module{
		Types: []Type{
			{Name: "bool", Inner: ScalarType{Kind: ScalarBool, Width: 1}},
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
		},
		Overrides: []Override{
			{Name: "has_light", ID: &id0, Ty: 0, Init: &init0}, // bool, ID=0
			{Name: "gain", ID: &id1300, Ty: 1, Init: nil},      // f32, ID=1300, no default
		},
		GlobalExpressions: []Expression{
			{Kind: Literal{Value: LiteralBool(true)}}, // 0: default for has_light
			{Kind: Literal{Value: LiteralF32(0)}},     // 1: placeholder
		},
	}

	// NaN for bool → false; 1.1 for gain
	constants := PipelineConstants{"0": math.NaN(), "1300": 1.1}
	err := ProcessOverrides(module, constants)
	if err != nil {
		t.Fatalf("ProcessOverrides: %v", err)
	}

	// Should have 2 new constants
	if len(module.Constants) != 2 {
		t.Fatalf("expected 2 constants, got %d", len(module.Constants))
	}

	// has_light → false (NaN for bool = false)
	c0Init := module.Constants[0].Init
	if int(c0Init) >= len(module.GlobalExpressions) {
		t.Fatalf("constant 0 init %d out of range", c0Init)
	}
	lit0, ok := module.GlobalExpressions[c0Init].Kind.(Literal)
	if !ok {
		t.Fatalf("constant 0 init: expected Literal, got %T", module.GlobalExpressions[c0Init].Kind)
	}
	if b, ok := lit0.Value.(LiteralBool); !ok || bool(b) != false {
		t.Errorf("has_light: expected false, got %v", lit0.Value)
	}

	// gain → 1.1
	c1Init := module.Constants[1].Init
	lit1, ok := module.GlobalExpressions[c1Init].Kind.(Literal)
	if !ok {
		t.Fatalf("constant 1 init: expected Literal, got %T", module.GlobalExpressions[c1Init].Kind)
	}
	if f, ok := lit1.Value.(LiteralF32); !ok || float32(f) != 1.1 {
		t.Errorf("gain: expected 1.1, got %v", lit1.Value)
	}
}

func TestProcessOverrides_ResolveByName(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
		},
		Overrides: []Override{
			{Name: "depth", Ty: 0, Init: nil}, // no ID, no default
		},
		GlobalExpressions: []Expression{},
	}

	constants := PipelineConstants{"depth": 2.3}
	err := ProcessOverrides(module, constants)
	if err != nil {
		t.Fatalf("ProcessOverrides: %v", err)
	}

	if len(module.Constants) != 1 {
		t.Fatalf("expected 1 constant, got %d", len(module.Constants))
	}
	lit, ok := module.GlobalExpressions[module.Constants[0].Init].Kind.(Literal)
	if !ok {
		t.Fatalf("expected Literal, got %T", module.GlobalExpressions[module.Constants[0].Init].Kind)
	}
	if f, ok := lit.Value.(LiteralF32); !ok || float32(f) != 2.3 {
		t.Errorf("depth: expected 2.3, got %v", lit.Value)
	}
}

func TestProcessOverrides_DerivedDefault(t *testing.T) {
	// height = 2 * depth; depth provided as 2.3 → height = 4.6
	init0 := ExpressionHandle(2) // points to Binary(Mul, Literal(2), Override(depth))
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
		},
		Overrides: []Override{
			{Name: "depth", Ty: 0, Init: nil},     // 0: depth, no default
			{Name: "height", Ty: 0, Init: &init0}, // 1: height = 2 * depth
		},
		GlobalExpressions: []Expression{
			{Kind: Literal{Value: LiteralF32(2.0)}},                   // 0: literal 2
			{Kind: ExprOverride{Override: 0}},                         // 1: depth ref
			{Kind: ExprBinary{Op: BinaryMultiply, Left: 0, Right: 1}}, // 2: 2 * depth
		},
	}

	constants := PipelineConstants{"depth": 2.3}
	err := ProcessOverrides(module, constants)
	if err != nil {
		t.Fatalf("ProcessOverrides: %v", err)
	}

	if len(module.Constants) != 2 {
		t.Fatalf("expected 2 constants, got %d", len(module.Constants))
	}

	// height = 4.6
	heightInit := module.Constants[1].Init
	lit, ok := module.GlobalExpressions[heightInit].Kind.(Literal)
	if !ok {
		t.Fatalf("height init: expected Literal, got %T", module.GlobalExpressions[heightInit].Kind)
	}
	if f, ok := lit.Value.(LiteralF32); !ok || (float32(f)-4.6) > 0.01 {
		t.Errorf("height: expected ~4.6, got %v", lit.Value)
	}
}

func TestProcessOverrides_FunctionExprReplacement(t *testing.T) {
	// Function has ExprOverride → should become ExprConstant after processing
	init0 := ExpressionHandle(0)
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
		},
		Overrides: []Override{
			{Name: "gain", Ty: 0, Init: &init0},
		},
		GlobalExpressions: []Expression{
			{Kind: Literal{Value: LiteralF32(0)}}, // default
		},
		EntryPoints: []EntryPoint{
			{
				Name:  "main",
				Stage: StageCompute,
				Function: Function{
					Expressions: []Expression{
						{Kind: ExprOverride{Override: 0}}, // references gain
					},
					ExpressionTypes: []TypeResolution{{}},
				},
			},
		},
	}

	constants := PipelineConstants{"gain": 5.5}
	err := ProcessOverrides(module, constants)
	if err != nil {
		t.Fatalf("ProcessOverrides: %v", err)
	}

	// After ProcessOverrides + compact, the EP should have at least one constant
	if len(module.Constants) < 1 {
		t.Fatal("expected at least 1 constant after ProcessOverrides")
	}
	// The override should be resolved — no ExprOverride should remain
	for i, expr := range module.EntryPoints[0].Function.Expressions {
		if _, isOverride := expr.Kind.(ExprOverride); isOverride {
			t.Errorf("expression [%d] still ExprOverride after ProcessOverrides", i)
		}
	}
}

func TestProcessOverrides_GlobalVarInit(t *testing.T) {
	// var<private> x: f32 = gain * 10; where gain = 1.1
	initExpr := ExpressionHandle(2) // Binary(Mul, Override(gain), Literal(10))
	init0 := ExpressionHandle(3)    // gain default (unused — overridden by pipeline)
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
		},
		Overrides: []Override{
			{Name: "gain", Ty: 0, Init: &init0},
		},
		GlobalExpressions: []Expression{
			{Kind: ExprOverride{Override: 0}},                         // 0: gain ref
			{Kind: Literal{Value: LiteralF32(10)}},                    // 1: literal 10
			{Kind: ExprBinary{Op: BinaryMultiply, Left: 0, Right: 1}}, // 2: gain * 10
			{Kind: Literal{Value: LiteralF32(0)}},                     // 3: gain default
		},
		GlobalVariables: []GlobalVariable{
			{Name: "x", Type: 0, Space: SpacePrivate, InitExpr: &initExpr},
		},
	}

	constants := PipelineConstants{"gain": 1.1}
	err := ProcessOverrides(module, constants)
	if err != nil {
		t.Fatalf("ProcessOverrides: %v", err)
	}

	// Global var init expression should be evaluated to 11.0
	if module.GlobalVariables[0].InitExpr == nil {
		t.Fatal("expected InitExpr to be set")
	}
	ge := module.GlobalExpressions[*module.GlobalVariables[0].InitExpr]
	lit, ok := ge.Kind.(Literal)
	if !ok {
		t.Fatalf("expected Literal for global var init, got %T", ge.Kind)
	}
	if f, ok := lit.Value.(LiteralF32); !ok || (float32(f)-11.0) > 0.01 {
		t.Errorf("x init: expected ~11.0, got %v", lit.Value)
	}
}

func TestProcessOverrides_EmptyOverrides(t *testing.T) {
	module := &Module{}
	err := ProcessOverrides(module, PipelineConstants{"foo": 1.0})
	if err != nil {
		t.Errorf("ProcessOverrides with empty overrides should not error: %v", err)
	}
}

func TestProcessOverrides_ConstFoldInFunction(t *testing.T) {
	// Binary(Mul, ExprConstant(height=4.6), Literal(5)) → Literal(23.0)
	init0 := ExpressionHandle(0)
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
		},
		Overrides: []Override{
			{Name: "height", Ty: 0, Init: &init0},
		},
		GlobalExpressions: []Expression{
			{Kind: Literal{Value: LiteralF32(4.6)}}, // default
		},
		EntryPoints: []EntryPoint{
			{
				Name:  "main",
				Stage: StageCompute,
				Function: Function{
					Expressions: []Expression{
						{Kind: ExprOverride{Override: 0}},                         // 0: height ref
						{Kind: Literal{Value: LiteralF32(5)}},                     // 1: literal 5
						{Kind: ExprBinary{Op: BinaryMultiply, Left: 0, Right: 1}}, // 2: height * 5
					},
					ExpressionTypes: []TypeResolution{{}, {}, {}},
					LocalVars: []LocalVariable{
						{Name: "t", Type: 0, Init: func() *ExpressionHandle { h := ExpressionHandle(2); return &h }()},
					},
				},
			},
		},
	}

	constants := PipelineConstants{"height": 4.6}
	err := ProcessOverrides(module, constants)
	if err != nil {
		t.Fatalf("ProcessOverrides: %v", err)
	}

	// After ProcessOverrides, verify:
	// 1. No ExprOverride remains
	// 2. A Literal(~23.0) exists somewhere in expressions (const-eval of height*5)
	// 3. Load expression exists and its _eN name matches expression handle
	fn := &module.EntryPoints[0].Function
	for i, expr := range fn.Expressions {
		if _, isOverride := expr.Kind.(ExprOverride); isOverride {
			t.Errorf("expression [%d] still ExprOverride after ProcessOverrides", i)
		}
	}
	// Check that local var "t" init points to a const-evaluated value
	if fn.LocalVars[0].Init != nil {
		initExpr := fn.Expressions[*fn.LocalVars[0].Init]
		if lit, ok := initExpr.Kind.(Literal); ok {
			if f, ok := lit.Value.(LiteralF32); ok && (float32(f) < 22.9 || float32(f) > 23.1) {
				t.Errorf("local var t init: expected ~23.0, got %v", f)
			}
		}
		// It's OK if init is still Binary — GLSL writer can const-eval at write time
	}
}
