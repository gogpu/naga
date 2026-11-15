package front

// This file holds the state of one frontend run: Rust naga's Frontend
// (front/glsl/mod.rs:34-105). The module being built, its type registry and
// the declared names are shared by every lowering context of a run; the
// per-function state lives in Context (context.go).

import (
	"github.com/gogpu/naga/frontend"
	"github.com/gogpu/naga/internal/registry"
	"github.com/gogpu/naga/ir"
)

// shaderMeta is what the shader says about the pipeline it feeds, Rust naga's
// ShaderMetadata (mod.rs:74-105) reduced to what lowering reads: the stage,
// the compute workgroup size and the early-fragment-tests request.
type shaderMeta struct {
	stage             ir.ShaderStage
	workgroupSize     [3]uint32
	earlyFragmentTest bool
}

// Frontend is the state of one GLSL frontend run.
type Frontend struct {
	mod      *ir.Module
	registry *registry.TypeRegistry
	layouter ir.Layouter

	meta shaderMeta

	// globalVars are the module-scope names in declaration order, Rust's
	// frontend.global_variables. A new context copies them into its symbol
	// table when it starts (context.rs:121-123), so a name declared after a
	// function still resolves inside it at lowering time.
	globalVars []namedLookup

	// types maps a declared struct or interface block name to its type.
	types map[string]ir.TypeHandle

	// functions holds the declared functions and builtins by name, Rust's
	// frontend.lookup_function (functions.go fills it).
	functions map[string]*functionDeclaration

	// entryArgs are the IO variables the entry-point wrapper wires, in
	// declaration order (Rust's frontend.entry_args).
	entryArgs []entryArg

	// ioLocations holds the locations given to module-scope in and out
	// variables so far, per IO class: an unlocated variable takes the next
	// free one instead of Rust's colliding 0 (design §10).
	ioLocations [ioClassCount]map[uint32]bool

	// usedBindings holds the (group, binding) pairs of bound resources, so
	// automatic placement picks a free one.
	usedBindings map[resourceKey]bool

	// samplerBindings places the sampler of a split combined sampler, keyed
	// by the (group, binding) of its image, as frontend.SamplerBindings does
	// for the SPIR-V frontend.
	samplerBindings map[resourceKey]uint32

	// combinedSamplers reports each split combined sampler in source order.
	combinedSamplers []combinedSamplerInfo

	// combinedPairs maps a split combined sampler's image global to its
	// sampler global. Every context rebuilds the pair in its own arena on the
	// first use (Context.addGlobal fills ctx.samplers), so a texture builtin
	// in a function body finds the sampler expression.
	combinedPairs map[ir.GlobalVariableHandle]ir.GlobalVariableHandle

	// combinedPolicy selects the treatment of combined image samplers; the
	// zero value splits them.
	combinedPolicy frontend.CombinedImageSamplerPolicy

	// constCtx is the context that lowers module-scope constant expressions.
	// One context serves the whole module, so its typifier types each new
	// global expression once instead of re-typing the arena for every
	// constant evaluation (review finding P1-5: a shader with thousands of
	// switch cases over a few constants grew the module arena without bound
	// and cost O(n^2)).
	constCtx *Context

	// errors holds the semantic errors lowering recovers from, in occurrence
	// order, as Rust's frontend.errors.
	errors []frontError
}

// namedLookup is one module-scope name and what it resolves to. sym is the
// symbol the parser declared for the name; lowering attaches the IR
// reference to it, and every reference the parser built points at it.
type namedLookup struct {
	name   string
	sym    *glslSymbol
	lookup globalLookup
}

// globalLookupKind tells what a module-scope name denotes, Rust naga's
// GlobalLookupKind (ast.rs:12-18).
type globalLookupKind uint8

const (
	// lookupVariable is a global variable.
	lookupVariable globalLookupKind = iota
	// lookupConstant is a module constant, with the type it was declared
	// with (which dynamic indexing needs for its proxy local).
	lookupConstant
	// lookupOverride is a pipeline-overridable constant.
	lookupOverride
	// lookupBlockSelect is one member of an instance-less interface block.
	lookupBlockSelect
)

// globalLookup tells how a module-scope name resolves during lowering, Rust
// naga's GlobalLookup (ast.rs:20-26).
type globalLookup struct {
	kind     globalLookupKind
	variable ir.GlobalVariableHandle // lookupVariable, lookupBlockSelect
	constant ir.ConstantHandle       // lookupConstant
	override ir.OverrideHandle       // lookupOverride
	ty       ir.TypeHandle           // lookupConstant, lookupOverride
	index    uint32                  // lookupBlockSelect: the block member
	entryArg int                     // the entry-point argument index, -1 for none
	mutable  bool
}

// parameterInfo is what lowering knows about a declared parameter beyond its
// type, Rust naga's ParameterInfo (ast.rs:26-32).
type parameterInfo struct {
	qualifier parameterQualifier
	// depth marks a parameter the call treats as a depth image instead of a
	// sampled image.
	depth bool
}

// entryArg is one IO variable the entry-point wrapper wires to an argument or
// a result member, Rust naga's EntryArg (ast.rs:91-97).
type entryArg struct {
	name    string
	binding ir.Binding
	handle  ir.GlobalVariableHandle
	storage storageQualifier
}

// functionDeclaration is a name's set of overloads, Rust naga's
// FunctionDeclaration (ast.rs:85-90).
type functionDeclaration struct {
	overloads  []*overload
	variations builtinVariations
}

// overload is one signature a name is callable with, Rust naga's Overload
// (ast.rs:46-62).
type overload struct {
	// parameters are the normalized parameter types: an inout parameter is
	// its value type, not a pointer.
	parameters     []ir.TypeHandle
	parametersInfo []parameterInfo
	kind           overloadKind
	// function is the module function a call to this overload reaches, for
	// an overloadCall.
	function ir.FunctionHandle
	// macro is the builtin expansion of an overloadMacro.
	macro    *macroCall
	defined  bool
	internal bool
	void     bool
}

// overloadKind tells how a call to an overload is implemented, Rust naga's
// FunctionKind (ast.rs:35-43).
type overloadKind uint8

const (
	// overloadCall calls a user-defined function.
	overloadCall overloadKind = iota
	// overloadMacro expands a builtin.
	overloadMacro
)

// builtinVariations tracks which builtin overload variations were generated
// for a name, Rust naga's BuiltinVariations (ast.rs:64-83). Some builtin
// overloads cannot be generated until they are used, because they would
// request capabilities (double, cube-array textures, 2D multisampled arrays)
// an unused shader must not ask for.
type builtinVariations uint32

const (
	// variationStandard is the set of overloads every shader may need.
	variationStandard builtinVariations = 1 << iota
	// variationDouble is the set of overloads that use the double type.
	variationDouble
	// variationCubeTexturesArray is the set of overloads that use
	// samplerCubeArray or samplerCubeArrayShadow.
	variationCubeTexturesArray
	// variationD2MultiTexturesArray is the set of overloads that use
	// sampler2DMSArray.
	variationD2MultiTexturesArray
)

// has reports whether every bit of v is set in b.
func (b builtinVariations) has(v builtinVariations) bool { return b&v == v }

// ioClass distinguishes the in and out classes of module-scope IO variables,
// whose locations are allocated independently.
type ioClass uint8

const (
	ioInput ioClass = iota
	ioOutput
	ioClassCount
)

// resourceKey is the (group, binding) pair a bound resource occupies.
type resourceKey struct {
	group   uint32
	binding uint32
}

// combinedSamplerInfo reports one combined sampler split into an image and a
// sampler, in source order, Rust naga's ParseInfo combined-sampler entry.
type combinedSamplerInfo struct {
	name           string
	imageBinding   ir.ResourceBinding
	samplerBinding ir.ResourceBinding
}

// newFrontend returns the frontend of one module.
func newFrontend(mod *ir.Module) *Frontend {
	return &Frontend{
		mod:             mod,
		registry:        registry.NewTypeRegistryWithCap(32),
		types:           map[string]ir.TypeHandle{},
		functions:       map[string]*functionDeclaration{},
		usedBindings:    map[resourceKey]bool{},
		samplerBindings: map[resourceKey]uint32{},
		combinedPairs:   map[ir.GlobalVariableHandle]ir.GlobalVariableHandle{},
	}
}

// constContext returns the context that lowers module-scope constant
// expressions, creating it on first use.
func (f *Frontend) constContext() (*Context, *frontError) {
	if f.constCtx != nil {
		return f.constCtx, nil
	}
	c, err := newContext(f, true)
	if err != nil {
		return nil, err
	}
	f.constCtx = c
	return c, nil
}

// globalLookup returns what a module-scope name denotes, searching the latest
// declaration first, as the parser's root scope does with a redeclared name.
func (f *Frontend) globalLookup(sym *glslSymbol) (globalLookup, bool) {
	for i := len(f.globalVars) - 1; i >= 0; i-- {
		if f.globalVars[i].sym == sym {
			return f.globalVars[i].lookup, true
		}
	}
	return globalLookup{}, false
}

// addError records a semantic error lowering recovers from.
func (f *Frontend) addError(err *frontError) {
	if err != nil {
		f.errors = append(f.errors, *err)
	}
}

// registerType adds inner to the module's types, deduplicated, and returns
// its handle: Rust naga's UniqueArena insert, which every GLSL type goes
// through (context.rs:337, variables.rs:176).
func (f *Frontend) registerType(inner ir.TypeInner) ir.TypeHandle {
	h := f.registry.GetOrCreate("", inner)
	f.mod.Types = f.registry.GetTypes()
	return h
}

// registerNamedType adds a named type (a struct or block definition) to the
// module and returns its handle. Two different names make two types, as
// Rust's UniqueArena keys on the full Type{name, inner}.
func (f *Frontend) registerNamedType(name string, inner ir.TypeInner) ir.TypeHandle {
	h := f.registry.GetOrCreate(name, inner)
	f.mod.Types = f.registry.GetTypes()
	return h
}

// typeInner returns the type inner of h, or nil for a handle outside the
// arena.
func (f *Frontend) typeInner(h ir.TypeHandle) ir.TypeInner {
	if int(h) >= len(f.mod.Types) {
		return nil
	}
	return f.mod.Types[h].Inner
}
