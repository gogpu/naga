package wgsl

// dependencyOrder returns declarations sorted in dependency order using
// DFS-based topological sort. This matches Rust naga's visit_ordered()
// from front/wgsl/index.rs — declarations are ordered so that every
// declaration appears after all declarations it references.
//
// When there are no dependencies between declarations, they appear
// in source order (the outer DFS loop iterates in original order).
func dependencyOrder(decls []Decl) []Decl {
	n := len(decls)
	if n == 0 {
		return decls
	}

	// Build name → index map for global declarations.
	nameToIdx := make(map[string]int, n)
	for i, d := range decls {
		name := declName(d)
		if name != "" {
			nameToIdx[name] = i
		}
	}

	// Collect dependencies for each declaration.
	deps := make([][]int, n)
	for i, d := range decls {
		refs := collectDeclDependencies(d)
		for _, ref := range refs {
			if j, ok := nameToIdx[ref]; ok && j != i {
				deps[i] = append(deps[i], j)
			}
		}
	}

	// DFS topological sort (Tarjan-style post-order).
	// Matches Rust naga's DependencySolver::dfs.
	visited := make([]bool, n)
	onStack := make([]bool, n)
	result := make([]Decl, 0, n)

	var dfs func(i int)
	dfs = func(i int) {
		if visited[i] {
			return
		}
		onStack[i] = true

		for _, j := range deps[i] {
			if onStack[j] {
				continue // cycle — skip (Rust reports error, we tolerate)
			}
			if !visited[j] {
				dfs(j)
			}
		}

		onStack[i] = false
		visited[i] = true
		result = append(result, decls[i])
	}

	// Iterate in source order — preserves source order when no dependencies.
	for i := range decls {
		if !visited[i] {
			dfs(i)
		}
	}

	return result
}

// declName returns the name of a declaration, or "" if unnamed.
func declName(d Decl) string {
	switch d := d.(type) {
	case *StructDecl:
		return d.Name
	case *FunctionDecl:
		return d.Name
	case *VarDecl:
		return d.Name
	case *ConstDecl:
		return d.Name
	case *OverrideDecl:
		return d.Name
	case *AliasDecl:
		return d.Name
	default:
		return ""
	}
}

// collectDeclDependencies returns the set of global names referenced by a declaration.
// Matches Rust naga's dependency collection during parsing.
func collectDeclDependencies(d Decl) []string {
	seen := make(map[string]bool)
	var refs []string
	add := func(name string) {
		if name != "" && !seen[name] && !isBuiltinName(name) {
			seen[name] = true
			refs = append(refs, name)
		}
	}

	switch d := d.(type) {
	case *StructDecl:
		for _, m := range d.Members {
			collectTypeRefs(m.Type, add)
		}
	case *FunctionDecl:
		for _, p := range d.Params {
			collectTypeRefs(p.Type, add)
		}
		if d.ReturnType != nil {
			collectTypeRefs(d.ReturnType, add)
		}
		// Body: track locals to avoid false dependencies
		locals := make(map[string]bool)
		for _, p := range d.Params {
			locals[p.Name] = true
		}
		if d.Body != nil {
			collectBlockDeps(d.Body, locals, add)
		}
	case *VarDecl:
		collectTypeRefs(d.Type, add)
		if d.Init != nil {
			collectExprDeps(d.Init, nil, add)
		}
	case *ConstDecl:
		collectTypeRefs(d.Type, add)
		if d.Init != nil {
			collectExprDeps(d.Init, nil, add)
		}
	case *OverrideDecl:
		collectTypeRefs(d.Type, add)
		if d.Init != nil {
			collectExprDeps(d.Init, nil, add)
		}
	case *AliasDecl:
		collectTypeRefs(d.Type, add)
	}
	return refs
}

// collectTypeRefs extracts type name references from a type AST.
func collectTypeRefs(t Type, add func(string)) {
	if t == nil {
		return
	}
	switch t := t.(type) {
	case *NamedType:
		add(t.Name)
		for _, p := range t.TypeParams {
			collectTypeRefs(p, add)
		}
	case *ArrayType:
		collectTypeRefs(t.Element, add)
		if t.Size != nil {
			collectExprDeps(t.Size, nil, add)
		}
	case *PtrType:
		collectTypeRefs(t.PointeeType, add)
	}
}

// collectExprDeps extracts identifier references from an expression AST.
func collectExprDeps(e Expr, locals map[string]bool, add func(string)) {
	if e == nil {
		return
	}
	switch e := e.(type) {
	case *Ident:
		if locals == nil || !locals[e.Name] {
			add(e.Name)
		}
	case *BinaryExpr:
		collectExprDeps(e.Left, locals, add)
		collectExprDeps(e.Right, locals, add)
	case *UnaryExpr:
		collectExprDeps(e.Operand, locals, add)
	case *CallExpr:
		add(e.Func.Name)
		for _, a := range e.Args {
			collectExprDeps(a, locals, add)
		}
	case *ConstructExpr:
		collectTypeRefs(e.Type, add)
		for _, a := range e.Args {
			collectExprDeps(a, locals, add)
		}
	case *IndexExpr:
		collectExprDeps(e.Expr, locals, add)
		collectExprDeps(e.Index, locals, add)
	case *MemberExpr:
		collectExprDeps(e.Expr, locals, add)
	}
}

// collectBlockDeps extracts dependencies from a block statement.
func collectBlockDeps(block *BlockStmt, locals map[string]bool, add func(string)) {
	if block == nil {
		return
	}
	for _, s := range block.Statements {
		collectStmtDeps(s, locals, add)
	}
}

// collectStmtDeps extracts identifier references from a statement.
func collectStmtDeps(s Stmt, locals map[string]bool, add func(string)) {
	if s == nil {
		return
	}
	switch s := s.(type) {
	case *VarDecl:
		collectTypeRefs(s.Type, add)
		if s.Init != nil {
			collectExprDeps(s.Init, locals, add)
		}
		locals[s.Name] = true
	case *ConstDecl:
		collectTypeRefs(s.Type, add)
		if s.Init != nil {
			collectExprDeps(s.Init, locals, add)
		}
		locals[s.Name] = true
	case *AssignStmt:
		collectExprDeps(s.Left, locals, add)
		collectExprDeps(s.Right, locals, add)
	case *ReturnStmt:
		if s.Value != nil {
			collectExprDeps(s.Value, locals, add)
		}
	case *IfStmt:
		collectExprDeps(s.Condition, locals, add)
		collectBlockDeps(s.Body, locals, add)
		if s.Else != nil {
			collectStmtDeps(s.Else, locals, add)
		}
	case *BlockStmt:
		collectBlockDeps(s, locals, add)
	case *ForStmt:
		if s.Init != nil {
			collectStmtDeps(s.Init, locals, add)
		}
		if s.Condition != nil {
			collectExprDeps(s.Condition, locals, add)
		}
		if s.Update != nil {
			collectStmtDeps(s.Update, locals, add)
		}
		collectBlockDeps(s.Body, locals, add)
	case *WhileStmt:
		collectExprDeps(s.Condition, locals, add)
		collectBlockDeps(s.Body, locals, add)
	case *LoopStmt:
		collectBlockDeps(s.Body, locals, add)
		collectBlockDeps(s.Continuing, locals, add)
	case *SwitchStmt:
		collectExprDeps(s.Selector, locals, add)
		for _, c := range s.Cases {
			for _, sel := range c.Selectors {
				collectExprDeps(sel, locals, add)
			}
			collectBlockDeps(c.Body, locals, add)
		}
	case *ExprStmt:
		collectExprDeps(s.Expr, locals, add)
	case *BreakIfStmt:
		collectExprDeps(s.Condition, locals, add)
	}
}

// isBuiltinName returns true for WGSL built-in type and function names.
func isBuiltinName(name string) bool {
	switch name {
	case "bool", "i32", "u32", "f32", "f16", "f64", "i64", "u64",
		"vec2", "vec3", "vec4",
		"vec2i", "vec3i", "vec4i", "vec2u", "vec3u", "vec4u",
		"vec2f", "vec3f", "vec4f", "vec2h", "vec3h", "vec4h",
		"mat2x2", "mat2x3", "mat2x4", "mat3x2", "mat3x3", "mat3x4",
		"mat4x2", "mat4x3", "mat4x4",
		"mat2x2f", "mat2x3f", "mat2x4f", "mat3x2f", "mat3x3f", "mat3x4f",
		"mat4x2f", "mat4x3f", "mat4x4f",
		"mat2x2h", "mat2x3h", "mat2x4h", "mat3x2h", "mat3x3h", "mat3x4h",
		"mat4x2h", "mat4x3h", "mat4x4h",
		"texture_1d", "texture_2d", "texture_2d_array",
		"texture_3d", "texture_cube", "texture_cube_array",
		"texture_multisampled_2d", "texture_depth_2d",
		"texture_depth_2d_array", "texture_depth_cube",
		"texture_depth_cube_array", "texture_depth_multisampled_2d",
		"texture_storage_1d", "texture_storage_2d",
		"texture_storage_2d_array", "texture_storage_3d",
		"texture_external",
		"sampler", "sampler_comparison",
		"array", "atomic", "ptr",
		"read", "write", "read_write",
		"function", "private", "workgroup", "uniform", "storage", "handle",
		"true", "false":
		return true
	}
	return false
}
