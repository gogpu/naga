package front

import (
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// parseEntryPointModule parses a module and returns the wrapper of its first
// entry point.
func parseEntryPointModule(t *testing.T, options Options, groups ...[]uint32) (*ir.Module, *ir.EntryPoint) {
	t.Helper()
	module, _, err := parseWords(t, options, groups...)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(module.EntryPoints) != 1 {
		t.Fatalf("entry points = %d, want 1", len(module.EntryPoints))
	}
	return module, &module.EntryPoints[0]
}

// voidFunctionBody is the shared body of an entry point's parsed function.
func voidFunctionBody() []uint32 {
	return instWords(spvenums.OpFunction, 10, 1, 0, 11)
}

// entryPointOp builds an OpEntryPoint for the function 1 named main.
func entryPointOp(model spvenums.ExecutionModel, variables ...uint32) []uint32 {
	words := []uint32{uint32(model), 1}
	words = append(words, stringWords("main")...)
	words = append(words, variables...)
	return instWords(spvenums.OpEntryPoint, words...)
}

func TestWrapperWithoutIO(t *testing.T) {
	_, ep := parseEntryPointModule(t, Options{},
		headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpMemoryModel, 0, 1),
		entryPointOp(spvenums.ExecutionModelGLCompute),
		instWords(spvenums.OpExecutionMode, 1, uint32(spvenums.ExecutionModeLocalSize), 1, 1, 1),
		instWords(spvenums.OpTypeVoid, 10),
		instWords(spvenums.OpTypeFunction, 11, 10),
		voidFunctionBody(),
		instWords(spvenums.OpLabel, 2),
		instWords(spvenums.OpReturn),
		instWords(spvenums.OpFunctionEnd),
	)
	if ep.Name != "main" || ep.Stage != ir.StageCompute {
		t.Errorf("entry point = %v %q", ep.Stage, ep.Name)
	}
	if ep.Function.Name != "main_wrap" {
		t.Errorf("wrapper name = %q, want main_wrap", ep.Function.Name)
	}
	if ep.Function.Result != nil || len(ep.Function.Arguments) != 0 {
		t.Errorf("wrapper has arguments or a result: %#v", ep.Function)
	}
	if !hasStatement(&ep.Function, func(k ir.StatementKind) bool {
		_, ok := k.(ir.StmtCall)
		return ok
	}) {
		t.Error("the wrapper does not call the parsed function")
	}
}

func TestWrapperOutputs(t *testing.T) {
	t.Run("one location output", func(t *testing.T) {
		_, ep := parseEntryPointModule(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			entryPointOp(spvenums.ExecutionModelFragment, 3),
			instWords(spvenums.OpDecorate, 3, uint32(spvenums.DecorationLocation), 0),
			instWords(spvenums.OpTypeVoid, 10),
			instWords(spvenums.OpTypeFunction, 11, 10),
			instWords(spvenums.OpTypeFloat, 12, 32),
			instWords(spvenums.OpTypeVector, 13, 12, 4),
			instWords(spvenums.OpTypePointer, 14, uint32(spvenums.StorageClassOutput), 13),
			instWords(spvenums.OpVariable, 14, 3, uint32(spvenums.StorageClassOutput)),
			voidFunctionBody(),
			instWords(spvenums.OpLabel, 2),
			instWords(spvenums.OpReturn),
			instWords(spvenums.OpFunctionEnd),
		)
		if ep.Function.Result == nil || ep.Function.Result.Binding == nil {
			t.Fatal("the wrapper has no bound result")
		}
		location, ok := (*ep.Function.Result.Binding).(ir.LocationBinding)
		if !ok || location.Location != 0 {
			t.Errorf("result binding = %#v, want location 0", ep.Function.Result.Binding)
		}
	})

	t.Run("position is flipped by default", func(t *testing.T) {
		body := func(t *testing.T, options Options) *ir.EntryPoint {
			_, ep := parseEntryPointModule(t, options,
				headerWords(),
				instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
				instWords(spvenums.OpMemoryModel, 0, 1),
				entryPointOp(spvenums.ExecutionModelVertex, 3),
				instWords(spvenums.OpDecorate, 3, uint32(spvenums.DecorationBuiltIn), uint32(spvenums.BuiltInPosition)),
				instWords(spvenums.OpTypeVoid, 10),
				instWords(spvenums.OpTypeFunction, 11, 10),
				instWords(spvenums.OpTypeFloat, 12, 32),
				instWords(spvenums.OpTypeVector, 13, 12, 4),
				instWords(spvenums.OpTypePointer, 14, uint32(spvenums.StorageClassOutput), 13),
				instWords(spvenums.OpVariable, 14, 3, uint32(spvenums.StorageClassOutput)),
				voidFunctionBody(),
				instWords(spvenums.OpLabel, 2),
				instWords(spvenums.OpReturn),
				instWords(spvenums.OpFunctionEnd),
			)
			return ep
		}
		flipped := body(t, Options{})
		if !hasStatement(&flipped.Function, func(k ir.StatementKind) bool {
			_, ok := k.(ir.StmtStore)
			return ok
		}) {
			t.Error("the default wrapper does not flip the position")
		}
		kept := body(t, Options{KeepCoordinateSpace: true})
		if hasStatement(&kept.Function, func(k ir.StatementKind) bool {
			_, ok := k.(ir.StmtStore)
			return ok
		}) {
			t.Error("KeepCoordinateSpace still flipped the position")
		}
	})

	t.Run("two outputs become a result struct", func(t *testing.T) {
		module, ep := parseEntryPointModule(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			entryPointOp(spvenums.ExecutionModelVertex, 3, 4),
			instWords(spvenums.OpDecorate, 3, uint32(spvenums.DecorationBuiltIn), uint32(spvenums.BuiltInPosition)),
			instWords(spvenums.OpDecorate, 4, uint32(spvenums.DecorationLocation), 0),
			instWords(spvenums.OpTypeVoid, 10),
			instWords(spvenums.OpTypeFunction, 11, 10),
			instWords(spvenums.OpTypeFloat, 12, 32),
			instWords(spvenums.OpTypeVector, 13, 12, 4),
			instWords(spvenums.OpTypePointer, 14, uint32(spvenums.StorageClassOutput), 13),
			instWords(spvenums.OpVariable, 14, 3, uint32(spvenums.StorageClassOutput)),
			instWords(spvenums.OpVariable, 14, 4, uint32(spvenums.StorageClassOutput)),
			voidFunctionBody(),
			instWords(spvenums.OpLabel, 2),
			instWords(spvenums.OpReturn),
			instWords(spvenums.OpFunctionEnd),
		)
		if ep.Function.Result == nil {
			t.Fatal("the wrapper has no result")
		}
		structType, ok := module.Types[ep.Function.Result.Type].Inner.(ir.StructType)
		if !ok || len(structType.Members) != 2 {
			t.Fatalf("result type = %#v, want a struct of two members", module.Types[ep.Function.Result.Type].Inner)
		}
		if structType.Members[0].Offset != 0 || structType.Members[1].Offset != 16 {
			t.Errorf("member offsets = %d, %d; want 0, 16", structType.Members[0].Offset, structType.Members[1].Offset)
		}
		if structType.Span != 32 {
			t.Errorf("span = %d, want 32", structType.Span)
		}
		for i, member := range structType.Members {
			if member.Binding == nil {
				t.Errorf("member %d has no binding", i)
			}
		}
		if _, ok := findExpression(&ep.Function, func(k ir.ExpressionKind) bool {
			_, ok := k.(ir.ExprCompose)
			return ok
		}); !ok {
			t.Error("the wrapper does not compose its result")
		}
	})

	t.Run("output struct with builtins", func(t *testing.T) {
		module, ep := parseEntryPointModule(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			entryPointOp(spvenums.ExecutionModelVertex, 3),
			instWords(spvenums.OpMemberDecorate, 12, 0, uint32(spvenums.DecorationBuiltIn), uint32(spvenums.BuiltInPosition)),
			instWords(spvenums.OpMemberDecorate, 12, 1, uint32(spvenums.DecorationBuiltIn), uint32(spvenums.BuiltInPointSize)),
			instWords(spvenums.OpTypeVoid, 10),
			instWords(spvenums.OpTypeFunction, 11, 10),
			instWords(spvenums.OpTypeFloat, 13, 32),
			instWords(spvenums.OpTypeVector, 14, 13, 4),
			instWords(spvenums.OpTypeStruct, 12, 14, 13),
			instWords(spvenums.OpTypePointer, 15, uint32(spvenums.StorageClassOutput), 12),
			instWords(spvenums.OpVariable, 15, 3, uint32(spvenums.StorageClassOutput)),
			voidFunctionBody(),
			instWords(spvenums.OpLabel, 2),
			instWords(spvenums.OpReturn),
			instWords(spvenums.OpFunctionEnd),
		)
		if ep.Function.Result == nil {
			t.Fatal("the wrapper has no result")
		}
		structType, ok := module.Types[ep.Function.Result.Type].Inner.(ir.StructType)
		if !ok || len(structType.Members) != 2 {
			t.Fatalf("result type = %#v, want two members", module.Types[ep.Function.Result.Type].Inner)
		}
		if structType.Members[1].Offset != 16 {
			t.Errorf("second member offset = %d, want 16", structType.Members[1].Offset)
		}
		// The output global got default values for both built-ins.
		global := module.GlobalVariables[0]
		if global.InitExpr == nil {
			t.Fatal("the output struct has no default initial value")
		}
		if _, ok := module.GlobalExpressions[*global.InitExpr].Kind.(ir.ExprCompose); !ok {
			t.Errorf("default init = %#v, want a compose", module.GlobalExpressions[*global.InitExpr].Kind)
		}
	})
}

// namedEntryPointOp builds an OpEntryPoint with a name for a function. Entry
// points are keyed by function, so two entry points need two functions.
func namedEntryPointOp(model spvenums.ExecutionModel, function uint32, name string, variables ...uint32) []uint32 {
	words := []uint32{uint32(model), function}
	words = append(words, stringWords(name)...)
	words = append(words, variables...)
	return instWords(spvenums.OpEntryPoint, words...)
}

// locationInterpolation returns the interpolation of a location binding.
func locationInterpolation(t *testing.T, binding *ir.Binding) *ir.Interpolation {
	t.Helper()
	if binding == nil {
		t.Fatal("no binding")
	}
	location, ok := (*binding).(ir.LocationBinding)
	if !ok {
		t.Fatalf("binding = %#v, want a location", *binding)
	}
	return location.Interpolation
}

// TestDefaultInterpolationDoesNotLeak: the default interpolation is applied to
// a copy of the binding, as Rust naga clones it (front/spv/function.rs, the
// entry point wrapper). Writing through the shared binding gave a vertex
// argument the interpolation a fragment wrapper had set, and changed the
// members of a stored output struct type.
func TestDefaultInterpolationDoesNotLeak(t *testing.T) {
	t.Run("input shared by a fragment and a vertex entry point", func(t *testing.T) {
		module, _, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			namedEntryPointOp(spvenums.ExecutionModelFragment, 1, "fs", 3),
			namedEntryPointOp(spvenums.ExecutionModelVertex, 5, "vs", 3),
			instWords(spvenums.OpDecorate, 3, uint32(spvenums.DecorationLocation), 0),
			instWords(spvenums.OpTypeVoid, 10),
			instWords(spvenums.OpTypeFunction, 11, 10),
			instWords(spvenums.OpTypeFloat, 12, 32),
			instWords(spvenums.OpTypeVector, 13, 12, 4),
			instWords(spvenums.OpTypePointer, 14, uint32(spvenums.StorageClassInput), 13),
			instWords(spvenums.OpVariable, 14, 3, uint32(spvenums.StorageClassInput)),
			voidFunctionBody(),
			instWords(spvenums.OpLabel, 2),
			instWords(spvenums.OpReturn),
			instWords(spvenums.OpFunctionEnd),
			instWords(spvenums.OpFunction, 10, 5, 0, 11),
			instWords(spvenums.OpLabel, 6),
			instWords(spvenums.OpReturn),
			instWords(spvenums.OpFunctionEnd),
		)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if len(module.EntryPoints) != 2 {
			t.Fatalf("entry points = %d, want 2", len(module.EntryPoints))
		}
		for _, ep := range module.EntryPoints {
			if len(ep.Function.Arguments) != 1 {
				t.Fatalf("%s arguments = %d, want 1", ep.Name, len(ep.Function.Arguments))
			}
			interpolation := locationInterpolation(t, ep.Function.Arguments[0].Binding)
			switch ep.Stage {
			case ir.StageFragment:
				if interpolation == nil || *interpolation != ir.InterpolationPerspective {
					t.Errorf("fragment interpolation = %v, want perspective", interpolation)
				}
			case ir.StageVertex:
				if interpolation != nil {
					t.Errorf("vertex interpolation = %v, want none", *interpolation)
				}
			}
		}
	})

	t.Run("output struct type stays unchanged", func(t *testing.T) {
		module, ep := parseEntryPointModule(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			entryPointOp(spvenums.ExecutionModelVertex, 3),
			instWords(spvenums.OpMemberDecorate, 12, 0, uint32(spvenums.DecorationBuiltIn), uint32(spvenums.BuiltInPosition)),
			instWords(spvenums.OpMemberDecorate, 12, 1, uint32(spvenums.DecorationLocation), 0),
			instWords(spvenums.OpTypeVoid, 10),
			instWords(spvenums.OpTypeFunction, 11, 10),
			instWords(spvenums.OpTypeFloat, 13, 32),
			instWords(spvenums.OpTypeVector, 14, 13, 4),
			instWords(spvenums.OpTypeStruct, 12, 14, 14),
			instWords(spvenums.OpTypePointer, 15, uint32(spvenums.StorageClassOutput), 12),
			instWords(spvenums.OpVariable, 15, 3, uint32(spvenums.StorageClassOutput)),
			voidFunctionBody(),
			instWords(spvenums.OpLabel, 2),
			instWords(spvenums.OpReturn),
			instWords(spvenums.OpFunctionEnd),
		)
		stored, ok := module.Types[module.GlobalVariables[0].Type].Inner.(ir.StructType)
		if !ok || len(stored.Members) != 2 {
			t.Fatalf("output global type = %#v, want a struct of two members", module.Types[module.GlobalVariables[0].Type].Inner)
		}
		if interpolation := locationInterpolation(t, stored.Members[1].Binding); interpolation != nil {
			t.Errorf("stored struct member interpolation = %v, want none", *interpolation)
		}
		result, ok := module.Types[ep.Function.Result.Type].Inner.(ir.StructType)
		if !ok || len(result.Members) != 2 {
			t.Fatalf("result type = %#v, want a struct of two members", module.Types[ep.Function.Result.Type].Inner)
		}
		if interpolation := locationInterpolation(t, result.Members[1].Binding); interpolation == nil || *interpolation != ir.InterpolationPerspective {
			t.Errorf("result member interpolation = %v, want perspective", interpolation)
		}
	})
}

func TestWrapperInputs(t *testing.T) {
	t.Run("fragment location gets default interpolation", func(t *testing.T) {
		_, ep := parseEntryPointModule(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			entryPointOp(spvenums.ExecutionModelFragment, 3),
			instWords(spvenums.OpDecorate, 3, uint32(spvenums.DecorationLocation), 0),
			instWords(spvenums.OpTypeVoid, 10),
			instWords(spvenums.OpTypeFunction, 11, 10),
			instWords(spvenums.OpTypeFloat, 12, 32),
			instWords(spvenums.OpTypeVector, 13, 12, 4),
			instWords(spvenums.OpTypePointer, 14, uint32(spvenums.StorageClassInput), 13),
			instWords(spvenums.OpVariable, 14, 3, uint32(spvenums.StorageClassInput)),
			voidFunctionBody(),
			instWords(spvenums.OpLabel, 2),
			instWords(spvenums.OpReturn),
			instWords(spvenums.OpFunctionEnd),
		)
		if len(ep.Function.Arguments) != 1 || ep.Function.Arguments[0].Binding == nil {
			t.Fatalf("arguments = %#v, want one bound argument", ep.Function.Arguments)
		}
		location, ok := (*ep.Function.Arguments[0].Binding).(ir.LocationBinding)
		if !ok {
			t.Fatalf("argument binding = %#v", ep.Function.Arguments[0].Binding)
		}
		if location.Interpolation == nil || *location.Interpolation != ir.InterpolationPerspective {
			t.Errorf("interpolation = %v, want perspective", location.Interpolation)
		}
		if location.Sampling == nil || *location.Sampling != ir.SamplingCenter {
			t.Errorf("sampling = %v, want center", location.Sampling)
		}
		if !hasStatement(&ep.Function, func(k ir.StatementKind) bool {
			_, ok := k.(ir.StmtStore)
			return ok
		}) {
			t.Error("the wrapper does not copy the input into its private global")
		}
	})

	t.Run("integer builtin input is read as unsigned", func(t *testing.T) {
		module, ep := parseEntryPointModule(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			entryPointOp(spvenums.ExecutionModelVertex, 3),
			instWords(spvenums.OpDecorate, 3, uint32(spvenums.DecorationBuiltIn), uint32(spvenums.BuiltInBaseInstance)),
			instWords(spvenums.OpTypeVoid, 10),
			instWords(spvenums.OpTypeFunction, 11, 10),
			instWords(spvenums.OpTypeInt, 12, 32, 1),
			instWords(spvenums.OpTypePointer, 14, uint32(spvenums.StorageClassInput), 12),
			instWords(spvenums.OpVariable, 14, 3, uint32(spvenums.StorageClassInput)),
			voidFunctionBody(),
			instWords(spvenums.OpLabel, 2),
			instWords(spvenums.OpReturn),
			instWords(spvenums.OpFunctionEnd),
		)
		if len(ep.Function.Arguments) != 1 {
			t.Fatalf("arguments = %d, want 1", len(ep.Function.Arguments))
		}
		scalar, ok := module.Types[ep.Function.Arguments[0].Type].Inner.(ir.ScalarType)
		if !ok || scalar.Kind != ir.ScalarUint || scalar.Width != 4 {
			t.Errorf("argument type = %#v, want u32", module.Types[ep.Function.Arguments[0].Type].Inner)
		}
		if _, ok := findExpression(&ep.Function, func(k ir.ExpressionKind) bool {
			as, ok := k.(ir.ExprAs)
			return ok && as.Kind == ir.ScalarSint
		}); !ok {
			t.Error("the wrapper does not convert the unsigned argument back to signed")
		}
	})
}

func TestOutputDefaults(t *testing.T) {
	cases := []struct {
		name    string
		typeID  uint32
		typeOps [][]uint32
		builtin spvenums.BuiltIn
		check   func(t *testing.T, module *ir.Module, global ir.GlobalVariable)
	}{
		{
			name:   "position",
			typeID: 14,
			typeOps: [][]uint32{
				instWords(spvenums.OpTypeFloat, 12, 32),
				instWords(spvenums.OpTypeVector, 14, 12, 4),
			},
			builtin: spvenums.BuiltInPosition,
			check: func(t *testing.T, module *ir.Module, global ir.GlobalVariable) {
				compose, ok := module.GlobalExpressions[*global.InitExpr].Kind.(ir.ExprCompose)
				if !ok || len(compose.Components) != 4 {
					t.Errorf("position default = %#v, want a vec4 compose", module.GlobalExpressions[*global.InitExpr].Kind)
				}
			},
		},
		{
			name:    "point size",
			typeID:  12,
			typeOps: [][]uint32{instWords(spvenums.OpTypeFloat, 12, 32)},
			builtin: spvenums.BuiltInPointSize,
			check: func(t *testing.T, module *ir.Module, global ir.GlobalVariable) {
				literal, ok := module.GlobalExpressions[*global.InitExpr].Kind.(ir.Literal)
				if !ok || literal.Value != ir.LiteralF32(1) {
					t.Errorf("point size default = %#v, want 1.0", module.GlobalExpressions[*global.InitExpr].Kind)
				}
			},
		},
		{
			name:    "frag depth",
			typeID:  12,
			typeOps: [][]uint32{instWords(spvenums.OpTypeFloat, 12, 32)},
			builtin: spvenums.BuiltInFragDepth,
			check: func(t *testing.T, module *ir.Module, global ir.GlobalVariable) {
				literal, ok := module.GlobalExpressions[*global.InitExpr].Kind.(ir.Literal)
				if !ok || literal.Value != ir.LiteralF32(0) {
					t.Errorf("frag depth default = %#v, want 0.0", module.GlobalExpressions[*global.InitExpr].Kind)
				}
			},
		},
		{
			name:    "sample mask",
			typeID:  15,
			typeOps: [][]uint32{instWords(spvenums.OpTypeInt, 15, 32, 0)},
			builtin: spvenums.BuiltInSampleMask,
			check: func(t *testing.T, module *ir.Module, global ir.GlobalVariable) {
				literal, ok := module.GlobalExpressions[*global.InitExpr].Kind.(ir.Literal)
				if !ok || literal.Value != ir.LiteralU32(0xffffffff) {
					t.Errorf("sample mask default = %#v, want 0xffffffff", module.GlobalExpressions[*global.InitExpr].Kind)
				}
			},
		},
		{
			name:   "clip distances",
			typeID: 16,
			typeOps: [][]uint32{
				instWords(spvenums.OpTypeFloat, 12, 32),
				instWords(spvenums.OpTypeInt, 15, 32, 0),
				instWords(spvenums.OpConstant, 15, 17, 1),
				instWords(spvenums.OpTypeArray, 16, 12, 17),
			},
			builtin: spvenums.BuiltInClipDistance,
			check: func(t *testing.T, module *ir.Module, global ir.GlobalVariable) {
				if _, ok := module.GlobalExpressions[*global.InitExpr].Kind.(ir.ExprZeroValue); !ok {
					t.Errorf("clip distance default = %#v, want a zero value", module.GlobalExpressions[*global.InitExpr].Kind)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			groups := [][]uint32{
				headerWords(),
				instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
				instWords(spvenums.OpMemoryModel, 0, 1),
				instWords(spvenums.OpDecorate, 3, uint32(spvenums.DecorationBuiltIn), uint32(tc.builtin)),
			}
			groups = append(groups, tc.typeOps...)
			groups = append(groups,
				instWords(spvenums.OpTypePointer, 19, uint32(spvenums.StorageClassOutput), tc.typeID),
				instWords(spvenums.OpVariable, 19, 3, uint32(spvenums.StorageClassOutput)),
			)
			module, _, err := parseWords(t, Options{}, groups...)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			global := module.GlobalVariables[0]
			if global.InitExpr == nil {
				t.Fatal("the output has no default value")
			}
			tc.check(t, module, global)
		})
	}
}

func TestWrapperReferencesUnknownVariable(t *testing.T) {
	_, _, err := parseWords(t, Options{},
		headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpMemoryModel, 0, 1),
		entryPointOp(spvenums.ExecutionModelGLCompute, 99),
		instWords(spvenums.OpExecutionMode, 1, uint32(spvenums.ExecutionModeLocalSize), 1, 1, 1),
		instWords(spvenums.OpTypeVoid, 10),
		instWords(spvenums.OpTypeFunction, 11, 10),
		voidFunctionBody(),
		instWords(spvenums.OpLabel, 2),
		instWords(spvenums.OpReturn),
		instWords(spvenums.OpFunctionEnd),
	)
	want := "invalid id %99"
	if got := requireError(t, err).Message; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}
