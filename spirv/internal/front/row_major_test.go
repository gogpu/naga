package front

import (
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
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
