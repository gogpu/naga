package codegen

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// A resource an entry point uses that its PerEntryPointMap entry lacks, or
// holds as another kind, is an error, as in Rust naga, and so is a missing
// sizes buffer. The resource was written at its WGSL binding number, which
// nothing binds, and the sizes argument without a slot.
func TestPerEntryPointMapMissingBindTarget(t *testing.T) {
	module := processOverridesForTest(t, lowerWGSLForTest(t, `
@group(0) @binding(0) var<storage, read_write> out: array<u32>;
@group(0) @binding(1) var tex: texture_2d<f32>;
@compute @workgroup_size(1) fn main() { out[0] = arrayLength(&out) + textureDimensions(tex).x; }
`), nil)
	slot := func(n uint8) *uint8 { return &n }
	full := map[ir.ResourceBinding]BindTarget{
		{Group: 0, Binding: 0}: {Buffer: slot(0)},
		{Group: 0, Binding: 1}: {Texture: slot(0)},
	}
	compile := func(name string, resources map[ir.ResourceBinding]BindTarget, sizes *uint8, fake bool) (string, error) {
		opts := DefaultOptions()
		opts.PerEntryPointMap = map[string]EntryPointResources{name: {Resources: resources, SizesBuffer: sizes}}
		opts.FakeMissingBindings = fake
		code, _, err := Compile(module, opts)
		return code, err
	}

	for name, resources := range map[string]map[ir.ResourceBinding]BindTarget{
		"missing":      {{Group: 0, Binding: 0}: {Buffer: slot(0)}},
		"another kind": {{Group: 0, Binding: 0}: {Buffer: slot(0)}, {Group: 0, Binding: 1}: {Buffer: slot(1)}},
	} {
		if _, err := compile("main", resources, slot(1), false); err == nil || !strings.Contains(err.Error(), "no bind target for tex") {
			t.Errorf("%s: err = %v, want no bind target for tex", name, err)
		}
	}
	if _, err := compile("main", full, nil, false); err == nil || !strings.Contains(err.Error(), "no sizes buffer") {
		t.Errorf("no sizes buffer: err = %v", err)
	}

	code, err := compile("main", full, slot(1), false)
	if err != nil {
		t.Fatalf("full map: %v", err)
	}
	mustContainMSL(t, code, "[[texture(0)]]")
	mustContainMSL(t, code, "_buffer_sizes [[buffer(1)]]")
	if _, err := compile("main", nil, nil, true); err != nil {
		t.Errorf("FakeMissingBindings: %v", err)
	}
	if _, err := compile("other", nil, nil, false); err != nil {
		t.Errorf("an entry point the map has no entry for: %v", err)
	}
}
