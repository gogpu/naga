package front

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
	"github.com/gogpu/naga/valid"
)

// The row-major cases share a struct in the Function space whose member is a
// row-major mat2x2, or an array of them. Ids: 50 the struct, 51 its pointer,
// 52 the array of matrices, 53 the array's struct, 54 its pointer, 55 a
// pointer to the array, 60 an i32 zero for the struct member index; 10 is the
// mat2x2 and 16 the u32 length the preamble already declares.
func addRowMajorMatrixMember(m *bodyModule) *bodyModule {
	m.annotate(
		instWords(spvenums.OpMemberDecorate, 50, 0, uint32(spvenums.DecorationRowMajor)),
	)
	return m.add(
		instWords(spvenums.OpTypeStruct, 50, 10),
		instWords(spvenums.OpTypePointer, 51, uint32(spvenums.StorageClassFunction), 50),
		instWords(spvenums.OpConstant, 3, 60, 0),
	)
}

// addRowMajorMatrixArrayMember declares a struct whose member is an array of
// row-major matrices.
func addRowMajorMatrixArrayMember(m *bodyModule) *bodyModule {
	m.annotate(
		instWords(spvenums.OpMemberDecorate, 53, 0, uint32(spvenums.DecorationRowMajor)),
	)
	return m.add(
		instWords(spvenums.OpTypeArray, 52, 10, 16),
		instWords(spvenums.OpTypeStruct, 53, 52),
		instWords(spvenums.OpTypePointer, 54, uint32(spvenums.StorageClassFunction), 53),
		instWords(spvenums.OpTypePointer, 55, uint32(spvenums.StorageClassFunction), 52),
		instWords(spvenums.OpConstant, 3, 60, 0),
	)
}

// countExpressions counts the expressions the predicate accepts.
func countMatchingExpressions(fn *ir.Function, accept func(ir.ExpressionKind) bool) int {
	count := 0
	for _, expression := range fn.Expressions {
		if accept(expression.Kind) {
			count++
		}
	}
	return count
}

// expressionHandle returns the handle of the first expression the predicate
// accepts.
func expressionHandle(fn *ir.Function, accept func(ir.ExpressionKind) bool) (ir.ExpressionHandle, bool) {
	for i, expression := range fn.Expressions {
		if accept(expression.Kind) {
			return ir.ExpressionHandle(i), true
		}
	}
	return 0, false
}

// isTranspose reports whether an expression is a matrix transpose.
func isTranspose(kind ir.ExpressionKind) bool {
	math, ok := kind.(ir.ExprMath)
	return ok && math.Fun == ir.MathTranspose
}

// isLoad reports whether an expression is a load.
func isLoad(kind ir.ExpressionKind) bool {
	_, ok := kind.(ir.ExprLoad)
	return ok
}

// TestRowMajorMatrixLoad checks the transposed load a row-major matrix member
// produces (Rust naga, front/spv/next_block.rs:321-347): the OpLoad of the
// member pointer must not read the column-major layout.
func TestRowMajorMatrixLoad(t *testing.T) {
	m := addRowMajorMatrixMember(newBodyModule())
	fn := m.parse(t,
		instWords(spvenums.OpVariable, 51, 200, uint32(spvenums.StorageClassFunction)),
		instWords(spvenums.OpAccessChain, 31, 201, 200, 60),
		instWords(spvenums.OpLoad, 10, 202, 201),
	)
	if transposes := countMatchingExpressions(fn, isTranspose); transposes != 1 {
		t.Fatalf("transposes = %d, want exactly one for the row-major member: %#v", transposes, fn.Expressions)
	}
	if loads := countMatchingExpressions(fn, isLoad); loads != 1 {
		t.Errorf("loads = %d, want 1: the OpLoad reads the side-loaded transposed value", loads)
	}
}

// TestRowMajorMatrixColumn checks that a column read from a row-major matrix
// comes from the transposed value, not from the pointer
// (front/spv/next_block.rs:353-387).
func TestRowMajorMatrixColumn(t *testing.T) {
	m := addRowMajorMatrixMember(newBodyModule())
	fn := m.parse(t,
		instWords(spvenums.OpVariable, 51, 200, uint32(spvenums.StorageClassFunction)),
		instWords(spvenums.OpAccessChain, 33, 201, 200, 60, 26),
		instWords(spvenums.OpLoad, 7, 202, 201),
	)
	transposed, ok := expressionHandle(fn, isTranspose)
	if !ok {
		t.Fatalf("want a Transpose in the expressions: %#v", fn.Expressions)
	}
	if _, ok := findExpression(fn, func(kind ir.ExpressionKind) bool {
		access, ok := kind.(ir.ExprAccessIndex)
		return ok && access.Base == transposed
	}); !ok {
		t.Errorf("want the column indexed from the transposed value: %#v", fn.Expressions)
	}
	if loads := countMatchingExpressions(fn, isLoad); loads != 1 {
		t.Errorf("loads = %d, want 1", loads)
	}
}

// TestRowMajorMatrixArrayElement checks the pending override: indexing an
// array of row-major matrices loads and transposes the selected element
// (front/spv/next_block.rs:401-434).
func TestRowMajorMatrixArrayElement(t *testing.T) {
	m := addRowMajorMatrixArrayMember(newBodyModule())
	fn := m.parse(t,
		instWords(spvenums.OpVariable, 54, 200, uint32(spvenums.StorageClassFunction)),
		instWords(spvenums.OpAccessChain, 31, 201, 200, 60, 26),
		instWords(spvenums.OpLoad, 10, 202, 201),
	)
	if transposes := countMatchingExpressions(fn, isTranspose); transposes != 1 {
		t.Fatalf("transposes = %d, want exactly one for the selected element: %#v", transposes, fn.Expressions)
	}
	if loads := countMatchingExpressions(fn, isLoad); loads != 1 {
		t.Errorf("loads = %d, want 1: the element load, not also the chain's OpLoad", loads)
	}
}

// TestRowMajorMatrixArrayLoad pins the known limit shared with Rust: loading
// the whole array of row-major matrices leaves the pending override unused and
// reads a plain load (front/spv/next_block.rs:752, "we aren't handling
// LookupLoadOverride::Pending properly here").
func TestRowMajorMatrixArrayLoad(t *testing.T) {
	m := addRowMajorMatrixArrayMember(newBodyModule())
	fn := m.parse(t,
		instWords(spvenums.OpVariable, 54, 200, uint32(spvenums.StorageClassFunction)),
		instWords(spvenums.OpAccessChain, 55, 201, 200, 60),
		instWords(spvenums.OpLoad, 52, 202, 201),
	)
	if transposes := countMatchingExpressions(fn, isTranspose); transposes != 0 {
		t.Errorf("transposes = %d, want 0: Rust loads the array as it is", transposes)
	}
	if loads := countMatchingExpressions(fn, isLoad); loads != 1 {
		t.Errorf("loads = %d, want 1", loads)
	}
}

// storeStatements returns every store in the function body.
func storeStatements(fn *ir.Function) []ir.StmtStore {
	var stores []ir.StmtStore
	for _, statement := range fn.Body {
		if store, ok := statement.Kind.(ir.StmtStore); ok {
			stores = append(stores, store)
		}
	}
	return stores
}

// accessParts splits an access expression into its base and the index it
// applies: a constant, or the handle of a computed index.
func accessParts(fn *ir.Function, handle ir.ExpressionHandle) (base ir.ExpressionHandle, index uint32, computed ir.ExpressionHandle, ok bool) {
	if int(handle) >= len(fn.Expressions) {
		return 0, 0, 0, false
	}
	switch access := fn.Expressions[handle].Kind.(type) {
	case ir.ExprAccessIndex:
		return access.Base, access.Index, 0, true
	case ir.ExprAccess:
		return access.Base, 0, access.Index, true
	}
	return 0, 0, 0, false
}

// expressionStatement returns the index of the Emit statement that evaluates an
// expression, or -1.
func expressionStatement(fn *ir.Function, handle ir.ExpressionHandle) int {
	for i, statement := range fn.Body {
		emit, ok := statement.Kind.(ir.StmtEmit)
		if ok && emit.Range.Start <= handle && handle < emit.Range.End {
			return i
		}
	}
	return -1
}

// parseValidated parses a module and validates its IR, which every translated
// body must pass.
func (m *bodyModule) parseValidated(t *testing.T, body ...[]uint32) *ir.Function {
	t.Helper()
	module, _, err := Parse(m.build(body...), Options{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, err := valid.Validate(module, valid.Options{Capabilities: valid.CapAll}); err != nil {
		t.Fatalf("valid.Validate: %v", err)
	}
	if len(module.Functions) != 1 {
		t.Fatalf("functions = %d, want 1", len(module.Functions))
	}
	return &module.Functions[0]
}

// TestRowMajorMatrixStore: the memory of a row-major matrix holds the transpose
// of the value the shader sees (the load path side-loads that transpose), so a
// whole-matrix store writes Transpose(value) at the member pointer
// (P3-SPVIN-007; the SPIR-V spec, RowMajor).
func TestRowMajorMatrixStore(t *testing.T) {
	m := addRowMajorMatrixMember(newBodyModule())
	fn := m.parseValidated(t,
		instWords(spvenums.OpVariable, 51, 200, uint32(spvenums.StorageClassFunction)),
		instWords(spvenums.OpAccessChain, 31, 201, 200, 60),
		instWords(spvenums.OpCompositeConstruct, 7, 210, 22, 22),
		instWords(spvenums.OpCompositeConstruct, 10, 211, 210, 210),
		instWords(spvenums.OpStore, 201, 211),
	)
	stores := storeStatements(fn)
	if len(stores) != 1 {
		t.Fatalf("stores = %d, want 1: %#v", len(stores), fn.Body)
	}
	math, ok := fn.Expressions[stores[0].Value].Kind.(ir.ExprMath)
	if !ok || math.Fun != ir.MathTranspose {
		t.Fatalf("store value = %#v, want a Transpose of the matrix", fn.Expressions[stores[0].Value].Kind)
	}
	if _, ok := fn.Expressions[math.Arg].Kind.(ir.ExprCompose); !ok {
		t.Errorf("transpose operand = %#v, want the composed matrix", fn.Expressions[math.Arg].Kind)
	}
	if _, index, _, ok := accessParts(fn, stores[0].Pointer); !ok || index != 0 {
		t.Errorf("store pointer = %#v, want the member access (index 0)", fn.Expressions[stores[0].Pointer].Kind)
	}
}

// TestRowMajorMatrixColumnStore: a column of a row-major matrix is strided in
// memory, and IR element [r][c] is the SPIR-V element [c][r], so the vector
// store becomes one store per component through the reversed access indices.
func TestRowMajorMatrixColumnStore(t *testing.T) {
	m := addRowMajorMatrixMember(newBodyModule())
	fn := m.parseValidated(t,
		instWords(spvenums.OpVariable, 51, 200, uint32(spvenums.StorageClassFunction)),
		instWords(spvenums.OpAccessChain, 33, 201, 200, 60, 26), // member 0, column 1
		instWords(spvenums.OpCompositeConstruct, 7, 210, 22, 22),
		instWords(spvenums.OpStore, 201, 210),
	)
	stores := storeStatements(fn)
	if len(stores) != 2 {
		t.Fatalf("stores = %d, want 2 (one per row): %#v", len(stores), fn.Body)
	}
	for r, store := range stores {
		componentPointer, columnIndex, _, ok := accessParts(fn, store.Pointer)
		if !ok || columnIndex != 1 {
			t.Fatalf("store %d pointer = %#v, want the column index 1 outermost", r, fn.Expressions[store.Pointer].Kind)
		}
		_, rowIndex, _, ok := accessParts(fn, componentPointer)
		if !ok || rowIndex != uint32(r) {
			t.Errorf("store %d row = %d, want %d: the SPIR-V column and row indices are reversed", r, rowIndex, r)
		}
		_, componentIndex, _, ok := accessParts(fn, store.Value)
		if !ok || componentIndex != uint32(r) {
			t.Errorf("store %d value = %#v, want component %d of the column", r, fn.Expressions[store.Value].Kind, r)
		}
	}
}

// TestRowMajorMatrixElementStore: the scalar V[c][r] of a row-major matrix is IR
// element [r][c], so the store pointer swaps the two access indices.
func TestRowMajorMatrixElementStore(t *testing.T) {
	m := addRowMajorMatrixMember(newBodyModule())
	fn := m.parseValidated(t,
		instWords(spvenums.OpVariable, 51, 200, uint32(spvenums.StorageClassFunction)),
		instWords(spvenums.OpAccessChain, 12, 201, 200, 60, 26, 60), // member 0, column 1, component 0
		instWords(spvenums.OpStore, 201, 22),
	)
	stores := storeStatements(fn)
	if len(stores) != 1 {
		t.Fatalf("stores = %d, want 1: %#v", len(stores), fn.Body)
	}
	componentPointer, columnIndex, _, ok := accessParts(fn, stores[0].Pointer)
	if !ok || columnIndex != 1 {
		t.Fatalf("store pointer = %#v, want the column index 1 outermost", fn.Expressions[stores[0].Pointer].Kind)
	}
	if _, rowIndex, _, ok := accessParts(fn, componentPointer); !ok || rowIndex != 0 {
		t.Errorf("column index = %d, want the SPIR-V row index 0: the access indices are reversed", rowIndex)
	}
	if _, ok := fn.Expressions[stores[0].Value].Kind.(ir.ExprConstant); !ok {
		t.Errorf("store value = %#v, want the constant", fn.Expressions[stores[0].Value].Kind)
	}
}

// TestRowMajorMatrixStoreReadBack: a load through the stored pointer reads the
// memory the store just wrote. The transposed side-load the access chain made
// before the store is stale, so the load must read the pointer again
// (P3-SPVIN-007).
func TestRowMajorMatrixStoreReadBack(t *testing.T) {
	m := addRowMajorMatrixMember(newBodyModule())
	fn := m.parseValidated(t,
		instWords(spvenums.OpVariable, 51, 200, uint32(spvenums.StorageClassFunction)),
		instWords(spvenums.OpAccessChain, 31, 201, 200, 60),
		instWords(spvenums.OpCompositeConstruct, 7, 210, 22, 22),
		instWords(spvenums.OpCompositeConstruct, 10, 211, 210, 210),
		instWords(spvenums.OpStore, 201, 211),
		instWords(spvenums.OpLoad, 10, 202, 201),
	)
	first, ok := expressionHandle(fn, isTranspose)
	if !ok {
		t.Fatalf("no Transpose in the expressions: %#v", fn.Expressions)
	}
	second := ir.ExpressionHandle(0)
	found := false
	for i := first + 1; i < ir.ExpressionHandle(len(fn.Expressions)); i++ {
		if isTranspose(fn.Expressions[i].Kind) {
			second, found = i, true
		}
	}
	if !found {
		t.Fatalf("only one Transpose: the load reused the stale side-load: %#v", fn.Body)
	}
	storeIndex := -1
	for i, statement := range fn.Body {
		if _, ok := statement.Kind.(ir.StmtStore); ok {
			storeIndex = i
		}
	}
	if storeIndex < 0 {
		t.Fatal("no store statement")
	}
	if expressionStatement(fn, first) >= storeIndex {
		t.Errorf("the first side-load is evaluated after the store")
	}
	if expressionStatement(fn, second) <= storeIndex {
		t.Errorf("the load reads the memory before the store: Emit at %d, store at %d", expressionStatement(fn, second), storeIndex)
	}
	firstLoad := fn.Expressions[first].Kind.(ir.ExprMath).Arg
	secondLoad := fn.Expressions[second].Kind.(ir.ExprMath).Arg
	if firstLoad == secondLoad {
		t.Fatalf("both Transposes read the same expression: the memory is not re-read")
	}
	if firstPointer := fn.Expressions[firstLoad].Kind.(ir.ExprLoad).Pointer; firstPointer != fn.Expressions[secondLoad].Kind.(ir.ExprLoad).Pointer {
		t.Errorf("the re-read loads a different pointer than the first side-load")
	}
}

// TestRowMajorMatrixStoreThroughCopy: an OpCopyObject of the member pointer is
// the same pointer, so the copy keeps the row-major translation and a store
// through it writes Transpose(value) too.
func TestRowMajorMatrixStoreThroughCopy(t *testing.T) {
	m := addRowMajorMatrixMember(newBodyModule())
	fn := m.parseValidated(t,
		instWords(spvenums.OpVariable, 51, 200, uint32(spvenums.StorageClassFunction)),
		instWords(spvenums.OpAccessChain, 31, 201, 200, 60),
		instWords(spvenums.OpCopyObject, 31, 202, 201),
		instWords(spvenums.OpCompositeConstruct, 7, 210, 22, 22),
		instWords(spvenums.OpCompositeConstruct, 10, 211, 210, 210),
		instWords(spvenums.OpStore, 202, 211),
	)
	stores := storeStatements(fn)
	if len(stores) != 1 {
		t.Fatalf("stores = %d, want 1: %#v", len(stores), fn.Body)
	}
	if math, ok := fn.Expressions[stores[0].Value].Kind.(ir.ExprMath); !ok || math.Fun != ir.MathTranspose {
		t.Errorf("store value = %#v, want a Transpose of the matrix", fn.Expressions[stores[0].Value].Kind)
	}
}

// addRowMajorNonSquareMatrixMember declares a struct whose member is a row-major
// mat3x2: three columns of two rows. Ids: 45 the matrix, 46 its struct, 47 the
// struct pointer, 48 the matrix pointer, 60 the struct member index.
func addRowMajorNonSquareMatrixMember(m *bodyModule) *bodyModule {
	m.annotate(
		instWords(spvenums.OpMemberDecorate, 46, 0, uint32(spvenums.DecorationRowMajor)),
	)
	return m.add(
		instWords(spvenums.OpTypeMatrix, 45, 7, 3),
		instWords(spvenums.OpTypeStruct, 46, 45),
		instWords(spvenums.OpTypePointer, 47, uint32(spvenums.StorageClassFunction), 46),
		instWords(spvenums.OpTypePointer, 48, uint32(spvenums.StorageClassFunction), 45),
		instWords(spvenums.OpConstant, 3, 60, 0),
	)
}

// TestRowMajorNonSquareStoreRefused: the transpose that maps row-major memory
// onto the IR's column-major matrices needs equal row and column counts, so a
// store through a non-square row-major matrix is refused instead of written at
// offsets the load could not read back.
func TestRowMajorNonSquareStoreRefused(t *testing.T) {
	m := addRowMajorNonSquareMatrixMember(newBodyModule())
	err := m.parseError(t,
		instWords(spvenums.OpVariable, 47, 200, uint32(spvenums.StorageClassFunction)),
		instWords(spvenums.OpAccessChain, 48, 201, 200, 60),
		instWords(spvenums.OpCompositeConstruct, 7, 210, 22, 22),
		instWords(spvenums.OpCompositeConstruct, 45, 211, 210, 210, 210),
		instWords(spvenums.OpStore, 201, 211),
	)
	if !strings.Contains(err.Error(), "not square") {
		t.Errorf("error = %q, want the non-square refusal", err)
	}
}

// loadsAfter returns the load expressions an Emit evaluates after the given
// store: the reads a store invalidated.
func loadsAfter(fn *ir.Function, store ir.StmtStore) []ir.ExpressionHandle {
	storeIndex := -1
	for i, statement := range fn.Body {
		if kind, ok := statement.Kind.(ir.StmtStore); ok && kind == store {
			storeIndex = i
		}
	}
	var loads []ir.ExpressionHandle
	for handle := range ir.ExpressionHandle(len(fn.Expressions)) {
		if _, ok := fn.Expressions[handle].Kind.(ir.ExprLoad); !ok {
			continue
		}
		if expressionStatement(fn, handle) > storeIndex {
			loads = append(loads, handle)
		}
	}
	return loads
}

// TestRowMajorMatrixColumnReadBack: a load of a column pointer after a store
// reads the column from the memory again — one element load per row, composed,
// because a column of a row-major matrix is strided.
func TestRowMajorMatrixColumnReadBack(t *testing.T) {
	m := addRowMajorMatrixMember(newBodyModule())
	fn := m.parseValidated(t,
		instWords(spvenums.OpVariable, 51, 200, uint32(spvenums.StorageClassFunction)),
		instWords(spvenums.OpAccessChain, 33, 201, 200, 60, 26), // member 0, column 1
		instWords(spvenums.OpCompositeConstruct, 7, 210, 22, 22),
		instWords(spvenums.OpStore, 201, 210),
		instWords(spvenums.OpLoad, 7, 202, 201),
	)
	stores := storeStatements(fn)
	if len(stores) != 2 {
		t.Fatalf("stores = %d, want 2 (one per row): %#v", len(stores), fn.Body)
	}
	loads := loadsAfter(fn, stores[0])
	if len(loads) != 2 {
		t.Fatalf("loads after the store = %d, want 2 (one per column element): %#v", len(loads), fn.Expressions)
	}
	composed, ok := findExpression(fn, func(kind ir.ExpressionKind) bool {
		compose, ok := kind.(ir.ExprCompose)
		if !ok || len(compose.Components) != 2 {
			return false
		}
		return compose.Components[0] == loads[0] && compose.Components[1] == loads[1]
	})
	if !ok {
		t.Errorf("no compose of the re-read column elements: %#v", fn.Expressions)
	}
	_ = composed
	for row, load := range loads {
		pointer, ok := fn.Expressions[load].Kind.(ir.ExprLoad)
		if !ok {
			t.Fatalf("load %d = %#v", row, fn.Expressions[load].Kind)
		}
		componentPointer, columnIndex, _, ok := accessParts(fn, pointer.Pointer)
		if !ok || columnIndex != 1 {
			t.Fatalf("load %d pointer = %#v, want the column index 1 outermost", row, fn.Expressions[pointer.Pointer].Kind)
		}
		if _, rowIndex, _, ok := accessParts(fn, componentPointer); !ok || rowIndex != uint32(row) {
			t.Errorf("load %d row = %d, want %d: the access indices are reversed", row, rowIndex, row)
		}
	}
}

// TestRowMajorMatrixElementReadBack: a load of an element pointer after a store
// reads the scalar again through the reversed access indices.
func TestRowMajorMatrixElementReadBack(t *testing.T) {
	m := addRowMajorMatrixMember(newBodyModule())
	fn := m.parseValidated(t,
		instWords(spvenums.OpVariable, 51, 200, uint32(spvenums.StorageClassFunction)),
		instWords(spvenums.OpAccessChain, 12, 201, 200, 60, 26, 60), // member 0, column 1, component 0
		instWords(spvenums.OpStore, 201, 22),
		instWords(spvenums.OpLoad, 5, 202, 201),
	)
	stores := storeStatements(fn)
	if len(stores) != 1 {
		t.Fatalf("stores = %d, want 1: %#v", len(stores), fn.Body)
	}
	loads := loadsAfter(fn, stores[0])
	if len(loads) != 1 {
		t.Fatalf("loads after the store = %d, want 1: %#v", len(loads), fn.Expressions)
	}
	pointer, ok := fn.Expressions[loads[0]].Kind.(ir.ExprLoad)
	if !ok {
		t.Fatalf("load = %#v", fn.Expressions[loads[0]].Kind)
	}
	rowPointer, columnIndex, _, ok := accessParts(fn, pointer.Pointer)
	if !ok || columnIndex != 1 {
		t.Fatalf("re-read pointer = %#v, want the column index 1 outermost", fn.Expressions[pointer.Pointer].Kind)
	}
	if _, _, _, ok := accessParts(fn, rowPointer); !ok {
		t.Errorf("re-read pointer = %#v, want the row index as its base", fn.Expressions[rowPointer].Kind)
	}
}

// TestRowMajorMatrixChainAfterStore: an access chain derived from a pointer a
// store wrote through refreshes the side-loaded value first, so the column it
// reads comes from the memory the store wrote.
func TestRowMajorMatrixChainAfterStore(t *testing.T) {
	m := addRowMajorMatrixMember(newBodyModule())
	fn := m.parseValidated(t,
		instWords(spvenums.OpVariable, 51, 200, uint32(spvenums.StorageClassFunction)),
		instWords(spvenums.OpAccessChain, 31, 201, 200, 60),
		instWords(spvenums.OpCompositeConstruct, 7, 210, 22, 22),
		instWords(spvenums.OpCompositeConstruct, 10, 211, 210, 210),
		instWords(spvenums.OpStore, 201, 211),
		instWords(spvenums.OpAccessChain, 33, 202, 201, 26), // column 1 of the stored pointer
		instWords(spvenums.OpLoad, 7, 203, 202),
	)
	stores := storeStatements(fn)
	if len(stores) != 1 {
		t.Fatalf("stores = %d, want 1: %#v", len(stores), fn.Body)
	}
	var transposes []ir.ExpressionHandle
	for handle := range ir.ExpressionHandle(len(fn.Expressions)) {
		if isTranspose(fn.Expressions[handle].Kind) {
			transposes = append(transposes, handle)
		}
	}
	if len(transposes) < 2 {
		t.Fatalf("transposes = %d, want the chain's side-load and the refresh after the store: %#v", len(transposes), fn.Expressions)
	}
	refreshed := transposes[len(transposes)-1]
	storeIndex := -1
	for i, statement := range fn.Body {
		if _, ok := statement.Kind.(ir.StmtStore); ok {
			storeIndex = i
		}
	}
	if expressionStatement(fn, refreshed) <= storeIndex {
		t.Fatalf("the refresh is evaluated before the store: Emit at %d, store at %d", expressionStatement(fn, refreshed), storeIndex)
	}
	if _, ok := findExpression(fn, func(kind ir.ExpressionKind) bool {
		access, ok := kind.(ir.ExprAccessIndex)
		return ok && access.Base == refreshed && access.Index == 1
	}); !ok {
		t.Errorf("the column is not read from the refreshed value: %#v", fn.Expressions)
	}
}

// TestRowMajorMatrixArrayStoreRefused: a whole array of row-major matrices has
// no transposed store — every element would need its own transpose — so the
// store is refused. Rust 30.0.0 writes it untransposed (its
// lookup_load_override covers OpLoad only); the SPIR-V spec decides.
func TestRowMajorMatrixArrayStoreRefused(t *testing.T) {
	m := addRowMajorMatrixArrayMember(newBodyModule())
	err := m.parseError(t,
		instWords(spvenums.OpVariable, 54, 200, uint32(spvenums.StorageClassFunction)),
		instWords(spvenums.OpAccessChain, 55, 201, 200, 60),
		instWords(spvenums.OpCompositeConstruct, 7, 210, 22, 22),
		instWords(spvenums.OpCompositeConstruct, 10, 211, 210, 210),
		instWords(spvenums.OpCompositeConstruct, 52, 212, 211, 211, 211, 211),
		instWords(spvenums.OpStore, 201, 212),
	)
	if !strings.Contains(err.Error(), "row-major") {
		t.Errorf("error = %q, want it to name the row-major store", err)
	}
}
