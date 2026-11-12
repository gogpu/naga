// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/ir"
)

// The backend writes Options.LangVersion and never raises it: a feature of a
// newer MSL is refused with a *backend.VersionError, as Rust naga's MSL
// writer refuses it (back/msl/mod.rs resolve_local_binding, writer.rs). A
// raised version would fail only when Metal compiles the library, on a
// device that lacks it.

// requireVersion returns the refusal of feature, which needs MSL required,
// when Options.LangVersion is older, and nil otherwise.
func (w *Writer) requireVersion(required Version, feature string) error {
	if !w.options.LangVersion.Less(required) {
		return nil
	}
	return &backend.VersionError{
		Space:     backend.VersionMSL,
		Feature:   feature,
		Required:  backend.Version{Major: required.Major, Minor: required.Minor},
		Requested: backend.Version{Major: w.options.LangVersion.Major, Minor: w.options.LangVersion.Minor},
	}
}

// bitFunctions are the WGSL names of the math functions MSL has since 1.2.
var bitFunctions = map[ir.MathFunction]string{
	ir.MathReverseBits: "reverseBits",
	ir.MathExtractBits: "extractBits",
	ir.MathInsertBits:  "insertBits",
}

// checkModuleVersion refuses what the whole module needs, before anything
// is written, in Rust's order (writer.rs Writer::write): mesh shaders need
// MSL 3.0 (UnsupportedMeshShader), ray queries 2.4 (UnsupportedRayTracing).
func (w *Writer) checkModuleVersion() error {
	if usesMeshShaders(w.module) {
		if err := w.requireVersion(Version3_0, "a mesh or task shader"); err != nil {
			return err
		}
	}
	if usesRayQueries(w.module) {
		return w.requireVersion(Version2_4, "a ray query")
	}
	return nil
}

// usesRayQueries reports a ray query anywhere in the module. Rust checks
// the RayDesc and RayIntersection special types at module level
// (writer.rs) and every ray query statement and getIntersection expression
// (ray.rs, writer.rs); each of those works on a ray_query variable, so a
// ray_query type or the RayIntersection type covers them all.
func usesRayQueries(module *ir.Module) bool {
	if module.SpecialTypes.RayIntersection != nil {
		return true
	}
	for i := range module.Types {
		if _, ok := module.Types[i].Inner.(ir.RayQueryType); ok {
			return true
		}
	}
	return false
}

// usesMeshShaders is Rust naga's Module::uses_mesh_shaders (proc/mod.rs): a
// mesh or task entry point, a mesh built-in or per-primitive location on a
// struct member or an entry point's interface, or a task payload global.
func usesMeshShaders(module *ir.Module) bool {
	usesMesh := func(b *ir.Binding) bool {
		if b == nil {
			return false
		}
		switch b := (*b).(type) {
		case ir.BuiltinBinding:
			switch b.Builtin {
			case ir.BuiltinMeshTaskSize, ir.BuiltinCullPrimitive, ir.BuiltinPointIndex, ir.BuiltinLineIndices,
				ir.BuiltinTriangleIndices, ir.BuiltinVertexCount, ir.BuiltinVertices, ir.BuiltinPrimitiveCount,
				ir.BuiltinPrimitives:
				return true
			}
		case ir.LocationBinding:
			return b.PerPrimitive
		}
		return false
	}
	for i := range module.Types {
		if st, ok := module.Types[i].Inner.(ir.StructType); ok {
			for j := range st.Members {
				if usesMesh(st.Members[j].Binding) {
					return true
				}
			}
		}
	}
	for i := range module.EntryPoints {
		ep := &module.EntryPoints[i]
		if ep.Stage == ir.StageMesh || ep.Stage == ir.StageTask {
			return true
		}
		for _, arg := range ep.Function.Arguments {
			if usesMesh(arg.Binding) {
				return true
			}
		}
		if ep.Function.Result != nil && usesMesh(ep.Function.Result.Binding) {
			return true
		}
	}
	for i := range module.GlobalVariables {
		if module.GlobalVariables[i].Space == ir.SpaceTaskPayload {
			return true
		}
	}
	return false
}

// checkInterfaceVersion refuses an entry point whose inputs or outputs use
// an attribute of a newer MSL: Rust naga's Options::resolve_local_binding
// (back/msl/mod.rs), which every argument and result binding of a written
// entry point, struct members included, goes through.
func (w *Writer) checkInterfaceVersion(ep *ir.EntryPoint) error {
	check := func(b *ir.Binding, ty ir.TypeHandle, output, member bool) error {
		if b == nil {
			return nil
		}
		switch b := (*b).(type) {
		case ir.BuiltinBinding:
			if err := w.checkBuiltinVersion(b); err != nil {
				return err
			}
			return w.checkBuiltinAttribute(ep, b, ty, output, member)
		case ir.LocationBinding:
			// Rust checks blend_src on fragment outputs only.
			if b.BlendSrc != nil && output && ep.Stage == ir.StageFragment {
				return w.requireVersion(Version1_2, "@blend_src")
			}
		}
		return nil
	}
	walk := func(binding *ir.Binding, ty ir.TypeHandle, output bool) error {
		if binding != nil {
			return check(binding, ty, output, false)
		}
		if int(ty) >= len(w.module.Types) {
			return nil
		}
		if st, ok := w.module.Types[ty].Inner.(ir.StructType); ok {
			for j := range st.Members {
				if err := check(st.Members[j].Binding, st.Members[j].Type, output, true); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, arg := range ep.Function.Arguments {
		if err := walk(arg.Binding, arg.Type, false); err != nil {
			return err
		}
	}
	if r := ep.Function.Result; r != nil {
		return walk(r.Binding, r.Type, true)
	}
	return nil
}

// checkBuiltinVersion is the built-in part of Rust's resolve_local_binding:
// invariant position 2.1, base_instance and instance_index 1.2 (base_instance,
// instance_id), primitive_index 2.3 (primitive_id), view_index 2.2
// (amplification_id) and barycentrics 2.3 (barycentric_coord).
func (w *Writer) checkBuiltinVersion(b ir.BuiltinBinding) error {
	switch b.Builtin {
	case ir.BuiltinPosition:
		if b.Invariant {
			return w.requireVersion(Version2_1, "@invariant")
		}
	case ir.BuiltinBaseInstance:
		return w.requireVersion(Version1_2, "@builtin(base_instance)")
	case ir.BuiltinInstanceIndex:
		return w.requireVersion(Version1_2, "@builtin(instance_index)")
	case ir.BuiltinPrimitiveIndex:
		return w.requireVersion(Version2_3, "@builtin(primitive_index)")
	case ir.BuiltinViewIndex:
		return w.requireVersion(Version2_2, "@builtin(view_index)")
	case ir.BuiltinBarycentric:
		return w.requireVersion(Version2_3, "@builtin(barycentric)")
	case ir.BuiltinBarycentricNoPerspective:
		return w.requireVersion(Version2_3, "@builtin(barycentric_no_perspective)")
	}
	return nil
}

// checkGlobalsVersion refuses a resource the entry point uses (directly or
// through the functions it calls) that its MSL version cannot pass, as Rust
// naga's write_functions does for the entry point's global arguments
// (writer.rs): below 1.2 a writable storage buffer in a fragment shader, a
// writable storage texture in a vertex or fragment shader and any
// read-write storage texture; binding arrays of samplers and of sampled,
// depth, read-only or write-only storage textures below 2.0, of read-write
// storage textures below 3.0. A 64-bit storage texture needs 3.1, where MSL
// has 64-bit texture atomics; Rust has no such check.
func (w *Writer) checkGlobalsVersion(ep *ir.EntryPoint, used map[uint32]struct{}) error {
	for h := range w.module.GlobalVariables {
		if _, ok := used[uint32(h)]; !ok {
			continue
		}
		global := &w.module.GlobalVariables[h]
		switch global.Space {
		case ir.SpaceStorage:
			if global.Access&ir.StorageAccessStore != 0 && ep.Stage == ir.StageFragment {
				if err := w.requireVersion(Version1_2, "a writable storage buffer in a fragment shader"); err != nil {
					return err
				}
			}
		case ir.SpaceHandle:
			if err := w.checkHandleVersion(ep, global.Type); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkHandleVersion is checkGlobalsVersion for one handle global of type ty.
func (w *Writer) checkHandleVersion(ep *ir.EntryPoint, ty ir.TypeHandle) error {
	switch inner := w.module.Types[ty].Inner.(type) {
	case ir.ImageType:
		if inner.Class != ir.ImageClassStorage {
			return nil
		}
		if inner.StorageAccess&ir.StorageAccessStore != 0 && (ep.Stage == ir.StageVertex || ep.Stage == ir.StageFragment) {
			if err := w.requireVersion(Version1_2, "a writable storage texture in a vertex or fragment shader"); err != nil {
				return err
			}
		}
		const readWrite = ir.StorageAccessLoad | ir.StorageAccessStore
		if inner.StorageAccess&readWrite == readWrite {
			if err := w.requireVersion(Version1_2, "a read-write storage texture"); err != nil {
				return err
			}
		}
		return w.check64BitTextureVersion(inner)
	case ir.BindingArrayType:
		switch base := w.module.Types[inner.Base].Inner.(type) {
		case ir.SamplerType:
			return w.requireVersion(Version2_0, "a binding array of samplers")
		case ir.ImageType:
			if err := w.checkTextureArrayVersion(base); err != nil {
				return err
			}
			return w.check64BitTextureVersion(base)
		}
	}
	return nil
}

// checkTextureArrayVersion is Rust's binding array check for an image base
// type: sampled, depth and read-only storage textures need MSL 2.0 (Rust's
// "textures"), write-only storage textures 2.0, other storage textures 3.0.
// An array of external textures is refused elsewhere, not for its version.
func (w *Writer) checkTextureArrayVersion(base ir.ImageType) error {
	switch {
	case base.Class == ir.ImageClassExternal:
		return nil
	case base.Class != ir.ImageClassStorage, base.StorageAccess == ir.StorageAccessLoad:
		return w.requireVersion(Version2_0, "a binding array of textures")
	case base.StorageAccess == ir.StorageAccessStore:
		return w.requireVersion(Version2_0, "a binding array of write-only storage textures")
	}
	return w.requireVersion(Version3_0, "a binding array of read-write storage textures")
}

// check64BitTextureVersion refuses an r64uint or r64sint storage texture
// below MSL 3.1, the version of 64-bit texture atomics.
func (w *Writer) check64BitTextureVersion(img ir.ImageType) error {
	if img.Class == ir.ImageClassStorage &&
		(img.StorageFormat == ir.StorageFormatR64Uint || img.StorageFormat == ir.StorageFormatR64Sint) {
		return w.requireVersion(Version3_1, "a 64-bit storage texture (r64uint, r64sint)")
	}
	return nil
}
