package front

import (
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

func TestPointerSpace(t *testing.T) {
	if space, ok := pointerSpace(ir.PointerType{Space: ir.SpaceStorage}); !ok || space != ir.SpaceStorage {
		t.Errorf("pointerSpace(pointer) = %v, %v; want storage", space, ok)
	}
	if space, ok := pointerSpace(ir.ValuePointerType{Space: ir.SpaceWorkGroup}); !ok || space != ir.SpaceWorkGroup {
		t.Errorf("pointerSpace(value pointer) = %v, %v; want workgroup", space, ok)
	}
	if _, ok := pointerSpace(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}); ok {
		t.Error("pointerSpace(scalar) reported a space")
	}
}

func TestScalarKindAndScalarOf(t *testing.T) {
	scalar := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	for name, inner := range map[string]ir.TypeInner{
		"scalar": scalar,
		"vector": ir.VectorType{Size: ir.Vec3, Scalar: scalar},
		"matrix": ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: scalar},
	} {
		if kind, ok := scalarKind(inner); !ok || kind != ir.ScalarFloat {
			t.Errorf("%s: scalarKind = %v, %v", name, kind, ok)
		}
		if got, ok := scalarOf(inner); !ok || got != scalar {
			t.Errorf("%s: scalarOf = %v, %v", name, got, ok)
		}
	}
	if _, ok := scalarKind(ir.SamplerType{}); ok {
		t.Error("scalarKind(sampler) reported a kind")
	}
	if _, ok := scalarOf(ir.SamplerType{}); ok {
		t.Error("scalarOf(sampler) reported a scalar")
	}
}

func TestErrorMethod(t *testing.T) {
	_, _, err := parseWords(t, Options{}, headerWords()[:1])
	if err == nil {
		t.Fatal("a truncated header was accepted")
	}
	if got := err.Error(); got != "incomplete data" {
		t.Errorf("Error() = %q, want incomplete data", got)
	}
}

func TestParseExecutionModeVariants(t *testing.T) {
	base := func(mode spvenums.ExecutionMode, args ...uint32) [][]uint32 {
		groups := [][]uint32{
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			entryPointOp(spvenums.ExecutionModelFragment),
			instWords(spvenums.OpExecutionMode, append([]uint32{1, uint32(mode)}, args...)...),
			instWords(spvenums.OpTypeVoid, 10),
			instWords(spvenums.OpTypeFunction, 11, 10),
			voidFunctionBody(),
			instWords(spvenums.OpLabel, 2),
			instWords(spvenums.OpReturn),
			instWords(spvenums.OpFunctionEnd),
		}
		return groups
	}

	t.Run("early fragment tests", func(t *testing.T) {
		p := newParser(spvWords(base(spvenums.ExecutionModeEarlyFragmentTests)...), Options{})
		if _, _, err := p.parse(); err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if _, ok := p.entryPoints[1].earlyDepthTest.(ir.EarlyDepthForce); !ok {
			t.Errorf("early depth test = %T, want force", p.entryPoints[1].earlyDepthTest)
		}
	})

	t.Run("depth unchanged and less keep allow", func(t *testing.T) {
		for _, tc := range []struct {
			mode spvenums.ExecutionMode
			want ir.ConservativeDepth
		}{
			{spvenums.ExecutionModeDepthUnchanged, ir.ConservativeDepthUnchanged},
			{spvenums.ExecutionModeDepthLess, ir.ConservativeDepthLessEqual},
			{spvenums.ExecutionModeDepthGreater, ir.ConservativeDepthGreaterEqual},
		} {
			p := newParser(spvWords(base(tc.mode)...), Options{})
			if _, _, err := p.parse(); err != nil {
				t.Fatalf("Parse: %v", err)
			}
			allow, ok := p.entryPoints[1].earlyDepthTest.(ir.EarlyDepthAllow)
			if !ok || allow.Conservative != tc.want {
				t.Errorf("%s: early depth test = %#v, want allow %v", tc.mode, p.entryPoints[1].earlyDepthTest, tc.want)
			}
		}
	})

	t.Run("force wins over a later depth mode", func(t *testing.T) {
		p := newParser(spvWords(
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			entryPointOp(spvenums.ExecutionModelFragment),
			instWords(spvenums.OpExecutionMode, 1, uint32(spvenums.ExecutionModeEarlyFragmentTests)),
			instWords(spvenums.OpExecutionMode, 1, uint32(spvenums.ExecutionModeDepthGreater)),
			instWords(spvenums.OpTypeVoid, 10),
			instWords(spvenums.OpTypeFunction, 11, 10),
			voidFunctionBody(),
			instWords(spvenums.OpLabel, 2),
			instWords(spvenums.OpReturn),
			instWords(spvenums.OpFunctionEnd),
		), Options{})
		if _, _, err := p.parse(); err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if _, ok := p.entryPoints[1].earlyDepthTest.(ir.EarlyDepthForce); !ok {
			t.Errorf("early depth test = %T, want force", p.entryPoints[1].earlyDepthTest)
		}
	})

	t.Run("ignored modes", func(t *testing.T) {
		for _, mode := range []spvenums.ExecutionMode{
			spvenums.ExecutionModeDepthReplacing,
			spvenums.ExecutionModeOriginUpperLeft,
		} {
			p := newParser(spvWords(base(mode)...), Options{})
			if _, _, err := p.parse(); err != nil {
				t.Fatalf("Parse(%s): %v", mode, err)
			}
		}
	})
}

func TestParseConstantWidths(t *testing.T) {
	cases := []struct {
		name  string
		typ   []uint32
		value []uint32
		check func(t *testing.T, module *ir.Module)
	}{
		{
			name:  "u64",
			typ:   instWords(spvenums.OpTypeInt, 1, 64, 0),
			value: []uint32{1, 2, 2, 0xffffffff},
			check: func(t *testing.T, module *ir.Module) {
				literal := module.GlobalExpressions[module.Constants[0].Init].Kind.(ir.Literal)
				if literal.Value != ir.LiteralU64(0xffffffff00000002) {
					t.Errorf("u64 literal = %v", literal.Value)
				}
			},
		},
		{
			name:  "i64",
			typ:   instWords(spvenums.OpTypeInt, 1, 64, 1),
			value: []uint32{1, 2, 0xfffffffe, 0xffffffff},
			check: func(t *testing.T, module *ir.Module) {
				literal := module.GlobalExpressions[module.Constants[0].Init].Kind.(ir.Literal)
				if literal.Value != ir.LiteralI64(-2) {
					t.Errorf("i64 literal = %v", literal.Value)
				}
			},
		},
		{
			name:  "f64",
			typ:   instWords(spvenums.OpTypeFloat, 1, 64),
			value: []uint32{1, 2, 0, 0x40040000},
			check: func(t *testing.T, module *ir.Module) {
				literal := module.GlobalExpressions[module.Constants[0].Init].Kind.(ir.Literal)
				if literal.Value != ir.LiteralF64(2.5) {
					t.Errorf("f64 literal = %v", literal.Value)
				}
			},
		},
		{
			name:  "f16",
			typ:   instWords(spvenums.OpTypeFloat, 1, 16),
			value: []uint32{1, 2, 0x3c00},
			check: func(t *testing.T, module *ir.Module) {
				literal := module.GlobalExpressions[module.Constants[0].Init].Kind.(ir.Literal)
				if literal.Value != ir.LiteralF16(1) {
					t.Errorf("f16 literal = %v", literal.Value)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			module, _, err := parseWords(t, Options{},
				headerWords(),
				instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
				instWords(spvenums.OpMemoryModel, 0, 1),
				tc.typ,
				instWords(spvenums.OpConstant, tc.value...),
			)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			tc.check(t, module)
		})
	}

	t.Run("bool literal through OpConstant", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpTypeBool, 1),
			instWords(spvenums.OpConstant, 1, 2, 1),
		)
		if got := requireError(t, err).Message; got != "unsupported type [0]" {
			t.Errorf("message = %q, want an unsupported type", got)
		}
	})
}

func TestUnsignedBuiltinArgumentTypes(t *testing.T) {
	cases := []struct {
		name    string
		builtin spvenums.BuiltIn
		scalar  []uint32 // the scalar type instruction, id 12
		vector  bool     // the variable holds the vec3 type 13, not the scalar
		check   func(t *testing.T, module *ir.Module, argumentType ir.TypeHandle)
	}{
		{
			name:    "vec3 builtin",
			builtin: spvenums.BuiltInGlobalInvocationId,
			scalar:  instWords(spvenums.OpTypeInt, 12, 32, 1),
			vector:  true,
			check: func(t *testing.T, module *ir.Module, argumentType ir.TypeHandle) {
				vector, ok := module.Types[argumentType].Inner.(ir.VectorType)
				if !ok || vector.Scalar.Kind != ir.ScalarUint {
					t.Errorf("argument type = %#v, want vec3<u32>", module.Types[argumentType].Inner)
				}
			},
		},
		{
			name:    "barycentric without perspective",
			builtin: spvenums.BuiltInBaryCoordNoPerspKHR,
			scalar:  instWords(spvenums.OpTypeFloat, 12, 32),
			vector:  true,
			check: func(t *testing.T, module *ir.Module, argumentType ir.TypeHandle) {
				vector, ok := module.Types[argumentType].Inner.(ir.VectorType)
				if !ok || vector.Scalar.Kind != ir.ScalarFloat {
					t.Errorf("argument type = %#v, want vec3<f32>", module.Types[argumentType].Inner)
				}
			},
		},
		{
			name:    "already unsigned keeps its type",
			builtin: spvenums.BuiltInVertexIndex,
			scalar:  instWords(spvenums.OpTypeInt, 12, 32, 0),
			check: func(t *testing.T, module *ir.Module, argumentType ir.TypeHandle) {
				scalar, ok := module.Types[argumentType].Inner.(ir.ScalarType)
				if !ok || scalar.Kind != ir.ScalarUint {
					t.Errorf("argument type = %#v, want u32", module.Types[argumentType].Inner)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			variableType := uint32(14)
			if tc.vector {
				variableType = 15
			}
			groups := [][]uint32{
				headerWords(),
				instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
				instWords(spvenums.OpMemoryModel, 0, 1),
				entryPointOp(spvenums.ExecutionModelVertex, 3),
				instWords(spvenums.OpDecorate, 3, uint32(spvenums.DecorationBuiltIn), uint32(tc.builtin)),
				instWords(spvenums.OpTypeVoid, 10),
				instWords(spvenums.OpTypeFunction, 11, 10),
				tc.scalar,
				instWords(spvenums.OpTypeVector, 13, 12, 3),
				instWords(spvenums.OpTypePointer, 14, uint32(spvenums.StorageClassInput), 12),
				instWords(spvenums.OpTypePointer, 15, uint32(spvenums.StorageClassInput), 13),
				instWords(spvenums.OpVariable, variableType, 3, uint32(spvenums.StorageClassInput)),
				voidFunctionBody(),
				instWords(spvenums.OpLabel, 2),
				instWords(spvenums.OpReturn),
				instWords(spvenums.OpFunctionEnd),
			}
			module, _, err := parseWords(t, Options{}, groups...)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if len(module.EntryPoints) != 1 {
				t.Fatalf("entry points = %d, want 1", len(module.EntryPoints))
			}
			if arguments := module.EntryPoints[0].Function.Arguments; len(arguments) != 1 {
				t.Fatalf("wrapper arguments = %d, want 1", len(arguments))
			}
			tc.check(t, module, module.EntryPoints[0].Function.Arguments[0].Type)
		})
	}
}
