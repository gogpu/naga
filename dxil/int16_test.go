package dxil

import (
	"errors"
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
)

// TestInt16Refused: 16-bit integers (enable wgpu_int16) are refused up
// front. Native i16/u16 in DXIL is DXC's -enable-16bit-types at shader
// model 6.2 (NativeLowPrecision, 16-bit dx.op overloads, 2-byte raw buffer
// access); the emitter does not implement it, and without the check an i16
// went through the 32-bit paths, a module whose values are not WGSL's.
// Rust naga has no DXIL backend; its HLSL output for int16.wgsl targets
// SM 6.2 with int16_t.
func TestInt16Refused(t *testing.T) {
	for name, src := range map[string]string{
		"i16 storage": `enable wgpu_int16;
@group(0) @binding(0) var<storage, read_write> a: array<i16, 2>;
@compute @workgroup_size(1) fn main() { a[0] = a[1] + i16(5); }`,
		"u16 local": `enable wgpu_int16;
@group(0) @binding(0) var<storage, read_write> o: u32;
@compute @workgroup_size(1) fn main() { var v = vec2<u16>(u16(o), u16(2)); o = u32(v.x + v.y); }`,
		"i16 literal only": `enable wgpu_int16;
@group(0) @binding(0) var<storage, read_write> o: i32;
@compute @workgroup_size(1) fn main() { o = i32(i16(o) * i16(3)); }`,
	} {
		for _, sm := range []backend.ShaderModel{{Major: 6, Minor: 0}, {Major: 6, Minor: 2}} {
			_, _, err := Compile(lowerForTest(t, src), Options{ShaderModel: sm})
			if !errors.Is(err, backend.ErrUnsupported) || !strings.Contains(err.Error(), "16-bit integer") {
				t.Errorf("%s, SM %d.%d: err = %v, want the unsupported 16-bit integer error", name, sm.Major, sm.Minor, err)
			}
		}
	}
}
