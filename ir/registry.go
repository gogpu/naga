package ir

import (
	"fmt"
)

// TypeRegistry ensures type deduplication for SPIR-V emission.
// SPIR-V requires that each unique type is declared exactly once.
type TypeRegistry struct {
	types   []Type
	typeMap map[string]TypeHandle
}

// NewTypeRegistry creates a new type registry for deduplication.
func NewTypeRegistry() *TypeRegistry {
	return &TypeRegistry{
		types:   []Type{},
		typeMap: make(map[string]TypeHandle),
	}
}

// GetOrCreate returns an existing handle for the type if it exists,
// or creates a new one if it's unique.
func (r *TypeRegistry) GetOrCreate(name string, inner TypeInner) TypeHandle {
	key := r.normalizeType(inner)

	// Check if type already exists
	if handle, exists := r.typeMap[key]; exists {
		return handle
	}

	// Create new type
	handle := TypeHandle(len(r.types))
	r.types = append(r.types, Type{
		Name:  name,
		Inner: inner,
	})
	r.typeMap[key] = handle

	return handle
}

// GetTypes returns all registered types.
func (r *TypeRegistry) GetTypes() []Type {
	return r.types
}

// normalizeType creates a unique key for a type based on its structure.
// Two structurally identical types will produce the same key.
func (r *TypeRegistry) normalizeType(inner TypeInner) string {
	switch t := inner.(type) {
	case ScalarType:
		return fmt.Sprintf("scalar:%d:%d", t.Kind, t.Width)

	case VectorType:
		scalarKey := r.normalizeType(t.Scalar)
		return fmt.Sprintf("vec:%d:%s", t.Size, scalarKey)

	case MatrixType:
		scalarKey := r.normalizeType(t.Scalar)
		return fmt.Sprintf("mat:%dx%d:%s", t.Columns, t.Rows, scalarKey)

	case ArrayType:
		var sizeKey string
		if t.Size.Constant != nil {
			sizeKey = fmt.Sprintf("%d", *t.Size.Constant)
		} else {
			sizeKey = "runtime"
		}
		// Note: We use the base handle directly as it's already deduplicated
		return fmt.Sprintf("array:%d:%s:%d", t.Base, sizeKey, t.Stride)

	case StructType:
		// For structs, we need to normalize all members
		key := fmt.Sprintf("struct:%d:%d", len(t.Members), t.Span)
		for _, member := range t.Members {
			key += fmt.Sprintf(":m(%s,%d,%d)", member.Name, member.Type, member.Offset)
		}
		return key

	case PointerType:
		return fmt.Sprintf("ptr:%d:%d", t.Base, t.Space)

	case SamplerType:
		return fmt.Sprintf("sampler:%v", t.Comparison)

	case ImageType:
		return fmt.Sprintf("image:%d:%v:%d:%v", t.Dim, t.Arrayed, t.Class, t.Multisampled)

	default:
		// Fallback for unknown types
		return fmt.Sprintf("unknown:%T", inner)
	}
}

// Lookup finds a type by its handle.
func (r *TypeRegistry) Lookup(handle TypeHandle) (Type, bool) {
	if int(handle) >= len(r.types) {
		return Type{}, false
	}
	return r.types[handle], true
}

// Count returns the number of unique types registered.
func (r *TypeRegistry) Count() int {
	return len(r.types)
}
