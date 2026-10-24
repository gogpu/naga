package lower

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// A var declared without an initializer inside a loop body is a new zero
// value on every iteration, while backends zero a local only once, at
// function entry. The lowerer stores a ZeroValue at such a declaration, as
// Rust naga does (#9489); outside a loop the backends' zeroing is enough.
func TestLoopVarWithoutInitializerStoresZero(t *testing.T) {
	m := helperCompileWGSL(t, `
@compute @workgroup_size(1) fn main() {
    var outside: u32;
    for (var i = 0u; i < 4u; i++) {
        var inside: u32;
        inside += 1u;
        outside += inside;
    }
}
`)
	fn := &m.EntryPoints[0].Function
	zeroStores := map[string]int{}
	var walk func(ir.Block)
	walk = func(b ir.Block) {
		for _, st := range b {
			switch s := st.Kind.(type) {
			case ir.StmtStore:
				lv, isLocal := fn.Expressions[s.Pointer].Kind.(ir.ExprLocalVariable)
				if _, isZero := fn.Expressions[s.Value].Kind.(ir.ExprZeroValue); isLocal && isZero {
					zeroStores[fn.LocalVars[lv.Variable].Name]++
				}
			case ir.StmtBlock:
				walk(s.Block)
			case ir.StmtIf:
				walk(s.Accept)
				walk(s.Reject)
			case ir.StmtLoop:
				walk(s.Body)
				walk(s.Continuing)
			}
		}
	}
	walk(fn.Body)
	if zeroStores["inside"] != 1 {
		t.Errorf("zero stores to inside = %d, want 1", zeroStores["inside"])
	}
	if zeroStores["outside"] != 0 {
		t.Errorf("zero stores to outside = %d, want 0", zeroStores["outside"])
	}
}
