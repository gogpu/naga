package snapshot_test

import (
	"errors"
	"io/fs"
	"os"
	"regexp"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/ir"
)

// dxilAtNeededModel compiles as dxil.Compile does and, when the entry point
// needs a newer shader model than opts asks for, compiles again at that
// model: the corpora check every shader at the lowest model it can take, as
// a device that supports it would be asked for (dxil.Compile refuses instead
// of raising the model).
func dxilAtNeededModel(module *ir.Module, opts dxil.Options) ([]byte, error) {
	blob, _, err := dxil.Compile(module, opts)
	var vErr *backend.VersionError
	if errors.As(err, &vErr) && vErr.Space == backend.VersionShaderModel {
		opts.ShaderModel = backend.ShaderModel(vErr.Required)
		blob, _, err = dxil.Compile(module, opts)
	}
	return blob, err
}

// withRustBindings returns opts with the resource placement of the shader's
// Rust HLSL parameters: its [[hlsl.binding_map]] (registers, spaces, binding
// array sizes, dynamic offset indices), sampler heap and sampler index buffer
// targets, dynamic storage buffer offset buffers, immediates target, and Rust's
// fake_missing_bindings default for the rest. These are the bindings Rust's
// HLSL test output is written for; a shader whose binding arrays overlap at
// their WGSL bindings (binding-arrays) needs them, as a pipeline layout would
// give them.
func withRustBindings(t *testing.T, shaderName string, opts dxil.Options) dxil.Options {
	t.Helper()
	h, err := rustHLSLOptions(shaderName)
	if err != nil {
		t.Fatalf("%s: Rust HLSL parameters: %v", shaderName, err)
	}
	opts.FakeMissingBindings = h.FakeMissingBindings
	if len(h.BindingMap) > 0 {
		opts.BindingMap = make(dxil.BindingMap, len(h.BindingMap))
		for rb, bt := range h.BindingMap {
			opts.BindingMap[rb] = dxil.BindTarget{
				Space:                            bt.Space,
				Register:                         bt.Register,
				BindingArraySize:                 bt.BindingArraySize,
				RestrictIndexing:                 bt.RestrictIndexing,
				DynamicStorageBufferOffsetsIndex: bt.DynamicStorageBufferOffsetsIndex,
				DynamicStorageBufferSizeIndex:    bt.DynamicStorageBufferSizeIndex,
			}
		}
	}
	if h.SamplerBufferBindingMap != nil {
		opts.SamplerBufferBindingMap = make(map[uint32]dxil.BindTarget, len(h.SamplerBufferBindingMap))
		for g, bt := range h.SamplerBufferBindingMap {
			opts.SamplerBufferBindingMap[g] = dxil.BindTarget{Space: bt.Space, Register: bt.Register}
		}
	}
	if s := h.SamplerHeapTargets; s != nil {
		opts.SamplerHeapTargets = &dxil.SamplerHeapBindTargets{
			StandardSamplers:   dxil.BindTarget{Space: s.StandardSamplers.Space, Register: s.StandardSamplers.Register},
			ComparisonSamplers: dxil.BindTarget{Space: s.ComparisonSamplers.Space, Register: s.ComparisonSamplers.Register},
		}
	}
	if h.DynamicStorageBufferOffsetsTargets != nil {
		opts.DynamicStorageBufferOffsetsTargets = make(map[uint32]dxil.OffsetsBindTarget, len(h.DynamicStorageBufferOffsetsTargets))
		for g, ot := range h.DynamicStorageBufferOffsetsTargets {
			opts.DynamicStorageBufferOffsetsTargets[g] = dxil.OffsetsBindTarget{Space: ot.Space, Register: ot.Register, Size: ot.Size}
		}
	}
	if it := h.ImmediatesTarget; it != nil {
		opts.ImmediatesTarget = &dxil.BindTarget{Space: it.Space, Register: it.Register}
	}

	data, err := os.ReadFile(rustParamsPath(shaderName))
	if errors.Is(err, fs.ErrNotExist) {
		return opts
	}
	if err != nil {
		t.Fatalf("%s: Rust parameters: %v", shaderName, err)
	}
	content := string(data)
	if m := specialConstantsRE.FindStringSubmatch(extractHLSLSection(content)); m != nil {
		bt := parseBindTargetFull(m[1])
		opts.SpecialConstantsBinding = &dxil.BindTarget{Space: bt.Space, Register: bt.Register}
	}
	// readHLSLConfig does not read binding_array_size; the [[spv.binding_map]]
	// of the same parameters gives every unbounded binding array the same
	// size its [[hlsl.binding_map]] does.
	for _, b := range mustRustParams(content).spvBindingMap {
		if b.bindingArraySize == nil {
			continue
		}
		rb := ir.ResourceBinding{Group: b.group, Binding: b.binding}
		target, ok := opts.BindingMap[rb]
		if !ok {
			if opts.BindingMap == nil {
				opts.BindingMap = dxil.BindingMap{}
			}
			target = dxil.BindTarget{Space: uint8(b.group), Register: b.binding}
		}
		if target.BindingArraySize == nil {
			target.BindingArraySize = b.bindingArraySize
		}
		opts.BindingMap[rb] = target
	}
	return opts
}

// specialConstantsRE matches the [hlsl] special_constants_binding target.
var specialConstantsRE = regexp.MustCompile(`special_constants_binding\s*=\s*\{([^}]+)\}`)
