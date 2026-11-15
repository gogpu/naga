package front

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// spanCheck asserts that every span a parse produced lies inside the source it
// came from, which is the property the AST promises and every later pass relies
// on (design §17).
type spanCheck struct {
	t     *testing.T
	limit uint32
}

// span checks one span; an undefined span points at nothing and is legal.
func (c spanCheck) span(s ir.Span, what string) {
	c.t.Helper()
	if !s.IsDefined() {
		return
	}
	if s.Start > s.End || s.End > c.limit {
		c.t.Fatalf("%s span %v lies outside the %d-byte source", what, s, c.limit)
	}
}

// exprs checks the nodes of one expression arena.
func (c spanCheck) exprs(e *exprs, what string) {
	c.t.Helper()
	for i := range e.Arena.Nodes {
		c.span(e.Arena.Nodes[i].Span, what)
	}
}

// initNodes checks an initializer tree.
func (c spanCheck) initNodes(nodes []initNode, what string) {
	c.t.Helper()
	for _, node := range nodes {
		c.span(node.Span, what)
		c.initNodes(node.Items, what)
	}
}

// initializer checks a declarator's initializer and its arena.
func (c spanCheck) initializer(init *initializer, what string) {
	c.t.Helper()
	if init == nil {
		return
	}
	c.span(init.Root.Span, what)
	c.initNodes(init.Root.Items, what)
	for i := range init.Arena.Nodes {
		c.span(init.Arena.Nodes[i].Span, what)
	}
}

// arraySpec checks one array dimension and the expression that sizes it.
func (c spanCheck) arraySpec(spec arraySpec) {
	c.t.Helper()
	c.span(spec.Span, "array specifier")
	if spec.Size != nil {
		c.exprs(spec.Size, "array size")
	}
}

// qualifiers checks the spans of a qualifier list, layout values included.
func (c spanCheck) qualifiers(q typeQualifiers) {
	c.t.Helper()
	c.span(q.Span, "qualifiers")
	for _, entry := range q.Layout {
		c.span(entry.Span, "layout entry")
		if entry.Value != nil {
			c.exprs(entry.Value, "layout value")
		}
	}
}

// typeSpec checks a type, its dimensions and an inline struct definition.
func (c spanCheck) typeSpec(spec typeSpec) {
	c.t.Helper()
	c.span(spec.Span, "type")
	for _, arrays := range spec.Arrays {
		c.arraySpec(arrays)
	}
	if spec.StructDef == nil {
		return
	}
	c.span(spec.StructDef.Span, "struct definition")
	for _, member := range spec.StructDef.Members {
		c.span(member.Span, "struct member")
		c.typeSpec(member.Typ)
		for _, arrays := range member.Arrays {
			c.arraySpec(arrays)
		}
	}
}

// members checks the members of an interface block.
func (c spanCheck) members(members []structMember) {
	c.t.Helper()
	for _, member := range members {
		c.span(member.Span, "block member")
		c.typeSpec(member.Typ)
		for _, arrays := range member.Arrays {
			c.arraySpec(arrays)
		}
	}
}

// decl checks one declaration.
func (c spanCheck) decl(decl astDecl) {
	c.t.Helper()
	c.span(decl.Span, "declaration")
	switch kind := decl.Kind.(type) {
	case declVarList:
		c.qualifiers(kind.Qualifiers)
		c.typeSpec(kind.Typ)
		for _, d := range kind.Declarators {
			c.span(d.Span, "declarator")
			for _, arrays := range d.Arrays {
				c.arraySpec(arrays)
			}
			c.initializer(d.Init, "initializer")
		}
	case declFunc:
		c.qualifiers(kind.Qualifiers)
		c.span(kind.Span, "function")
		if kind.Return != nil {
			c.typeSpec(*kind.Return)
		}
		for _, prm := range kind.Params {
			c.span(prm.Span, "parameter")
			c.typeSpec(prm.Typ)
			for _, arrays := range prm.Arrays {
				c.arraySpec(arrays)
			}
		}
		for _, stmt := range kind.Body {
			c.stmt(stmt)
		}
	case declBlock:
		c.qualifiers(kind.Qualifiers)
		c.span(kind.Span, "interface block")
		c.members(kind.Members)
		for _, arrays := range kind.Arrays {
			c.arraySpec(arrays)
		}
	case declQualifiers:
		c.qualifiers(kind.Qualifiers)
	case declPrecision:
		c.typeSpec(kind.Typ)
	}
}

// stmt checks one statement and everything below it.
func (c spanCheck) stmt(stmt astStmt) {
	c.t.Helper()
	c.span(stmt.Span, "statement")
	switch kind := stmt.Kind.(type) {
	case stmtBlock:
		for _, inner := range kind.Body {
			c.stmt(inner)
		}
	case stmtDecl:
		c.decl(kind.Decl)
	case stmtExpr:
		c.exprs(&kind.Expr, "expression")
	case stmtIf:
		c.exprs(&kind.Cond, "condition")
		if kind.Accept != nil {
			c.stmt(*kind.Accept)
		}
		if kind.Reject != nil {
			c.stmt(*kind.Reject)
		}
	case stmtSwitch:
		c.exprs(&kind.Selector, "selector")
		for _, oneCase := range kind.Cases {
			c.span(oneCase.Span, "switch case")
			if oneCase.Label != nil {
				c.exprs(oneCase.Label, "case label")
			}
			for _, inner := range oneCase.Body {
				c.stmt(inner)
			}
		}
	case stmtWhile:
		c.exprs(&kind.Cond, "condition")
		c.stmt(kind.Body)
	case stmtDo:
		c.stmt(kind.Body)
		c.exprs(&kind.Cond, "condition")
	case stmtFor:
		if kind.InitDecl != nil {
			c.decl(*kind.InitDecl)
		}
		for _, e := range []*exprs{kind.InitExpr, kind.Cond, kind.Update} {
			if e != nil {
				c.exprs(e, "for clause")
			}
		}
		c.stmt(kind.Body)
	case stmtReturn:
		if kind.Value != nil {
			c.exprs(kind.Value, "return value")
		}
	}
}

// FuzzParseGLSL checks that arbitrary input never panics the preprocessor, the
// classifier or the parser, and that every span of the resulting AST points into
// the source (design §17).
func FuzzParseGLSL(f *testing.F) {
	seeds := []string{
		"#version 460\nvoid main() {}\n",
		"#version 450\nlayout(location = 0) in vec3 position;\n" +
			"layout(location = 0) out vec4 color;\nvoid main() { color = vec4(position, 1.0); }\n",
		"#version 460\nlayout(local_size_x = 8) in;\n" +
			"layout(std430, binding = 0) buffer Buf { float data[]; };\n" +
			"void main() { data[0] = 1.0; }\n",
		"#version 460\nstruct S { float x; vec2 y[2]; };\n" +
			"float f(in S s, out float o, float) { o = s.x; return s.y[0].x; }\n",
		"#version 460\nvoid main() {\n" +
			"\tfor (int i = 0; i < 4; i++) { if (i == 2) break; else continue; }\n" +
			"\tswitch (1) { case 0: discard; default: break; }\n" +
			"\tdo { } while (false); while (true) { return; }\n}\n",
		"#version 460\nprecision highp float;\ninvariant gl_Position;\n" +
			"layout(std140) uniform B { vec4 c; } b;\n" +
			"void main() { gl_Position = vec4(1.0); b.c = { 1.0, 2.0, 3.0, 4.0 }; }\n",
		"#version 460\n",
		"\n\n// comment only\n",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, src string) {
		res := preprocess(src, nil)
		if len(res.Errors) > 0 {
			// The frontend stops at a preprocessor error, as it does for a real
			// shader.
			return
		}
		file := parse(lex(res.Tokens), res.Version.Value)
		check := spanCheck{t: t, limit: uint32(len(src))}
		for _, decl := range file.Decls {
			check.decl(decl)
		}
		for _, e := range file.Errors {
			check.span(e.Span, "diagnostic")
		}
	})
}
