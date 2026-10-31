package dxil

import (
	"regexp"
	"strconv"
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
)

// A helper inlined into a helper that is itself inlined leaves a chain of
// return-slot locals: the inner result is stored into its slot, loaded and
// stored into the outer slot, whose load the entry point uses. DCE decided
// the dead locals before it saw that the outer slot's store reads the inner
// slot, so it dropped the inner store and the whole inner computation: the
// load read an uninitialized alloca (P0-DXIL-055, gg's blend.wgsl on the
// DXIL path returned garbage for most blend modes). Any two locals in a row
// hit it, so a helper's result assigned to a var lost its body too.
// Validators accept that bitcode, so the tests count the instructions the
// inner helper must emit.

func TestInlinedHelperChainKeepsInnerBody(t *testing.T) {
	const overlay = `fn blend_overlay(src: vec3<f32>, dst: vec3<f32>) -> vec3<f32> {
    return mix(2.0 * src * dst, 1.0 - 2.0 * (1.0 - src) * (1.0 - dst), step(vec3<f32>(0.5), dst));
}
`
	const in = "struct In { @location(0) src: vec3<f32>, @location(1) dst: vec3<f32>, @location(2) @interpolate(flat) mode: u32 }\n"
	// cmp = two icmp for the switch cases (where there is a switch) plus
	// three fcmp for step on a vec3.
	tests := []struct {
		name string
		src  string
		cmp  int
	}{
		{"switch in a helper", in + overlay + `fn blend_separable(src: vec3<f32>, dst: vec3<f32>, mode: u32) -> vec3<f32> {
    switch mode {
        case 1u: { return src; }
        case 3u: { return blend_overlay(src, dst); }
        default: { return dst; }
    }
}
@fragment fn main(in: In) -> @location(0) vec4<f32> {
    return vec4<f32>(blend_separable(in.src, in.dst, in.mode), 1.0);
}`, 5},
		{"two plain helpers", in + overlay + `fn twice(src: vec3<f32>, dst: vec3<f32>) -> vec3<f32> {
    return blend_overlay(src, dst) * 2.0;
}
@fragment fn main(in: In) -> @location(0) vec4<f32> {
    return vec4<f32>(twice(in.src, in.dst), 1.0);
}`, 3},
		// The helper's slot feeds a var: a chain of two locals as well.
		{"switch in the entry point", in + overlay + `@fragment fn main(in: In) -> @location(0) vec4<f32> {
    var c = in.dst;
    switch in.mode {
        case 1u: { c = in.src; }
        case 3u: { c = blend_overlay(in.src, in.dst); }
        default: {}
    }
    return vec4<f32>(c, 1.0);
}`, 5},
		// Control: one local between the helper and the root was already right.
		{"one helper", in + overlay + `@fragment fn main(in: In) -> @location(0) vec4<f32> {
    return vec4<f32>(blend_overlay(in.src, in.dst), 1.0);
}`, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, fn := emitMainModule(t, tt.src)
			if got := countKind(fn, module.InstrCmp); got != tt.cmp {
				t.Errorf("compares = %d, want %d (the inlined blend_overlay must keep its step)", got, tt.cmp)
			}
			compileWGSLToDXIL(t, tt.src)
		})
	}
}

var (
	reGEPLast   = regexp.MustCompile(`^\s*(%\d+) = getelementptr inbounds .*, i32 (\d+)$`)
	reLoadPtr   = regexp.MustCompile(`^\s*(%\d+) = load float, float\* (%\d+)`)
	reStoreOutF = regexp.MustCompile(`@dx\.op\.storeOutput\.f32\(i32 5, i32 0, i32 0, i8 (\d), float (%\d+)\)`)
)

// outputElements maps each output component written from a load to the
// last GEP index of the pointer it was loaded through: the element of the
// flattened alloca it reads.
func outputElements(text string) map[int]int {
	gepIdx := map[string]int{}
	loadIdx := map[string]int{}
	out := map[int]int{}
	for _, line := range regexp.MustCompile("\r?\n").Split(text, -1) {
		if m := reGEPLast.FindStringSubmatch(line); m != nil {
			gepIdx[m[1]], _ = strconv.Atoi(m[2])
		}
		if m := reLoadPtr.FindStringSubmatch(line); m != nil {
			if i, ok := gepIdx[m[2]]; ok {
				loadIdx[m[1]] = i
			}
		}
		if m := reStoreOutF.FindStringSubmatch(line); m != nil {
			comp, _ := strconv.Atoi(m[1])
			if i, ok := loadIdx[m[2]]; ok {
				out[comp] = i
			}
		}
	}
	return out
}

// An element of a nested aggregate read from a helper's return slot
// (a[i][j], a[i].y) must load that element of the slot's flattened alloca.
// The value access took one index as the flat index and loaded a single
// scalar for a[i], so a[0][1] read a[0][0]'s slot and its component lookup
// returned an unrelated value ID: a wrong value while DCE dropped the slot's
// stores anyway, "Invalid record" once the stores were kept.
func TestNestedAggregateFromHelperElements(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want map[int]int // output component -> element of the flat alloca
	}{
		{"array of arrays", `fn ret_array() -> array<f32, 2> { return array<f32, 2>(1.0, 2.0); }
fn ret_aa() -> array<array<f32, 2>, 1> { return array<array<f32, 2>, 1>(ret_array()); }
@fragment fn main() -> @location(0) vec4<f32> {
    let a = ret_aa();
    return vec4<f32>(a[0][0], a[0][1], 0.0, 1.0);
}`, map[int]int{0: 0, 1: 1}},
		{"array of arrays, later elements", `fn ret_array() -> array<f32, 2> { return array<f32, 2>(1.0, 2.0); }
fn ret_aa() -> array<array<f32, 2>, 3> { return array<array<f32, 2>, 3>(ret_array(), ret_array(), ret_array()); }
@fragment fn main() -> @location(0) vec4<f32> {
    let a = ret_aa();
    return vec4<f32>(a[2][1], a[1][0], a[0][1], 1.0);
}`, map[int]int{0: 5, 1: 2, 2: 1}},
		{"array of vectors", `fn ret_av(x: f32) -> array<vec2<f32>, 2> { return array<vec2<f32>, 2>(vec2<f32>(x, 2.0), vec2<f32>(3.0, x)); }
@fragment fn main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let a = ret_av(x);
    return vec4<f32>(a[1].y, a[0].x, a[1].x, 1.0);
}`, map[int]int{0: 3, 1: 0, 2: 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, ok := dumpbin(t, tt.src)
			if !ok {
				return
			}
			got := outputElements(text)
			for comp, want := range tt.want {
				if g, ok := got[comp]; !ok || g != want {
					t.Errorf("output component %d: element %d (loaded: %v), want element %d", comp, g, ok, want)
				}
			}
		})
	}
}
