package ir

import "testing"

func TestLiteralType(t *testing.T) {
	tests := []struct {
		name     string
		literal  Literal
		wantType TypeInner
	}{
		{
			name:     "f32 literal",
			literal:  Literal{Value: LiteralF32(3.14)},
			wantType: ScalarType{Kind: ScalarFloat, Width: 4},
		},
		{
			name:     "i32 literal",
			literal:  Literal{Value: LiteralI32(42)},
			wantType: ScalarType{Kind: ScalarSint, Width: 4},
		},
		{
			name:     "u32 literal",
			literal:  Literal{Value: LiteralU32(100)},
			wantType: ScalarType{Kind: ScalarUint, Width: 4},
		},
		{
			name:     "bool literal true",
			literal:  Literal{Value: LiteralBool(true)},
			wantType: ScalarType{Kind: ScalarBool, Width: 1},
		},
		{
			name:     "bool literal false",
			literal:  Literal{Value: LiteralBool(false)},
			wantType: ScalarType{Kind: ScalarBool, Width: 1},
		},
		{
			name:     "abstract int",
			literal:  Literal{Value: LiteralAbstractInt(42)},
			wantType: ScalarType{Kind: ScalarAbstractInt, Width: 8},
		},
		{
			name:     "abstract float",
			literal:  Literal{Value: LiteralAbstractFloat(3.14)},
			wantType: ScalarType{Kind: ScalarAbstractFloat, Width: 8},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.literal.Type()
			if err != nil {
				t.Fatalf("Literal.Type() error = %v", err)
			}
			if got.Handle != nil {
				t.Errorf("expected inline type, got handle")
			}
			if !typeInnerEqual(got.Value, tt.wantType) {
				t.Errorf("Literal.Type() = %v, want %v", got.Value, tt.wantType)
			}
		})
	}
}

func TestLiteralTypeWideAndHalf(t *testing.T) {
	tests := []struct {
		name      string
		literal   Literal
		wantKind  ScalarKind
		wantWidth uint8
	}{
		{"f64", Literal{Value: LiteralF64(1.0)}, ScalarFloat, 8},
		{"f16", Literal{Value: LiteralF16(1.0)}, ScalarFloat, 2},
		{"u64", Literal{Value: LiteralU64(1)}, ScalarUint, 8},
		{"i64", Literal{Value: LiteralI64(1)}, ScalarSint, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.literal.Type()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			s, ok := got.Value.(ScalarType)
			if !ok || s.Kind != tt.wantKind || s.Width != tt.wantWidth {
				t.Errorf("expected %d/%d, got %v", tt.wantKind, tt.wantWidth, got.Value)
			}
		})
	}
}

// Helper function to compare TypeInner values
func typeInnerEqual(a, b TypeInner) bool {
	switch at := a.(type) {
	case ScalarType:
		bt, ok := b.(ScalarType)
		return ok && at.Kind == bt.Kind && at.Width == bt.Width
	case VectorType:
		bt, ok := b.(VectorType)
		return ok && at.Size == bt.Size && typeInnerEqual(at.Scalar, bt.Scalar)
	case MatrixType:
		bt, ok := b.(MatrixType)
		return ok && at.Columns == bt.Columns && at.Rows == bt.Rows && typeInnerEqual(at.Scalar, bt.Scalar)
	default:
		return false
	}
}
