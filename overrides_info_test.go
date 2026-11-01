package naga_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/gogpu/naga"
	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
)

// recognizable is a ModuleInfo no validation of the modules below makes.
var recognizable = valid.ModuleInfo{
	Functions:   []valid.FunctionInfo{{GlobalUses: []valid.GlobalUse{valid.GlobalUseAtomic}}},
	EntryPoints: []valid.FunctionInfo{{GlobalUses: []valid.GlobalUse{valid.GlobalUseWrite, valid.GlobalUseQuery}}},
}

// TestProcessOverridesNoOpKeepsInfo: with nothing to process (no overrides,
// no entry point selected) ProcessOverrides returns the module and the
// caller's info as they are, without validating (Rust pipeline_constants.rs:
// the Cow::Borrowed return) — a module the validator rejects comes back
// without an error.
func TestProcessOverridesNoOpKeepsInfo(t *testing.T) {
	// Two entry points with one name: invalid, and nothing to process.
	ep := ir.EntryPoint{Name: "main", Stage: ir.StageCompute, Workgroup: [3]uint32{1, 1, 1}}
	m := &ir.Module{EntryPoints: []ir.EntryPoint{ep, ep}}
	if _, err := valid.Validate(m, valid.Options{}); err == nil {
		t.Fatal("the module should be invalid")
	}
	out, info, err := naga.ProcessOverrides(m, recognizable, naga.OverrideOptions{})
	if err != nil {
		t.Fatalf("no-op: err = %v, want none (no validation)", err)
	}
	if out != m {
		t.Error("no-op: the module was not returned as it is")
	}
	if !reflect.DeepEqual(info, recognizable) {
		t.Errorf("no-op: info = %+v, want the caller's %+v", info, recognizable)
	}

	// An unknown key is an error even then, checked first (Rust :82-95).
	_, _, err = naga.ProcessOverrides(m, recognizable, naga.OverrideOptions{Constants: naga.PipelineConstants{"nope": 1}})
	if !errors.Is(err, naga.ErrOverrideNotFound) {
		t.Errorf("unknown key: err = %v, want ErrOverrideNotFound", err)
	}
}

// TestProcessOverridesFreshInfo: a processed module is validated again and
// its own ModuleInfo returned, the caller's dropped: for a module with an
// override, and for one entry point selected out of two.
func TestProcessOverridesFreshInfo(t *testing.T) {
	src := `
override n: u32 = 4u;
@group(0) @binding(0) var<storage, read_write> o: array<u32>;
@compute @workgroup_size(1) fn a() { o[0] = n; }
@compute @workgroup_size(1) fn b() { _ = arrayLength(&o); }
`
	noOverride := `
@group(0) @binding(0) var<storage, read_write> o: array<u32>;
@compute @workgroup_size(1) fn a() { o[0] = 1u; }
@compute @workgroup_size(1) fn b() { _ = arrayLength(&o); }
`
	for _, tc := range []struct {
		name string
		src  string
		ep   *backend.EntryPointSelector
	}{
		{"override", src, nil},
		{"entry point", noOverride, selectA},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := lowerSource(t, tc.src)
			out, info, err := naga.ProcessOverrides(m, recognizable, naga.OverrideOptions{EntryPoint: tc.ep})
			if err != nil {
				t.Fatal(err)
			}
			if out == m {
				t.Fatal("the module was returned as it is; it had something to process")
			}
			want, err := valid.Validate(out, valid.Options{Capabilities: valid.CapAll})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(info, want) || reflect.DeepEqual(info, recognizable) {
				t.Errorf("info = %+v, want the processed module's %+v", info, want)
			}
		})
	}
}
