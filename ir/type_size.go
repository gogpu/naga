package ir

// TypeSize returns the byte size of a type following WGSL/WebGPU alignment rules.
// Matches Rust naga's TypeInner::try_size(gctx).
// Returns 0 for opaque types (samplers, images, pointers) and for an array
// whose size is given by an override not yet evaluated; a runtime-sized array
// counts one element.
func TypeSize(module *Module, handle TypeHandle) uint32 {
	if int(handle) >= len(module.Types) {
		return 0
	}
	return typeInnerSize(module, module.Types[handle].Inner)
}

func typeInnerSize(module *Module, inner TypeInner) uint32 {
	switch t := inner.(type) {
	case ScalarType:
		return uint32(t.Width)
	case AtomicType:
		return uint32(t.Scalar.Width)
	case VectorType:
		return uint32(t.Size) * uint32(t.Scalar.Width)
	case MatrixType:
		// Matrices are arrays of aligned columns. Column vector alignment
		// follows WGSL rules: vec2→align 2, vec3/vec4→align 4.
		colAlign := vectorAlignment(t.Rows)
		colStride := colAlign * uint32(t.Scalar.Width)
		return colStride * uint32(t.Columns)
	case ArrayType:
		// Rust try_size: a runtime-sized array counts one element; a pending
		// size counts its override's value, or 0 while it is not evaluated.
		l, err := t.Size.Resolve(module)
		switch {
		case err != nil:
			return 0
		case l.Dynamic:
			return t.Stride
		}
		return l.Known * t.Stride
	case StructType:
		return t.Span
	case PointerType, ValuePointerType:
		return 0
	case ImageType, SamplerType, AccelerationStructureType, RayQueryType, BindingArrayType:
		return 0
	default:
		return 0
	}
}

// TypeAlignment returns the byte alignment of a type following WGSL/WebGPU
// layout rules, as Rust naga's proc::Layouter. Opaque types (samplers,
// images, pointers, ...) have alignment 1.
func TypeAlignment(module *Module, handle TypeHandle) uint32 {
	if int(handle) >= len(module.Types) {
		return 1
	}
	switch t := module.Types[handle].Inner.(type) {
	case ScalarType:
		return max(uint32(t.Width), 1)
	case AtomicType:
		return max(uint32(t.Scalar.Width), 1)
	case VectorType:
		return vectorAlignment(t.Size) * uint32(t.Scalar.Width)
	case MatrixType:
		return vectorAlignment(t.Rows) * uint32(t.Scalar.Width)
	case ArrayType:
		return TypeAlignment(module, t.Base)
	case BindingArrayType:
		return TypeAlignment(module, t.Base)
	case StructType:
		align := uint32(1)
		for _, m := range t.Members {
			align = max(align, TypeAlignment(module, m.Type))
		}
		return align
	default:
		return 1
	}
}

// vectorAlignment returns the alignment in components for a vector size.
// vec2→2, vec3→4, vec4→4 (matches Rust Alignment::from(VectorSize)).
func vectorAlignment(size VectorSize) uint32 {
	switch size {
	case Vec2:
		return 2
	case Vec3, Vec4:
		return 4
	default:
		return 4
	}
}
