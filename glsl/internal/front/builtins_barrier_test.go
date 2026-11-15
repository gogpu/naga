package front

// This file covers the GLSL barrier family: the execution barrier barrier()
// and the memory barriers of GLSL 4.50 §8.16 Memory Barrier Functions
// (memoryBarrier, memoryBarrierBuffer, memoryBarrierShared, memoryBarrierImage,
// groupMemoryBarrier). Rust naga's GLSL frontend declares barrier() only
// (builtins.rs:1039-1040), so the memory barriers are our extension (design
// §13, gfx-rs/wgpu#6013).
//
// Every expected scope and memory-semantics operand below was measured with
// glslangValidator on a #version 450 compute shader and read back with
// spirv-dis; the glslang column of each case records it, so a change to the
// mapping cannot pass silently.

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/internal/testtools"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/valid"
)

// SPIR-V opcodes the barrier test reads.
const (
	opConstant       = 43
	opControlBarrier = 224
	opMemoryBarrier  = 225
)

// barrierInstruction is one OpControlBarrier or OpMemoryBarrier of a module,
// with its constant operands resolved.
type barrierInstruction struct {
	control   bool
	execScope uint32 // OpControlBarrier only
	memScope  uint32
	semantics uint32
}

// String renders the instruction the way spirv-dis does, for a readable
// failure message.
func (b barrierInstruction) String() string {
	if b.control {
		return fmt.Sprintf("OpControlBarrier exec=%d mem=%d semantics=%d", b.execScope, b.memScope, b.semantics)
	}
	return fmt.Sprintf("OpMemoryBarrier scope=%d semantics=%d", b.memScope, b.semantics)
}

// decodeBarriers returns the control and memory barriers of a SPIR-V binary,
// with the operand ids resolved through the module's OpConstant instructions.
func decodeBarriers(t *testing.T, blob []byte) []barrierInstruction {
	t.Helper()
	if len(blob)%4 != 0 {
		t.Fatalf("SPIR-V binary length %d is not a multiple of 4", len(blob))
	}
	words := make([]uint32, len(blob)/4)
	for i := range words {
		words[i] = binary.LittleEndian.Uint32(blob[i*4:])
	}

	constants := make(map[uint32]uint32)
	for i := 5; i < len(words); {
		count := int(words[i] >> 16)
		op := words[i] & 0xFFFF
		if count == 0 {
			t.Fatalf("SPIR-V instruction at word %d has zero word count", i)
		}
		if op == opConstant {
			constants[words[i+2]] = words[i+3]
		}
		i += count
	}

	lookup := func(id uint32) uint32 {
		value, ok := constants[id]
		if !ok {
			t.Fatalf("barrier operand %d is not an OpConstant", id)
		}
		return value
	}

	var out []barrierInstruction
	for i := 5; i < len(words); {
		count := int(words[i] >> 16)
		switch words[i] & 0xFFFF {
		case opControlBarrier:
			out = append(out, barrierInstruction{
				control:   true,
				execScope: lookup(words[i+1]),
				memScope:  lookup(words[i+2]),
				semantics: lookup(words[i+3]),
			})
		case opMemoryBarrier:
			out = append(out, barrierInstruction{
				memScope:  lookup(words[i+1]),
				semantics: lookup(words[i+2]),
			})
		}
		i += count
	}
	return out
}

// SPIR-V scope and memory-semantics operands (SPIR-V 1.6, "Scope" and
// "Memory Semantics").
const (
	scopeDevice    = 1
	scopeWorkgroup = 2

	acquireRelease  = 8
	uniformMemory   = 64
	workgroupMemory = 256
	imageMemory     = 2048
)

// barrierCase is one barrier function: the IR flags it must expand to, the
// instruction those flags produce, and what glslang emits for the same call.
// No barrier here needs a capability its flags do not imply, and none needs a
// SPIR-V version above the default: the subgroup bit is exactly what
// barrierFlags avoids.
type barrierCase struct {
	name  string
	flags ir.BarrierFlags
	want  barrierInstruction
	// glslang is the operand text spirv-dis printed for glslangValidator's own
	// compilation of the same function.
	glslang string
}

// barrierCases is the whole family. The glslang column records the two places
// the IR cannot follow it exactly: glslang orders atomic counters (a memory
// class ir.BarrierFlags does not have, and one Vulkan GLSL does not allow) and
// keeps groupMemoryBarrier at Workgroup scope while the IR ties UniformMemory
// to Device scope. Both deviations are over-synchronizing, never
// under-synchronizing; memoryBarrierShared is the mirror case, where the
// narrower Workgroup scope is still enough because workgroup memory is only
// visible inside its workgroup.
func barrierCases() []barrierCase {
	return []barrierCase{
		{
			// barrier() is a workgroup barrier. Rust naga's frontend emits
			// Barrier::all() (front/glsl/builtins.rs:2039), which orders at
			// Subgroup execution scope, adds the uniform and image memory
			// classes at Device scope and raises the module to SPIR-V 1.3 with
			// CapabilitySubgroupBarrier — none of which a workgroup barrier
			// needs, and the version requirement makes an ordinary #version 450
			// compute shader fail on a SPIR-V 1.0 device. The GLSL spec (§8.16)
			// gives barrier() no scope qualifier, and glslang compiles it to
			// exactly the Workgroup/Workgroup barrier below.
			name:  "barrier",
			flags: barrierFlags,
			want: barrierInstruction{
				control:   true,
				execScope: scopeWorkgroup,
				memScope:  scopeWorkgroup,
				semantics: acquireRelease | workgroupMemory,
			},
			glslang: "OpControlBarrier %uint_2 %uint_2 %uint_264",
		},
		{
			name:  "memoryBarrier",
			flags: memoryBarrierAllMemory,
			want: barrierInstruction{
				memScope:  scopeDevice,
				semantics: acquireRelease | uniformMemory | workgroupMemory | imageMemory,
			},
			glslang: "OpMemoryBarrier %uint_1 %uint_3400 (also AtomicCounterMemory = 1024)",
		},
		{
			name:  "memoryBarrierBuffer",
			flags: memoryBarrierBufferMemory,
			want: barrierInstruction{
				memScope:  scopeDevice,
				semantics: acquireRelease | uniformMemory,
			},
			glslang: "OpMemoryBarrier %uint_1 %uint_72",
		},
		{
			name:  "memoryBarrierShared",
			flags: memoryBarrierSharedMemory,
			want: barrierInstruction{
				memScope:  scopeWorkgroup,
				semantics: acquireRelease | workgroupMemory,
			},
			glslang: "OpMemoryBarrier %uint_1 %uint_264 (Device scope)",
		},
		{
			name:  "memoryBarrierImage",
			flags: memoryBarrierImageMemory,
			want: barrierInstruction{
				memScope:  scopeDevice,
				semantics: acquireRelease | uniformMemory | imageMemory,
			},
			glslang: "OpMemoryBarrier %uint_1 %uint_2056 (no UniformMemory)",
		},
		{
			name:  "groupMemoryBarrier",
			flags: groupMemoryBarrierMemory,
			want: barrierInstruction{
				memScope:  scopeDevice,
				semantics: acquireRelease | uniformMemory | workgroupMemory | imageMemory,
			},
			glslang: "OpMemoryBarrier %uint_2 %uint_3400 (Workgroup scope, also AtomicCounterMemory)",
		},
	}
}

// TestBuiltinBarrierFamilyExpands pins the IR each barrier function expands to:
// the statement kind, which is what keeps groupMemoryBarrier a memory barrier
// and barrier() an execution barrier, and the flags.
func TestBuiltinBarrierFamilyExpands(t *testing.T) {
	for _, tt := range barrierCases() {
		t.Run(tt.name, func(t *testing.T) {
			f, c := newTestContext(t)
			decl := &functionDeclaration{}
			f.injectBuiltin(decl, tt.name, variationStandard)
			ov := builtinOverload(t, f, decl)

			if _, ok, err := ov.macro.call(c, nil, ir.Span{}); err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			} else if ok {
				t.Fatalf("%s reported a value; a barrier is a statement", tt.name)
			}

			var statements []ir.StatementKind
			for i := range c.body {
				statements = append(statements, c.body[i].Kind)
			}
			if len(statements) != 1 {
				t.Fatalf("%s emitted %d statements, want one: %v", tt.name, len(statements), statements)
			}

			if tt.want.control {
				barrier, isControl := statements[0].(ir.StmtControlBarrier)
				if !isControl {
					t.Fatalf("%s emitted %T, want StmtControlBarrier", tt.name, statements[0])
				}
				if barrier.Flags != tt.flags {
					t.Fatalf("%s flags are %b, want %b", tt.name, barrier.Flags, tt.flags)
				}
				return
			}
			barrier, isMemory := statements[0].(ir.StmtMemoryBarrier)
			if !isMemory {
				t.Fatalf("%s emitted %T, want StmtMemoryBarrier", tt.name, statements[0])
			}
			if barrier.Flags != tt.flags {
				t.Fatalf("%s flags are %b, want %b", tt.name, barrier.Flags, tt.flags)
			}
		})
	}
}

// TestBuiltinMemoryBarrierAtomicCounterIsRefused checks the deliberate refusal:
// no overload is injected and the reason names the unexpressible memory class,
// so the diagnostic is not a bare "unknown function".
func TestBuiltinMemoryBarrierAtomicCounterIsRefused(t *testing.T) {
	f, _ := newTestContext(t)
	decl := &functionDeclaration{}
	f.injectBuiltin(decl, "memoryBarrierAtomicCounter", variationStandard)

	if len(decl.overloads) != 0 {
		t.Fatalf("memoryBarrierAtomicCounter injected %d overload(s), want none", len(decl.overloads))
	}
	if len(f.errors) != 1 {
		t.Fatalf("recorded %d error(s), want one: %v", len(f.errors), f.errors)
	}
	if message := f.errors[0].Message; !strings.Contains(message, "AtomicCounterMemory") {
		t.Fatalf("error %q does not name the AtomicCounterMemory limitation", message)
	}
}

// TestParseRefusesMemoryBarrierAtomicCounter checks the refusal end to end: the
// reason is the primary diagnostic a shader author sees.
func TestParseRefusesMemoryBarrierAtomicCounter(t *testing.T) {
	source := "#version 450\nlayout(local_size_x = 1) in;\nvoid main() { memoryBarrierAtomicCounter(); }\n"
	_, _, err := Parse(source, Options{Stage: ir.StageCompute})
	if err == nil {
		t.Fatal("Parse accepted memoryBarrierAtomicCounter")
	}
	if !strings.Contains(err.Error(), "AtomicCounterMemory") {
		t.Fatalf("error %q does not name the AtomicCounterMemory limitation", err.Error())
	}
	if len(err.Errors) == 0 || !err.Errors[0].Span.IsDefined() {
		t.Fatalf("refusal carries no source location: %+v", err.Errors)
	}
}

// TestParseBarrierComputeShaders is the end-to-end gate of the family: one
// compute shader per function parses, validates, compiles to SPIR-V with the
// scope and semantics glslang's compilation of the same function implies, and
// passes spirv-val when the validator is installed.
func TestParseBarrierComputeShaders(t *testing.T) {
	for _, tt := range barrierCases() {
		t.Run(tt.name, func(t *testing.T) {
			source := fmt.Sprintf("#version 450\nlayout(local_size_x = 1) in;\nvoid main() { %s(); }\n", tt.name)
			mod, _, err := Parse(source, Options{Stage: ir.StageCompute})
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
				t.Fatalf("the parsed module does not validate: %v", verr)
			}

			// The default options are part of the assertion: no barrier here may
			// raise the module above SPIR-V 1.0, which is exactly what the
			// subgroup bit Rust's barrier() carries used to force.
			binarySPIRV, _, cerr := spirv.Compile(mod, spirv.Options{})
			if cerr != nil {
				t.Fatalf("spirv.Compile: %v", cerr)
			}
			barriers := decodeBarriers(t, binarySPIRV)
			if len(barriers) != 1 {
				t.Fatalf("emitted %d barriers, want one: %v", len(barriers), barriers)
			}
			if got := barriers[0]; got != tt.want {
				t.Fatalf("emitted %v, want %v (glslang emits %s)", got, tt.want, tt.glslang)
			}

			if validator, ok := testtools.Optional(t, "spirv-val", testtools.InstallSPIRVTools); ok {
				path := filepath.Join(t.TempDir(), "barrier.spv")
				if werr := os.WriteFile(path, binarySPIRV, 0o600); werr != nil {
					t.Fatalf("write the SPIR-V binary: %v", werr)
				}
				if out, verr := exec.Command(validator, path).CombinedOutput(); verr != nil {
					t.Fatalf("spirv-val: %v: %s", verr, out)
				}
			}
		})
	}
}

// TestParseBarrierSharedCompiles is the regression the barrier mapping exists
// for: an ordinary #version 450 compute shader guarding a shared buffer with
// barrier() must compile at the default SPIR-V version. Rust naga's
// Barrier::all() mapping made this fail with "a subgroup barrier needs SPIR-V
// 1.3, Options.Version is 1.0".
func TestParseBarrierSharedCompiles(t *testing.T) {
	source := `#version 450
layout(local_size_x = 64) in;
shared uint counter;
void main() {
    counter = 1u;
    barrier();
    counter = counter + 1u;
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageCompute})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}

	// The zero version is SPIR-V 1.0, the version every Vulkan 1.0 device
	// accepts. barrier() must not need a newer one.
	blob, _, cerr := spirv.Compile(mod, spirv.Options{})
	if cerr != nil {
		t.Fatalf("spirv.Compile at the default SPIR-V version: %v", cerr)
	}

	// Two barriers are expected: the entry-point wrapper initializes the
	// shared variable under a single-invocation guard and borders the
	// initialization with a workgroup barrier, then the body's own barrier()
	// follows.
	barriers := decodeBarriers(t, blob)
	if len(barriers) == 0 {
		t.Fatal("emitted no barrier")
	}
	want := barrierInstruction{control: true, execScope: scopeWorkgroup, memScope: scopeWorkgroup, semantics: acquireRelease | workgroupMemory}
	for i, got := range barriers {
		if got != want {
			t.Fatalf("barrier %d is %v, want %v", i, got, want)
		}
	}

	if validator, ok := testtools.Optional(t, "spirv-val", testtools.InstallSPIRVTools); ok {
		path := filepath.Join(t.TempDir(), "shared.spv")
		if werr := os.WriteFile(path, blob, 0o600); werr != nil {
			t.Fatalf("write the SPIR-V binary: %v", werr)
		}
		if out, verr := exec.Command(validator, path).CombinedOutput(); verr != nil {
			t.Fatalf("spirv-val: %v: %s", verr, out)
		}
	}
}

// computeBarrierModule parses one compute shader calling call and returns the
// module, ready for a backend.
func computeBarrierModule(t *testing.T, call string) *ir.Module {
	t.Helper()
	source := fmt.Sprintf("#version 450\nlayout(local_size_x = 1) in;\nvoid main() { %s; }\n", call)
	mod, _, err := Parse(source, Options{Stage: ir.StageCompute})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return mod
}

// TestBarrierBackendRendering records how the two text backends render the
// partial flags, which the IR can carry and GLSL's own barrier functions ask
// for:
//
//   - HLSL expresses every class the IR names: DeviceMemoryBarrier() for
//     Uniform/StorageBuffer and Image memory, GroupMemoryBarrier() for
//     WorkGroup memory, with no group sync, which is exactly a memory barrier.
//   - Metal has no barrier without execution sync, so MSL writes the memory
//     barrier as a threadgroup_barrier (msl/internal/codegen/statements.go,
//     writeBarrier). It is a superset of what was asked for and is recorded
//     here rather than papered over.
func TestBarrierBackendRendering(t *testing.T) {
	tests := []struct {
		name     string
		call     string
		wantMSL  []string
		wantHLSL []string
	}{
		{
			name:     "memoryBarrierBuffer",
			call:     "memoryBarrierBuffer()",
			wantMSL:  []string{"threadgroup_barrier(metal::mem_flags::mem_device);"},
			wantHLSL: []string{"DeviceMemoryBarrier();"},
		},
		{
			name:     "memoryBarrierShared",
			call:     "memoryBarrierShared()",
			wantMSL:  []string{"threadgroup_barrier(metal::mem_flags::mem_threadgroup);"},
			wantHLSL: []string{"GroupMemoryBarrier();"},
		},
		{
			name:     "memoryBarrier",
			call:     "memoryBarrier()",
			wantMSL:  []string{"mem_device", "mem_threadgroup", "mem_texture"},
			wantHLSL: []string{"DeviceMemoryBarrier();", "GroupMemoryBarrier();"},
		},
		{
			name:     "groupMemoryBarrier",
			call:     "groupMemoryBarrier()",
			wantMSL:  []string{"mem_device", "mem_threadgroup", "mem_texture"},
			wantHLSL: []string{"DeviceMemoryBarrier();", "GroupMemoryBarrier();"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mod := computeBarrierModule(t, tt.call)

			mslText, _, err := msl.Compile(mod, msl.Options{})
			if err != nil {
				t.Fatalf("msl.Compile: %v", err)
			}
			for _, want := range tt.wantMSL {
				if !strings.Contains(mslText, want) {
					t.Fatalf("MSL output has no %q:\n%s", want, mslText)
				}
			}
			// A memory barrier must never gain a group sync in HLSL: that is
			// what a control barrier writes.
			if strings.Contains(mslText, "mem_none") {
				t.Fatalf("MSL wrote an execution-only barrier for a memory barrier:\n%s", mslText)
			}

			hlslText, _, err := hlsl.Compile(mod, hlsl.Options{})
			if err != nil {
				t.Fatalf("hlsl.Compile: %v", err)
			}
			for _, want := range tt.wantHLSL {
				if !strings.Contains(hlslText, want) {
					t.Fatalf("HLSL output has no %q:\n%s", want, hlslText)
				}
			}
			if strings.Contains(hlslText, "WithGroupSync") {
				t.Fatalf("HLSL wrote an execution barrier for a memory barrier:\n%s", hlslText)
			}
		})
	}
}
