// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/ir"
)

// compileOnMetal compiles code with the Metal compiler where there is one
// (macOS, as on CI): the slots above are what Metal itself checks.
func compileOnMetal(t *testing.T, code string) {
	t.Helper()
	if runtime.GOOS == "darwin" {
		verifyMSLWithXcrun(t, code)
	}
}

// TestNilMapSlotsImmediatesAndSizes: without a PerEntryPointMap every
// entry-point parameter that Metal binds gets a slot, numbered as Rust
// wgpu-hal's Metal device numbers a pipeline layout: the immediates buffer
// first, the resources by (group, binding), the sizes buffer after the
// last buffer. P0-MSL-096: the immediates and sizes parameters had no
// [[buffer(N)]] at all, which Metal rejects.
func TestNilMapSlotsImmediatesAndSizes(t *testing.T) {
	const src = `
struct Params { scale: f32, idx: u32 }
var<immediate> pc: Params;
@group(0) @binding(0) var<storage, read_write> data: array<f32>;
@group(0) @binding(1) var<uniform> factor: vec4<f32>;

@compute @workgroup_size(1)
fn main() {
    data[pc.idx] = pc.scale * factor.x * f32(arrayLength(&data));
}`
	got := compileWGSLWithOpts(t, src, DefaultOptions())
	for _, want := range []string{
		"pc [[buffer(0)]]",
		"data [[buffer(1)]]",
		"factor [[buffer(2)]]",
		"_buffer_sizes [[buffer(3)]]",
	} {
		mustContainMSL(t, got, want)
	}
	compileOnMetal(t, got)
}

// TestNilMapSlotsWithoutImmediates: a module without immediate data keeps
// slot 0 for its first buffer, as wgpu-hal places no immediates buffer for
// a layout without immediates.
func TestNilMapSlotsWithoutImmediates(t *testing.T) {
	const src = `
@group(0) @binding(0) var<storage, read_write> data: array<f32>;

@compute @workgroup_size(1)
fn main() {
    data[0] = f32(arrayLength(&data));
}`
	got := compileWGSLWithOpts(t, src, DefaultOptions())
	mustContainMSL(t, got, "data [[buffer(0)]]")
	mustContainMSL(t, got, "_buffer_sizes [[buffer(1)]]")
	compileOnMetal(t, got)
}

// TestNilMapSlotsExternalTexture: an external texture takes three texture
// slots for its planes and a buffer slot for its parameters, as in
// wgpu-hal; the resources after it continue from there.
func TestNilMapSlotsExternalTexture(t *testing.T) {
	const src = `
@group(0) @binding(0) var ext: texture_external;
@group(0) @binding(1) var samp: sampler;
@group(0) @binding(2) var tex: texture_2d<f32>;
@group(0) @binding(3) var<uniform> u: vec4<f32>;

@fragment
fn main() -> @location(0) vec4<f32> {
    return textureSampleBaseClampToEdge(ext, samp, vec2(0.5)) + textureSampleLevel(tex, samp, vec2(0.5), 0.0) + u;
}`
	got := compileWGSLWithOpts(t, src, DefaultOptions())
	for _, want := range []string{
		"[[texture(0)]]",
		"[[texture(1)]]",
		"[[texture(2)]]",
		"samp [[sampler(0)]]",
		"tex [[texture(3)]]",
		"u [[buffer(1)]]",
	} {
		mustContainMSL(t, got, want)
	}
	if !strings.Contains(got, "& ext_params [[buffer(0)]]") {
		t.Errorf("the external texture's params buffer is not at slot 0:\n%s", got)
	}
	compileOnMetal(t, got)
}

// TestNilMapSlotsFragDepthClamp: the frag_depth clamp reads its range from
// the immediates buffer, so the automatic numbering places that buffer for
// a module without immediate data too.
func TestNilMapSlotsFragDepthClamp(t *testing.T) {
	const src = `
@group(0) @binding(0) var<uniform> u: vec4<f32>;
@fragment fn fs_main(@builtin(position) p: vec4<f32>) -> @builtin(frag_depth) f32 { return p.z * u.x; }`
	opts := clampOptions(8)
	opts.PerEntryPointMap = nil
	got := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, got, "constant char* _immediates [[buffer(0)]]")
	mustContainMSL(t, got, "u [[buffer(1)]]")
	compileOnMetal(t, got)
}

// TestSlotCounterOverflow: slots are uint8, so a 257th slot of one kind is
// an error instead of a wrap onto slot 0.
func TestSlotCounterOverflow(t *testing.T) {
	c := slotCounter{what: "buffer"}
	if _, err := c.take(255); err != nil {
		t.Fatalf("255 slots: %v", err)
	}
	slot, err := c.take(1)
	if err != nil || slot != 255 {
		t.Fatalf("slot 255: slot %d, err %v", slot, err)
	}
	if _, err := c.take(1); err == nil || !strings.Contains(err.Error(), "more than 256 buffer slots") {
		t.Errorf("slot 256: err = %v, want an overflow error", err)
	}
	ext := slotCounter{what: "texture", next: 254}
	if _, err := takeExternalTextureSlots(&ext, &slotCounter{what: "buffer"}); err == nil {
		t.Errorf("three planes from slot 254: no error")
	}
}

// TestMissingBindingErrorsPrefixedOnce: an error raised while writing the
// module carries the "msl:" prefix once; Compile adds it at its boundary,
// so the errors inside the writer do not repeat it.
func TestMissingBindingErrorsPrefixedOnce(t *testing.T) {
	module := processOverridesForTest(t, lowerWGSLForTest(t, `
struct Params { scale: f32 }
var<immediate> pc: Params;
@group(0) @binding(0) var<storage, read_write> data: array<f32, 4>;
@compute @workgroup_size(1) fn main() { data[0] = pc.scale; }
`), nil)
	slot := func(n uint8) *uint8 { return &n }
	opts := DefaultOptions()
	opts.PerEntryPointMap = map[string]EntryPointResources{"main": {
		Resources: map[ir.ResourceBinding]BindTarget{{Group: 0, Binding: 0}: {Buffer: slot(0)}},
	}}
	_, _, err := Compile(module, opts)
	if !errors.Is(err, backend.ErrMissingBinding) {
		t.Fatalf("err = %v, want backend.ErrMissingBinding", err)
	}
	if !strings.Contains(err.Error(), "no immediates buffer") {
		t.Errorf("err = %v, want the missing immediates buffer named", err)
	}
	if n := strings.Count(err.Error(), "msl:"); n != 1 {
		t.Errorf("err = %q has %d \"msl:\" prefixes, want 1", err, n)
	}

	opts.FakeMissingBindings = true
	code, _, err := Compile(module, opts)
	if err != nil {
		t.Fatalf("FakeMissingBindings: %v", err)
	}
	mustContainMSL(t, code, "pc [[user(fake0)]]")
}

// TestUnresolvedOverrideErrorPrefixedOnce: the same for an override
// expression that reaches the writer without naga.ProcessOverrides.
func TestUnresolvedOverrideErrorPrefixedOnce(t *testing.T) {
	module := lowerWGSLForTest(t, `
override k: f32 = 2.0;
@group(0) @binding(0) var<storage, read_write> data: array<f32, 4>;
@compute @workgroup_size(1) fn main() { data[0] = k; }
`)
	_, _, err := Compile(module, DefaultOptions())
	if !errors.Is(err, ir.ErrUnresolvedOverride) {
		t.Fatalf("err = %v, want ir.ErrUnresolvedOverride", err)
	}
	if n := strings.Count(err.Error(), "msl:"); n != 1 {
		t.Errorf("err = %q has %d \"msl:\" prefixes, want 1", err, n)
	}
}
