package valid_test

import (
	"errors"
	"testing"

	"github.com/gogpu/naga/valid"
)

// TestModuleInfoGlobalUses: ModuleInfo records per function and entry point
// how each global is used, callees included, with Rust naga's flags
// (valid/analyzer.rs add_ref_impl): a load reads, a store writes, an
// atomic reads and writes, arrayLength and textureDimensions query,
// sampling reads the texture and the sampler, textureStore writes, a
// phony assignment queries, and an unused global has no flags.
func TestModuleInfoGlobalUses(t *testing.T) {
	const src = `
@group(0) @binding(0) var<storage, read> ro: array<u32>;
@group(0) @binding(1) var<storage, read_write> rw: array<u32>;
@group(0) @binding(2) var<storage, read_write> counter: atomic<u32>;
@group(0) @binding(3) var tex: texture_2d<f32>;
@group(0) @binding(4) var samp: sampler;
@group(0) @binding(5) var out_tex: texture_storage_2d<rgba8unorm, write>;
@group(0) @binding(6) var<uniform> named: vec4<f32>;
@group(0) @binding(7) var<uniform> unused: vec4<f32>;
@group(0) @binding(8) var sized: texture_2d<f32>;

fn write_rw(v: u32) { rw[0] = v; }

@compute @workgroup_size(1) fn cs() {
    let n = arrayLength(&ro);
    write_rw(ro[0] + n);
    atomicAdd(&counter, 1u);
    _ = &named;
    let d = textureDimensions(sized);
    textureStore(out_tex, vec2<i32>(0), vec4<f32>(f32(d.x)));
}

@fragment fn fs() -> @location(0) vec4<f32> {
    return textureSample(tex, samp, vec2<f32>(0.5));
}`
	m, err := lowerWGSLNoFatal(src)
	if err != nil {
		t.Fatal(err)
	}
	info, err := valid.Validate(m, valid.Options{})
	if err != nil {
		t.Fatal(err)
	}
	global := func(name string) int {
		for i, g := range m.GlobalVariables {
			if g.Name == name {
				return i
			}
		}
		t.Fatalf("no global %q", name)
		return 0
	}
	const (
		r = valid.GlobalUseRead
		w = valid.GlobalUseWrite
		q = valid.GlobalUseQuery
	)
	cs := map[string]valid.GlobalUse{
		"ro": r | q, "rw": w, "counter": r | w, "named": q, "sized": q, "out_tex": w,
		"tex": 0, "samp": 0, "unused": 0,
	}
	fs := map[string]valid.GlobalUse{"tex": r, "samp": r, "ro": 0, "rw": 0, "unused": 0}
	for i, want := range []map[string]valid.GlobalUse{cs, fs} {
		ep := m.EntryPoints[i].Name
		for name, use := range want {
			if got := info.EntryPoints[i].GlobalUses[global(name)]; got != use {
				t.Errorf("%s: %s uses %04b, want %04b", ep, name, got, use)
			}
		}
	}
	if len(info.Functions) != 1 || info.Functions[0].GlobalUses[global("rw")] != w {
		t.Errorf("write_rw: %+v, want rw written", info.Functions)
	}
}

// TestValidateErrors: an invalid module returns every problem as a
// Errors, and errors.As finds each *Error; the zero
// Options allow no optional capability.
func TestValidateErrors(t *testing.T) {
	m, err := lowerWGSLNoFatal(`var<private> a: f64; var<private> b: i64;
@compute @workgroup_size(1) fn main() { a = 1.0lf; b = 1li; }`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = valid.Validate(m, valid.Options{})
	var all valid.Errors
	if !errors.As(err, &all) || len(all) < 2 {
		t.Fatalf("err = %v, want an Errors with f64 and i64", err)
	}
	var one *valid.Error
	if !errors.As(err, &one) {
		t.Errorf("errors.As(*Error) found nothing in %v", err)
	}
	if _, err := valid.Validate(m, valid.Options{Capabilities: valid.CapFloat64 | valid.CapShaderInt64}); err != nil {
		t.Errorf("with the capabilities: %v", err)
	}
}
