package front

import (
	"github.com/gogpu/naga/frontend"
	"github.com/gogpu/naga/ir"
)

// This file names the GLSL builtin variables the parser meets without a
// declaration, Rust naga's lookup_variable fallback (front/glsl/variables.rs:101-224).
//
// Rust resolves a builtin lazily, on its first use, and declares the result in
// the root scope of the symbol table then (variables.rs:96, SymbolTable::add_root),
// so a shader that never writes gl_Position carries no such name. The AST
// records the name and its symbol kind; the type, the storage class and the
// ir.BuiltIn are lowering's, from the same table in Rust (variables.rs:111-221).

// builtinVariableNames are the variable names GLSL provides without a
// declaration, in the order Rust naga's lookup_variable lists them
// (variables.rs:112-210). gl_FragColor and gl_FragData are not among them: they
// were removed in GLSL 3.30 and Rust has never supported them.
var builtinVariableNames = map[string]bool{
	"gl_Position":             true,
	"gl_FragCoord":            true,
	"gl_PointCoord":           true,
	"gl_GlobalInvocationID":   true,
	"gl_NumWorkGroups":        true,
	"gl_WorkGroupSize":        true,
	"gl_WorkGroupID":          true,
	"gl_LocalInvocationID":    true,
	"gl_FrontFacing":          true,
	"gl_PointSize":            true,
	"gl_FragDepth":            true,
	"gl_ClipDistance":         true,
	"gl_CullDistance":         true,
	"gl_BaseVertex":           true,
	"gl_BaseInstance":         true,
	"gl_PrimitiveID":          true,
	"gl_BaryCoordEXT":         true,
	"gl_BaryCoordNoPerspEXT":  true,
	"gl_InstanceIndex":        true,
	"gl_VertexIndex":          true,
	"gl_SampleID":             true,
	"gl_LocalInvocationIndex": true,
	"gl_DrawID":               true,
	// The GL_KHR_shader_subgroup_basic scalar variables (§8.18); their
	// availability is the extension's, checked when they are lowered.
	"gl_SubgroupSize":         true,
	"gl_SubgroupInvocationID": true,
	"gl_SubgroupID":           true,
	"gl_NumSubgroups":         true,
}

// builtinSymbol returns the symbol of the builtin variable name denotes, or nil
// when the name is not a builtin variable. Rust naga resolves these lazily and
// declares each in the root scope the first time it is used (variables.rs:96,
// 223); the ordinary lookup runs first here, so a name the shader declared
// itself shadows a builtin of the same name.
// fieldSelection lowers a struct field access or a vector swizzle, Rust
// naga's Frontend::field_selection (variables.rs:244-413). A one-component
// swizzle is an access, not a swizzle; a struct field in an Rhs position is
// loaded through the pointer, as is a swizzle of a pointer.
func (f *Frontend) fieldSelection(c *Context, pos exprPos, expression ir.ExpressionHandle, name string, span ir.Span) (ir.ExpressionHandle, *frontError) {
	ty, err := c.resolveType(expression, span)
	if err != nil {
		return 0, err
	}
	isPointer := false
	if ptr, ok := ty.(ir.PointerType); ok {
		ty = c.mod.Types[ptr.Base].Inner
		isPointer = true
	}

	switch t := ty.(type) {
	case ir.StructType:
		index := -1
		for i, member := range t.Members {
			if member.Name == name {
				index = i
				break
			}
		}
		if index < 0 {
			return 0, errMessage(span, "Unknown field: "+name)
		}
		pointer, err := c.addExpression(ir.ExprAccessIndex{Base: expression, Index: uint32(index)}, span)
		if err != nil {
			return 0, err
		}
		if pos.kind == posRHS && isPointer {
			return c.addExpression(ir.ExprLoad{Pointer: pointer}, span)
		}
		return pointer, nil

	case ir.VectorType:
		components, ok := swizzleComponents(name, t.Size)
		if !ok {
			return 0, errMessage(span, "Invalid swizzle for vector \""+name+"\"")
		}

		if pos.kind == posLHS && swizzleHasDuplicates(components) {
			// Rust's text here nests a quoted name inside a quoted clause.
			f.addError(errMessage(span, "swizzle cannot have duplicate components in left-hand-side expression for \"\""+name+"\"\""))
		}

		var pattern [4]ir.SwizzleComponent
		copy(pattern[:], components)

		// Flatten a nested swizzle: vec.zyx.xy.x is vec.z.
		vector := expression
		if sw, ok := c.Expression(expression).(ir.ExprSwizzle); ok {
			vector = sw.Vector
			for i := range pattern {
				pattern[i] = sw.Pattern[pattern[i]]
			}
		}

		var size ir.VectorSize
		switch len(components) {
		case 1:
			// A one-component swizzle is an access. In an Rhs position the
			// pointer itself would be loaded, so load it here first.
			if pos.kind == posRHS && isPointer {
				vector, err = c.addExpression(ir.ExprLoad{Pointer: vector}, span)
				if err != nil {
					return 0, err
				}
			}
			return c.addExpression(ir.ExprAccessIndex{Base: vector, Index: uint32(pattern[0])}, span)
		case 2:
			size = ir.Vec2
		case 3:
			size = ir.Vec3
		case 4:
			size = ir.Vec4
		default:
			f.addError(errMessage(span, "Bad swizzle size for \"\""+name+"\"\""))
			size = ir.Vec4
		}

		if isPointer {
			// Swizzles cannot operate on pointers, so load even in an Lhs
			// position; the assignment extracts the pointer anyway.
			vector, err = c.addExpression(ir.ExprLoad{Pointer: vector}, span)
			if err != nil {
				return 0, err
			}
		}
		return c.addExpression(ir.ExprSwizzle{Size: size, Vector: vector, Pattern: pattern}, span)

	default:
		return 0, errMessage(span, "Can't lookup field on this type \""+name+"\"")
	}
}

// swizzleComponents reads a swizzle name, Rust naga's
// check_swizzle_components: the name's characters in one component set
// (xyzw, rgba or stpq), each below the vector's size.
func swizzleComponents(name string, size ir.VectorSize) ([]ir.SwizzleComponent, bool) {
	for _, set := range [...]string{"xyzw", "rgba", "stpq"} {
		if components, ok := componentsIn(name, set, size); ok {
			return components, true
		}
	}
	return nil, false
}

// componentsIn reads name's components in one component set.
func componentsIn(name, set string, size ir.VectorSize) ([]ir.SwizzleComponent, bool) {
	components := make([]ir.SwizzleComponent, 0, len(name))
	for i := 0; i < len(name); i++ {
		index := -1
		for j := 0; j < len(set); j++ {
			if set[j] == name[i] {
				index = j
				break
			}
		}
		if index < 0 || index >= int(size) {
			return nil, false
		}
		components = append(components, ir.SwizzleComponent(index))
	}
	return components, true
}

// swizzleHasDuplicates reports whether a swizzle names a component twice.
func swizzleHasDuplicates(components []ir.SwizzleComponent) bool {
	for i := 1; i < len(components); i++ {
		for _, later := range components[i:] {
			if later == components[i-1] {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Lowering: builtin variables and module-scope declarations (Rust naga's
// front/glsl/variables.rs).
// ---------------------------------------------------------------------------

// builtinData describes a builtin variable the frontend creates on first use,
// Rust naga's BuiltInData (variables.rs:22-32).
type builtinData struct {
	// inner is the builtin's type; a dynamic array for the clip and cull
	// distances.
	inner   ir.TypeInner
	builtin ir.BuiltinValue
	mutable bool
	storage storageQualifier
	// clipCull selects the dynamic-array shape of gl_ClipDistance and
	// gl_CullDistance, whose element type is registered on first use.
	clipCull bool
}

// builtinDataFor returns what a GLSL builtin variable name denotes, or false
// when the name is not a builtin variable, Rust naga's lookup_variable table
// (variables.rs:111-221). The clip and cull distances are writable only in the
// vertex stage, which is why the stage is part of the lookup.
func builtinDataFor(name string, stage ir.ShaderStage) (builtinData, bool) {
	vec4f := ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	vec2f := ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	vec3u := ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	u32 := ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	boolean := ir.ScalarType{Kind: ir.ScalarBool, Width: 1}

	switch name {
	case "gl_Position":
		return builtinData{inner: vec4f, builtin: ir.BuiltinPosition, mutable: true, storage: storageOutput}, true
	case "gl_FragCoord":
		return builtinData{inner: vec4f, builtin: ir.BuiltinPosition, storage: storageInput}, true
	case "gl_PointCoord":
		return builtinData{inner: vec2f, builtin: ir.BuiltinPointCoord, storage: storageInput}, true
	case "gl_GlobalInvocationID":
		return builtinData{inner: vec3u, builtin: ir.BuiltinGlobalInvocationID, storage: storageInput}, true
	case "gl_NumWorkGroups":
		return builtinData{inner: vec3u, builtin: ir.BuiltinNumWorkGroups, storage: storageInput}, true
	case "gl_WorkGroupSize":
		return builtinData{inner: vec3u, builtin: ir.BuiltinWorkGroupSize, storage: storageInput}, true
	case "gl_WorkGroupID":
		return builtinData{inner: vec3u, builtin: ir.BuiltinWorkGroupID, storage: storageInput}, true
	case "gl_LocalInvocationID":
		return builtinData{inner: vec3u, builtin: ir.BuiltinLocalInvocationID, storage: storageInput}, true
	case "gl_FrontFacing":
		return builtinData{inner: boolean, builtin: ir.BuiltinFrontFacing, storage: storageInput}, true
	case "gl_PointSize":
		return builtinData{inner: f32, builtin: ir.BuiltinPointSize, mutable: true, storage: storageOutput}, true
	case "gl_FragDepth":
		return builtinData{inner: f32, builtin: ir.BuiltinFragDepth, mutable: true, storage: storageOutput}, true
	case "gl_ClipDistance":
		return builtinData{
			inner:    ir.ArrayType{Size: ir.ArraySize{}, Stride: 4},
			builtin:  ir.BuiltinClipDistances,
			mutable:  stage == ir.StageVertex,
			storage:  storageOutput,
			clipCull: true,
		}, true
	case "gl_CullDistance":
		return builtinData{
			inner:    ir.ArrayType{Size: ir.ArraySize{}, Stride: 4},
			builtin:  ir.BuiltinCullDistance,
			mutable:  stage == ir.StageVertex,
			storage:  storageOutput,
			clipCull: true,
		}, true
	case "gl_BaseVertex":
		return builtinData{inner: u32, builtin: ir.BuiltinBaseVertex, storage: storageInput}, true
	case "gl_BaseInstance":
		return builtinData{inner: u32, builtin: ir.BuiltinBaseInstance, storage: storageInput}, true
	case "gl_PrimitiveID":
		return builtinData{inner: u32, builtin: ir.BuiltinPrimitiveIndex, storage: storageInput}, true
	case "gl_BaryCoordEXT":
		return builtinData{inner: vec3u, builtin: ir.BuiltinBarycentric, storage: storageInput}, true
	case "gl_BaryCoordNoPerspEXT":
		return builtinData{inner: vec3u, builtin: ir.BuiltinBarycentricNoPerspective, storage: storageInput}, true
	case "gl_InstanceIndex":
		return builtinData{inner: u32, builtin: ir.BuiltinInstanceIndex, storage: storageInput}, true
	case "gl_VertexIndex":
		return builtinData{inner: u32, builtin: ir.BuiltinVertexIndex, storage: storageInput}, true
	case "gl_SampleID":
		return builtinData{inner: u32, builtin: ir.BuiltinSampleIndex, storage: storageInput}, true
	case "gl_LocalInvocationIndex":
		return builtinData{inner: u32, builtin: ir.BuiltinLocalInvocationIndex, storage: storageInput}, true
	case "gl_DrawID":
		return builtinData{inner: u32, builtin: ir.BuiltinDrawIndex, storage: storageInput}, true

	// The four scalar variables of GL_KHR_shader_subgroup_basic (GLSL 4.60
	// §8.18). Their ir.Builtin values exist and every backend writes them; like
	// the subgroup builtins they are available only where the source enables
	// the family's extension, which addBuiltin checks (subgroupVariableGate)
	// because the availability rule is the frontend's, not the type table's.
	case "gl_SubgroupSize":
		return builtinData{inner: u32, builtin: ir.BuiltinSubgroupSize, storage: storageInput}, true
	case "gl_SubgroupInvocationID":
		return builtinData{inner: u32, builtin: ir.BuiltinSubgroupInvocationID, storage: storageInput}, true
	case "gl_SubgroupID":
		return builtinData{inner: u32, builtin: ir.BuiltinSubgroupID, storage: storageInput}, true
	case "gl_NumSubgroups":
		return builtinData{inner: u32, builtin: ir.BuiltinNumSubgroups, storage: storageInput}, true
	}
	return builtinData{}, false
}

// addBuiltin creates the global variable a builtin name denotes on first use,
// Rust naga's Frontend::add_builtin (variables.rs:41-99). The global is
// private storage the entry-point wrapper fills or reads, and it is reported
// as an entry argument.
func (f *Frontend) addBuiltin(c *Context, sym *glslSymbol, span ir.Span) (*variableRef, *frontError) {
	data, ok := builtinDataFor(sym.Name, f.meta.stage)
	if !ok {
		return nil, errUnknownVariable(sym.Name, span)
	}
	// The subgroup variables are available only where the family's extension is
	// enabled (and not in GLSL ES before 3.10); glslang resolves the rule before
	// the name.
	if err := f.subgroupVariableGate(sym.Name, span); err != nil {
		return nil, err
	}
	var ty ir.TypeHandle
	if data.clipCull {
		element := f.registerType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
		ty = f.registerType(ir.ArrayType{Base: element, Size: ir.ArraySize{}, Stride: 4})
	} else {
		ty = f.registerType(data.inner)
	}

	handle := ir.GlobalVariableHandle(len(f.mod.GlobalVariables))
	f.mod.GlobalVariables = append(f.mod.GlobalVariables, ir.GlobalVariable{
		Name:  sym.Name,
		Space: ir.SpacePrivate,
		Type:  ty,
	})

	idx := len(f.entryArgs)
	f.entryArgs = append(f.entryArgs, entryArg{
		name:    sym.Name,
		binding: ir.BuiltinBinding{Builtin: data.builtin},
		handle:  handle,
		storage: data.storage,
	})

	lookup := globalLookup{kind: lookupVariable, variable: handle, entryArg: idx, mutable: data.mutable}
	f.globalVars = append(f.globalVars, namedLookup{name: sym.Name, sym: sym, lookup: lookup})

	expr, err := c.addExpression(ir.ExprGlobalVariable{Variable: handle}, span)
	if err != nil {
		return nil, err
	}
	ref := &variableRef{expr: expr, load: true, mutable: data.mutable, entryArg: idx}
	c.vars[sym] = ref
	return ref, nil
}

// makeVariableInvariant marks an invariant redeclaration, Rust naga's
// Frontend::make_variable_invariant (variables.rs:226-242): only the Position
// builtin carries the flag in the IR.
func (f *Frontend) makeVariableInvariant(c *Context, sym *glslSymbol, span ir.Span) *frontError {
	ref, err := c.variable(sym, span)
	if err != nil {
		return err
	}
	if ref.entryArg < 0 || ref.entryArg >= len(f.entryArgs) {
		return nil
	}
	if pos, ok := f.entryArgs[ref.entryArg].binding.(ir.BuiltinBinding); ok && pos.Builtin == ir.BuiltinPosition {
		pos.Invariant = true
		f.entryArgs[ref.entryArg].binding = pos
	}
	return nil
}

// qualifierKey identifies one entry of a layout(...) list, Rust naga's
// QualifierKey (ast.rs:180-189).
type qualifierKey struct {
	kind layoutKind
	name string
}

// qualifierEntry is one layout entry with its value evaluated, Rust naga's
// (QualifierValue, Span) pair.
type qualifierEntry struct {
	value    uint32
	hasValue bool
	layout   structLayout
	format   ir.StorageFormat
	// formatName is the format as the source spelled it (`r32ui`), for the
	// diagnostics that name the qualifier back to the shader.
	formatName string
	span       ir.Span
}

// layoutUse is one declaration's layout qualifiers as lowering consumes them:
// a repeated key keeps the last value (Rust stores them in a map, where a
// later insert replaces an earlier one), taking a key removes it, and the
// leftover entries are reported as unexpected.
type layoutUse struct {
	f       *Frontend
	entries map[qualifierKey]*qualifierEntry
	order   []qualifierKey
}

// takeLayoutQualifiers evaluates the values of a declaration's layout entries
// in a const context and returns them for consumption, Rust naga's
// TypeQualifiers layout map as the parser builds it (parser/types.rs:325-380).
// A value that is not a uint constant is reported and taken as 0, as Rust
// does when it reports the error at parse time.
func (f *Frontend) takeLayoutQualifiers(q *typeQualifiers) *layoutUse {
	use := &layoutUse{f: f, entries: map[qualifierKey]*qualifierEntry{}}
	for i := range q.Layout {
		entry := &q.Layout[i]
		key := qualifierKey{kind: entry.Kind, name: entry.Name}
		if entry.Kind == layoutFormat || entry.Kind == layoutIndex || entry.Kind == layoutStructure {
			// These three are single slots in Rust's map (QualifierKey::Format,
			// Index and Layout carry no name), so a repeated one keeps the
			// last value.
			key.name = ""
		}
		value := &qualifierEntry{
			layout: entry.Struct,
			format: entry.Format,
			span:   entry.Span,
		}
		if entry.Kind == layoutFormat {
			// The name of a format entry is its spelling in the source, which a
			// refusal quotes back (`'r32ui' : does not apply to …`); the entry's
			// key drops it like Rust's QualifierKey::Format, but the diagnostic
			// keeps it.
			value.formatName = entry.Name
		}
		if entry.HasValue && entry.Value != nil {
			v, err := f.constU32(entry.Value)
			if err != nil {
				f.addError(err)
			} else {
				value.value, value.hasValue = v, true
			}
		}
		if _, seen := use.entries[key]; !seen {
			use.order = append(use.order, key)
		}
		use.entries[key] = value
	}
	return use
}

// take removes an entry and returns it.
func (u *layoutUse) take(key qualifierKey) (*qualifierEntry, bool) {
	entry, ok := u.entries[key]
	if !ok {
		return nil, false
	}
	delete(u.entries, key)
	return entry, true
}

// peek returns a named entry without consuming it, so a check can run before
// the consumer takes the value.
func (u *layoutUse) peek(name string) (*qualifierEntry, bool) {
	entry, ok := u.entries[qualifierKey{kind: layoutNamed, name: name}]
	return entry, ok
}

// takeUint removes a named uint entry, Rust naga's uint_layout_qualifier
// (ast.rs:283-311): an entry without a value is an error, and 0 is returned
// so the caller does not report a second, less useful error.
func (u *layoutUse) takeUint(name string) (uint32, bool) {
	entry, ok := u.take(qualifierKey{kind: layoutNamed, name: name})
	if !ok {
		return 0, false
	}
	if !entry.hasValue {
		u.f.addError(errMessage(entry.span, "Qualifier expects a uint value"))
		return 0, true
	}
	return entry.value, true
}

// takeNone removes a named entry that takes no value, Rust naga's
// none_layout_qualifier (ast.rs:314-337).
func (u *layoutUse) takeNone(name string) bool {
	entry, ok := u.take(qualifierKey{kind: layoutNamed, name: name})
	if !ok {
		return false
	}
	if entry.hasValue {
		u.f.addError(errMessage(entry.span, "Qualifier doesn't expect a value"))
	}
	return true
}

// takeStructure removes the std140 or std430 entry.
func (u *layoutUse) takeStructure() (structLayout, bool) {
	entry, ok := u.take(qualifierKey{kind: layoutStructure})
	if !ok {
		return structLayoutNone, false
	}
	return entry.layout, true
}

// takeFormatEntry removes the storage-image format entry and returns it whole:
// the caller reports a class mismatch at the qualifier's own span, quoting the
// format as the source spelled it.
func (u *layoutUse) takeFormatEntry() (*qualifierEntry, bool) {
	return u.take(qualifierKey{kind: layoutFormat})
}

// takeIndex removes the `index = N` entry of an output variable,
// Rust naga's QualifierKey::Index.
func (u *layoutUse) takeIndex() (uint32, bool) {
	entry, ok := u.take(qualifierKey{kind: layoutIndex})
	if !ok {
		return 0, false
	}
	return entry.value, true
}

// unusedErrors reports the layout entries no part of the declaration consumed,
// Rust naga's TypeQualifiers::unused_errors for the layout map.
func (u *layoutUse) unusedErrors() {
	for _, key := range u.order {
		if entry, ok := u.entries[key]; ok {
			u.f.addError(errMessage(entry.span, "Unexpected qualifier"))
		}
	}
}

// takeInterpolation reads the interpolation qualifier, Rust naga's
// `qualifiers.interpolation.take()`: a list's later declarators do not see it
// again.
func (q *typeQualifiers) takeInterpolation() (interpolationQualifier, bool) {
	if q.Interpolation == interpolationNone {
		return interpolationNone, false
	}
	interpolation := q.Interpolation
	q.Interpolation = interpolationNone
	return interpolation, true
}

// takeSampling reads the sampling qualifier.
func (q *typeQualifiers) takeSampling() (samplingQualifier, bool) {
	if q.Sampling == samplingNone {
		return samplingNone, false
	}
	sampling := q.Sampling
	q.Sampling = samplingNone
	return sampling, true
}

// takeStorageAccess reads the memory qualifiers' access, if any were written.
func (q *typeQualifiers) takeStorageAccess() (ir.StorageAccess, bool) {
	if !q.HasStorageAccess {
		return 0, false
	}
	access := q.StorageAccess
	q.HasStorageAccess = false
	return access, true
}

// takeInvariant reads the invariant qualifier.
func (q *typeQualifiers) takeInvariant() bool {
	if !q.Invariant {
		return false
	}
	q.Invariant = false
	return true
}

// unusedQualifierErrors reports the qualifiers no part of the declaration
// consumed, Rust naga's TypeQualifiers::unused_errors (ast.rs:224-265).
func (f *Frontend) unusedQualifierErrors(q *typeQualifiers) {
	if q.Invariant {
		f.addError(errMessage(q.InvariantSpan, "Invariant qualifier can only be used in in/out variables"))
	}
	if q.Interpolation != interpolationNone {
		f.addError(errMessage(q.InterpolationSpan, "Interpolation qualifiers can only be used in in/out variables"))
	}
	if q.Sampling != samplingNone {
		f.addError(errMessage(q.SamplingSpan, "Sampling qualifiers can only be used in in/out variables"))
	}
	if q.HasStorageAccess {
		f.addError(errMessage(q.StorageAccessSpan, "Memory qualifiers can only be used in storage variables"))
	}
}

// nextIOLocation returns the location an in or out variable takes, Rust
// naga's location layout qualifier with our fix for its default-0 collision
// (design §10): an explicit location is used and reserved, and a variable
// without one takes the lowest location no other IO variable of its class
// uses.
func (f *Frontend) nextIOLocation(class ioClass, used *layoutUse) uint32 {
	if f.ioLocations[class] == nil {
		f.ioLocations[class] = map[uint32]bool{}
	}
	locations := f.ioLocations[class]
	if location, ok := used.takeUint("location"); ok {
		locations[location] = true
		return location
	}
	for location := uint32(0); ; location++ {
		if !locations[location] {
			locations[location] = true
			return location
		}
	}
}

// applyDefaultInterpolation gives a float location binding Rust naga's default
// interpolation (front/interpolator.rs:39-53): perspective-correct at the
// pixel center. An integer binding keeps none, which the validator then
// requires to be flat; that is the GLSL rule, not a gap.
func applyDefaultInterpolation(binding *ir.LocationBinding, inner ir.TypeInner) {
	if binding.Interpolation != nil {
		return
	}
	scalar, ok := scalarComponents(inner)
	if !ok || scalar.Kind != ir.ScalarFloat {
		return
	}
	interpolation := ir.InterpolationPerspective
	sampling := ir.SamplingCenter
	binding.Interpolation = &interpolation
	binding.Sampling = &sampling
}

// interpolationBinding converts a GLSL interpolation qualifier, Rust naga's
// into_interpolation.
func interpolationBinding(q interpolationQualifier) *ir.Interpolation {
	var interpolation ir.Interpolation
	switch q {
	case interpolationFlat:
		interpolation = ir.InterpolationFlat
	case interpolationSmooth:
		interpolation = ir.InterpolationPerspective
	case interpolationNoPerspective:
		interpolation = ir.InterpolationLinear
	default:
		return nil
	}
	return &interpolation
}

// samplingBinding converts a GLSL sampling qualifier.
func samplingBinding(q samplingQualifier) *ir.Sampling {
	var sampling ir.Sampling
	switch q {
	case samplingCentroid:
		sampling = ir.SamplingCentroid
	case samplingSample:
		sampling = ir.SamplingSample
	default:
		return nil
	}
	return &sampling
}

// globalOrConstant is what a module-scope declaration produced, Rust naga's
// GlobalOrConstant (variables.rs:34-38).
type globalOrConstant struct {
	kind     globalLookupKind
	handle   ir.GlobalVariableHandle
	constant ir.ConstantHandle
	override ir.OverrideHandle
}

// opaqueUniformDeclaration reports whether a declaration's type is an opaque
// uniform — a plain sampler or an image — the only module-scope uniforms GL 3.30
// core and ES 3.00 allow outside a block. A combined image sampler never
// reaches this check: it has its own declaration path.
func opaqueUniformDeclaration(spec typeSpec) bool {
	if spec.Builtin == nil {
		return false
	}
	switch spec.Builtin.Inner.(type) {
	case ir.SamplerType, ir.ImageType:
		return true
	default:
		return false
	}
}

// imageClassPhrase names the scalar class of an image type's declared class in
// glslang's words, for the refusal of a format the class cannot carry:
// "'r32ui' : does not apply to signed integer images".
func imageClassPhrase(kind ir.ScalarKind) string {
	switch kind {
	case ir.ScalarSint:
		return "signed integer"
	case ir.ScalarUint:
		return "unsigned integer"
	default:
		return "floating point"
	}
}

// addGlobalVar declares one module-scope variable, constant or override, Rust
// naga's Frontend::add_global_var (variables.rs:415-663). sym and name are nil
// and empty for an instance-less interface block, whose members are the
// names that matter. bindingName is the source name ParseOptions.
// ResourceBindings keys on: the declared variable name, or an interface
// block's type name when it has no instance name.
func (f *Frontend) addGlobalVar(c *Context, used *layoutUse, qualifiers *typeQualifiers, ty ir.TypeHandle, name, bindingName string, sym *glslSymbol, init *ir.ExpressionHandle, span ir.Span) (globalOrConstant, *frontError) {
	var (
		out    globalOrConstant
		lookup globalLookup
	)
	// Every named module-scope declaration claims its name before it builds
	// anything, so a reserved or repeated one is refused with nothing left
	// behind (module_names.go). An instance-less interface block has no name of
	// its own; its members are the names that matter and are registered by
	// lowerBlockDeclaration.
	if name != "" {
		if err := f.declareModuleName(name, span); err != nil {
			return out, err
		}
	}
	switch storage := qualifiers.Storage; storage {
	case storageInput, storageOutput:
		input := storage == storageInput
		// GL 3.30 core and ES 3.00 give an explicit location only to vertex
		// shader inputs and fragment shader outputs; a varying location is
		// GLSL 4.10 / ES 3.10 (design §15, dialect.go).
		if entry, ok := used.peek("location"); ok && !f.dialect.allowsIOLocation(f.meta.stage, storage) {
			f.addError(errMessage(entry.span, f.dialect.unsupported(
				"location qualifier on "+ioDirection(storage),
				" on a "+stageAdjective(f.meta.stage)+" "+ioDirection(storage)+
					" ("+f.dialect.ioLocationRequirement(f.meta.stage, storage)+")")))
		}
		location := f.nextIOLocation(ioClassOf(storage), used)

		var interpolation *ir.Interpolation
		if q, ok := qualifiers.takeInterpolation(); ok {
			interpolation = interpolationBinding(q)
		}
		var sampling *ir.Sampling
		if q, ok := qualifiers.takeSampling(); ok {
			sampling = samplingBinding(q)
		}

		handle := ir.GlobalVariableHandle(len(f.mod.GlobalVariables))
		f.mod.GlobalVariables = append(f.mod.GlobalVariables, ir.GlobalVariable{
			Name:        name,
			NamePresent: false,
			Space:       ir.SpacePrivate,
			Type:        ty,
			InitExpr:    init,
		})

		binding := ir.LocationBinding{
			Location:      location,
			Interpolation: interpolation,
			Sampling:      sampling,
		}
		if blendSrc, ok := used.takeIndex(); ok {
			binding.BlendSrc = &blendSrc
		}
		applyDefaultInterpolation(&binding, f.mod.Types[ty].Inner)

		idx := len(f.entryArgs)
		f.entryArgs = append(f.entryArgs, entryArg{
			name:    name,
			binding: binding,
			handle:  handle,
			storage: storage,
		})

		out = globalOrConstant{kind: lookupVariable, handle: handle}
		lookup = globalLookup{kind: lookupVariable, variable: handle, entryArg: idx, mutable: !input}

	case storageConst:
		if id, ok := used.takeUint("constant_id"); ok {
			// A specialization constant becomes an IR override; its id must
			// fit the IR's u16 (Rust's SpecIdTooHigh).
			var ptr *uint16
			if id > 0xffff {
				f.addError(errSemantic(span, "constant_id value %d is too high (maximum is %d)", id, 0xffff))
			} else {
				v := uint16(id)
				ptr = &v
			}
			h := ir.OverrideHandle(len(f.mod.Overrides))
			f.mod.Overrides = append(f.mod.Overrides, ir.Override{Name: name, ID: ptr, Ty: ty, Init: init})
			out = globalOrConstant{kind: lookupOverride, override: h}
			lookup = globalLookup{kind: lookupOverride, override: h, ty: ty}
			break
		}
		if init == nil {
			return out, errMessage(span, "const values must have an initializer")
		}
		h := ir.ConstantHandle(len(f.mod.Constants))
		f.mod.Constants = append(f.mod.Constants, ir.Constant{Name: name, Type: ty, Init: *init})
		out = globalOrConstant{kind: lookupConstant, constant: h}
		lookup = globalLookup{kind: lookupConstant, constant: h, ty: ty}

	default:
		// The storage qualifier is the address space (Rust's
		// StorageQualifier::AddressSpace, ast.rs:344-350): a uniform block is
		// uniform, a buffer block storage and a shared one workgroup, each
		// narrowed to handle space for an image or a sampler.
		space := ir.SpacePrivate
		switch storage {
		case storageUniform:
			space = ir.SpaceUniform
			switch inner := f.mod.Types[ty].Inner.(type) {
			case ir.ImageType:
				if inner.Class == ir.ImageClassStorage {
					access := inner.StorageAccess
					if allowed, ok := qualifiers.takeStorageAccess(); ok {
						access = allowed
					}
					if access == 0 {
						// `writeonly readonly` cancels out; the empty set is
						// the IR's "unset" (ADR-010 E2), which validation
						// refuses, and the GLSL spec gives the combination no
						// meaning — treat it as unrestricted.
						access = ir.StorageAccessLoad | ir.StorageAccessStore
					}
					format := inner.StorageFormat
					// The declared class fixes the sign of the type the image
					// loads and stores, and the format fixes it too, so the two
					// must agree — GLSL 4.60 §4.4.8: "It is a compile-time error
					// to declare an image variable where the format qualifier
					// does not match the image variable type", with iimage
					// taking the int-image-format-qualifiers, uimage the
					// uint ones and image the float ones. A mismatch used to be
					// lowered silently — the placeholder format was replaced and
					// the declared class dropped (P2-GLSLIN-023) — where glslang
					// refuses at the qualifier: "'r32ui' : does not apply to
					// signed integer images". Rust naga replaces the format
					// without comparing the two (its add_global_var's storage
					// image arm, variables.rs:563-601), so the refusal is a
					// deliberate difference from Rust where the spec decides.
					if entry, ok := used.takeFormatEntry(); ok {
						format = entry.format
						if entry.format.ScalarKind() != inner.SampledKind {
							f.addError(errSemantic(entry.span, "'%s' : does not apply to %s images",
								entry.formatName, imageClassPhrase(inner.SampledKind)))
						}
					} else {
						f.addError(errMessage(span, "image types require a format layout qualifier"))
					}
					ty = f.registerType(ir.ImageType{
						Dim:           inner.Dim,
						Arrayed:       inner.Arrayed,
						Class:         ir.ImageClassStorage,
						StorageFormat: format,
						StorageAccess: access,
					})
				}
				space = ir.SpaceHandle
			case ir.SamplerType:
				space = ir.SpaceHandle
			default:
				if entry, ok := used.peek("push_constant"); ok {
					used.take(qualifierKey{kind: layoutNamed, name: "push_constant"})
					switch {
					case entry.hasValue:
						// The qualifier takes no value; Rust naga's text for it
						// (none_layout_qualifier, ast.rs:314-337), and the space
						// must not become immediate on a malformed qualifier.
						f.addError(errMessage(entry.span, "Qualifier doesn't expect a value"))
					case f.dialect.supportsPushConstants():
						space = ir.SpaceImmediate
					default:
						f.addError(errMessage(entry.span, "'push_constant' : only allowed when using GLSL for Vulkan"))
					}
				}
			}
		case storageBuffer:
			space = ir.SpaceStorage
		case storageShared:
			space = ir.SpaceWorkGroup
		}

		var binding *ir.ResourceBinding
		if space == ir.SpaceUniform || space == ir.SpaceStorage || space == ir.SpaceHandle {
			binding = f.resourceBinding(bindingName, used, span)
		}

		handle := ir.GlobalVariableHandle(len(f.mod.GlobalVariables))
		f.mod.GlobalVariables = append(f.mod.GlobalVariables, ir.GlobalVariable{
			Name:     name,
			Space:    space,
			Binding:  binding,
			Type:     ty,
			InitExpr: init,
			Access:   storageAccessOf(space, qualifiers),
		})
		out = globalOrConstant{kind: lookupVariable, handle: handle}
		lookup = globalLookup{kind: lookupVariable, variable: handle, mutable: true, entryArg: -1}
	}

	if sym != nil {
		if err := c.addGlobal(namedLookup{name: name, sym: sym, lookup: lookup}); err != nil {
			return out, err
		}
		f.globalVars = append(f.globalVars, namedLookup{name: name, sym: sym, lookup: lookup})
	}

	f.unusedQualifierErrors(qualifiers)
	used.unusedErrors()
	return out, nil
}

// storageAccessOf is the access of a storage variable, Rust naga's
// AddressSpace::Storage { access }: read and write unless memory qualifiers
// narrow it.
func storageAccessOf(space ir.AddressSpace, qualifiers *typeQualifiers) ir.StorageAccess {
	if space != ir.SpaceStorage {
		return 0
	}
	if access, ok := qualifiers.takeStorageAccess(); ok && access != 0 {
		return access
	}
	// No qualifier, or one whose restrictions cancel: unrestricted, never the
	// empty set that validation refuses (ADR-010 E2).
	return ir.StorageAccessLoad | ir.StorageAccessStore
}

// ioClassOf maps an IO storage qualifier to its location class.
func ioClassOf(storage storageQualifier) ioClass {
	if storage == storageInput {
		return ioInput
	}
	return ioOutput
}

// lowerGlobalVarList lowers one module-scope variable declaration, Rust
// naga's parse_init_declarator_list for an external declaration
// (declarations.rs:167-291). A combined sampler takes the declaration's own
// path, because the IR has no type for it (design §11).
func (f *Frontend) lowerGlobalVarList(c *Context, d *declVarList) *frontError {
	used := f.takeLayoutQualifiers(&d.Qualifiers)

	// A combined sampler has its own declaration path: the IR has no type for
	// it (design §11).
	if spec := d.Typ.Builtin; spec != nil && spec.Combined {
		for i := range d.Declarators {
			if err := f.addCombinedSampler(c, used, &d.Qualifiers, spec, &d.Declarators[i], d.Typ.Span); err != nil {
				return err
			}
		}
		return nil
	}

	// A plain uniform — a module-scope `uniform` outside any block that is not
	// an opaque resource — has no IR resource of its own. GL 3.30 core and
	// ES 3.00 give it the default uniform block, which the frontend
	// synthesizes (ADR-013, default_block.go); Vulkan GLSL has no such block,
	// and neither has the IR, so the declaration stays refused there. Rust naga
	// refuses it in every dialect ("uniform/buffer blocks require
	// layout(binding=X)"); glslang: "'non-opaque uniforms outside a block' :
	// not allowed when using GLSL for Vulkan".
	plainUniform := d.Qualifiers.Storage == storageUniform && !opaqueUniformDeclaration(d.Typ)
	if plainUniform && f.dialect.vulkan() {
		return errMessage(d.Typ.Span,
			"plain uniform variables are not allowed in Vulkan GLSL: 'non-opaque uniforms outside a block'; declare the uniform inside a uniform block")
	}

	base, err := f.resolveTypeSpec(&d.Typ, structLayoutStd140)
	if err != nil {
		return err
	}
	if plainUniform {
		return f.lowerPlainUniformList(c, used, d, base)
	}
	isConst := d.Qualifiers.Storage == storageConst

	for i := range d.Declarators {
		decl := &d.Declarators[i]
		// GLSL 4.60 §4.3.8: "Variables declared as shared may not have
		// initializers and their contents are undefined at the beginning of
		// shader execution." glslang refuses the initializer ("'shared' :
		// initializer can only be a null initializer ('{}')", and refuses the
		// null one too), and keeping it would promise a starting value the GL
		// model never gives (P2-GLSLIN-024). Refused here, before the
		// initializer is lowered, so a variable that is also accessed with
		// atomic memory functions reports this one error and not the atomic
		// upgrade's. Rust naga carries the initializer into the global's init
		// without examining the storage class, so this refusal is another
		// deliberate difference from Rust where the spec decides.
		if d.Qualifiers.Storage == storageShared && decl.Init != nil {
			return errMessage(decl.Span,
				"'shared' : a shared variable cannot have an initializer (GLSL 4.60 §4.3.8)")
		}
		ty, err := f.resolveTypeArrays(base, decl.Arrays)
		if err != nil {
			return err
		}

		var init *ir.ExpressionHandle
		if decl.Init != nil {
			if isConst {
				// A module-scope const is a const-expression: it is lowered
				// in the shared const context over the module's global arena
				// (declarations.rs:221-243).
				constCtx, cerr := f.constContext()
				if cerr != nil {
					return cerr
				}
				h, ierr := constCtx.lowerInitializer(&decl.Init.Arena, decl.Init.Root, ty)
				if ierr != nil {
					return ierr
				}
				init = &h
			} else {
				h, ierr := c.lowerInitializer(&decl.Init.Arena, decl.Init.Root, ty)
				if ierr != nil {
					return ierr
				}
				if lifted, lerr := c.liftUpConstExpression(h, decl.Span); lerr == nil {
					init = &lifted
				}
				// A module-scope initializer that is not a const-expression
				// is dropped, as Rust's lift_up_const_expression().ok() does
				// (declarations.rs:250-253).
			}
		}

		if _, err := f.addGlobalVar(c, used, &d.Qualifiers, ty, decl.Name, decl.Name, decl.Sym, init, decl.Span); err != nil {
			return err
		}
	}
	return nil
}

// addCombinedSampler declares a combined image sampler: the declaration
// becomes an image global on its declared binding and a sampler global of its
// own, and expressions naming it carry the image while ctx.samplers maps that
// expression to the sampler (design §11; Rust has no such declaration,
// gfx-rs/wgpu#4342).
func (f *Frontend) addCombinedSampler(c *Context, used *layoutUse, qualifiers *typeQualifiers, spec *typeName, decl *declarator, span ir.Span) *frontError {
	if len(decl.Arrays) != 0 {
		return errMessage(decl.Span, "Not implemented: array of combined image samplers")
	}
	if qualifiers.Storage != storageUniform {
		return errMessage(span, "combined image sampler types are only supported for module-scope uniform variables")
	}
	if f.combinedPolicy == frontend.CombinedImageSamplerReject {
		return errMessage(span, "combined image samplers are refused by CombinedImageSamplerReject")
	}
	// The declaration's name is a module-scope name like any other: reserved and
	// repeated ones are refused here (module_names.go).
	if err := f.declareModuleName(decl.Name, decl.Span); err != nil {
		return err
	}
	// The image takes the declared layout(binding) of a Vulkan GLSL source, or
	// is placed by the GL/ES binding model under the source name (design §11);
	// the sampler's binding is resolved once every module-scope declaration is
	// known (resolveCombinedSamplerBindings).
	imageBinding := *f.resourceBinding(decl.Name, used, span)

	image := ir.ImageType{
		Dim:          spec.Dim,
		Arrayed:      spec.Arrayed,
		Multisampled: spec.Multi,
		Class:        ir.ImageClassSampled,
		SampledKind:  spec.Kind,
	}
	if spec.Comparison {
		image.Class = ir.ImageClassDepth
	}
	imageTy := f.registerType(image)
	samplerTy := f.registerType(ir.SamplerType{Comparison: spec.Comparison})

	imageHandle := ir.GlobalVariableHandle(len(f.mod.GlobalVariables))
	f.mod.GlobalVariables = append(f.mod.GlobalVariables, ir.GlobalVariable{
		Name:    decl.Name,
		Space:   ir.SpaceHandle,
		Binding: &imageBinding,
		Type:    imageTy,
	})
	// The sampler's binding is resolved once every module-scope declaration is
	// known (resolveCombinedSamplerBindings): the next free slot depends on the
	// whole module, not on the declarations that happen to precede this one.
	samplerHandle := ir.GlobalVariableHandle(len(f.mod.GlobalVariables))
	f.mod.GlobalVariables = append(f.mod.GlobalVariables, ir.GlobalVariable{
		Name:  decl.Name + "_sampler",
		Space: ir.SpaceHandle,
		Type:  samplerTy,
	})

	f.combinedPairs[imageHandle] = samplerHandle
	imageLookup := globalLookup{kind: lookupVariable, variable: imageHandle, mutable: true, entryArg: -1}
	if decl.Sym != nil {
		if err := c.addGlobal(namedLookup{name: decl.Name, sym: decl.Sym, lookup: imageLookup}); err != nil {
			return err
		}
		f.globalVars = append(f.globalVars, namedLookup{name: decl.Name, sym: decl.Sym, lookup: imageLookup})
	}
	samplerExpr, err := c.addExpression(ir.ExprGlobalVariable{Variable: samplerHandle}, span)
	if err != nil {
		return err
	}
	if ref := c.vars[decl.Sym]; ref != nil {
		c.samplers[ref.expr] = samplerExpr
	}

	f.combinedSamplers = append(f.combinedSamplers, combinedSamplerInfo{
		name:         decl.Name,
		imageBinding: imageBinding,
	})

	f.unusedQualifierErrors(qualifiers)
	used.unusedErrors()
	return nil
}

// combinedSamplerSpans returns the source span of each combined image sampler
// declaration by its variable name, so the binding resolution, which runs after
// the declarations are lowered, can report a refused mapping where the resource
// was declared.
func combinedSamplerSpans(decls []astDecl) map[string]ir.Span {
	spans := map[string]ir.Span{}
	for i := range decls {
		d, ok := decls[i].Kind.(declVarList)
		if !ok || d.Typ.Builtin == nil || !d.Typ.Builtin.Combined {
			continue
		}
		for j := range d.Declarators {
			spans[d.Declarators[j].Name] = d.Declarators[j].Span
		}
	}
	return spans
}

// splitSamplerHandle returns the sampler global the image global of a split
// combined sampler was split into.
func (f *Frontend) splitSamplerHandle(split *combinedSamplerInfo) (ir.GlobalVariableHandle, bool) {
	for i := range f.mod.GlobalVariables {
		g := &f.mod.GlobalVariables[i]
		if g.Name != split.name || g.Binding == nil || *g.Binding != split.imageBinding {
			continue
		}
		sampler, ok := f.combinedPairs[ir.GlobalVariableHandle(i)]
		return sampler, ok
	}
	return 0, false
}

// resolveCombinedSamplerBindings gives every split sampler its binding once all
// module-scope declarations are known: one counter per group over every bound
// resource, in source order (design §11). A ParseOptions.SamplerBindings entry
// wins over the next free slot, and is reserved like any other binding; a
// mapping onto a slot another resource of the module already holds is an error,
// because the caller asked for two globals to share one.
//
// The allocation waits for the whole module because the next free slot depends
// on it: allocating at the declaration let a later resource take the slot, and
// the resulting module failed validation (P0-GLSLIN-003). The SPIR-V frontend
// resolves its splits in the same post-pass
// (spirv/internal/front/combined_sampler.go:78-111); Rust naga has no combined
// samplers at all (gfx-rs/wgpu#4342).
func (f *Frontend) resolveCombinedSamplerBindings(decls []astDecl) *frontError {
	if len(f.combinedSamplers) == 0 {
		return nil
	}
	spans := combinedSamplerSpans(decls)
	for i := range f.combinedSamplers {
		split := &f.combinedSamplers[i]
		span := spans[split.name]
		samplerHandle, ok := f.splitSamplerHandle(split)
		if !ok {
			return errInternal("split combined sampler without a sampler global", span)
		}
		group := split.imageBinding.Group
		var samplerBinding ir.ResourceBinding
		if supplied, mapped := f.samplerBindings[resourceKey{group: group, binding: split.imageBinding.Binding}]; mapped {
			target := resourceKey{group: group, binding: supplied}
			if f.bindings.taken(target) {
				return errSemantic(span,
					"the sampler of combined image sampler (group %d, binding %d) is mapped to (group %d, binding %d), which another resource of the module already uses",
					group, split.imageBinding.Binding, group, supplied)
			}
			samplerBinding = ir.ResourceBinding{Group: group, Binding: supplied}
			f.bindings.claim(target)
		} else {
			samplerBinding = ir.ResourceBinding{Group: group, Binding: f.bindings.nextFree(group)}
		}
		f.mod.GlobalVariables[samplerHandle].Binding = &samplerBinding
		split.samplerBinding = samplerBinding
	}
	return nil
}

// resourceBinding places one bound module-scope resource, design §11 (R2).
// A Vulkan GLSL source declares its layout(binding)/layout(set) and those win;
// GL 3.30 core and ES 3.00 have no such qualifier, so the resource takes the
// binding ParseOptions.ResourceBindings gives its source name, else the next
// free slot of the module's one counter (bindings.go).
func (f *Frontend) resourceBinding(bindingName string, used *layoutUse, span ir.Span) *ir.ResourceBinding {
	if f.dialect.hasLayoutBindings() {
		value, ok := used.takeUint("binding")
		if !ok {
			// Rust naga's text for a resource the source does not bind.
			f.addError(errMessage(span, "uniform/buffer blocks require layout(binding=X)"))
		}
		group, _ := used.takeUint("set")
		binding := ir.ResourceBinding{Group: group, Binding: value}
		f.bindings.markUsed(resourceKey{group: group, binding: value})
		return &binding
	}
	f.rejectResourceLayoutQualifiers(used)
	binding := f.autoResourceBinding(bindingName, span)
	return &binding
}

// rejectResourceLayoutQualifiers reports the layout(binding) and layout(set)
// qualifiers a GL 3.30 core or ES 3.00 declaration cannot carry, consuming them
// so the declaration does not report them a second time as unexpected. Vulkan
// GLSL never reaches it: it takes them as the resource's declared binding.
func (f *Frontend) rejectResourceLayoutQualifiers(used *layoutUse) {
	if entry, ok := used.take(qualifierKey{kind: layoutNamed, name: "binding"}); ok {
		f.addError(errMessage(entry.span, f.dialect.unsupported(
			"binding", " (requires GLSL 4.20 or GL_ARB_shading_language_420pack)")))
	}
	if entry, ok := used.take(qualifierKey{kind: layoutNamed, name: "set"}); ok {
		f.addError(errMessage(entry.span, "'set' : only allowed when using GLSL for Vulkan"))
	}
}

// autoResourceBinding places a resource the source cannot bind itself: the slot
// ParseOptions.ResourceBindings gives its source name, else the next free slot
// of group 0 (bindings.go). A supplied entry that claims an occupied slot is
// reported, because the caller asked for two globals to share one.
func (f *Frontend) autoResourceBinding(bindingName string, span ir.Span) ir.ResourceBinding {
	binding, reason := f.bindings.forResource(bindingName, 0)
	if reason != "" {
		f.addError(errSemantic(span, "%s", reason))
	}
	return binding
}

// ioDirection names an in or out variable in a diagnostic.
func ioDirection(storage storageQualifier) string {
	if storage == storageInput {
		return "input"
	}
	return "output"
}

// stageAdjective names a stage as a shader in a diagnostic: "vertex shader".
func stageAdjective(stage ir.ShaderStage) string {
	switch stage {
	case ir.StageVertex:
		return "vertex shader"
	case ir.StageFragment:
		return "fragment shader"
	case ir.StageCompute:
		return "compute shader"
	default:
		return stage.String() + " shader"
	}
}

// lowerBlockDeclaration lowers an interface block, Rust naga's
// parse_block_declaration (parser/declarations.rs:534-622): the block type is
// laid out with the declared layout (std140 by default, std430 for a storage
// block), the instance name declares the global, and the member names resolve
// to the block's members.
func (f *Frontend) lowerBlockDeclaration(c *Context, d *declBlock) *frontError {
	used := f.takeLayoutQualifiers(&d.Qualifiers)
	layout, ok := used.takeStructure()
	if !ok {
		layout = structLayoutStd140
		if d.Qualifiers.Storage == storageBuffer {
			layout = structLayoutStd430
		}
	}
	inner, err := f.layoutStructMembers(d.Members, layout)
	if err != nil {
		return err
	}
	ty := f.registerNamedType(d.Name, inner)
	ty, err = f.resolveTypeArrays(ty, d.Arrays)
	if err != nil {
		return err
	}
	// The ResourceBindings key of an interface block is its instance name, or
	// its type name when it has none (the block is then addressed by members).
	bindingName := d.Instance
	if bindingName == "" {
		bindingName = d.Name
	}
	global, err := f.addGlobalVar(c, used, &d.Qualifiers, ty, d.Instance, bindingName, d.InstanceSym, nil, d.Span)
	if err != nil {
		return err
	}

	for i := range d.Members {
		member := &d.Members[i]
		// A member is a variable declaration, so the reserved prefix is refused
		// whether or not the block has an instance name — glslang refuses
		// `uniform Blk { float gl_z; } blk;` as well as the instance-less form
		// (probed 2026-11-16). The redefinition table only takes the member of
		// an instance-less block: GLSL 4.60 §4.3.9 scopes the members of a named
		// block to its instance ("it puts all the members inside a scope within
		// its own name space"), and glslang accepts them repeating a global, a
		// sibling block's member, or even the instance's own name — so only the
		// instance-less member is a module-scope name.
		if d.Instance == "" {
			if err := f.declareModuleName(member.Name, member.Span); err != nil {
				return err
			}
		} else if err := refuseReservedIdentifier(member.Name, member.Span); err != nil {
			return err
		}
		if member.Sym == nil {
			continue
		}
		lookup := globalLookup{entryArg: -1}
		memberTy := inner.Members[i].Type
		switch global.kind {
		case lookupVariable:
			lookup.kind = lookupBlockSelect
			lookup.variable = global.handle
			lookup.index = uint32(i)
			lookup.mutable = true
		case lookupConstant:
			lookup.kind = lookupConstant
			lookup.constant = global.constant
			lookup.ty = memberTy
		case lookupOverride:
			lookup.kind = lookupOverride
			lookup.override = global.override
			lookup.ty = memberTy
		}
		if err := c.addGlobal(namedLookup{name: member.Name, sym: member.Sym, lookup: lookup}); err != nil {
			return err
		}
		f.globalVars = append(f.globalVars, namedLookup{name: member.Name, sym: member.Sym, lookup: lookup})
	}
	return nil
}

// lowerQualifierDeclaration reads the module-level layout settings a
// qualifier-only declaration carries, Rust naga's Semicolon arm of
// parse_declaration (declarations.rs:453-476): the compute workgroup size and
// the early fragment tests request.
func (f *Frontend) lowerQualifierDeclaration(d *declQualifiers) {
	used := f.takeLayoutQualifiers(&d.Qualifiers)
	// The workgroup size exists in the compute stage only; glslang, for a
	// `layout(local_size_x = 1) in;` declaration outside compute: "'local_size_x'
	// : there is no such layout identifier for this stage taking an assigned
	// value". The compute stage itself is Vulkan GLSL only and refused earlier
	// for GL 3.30 and ES 3.00 (parse.go).
	for _, name := range [...]string{"local_size_x", "local_size_y", "local_size_z"} {
		entry, ok := used.peek(name)
		if ok && f.meta.stage != ir.StageCompute {
			f.addError(errMessage(entry.span, "'"+name+"' : there is no such layout identifier for this stage taking an assigned value"))
		}
	}
	if value, ok := used.takeUint("local_size_x"); ok {
		f.meta.workgroupSize[0] = value
	}
	if value, ok := used.takeUint("local_size_y"); ok {
		f.meta.workgroupSize[1] = value
	}
	if value, ok := used.takeUint("local_size_z"); ok {
		f.meta.workgroupSize[2] = value
	}
	if used.takeNone("early_fragment_tests") {
		f.meta.earlyFragmentTest = true
	}
	f.unusedQualifierErrors(&d.Qualifiers)
	used.unusedErrors()
}

// lowerPrecisionDeclaration checks a default precision statement. The precision
// hints themselves are not in the IR (Rust's TODO), but the statement's type is
// validated: glslang accepts float, int and sampler types and refuses the rest
// with "cannot apply precision statement to this type; use 'float', 'int' or a
// sampler type" (GLSL 4.60 §4.5.4, GLSL ES 3.00 §4.5.4; prec_vec.frag,
// prec_mat.frag, prec_bool.frag and prec_uint.frag fail, prec_sampler.frag
// passes). A combined image sampler is a sampler type here — `precision lowp
// sampler2D;` is the ES 3.00 declaration es300/precision-sampler-type.frag
// needs — so it is classified from the type name instead of resolved into IR.
func (f *Frontend) lowerPrecisionDeclaration(d *declPrecision) {
	if !precisionStatementTypeAllowed(d.Typ) {
		f.addError(errMessage(d.Typ.Span, "Precision statement can only work on floats, ints and sampler types"))
	}
}

// precisionStatementTypeAllowed reports whether a precision statement may name
// the type of spec: a float or signed integer scalar (of any width), a sampler
// type, or a combined image sampler. Anything else — vectors, matrices, bool,
// uint, structs, arrays — is refused, as glslang refuses it.
func precisionStatementTypeAllowed(spec typeSpec) bool {
	if spec.Builtin == nil {
		return false
	}
	if spec.Builtin.Combined {
		return len(spec.Arrays) == 0
	}
	if len(spec.Arrays) != 0 {
		return false
	}
	switch inner := spec.Builtin.Inner.(type) {
	case ir.ScalarType:
		return inner.Kind == ir.ScalarFloat || inner.Kind == ir.ScalarSint
	case ir.SamplerType:
		return true
	default:
		return false
	}
}

func (p *parser) builtinSymbol(name string) *glslSymbol {
	if !builtinVariableNames[name] {
		return nil
	}
	if sym := p.symbols.lookup(name); sym != nil {
		return sym
	}
	sym := &glslSymbol{Kind: symBuiltinVar, Name: name}
	// The root scope holds builtin declarations, as Rust's add_root does: a use
	// inside a block must not leave the declaration behind when the block ends,
	// and later uses find the same symbol instead of a new one.
	p.symbols.declareGlobal(sym)
	return sym
}
