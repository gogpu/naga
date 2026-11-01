package lower

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestVarAddressSpaceWords pins the WGSL address space words to Rust naga's
// front/wgsl/parse/conv.rs map_address_space: `immediate` is the only word
// for immediate data (gpuweb proposals/immediate-data.md), and an unknown word
// is an error ("unknown address space"), never a silent function-space
// variable. `push_constant` was renamed to `immediate`; Rust reports it as an
// unknown address space, and we add the rename to the message.
func TestVarAddressSpaceWords(t *testing.T) {
	const body = `
@compute @workgroup_size(1)
fn main() {
    _ = pc.x;
}`
	t.Run("immediate", func(t *testing.T) {
		module, err := compileWGSL(t, "struct P { x: u32 }\nvar<immediate> pc: P;"+body)
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		if got := module.GlobalVariables[0].Space; got != ir.SpaceImmediate {
			t.Errorf("space = %v, want SpaceImmediate", got)
		}
	})
	for _, tc := range []struct {
		word string
		want []string
	}{
		{"push_constant", []string{"unknown address space: `push_constant`", "renamed to `immediate`"}},
		{"handle", []string{"unknown address space: `handle`"}},
		{"bogus", []string{"unknown address space: `bogus`"}},
	} {
		t.Run(tc.word, func(t *testing.T) {
			_, err := compileWGSL(t, "struct P { x: u32 }\nvar<"+tc.word+"> pc: P;"+body)
			if err == nil {
				t.Fatalf("var<%s> compiled, want an unknown address space error", tc.word)
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
		})
	}
	t.Run("pointer", func(t *testing.T) {
		src := "fn f(p: ptr<bogus, u32>) {}\n@compute @workgroup_size(1)\nfn main() {}"
		if _, err := compileWGSL(t, src); err == nil || !strings.Contains(err.Error(), "unknown address space: `bogus`") {
			t.Errorf("ptr<bogus, u32>: err = %v, want unknown address space", err)
		}
	})
}
