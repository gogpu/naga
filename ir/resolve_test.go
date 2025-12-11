package ir

import "testing"

func TestResolveLiteralType(t *testing.T) {
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
			wantType: ScalarType{Kind: ScalarSint, Width: 4},
		},
		{
			name:     "abstract float",
			literal:  Literal{Value: LiteralAbstractFloat(3.14)},
			wantType: ScalarType{Kind: ScalarFloat, Width: 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveLiteralType(tt.literal)
			if err != nil {
				t.Fatalf("resolveLiteralType() error = %v", err)
			}
			if got.Handle != nil {
				t.Errorf("expected inline type, got handle")
			}
			if !typeInnerEqual(got.Value, tt.wantType) {
				t.Errorf("resolveLiteralType() = %v, want %v", got.Value, tt.wantType)
			}
		})
	}
}

func TestResolveExpressionType_Constant(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
		},
		Constants: []Constant{
			{Name: "PI", Type: 0, Value: ScalarValue{Bits: 0, Kind: ScalarFloat}},
		},
	}

	fn := &Function{
		Name:            "test",
		Expressions:     []Expression{{Kind: ExprConstant{Constant: 0}}},
		ExpressionTypes: []TypeResolution{},
	}

	got, err := ResolveExpressionType(module, fn, 0)
	if err != nil {
		t.Fatalf("ResolveExpressionType() error = %v", err)
	}

	if got.Handle == nil {
		t.Errorf("expected type handle, got inline type")
	} else if *got.Handle != 0 {
		t.Errorf("expected type handle 0, got %d", *got.Handle)
	}
}

func TestResolveExpressionType_FunctionArgument(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "vec4<f32>", Inner: VectorType{
				Size:   Vec4,
				Scalar: ScalarType{Kind: ScalarFloat, Width: 4},
			}},
		},
	}

	fn := &Function{
		Name: "test",
		Arguments: []FunctionArgument{
			{Name: "pos", Type: 0},
		},
		Expressions: []Expression{
			{Kind: ExprFunctionArgument{Index: 0}},
		},
		ExpressionTypes: []TypeResolution{},
	}

	got, err := ResolveExpressionType(module, fn, 0)
	if err != nil {
		t.Fatalf("ResolveExpressionType() error = %v", err)
	}

	if got.Handle == nil {
		t.Errorf("expected type handle, got inline type")
	} else if *got.Handle != 0 {
		t.Errorf("expected type handle 0, got %d", *got.Handle)
	}
}

func TestResolveExpressionType_Splat(t *testing.T) {
	module := &Module{}

	fn := &Function{
		Name: "test",
		Expressions: []Expression{
			{Kind: Literal{Value: LiteralF32(1.0)}}, // Expression 0: scalar
			{Kind: ExprSplat{Size: Vec4, Value: 0}}, // Expression 1: splat
		},
		ExpressionTypes: []TypeResolution{
			{Value: ScalarType{Kind: ScalarFloat, Width: 4}},
			{},
		},
	}

	got, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("ResolveExpressionType() error = %v", err)
	}

	if got.Handle != nil {
		t.Errorf("expected inline type, got handle")
	}

	vec, ok := got.Value.(VectorType)
	if !ok {
		t.Fatalf("expected VectorType, got %T", got.Value)
	}

	if vec.Size != Vec4 {
		t.Errorf("expected size Vec4, got %d", vec.Size)
	}
	if vec.Scalar.Kind != ScalarFloat {
		t.Errorf("expected ScalarFloat, got %d", vec.Scalar.Kind)
	}
}

func TestResolveExpressionType_AccessIndex(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			{Name: "vec4<f32>", Inner: VectorType{
				Size:   Vec4,
				Scalar: ScalarType{Kind: ScalarFloat, Width: 4},
			}},
		},
	}

	fn := &Function{
		Name: "test",
		Expressions: []Expression{
			{Kind: ExprFunctionArgument{Index: 0}},     // vec4<f32>
			{Kind: ExprAccessIndex{Base: 0, Index: 2}}, // vec[2] -> f32
		},
		ExpressionTypes: []TypeResolution{
			{Handle: func() *TypeHandle { h := TypeHandle(1); return &h }()},
			{},
		},
	}
	fn.Arguments = []FunctionArgument{{Name: "v", Type: 1}}

	got, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("ResolveExpressionType() error = %v", err)
	}

	if got.Handle != nil {
		t.Errorf("expected inline type, got handle")
	}

	scalar, ok := got.Value.(ScalarType)
	if !ok {
		t.Fatalf("expected ScalarType, got %T", got.Value)
	}

	if scalar.Kind != ScalarFloat {
		t.Errorf("expected ScalarFloat, got %d", scalar.Kind)
	}
}

func TestResolveExpressionType_Binary(t *testing.T) {
	module := &Module{}

	tests := []struct {
		name     string
		op       BinaryOperator
		wantType TypeInner
	}{
		{
			name:     "arithmetic preserves type",
			op:       BinaryAdd,
			wantType: ScalarType{Kind: ScalarFloat, Width: 4},
		},
		{
			name:     "comparison returns bool",
			op:       BinaryEqual,
			wantType: ScalarType{Kind: ScalarBool, Width: 1},
		},
		{
			name:     "logical and returns bool",
			op:       BinaryLogicalAnd,
			wantType: ScalarType{Kind: ScalarBool, Width: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := &Function{
				Name: "test",
				Expressions: []Expression{
					{Kind: Literal{Value: LiteralF32(1.0)}}, // Left operand
					{Kind: Literal{Value: LiteralF32(2.0)}}, // Right operand
					{Kind: ExprBinary{Op: tt.op, Left: 0, Right: 1}},
				},
				ExpressionTypes: []TypeResolution{
					{Value: ScalarType{Kind: ScalarFloat, Width: 4}},
					{Value: ScalarType{Kind: ScalarFloat, Width: 4}},
					{},
				},
			}

			got, err := ResolveExpressionType(module, fn, 2)
			if err != nil {
				t.Fatalf("ResolveExpressionType() error = %v", err)
			}

			if !typeInnerEqual(got.Value, tt.wantType) {
				t.Errorf("got type %v, want %v", got.Value, tt.wantType)
			}
		})
	}
}

func TestResolveExpressionType_MathFunctions(t *testing.T) {
	module := &Module{}

	tests := []struct {
		name     string
		mathFunc MathFunction
		argType  TypeInner
		wantType TypeInner
	}{
		{
			name:     "sin preserves type",
			mathFunc: MathSin,
			argType:  ScalarType{Kind: ScalarFloat, Width: 4},
			wantType: ScalarType{Kind: ScalarFloat, Width: 4},
		},
		{
			name:     "dot returns scalar",
			mathFunc: MathDot,
			argType: VectorType{
				Size:   Vec3,
				Scalar: ScalarType{Kind: ScalarFloat, Width: 4},
			},
			wantType: ScalarType{Kind: ScalarFloat, Width: 4},
		},
		{
			name:     "length returns f32",
			mathFunc: MathLength,
			argType: VectorType{
				Size:   Vec3,
				Scalar: ScalarType{Kind: ScalarFloat, Width: 4},
			},
			wantType: ScalarType{Kind: ScalarFloat, Width: 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := &Function{
				Name: "test",
				Expressions: []Expression{
					{Kind: Literal{Value: LiteralF32(1.0)}}, // Placeholder arg
					{Kind: ExprMath{Fun: tt.mathFunc, Arg: 0}},
				},
				ExpressionTypes: []TypeResolution{
					{Value: tt.argType},
					{},
				},
			}

			got, err := ResolveExpressionType(module, fn, 1)
			if err != nil {
				t.Fatalf("ResolveExpressionType() error = %v", err)
			}

			if !typeInnerEqual(got.Value, tt.wantType) {
				t.Errorf("got type %v, want %v", got.Value, tt.wantType)
			}
		})
	}
}

func TestResolveExpressionType_ImageSample(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "texture_2d<f32>", Inner: ImageType{
				Dim:   Dim2D,
				Class: ImageClassSampled,
			}},
		},
	}

	fn := &Function{
		Name: "test",
		Expressions: []Expression{
			{Kind: ExprGlobalVariable{Variable: 0}}, // Image
			{Kind: ExprImageSample{
				Image:      0,
				Sampler:    0,
				Coordinate: 0,
				Level:      SampleLevelZero{},
			}},
		},
		ExpressionTypes: []TypeResolution{
			{Handle: func() *TypeHandle { h := TypeHandle(0); return &h }()},
			{},
		},
	}

	module.GlobalVariables = []GlobalVariable{
		{Name: "tex", Type: 0, Space: SpaceHandle},
	}

	got, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("ResolveExpressionType() error = %v", err)
	}

	vec, ok := got.Value.(VectorType)
	if !ok {
		t.Fatalf("expected VectorType, got %T", got.Value)
	}

	if vec.Size != Vec4 {
		t.Errorf("expected Vec4, got %d", vec.Size)
	}
	if vec.Scalar.Kind != ScalarFloat {
		t.Errorf("expected ScalarFloat, got %d", vec.Scalar.Kind)
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
