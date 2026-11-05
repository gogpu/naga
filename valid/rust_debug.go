package valid

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// This file prints IR types as Rust's derived Debug prints naga's TypeInner,
// for the error messages that embed a type.

// scalarKindNames are Rust naga's ScalarKind variant names.
var scalarKindNames = [...]string{
	ir.ScalarSint:          "Sint",
	ir.ScalarUint:          "Uint",
	ir.ScalarFloat:         "Float",
	ir.ScalarBool:          "Bool",
	ir.ScalarAbstractInt:   "AbstractInt",
	ir.ScalarAbstractFloat: "AbstractFloat",
}

// vectorSizeNames are Rust naga's VectorSize variant names.
var vectorSizeNames = [...]string{
	ir.Vec2: "Bi",
	ir.Vec3: "Tri",
	ir.Vec4: "Quad",
}

// addressSpaceNames are Rust naga's AddressSpace variant names as its
// derived Debug prints them. Rust's Storage variant carries the access
// (`Storage { access: StorageAccess(LOAD | STORE) }`); naga's pointer types
// do not, so the field is elided as typeInnerDebug elides fields.
var addressSpaceNames = [...]string{
	ir.SpaceFunction:    "Function",
	ir.SpacePrivate:     "Private",
	ir.SpaceWorkGroup:   "WorkGroup",
	ir.SpaceUniform:     "Uniform",
	ir.SpaceStorage:     "Storage { .. }",
	ir.SpaceHandle:      "Handle",
	ir.SpaceImmediate:   "Immediate",
	ir.SpaceTaskPayload: "TaskPayload",
}

// addressSpaceDebug is s as Rust's Debug prints an AddressSpace.
func addressSpaceDebug(s ir.AddressSpace) string {
	if int(s) < len(addressSpaceNames) {
		return addressSpaceNames[s]
	}
	return fmt.Sprintf("AddressSpace(%d)", s)
}

// scalarDebug is s as Rust's Debug prints a Scalar.
func scalarDebug(s ir.ScalarType) string {
	kind := fmt.Sprintf("ScalarKind(%d)", s.Kind)
	if int(s.Kind) < len(scalarKindNames) {
		kind = scalarKindNames[s.Kind]
	}
	return fmt.Sprintf("Scalar { kind: %s, width: %d }", kind, s.Width)
}

// vectorSizeDebug is n as Rust's Debug prints a VectorSize.
func vectorSizeDebug(n ir.VectorSize) string {
	if int(n) < len(vectorSizeNames) && vectorSizeNames[n] != "" {
		return vectorSizeNames[n]
	}
	return fmt.Sprintf("VectorSize(%d)", n)
}

// typeInnerDebug is inner as Rust's Debug prints a TypeInner. Scalars,
// vectors and matrices are printed in full; other types by their variant
// name only, which no current message needs more of.
func typeInnerDebug(inner ir.TypeInner) string {
	switch t := inner.(type) {
	case ir.ScalarType:
		return "Scalar(" + scalarDebug(t) + ")"
	case ir.VectorType:
		return fmt.Sprintf("Vector { size: %s, scalar: %s }", vectorSizeDebug(t.Size), scalarDebug(t.Scalar))
	case ir.MatrixType:
		return fmt.Sprintf("Matrix { columns: %s, rows: %s, scalar: %s }",
			vectorSizeDebug(t.Columns), vectorSizeDebug(t.Rows), scalarDebug(t.Scalar))
	case ir.ArrayType:
		return "Array { .. }"
	case ir.StructType:
		return "Struct { .. }"
	case ir.PointerType:
		return "Pointer { .. }"
	}
	return fmt.Sprintf("%T", inner)
}
