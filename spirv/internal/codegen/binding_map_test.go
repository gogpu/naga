package codegen

import (
	"errors"
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
	"github.com/gogpu/naga/wgsl"
)

// compileWGSLWithOpts lowers source and compiles it, returning the backend
// error instead of failing the test.
func compileWGSLWithOpts(t *testing.T, source string, opts Options) ([]byte, error) {
	t.Helper()
	module, _, err := wgsl.Parse(source, wgsl.ParseOptions{})
	if err != nil {
		t.Fatalf("Lower: %v", err)
	}
	return NewBackend(opts).Compile(module)
}

// decorations returns the literal of every OpDecorate with the given
// decoration, keyed by target id.
func (v *spvView) decorations(d spvenums.Decoration) map[uint32]uint32 {
	out := map[uint32]uint32{}
	for _, in := range v.instrs {
		if in.opcode == spvenums.OpDecorate && len(in.words) == 4 && spvenums.Decoration(in.words[2]) == d {
			out[in.words[1]] = in.words[3]
		}
	}
	return out
}

// typeDef returns the type instruction (OpType*) whose result id is id.
func (v *spvView) typeDef(id uint32) (spirvInstruction, bool) {
	for _, in := range v.instrs {
		switch in.opcode {
		case spvenums.OpTypeArray, spvenums.OpTypeRuntimeArray, spvenums.OpTypePointer, spvenums.OpTypeStruct:
			if len(in.words) > 1 && in.words[1] == id {
				return in, true
			}
		}
	}
	return spirvInstruction{}, false
}

// boundVariable returns the OpVariable decorated with (set, binding).
func (v *spvView) boundVariable(t *testing.T, set, binding uint32) spirvInstruction {
	t.Helper()
	sets := v.decorations(spvenums.DecorationDescriptorSet)
	bindings := v.decorations(spvenums.DecorationBinding)
	for _, in := range v.all(spvenums.OpVariable, 0) {
		id := in.words[2]
		if s, ok := sets[id]; ok && s == set && bindings[id] == binding {
			return in
		}
	}
	t.Fatalf("no variable decorated with DescriptorSet %d Binding %d", set, binding)
	return spirvInstruction{}
}

// variableArrayLength returns the length operand of the array a binding
// array variable points to, or ok=false for a runtime array.
func (v *spvView) variableArrayLength(t *testing.T, variable spirvInstruction) (length uint32, sized bool) {
	t.Helper()
	ptr, ok := v.typeDef(variable.words[1])
	if !ok || ptr.opcode != spvenums.OpTypePointer {
		t.Fatalf("variable type %d is not an OpTypePointer", variable.words[1])
	}
	arr, ok := v.typeDef(ptr.words[3])
	if !ok {
		t.Fatalf("pointee type %d not found", ptr.words[3])
	}
	switch arr.opcode {
	case spvenums.OpTypeArray:
		n, ok := v.consts[arr.words[3]]
		if !ok {
			t.Fatalf("array length %d is not an OpConstant", arr.words[3])
		}
		return n, true
	case spvenums.OpTypeRuntimeArray:
		return 0, false
	}
	t.Fatalf("pointee type is opcode %d, want OpTypeArray or OpTypeRuntimeArray", arr.opcode)
	return 0, false
}

func u32Ptr(v uint32) *uint32 { return &v }

const bindingMapShader = `
@group(0) @binding(0) var<storage, read_write> out: u32;
@compute @workgroup_size(1)
fn main() { out = 1u; }`

// TestBindingMapRemapsDescriptorSetAndBinding pins Rust naga's
// resolve_resource_binding: a binding-map entry replaces the WGSL
// @group/@binding in the DescriptorSet and Binding decorations.
func TestBindingMapRemapsDescriptorSetAndBinding(t *testing.T) {
	spv, err := compileWGSLWithOpts(t, bindingMapShader, Options{
		Version: Version1_3,
		BindingMap: map[ir.ResourceBinding]BindingInfo{
			{Group: 0, Binding: 0}: {DescriptorSet: 2, Binding: 5},
		},
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	validateWithVulkanSDK(t, spv)
	v := newSPVView(t, spv)
	v.boundVariable(t, 2, 5)
	if sets := v.decorations(spvenums.DecorationDescriptorSet); len(sets) != 1 {
		t.Errorf("DescriptorSet decorations = %d, want 1 (the remapped one only)", len(sets))
	}
}

// TestBindingMapMissingEntry: a nil map keeps every resource at its WGSL
// @group/@binding; a non-nil map without an entry for a resource is an error
// wrapping backend.ErrMissingBinding, as Rust naga with
// fake_missing_bindings = false (what wgpu-hal passes), an empty map too;
// FakeMissingBindings keeps the resource at its WGSL @group/@binding.
func TestBindingMapMissingEntry(t *testing.T) {
	other := map[ir.ResourceBinding]BindingInfo{{Group: 7, Binding: 7}: {DescriptorSet: 1, Binding: 1}}

	t.Run("nil_map_keeps_wgsl_binding", func(t *testing.T) {
		spv, err := compileWGSLWithOpts(t, bindingMapShader, Options{Version: Version1_3})
		if err != nil {
			t.Fatalf("Compile: %v", err)
		}
		newSPVView(t, spv).boundVariable(t, 0, 0)
	})
	t.Run("fake_missing_bindings_keeps_wgsl_binding", func(t *testing.T) {
		spv, err := compileWGSLWithOpts(t, bindingMapShader, Options{Version: Version1_3, BindingMap: other, FakeMissingBindings: true})
		if err != nil {
			t.Fatalf("Compile: %v", err)
		}
		newSPVView(t, spv).boundVariable(t, 0, 0)
	})
	t.Run("absent_entry_is_an_error", func(t *testing.T) {
		_, err := compileWGSLWithOpts(t, bindingMapShader, Options{Version: Version1_3, BindingMap: other})
		if err == nil {
			t.Fatal("Compile succeeded, want a missing-binding error")
		}
		if !strings.Contains(err.Error(), "group 0") || !strings.Contains(err.Error(), "binding 0") {
			t.Errorf("error %q does not name the missing @group(0) @binding(0)", err)
		}
		if !errors.Is(err, backend.ErrMissingBinding) {
			t.Errorf("error %q does not wrap backend.ErrMissingBinding", err)
		}
	})
	t.Run("empty_map_is_an_error", func(t *testing.T) {
		_, err := compileWGSLWithOpts(t, bindingMapShader, Options{Version: Version1_3, BindingMap: map[ir.ResourceBinding]BindingInfo{}})
		if !errors.Is(err, backend.ErrMissingBinding) {
			t.Errorf("empty map: err = %v, want backend.ErrMissingBinding", err)
		}
	})
}

const bindingArrayShader = `
enable wgpu_binding_array;
struct Foo { x: u32 }
@group(0) @binding(0) var<storage, read> foos: binding_array<Foo, %s>;
@group(0) @binding(1) var<storage, read_write> out: u32;
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) { out = foos[i].x; }`

func bindingArraySource(size string) string {
	if size == "" {
		return strings.Replace(bindingArrayShader, "binding_array<Foo, %s>", "binding_array<Foo>", 1)
	}
	return strings.Replace(bindingArrayShader, "%s", size, 1)
}

// TestBindingMapBindingArraySize pins Rust naga's write_global_variable: a
// binding-map entry with binding_array_size declares the variable as an
// array of that size, sized or unbounded in WGSL alike.
func TestBindingMapBindingArraySize(t *testing.T) {
	sized := map[ir.ResourceBinding]BindingInfo{
		{Group: 0, Binding: 0}: {DescriptorSet: 0, Binding: 0, BindingArraySize: u32Ptr(10)},
	}
	for _, tc := range []struct {
		name, size string
	}{
		{"declared_size_1", "1"},
		{"unbounded", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spv, err := compileWGSLWithOpts(t, bindingArraySource(tc.size), Options{Version: Version1_3, BindingMap: sized, FakeMissingBindings: true})
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			validateWithVulkanSDK(t, spv)
			v := newSPVView(t, spv)
			length, isSized := v.variableArrayLength(t, v.boundVariable(t, 0, 0))
			if !isSized || length != 10 {
				t.Errorf("binding array variable: sized=%v length=%d, want an OpTypeArray of 10", isSized, length)
			}
		})
	}

	t.Run("no_size_keeps_declared_type", func(t *testing.T) {
		plain := map[ir.ResourceBinding]BindingInfo{{Group: 0, Binding: 0}: {DescriptorSet: 0, Binding: 0}}
		spv, err := compileWGSLWithOpts(t, bindingArraySource("4"), Options{Version: Version1_3, BindingMap: plain, FakeMissingBindings: true})
		if err != nil {
			t.Fatalf("Compile: %v", err)
		}
		v := newSPVView(t, spv)
		if length, isSized := v.variableArrayLength(t, v.boundVariable(t, 0, 0)); !isSized || length != 4 {
			t.Errorf("binding array variable: sized=%v length=%d, want the declared OpTypeArray of 4", isSized, length)
		}
	})
}

// TestBindingMapUnboundedBindingArrayBoundsCheck pins Rust naga's
// binding_array_layout_size: the length of an unbounded binding array for a
// bounds check comes from the binding map; without an entry it stays an error.
func TestBindingMapUnboundedBindingArrayBoundsCheck(t *testing.T) {
	policies := backend.BoundsCheckPolicies{Buffer: backend.BoundsCheckRestrict, BindingArray: backend.BoundsCheckRestrict}

	t.Run("size_from_map", func(t *testing.T) {
		spv, err := compileWGSLWithOpts(t, bindingArraySource(""), Options{
			Version:             Version1_3,
			BoundsCheckPolicies: policies,
			BindingMap: map[ir.ResourceBinding]BindingInfo{
				{Group: 0, Binding: 0}: {DescriptorSet: 0, Binding: 0, BindingArraySize: u32Ptr(10)},
			},
			FakeMissingBindings: true,
		})
		if err != nil {
			t.Fatalf("Compile: %v", err)
		}
		validateWithVulkanSDK(t, spv)
		v := newSPVView(t, spv)
		umins := v.all(spvenums.OpExtInst, uint32(spvenums.GlslStd450UMin))
		if len(umins) != 1 {
			t.Fatalf("UMin count = %d, want 1 (the binding-array index)", len(umins))
		}
		if !v.isConst(umins[0].words[6], 9) {
			t.Errorf("UMin bound operand is not the constant 9 (binding_array_size 10 - 1)")
		}
	})
	t.Run("no_entry_is_an_error", func(t *testing.T) {
		_, err := compileWGSLWithOpts(t, bindingArraySource(""), Options{Version: Version1_3, BoundsCheckPolicies: policies})
		if err == nil {
			t.Fatal("Compile succeeded, want an error: the unbounded binding array has no known length")
		}
	})
}
