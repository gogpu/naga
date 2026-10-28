package dxil

import (
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
)

// Math built-ins and derivatives on vectors must produce a value for every
// component. DXIL has no vector values: the emitter scalarizes, so each
// component needs its own instruction, as emitMathVectorized does for the
// plain dx.op math functions. DXC does the same for derivatives: its golden
// snapshot/testdata/golden/dxil-dxc/standard_fs_derivatives.ll has four
// DerivCoarseX, four DerivCoarseY and eight FAbs for fwidth on a vec4.
//
// The functions below took a path that emitted one instruction on the
// first component only (P0-DXIL-048 derivatives, P0-DXIL-054 step and
// smoothstep). The other components then read whatever value the emitter
// had emitted next: wrong results where the bitcode stayed well formed
// (gg's msdf_text, blend and composite shaders), and records that do not
// parse ("Invalid record") where it did not.

// countCalls counts the calls to the named function in fn.
func countCalls(fn *module.Function, name string) int {
	n := 0
	for _, bb := range fn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrCall && instr.CalledFunc != nil && instr.CalledFunc.Name == name {
				n++
			}
		}
	}
	return n
}

// countKind counts the instructions of one kind in fn.
func countKind(fn *module.Function, kind module.InstrKind) int {
	n := 0
	for _, bb := range fn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == kind {
				n++
			}
		}
	}
	return n
}

func TestVectorDerivativesPerComponent(t *testing.T) {
	// unary = calls to dx.op.unary.f32: one derivative per component and
	// axis, plus two FAbs per component for fwidth.
	tests := []struct {
		name  string
		src   string
		unary int
	}{
		{"dpdx f32", `@fragment fn main(@location(0) v: f32) -> @location(0) vec4<f32> {
    return vec4<f32>(dpdx(v), 0.0, 0.0, 1.0);
}`, 1},
		{"dpdx vec3", `@fragment fn main(@location(0) v: vec3<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(dpdx(v), 1.0);
}`, 3},
		{"dpdyFine vec4", `@fragment fn main(@location(0) v: vec4<f32>) -> @location(0) vec4<f32> {
    return dpdyFine(v);
}`, 4},
		{"fwidth f32", `@fragment fn main(@location(0) v: f32) -> @location(0) vec4<f32> {
    return vec4<f32>(fwidth(v), 0.0, 0.0, 1.0);
}`, 4},
		{"fwidth vec2", `@fragment fn main(@location(0) v: vec2<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(fwidth(v), 0.0, 1.0);
}`, 8},
		{"fwidthFine vec4", `@fragment fn main(@location(0) v: vec4<f32>) -> @location(0) vec4<f32> {
    return fwidthFine(v);
}`, 16},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, fn := emitMainModule(t, tt.src)
			if got := countCalls(fn, "dx.op.unary.f32"); got != tt.unary {
				t.Errorf("dx.op.unary.f32 calls = %d, want %d (one per component)", got, tt.unary)
			}
			compileWGSLToDXIL(t, tt.src)
		})
	}
}

func TestVectorCompositeMathPerComponent(t *testing.T) {
	// Each function's characteristic instruction, counted in main: one per
	// component (pow: log + exp, sign: two compares and two selects).
	const buf = "@group(0) @binding(0) var<storage, read_write> b: array<vec4<u32>, 2>;\n"
	tests := []struct {
		name  string
		src   string
		call  string           // dx.op function to count, or ""
		kind  module.InstrKind // instruction kind to count when call is ""
		count int
	}{
		{"pow vec3", `@fragment fn main(@location(0) x: vec3<f32>, @location(1) y: vec3<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(pow(x, y), 1.0);
}`, "dx.op.unary.f32", 0, 6},
		{"degrees vec2", `@fragment fn main(@location(0) v: vec2<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(degrees(v), 0.0, 1.0);
}`, "", module.InstrBinOp, 2},
		{"radians vec4", `@fragment fn main(@location(0) v: vec4<f32>) -> @location(0) vec4<f32> {
    return radians(v);
}`, "", module.InstrBinOp, 4},
		{"sign vec3 f32", `@fragment fn main(@location(0) v: vec3<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(sign(v), 1.0);
}`, "", module.InstrSelect, 6},
		{"sign vec2 i32", buf + `@compute @workgroup_size(1) fn main() {
    let v = bitcast<vec2<i32>>(b[0].xy);
    b[1] = vec4<u32>(bitcast<vec2<u32>>(sign(v)), 0u, 0u);
}`, "", module.InstrSelect, 4},
		{"extractBits vec2 u32", buf + `@compute @workgroup_size(1) fn main() {
    b[1] = vec4<u32>(extractBits(b[0].xy, 3u, 5u), 0u, 0u);
}`, "dx.op.tertiary.i32", 0, 2},
		{"insertBits vec3 u32", buf + `@compute @workgroup_size(1) fn main() {
    b[1] = vec4<u32>(insertBits(b[0].xyz, b[1].xyz, 4u, 8u), 0u);
}`, "dx.op.quaternary.i32", 0, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, fn := emitMainModule(t, tt.src)
			var got int
			if tt.call != "" {
				got = countCalls(fn, tt.call)
			} else {
				got = countKind(fn, tt.kind)
			}
			if got != tt.count {
				t.Errorf("count = %d, want %d (one chain per component)", got, tt.count)
			}
			compileWGSLToDXIL(t, tt.src)
		})
	}
}

func TestVectorStepPerComponent(t *testing.T) {
	// step(edge, x) = select(x >= edge, 1, 0): one fcmp and one select per
	// component. smoothstep has no compare; its per-component fsub count
	// (x - e0 and e1 - e0) is the witness.
	tests := []struct {
		name    string
		src     string
		kind    module.InstrKind
		perComp int
		comps   int
	}{
		{"step f32", `@fragment fn main(@location(0) v: f32) -> @location(0) vec4<f32> {
    return vec4<f32>(step(0.5, v), 0.0, 0.0, 1.0);
}`, module.InstrSelect, 1, 1},
		{"step vec3 splat edge", `@fragment fn main(@location(0) v: vec3<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(step(vec3<f32>(0.5), v), 1.0);
}`, module.InstrSelect, 1, 3},
		{"step vec2 vector edge", `@fragment fn main(@location(0) e: vec2<f32>, @location(1) v: vec2<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(step(e, v), 0.0, 1.0);
}`, module.InstrSelect, 1, 2},
		{"smoothstep vec3", `@fragment fn main(@location(0) e0: vec3<f32>, @location(1) e1: vec3<f32>, @location(2) v: vec3<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(smoothstep(e0, e1, v), 1.0);
}`, module.InstrBinOp, 0, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, fn := emitMainModule(t, tt.src)
			if tt.kind == module.InstrSelect {
				if got := countKind(fn, module.InstrCmp); got != tt.comps {
					t.Errorf("compares = %d, want %d (one per component)", got, tt.comps)
				}
				if got := countKind(fn, module.InstrSelect); got != tt.perComp*tt.comps {
					t.Errorf("selects = %d, want %d (one per component)", got, tt.perComp*tt.comps)
				}
			} else if got := countKind(fn, module.InstrBinOp); got < 5*tt.comps {
				// x-e0, e1-e0, div, t*t, 2*t, 3-2t, mul: at least 5 binops per component.
				t.Errorf("binops = %d, want at least %d (smoothstep per component)", got, 5*tt.comps)
			}
			compileWGSLToDXIL(t, tt.src)
		})
	}
}
