package naga

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
)

// zeroOptionsShader is a compute entry point that reads workgroup memory, so
// every backend has a variable to zero.
const zeroOptionsShader = `
var<workgroup> shared_data: array<u32, 64>;
@group(0) @binding(0) var<storage, read_write> out: array<u32, 64>;
@compute @workgroup_size(64)
fn main(@builtin(local_invocation_index) i: u32) {
    out[i] = shared_data[i];
}`

func zeroOptionsModule(t *testing.T) *ir.Module {
	t.Helper()
	ast, err := Parse(zeroOptionsShader)
	if err != nil {
		t.Fatal(err)
	}
	module, err := LowerWithSource(ast, zeroOptionsShader)
	if err != nil {
		t.Fatal(err)
	}
	return module
}

// zeroInitBackend compiles zeroOptionsShader with one workgroup zero-init
// mode. withZero compiles it with the backend's Options left at their zero
// value for this concept: for msl, hlsl and glsl the whole Options value;
// spirv and dxil still need a version until P1-API-002 makes their zero
// versions valid, so only the field under test is left out.
type zeroInitBackend struct {
	compile  func(m *ir.Module, mode backend.ZeroInitializeWorkgroupMemoryMode) ([]byte, error)
	withZero func(m *ir.Module) ([]byte, error)
	marker   string // in text output when the entry point zeroes the variable
}

func zeroInitBackends() map[string]zeroInitBackend {
	text := func(code string, _ any, err error) ([]byte, error) { return []byte(code), err }
	return map[string]zeroInitBackend{
		"spirv": {
			compile: func(m *ir.Module, mode backend.ZeroInitializeWorkgroupMemoryMode) ([]byte, error) {
				return spirvBinary(m, spirv.Options{Version: spirv.Version1_3, ZeroInitializeWorkgroupMemory: mode})
			},
			withZero: func(m *ir.Module) ([]byte, error) {
				return spirvBinary(m, spirv.Options{Version: spirv.Version1_3})
			},
		},
		"msl": {
			compile: func(m *ir.Module, mode backend.ZeroInitializeWorkgroupMemoryMode) ([]byte, error) {
				return text(msl.Compile(m, msl.Options{ZeroInitializeWorkgroupMemory: mode}))
			},
			withZero: func(m *ir.Module) ([]byte, error) { return text(msl.Compile(m, msl.Options{})) },
			marker:   "metal::all(__local_invocation_id == metal::uint3(0u))",
		},
		"hlsl": {
			compile: func(m *ir.Module, mode backend.ZeroInitializeWorkgroupMemoryMode) ([]byte, error) {
				return text(hlsl.Compile(m, hlsl.Options{ZeroInitializeWorkgroupMemory: mode}))
			},
			withZero: func(m *ir.Module) ([]byte, error) { return text(hlsl.Compile(m, hlsl.Options{})) },
			marker:   "all(__local_invocation_id == uint3(0u, 0u, 0u))",
		},
		"glsl": {
			compile: func(m *ir.Module, mode backend.ZeroInitializeWorkgroupMemoryMode) ([]byte, error) {
				return text(glsl.Compile(m, glsl.Options{LangVersion: glsl.Version430, ZeroInitializeWorkgroupMemory: mode}))
			},
			withZero: func(m *ir.Module) ([]byte, error) {
				return text(glsl.Compile(m, glsl.Options{LangVersion: glsl.Version430}))
			},
			marker: "gl_LocalInvocationID == uvec3(0u)",
		},
		"dxil": {
			compile: func(m *ir.Module, mode backend.ZeroInitializeWorkgroupMemoryMode) ([]byte, error) {
				return dxilBlob(m, dxil.Options{ZeroInitializeWorkgroupMemory: mode})
			},
			withZero: func(m *ir.Module) ([]byte, error) {
				return dxilBlob(m, dxil.Options{})
			},
		},
	}
}

// The zero value of every backend's Options is the behavior that is safe on
// every device (ADR-009 rule 1). For workgroup memory that is Polyfill: a
// compute entry point zeroes the variables it uses before it runs, as WebGPU
// requires, in every backend (Rust naga's default in each of them).
// ZeroInitWorkgroupNone leaves them as they are; Native is SPIR-V only; a
// value outside the enum is an error everywhere.
func TestZeroOptionsAreSafe(t *testing.T) {
	for name, b := range zeroInitBackends() {
		t.Run("workgroup-zero-init/"+name, func(t *testing.T) {
			zero, err := b.withZero(zeroOptionsModule(t))
			if err != nil {
				t.Fatalf("zero Options: %v", err)
			}
			polyfill, err := b.compile(zeroOptionsModule(t), backend.ZeroInitWorkgroupPolyfill)
			if err != nil {
				t.Fatalf("Polyfill: %v", err)
			}
			none, err := b.compile(zeroOptionsModule(t), backend.ZeroInitWorkgroupNone)
			if err != nil {
				t.Fatalf("None: %v", err)
			}
			if !bytes.Equal(zero, polyfill) {
				t.Errorf("zero Options differ from Polyfill")
			}
			if bytes.Equal(none, polyfill) {
				t.Errorf("None compiles the same as Polyfill")
			}
			if b.marker != "" {
				if !bytes.Contains(zero, []byte(b.marker)) {
					t.Errorf("zero Options: no workgroup zero-init (%q):\n%s", b.marker, zero)
				}
				if bytes.Contains(none, []byte(b.marker)) {
					t.Errorf("None: workgroup zero-init (%q) emitted:\n%s", b.marker, none)
				}
			}

			native, err := b.compile(zeroOptionsModule(t), backend.ZeroInitWorkgroupNative)
			if name == "spirv" {
				if err != nil {
					t.Errorf("Native: %v", err)
				} else if bytes.Equal(native, polyfill) || bytes.Equal(native, none) {
					t.Errorf("Native compiles the same as Polyfill or None")
				}
			} else if err == nil || !strings.Contains(err.Error(), "ZeroInitializeWorkgroupMemory Native is not supported") {
				t.Errorf("Native: err %v, want ZeroInitializeWorkgroupMemory Native is not supported", err)
			}

			const bogus = backend.ZeroInitializeWorkgroupMemoryMode(7)
			if _, err := b.compile(zeroOptionsModule(t), bogus); err == nil ||
				!strings.Contains(err.Error(), "ZeroInitializeWorkgroupMemoryMode(7) is not supported") {
				t.Errorf("mode 7: err %v, want ZeroInitializeWorkgroupMemoryMode(7) is not supported", err)
			}
		})
	}
}

// boundsShader indexes a local array, a storage array and a texture with
// run-time values, so every bounds-check field has an access to guard.
const boundsShader = `
@group(0) @binding(0) var<storage, read_write> data: array<u32, 16>;
@group(0) @binding(1) var tex: texture_2d<f32>;
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    var local = array<u32, 4>(1u, 2u, 3u, 4u);
    data[i] = local[i] + u32(textureLoad(tex, vec2<u32>(i, i), i).x);
}`

func boundsModule(t *testing.T) *ir.Module {
	t.Helper()
	ast, err := Parse(boundsShader)
	if err != nil {
		t.Fatal(err)
	}
	module, err := LowerWithSource(ast, boundsShader)
	if err != nil {
		t.Fatal(err)
	}
	return module
}

func allPolicies(p backend.BoundsCheckPolicy) backend.BoundsCheckPolicies {
	return backend.BoundsCheckPolicies{Index: p, Buffer: p, ImageLoad: p, BindingArray: p}
}

// boundsBackend compiles boundsShader with the given policies, and with the
// policies left at their zero value.
type boundsBackend struct {
	compile  func(m *ir.Module, p backend.BoundsCheckPolicies) ([]byte, error)
	withZero func(m *ir.Module) ([]byte, error)
}

func boundsBackends() map[string]boundsBackend {
	return map[string]boundsBackend{
		"spirv": {
			compile: func(m *ir.Module, p backend.BoundsCheckPolicies) ([]byte, error) {
				return spirvBinary(m, spirv.Options{Version: spirv.Version1_3, BoundsCheckPolicies: p})
			},
			withZero: func(m *ir.Module) ([]byte, error) {
				return spirvBinary(m, spirv.Options{Version: spirv.Version1_3})
			},
		},
		"msl": {
			compile: func(m *ir.Module, p backend.BoundsCheckPolicies) ([]byte, error) {
				code, _, err := msl.Compile(m, msl.Options{BoundsCheckPolicies: p})
				return []byte(code), err
			},
			withZero: func(m *ir.Module) ([]byte, error) {
				code, _, err := msl.Compile(m, msl.Options{})
				return []byte(code), err
			},
		},
		"glsl": {
			compile: func(m *ir.Module, p backend.BoundsCheckPolicies) ([]byte, error) {
				code, _, err := glsl.Compile(m, glsl.Options{LangVersion: glsl.Version430, BoundsCheckPolicies: p})
				return []byte(code), err
			},
			withZero: func(m *ir.Module) ([]byte, error) {
				code, _, err := glsl.Compile(m, glsl.Options{LangVersion: glsl.Version430})
				return []byte(code), err
			},
		},
		"hlsl": {
			compile: func(m *ir.Module, p backend.BoundsCheckPolicies) ([]byte, error) {
				code, _, err := hlsl.Compile(m, hlsl.Options{FakeMissingBindings: true, BoundsCheckPolicies: p})
				return []byte(code), err
			},
			withZero: func(m *ir.Module) ([]byte, error) {
				code, _, err := hlsl.Compile(m, hlsl.Options{FakeMissingBindings: true})
				return []byte(code), err
			},
		},
		"dxil": {
			compile: func(m *ir.Module, p backend.BoundsCheckPolicies) ([]byte, error) {
				return dxilBlob(m, dxil.Options{BoundsCheckPolicies: p})
			},
			withZero: func(m *ir.Module) ([]byte, error) {
				return dxilBlob(m, dxil.Options{})
			},
		},
	}
}

// The zero BoundsCheckPolicies is Restrict on every field, which WebGPU
// requires for array, vector and matrix accesses, in every backend that has
// the policies. Rust naga's default is Unchecked; a caller asks for it. A
// value outside the enum is an error.
func TestZeroBoundsCheckPoliciesAreRestrict(t *testing.T) {
	for name, b := range boundsBackends() {
		t.Run(name, func(t *testing.T) {
			zero, err := b.withZero(boundsModule(t))
			if err != nil {
				t.Fatalf("zero policies: %v", err)
			}
			restrict, err := b.compile(boundsModule(t), allPolicies(backend.BoundsCheckRestrict))
			if err != nil {
				t.Fatalf("Restrict: %v", err)
			}
			unchecked, err := b.compile(boundsModule(t), allPolicies(backend.BoundsCheckUnchecked))
			if err != nil {
				t.Fatalf("Unchecked: %v", err)
			}
			if !bytes.Equal(zero, restrict) {
				t.Errorf("zero policies differ from Restrict")
			}
			if bytes.Equal(zero, unchecked) {
				t.Errorf("zero policies compile the same as Unchecked")
			}
			bogus := allPolicies(backend.BoundsCheckUnchecked)
			bogus.ImageLoad = 7
			if _, err := b.compile(boundsModule(t), bogus); err == nil ||
				!strings.Contains(err.Error(), "BoundsCheckPolicy(7) is not supported") {
				t.Errorf("policy 7: err %v, want BoundsCheckPolicy(7) is not supported", err)
			}
		})
	}
}

// loopShader has a loop whose exit the compiler cannot prove.
const loopShader = `
@group(0) @binding(0) var<storage, read_write> data: array<u32, 4>;
@compute @workgroup_size(1)
fn main() {
    var i = 0u;
    loop {
        if data[i & 3u] == 7u { break; }
        i += 1u;
    }
}`

// The zero value bounds loops, as Rust naga's default force_loop_bounding:
// true does and wgpu-hal passes: an infinite loop is undefined behavior that
// hangs or loses the device.
func TestZeroOptionsBoundLoops(t *testing.T) {
	module := func(t *testing.T) *ir.Module {
		t.Helper()
		ast, err := Parse(loopShader)
		if err != nil {
			t.Fatal(err)
		}
		m, err := LowerWithSource(ast, loopShader)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	for name, compile := range map[string]func(*ir.Module) (string, error){
		"msl": func(m *ir.Module) (string, error) {
			code, _, err := msl.Compile(m, msl.Options{})
			return code, err
		},
		"hlsl": func(m *ir.Module) (string, error) {
			code, _, err := hlsl.Compile(m, hlsl.Options{FakeMissingBindings: true})
			return code, err
		},
	} {
		code, err := compile(module(t))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !strings.Contains(code, "loop_bound") {
			t.Errorf("%s, zero Options: loop not bounded:\n%s", name, code)
		}
	}

	// SPIR-V: the zero value adds the counter, so the module is longer than
	// with SkipLoopBounding.
	bounded, err := spirvBinary(module(t), spirv.Options{Version: spirv.Version1_3})
	if err != nil {
		t.Fatal(err)
	}
	skipped, err := spirvBinary(module(t), spirv.Options{Version: spirv.Version1_3, SkipLoopBounding: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(bounded) <= len(skipped) {
		t.Errorf("spirv: zero Options (%d bytes) not longer than SkipLoopBounding (%d bytes): loop not bounded", len(bounded), len(skipped))
	}
}

// bindingShader uses two resources; the binding maps below list only one.
const bindingShader = `
@group(0) @binding(0) var<storage, read_write> a: array<u32, 4>;
@group(1) @binding(3) var<storage, read_write> b: array<u32, 4>;
@compute @workgroup_size(1)
fn main() { a[0] = b[1]; }`

func bindingModule(t *testing.T) *ir.Module {
	t.Helper()
	ast, err := Parse(bindingShader)
	if err != nil {
		t.Fatal(err)
	}
	m, err := LowerWithSource(ast, bindingShader)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// A binding map that leaves out a resource the shader uses is an error
// wrapping backend.ErrMissingBinding at the zero value (ADR-009 §6 row 5), as
// wgpu-hal runs Rust naga (fake_missing_bindings: false); FakeMissingBindings
// keeps the resource at its WGSL group and binding. A nil map is the
// documented identity mapping and never an error.
func TestZeroOptionsRefuseMissingBindings(t *testing.T) {
	spirvWith := func(m map[spirv.ResourceBinding]spirv.BindingInfo, fake bool) error {
		_, err := spirvBinary(bindingModule(t), spirv.Options{Version: spirv.Version1_3, BindingMap: m, FakeMissingBindings: fake})
		return err
	}
	partial := map[spirv.ResourceBinding]spirv.BindingInfo{{Group: 0, Binding: 0}: {DescriptorSet: 0, Binding: 0}}
	if err := spirvWith(partial, false); !errors.Is(err, backend.ErrMissingBinding) {
		t.Errorf("spirv, partial map: err = %v, want backend.ErrMissingBinding", err)
	}
	if err := spirvWith(partial, true); err != nil {
		t.Errorf("spirv, partial map, FakeMissingBindings: %v", err)
	}
	if err := spirvWith(nil, false); err != nil {
		t.Errorf("spirv, nil map: %v", err)
	}

	mslWith := func(m map[string]msl.EntryPointResources, fake bool) error {
		_, _, err := msl.Compile(bindingModule(t), msl.Options{PerEntryPointMap: m, FakeMissingBindings: fake})
		return err
	}
	slot := uint8(0)
	mslPartial := map[string]msl.EntryPointResources{"main": {Resources: map[ir.ResourceBinding]msl.BindTarget{{Group: 0, Binding: 0}: {Buffer: &slot}}}}
	if err := mslWith(mslPartial, false); !errors.Is(err, backend.ErrMissingBinding) {
		t.Errorf("msl, partial map: err = %v, want backend.ErrMissingBinding", err)
	}
	if err := mslWith(map[string]msl.EntryPointResources{}, false); !errors.Is(err, backend.ErrMissingBinding) {
		t.Errorf("msl, map without the entry point: err = %v, want backend.ErrMissingBinding", err)
	}
	if err := mslWith(mslPartial, true); err != nil {
		t.Errorf("msl, partial map, FakeMissingBindings: %v", err)
	}
	if err := mslWith(nil, false); err != nil {
		t.Errorf("msl, nil map: %v", err)
	}

	glslWith := func(m map[glsl.BindingMapKey]uint8, fake bool) error {
		_, _, err := glsl.Compile(bindingModule(t), glsl.Options{LangVersion: glsl.Version430, BindingMap: m, FakeMissingBindings: fake})
		return err
	}
	glslPartial := map[glsl.BindingMapKey]uint8{{Group: 0, Binding: 0}: 0}
	if err := glslWith(glslPartial, false); !errors.Is(err, backend.ErrMissingBinding) {
		t.Errorf("glsl, partial map: err = %v, want backend.ErrMissingBinding", err)
	}
	if err := glslWith(glslPartial, true); err != nil {
		t.Errorf("glsl, partial map, FakeMissingBindings: %v", err)
	}
	if err := glslWith(nil, false); err != nil {
		t.Errorf("glsl, nil map: %v", err)
	}

	hlslWith := func(m map[hlsl.ResourceBinding]hlsl.BindTarget, fake bool) error {
		_, _, err := hlsl.Compile(bindingModule(t), hlsl.Options{BindingMap: m, FakeMissingBindings: fake})
		return err
	}
	hlslPartial := map[hlsl.ResourceBinding]hlsl.BindTarget{{Group: 0, Binding: 0}: {}}
	if err := hlslWith(hlslPartial, false); !errors.Is(err, backend.ErrMissingBinding) {
		t.Errorf("hlsl, partial map: err = %v, want backend.ErrMissingBinding", err)
	}
	if err := hlslWith(hlslPartial, true); err != nil {
		t.Errorf("hlsl, partial map, FakeMissingBindings: %v", err)
	}
	if err := hlslWith(nil, false); err != nil {
		t.Errorf("hlsl, nil map: %v", err)
	}

	dxilWith := func(m dxil.BindingMap, fake bool) error {
		_, _, err := dxil.Compile(bindingModule(t), dxil.Options{BindingMap: m, FakeMissingBindings: fake})
		return err
	}
	dxilPartial := dxil.BindingMap{{Group: 0, Binding: 0}: {}}
	if err := dxilWith(dxilPartial, false); !errors.Is(err, backend.ErrMissingBinding) {
		t.Errorf("dxil, partial map: err = %v, want backend.ErrMissingBinding", err)
	}
	if err := dxilWith(dxilPartial, true); err != nil {
		t.Errorf("dxil, partial map, FakeMissingBindings: %v", err)
	}
	if err := dxilWith(nil, false); err != nil {
		t.Errorf("dxil, nil map: %v", err)
	}
}

// The zero SPIR-V version is 1.0, the only one every Vulkan device accepts
// without knowing the device, as Rust naga's default. A feature that needs a
// newer one is an error, never a silently raised version (Rust's
// SpirvVersionTooLow).
func TestZeroSPIRVVersion(t *testing.T) {
	spv, err := spirvBinary(zeroOptionsModule(t), spirv.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if spv[5] != 0 || spv[6] != 1 {
		t.Errorf("zero Options: SPIR-V %d.%d, want 1.0", spv[6], spv[5])
	}

	const subgroups = `
@group(0) @binding(0) var<storage, read_write> out: u32;
@compute @workgroup_size(64) fn main(@builtin(subgroup_size) n: u32) { out = subgroupAdd(n); }`
	ast, err := Parse(subgroups)
	if err != nil {
		t.Fatal(err)
	}
	m, err := LowerWithCapabilities(ast, subgroups, ir.CapAll)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := spirvBinary(m, spirv.Options{}); err == nil || !strings.Contains(err.Error(), "need SPIR-V 1.3") {
		t.Errorf("subgroups at the zero version: err = %v, want need SPIR-V 1.3", err)
	}
	spv, err = spirvBinary(m, spirv.Options{Version: spirv.Version1_3})
	if err != nil || spv[5] != 3 {
		t.Errorf("subgroups at 1.3: err = %v", err)
	}
}

// The zero shader model is the backend's floor: 5.1 for HLSL (Rust naga's
// default), 6.0 for DXIL; a model the backend cannot write is an error.
func TestZeroShaderModel(t *testing.T) {
	hlslCode := func(sm backend.ShaderModel) (string, error) {
		code, _, err := hlsl.Compile(zeroOptionsModule(t), hlsl.Options{ShaderModel: sm})
		return code, err
	}
	zero, err := hlslCode(backend.ShaderModel{})
	if err != nil {
		t.Fatal(err)
	}
	if v51, err := hlslCode(backend.ShaderModel{Major: 5, Minor: 1}); err != nil || v51 != zero {
		t.Errorf("hlsl: zero shader model differs from 5.1 (err %v)", err)
	}
	if _, err := hlslCode(backend.ShaderModel{Major: 7, Minor: 0}); err == nil || !strings.Contains(err.Error(), "shader model 7.0 is not supported") {
		t.Errorf("hlsl 7.0: err = %v", err)
	}

	dxilZero, _, err := dxil.Compile(zeroOptionsModule(t), dxil.Options{})
	if err != nil {
		t.Fatal(err)
	}
	dxil60, _, err := dxil.Compile(zeroOptionsModule(t), dxil.Options{ShaderModel: backend.ShaderModel{Major: 6, Minor: 0}})
	if err != nil || !bytes.Equal(dxilZero, dxil60) {
		t.Errorf("dxil: zero shader model differs from 6.0 (err %v)", err)
	}
	if _, _, err := dxil.Compile(zeroOptionsModule(t), dxil.Options{ShaderModel: backend.ShaderModel{Major: 5, Minor: 1}}); err == nil ||
		!strings.Contains(err.Error(), "shader model 5.1 is not supported") {
		t.Errorf("dxil 5.1: err = %v", err)
	}
}

// TestZeroSPIRVAdjustCoordinateSpace: the zero value leaves Position as the
// entry point writes it (Vulkan flips y with a negative viewport height);
// AdjustCoordinateSpace negates its y after the store, as Rust naga's
// write_epilogue_position_y_flip. The field used to be accepted and ignored.
func TestZeroSPIRVAdjustCoordinateSpace(t *testing.T) {
	const opFNegate = 127
	for _, adjust := range []bool{false, true} {
		bin, err := spirvBinary(entryPointModule(t), spirv.Options{EntryPoint: selVS, AdjustCoordinateSpace: adjust})
		if err != nil {
			t.Fatal(err)
		}
		ops, _ := spirvOps(t, bin)
		want := 0
		if adjust {
			want = 1
		}
		if got := countOp(ops, opFNegate); got != want {
			t.Errorf("AdjustCoordinateSpace %v: %d OpFNegate, want %d", adjust, got, want)
		}
	}
}

// TestDefaultOptionsAreZero: every backend's DefaultOptions is its zero
// Options (ADR-009 row 12), so the two can never drift apart again: msl's
// gave ReadZeroSkipWrite after the zero became Restrict, spirv's 1.1 with
// native f16 inputs and outputs.
func TestDefaultOptionsAreZero(t *testing.T) {
	for name, pair := range map[string][2]any{
		"spirv": {spirv.DefaultOptions(), spirv.Options{}},
		"msl":   {msl.DefaultOptions(), msl.Options{}},
		"glsl":  {glsl.DefaultOptions(), glsl.Options{}},
		"hlsl":  {hlsl.DefaultOptions(), hlsl.Options{}},
		"dxil":  {dxil.DefaultOptions(), dxil.Options{}},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			t.Errorf("%s.DefaultOptions() = %+v, want the zero Options", name, pair[0])
		}
	}
}

// TestZeroCompileOptions: the zero CompileOptions is DefaultOptions, and it
// validates the module with the core capabilities, as Rust naga's CLI does:
// f64 needs CapFloat64, and SkipValidation compiles without the check.
func TestZeroCompileOptions(t *testing.T) {
	if !reflect.DeepEqual(DefaultOptions(), CompileOptions{}) {
		t.Errorf("DefaultOptions() = %+v, want the zero value", DefaultOptions())
	}
	const f64Shader = `@compute @workgroup_size(1) fn main() { var x: f64; }`
	if _, err := Compile(f64Shader); err == nil || !strings.Contains(err.Error(), "validation") {
		t.Errorf("zero options, f64: err = %v, want a validation error", err)
	}
	if _, err := CompileWithOptions(f64Shader, CompileOptions{Capabilities: CapFloat64}); err != nil {
		t.Errorf("CapFloat64: %v", err)
	}
	if _, err := CompileWithOptions(f64Shader, CompileOptions{SkipValidation: true}); err != nil {
		t.Errorf("SkipValidation: %v", err)
	}
}

// TestZeroGLSLAdjustCoordinateSpace: GL clip space differs from WebGPU's, so
// the zero value converts gl_Position (Rust naga's default
// ADJUST_COORDINATE_SPACE, which wgpu-hal's GLES backend always sets); the
// zero WriterFlags used to leave it unconverted.
func TestZeroGLSLAdjustCoordinateSpace(t *testing.T) {
	const adjust = "gl_Position.yz = vec2(-gl_Position.y, gl_Position.z * 2.0 - gl_Position.w);"
	for _, skip := range []bool{false, true} {
		src, _, err := glsl.Compile(entryPointModule(t), glsl.Options{EntryPoint: selVS, SkipAdjustCoordinateSpace: skip})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(src, adjust) == skip {
			t.Errorf("SkipAdjustCoordinateSpace %v: adjusted = %v:\n%s", skip, !skip, src)
		}
	}
}
