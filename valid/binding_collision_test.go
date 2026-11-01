package valid_test

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/valid"
)

// Rust naga checks binding collisions per entry point, among the globals it
// uses (valid/interface.rs, EntryPointError::BindingCollision): two entry
// points may each put their own resource at one @group/@binding, as the
// corpus's image, shadow and debug-symbol shaders do. Ours rejected any two
// globals sharing a binding in the module (P1-IR-027).
func TestBindingCollisionPerEntryPoint(t *testing.T) {
	const twoEntryPoints = `
@group(0) @binding(0) var<storage, read_write> a: array<u32>;
@group(0) @binding(0) var<storage, read_write> b: array<f32>;

@compute @workgroup_size(1)
fn main_a() { a[0] = 1u; }

@compute @workgroup_size(1)
fn main_b() { b[0] = 1.0; }
`
	m, err := lowerWGSLNoFatal(twoEntryPoints)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := valid.Validate(m, valid.Options{}); err != nil {
		t.Errorf("two entry points with their own resource at one binding: %v", err)
	}

	// One entry point that uses both, one of them through a helper, is
	// Rust's BindingCollision.
	const oneEntryPoint = `
@group(0) @binding(0) var<storage, read_write> a: array<u32>;
@group(0) @binding(0) var<storage, read_write> b: array<f32>;

fn touch_b() { b[0] = 1.0; }

@compute @workgroup_size(1)
fn main() {
    a[0] = 1u;
    touch_b();
}
`
	m, err = lowerWGSLNoFatal(oneEntryPoint)
	if err != nil {
		t.Fatal(err)
	}
	_, err = valid.Validate(m, valid.Options{})
	const want = "Entry point main at Compute is invalid: Bindings for [1] conflict with other resource"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("one entry point using two resources at one binding: got %v, want an error containing %q", err, want)
	}
}
