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
