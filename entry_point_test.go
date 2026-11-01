package naga

import (
	"bytes"
	"encoding/binary"
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/wgsl"
)

// entryPointShader has one entry point per stage. Each calls a function only
// its stage can run (derivatives: fragment, barriers: compute; Rust naga's
// separate-entry-points.wgsl) and only the compute one uses buf, so a SPIR-V
// module for one entry point leaves out the other's function and buf.
const entryPointShader = `
fn derivatives() { let x = dpdx(0.0); }
fn barriers() { workgroupBarrier(); }
@group(0) @binding(0) var<storage, read_write> buf: array<u32, 4>;
@vertex fn vs() -> @builtin(position) @invariant vec4<f32> { return vec4<f32>(); }
@fragment fn fs() -> @location(0) vec4<f32> { derivatives(); return vec4<f32>(); }
@compute @workgroup_size(1) fn cs() { barriers(); buf[0] = 1u; }`

func entryPointModule(t *testing.T) *ir.Module {
	t.Helper()
	module, _, err := wgsl.Parse(entryPointShader, wgsl.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return module
}

var (
	selVS = &backend.EntryPointSelector{Stage: ir.StageVertex, Name: "vs"}
	selFS = &backend.EntryPointSelector{Stage: ir.StageFragment, Name: "fs"}
	selCS = &backend.EntryPointSelector{Stage: ir.StageCompute, Name: "cs"}
)

// spirvBinary is spirv.Compile without its TranslationInfo.
func spirvBinary(module *ir.Module, opts spirv.Options) ([]byte, error) {
	binary, _, err := spirv.Compile(module, opts)
	return binary, err
}

// dxilBlob is dxil.Compile without its TranslationInfo.
func dxilBlob(module *ir.Module, opts dxil.Options) ([]byte, error) {
	blob, _, err := dxil.Compile(module, opts)
	return blob, err
}

// entryPointBackends compiles a module with an entry point selector in each
// backend.
func entryPointBackends() map[string]func(*ir.Module, *backend.EntryPointSelector) ([]byte, error) {
	return map[string]func(*ir.Module, *backend.EntryPointSelector) ([]byte, error){
		"spirv": func(m *ir.Module, sel *backend.EntryPointSelector) ([]byte, error) {
			return spirvBinary(m, spirv.Options{EntryPoint: sel})
		},
		"msl": func(m *ir.Module, sel *backend.EntryPointSelector) ([]byte, error) {
			src, _, err := msl.Compile(m, msl.Options{EntryPoint: sel})
			return []byte(src), err
		},
		"hlsl": func(m *ir.Module, sel *backend.EntryPointSelector) ([]byte, error) {
			src, _, err := hlsl.Compile(m, hlsl.Options{EntryPoint: sel})
			return []byte(src), err
		},
		"glsl": func(m *ir.Module, sel *backend.EntryPointSelector) ([]byte, error) {
			src, _, err := glsl.Compile(m, glsl.Options{LangVersion: glsl.Version430, EntryPoint: sel})
			return []byte(src), err
		},
		"dxil": func(m *ir.Module, sel *backend.EntryPointSelector) ([]byte, error) {
			return dxilBlob(m, dxil.Options{EntryPoint: sel})
		},
	}
}

// TestEntryPointNotFound: a selector the module lacks, by name or by stage,
// is an error wrapping backend.ErrEntryPointNotFound in every backend.
func TestEntryPointNotFound(t *testing.T) {
	for name, compile := range entryPointBackends() {
		for _, sel := range []*backend.EntryPointSelector{
			{Stage: ir.StageCompute, Name: "missing"},
			{Stage: ir.StageVertex, Name: "fs"}, // fs is a fragment entry point
		} {
			_, err := compile(entryPointModule(t), sel)
			if !errors.Is(err, backend.ErrEntryPointNotFound) {
				t.Errorf("%s, %s %q: err = %v, want backend.ErrEntryPointNotFound", name, sel.Stage, sel.Name, err)
			}
		}
	}
}

// TestEntryPointNilSelector: GLSL and a DXIL container hold one entry point,
// so nil is an error for a module with several; SPIR-V, MSL and HLSL write
// them all.
func TestEntryPointNilSelector(t *testing.T) {
	single := map[string]bool{"glsl": true, "dxil": true}
	for name, compile := range entryPointBackends() {
		_, err := compile(entryPointModule(t), nil)
		if single[name] {
			if !errors.Is(err, backend.ErrEntryPointNotFound) {
				t.Errorf("%s: err = %v, want backend.ErrEntryPointNotFound", name, err)
			}
		} else if err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	// One entry point needs no selector.
	m := entryPointModule(t)
	m.EntryPoints = m.EntryPoints[2:]
	for name, compile := range entryPointBackends() {
		if _, err := compile(m, nil); err != nil {
			t.Errorf("%s, module with only cs: %v", name, err)
		}
	}
}

// TestEntryPointTextSelection: the text backends write the selected entry
// point only, without the functions of the others' stages' names changed.
func TestEntryPointTextSelection(t *testing.T) {
	backends := entryPointBackends()
	for _, tt := range []struct {
		backend   string
		sel       *backend.EntryPointSelector
		want, not []string
	}{
		{"msl", selFS, []string{"fragment "}, []string{"vertex ", "kernel "}},
		{"msl", selCS, []string{"kernel "}, []string{"vertex ", "fragment "}},
		{"hlsl", selCS, []string{"[numthreads(1, 1, 1)]", " cs("}, []string{" vs(", " fs("}},
		{"hlsl", selVS, []string{" vs("}, []string{"[numthreads", " fs("}},
		{"glsl", selCS, []string{"layout(local_size_x = 1"}, []string{"gl_Position"}},
		{"glsl", selVS, []string{"gl_Position"}, []string{"local_size_x"}},
	} {
		out, err := backends[tt.backend](entryPointModule(t), tt.sel)
		if err != nil {
			t.Errorf("%s %s: %v", tt.backend, tt.sel.Name, err)
			continue
		}
		for _, s := range tt.want {
			if !strings.Contains(string(out), s) {
				t.Errorf("%s %s: output lacks %q:\n%s", tt.backend, tt.sel.Name, s, out)
			}
		}
		for _, s := range tt.not {
			if strings.Contains(string(out), s) {
				t.Errorf("%s %s: output has %q:\n%s", tt.backend, tt.sel.Name, s, out)
			}
		}
	}
}

// TestEntryPointSameNameTwoStages: a selector names a stage and a name, as a
// SPIR-V or GLSL input may hold two entry points of one name.
func TestEntryPointSameNameTwoStages(t *testing.T) {
	m := entryPointModule(t)
	m.EntryPoints[1].Name = "vs" // the fragment entry point
	src, _, err := msl.Compile(m, msl.Options{EntryPoint: &backend.EntryPointSelector{Stage: ir.StageFragment, Name: "vs"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(src, "fragment ") || strings.Contains(src, "vertex ") {
		t.Errorf("selected the vertex entry point:\n%s", src)
	}
}

// spirvOps returns the opcodes of a SPIR-V binary and each instruction's
// operand words.
func spirvOps(t *testing.T, bin []byte) (ops []uint32, operands [][]uint32) {
	t.Helper()
	words := make([]uint32, len(bin)/4)
	for i := range words {
		words[i] = binary.LittleEndian.Uint32(bin[i*4:])
	}
	for i := 5; i < len(words); {
		n := int(words[i] >> 16)
		if n == 0 || i+n > len(words) {
			t.Fatalf("malformed instruction at word %d", i)
		}
		ops = append(ops, words[i]&0xFFFF)
		operands = append(operands, words[i+1:i+n])
		i += n
	}
	return ops, operands
}

const (
	opEntryPoint = 15
	opFunction   = 54
	opVariable   = 59
	opDecorate   = 71

	storageClassStorageBuffer = 12
	decorationInvariant       = 18
)

func countOp(ops []uint32, op uint32) int {
	n := 0
	for _, o := range ops {
		if o == op {
			n++
		}
	}
	return n
}

// TestSPIRVEntryPointFilter: a SPIR-V module for one entry point leaves out
// the globals it does not use and the functions it can't call (Rust naga's
// write_logical_layout with an ep_index): the fragment module has no
// barriers() and no buf, the compute one no derivatives().
func TestSPIRVEntryPointFilter(t *testing.T) {
	storageVars := func(ops []uint32, operands [][]uint32) int {
		n := 0
		for i, op := range ops {
			if op == opVariable && operands[i][2] == storageClassStorageBuffer {
				n++
			}
		}
		return n
	}
	for _, tt := range []struct {
		sel                   *backend.EntryPointSelector
		entryPoints, funcs    int
		storageBufferVariable int
	}{
		{nil, 3, 5, 1},   // vs, fs, cs, derivatives, barriers
		{selFS, 1, 2, 0}, // fs, derivatives
		{selCS, 1, 2, 1}, // cs, barriers
		{selVS, 1, 1, 0}, // vs: both helpers are stage-bound
	} {
		name := "nil"
		if tt.sel != nil {
			name = tt.sel.Name
		}
		bin, err := spirvBinary(entryPointModule(t), spirv.Options{EntryPoint: tt.sel})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		ops, operands := spirvOps(t, bin)
		if got := countOp(ops, opEntryPoint); got != tt.entryPoints {
			t.Errorf("%s: %d OpEntryPoint, want %d", name, got, tt.entryPoints)
		}
		if got := countOp(ops, opFunction); got != tt.funcs {
			t.Errorf("%s: %d OpFunction, want %d", name, got, tt.funcs)
		}
		if got := storageVars(ops, operands); got != tt.storageBufferVariable {
			t.Errorf("%s: %d StorageBuffer variables, want %d", name, got, tt.storageBufferVariable)
		}
	}
}

// TestSPIRVInvariantPosition: @invariant on the position built-in is the
// Invariant decoration (Rust naga back/spv/writer.rs, Position { invariant });
// the SPIR-V backend used to drop it.
func TestSPIRVInvariantPosition(t *testing.T) {
	bin, err := spirvBinary(entryPointModule(t), spirv.Options{})
	if err != nil {
		t.Fatal(err)
	}
	ops, operands := spirvOps(t, bin)
	n := 0
	for i, op := range ops {
		if op == opDecorate && operands[i][1] == decorationInvariant {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d Invariant decorations, want 1 (the vertex position output)", n)
	}
}

// TestTranslationInfoEntryPointNames: spirv and dxil report the entry points
// they wrote, by name, as the text backends do.
func TestTranslationInfoEntryPointNames(t *testing.T) {
	_, all, err := spirv.Compile(entryPointModule(t), spirv.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"vs": "vs", "fs": "fs", "cs": "cs"}; !maps.Equal(all.EntryPointNames, want) {
		t.Errorf("spirv, every entry point: %v, want %v", all.EntryPointNames, want)
	}
	_, one, err := spirv.Compile(entryPointModule(t), spirv.Options{EntryPoint: selFS})
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"fs": "fs"}; !maps.Equal(one.EntryPointNames, want) {
		t.Errorf("spirv, fs: %v, want %v", one.EntryPointNames, want)
	}
	_, info, err := dxil.Compile(entryPointModule(t), dxil.Options{EntryPoint: selCS})
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"cs": "cs"}; !maps.Equal(info.EntryPointNames, want) {
		t.Errorf("dxil, cs: %v, want %v", info.EntryPointNames, want)
	}
}

// TestDXILEntryPointSelection: each selection compiles its own container.
func TestDXILEntryPointSelection(t *testing.T) {
	var blobs [][]byte
	for _, sel := range []*backend.EntryPointSelector{selVS, selFS, selCS} {
		blob, _, err := dxil.Compile(entryPointModule(t), dxil.Options{EntryPoint: sel, FakeMissingBindings: true})
		if err != nil {
			t.Fatalf("%s: %v", sel.Name, err)
		}
		for _, b := range blobs {
			if bytes.Equal(b, blob) {
				t.Errorf("%s: same container as an earlier selection", sel.Name)
			}
		}
		blobs = append(blobs, blob)
	}
}
