package naga

import (
	"bytes"
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
				return spirv.NewBackend(spirv.Options{Version: spirv.Version1_3, ZeroInitializeWorkgroupMemory: mode}).Compile(m)
			},
			withZero: func(m *ir.Module) ([]byte, error) {
				return spirv.NewBackend(spirv.Options{Version: spirv.Version1_3}).Compile(m)
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
				return text(hlsl.Compile(m, &hlsl.Options{ZeroInitializeWorkgroupMemory: mode}))
			},
			withZero: func(m *ir.Module) ([]byte, error) { return text(hlsl.Compile(m, &hlsl.Options{})) },
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
				return dxil.Compile(m, dxil.Options{ShaderModel: dxil.SM6_0, ZeroInitializeWorkgroupMemory: mode})
			},
			withZero: func(m *ir.Module) ([]byte, error) {
				return dxil.Compile(m, dxil.Options{ShaderModel: dxil.SM6_0})
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
				return spirv.NewBackend(spirv.Options{Version: spirv.Version1_3, BoundsCheckPolicies: p}).Compile(m)
			},
			withZero: func(m *ir.Module) ([]byte, error) {
				return spirv.NewBackend(spirv.Options{Version: spirv.Version1_3}).Compile(m)
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
