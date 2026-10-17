package consteval

import "github.com/gogpu/naga/ir"

// binary evaluates a binary expression; operations with a matrix operand
// follow Rust naga's binary_op (matrix +/- matrix, matrix * scalar, the
// matrix-vector and matrix-matrix products).
func (e *Evaluator) binary(k ir.ExprBinary) (result, error) {
	lc, lt, lm := e.matrix(k.Left)
	rc, rt, rm := e.matrix(k.Right)
	if !lm && !rm {
		return e.scalarOp(func(v []Value) (Value, error) { return Binary(k.Op, v[0], v[1]) }, k.Left, k.Right)
	}
	switch {
	case lm && rm && (k.Op == ir.BinaryAdd || k.Op == ir.BinarySubtract):
		if lt.Columns != rt.Columns || lt.Rows != rt.Rows {
			return result{}, errInvalidBinaryArgs()
		}
		return e.mapColumns(lt, lc, lt.Scalar, func(i int, c Value) (Value, error) { return Binary(k.Op, c, rc[i]) })
	case k.Op != ir.BinaryMultiply:
		return result{}, errInvalidBinaryArgs()
	case lm && rm:
		return matMul(lt, lc, rt, rc)
	case lm:
		r, err := e.value(k.Right)
		if err != nil {
			return result{}, err
		}
		if r.IsScalar() {
			return e.mapColumns(lt, lc, lt.Scalar, func(_ int, c Value) (Value, error) { return Binary(ir.BinaryMultiply, c, r) })
		}
		return matVec(lt, lc, r)
	default:
		l, err := e.value(k.Left)
		if err != nil {
			return result{}, err
		}
		if l.IsScalar() {
			return e.mapColumns(rt, rc, rt.Scalar, func(_ int, c Value) (Value, error) { return Binary(ir.BinaryMultiply, l, c) })
		}
		return vecMat(l, rt, rc)
	}
}

// matrix reads an operand as a matrix: its columns and type. Constants are
// read where they live, without copying.
func (e *Evaluator) matrix(h ir.ExpressionHandle) ([]Value, ir.MatrixType, bool) {
	global, src := e.source(h)
	var th ir.TypeHandle
	var cols []ir.ExpressionHandle
	switch k := e.read(global, src).(type) {
	case ir.ExprCompose:
		th, cols = k.Type, k.Components
	case ir.ExprZeroValue:
		th = k.Type
	default:
		return nil, ir.MatrixType{}, false
	}
	mt, ok := e.Module.Types[th].Inner.(ir.MatrixType)
	if !ok {
		return nil, ir.MatrixType{}, false
	}
	out := make([]Value, mt.Columns)
	for i := range out {
		if cols == nil {
			zero := make([]ir.LiteralValue, mt.Rows)
			for j := range zero {
				zero[j] = zeroLiteral(mt.Scalar)
			}
			out[i] = Vector(zero...)
			continue
		}
		v, err := e.valueIn(global, cols[i])
		if err != nil {
			return nil, ir.MatrixType{}, false
		}
		out[i] = v
	}
	return out, mt, true
}

// appendMatrix appends a matrix as a Compose of column Composes.
func (e *Evaluator) appendMatrix(mt ir.MatrixType, cols []Value) ir.ExpressionHandle {
	comps := make([]ir.ExpressionHandle, len(cols))
	for i, c := range cols {
		comps[i] = e.appendValue(c)
	}
	return e.appendConst(ir.ExprCompose{Type: e.Types.Type(mt), Components: comps})
}

// dotProduct is Rust naga's dot_exprs: acc = a0*b0, then acc = acc + ai*bi,
// each step a binary operation of the operands' type.
func dotProduct(a, b []ir.LiteralValue) (ir.LiteralValue, error) {
	acc, err := scalarBinary(ir.BinaryMultiply, a[0], b[0])
	if err != nil {
		return nil, err
	}
	for i := 1; i < len(a); i++ {
		p, err := scalarBinary(ir.BinaryMultiply, a[i], b[i])
		if err != nil {
			return nil, err
		}
		if acc, err = scalarBinary(ir.BinaryAdd, acc, p); err != nil {
			return nil, err
		}
	}
	return acc, nil
}

// row returns row r of a matrix given by its columns.
func row(cols []Value, r int) []ir.LiteralValue {
	out := make([]ir.LiteralValue, len(cols))
	for c, col := range cols {
		out[c] = col.comps[r]
	}
	return out
}

// matVec is matCxR * vecC: component r is dot(row r, v).
func matVec(mt ir.MatrixType, cols []Value, v Value) (result, error) {
	if v.Len() != int(mt.Columns) {
		return result{}, errInvalidBinaryArgs()
	}
	out := make([]ir.LiteralValue, mt.Rows)
	for r := range out {
		d, err := dotProduct(row(cols, r), v.comps)
		if err != nil {
			return result{}, err
		}
		out[r] = d
	}
	return result{v: Vector(out...)}, nil
}

// vecMat is vecR * matCxR: component c is dot(v, column c).
func vecMat(v Value, mt ir.MatrixType, cols []Value) (result, error) {
	if v.Len() != int(mt.Rows) {
		return result{}, errInvalidBinaryArgs()
	}
	out := make([]ir.LiteralValue, mt.Columns)
	for c := range out {
		d, err := dotProduct(v.comps, cols[c].comps)
		if err != nil {
			return result{}, err
		}
		out[c] = d
	}
	return result{v: Vector(out...)}, nil
}

// matMul is matKxR * matCxK = matCxR: element [c][r] is dot(row r of the
// left, column c of the right).
func matMul(lt ir.MatrixType, lc []Value, rt ir.MatrixType, rc []Value) (result, error) {
	if lt.Columns != rt.Rows {
		return result{}, errInvalidBinaryArgs()
	}
	out := make([]Value, rt.Columns)
	for c := range out {
		col := make([]ir.LiteralValue, lt.Rows)
		for r := range col {
			d, err := dotProduct(row(lc, r), rc[c].comps)
			if err != nil {
				return result{}, err
			}
			col[r] = d
		}
		out[c] = Vector(col...)
	}
	return result{cols: out, matrix: ir.MatrixType{Columns: rt.Columns, Rows: lt.Rows, Scalar: lt.Scalar}, isMat: true}, nil
}
