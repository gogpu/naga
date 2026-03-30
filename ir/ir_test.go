package ir

import (
	"testing"
)

// --- TypeResInner tests ---

func TestTypeResInner(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			{Name: "vec4f", Inner: VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
		},
	}

	t.Run("handle resolution", func(t *testing.T) {
		h := TypeHandle(0)
		res := TypeResolution{Handle: &h}
		inner := TypeResInner(module, res)
		s, ok := inner.(ScalarType)
		if !ok || s.Kind != ScalarFloat || s.Width != 4 {
			t.Errorf("expected ScalarFloat/4, got %v", inner)
		}
	})

	t.Run("value resolution", func(t *testing.T) {
		res := TypeResolution{Value: ScalarType{Kind: ScalarUint, Width: 4}}
		inner := TypeResInner(module, res)
		s, ok := inner.(ScalarType)
		if !ok || s.Kind != ScalarUint || s.Width != 4 {
			t.Errorf("expected ScalarUint/4, got %v", inner)
		}
	})

	t.Run("nil handle uses value", func(t *testing.T) {
		res := TypeResolution{
			Handle: nil,
			Value:  VectorType{Size: Vec3, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}},
		}
		inner := TypeResInner(module, res)
		v, ok := inner.(VectorType)
		if !ok || v.Size != Vec3 {
			t.Errorf("expected Vec3, got %v", inner)
		}
	})
}

// --- StorageFormat.ScalarKind tests ---

func TestStorageFormatScalarKind(t *testing.T) {
	tests := []struct {
		name   string
		format StorageFormat
		want   ScalarKind
	}{
		// Uint formats
		{"R8Uint", StorageFormatR8Uint, ScalarUint},
		{"R16Uint", StorageFormatR16Uint, ScalarUint},
		{"R32Uint", StorageFormatR32Uint, ScalarUint},
		{"Rg8Uint", StorageFormatRg8Uint, ScalarUint},
		{"Rg16Uint", StorageFormatRg16Uint, ScalarUint},
		{"Rg32Uint", StorageFormatRg32Uint, ScalarUint},
		{"Rgba8Uint", StorageFormatRgba8Uint, ScalarUint},
		{"Rgba16Uint", StorageFormatRgba16Uint, ScalarUint},
		{"Rgba32Uint", StorageFormatRgba32Uint, ScalarUint},
		{"Rgb10a2Uint", StorageFormatRgb10a2Uint, ScalarUint},
		{"R64Uint", StorageFormatR64Uint, ScalarUint},

		// Sint formats
		{"R8Sint", StorageFormatR8Sint, ScalarSint},
		{"R16Sint", StorageFormatR16Sint, ScalarSint},
		{"R32Sint", StorageFormatR32Sint, ScalarSint},
		{"Rg8Sint", StorageFormatRg8Sint, ScalarSint},
		{"Rg16Sint", StorageFormatRg16Sint, ScalarSint},
		{"Rg32Sint", StorageFormatRg32Sint, ScalarSint},
		{"Rgba8Sint", StorageFormatRgba8Sint, ScalarSint},
		{"Rgba16Sint", StorageFormatRgba16Sint, ScalarSint},
		{"Rgba32Sint", StorageFormatRgba32Sint, ScalarSint},
		{"R64Sint", StorageFormatR64Sint, ScalarSint},

		// Float formats (unorm, snorm, float, ufloat)
		{"R8Unorm", StorageFormatR8Unorm, ScalarFloat},
		{"R8Snorm", StorageFormatR8Snorm, ScalarFloat},
		{"R16Float", StorageFormatR16Float, ScalarFloat},
		{"R32Float", StorageFormatR32Float, ScalarFloat},
		{"Rgba8Unorm", StorageFormatRgba8Unorm, ScalarFloat},
		{"Rgba8Snorm", StorageFormatRgba8Snorm, ScalarFloat},
		{"Bgra8Unorm", StorageFormatBgra8Unorm, ScalarFloat},
		{"Rg11b10Ufloat", StorageFormatRg11b10Ufloat, ScalarFloat},
		{"Rgb10a2Unorm", StorageFormatRgb10a2Unorm, ScalarFloat},
		{"Rgba16Float", StorageFormatRgba16Float, ScalarFloat},
		{"Rgba32Float", StorageFormatRgba32Float, ScalarFloat},
		{"R16Unorm", StorageFormatR16Unorm, ScalarFloat},
		{"R16Snorm", StorageFormatR16Snorm, ScalarFloat},
		{"Rg16Unorm", StorageFormatRg16Unorm, ScalarFloat},
		{"Rg16Snorm", StorageFormatRg16Snorm, ScalarFloat},
		{"Rgba16Unorm", StorageFormatRgba16Unorm, ScalarFloat},
		{"Rgba16Snorm", StorageFormatRgba16Snorm, ScalarFloat},
		{"Unknown", StorageFormatUnknown, ScalarFloat},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.format.ScalarKind()
			if got != tt.want {
				t.Errorf("StorageFormat(%d).ScalarKind() = %d, want %d", tt.format, got, tt.want)
			}
		})
	}
}

// --- TypeInner interface marker tests ---

func TestTypeInnerInterfaceImplementation(t *testing.T) {
	// Verify all TypeInner implementations satisfy the interface.
	// This catches regressions if the interface method changes.
	tests := []struct {
		name  string
		inner TypeInner
	}{
		{"ScalarType", ScalarType{Kind: ScalarFloat, Width: 4}},
		{"VectorType", VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
		{"MatrixType", MatrixType{Columns: Vec4, Rows: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
		{"ArrayType", ArrayType{Base: 0, Stride: 16}},
		{"StructType", StructType{Members: nil, Span: 0}},
		{"PointerType", PointerType{Base: 0, Space: SpaceFunction}},
		{"AtomicType", AtomicType{Scalar: ScalarType{Kind: ScalarUint, Width: 4}}},
		{"BindingArrayType", BindingArrayType{Base: 0}},
		{"AccelerationStructureType", AccelerationStructureType{}},
		{"RayQueryType", RayQueryType{}},
		{"SamplerType", SamplerType{Comparison: false}},
		{"ImageType", ImageType{Dim: Dim2D, Class: ImageClassSampled}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify it compiles and satisfies the interface
			tt.inner.typeInner()
		})
	}
}

// --- ExpressionKind interface marker tests ---

func TestExpressionKindInterfaceImplementation(t *testing.T) {
	tests := []struct {
		name string
		kind ExpressionKind
	}{
		{"Literal", Literal{Value: LiteralF32(0)}},
		{"ExprConstant", ExprConstant{}},
		{"ExprOverride", ExprOverride{}},
		{"ExprZeroValue", ExprZeroValue{}},
		{"ExprCompose", ExprCompose{}},
		{"ExprAccess", ExprAccess{}},
		{"ExprAccessIndex", ExprAccessIndex{}},
		{"ExprSplat", ExprSplat{}},
		{"ExprSwizzle", ExprSwizzle{}},
		{"ExprFunctionArgument", ExprFunctionArgument{}},
		{"ExprGlobalVariable", ExprGlobalVariable{}},
		{"ExprLocalVariable", ExprLocalVariable{}},
		{"ExprLoad", ExprLoad{}},
		{"ExprImageSample", ExprImageSample{}},
		{"ExprImageLoad", ExprImageLoad{}},
		{"ExprImageQuery", ExprImageQuery{}},
		{"ExprUnary", ExprUnary{}},
		{"ExprBinary", ExprBinary{}},
		{"ExprSelect", ExprSelect{}},
		{"ExprDerivative", ExprDerivative{}},
		{"ExprRelational", ExprRelational{}},
		{"ExprMath", ExprMath{}},
		{"ExprAs", ExprAs{}},
		{"ExprCallResult", ExprCallResult{}},
		{"ExprArrayLength", ExprArrayLength{}},
		{"ExprAtomicResult", ExprAtomicResult{}},
		{"ExprWorkGroupUniformLoadResult", ExprWorkGroupUniformLoadResult{}},
		{"ExprRayQueryProceedResult", ExprRayQueryProceedResult{}},
		{"ExprRayQueryGetIntersection", ExprRayQueryGetIntersection{}},
		{"ExprSubgroupBallotResult", ExprSubgroupBallotResult{}},
		{"ExprSubgroupOperationResult", ExprSubgroupOperationResult{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.kind.expressionKind()
		})
	}
}

// --- StatementKind interface marker tests ---

func TestStatementKindInterfaceImplementation(t *testing.T) {
	tests := []struct {
		name string
		kind StatementKind
	}{
		{"StmtEmit", StmtEmit{}},
		{"StmtBlock", StmtBlock{}},
		{"StmtIf", StmtIf{}},
		{"StmtSwitch", StmtSwitch{}},
		{"StmtLoop", StmtLoop{}},
		{"StmtBreak", StmtBreak{}},
		{"StmtContinue", StmtContinue{}},
		{"StmtReturn", StmtReturn{}},
		{"StmtKill", StmtKill{}},
		{"StmtBarrier", StmtBarrier{}},
		{"StmtStore", StmtStore{}},
		{"StmtImageStore", StmtImageStore{}},
		{"StmtAtomic", StmtAtomic{}},
		{"StmtWorkGroupUniformLoad", StmtWorkGroupUniformLoad{}},
		{"StmtCall", StmtCall{}},
		{"StmtRayQuery", StmtRayQuery{}},
		{"StmtSubgroupBallot", StmtSubgroupBallot{}},
		{"StmtSubgroupCollectiveOperation", StmtSubgroupCollectiveOperation{}},
		{"StmtSubgroupGather", StmtSubgroupGather{}},
		{"StmtImageAtomic", StmtImageAtomic{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.kind.statementKind()
		})
	}
}

// --- LiteralValue interface tests ---

func TestLiteralValueInterfaceImplementation(t *testing.T) {
	tests := []struct {
		name string
		val  LiteralValue
	}{
		{"F64", LiteralF64(1.0)},
		{"F32", LiteralF32(1.0)},
		{"F16", LiteralF16(1.0)},
		{"U32", LiteralU32(1)},
		{"I32", LiteralI32(1)},
		{"U64", LiteralU64(1)},
		{"I64", LiteralI64(1)},
		{"Bool", LiteralBool(true)},
		{"AbstractInt", LiteralAbstractInt(1)},
		{"AbstractFloat", LiteralAbstractFloat(1.0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.val.literalValue()
		})
	}
}

// --- OverrideInitExpr interface tests ---

func TestOverrideInitExprInterface(t *testing.T) {
	tests := []struct {
		name string
		expr OverrideInitExpr
	}{
		{"Literal", OverrideInitLiteral{Value: 1.0}},
		{"Ref", OverrideInitRef{Handle: 0}},
		{"Binary", OverrideInitBinary{Op: BinaryAdd, Left: OverrideInitLiteral{Value: 1.0}, Right: OverrideInitLiteral{Value: 2.0}}},
		{"Unary", OverrideInitUnary{Op: UnaryNegate, Expr: OverrideInitLiteral{Value: 1.0}}},
		{"BoolLiteral", OverrideInitBoolLiteral{Value: true}},
		{"UintLiteral", OverrideInitUintLiteral{Value: 42}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.expr.overrideInitExpr()
		})
	}
}

// --- Binding interface tests ---

func TestBindingInterface(t *testing.T) {
	tests := []struct {
		name    string
		binding Binding
	}{
		{"Builtin", BuiltinBinding{Builtin: BuiltinPosition}},
		{"Location", LocationBinding{Location: 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.binding.binding()
		})
	}
}

// --- ConstantValue interface tests ---

func TestConstantValueInterface(t *testing.T) {
	tests := []struct {
		name string
		val  ConstantValue
	}{
		{"Scalar", ScalarValue{Bits: 0, Kind: ScalarFloat}},
		{"Composite", CompositeValue{Components: nil}},
		{"Zero", ZeroConstantValue{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.val.constantValue()
		})
	}
}

// --- SwitchValue interface tests ---

func TestSwitchValueInterface(t *testing.T) {
	tests := []struct {
		name string
		val  SwitchValue
	}{
		{"I32", SwitchValueI32(1)},
		{"U32", SwitchValueU32(1)},
		{"Default", SwitchValueDefault{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.val.switchValue()
		})
	}
}

// --- SampleLevel interface tests ---

func TestSampleLevelInterface(t *testing.T) {
	tests := []struct {
		name  string
		level SampleLevel
	}{
		{"Auto", SampleLevelAuto{}},
		{"Zero", SampleLevelZero{}},
		{"Exact", SampleLevelExact{Level: 0}},
		{"Bias", SampleLevelBias{Bias: 0}},
		{"Gradient", SampleLevelGradient{X: 0, Y: 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.level.sampleLevel()
		})
	}
}

// --- ImageQuery interface tests ---

func TestImageQueryInterface(t *testing.T) {
	tests := []struct {
		name  string
		query ImageQuery
	}{
		{"Size", ImageQuerySize{}},
		{"NumLevels", ImageQueryNumLevels{}},
		{"NumLayers", ImageQueryNumLayers{}},
		{"NumSamples", ImageQueryNumSamples{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.query.imageQuery()
		})
	}
}

// --- AtomicFunction interface tests ---

func TestAtomicFunctionInterface(t *testing.T) {
	tests := []struct {
		name string
		fun  AtomicFunction
	}{
		{"Add", AtomicAdd{}},
		{"Subtract", AtomicSubtract{}},
		{"And", AtomicAnd{}},
		{"ExclusiveOr", AtomicExclusiveOr{}},
		{"InclusiveOr", AtomicInclusiveOr{}},
		{"Min", AtomicMin{}},
		{"Max", AtomicMax{}},
		{"Exchange", AtomicExchange{}},
		{"Store", AtomicStore{}},
		{"Load", AtomicLoad{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.fun.atomicFunction()
		})
	}
}

// --- RayQueryFunction interface tests ---

func TestRayQueryFunctionInterface(t *testing.T) {
	tests := []struct {
		name string
		fun  RayQueryFunction
	}{
		{"Initialize", RayQueryInitialize{}},
		{"Proceed", RayQueryProceed{}},
		{"Terminate", RayQueryTerminate{}},
		{"GenerateIntersection", RayQueryGenerateIntersection{}},
		{"ConfirmIntersection", RayQueryConfirmIntersection{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.fun.rayQueryFunction()
		})
	}
}

// --- GatherMode interface tests ---

func TestGatherModeInterface(t *testing.T) {
	tests := []struct {
		name string
		mode GatherMode
	}{
		{"BroadcastFirst", GatherBroadcastFirst{}},
		{"Broadcast", GatherBroadcast{}},
		{"Shuffle", GatherShuffle{}},
		{"ShuffleDown", GatherShuffleDown{}},
		{"ShuffleUp", GatherShuffleUp{}},
		{"ShuffleXor", GatherShuffleXor{}},
		{"QuadBroadcast", GatherQuadBroadcast{}},
		{"QuadSwap", GatherQuadSwap{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mode.gatherMode()
		})
	}
}

// --- TypeRegistry.Append / GetTypes tests ---

func TestTypeRegistry_Append(t *testing.T) {
	reg := NewTypeRegistry()

	h1 := reg.Append("", ScalarType{Kind: ScalarFloat, Width: 4})
	h2 := reg.Append("", ScalarType{Kind: ScalarFloat, Width: 4}) // same type, but Append creates new

	if h1 == h2 {
		t.Errorf("Append should create distinct handles, got %d and %d", h1, h2)
	}
	if reg.Count() != 2 {
		t.Errorf("expected 2 types, got %d", reg.Count())
	}
}

func TestTypeRegistry_GetTypes(t *testing.T) {
	reg := NewTypeRegistry()
	reg.GetOrCreate("f32", ScalarType{Kind: ScalarFloat, Width: 4})
	reg.GetOrCreate("u32", ScalarType{Kind: ScalarUint, Width: 4})

	types := reg.GetTypes()
	if len(types) != 2 {
		t.Fatalf("expected 2 types, got %d", len(types))
	}
	if types[0].Name != "f32" {
		t.Errorf("expected f32, got %s", types[0].Name)
	}
	if types[1].Name != "u32" {
		t.Errorf("expected u32, got %s", types[1].Name)
	}
}

func TestTypeRegistry_Deduplication(t *testing.T) {
	reg := NewTypeRegistry()

	h1 := reg.GetOrCreate("", ScalarType{Kind: ScalarFloat, Width: 4})
	h2 := reg.GetOrCreate("", ScalarType{Kind: ScalarFloat, Width: 4})

	if h1 != h2 {
		t.Errorf("GetOrCreate should deduplicate same types, got %d and %d", h1, h2)
	}
	if reg.Count() != 1 {
		t.Errorf("expected 1 type (deduplicated), got %d", reg.Count())
	}
}

func TestTypeRegistry_NamedStructsNotDeduplicated(t *testing.T) {
	reg := NewTypeRegistry()

	h1 := reg.GetOrCreate("Input1", StructType{Members: nil, Span: 0})
	h2 := reg.GetOrCreate("Input2", StructType{Members: nil, Span: 0})

	if h1 == h2 {
		t.Error("named structs with different names should not be deduplicated")
	}
}

// --- stmtSubBlocks tests ---

func TestStmtSubBlocks(t *testing.T) {
	t.Run("Block", func(t *testing.T) {
		inner := Block{Statement{Kind: StmtBreak{}}}
		blocks := stmtSubBlocks(Statement{Kind: StmtBlock{Block: inner}})
		if len(blocks) != 1 {
			t.Fatalf("expected 1 block, got %d", len(blocks))
		}
		if len(blocks[0]) != 1 {
			t.Error("expected 1 statement in block")
		}
	})

	t.Run("If", func(t *testing.T) {
		blocks := stmtSubBlocks(Statement{Kind: StmtIf{
			Accept: Block{Statement{Kind: StmtBreak{}}},
			Reject: Block{Statement{Kind: StmtContinue{}}},
		}})
		if len(blocks) != 2 {
			t.Fatalf("expected 2 blocks, got %d", len(blocks))
		}
	})

	t.Run("Loop", func(t *testing.T) {
		blocks := stmtSubBlocks(Statement{Kind: StmtLoop{
			Body:       Block{Statement{Kind: StmtBreak{}}},
			Continuing: Block{},
		}})
		if len(blocks) != 2 {
			t.Fatalf("expected 2 blocks, got %d", len(blocks))
		}
	})

	t.Run("Switch", func(t *testing.T) {
		blocks := stmtSubBlocks(Statement{Kind: StmtSwitch{
			Cases: []SwitchCase{
				{Body: Block{Statement{Kind: StmtBreak{}}}},
				{Body: Block{Statement{Kind: StmtReturn{}}}},
			},
		}})
		if len(blocks) != 2 {
			t.Fatalf("expected 2 blocks, got %d", len(blocks))
		}
	})

	t.Run("Store has no sub-blocks", func(t *testing.T) {
		blocks := stmtSubBlocks(Statement{Kind: StmtStore{Pointer: 0, Value: 1}})
		if blocks != nil {
			t.Errorf("expected nil, got %v", blocks)
		}
	})

	t.Run("Return has no sub-blocks", func(t *testing.T) {
		blocks := stmtSubBlocks(Statement{Kind: StmtReturn{}})
		if blocks != nil {
			t.Errorf("expected nil, got %v", blocks)
		}
	})
}
