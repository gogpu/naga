package front

import "testing"

func TestBuiltinVariableNames(t *testing.T) {
	// The variable names of Rust naga's lookup_variable
	// (front/glsl/variables.rs:112-210), in the order it lists them, plus the
	// four GL_KHR_shader_subgroup_basic scalars and the five
	// GL_KHR_shader_subgroup_ballot masks, which Rust has no counterpart for:
	// its GLSL frontend has no subgroups at all (upstream #7571). Their
	// availability is the family's extension, checked when they are lowered
	// (subgroupVariableGate).
	want := []string{
		"gl_Position", "gl_FragCoord", "gl_PointCoord",
		"gl_GlobalInvocationID", "gl_NumWorkGroups", "gl_WorkGroupSize",
		"gl_WorkGroupID", "gl_LocalInvocationID",
		"gl_FrontFacing", "gl_PointSize", "gl_FragDepth",
		"gl_ClipDistance", "gl_CullDistance",
		"gl_BaseVertex", "gl_BaseInstance", "gl_PrimitiveID",
		"gl_BaryCoordEXT", "gl_BaryCoordNoPerspEXT",
		"gl_InstanceIndex", "gl_VertexIndex", "gl_SampleID",
		"gl_LocalInvocationIndex", "gl_DrawID",
		"gl_SubgroupSize", "gl_SubgroupInvocationID", "gl_SubgroupID", "gl_NumSubgroups",
		"gl_SubgroupEqMask", "gl_SubgroupGeMask", "gl_SubgroupGtMask",
		"gl_SubgroupLeMask", "gl_SubgroupLtMask",
	}
	if len(builtinVariableNames) != len(want) {
		t.Errorf("%d builtin names, want %d", len(builtinVariableNames), len(want))
	}
	for _, name := range want {
		if !builtinVariableNames[name] {
			t.Errorf("%q is not a builtin variable name", name)
		}
	}
	for _, name := range []string{"gl_FragColor", "gl_FragData", "position", ""} {
		if builtinVariableNames[name] {
			t.Errorf("%q is a builtin variable name, want none", name)
		}
	}
}

func TestBuiltinSymbol(t *testing.T) {
	t.Run("not a builtin", func(t *testing.T) {
		p := newTestParser(t, "")
		if sym := p.builtinSymbol("position"); sym != nil {
			t.Errorf("builtinSymbol(position) = %+v, want nil", sym)
		}
	})

	t.Run("declared on first use in the root scope", func(t *testing.T) {
		p := newTestParser(t, "")
		p.symbols.pushScope()
		sym := p.builtinSymbol("gl_Position")
		if sym == nil {
			t.Fatal("builtinSymbol(gl_Position) = nil")
		}
		if sym.Kind != symBuiltinVar || sym.Name != "gl_Position" {
			t.Errorf("symbol = %+v, want a builtin variable named gl_Position", sym)
		}
		// The declaration must outlive the block that first used it (Rust's
		// add_root, variables.rs:96).
		p.symbols.popScope()
		if got := p.symbols.lookup("gl_Position"); got != sym {
			t.Errorf("lookup after the block = %+v, want the declared symbol", got)
		}
		if p.symbols.Scopes[0]["gl_Position"] != sym {
			t.Error("the builtin was not declared in the root scope")
		}
	})

	t.Run("existing symbol wins", func(t *testing.T) {
		p := newTestParser(t, "")
		declared := &glslSymbol{Kind: symGlobal, Name: "gl_Position"}
		p.symbols.declare(declared)
		if sym := p.builtinSymbol("gl_Position"); sym != declared {
			t.Errorf("builtinSymbol(gl_Position) = %+v, want the declared symbol", sym)
		}
	})
}

// assignedSymbol returns the symbol an expression statement's assignment target
// names.
func assignedSymbol(t *testing.T, stmt astStmt) *glslSymbol {
	t.Helper()
	expr := wantStmtKind[stmtExpr](t, stmt).Expr
	assign, ok := expr.Arena.get(expr.Root).Kind.(hirAssign)
	if !ok {
		t.Fatalf("statement root = %T, want an assignment", expr.Arena.get(expr.Root).Kind)
	}
	variable, ok := expr.Arena.get(assign.Tgt).Kind.(hirVariable)
	if !ok {
		t.Fatalf("assignment target = %T, want a variable", expr.Arena.get(assign.Tgt).Kind)
	}
	return variable.Sym
}

func TestParseBuiltinVariable(t *testing.T) {
	t.Run("resolved on use", func(t *testing.T) {
		stmts := mainBody(t, "gl_PointSize = 1.0;\ngl_PointSize = 2.0;")
		if len(stmts) != 2 {
			t.Fatalf("%d statements, want 2", len(stmts))
		}
		first := assignedSymbol(t, stmts[0])
		if first == nil || first.Kind != symBuiltinVar || first.Name != "gl_PointSize" {
			t.Fatalf("symbol = %+v, want the builtin gl_PointSize", first)
		}
		// The second use finds the declaration of the first.
		if second := assignedSymbol(t, stmts[1]); second != first {
			t.Errorf("second use resolved to %+v, want the symbol of the first", second)
		}
	})

	t.Run("shadowed by a declaration", func(t *testing.T) {
		stmts := bodyOf(t, "out float gl_Position;\nvoid main() { gl_Position = 1.0; }", "main")
		sym := assignedSymbol(t, stmts[0])
		if sym == nil || sym.Kind != symGlobal {
			t.Fatalf("symbol = %+v, want the declared global", sym)
		}
	})

	t.Run("unknown name", func(t *testing.T) {
		wantParseError(t, versioned("void main() { foo = 1.0; }"), "Unknown variable: foo")
	})
}
