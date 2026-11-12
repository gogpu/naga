package front

import (
	"strconv"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// typeDefinition asserts a type id was parsed to the expected handle and
// returns the type behind it.
func typeDefinition(t *testing.T, module *ir.Module, id uint32, p *parser) ir.Type {
	t.Helper()
	lookup, ok := p.lookupType[id]
	if !ok {
		t.Fatalf("type id %%%d was not recorded", id)
	}
	if int(lookup.handle) >= len(module.Types) {
		t.Fatalf("type handle %d is outside the arena", lookup.handle)
	}
	return module.Types[lookup.handle]
}

func TestParseTypeImage(t *testing.T) {
	t.Run("sampled 2D", func(t *testing.T) {
		p := newParser(spvWords(headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpTypeFloat, 1, 32),
			instWords(spvenums.OpTypeImage, 2, 1, uint32(spvenums.Dim2D), 0, 0, 0, 1, 0),
		), Options{})
		module, _, err := p.parse()
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		image, ok := typeDefinition(t, module, 2, p).Inner.(ir.ImageType)
		if !ok {
			t.Fatalf("type 2 = %T, want an image", typeDefinition(t, module, 2, p).Inner)
		}
		if image.Class != ir.ImageClassSampled || image.Dim != ir.Dim2D || image.SampledKind != ir.ScalarFloat {
			t.Errorf("image = %#v, want a sampled 2D image of f32", image)
		}
		if image.Arrayed || image.Multisampled {
			t.Errorf("image = %#v, want neither arrayed nor multisampled", image)
		}
	})

	t.Run("depth multisampled", func(t *testing.T) {
		p := newParser(spvWords(headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpTypeFloat, 1, 32),
			instWords(spvenums.OpTypeImage, 2, 1, uint32(spvenums.Dim2D), 1, 0, 1, 1, 0),
		), Options{})
		module, _, err := p.parse()
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		image := typeDefinition(t, module, 2, p).Inner.(ir.ImageType)
		if image.Class != ir.ImageClassDepth || !image.Multisampled {
			t.Errorf("image = %#v, want a multisampled depth image", image)
		}
	})

	t.Run("storage with format", func(t *testing.T) {
		p := newParser(spvWords(headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpTypeFloat, 1, 32),
			instWords(spvenums.OpTypeImage, 2, 1, uint32(spvenums.Dim2D), 0, 0, 0, 2, uint32(spvenums.ImageFormatRgba8)),
		), Options{})
		module, _, err := p.parse()
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		image := typeDefinition(t, module, 2, p).Inner.(ir.ImageType)
		if image.Class != ir.ImageClassStorage || image.StorageFormat != ir.StorageFormatRgba8Unorm {
			t.Errorf("image = %#v, want a storage image with Rgba8Unorm", image)
		}
	})

	t.Run("storage without format", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpTypeFloat, 1, 32),
			instWords(spvenums.OpTypeImage, 2, 1, uint32(spvenums.Dim2D), 0, 0, 0, 2, 0),
		)
		if got := requireError(t, err).Message; got == "" {
			t.Error("a storage image without a format was accepted")
		}
	})

	t.Run("depth and storage", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpTypeFloat, 1, 32),
			instWords(spvenums.OpTypeImage, 2, 1, uint32(spvenums.Dim2D), 1, 0, 0, 2, uint32(spvenums.ImageFormatRgba8)),
		)
		if got := requireError(t, err).Message; got != "cannot create a OpTypeImage as both a depth and storage image" {
			t.Errorf("message = %q", got)
		}
	})

	t.Run("unsupported dimension", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpTypeFloat, 1, 32),
			instWords(spvenums.OpTypeImage, 2, 1, uint32(spvenums.DimRect), 0, 0, 0, 1, 0),
		)
		if got := requireError(t, err).Message; got != "unsupported image dimension %"+strconv.Itoa(int(spvenums.DimRect)) {
			t.Errorf("message = %q", got)
		}
	})

	t.Run("invalid base type", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpTypeStruct, 1),
			instWords(spvenums.OpTypeImage, 2, 1, uint32(spvenums.Dim2D), 0, 0, 0, 1, 0),
		)
		if got := requireError(t, err).Message; !strings.Contains(got, "invalid image base type") {
			t.Errorf("message = %q", got)
		}
	})
}

func TestParseTypeSamplerAndSampledImage(t *testing.T) {
	p := newParser(spvWords(headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpMemoryModel, 0, 1),
		instWords(spvenums.OpTypeFloat, 1, 32),
		instWords(spvenums.OpTypeImage, 2, 1, uint32(spvenums.Dim2D), 0, 0, 0, 1, 0),
		instWords(spvenums.OpTypeSampledImage, 3, 2),
		instWords(spvenums.OpTypeSampler, 4),
		instWords(spvenums.OpTypePointer, 5, uint32(spvenums.StorageClassUniformConstant), 3),
	), Options{})
	module, _, err := p.parse()
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	image := typeDefinition(t, module, 2, p)
	sampled := typeDefinition(t, module, 3, p)
	if sampled.Inner != image.Inner {
		t.Errorf("sampled image type = %#v, want the image type", sampled.Inner)
	}
	sampler, ok := typeDefinition(t, module, 4, p).Inner.(ir.SamplerType)
	if !ok || sampler.Comparison {
		t.Errorf("sampler = %#v, want a non-comparison sampler", typeDefinition(t, module, 4, p).Inner)
	}
	// A pointer in the handle space is dropped: the image type stands for it.
	pointer := typeDefinition(t, module, 5, p)
	if pointer.Inner != image.Inner {
		t.Errorf("handle pointer = %#v, want the image type", pointer.Inner)
	}
}

func TestParseTypeRuntimeArray(t *testing.T) {
	t.Run("of a scalar", func(t *testing.T) {
		p := newParser(spvWords(headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpTypeFloat, 1, 32),
			instWords(spvenums.OpTypeRuntimeArray, 2, 1),
			instWords(spvenums.OpTypeStruct, 3, 2),
			instWords(spvenums.OpTypePointer, 4, uint32(spvenums.StorageClassStorageBuffer), 3),
		), Options{})
		module, _, err := p.parse()
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		array, ok := typeDefinition(t, module, 2, p).Inner.(ir.ArrayType)
		if !ok || array.Size.Constant != nil || array.Stride != 4 {
			t.Errorf("runtime array = %#v, want a dynamic array with stride 4", typeDefinition(t, module, 2, p).Inner)
		}
	})

	t.Run("of an image", func(t *testing.T) {
		p := newParser(spvWords(headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpTypeFloat, 1, 32),
			instWords(spvenums.OpTypeImage, 2, 1, uint32(spvenums.Dim2D), 0, 0, 0, 1, 0),
			instWords(spvenums.OpTypeRuntimeArray, 3, 2),
		), Options{})
		module, _, err := p.parse()
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		bindingArray, ok := typeDefinition(t, module, 3, p).Inner.(ir.BindingArrayType)
		if !ok || bindingArray.Size != nil {
			t.Errorf("runtime image array = %#v, want an unbounded binding array", typeDefinition(t, module, 3, p).Inner)
		}
	})

	t.Run("outside storage", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpTypeFloat, 1, 32),
			instWords(spvenums.OpTypeRuntimeArray, 2, 1),
			instWords(spvenums.OpTypePointer, 3, uint32(spvenums.StorageClassUniform), 2),
		)
		if got := requireError(t, err).Message; got != errUnsupportedRuntimeArrayStorageClass.Message {
			t.Errorf("message = %q", got)
		}
	})
}

func TestParseTypeMistakes(t *testing.T) {
	cases := []struct {
		name string
		ops  [][]uint32
		want string
	}{
		{
			name: "integer sign",
			ops:  [][]uint32{instWords(spvenums.OpTypeInt, 1, 32, 2)},
			want: "invalid sign %2",
		},
		{
			name: "integer width",
			ops:  [][]uint32{instWords(spvenums.OpTypeInt, 1, 4096, 1)},
			want: "invalid type width %4096",
		},
		{
			name: "vector size",
			ops: [][]uint32{
				instWords(spvenums.OpTypeFloat, 1, 32),
				instWords(spvenums.OpTypeVector, 2, 1, 5),
			},
			want: "invalid vector size %5",
		},
		{
			name: "vector of a vector",
			ops: [][]uint32{
				instWords(spvenums.OpTypeFloat, 1, 32),
				instWords(spvenums.OpTypeVector, 2, 1, 2),
				instWords(spvenums.OpTypeVector, 3, 2, 2),
			},
			want: "invalid inner type %2",
		},
		{
			name: "matrix columns",
			ops: [][]uint32{
				instWords(spvenums.OpTypeFloat, 1, 32),
				instWords(spvenums.OpTypeVector, 2, 1, 2),
				instWords(spvenums.OpTypeMatrix, 3, 2, 5),
			},
			want: "invalid vector size %5",
		},
		{
			name: "matrix of a scalar",
			ops: [][]uint32{
				instWords(spvenums.OpTypeFloat, 1, 32),
				instWords(spvenums.OpTypeMatrix, 2, 1, 2),
			},
			want: "invalid inner type %1",
		},
		{
			name: "unknown type id",
			ops:  [][]uint32{instWords(spvenums.OpTypeVector, 1, 99, 2)},
			want: "invalid id %99",
		},
		{
			name: "unsupported storage class",
			ops: [][]uint32{
				instWords(spvenums.OpTypeInt, 1, 32, 1),
				instWords(spvenums.OpTypePointer, 2, uint32(spvenums.StorageClassAtomicCounter), 1),
			},
			want: "unsupported storage class %" + strconv.Itoa(int(spvenums.StorageClassAtomicCounter)),
		},
		{
			name: "matrix stride",
			ops: [][]uint32{
				instWords(spvenums.OpMemberDecorate, 4, 0, uint32(spvenums.DecorationMatrixStride), 8),
				instWords(spvenums.OpTypeFloat, 1, 32),
				instWords(spvenums.OpTypeVector, 2, 1, 4),
				instWords(spvenums.OpTypeMatrix, 3, 2, 4),
				instWords(spvenums.OpTypeStruct, 4, 3),
			},
			want: "unsupported matrix stride 8 for a 4x4 matrix with scalar width=4",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			groups := [][]uint32{
				headerWords(),
				instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
				instWords(spvenums.OpMemoryModel, 0, 1),
			}
			groups = append(groups, tc.ops...)
			_, _, err := parseWords(t, Options{}, groups...)
			if got := requireError(t, err).Message; got != tc.want {
				t.Errorf("message = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseTypeFunctionAndArrayStride(t *testing.T) {
	p := newParser(spvWords(headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpMemoryModel, 0, 1),
		instWords(spvenums.OpDecorate, 3, uint32(spvenums.DecorationArrayStride), 16),
		instWords(spvenums.OpTypeFloat, 1, 32),
		instWords(spvenums.OpTypeVector, 2, 1, 4),
		instWords(spvenums.OpTypeInt, 4, 32, 0),
		instWords(spvenums.OpConstant, 4, 6, 4),
		instWords(spvenums.OpTypeArray, 3, 2, 6),
		instWords(spvenums.OpTypeFunction, 5, 1, 2, 1),
	), Options{})
	module, _, err := p.parse()
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	array, ok := typeDefinition(t, module, 3, p).Inner.(ir.ArrayType)
	if !ok || array.Stride != 16 || array.Size.Constant == nil || *array.Size.Constant != 4 {
		t.Errorf("array = %#v, want four vec4 with stride 16", typeDefinition(t, module, 3, p).Inner)
	}
	functionType, ok := p.lookupFunctionType[5]
	if !ok || functionType.returnTypeID != 1 || len(functionType.parameterTypeIDs) != 2 {
		t.Errorf("function type = %#v, want void(vec4, f32)", functionType)
	}
}

func TestParseStructMemberBindingAndStorageAccess(t *testing.T) {
	p := newParser(spvWords(headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpMemoryModel, 0, 1),
		instWords(spvenums.OpDecorate, 4, uint32(spvenums.DecorationBufferBlock)),
		instWords(spvenums.OpDecorate, 4, uint32(spvenums.DecorationBlock)),
		instWords(spvenums.OpMemberDecorate, 4, 0, uint32(spvenums.DecorationOffset), 0),
		instWords(spvenums.OpMemberDecorate, 4, 0, uint32(spvenums.DecorationNonWritable)),
		instWords(spvenums.OpMemberDecorate, 4, 1, uint32(spvenums.DecorationOffset), 16),
		instWords(spvenums.OpMemberDecorate, 4, 1, uint32(spvenums.DecorationNonWritable)),
		instWords(spvenums.OpMemberDecorate, 4, 1, uint32(spvenums.DecorationBuiltIn), uint32(spvenums.BuiltInPosition)),
		instWords(spvenums.OpTypeFloat, 1, 32),
		instWords(spvenums.OpTypeVector, 2, 1, 4),
		instWords(spvenums.OpTypeInt, 3, 32, 0),
		instWords(spvenums.OpTypeStruct, 4, 3, 2),
		instWords(spvenums.OpTypePointer, 5, uint32(spvenums.StorageClassStorageBuffer), 4),
		instWords(spvenums.OpVariable, 5, 6, uint32(spvenums.StorageClassStorageBuffer)),
	), Options{})
	module, _, err := p.parse()
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	structType, ok := typeDefinition(t, module, 4, p).Inner.(ir.StructType)
	if !ok || len(structType.Members) != 2 {
		t.Fatalf("struct = %#v, want two members", typeDefinition(t, module, 4, p).Inner)
	}
	if structType.Members[1].Offset != 16 || structType.Members[1].Binding == nil {
		t.Errorf("member 1 = %#v, want offset 16 with a position binding", structType.Members[1])
	}
	if binding, ok := (*structType.Members[1].Binding).(ir.BuiltinBinding); !ok || binding.Builtin != ir.BuiltinPosition {
		t.Errorf("member 1 binding = %#v, want position", structType.Members[1].Binding)
	}
	// The BufferBlock access comes from the members: both are NonWritable, so
	// the struct is read-only.
	global := module.GlobalVariables[0]
	if global.Space != ir.SpaceStorage {
		t.Errorf("global space = %v, want storage", global.Space)
	}
	if global.Access != ir.StorageAccessLoad {
		t.Errorf("global access = %d, want load only", global.Access)
	}
}
