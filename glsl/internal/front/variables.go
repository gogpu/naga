package front

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
}

// builtinSymbol returns the symbol of the builtin variable name denotes, or nil
// when the name is not a builtin variable. Rust naga resolves these lazily and
// declares each in the root scope the first time it is used (variables.rs:96,
// 223); the ordinary lookup runs first here, so a name the shader declared
// itself shadows a builtin of the same name.
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
