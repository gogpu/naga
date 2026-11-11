package codegen

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestWriteEntryPointOutputReturnValueError covers the writer-error branch of
// the struct return path: an out-of-range value expression surfaces as an
// error instead of a panic or a silent write. Valid modules never reach it;
// the test pins the contract.
func TestWriteEntryPointOutputReturnValueError(t *testing.T) {
	w := &Writer{
		module:                     &ir.Module{Types: []ir.Type{{Inner: ir.StructType{}}}},
		namer:                      newNamer(),
		entryPointOutputTypeActive: true,
		entryPointOutputType:       0,
		entryPointOutputStructName: "Output",
		currentFunction:            &ir.Function{},
	}
	ok, err := w.writeEntryPointOutputReturn(99)
	if err == nil {
		t.Fatalf("an out-of-range value wrote a return (ok=%v, out=%q)", ok, w.Out.String())
	}
	if ok {
		t.Error("reported a written return together with an error")
	}
}

// TestWriteWorkGroupUniformLoadPointerError covers the writer-error branch of
// the statement: an out-of-range pointer surfaces as an error.
func TestWriteWorkGroupUniformLoadPointerError(t *testing.T) {
	w := &Writer{
		module:           &ir.Module{},
		namer:            newNamer(),
		namedExpressions: map[ir.ExpressionHandle]string{},
		options:          &Options{},
		currentFunction:  &ir.Function{},
	}
	if err := w.writeWorkGroupUniformLoad(ir.StmtWorkGroupUniformLoad{Pointer: 99, Result: 5}); err == nil {
		t.Fatalf("an out-of-range pointer wrote a load (out=%q)", w.Out.String())
	}
}
