package ir

// TypeSize returns the byte size of a type following WGSL/WebGPU alignment rules.
// Matches Rust naga's TypeInner::try_size(gctx).
// Returns 0 for opaque types (samplers, images, pointers) and runtime-sized arrays.
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
		var count uint32
		if t.Size.Constant != nil {
			count = *t.Size.Constant
		} else {
			count = 1 // dynamic arrays have at least 1 element
		}
		return count * t.Stride
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
