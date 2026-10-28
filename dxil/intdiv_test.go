package dxil

import (
	"regexp"
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
)

// LLVM's sdiv, udiv, srem and urem are undefined for a zero divisor and for
// the most negative value divided by -1; WGSL defines e1 / 0 = e1,
// MIN / -1 = MIN, e1 % 0 = 0 and MIN % -1 = 0 for run-time operands. naga's
// HLSL backend writes naga_div/naga_mod wrappers that replace such a divisor
// by 1, and DXC compiles them to a select in front of the division, which it
// folds away for a constant divisor that cannot trigger it (P1-DXIL-043).

func TestIntegerDivisionGuards(t *testing.T) {
	tests := []struct {
		name string
		body string
		// selects is the number of guard selects; signed guards also compare
		// the dividend with the most negative value.
		selects int
		minCmp  bool
		op      string // the LLVM division instruction in dxc's disassembly
	}{
		{"i32 / run-time", "s[2] = bitcast<u32>(bitcast<i32>(s[0]) / bitcast<i32>(s[1]));", 1, true, "sdiv"},
		{"i32 % run-time", "s[2] = bitcast<u32>(bitcast<i32>(s[0]) % bitcast<i32>(s[1]));", 1, true, "srem"},
		{"u32 / run-time", "s[2] = s[0] / s[1];", 1, false, "udiv"},
		{"u32 % run-time", "s[2] = s[0] % s[1];", 1, false, "urem"},
		{"vec2 i32 % run-time", "let r = vec2<i32>(bitcast<i32>(s[0]), bitcast<i32>(s[1])) % vec2<i32>(bitcast<i32>(s[2]), bitcast<i32>(s[3]));\n    s[4] = bitcast<u32>(r.x); s[5] = bitcast<u32>(r.y);", 2, true, "srem"},
		// A constant divisor that cannot be 0 or -1 needs no guard (DXC
		// folds it), and -1 still needs the dividend check.
		{"i32 / 3", "s[2] = bitcast<u32>(bitcast<i32>(s[0]) / 3);", 0, false, "sdiv"},
		{"u32 % 6", "s[2] = s[0] % 6u;", 0, false, "urem"},
		{"i32 / -1", "s[2] = bitcast<u32>(bitcast<i32>(s[0]) / -1);", 1, true, "sdiv"},
		// Float division shares sdiv's opcode in the emitter (BinOpFDiv ==
		// BinOpSDiv) and must get no guard: vec2 / scalar is the shape of
		// compute-store-particle-worst-case.
		{"vec2 f32 / run-time", "let v = vec2<f32>(bitcast<f32>(s[0]), bitcast<f32>(s[1])) / bitcast<f32>(s[2]);\n    s[4] = bitcast<u32>(v.x); s[5] = bitcast<u32>(v.y);", 0, false, "fdiv fast float"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := bitsBuffer + "@compute @workgroup_size(1) fn main() {\n    " + tt.body + "\n}\n"
			_, fn := emitMainModule(t, src)
			if got := countKind(fn, module.InstrSelect); got != tt.selects {
				t.Errorf("guard selects = %d, want %d", got, tt.selects)
			}
			compileWGSLToDXIL(t, src)

			text, ok := dumpbin(t, src)
			if !ok {
				return
			}
			if !regexp.MustCompile(` = ` + regexp.QuoteMeta(tt.op) + ` `).MatchString(text) {
				t.Errorf("no %s in the disassembly", tt.op)
			}
			if got := regexp.MustCompile(`icmp eq i32 %\d+, -2147483648`).MatchString(text); got != tt.minCmp {
				t.Errorf("compares the dividend with the most negative value: %v, want %v", got, tt.minCmp)
			}
			if tt.selects > 0 && !regexp.MustCompile(`select i1 %\d+, i32 1, i32 (%\d+|-?\d+)`).MatchString(text) {
				t.Errorf("no select of the divisor 1:\n%s", text)
			}
		})
	}
}
