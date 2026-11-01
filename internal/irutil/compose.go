package irutil

import "github.com/gogpu/naga/ir"

// FlattenCompose returns the components of a module-scope Compose of type ty,
// spread the way a vector constant spells them out: one handle per lane.
//
// When ty is a vector, nested vector arguments are expanded into their scalars,
// so vec4(vec3(vec2(6, 7), 8), 9) yields four handles, and a Splat argument
// yields its value once per lane, as in vec4(vec3(1.0), 1.0). A Constant
// argument holding a vector is followed to its Init expression and expanded the
// same way. For any other ty the components are returned unchanged.
//
// This mirrors Rust naga's proc::flatten_compose. Rust's constant evaluator has
// already replaced a vector Constant or ZeroValue argument by the time the
// backends run; ours has not, so the Constant case is handled here. A vector
// ZeroValue argument has no scalar handles to expand into and is returned as is,
// for the caller to expand.
//
// Arguments are followed only to lower handles, which the arena's ordering
// guarantees for a well-formed module, so a malformed cycle cannot recurse.
func FlattenCompose(module *ir.Module, ty ir.TypeHandle, components []ir.ExpressionHandle) []ir.ExpressionHandle {
	if int(ty) >= len(module.Types) {
		return components
	}
	if _, isVector := module.Types[ty].Inner.(ir.VectorType); !isVector {
		return components
	}
	flat := make([]ir.ExpressionHandle, 0, 4)
	for _, component := range components {
		flat = appendFlattened(flat, module, component, len(module.GlobalExpressions))
	}
	return flat
}

// appendFlattened appends the lanes of the global expression handle to flat.
// Only handles below limit are expanded; anything else is a lane of its own.
func appendFlattened(flat []ir.ExpressionHandle, module *ir.Module, handle ir.ExpressionHandle, limit int) []ir.ExpressionHandle {
	if int(handle) >= limit {
		return append(flat, handle)
	}
	switch kind := module.GlobalExpressions[handle].Kind.(type) {
	case ir.ExprCompose:
		for _, component := range kind.Components {
			flat = appendFlattened(flat, module, component, int(handle))
		}
		return flat

	case ir.ExprSplat:
		for range kind.Size {
			flat = append(flat, kind.Value)
		}
		return flat

	case ir.ExprConstant:
		if int(kind.Constant) < len(module.Constants) {
			constant := &module.Constants[kind.Constant]
			if constant.Init < handle && int(constant.Type) < len(module.Types) {
				if _, isVector := module.Types[constant.Type].Inner.(ir.VectorType); isVector {
					return appendFlattened(flat, module, constant.Init, int(handle))
				}
			}
		}
	}
	return append(flat, handle)
}
