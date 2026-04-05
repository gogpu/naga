package ir

import (
	"fmt"
	"strconv"
)

// TypeRegistry ensures type deduplication for SPIR-V emission.
// SPIR-V requires that each unique type is declared exactly once.
type TypeRegistry struct {
	types   []Type
	typeMap map[string]TypeHandle
	keyBuf  []byte // reusable buffer for building type keys
}

// NewTypeRegistry creates a new type registry for deduplication.
func NewTypeRegistry() *TypeRegistry {
	return NewTypeRegistryWithCap(16)
}

// NewTypeRegistryWithCap creates a new type registry with pre-allocated capacity.
// Use this when the expected number of types is known to avoid slice regrowth.
func NewTypeRegistryWithCap(initialCap int) *TypeRegistry {
	if initialCap < 16 {
		initialCap = 16
	}
	return &TypeRegistry{
		types:   make([]Type, 0, initialCap),
		typeMap: make(map[string]TypeHandle, initialCap),
		keyBuf:  make([]byte, 0, 64),
	}
}

// GetOrCreate returns an existing handle for the type if it exists,
// or creates a new one if it's unique.
// Named struct types are never deduplicated with each other (different names
// mean different types), matching Rust naga's Arena behavior.
func (r *TypeRegistry) GetOrCreate(name string, inner TypeInner) TypeHandle {
	// Build the full key in keyBuf. normalizeType writes the type portion,
	// and we prepend the name prefix if needed.
	r.buildKey(name, inner)

	// Use string(r.keyBuf) directly in the map index expression.
	// Go compiler optimizes m[string(b)] to avoid heap allocation for lookups.
	if handle, exists := r.typeMap[string(r.keyBuf)]; exists {
		return handle
	}

	// Cache miss — allocate key string for storage in map.
	key := string(r.keyBuf)

	// Create new type
	handle := TypeHandle(len(r.types))
	r.types = append(r.types, Type{
		Name:  name,
		Inner: inner,
	})
	r.typeMap[key] = handle

	return handle
}

// buildKey writes the full dedup key into r.keyBuf.
// In Rust naga, UniqueArena deduplicates on the full Type{name, inner} pair.
// Type{name: None, inner: X} and Type{name: Some("A"), inner: X} are distinct.
// We replicate this by including the name in the dedup key for ALL named types,
// not just structs. Anonymous types (name="") still dedup on inner alone.
func (r *TypeRegistry) buildKey(name string, inner TypeInner) {
	r.keyBuf = r.keyBuf[:0]
	if name != "" {
		r.keyBuf = append(r.keyBuf, "named:"...)
		r.keyBuf = append(r.keyBuf, name...)
		r.keyBuf = append(r.keyBuf, ':')
	}
	r.appendTypeKey(inner)
}

// SetName renames an existing type in the registry and updates the dedup key.
// Used when a type alias renames an anonymous type in-place (e.g., `alias rq = ray_query;`
// renames the anonymous RayQuery entry to "rq").
func (r *TypeRegistry) SetName(handle TypeHandle, name string) {
	if int(handle) >= len(r.types) {
		return
	}
	oldName := r.types[handle].Name
	r.types[handle].Name = name

	// Remove old key
	r.buildKey(oldName, r.types[handle].Inner)
	delete(r.typeMap, string(r.keyBuf))

	// Add new key
	r.buildKey(name, r.types[handle].Inner)
	r.typeMap[string(r.keyBuf)] = handle
}

// Append adds a type without deduplication, always creating a new entry.
// This matches Rust naga's Arena behavior where each append() creates a new
// entry even for structurally identical types. Use this for array types that
// Rust naga does not deduplicate.
func (r *TypeRegistry) Append(name string, inner TypeInner) TypeHandle {
	handle := TypeHandle(len(r.types))
	r.types = append(r.types, Type{
		Name:  name,
		Inner: inner,
	})
	return handle
}

// GetTypes returns all registered types.
func (r *TypeRegistry) GetTypes() []Type {
	return r.types
}

// appendTypeKey appends a unique key for the type to r.keyBuf.
// Two structurally identical types will produce the same key.
// All output goes to keyBuf to enable the Go map[string([]byte)] optimization
// in callers, avoiding heap allocations for cache-hit lookups.
func (r *TypeRegistry) appendTypeKey(inner TypeInner) {
	switch t := inner.(type) {
	case ScalarType:
		r.keyBuf = append(r.keyBuf, "scalar:"...)
		r.keyBuf = strconv.AppendInt(r.keyBuf, int64(t.Kind), 10)
		r.keyBuf = append(r.keyBuf, ':')
		r.keyBuf = strconv.AppendUint(r.keyBuf, uint64(t.Width), 10)

	case VectorType:
		r.keyBuf = append(r.keyBuf, "vec:"...)
		r.keyBuf = strconv.AppendUint(r.keyBuf, uint64(t.Size), 10)
		r.keyBuf = append(r.keyBuf, ':')
		r.appendTypeKey(t.Scalar)

	case MatrixType:
		r.keyBuf = append(r.keyBuf, "mat:"...)
		r.keyBuf = strconv.AppendUint(r.keyBuf, uint64(t.Columns), 10)
		r.keyBuf = append(r.keyBuf, 'x')
		r.keyBuf = strconv.AppendUint(r.keyBuf, uint64(t.Rows), 10)
		r.keyBuf = append(r.keyBuf, ':')
		r.appendTypeKey(t.Scalar)

	case ArrayType:
		r.keyBuf = append(r.keyBuf, "array:"...)
		r.keyBuf = strconv.AppendInt(r.keyBuf, int64(t.Base), 10)
		r.keyBuf = append(r.keyBuf, ':')
		if t.Size.Constant != nil {
			r.keyBuf = strconv.AppendUint(r.keyBuf, uint64(*t.Size.Constant), 10)
		} else {
			r.keyBuf = append(r.keyBuf, "runtime"...)
		}
		r.keyBuf = append(r.keyBuf, ':')
		r.keyBuf = strconv.AppendUint(r.keyBuf, uint64(t.Stride), 10)

	case StructType:
		r.keyBuf = append(r.keyBuf, "struct:"...)
		r.keyBuf = strconv.AppendInt(r.keyBuf, int64(len(t.Members)), 10)
		r.keyBuf = append(r.keyBuf, ':')
		r.keyBuf = strconv.AppendUint(r.keyBuf, uint64(t.Span), 10)
		for _, member := range t.Members {
			r.keyBuf = append(r.keyBuf, ":m("...)
			r.keyBuf = append(r.keyBuf, member.Name...)
			r.keyBuf = append(r.keyBuf, ',')
			r.keyBuf = strconv.AppendInt(r.keyBuf, int64(member.Type), 10)
			r.keyBuf = append(r.keyBuf, ',')
			r.keyBuf = strconv.AppendUint(r.keyBuf, uint64(member.Offset), 10)
			r.keyBuf = append(r.keyBuf, ')')
		}

	case PointerType:
		r.keyBuf = append(r.keyBuf, "ptr:"...)
		r.keyBuf = strconv.AppendInt(r.keyBuf, int64(t.Base), 10)
		r.keyBuf = append(r.keyBuf, ':')
		r.keyBuf = strconv.AppendInt(r.keyBuf, int64(t.Space), 10)

	case SamplerType:
		r.keyBuf = append(r.keyBuf, "sampler:"...)
		if t.Comparison {
			r.keyBuf = append(r.keyBuf, "true"...)
		} else {
			r.keyBuf = append(r.keyBuf, "false"...)
		}

	case ImageType:
		r.keyBuf = append(r.keyBuf, "image:"...)
		r.keyBuf = strconv.AppendInt(r.keyBuf, int64(t.Dim), 10)
		r.keyBuf = append(r.keyBuf, ':')
		if t.Arrayed {
			r.keyBuf = append(r.keyBuf, "true"...)
		} else {
			r.keyBuf = append(r.keyBuf, "false"...)
		}
		r.keyBuf = append(r.keyBuf, ':')
		r.keyBuf = strconv.AppendInt(r.keyBuf, int64(t.Class), 10)
		r.keyBuf = append(r.keyBuf, ':')
		if t.Multisampled {
			r.keyBuf = append(r.keyBuf, "true"...)
		} else {
			r.keyBuf = append(r.keyBuf, "false"...)
		}
		r.keyBuf = append(r.keyBuf, ':')
		r.keyBuf = strconv.AppendInt(r.keyBuf, int64(t.StorageFormat), 10)
		r.keyBuf = append(r.keyBuf, ':')
		r.keyBuf = strconv.AppendUint(r.keyBuf, uint64(t.StorageAccess), 10)
		r.keyBuf = append(r.keyBuf, ':')
		r.keyBuf = strconv.AppendInt(r.keyBuf, int64(t.SampledKind), 10)

	case AtomicType:
		r.keyBuf = append(r.keyBuf, "atomic:"...)
		r.keyBuf = strconv.AppendInt(r.keyBuf, int64(t.Scalar.Kind), 10)
		r.keyBuf = append(r.keyBuf, ':')
		r.keyBuf = strconv.AppendUint(r.keyBuf, uint64(t.Scalar.Width), 10)

	case AccelerationStructureType:
		r.keyBuf = append(r.keyBuf, "acceleration_structure"...)

	case RayQueryType:
		r.keyBuf = append(r.keyBuf, "ray_query"...)

	case BindingArrayType:
		r.keyBuf = append(r.keyBuf, "binding_array:"...)
		r.keyBuf = strconv.AppendInt(r.keyBuf, int64(t.Base), 10)
		r.keyBuf = append(r.keyBuf, ':')
		if t.Size != nil {
			r.keyBuf = strconv.AppendUint(r.keyBuf, uint64(*t.Size), 10)
		} else {
			r.keyBuf = append(r.keyBuf, "unbounded"...)
		}

	default:
		r.keyBuf = append(r.keyBuf, "unknown:"...)
		r.keyBuf = fmt.Appendf(r.keyBuf, "%T", inner)
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
