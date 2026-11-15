package front

// This file is the GL/ES binding model of design §11 (R2): a source that
// cannot bind itself (GL 3.30 core and ES 3.00 have no layout(binding)) gets
// its module-scope resources numbered by one counter per group over every
// bound resource, with ParseOptions.ResourceBindings reserving the slots the
// caller names. Vulkan GLSL keeps its declared layout(binding) and consults no
// map.

import (
	"fmt"
	"sort"

	"github.com/gogpu/naga/ir"
)

// bindingAllocator places the module-scope resources of one module. A supplied
// entry reserves its slot before any declaration is lowered, so an automatic
// allocation never takes a slot the caller asked for and never depends on
// declaration order relative to the named resource.
type bindingAllocator struct {
	// supplied is ParseOptions.ResourceBindings by source name.
	supplied map[string]ir.ResourceBinding
	// consumed marks the supplied names a resource declaration used.
	consumed map[string]bool
	// used holds the (group, binding) pairs any resource occupies, whether
	// declared by the source (Vulkan GLSL) or assigned here.
	used map[resourceKey]bool
	// reserved holds the slots a supplied entry claims, by its source name.
	// Automatic placement skips them.
	reserved map[resourceKey]string
}

// newBindingAllocator builds the allocator of a module and reserves every slot
// the caller supplied.
func newBindingAllocator(supplied map[string]ir.ResourceBinding) *bindingAllocator {
	a := &bindingAllocator{
		supplied: supplied,
		consumed: map[string]bool{},
		used:     map[resourceKey]bool{},
		reserved: map[resourceKey]string{},
	}
	for name, binding := range supplied {
		a.reserved[resourceKey{group: binding.Group, binding: binding.Binding}] = name
	}
	return a
}

// markUsed records a slot the source itself declares (Vulkan GLSL). A
// duplicate declared (set, binding) is left to validation, as Rust does
// (design §11: the pair may be shared by two resources no entry point uses
// together).
func (a *bindingAllocator) markUsed(key resourceKey) {
	a.used[key] = true
}

// nextFree returns the lowest binding of group no resource uses and no
// supplied entry reserves, and claims it.
func (a *bindingAllocator) nextFree(group uint32) uint32 {
	for binding := uint32(0); ; binding++ {
		key := resourceKey{group: group, binding: binding}
		if a.used[key] {
			continue
		}
		if _, reserved := a.reserved[key]; reserved {
			continue
		}
		a.used[key] = true
		return binding
	}
}

// forResource places the resource declared with the source name name in group.
// A supplied entry wins over the counter; claiming a slot another resource of
// the module already occupies is refused, because the caller asked for two
// globals to share one (the reason string is the message, empty on success).
func (a *bindingAllocator) forResource(name string, group uint32) (ir.ResourceBinding, string) {
	if binding, ok := a.supplied[name]; ok {
		a.consumed[name] = true
		key := resourceKey{group: binding.Group, binding: binding.Binding}
		if a.used[key] {
			return binding, fmt.Sprintf(
				"the ResourceBindings entry for %q places it at (group %d, binding %d), which another resource of the module already uses",
				name, binding.Group, binding.Binding)
		}
		a.used[key] = true
		delete(a.reserved, key)
		return binding, ""
	}
	return ir.ResourceBinding{Group: group, Binding: a.nextFree(group)}, ""
}

// taken reports whether any resource of the module occupies key.
func (a *bindingAllocator) taken(key resourceKey) bool {
	return a.used[key]
}

// claim marks key occupied.
func (a *bindingAllocator) claim(key resourceKey) {
	a.used[key] = true
}

// unusedNames returns the supplied names no resource declaration used, sorted
// so the warning order does not depend on map iteration. Design §11 (R5): an
// unmatched entry is a warning, never a silent fallback. Every entry of a
// Vulkan GLSL source is unmatched by construction, because the source declares
// its own bindings and the map is not consulted there.
func (a *bindingAllocator) unusedNames() []string {
	names := make([]string, 0, len(a.supplied))
	for name := range a.supplied {
		if !a.consumed[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
