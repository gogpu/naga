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
	return &TypeRegistry{
		types:   make([]Type, 0, 16),
		typeMap: make(map[string]TypeHandle, 16),
		keyBuf:  make([]byte, 0, 64),
	}
}

// GetOrCreate returns an existing handle for the type if it exists,
// or creates a new one if it's unique.
// Named struct types are never deduplicated with each other (different names
// mean different types), matching Rust naga's Arena behavior.
func (r *TypeRegistry) GetOrCreate(name string, inner TypeInner) TypeHandle {
	key := r.normalizeType(inner)
	// In Rust naga, UniqueArena deduplicates on the full Type{name, inner} pair.
	// Type{name: None, inner: X} and Type{name: Some("A"), inner: X} are distinct.
	// We replicate this by including the name in the dedup key for ALL named types,
	// not just structs. Anonymous types (name="") still dedup on inner alone.
	if name != "" {
		key = "named:" + name + ":" + key
	}

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

// SetName renames an existing type in the registry and updates the dedup key.
// Used when a type alias renames an anonymous type in-place (e.g., `alias rq = ray_query;`
// renames the anonymous RayQuery entry to "rq").
func (r *TypeRegistry) SetName(handle TypeHandle, name string) {
	if int(handle) >= len(r.types) {
		return
	}
	oldName := r.types[handle].Name
	r.types[handle].Name = name

	// Update dedup key: remove old key, add new one
	oldKey := r.normalizeType(r.types[handle].Inner)
	if oldName != "" {
		oldKey = "named:" + oldName + ":" + oldKey
	}
	delete(r.typeMap, oldKey)

	newKey := r.normalizeType(r.types[handle].Inner)
	if name != "" {
		newKey = "named:" + name + ":" + newKey
	}
	r.typeMap[newKey] = handle
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

// normalizeType creates a unique key for a type based on its structure.
// Two structurally identical types will produce the same key.
// Uses a reusable byte buffer to avoid fmt.Sprintf allocations for common types.
func (r *TypeRegistry) normalizeType(inner TypeInner) string {
	b := r.keyBuf[:0]

	switch t := inner.(type) {
	case ScalarType:
		b = append(b, "scalar:"...)
		b = strconv.AppendInt(b, int64(t.Kind), 10)
		b = append(b, ':')
		b = strconv.AppendUint(b, uint64(t.Width), 10)
		r.keyBuf = b
		return string(b)

	case VectorType:
		// Recursive call clobbers keyBuf, so build with string concat.
		scalarKey := r.normalizeType(t.Scalar)
		return "vec:" + strconv.FormatUint(uint64(t.Size), 10) + ":" + scalarKey

	case MatrixType:
		scalarKey := r.normalizeType(t.Scalar)
		return "mat:" + strconv.FormatUint(uint64(t.Columns), 10) + "x" + strconv.FormatUint(uint64(t.Rows), 10) + ":" + scalarKey

	case ArrayType:
		var sizeKey string
		if t.Size.Constant != nil {
			sizeKey = strconv.FormatUint(uint64(*t.Size.Constant), 10)
		} else {
			sizeKey = "runtime"
		}
		return "array:" + strconv.FormatInt(int64(t.Base), 10) + ":" + sizeKey + ":" + strconv.FormatUint(uint64(t.Stride), 10)

	case StructType:
		// Structs use fmt.Sprintf since they're less frequent and more complex.
		key := fmt.Sprintf("struct:%d:%d", len(t.Members), t.Span)
		for _, member := range t.Members {
			key += fmt.Sprintf(":m(%s,%d,%d)", member.Name, member.Type, member.Offset)
		}
		return key

	case PointerType:
		return "ptr:" + strconv.FormatInt(int64(t.Base), 10) + ":" + strconv.FormatInt(int64(t.Space), 10)

	case SamplerType:
		if t.Comparison {
			return "sampler:true"
		}
		return "sampler:false"

	case ImageType:
		return fmt.Sprintf("image:%d:%v:%d:%v:%d:%d:%d", t.Dim, t.Arrayed, t.Class, t.Multisampled, t.StorageFormat, t.StorageAccess, t.SampledKind)

	case AtomicType:
		b = append(b, "atomic:"...)
		b = strconv.AppendInt(b, int64(t.Scalar.Kind), 10)
		b = append(b, ':')
		b = strconv.AppendUint(b, uint64(t.Scalar.Width), 10)
		r.keyBuf = b
		return string(b)

	case AccelerationStructureType:
		return "acceleration_structure"

	case RayQueryType:
		return "ray_query"

	case BindingArrayType:
		sizeKey := "unbounded"
		if t.Size != nil {
			sizeKey = strconv.FormatUint(uint64(*t.Size), 10)
		}
		return "binding_array:" + strconv.FormatInt(int64(t.Base), 10) + ":" + sizeKey

	default:
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
