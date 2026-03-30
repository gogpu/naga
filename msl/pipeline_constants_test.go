package msl

import (
	"math"
	"testing"

	"github.com/gogpu/naga/ir"
)

func TestMapValueToScalar(t *testing.T) {
	tests := []struct {
		name    string
		value   float64
		scalar  ir.ScalarType
		want    ir.ScalarValue
		wantErr bool
	}{
		// Bool conversion (WebIDL semantics: NaN and 0 are false)
		{"bool false from 0", 0.0, ir.ScalarType{Kind: ir.ScalarBool, Width: 1},
			ir.ScalarValue{Bits: 0, Kind: ir.ScalarBool}, false},
		{"bool false from -0", math.Copysign(0, -1), ir.ScalarType{Kind: ir.ScalarBool, Width: 1},
			ir.ScalarValue{Bits: 0, Kind: ir.ScalarBool}, false},
		{"bool false from NaN", math.NaN(), ir.ScalarType{Kind: ir.ScalarBool, Width: 1},
			ir.ScalarValue{Bits: 0, Kind: ir.ScalarBool}, false},
		{"bool true from 1", 1.0, ir.ScalarType{Kind: ir.ScalarBool, Width: 1},
			ir.ScalarValue{Bits: 1, Kind: ir.ScalarBool}, false},
		{"bool true from Inf", math.Inf(1), ir.ScalarType{Kind: ir.ScalarBool, Width: 1},
			ir.ScalarValue{Bits: 1, Kind: ir.ScalarBool}, false},

		// i32 conversion
		{"i32 from 2.0", 2.0, ir.ScalarType{Kind: ir.ScalarSint, Width: 4},
			ir.ScalarValue{Bits: 2, Kind: ir.ScalarSint}, false},
		{"i32 from -5.0", -5.0, ir.ScalarType{Kind: ir.ScalarSint, Width: 4},
			ir.ScalarValue{Bits: 0xFFFFFFFFFFFFFFFB, Kind: ir.ScalarSint}, false},
		{"i32 from NaN", math.NaN(), ir.ScalarType{Kind: ir.ScalarSint, Width: 4},
			ir.ScalarValue{}, true},

		// u32 conversion
		{"u32 from 2.0", 2.0, ir.ScalarType{Kind: ir.ScalarUint, Width: 4},
			ir.ScalarValue{Bits: 2, Kind: ir.ScalarUint}, false},
		{"u32 from negative", -1.0, ir.ScalarType{Kind: ir.ScalarUint, Width: 4},
			ir.ScalarValue{}, true},

		// f32 conversion
		{"f32 from 2.3", 2.3, ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
			ir.ScalarValue{Bits: uint64(math.Float32bits(float32(2.3))), Kind: ir.ScalarFloat}, false},
		{"f32 from 0.0", 0.0, ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
			ir.ScalarValue{Bits: uint64(math.Float32bits(0.0)), Kind: ir.ScalarFloat}, false},

		// f64 conversion
		{"f64 from 2.3", 2.3, ir.ScalarType{Kind: ir.ScalarFloat, Width: 8},
			ir.ScalarValue{Bits: math.Float64bits(2.3), Kind: ir.ScalarFloat}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapValueToScalar(tt.value, tt.scalar)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("got = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestProcessFunctionOverrides_NamedExpressions(t *testing.T) {
	// Verify that processFunctionOverrides updates named expression handles.
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Overrides: []ir.Override{
			{Name: "o", Ty: 0, Init: nil},
		},
		GlobalExpressions: []ir.Expression{},
		EntryPoints: []ir.EntryPoint{{
			Name:  "main",
			Stage: ir.StageCompute,
			Function: ir.Function{
				Expressions: []ir.Expression{
					{Kind: ir.ExprOverride{Override: 0}},                            // [0] o
					{Kind: ir.Literal{Value: ir.LiteralF32(5)}},                     // [1] 5.0
					{Kind: ir.ExprBinary{Op: ir.BinaryMultiply, Left: 0, Right: 1}}, // [2] o*5
				},
				NamedExpressions: map[ir.ExpressionHandle]string{2: "result"},
				Body: ir.Block{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 2, End: 3}}},
					{Kind: ir.StmtReturn{Value: nil}},
				},
			},
		}},
	}

	result := applyPipelineConstants(module, map[string]float64{"o": 2.0})
	ep := &result.EntryPoints[0]

	// The Binary(o*5) should fold to Literal(10.0). The named expression
	// "result" should point to the Literal expression in the rebuilt arena.
	found := false
	for h, name := range ep.Function.NamedExpressions {
		if name != "result" {
			continue
		}
		found = true
		if int(h) >= len(ep.Function.Expressions) {
			t.Fatalf("named handle %d out of range (len=%d)", h, len(ep.Function.Expressions))
		}
		lit, ok := ep.Function.Expressions[h].Kind.(ir.Literal)
		if !ok {
			t.Fatalf("expected Literal at handle %d, got %T", h, ep.Function.Expressions[h].Kind)
		}
		if f, ok := lit.Value.(ir.LiteralF32); !ok || float32(f) != 10.0 {
			t.Errorf("expected Literal(10.0), got %v", lit.Value)
		}
	}
	if !found {
		t.Error("named expression 'result' not found")
	}
}

func TestProcessFunctionOverrides_ExpressionIndexShift(t *testing.T) {
	// Verify that expression indices shift correctly when override const
	// values are deep-copied into the arena (matching Rust naga behavior).
	// Override(X) + const eval creates extra expressions in the arena.
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Overrides: []ir.Override{
			{Name: "x", Ty: 0, Init: nil},
		},
		GlobalExpressions: []ir.Expression{},
		EntryPoints: []ir.EntryPoint{{
			Name:  "main",
			Stage: ir.StageCompute,
			Function: ir.Function{
				Expressions: []ir.Expression{
					{Kind: ir.ExprOverride{Override: 0}},                            // [0] x
					{Kind: ir.Literal{Value: ir.LiteralF32(3)}},                     // [1] 3.0
					{Kind: ir.ExprBinary{Op: ir.BinaryMultiply, Left: 0, Right: 1}}, // [2] x*3
					{Kind: ir.ExprGlobalVariable{Variable: 0}},                      // [3] global_var
					{Kind: ir.ExprLoad{Pointer: 3}},                                 // [4] load
				},
				Body: ir.Block{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 2, End: 3}}},
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 4, End: 5}}},
					{Kind: ir.StmtReturn{Value: nil}},
				},
			},
		}},
		GlobalVariables: []ir.GlobalVariable{{Name: "g", Type: 0, Space: ir.SpacePrivate}},
	}

	result := applyPipelineConstants(module, map[string]float64{"x": 2.0})
	ep := &result.EntryPoints[0]

	// After processing:
	// [0] Override(0) → kept
	// [1] Literal(3.0) → kept
	// [2] deep-copy Literal(2.0) (from checkAndGetConst)
	// [3] Literal(6.0) (eval result: 2.0 * 3.0)
	// [4] GlobalVariable(0) → shifted from [3] to [4]
	// [5] Load(4) → shifted from [4] to [5]
	//
	// The Load was at index 4, now at index 5. The deep-copy of the override
	// value at index 2 pushed everything after it by 1.

	if len(ep.Function.Expressions) != 6 {
		t.Fatalf("expected 6 expressions, got %d", len(ep.Function.Expressions))
	}

	// Check the Load expression references the correct GlobalVariable handle.
	loadExpr, ok := ep.Function.Expressions[5].Kind.(ir.ExprLoad)
	if !ok {
		t.Fatalf("expected Load at [5], got %T", ep.Function.Expressions[5].Kind)
	}
	if loadExpr.Pointer != 4 {
		t.Errorf("Load.Pointer = %d, want 4", loadExpr.Pointer)
	}
}

func TestProcessFunctionOverrides_TypeConversion(t *testing.T) {
	// Verify that ExprAs (type conversion) with override operand folds correctly.
	// u32(i32_override) with value 2 should become LiteralU32(2).
	width := uint8(4)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		},
		Overrides: []ir.Override{
			{Name: "o", Ty: 0, Init: nil},
		},
		GlobalExpressions: []ir.Expression{},
		EntryPoints: []ir.EntryPoint{{
			Name:  "main",
			Stage: ir.StageCompute,
			Function: ir.Function{
				Expressions: []ir.Expression{
					{Kind: ir.ExprOverride{Override: 0}},                             // [0] o (i32)
					{Kind: ir.ExprAs{Expr: 0, Kind: ir.ScalarUint, Convert: &width}}, // [1] u32(o)
				},
				Body: ir.Block{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 2}}},
					{Kind: ir.StmtReturn{Value: nil}},
				},
			},
		}},
	}

	result := applyPipelineConstants(module, map[string]float64{"o": 2.0})
	ep := &result.EntryPoints[0]

	// After processing: Override → deep-copy Literal(I32(2)) → eval u32(2) → Literal(U32(2))
	// Find the last Literal expression and check its type.
	foundU32 := false
	for i := len(ep.Function.Expressions) - 1; i >= 0; i-- {
		if lit, ok := ep.Function.Expressions[i].Kind.(ir.Literal); ok {
			if u, ok := lit.Value.(ir.LiteralU32); ok && uint32(u) == 2 {
				foundU32 = true
				break
			}
		}
	}
	if !foundU32 {
		t.Error("expected LiteralU32(2) in expressions after type conversion folding")
		for i, expr := range ep.Function.Expressions {
			t.Logf("  [%d] %T %+v", i, expr.Kind, expr.Kind)
		}
	}
}

func TestProcessFunctionOverrides_AtomicFunAdjusted(t *testing.T) {
	// Verify that AtomicExchange.Compare handle is adjusted during rebuild.
	width := uint8(4)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		},
		Overrides: []ir.Override{
			{Name: "o", Ty: 0, Init: nil},
		},
		GlobalExpressions: []ir.Expression{},
		EntryPoints: []ir.EntryPoint{{
			Name:  "main",
			Stage: ir.StageCompute,
			Function: ir.Function{
				Expressions: []ir.Expression{
					{Kind: ir.ExprGlobalVariable{Variable: 0}},                       // [0] &a
					{Kind: ir.ExprOverride{Override: 0}},                             // [1] o
					{Kind: ir.ExprAs{Expr: 1, Kind: ir.ScalarUint, Convert: &width}}, // [2] u32(o)
					{Kind: ir.Literal{Value: ir.LiteralU32(1)}},                      // [3] 1u
					{Kind: ir.ExprAtomicResult{Ty: 1, Comparison: true}},             // [4] result
				},
				Body: ir.Block{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 2, End: 3}}},
					{Kind: ir.StmtAtomic{
						Pointer: 0,
						Fun:     ir.AtomicExchange{Compare: ptrHandle(2)},
						Value:   3,
						Result:  ptrHandle(4),
					}},
					{Kind: ir.StmtReturn{Value: nil}},
				},
			},
		}},
		GlobalVariables: []ir.GlobalVariable{{Name: "a", Type: 1, Space: ir.SpaceWorkGroup}},
	}

	result := applyPipelineConstants(module, map[string]float64{"o": 2.0})
	ep := &result.EntryPoints[0]

	// Find the atomic statement and check Compare handle points to LiteralU32.
	for _, stmt := range ep.Function.Body {
		atomic, ok := stmt.Kind.(ir.StmtAtomic)
		if !ok {
			continue
		}
		exchange, ok := atomic.Fun.(ir.AtomicExchange)
		if !ok || exchange.Compare == nil {
			t.Fatal("expected AtomicExchange with Compare")
		}
		cmpH := *exchange.Compare
		if int(cmpH) >= len(ep.Function.Expressions) {
			t.Fatalf("Compare handle %d out of range", cmpH)
		}
		lit, ok := ep.Function.Expressions[cmpH].Kind.(ir.Literal)
		if !ok {
			t.Fatalf("expected Literal at Compare handle %d, got %T", cmpH, ep.Function.Expressions[cmpH].Kind)
		}
		if _, ok := lit.Value.(ir.LiteralU32); !ok {
			t.Errorf("expected LiteralU32, got %T", lit.Value)
		}
		return
	}
	t.Error("no atomic statement found in body")
}

func ptrHandle(h ir.ExpressionHandle) *ir.ExpressionHandle {
	return &h
}

func TestProcessFunctionOverrides_NoOverrides(t *testing.T) {
	// Functions without override references should not be modified.
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Overrides: []ir.Override{
			{Name: "o", Ty: 0, Init: nil},
		},
		GlobalExpressions: []ir.Expression{},
		EntryPoints: []ir.EntryPoint{{
			Name:  "main",
			Stage: ir.StageCompute,
			Function: ir.Function{
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralF32(1)}},
					{Kind: ir.Literal{Value: ir.LiteralF32(2)}},
					{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 1}},
				},
				Body: ir.Block{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 2, End: 3}}},
					{Kind: ir.StmtReturn{Value: nil}},
				},
			},
		}},
	}

	result := applyPipelineConstants(module, map[string]float64{"o": 5.0})
	ep := &result.EntryPoints[0]

	// No overrides referenced — expressions should remain unchanged.
	if len(ep.Function.Expressions) != 3 {
		t.Errorf("expected 3 expressions, got %d", len(ep.Function.Expressions))
	}
}

func TestApplyPipelineConstants(t *testing.T) {
	// Build a minimal module with overrides.
	id0 := uint16(100)
	initExpr0 := ir.ExpressionHandle(0) // Literal(I32(0)) at index 0

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "i32", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Overrides: []ir.Override{
			{Name: "o", Ty: 0, Init: &initExpr0}, // override o: i32 = 0
			{Name: "width", Ty: 1, Init: nil},    // override width: f32 (no default)
		},
		GlobalExpressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralI32(0)}}, // init for override 0
		},
		Constants: []ir.Constant{
			{Name: "regular", Type: 0, Value: ir.ScalarValue{Bits: 42, Kind: ir.ScalarSint}},
		},
	}

	t.Run("substitute by name", func(t *testing.T) {
		result := applyPipelineConstants(module, map[string]float64{
			"o": 5.0,
		})
		// Overrides should exist in result.
		if len(result.Overrides) != 2 {
			t.Fatalf("expected 2 overrides, got %d", len(result.Overrides))
		}
		// Original module should be unchanged.
		if len(module.Overrides) != 2 {
			t.Fatalf("original module was mutated")
		}
	})

	t.Run("substitute by id", func(t *testing.T) {
		moduleWithID := &ir.Module{
			Types: module.Types,
			Overrides: []ir.Override{
				{Name: "o", ID: &id0, Ty: 0, Init: &initExpr0},
			},
			GlobalExpressions: module.GlobalExpressions,
		}

		result := applyPipelineConstants(moduleWithID, map[string]float64{
			"100": 7.0,
		})
		if len(result.Overrides) != 1 {
			t.Fatalf("expected 1 override, got %d", len(result.Overrides))
		}
	})
}
