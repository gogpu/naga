package codegen

import "testing"

// BUG-MSL-WGULOAD-ATOMIC-001: workgroupUniformLoad(&atomic) copied the atomic
// object (`uint unnamed = wg;`), which is ill-formed MSL: an atomic object has
// no copy constructor. Rust naga emits a load through the atomic instead.
//
// Reference: back/msl/writer.rs Statement::WorkGroupUniformLoad calls
// put_load, and put_unchecked_load sees an atomic pointer and writes
// metal::atomic_load_explicit(&ptr, metal::memory_order_relaxed). Expected
// line below is Rust naga's output for the same source.
func TestWorkgroupUniformLoadAtomicUsesAtomicLoad(t *testing.T) {
	src := `var<workgroup> wg: atomic<u32>;

@compute @workgroup_size(1)
fn main() {
    let v = workgroupUniformLoad(&wg);
    if v == 0u {
        return;
    }
}
`
	code := compileWGSL(t, src)

	want := "uint unnamed = metal::atomic_load_explicit(&wg, metal::memory_order_relaxed);"
	if !containsLine(code, want) {
		t.Errorf("missing line (Rust naga output):\n  %s\ngot:\n%s", want, linesContaining(code, "unnamed"))
	}
	if bad := linesContaining(code, "= wg;"); bad != "" {
		t.Errorf("workgroupUniformLoad copies the atomic object:\n%s", bad)
	}
}
