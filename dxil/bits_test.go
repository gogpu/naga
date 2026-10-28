package dxil

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/internal/testtools"
)

// DXIL's FirstbitHi (33) and FirstbitSHi (34) count from the most significant
// bit and return -1 when no bit qualifies; FirstbitLo (32) counts from the
// least significant bit. DXC lowers HLSL firstbithigh(x) to
// `fb == -1 ? -1 : 31 - fb` and firstbitlow(x) to the bare op. So WGSL's
// countLeadingZeros(x) is `fb == -1 ? 32 : fb` on FirstbitHi (for i32 too:
// it counts the bits of the value as they are), and firstLeadingBit is
// DXC's firstbithigh, with FirstbitSHi for i32 (P0-DXIL-044). On vectors
// every component gets its own op (P0-DXIL-045).

// The cases read and write a buffer of scalars and build vectors from them:
// on main an arithmetic use of a component of a vec4 loaded from a storage
// buffer (b[0].x + 1u) gives bitcode dxc cannot read, a separate bug (fixed
// by #128's storage layout work) these tests must not depend on.
const bitsBuffer = "@group(0) @binding(0) var<storage, read_write> s: array<u32, 8>;\n"

// dumpbin compiles src and returns the text dxc -dumpbin prints for it, or
// false when dxc is missing (testtools policy). A blob dxc cannot read fails
// the test.
func dumpbin(t *testing.T, src string) (string, bool) {
	t.Helper()
	dxc, ok := testtools.Optional(t, "dxc", testtools.InstallDXC)
	if !ok {
		return "", false
	}
	blob, err := Compile(lowerSource(t, src), DefaultOptions())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	path := filepath.Join(t.TempDir(), "shader.dxil")
	if err := os.WriteFile(path, blob, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(dxc, "-dumpbin", path).CombinedOutput()
	if err != nil {
		t.Fatalf("dxc -dumpbin: %v\n%s", err, out)
	}
	return string(out), true
}

func TestBitScanOps(t *testing.T) {
	tests := []struct {
		name string
		body string
		// op is the dx.op unaryBits opcode and n how often it is called.
		op, n int
		// sub31 is whether `31 - fb` is computed; empty is the value of
		// the select for "no bit": 32 (counts) or -1 (bit positions).
		sub31 bool
		empty string
	}{
		{"countLeadingZeros u32", "s[4] = countLeadingZeros(s[0]);", 33, 1, false, "32"},
		{"countLeadingZeros i32", "s[4] = bitcast<u32>(countLeadingZeros(bitcast<i32>(s[0])));", 33, 1, false, "32"},
		{"countTrailingZeros u32", "s[4] = countTrailingZeros(s[0]);", 32, 1, false, "32"},
		{"firstLeadingBit u32", "s[4] = firstLeadingBit(s[0]);", 33, 1, true, "-1"},
		{"firstLeadingBit i32", "s[4] = bitcast<u32>(firstLeadingBit(bitcast<i32>(s[0])));", 34, 1, true, "-1"},
		{"countLeadingZeros vec4 u32", "let r = countLeadingZeros(vec4<u32>(s[0], s[1], s[2], s[3]));\n    s[4] = r.x; s[5] = r.y; s[6] = r.z; s[7] = r.w;", 33, 4, false, "32"},
		{"countTrailingZeros vec4 i32", "let r = countTrailingZeros(vec4<i32>(bitcast<i32>(s[0]), bitcast<i32>(s[1]), bitcast<i32>(s[2]), bitcast<i32>(s[3])));\n    s[4] = bitcast<u32>(r.x); s[5] = bitcast<u32>(r.y); s[6] = bitcast<u32>(r.z); s[7] = bitcast<u32>(r.w);", 32, 4, false, "32"},
		{"firstLeadingBit vec2 i32", "let r = firstLeadingBit(vec2<i32>(bitcast<i32>(s[0]), bitcast<i32>(s[1])));\n    s[4] = bitcast<u32>(r.x); s[5] = bitcast<u32>(r.y);", 34, 2, true, "-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := bitsBuffer + "@compute @workgroup_size(1) fn main() {\n    " + tt.body + "\n}\n"

			// Structure, without external tools: one op per component, and
			// bitcode the pure-Go reader accepts.
			_, fn := emitMainModule(t, src)
			if got := countCalls(fn, "dx.op.unaryBits.i32"); got != tt.n {
				t.Errorf("dx.op.unaryBits.i32 calls = %d, want %d (one per component)", got, tt.n)
			}
			if got := countKind(fn, module.InstrSelect); got != tt.n {
				t.Errorf("selects = %d, want %d (the no-bit case per component)", got, tt.n)
			}
			compileWGSLToDXIL(t, src)

			// Opcodes and constants, from dxc's disassembly.
			text, ok := dumpbin(t, src)
			if !ok {
				return
			}
			if got := len(regexp.MustCompile(`@dx\.op\.unaryBits\.i32\(i32 `+strconv.Itoa(tt.op)+`,`).FindAllString(text, -1)); got != tt.n {
				t.Errorf("unaryBits with opcode %d: %d calls, want %d", tt.op, got, tt.n)
			}
			for _, other := range []int{32, 33, 34} {
				if other != tt.op && regexp.MustCompile(`@dx\.op\.unaryBits\.i32\(i32 `+strconv.Itoa(other)+`,`).MatchString(text) {
					t.Errorf("unexpected unaryBits opcode %d", other)
				}
			}
			if got := regexp.MustCompile(`sub i32 31, %`).MatchString(text); got != tt.sub31 {
				t.Errorf("computes 31 - fb: %v, want %v", got, tt.sub31)
			}
			if !regexp.MustCompile(`select i1 %\d+, i32 ` + regexp.QuoteMeta(tt.empty) + `, i32 %`).MatchString(text) {
				t.Errorf("no select of %s for the no-bit case:\n%s", tt.empty, text)
			}
		})
	}
}
