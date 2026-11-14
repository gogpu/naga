// Package registry provides type deduplication for the naga IR.
// It ensures each unique type is registered exactly once, which is
// required by SPIR-V and other backends that need unique type declarations.
package registry

import (
	"fmt"
	"strconv"

	"github.com/gogpu/naga/ir"
)

// TypeRegistry ensures type deduplication for SPIR-V emission.
// SPIR-V requires that each unique type is declared exactly once.
type TypeRegistry struct {
	types   []ir.Type
	typeMap map[string]ir.TypeHandle
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
		types:   make([]ir.Type, 0, initialCap),
		typeMap: make(map[string]ir.TypeHandle, initialCap),
		keyBuf:  make([]byte, 0, 64),
	}
}

// NewTypeRegistryFrom creates a registry holding a copy of types, in order,
// so that GetOrCreate finds a module's existing types. Where types holds
// structurally equal entries the first one is found.
func NewTypeRegistryFrom(types []ir.Type) *TypeRegistry {
	r := NewTypeRegistryWithCap(len(types) + 8)
	for i, t := range types {
		r.types = append(r.types, t)
		r.buildKeyNamed(t.Name, t.NamePresent, t.Inner)
		if _, exists := r.typeMap[string(r.keyBuf)]; !exists {
			r.typeMap[string(r.keyBuf)] = ir.TypeHandle(i)
		}
	}
	return r
}

// GetOrCreate returns an existing handle for the type if it exists,
// or creates a new one if it's unique.
// Named struct types are never deduplicated with each other (different names
// mean different types), matching Rust naga's Arena behavior.
func (r *TypeRegistry) GetOrCreate(name string, inner ir.TypeInner) ir.TypeHandle {
	return r.getOrCreate(name, false, inner)
}

// GetOrCreateNamed is GetOrCreate for a name with its presence
// (ir.NamePresence): a present but empty name is a distinct type from an
// absent one, as Rust naga's UniqueArena sees Some("") and None.
func (r *TypeRegistry) GetOrCreateNamed(name string, present bool, inner ir.TypeInner) ir.TypeHandle {
	return r.getOrCreate(name, present, inner)
}

func (r *TypeRegistry) getOrCreate(name string, present bool, inner ir.TypeInner) ir.TypeHandle {
	// Build the full key in keyBuf. normalizeType writes the type portion,
	// and we prepend the name prefix if needed.
	r.buildKeyNamed(name, present, inner)

	// Use string(r.keyBuf) directly in the map index expression.
	// Go compiler optimizes m[string(b)] to avoid heap allocation for lookups.
	if handle, exists := r.typeMap[string(r.keyBuf)]; exists {
		return handle
	}

	// Cache miss — allocate key string for storage in map.
	key := string(r.keyBuf)

	// Create new type
	handle := ir.TypeHandle(len(r.types))
	r.types = append(r.types, ir.Type{
		Name:        name,
		NamePresent: present && name == "",
		Inner:       inner,
	})
	r.typeMap[key] = handle

	return handle
}

// buildKey writes the full dedup key into r.keyBuf.
// In Rust naga, UniqueArena deduplicates on the full ir.Type{name, inner} pair.
// ir.Type{name: None, inner: X} and ir.Type{name: Some("A"), inner: X} are distinct.
// We replicate this by including the name in the dedup key for ALL named types,
// not just structs. Anonymous types (name="") still dedup on inner alone.
// buildKeyNamed writes the dedup key of a type named with presence: a present
// but empty name gets a marker no absent one shares.
func (r *TypeRegistry) buildKeyNamed(name string, present bool, inner ir.TypeInner) {
	r.keyBuf = r.keyBuf[:0]
	if name != "" {
		r.keyBuf = append(r.keyBuf, "named:"...)
		r.appendStringKey(name)
		r.keyBuf = append(r.keyBuf, ':')
	} else if present {
		r.keyBuf = append(r.keyBuf, "empty-name:"...)
	}
	r.appendTypeKey(inner)
}

// SetName renames an existing type in the registry and updates the dedup key.
// Used when a type alias renames an anonymous type in-place (e.g., `alias rq = ray_query;`
// renames the anonymous RayQuery entry to "rq").
func (r *TypeRegistry) SetName(handle ir.TypeHandle, name string) {
	if int(handle) >= len(r.types) {
		return
	}
	oldName := r.types[handle].Name
	oldPresent := r.types[handle].NamePresent

	// Remove old key
	r.buildKeyNamed(oldName, oldPresent, r.types[handle].Inner)
	delete(r.typeMap, string(r.keyBuf))

	// A bit only says "present while empty"; renaming to a non-empty name
	// clears it, keeping the pair validateNamePresence accepts.
	r.types[handle].Name = name
	if name != "" {
		r.types[handle].NamePresent = false
	}
	r.buildKeyNamed(name, r.types[handle].NamePresent, r.types[handle].Inner)
	r.typeMap[string(r.keyBuf)] = handle
}

// Replace rewrites the type at handle, keeping its position in the arena, as
// Rust naga's UniqueArena::replace does, and reports whether it did. The dedup
// keys are updated with it. A definition equal to another type of the arena is
// refused: Rust naga panics there, and rewriting anyway would leave that
// type's dedup key pointing at this handle.
func (r *TypeRegistry) Replace(handle ir.TypeHandle, name string, inner ir.TypeInner) bool {
	if int(handle) >= len(r.types) {
		return false
	}
	r.buildKeyNamed(name, r.types[handle].NamePresent, inner)
	if existing, ok := r.typeMap[string(r.keyBuf)]; ok && existing != handle {
		return false
	}
	newKey := string(r.keyBuf)
	oldName, oldInner := r.types[handle].Name, r.types[handle].Inner
	r.buildKeyNamed(oldName, r.types[handle].NamePresent, oldInner)
	if r.typeMap[string(r.keyBuf)] == handle {
		delete(r.typeMap, string(r.keyBuf))
	}
	present := r.types[handle].NamePresent && name == ""
	r.types[handle] = ir.Type{Name: name, NamePresent: present, Inner: inner}
	r.typeMap[newKey] = handle
	return true
}

// Append adds a type without deduplication, always creating a new entry.
// This matches Rust naga's Arena behavior where each append() creates a new
// entry even for structurally identical types. Use this for array types that
// Rust naga does not deduplicate.
func (r *TypeRegistry) Append(name string, inner ir.TypeInner) ir.TypeHandle {
	handle := ir.TypeHandle(len(r.types))
	r.types = append(r.types, ir.Type{
		Name:  name,
		Inner: inner,
	})
	return handle
}

// GetTypes returns all registered types.
func (r *TypeRegistry) GetTypes() []ir.Type {
	return r.types
}

// appendTypeKey appends a unique key for the type to r.keyBuf.
// Two structurally identical types will produce the same key.
// All output goes to keyBuf to enable the Go map[string([]byte)] optimization
// in callers, avoiding heap allocations for cache-hit lookups.
func (r *TypeRegistry) appendTypeKey(inner ir.TypeInner) {
	switch t := inner.(type) {
	case ir.ScalarType:
		r.keyBuf = append(r.keyBuf, "scalar:"...)
		r.keyBuf = strconv.AppendInt(r.keyBuf, int64(t.Kind), 10)
		r.keyBuf = append(r.keyBuf, ':')
		r.keyBuf = strconv.AppendUint(r.keyBuf, uint64(t.Width), 10)

	case ir.VectorType:
		r.keyBuf = append(r.keyBuf, "vec:"...)
		r.keyBuf = strconv.AppendUint(r.keyBuf, uint64(t.Size), 10)
		r.keyBuf = append(r.keyBuf, ':')
		r.appendTypeKey(t.Scalar)

	case ir.MatrixType:
		r.keyBuf = append(r.keyBuf, "mat:"...)
		r.keyBuf = strconv.AppendUint(r.keyBuf, uint64(t.Columns), 10)
		r.keyBuf = append(r.keyBuf, 'x')
		r.keyBuf = strconv.AppendUint(r.keyBuf, uint64(t.Rows), 10)
		r.keyBuf = append(r.keyBuf, ':')
		r.appendTypeKey(t.Scalar)

	case ir.ArrayType:
		r.keyBuf = append(r.keyBuf, "array:"...)
		r.keyBuf = strconv.AppendInt(r.keyBuf, int64(t.Base), 10)
		r.keyBuf = append(r.keyBuf, ':')
		switch {
		case t.Size.Constant != nil:
			r.keyBuf = strconv.AppendUint(r.keyBuf, uint64(*t.Size.Constant), 10)
		case t.Size.Pending != nil:
			// Rust ArraySize::Pending(handle): one type per override.
			r.keyBuf = append(r.keyBuf, "override"...)
			r.keyBuf = strconv.AppendUint(r.keyBuf, uint64(*t.Size.Pending), 10)
		default:
			r.keyBuf = append(r.keyBuf, "runtime"...)
		}
		r.keyBuf = append(r.keyBuf, ':')
		r.keyBuf = strconv.AppendUint(r.keyBuf, uint64(t.Stride), 10)

	case ir.StructType:
		r.keyBuf = append(r.keyBuf, "struct:"...)
		r.keyBuf = strconv.AppendInt(r.keyBuf, int64(len(t.Members)), 10)
		r.keyBuf = append(r.keyBuf, ':')
		r.keyBuf = strconv.AppendUint(r.keyBuf, uint64(t.Span), 10)
		for _, member := range t.Members {
			r.keyBuf = append(r.keyBuf, ":m("...)
			r.appendStringKey(member.Name)
			r.keyBuf = append(r.keyBuf, ',')
			r.keyBuf = strconv.AppendInt(r.keyBuf, int64(member.Type), 10)
			r.keyBuf = append(r.keyBuf, ',')
			r.keyBuf = strconv.AppendUint(r.keyBuf, uint64(member.Offset), 10)
			r.keyBuf = append(r.keyBuf, ',')
			r.appendBindingKey(member.Binding)
			r.keyBuf = append(r.keyBuf, ')')
		}

	case ir.PointerType:
		r.keyBuf = append(r.keyBuf, "ptr:"...)
		r.keyBuf = strconv.AppendInt(r.keyBuf, int64(t.Base), 10)
		r.keyBuf = append(r.keyBuf, ':')
		r.keyBuf = strconv.AppendInt(r.keyBuf, int64(t.Space), 10)

	case ir.SamplerType:
		r.keyBuf = append(r.keyBuf, "sampler:"...)
		if t.Comparison {
			r.keyBuf = append(r.keyBuf, "true"...)
		} else {
			r.keyBuf = append(r.keyBuf, "false"...)
		}

	case ir.ImageType:
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

	case ir.AtomicType:
		r.keyBuf = append(r.keyBuf, "atomic:"...)
		r.keyBuf = strconv.AppendInt(r.keyBuf, int64(t.Scalar.Kind), 10)
		r.keyBuf = append(r.keyBuf, ':')
		r.keyBuf = strconv.AppendUint(r.keyBuf, uint64(t.Scalar.Width), 10)

	case ir.AccelerationStructureType:
		r.keyBuf = append(r.keyBuf, "acceleration_structure"...)

	case ir.RayQueryType:
		r.keyBuf = append(r.keyBuf, "ray_query"...)

	case ir.BindingArrayType:
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

// appendStringKey writes a string as its length, a colon and its bytes, so
// that a name containing the key's own delimiters cannot make two different
// types look equal. The strings of a SPIR-V module are untrusted.
func (r *TypeRegistry) appendStringKey(s string) {
	r.keyBuf = strconv.AppendInt(r.keyBuf, int64(len(s)), 10)
	r.keyBuf = append(r.keyBuf, ':')
	r.keyBuf = append(r.keyBuf, s...)
}

// appendBindingKey writes the key of a member binding. ir.Type equality
// includes the binding, so two structs that differ only in where their members
// bind are different types: the SPIR-V frontend makes exactly that difference
// between an input and an output block of the same layout.
func (r *TypeRegistry) appendBindingKey(binding *ir.Binding) {
	if binding == nil {
		r.keyBuf = append(r.keyBuf, "none"...)
		return
	}
	switch b := (*binding).(type) {
	case ir.BuiltinBinding:
		r.keyBuf = append(r.keyBuf, "builtin:"...)
		r.keyBuf = strconv.AppendInt(r.keyBuf, int64(b.Builtin), 10)
		r.keyBuf = append(r.keyBuf, ':')
		r.keyBuf = strconv.AppendBool(r.keyBuf, b.Invariant)
	case ir.LocationBinding:
		r.keyBuf = append(r.keyBuf, "location:"...)
		r.keyBuf = strconv.AppendUint(r.keyBuf, uint64(b.Location), 10)
		r.keyBuf = append(r.keyBuf, ':')
		if b.Interpolation != nil {
			r.keyBuf = strconv.AppendInt(r.keyBuf, int64(*b.Interpolation), 10)
		}
		r.keyBuf = append(r.keyBuf, ':')
		if b.Sampling != nil {
			r.keyBuf = strconv.AppendInt(r.keyBuf, int64(*b.Sampling), 10)
		}
		r.keyBuf = append(r.keyBuf, ':')
		if b.BlendSrc != nil {
			r.keyBuf = strconv.AppendUint(r.keyBuf, uint64(*b.BlendSrc), 10)
		}
		r.keyBuf = append(r.keyBuf, ':')
		r.keyBuf = strconv.AppendBool(r.keyBuf, b.PerPrimitive)
	default:
		r.keyBuf = fmt.Appendf(r.keyBuf, "binding:%T", b)
	}
}

// Lookup finds a type by its handle.
func (r *TypeRegistry) Lookup(handle ir.TypeHandle) (ir.Type, bool) {
	if int(handle) >= len(r.types) {
		return ir.Type{}, false
	}
	return r.types[handle], true
}

// Count returns the number of unique types registered.
func (r *TypeRegistry) Count() int {
	return len(r.types)
}
