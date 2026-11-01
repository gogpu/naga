package lower

import (
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
)

// TestAtomicStoreValueEmittedBeforeStore: atomicStore of a computed value
// emits the value before the Store, as Rust's lowerer does (it finishes the
// emitter before pushing the Store). The Store came first, so the value was
// out of scope: Rust's validator rejects that (NotInScope), and a backend
// stored an expression it had not yet evaluated.
func TestAtomicStoreValueEmittedBeforeStore(t *testing.T) {
	src := `
var<workgroup> a: atomic<u32>;
@compute @workgroup_size(1) fn main(@builtin(local_invocation_index) i: u32) {
    let h = f32(i) * 2.0;
    atomicStore(&a, u32(h));
}`
	m, err := compileWGSL(t, src)
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}
	if _, err := valid.Validate(m, valid.Options{}); err != nil {
		t.Fatalf("validation: %v", err)
	}
	fn := &m.EntryPoints[0].Function
	for i, st := range fn.Body {
		store, ok := st.Kind.(ir.StmtStore)
		if !ok {
			continue
		}
		if _, ok := fn.Expressions[store.Value].Kind.(ir.ExprAs); !ok {
			t.Fatalf("stored %T, want the u32 conversion", fn.Expressions[store.Value].Kind)
		}
		emitted := false
		for _, before := range fn.Body[:i] {
			if e, ok := before.Kind.(ir.StmtEmit); ok && e.Range.Start <= store.Value && store.Value < e.Range.End {
				emitted = true
			}
		}
		if !emitted {
			t.Errorf("value [%d] is not emitted before the Store", store.Value)
		}
		return
	}
	t.Fatal("no Store")
}
