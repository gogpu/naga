package lower

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/gogpu/naga/internal/consteval"
	"github.com/gogpu/naga/internal/registry"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl/internal/parser"
)

// Warning represents a compiler warning (not an error).
type Warning struct {
	Message string
	Span    parser.Span
}

// Lowerer converts WGSL AST to Naga IR.
type Lowerer struct {
	module *ir.Module
	source string // Original source code for error messages

	// Type resolution
	registry *registry.TypeRegistry   // Deduplicates types
	types    map[string]ir.TypeHandle // Named type lookup

	// Variable resolution
	globals         map[string]ir.GlobalVariableHandle
	locals          map[string]ir.ExpressionHandle
	moduleConstants map[string]ir.ConstantHandle
	moduleOverrides map[string]ir.OverrideHandle // override name -> handle into Module.Overrides
	inlineConstants map[string]ir.LiteralValue   // predeclared WGSL constants (RAY_FLAG_*, etc.)
	globalIdx       uint32

	// Function resolution
	functions       map[string]ir.FunctionHandle // Named function lookup (non-entry-point only)
	entryPointFuncs map[string]bool              // Names of entry point functions
	funcMustUse     map[string]bool              // Functions with @must_use attribute

	// Variable usage tracking for unused variable warnings
	localDecls        map[string]parser.Span // Where each local variable was declared
	usedLocals        map[string]bool        // Which local variables have been used
	localConsts       map[string]bool        // Which locals are const declarations (not let/var)
	localIsVar        map[string]bool        // Which locals are var declarations (not let/const)
	localIsPtr        map[string]bool        // Which locals are pointer let-bindings (let p = &v[i])
	localAbstractASTs map[string]parser.Expr // Abstract local const init ASTs (deferred to use site)

	// Scope stack for lexical scoping of local variables.
	// Each entry saves the previous binding for names shadowed in a block scope.
	scopeStack []scopeFrame

	// Current function context
	currentFunc    *ir.Function
	currentFuncIdx ir.FunctionHandle
	currentExprIdx ir.ExpressionHandle
	isInsideLoop   bool // true when lowering statements inside a loop body
	isStatement    bool // true when lowering an expression as a statement (ExprStmt)

	// fnEval evaluates the current function's expressions with constant
	// operands and tracks which expressions are const-expressions (Rust
	// naga's ConstantEvaluator::for_wgsl_function + ExpressionKindTracker).
	fnEval     *consteval.Evaluator
	fnEvalFunc *ir.Function // the function fnEval belongs to

	// moduleConstArena is the arena of the module const context (aliasing
	// Module.GlobalExpressions, with their types) and moduleEval its
	// evaluator; see const_context.go.
	moduleConstArena *ir.Function
	moduleEval       *consteval.Evaluator

	// enableSwizzleAssignment is true when the shader includes
	// "enable swizzle_assignment;", allowing multi-component swizzle on the LHS
	// of assignment statements (e.g., v.xz = vec2(1,2)).
	enableSwizzleAssignment bool

	// emitStateStart tracks the start of the current pending emit range.
	// Set by emitStart(), cleared by emitFinish(). Used by lowerCall()
	// to flush pending argument sub-expressions before the StmtCall,
	// matching Rust naga's Emitter pattern where the emitter is flushed
	// before any side-effectful statement.
	emitStateStart *ir.ExpressionHandle

	// currentEmitTarget is the statement block that the current emitter
	// will flush to. Set by emitStart, used by interruptEmitter.
	currentEmitTarget *[]ir.Statement

	// Per-function cache for GlobalVariable expressions.
	// Rust naga creates ONE expression per global variable reference and reuses it.
	// Without caching, we create a new expression each time a global is referenced,
	// causing expression count divergence (e.g., 39 vs 37 expressions).
	globalExprCache map[ir.GlobalVariableHandle]ir.ExpressionHandle

	// constsWithInlineInit tracks constants whose Init was set inline during lowering.
	constsWithInlineInit map[ir.ConstantHandle]bool

	// Errors and warnings
	errors   parser.SourceErrors
	warnings []Warning
}

// LowerResult contains the result of lowering, including any warnings.
type LowerResult struct {
	Module   *ir.Module
	Warnings []Warning
}

// Lower converts a WGSL AST module to Naga IR.
// All capabilities are enabled (permissive mode for tools and tests).
func Lower(ast *parser.Module) (*ir.Module, error) {
	return LowerWithSource(ast, "")
}

// LowerWithSource converts a WGSL AST module to Naga IR, keeping source for error messages.
// The lowerer is always permissive — all valid WGSL types are accepted.
// Capability validation is performed by [ir.ValidateWithCapabilities] after lowering.
func LowerWithSource(ast *parser.Module, source string) (*ir.Module, error) {
	result, err := LowerWithWarnings(ast, source)
	if err != nil {
		return nil, err
	}
	return result.Module, nil
}

// LowerWithWarnings converts a WGSL AST module to Naga IR, returning warnings.
// The lowerer is always permissive — all valid WGSL types are accepted.
// Capability validation is performed by [ir.ValidateWithCapabilities] after lowering.
func LowerWithWarnings(ast *parser.Module, source string) (*LowerResult, error) {
	return lowerImpl(ast, source)
}

// lowerImpl is the internal implementation shared by all Lower* functions.
func lowerImpl(ast *parser.Module, source string) (*LowerResult, error) {
	// Pre-size module-level slices based on AST declaration counts.
	// This avoids repeated slice growth during lowering.
	nFuncs := len(ast.Functions)
	nStructs := len(ast.Structs)
	nGlobals := len(ast.GlobalVars)
	nConsts := len(ast.Constants)
	nOverrides := len(ast.Overrides)
	// Types: builtins (~15) + struct types + param/return types.
	estTypes := 16 // default matches NewTypeRegistry
	if nStructs > 1 || nFuncs > 2 || nGlobals > 2 {
		estTypes = nStructs*2 + nFuncs + nGlobals + 16
	}
	// Global expressions: roughly 1 per constant/override + 1 per global var init.
	estGlobalExprs := nConsts + nOverrides + nGlobals + 4

	// Build module with pre-sized slices. Only pre-allocate slices that
	// have enough expected items to benefit from avoiding regrowth.
	// For small counts (0-2), nil slice append is fine — Go allocates
	// on first append with no regrowth for single-element slices.
	mod := &ir.Module{}
	if nConsts > 2 {
		mod.Constants = make([]ir.Constant, 0, nConsts)
	}
	if nGlobals > 2 {
		mod.GlobalVariables = make([]ir.GlobalVariable, 0, nGlobals)
		mod.GlobalExpressions = make([]ir.Expression, 0, estGlobalExprs)
	}
	if nOverrides > 2 {
		mod.Overrides = make([]ir.Override, 0, nOverrides)
	}

	l := &Lowerer{
		module:            mod,
		source:            source,
		registry:          registry.NewTypeRegistryWithCap(estTypes),
		types:             make(map[string]ir.TypeHandle, 16),
		globals:           make(map[string]ir.GlobalVariableHandle, max(nGlobals, 8)),
		locals:            make(map[string]ir.ExpressionHandle, 16),
		moduleConstants:   make(map[string]ir.ConstantHandle, max(nConsts, 16)),
		moduleOverrides:   make(map[string]ir.OverrideHandle, max(nOverrides, 8)),
		inlineConstants:   make(map[string]ir.LiteralValue, 32),
		functions:         make(map[string]ir.FunctionHandle, nFuncs),
		entryPointFuncs:   make(map[string]bool, 4),
		funcMustUse:       make(map[string]bool, 4),
		localDecls:        make(map[string]parser.Span, 16),
		usedLocals:        make(map[string]bool, 16),
		localConsts:       make(map[string]bool, 4),
		localIsVar:        make(map[string]bool, 16),
		localIsPtr:        make(map[string]bool, 4),
		localAbstractASTs: make(map[string]parser.Expr, 4),
	}

	// Process enable directives.
	for i := range ast.Enables {
		for _, ext := range ast.Enables[i].Extensions {
			if ext == "swizzle_assignment" {
				l.enableSwizzleAssignment = true
			}
		}
	}

	// Register built-in types
	l.registerBuiltinTypes()

	// Dependency-ordered single-pass processing matching Rust naga's visit_ordered().
	// Declarations are topologically sorted by their dependencies, then processed
	// in a single pass. This ensures every declaration is lowered AFTER all
	// declarations it references, producing identical type registration order.
	sortedDecls, err := parser.DependencyOrder(ast.Declarations)
	if err != nil {
		var de *parser.DeclarationError
		span := parser.Span{}
		if errors.As(err, &de) {
			span = de.Decl.Pos()
		}
		l.addError(err.Error(), span)
		return nil, &l.errors
	}

	// Pre-register function names to support forward references.
	// Entry point functions are NOT added to Module.Functions[] — they are
	// stored inline in EntryPoint.Function (matching Rust naga). Only
	// regular (non-entry-point) functions get FunctionHandle assignments.
	// IMPORTANT: Handles are assigned in dependency-sorted order (not source order)
	// to match Rust naga's visit_ordered() which processes functions in DFS post-order.
	{
		// First pass: identify entry points and @must_use functions
		for _, f := range ast.Functions {
			if l.entryPointStage(f.Attributes) != nil {
				l.entryPointFuncs[f.Name] = true
			}
			for _, attr := range f.Attributes {
				if attr.Name == "must_use" {
					l.funcMustUse[f.Name] = true
					break
				}
			}
		}
		// Second pass: assign handles in dependency order
		nextHandle := ir.FunctionHandle(0)
		for _, decl := range sortedDecls {
			if f, ok := decl.(*parser.FunctionDecl); ok {
				if !l.entryPointFuncs[f.Name] {
					l.functions[f.Name] = nextHandle
					nextHandle++
				}
			}
		}
	}
	processedFunctions := make(map[string]bool)

	for _, decl := range sortedDecls {
		switch d := decl.(type) {
		case *parser.AliasDecl:
			if err := l.lowerAlias(d); err != nil {
				l.addError(err.Error(), d.Span)
			}
		case *parser.StructDecl:
			if err := l.lowerStruct(d); err != nil {
				l.addError(err.Error(), d.Span)
			}
		case *parser.VarDecl:
			if err := l.lowerGlobalVar(d); err != nil {
				l.addError(err.Error(), d.Span)
			}
		case *parser.OverrideDecl:
			if err := l.lowerOverride(d); err != nil {
				l.addError(err.Error(), d.Span)
			}
		case *parser.ConstDecl:
			if err := l.lowerConstant(d); err != nil {
				l.addError(err.Error(), d.Span)
			}
		case *parser.FunctionDecl:
			if err := l.lowerFunction(d); err != nil {
				l.addError(err.Error(), d.Span)
			}
			processedFunctions[d.Name] = true
		case *parser.ConstAssertDecl:
			// Module-scope const_assert — evaluate and error if false.
			// Matches Rust naga: ConstAssertFailed / NotBool.
			if err := l.evalConstAssert(d.Condition); err != nil {
				l.addError(err.Error(), d.Span)
			}
		}
	}

	// Fallback: process any functions not in Declarations (e.g., from tests
	// that build AST manually without populating Declarations).
	for _, f := range ast.Functions {
		if !processedFunctions[f.Name] {
			if err := l.lowerFunction(f); err != nil {
				l.addError(err.Error(), f.Span)
			}
		}
	}

	if l.errors.HasErrors() {
		return nil, &l.errors
	}

	// Build the GlobalExpressions of overrides and override-dependent global
	// variable initializers (constants already have theirs), so that the
	// module is complete before it is compacted, as in Rust naga's lower().
	l.buildGlobalExpressions()

	// Copy deduplicated types from registry to module
	l.module.Types = l.registry.GetTypes()

	// Rust naga's lower() ends with compact(KeepUnused::Yes): unused globals
	// and functions stay (CompactUnused is the separate KeepUnused::No pass),
	// dead expressions, abstract constants and what only they use go.
	//
	// Order matters: dead function expressions first, so that they do not
	// keep a constant alive; then constants with the global expressions only
	// they use; then the types nothing refers to any more.
	ir.CompactExpressions(l.module)
	ir.CompactConstants(l.module, ir.KeepUnusedYes)

	// Compact types: remove anonymous types not referenced by any handle.
	// This matches Rust naga's compact::compact(module, KeepUnused::Yes)
	// called at the end of lower(), which removes scalar types that were
	// registered during vec/mat/atomic resolution but are only embedded
	// by value (not referenced by handle) in Vector/Matrix/Atomic types.
	ir.CompactTypes(l.module)

	// Reorder surviving types in first-registration order.
	ir.ReorderTypes(l.module)

	// Remove duplicate/redundant Emit statements.
	// Our emitter sometimes generates duplicate Emit ranges (e.g., Emit(7..8) twice)
	// when function call flushes interact with statement-level emit wrappers.
	// Rust naga doesn't have this issue because its emitter uses a different restart mechanism.
	ir.DeduplicateEmits(l.module)

	return &LowerResult{
		Module:   l.module,
		Warnings: l.warnings,
	}, nil
}

// addError adds an error with source location.
func (l *Lowerer) addError(message string, span parser.Span) {
	l.errors.Add(parser.NewSourceError(message, span, l.source))
}

// markConstInlineInit marks a constant as having its Init set inline during lowering.
func (l *Lowerer) markConstInlineInit(ch ir.ConstantHandle) {
	if l.constsWithInlineInit == nil {
		l.constsWithInlineInit = make(map[ir.ConstantHandle]bool)
	}
	l.constsWithInlineInit[ch] = true
}

// registerBuiltinTypes is a no-op placeholder for future use.
// All types (including f32, i32, u32, bool) are created on-demand when first
// referenced. This matches Rust naga behavior where types are added to the arena
// lazily, ensuring identical type numbering for Rust reference compatibility.
// See resolveNamedType() for lazy type registration of all named types.
func (l *Lowerer) registerBuiltinTypes() {
	// All types are registered lazily in resolveNamedType().
}

// generateExternalTextureTypes creates the special NagaExternalTextureParams and
// NagaExternalTextureTransferFn types needed by backends for external texture lowering.
// Mirrors Rust naga's Module::generate_external_texture_types().
func (l *Lowerer) generateExternalTextureTypes() {
	if l.module.SpecialTypes.ExternalTextureParams != nil {
		return // Already generated
	}

	// Register component types in the same order as Rust naga
	tyF32 := l.registerType("", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	tyU32 := l.registerType("", ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	tyVec2U := l.registerType("", ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}})
	tyMat3x2F := l.registerType("", ir.MatrixType{Columns: ir.Vec3, Rows: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}})
	tyMat3x3F := l.registerType("", ir.MatrixType{Columns: ir.Vec3, Rows: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}})
	tyMat4x4F := l.registerType("", ir.MatrixType{Columns: ir.Vec4, Rows: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}})

	// NagaExternalTextureTransferFn struct: { a: f32, b: f32, g: f32, k: f32 }
	transferFnHandle := l.registerNamedType("NagaExternalTextureTransferFn", ir.StructType{
		Members: []ir.StructMember{
			{Name: "a", Type: tyF32, Offset: 0},
			{Name: "b", Type: tyF32, Offset: 4},
			{Name: "g", Type: tyF32, Offset: 8},
			{Name: "k", Type: tyF32, Offset: 12},
		},
		Span: 16,
	})

	// NagaExternalTextureParams struct
	paramsHandle := l.registerNamedType("NagaExternalTextureParams", ir.StructType{
		Members: []ir.StructMember{
			{Name: "yuv_conversion_matrix", Type: tyMat4x4F, Offset: 0},
			{Name: "gamut_conversion_matrix", Type: tyMat3x3F, Offset: 64},
			{Name: "src_tf", Type: transferFnHandle, Offset: 112},
			{Name: "dst_tf", Type: transferFnHandle, Offset: 128},
			{Name: "sample_transform", Type: tyMat3x2F, Offset: 144},
			{Name: "load_transform", Type: tyMat3x2F, Offset: 168},
			{Name: "size", Type: tyVec2U, Offset: 192},
			{Name: "num_planes", Type: tyU32, Offset: 200},
		},
		Span: 208,
	})

	l.module.SpecialTypes.ExternalTextureTransferFunction = &transferFnHandle
	l.module.SpecialTypes.ExternalTextureParams = &paramsHandle
}

// registerRayQueryConstants registers RAY_FLAG_* and RAY_QUERY_INTERSECTION_* constants.
// Called when ray_query type is first resolved.
func (l *Lowerer) registerRayQueryConstants() {
	if _, exists := l.inlineConstants["RAY_FLAG_NONE"]; exists {
		return
	}

	// Register RAY_FLAG_* constants
	rayFlagConstants := []struct {
		name string
		val  uint32
	}{
		{"RAY_FLAG_NONE", 0x00},
		{"RAY_FLAG_FORCE_OPAQUE", 0x01},
		{"RAY_FLAG_FORCE_NO_OPAQUE", 0x02},
		{"RAY_FLAG_TERMINATE_ON_FIRST_HIT", 0x04},
		{"RAY_FLAG_SKIP_CLOSEST_HIT_SHADER", 0x08},
		{"RAY_FLAG_CULL_BACK_FACING", 0x10},
		{"RAY_FLAG_CULL_FRONT_FACING", 0x20},
		{"RAY_FLAG_CULL_OPAQUE", 0x40},
		{"RAY_FLAG_CULL_NO_OPAQUE", 0x80},
		{"RAY_FLAG_SKIP_TRIANGLES", 0x100},
		{"RAY_FLAG_SKIP_AABBS", 0x200},
	}
	// Store as inline constants (Rust naga inlines these during lowering,
	// they don't appear as module-level Constants).
	for _, c := range rayFlagConstants {
		l.inlineConstants[c.name] = ir.LiteralU32(c.val)
	}

	// Register RAY_QUERY_INTERSECTION_* constants
	rqIntConstants := []struct {
		name string
		val  uint32
	}{
		{"RAY_QUERY_INTERSECTION_NONE", 0},
		{"RAY_QUERY_INTERSECTION_TRIANGLE", 1},
		{"RAY_QUERY_INTERSECTION_GENERATED", 2},
		{"RAY_QUERY_INTERSECTION_AABB", 3},
	}
	for _, c := range rqIntConstants {
		l.inlineConstants[c.name] = ir.LiteralU32(c.val)
	}
}

// registerRayDescType registers the RayDesc predeclared struct type on demand.
// Called when RayDesc constructor or rayQueryInitialize is used.
func (l *Lowerer) registerRayDescType() {
	if _, exists := l.types["RayDesc"]; exists {
		return
	}
	f32Handle := l.registerType("f32", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	u32Handle := l.registerType("u32", ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	vec3f32Handle := l.registerType("", ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}})

	l.registerNamedType("RayDesc", ir.StructType{
		Members: []ir.StructMember{
			{Name: "flags", Type: u32Handle, Offset: 0},
			{Name: "cull_mask", Type: u32Handle, Offset: 4},
			{Name: "tmin", Type: f32Handle, Offset: 8},
			{Name: "tmax", Type: f32Handle, Offset: 12},
			{Name: "origin", Type: vec3f32Handle, Offset: 16},
			{Name: "dir", Type: vec3f32Handle, Offset: 32},
		},
		Span: 48,
	})
}

// registerRayIntersectionType registers the RayIntersection struct type on demand.
// Called lazily when ray query intersection functions are actually used.
func (l *Lowerer) registerRayIntersectionType() {
	if _, exists := l.types["RayIntersection"]; exists {
		return
	}
	// Match Rust naga type_gen.rs registration order exactly:
	// ty_flag (U32), ty_scalar (F32), ty_barycentrics (Vec2<f32>), ty_bool, ty_transform (Mat4x3<f32>)
	u32Handle := l.registerType("u32", ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	f32Handle := l.registerType("f32", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	vec2f32Handle := l.registerType("", ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}})
	boolHandle := l.registerType("bool", ir.ScalarType{Kind: ir.ScalarBool, Width: 1})
	mat4x3f32Handle := l.registerType("", ir.MatrixType{Columns: ir.Vec4, Rows: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}})

	riHandle := l.registerNamedType("RayIntersection", ir.StructType{
		Members: []ir.StructMember{
			{Name: "kind", Type: u32Handle, Offset: 0},
			{Name: "t", Type: f32Handle, Offset: 4},
			{Name: "instance_custom_data", Type: u32Handle, Offset: 8},
			{Name: "instance_index", Type: u32Handle, Offset: 12},
			{Name: "sbt_record_offset", Type: u32Handle, Offset: 16},
			{Name: "geometry_index", Type: u32Handle, Offset: 20},
			{Name: "primitive_index", Type: u32Handle, Offset: 24},
			{Name: "barycentrics", Type: vec2f32Handle, Offset: 28},
			{Name: "front_face", Type: boolHandle, Offset: 36},
			{Name: "object_to_world", Type: mat4x3f32Handle, Offset: 48},
			{Name: "world_to_object", Type: mat4x3f32Handle, Offset: 112},
		},
		Span: 176,
	})
	l.module.SpecialTypes.RayIntersection = &riHandle
}

// registerType adds a type to the registry with deduplication and maps its name.
func (l *Lowerer) registerType(name string, inner ir.TypeInner) ir.TypeHandle {
	handle := l.registry.GetOrCreate("", inner)

	// Record type usage order for ReorderTypes.
	// Always append (even for existing types) — ReorderTypes deduplicates via `seen`.
	// This ensures types registered via dedup still appear in TypeUseOrder at their
	// correct lowering position, not just at initial creation.
	l.module.TypeUseOrder = append(l.module.TypeUseOrder, handle)

	// Map named types for lookup by the lowerer
	if name != "" {
		l.types[name] = handle
	}
	// Keep module types in sync so type resolution works during lowering.
	l.module.Types = l.registry.GetTypes()

	return handle
}

// registerTypeSilent registers a type WITHOUT recording in TypeUseOrder.
// Used for intermediate types that may become dead before compaction. If the
// type survives compact, ReorderTypes will pick it up from "remaining types"
// in its current position.
func (l *Lowerer) registerTypeSilent(inner ir.TypeInner) ir.TypeHandle {
	handle := l.registry.GetOrCreate("", inner)
	l.module.Types = l.registry.GetTypes()
	return handle
}

// registerNamedType stores the type with a visible name in the arena.
// Use this for types that should have a name in the output (structs, aliases).
// In Rust naga, struct types are inserted with name: Some("StructName").
func (l *Lowerer) registerNamedType(name string, inner ir.TypeInner) ir.TypeHandle {
	handle := l.registry.GetOrCreate(name, inner)

	l.module.TypeUseOrder = append(l.module.TypeUseOrder, handle)

	if name != "" {
		l.types[name] = handle
	}
	l.module.Types = l.registry.GetTypes()

	return handle
}

// lowerAlias registers a type alias, mapping the alias name to the resolved type.
// Matches Rust naga where ensure_type_exists(Some(alias_name), inner) creates a
// new type entry with the alias name, distinct from any anonymous type with the
// same inner. This is because Rust's UniqueArena deduplicates by the full Type
// struct including name, so Type{name: Some("Mat"), inner: Matrix{...}} is
// distinct from Type{name: None, inner: Matrix{...}}.
func (l *Lowerer) lowerAlias(a *parser.AliasDecl) error {
	// Save TypeUseOrder length — resolveType may register intermediate types
	// (e.g., anonymous scalar for vec3<f32>) that should NOT appear in
	// TypeUseOrder at the alias position. In Rust naga, resolve_named_ast_type
	// computes the inner type and calls ensure_type_exists(Some(name), inner)
	// which only registers the NAMED type in the arena order. Intermediate
	// types (like anonymous scalars) are registered too, but at their resolution
	// position, not at the alias position. We achieve the same by rolling back
	// TypeUseOrder after resolveType and only appending the named handle.
	savedLen := len(l.module.TypeUseOrder)

	typeHandle, err := l.resolveType(a.Type)
	if err != nil {
		return fmt.Errorf("alias %s: %w", a.Name, err)
	}

	// Get the inner type from the resolved handle
	inner := l.module.Types[typeHandle].Inner

	// In Rust naga, ensure_type_exists(Some(alias_name), inner) creates a new
	// type entry with the alias name. The UniqueArena treats
	// Type{name: Some("Mat"), inner: Matrix{...}} as distinct from
	// Type{name: None, inner: Matrix{...}}.
	//
	// We replicate this for Scalar/Vector/Matrix types (which are commonly aliased
	// with user-visible names like "alias Mat = mat2x2<f32>"). For special types
	// (RayQuery, AccelerationStructure, etc.), we keep the existing mapping to
	// avoid shifting type handle indices which would break backend output.
	switch inner.(type) {
	case ir.ScalarType, ir.VectorType, ir.MatrixType:
		// For Scalar/Vector/Matrix: create a SEPARATE named type entry.
		// In Rust naga, UniqueArena deduplicates on (name, inner), so
		// Type{name: Some("Mat"), inner: Matrix{...}} is distinct from
		// Type{name: None, inner: Matrix{...}}.
		l.module.TypeUseOrder = l.module.TypeUseOrder[:savedLen]
		aliasHandle := l.registry.GetOrCreate(a.Name, inner)
		l.module.Types = l.registry.GetTypes()
		l.module.TypeUseOrder = append(l.module.TypeUseOrder, aliasHandle)
		l.types[a.Name] = aliasHandle
	default:
		// For RayQuery, AccelerationStructure, etc.: rename the existing type
		// entry in-place. In Rust naga, resolve_named_ast_type creates a single
		// entry with the alias name. There's no separate anonymous entry.
		// E.g., `alias rq = ray_query;` → Type{name: "rq", inner: RayQuery}.
		if l.module.Types[typeHandle].Name == "" {
			l.module.Types[typeHandle].Name = a.Name
			l.registry.SetName(typeHandle, a.Name)
		}
		l.types[a.Name] = typeHandle
	}

	return nil
}

// lowerStruct converts a struct declaration to IR.
func (l *Lowerer) lowerStruct(s *parser.StructDecl) error {
	members := make([]ir.StructMember, len(s.Members))
	var offset uint32
	var maxAlign uint32 = 1
	for i, m := range s.Members {
		typeHandle, err := l.resolveType(m.Type)
		if err != nil {
			return fmt.Errorf("struct %s member %s: %w", s.Name, m.Name, err)
		}

		// Extract binding from member attributes (@builtin, @location, @blend_src, @interpolate, etc.)
		binding := l.memberBindings(m.Attributes)
		// Apply default interpolation for Location bindings based on type
		// (Rust naga's Binding::apply_default_interpolation)
		binding = l.applyDefaultInterpolation(binding, typeHandle)

		// Calculate proper alignment and size for WGSL uniform buffer layout
		align, size := l.typeAlignmentAndSize(typeHandle)

		// Check for explicit @align(N) attribute on the member
		if explicitAlign := getAlignAttribute(m.Attributes); explicitAlign > 0 {
			align = explicitAlign
		}

		// Check for explicit @size(N) attribute on the member
		if explicitSize := getSizeAttribute(m.Attributes); explicitSize > 0 {
			size = explicitSize
		}

		if align > maxAlign {
			maxAlign = align
		}

		// Align offset to the type's alignment requirement
		offset = (offset + align - 1) &^ (align - 1)

		members[i] = ir.StructMember{
			Name:    m.Name,
			Type:    typeHandle,
			Binding: binding,
			Offset:  offset,
		}
		offset += size
	}
	// Round struct size up to alignment of largest member
	structSize := (offset + maxAlign - 1) &^ (maxAlign - 1)
	l.registerNamedType(s.Name, ir.StructType{Members: members, Span: structSize})
	return nil
}

// getAlignAttribute extracts the value from an @align(N) attribute, returns 0 if not found.
func getAlignAttribute(attrs []parser.Attribute) uint32 {
	for _, attr := range attrs {
		if attr.Name == "align" && len(attr.Args) == 1 {
			if lit, ok := attr.Args[0].(*parser.Literal); ok {
				var val uint32
				if _, err := fmt.Sscanf(lit.Value, "%d", &val); err == nil {
					return val
				}
			}
		}
	}
	return 0
}

// getSizeAttribute extracts the value from a @size(N) attribute, returns 0 if not found.
func getSizeAttribute(attrs []parser.Attribute) uint32 {
	for _, attr := range attrs {
		if attr.Name == "size" && len(attr.Args) == 1 {
			if lit, ok := attr.Args[0].(*parser.Literal); ok {
				var val uint32
				if _, err := fmt.Sscanf(lit.Value, "%d", &val); err == nil {
					return val
				}
			}
		}
	}
	return 0
}

// typeAlignmentAndSize returns the alignment and size of a type for uniform buffer layout.
// Follows WGSL/WebGPU alignment rules (similar to std140 but with some differences).
func (l *Lowerer) typeAlignmentAndSize(handle ir.TypeHandle) (align, size uint32) {
	typ := l.module.Types[handle]

	switch t := typ.Inner.(type) {
	case ir.ScalarType:
		// WGSL layout: use scalar.width as both alignment and size.
		// Matches Rust naga layouter: Alignment::new(scalar.width), size = scalar.width.
		// Bool(1) → align=1, size=1; f16(2) → align=2, size=2; f32(4) → align=4, size=4.
		w := uint32(t.Width)
		return w, w

	case ir.VectorType:
		// Matches Rust naga layouter:
		// size = vec_size * scalar.width
		// alignment = Alignment::from(vec_size) * Alignment::new(scalar.width)
		// where Alignment::from: Bi→2, Tri→4, Quad→4
		scalarWidth := uint32(t.Scalar.Width)
		var vecAlignFactor uint32
		switch t.Size {
		case ir.Vec2:
			vecAlignFactor = 2
		case ir.Vec3, ir.Vec4:
			vecAlignFactor = 4
		}
		alignment := vecAlignFactor * scalarWidth
		size := uint32(t.Size) * scalarWidth
		return alignment, size

	case ir.MatrixType:
		// Matrix layout: column-major, each column is a vec with alignment.
		// Matches Rust naga layouter:
		//   alignment = Alignment::from(rows) * Alignment::new(scalar.width)
		//   size = alignment * columns (via try_size = Alignment::from(rows) * scalar.width * columns)
		scalarWidth := uint32(t.Scalar.Width)
		var rowsAlignFactor uint32
		switch t.Rows {
		case ir.Vec2:
			rowsAlignFactor = 2
		case ir.Vec3, ir.Vec4:
			rowsAlignFactor = 4
		default:
			rowsAlignFactor = 1
		}
		colAlign := rowsAlignFactor * scalarWidth
		return colAlign, colAlign * uint32(t.Columns)

	case ir.ArrayType:
		// Array layout uses element alignment and stride.
		// Matches Rust naga layouter: alignment = base element alignment,
		// stride = alignment.round_up(element_size).
		// Note: uniform buffer 16-byte array stride requirement is enforced
		// by the WGSL spec for uniform address space, but the general type
		// layout uses natural alignment (Rust naga: layouter.rs line 219-230).
		elemAlign, elemSize := l.typeAlignmentAndSize(t.Base)
		stride := (elemSize + elemAlign - 1) &^ (elemAlign - 1)
		if t.Size.Constant != nil {
			return elemAlign, stride * *t.Size.Constant
		}
		// Runtime-sized array
		return elemAlign, stride

	case ir.StructType:
		// Struct alignment is the max of its members, size is pre-calculated
		var maxMemberAlign uint32 = 1
		for _, member := range t.Members {
			memberAlign, _ := l.typeAlignmentAndSize(member.Type)
			if memberAlign > maxMemberAlign {
				maxMemberAlign = memberAlign
			}
		}
		return maxMemberAlign, t.Span

	case ir.AtomicType:
		// Atomic types have the same alignment and size as their base scalar.
		w := uint32(t.Scalar.Width)
		return w, w
	}

	// Default fallback
	return 4, 4
}

// lowerGlobalVar converts a global variable declaration to IR.
func (l *Lowerer) lowerGlobalVar(v *parser.VarDecl) error {
	var explicit *ir.TypeHandle
	if v.Type != nil {
		th, err := l.resolveType(v.Type)
		if err != nil {
			return fmt.Errorf("global var %s: %w", v.Name, err)
		}
		explicit = &th
	}
	typeHandle, initExpr, err := l.lowerGlobalVarInit(v, explicit)
	if err != nil {
		return fmt.Errorf("global var %s: %w", v.Name, err)
	}

	space := l.addressSpace(v.AddressSpace)

	// Samplers and textures must use SpaceHandle (maps to UniformConstant in SPIR-V)
	// This is required by Vulkan: "Variables identified with the UniformConstant
	// storage class are used only as handles to refer to opaque resources.
	// Such variables must be typed as OpTypeImage, OpTypeSampler, OpTypeSampledImage"
	if l.isOpaqueResourceType(typeHandle) {
		space = ir.SpaceHandle
	}

	var binding *ir.ResourceBinding

	// Parse @group and @binding attributes
	hasGroup := false
	hasBinding := false
	for _, attr := range v.Attributes {
		if attr.Name == "group" && len(attr.Args) > 0 {
			if lit, ok := attr.Args[0].(*parser.Literal); ok {
				group, _ := strconv.ParseUint(lit.Value, 10, 32)
				if binding == nil {
					binding = &ir.ResourceBinding{}
				}
				binding.Group = uint32(group)
				hasGroup = true
			}
		}
		if attr.Name == "binding" && len(attr.Args) > 0 {
			if lit, ok := attr.Args[0].(*parser.Literal); ok {
				bind, _ := strconv.ParseUint(lit.Value, 10, 32)
				if binding == nil {
					binding = &ir.ResourceBinding{}
				}
				binding.Binding = uint32(bind)
				hasBinding = true
			}
		}
	}

	// Validate that @binding and @group appear together.
	// WGSL spec requires both attributes on resource variables.
	if hasBinding && !hasGroup {
		return fmt.Errorf("global var '%s': @binding requires @group attribute", v.Name)
	}
	if hasGroup && !hasBinding {
		return fmt.Errorf("global var '%s': @group requires @binding attribute", v.Name)
	}

	// Determine storage access mode from WGSL access mode annotation.
	// var<storage, read_write> → StorageReadWrite (LOAD|STORE)
	// var<storage, read> or var<storage> → StorageRead (LOAD only)
	accessMode := ir.StorageReadWrite
	if space == ir.SpaceStorage {
		switch v.AccessMode {
		case "read":
			accessMode = ir.StorageRead
		case "read_write":
			accessMode = ir.StorageReadWrite
		default:
			// Default for storage without explicit access mode is read-only
			accessMode = ir.StorageRead
		}
	}

	handle := ir.GlobalVariableHandle(l.globalIdx)
	l.globalIdx++
	l.module.GlobalVariables = append(l.module.GlobalVariables, ir.GlobalVariable{
		Name:     v.Name,
		Space:    space,
		Binding:  binding,
		Type:     typeHandle,
		InitExpr: initExpr,
		Access:   accessMode,
	})
	l.globals[v.Name] = handle
	return nil
}

// lowerOverride converts an override declaration to an ir.Override.
// Overrides are stored in Module.Overrides (separate from Constants).
// Init expressions are deferred to buildGlobalExpressions.
func (l *Lowerer) lowerOverride(o *parser.OverrideDecl) error {
	overrideHandle := ir.OverrideHandle(len(l.module.Overrides))

	var explicit *ir.TypeHandle
	if o.Type != nil {
		th, err := l.resolveType(o.Type)
		if err != nil {
			return fmt.Errorf("override %s: %w", o.Name, err)
		}
		explicit = &th
	}
	// The initializer goes to GlobalExpressions now, in declaration order,
	// like a var<private> initializer (Rust: type_and_init with
	// AbstractRule::Concretize in ctx.as_override()).
	typeHandle, init, err := l.lowerOverrideContextInit(o.Name, o.Init, explicit)
	if err != nil {
		return fmt.Errorf("override %s: %w", o.Name, err)
	}

	// Parse @id attribute.
	var id *uint16
	for _, attr := range o.Attributes {
		if attr.Name == "id" && len(attr.Args) > 0 {
			if lit, ok := attr.Args[0].(*parser.Literal); ok {
				if idVal, parseErr := strconv.ParseUint(lit.Value, 10, 16); parseErr == nil {
					id16 := uint16(idVal)
					id = &id16
				}
			}
		}
	}

	l.module.Overrides = append(l.module.Overrides, ir.Override{
		Name: o.Name,
		ID:   id,
		Ty:   typeHandle,
		Init: init,
	})
	l.moduleOverrides[o.Name] = overrideHandle
	return nil
}

// halfToFloat32 converts IEEE 754 half-precision (16-bit) bits to float32.
func halfToFloat32(h uint16) float32 {
	sign := uint32(h>>15) & 1
	exp := uint32(h>>10) & 0x1f
	frac := uint32(h) & 0x3ff

	switch {
	case exp == 0x1f: // inf or nan
		if frac == 0 {
			return math.Float32frombits(sign<<31 | 0xff<<23) // inf
		}
		return math.Float32frombits(sign<<31 | 0xff<<23 | frac<<13) // nan
	case exp == 0: // zero or subnormal
		if frac == 0 {
			return math.Float32frombits(sign << 31) // zero
		}
		// Subnormal: normalize
		for frac&0x400 == 0 {
			frac <<= 1
			exp--
		}
		frac &= 0x3ff
		exp++
		fallthrough
	default: // normal
		f32exp := exp + 127 - 15
		return math.Float32frombits(sign<<31 | f32exp<<23 | frac<<13)
	}
}

// lowerFunction converts a function declaration to IR.
func (l *Lowerer) lowerFunction(f *parser.FunctionDecl) error {
	// Outside a function the lowerer is at module scope: const-expressions
	// met there (constLiteral) go to Module.GlobalExpressions.
	defer func() { l.currentFunc = nil }()

	// Reset local context by clearing maps instead of reallocating.
	for k := range l.locals {
		delete(l.locals, k)
	}
	for k := range l.localDecls {
		delete(l.localDecls, k)
	}
	for k := range l.usedLocals {
		delete(l.usedLocals, k)
	}
	for k := range l.localConsts {
		delete(l.localConsts, k)
	}
	for k := range l.localIsVar {
		delete(l.localIsVar, k)
	}
	for k := range l.localIsPtr {
		delete(l.localIsPtr, k)
	}
	for k := range l.localAbstractASTs {
		delete(l.localAbstractASTs, k)
	}
	l.scopeStack = l.scopeStack[:0]
	// Reset per-function GlobalVariable expression cache.
	// Each function gets its own expression arena, so cached handles from
	// previous functions are invalid.
	if l.globalExprCache == nil {
		l.globalExprCache = make(map[ir.GlobalVariableHandle]ir.ExpressionHandle, len(l.globals))
	} else {
		for k := range l.globalExprCache {
			delete(l.globalExprCache, k)
		}
	}
	l.currentExprIdx = 0

	// Estimate sizes based on function complexity.
	// Each AST statement typically produces ~3 IR expressions (binary ops, loads, etc.).
	// Parameters each produce 1 expression. Globals referenced add ~1 each.
	var bodySize int
	if f.Body != nil {
		bodySize = countStatementsDeep(f.Body)
	}
	nParams := len(f.Params)
	estExprs := bodySize*3 + nParams + len(l.globals) + 4
	if estExprs < 8 {
		estExprs = 8
	}

	fn := &ir.Function{
		Name:             f.Name,
		Arguments:        make([]ir.FunctionArgument, len(f.Params)),
		LocalVars:        make([]ir.LocalVariable, 0, 4),
		Expressions:      make([]ir.Expression, 0, estExprs),
		ExpressionTypes:  make([]ir.TypeResolution, 0, estExprs),
		Body:             make([]ir.Statement, 0, bodySize),
		NamedExpressions: make(map[ir.ExpressionHandle]string, nParams+4),
	}
	l.currentFunc = fn

	// Lower parameters
	for i, p := range f.Params {
		typeHandle, err := l.resolveType(p.Type)
		if err != nil {
			return fmt.Errorf("function %s param %s: %w", f.Name, p.Name, err)
		}

		binding := l.paramBinding(p.Attributes)
		// Apply default interpolation for Location bindings based on type
		// (Rust naga's Binding::apply_default_interpolation)
		binding = l.applyDefaultInterpolation(binding, typeHandle)
		fn.Arguments[i] = ir.FunctionArgument{
			Name:    p.Name,
			Type:    typeHandle,
			Binding: binding,
		}

		// Register parameter as local expression (FunctionArgument)
		exprHandle, err := l.addExpression(ir.Expression{
			Kind: ir.ExprFunctionArgument{Index: uint32(i)},
		})
		if err != nil {
			return err
		}
		l.locals[p.Name] = exprHandle
		// Rust naga adds function arguments to named_expressions
		fn.NamedExpressions[exprHandle] = p.Name
	}

	// Lower return type
	if f.ReturnType != nil {
		typeHandle, err := l.resolveType(f.ReturnType)
		if err != nil {
			return fmt.Errorf("function %s return type: %w", f.Name, err)
		}
		retBinding := l.returnBinding(f.ReturnAttrs)
		// Apply default interpolation for Location bindings based on type
		// (Rust naga's Binding::apply_default_interpolation)
		retBinding = l.applyDefaultInterpolation(retBinding, typeHandle)
		fn.Result = &ir.FunctionResult{
			Type:    typeHandle,
			Binding: retBinding,
		}
	}

	// Lower function body
	if f.Body != nil {
		if err := l.lowerBlock(f.Body, &fn.Body); err != nil {
			return fmt.Errorf("function %s body: %w", f.Name, err)
		}
	}

	// Rust naga calls proc::ensure_block_returns after lowering the body.
	// This ensures every control flow path ends with a Return statement.
	ensureBlockReturns(&fn.Body)

	// Check for unused local variables
	l.checkUnusedVariables(f.Name)

	// Register unused let bindings in NamedExpressions so backends emit them.
	// Used let bindings are already emitted through the normal baking mechanism.
	l.registerUnusedLetBindings()

	// Check if this is an entry point
	stage := l.entryPointStage(f.Attributes)
	if stage != nil {
		// Entry point functions are stored inline in EntryPoint.Function,
		// NOT in Module.Functions[] (matching Rust naga).
		ep := ir.EntryPoint{
			Name:     f.Name,
			Stage:    *stage,
			Function: *fn,
		}
		// Extract workgroup_size for compute/mesh/task shaders.
		// Validate that @workgroup_size is present — required by WGSL spec.
		// Matches Rust naga: Error::MissingWorkgroupSize.
		if *stage == ir.StageCompute || *stage == ir.StageMesh || *stage == ir.StageTask {
			hasWGSize := false
			for _, attr := range f.Attributes {
				if attr.Name == "workgroup_size" {
					hasWGSize = true
					break
				}
			}
			if !hasWGSize {
				return fmt.Errorf("@compute entry point '%s' is missing @workgroup_size attribute", f.Name)
			}
			wg, wgOverrides, err := l.extractWorkgroupSize(f.Attributes)
			if err != nil {
				return err
			}
			ep.Workgroup, ep.WorkgroupSizeOverrides = wg, wgOverrides
		}
		// Extract early_depth_test for fragment shaders
		if *stage == ir.StageFragment {
			ep.EarlyDepthTest = l.extractEarlyDepthTest(f.Attributes)
		}
		// Extract task_payload from @payload(varName) attribute
		ep.TaskPayload = l.extractTaskPayload(f.Attributes)
		// Extract mesh_info from @mesh(outputVar) attribute
		if *stage == ir.StageMesh {
			ep.MeshInfo = l.extractMeshInfo(f.Attributes)
		}
		l.module.EntryPoints = append(l.module.EntryPoints, ep)
	} else {
		// Regular function — add to Module.Functions[]
		funcHandle := l.functions[f.Name]
		l.module.Functions = append(l.module.Functions, *fn)
		l.currentFuncIdx = funcHandle
	}

	return nil
}

// scopeEntry records the previous binding for a name that was shadowed.
type scopeEntry struct {
	name     string
	hadLocal bool                // was there a previous binding in l.locals?
	prevExpr ir.ExpressionHandle // previous l.locals[name] (if hadLocal)
	hadConst bool                // was there a previous l.localConsts[name]?
	hadVar   bool                // was there a previous l.localIsVar[name]?
	hadPtr   bool                // was there a previous l.localIsPtr[name]?
}

// scopeFrame represents one lexical scope level.
type scopeFrame struct {
	entries []scopeEntry
}

// pushScope starts a new lexical scope. Call popScope when leaving.
func (l *Lowerer) pushScope() {
	l.scopeStack = append(l.scopeStack, scopeFrame{})
}

// popScope restores variable bindings to their state before pushScope.
func (l *Lowerer) popScope() {
	if len(l.scopeStack) == 0 {
		return
	}
	frame := l.scopeStack[len(l.scopeStack)-1]
	l.scopeStack = l.scopeStack[:len(l.scopeStack)-1]

	for _, e := range frame.entries {
		if e.hadLocal {
			l.locals[e.name] = e.prevExpr
		} else {
			delete(l.locals, e.name)
		}
		if !e.hadConst {
			delete(l.localConsts, e.name)
		}
		if !e.hadVar {
			delete(l.localIsVar, e.name)
		}
		if !e.hadPtr {
			delete(l.localIsPtr, e.name)
		}
	}
}

// scopeSet records that a name is being bound in the current scope, saving
// any previous binding for restoration by popScope.
func (l *Lowerer) scopeSet(name string) {
	if len(l.scopeStack) == 0 {
		return
	}
	frame := &l.scopeStack[len(l.scopeStack)-1]

	// Only save the first time this name is shadowed in this scope.
	for _, e := range frame.entries {
		if e.name == name {
			return // already saved
		}
	}

	prevExpr, hadLocal := l.locals[name]
	_, hadConst := l.localConsts[name]
	_, hadVar := l.localIsVar[name]
	_, hadPtr := l.localIsPtr[name]

	frame.entries = append(frame.entries, scopeEntry{
		name:     name,
		hadLocal: hadLocal,
		prevExpr: prevExpr,
		hadConst: hadConst,
		hadVar:   hadVar,
		hadPtr:   hadPtr,
	})
}

// lowerBlock converts a block statement to IR statements.
func (l *Lowerer) lowerBlock(block *parser.BlockStmt, target *[]ir.Statement) error {
	for _, stmt := range block.Statements {
		if err := l.lowerStatement(stmt, target); err != nil {
			return err
		}
	}
	return nil
}

// lowerStatement converts a statement to IR.
func (l *Lowerer) lowerStatement(stmt parser.Stmt, target *[]ir.Statement) error {
	switch s := stmt.(type) {
	case *parser.ReturnStmt:
		return l.lowerReturn(s, target)
	case *parser.VarDecl:
		return l.lowerLocalVar(s, target)
	case *parser.AssignStmt:
		return l.lowerAssign(s, target)
	case *parser.IfStmt:
		return l.lowerIf(s, target)
	case *parser.ForStmt:
		return l.lowerFor(s, target)
	case *parser.WhileStmt:
		return l.lowerWhile(s, target)
	case *parser.LoopStmt:
		return l.lowerLoop(s, target)
	case *parser.SwitchStmt:
		return l.lowerSwitch(s, target)
	case *parser.ConstDecl:
		// Local const is treated like let (named expression)
		return l.lowerLocalConst(s, target)
	case *parser.BreakStmt:
		*target = append(*target, ir.Statement{Kind: ir.StmtBreak{}})
		return nil
	case *parser.BreakIfStmt:
		// BreakIfStmt is handled specially during loop lowering (lowerLoop extracts it
		// from the continuing block). It should not reach here in normal code flow.
		return fmt.Errorf("'break if' must appear inside a continuing block of a loop")
	case *parser.ConstAssertDecl:
		// const_assert is a compile-time assertion — WGSL spec requires evaluation.
		// Matches Rust naga: eval_expr_to_bool → ConstAssertFailed / NotBool.
		return l.evalConstAssert(s.Condition)
	case *parser.ContinueStmt:
		*target = append(*target, ir.Statement{Kind: ir.StmtContinue{}})
		return nil
	case *parser.DiscardStmt:
		*target = append(*target, ir.Statement{Kind: ir.StmtKill{}})
		return nil
	case *parser.ExprStmt:
		// Evaluate expression for side effects.
		// Set isStatement flag so atomic calls know their result is discarded.
		l.isStatement = true
		emitStart := l.emitStartWithTarget(target)
		_, err := l.lowerExpression(s.Expr, target)
		if err != nil {
			l.isStatement = false
			return err
		}
		l.emitFinish(emitStart, target)
		l.isStatement = false
		return nil
	case *parser.BlockStmt:
		l.pushScope()
		var body []ir.Statement
		if err := l.lowerBlock(s, &body); err != nil {
			l.popScope()
			return err
		}
		l.popScope()
		*target = append(*target, ir.Statement{Kind: ir.StmtBlock{Block: body}})
		return nil
	default:
		return fmt.Errorf("unsupported statement type: %T", stmt)
	}
}

// lowerReturn converts a return statement to IR (Rust StatementKind::Return):
// the value, abstract or not, converts to the function's result type with
// automatic conversions (`return 1;` in an f32 function returns 1.0).
func (l *Lowerer) lowerReturn(ret *parser.ReturnStmt, target *[]ir.Statement) error {
	var valueHandle *ir.ExpressionHandle
	if ret.Value != nil {
		emitStart := l.emitStartWithTarget(target)
		handle, err := l.lowerExpression(ret.Value, target)
		if err != nil {
			return err
		}
		if l.currentFunc != nil && l.currentFunc.Result != nil {
			if handle, err = l.tryAutomaticConversions(handle, l.module.Types[l.currentFunc.Result.Type].Inner); err != nil {
				return err
			}
		}
		l.emitFinish(emitStart, target)
		valueHandle = &handle
	}
	*target = append(*target, ir.Statement{
		Kind: ir.StmtReturn{Value: valueHandle},
	})
	return nil
}

// lowerLocalVar converts a local variable declaration to IR.
func (l *Lowerer) lowerLocalVar(v *parser.VarDecl, target *[]ir.Statement) error {
	var typeHandle ir.TypeHandle
	var initHandle *ir.ExpressionHandle
	hasExplicitType := false

	// Resolve explicit type BEFORE initializer (matching Rust naga order).
	// Rust's resolve_ast_type runs before type_and_init, ensuring types like
	// i32 are registered before vec2<i32> constructors reference them.
	if v.Type != nil {
		var err error
		typeHandle, err = l.resolveType(v.Type)
		if err != nil {
			return fmt.Errorf("local var %s: %w", v.Name, err)
		}
		hasExplicitType = true
	}

	// Lower initializer
	if v.Init != nil {
		_ = l.emitStartWithTarget(target)
		init, err := l.lowerExpression(v.Init, target)
		if err != nil {
			return err
		}
		// DON'T emitFinish here — emit after LocalVariable expression
		// to match Rust's interrupt_emitter(LocalVariable) + emitter.finish() pattern.
		initHandle = &init
	}

	// Rust type_and_init: convert the initializer to the declared type, or
	// concretize it and take its type.
	if initHandle != nil {
		var explicit *ir.TypeHandle
		if hasExplicitType {
			explicit = &typeHandle
		}
		ty, init, err := l.typeAndInit(v.Name, *initHandle, explicit, concretizeAbstract)
		if err != nil {
			return err
		}
		typeHandle, *initHandle = ty, init
	} else if !hasExplicitType {
		return fmt.Errorf("local var %s: type required without initializer", v.Name)
	}

	localIdx := uint32(len(l.currentFunc.LocalVars))

	// Rust naga merges const initializers into LocalVariable.Init when outside
	// a loop, and splits into zero-init + Store for runtime expressions or when
	// inside a loop. This ensures correct behavior for side-effectful expressions
	// (function calls, derivatives, loads) while producing cleaner output for
	// simple constant initializers.
	var localInit *ir.ExpressionHandle
	needStore := false
	if initHandle != nil {
		if l.isInsideLoop || !l.isConstExpression(*initHandle) {
			// Runtime expression or inside loop: zero-init + Store
			localInit = nil
			needStore = true
		} else {
			// Const expression outside loop: merge into declaration
			localInit = initHandle
		}
	}

	l.currentFunc.LocalVars = append(l.currentFunc.LocalVars, ir.LocalVariable{
		Name: v.Name,
		Type: typeHandle,
		Init: localInit,
	})

	// Create local variable expression using interrupt_emitter pattern.
	// Matches Rust: interrupt_emitter(LocalVariable) flushes the current emit
	// (covering init expressions), adds LocalVariable outside emit range,
	// then block.extend(emitter.finish()) adds any remaining emit.
	exprHandle, err := l.interruptEmitter(ir.Expression{
		Kind: ir.ExprLocalVariable{Variable: localIdx},
	})
	if err != nil {
		return err
	}
	// emitter.finish() — any expressions after LocalVariable (usually empty)
	if v.Init != nil {
		endEmit := l.currentExprIdx
		if l.emitStateStart != nil && endEmit > *l.emitStateStart {
			*target = append(*target, ir.Statement{Kind: ir.StmtEmit{
				Range: ir.Range{Start: *l.emitStateStart, End: endEmit},
			}})
		}
		l.emitStateStart = nil
	}
	l.scopeSet(v.Name)
	l.locals[v.Name] = exprHandle

	// Emit Store for runtime initial values (or const values inside loops).
	if needStore {
		*target = append(*target, ir.Statement{Kind: ir.StmtStore{
			Pointer: exprHandle,
			Value:   *initHandle,
		}})
	}

	// Track declaration for unused variable warnings
	l.localDecls[v.Name] = v.Span
	l.localIsVar[v.Name] = true

	return nil
}

// isConstExpression returns true if the expression is a compile-time constant
// that can be used as a local variable initializer (merged into the declaration).
// This matches Rust naga's is_const_or_override check: literals, zero values,
// module constants, and compositions/splats of constant sub-expressions.
func (l *Lowerer) isConstExpression(handle ir.ExpressionHandle) bool {
	if l.currentFunc == nil || int(handle) >= len(l.currentFunc.Expressions) {
		return false
	}
	return l.eval().IsConstOrOverride(handle)
}

// constEvalExprToU32 tries to evaluate an expression as a compile-time constant u32.
// This matches Rust naga's const_eval_expr_to_u32: it checks if the expression is a
// const expression (literal, constant, etc.) and evaluates it to a u32 value.
// Returns (value, true) on success, (0, false) if not a constant or out of range.
func (l *Lowerer) constEvalExprToU32(handle ir.ExpressionHandle) (uint32, bool) {
	if l.currentFunc == nil || int(handle) >= len(l.currentFunc.Expressions) {
		return 0, false
	}
	if !l.isConstExpression(handle) {
		return 0, false
	}
	expr := &l.currentFunc.Expressions[handle]
	switch k := expr.Kind.(type) {
	case ir.Literal:
		switch v := k.Value.(type) {
		case ir.LiteralI32:
			if int32(v) >= 0 {
				return uint32(v), true
			}
		case ir.LiteralU32:
			return uint32(v), true
		case ir.LiteralAbstractInt:
			if int64(v) >= 0 && int64(v) <= int64(^uint32(0)) {
				return uint32(v), true
			}
		}
	case ir.ExprConstant:
		// Could evaluate constant init, but for now just handle literals
	}
	return 0, false
}

// lowerAssign converts an assignment statement to IR.
func (l *Lowerer) lowerAssign(assign *parser.AssignStmt, target *[]ir.Statement) error {
	// WGSL discard pattern: _ = expr; evaluates RHS for side effects, discards result.
	// Matches Rust naga: preserve as named expression "phony" so backends emit it.
	if ident, ok := assign.Left.(*parser.Ident); ok && ident.Name == "_" {
		emitStart := l.emitStartWithTarget(target)
		rhs, err := l.lowerExpression(assign.Right, target)
		if err != nil {
			return err
		}
		// Rust: a phony assignment lowers its value with expression(),
		// which concretizes abstract values.
		if rhs, err = l.concretize(rhs); err != nil {
			return err
		}
		l.emitFinish(emitStart, target)
		// Matches Rust naga: ALL phony assignments are added unconditionally.
		l.addPhonyExpression(rhs)
		return nil
	}

	// Check for chained/indexed swizzle assignment patterns (e.g., v.zyx.x, v.zyx[1]).
	// Must be checked BEFORE the simple swizzle check because chained patterns
	// have a MemberExpr wrapping a MemberExpr (or IndexExpr wrapping a MemberExpr).
	if csa, ok := l.detectChainedSwizzleAssignment(assign.Left); ok {
		if !l.enableSwizzleAssignment {
			return fmt.Errorf("chained swizzle assignment requires 'enable swizzle_assignment'")
		}
		return l.lowerChainedSwizzleAssign(csa, assign.Op, assign.Right, target)
	}

	// Check for multi-component swizzle assignment (e.g., v.xz = vec2(1,2)).
	// Requires "enable swizzle_assignment;" in the shader.
	// Decomposed into: Load + AccessIndex + Compose + Store at IR level,
	// so all backends handle it through standard IR without changes.
	if sa, ok := l.detectSwizzleAssignment(assign.Left); ok {
		if !l.enableSwizzleAssignment {
			return fmt.Errorf("multi-component swizzle assignment requires 'enable swizzle_assignment'")
		}
		return l.lowerSwizzleAssign(sa, assign.Op, assign.Right, target)
	}

	// Start emit range BEFORE LHS lowering. Rust naga's emitter covers both
	// the LHS reference chain and RHS value in a single Emit statement. This
	// ensures that Load expressions created as part of LHS dynamic indexing
	// (e.g., alignment.v3[idx] = 3.0 creates a Load of idx) are covered by
	// the Emit and get named expressions like _eN in the backend.
	emitStart := l.emitStartWithTarget(target)

	// LHS is lowered as a reference (pointer) for Store.
	// Special case: *ptr = value extracts the inner pointer.
	var pointer ir.ExpressionHandle
	var err error
	if unary, ok := assign.Left.(*parser.UnaryExpr); ok && unary.Op == parser.TokenStar {
		// *ptr dereference: the operand itself is the pointer
		pointer, err = l.lowerExpressionForRef(unary.Operand, target)
	} else {
		pointer, err = l.lowerExpressionForRef(assign.Left, target)
	}
	if err != nil {
		return err
	}
	value, err := l.lowerAssignValue(pointer, assign.Op, assign.Right, assign.Increment, target)
	if err != nil {
		return err
	}
	l.emitFinish(emitStart, target)

	*target = append(*target, ir.Statement{
		Kind: ir.StmtStore{Pointer: pointer, Value: value},
	})
	return nil
}

// swizzleAssignInfo describes a multi-component swizzle on the LHS of an assignment.
// Used by detectSwizzleAssignment and lowerSwizzleAssign.
type swizzleAssignInfo struct {
	base    parser.Expr // The vector variable base expression (e.g., "v" in "v.xz")
	indices []uint32    // Component indices in original vector order (e.g., [0,2] for .xz)
	vecSize ir.VectorSize
}

// detectSwizzleAssignment checks if the LHS expression is a multi-component
// swizzle on a vector (e.g., v.xz, v.rgb). Single-component access (.x)
// is handled by the normal AccessIndex path and does not need decomposition.
//
// Returns (info, true) if this is a multi-component swizzle assignment,
// or (zero, false) otherwise.
func (l *Lowerer) detectSwizzleAssignment(expr parser.Expr) (swizzleAssignInfo, bool) {
	mem, ok := expr.(*parser.MemberExpr)
	if !ok || len(mem.Member) < 2 {
		return swizzleAssignInfo{}, false
	}

	// Validate that all chars are valid swizzle components in the same namespace.
	firstNs := swizzleComponentNamespace(mem.Member[0])
	if firstNs == swizzleNsNone {
		return swizzleAssignInfo{}, false
	}
	for i := 1; i < len(mem.Member); i++ {
		ns := swizzleComponentNamespace(mem.Member[i])
		if ns == swizzleNsNone || ns != firstNs {
			return swizzleAssignInfo{}, false
		}
	}

	// Parse each component index.
	indices := make([]uint32, len(mem.Member))
	for i := 0; i < len(mem.Member); i++ {
		comp, ok := swizzleComponent(mem.Member[i])
		if !ok {
			return swizzleAssignInfo{}, false
		}
		indices[i] = uint32(comp)
	}

	// Check for duplicate indices (WGSL spec: indices must be distinct).
	seen := make(map[uint32]bool, len(indices))
	for _, idx := range indices {
		if seen[idx] {
			return swizzleAssignInfo{}, false
		}
		seen[idx] = true
	}

	// Determine vector size from indices.
	var vecSize ir.VectorSize
	switch len(indices) {
	case 2:
		vecSize = ir.Vec2
	case 3:
		vecSize = ir.Vec3
	case 4:
		vecSize = ir.Vec4
	default:
		return swizzleAssignInfo{}, false
	}

	return swizzleAssignInfo{
		base:    mem.Expr,
		indices: indices,
		vecSize: vecSize,
	}, true
}

// lowerSwizzleAssign decomposes a multi-component swizzle assignment into
// standard IR operations: Load + AccessIndex (extract components) + Compose + Store.
//
// For simple assignment (v.ywx = source):
//  1. Lower base as pointer reference
//  2. Load original vector value
//  3. Lower RHS value expression
//  4. Extract each source component via AccessIndex
//  5. Extract unchanged components from original via AccessIndex
//  6. Compose new vector with all components in correct positions
//  7. Store the new vector back
//
// For compound assignment (v.ywx += source):
//  1. Lower base as pointer reference
//  2. Load original vector value
//  3. Extract swizzled components from original → build sub-vector
//  4. Lower RHS value expression
//  5. Apply binary op between extracted sub-vector and RHS
//  6. Extract result components
//  7. Compose new full vector with updated components at swizzle positions
//  8. Store the new vector back
func (l *Lowerer) lowerSwizzleAssign(
	sa swizzleAssignInfo,
	op parser.TokenKind,
	rhs parser.Expr,
	target *[]ir.Statement,
) error {
	emitStart := l.emitStartWithTarget(target)

	// Step 1: Lower the base as a pointer reference.
	basePtr, err := l.lowerExpressionForRef(sa.base, target)
	if err != nil {
		return fmt.Errorf("swizzle assignment base: %w", err)
	}

	// Resolve the base vector type to determine the full vector size and scalar type.
	baseType, err := ir.ResolveExpressionType(l.module, l.currentFunc, basePtr)
	if err != nil {
		return fmt.Errorf("swizzle assignment base type: %w", err)
	}
	vec, vecOk, vecErr := l.vectorType(baseType)
	if vecErr != nil {
		return vecErr
	}
	if !vecOk {
		return fmt.Errorf("swizzle assignment target is not a vector type")
	}
	fullSize := int(vec.Size)

	// Step 2: Load original vector value.
	loadExpr, err := l.addExpression(ir.Expression{
		Kind: ir.ExprLoad{Pointer: basePtr},
	})
	if err != nil {
		return err
	}

	// Step 3: Lower RHS value.
	rhsValue, err := l.lowerExpression(rhs, target)
	if err != nil {
		return fmt.Errorf("swizzle assignment rhs: %w", err)
	}
	// The value converts to the vector's scalar, a shift amount to u32, as in
	// lowerAssignValue.
	goal := vec.Scalar
	binOp := l.assignOpToBinary(op)
	if op != parser.TokenEqual && (binOp == ir.BinaryShiftLeft || binOp == ir.BinaryShiftRight) {
		goal = ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	}
	if rhsValue, err = l.tryAutomaticConversionForLeafScalar(rhsValue, goal); err != nil {
		return fmt.Errorf("swizzle assignment rhs: %w", err)
	}

	// For compound assignment, compute the binary op result first.
	var sourceValue ir.ExpressionHandle
	if op != parser.TokenEqual {
		// Build a swizzle of the original vector to match the LHS swizzle pattern,
		// so we can apply the binary op element-wise.
		swizzleSize := sa.vecSize
		var pattern [4]ir.SwizzleComponent
		for i, idx := range sa.indices {
			pattern[i] = ir.SwizzleComponent(idx)
		}
		origSwizzled, err := l.addExpression(ir.Expression{
			Kind: ir.ExprSwizzle{Size: swizzleSize, Vector: loadExpr, Pattern: pattern},
		})
		if err != nil {
			return err
		}
		if origSwizzled, rhsValue, err = l.binaryOpSplat(binOp, origSwizzled, rhsValue); err != nil {
			return err
		}

		// Apply binary operation: origSwizzled op rhs.
		sourceValue, err = l.addExpression(ir.Expression{
			Kind: ir.ExprBinary{
				Op:    binOp,
				Left:  origSwizzled,
				Right: rhsValue,
			},
		})
		if err != nil {
			return err
		}
	} else {
		sourceValue = rhsValue
	}

	// Step 4: Extract source components via AccessIndex.
	sourceComponents := make([]ir.ExpressionHandle, len(sa.indices))
	for i := range sa.indices {
		sourceComponents[i], err = l.addExpression(ir.Expression{
			Kind: ir.ExprAccessIndex{Base: sourceValue, Index: uint32(i)},
		})
		if err != nil {
			return err
		}
	}

	// Step 5: Extract unchanged components from the original vector.
	// Build a set of swizzle target indices for quick lookup.
	swizzleSet := make(map[uint32]int, len(sa.indices))
	for i, idx := range sa.indices {
		swizzleSet[idx] = i
	}

	origComponents := make(map[uint32]ir.ExpressionHandle, fullSize)
	for i := 0; i < fullSize; i++ {
		ci := uint32(i)
		if _, isSwizzled := swizzleSet[ci]; !isSwizzled {
			origComponents[ci], err = l.addExpression(ir.Expression{
				Kind: ir.ExprAccessIndex{Base: loadExpr, Index: ci},
			})
			if err != nil {
				return err
			}
		}
	}

	// Step 6: Compose the new full vector.
	// Each position gets either the source component or the original unchanged component.
	composeComponents := make([]ir.ExpressionHandle, fullSize)
	for i := 0; i < fullSize; i++ {
		ci := uint32(i)
		if srcIdx, isSwizzled := swizzleSet[ci]; isSwizzled {
			composeComponents[i] = sourceComponents[srcIdx]
		} else {
			composeComponents[i] = origComponents[ci]
		}
	}

	// Get the full vector type handle for the Compose expression.
	vecTypeHandle := l.registerTypeSilent(ir.VectorType{
		Size:   vec.Size,
		Scalar: vec.Scalar,
	})
	l.module.Types = l.registry.GetTypes()

	newVec, err := l.addExpression(ir.Expression{
		Kind: ir.ExprCompose{Type: vecTypeHandle, Components: composeComponents},
	})
	if err != nil {
		return err
	}

	l.emitFinish(emitStart, target)

	// Step 7: Store the new vector back.
	*target = append(*target, ir.Statement{
		Kind: ir.StmtStore{Pointer: basePtr, Value: newVec},
	})

	return nil
}

// chainedSwizzleAssignInfo describes a chained or indexed swizzle on the LHS of an
// assignment. Covers Dawn patterns 3-4, 7-11:
//   - v.zyx.x = 1.0           (chained single: resolvedIndices = [2])
//   - v.zyx.yz = vec2(1,2)    (chained multi:  resolvedIndices = [1,0])
//   - v.zyx[1] = 1.0          (indexed const:  resolvedIndices = [1])
//   - v.zyx[i] = 1.0          (indexed dynamic: swizzleMap + dynamicIndex)
type chainedSwizzleAssignInfo struct {
	base            parser.Expr // The ultimate vector variable base (e.g., "v")
	resolvedIndices []uint32    // Final indices in original vector space (nil for dynamic)
	swizzleMap      []uint32    // The outer swizzle mapping (e.g., [2,1,0] for "zyx")
	dynamicIndex    parser.Expr // Non-nil for dynamic-indexed patterns (v.zyx[i])
	isCompound      bool        // True if compound assignment (+=, etc.)
}

// detectChainedSwizzleAssignment checks if the LHS expression is a chained swizzle
// or indexed swizzle pattern. These patterns have:
//   - MemberExpr on top of a multi-component MemberExpr (chained: v.zyx.x, v.zyx.yz)
//   - IndexExpr on top of a multi-component MemberExpr (indexed: v.zyx[1], v.zyx[i])
//
// Returns (info, true) if a chained/indexed swizzle pattern is detected.
func (l *Lowerer) detectChainedSwizzleAssignment(expr parser.Expr) (chainedSwizzleAssignInfo, bool) {
	switch outer := expr.(type) {
	case *parser.MemberExpr:
		// Check for chained swizzle: v.zyx.x or v.zyx.yz
		// The inner expression must be a multi-component swizzle MemberExpr.
		innerMem, ok := outer.Expr.(*parser.MemberExpr)
		if !ok || len(innerMem.Member) < 2 {
			return chainedSwizzleAssignInfo{}, false
		}

		// Parse outer swizzle map.
		outerMap, ok := parseSwizzleMap(innerMem.Member)
		if !ok {
			return chainedSwizzleAssignInfo{}, false
		}

		// Parse the second (chained) member access.
		innerIndices, ok := parseSwizzleIndices(outer.Member)
		if !ok {
			return chainedSwizzleAssignInfo{}, false
		}

		// Resolve through the swizzle chain: each index in the second access
		// selects from the first swizzle's mapping.
		resolved := make([]uint32, len(innerIndices))
		for i, idx := range innerIndices {
			if int(idx) >= len(outerMap) {
				return chainedSwizzleAssignInfo{}, false
			}
			resolved[i] = outerMap[idx]
		}

		// For multi-component resolved indices, check for duplicates.
		if len(resolved) > 1 {
			seen := make(map[uint32]bool, len(resolved))
			for _, idx := range resolved {
				if seen[idx] {
					return chainedSwizzleAssignInfo{}, false
				}
				seen[idx] = true
			}
		}

		return chainedSwizzleAssignInfo{
			base:            innerMem.Expr,
			resolvedIndices: resolved,
			swizzleMap:      outerMap,
		}, true

	case *parser.IndexExpr:
		// Check for indexed swizzle: v.zyx[1] or v.zyx[i]
		// The base expression must be a multi-component swizzle MemberExpr.
		innerMem, ok := outer.Expr.(*parser.MemberExpr)
		if !ok || len(innerMem.Member) < 2 {
			return chainedSwizzleAssignInfo{}, false
		}

		// Parse outer swizzle map.
		outerMap, ok := parseSwizzleMap(innerMem.Member)
		if !ok {
			return chainedSwizzleAssignInfo{}, false
		}

		// Try to evaluate the index as a compile-time constant.
		if constIdx, ok := tryParseConstIndex(outer.Index); ok {
			// Constant index: resolve to specific original component.
			if constIdx >= len(outerMap) {
				return chainedSwizzleAssignInfo{}, false
			}
			return chainedSwizzleAssignInfo{
				base:            innerMem.Expr,
				resolvedIndices: []uint32{outerMap[constIdx]},
				swizzleMap:      outerMap,
			}, true
		}

		// Dynamic index: need runtime remapping.
		return chainedSwizzleAssignInfo{
			base:         innerMem.Expr,
			swizzleMap:   outerMap,
			dynamicIndex: outer.Index,
		}, true
	}

	return chainedSwizzleAssignInfo{}, false
}

// parseSwizzleMap parses a multi-component swizzle string (e.g., "zyx") into
// its component indices [2, 1, 0]. Returns the index mapping and true if valid.
// Rejects duplicate indices since LHS swizzle assignments require distinct components.
// This also prevents false positives for struct member names like "arr" whose characters
// happen to be valid RGBA components.
func parseSwizzleMap(member string) ([]uint32, bool) {
	if len(member) < 2 {
		return nil, false
	}

	firstNs := swizzleComponentNamespace(member[0])
	if firstNs == swizzleNsNone {
		return nil, false
	}

	indices := make([]uint32, len(member))
	seen := make(map[uint32]bool, len(member))
	for i := 0; i < len(member); i++ {
		ns := swizzleComponentNamespace(member[i])
		if ns == swizzleNsNone || ns != firstNs {
			return nil, false
		}
		comp, ok := swizzleComponent(member[i])
		if !ok {
			return nil, false
		}
		idx := uint32(comp)
		if seen[idx] {
			return nil, false
		}
		seen[idx] = true
		indices[i] = idx
	}
	return indices, true
}

// parseSwizzleIndices parses a swizzle member string into component indices.
// Works for both single ("x") and multi-component ("yz") access.
func parseSwizzleIndices(member string) ([]uint32, bool) {
	if len(member) == 0 {
		return nil, false
	}

	firstNs := swizzleComponentNamespace(member[0])
	if firstNs == swizzleNsNone {
		return nil, false
	}

	indices := make([]uint32, len(member))
	for i := 0; i < len(member); i++ {
		ns := swizzleComponentNamespace(member[i])
		if ns == swizzleNsNone || ns != firstNs {
			return nil, false
		}
		comp, ok := swizzleComponent(member[i])
		if !ok {
			return nil, false
		}
		indices[i] = uint32(comp)
	}
	return indices, true
}

// tryParseConstIndex attempts to extract a compile-time integer constant from
// an AST expression (integer literal). Returns the integer value and true if
// the expression is a constant integer.
func tryParseConstIndex(expr parser.Expr) (int, bool) {
	lit, ok := expr.(*parser.Literal)
	if !ok || lit.Kind != parser.TokenIntLiteral {
		return 0, false
	}
	// Parse integer value from string, stripping optional suffix (u, i).
	s := lit.Value
	if len(s) > 0 && (s[len(s)-1] == 'u' || s[len(s)-1] == 'i') {
		s = s[:len(s)-1]
	}
	val, err := strconv.ParseInt(s, 0, 64)
	if err != nil {
		return 0, false
	}
	return int(val), true
}

// lowerChainedSwizzleAssign handles chained and indexed swizzle assignment patterns.
// For single-resolved-index patterns (v.zyx.x = 1.0, v.zyx[1] = 1.0):
//   - Emits store_vector_element on the resolved original index.
//
// For multi-resolved-index patterns (v.zyx.yz = vec2(1,2)):
//   - Decomposes into the same Load + AccessIndex + Compose + Store pattern
//     as regular swizzle assignment, but with remapped indices.
//
// For dynamic-index patterns (v.zyx[i] = 1.0):
//   - Emits a swizzle index remapping array and store_vector_element with
//     a dynamic access into the remapping array.
func (l *Lowerer) lowerChainedSwizzleAssign(
	csa chainedSwizzleAssignInfo,
	op parser.TokenKind,
	rhs parser.Expr,
	target *[]ir.Statement,
) error {
	// Dynamic-index case: v.zyx[i] = 1.0 or v.zyx[i] += 1.0
	if csa.dynamicIndex != nil {
		return l.lowerDynamicIndexedSwizzleAssign(csa, op, rhs, target)
	}

	// Single resolved index: v.zyx.x = 1.0, v.zyx[1] = 1.0
	// This resolves to a single-component store (store_vector_element).
	if len(csa.resolvedIndices) == 1 {
		return l.lowerSingleComponentSwizzleAssign(csa, op, rhs, target)
	}

	// Multi resolved indices: v.zyx.yz = vec2(1,2) → resolvedIndices = [1, 0]
	// Reuse the existing swizzle assignment decomposition with remapped indices.
	sa := swizzleAssignInfo{
		base:    csa.base,
		indices: csa.resolvedIndices,
	}
	switch len(csa.resolvedIndices) {
	case 2:
		sa.vecSize = ir.Vec2
	case 3:
		sa.vecSize = ir.Vec3
	case 4:
		sa.vecSize = ir.Vec4
	}
	return l.lowerSwizzleAssign(sa, op, rhs, target)
}

// lowerSingleComponentSwizzleAssign handles assignment to a single resolved
// component of a chained/indexed swizzle (e.g., v.zyx.x = 1.0 → v.z = 1.0).
// Emits the same IR as a simple v.z = 1.0 assignment: pointer + AccessIndex + Store.
func (l *Lowerer) lowerSingleComponentSwizzleAssign(
	csa chainedSwizzleAssignInfo,
	op parser.TokenKind,
	rhs parser.Expr,
	target *[]ir.Statement,
) error {
	emitStart := l.emitStartWithTarget(target)

	// Lower the base as pointer reference.
	basePtr, err := l.lowerExpressionForRef(csa.base, target)
	if err != nil {
		return fmt.Errorf("chained swizzle assignment base: %w", err)
	}

	// Create AccessIndex to the resolved component.
	componentPtr, err := l.addExpression(ir.Expression{
		Kind: ir.ExprAccessIndex{Base: basePtr, Index: csa.resolvedIndices[0]},
	})
	if err != nil {
		return err
	}

	value, err := l.lowerAssignValue(componentPtr, op, rhs, false, target)
	if err != nil {
		return fmt.Errorf("chained swizzle assignment rhs: %w", err)
	}
	l.emitFinish(emitStart, target)

	*target = append(*target, ir.Statement{
		Kind: ir.StmtStore{Pointer: componentPtr, Value: value},
	})

	return nil
}

// lowerDynamicIndexedSwizzleAssign handles assignment to a dynamically-indexed
// swizzle component (e.g., v.zyx[i] = 1.0 or v.zyx[i] += 1.0).
//
// Dawn's approach: create a constant array of the swizzle map indices, then use
// the dynamic index to look up the original vector component index at runtime.
//
// For v.zyx[i] = 1.0 with swizzleMap [2,1,0]:
//  1. Lower dynamic index expression
//  2. Create constant array [2u, 1u, 0u]
//  3. Access array at dynamic index → original component index
//  4. store_vector_element %v, resolved_idx, value
//
// For compound (v.zyx[i] += 1.0):
//  1. Same as above, but load_vector_element first, apply binary op, then store.
func (l *Lowerer) lowerDynamicIndexedSwizzleAssign(
	csa chainedSwizzleAssignInfo,
	op parser.TokenKind,
	rhs parser.Expr,
	target *[]ir.Statement,
) error {
	emitStart := l.emitStartWithTarget(target)

	// Lower the base as pointer reference.
	basePtr, err := l.lowerExpressionForRef(csa.base, target)
	if err != nil {
		return fmt.Errorf("dynamic indexed swizzle base: %w", err)
	}

	// Lower the dynamic index expression.
	dynIdx, err := l.lowerExpression(csa.dynamicIndex, target)
	if err != nil {
		return fmt.Errorf("dynamic indexed swizzle index: %w", err)
	}

	// Build constant array of swizzle map indices: array<u32, N>(2u, 1u, 0u)
	// Create u32 constants for each swizzle map entry.
	mapConstants := make([]ir.ExpressionHandle, len(csa.swizzleMap))
	for i, idx := range csa.swizzleMap {
		mapConstants[i], err = l.addExpression(ir.Expression{
			Kind: ir.Literal{Value: ir.LiteralU32(idx)},
		})
		if err != nil {
			return err
		}
	}

	// Create the array type: array<u32, N>
	u32Scalar := ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	u32TypeHandle := l.registerTypeSilent(u32Scalar)
	arrSize := ir.ArraySize{Constant: new(uint32)}
	*arrSize.Constant = uint32(len(csa.swizzleMap))
	arrType := ir.ArrayType{
		Base:   u32TypeHandle,
		Size:   arrSize,
		Stride: 4,
	}
	arrTypeHandle := l.registerTypeSilent(arrType)
	l.module.Types = l.registry.GetTypes()

	// Compose the constant array.
	mapArray, err := l.addExpression(ir.Expression{
		Kind: ir.ExprCompose{Type: arrTypeHandle, Components: mapConstants},
	})
	if err != nil {
		return err
	}

	// Access the array at the dynamic index to get the original component index.
	resolvedIdx, err := l.addExpression(ir.Expression{
		Kind: ir.ExprAccess{Base: mapArray, Index: dynIdx},
	})
	if err != nil {
		return err
	}

	// Lower the RHS value.
	value, err := l.lowerExpression(rhs, target)
	if err != nil {
		return fmt.Errorf("dynamic indexed swizzle rhs: %w", err)
	}

	// For compound assignment: load current value at resolved index, apply binary op.
	if op != parser.TokenEqual {
		binOp := l.assignOpToBinary(op)

		// Load the current value at the resolved component index.
		// Use Access (dynamic index) on the loaded vector.
		loadedVec, err := l.addExpression(ir.Expression{
			Kind: ir.ExprLoad{Pointer: basePtr},
		})
		if err != nil {
			return err
		}

		// Duplicate the array access for the load side (Dawn emits two accesses).
		resolvedIdxForLoad, err := l.addExpression(ir.Expression{
			Kind: ir.ExprAccess{Base: mapArray, Index: dynIdx},
		})
		if err != nil {
			return err
		}

		currentVal, err := l.addExpression(ir.Expression{
			Kind: ir.ExprAccess{Base: loadedVec, Index: resolvedIdxForLoad},
		})
		if err != nil {
			return err
		}

		value, err = l.addExpression(ir.Expression{
			Kind: ir.ExprBinary{
				Op:    binOp,
				Left:  currentVal,
				Right: value,
			},
		})
		if err != nil {
			return err
		}
	}

	l.emitFinish(emitStart, target)

	// Emit store_vector_element with dynamic resolved index.
	// In naga IR, this maps to Access + Store on a pointer.
	// Create Access on the pointer base with the resolved index.
	componentPtr, err := l.addExpression(ir.Expression{
		Kind: ir.ExprAccess{Base: basePtr, Index: resolvedIdx},
	})
	if err != nil {
		return err
	}

	*target = append(*target, ir.Statement{
		Kind: ir.StmtStore{Pointer: componentPtr, Value: value},
	})

	return nil
}

// lowerIf converts an if statement to IR.
func (l *Lowerer) lowerIf(ifStmt *parser.IfStmt, target *[]ir.Statement) error {
	emitStart := l.emitStartWithTarget(target)
	condition, err := l.lowerExpression(ifStmt.Condition, target)
	if err != nil {
		return err
	}
	l.emitFinish(emitStart, target)

	var accept, reject []ir.Statement
	l.pushScope()
	if err := l.lowerBlock(ifStmt.Body, &accept); err != nil {
		l.popScope()
		return err
	}
	l.popScope()

	if ifStmt.Else != nil {
		l.pushScope()
		switch e := ifStmt.Else.(type) {
		case *parser.BlockStmt:
			// Plain else block: lower contents directly into reject (no StmtBlock wrapper).
			// Matches Rust naga where else body content goes directly into reject block.
			if err := l.lowerBlock(e, &reject); err != nil {
				l.popScope()
				return err
			}
		default:
			// Else-if chain: lower as statement (creates nested StmtIf).
			if err := l.lowerStatement(ifStmt.Else, &reject); err != nil {
				l.popScope()
				return err
			}
		}
		l.popScope()
	}

	*target = append(*target, ir.Statement{
		Kind: ir.StmtIf{
			Condition: condition,
			Accept:    accept,
			Reject:    reject,
		},
	})
	return nil
}

// lowerFor converts a for loop to IR.
func (l *Lowerer) lowerFor(forStmt *parser.ForStmt, target *[]ir.Statement) error {
	// For loops become: init; loop { if !condition { break }; body; update }
	// The for's init, body, condition, and update share a scope.
	l.pushScope()
	defer l.popScope()

	// The init statement is OUTSIDE the loop — isInsideLoop is unchanged for it.
	if forStmt.Init != nil {
		if err := l.lowerStatement(forStmt.Init, target); err != nil {
			return err
		}
	}

	// Body, condition, and continuing are INSIDE the loop.
	prevInsideLoop := l.isInsideLoop
	l.isInsideLoop = true
	defer func() { l.isInsideLoop = prevInsideLoop }()

	var body, continuing []ir.Statement

	// Add condition check at start of body.
	// Rust naga uses: if (condition) {} else { break; }
	// instead of: if (!condition) { break; }
	// This avoids creating an extra negation expression in the IR.
	if forStmt.Condition != nil {
		emitStart := l.emitStartWithTarget(&body)
		condition, err := l.lowerExpression(forStmt.Condition, &body)
		if err != nil {
			return err
		}
		l.emitFinish(emitStart, &body)
		body = append(body, ir.Statement{
			Kind: ir.StmtIf{
				Condition: condition,
				Accept:    []ir.Statement{},
				Reject:    []ir.Statement{{Kind: ir.StmtBreak{}}},
			},
		})
	}

	// Rust naga wraps the for loop body in a Block statement.
	// This produces { ... } scope in the output, matching the reference.
	l.pushScope()
	var innerBody []ir.Statement
	if err := l.lowerBlock(forStmt.Body, &innerBody); err != nil {
		l.popScope()
		return err
	}
	l.popScope()
	body = append(body, ir.Statement{
		Kind: ir.StmtBlock{Block: innerBody},
	})

	// Add update in continuing block
	if forStmt.Update != nil {
		if err := l.lowerStatement(forStmt.Update, &continuing); err != nil {
			return err
		}
	}

	*target = append(*target, ir.Statement{
		Kind: ir.StmtLoop{
			Body:       body,
			Continuing: continuing,
		},
	})
	return nil
}

// lowerWhile converts a while loop to IR.
func (l *Lowerer) lowerWhile(whileStmt *parser.WhileStmt, target *[]ir.Statement) error {
	prevInsideLoop := l.isInsideLoop
	l.isInsideLoop = true
	defer func() { l.isInsideLoop = prevInsideLoop }()

	var body []ir.Statement

	// Check condition at start
	emitStart := l.emitStartWithTarget(&body)
	condition, err := l.lowerExpression(whileStmt.Condition, &body)
	if err != nil {
		return err
	}

	l.emitFinish(emitStart, &body)
	// Rust naga uses: if (condition) {} else { break; }
	// instead of: if (!condition) { break; }
	// This avoids negation and matches the Rust MSL output.
	body = append(body, ir.Statement{
		Kind: ir.StmtIf{
			Condition: condition,
			Accept:    []ir.Statement{},
			Reject:    []ir.Statement{{Kind: ir.StmtBreak{}}},
		},
	})

	// Rust naga wraps the while loop body in a Block statement.
	// This produces { ... } scope in the output, matching the reference.
	l.pushScope()
	var innerBody []ir.Statement
	if err := l.lowerBlock(whileStmt.Body, &innerBody); err != nil {
		l.popScope()
		return err
	}
	l.popScope()
	body = append(body, ir.Statement{
		Kind: ir.StmtBlock{Block: innerBody},
	})

	*target = append(*target, ir.Statement{
		Kind: ir.StmtLoop{
			Body:       body,
			Continuing: []ir.Statement{},
		},
	})
	return nil
}

// lowerLoop converts a loop statement to IR.
func (l *Lowerer) lowerLoop(loopStmt *parser.LoopStmt, target *[]ir.Statement) error {
	prevInsideLoop := l.isInsideLoop
	l.isInsideLoop = true
	defer func() { l.isInsideLoop = prevInsideLoop }()

	var body, continuing []ir.Statement

	// Loop body and continuing block share one scope: variables declared
	// in the body are visible in the continuing block (e.g., break if).
	l.pushScope()
	if err := l.lowerBlock(loopStmt.Body, &body); err != nil {
		l.popScope()
		return err
	}

	// Extract "break if" from continuing block if present.
	// WGSL spec: "break if expr;" must be the last statement in a continuing block.
	var breakIfHandle *ir.ExpressionHandle
	if loopStmt.Continuing != nil {
		// Lower all statements except the last if it's a BreakIfStmt
		stmts := loopStmt.Continuing.Statements
		var nonBreakIfStmts []parser.Stmt
		var breakIfStmt *parser.BreakIfStmt

		if len(stmts) > 0 {
			if bi, ok := stmts[len(stmts)-1].(*parser.BreakIfStmt); ok {
				breakIfStmt = bi
				nonBreakIfStmts = stmts[:len(stmts)-1]
			} else {
				nonBreakIfStmts = stmts
			}
		}

		for _, stmt := range nonBreakIfStmts {
			if err := l.lowerStatement(stmt, &continuing); err != nil {
				return err
			}
		}

		if breakIfStmt != nil {
			emitStart := l.emitStartWithTarget(&continuing)
			handle, err := l.lowerExpression(breakIfStmt.Condition, &continuing)
			if err != nil {
				return fmt.Errorf("break if condition: %w", err)
			}
			l.emitFinish(emitStart, &continuing)
			breakIfHandle = &handle
		}
	}

	l.popScope() // end scope shared by body and continuing

	*target = append(*target, ir.Statement{
		Kind: ir.StmtLoop{
			Body:       body,
			Continuing: continuing,
			BreakIf:    breakIfHandle,
		},
	})
	return nil
}

// lowerSwitch converts a switch statement to IR.
func (l *Lowerer) lowerSwitch(switchStmt *parser.SwitchStmt, target *[]ir.Statement) error {
	// Rust naga: the selector and every case value are lowered (abstract
	// kept) under one emitter, converted to their consensus scalar (i32 when
	// all are abstract), and the case values read as constants.
	emitStart := l.emitStartWithTarget(target)
	exprs, err := l.lowerSwitchExprs(switchStmt, target)
	if err != nil {
		return err
	}
	l.emitFinish(emitStart, target)
	selector, caseExprs := exprs[0], exprs[1:]

	var cases []ir.SwitchCase
	for i, clause := range switchStmt.Cases {
		l.pushScope()
		var caseBody []ir.Statement
		if err := l.lowerBlock(clause.Body, &caseBody); err != nil {
			l.popScope()
			return fmt.Errorf("switch case %d body: %w", i, err)
		}
		l.popScope()

		if clause.IsDefault && len(clause.Selectors) == 0 {
			// Pure default case: case default: { body } or default: { body }
			cases = append(cases, ir.SwitchCase{
				Value: ir.SwitchValueDefault{},
				Body:  caseBody,
			})
		} else {
			// For multi-selector cases (e.g., case 3, 4: { body }),
			// emit N-1 fallthrough cases with empty bodies, then the
			// final case with the actual body. Matches Rust naga IR.
			//
			// When IsDefault is true with selectors, emit in source order.
			// case default, 6: -> default: (fallthrough), case 6: { body }
			// case 1, default: -> case 1: (fallthrough), default: { body }
			if clause.IsDefault && clause.DefaultFirst {
				// Default appears first: emit as fallthrough
				cases = append(cases, ir.SwitchCase{
					Value:       ir.SwitchValueDefault{},
					Body:        nil,
					FallThrough: true,
				})
			}
			for j := range clause.Selectors {
				value, err := l.switchCaseValue(caseExprs[0])
				if err != nil {
					return fmt.Errorf("switch case %d selector: %w", i, err)
				}
				caseExprs = caseExprs[1:]
				isLast := j == len(clause.Selectors)-1
				if clause.IsDefault && !clause.DefaultFirst && isLast {
					// Default appears after selectors: last selector is fallthrough,
					// default gets the body
					cases = append(cases, ir.SwitchCase{
						Value:       value,
						Body:        nil,
						FallThrough: true,
					})
				} else if isLast && (!clause.IsDefault || clause.DefaultFirst) {
					// Last selector gets the body (no default after, or default already emitted)
					cases = append(cases, ir.SwitchCase{
						Value: value,
						Body:  caseBody,
					})
				} else {
					cases = append(cases, ir.SwitchCase{
						Value:       value,
						Body:        nil,
						FallThrough: true,
					})
				}
			}
			if clause.IsDefault && !clause.DefaultFirst {
				// Default appears after all selectors: gets the body
				cases = append(cases, ir.SwitchCase{
					Value: ir.SwitchValueDefault{},
					Body:  caseBody,
				})
			}
		}
	}

	*target = append(*target, ir.Statement{
		Kind: ir.StmtSwitch{
			Selector: selector,
			Cases:    cases,
		},
	})
	return nil
}

// lowerSwitchExprs lowers the selector and the case values of a switch, in
// source order, and converts them to their consensus scalar (Rust
// StatementKind::Switch). The result is the selector followed by one
// expression per case value.
func (l *Lowerer) lowerSwitchExprs(sw *parser.SwitchStmt, target *[]ir.Statement) ([]ir.ExpressionHandle, error) {
	isSwitchInt := func(h ir.ExpressionHandle) bool {
		s, ok := l.exprInner(h).(ir.ScalarType)
		return ok && (s == ir.ScalarType{Kind: ir.ScalarSint, Width: 4} ||
			s == ir.ScalarType{Kind: ir.ScalarUint, Width: 4} || s.Kind == ir.ScalarAbstractInt)
	}
	selector, err := l.lowerExpression(sw.Selector, target)
	if err != nil {
		return nil, fmt.Errorf("switch selector: %w", err)
	}
	if !isSwitchInt(selector) {
		return nil, fmt.Errorf("invalid `switch` selector: `switch` selector must be a scalar integer")
	}
	exprs := []ir.ExpressionHandle{selector}
	for _, clause := range sw.Cases {
		for _, sel := range clause.Selectors {
			h, err := l.lowerExpression(sel, target)
			if err != nil {
				return nil, fmt.Errorf("switch case selector: %w", err)
			}
			if !isSwitchInt(h) {
				return nil, fmt.Errorf("invalid `switch` case selector value: `switch` case selector must be a scalar integer const expression")
			}
			exprs = append(exprs, h)
		}
	}
	consensus, _, ok := l.conversionConsensus(nil, exprs)
	if !ok {
		return nil, fmt.Errorf("invalid `switch` case selector value: `switch` case selector must have the same type as the `switch` selector expression")
	}
	if consensus.Kind == ir.ScalarAbstractInt {
		consensus = ir.ScalarType{Kind: ir.ScalarSint, Width: 4}
	}
	if err := l.convertSliceToCommonLeafScalar(exprs, consensus); err != nil {
		return nil, err
	}
	return exprs, nil
}

// switchCaseValue reads a converted case value (Rust get_const_val).
func (l *Lowerer) switchCaseValue(h ir.ExpressionHandle) (ir.SwitchValue, error) {
	lit, _ := l.eval().Literal(h)
	switch v := lit.(type) {
	case ir.LiteralI32:
		return ir.SwitchValueI32(v), nil
	case ir.LiteralU32:
		return ir.SwitchValueU32(v), nil
	}
	return nil, fmt.Errorf("invalid `switch` case selector value: `switch` case selector must be a scalar integer const expression")
}

// evalConstAssert evaluates a const_assert condition (Rust
// GlobalDeclKind::ConstAssert / StatementKind::ConstAssert): a
// const-expression of type bool that must be true.
func (l *Lowerer) evalConstAssert(condition parser.Expr) error {
	lit, err := l.constLiteral(condition)
	if err != nil {
		return fmt.Errorf("const_assert: %w", err)
	}
	val, ok := lit.(ir.LiteralBool)
	if !ok {
		return fmt.Errorf("const_assert: must be a const-expression that resolves to a `bool`")
	}
	if !val {
		return fmt.Errorf("const_assert failed")
	}
	return nil
}

// lowerLocalConst converts a local const declaration to IR.
// Local const is treated as a named expression (similar to let).
// Matches Rust naga: let bindings are stored in Function.NamedExpressions
// so backends emit them as named temporaries even if unused.
func (l *Lowerer) lowerLocalConst(decl *parser.ConstDecl, target *[]ir.Statement) error {
	if decl.Init == nil {
		return fmt.Errorf("local const '%s' must have initializer", decl.Name)
	}

	// Resolve explicit type BEFORE initializer (matching Rust naga order).
	// Rust's resolve_ast_type runs in const context before type_and_init,
	// ensuring scalar types are registered before vector constructors reference them.
	var explicitType ir.TypeHandle
	hasExplicitType := false
	if decl.Type != nil {
		th, typeErr := l.resolveType(decl.Type)
		if typeErr != nil {
			return fmt.Errorf("local '%s' type: %w", decl.Name, typeErr)
		}
		explicitType = th
		hasExplicitType = true
	}

	// Rust: a const is lowered in the const context of the function (every
	// expression must evaluate) and may stay abstract; a let is a run-time
	// expression and is concretized (type_and_init with AbstractRule).
	var explicit *ir.TypeHandle
	if hasExplicitType {
		explicit = &explicitType
	}
	lowerInit := func(rule abstractRule) (ir.ExpressionHandle, error) {
		emitStart := l.emitStartWithTarget(target)
		h, err := l.lowerExpression(decl.Init, target)
		if err != nil {
			return 0, fmt.Errorf("const '%s' initializer: %w", decl.Name, err)
		}
		if _, h, err = l.typeAndInit(decl.Name, h, explicit, rule); err != nil {
			return 0, err
		}
		l.emitFinish(emitStart, target)
		return h, nil
	}
	var initHandle ir.ExpressionHandle
	var err error
	if decl.IsConst {
		initHandle, err = l.inConstContext(func() (ir.ExpressionHandle, error) { return lowerInit(allowAbstract) })
	} else {
		initHandle, err = lowerInit(concretizeAbstract)
	}
	if err != nil {
		return err
	}

	// WGSL discard pattern: let _ = expr; evaluates expr, discards the result.
	if decl.Name == "_" {
		l.addPhonyExpression(initHandle)
		return nil
	}

	l.scopeSet(decl.Name)
	l.locals[decl.Name] = initHandle

	// Track pointer let-bindings: let p = &v[i]
	if un, ok := decl.Init.(*parser.UnaryExpr); ok && un.Op == parser.TokenAmpersand {
		l.localIsPtr[decl.Name] = true
	}

	if decl.IsConst {
		l.localConsts[decl.Name] = true
	} else {
		// `let` binding: register as named expression so backends emit a local variable.
		if l.currentFunc != nil && l.currentFunc.NamedExpressions != nil {
			l.currentFunc.NamedExpressions[initHandle] = decl.Name
		}

		// WGSL spec: let-bound expressions are NOT const expressions
		// (Rust force_non_const).
		l.eval().ForceRuntime(initHandle)
	}

	return nil
}

// lowerExpression converts an expression to IR.
func (l *Lowerer) lowerExpression(expr parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	switch e := expr.(type) {
	case *parser.Literal:
		return l.lowerLiteral(e)
	case *parser.Ident:
		handle, err := l.resolveIdentifier(e.Name)
		if err != nil {
			return 0, err
		}
		// Pointer let-bindings (let p = &v[i]) should NOT be loaded.
		// They hold pointers that are passed directly to pointer-taking functions.
		if l.localIsPtr[e.Name] {
			return handle, nil
		}
		return l.applyLoadRule(handle)
	case *parser.BinaryExpr:
		return l.lowerBinary(e, target)
	case *parser.UnaryExpr:
		return l.lowerUnary(e, target)
	case *parser.CallExpr:
		return l.lowerCall(e, target)
	case *parser.ConstructExpr:
		return l.lowerConstruct(e, target)
	case *parser.MemberExpr:
		handle, err := l.lowerMember(e, target)
		if err != nil {
			return 0, err
		}
		return l.applyLoadRule(handle)
	case *parser.IndexExpr:
		handle, err := l.lowerIndex(e, target)
		if err != nil {
			return 0, err
		}
		return l.applyLoadRule(handle)
	case *parser.BitcastExpr:
		return l.lowerBitcast(e, target)
	default:
		return 0, fmt.Errorf("unsupported expression type: %T", expr)
	}
}

// lowerLiteral converts a literal to IR.
func (l *Lowerer) lowerLiteral(lit *parser.Literal) (ir.ExpressionHandle, error) {
	var value ir.LiteralValue

	switch lit.Kind {
	case parser.TokenIntLiteral:
		text := lit.Value
		isUnsigned := false
		is64bit := false
		// Check 64-bit suffixes first: li, lu
		if len(text) >= 2 && text[len(text)-2:] == "lu" {
			text = text[:len(text)-2]
			isUnsigned = true
			is64bit = true
		} else if len(text) >= 2 && text[len(text)-2:] == "li" {
			text = text[:len(text)-2]
			is64bit = true
		} else if len(text) > 0 && text[len(text)-1] == 'u' {
			text = text[:len(text)-1]
			isUnsigned = true
		} else if len(text) > 0 && text[len(text)-1] == 'i' {
			text = text[:len(text)-1]
		}
		hasSuffix := isUnsigned || is64bit || (len(lit.Value) > 0 && lit.Value[len(lit.Value)-1] == 'i')
		if isUnsigned {
			if is64bit {
				v, _ := strconv.ParseUint(text, 0, 64)
				value = ir.LiteralU64(v)
			} else {
				v, _ := strconv.ParseUint(text, 0, 32)
				value = ir.LiteralU32(v)
			}
		} else if is64bit {
			v, _ := strconv.ParseInt(text, 0, 64)
			value = ir.LiteralI64(v)
		} else if hasSuffix {
			v, _ := strconv.ParseInt(text, 0, 32)
			value = ir.LiteralI32(v)
		} else {
			// No suffix: abstract integer literal (concretized later by context)
			v, _ := strconv.ParseInt(text, 0, 64)
			value = ir.LiteralAbstractInt(v)
		}
	case parser.TokenFloatLiteral:
		text := lit.Value
		// Check for 64-bit suffix: lf
		if len(text) >= 2 && text[len(text)-2:] == "lf" {
			text = text[:len(text)-2]
			v, _ := strconv.ParseFloat(text, 64)
			value = ir.LiteralF64(v)
		} else if len(text) > 0 && text[len(text)-1] == 'h' {
			text = text[:len(text)-1]
			v, _ := strconv.ParseFloat(text, 32)
			value = ir.LiteralF16(roundToF16(float32(v)))
		} else if len(text) > 0 && text[len(text)-1] == 'f' {
			// Explicit 'f' suffix → concrete f32
			text = text[:len(text)-1]
			v, _ := strconv.ParseFloat(text, 32)
			value = ir.LiteralF32(v)
		} else {
			// No suffix → abstract float (concretized later by context)
			v, _ := strconv.ParseFloat(text, 64)
			value = ir.LiteralAbstractFloat(v)
		}
	case parser.TokenTrue:
		value = ir.LiteralBool(true)
	case parser.TokenFalse:
		value = ir.LiteralBool(false)
	case parser.TokenBoolLiteral:
		// Parser normalizes TokenTrue/TokenFalse to TokenBoolLiteral
		value = ir.LiteralBool(lit.Value == "true")
	default:
		return 0, fmt.Errorf("unsupported literal kind: %v", lit.Kind)
	}

	return l.interruptEmitter(ir.Expression{
		Kind: ir.Literal{Value: value},
	})
}

// lowerBinary converts a binary expression to IR.
func (l *Lowerer) lowerBinary(bin *parser.BinaryExpr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	op := l.tokenToBinaryOp(bin.Op)

	// Short-circuit logical operators (&&, ||) in runtime context.
	// Rust naga converts these to If-blocks to avoid evaluating RHS when
	// LHS determines the result. See front/wgsl/lower/mod.rs:2441-2563.
	if (op == ir.BinaryLogicalAnd || op == ir.BinaryLogicalOr) && l.currentFunc != nil {
		return l.lowerLogicalShortCircuit(op, bin.Left, bin.Right, target)
	}

	left, err := l.lowerExpression(bin.Left, target)
	if err != nil {
		return 0, err
	}

	right, err := l.lowerExpression(bin.Right, target)
	if err != nil {
		return 0, err
	}

	// Convert `scalar op vector` to `vector op vector` (Rust binary_op_splat:
	// + - / % only; backends handle vec * scalar).
	left, right, err = l.binaryOpSplat(op, left, right)
	if err != nil {
		return 0, err
	}

	// Automatic conversions, as Rust naga's binary().
	if op == ir.BinaryShiftLeft || op == ir.BinaryShiftRight {
		// The right operand must be, or convert to, a u32 leaf scalar.
		if right, err = l.tryAutomaticConversionForLeafScalar(right, scalarU32); err != nil {
			return 0, err
		}
		// WGSL overload resolution: an abstract left operand is concretized
		// when the right operand is not a const-expression.
		if !l.eval().IsConst(right) {
			if left, err = l.concretize(left); err != nil {
				return 0, err
			}
		}
	} else if consensus, _, ok := l.conversionConsensus(nil, []ir.ExpressionHandle{left, right}); ok {
		// Reconcile the leaf scalars; if that is impossible, validation
		// reports the operand types.
		if left, err = l.convertToLeafScalar(left, consensus); err != nil {
			return 0, err
		}
		if right, err = l.convertToLeafScalar(right, consensus); err != nil {
			return 0, err
		}
	}

	return l.addExpression(ir.Expression{
		Kind: ir.ExprBinary{Op: op, Left: left, Right: right},
	})
}

// lowerLogicalShortCircuit generates short-circuit IR for && and ||.
// Matches Rust naga front/wgsl/lower/mod.rs:2441-2563.
//
// For &&:
//
//	var _eN: bool;
//	if lhs { _eN = rhs; } else { _eN = false; }
//	result = load(_eN)
//
// For ||:
//
//	var _eN: bool;
//	if !lhs { _eN = rhs; } else { _eN = true; }
//	result = load(_eN)
func (l *Lowerer) lowerLogicalShortCircuit(op ir.BinaryOperator, leftAST, rightAST parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	// Lower LHS
	left, err := l.lowerExpression(leftAST, target)
	if err != nil {
		return 0, err
	}

	// Outside a function body (Rust !ctx.is_runtime()) a constant left operand
	// that decides the result short-circuits: the right operand is not
	// lowered. Otherwise the left operand is a constant that does not decide,
	// an override or not well-formed: build the Binary for the evaluator,
	// pipeline creation or type checking to deal with. In a function body the
	// If-block below is built whatever the left operand is, as in Rust.
	if !l.inRuntimeContext() {
		if lit, ok := l.eval().Literal(left); ok {
			if b, isBool := lit.(ir.LiteralBool); isBool &&
				(op == ir.BinaryLogicalAnd && !bool(b) || op == ir.BinaryLogicalOr && bool(b)) {
				return left, nil
			}
		}
		right, err := l.lowerExpression(rightAST, target)
		if err != nil {
			return 0, err
		}
		return l.addExpression(ir.Expression{
			Kind: ir.ExprBinary{Op: op, Left: left, Right: right},
		})
	}

	// Runtime short-circuit: generate If-block pattern
	var condition ir.ExpressionHandle
	var elseVal ir.ExpressionHandle

	if op == ir.BinaryLogicalAnd {
		// if (lhs) { result = rhs } else { result = false }
		condition = left
		elseVal, err = l.interruptEmitter(ir.Expression{
			Kind: ir.Literal{Value: ir.LiteralBool(false)},
		})
		if err != nil {
			return 0, err
		}
	} else {
		// if (!lhs) { result = rhs } else { result = true }
		condition, err = l.addExpression(ir.Expression{
			Kind: ir.ExprUnary{Op: ir.UnaryLogicalNot, Expr: left},
		})
		if err != nil {
			return 0, err
		}
		elseVal, err = l.interruptEmitter(ir.Expression{
			Kind: ir.Literal{Value: ir.LiteralBool(true)},
		})
		if err != nil {
			return 0, err
		}
	}

	// Create local variable for result
	boolType := l.registerType("", ir.ScalarType{Kind: ir.ScalarBool, Width: 1})
	localIdx := ir.ExpressionHandle(len(l.currentFunc.LocalVars))
	l.currentFunc.LocalVars = append(l.currentFunc.LocalVars, ir.LocalVariable{
		Name: "",
		Type: boolType,
		Init: nil,
	})
	pointer, err := l.interruptEmitter(ir.Expression{
		Kind: ir.ExprLocalVariable{Variable: uint32(localIdx)},
	})
	if err != nil {
		return 0, err
	}

	// Flush current emit before If statement
	if l.emitStateStart != nil {
		emitStart := *l.emitStateStart
		l.emitFinish(emitStart, target)
	}

	// Build accept block: lower RHS, store to result var
	acceptBlock := make([]ir.Statement, 0, 1)
	savedEmitStart := l.emitStateStart
	savedEmitTarget := l.currentEmitTarget

	acceptEmitStart := l.emitStartWithTarget(&acceptBlock)
	right, err := l.lowerExpression(rightAST, &acceptBlock)
	if err != nil {
		return 0, err
	}
	l.emitFinish(acceptEmitStart, &acceptBlock)
	acceptBlock = append(acceptBlock, ir.Statement{
		Kind: ir.StmtStore{Pointer: pointer, Value: right},
	})

	// Build reject block: store else_val to result var
	rejectBlock := []ir.Statement{
		{Kind: ir.StmtStore{Pointer: pointer, Value: elseVal}},
	}

	// Emit If statement
	*target = append(*target, ir.Statement{
		Kind: ir.StmtIf{
			Condition: condition,
			Accept:    ir.Block(acceptBlock),
			Reject:    ir.Block(rejectBlock),
		},
	})

	// Restore emit state and restart after the If
	l.emitStateStart = savedEmitStart
	l.currentEmitTarget = savedEmitTarget
	newStart := l.currentExprIdx
	l.emitStateStart = &newStart
	l.currentEmitTarget = target

	// Result: create a NEW LocalVariable reference for the Load.
	// Rust naga creates a separate expression for each reference to a local var.
	loadPointer, err := l.interruptEmitter(ir.Expression{
		Kind: ir.ExprLocalVariable{Variable: uint32(localIdx)},
	})
	if err != nil {
		return 0, err
	}
	return l.addExpression(ir.Expression{
		Kind: ir.ExprLoad{Pointer: loadPointer},
	})
}

// binaryOpSplat inserts Splat expressions for non-multiply binary operations with
// mixed vector/scalar operands. This matches Rust naga's binary_op_splat behavior.
// Multiply is excluded because backends handle vec*scalar natively.
func (l *Lowerer) binaryOpSplat(op ir.BinaryOperator, left, right ir.ExpressionHandle) (ir.ExpressionHandle, ir.ExpressionHandle, error) {
	var err error
	switch op {
	case ir.BinaryAdd, ir.BinarySubtract, ir.BinaryDivide, ir.BinaryModulo:
		// Check if left is vector and right is scalar
		leftInner, rightInner := l.resolveExprTypeInner(left), l.resolveExprTypeInner(right)
		if leftInner == nil || rightInner == nil {
			return left, right, nil
		}
		if leftVec, ok := leftInner.(ir.VectorType); ok {
			if _, ok := rightInner.(ir.ScalarType); ok {
				// Wrap right scalar in Splat to match left vector
				right, err = l.addExpression(ir.Expression{
					Kind: ir.ExprSplat{Size: leftVec.Size, Value: right},
				})
				if err != nil {
					return 0, 0, err
				}
			}
		} else if rightVec, ok := rightInner.(ir.VectorType); ok {
			if _, ok := leftInner.(ir.ScalarType); ok {
				// Wrap left scalar in Splat to match right vector
				left, err = l.addExpression(ir.Expression{
					Kind: ir.ExprSplat{Size: rightVec.Size, Value: left},
				})
				if err != nil {
					return 0, 0, err
				}
			}
		}
	}
	return left, right, nil
}

// resolveExprTypeInner resolves the TypeInner of an expression. Returns nil if unresolvable.
func (l *Lowerer) resolveExprTypeInner(handle ir.ExpressionHandle) ir.TypeInner {
	if l.currentFunc == nil || int(handle) >= len(l.currentFunc.ExpressionTypes) {
		return nil
	}
	typeRes := l.currentFunc.ExpressionTypes[handle]
	inner, _, err := l.resolveTypeInner(typeRes)
	if err != nil {
		return nil
	}
	return inner
}

// checkArgumentType verifies that a call argument's resolved type matches the
// expected parameter type. Abstract types are allowed (concretization handles them).
func (l *Lowerer) checkArgumentType(argHandle ir.ExpressionHandle, paramType ir.TypeHandle, funcName string, argIndex int) error {
	if l.currentFunc == nil || int(argHandle) >= len(l.currentFunc.ExpressionTypes) {
		return nil
	}
	argInner := ir.TypeResInner(l.module, l.currentFunc.ExpressionTypes[argHandle])
	if argInner == nil {
		return nil
	}
	// Skip abstract types — concretization will handle or report them.
	if s, ok := argInner.(ir.ScalarType); ok && (s.Kind == ir.ScalarAbstractInt || s.Kind == ir.ScalarAbstractFloat) {
		return nil
	}
	if int(paramType) >= len(l.module.Types) {
		return nil
	}
	paramInner := l.module.Types[paramType].Inner
	if !typeShapeMatches(argInner, paramInner) {
		return fmt.Errorf("function '%s' argument %d: type mismatch (expected %s, got %s)", funcName, argIndex, typeName(paramInner), typeName(argInner))
	}
	return nil
}

// typeShapeMatches checks if two TypeInner values have compatible shapes.
// Scalar↔Scalar, Vector↔Vector (same size), Matrix↔Matrix (same dims), etc.
func typeShapeMatches(arg, param ir.TypeInner) bool {
	switch p := param.(type) {
	case ir.ScalarType:
		a, ok := arg.(ir.ScalarType)
		return ok && (a.Kind == p.Kind || a.Kind == ir.ScalarAbstractInt || a.Kind == ir.ScalarAbstractFloat)
	case ir.VectorType:
		a, ok := arg.(ir.VectorType)
		return ok && a.Size == p.Size
	case ir.MatrixType:
		a, ok := arg.(ir.MatrixType)
		return ok && a.Columns == p.Columns && a.Rows == p.Rows
	case ir.ArrayType:
		_, ok := arg.(ir.ArrayType)
		return ok
	case ir.StructType:
		_, ok := arg.(ir.StructType)
		return ok
	case ir.PointerType:
		_, ok := arg.(ir.PointerType)
		return ok
	case ir.AtomicType:
		_, ok := arg.(ir.AtomicType)
		return ok
	default:
		return true // opaque types — trust downstream validation
	}
}

func typeName(inner ir.TypeInner) string {
	switch t := inner.(type) {
	case ir.ScalarType:
		switch t.Kind {
		case ir.ScalarFloat:
			if t.Width == 2 {
				return "f16"
			}
			return "f32"
		case ir.ScalarSint:
			return "i32"
		case ir.ScalarUint:
			return "u32"
		case ir.ScalarBool:
			return "bool"
		}
	case ir.VectorType:
		return fmt.Sprintf("vec%d<%s>", t.Size, typeName(t.Scalar))
	case ir.MatrixType:
		return fmt.Sprintf("mat%dx%d<%s>", t.Columns, t.Rows, typeName(t.Scalar))
	case ir.ArrayType:
		return "array<...>"
	case ir.StructType:
		return "struct"
	case ir.PointerType:
		return "ptr<...>"
	}
	return "unknown"
}

// lowerUnary converts a unary expression to IR.
func (l *Lowerer) lowerUnary(un *parser.UnaryExpr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	// Handle address-of operator (&)
	// Returns the reference (pointer) for the operand without applying load rule.
	if un.Op == parser.TokenAmpersand {
		return l.lowerExpressionForRef(un.Operand, target)
	}

	// Handle dereference operator (*)
	if un.Op == parser.TokenStar {
		pointer, err := l.lowerExpression(un.Operand, target)
		if err != nil {
			return 0, err
		}
		return l.addExpression(ir.Expression{
			Kind: ir.ExprLoad{Pointer: pointer},
		})
	}

	// Constant fold: negate of a literal directly, without creating the positive
	// literal first. This matches Rust naga's constant evaluator which evaluates
	// the entire expression, avoiding extra expression handles.
	if un.Op == parser.TokenMinus {
		if lit, ok := un.Operand.(*parser.Literal); ok {
			if result, err := l.lowerNegatedLiteral(lit); err == nil {
				return result, nil
			}
		}
	}

	operand, err := l.lowerExpression(un.Operand, target)
	if err != nil {
		return 0, err
	}

	op := l.tokenToUnaryOp(un.Op)

	return l.addExpression(ir.Expression{
		Kind: ir.ExprUnary{Op: op, Expr: operand},
	})
}

// lowerNegatedLiteral directly creates a negative literal from a positive one,
// without creating the positive literal first. Returns the expression handle.
func (l *Lowerer) lowerNegatedLiteral(lit *parser.Literal) (ir.ExpressionHandle, error) {
	// First, lower the literal to get its value and type
	posHandle, err := l.lowerLiteral(lit)
	if err != nil {
		return 0, err
	}
	// Get the literal value and negate it
	expr := l.currentFunc.Expressions[posHandle]
	posLit, ok := expr.Kind.(ir.Literal)
	if !ok {
		return 0, fmt.Errorf("expected literal expression")
	}
	var negated ir.LiteralValue
	switch v := posLit.Value.(type) {
	case ir.LiteralF16:
		negated = ir.LiteralF16(-float32(v))
	case ir.LiteralF32:
		negated = ir.LiteralF32(-float32(v))
	case ir.LiteralF64:
		negated = ir.LiteralF64(-float64(v))
	case ir.LiteralI32:
		negated = ir.LiteralI32(-int32(v))
	case ir.LiteralI64:
		negated = ir.LiteralI64(-int64(v))
	case ir.LiteralAbstractInt:
		negated = ir.LiteralAbstractInt(-int64(v))
	case ir.LiteralAbstractFloat:
		negated = ir.LiteralAbstractFloat(-float64(v))
	default:
		return 0, fmt.Errorf("cannot negate literal type %T", posLit.Value)
	}
	// Replace the positive literal in-place with the negative one
	l.currentFunc.Expressions[posHandle] = ir.Expression{
		Kind: ir.Literal{Value: negated},
	}
	return posHandle, nil
}

// lowerCall converts a call expression to IR.
func (l *Lowerer) lowerCall(call *parser.CallExpr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	funcName := call.Func.Name

	// Check if this is a built-in function (vec4, vec3, etc.)
	if l.isBuiltinConstructor(funcName) {
		return l.lowerBuiltinConstructor(funcName, call.Args, target)
	}

	// Check if this is a short type alias constructor (e.g., vec3f(1.0, 2.0, 3.0))
	if expanded, ok := shortTypeAliases[funcName]; ok {
		return l.lowerShortAliasConstructor(expanded, call.Args, target)
	}

	// Check if this is the select() built-in (uses ExprSelect, not ExprMath)
	if funcName == "select" {
		return l.lowerSelectCall(call.Args, target)
	}

	// Check if this is a derivative function (dpdx, dpdy, fwidth + coarse/fine)
	if deriv, ok := l.getDerivativeFunction(funcName); ok {
		return l.lowerDerivativeCall(deriv, call.Args, target)
	}

	// Check if this is a relational function (all, any)
	if relFun, ok := l.getRelationalFunction(funcName); ok {
		return l.lowerRelationalCall(relFun, call.Args, target)
	}

	// Check if this is arrayLength
	if funcName == "arrayLength" {
		return l.lowerArrayLengthCall(call.Args, target)
	}

	// Check if this is a math function
	if mathFunc, ok := l.getMathFunction(funcName); ok {
		return l.lowerMathCall(mathFunc, call.Args, target)
	}

	// Check if this is a texture function
	if l.isTextureFunction(funcName) {
		return l.lowerTextureCall(funcName, call.Args, target)
	}

	// Check if this is atomicStore (special case - no result)
	if funcName == "atomicStore" {
		return l.lowerAtomicStore(call.Args, target)
	}

	// Check if this is atomicLoad (special case - 1 arg, returns value)
	if funcName == "atomicLoad" {
		return l.lowerAtomicLoad(call.Args, target)
	}

	// Check if this is an atomic function
	if atomicFunc := l.getAtomicFunction(funcName); atomicFunc != nil {
		return l.lowerAtomicCall(atomicFunc, call.Args, target)
	}

	// Check if this is atomicCompareExchangeWeak (special case - 3 args)
	if funcName == "atomicCompareExchangeWeak" {
		return l.lowerAtomicCompareExchange(call.Args, target)
	}

	// Check if this is workgroupUniformLoad
	if funcName == "workgroupUniformLoad" {
		return l.lowerWorkgroupUniformLoad(call.Args, target)
	}

	// Check if this is a subgroup ballot
	if funcName == "subgroupBallot" {
		return l.lowerSubgroupBallot(call.Args, target)
	}

	// Check if this is a subgroup collective operation
	if op, cop, ok := getSubgroupOperation(funcName); ok {
		return l.lowerSubgroupCollectiveOperation(op, cop, call.Args, target)
	}

	// Check if this is a subgroup gather operation
	if gatherKind, ok := getSubgroupGather(funcName); ok {
		return l.lowerSubgroupGather(gatherKind, call.Args, target)
	}

	// Check if this is a quad operation
	if funcName == "quadSwapX" || funcName == "quadSwapY" || funcName == "quadSwapDiagonal" {
		return l.lowerQuadSwap(funcName, call.Args, target)
	}

	// Check if this is a ray query function
	if l.isRayQueryFunction(funcName) {
		return l.lowerRayQueryCall(funcName, call.Args, target)
	}

	// Check if this is a barrier function
	if barrierFlags := l.getBarrierFlags(funcName); barrierFlags != 0 {
		*target = append(*target, ir.Statement{
			Kind: ir.StmtBarrier{Flags: barrierFlags},
		})
		return 0, nil // Barriers don't return a value
	}

	// Check if this is a type constructor (struct, vector, matrix, scalar, or type alias).
	// This includes lazily-registered types like f16, f64, i64, u64.
	typeHandle, typeExists := l.types[funcName]
	if !typeExists {
		// Try resolving as a named type (triggers lazy registration for f16, i64, etc.)
		resolved, err := l.resolveNamedType(&parser.NamedType{Name: funcName})
		if err == nil {
			typeHandle = resolved
			typeExists = true
		}
	}
	if typeExists {
		return l.lowerTypeConstructorCall(typeHandle, call.Args, target)
	}

	// Regular function call - look up function handle
	funcHandle, ok := l.functions[funcName]
	if !ok {
		return 0, fmt.Errorf("unknown function: %s", funcName)
	}

	// Enforce @must_use: if the function is marked @must_use and its result
	// is discarded as a statement, emit an error.
	// Matches Rust naga: FunctionMustUseUnused.
	if l.funcMustUse[funcName] && l.isStatement {
		return 0, fmt.Errorf("result of @must_use function '%s' must be used", funcName)
	}

	// Each argument is lowered and then converted to its parameter type
	// before the next one is lowered, as in Rust naga's call() (the
	// arguments map applies try_automatic_conversions per argument). The
	// order matters: a converted abstract argument is appended to the arena
	// before the expressions of the following arguments.
	known := int(funcHandle) < len(l.module.Functions)
	countMatches := known && len(call.Args) == len(l.module.Functions[funcHandle].Arguments)
	args := make([]ir.ExpressionHandle, len(call.Args))
	for i, arg := range call.Args {
		handle, err := l.lowerExpression(arg, target)
		if err != nil {
			return 0, err
		}
		if countMatches {
			if handle, err = l.convertCallArgument(handle, funcHandle, funcName, i); err != nil {
				return 0, err
			}
		}
		args[i] = handle
	}
	if known && !countMatches {
		return 0, fmt.Errorf("function '%s' expects %d argument(s), got %d",
			funcName, len(l.module.Functions[funcHandle].Arguments), len(args))
	}

	// Check if the called function has a return type.
	// Void functions don't get a CallResult expression.
	// Functions are lowered in order, so previously-lowered functions are accessible.
	// Forward-referenced functions default to having a result (safe fallback).
	hasResult := true
	if int(funcHandle) < len(l.module.Functions) {
		hasResult = l.module.Functions[funcHandle].Result != nil
	}

	// Flush pending emit range before the StmtCall.
	// Rust naga emits argument sub-expressions BEFORE the call statement,
	// then emits the CallResult in a separate emit range AFTER the call.
	// Without this flush, all expressions (arguments + result) end up in
	// a single Emit range AFTER the call, causing the MSL writer to inline
	// arguments into the call and emit dead loads afterward.
	if l.emitStateStart != nil {
		emitStart := *l.emitStateStart
		l.emitFinish(emitStart, target)
	}

	var resultPtr *ir.ExpressionHandle
	var resultHandle ir.ExpressionHandle
	if hasResult {
		// CallResult is non-emittable — use addExpression directly
		// (not interruptEmitter, since we already flushed the emitter above).
		var err error
		resultHandle, err = l.addExpression(ir.Expression{
			Kind: ir.ExprCallResult{Function: funcHandle},
		})
		if err != nil {
			return 0, err
		}
		resultPtr = &resultHandle
	}

	*target = append(*target, ir.Statement{
		Kind: ir.StmtCall{
			Function:  funcHandle,
			Arguments: args,
			Result:    resultPtr,
		},
	})

	// Restart emit tracking AFTER the Call statement.
	// For functions with results: skip past CallResult (non-emittable).
	// For void functions: restart so the caller's emitFinish picks up
	// the advanced position instead of re-emitting pre-call expressions.
	newStart := l.currentExprIdx
	l.emitStateStart = &newStart
	l.currentEmitTarget = target

	return resultHandle, nil
}

// lowerConstruct converts a type constructor (`T(...)` with T written as a
// type, possibly with template parameters) to IR.
func (l *Lowerer) lowerConstruct(cons *parser.ConstructExpr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if nt, ok := cons.Type.(*parser.NamedType); ok && len(nt.TypeParams) == 0 {
		if ctor, partial := partialConstructor(nt.Name); partial {
			return l.construct(ctor, cons.Args, target)
		}
	}
	typeHandle, err := l.resolveType(cons.Type)
	if err != nil {
		return 0, err
	}
	return l.construct(constructor{kind: ctorType, ty: typeHandle}, cons.Args, target)
}

// lowerMember converts a member access to IR.
//
// Matches Rust naga's Load Rule: member access on a pointer base (variable reference)
// produces AccessIndex on the pointer, NOT Load-then-AccessIndex on the value.
// This means the IR is: GlobalVar → AccessIndex(.x) → Load, producing a scalar load,
// rather than: GlobalVar → Load (whole struct) → AccessIndex(.x).
// The caller's applyLoadRule handles the final Load on the pointer-based AccessIndex.
func (l *Lowerer) lowerMember(mem *parser.MemberExpr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	// Check for builtin result member access (e.g., modf(x).fract, frexp(x).exp).
	// These builtins conceptually return structs but we decompose them into
	// equivalent scalar math operations at lowering time.
	result, ok, err := l.tryLowerBuiltinResultMember(mem, target)
	if err != nil {
		return 0, err
	}
	if ok {
		return result, nil
	}

	// Use lowerExpressionForRef to keep the base as a reference/pointer when possible.
	// For variable references (local/global), this avoids loading the whole struct/vector
	// before accessing a member. Instead, AccessIndex operates on the pointer, and the
	// caller's applyLoadRule will add Load on the resulting member pointer.
	// For non-reference expressions (function calls, composes, etc.), lowerExpressionForRef
	// falls through to lowerExpression, so behavior is unchanged.
	base, err := l.lowerExpressionForRef(mem.Expr, target)
	if err != nil {
		return 0, err
	}

	// Check for atomic compare-exchange result member access (.old_value, .exchanged)
	if l.currentFunc != nil && int(base) < len(l.currentFunc.Expressions) {
		if _, ok := l.currentFunc.Expressions[base].Kind.(ir.ExprAtomicResult); ok {
			switch mem.Member {
			case "old_value":
				return l.addExpression(ir.Expression{
					Kind: ir.ExprAccessIndex{Base: base, Index: 0},
				})
			case "exchanged":
				return l.addExpression(ir.Expression{
					Kind: ir.ExprAccessIndex{Base: base, Index: 1},
				})
			}
		}
	}

	baseType, err := ir.ResolveExpressionType(l.module, l.currentFunc, base)
	if err != nil {
		return 0, fmt.Errorf("member access base type: %w", err)
	}

	if index, ok, err := l.structMemberIndex(baseType, mem.Member); err != nil {
		return 0, err
	} else if ok {
		return l.addExpression(ir.Expression{
			Kind: ir.ExprAccessIndex{Base: base, Index: index},
		})
	}

	vec, ok, err := l.vectorType(baseType)
	if err != nil {
		return 0, err
	}

	if !ok {
		return 0, fmt.Errorf("unsupported member access %q", mem.Member)
	}

	if len(mem.Member) == 1 {
		index, err := l.swizzleIndex(mem.Member, vec.Size)
		if err != nil {
			return 0, err
		}
		return l.addExpression(ir.Expression{
			Kind: ir.ExprAccessIndex{Base: base, Index: index},
		})
	}

	// Multi-component swizzle (.xy, .xyz, etc.) operates on a value, not a pointer.
	// Apply the Load Rule to get the value before creating the Swizzle expression.
	loadedBase, err := l.applyLoadRule(base)
	if err != nil {
		return 0, err
	}

	size, pattern, err := l.swizzlePattern(mem.Member, vec.Size)
	if err != nil {
		return 0, err
	}
	return l.addExpression(ir.Expression{
		Kind: ir.ExprSwizzle{Size: size, Vector: loadedBase, Pattern: pattern},
	})
}

// tryLowerBuiltinResultMember checks if a member access is on a builtin that returns
// a struct result (modf, frexp) and lowers it as Math + AccessIndex, matching Rust naga.
//
// WGSL modf(x) returns __modf_result_f32 { fract: f32, whole: f32 }
// WGSL frexp(x) returns __frexp_result_f32 { fract: f32, exp: i32 }
//
// Rust naga keeps the Modf/Frexp expression returning the struct and uses AccessIndex
// to extract members: index 0 = fract, index 1 = whole/exp.
//
// Returns (handle, true) if handled, or (0, false) if not a builtin result access.
func (l *Lowerer) tryLowerBuiltinResultMember(mem *parser.MemberExpr, target *[]ir.Statement) (ir.ExpressionHandle, bool, error) {
	// The base expression must be a call: modf(x) or frexp(x)
	call, ok := mem.Expr.(*parser.CallExpr)
	if !ok {
		return 0, false, nil
	}

	funcName := call.Func.Name

	switch funcName {
	case "modf":
		if len(call.Args) != 1 {
			return 0, false, nil
		}
		arg, err := l.lowerExpression(call.Args[0], target)
		if err != nil {
			return 0, false, err
		}
		// modf is float-only: concretize abstract args to F32
		if err := l.concretizeFloatInto(&arg); err != nil {
			return 0, false, err
		}

		// Determine the argument's value type (scalar or vector of float)
		argType := l.resolveExprValueType(arg)

		// Create modf result struct type
		modfStructType := l.getOrCreateModfResultType(argType)

		// Create the Modf expression and override its type to the struct
		modfExpr, err := l.addExpression(ir.Expression{
			Kind: ir.ExprMath{Fun: ir.MathModf, Arg: arg},
		})
		if err != nil {
			return 0, false, err
		}
		// Override type resolution: MathModf returns the struct, not the arg type
		l.currentFunc.ExpressionTypes[modfExpr] = ir.TypeResolution{Handle: &modfStructType}

		// Create AccessIndex for the member, override its type to the member type
		var memberIndex uint32
		switch mem.Member {
		case "fract":
			memberIndex = 0
		case "whole":
			memberIndex = 1
		default:
			return 0, false, nil
		}
		result, err := l.addExpression(ir.Expression{
			Kind: ir.ExprAccessIndex{Base: modfExpr, Index: memberIndex},
		})
		if err != nil {
			return 0, false, err
		}
		// Override type: both fract and whole have the same type as the argument
		l.overrideExprTypeFromValue(result, argType)
		return result, true, nil

	case "frexp":
		if len(call.Args) != 1 {
			return 0, false, nil
		}
		arg, err := l.lowerExpression(call.Args[0], target)
		if err != nil {
			return 0, false, err
		}
		// frexp is float-only: concretize abstract args to F32
		if err := l.concretizeFloatInto(&arg); err != nil {
			return 0, false, err
		}

		// Determine the argument's value type
		argType := l.resolveExprValueType(arg)

		// Create frexp result struct type
		frexpStructType := l.getOrCreateFrexResultType(argType)

		// Create the Frexp expression and override its type
		frexpExpr, err := l.addExpression(ir.Expression{
			Kind: ir.ExprMath{Fun: ir.MathFrexp, Arg: arg},
		})
		if err != nil {
			return 0, false, err
		}
		l.currentFunc.ExpressionTypes[frexpExpr] = ir.TypeResolution{Handle: &frexpStructType}

		var memberIndex uint32
		switch mem.Member {
		case "fract":
			memberIndex = 0
			result, err := l.addExpression(ir.Expression{
				Kind: ir.ExprAccessIndex{Base: frexpExpr, Index: memberIndex},
			})
			if err != nil {
				return 0, false, err
			}
			// fract has the same type as the argument
			l.overrideExprTypeFromValue(result, argType)
			return result, true, nil
		case "exp":
			memberIndex = 1
			result, err := l.addExpression(ir.Expression{
				Kind: ir.ExprAccessIndex{Base: frexpExpr, Index: memberIndex},
			})
			if err != nil {
				return 0, false, err
			}
			// exp type is i32 (scalar) or vecN<i32> (vector)
			l.overrideExprTypeToInt(result, argType)
			return result, true, nil
		}
	}

	return 0, false, nil
}

// resolveExprValueType returns the inner TypeInner for an expression.
func (l *Lowerer) resolveExprValueType(handle ir.ExpressionHandle) ir.TypeInner {
	if l.currentFunc == nil || int(handle) >= len(l.currentFunc.ExpressionTypes) {
		return nil
	}
	res := &l.currentFunc.ExpressionTypes[handle]
	if res.Handle != nil && int(*res.Handle) < len(l.module.Types) {
		return l.module.Types[*res.Handle].Inner
	}
	return res.Value
}

// overrideExprTypeFromValue sets the expression type to the given type inner.
func (l *Lowerer) overrideExprTypeFromValue(handle ir.ExpressionHandle, inner ir.TypeInner) {
	if l.currentFunc == nil || int(handle) >= len(l.currentFunc.ExpressionTypes) {
		return
	}
	l.currentFunc.ExpressionTypes[handle] = ir.TypeResolution{Value: inner}
}

// overrideExprTypeToInt sets the expression type to i32 or vecN<i32> matching the vector size.
func (l *Lowerer) overrideExprTypeToInt(handle ir.ExpressionHandle, argType ir.TypeInner) {
	if l.currentFunc == nil || int(handle) >= len(l.currentFunc.ExpressionTypes) {
		return
	}
	switch argType.(type) {
	case ir.VectorType:
		vec := argType.(ir.VectorType)
		l.currentFunc.ExpressionTypes[handle] = ir.TypeResolution{
			Value: ir.VectorType{Size: vec.Size, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		}
	default:
		l.currentFunc.ExpressionTypes[handle] = ir.TypeResolution{
			Value: ir.ScalarType{Kind: ir.ScalarSint, Width: 4},
		}
	}
}

// getOrCreateModfResultType creates or retrieves the __modf_result_* struct type.
// The struct has two members: fract and whole, both with the same type as the argument.
func (l *Lowerer) getOrCreateModfResultType(argType ir.TypeInner) ir.TypeHandle {
	// Determine the member type handle
	var memberTypeHandle ir.TypeHandle
	var structName string

	switch t := argType.(type) {
	case ir.VectorType:
		memberTypeHandle = l.registerType("", t)
		sizeStr := ""
		switch t.Size {
		case ir.Vec2:
			sizeStr = "vec2"
		case ir.Vec3:
			sizeStr = "vec3"
		case ir.Vec4:
			sizeStr = "vec4"
		}
		widthStr := "f32"
		if t.Scalar.Width == 8 {
			widthStr = "f64"
		} else if t.Scalar.Width == 2 {
			widthStr = "f16"
		}
		structName = "__modf_result_" + sizeStr + "_" + widthStr
	default:
		memberTypeHandle = l.registerType("", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
		structName = "__modf_result_f32"
	}

	// Check if already created
	if handle, ok := l.types[structName]; ok {
		return handle
	}

	// Calculate offsets and span based on member type size
	memberSize := l.typeSize(memberTypeHandle)
	members := []ir.StructMember{
		{Name: "fract", Type: memberTypeHandle, Offset: 0},
		{Name: "whole", Type: memberTypeHandle, Offset: memberSize},
	}
	span := memberSize * 2

	return l.registerNamedType(structName, ir.StructType{
		Members: members,
		Span:    span,
	})
}

// getOrCreateFrexResultType creates or retrieves the __frexp_result_* struct type.
// The struct has two members: fract (same type as arg) and exp (i32 or vecN<i32>).
func (l *Lowerer) getOrCreateFrexResultType(argType ir.TypeInner) ir.TypeHandle {
	var fractTypeHandle ir.TypeHandle
	var expTypeHandle ir.TypeHandle
	var structName string

	switch t := argType.(type) {
	case ir.VectorType:
		fractTypeHandle = l.registerType("", t)
		expTypeHandle = l.registerType("", ir.VectorType{
			Size:   t.Size,
			Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4},
		})
		sizeStr := ""
		switch t.Size {
		case ir.Vec2:
			sizeStr = "vec2"
		case ir.Vec3:
			sizeStr = "vec3"
		case ir.Vec4:
			sizeStr = "vec4"
		}
		widthStr := "f32"
		if t.Scalar.Width == 8 {
			widthStr = "f64"
		} else if t.Scalar.Width == 2 {
			widthStr = "f16"
		}
		structName = "__frexp_result_" + sizeStr + "_" + widthStr
	default:
		fractTypeHandle = l.registerType("", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
		expTypeHandle = l.registerType("", ir.ScalarType{Kind: ir.ScalarSint, Width: 4})
		structName = "__frexp_result_f32"
	}

	if handle, ok := l.types[structName]; ok {
		return handle
	}

	fractSize := l.typeSize(fractTypeHandle)
	expSize := l.typeSize(expTypeHandle)
	expOffset := fractSize
	span := expOffset + expSize

	members := []ir.StructMember{
		{Name: "fract", Type: fractTypeHandle, Offset: 0},
		{Name: "exp", Type: expTypeHandle, Offset: expOffset},
	}

	return l.registerNamedType(structName, ir.StructType{
		Members: members,
		Span:    span,
	})
}

// typeSize returns the size in bytes of a type.
func (l *Lowerer) typeSize(handle ir.TypeHandle) uint32 {
	if int(handle) >= len(l.module.Types) {
		return 4
	}
	inner := l.module.Types[handle].Inner
	switch t := inner.(type) {
	case ir.ScalarType:
		return uint32(t.Width)
	case ir.VectorType:
		return uint32(t.Size) * uint32(t.Scalar.Width)
	default:
		return 4
	}
}

// lowerIndex converts an index expression to IR.
// lowerBitcast converts a bitcast<Type>(expr) to IR.
func (l *Lowerer) lowerBitcast(bc *parser.BitcastExpr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	exprHandle, err := l.lowerExpression(bc.Expr, target)
	if err != nil {
		return 0, err
	}
	// Rust lowers the operand with expression(), which concretizes it.
	if err := l.concretizeInto(&exprHandle); err != nil {
		return 0, err
	}

	// Resolve the target type to determine the scalar kind
	targetScalarKind, err := l.resolveTargetScalarKind(bc.Type)
	if err != nil {
		return 0, fmt.Errorf("bitcast target type: %w", err)
	}

	return l.addExpression(ir.Expression{
		Kind: ir.ExprAs{
			Expr:    exprHandle,
			Kind:    targetScalarKind,
			Convert: nil, // nil Convert = bitcast
		},
	})
}

// resolveTargetScalarKind extracts the scalar kind from a type for bitcast.
func (l *Lowerer) resolveTargetScalarKind(t parser.Type) (ir.ScalarKind, error) {
	switch ty := t.(type) {
	case *parser.NamedType:
		switch ty.Name {
		case "f32", "f16", "f64":
			return ir.ScalarFloat, nil
		case "i32", "i64":
			return ir.ScalarSint, nil
		case "u32", "u64":
			return ir.ScalarUint, nil
		case "bool":
			return ir.ScalarBool, nil
		case "vec2", "vec3", "vec4":
			// For vector bitcast, extract scalar kind from type parameter
			if len(ty.TypeParams) > 0 {
				return l.resolveTargetScalarKind(ty.TypeParams[0])
			}
			return ir.ScalarFloat, nil // default to float
		default:
			return 0, fmt.Errorf("unsupported bitcast target type '%s'", ty.Name)
		}
	default:
		return 0, fmt.Errorf("unsupported bitcast target type %T", t)
	}
}

func (l *Lowerer) lowerIndex(idx *parser.IndexExpr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	// Use lowerExpressionForRef to keep the base as a reference/pointer when possible.
	// This avoids loading the whole struct/array before indexing. Instead, AccessIndex
	// operates on the pointer chain, and the caller's applyLoadRule adds Load on the
	// final result. Matches Rust naga's pointer-chain-then-load pattern.
	base, err := l.lowerExpressionForRef(idx.Expr, target)
	if err != nil {
		return 0, err
	}
	return l.lowerIndexWithBase(idx, base, target)
}

// Helper methods

// convertCallArgument converts argument i of a call to funcHandle to the
// parameter type (Rust: try_automatic_conversions) and checks the result.
func (l *Lowerer) convertCallArgument(arg ir.ExpressionHandle, funcHandle ir.FunctionHandle, funcName string, i int) (ir.ExpressionHandle, error) {
	paramTy := l.module.Functions[funcHandle].Arguments[i].Type
	converted, convErr := l.tryAutomaticConversions(arg, l.module.Types[paramTy].Inner)
	if convErr != nil {
		if err := l.checkArgumentType(arg, paramTy, funcName, i); err != nil {
			return 0, err
		}
		return 0, convErr
	}
	if err := l.checkArgumentType(converted, paramTy, funcName, i); err != nil {
		return 0, err
	}
	return converted, nil
}

// isExprCallResult returns true if the expression kind is ExprCallResult.
func isExprCallResult(kind ir.ExpressionKind) bool {
	_, ok := kind.(ir.ExprCallResult)
	return ok
}

// addExpression appends expr to the current function, evaluated by the
// constant evaluator when its operands are constant: Rust naga's
// append_expression -> try_eval_and_append. The error is a shader-creation
// error (a const-expression that overflows, leaves a function's domain,
// divides by zero, ...).
func (l *Lowerer) addExpression(expr ir.Expression) (ir.ExpressionHandle, error) {
	return l.eval().TryEvalAndAppend(expr.Kind)
}

// fnArena is the current function's expression arena as the constant
// evaluator sees it.
type fnArena struct{ l *Lowerer }

func (a fnArena) Expression(h ir.ExpressionHandle) ir.ExpressionKind {
	return a.l.currentFunc.Expressions[h].Kind
}

func (a fnArena) Append(kind ir.ExpressionKind) ir.ExpressionHandle {
	return a.l.appendFunctionExpr(ir.Expression{Kind: kind})
}

func (fnArena) IsGlobal() bool { return false }

// Type registers an anonymous result type of the evaluator.
func (a fnArena) Type(inner ir.TypeInner) ir.TypeHandle { return a.l.registerType("", inner) }

// Array registers the array of base with the given size; the stride is
// base's size rounded up to its alignment, as Rust's layouter computes it.
func (a fnArena) Array(base ir.TypeHandle, size ir.ArraySize) ir.TypeHandle {
	return a.l.registerArrayType(base, size)
}

func (l *Lowerer) registerArrayType(base ir.TypeHandle, size ir.ArraySize) ir.TypeHandle {
	align, sz := l.typeAlignmentAndSize(base)
	stride := (sz + align - 1) &^ (align - 1)
	return l.registerType("", ir.ArrayType{Base: base, Size: size, Stride: stride})
}

// eval returns the constant evaluator of the current function. Each
// function has its own (its stage tracking is per arena), created when the
// function's first expression is appended.
func (l *Lowerer) eval() *consteval.Evaluator {
	if l.inModuleConstContext() {
		return l.moduleEval
	}
	if l.fnEval == nil || l.fnEvalFunc != l.currentFunc {
		arena := fnArena{l}
		l.fnEval = &consteval.Evaluator{Module: l.module, Arena: arena, Types: arena, Behavior: consteval.BehaviorRuntime}
		l.fnEvalFunc = l.currentFunc
	}
	return l.fnEval
}

// appendFunctionExpr appends expr to the current function. Like Rust naga's
// constant_evaluator::append_expr it keeps the "needs_pre_emit" expressions
// (Literal, Constant, ZeroValue, GlobalVariable, FunctionArgument,
// LocalVariable, Override) out of Emit ranges: when the emitter is running,
// the pending range is flushed, expr is added outside it and the emitter is
// restarted.
func (l *Lowerer) appendFunctionExpr(expr ir.Expression) ir.ExpressionHandle {
	if l.emitStateStart != nil && l.currentEmitTarget != nil && needsPreEmit(expr) {
		start := *l.emitStateStart
		if l.currentExprIdx > start {
			*l.currentEmitTarget = append(*l.currentEmitTarget, ir.Statement{Kind: ir.StmtEmit{
				Range: ir.Range{Start: start, End: l.currentExprIdx},
			}})
		}
		handle := l.addExpressionRaw(expr)
		newStart := l.currentExprIdx
		l.emitStateStart = &newStart
		return handle
	}
	return l.addExpressionRaw(expr)
}

// needsPreEmit returns true if the expression is considered pre-emitted
// (emitted at function start) and should not be covered by Emit statements.
// Matches Rust naga's Expression::needs_pre_emit().
func needsPreEmit(expr ir.Expression) bool {
	switch expr.Kind.(type) {
	case ir.Literal, ir.ExprConstant, ir.ExprOverride, ir.ExprZeroValue,
		ir.ExprFunctionArgument, ir.ExprGlobalVariable, ir.ExprLocalVariable:
		return true
	default:
		return false
	}
}

// addExpressionRaw adds an expression without const-fold attempt (used by const-fold itself).
func (l *Lowerer) addExpressionRaw(expr ir.Expression) ir.ExpressionHandle {
	handle := l.currentExprIdx
	l.currentExprIdx++
	l.currentFunc.Expressions = append(l.currentFunc.Expressions, expr)

	exprType, err := ir.ResolveExpressionType(l.module, l.currentFunc, handle)
	if err != nil {
		exprType = ir.TypeResolution{}
	}
	l.currentFunc.ExpressionTypes = append(l.currentFunc.ExpressionTypes, exprType)
	if l.inModuleConstContext() {
		// The evaluator reads Module.GlobalExpressions directly.
		l.module.GlobalExpressions = l.currentFunc.Expressions
	}

	return handle
}

// applyLoadRule implements the WGSL Load Rule: if expr is a reference-producing
// expression (GlobalVariable in non-Handle space, LocalVariable, or FunctionArgument
// with pointer type), wrap it with ExprLoad to produce a value.
// This matches Rust naga's apply_load_rule which inserts explicit Load expressions
// for variable references, ensuring expression handle numbering matches Rust.
func (l *Lowerer) applyLoadRule(handle ir.ExpressionHandle) (ir.ExpressionHandle, error) {
	if l.currentFunc == nil {
		return handle, nil
	}
	if int(handle) >= len(l.currentFunc.Expressions) {
		return handle, nil
	}

	expr := l.currentFunc.Expressions[handle]
	switch kind := expr.Kind.(type) {
	case ir.ExprGlobalVariable:
		// Don't load Handle-space globals (textures, samplers) — they are opaque
		// resources, not true pointers.
		if int(kind.Variable) < len(l.module.GlobalVariables) {
			gv := &l.module.GlobalVariables[kind.Variable]
			if gv.Space == ir.SpaceHandle {
				return handle, nil
			}
		}
		return l.addExpression(ir.Expression{
			Kind: ir.ExprLoad{Pointer: handle},
		})
	case ir.ExprLocalVariable:
		return l.addExpression(ir.Expression{
			Kind: ir.ExprLoad{Pointer: handle},
		})
	case ir.ExprAccessIndex:
		// AccessIndex on a pointer base (variable reference) is itself a reference.
		// Apply load to the final access result, unless the result type is unsized
		// (e.g., runtime-sized array) which cannot be loaded.
		if l.isPointerExpressionInLowerer(kind.Base) {
			if l.isUnsizedType(handle) {
				return handle, nil
			}
			return l.addExpression(ir.Expression{
				Kind: ir.ExprLoad{Pointer: handle},
			})
		}
		return handle, nil
	case ir.ExprAccess:
		// Access on a pointer base (variable reference) is itself a reference.
		if l.isPointerExpressionInLowerer(kind.Base) {
			if l.isUnsizedType(handle) {
				return handle, nil
			}
			return l.addExpression(ir.Expression{
				Kind: ir.ExprLoad{Pointer: handle},
			})
		}
		return handle, nil
	default:
		return handle, nil
	}
}

// isPointerExpressionInLowerer returns true if the expression produces a
// pointer/reference (variable references and access chains on them).
func (l *Lowerer) isPointerExpressionInLowerer(handle ir.ExpressionHandle) bool {
	if l.currentFunc == nil || int(handle) >= len(l.currentFunc.Expressions) {
		return false
	}
	expr := l.currentFunc.Expressions[handle]
	switch kind := expr.Kind.(type) {
	case ir.ExprGlobalVariable:
		if int(kind.Variable) < len(l.module.GlobalVariables) {
			return l.module.GlobalVariables[kind.Variable].Space != ir.SpaceHandle
		}
		return true
	case ir.ExprLocalVariable:
		return true
	case ir.ExprFunctionArgument:
		// Function arguments with pointer type are pointer expressions.
		if l.currentFunc != nil && int(kind.Index) < len(l.currentFunc.Arguments) {
			argType := l.currentFunc.Arguments[kind.Index].Type
			if int(argType) < len(l.module.Types) {
				if _, ok := l.module.Types[argType].Inner.(ir.PointerType); ok {
					return true
				}
			}
		}
		return false
	case ir.ExprAccessIndex:
		return l.isPointerExpressionInLowerer(kind.Base)
	case ir.ExprAccess:
		return l.isPointerExpressionInLowerer(kind.Base)
	default:
		return false
	}
}

// isUnsizedType returns true if the expression's resolved type is unsized
// (e.g., a runtime-sized array). Unsized types cannot be loaded via ExprLoad
// in SPIR-V — they must remain as pointer references.
func (l *Lowerer) isUnsizedType(handle ir.ExpressionHandle) bool {
	if l.currentFunc == nil || int(handle) >= len(l.currentFunc.ExpressionTypes) {
		return false
	}
	resolution := l.currentFunc.ExpressionTypes[handle]
	var inner ir.TypeInner
	if resolution.Handle != nil {
		if int(*resolution.Handle) < len(l.module.Types) {
			inner = l.module.Types[*resolution.Handle].Inner
		}
	} else {
		inner = resolution.Value
	}
	if inner == nil {
		return false
	}
	if arr, ok := inner.(ir.ArrayType); ok {
		return arr.Size.Constant == nil // runtime-sized array
	}
	return false
}

// lowerExpressionForRef lowers an expression in a reference/pointer context.
// Unlike lowerExpression, this does NOT apply the WGSL Load Rule, so the result
// may be a reference (pointer) to a variable. Used for Store targets (assignment LHS),
// address-of (&) operator, and atomic pointer arguments.
func (l *Lowerer) lowerExpressionForRef(expr parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	switch e := expr.(type) {
	case *parser.Ident:
		return l.resolveIdentifier(e.Name)
	case *parser.MemberExpr:
		return l.lowerMemberForRef(e, target)
	case *parser.IndexExpr:
		return l.lowerIndexForRef(e, target)
	case *parser.UnaryExpr:
		if e.Op == parser.TokenStar {
			// *ptr dereference: the operand is a pointer, load it to get the inner pointer
			return l.lowerExpression(e.Operand, target)
		}
		return l.lowerUnary(e, target)
	default:
		// For other expression types, no special reference handling needed
		return l.lowerExpression(expr, target)
	}
}

// lowerMemberForRef lowers a member access expression in reference context.
// The base is kept as a reference (no load rule applied).
func (l *Lowerer) lowerMemberForRef(mem *parser.MemberExpr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	base, err := l.lowerExpressionForRef(mem.Expr, target)
	if err != nil {
		return 0, err
	}

	baseType, err := ir.ResolveExpressionType(l.module, l.currentFunc, base)
	if err != nil {
		return 0, fmt.Errorf("member access base type: %w", err)
	}

	if index, ok, err := l.structMemberIndex(baseType, mem.Member); err != nil {
		return 0, err
	} else if ok {
		return l.addExpression(ir.Expression{
			Kind: ir.ExprAccessIndex{Base: base, Index: index},
		})
	}

	// Single-component vector access (.x, .y, .z, .w) in reference context:
	// Produce AccessIndex on the pointer-based vector, not a Load + Swizzle.
	// This avoids re-lowering the base expression via lowerMember's fallthrough,
	// which would create duplicate GlobalVariable expressions (Bug B).
	vec, vecOk, vecErr := l.vectorType(baseType)
	if vecErr != nil {
		return 0, vecErr
	}
	if vecOk && len(mem.Member) == 1 {
		index, err := l.swizzleIndex(mem.Member, vec.Size)
		if err != nil {
			return 0, err
		}
		return l.addExpression(ir.Expression{
			Kind: ir.ExprAccessIndex{Base: base, Index: index},
		})
	}

	// For multi-component swizzle (.xy, .xyz, etc.), fall through to regular lowering.
	// This WILL re-lower the base expression (creating duplicate expressions), but
	// multi-component swizzles are rare in store targets.
	return l.lowerMember(mem, target)
}

// lowerIndexForRef lowers an index expression in reference context.
// The base is kept as a reference (no load rule applied).
func (l *Lowerer) lowerIndexForRef(idx *parser.IndexExpr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	base, err := l.lowerExpressionForRef(idx.Expr, target)
	if err != nil {
		return 0, err
	}
	return l.lowerIndexWithBase(idx, base, target)
}

// lowerIndexWithBase is the shared implementation for lowerIndex and lowerIndexForRef.
// It resolves the index expression and decides between AccessIndex (compile-time constant)
// and Access (runtime expression). Matches Rust naga's const_eval_expr_to_u32 approach:
// always lower the index expression first (adding it to the arena, which interrupts the
// emitter for needs_pre_emit expressions like Literals), THEN check if it evaluates to
// a compile-time constant u32. This ensures the same emitter interrupt pattern as Rust.
// The intermediate expression is removed by the compact pass after lowering.
func (l *Lowerer) lowerIndexWithBase(idx *parser.IndexExpr, base ir.ExpressionHandle, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	// Always lower the index expression first (matching Rust naga).
	// This adds it to the arena and interrupts the emitter for literals.
	index, err := l.lowerExpression(idx.Index, target)
	if err != nil {
		return 0, err
	}
	// Rust lowers the index with expression(): concretized.
	if index, err = l.concretize(index); err != nil {
		return 0, err
	}

	// Try to evaluate the index as a compile-time constant u32.
	// If successful, use AccessIndex (compile-time constant index).
	// The intermediate index expression remains in the arena but will be
	// removed by compaction since it's not referenced by AccessIndex.
	if val, ok := l.constEvalExprToU32(index); ok {
		return l.addExpression(ir.Expression{
			Kind: ir.ExprAccessIndex{Base: base, Index: val},
		})
	}

	// WGSL array-access-expr: an abstract array, vector or matrix indexed by
	// an expression that is not a const-expression is concretized first.
	if base, err = l.concretize(base); err != nil {
		return 0, err
	}
	return l.addExpression(ir.Expression{
		Kind: ir.ExprAccess{Base: base, Index: index},
	})
}

// emitStart records the current expression index as the start of an emit range.
// Call emitFinish after evaluating a top-level expression tree to create a
// StmtEmit covering all newly added expressions.
func (l *Lowerer) emitStart() ir.ExpressionHandle {
	start := l.currentExprIdx
	l.emitStateStart = &start
	return start
}

// emitStartWithTarget records the emit start and also stores the target block
// so that interruptEmitter can flush the current emit range when creating
// non-emittable expressions (Literal, GlobalVariable, etc.).
func (l *Lowerer) emitStartWithTarget(target *[]ir.Statement) ir.ExpressionHandle {
	l.currentEmitTarget = target
	return l.emitStart()
}

// emitFinish appends a StmtEmit for the range [start, currentExprIdx) if any
// expressions were added since emitStart was called.
// If emitStateStart was advanced by a nested lowerCall (which flushes
// pending sub-expressions before side-effectful statements), the actual
// start is taken from emitStateStart instead of the passed start parameter.
func (l *Lowerer) emitFinish(start ir.ExpressionHandle, target *[]ir.Statement) {
	// Use the tracked emit state start if it was advanced by lowerCall.
	// This handles the case where lowerCall already flushed expressions
	// from [original_start, call_point) and we only need to emit
	// [call_result, currentExprIdx).
	actualStart := start
	if l.emitStateStart != nil && *l.emitStateStart > start {
		actualStart = *l.emitStateStart
	}
	if l.currentExprIdx > actualStart {
		*target = append(*target, ir.Statement{Kind: ir.StmtEmit{
			Range: ir.Range{Start: actualStart, End: l.currentExprIdx},
		}})
	}
	l.emitStateStart = nil
	l.currentEmitTarget = nil
}

// interruptEmitter adds a non-emittable expression (Literal, GlobalVariable,
// Constant, Override, LocalVariable, FunctionArgument, CallResult, etc.)
// while ensuring it falls outside any emit range.
//
// This matches Rust naga's interrupt_emitter pattern: the current emit range
// is flushed (if any emittable expressions were added), then the expression
// is appended, then the emitter is restarted after the new expression.
// The result is that non-emittable expressions are never covered by Emit statements.
func (l *Lowerer) interruptEmitter(expr ir.Expression) (ir.ExpressionHandle, error) {
	// Flush any pending emittable expressions in the current range.
	if l.emitStateStart != nil && l.currentEmitTarget != nil {
		start := *l.emitStateStart
		if l.currentExprIdx > start {
			*l.currentEmitTarget = append(*l.currentEmitTarget, ir.Statement{Kind: ir.StmtEmit{
				Range: ir.Range{Start: start, End: l.currentExprIdx},
			}})
		}
		flushed := l.currentExprIdx
		l.emitStateStart = &flushed
	}

	// Add the expression (outside any emit range), through the evaluator
	// like every other expression (Rust interrupt_emitter -> append_expression).
	handle, err := l.addExpression(expr)
	if err != nil {
		return 0, err
	}

	// Restart the emitter after this expression.
	if l.emitStateStart != nil {
		newStart := l.currentExprIdx
		l.emitStateStart = &newStart
	}

	return handle, nil
}

// countStatementsDeep returns a rough count of statements in a block,
// recursively counting sub-blocks. Used to estimate IR expression count
// for slice pre-allocation.
func countStatementsDeep(block *parser.BlockStmt) int {
	if block == nil {
		return 0
	}
	count := len(block.Statements)
	for _, s := range block.Statements {
		switch stmt := s.(type) {
		case *parser.IfStmt:
			count += countStatementsDeep(stmt.Body)
			if elseBlock, ok := stmt.Else.(*parser.BlockStmt); ok {
				count += countStatementsDeep(elseBlock)
			}
		case *parser.ForStmt:
			count += countStatementsDeep(stmt.Body)
		case *parser.WhileStmt:
			count += countStatementsDeep(stmt.Body)
		case *parser.LoopStmt:
			count += countStatementsDeep(stmt.Body)
		case *parser.SwitchStmt:
			for _, c := range stmt.Cases {
				count += countStatementsDeep(c.Body)
			}
		case *parser.BlockStmt:
			count += countStatementsDeep(stmt)
		}
	}
	return count
}

// ensureBlockReturns ensures every control flow path in a block ends with a Return.
// This matches Rust naga's proc::ensure_block_returns (terminator.rs).
// It recursively descends into the last statement's sub-blocks if they are
// Block, If, or Switch, and appends Return{value: None} where needed.
func ensureBlockReturns(block *[]ir.Statement) {
	if len(*block) == 0 {
		*block = append(*block, ir.Statement{Kind: ir.StmtReturn{}})
		return
	}
	last := &(*block)[len(*block)-1]
	switch s := last.Kind.(type) {
	case ir.StmtBlock:
		ensureBlockReturns((*[]ir.Statement)(&s.Block))
		last.Kind = s
	case ir.StmtIf:
		ensureBlockReturns((*[]ir.Statement)(&s.Accept))
		ensureBlockReturns((*[]ir.Statement)(&s.Reject))
		last.Kind = s
	case ir.StmtSwitch:
		for i := range s.Cases {
			if !s.Cases[i].FallThrough {
				body := s.Cases[i].Body
				ensureBlockReturns((*[]ir.Statement)(&body))
				s.Cases[i].Body = body
			}
		}
		last.Kind = s
	case ir.StmtBreak, ir.StmtContinue, ir.StmtReturn, ir.StmtKill:
		// Already terminates — nothing to do.
	default:
		// Emit, Loop, Store, ImageStore, Call, RayQuery, Atomic, barriers, etc.
		*block = append(*block, ir.Statement{Kind: ir.StmtReturn{}})
	}
}

// addPhonyExpression registers an expression as a phony named expression.
// This is used for WGSL phony assignments (_ = expr) and let _ = expr.
// Matches Rust naga: phony assignments are stored in NamedExpressions with
// the name "phony", and the namer handles collision avoidance (phony_1, etc.).
func (l *Lowerer) addPhonyExpression(handle ir.ExpressionHandle) {
	if l.currentFunc == nil || l.currentFunc.NamedExpressions == nil {
		return
	}
	l.currentFunc.NamedExpressions[handle] = "phony"
}

func scalarValueToLiteral(sv ir.ScalarValue) ir.LiteralValue {
	switch sv.Kind {
	case ir.ScalarBool:
		return ir.LiteralBool(sv.Bits != 0)
	case ir.ScalarSint:
		return ir.LiteralI32(int32(sv.Bits))
	case ir.ScalarUint:
		return ir.LiteralU32(uint32(sv.Bits))
	case ir.ScalarFloat:
		return ir.LiteralF32(math.Float32frombits(uint32(sv.Bits)))
	case ir.ScalarAbstractInt:
		// Concretize: AbstractInt → I32 (WGSL default)
		return ir.LiteralI32(int32(sv.Bits))
	case ir.ScalarAbstractFloat:
		// Concretize: AbstractFloat → F32 (WGSL default)
		return ir.LiteralF32(math.Float32frombits(uint32(sv.Bits)))
	default:
		return nil
	}
}

// scalarValueToLiteralWithType converts a ScalarValue to a LiteralValue,
// using the associated type handle to determine the correct width.
// This is important for f16 values where bits are stored as 16-bit half-precision.
func (l *Lowerer) scalarValueToLiteralWithType(sv ir.ScalarValue, typeHandle ir.TypeHandle) ir.LiteralValue {
	// Get the scalar type width from the type arena
	if int(typeHandle) < len(l.module.Types) {
		if st, ok := l.module.Types[typeHandle].Inner.(ir.ScalarType); ok {
			switch sv.Kind {
			case ir.ScalarFloat, ir.ScalarAbstractFloat:
				switch st.Width {
				case 2:
					return ir.LiteralF16(halfToFloat32(uint16(sv.Bits)))
				case 4:
					return ir.LiteralF32(math.Float32frombits(uint32(sv.Bits)))
				case 8:
					if sv.Kind == ir.ScalarAbstractFloat {
						// Concretize AbstractFloat → F32 when deep-copying to function
						return ir.LiteralF32(math.Float32frombits(uint32(sv.Bits)))
					}
					return ir.LiteralF64(math.Float64frombits(sv.Bits))
				}
			case ir.ScalarSint, ir.ScalarAbstractInt:
				switch st.Width {
				case 8:
					if sv.Kind == ir.ScalarAbstractInt {
						// Concretize AbstractInt → I32 when deep-copying to function
						return ir.LiteralI32(int32(sv.Bits))
					}
					return ir.LiteralI64(int64(sv.Bits))
				default:
					return ir.LiteralI32(int32(sv.Bits))
				}
			case ir.ScalarUint:
				switch st.Width {
				case 8:
					return ir.LiteralU64(sv.Bits)
				default:
					return ir.LiteralU32(uint32(sv.Bits))
				}
			}
		}
	}
	// Fallback to non-type-aware conversion
	return scalarValueToLiteral(sv)
}

func (l *Lowerer) resolveIdentifier(name string) (ir.ExpressionHandle, error) {
	// Check abstract local consts first — re-lower the AST fresh at use site.
	// In Rust naga, the original abstract expression becomes dead and compact
	// removes it. A fresh concretized expression is created at the reference site.
	if ast, ok := l.localAbstractASTs[name]; ok {
		l.usedLocals[name] = true
		handle, err := l.lowerExpression(ast, l.currentEmitTarget)
		if err != nil {
			return 0, err
		}
		return handle, nil
	}

	// Check locals first
	if handle, ok := l.locals[name]; ok {
		// Mark as used for unused variable warnings
		l.usedLocals[name] = true
		return handle, nil
	}

	// Check inline constants (predeclared WGSL values like RAY_FLAG_*).
	// These are inlined as literal expressions, matching Rust naga behavior.
	if lit, ok := l.inlineConstants[name]; ok {
		return l.interruptEmitter(ir.Expression{
			Kind: ir.Literal{Value: lit},
		})
	}

	// Check module-level overrides — interrupt emitter (non-emittable).
	// Override references produce Expression::Override(handle), not Expression::Constant.
	if handle, ok := l.moduleOverrides[name]; ok {
		return l.interruptEmitter(ir.Expression{
			Kind: ir.ExprOverride{Override: handle},
		})
	}

	// Module constants, abstract or not, are Expression::Constant (Rust
	// LoweredGlobalDecl::Const); the evaluator copies the value where it is
	// needed (an operand, a conversion) and an abstract one is converted
	// there.
	if handle, ok := l.moduleConstants[name]; ok {
		return l.interruptEmitter(ir.Expression{
			Kind: ir.ExprConstant{Constant: handle},
		})
	}

	// Check globals — interrupt emitter (non-emittable).
	// Rust naga creates separate GlobalVariable expressions for each reference,
	// each via interrupt_emitter so they fall outside emit ranges.
	if handle, ok := l.globals[name]; ok {
		exprHandle, err := l.interruptEmitter(ir.Expression{
			Kind: ir.ExprGlobalVariable{Variable: handle},
		})
		if err != nil {
			return 0, err
		}
		return exprHandle, nil
	}

	return 0, fmt.Errorf("unresolved identifier: %s", name)
}

// resolveType converts a WGSL type to an IR type handle.
func (l *Lowerer) resolveType(typ parser.Type) (ir.TypeHandle, error) {
	switch t := typ.(type) {
	case *parser.NamedType:
		return l.resolveNamedType(t)
	case *parser.ArrayType:
		base, err := l.resolveType(t.Element)
		if err != nil {
			return 0, err
		}
		// Parse size expression if present
		var size ir.ArraySize
		if t.Size != nil {
			n, err := l.constU32(t.Size)
			switch {
			case errors.Is(err, consteval.ErrOverrideInConst):
				// Rust ArraySize::Pending: the count is the override's
				// value, known once pipeline constants are applied.
				o, err := l.arraySizeOverride(t.Size)
				if err != nil {
					return 0, fmt.Errorf("array size: %w", err)
				}
				size.Pending = &o
			case err != nil:
				return 0, fmt.Errorf("array size: %w", err)
			case n == 0:
				return 0, fmt.Errorf("array size must be greater than 0")
			default:
				size.Constant = &n
			}
		}
		// Compute element stride for SPIR-V ArrayStride decoration.
		// Runtime arrays are always in storage buffers (std430 layout),
		// so stride = roundUp(elemAlign, elemSize).
		elemAlign, elemSize := l.typeAlignmentAndSize(base)
		stride := (elemSize + elemAlign - 1) &^ (elemAlign - 1)
		return l.registerType("", ir.ArrayType{Base: base, Size: size, Stride: stride}), nil
	case *parser.PtrType:
		pointee, err := l.resolveType(t.PointeeType)
		if err != nil {
			return 0, err
		}
		space := l.addressSpace(t.AddressSpace)
		return l.registerType("", ir.PointerType{Base: pointee, Space: space}), nil
	case *parser.BindingArrayType:
		base, err := l.resolveType(t.Element)
		if err != nil {
			return 0, err
		}
		var size *uint32
		if t.Size != nil {
			if lit, ok := t.Size.(*parser.Literal); ok && lit.Kind == parser.TokenIntLiteral {
				n, _ := strconv.ParseUint(lit.Value, 0, 32)
				s := uint32(n)
				size = &s
			}
		}
		return l.registerType("", ir.BindingArrayType{Base: base, Size: size}), nil
	default:
		return 0, fmt.Errorf("unsupported type: %T", typ)
	}
}

func (l *Lowerer) resolveNamedType(t *parser.NamedType) (ir.TypeHandle, error) {
	// Check for built-in vector types
	if len(t.TypeParams) > 0 {
		return l.resolveParameterizedType(t)
	}

	// Look up simple named type
	if handle, ok := l.types[t.Name]; ok {
		return handle, nil
	}

	// Lazy registration of all named types — created on first use only.
	// This matches Rust naga behavior where types are added to the arena on demand,
	// ensuring identical type numbering for Rust reference compatibility.
	switch t.Name {
	case "f32":
		return l.registerType("f32", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}), nil
	case "i32":
		return l.registerType("i32", ir.ScalarType{Kind: ir.ScalarSint, Width: 4}), nil
	case "u32":
		return l.registerType("u32", ir.ScalarType{Kind: ir.ScalarUint, Width: 4}), nil
	case "bool":
		return l.registerType("bool", ir.ScalarType{Kind: ir.ScalarBool, Width: 1}), nil
	case "sampler":
		return l.registerType("sampler", ir.SamplerType{Comparison: false}), nil
	case "sampler_comparison":
		return l.registerType("sampler_comparison", ir.SamplerType{Comparison: true}), nil
	case "f16":
		return l.registerType("f16", ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}), nil
	case "f64":
		return l.registerType("f64", ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}), nil
	case "i64":
		return l.registerType("i64", ir.ScalarType{Kind: ir.ScalarSint, Width: 8}), nil
	case "u64":
		return l.registerType("u64", ir.ScalarType{Kind: ir.ScalarUint, Width: 8}), nil
	case "acceleration_structure":
		return l.registerType("acceleration_structure", ir.AccelerationStructureType{}), nil
	case "ray_query":
		l.registerRayQueryConstants()
		return l.registerType("ray_query", ir.RayQueryType{}), nil
	case "RayDesc":
		l.registerRayDescType()
		return l.types["RayDesc"], nil
	case "RayIntersection":
		l.registerRayIntersectionType()
		return l.types["RayIntersection"], nil
	}

	// Texture types without type parameters (e.g., texture_depth_2d, texture_depth_2d_array)
	if len(t.Name) >= 7 && t.Name[:7] == "texture" {
		imgType := l.parseTextureType(t)
		// When encountering texture_external, generate the special param/transfer types
		// that backends need for lowering external textures to ordinary textures.
		// This mirrors Rust naga's ctx.module.generate_external_texture_types().
		if imgType.Class == ir.ImageClassExternal {
			l.generateExternalTextureTypes()
		}
		return l.registerType("", imgType), nil
	}

	// Check for WGSL predeclared short type aliases (e.g., vec3f -> vec3<f32>)
	if expanded, ok := shortTypeAliases[t.Name]; ok {
		return l.resolveNamedType(&parser.NamedType{
			Name:       expanded.baseName,
			TypeParams: []parser.Type{&parser.NamedType{Name: expanded.scalarName}},
		})
	}

	return 0, fmt.Errorf("unknown type: %s", t.Name)
}

func (l *Lowerer) resolveParameterizedType(t *parser.NamedType) (ir.TypeHandle, error) {
	// Vector types: vec2<f32>, vec3<T>, vec4<T>
	// Rust naga registers the scalar type via resolve_ast_type, then compaction
	// (compact::compact with KeepUnused::Yes at the end of lower()) removes
	// anonymous scalars only embedded in Vector/Matrix. We replicate this by
	// registering the scalar here, and running compactTypes() after lowering.
	if len(t.Name) == 4 && t.Name[:3] == "vec" {
		size := t.Name[3] - '0'
		scalarType, err := l.resolveType(t.TypeParams[0])
		if err != nil {
			return 0, err
		}
		// Get scalar from registry
		typ, ok := l.registry.Lookup(scalarType)
		if !ok {
			return 0, fmt.Errorf("scalar type handle %d not found in registry", scalarType)
		}
		scalar := typ.Inner.(ir.ScalarType)
		return l.registerType("", ir.VectorType{
			Size:   ir.VectorSize(size),
			Scalar: scalar,
		}), nil
	}

	// Matrix types: mat2x2<f32>, mat4x4<f32>
	if len(t.Name) >= 3 && t.Name[:3] == "mat" {
		// Simple parsing: mat4x4 -> 4 columns, 4 rows
		cols := t.Name[3] - '0'
		rows := t.Name[5] - '0'
		scalarType, err := l.resolveType(t.TypeParams[0])
		if err != nil {
			return 0, err
		}
		// Get scalar from registry
		typ, ok := l.registry.Lookup(scalarType)
		if !ok {
			return 0, fmt.Errorf("scalar type handle %d not found in registry", scalarType)
		}
		scalar := typ.Inner.(ir.ScalarType)
		return l.registerType("", ir.MatrixType{
			Columns: ir.VectorSize(cols),
			Rows:    ir.VectorSize(rows),
			Scalar:  scalar,
		}), nil
	}

	// Texture types: texture_2d<f32>, texture_storage_2d<rgba8unorm, write>, etc.
	if len(t.Name) >= 7 && t.Name[:7] == "texture" {
		imgType := l.parseTextureType(t)
		if imgType.Class == ir.ImageClassExternal {
			l.generateExternalTextureTypes()
		}
		return l.registerType("", imgType), nil
	}

	// Atomic types: atomic<u32>, atomic<i32>
	// Rust naga embeds the scalar directly in the Atomic TypeInner without
	// creating a separate type arena entry for it (ast::Type::Atomic(scalar)).
	// We replicate this by resolving the scalar from the type name instead of
	// calling resolveType, which would register a standalone scalar handle and
	// shift all subsequent type handles by one.
	if t.Name == "atomic" {
		if len(t.TypeParams) != 1 {
			return 0, fmt.Errorf("atomic type requires exactly one type parameter")
		}
		scalar, err := l.resolveScalarFromName(t.TypeParams[0])
		if err != nil {
			return 0, err
		}
		return l.registerType("", ir.AtomicType{
			Scalar: scalar,
		}), nil
	}

	return 0, fmt.Errorf("unsupported parameterized type: %s", t.Name)
}

// resolveScalarFromName extracts a scalar type from a type AST node without
// registering it in the type arena. This matches Rust naga behavior where
// atomic<T> embeds the scalar directly (ast::Type::Atomic(scalar)) rather than
// referencing a separate type handle.
func (l *Lowerer) resolveScalarFromName(typ parser.Type) (ir.ScalarType, error) {
	named, ok := typ.(*parser.NamedType)
	if !ok || len(named.TypeParams) != 0 {
		return ir.ScalarType{}, fmt.Errorf("expected scalar type name, got %T", typ)
	}
	switch named.Name {
	case "f32":
		return ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, nil
	case "f16":
		return ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}, nil
	case "f64":
		return ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}, nil
	case "i32":
		return ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, nil
	case "i64":
		return ir.ScalarType{Kind: ir.ScalarSint, Width: 8}, nil
	case "u32":
		return ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, nil
	case "u64":
		return ir.ScalarType{Kind: ir.ScalarUint, Width: 8}, nil
	case "bool":
		return ir.ScalarType{Kind: ir.ScalarBool, Width: 1}, nil
	default:
		return ir.ScalarType{}, fmt.Errorf("unknown scalar type for atomic: %s", named.Name)
	}
}

func (l *Lowerer) isBuiltinConstructor(name string) bool {
	// vec2, vec3, vec4 (NOT vec2f, vec2i — those are short aliases)
	if len(name) == 4 && name[:3] == "vec" {
		return true
	}
	// matNxM where N,M are digits (e.g., mat2x2, mat4x3)
	// NOT mat4x3f, mat2x2h — those are short aliases handled separately
	if len(name) == 6 && name[:3] == "mat" && name[4] == 'x' {
		return true
	}
	return name == "array"
}

// lowerBuiltinConstructor lowers a partial constructor call (vecN(...),
// matCxR(...), array(...)) whose component type is inferred.
func (l *Lowerer) lowerBuiltinConstructor(name string, args []parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	ctor, ok := partialConstructor(name)
	if !ok {
		return 0, fmt.Errorf("unknown constructor %s", name)
	}
	return l.construct(ctor, args, target)
}

// shortTypeAlias represents a WGSL predeclared short type alias.
type shortTypeAlias struct {
	baseName   string // e.g., "vec3", "mat4x4"
	scalarName string // e.g., "f32", "i32", "u32", "f16"
}

// shortTypeAliases maps WGSL predeclared short type names to their expanded forms.
// Per WGSL spec, these are built-in type aliases, not keywords.
var shortTypeAliases = map[string]shortTypeAlias{
	// Vector aliases
	"vec2i": {"vec2", "i32"}, "vec3i": {"vec3", "i32"}, "vec4i": {"vec4", "i32"},
	"vec2u": {"vec2", "u32"}, "vec3u": {"vec3", "u32"}, "vec4u": {"vec4", "u32"},
	"vec2f": {"vec2", "f32"}, "vec3f": {"vec3", "f32"}, "vec4f": {"vec4", "f32"},
	"vec2h": {"vec2", "f16"}, "vec3h": {"vec3", "f16"}, "vec4h": {"vec4", "f16"},
	// Matrix aliases - f32
	"mat2x2f": {"mat2x2", "f32"}, "mat2x3f": {"mat2x3", "f32"}, "mat2x4f": {"mat2x4", "f32"},
	"mat3x2f": {"mat3x2", "f32"}, "mat3x3f": {"mat3x3", "f32"}, "mat3x4f": {"mat3x4", "f32"},
	"mat4x2f": {"mat4x2", "f32"}, "mat4x3f": {"mat4x3", "f32"}, "mat4x4f": {"mat4x4", "f32"},
	// Matrix aliases - f16
	"mat2x2h": {"mat2x2", "f16"}, "mat2x3h": {"mat2x3", "f16"}, "mat2x4h": {"mat2x4", "f16"},
	"mat3x2h": {"mat3x2", "f16"}, "mat3x3h": {"mat3x3", "f16"}, "mat3x4h": {"mat3x4", "f16"},
	"mat4x2h": {"mat4x2", "f16"}, "mat4x3h": {"mat4x3", "f16"}, "mat4x4h": {"mat4x4", "f16"},
}

// mathFuncTable is a package-level lookup table for WGSL math function names to IR math functions.
// Declared at package scope to avoid reallocating the map on every call to getMathFunction.
var mathFuncTable = map[string]ir.MathFunction{
	// Comparison functions
	"abs":      ir.MathAbs,
	"min":      ir.MathMin,
	"max":      ir.MathMax,
	"clamp":    ir.MathClamp,
	"saturate": ir.MathSaturate,

	// Trigonometric functions
	"cos":   ir.MathCos,
	"cosh":  ir.MathCosh,
	"sin":   ir.MathSin,
	"sinh":  ir.MathSinh,
	"tan":   ir.MathTan,
	"tanh":  ir.MathTanh,
	"acos":  ir.MathAcos,
	"asin":  ir.MathAsin,
	"atan":  ir.MathAtan,
	"atan2": ir.MathAtan2,
	"asinh": ir.MathAsinh,
	"acosh": ir.MathAcosh,
	"atanh": ir.MathAtanh,

	// Angle conversion
	"radians": ir.MathRadians,
	"degrees": ir.MathDegrees,

	// Decomposition functions
	"ceil":  ir.MathCeil,
	"floor": ir.MathFloor,
	"round": ir.MathRound,
	"fract": ir.MathFract,
	"trunc": ir.MathTrunc,

	// Exponential functions
	"exp":  ir.MathExp,
	"exp2": ir.MathExp2,
	"log":  ir.MathLog,
	"log2": ir.MathLog2,
	"pow":  ir.MathPow,

	// Geometric functions
	"dot":          ir.MathDot,
	"dot4I8Packed": ir.MathDot4I8Packed,
	"dot4U8Packed": ir.MathDot4U8Packed,
	"cross":        ir.MathCross,
	"distance":     ir.MathDistance,
	"length":       ir.MathLength,
	"normalize":    ir.MathNormalize,
	"faceForward":  ir.MathFaceForward,
	"reflect":      ir.MathReflect,
	"refract":      ir.MathRefract,

	// Computational functions
	"sign":        ir.MathSign,
	"fma":         ir.MathFma,
	"mix":         ir.MathMix,
	"step":        ir.MathStep,
	"smoothstep":  ir.MathSmoothStep,
	"sqrt":        ir.MathSqrt,
	"inverseSqrt": ir.MathInverseSqrt,

	// Matrix functions
	"transpose":   ir.MathTranspose,
	"determinant": ir.MathDeterminant,

	// Bit manipulation functions
	"countTrailingZeros": ir.MathCountTrailingZeros,
	"countLeadingZeros":  ir.MathCountLeadingZeros,
	"countOneBits":       ir.MathCountOneBits,
	"reverseBits":        ir.MathReverseBits,
	"extractBits":        ir.MathExtractBits,
	"insertBits":         ir.MathInsertBits,
	"firstTrailingBit":   ir.MathFirstTrailingBit,
	"firstLeadingBit":    ir.MathFirstLeadingBit,

	// Data packing functions
	"pack4x8snorm":  ir.MathPack4x8snorm,
	"pack4x8unorm":  ir.MathPack4x8unorm,
	"pack2x16snorm": ir.MathPack2x16snorm,
	"pack2x16unorm": ir.MathPack2x16unorm,
	"pack2x16float": ir.MathPack2x16float,

	// Data unpacking functions
	"unpack4x8snorm":  ir.MathUnpack4x8snorm,
	"unpack4x8unorm":  ir.MathUnpack4x8unorm,
	"unpack2x16snorm": ir.MathUnpack2x16snorm,
	"unpack2x16unorm": ir.MathUnpack2x16unorm,
	"unpack2x16float": ir.MathUnpack2x16float,
	"unpack4xI8":      ir.MathUnpack4xI8,
	"unpack4xU8":      ir.MathUnpack4xU8,
	"pack4xI8":        ir.MathPack4xI8,
	"pack4xU8":        ir.MathPack4xU8,
	"pack4xI8Clamp":   ir.MathPack4xI8Clamp,
	"pack4xU8Clamp":   ir.MathPack4xU8Clamp,

	// Decomposition functions (struct return)
	"modf":  ir.MathModf,
	"frexp": ir.MathFrexp,
	"ldexp": ir.MathLdexp,

	// Matrix functions
	"inverse": ir.MathInverse,

	// Precision
	"quantizeToF16": ir.MathQuantizeF16,

	// Vector/matrix operations
	"outerProduct": ir.MathOuter,
}

func (l *Lowerer) getMathFunction(name string) (ir.MathFunction, bool) {
	fn, ok := mathFuncTable[name]
	return fn, ok
}

// isFloatOnlyMathFunc returns true for math functions that only accept float arguments.
// When all arguments are AbstractInt, they must be concretized to F32 (not I32).
func isFloatOnlyMathFunc(f ir.MathFunction) bool {
	switch f {
	case ir.MathPow, ir.MathSqrt, ir.MathInverseSqrt,
		ir.MathExp, ir.MathExp2, ir.MathLog, ir.MathLog2,
		ir.MathCos, ir.MathCosh, ir.MathSin, ir.MathSinh,
		ir.MathTan, ir.MathTanh, ir.MathAcos, ir.MathAsin,
		ir.MathAtan, ir.MathAtan2, ir.MathAsinh, ir.MathAcosh, ir.MathAtanh,
		ir.MathRadians, ir.MathDegrees,
		ir.MathCeil, ir.MathFloor, ir.MathRound, ir.MathFract, ir.MathTrunc,
		ir.MathFma, ir.MathStep, ir.MathSmoothStep, ir.MathMix,
		ir.MathSaturate,
		ir.MathLength, ir.MathDistance, ir.MathNormalize,
		ir.MathFaceForward, ir.MathReflect, ir.MathRefract,
		ir.MathModf, ir.MathFrexp, ir.MathLdexp,
		ir.MathQuantizeF16:
		return true
	default:
		return false
	}
}

func mathMinArgs(f ir.MathFunction) int {
	switch f {
	case ir.MathDot, ir.MathDot4I8Packed, ir.MathDot4U8Packed,
		ir.MathCross, ir.MathDistance, ir.MathReflect,
		ir.MathStep, ir.MathPow, ir.MathAtan2,
		ir.MathMin, ir.MathMax:
		return 2
	case ir.MathMix, ir.MathSmoothStep, ir.MathFma,
		ir.MathClamp, ir.MathRefract, ir.MathFaceForward,
		ir.MathExtractBits, ir.MathInsertBits:
		return 3
	default:
		return 1
	}
}

func (l *Lowerer) lowerMathCall(mathFunc ir.MathFunction, args []parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) == 0 {
		return 0, fmt.Errorf("math function requires at least one argument")
	}
	if minArgs := mathMinArgs(mathFunc); len(args) < minArgs {
		return 0, fmt.Errorf("math function requires at least %d arguments, got %d", minArgs, len(args))
	}

	arg0, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	var arg1, arg2, arg3 *ir.ExpressionHandle
	if len(args) > 1 {
		a, err := l.lowerExpression(args[1], target)
		if err != nil {
			return 0, err
		}
		arg1 = &a
	}
	if len(args) > 2 {
		a, err := l.lowerExpression(args[2], target)
		if err != nil {
			return 0, err
		}
		arg2 = &a
	}
	if len(args) > 3 {
		a, err := l.lowerExpression(args[3], target)
		if err != nil {
			return 0, err
		}
		arg3 = &a
	}

	// WGSL automatic conversions for the call's overload.
	allArgs := []ir.ExpressionHandle{arg0}
	for _, a := range []*ir.ExpressionHandle{arg1, arg2, arg3} {
		if a != nil {
			allArgs = append(allArgs, *a)
		}
	}
	if err := l.convertMathArgs(mathFunc, allArgs); err != nil {
		return 0, err
	}
	arg0 = allArgs[0]
	for i, a := range []**ir.ExpressionHandle{&arg1, &arg2, &arg3} {
		if *a != nil {
			v := allArgs[i+1]
			*a = &v
		}
	}

	// For modf/frexp, create the result struct type so the resolver can find it.
	if mathFunc == ir.MathModf {
		argType := l.resolveExprValueType(arg0)
		structHandle := l.getOrCreateModfResultType(argType)
		handle, err := l.addExpression(ir.Expression{
			Kind: ir.ExprMath{Fun: mathFunc, Arg: arg0},
		})
		if err != nil {
			return 0, err
		}
		// Override the expression type to the struct
		l.currentFunc.ExpressionTypes[handle] = ir.TypeResolution{Handle: &structHandle}
		return handle, nil
	}
	if mathFunc == ir.MathFrexp {
		argType := l.resolveExprValueType(arg0)
		structHandle := l.getOrCreateFrexResultType(argType)
		handle, err := l.addExpression(ir.Expression{
			Kind: ir.ExprMath{Fun: mathFunc, Arg: arg0},
		})
		if err != nil {
			return 0, err
		}
		l.currentFunc.ExpressionTypes[handle] = ir.TypeResolution{Handle: &structHandle}
		return handle, nil
	}

	return l.addExpression(ir.Expression{
		Kind: ir.ExprMath{
			Fun:  mathFunc,
			Arg:  arg0,
			Arg1: arg1,
			Arg2: arg2,
			Arg3: arg3,
		},
	})
}

// roundToF16 converts a float32 value to half-precision (float16) and back,
// rounding to the nearest representable f16 value. This ensures f16 arithmetic
// uses the correct precision, matching Rust naga's half-precision evaluation.
func roundToF16(v float32) float32 {
	bits := math.Float32bits(v)
	sign := bits >> 31
	exp := int((bits>>23)&0xFF) - 127
	mant := bits & 0x7FFFFF

	// Handle special cases
	if exp == 128 { // inf/nan
		if mant != 0 {
			return float32(math.NaN())
		}
		if sign != 0 {
			return float32(math.Inf(-1))
		}
		return float32(math.Inf(1))
	}

	// Subnormal or zero
	if exp < -24 {
		// Too small for f16
		if sign != 0 {
			return -0.0
		}
		return 0.0
	}

	// Overflow
	if exp > 15 {
		if sign != 0 {
			return float32(math.Inf(-1))
		}
		return float32(math.Inf(1))
	}

	// Normal range: round mantissa from 23 bits to 10 bits
	// Add rounding bias (round to nearest even)
	roundBit := uint32(1 << 12) // bit 12 is the rounding position
	mant += roundBit
	if mant >= 0x800000 { // mantissa overflow → increment exponent
		mant = 0
		exp++
		if exp > 15 {
			if sign != 0 {
				return float32(math.Inf(-1))
			}
			return float32(math.Inf(1))
		}
	}
	mant &= 0x7FE000 // keep only top 10 bits of mantissa

	// Reconstruct f32
	result := (sign << 31) | (uint32(exp+127) << 23) | mant
	return math.Float32frombits(result)
}

// lowerSelectCall converts select(falseVal, trueVal, condition) to IR ExprSelect.
// WGSL select() has signature: select(f, t, cond) -- returns t if cond is true, f otherwise.
func (l *Lowerer) lowerSelectCall(args []parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) != 3 {
		return 0, fmt.Errorf("select() requires exactly 3 arguments, got %d", len(args))
	}

	// WGSL: select(falseVal, trueVal, condition)
	falseVal, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	trueVal, err := l.lowerExpression(args[1], target)
	if err != nil {
		return 0, err
	}

	condition, err := l.lowerExpression(args[2], target)
	if err != nil {
		return 0, err
	}
	if condition, err = l.concretize(condition); err != nil {
		return 0, err
	}

	// Rust naga: the values convert to their consensus scalar, concretized
	// unless the condition is a const-expression (then select is evaluated
	// in the abstract type).
	values := []ir.ExpressionHandle{falseVal, trueVal}
	for i, v := range values {
		switch l.exprInner(v).(type) {
		case ir.ScalarType, ir.VectorType:
		default:
			return 0, fmt.Errorf("select() argument %d has type `%s`, which is not a scalar or vector", i+1, l.wgslType(l.exprInner(v)))
		}
	}
	consensus, _, ok := l.conversionConsensus(nil, values)
	if !ok {
		return 0, fmt.Errorf("select() rejected and accepted values have no common type: `%s` and `%s`",
			l.wgslType(l.exprInner(falseVal)), l.wgslType(l.exprInner(trueVal)))
	}
	if !l.eval().IsConst(condition) {
		consensus = concretizeScalar(consensus)
	}
	if err := l.convertSliceToCommonLeafScalar(values, consensus); err != nil {
		return 0, err
	}
	falseVal, trueVal = values[0], values[1]

	return l.addExpression(ir.Expression{
		Kind: ir.ExprSelect{
			Condition: condition,
			Accept:    trueVal,
			Reject:    falseVal,
		},
	})
}

// getDerivativeFunction maps WGSL derivative function names to IR derivative parameters.
func (l *Lowerer) getDerivativeFunction(name string) (ir.ExprDerivative, bool) {
	switch name {
	case "dpdx":
		return ir.ExprDerivative{Axis: ir.DerivativeX, Control: ir.DerivativeNone}, true
	case "dpdy":
		return ir.ExprDerivative{Axis: ir.DerivativeY, Control: ir.DerivativeNone}, true
	case "fwidth":
		return ir.ExprDerivative{Axis: ir.DerivativeWidth, Control: ir.DerivativeNone}, true
	case "dpdxCoarse":
		return ir.ExprDerivative{Axis: ir.DerivativeX, Control: ir.DerivativeCoarse}, true
	case "dpdyCoarse":
		return ir.ExprDerivative{Axis: ir.DerivativeY, Control: ir.DerivativeCoarse}, true
	case "fwidthCoarse":
		return ir.ExprDerivative{Axis: ir.DerivativeWidth, Control: ir.DerivativeCoarse}, true
	case "dpdxFine":
		return ir.ExprDerivative{Axis: ir.DerivativeX, Control: ir.DerivativeFine}, true
	case "dpdyFine":
		return ir.ExprDerivative{Axis: ir.DerivativeY, Control: ir.DerivativeFine}, true
	case "fwidthFine":
		return ir.ExprDerivative{Axis: ir.DerivativeWidth, Control: ir.DerivativeFine}, true
	default:
		return ir.ExprDerivative{}, false
	}
}

func (l *Lowerer) lowerDerivativeCall(deriv ir.ExprDerivative, args []parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) != 1 {
		return 0, fmt.Errorf("derivative function requires exactly 1 argument, got %d", len(args))
	}
	expr, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}
	// Matches Rust naga: expression() concretizes abstract values.
	// Derivative functions accept float types, so concretize AbstractFloat → F32.
	if err := l.concretizeInto(&expr); err != nil {
		return 0, err
	}
	deriv.Expr = expr
	return l.addExpression(ir.Expression{Kind: deriv})
}

// getRelationalFunction maps WGSL relational function names to IR.
func (l *Lowerer) getRelationalFunction(name string) (ir.RelationalFunction, bool) {
	switch name {
	case "all":
		return ir.RelationalAll, true
	case "any":
		return ir.RelationalAny, true
	case "isnan":
		return ir.RelationalIsNan, true
	case "isinf":
		return ir.RelationalIsInf, true
	default:
		return 0, false
	}
}

func (l *Lowerer) lowerRelationalCall(fun ir.RelationalFunction, args []parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) != 1 {
		return 0, fmt.Errorf("relational function requires exactly 1 argument, got %d", len(args))
	}
	arg, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	// any(bool) and all(bool) are no-ops: a scalar bool has no vector to reduce,
	// so the argument is returned unchanged and no Relational reaches the IR.
	// Matches Rust naga ("Check for no-op all(bool) and any(bool)"), which decides
	// on the argument's resolved type, not on whether it is a literal.
	if fun == ir.RelationalAll || fun == ir.RelationalAny {
		if scalar, ok := l.resolveExprTypeInner(arg).(ir.ScalarType); ok && scalar.Kind == ir.ScalarBool {
			return arg, nil
		}
	}

	return l.addExpression(ir.Expression{
		Kind: ir.ExprRelational{Fun: fun, Argument: arg},
	})
}

func (l *Lowerer) lowerArrayLengthCall(args []parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) != 1 {
		return 0, fmt.Errorf("arrayLength requires exactly 1 argument, got %d", len(args))
	}
	arg, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}
	return l.addExpression(ir.Expression{
		Kind: ir.ExprArrayLength{Array: arg},
	})
}

// Attribute parsing

func (l *Lowerer) paramBinding(attrs []parser.Attribute) *ir.Binding {
	return l.collectBinding(attrs)
}

func (l *Lowerer) returnBinding(attrs []parser.Attribute) *ir.Binding {
	return l.collectBinding(attrs)
}

// memberBindings extracts a binding from a list of struct member attributes.
// Unlike the single-attribute memberBinding, this accumulates @location, @blend_src,
// and @interpolate into a single LocationBinding.
func (l *Lowerer) memberBindings(attrs []parser.Attribute) *ir.Binding {
	return l.collectBinding(attrs)
}

// collectBinding processes a list of attributes and returns the combined binding.
// For LocationBinding, it accumulates @location, @blend_src, and @interpolate
// from separate attributes into a single binding.
func (l *Lowerer) collectBinding(attrs []parser.Attribute) *ir.Binding {
	var locBinding *ir.LocationBinding
	var builtinBinding *ir.Binding

	for i := range attrs {
		attr := &attrs[i]
		switch attr.Name {
		case "builtin":
			if len(attr.Args) > 0 {
				if id, ok := attr.Args[0].(*parser.Ident); ok {
					var b ir.Binding = ir.BuiltinBinding{Builtin: l.builtin(id.Name)}
					builtinBinding = &b
				}
			}
		case "location":
			if len(attr.Args) > 0 {
				if lit, ok := attr.Args[0].(*parser.Literal); ok {
					loc, _ := strconv.ParseUint(lit.Value, 10, 32)
					if locBinding == nil {
						locBinding = &ir.LocationBinding{}
					}
					locBinding.Location = uint32(loc)
				}
			}
		case "blend_src":
			if len(attr.Args) > 0 {
				if lit, ok := attr.Args[0].(*parser.Literal); ok {
					idx, _ := strconv.ParseUint(lit.Value, 10, 32)
					if locBinding == nil {
						locBinding = &ir.LocationBinding{}
					}
					v := uint32(idx)
					locBinding.BlendSrc = &v
				}
			}
		case "interpolate":
			interp := l.parseInterpolateAttr(attr)
			if interp != nil {
				if locBinding == nil {
					locBinding = &ir.LocationBinding{}
				}
				locBinding.Interpolation = interp
			}
		}
	}

	// Check for @invariant attribute and apply to Position built-in
	if builtinBinding != nil {
		for i := range attrs {
			if attrs[i].Name == "invariant" {
				if bb, ok := (*builtinBinding).(ir.BuiltinBinding); ok {
					if bb.Builtin == ir.BuiltinPosition {
						bb.Invariant = true
						var b ir.Binding = bb
						builtinBinding = &b
					}
				}
				break
			}
		}
		return builtinBinding
	}
	if locBinding != nil {
		var b ir.Binding = *locBinding
		return &b
	}
	return nil
}

// parseInterpolateAttr parses @interpolate(kind[, sampling]) into an Interpolation.
func (l *Lowerer) parseInterpolateAttr(attr *parser.Attribute) *ir.Interpolation {
	if len(attr.Args) == 0 {
		return nil
	}
	kindIdent, ok := attr.Args[0].(*parser.Ident)
	if !ok {
		return nil
	}

	var kind ir.InterpolationKind
	switch kindIdent.Name {
	case "flat":
		kind = ir.InterpolationFlat
	case "linear":
		kind = ir.InterpolationLinear
	case "perspective":
		kind = ir.InterpolationPerspective
	default:
		return nil
	}

	sampling := ir.SamplingCenter // default
	if len(attr.Args) >= 2 {
		if sampIdent, ok := attr.Args[1].(*parser.Ident); ok {
			switch sampIdent.Name {
			case "center":
				sampling = ir.SamplingCenter
			case "centroid":
				sampling = ir.SamplingCentroid
			case "sample":
				sampling = ir.SamplingSample
			}
		}
	}

	return &ir.Interpolation{
		Kind:     kind,
		Sampling: sampling,
	}
}

// applyDefaultInterpolation applies the default interpolation for a binding
// based on the type, matching Rust naga's Binding::apply_default_interpolation.
//
// For Location bindings with no explicit interpolation:
// - Float scalar/vector/matrix -> Perspective + Center
// - Integer/bool scalar/vector -> Flat (no sampling)
//
// This ensures that integer Location bindings get the Flat decoration in SPIR-V
// (per Vulkan VUID-StandaloneSpirv-Flat-04744) and that the IR matches Rust naga's
// expected interpolation values for all I/O bindings.
// applyDefaultInterpolation modifies a *ir.Binding in place if it is a LocationBinding
// without explicit interpolation. Returns the (possibly modified) binding.
func (l *Lowerer) applyDefaultInterpolation(binding *ir.Binding, typeHandle ir.TypeHandle) *ir.Binding {
	if binding == nil {
		return nil
	}
	loc, ok := (*binding).(ir.LocationBinding)
	if !ok {
		return binding
	}
	// Only apply defaults if interpolation is not already set
	if loc.Interpolation != nil {
		return binding
	}
	if int(typeHandle) >= len(l.module.Types) {
		return binding
	}
	inner := l.module.Types[typeHandle].Inner
	var interp *ir.Interpolation
	switch t := inner.(type) {
	case ir.ScalarType:
		switch t.Kind {
		case ir.ScalarFloat:
			interp = &ir.Interpolation{
				Kind:     ir.InterpolationPerspective,
				Sampling: ir.SamplingCenter,
			}
		case ir.ScalarUint, ir.ScalarSint, ir.ScalarBool:
			interp = &ir.Interpolation{
				Kind: ir.InterpolationFlat,
			}
		}
	case ir.VectorType:
		switch t.Scalar.Kind {
		case ir.ScalarFloat:
			interp = &ir.Interpolation{
				Kind:     ir.InterpolationPerspective,
				Sampling: ir.SamplingCenter,
			}
		case ir.ScalarUint, ir.ScalarSint, ir.ScalarBool:
			interp = &ir.Interpolation{
				Kind: ir.InterpolationFlat,
			}
		}
	case ir.MatrixType:
		// Matrices are always float, so Perspective + Center
		interp = &ir.Interpolation{
			Kind:     ir.InterpolationPerspective,
			Sampling: ir.SamplingCenter,
		}
	}
	if interp != nil {
		loc.Interpolation = interp
		var b ir.Binding = loc
		return &b
	}
	return binding
}

func (l *Lowerer) entryPointStage(attrs []parser.Attribute) *ir.ShaderStage {
	for _, attr := range attrs {
		switch attr.Name {
		case "vertex":
			stage := ir.StageVertex
			return &stage
		case "fragment":
			stage := ir.StageFragment
			return &stage
		case "compute":
			stage := ir.StageCompute
			return &stage
		case "task":
			stage := ir.StageTask
			return &stage
		case "mesh":
			stage := ir.StageMesh
			return &stage
		}
	}
	return nil
}

// extractWorkgroupSize extracts workgroup_size from attributes.
// Returns [x, y, z] where defaults are 1.
// Handles literal values, constant references (TWO, THREE), and simple
// constant expressions (TWO - 1u). Matches Rust naga's const_u32 evaluation.
// extractEarlyDepthTest checks for @early_depth_test attribute and returns the configuration.
// Matches Rust: @early_depth_test(force) → Force, @early_depth_test(less_equal) → Allow(LessEqual).
func (l *Lowerer) extractEarlyDepthTest(attrs []parser.Attribute) *ir.EarlyDepthTest {
	for _, attr := range attrs {
		if attr.Name != "early_depth_test" {
			continue
		}
		edt := &ir.EarlyDepthTest{}
		if len(attr.Args) > 0 {
			if ident, ok := attr.Args[0].(*parser.Ident); ok {
				switch ident.Name {
				case "force":
					edt.Conservative = ir.ConservativeDepthUnchanged
				case "greater_equal":
					edt.Conservative = ir.ConservativeDepthGreaterEqual
				case "less_equal":
					edt.Conservative = ir.ConservativeDepthLessEqual
				case "unchanged":
					edt.Conservative = ir.ConservativeDepthUnchanged
				}
			}
		}
		return edt
	}
	return nil
}

// extractWorkgroupSize returns the workgroup size of an entry point, as
// Rust's lowerer: each size is const_u32 in the module's context; a size that
// is an override-expression is lowered in the override context and kept for
// pipeline creation (workgroup_size_overrides), its literal size staying 1.
// Any other error is reported.
func (l *Lowerer) extractWorkgroupSize(attrs []parser.Attribute) ([3]uint32, [3]*ir.ExpressionHandle, error) {
	result := [3]uint32{1, 1, 1}
	var overrides [3]*ir.ExpressionHandle
	for _, attr := range attrs {
		if attr.Name != "workgroup_size" {
			continue
		}
		for i, arg := range attr.Args {
			if i >= 3 {
				break
			}
			val, err := l.moduleConstU32(arg)
			switch {
			case err == nil:
				result[i] = val
			case errors.Is(err, consteval.ErrOverrideInConst):
				h, err := l.workgroupSizeOverride(arg)
				if err != nil {
					return result, overrides, err
				}
				overrides[i] = &h
			default:
				return result, overrides, err
			}
		}
		break
	}
	return result, overrides, nil
}

// workgroupSizeOverride lowers an override-expression workgroup size in the
// override context; it must be a concrete integer (Rust
// workgroup_size_override).
func (l *Lowerer) workgroupSizeOverride(e parser.Expr) (ir.ExpressionHandle, error) {
	h, _, err := l.lowerOverrideInteger(e)
	return h, err
}

// arraySizeOverride is the override that gives an override-expression array
// size (Rust array_size_override): the override itself for a bare override,
// otherwise a new override without name or id initialized with the
// expression.
func (l *Lowerer) arraySizeOverride(e parser.Expr) (ir.OverrideHandle, error) {
	h, s, err := l.lowerOverrideInteger(e)
	if err != nil {
		return 0, err
	}
	if eo, ok := l.module.GlobalExpressions[h].Kind.(ir.ExprOverride); ok {
		return eo.Override, nil
	}
	o := ir.OverrideHandle(len(l.module.Overrides))
	l.module.Overrides = append(l.module.Overrides, ir.Override{Ty: l.registerType("", s), Init: &h})
	return o, nil
}

// lowerOverrideInteger lowers e in the module's override context and
// concretizes it; it must be a concrete integer scalar.
func (l *Lowerer) lowerOverrideInteger(e parser.Expr) (ir.ExpressionHandle, ir.ScalarType, error) {
	saved := l.enterModuleConstContext()
	l.moduleEval.Behavior = consteval.BehaviorOverride
	defer func() {
		l.moduleEval.Behavior = consteval.BehaviorConst
		l.leaveModuleConstContext(saved)
	}()
	h, err := l.lowerConstExpr(e, nil)
	if err != nil {
		return 0, ir.ScalarType{}, err
	}
	if h, err = l.concretize(h); err != nil {
		return 0, ir.ScalarType{}, err
	}
	s, ok := l.exprInner(h).(ir.ScalarType)
	if !ok || (s.Kind != ir.ScalarSint && s.Kind != ir.ScalarUint) {
		return 0, ir.ScalarType{}, errors.New("must be a const-expression that resolves to a concrete integer scalar (`u32` or `i32`)")
	}
	return h, s, nil
}

// extractTaskPayload extracts the task payload global variable from @payload(varName) attribute.
func (l *Lowerer) extractTaskPayload(attrs []parser.Attribute) *ir.GlobalVariableHandle {
	for _, attr := range attrs {
		if attr.Name != "payload" {
			continue
		}
		if len(attr.Args) < 1 {
			continue
		}
		if ident, ok := attr.Args[0].(*parser.Ident); ok {
			// Look up the global variable by name
			for i, gv := range l.module.GlobalVariables {
				if gv.Name == ident.Name {
					h := ir.GlobalVariableHandle(i)
					return &h
				}
			}
		}
	}
	return nil
}

// extractMeshInfo extracts mesh shader info from @mesh(outputVar) attribute.
// Analyzes the output variable's type to determine topology, max_vertices, max_primitives,
// vertex_output_type, and primitive_output_type.
func (l *Lowerer) extractMeshInfo(attrs []parser.Attribute) *ir.MeshStageInfo {
	for _, attr := range attrs {
		if attr.Name != "mesh" {
			continue
		}
		if len(attr.Args) < 1 {
			continue
		}
		ident, ok := attr.Args[0].(*parser.Ident)
		if !ok {
			continue
		}

		// Find the output variable
		var outputVarHandle ir.GlobalVariableHandle
		var outputVarType ir.TypeHandle
		found := false
		for i, gv := range l.module.GlobalVariables {
			if gv.Name == ident.Name {
				outputVarHandle = ir.GlobalVariableHandle(i)
				outputVarType = gv.Type
				found = true
				break
			}
		}
		if !found {
			continue
		}

		// Analyze the MeshOutput struct type to extract mesh info
		info := &ir.MeshStageInfo{
			OutputVariable: outputVarHandle,
		}

		if int(outputVarType) < len(l.module.Types) {
			meshOutputType := l.module.Types[outputVarType]
			if st, ok := meshOutputType.Inner.(ir.StructType); ok {
				l.analyzeMeshOutputStruct(&st, info)
			}
		}

		return info
	}
	return nil
}

// analyzeMeshOutputStruct extracts mesh info from the MeshOutput struct members.
// It looks at builtin bindings: Vertices (array<VertexOutput, N>), Primitives (array<PrimitiveOutput, N>),
// VertexCount, PrimitiveCount, and determines topology from PrimitiveOutput's index builtin.
func (l *Lowerer) analyzeMeshOutputStruct(st *ir.StructType, info *ir.MeshStageInfo) {
	for _, member := range st.Members {
		if member.Binding == nil {
			continue
		}
		bb, ok := (*member.Binding).(ir.BuiltinBinding)
		if !ok {
			continue
		}
		switch bb.Builtin {
		case ir.BuiltinVertices:
			// vertices: array<VertexOutput, N>
			// Extract N as max_vertices and VertexOutput as vertex_output_type
			if int(member.Type) < len(l.module.Types) {
				arrType := l.module.Types[member.Type]
				if arr, ok := arrType.Inner.(ir.ArrayType); ok {
					info.VertexOutputType = arr.Base
					if arr.Size.Constant != nil {
						info.MaxVertices = *arr.Size.Constant
					}
				}
			}
		case ir.BuiltinPrimitives:
			// primitives: array<PrimitiveOutput, N>
			// Extract N as max_primitives and PrimitiveOutput as primitive_output_type
			if int(member.Type) < len(l.module.Types) {
				arrType := l.module.Types[member.Type]
				if arr, ok := arrType.Inner.(ir.ArrayType); ok {
					info.PrimitiveOutputType = arr.Base
					if arr.Size.Constant != nil {
						info.MaxPrimitives = *arr.Size.Constant
					}
					// Determine topology from PrimitiveOutput struct's index builtin
					if int(arr.Base) < len(l.module.Types) {
						primType := l.module.Types[arr.Base]
						if primSt, ok := primType.Inner.(ir.StructType); ok {
							info.Topology = l.determineMeshTopology(&primSt)
						}
					}
				}
			}
		}
	}
}

// determineMeshTopology determines the mesh output topology from the PrimitiveOutput struct.
// Looks for TriangleIndices (vec3<u32>), LineIndices (vec2<u32>), or PointIndex (u32).
func (l *Lowerer) determineMeshTopology(st *ir.StructType) ir.MeshOutputTopology {
	for _, member := range st.Members {
		if member.Binding == nil {
			continue
		}
		bb, ok := (*member.Binding).(ir.BuiltinBinding)
		if !ok {
			continue
		}
		switch bb.Builtin {
		case ir.BuiltinTriangleIndices:
			return ir.MeshTopologyTriangles
		case ir.BuiltinLineIndices:
			return ir.MeshTopologyLines
		case ir.BuiltinPointIndex:
			return ir.MeshTopologyPoints
		}
	}
	return ir.MeshTopologyTriangles // default
}

// builtinTable maps WGSL builtin names to IR builtin values.
var builtinTable = map[string]ir.BuiltinValue{
	"position":               ir.BuiltinPosition,
	"vertex_index":           ir.BuiltinVertexIndex,
	"instance_index":         ir.BuiltinInstanceIndex,
	"front_facing":           ir.BuiltinFrontFacing,
	"frag_depth":             ir.BuiltinFragDepth,
	"local_invocation_id":    ir.BuiltinLocalInvocationID,
	"local_invocation_index": ir.BuiltinLocalInvocationIndex,
	"global_invocation_id":   ir.BuiltinGlobalInvocationID,
	"workgroup_id":           ir.BuiltinWorkGroupID,
	"num_workgroups":         ir.BuiltinNumWorkGroups,
	"num_subgroups":          ir.BuiltinNumSubgroups,
	"subgroup_id":            ir.BuiltinSubgroupID,
	"subgroup_size":          ir.BuiltinSubgroupSize,
	"subgroup_invocation_id": ir.BuiltinSubgroupInvocationID,
	"barycentric":            ir.BuiltinBarycentric,
	"view_index":             ir.BuiltinViewIndex,
	"primitive_index":        ir.BuiltinPrimitiveIndex,
	"sample_index":           ir.BuiltinSampleIndex,
	"sample_mask":            ir.BuiltinSampleMask,
	"mesh_task_size":         ir.BuiltinMeshTaskSize,
	"cull_primitive":         ir.BuiltinCullPrimitive,
	"point_index":            ir.BuiltinPointIndex,
	"line_indices":           ir.BuiltinLineIndices,
	"triangle_indices":       ir.BuiltinTriangleIndices,
	"vertex_count":           ir.BuiltinVertexCount,
	"vertices":               ir.BuiltinVertices,
	"primitive_count":        ir.BuiltinPrimitiveCount,
	"primitives":             ir.BuiltinPrimitives,
	"clip_distances":         ir.BuiltinClipDistance,
}

func (l *Lowerer) builtin(name string) ir.BuiltinValue {
	if b, ok := builtinTable[name]; ok {
		return b
	}
	return ir.BuiltinPosition // Default
}

// addressSpaceTable maps WGSL address space names to IR address spaces.
var addressSpaceTable = map[string]ir.AddressSpace{
	"function":      ir.SpaceFunction,
	"private":       ir.SpacePrivate,
	"workgroup":     ir.SpaceWorkGroup,
	"uniform":       ir.SpaceUniform,
	"storage":       ir.SpaceStorage,
	"push_constant": ir.SpacePushConstant,
	"task_payload":  ir.SpaceTaskPayload,
	"handle":        ir.SpaceHandle,
	"immediate":     ir.SpaceImmediate,
}

func (l *Lowerer) addressSpace(space string) ir.AddressSpace {
	if s, ok := addressSpaceTable[space]; ok {
		return s
	}
	return ir.SpaceFunction // Default
}

// isOpaqueResourceType checks if a type is an opaque resource (sampler or image/texture).
// These types require SpaceHandle address space (UniformConstant in SPIR-V).
func (l *Lowerer) isOpaqueResourceType(handle ir.TypeHandle) bool {
	if int(handle) >= len(l.module.Types) {
		return false
	}
	typ := l.module.Types[handle]
	switch t := typ.Inner.(type) {
	case ir.SamplerType, ir.ImageType, ir.AccelerationStructureType:
		return true
	case ir.BindingArrayType:
		// Binding arrays of opaque resources (textures, samplers) are Handle space.
		// Binding arrays of buffers (structs, scalars) use the declared space (Storage/Uniform).
		return l.isOpaqueResourceType(t.Base)
	default:
		return false
	}
}

// parseTextureType parses a texture type specification and returns an ImageType.
// Handles: texture_2d<f32>, texture_storage_2d<rgba8unorm, write>, texture_depth_2d, etc.
func (l *Lowerer) parseTextureType(t *parser.NamedType) ir.ImageType {
	name := t.Name
	img := ir.ImageType{
		Dim:   ir.Dim2D,
		Class: ir.ImageClassSampled,
	}

	// Check for storage textures: texture_storage_1d, texture_storage_2d, etc.
	if strings.HasPrefix(name, "texture_storage_") {
		img.Class = ir.ImageClassStorage
		suffix := name[16:] // After "texture_storage_"
		img.Dim = l.parseTextureDimSuffix(suffix)
		img.Arrayed = strings.Contains(suffix, "_array")

		// Parse format and access from type params: <rgba8unorm, write>
		if len(t.TypeParams) >= 1 {
			img.StorageFormat = l.parseStorageFormat(t.TypeParams[0])
		}
		if len(t.TypeParams) >= 2 {
			img.StorageAccess = l.parseStorageAccess(t.TypeParams[1])
		}
		return img
	}

	// Check for depth textures: texture_depth_2d, texture_depth_cube, etc.
	if strings.HasPrefix(name, "texture_depth_") {
		img.Class = ir.ImageClassDepth
		suffix := name[14:] // After "texture_depth_"
		img.Dim = l.parseTextureDimSuffix(suffix)
		img.Arrayed = strings.Contains(suffix, "_array")
		img.Multisampled = strings.Contains(suffix, "multisampled")
		return img
	}

	// Check for external textures: texture_external
	if name == "texture_external" {
		img.Class = ir.ImageClassExternal
		img.Dim = ir.Dim2D
		return img
	}

	// Check for multisampled textures: texture_multisampled_2d
	if strings.HasPrefix(name, "texture_multisampled_") {
		img.Multisampled = true
		suffix := name[21:] // After "texture_multisampled_"
		img.Dim = l.parseTextureDimSuffix(suffix)
		// Parse sampled scalar kind from type parameter: texture_multisampled_2d<u32>
		if len(t.TypeParams) >= 1 {
			img.SampledKind = l.parseSampledScalarKind(t.TypeParams[0])
		}
		return img
	}

	// Regular sampled textures: texture_1d, texture_2d, texture_3d, texture_cube, etc.
	suffix := name[8:] // After "texture_"
	img.Dim = l.parseTextureDimSuffix(suffix)
	img.Arrayed = strings.Contains(suffix, "_array")

	// Parse sampled scalar kind from type parameter: texture_2d<u32>
	if len(t.TypeParams) >= 1 {
		img.SampledKind = l.parseSampledScalarKind(t.TypeParams[0])
	}

	return img
}

// parseTextureDimSuffix parses dimension from suffix like "1d", "2d", "3d", "cube", "2d_array".
func (l *Lowerer) parseTextureDimSuffix(suffix string) ir.ImageDimension {
	if strings.HasPrefix(suffix, "1d") {
		return ir.Dim1D
	}
	if strings.HasPrefix(suffix, "2d") {
		return ir.Dim2D
	}
	if strings.HasPrefix(suffix, "3d") {
		return ir.Dim3D
	}
	if strings.HasPrefix(suffix, "cube") {
		return ir.DimCube
	}
	return ir.Dim2D
}

// parseSampledScalarKind parses the scalar kind from a texture type parameter.
// Maps WGSL type names like "f32", "i32", "u32" to IR ScalarKind.
func (l *Lowerer) parseSampledScalarKind(param parser.Type) ir.ScalarKind {
	if param == nil {
		return ir.ScalarFloat
	}
	named, ok := param.(*parser.NamedType)
	if !ok {
		return ir.ScalarFloat
	}
	switch named.Name {
	case "u32":
		return ir.ScalarUint
	case "i32":
		return ir.ScalarSint
	default:
		return ir.ScalarFloat
	}
}

// parseStorageFormat parses a storage texture format from a type parameter.
// storageFormatTable maps WGSL storage format names to IR storage formats.
var storageFormatTable = map[string]ir.StorageFormat{
	// 8-bit formats
	"r8unorm": ir.StorageFormatR8Unorm,
	"r8snorm": ir.StorageFormatR8Snorm,
	"r8uint":  ir.StorageFormatR8Uint,
	"r8sint":  ir.StorageFormatR8Sint,

	// 16-bit formats
	"r16uint":  ir.StorageFormatR16Uint,
	"r16sint":  ir.StorageFormatR16Sint,
	"r16float": ir.StorageFormatR16Float,
	"rg8unorm": ir.StorageFormatRg8Unorm,
	"rg8snorm": ir.StorageFormatRg8Snorm,
	"rg8uint":  ir.StorageFormatRg8Uint,
	"rg8sint":  ir.StorageFormatRg8Sint,

	// 32-bit formats
	"r32uint":    ir.StorageFormatR32Uint,
	"r32sint":    ir.StorageFormatR32Sint,
	"r32float":   ir.StorageFormatR32Float,
	"rg16uint":   ir.StorageFormatRg16Uint,
	"rg16sint":   ir.StorageFormatRg16Sint,
	"rg16float":  ir.StorageFormatRg16Float,
	"rgba8unorm": ir.StorageFormatRgba8Unorm,
	"rgba8snorm": ir.StorageFormatRgba8Snorm,
	"rgba8uint":  ir.StorageFormatRgba8Uint,
	"rgba8sint":  ir.StorageFormatRgba8Sint,
	"bgra8unorm": ir.StorageFormatBgra8Unorm,

	// Packed 32-bit formats
	"rgb10a2uint":  ir.StorageFormatRgb10a2Uint,
	"rgb10a2unorm": ir.StorageFormatRgb10a2Unorm,
	"rg11b10float": ir.StorageFormatRg11b10Ufloat,

	// 64-bit formats
	"rg32uint":    ir.StorageFormatRg32Uint,
	"rg32sint":    ir.StorageFormatRg32Sint,
	"rg32float":   ir.StorageFormatRg32Float,
	"rgba16uint":  ir.StorageFormatRgba16Uint,
	"rgba16sint":  ir.StorageFormatRgba16Sint,
	"rgba16float": ir.StorageFormatRgba16Float,

	// 128-bit formats
	"rgba32uint":  ir.StorageFormatRgba32Uint,
	"rgba32sint":  ir.StorageFormatRgba32Sint,
	"rgba32float": ir.StorageFormatRgba32Float,

	// Normalized 16-bit per channel formats
	"r16unorm":    ir.StorageFormatR16Unorm,
	"r16snorm":    ir.StorageFormatR16Snorm,
	"rg16unorm":   ir.StorageFormatRg16Unorm,
	"rg16snorm":   ir.StorageFormatRg16Snorm,
	"rgba16unorm": ir.StorageFormatRgba16Unorm,
	"rgba16snorm": ir.StorageFormatRgba16Snorm,

	// 64-bit formats
	"r64uint": ir.StorageFormatR64Uint,
	"r64sint": ir.StorageFormatR64Sint,
}

func (l *Lowerer) parseStorageFormat(param parser.Type) ir.StorageFormat {
	// The format is typically an identifier like "rgba8unorm"
	namedType, ok := param.(*parser.NamedType)
	if !ok {
		return ir.StorageFormatUnknown
	}
	if format, ok := storageFormatTable[namedType.Name]; ok {
		return format
	}
	return ir.StorageFormatUnknown
}

// parseStorageAccess parses a storage texture access mode from a type parameter.
func (l *Lowerer) parseStorageAccess(param parser.Type) ir.StorageAccess {
	namedType, ok := param.(*parser.NamedType)
	if !ok {
		return ir.StorageAccessWrite
	}
	name := namedType.Name
	switch name {
	case "read":
		return ir.StorageAccessRead
	case "write":
		return ir.StorageAccessWrite
	case "read_write":
		return ir.StorageAccessReadWrite
	case "atomic":
		return ir.StorageAccessReadWrite
	default:
		return ir.StorageAccessWrite // Default to write
	}
}

// binaryOpTable maps token kinds to binary operators.
var binaryOpTable = map[parser.TokenKind]ir.BinaryOperator{
	parser.TokenPlus:           ir.BinaryAdd,
	parser.TokenMinus:          ir.BinarySubtract,
	parser.TokenStar:           ir.BinaryMultiply,
	parser.TokenSlash:          ir.BinaryDivide,
	parser.TokenPercent:        ir.BinaryModulo,
	parser.TokenEqualEqual:     ir.BinaryEqual,
	parser.TokenBangEqual:      ir.BinaryNotEqual,
	parser.TokenLess:           ir.BinaryLess,
	parser.TokenLessEqual:      ir.BinaryLessEqual,
	parser.TokenGreater:        ir.BinaryGreater,
	parser.TokenGreaterEqual:   ir.BinaryGreaterEqual,
	parser.TokenAmpAmp:         ir.BinaryLogicalAnd,
	parser.TokenPipePipe:       ir.BinaryLogicalOr,
	parser.TokenAmpersand:      ir.BinaryAnd,
	parser.TokenPipe:           ir.BinaryInclusiveOr,
	parser.TokenCaret:          ir.BinaryExclusiveOr,
	parser.TokenLessLess:       ir.BinaryShiftLeft,
	parser.TokenGreaterGreater: ir.BinaryShiftRight,
}

// unaryOpTable maps token kinds to unary operators.
var unaryOpTable = map[parser.TokenKind]ir.UnaryOperator{
	parser.TokenMinus: ir.UnaryNegate,
	parser.TokenBang:  ir.UnaryLogicalNot,
	parser.TokenTilde: ir.UnaryBitwiseNot,
}

func (l *Lowerer) tokenToBinaryOp(tok parser.TokenKind) ir.BinaryOperator {
	if op, ok := binaryOpTable[tok]; ok {
		return op
	}
	return ir.BinaryAdd // Default
}

func (l *Lowerer) tokenToUnaryOp(tok parser.TokenKind) ir.UnaryOperator {
	if op, ok := unaryOpTable[tok]; ok {
		return op
	}
	return ir.UnaryNegate // Default
}

// checkUnusedVariables reports warnings for local variables that are declared but never used.
func (l *Lowerer) checkUnusedVariables(funcName string) {
	for name, span := range l.localDecls {
		if !l.usedLocals[name] {
			// Variables starting with _ are intentionally unused
			if len(name) > 0 && name[0] == '_' {
				continue
			}
			l.warnings = append(l.warnings, Warning{
				Message: fmt.Sprintf("unused variable '%s' in function '%s'", name, funcName),
				Span:    span,
			})
		}
	}
}

// registerUnusedLetBindings ensures unused let bindings are in NamedExpressions
// so backends emit them as named temporaries. Most let bindings are already
// registered at declaration time in lowerLocalConst. This catches any that
// were missed (e.g., from function arguments or other paths).
func (l *Lowerer) registerUnusedLetBindings() {
	if l.currentFunc == nil || l.currentFunc.NamedExpressions == nil {
		return
	}
	for name, handle := range l.locals {
		// Skip local const declarations — they are inlined, not named expressions.
		// Matches Rust naga where local const is Declared::Const, not in named_expressions.
		if l.localConsts[name] {
			continue
		}
		// Skip var declarations — Rust naga does NOT add var declarations to
		// named_expressions. Only let bindings get named expression treatment.
		// This allows CompactExpressions to remove unused LocalVariable expressions
		// for const-init vars that are never referenced.
		if l.localIsVar[name] {
			continue
		}
		// Skip if the let binding was used
		if l.usedLocals[name] {
			continue
		}
		// Skip variables starting with _ (intentionally unused)
		if len(name) > 0 && name[0] == '_' {
			continue
		}
		// Skip call result expressions — handled by StmtCall directly
		if int(handle) < len(l.currentFunc.Expressions) &&
			isExprCallResult(l.currentFunc.Expressions[handle].Kind) {
			continue
		}
		// Skip bare literals — they are constant-folded and inlined at use sites,
		// matching Rust naga behavior where literal let bindings don't appear as
		// named temporaries.
		if int(handle) < len(l.currentFunc.Expressions) {
			if _, isLiteral := l.currentFunc.Expressions[handle].Kind.(ir.Literal); isLiteral {
				continue
			}
		}
		l.currentFunc.NamedExpressions[handle] = name
	}
}

// assignOpTable maps compound assignment token kinds to binary operators.
var assignOpTable = map[parser.TokenKind]ir.BinaryOperator{
	parser.TokenPlusEqual:           ir.BinaryAdd,
	parser.TokenMinusEqual:          ir.BinarySubtract,
	parser.TokenStarEqual:           ir.BinaryMultiply,
	parser.TokenSlashEqual:          ir.BinaryDivide,
	parser.TokenPercentEqual:        ir.BinaryModulo,
	parser.TokenAmpEqual:            ir.BinaryAnd,
	parser.TokenPipeEqual:           ir.BinaryInclusiveOr,
	parser.TokenCaretEqual:          ir.BinaryExclusiveOr,
	parser.TokenLessLessEqual:       ir.BinaryShiftLeft,
	parser.TokenGreaterGreaterEqual: ir.BinaryShiftRight,
}

func (l *Lowerer) assignOpToBinary(tok parser.TokenKind) ir.BinaryOperator {
	if op, ok := assignOpTable[tok]; ok {
		return op
	}
	return ir.BinaryAdd // Default
}

func (l *Lowerer) structMemberIndex(base ir.TypeResolution, name string) (uint32, bool, error) {
	inner, ok, err := l.resolveTypeInner(base)
	if err != nil {
		return 0, false, err
	}
	if !ok {
		return 0, false, nil
	}
	st, isStruct := inner.(ir.StructType)
	if !isStruct {
		return 0, false, nil
	}
	var idx uint32
	for _, member := range st.Members {
		if member.Name == name {
			return idx, true, nil
		}
		idx++
	}
	return 0, false, fmt.Errorf("struct has no member %q", name)
}

func (l *Lowerer) vectorType(base ir.TypeResolution) (ir.VectorType, bool, error) {
	inner, ok, err := l.resolveTypeInner(base)
	if err != nil {
		return ir.VectorType{}, false, err
	}
	if !ok {
		return ir.VectorType{}, false, nil
	}
	if vec, isVec := inner.(ir.VectorType); isVec {
		return vec, true, nil
	}
	return ir.VectorType{}, false, nil
}

func (l *Lowerer) resolveTypeInner(base ir.TypeResolution) (ir.TypeInner, bool, error) {
	resolvePointer := func(inner ir.TypeInner) (ir.TypeInner, error) {
		// Unwrap PointerType → pointee type
		if pt, ok := inner.(ir.PointerType); ok {
			baseType, ok := l.registry.Lookup(pt.Base)
			if !ok {
				return nil, fmt.Errorf("pointer base type %d out of range", pt.Base)
			}
			return baseType.Inner, nil
		}
		// Unwrap ValuePointerType → VectorType or ScalarType
		if vp, ok := inner.(ir.ValuePointerType); ok {
			if vp.Size != nil {
				return ir.VectorType{Size: *vp.Size, Scalar: vp.Scalar}, nil
			}
			return vp.Scalar, nil
		}
		return inner, nil
	}

	if base.Handle != nil {
		handle := *base.Handle
		typ, ok := l.registry.Lookup(handle)
		if !ok {
			return nil, false, fmt.Errorf("type handle %d out of range", handle)
		}
		inner, err := resolvePointer(typ.Inner)
		if err != nil {
			return nil, false, err
		}
		return inner, true, nil
	}
	if base.Value != nil {
		inner, err := resolvePointer(base.Value)
		if err != nil {
			return nil, false, err
		}
		return inner, true, nil
	}
	return nil, false, nil
}

func (l *Lowerer) swizzleIndex(member string, vecSize ir.VectorSize) (uint32, error) {
	if len(member) != 1 {
		return 0, fmt.Errorf("invalid swizzle %q", member)
	}
	comp, ok := swizzleComponent(member[0])
	if !ok {
		return 0, fmt.Errorf("invalid swizzle component %q", member)
	}
	if uint8(comp) >= uint8(vecSize) {
		return 0, fmt.Errorf("swizzle component %q out of range for vec%v", member, vecSize)
	}
	return uint32(comp), nil
}

func (l *Lowerer) swizzlePattern(member string, vecSize ir.VectorSize) (ir.VectorSize, [4]ir.SwizzleComponent, error) {
	if len(member) < 2 || len(member) > 4 {
		return 0, [4]ir.SwizzleComponent{}, fmt.Errorf("invalid swizzle %q", member)
	}

	// Validate namespace consistency: all components must be xyzw or all rgba.
	// Mixing namespaces (e.g., v.xg) is invalid per WGSL spec.
	// Matches Rust naga: Components::new() validates all chars are same namespace.
	firstNs := swizzleComponentNamespace(member[0])
	if firstNs == swizzleNsNone {
		return 0, [4]ir.SwizzleComponent{}, fmt.Errorf("invalid swizzle component %q", member)
	}
	for i := 1; i < len(member); i++ {
		ns := swizzleComponentNamespace(member[i])
		if ns == swizzleNsNone {
			return 0, [4]ir.SwizzleComponent{}, fmt.Errorf("invalid swizzle component %q", member)
		}
		if ns != firstNs {
			return 0, [4]ir.SwizzleComponent{}, fmt.Errorf("invalid swizzle %q: cannot mix xyzw and rgba components", member)
		}
	}

	var pattern [4]ir.SwizzleComponent
	for i := 0; i < len(member); i++ {
		comp, ok := swizzleComponent(member[i])
		if !ok {
			return 0, [4]ir.SwizzleComponent{}, fmt.Errorf("invalid swizzle component %q", member)
		}
		if uint8(comp) >= uint8(vecSize) {
			return 0, [4]ir.SwizzleComponent{}, fmt.Errorf("swizzle component %q out of range for vec%v", member, vecSize)
		}
		pattern[i] = comp
	}
	var size ir.VectorSize
	switch len(member) {
	case 2:
		size = ir.Vec2
	case 3:
		size = ir.Vec3
	case 4:
		size = ir.Vec4
	default:
		return 0, [4]ir.SwizzleComponent{}, fmt.Errorf("invalid swizzle %q", member)
	}
	return size, pattern, nil
}

// swizzleNamespace identifies which WGSL swizzle namespace a character belongs to.
// WGSL only allows xyzw and rgba — s/t/p/q are GLSL-only and rejected.
type swizzleNamespace int

const (
	swizzleNsNone swizzleNamespace = iota
	swizzleNsXYZW
	swizzleNsRGBA
)

func swizzleComponent(c byte) (ir.SwizzleComponent, bool) {
	switch c {
	case 'x':
		return ir.SwizzleX, true
	case 'y':
		return ir.SwizzleY, true
	case 'z':
		return ir.SwizzleZ, true
	case 'w':
		return ir.SwizzleW, true
	case 'r':
		return ir.SwizzleX, true
	case 'g':
		return ir.SwizzleY, true
	case 'b':
		return ir.SwizzleZ, true
	case 'a':
		return ir.SwizzleW, true
	default:
		return 0, false
	}
}

// swizzleComponentNamespace returns the namespace of a swizzle character.
func swizzleComponentNamespace(c byte) swizzleNamespace {
	switch c {
	case 'x', 'y', 'z', 'w':
		return swizzleNsXYZW
	case 'r', 'g', 'b', 'a':
		return swizzleNsRGBA
	default:
		return swizzleNsNone
	}
}

// inferTypeFromExpression infers a type handle from an expression's resolved type.
// This is used for `let` bindings without explicit type annotations.
func (l *Lowerer) inferTypeFromExpression(handle ir.ExpressionHandle) (ir.TypeHandle, error) {
	if int(handle) >= len(l.currentFunc.ExpressionTypes) {
		return 0, fmt.Errorf("expression %d has no resolved type", handle)
	}

	resolution := l.currentFunc.ExpressionTypes[handle]

	// If it's already a handle, return it
	if resolution.Handle != nil {
		return *resolution.Handle, nil
	}

	// If it's an inline type, register it in the registry
	if resolution.Value != nil {
		return l.registerType("", resolution.Value), nil
	}

	return 0, fmt.Errorf("expression %d has empty type resolution", handle)
}

// isTextureFunction checks if a function name is a texture sampling/loading function.
func (l *Lowerer) isTextureFunction(name string) bool {
	switch name {
	case "textureSample", "textureSampleBias", "textureSampleLevel", "textureSampleGrad",
		"textureSampleCompare", "textureSampleCompareLevel",
		"textureSampleBaseClampToEdge",
		"textureGather", "textureGatherCompare",
		"textureLoad", "textureStore",
		"textureDimensions", "textureNumLevels", "textureNumLayers", "textureNumSamples",
		"textureAtomicMin", "textureAtomicMax", "textureAtomicAdd",
		"textureAtomicAnd", "textureAtomicOr", "textureAtomicXor":
		return true
	}
	return false
}

// lowerTextureCall converts a texture function call to IR.
func (l *Lowerer) lowerTextureCall(name string, args []parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) < 1 {
		return 0, fmt.Errorf("%s requires at least 1 argument", name)
	}

	switch name {
	case "textureSample":
		// textureSample(t, s, coord) or textureSample(t, s, coord, offset)
		return l.lowerTextureSample(args, target, ir.SampleLevelAuto{})

	case "textureSampleBias":
		// textureSampleBias(t, s, coord, [array_index,] bias [, offset])
		// Pass all args — lowerTextureSample lowers image/sampler/coord/array_index first,
		// then we lower bias AFTER to match Rust expression ordering.
		if len(args) < 4 {
			return 0, fmt.Errorf("textureSampleBias requires at least 4 arguments")
		}
		biasIdx := 3
		if l.isTextureArrayed(args[0]) {
			biasIdx = 4
		}
		if biasIdx >= len(args) {
			return 0, fmt.Errorf("textureSampleBias: missing bias argument")
		}
		// Lower image/sampler/coord/array_index first (args before bias + args after bias)
		sampleArgs := append(args[:biasIdx:biasIdx], args[biasIdx+1:]...)
		// Use placeholder level — will be replaced after lowerTextureSample lowers image/sampler/coord
		return l.lowerTextureSampleWithDeferredLevel(sampleArgs, args[biasIdx], "bias", target)

	case "textureSampleLevel":
		// textureSampleLevel(t, s, coord, [array_index,] level [, offset])
		if len(args) < 4 {
			return 0, fmt.Errorf("textureSampleLevel requires at least 4 arguments")
		}
		levelIdx := 3
		if l.isTextureArrayed(args[0]) {
			levelIdx = 4
		}
		if levelIdx >= len(args) {
			return 0, fmt.Errorf("textureSampleLevel: missing level argument")
		}
		sampleArgs := append(args[:levelIdx:levelIdx], args[levelIdx+1:]...)
		levelKind := "level"
		if l.isTextureDepth(args[0]) {
			levelKind = "level_depth"
		}
		return l.lowerTextureSampleWithDeferredLevel(sampleArgs, args[levelIdx], levelKind, target)

	case "textureSampleGrad":
		// textureSampleGrad(t, s, coord, [array_index,] ddx, ddy [, offset])
		if len(args) < 5 {
			return 0, fmt.Errorf("textureSampleGrad requires at least 5 arguments")
		}
		ddxIdx := 3
		if l.isTextureArrayed(args[0]) {
			ddxIdx = 4
		}
		if ddxIdx+1 >= len(args) {
			return 0, fmt.Errorf("textureSampleGrad: missing gradient arguments")
		}
		sampleArgs := append(args[:ddxIdx:ddxIdx], args[ddxIdx+2:]...)
		return l.lowerTextureSampleWithDeferredGrad(sampleArgs, args[ddxIdx], args[ddxIdx+1], target)

	case "textureSampleCompare":
		// textureSampleCompare(t, s, coord, depth_ref) or (t, s, coord, array_index, depth_ref)
		return l.lowerTextureSampleCompare(args, target, ir.SampleLevelAuto{})

	case "textureSampleCompareLevel":
		// textureSampleCompareLevel(t, s, coord, depth_ref) or (t, s, coord, array_index, depth_ref)
		// Always samples at level 0 per WGSL spec.
		return l.lowerTextureSampleCompare(args, target, ir.SampleLevelZero{})

	case "textureSampleBaseClampToEdge":
		// textureSampleBaseClampToEdge(t, s, coord)
		// Samples at level 0 with coordinates clamped to [half_texel, 1-half_texel].
		return l.lowerTextureSampleClampToEdge(args, target)

	case "textureGather":
		// textureGather(component, t, s, coord [, offset])
		// Gathers one component from 4 texels in a 2x2 footprint.
		return l.lowerTextureGather(args, target)

	case "textureGatherCompare":
		// textureGatherCompare(t, s, coord, depth_ref [, offset])
		// Gather with depth comparison (always gathers component 0).
		return l.lowerTextureGatherCompare(args, target)

	case "textureLoad":
		// textureLoad(t, coord, level) or textureLoad(t, coord) for storage textures
		return l.lowerTextureLoad(args, target)

	case "textureStore":
		// textureStore(t, coord, value)
		return l.lowerTextureStore(args, target)

	case "textureAtomicMin", "textureAtomicMax", "textureAtomicAdd",
		"textureAtomicAnd", "textureAtomicOr", "textureAtomicXor":
		return l.lowerTextureAtomic(name, args, target)

	case "textureDimensions":
		// textureDimensions(t) or textureDimensions(t, level)
		return l.lowerTextureQuery(args, target, ir.ImageQuerySize{})

	case "textureNumLevels":
		// textureNumLevels(t)
		return l.lowerTextureQuery(args, target, ir.ImageQueryNumLevels{})

	case "textureNumLayers":
		// textureNumLayers(t)
		return l.lowerTextureQuery(args, target, ir.ImageQueryNumLayers{})

	case "textureNumSamples":
		// textureNumSamples(t)
		return l.lowerTextureQuery(args, target, ir.ImageQueryNumSamples{})

	default:
		return 0, fmt.Errorf("unknown texture function: %s", name)
	}
}

// lowerTextureSample converts a texture sampling call to IR.
// lowerTextureSampleWithDeferredLevel lowers image/sampler/coord/array_index via
// lowerTextureSample, then lowers the level/bias arg AFTER (matching Rust expression order).
func (l *Lowerer) lowerTextureSampleWithDeferredLevel(sampleArgs []parser.Expr, levelArg parser.Expr, kind string, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	// First lower image/sampler/coord/array_index (creates GlobalVariable expressions)
	// by calling lowerTextureSample with a placeholder level.
	// BUT we need to intercept BEFORE addExpression creates ImageSample.
	// Simpler: just lower image/sampler/coord first, then level, then create ImageSample.

	image, err := l.lowerExpression(sampleArgs[0], target)
	if err != nil {
		return 0, err
	}
	sampler, err := l.lowerExpression(sampleArgs[1], target)
	if err != nil {
		return 0, err
	}
	coord, err := l.lowerExpression(sampleArgs[2], target)
	if err != nil {
		return 0, err
	}
	if err := l.convertInto(&coord, scalarF32); err != nil {
		return 0, err
	}

	var arrayIndex *ir.ExpressionHandle
	var offset *ir.ExpressionHandle
	nextArg := 3
	if nextArg < len(sampleArgs) && l.isTextureArrayed(sampleArgs[0]) {
		ai, aiErr := l.lowerExpression(sampleArgs[nextArg], target)
		if aiErr != nil {
			return 0, aiErr
		}
		if err := l.concretizeInto(&ai); err != nil {
			return 0, err
		}
		arrayIndex = &ai
		nextArg++
	}

	// NOW lower level/bias (AFTER image/sampler/coord/array_index)
	levelHandle, err := l.lowerExpression(levelArg, target)
	if err != nil {
		return 0, err
	}

	var level ir.SampleLevel
	switch kind {
	case "bias":
		if err := l.convertInto(&levelHandle, scalarF32); err != nil {
			return 0, err
		}
		level = ir.SampleLevelBias{Bias: levelHandle}
	case "level":
		if err := l.convertInto(&levelHandle, scalarF32); err != nil {
			return 0, err
		}
		level = ir.SampleLevelExact{Level: levelHandle}
	case "level_depth":
		// A depth texture's level is an i32 or u32 (Rust: expression()).
		if err := l.concretizeInto(&levelHandle); err != nil {
			return 0, err
		}
		level = ir.SampleLevelExact{Level: levelHandle}
	}

	// Offset (from remaining sampleArgs)
	if nextArg < len(sampleArgs) {
		off, offErr := l.lowerExpression(sampleArgs[nextArg], target)
		if offErr != nil {
			return 0, offErr
		}
		if err := l.convertInto(&off, scalarI32); err != nil {
			return 0, err
		}
		offset = &off
	}

	return l.addExpression(ir.Expression{
		Kind: ir.ExprImageSample{
			Image:      image,
			Sampler:    sampler,
			Coordinate: coord,
			ArrayIndex: arrayIndex,
			Offset:     offset,
			Level:      level,
		},
	})
}

// lowerTextureSampleWithDeferredGrad is like lowerTextureSampleWithDeferredLevel but for gradients.
func (l *Lowerer) lowerTextureSampleWithDeferredGrad(sampleArgs []parser.Expr, ddxArg, ddyArg parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	image, err := l.lowerExpression(sampleArgs[0], target)
	if err != nil {
		return 0, err
	}
	sampler, err := l.lowerExpression(sampleArgs[1], target)
	if err != nil {
		return 0, err
	}
	coord, err := l.lowerExpression(sampleArgs[2], target)
	if err != nil {
		return 0, err
	}
	if err := l.convertInto(&coord, scalarF32); err != nil {
		return 0, err
	}

	var arrayIndex *ir.ExpressionHandle
	var offset *ir.ExpressionHandle
	nextArg := 3
	if nextArg < len(sampleArgs) && l.isTextureArrayed(sampleArgs[0]) {
		ai, aiErr := l.lowerExpression(sampleArgs[nextArg], target)
		if aiErr != nil {
			return 0, aiErr
		}
		if err := l.concretizeInto(&ai); err != nil {
			return 0, err
		}
		arrayIndex = &ai
		nextArg++
	}

	// Lower gradients AFTER image/sampler/coord
	ddx, err := l.lowerExpression(ddxArg, target)
	if err != nil {
		return 0, err
	}
	if err := l.convertInto(&ddx, scalarF32); err != nil {
		return 0, err
	}
	ddy, err := l.lowerExpression(ddyArg, target)
	if err != nil {
		return 0, err
	}
	if err := l.convertInto(&ddy, scalarF32); err != nil {
		return 0, err
	}

	if nextArg < len(sampleArgs) {
		off, offErr := l.lowerExpression(sampleArgs[nextArg], target)
		if offErr != nil {
			return 0, offErr
		}
		if err := l.convertInto(&off, scalarI32); err != nil {
			return 0, err
		}
		offset = &off
	}

	return l.addExpression(ir.Expression{
		Kind: ir.ExprImageSample{
			Image:      image,
			Sampler:    sampler,
			Coordinate: coord,
			ArrayIndex: arrayIndex,
			Offset:     offset,
			Level:      ir.SampleLevelGradient{X: ddx, Y: ddy},
		},
	})
}

func (l *Lowerer) lowerTextureSample(args []parser.Expr, target *[]ir.Statement, level ir.SampleLevel) (ir.ExpressionHandle, error) {
	// args: texture, sampler, coordinate [, array_index_or_offset] [, offset]
	image, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	sampler, err := l.lowerExpression(args[1], target)
	if err != nil {
		return 0, err
	}

	coord, err := l.lowerExpression(args[2], target)
	if err != nil {
		return 0, err
	}

	// Texture sample coordinates must be float. Convert abstract/concrete int to float.
	// Matches Rust naga's automatic_conversion for texture coordinate arguments.
	if err := l.convertInto(&coord, scalarF32); err != nil {
		return 0, err
	}

	// Check if texture is arrayed to determine how to interpret extra arguments
	var arrayIndex *ir.ExpressionHandle
	var offset *ir.ExpressionHandle
	nextArg := 3
	if nextArg < len(args) && l.isTextureArrayed(args[0]) {
		ai, aiErr := l.lowerExpression(args[nextArg], target)
		if aiErr != nil {
			return 0, aiErr
		}
		if err := l.concretizeInto(&ai); err != nil {
			return 0, err
		}
		arrayIndex = &ai
		nextArg++
	}

	// Parse optional offset argument (const_expr of type vecN<i32>)
	if nextArg < len(args) {
		off, offErr := l.lowerExpression(args[nextArg], target)
		if offErr != nil {
			return 0, offErr
		}
		// Concretize offset to i32 (texture offsets are always signed integer)
		if err := l.convertInto(&off, scalarI32); err != nil {
			return 0, err
		}
		offset = &off
	}

	return l.addExpression(ir.Expression{
		Kind: ir.ExprImageSample{
			Image:      image,
			Sampler:    sampler,
			Coordinate: coord,
			ArrayIndex: arrayIndex,
			Offset:     offset,
			Level:      level,
		},
	})
}

// lowerTextureSampleCompare converts a depth texture comparison sampling call to IR.
// textureSampleCompare(t, s, coord, depth_ref) or (t, s, coord, array_index, depth_ref)
func (l *Lowerer) lowerTextureSampleCompare(args []parser.Expr, target *[]ir.Statement, level ir.SampleLevel) (ir.ExpressionHandle, error) {
	if len(args) < 4 {
		return 0, fmt.Errorf("textureSampleCompare requires at least 4 arguments")
	}

	image, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	sampler, err := l.lowerExpression(args[1], target)
	if err != nil {
		return 0, err
	}

	coord, err := l.lowerExpression(args[2], target)
	if err != nil {
		return 0, err
	}
	if err := l.convertInto(&coord, scalarF32); err != nil {
		return 0, err
	}

	var arrayIndex *ir.ExpressionHandle
	depthRefIdx := 3

	if l.isTextureArrayed(args[0]) && len(args) >= 5 {
		ai, aiErr := l.lowerExpression(args[3], target)
		if aiErr != nil {
			return 0, aiErr
		}
		if err := l.concretizeInto(&ai); err != nil {
			return 0, err
		}
		arrayIndex = &ai
		depthRefIdx = 4
	}

	depthRef, err := l.lowerExpression(args[depthRefIdx], target)
	if err != nil {
		return 0, err
	}
	if err := l.convertInto(&depthRef, scalarF32); err != nil {
		return 0, err
	}

	return l.addExpression(ir.Expression{
		Kind: ir.ExprImageSample{
			Image:      image,
			Sampler:    sampler,
			Coordinate: coord,
			ArrayIndex: arrayIndex,
			Level:      level,
			DepthRef:   &depthRef,
		},
	})
}

// lowerTextureSampleClampToEdge converts textureSampleBaseClampToEdge(t, s, coord) to IR.
// Samples at mip level 0 with coordinates clamped to [half_texel, 1-half_texel].
func (l *Lowerer) lowerTextureSampleClampToEdge(args []parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) < 3 {
		return 0, fmt.Errorf("textureSampleBaseClampToEdge requires at least 3 arguments, got %d", len(args))
	}

	image, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	sampler, err := l.lowerExpression(args[1], target)
	if err != nil {
		return 0, err
	}

	coord, err := l.lowerExpression(args[2], target)
	if err != nil {
		return 0, err
	}
	if err := l.convertInto(&coord, scalarF32); err != nil {
		return 0, err
	}

	return l.addExpression(ir.Expression{
		Kind: ir.ExprImageSample{
			Image:       image,
			Sampler:     sampler,
			Coordinate:  coord,
			Level:       ir.SampleLevelZero{},
			ClampToEdge: true,
		},
	})
}

// lowerTextureGather converts textureGather to IR.
// Two overloads:
//
//	textureGather(component, texture, sampler, coords [, offset])    — sampled/multisampled textures
//	textureGather(texture, sampler, coords [, offset])               — depth textures (component always 0)
func (l *Lowerer) lowerTextureGather(args []parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) < 3 {
		return 0, fmt.Errorf("textureGather requires at least 3 arguments, got %d", len(args))
	}

	// Detect depth texture overload: first arg is a depth texture identifier
	isDepth := l.isTextureDepth(args[0])

	var component ir.SwizzleComponent
	var textureArgIdx int

	if isDepth {
		// Depth overload: textureGather(texture, sampler, coords [, offset])
		component = ir.SwizzleX // always component 0 for depth
		textureArgIdx = 0
	} else {
		// Normal overload: textureGather(component, texture, sampler, coords [, offset])
		if len(args) < 4 {
			return 0, fmt.Errorf("textureGather requires at least 4 arguments for non-depth textures, got %d", len(args))
		}
		var err error
		component, err = l.evalGatherComponent(args[0])
		if err != nil {
			return 0, fmt.Errorf("textureGather component: %w", err)
		}
		textureArgIdx = 1
	}

	image, err := l.lowerExpression(args[textureArgIdx], target)
	if err != nil {
		return 0, err
	}

	sampler, err := l.lowerExpression(args[textureArgIdx+1], target)
	if err != nil {
		return 0, err
	}

	coord, err := l.lowerExpression(args[textureArgIdx+2], target)
	if err != nil {
		return 0, err
	}
	if err := l.convertInto(&coord, scalarF32); err != nil {
		return 0, err
	}

	// Check for array index and optional offset
	var arrayIndex *ir.ExpressionHandle
	var offset *ir.ExpressionHandle
	nextArg := textureArgIdx + 3

	if l.isTextureArrayed(args[textureArgIdx]) && len(args) > nextArg {
		ai, aiErr := l.lowerExpression(args[nextArg], target)
		if aiErr != nil {
			return 0, aiErr
		}
		if err := l.concretizeInto(&ai); err != nil {
			return 0, err
		}
		arrayIndex = &ai
		nextArg++
	}

	if len(args) > nextArg {
		off, offErr := l.lowerExpression(args[nextArg], target)
		if offErr != nil {
			return 0, offErr
		}
		offset = &off
	}

	return l.addExpression(ir.Expression{
		Kind: ir.ExprImageSample{
			Image:      image,
			Sampler:    sampler,
			Coordinate: coord,
			ArrayIndex: arrayIndex,
			Offset:     offset,
			Level:      ir.SampleLevelZero{},
			Gather:     &component,
		},
	})
}

// lowerTextureGatherCompare converts textureGatherCompare(t, s, coord, depth_ref [, offset]) to IR.
// Performs a gather operation with depth comparison, always gathering component 0.
func (l *Lowerer) lowerTextureGatherCompare(args []parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) < 4 {
		return 0, fmt.Errorf("textureGatherCompare requires at least 4 arguments, got %d", len(args))
	}

	image, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	sampler, err := l.lowerExpression(args[1], target)
	if err != nil {
		return 0, err
	}

	coord, err := l.lowerExpression(args[2], target)
	if err != nil {
		return 0, err
	}
	if err := l.convertInto(&coord, scalarF32); err != nil {
		return 0, err
	}

	var arrayIndex *ir.ExpressionHandle
	depthRefIdx := 3

	if l.isTextureArrayed(args[0]) && len(args) >= 5 {
		ai, aiErr := l.lowerExpression(args[3], target)
		if aiErr != nil {
			return 0, aiErr
		}
		if err := l.concretizeInto(&ai); err != nil {
			return 0, err
		}
		arrayIndex = &ai
		depthRefIdx = 4
	}

	depthRef, err := l.lowerExpression(args[depthRefIdx], target)
	if err != nil {
		return 0, err
	}
	if err := l.convertInto(&depthRef, scalarF32); err != nil {
		return 0, err
	}

	// Check for optional offset after depth_ref
	var offset *ir.ExpressionHandle
	if len(args) > depthRefIdx+1 {
		off, offErr := l.lowerExpression(args[depthRefIdx+1], target)
		if offErr != nil {
			return 0, offErr
		}
		offset = &off
	}

	// textureGatherCompare always gathers component X (0)
	gatherComp := ir.SwizzleX

	return l.addExpression(ir.Expression{
		Kind: ir.ExprImageSample{
			Image:      image,
			Sampler:    sampler,
			Coordinate: coord,
			ArrayIndex: arrayIndex,
			Offset:     offset,
			Level:      ir.SampleLevelZero{},
			DepthRef:   &depthRef,
			Gather:     &gatherComp,
		},
	})
}

// evalGatherComponent evaluates the component argument of textureGather.
// The component must be a constant integer expression in range [0, 3].
func (l *Lowerer) evalGatherComponent(expr parser.Expr) (ir.SwizzleComponent, error) {
	val, err := l.constU32(expr)
	if err != nil {
		return 0, fmt.Errorf("component must be a constant expression: %w", err)
	}
	if val > 3 {
		return 0, fmt.Errorf("component index %d out of range [0, 3]", val)
	}
	return ir.SwizzleComponent(val), nil
}

// isTextureArrayed checks if a texture expression refers to an arrayed image type.
func (l *Lowerer) isTextureArrayed(expr parser.Expr) bool {
	ident, ok := expr.(*parser.Ident)
	if !ok {
		return false
	}
	for _, gv := range l.module.GlobalVariables {
		if gv.Name == ident.Name {
			if int(gv.Type) < len(l.module.Types) {
				if img, ok := l.module.Types[gv.Type].Inner.(ir.ImageType); ok {
					return img.Arrayed
				}
			}
			return false
		}
	}
	return false
}

// isTextureDepth checks if a texture expression refers to a depth image type.
func (l *Lowerer) isTextureDepth(expr parser.Expr) bool {
	ident, ok := expr.(*parser.Ident)
	if !ok {
		return false
	}
	for _, gv := range l.module.GlobalVariables {
		if gv.Name == ident.Name {
			if int(gv.Type) < len(l.module.Types) {
				if img, ok := l.module.Types[gv.Type].Inner.(ir.ImageType); ok {
					return img.Class == ir.ImageClassDepth
				}
			}
			return false
		}
	}
	return false
}

// isTextureMultisampled checks if a texture expression refers to a multisampled image type.
func (l *Lowerer) isTextureMultisampled(expr parser.Expr) bool {
	ident, ok := expr.(*parser.Ident)
	if !ok {
		return false
	}
	for _, gv := range l.module.GlobalVariables {
		if gv.Name == ident.Name {
			if int(gv.Type) < len(l.module.Types) {
				if img, ok := l.module.Types[gv.Type].Inner.(ir.ImageType); ok {
					return img.Multisampled
				}
			}
			return false
		}
	}
	return false
}

// getTextureImageType retrieves the ImageType for a texture expression, if available.
func (l *Lowerer) getTextureImageType(expr parser.Expr) (ir.ImageType, bool) {
	ident, ok := expr.(*parser.Ident)
	if !ok {
		return ir.ImageType{}, false
	}
	for _, gv := range l.module.GlobalVariables {
		if gv.Name == ident.Name {
			if int(gv.Type) < len(l.module.Types) {
				if img, ok := l.module.Types[gv.Type].Inner.(ir.ImageType); ok {
					return img, true
				}
			}
			return ir.ImageType{}, false
		}
	}
	return ir.ImageType{}, false
}

// lowerTextureLoad converts a texture load call to IR.
func (l *Lowerer) lowerTextureLoad(args []parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	// textureLoad has different signatures:
	//   textureLoad(t, coords, level)               — sampled textures
	//   textureLoad(t, coords, array_index, level)  — arrayed sampled textures
	//   textureLoad(t, coords, sample_index)         — multisampled textures
	//   textureLoad(t, coords)                       — storage textures
	image, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	coord, err := l.lowerExpression(args[1], target)
	if err != nil {
		return 0, err
	}
	// Rust: the coordinate is lowered with expression() (concretized).
	if err := l.concretizeInto(&coord); err != nil {
		return 0, err
	}

	var arrayIndex *ir.ExpressionHandle
	var sample *ir.ExpressionHandle
	var level *ir.ExpressionHandle

	isArrayed := l.isTextureArrayed(args[0])
	isMultisampled := l.isTextureMultisampled(args[0])

	nextArg := 2

	if isArrayed && nextArg < len(args) {
		ai, aiErr := l.lowerExpression(args[nextArg], target)
		if aiErr != nil {
			return 0, aiErr
		}
		if err := l.concretizeInto(&ai); err != nil {
			return 0, err
		}
		arrayIndex = &ai
		nextArg++
	}

	if isMultisampled && nextArg < len(args) {
		si, siErr := l.lowerExpression(args[nextArg], target)
		if siErr != nil {
			return 0, siErr
		}
		if err := l.concretizeInto(&si); err != nil {
			return 0, err
		}
		sample = &si
		nextArg++
	}

	if !isMultisampled && nextArg < len(args) {
		lv, lvErr := l.lowerExpression(args[nextArg], target)
		if lvErr != nil {
			return 0, lvErr
		}
		if err := l.concretizeInto(&lv); err != nil {
			return 0, err
		}
		level = &lv
	}

	return l.addExpression(ir.Expression{
		Kind: ir.ExprImageLoad{
			Image:      image,
			Coordinate: coord,
			ArrayIndex: arrayIndex,
			Sample:     sample,
			Level:      level,
		},
	})
}

// lowerTextureStore converts a texture store call to IR.
func (l *Lowerer) lowerTextureStore(args []parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	// textureStore has different signatures:
	//   textureStore(t, coords, value)               — non-arrayed storage textures
	//   textureStore(t, coords, array_index, value)  — arrayed storage textures
	if len(args) < 3 {
		return 0, fmt.Errorf("textureStore requires at least 3 arguments")
	}

	image, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	coord, err := l.lowerExpression(args[1], target)
	if err != nil {
		return 0, err
	}
	// Rust: the coordinate is lowered with expression() (concretized).
	if err := l.concretizeInto(&coord); err != nil {
		return 0, err
	}

	var arrayIndex *ir.ExpressionHandle
	nextArg := 2

	if l.isTextureArrayed(args[0]) && nextArg < len(args)-1 {
		ai, aiErr := l.lowerExpression(args[nextArg], target)
		if aiErr != nil {
			return 0, aiErr
		}
		if err := l.concretizeInto(&ai); err != nil {
			return 0, err
		}
		arrayIndex = &ai
		nextArg++
	}

	value, err := l.lowerExpression(args[nextArg], target)
	if err != nil {
		return 0, err
	}

	// The value converts to the scalar of the storage format (Rust
	// expression_with_leaf_scalar for textureStore).
	if imgType, ok := l.getTextureImageType(args[0]); ok && imgType.Class == ir.ImageClassStorage {
		if err := l.convertInto(&value, imgType.StorageFormat.Scalar()); err != nil {
			return 0, err
		}
	}

	// textureStore is a statement, not an expression — flush pending emit range first
	if l.emitStateStart != nil && l.currentEmitTarget != nil {
		start := *l.emitStateStart
		if l.currentExprIdx > start {
			*l.currentEmitTarget = append(*l.currentEmitTarget, ir.Statement{Kind: ir.StmtEmit{
				Range: ir.Range{Start: start, End: l.currentExprIdx},
			}})
		}
		newStart := l.currentExprIdx
		l.emitStateStart = &newStart
	}
	*target = append(*target, ir.Statement{
		Kind: ir.StmtImageStore{
			Image:      image,
			Coordinate: coord,
			ArrayIndex: arrayIndex,
			Value:      value,
		},
	})

	// Return a zero value expression since textureStore doesn't return anything useful
	return l.interruptEmitter(ir.Expression{
		Kind: ir.ExprZeroValue{Type: 0}, // void
	})
}

// lowerTextureAtomic converts a textureAtomic* call to IR.
func (l *Lowerer) lowerTextureAtomic(name string, args []parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) < 3 {
		return 0, fmt.Errorf("%s requires at least 3 arguments", name)
	}

	image, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	coord, err := l.lowerExpression(args[1], target)
	if err != nil {
		return 0, err
	}
	// Rust: the coordinate is lowered with expression() (concretized).
	if err := l.concretizeInto(&coord); err != nil {
		return 0, err
	}

	var arrayIndex *ir.ExpressionHandle
	nextArg := 2

	if l.isTextureArrayed(args[0]) && nextArg < len(args)-1 {
		ai, aiErr := l.lowerExpression(args[nextArg], target)
		if aiErr != nil {
			return 0, aiErr
		}
		if err := l.concretizeInto(&ai); err != nil {
			return 0, err
		}
		arrayIndex = &ai
		nextArg++
	}

	value, err := l.lowerExpression(args[nextArg], target)
	if err != nil {
		return 0, err
	}

	var fun ir.AtomicFunction
	switch name {
	case "textureAtomicMin":
		fun = ir.AtomicMin{}
	case "textureAtomicMax":
		fun = ir.AtomicMax{}
	case "textureAtomicAdd":
		fun = ir.AtomicAdd{}
	case "textureAtomicAnd":
		fun = ir.AtomicAnd{}
	case "textureAtomicOr":
		fun = ir.AtomicInclusiveOr{}
	case "textureAtomicXor":
		fun = ir.AtomicExclusiveOr{}
	}

	*target = append(*target, ir.Statement{
		Kind: ir.StmtImageAtomic{
			Image:      image,
			Coordinate: coord,
			ArrayIndex: arrayIndex,
			Fun:        fun,
			Value:      value,
		},
	})

	// textureAtomic* functions return nothing in WGSL
	return 0, nil
}

// lowerTextureQuery converts a texture query call to IR.
func (l *Lowerer) lowerTextureQuery(args []parser.Expr, target *[]ir.Statement, query ir.ImageQuery) (ir.ExpressionHandle, error) {
	image, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	// For textureDimensions with level argument — concretize abstract int to i32.
	// Rust naga concretizes the level argument (e.g., literal 1 → I32(1)).
	if len(args) > 1 {
		if sizeQuery, ok := query.(ir.ImageQuerySize); ok {
			level, err := l.lowerExpression(args[1], target)
			if err != nil {
				return 0, err
			}
			// Concretize AbstractInt → I32 for image query level
			if int(level) < len(l.currentFunc.Expressions) {
				if lit, ok := l.currentFunc.Expressions[level].Kind.(ir.Literal); ok {
					if ai, ok := lit.Value.(ir.LiteralAbstractInt); ok {
						l.currentFunc.Expressions[level].Kind = ir.Literal{Value: ir.LiteralI32(int32(ai))}
					}
				}
			}
			sizeQuery.Level = &level
			query = sizeQuery
		}
	}

	return l.addExpression(ir.Expression{
		Kind: ir.ExprImageQuery{
			Image: image,
			Query: query,
		},
	})
}

// getBarrierFlags returns barrier flags for a given function name, or 0 if not a barrier.
func (l *Lowerer) getBarrierFlags(name string) ir.BarrierFlags {
	switch name {
	case "workgroupBarrier":
		return ir.BarrierWorkGroup
	case "storageBarrier":
		return ir.BarrierStorage
	case "textureBarrier":
		return ir.BarrierTexture
	case "subgroupBarrier":
		return ir.BarrierSubGroup
	}
	return 0
}

// getAtomicFunction returns the atomic function for a given name, or nil if not an atomic.
func (l *Lowerer) getAtomicFunction(name string) ir.AtomicFunction {
	switch name {
	case "atomicAdd":
		return ir.AtomicAdd{}
	case "atomicSub":
		return ir.AtomicSubtract{}
	case "atomicAnd":
		return ir.AtomicAnd{}
	case "atomicOr":
		return ir.AtomicInclusiveOr{}
	case "atomicXor":
		return ir.AtomicExclusiveOr{}
	case "atomicMin":
		return ir.AtomicMin{}
	case "atomicMax":
		return ir.AtomicMax{}
	case "atomicExchange":
		return ir.AtomicExchange{}
	}
	return nil
}

// lowerAtomicCall converts an atomic function call to IR.
// Atomic functions have the form: atomicOp(&ptr, value) -> old_value
func (l *Lowerer) lowerAtomicCall(atomicFunc ir.AtomicFunction, args []parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) < 2 {
		return 0, fmt.Errorf("atomic function requires at least 2 arguments")
	}

	// First argument is a pointer (passed with &)
	pointer, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	// Second argument is the value
	value, err := l.lowerExpression(args[1], target)
	if err != nil {
		return 0, err
	}

	// Concretize abstract value to match the atomic's element type.
	if err := l.convertAtomicValue(pointer, &value); err != nil {
		return 0, err
	}

	// For 64-bit atomic min/max called as a statement (result discarded),
	// set Result to nil. MSL backend then uses non-fetch versions
	// (atomic_min_explicit, atomic_max_explicit) for 64-bit values.
	// Matches Rust naga: SHADER_INT64_ATOMIC_MIN_MAX support means
	// 64-bit min/max never have result handles, while 32-bit atomics always do.
	is64BitMinMax := false
	if l.isStatement {
		switch atomicFunc.(type) {
		case ir.AtomicMin, ir.AtomicMax:
			// Check if the pointed-to type is 64-bit
			if valueType := l.resolveExprTypeInner(value); valueType != nil {
				if scalar, ok := valueType.(ir.ScalarType); ok && scalar.Width == 8 {
					is64BitMinMax = true
				}
			}
		}
	}

	if is64BitMinMax {
		// Flush pending emit range before the atomic statement.
		// Ensures Load expressions for the value argument are emitted
		// before the StmtAtomic, matching Rust naga's named expression ordering.
		if l.emitStateStart != nil {
			emitStart := *l.emitStateStart
			l.emitFinish(emitStart, target)
		}
		*target = append(*target, ir.Statement{
			Kind: ir.StmtAtomic{
				Pointer: pointer,
				Fun:     atomicFunc,
				Value:   value,
				Result:  nil,
			},
		})
		// Restart emit tracking after the atomic statement.
		newStart := l.currentExprIdx
		l.emitStateStart = &newStart
		l.currentEmitTarget = target
		// Return 0 as dummy handle (won't be used since this is a statement,
		// i.e., isStatement=true and the caller discards the result).
		// Do NOT create a dummy AtomicResult expression — Rust naga doesn't,
		// and extra expressions shift all subsequent expression indices.
		return 0, nil
	}

	// Resolve the atomic element scalar type from the pointer.
	atomicScalar := l.resolveAtomicScalarFromPointer(pointer)
	scalarTypeHandle := l.registerType("", atomicScalar)

	// Create atomic result expression with the scalar type.
	// Use interruptEmitter: AtomicResult is not a computed expression,
	// it's produced by the Atomic statement (matches Rust naga).
	resultHandle, err := l.interruptEmitter(ir.Expression{
		Kind: ir.ExprAtomicResult{Ty: scalarTypeHandle, Comparison: false},
	})
	if err != nil {
		return 0, err
	}

	*target = append(*target, ir.Statement{
		Kind: ir.StmtAtomic{
			Pointer: pointer,
			Fun:     atomicFunc,
			Value:   value,
			Result:  &resultHandle,
		},
	})

	return resultHandle, nil
}

// lowerAtomicStore converts atomicStore(&ptr, value) to IR.
// atomicStore is a statement - it has no return value.
func (l *Lowerer) lowerAtomicStore(args []parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) < 2 {
		return 0, fmt.Errorf("atomicStore requires 2 arguments")
	}

	pointer, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	value, err := l.lowerExpression(args[1], target)
	if err != nil {
		return 0, err
	}

	// Concretize abstract value to match the atomic's element type.
	if err := l.convertAtomicValue(pointer, &value); err != nil {
		return 0, err
	}

	// Rust naga emits a plain Store for atomicStore, not an Atomic statement.
	// See naga/src/front/wgsl/lower/mod.rs around line 2895.
	*target = append(*target, ir.Statement{
		Kind: ir.StmtStore{
			Pointer: pointer,
			Value:   value,
		},
	})

	return 0, nil // No return value
}

// lowerAtomicLoad converts atomicLoad(&ptr) to IR.
// Rust naga lowers atomicLoad to a plain Expression::Load { pointer },
// not to a Statement::Atomic. See naga/src/front/wgsl/lower/mod.rs.
func (l *Lowerer) lowerAtomicLoad(args []parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) < 1 {
		return 0, fmt.Errorf("atomicLoad requires 1 argument")
	}

	pointer, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	// Rust naga emits Expression::Load { pointer } for atomicLoad.
	return l.addExpression(ir.Expression{
		Kind: ir.ExprLoad{Pointer: pointer},
	})
}

// lowerWorkgroupUniformLoad converts workgroupUniformLoad(ptr) to IR.
// WGSL builtin: workgroupUniformLoad(&workgroup_var) -> value with barrier semantics.
// The argument is a pointer — do NOT apply the WGSL load rule.
func (l *Lowerer) lowerWorkgroupUniformLoad(args []parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) != 1 {
		return 0, fmt.Errorf("workgroupUniformLoad requires 1 argument")
	}

	// Use lowerExpressionForRef to preserve the pointer (no load rule).
	// Matches Rust naga which passes the pointer directly to WorkGroupUniformLoad.
	pointer, err := l.lowerExpressionForRef(args[0], target)
	if err != nil {
		return 0, err
	}

	// Use interruptEmitter to ensure WorkGroupUniformLoadResult falls outside
	// emit ranges (it's a pre-emit expression, like AtomicResult and CallResult).
	// Matches Rust naga's interrupt_emitter call for this expression.
	resultHandle, err := l.interruptEmitter(ir.Expression{
		Kind: ir.ExprWorkGroupUniformLoadResult{},
	})
	if err != nil {
		return 0, err
	}

	// Fix up the expression type: interruptEmitter -> addExpression can't resolve
	// WorkGroupUniformLoadResult because the statement hasn't been added yet.
	// Resolve from the pointer type: result is the pointee type of the pointer.
	if l.currentFunc != nil && int(resultHandle) < len(l.currentFunc.ExpressionTypes) {
		ptrType, ptrErr := ir.ResolveExpressionType(l.module, l.currentFunc, pointer)
		if ptrErr == nil {
			ptrInner := ir.TypeResInner(l.module, ptrType)
			if pt, ok := ptrInner.(ir.PointerType); ok {
				h := pt.Base
				// Like Load, a pointer to atomic<T> yields a T (Rust).
				if a, isAtomic := l.module.Types[h].Inner.(ir.AtomicType); isAtomic {
					h = l.registerType("", a.Scalar)
				}
				l.currentFunc.ExpressionTypes[resultHandle] = ir.TypeResolution{Handle: &h}
			}
		}
	}

	*target = append(*target, ir.Statement{
		Kind: ir.StmtWorkGroupUniformLoad{
			Pointer: pointer,
			Result:  resultHandle,
		},
	})

	return resultHandle, nil
}

// getSubgroupOperation maps WGSL subgroup function names to IR operation and collective operation.
func getSubgroupOperation(name string) (ir.SubgroupOperation, ir.CollectiveOperation, bool) {
	switch name {
	case "subgroupAll":
		return ir.SubgroupOperationAll, ir.CollectiveReduce, true
	case "subgroupAny":
		return ir.SubgroupOperationAny, ir.CollectiveReduce, true
	case "subgroupAdd":
		return ir.SubgroupOperationAdd, ir.CollectiveReduce, true
	case "subgroupMul":
		return ir.SubgroupOperationMul, ir.CollectiveReduce, true
	case "subgroupMin":
		return ir.SubgroupOperationMin, ir.CollectiveReduce, true
	case "subgroupMax":
		return ir.SubgroupOperationMax, ir.CollectiveReduce, true
	case "subgroupAnd":
		return ir.SubgroupOperationAnd, ir.CollectiveReduce, true
	case "subgroupOr":
		return ir.SubgroupOperationOr, ir.CollectiveReduce, true
	case "subgroupXor":
		return ir.SubgroupOperationXor, ir.CollectiveReduce, true
	case "subgroupExclusiveAdd":
		return ir.SubgroupOperationAdd, ir.CollectiveExclusiveScan, true
	case "subgroupExclusiveMul":
		return ir.SubgroupOperationMul, ir.CollectiveExclusiveScan, true
	case "subgroupInclusiveAdd":
		return ir.SubgroupOperationAdd, ir.CollectiveInclusiveScan, true
	case "subgroupInclusiveMul":
		return ir.SubgroupOperationMul, ir.CollectiveInclusiveScan, true
	}
	return 0, 0, false
}

// getSubgroupGather maps WGSL subgroup gather function names to a string key.
func getSubgroupGather(name string) (string, bool) {
	switch name {
	case "subgroupBroadcastFirst", "subgroupBroadcast", "subgroupShuffle",
		"subgroupShuffleDown", "subgroupShuffleUp", "subgroupShuffleXor",
		"quadBroadcast":
		return name, true
	}
	return "", false
}

// lowerSubgroupBallot converts subgroupBallot() or subgroupBallot(predicate) to IR.
func (l *Lowerer) lowerSubgroupBallot(args []parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	var predicate *ir.ExpressionHandle
	if len(args) == 1 {
		pred, err := l.lowerExpression(args[0], target)
		if err != nil {
			return 0, err
		}
		predicate = &pred
	}

	// SubgroupBallotResult uses interrupt_emitter in Rust naga — must be outside Emit range.
	resultHandle, err := l.interruptEmitter(ir.Expression{
		Kind: ir.ExprSubgroupBallotResult{},
	})
	if err != nil {
		return 0, err
	}

	*target = append(*target, ir.Statement{
		Kind: ir.StmtSubgroupBallot{
			Result:    resultHandle,
			Predicate: predicate,
		},
	})

	return resultHandle, nil
}

// lowerSubgroupCollectiveOperation converts subgroup collective operations (subgroupAdd, etc.) to IR.
func (l *Lowerer) lowerSubgroupCollectiveOperation(op ir.SubgroupOperation, cop ir.CollectiveOperation, args []parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) < 1 {
		return 0, fmt.Errorf("subgroup collective operation requires 1 argument")
	}

	argument, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	// Resolve the argument type to use as the result type
	argType, err := ir.ResolveExpressionType(l.module, l.currentFunc, argument)
	if err != nil {
		return 0, fmt.Errorf("cannot resolve subgroup argument type: %w", err)
	}

	typeHandle := l.ensureTypeHandle(argType)

	// SubgroupOperationResult uses interrupt_emitter in Rust naga — must be outside Emit range.
	resultHandle, err := l.interruptEmitter(ir.Expression{
		Kind: ir.ExprSubgroupOperationResult{Type: typeHandle},
	})
	if err != nil {
		return 0, err
	}

	*target = append(*target, ir.Statement{
		Kind: ir.StmtSubgroupCollectiveOperation{
			Op:           op,
			CollectiveOp: cop,
			Argument:     argument,
			Result:       resultHandle,
		},
	})

	return resultHandle, nil
}

// lowerSubgroupGather converts subgroup gather operations to IR.
func (l *Lowerer) lowerSubgroupGather(gatherKind string, args []parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) < 1 {
		return 0, fmt.Errorf("subgroup gather requires at least 1 argument")
	}

	argument, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	var mode ir.GatherMode
	switch gatherKind {
	case "subgroupBroadcastFirst":
		mode = ir.GatherBroadcastFirst{}
	case "subgroupBroadcast":
		if len(args) < 2 {
			return 0, fmt.Errorf("subgroupBroadcast requires 2 arguments")
		}
		index, err := l.lowerExpression(args[1], target)
		if err != nil {
			return 0, err
		}
		mode = ir.GatherBroadcast{Index: index}
	case "subgroupShuffle":
		if len(args) < 2 {
			return 0, fmt.Errorf("subgroupShuffle requires 2 arguments")
		}
		index, err := l.lowerExpression(args[1], target)
		if err != nil {
			return 0, err
		}
		mode = ir.GatherShuffle{Index: index}
	case "subgroupShuffleDown":
		if len(args) < 2 {
			return 0, fmt.Errorf("subgroupShuffleDown requires 2 arguments")
		}
		delta, err := l.lowerExpression(args[1], target)
		if err != nil {
			return 0, err
		}
		mode = ir.GatherShuffleDown{Delta: delta}
	case "subgroupShuffleUp":
		if len(args) < 2 {
			return 0, fmt.Errorf("subgroupShuffleUp requires 2 arguments")
		}
		delta, err := l.lowerExpression(args[1], target)
		if err != nil {
			return 0, err
		}
		mode = ir.GatherShuffleUp{Delta: delta}
	case "subgroupShuffleXor":
		if len(args) < 2 {
			return 0, fmt.Errorf("subgroupShuffleXor requires 2 arguments")
		}
		mask, err := l.lowerExpression(args[1], target)
		if err != nil {
			return 0, err
		}
		mode = ir.GatherShuffleXor{Mask: mask}
	case "quadBroadcast":
		if len(args) < 2 {
			return 0, fmt.Errorf("quadBroadcast requires 2 arguments")
		}
		index, err := l.lowerExpression(args[1], target)
		if err != nil {
			return 0, err
		}
		mode = ir.GatherQuadBroadcast{Index: index}
	default:
		return 0, fmt.Errorf("unknown subgroup gather: %s", gatherKind)
	}

	// Resolve the argument type to use as the result type
	argType, err := ir.ResolveExpressionType(l.module, l.currentFunc, argument)
	if err != nil {
		return 0, fmt.Errorf("cannot resolve subgroup gather argument type: %w", err)
	}

	typeHandle := l.ensureTypeHandle(argType)

	// SubgroupOperationResult uses interrupt_emitter in Rust naga — must be outside Emit range.
	resultHandle, err := l.interruptEmitter(ir.Expression{
		Kind: ir.ExprSubgroupOperationResult{Type: typeHandle},
	})
	if err != nil {
		return 0, err
	}

	*target = append(*target, ir.Statement{
		Kind: ir.StmtSubgroupGather{
			Mode:     mode,
			Argument: argument,
			Result:   resultHandle,
		},
	})

	return resultHandle, nil
}

// lowerQuadSwap converts quadSwapX/quadSwapY/quadSwapDiagonal to IR.
func (l *Lowerer) lowerQuadSwap(funcName string, args []parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) < 1 {
		return 0, fmt.Errorf("%s requires 1 argument", funcName)
	}

	argument, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	var dir ir.QuadDirection
	switch funcName {
	case "quadSwapX":
		dir = ir.QuadDirectionX
	case "quadSwapY":
		dir = ir.QuadDirectionY
	case "quadSwapDiagonal":
		dir = ir.QuadDirectionDiagonal
	}

	// Resolve the argument type to use as the result type
	argType, err := ir.ResolveExpressionType(l.module, l.currentFunc, argument)
	if err != nil {
		return 0, fmt.Errorf("cannot resolve quad swap argument type: %w", err)
	}

	typeHandle := l.ensureTypeHandle(argType)

	// SubgroupOperationResult uses interrupt_emitter in Rust naga — must be outside Emit range.
	resultHandle, err := l.interruptEmitter(ir.Expression{
		Kind: ir.ExprSubgroupOperationResult{Type: typeHandle},
	})
	if err != nil {
		return 0, err
	}

	*target = append(*target, ir.Statement{
		Kind: ir.StmtSubgroupGather{
			Mode:     ir.GatherQuadSwap{Direction: dir},
			Argument: argument,
			Result:   resultHandle,
		},
	})

	return resultHandle, nil
}

// ensureTypeHandle returns a TypeHandle for the given TypeResolution.
// If the resolution already has a handle, it is returned directly.
// Otherwise, the type is registered through the type registry for proper deduplication.
func (l *Lowerer) ensureTypeHandle(res ir.TypeResolution) ir.TypeHandle {
	if res.Handle != nil {
		return *res.Handle
	}
	// Register via the type registry (same as registerType but without name mapping)
	return l.registerTypeSilent(res.Value)
}

// lowerAtomicCompareExchange converts atomicCompareExchangeWeak to IR.
// atomicCompareExchangeWeak(ptr, compare, value) -> __atomic_compare_exchange_result<T>
// Rust naga creates a predeclared struct result type with old_value and exchanged fields.
func (l *Lowerer) lowerAtomicCompareExchange(args []parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if len(args) < 3 {
		return 0, fmt.Errorf("atomicCompareExchangeWeak requires 3 arguments")
	}

	// First argument is a pointer
	pointer, err := l.lowerExpression(args[0], target)
	if err != nil {
		return 0, err
	}

	// Second argument is the compare value
	compare, err := l.lowerExpression(args[1], target)
	if err != nil {
		return 0, err
	}

	// Third argument is the new value
	value, err := l.lowerExpression(args[2], target)
	if err != nil {
		return 0, err
	}

	// Concretize abstract arguments to match the atomic's element type.
	if err := l.convertAtomicValue(pointer, &compare); err != nil {
		return 0, err
	}
	if err := l.convertAtomicValue(pointer, &value); err != nil {
		return 0, err
	}

	// Resolve the atomic element scalar type from the pointer.
	atomicScalar := l.resolveAtomicScalarFromPointer(pointer)

	// Create the predeclared __atomic_compare_exchange_result<T> struct type.
	// This matches Rust naga's special_types.predeclared_types entry.
	resultStructType := l.getOrCreateAtomicCompareExchangeResultType(atomicScalar)

	// Create atomic result expression with the struct type.
	// Use interruptEmitter: AtomicResult is produced by the Atomic statement.
	resultHandle, err := l.interruptEmitter(ir.Expression{
		Kind: ir.ExprAtomicResult{Ty: resultStructType, Comparison: true},
	})
	if err != nil {
		return 0, err
	}

	*target = append(*target, ir.Statement{
		Kind: ir.StmtAtomic{
			Pointer: pointer,
			Fun:     ir.AtomicExchange{Compare: &compare},
			Value:   value,
			Result:  &resultHandle,
		},
	})

	return resultHandle, nil
}

// resolveAtomicScalarFromPointer resolves the scalar type of an atomic from its pointer expression.
func (l *Lowerer) resolveAtomicScalarFromPointer(pointer ir.ExpressionHandle) ir.ScalarType {
	scalar := ir.ResolveAtomicPointerScalar(l.module, l.currentFunc, pointer)
	if scalar != nil {
		return *scalar
	}
	// Fallback: try expression types
	if l.currentFunc != nil && int(pointer) < len(l.currentFunc.ExpressionTypes) {
		ptrRes := l.currentFunc.ExpressionTypes[pointer]
		if ptrInner := ir.TypeResInner(l.module, ptrRes); ptrInner != nil {
			if at, ok := ptrInner.(ir.AtomicType); ok {
				return at.Scalar
			}
			if pt, ok := ptrInner.(ir.PointerType); ok {
				if int(pt.Base) < len(l.module.Types) {
					if at, ok := l.module.Types[pt.Base].Inner.(ir.AtomicType); ok {
						return at.Scalar
					}
				}
			}
		}
	}
	return ir.ScalarType{Kind: ir.ScalarUint, Width: 4} // default fallback
}

// getOrCreateAtomicCompareExchangeResultType creates (or reuses) the predeclared
// __atomic_compare_exchange_result<T> struct type for the given scalar.
// Matches Rust naga's predeclared_types handling.
func (l *Lowerer) getOrCreateAtomicCompareExchangeResultType(scalar ir.ScalarType) ir.TypeHandle {
	// Build the name: e.g., "__atomic_compare_exchange_result<Uint,4>"
	kindName := ""
	switch scalar.Kind {
	case ir.ScalarUint:
		kindName = "Uint"
	case ir.ScalarSint:
		kindName = "Sint"
	default:
		kindName = "Float"
	}
	name := fmt.Sprintf("__atomic_compare_exchange_result<%s,%d>", kindName, scalar.Width)

	// Check if already registered
	for i, t := range l.module.Types {
		if t.Name == name {
			return ir.TypeHandle(i)
		}
	}

	// Register bool type first, then scalar type (matching Rust naga's
	// predeclared type registration order where bool comes before the value scalar).
	boolTypeHandle := l.registerType("", ir.ScalarType{Kind: ir.ScalarBool, Width: 1})

	// Register the scalar type (for old_value member)
	scalarTypeHandle := l.registerType("", scalar)

	// Create the struct type (named, matching Rust naga's predeclared types)
	return l.registerNamedType(name, ir.StructType{
		Members: []ir.StructMember{
			{Name: "old_value", Type: scalarTypeHandle, Offset: 0},
			{Name: "exchanged", Type: boolTypeHandle, Offset: uint32(scalar.Width)},
		},
		Span: uint32(scalar.Width) * 2, // Matches Rust: scalar.width * 2
	})
}

// isRayQueryFunction checks if a function name is a ray query builtin.
func (l *Lowerer) isRayQueryFunction(name string) bool {
	switch name {
	case "rayQueryInitialize", "rayQueryProceed",
		"rayQueryGetCommittedIntersection", "rayQueryGetCandidateIntersection",
		"rayQueryGenerateIntersection", "rayQueryConfirmIntersection",
		"rayQueryTerminate":
		return true
	}
	return false
}

// lowerRayQueryCall lowers a ray query builtin function call.
func (l *Lowerer) lowerRayQueryCall(name string, args []parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	l.registerRayQueryConstants()
	l.registerRayDescType()

	switch name {
	case "rayQueryInitialize":
		// rayQueryInitialize(&rq, accel_struct, ray_desc)
		if len(args) != 3 {
			return 0, fmt.Errorf("rayQueryInitialize requires 3 arguments, got %d", len(args))
		}
		query, err := l.lowerRayQueryPointer(args[0], target)
		if err != nil {
			return 0, fmt.Errorf("rayQueryInitialize: %w", err)
		}
		accelStruct, err := l.lowerExpression(args[1], target)
		if err != nil {
			return 0, fmt.Errorf("rayQueryInitialize acceleration structure: %w", err)
		}
		descriptor, err := l.lowerExpression(args[2], target)
		if err != nil {
			return 0, fmt.Errorf("rayQueryInitialize descriptor: %w", err)
		}
		// Flush emitter before RayQuery statement (matching Rust naga:
		// emitter.finish() → push RayQuery → emitter.start())
		if l.emitStateStart != nil && l.currentEmitTarget != nil {
			start := *l.emitStateStart
			if l.currentExprIdx > start {
				*l.currentEmitTarget = append(*l.currentEmitTarget, ir.Statement{Kind: ir.StmtEmit{
					Range: ir.Range{Start: start, End: l.currentExprIdx},
				}})
			}
		}
		*target = append(*target, ir.Statement{
			Kind: ir.StmtRayQuery{
				Query: query,
				Fun: ir.RayQueryInitialize{
					AccelerationStructure: accelStruct,
					Descriptor:            descriptor,
				},
			},
		})
		// Restart emitter after the RayQuery statement
		if l.emitStateStart != nil {
			newStart := l.currentExprIdx
			l.emitStateStart = &newStart
		}
		return 0, nil

	case "rayQueryProceed":
		// rayQueryProceed(&rq) -> bool
		if len(args) != 1 {
			return 0, fmt.Errorf("rayQueryProceed requires 1 argument, got %d", len(args))
		}
		query, err := l.lowerRayQueryPointer(args[0], target)
		if err != nil {
			return 0, fmt.Errorf("rayQueryProceed: %w", err)
		}
		// Rust naga uses interrupt_emitter for RayQueryProceedResult,
		// which flushes the current emit range before the expression.
		// This produces: Emit → RayQueryProceedResult → StmtRayQuery.
		resultHandle, err := l.interruptEmitter(ir.Expression{
			Kind: ir.ExprRayQueryProceedResult{},
		})
		if err != nil {
			return 0, err
		}
		*target = append(*target, ir.Statement{
			Kind: ir.StmtRayQuery{
				Query: query,
				Fun:   ir.RayQueryProceed{Result: resultHandle},
			},
		})
		return resultHandle, nil

	case "rayQueryGetCommittedIntersection":
		// rayQueryGetCommittedIntersection(&rq) -> RayIntersection
		if len(args) != 1 {
			return 0, fmt.Errorf("rayQueryGetCommittedIntersection requires 1 argument, got %d", len(args))
		}
		l.registerRayIntersectionType() // Register RayIntersection on first use
		query, err := l.lowerRayQueryPointer(args[0], target)
		if err != nil {
			return 0, fmt.Errorf("rayQueryGetCommittedIntersection: %w", err)
		}
		return l.addExpression(ir.Expression{
			Kind: ir.ExprRayQueryGetIntersection{Query: query, Committed: true},
		})

	case "rayQueryGetCandidateIntersection":
		// rayQueryGetCandidateIntersection(&rq) -> RayIntersection
		if len(args) != 1 {
			return 0, fmt.Errorf("rayQueryGetCandidateIntersection requires 1 argument, got %d", len(args))
		}
		l.registerRayIntersectionType() // Register RayIntersection on first use
		query, err := l.lowerRayQueryPointer(args[0], target)
		if err != nil {
			return 0, fmt.Errorf("rayQueryGetCandidateIntersection: %w", err)
		}
		return l.addExpression(ir.Expression{
			Kind: ir.ExprRayQueryGetIntersection{Query: query, Committed: false},
		})

	case "rayQueryGenerateIntersection":
		// rayQueryGenerateIntersection(&rq, hit_t)
		if len(args) != 2 {
			return 0, fmt.Errorf("rayQueryGenerateIntersection requires 2 arguments, got %d", len(args))
		}
		query, err := l.lowerRayQueryPointer(args[0], target)
		if err != nil {
			return 0, fmt.Errorf("rayQueryGenerateIntersection: %w", err)
		}
		hitT, err := l.lowerExpression(args[1], target)
		if err != nil {
			return 0, fmt.Errorf("rayQueryGenerateIntersection hit_t: %w", err)
		}
		// hit_t is always f32 — concretize abstract literals
		if err := l.concretizeFloatInto(&hitT); err != nil {
			return 0, err
		}
		*target = append(*target, ir.Statement{
			Kind: ir.StmtRayQuery{
				Query: query,
				Fun:   ir.RayQueryGenerateIntersection{HitT: hitT},
			},
		})
		return 0, nil

	case "rayQueryConfirmIntersection":
		// rayQueryConfirmIntersection(&rq)
		if len(args) != 1 {
			return 0, fmt.Errorf("rayQueryConfirmIntersection requires 1 argument, got %d", len(args))
		}
		query, err := l.lowerRayQueryPointer(args[0], target)
		if err != nil {
			return 0, fmt.Errorf("rayQueryConfirmIntersection: %w", err)
		}
		*target = append(*target, ir.Statement{
			Kind: ir.StmtRayQuery{
				Query: query,
				Fun:   ir.RayQueryConfirmIntersection{},
			},
		})
		return 0, nil

	case "rayQueryTerminate":
		// rayQueryTerminate(&rq)
		if len(args) != 1 {
			return 0, fmt.Errorf("rayQueryTerminate requires 1 argument, got %d", len(args))
		}
		query, err := l.lowerRayQueryPointer(args[0], target)
		if err != nil {
			return 0, fmt.Errorf("rayQueryTerminate: %w", err)
		}
		*target = append(*target, ir.Statement{
			Kind: ir.StmtRayQuery{
				Query: query,
				Fun:   ir.RayQueryTerminate{},
			},
		})
		return 0, nil

	default:
		return 0, fmt.Errorf("unknown ray query function: %s", name)
	}
}

// lowerRayQueryPointer extracts the pointer expression from a &rq argument.
// Ray query builtins take &rq (address-of ray_query variable).
// Returns the LocalVariable handle directly (no load rule applied).
func (l *Lowerer) lowerRayQueryPointer(arg parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	// The argument is typically &rq (UnaryExpr with TokenAmpersand).
	// Use lowerExpressionForRef to avoid applying the load rule,
	// matching Rust naga which keeps the pointer as LocalVariable.
	if unary, ok := arg.(*parser.UnaryExpr); ok && unary.Op == parser.TokenAmpersand {
		return l.lowerExpressionForRef(unary.Operand, target)
	}
	// If not address-of, use reference lowering
	return l.lowerExpressionForRef(arg, target)
}

// buildGlobalExpressions populates Module.GlobalExpressions from Overrides,
// Constants, and GlobalVariable init values. This mirrors Rust naga's
// global_expressions arena where all module-scope init values are stored.
//
// The order follows Rust naga's lowering convention:
// 1. Override init expressions (created during override lowering)
// 2. GlobalVariable init expressions
// 3. Constant init expressions
func (l *Lowerer) buildGlobalExpressions() {
	m := l.module

	// Map from ConstantHandle to ExpressionHandle in GlobalExpressions.
	constToGlobalExpr := make(map[ir.ConstantHandle]ir.ExpressionHandle, len(m.Constants))

	// Helper to add an expression to GlobalExpressions.
	addExpr := func(kind ir.ExpressionKind) ir.ExpressionHandle {
		h := ir.ExpressionHandle(len(m.GlobalExpressions))
		m.GlobalExpressions = append(m.GlobalExpressions, ir.Expression{Kind: kind})
		return h
	}

	// Convert a ConstantValue to a global expression, recursively for composites.
	var convertValue func(ch ir.ConstantHandle, c *ir.Constant) ir.ExpressionHandle
	convertValue = func(ch ir.ConstantHandle, c *ir.Constant) ir.ExpressionHandle {
		if h, ok := constToGlobalExpr[ch]; ok {
			return h
		}

		var h ir.ExpressionHandle
		switch v := c.Value.(type) {
		case ir.ScalarValue:
			h = addExpr(ir.Literal{Value: scalarValueToLiteral(v)})
		case ir.CompositeValue:
			components := make([]ir.ExpressionHandle, len(v.Components))
			for i, compCH := range v.Components {
				if int(compCH) < len(m.Constants) {
					components[i] = convertValue(compCH, &m.Constants[compCH])
				}
			}
			h = addExpr(ir.ExprCompose{
				Type:       c.Type,
				Components: components,
			})
		case ir.ZeroConstantValue:
			h = addExpr(ir.ExprZeroValue{Type: c.Type})
		case nil:
			// Init-based constant: GE already exists via inline path.
			// Use c.Init directly — don't create duplicate GE.
			h = c.Init
		default:
			h = addExpr(ir.ExprZeroValue{Type: c.Type})
		}

		constToGlobalExpr[ch] = h
		if c.Value != nil {
			c.Init = h
		}
		return h
	}

	// Override initializers are already in GlobalExpressions: lowerOverride
	// appends them when the declaration is lowered.

	// Phase 2: Constants — populate constToGlobalExpr from inline-built Init handles.
	// Constants already have Init set when lowered in the module const context.
	// We just need to register their handles so Phase 3 (GlobalVar init) can find them.
	for i := range m.Constants {
		ch := ir.ConstantHandle(i)
		if l.constsWithInlineInit[ch] {
			constToGlobalExpr[ch] = m.Constants[i].Init
		}
	}

	// Phase 3: Set GlobalVariable.InitExpr to point into GlobalExpressions.
	for i := range m.GlobalVariables {
		gv := &m.GlobalVariables[i]
		if gv.Init != nil {
			// Global var init references a constant -> use that constant's global expression.
			if h, ok := constToGlobalExpr[*gv.Init]; ok {
				gv.InitExpr = &h
			}
		}
		// Other initializers already set InitExpr in lowerGlobalVar.
	}
}
