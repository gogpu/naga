package snapshot_test

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestLayouterMatchesTypeSize proves that ir.Layouter, which replaced the
// public ir.TypeSize and ir.TypeAlignment (ADR-010 B7), gives the numbers
// they gave for every type of every corpus shader that lowers.
// legacyTypeSize and legacyTypeAlignment are those two functions as they
// were, kept here as the oracle.
func TestLayouterMatchesTypeSize(t *testing.T) {
	modules, types := 0, 0
	for _, shader := range loadCorpus(t) {
		m, err := tryCompileToIR(shader.source)
		if err != nil {
			continue // an unsupported shader; TestRustReference tracks those
		}
		var l ir.Layouter
		if err := l.Update(m); err != nil {
			t.Errorf("%s: Layouter.Update: %v", shader.name, err)
			continue
		}
		modules++
		for h := range m.Types {
			th := ir.TypeHandle(h)
			got := l.Layout(th)
			want := ir.TypeLayout{Size: legacyTypeSize(m, th), Alignment: legacyTypeAlignment(m, th)}
			if got != want {
				t.Errorf("%s: type %d (%T): Layout = %+v, TypeSize/TypeAlignment gave %+v", shader.name, h, m.Types[h].Inner, got, want)
			}
			types++
		}
	}
	// About 200 corpus shaders lower (1227 types on 2026-11-01); a handful
	// would mean the comparison silently ran on almost nothing.
	if modules < 150 || types < 1000 {
		t.Fatalf("compared %d types of %d modules, want at least 1000 of 150", types, modules)
	}
	t.Logf("compared %d types of %d modules", types, modules)
}

// legacyTypeSize is ir.TypeSize before ADR-010 B7.
func legacyTypeSize(module *ir.Module, handle ir.TypeHandle) uint32 {
	if int(handle) >= len(module.Types) {
		return 0
	}
	return legacyTypeInnerSize(module, module.Types[handle].Inner)
}

func legacyTypeInnerSize(module *ir.Module, inner ir.TypeInner) uint32 {
	switch t := inner.(type) {
	case ir.ScalarType:
		return uint32(t.Width)
	case ir.AtomicType:
		return uint32(t.Scalar.Width)
	case ir.VectorType:
		return uint32(t.Size) * uint32(t.Scalar.Width)
	case ir.MatrixType:
		colAlign := legacyVectorAlignment(t.Rows)
		colStride := colAlign * uint32(t.Scalar.Width)
		return colStride * uint32(t.Columns)
	case ir.ArrayType:
		l, err := t.Size.Resolve(module)
		switch {
		case err != nil:
			return 0
		case l.Dynamic:
			return t.Stride
		}
		return l.Known * t.Stride
	case ir.StructType:
		return t.Span
	case ir.PointerType, ir.ValuePointerType:
		return 0
	case ir.ImageType, ir.SamplerType, ir.AccelerationStructureType, ir.RayQueryType, ir.BindingArrayType:
		return 0
	default:
		return 0
	}
}

// legacyTypeAlignment is ir.TypeAlignment before ADR-010 B7.
func legacyTypeAlignment(module *ir.Module, handle ir.TypeHandle) uint32 {
	if int(handle) >= len(module.Types) {
		return 1
	}
	switch t := module.Types[handle].Inner.(type) {
	case ir.ScalarType:
		return max(uint32(t.Width), 1)
	case ir.AtomicType:
		return max(uint32(t.Scalar.Width), 1)
	case ir.VectorType:
		return legacyVectorAlignment(t.Size) * uint32(t.Scalar.Width)
	case ir.MatrixType:
		return legacyVectorAlignment(t.Rows) * uint32(t.Scalar.Width)
	case ir.ArrayType:
		return legacyTypeAlignment(module, t.Base)
	case ir.BindingArrayType:
		return legacyTypeAlignment(module, t.Base)
	case ir.StructType:
		align := uint32(1)
		for _, m := range t.Members {
			align = max(align, legacyTypeAlignment(module, m.Type))
		}
		return align
	default:
		return 1
	}
}

func legacyVectorAlignment(size ir.VectorSize) uint32 {
	switch size {
	case ir.Vec2:
		return 2
	case ir.Vec3, ir.Vec4:
		return 4
	default:
		return 4
	}
}
