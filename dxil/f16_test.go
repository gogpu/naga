package dxil

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
)

// Every float constant an f16 or f64 operation takes is of its type: the
// constants of sign, negation, step, smoothstep, degrees, refract and
// frexp, a zero value and constants folded from ones of the type. They
// were f32 constants, which a half or double operation can't take
// ("Invalid record").
func TestFloatConstantsOfTheOperationsType(t *testing.T) {
	for _, ty := range []struct {
		name, enable, suffix string
		bits                 uint
	}{{"f16", "enable f16;", "h", 16}, {"f64", "", "lf", 64}} {
		mod, fn := emitMainModule(t, strings.NewReplacer("{T}", ty.name, "{h}", ty.suffix).Replace(ty.enable+`
@group(0) @binding(0) var<storage, read_write> s: array<{T}>;
@compute @workgroup_size(1) fn main() {
	let x = s[0];
	var z: vec2<{T}>;
	s[1] = sign(x) - x + -x + step(x, 0.5{h}) + smoothstep(0.0{h}, 1.0{h}, x) + degrees(x) + (1{h} - 33333{h}) + z.y;
	s[2] = refract(vec2(x), vec2(x), x).y + frexp(x).fract;
}
`))
		consts := map[int]*module.Constant{}
		for _, c := range mod.Constants {
			consts[c.ValueID] = c
		}
		for _, bb := range fn.BasicBlocks {
			for _, in := range bb.Instructions {
				if in.ResultType == nil || in.ResultType.Kind != module.TypeFloat || in.ResultType.FloatBits != ty.bits {
					continue
				}
				for _, op := range in.Operands {
					if c, ok := consts[op]; ok && c.ConstType != nil && c.ConstType.Kind == module.TypeFloat && c.ConstType.FloatBits != ty.bits {
						t.Errorf("%s instruction %%%d takes the f%d constant %v", ty.name, in.ValueID, c.ConstType.FloatBits, c.FloatValue)
					}
				}
			}
		}
	}
}
