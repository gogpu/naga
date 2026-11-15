package front

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// versioned prefixes a source with the #version directive every other test
// source leaves out.
func versioned(src string) string { return "#version 460\n" + src }

// parseSource preprocesses, lexes and parses src. The preprocessor and the
// classifier must accept it; the parser's diagnostics are the caller's to check.
func parseSource(t *testing.T, src string) *parsedFile {
	t.Helper()
	res := preprocessOK(t, src, nil)
	return parse(lex(res.Tokens), res.Version.Value)
}

// newTestParser returns a parser over the classified tokens of src, for the
// tests that call one parsing step directly.
func newTestParser(t *testing.T, src string) *parser {
	t.Helper()
	res := preprocessOK(t, versioned(src), nil)
	return &parser{
		tokens:  lex(res.Tokens),
		version: res.Version.Value,
		types:   map[string]bool{},
		symbols: newSymbolTable(),
	}
}

// hasError reports whether one diagnostic contains every substring.
func hasError(file *parsedFile, substrings ...string) bool {
	for _, e := range file.Errors {
		matched := true
		for _, s := range substrings {
			if !strings.Contains(e.Message, s) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

// wantParseError parses src and fails unless one diagnostic contains every
// substring.
func wantParseError(t *testing.T, src string, substrings ...string) {
	t.Helper()
	file := parseSource(t, src)
	if !hasError(file, substrings...) {
		t.Fatalf("parse(%q) errors %v do not contain %v", src, file.Errors, substrings)
	}
}

// wantParsed parses src and fails unless the parser reported no error.
func wantParsed(t *testing.T, src string) *parsedFile {
	t.Helper()
	file := parseSource(t, src)
	if len(file.Errors) > 0 {
		t.Fatalf("parse(%q) reported %v", src, file.Errors)
	}
	return file
}

// onlyDecl returns the single declaration of a parsed source.
func onlyDecl(t *testing.T, file *parsedFile) astDecl {
	t.Helper()
	if len(file.Decls) != 1 {
		t.Fatalf("%d declarations, want 1", len(file.Decls))
	}
	return file.Decls[0]
}

// wantDeclKind asserts the kind of a declaration and returns it.
func wantDeclKind[T declaration](t *testing.T, decl astDecl) T {
	t.Helper()
	kind, ok := decl.Kind.(T)
	if !ok {
		var want T
		t.Fatalf("declaration kind = %T, want %T", decl.Kind, want)
	}
	return kind
}

// funcDecl returns the function declaration named name.
func funcDecl(t *testing.T, file *parsedFile, name string) *declFunc {
	t.Helper()
	for i := range file.Decls {
		if fn, ok := file.Decls[i].Kind.(declFunc); ok && fn.Name == name {
			return &fn
		}
	}
	t.Fatalf("no function %q among %d declarations", name, len(file.Decls))
	return nil
}

// onlyVarList parses a source of one declaration and returns it as a variable
// declaration list.
func onlyVarList(t *testing.T, src string) *declVarList {
	t.Helper()
	list := wantDeclKind[declVarList](t, onlyDecl(t, wantParsed(t, versioned(src))))
	return &list
}

func TestParseVariableDeclarationList(t *testing.T) {
	tests := []struct {
		name        string
		src         string
		names       []string
		typeArrays  int
		declArrays  []int
		initialized []bool
	}{
		{
			name: "single", src: "float a;",
			names: []string{"a"}, declArrays: []int{0}, initialized: []bool{false},
		},
		{
			name: "initialized", src: "float a = 1.0;",
			names: []string{"a"}, declArrays: []int{0}, initialized: []bool{true},
		},
		{
			name: "list without initializers", src: "int a, b, c;",
			names: []string{"a", "b", "c"}, declArrays: []int{0, 0, 0},
			initialized: []bool{false, false, false},
		},
		{
			name: "initializer on one name of a list", src: "int a = 1, b, c = 2;",
			names: []string{"a", "b", "c"}, declArrays: []int{0, 0, 0},
			initialized: []bool{true, false, true},
		},
		{
			name: "array on the type", src: "float[2] a;",
			names: []string{"a"}, typeArrays: 1, declArrays: []int{0}, initialized: []bool{false},
		},
		{
			name: "array on the declarator", src: "float a[2][3];",
			names: []string{"a"}, declArrays: []int{2}, initialized: []bool{false},
		},
		{
			name: "array on both", src: "float[2] a[3];",
			names: []string{"a"}, typeArrays: 1, declArrays: []int{1}, initialized: []bool{false},
		},
		{
			// Rust consumes one leading comma (parser/declarations.rs:187-193).
			name: "leading comma", src: "float, a = 1.0;",
			names: []string{"a"}, declArrays: []int{0}, initialized: []bool{true},
		},
		{
			name: "unsized array", src: "float a[];",
			names: []string{"a"}, declArrays: []int{1}, initialized: []bool{false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list := onlyVarList(t, tt.src)
			if list.Typ.Builtin == nil {
				t.Fatalf("type %+v carries no builtin type name", list.Typ)
			}
			if len(list.Typ.Arrays) != tt.typeArrays {
				t.Errorf("%d type dimensions, want %d", len(list.Typ.Arrays), tt.typeArrays)
			}
			if len(list.Declarators) != len(tt.names) {
				t.Fatalf("%d declarators, want %d", len(list.Declarators), len(tt.names))
			}
			for i, d := range list.Declarators {
				if d.Name != tt.names[i] {
					t.Errorf("declarator %d = %q, want %q", i, d.Name, tt.names[i])
				}
				if len(d.Arrays) != tt.declArrays[i] {
					t.Errorf("declarator %q has %d dimensions, want %d",
						d.Name, len(d.Arrays), tt.declArrays[i])
				}
				if got := d.Init != nil; got != tt.initialized[i] {
					t.Errorf("declarator %q initialized = %t, want %t", d.Name, got, tt.initialized[i])
				}
				if !d.Span.IsDefined() {
					t.Errorf("declarator %q has no span", d.Name)
				}
			}
		})
	}
}

func TestParseDeclarationQualifiers(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want func(t *testing.T, q typeQualifiers)
	}{
		{
			name: "storage", src: "const float k = 1.0;",
			want: func(t *testing.T, q typeQualifiers) {
				if q.Storage != storageConst {
					t.Errorf("storage = %v, want const", q.Storage)
				}
			},
		},
		{
			name: "storage and layout", src: "layout(location = 0) out vec4 color;",
			want: func(t *testing.T, q typeQualifiers) {
				if q.Storage != storageOutput {
					t.Errorf("storage = %v, want out", q.Storage)
				}
				if len(q.Layout) != 1 {
					t.Fatalf("%d layout entries, want 1", len(q.Layout))
				}
				entry := q.Layout[0]
				if entry.Name != "location" || entry.Kind != layoutNamed || !entry.HasValue {
					t.Errorf("layout entry = %+v, want named location with a value", entry)
				}
				if entry.Value == nil {
					t.Error("layout entry carries no value expression")
				}
			},
		},
		{
			name: "interpolation", src: "flat in int id;",
			want: func(t *testing.T, q typeQualifiers) {
				if q.Interpolation != interpolationFlat {
					t.Errorf("interpolation = %v, want flat", q.Interpolation)
				}
			},
		},
		{
			name: "sampling", src: "centroid in vec2 uv;",
			want: func(t *testing.T, q typeQualifiers) {
				if q.Sampling != samplingCentroid {
					t.Errorf("sampling = %v, want centroid", q.Sampling)
				}
			},
		},
		{
			name: "precision", src: "highp float x;",
			want: func(t *testing.T, q typeQualifiers) {
				if q.Precision != precisionHigh {
					t.Errorf("precision = %v, want high", q.Precision)
				}
			},
		},
		{
			name: "memory qualifier", src: "readonly buffer Buf { float d[]; } b;",
			want: func(t *testing.T, q typeQualifiers) {
				if !q.HasStorageAccess || q.StorageAccess != ir.StorageAccessLoad {
					t.Errorf("storage access = %v (set %t), want load only",
						q.StorageAccess, q.HasStorageAccess)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decl := onlyDecl(t, wantParsed(t, versioned(tt.src)))
			switch kind := decl.Kind.(type) {
			case declVarList:
				tt.want(t, kind.Qualifiers)
			case declBlock:
				tt.want(t, kind.Qualifiers)
			default:
				t.Fatalf("declaration kind = %T, want a variable list or a block", decl.Kind)
			}
		})
	}
}

func TestParseStructDeclaration(t *testing.T) {
	t.Run("definition without declarator", func(t *testing.T) {
		list := onlyVarList(t, "struct S { float x; vec2 y, z[2]; };")
		if list.Typ.StructDef == nil {
			t.Fatalf("type %+v carries no struct definition", list.Typ)
		}
		def := list.Typ.StructDef
		if def.Name != "S" {
			t.Errorf("struct name = %q, want S", def.Name)
		}
		names := []string{"x", "y", "z"}
		if len(def.Members) != len(names) {
			t.Fatalf("%d members, want %d", len(def.Members), len(names))
		}
		for i, member := range def.Members {
			if member.Name != names[i] {
				t.Errorf("member %d = %q, want %q", i, member.Name, names[i])
			}
		}
		if got := len(def.Members[2].Arrays); got != 1 {
			t.Errorf("member z has %d dimensions, want 1", got)
		}
		if len(list.Declarators) != 0 {
			t.Errorf("%d declarators, want none", len(list.Declarators))
		}
	})

	t.Run("definition with declarator", func(t *testing.T) {
		list := onlyVarList(t, "struct S { float x; } s;")
		if list.Typ.StructDef == nil {
			t.Fatal("the declaration carries no struct definition")
		}
		if len(list.Declarators) != 1 || list.Declarators[0].Name != "s" {
			t.Fatalf("declarators = %+v, want one named s", list.Declarators)
		}
	})

	t.Run("name is a type afterwards", func(t *testing.T) {
		file := wantParsed(t, versioned("struct S { float x; };\nS s;"))
		if len(file.Decls) != 2 {
			t.Fatalf("%d declarations, want 2", len(file.Decls))
		}
		second := wantDeclKind[declVarList](t, file.Decls[1])
		if second.Typ.Name != "S" || second.Typ.StructDef != nil {
			t.Errorf("type = %+v, want the named type S", second.Typ)
		}
	})

	t.Run("unknown type name", func(t *testing.T) {
		// Rust gates a declaration on peek_type_name, which only accepts a
		// struct name the shader declared (parser/types.rs:391-398), so an
		// unknown name is never a declaration: the parser reports the stray
		// token instead of UnknownType.
		wantParseError(t, versioned("S s;"), "Expected Semicolon or end of file, found Identifier(\"S\")")
	})

	t.Run("unknown name in an expression", func(t *testing.T) {
		wantParseError(t, versioned("void main() { S s; }"), "Unknown variable: S")
	})

	t.Run("missing name", func(t *testing.T) {
		wantParseError(t, versioned("struct { float x; } s;"), "Expected identifier, found LeftBrace")
	})

	t.Run("members are not module-scope names", func(t *testing.T) {
		wantParseError(t, versioned("struct S { float m; };\nvoid main() { float x = m; }"),
			"Unknown variable: m")
	})
}

func TestParseFunctionPrototype(t *testing.T) {
	file := wantParsed(t, versioned("float f(in float x, out vec2 y, inout vec3 z, const int w);"))
	fn := funcDecl(t, file, "f")
	if fn.Body != nil {
		t.Errorf("prototype carries a body of %d statements", len(fn.Body))
	}
	if fn.Return == nil || fn.Return.Builtin == nil {
		t.Fatalf("return type = %+v, want a builtin type", fn.Return)
	}
	wantNames := []string{"x", "y", "z", "w"}
	wantQualifiers := []parameterQualifier{paramIn, paramOut, paramInOut, paramConst}
	if len(fn.Params) != len(wantNames) {
		t.Fatalf("%d parameters, want %d", len(fn.Params), len(wantNames))
	}
	for i, prm := range fn.Params {
		if prm.Name != wantNames[i] {
			t.Errorf("parameter %d = %q, want %q", i, prm.Name, wantNames[i])
		}
		if prm.Qualifier != wantQualifiers[i] {
			t.Errorf("parameter %q qualifier = %v, want %v", prm.Name, prm.Qualifier, wantQualifiers[i])
		}
	}
}

func TestParseFunctionParameters(t *testing.T) {
	t.Run("void list", func(t *testing.T) {
		fn := funcDecl(t, wantParsed(t, versioned("void f(void);")), "f")
		if fn.Return != nil {
			t.Errorf("return type = %+v, want none for void", fn.Return)
		}
		if len(fn.Params) != 0 {
			t.Errorf("%d parameters, want none", len(fn.Params))
		}
	})
	t.Run("unnamed with comma", func(t *testing.T) {
		fn := funcDecl(t, wantParsed(t, versioned("float f(float, vec2);")), "f")
		if len(fn.Params) != 2 {
			t.Fatalf("%d parameters, want 2", len(fn.Params))
		}
		for i, prm := range fn.Params {
			if prm.Name != "" {
				t.Errorf("parameter %d named %q, want unnamed", i, prm.Name)
			}
		}
	})
	t.Run("unnamed at the end", func(t *testing.T) {
		fn := funcDecl(t, wantParsed(t, versioned("void f(float);")), "f")
		if len(fn.Params) != 1 || fn.Params[0].Name != "" {
			t.Fatalf("parameters = %+v, want one unnamed parameter", fn.Params)
		}
	})
	t.Run("array", func(t *testing.T) {
		fn := funcDecl(t, wantParsed(t, versioned("void f(float x[2]);")), "f")
		if len(fn.Params) != 1 || len(fn.Params[0].Arrays) != 1 {
			t.Fatalf("parameters = %+v, want one with a dimension", fn.Params)
		}
	})
	t.Run("void parameter mid-list", func(t *testing.T) {
		wantParseError(t, versioned("void f(in void);"), "Type can't be void")
	})
}

func TestParseFunctionDefinition(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		function string
		wantBody int
	}{
		{"one statement", "float f(float x) { return x; }", "f", 1},
		{"empty body", "void main() {}", "main", 0},
		{"nested block", "void main() { { } }", "main", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := funcDecl(t, wantParsed(t, versioned(tt.src)), tt.function)
			if fn.Body == nil {
				t.Fatal("the definition recorded no body; a nil body means a prototype")
			}
			if len(fn.Body) != tt.wantBody {
				t.Errorf("%d body statements, want %d", len(fn.Body), tt.wantBody)
			}
		})
	}

	t.Run("parameter visible in the body", func(t *testing.T) {
		fn := funcDecl(t, wantParsed(t, versioned("float f(float x) { return x; }")), "f")
		if len(fn.Body) != 1 {
			t.Fatalf("%d body statements, want 1", len(fn.Body))
		}
	})
}

func TestParseInterfaceBlock(t *testing.T) {
	t.Run("with instance", func(t *testing.T) {
		block := wantDeclKind[declBlock](t, onlyDecl(t,
			wantParsed(t, versioned("layout(std140, binding = 0) uniform Blk { vec4 color; float x; } blk;"))))
		if block.Name != "Blk" || block.Instance != "blk" {
			t.Errorf("block = %q, instance = %q, want Blk and blk", block.Name, block.Instance)
		}
		if len(block.Members) != 2 {
			t.Fatalf("%d members, want 2", len(block.Members))
		}
		if block.Members[0].Name != "color" || block.Members[1].Name != "x" {
			t.Errorf("member names = %q, %q", block.Members[0].Name, block.Members[1].Name)
		}
		if len(block.Qualifiers.Layout) != 2 {
			t.Fatalf("%d layout entries, want 2", len(block.Qualifiers.Layout))
		}
		if entry := block.Qualifiers.Layout[0]; entry.Kind != layoutStructure || entry.Struct != structLayoutStd140 {
			t.Errorf("first layout entry = %+v, want std140", entry)
		}
		if entry := block.Qualifiers.Layout[1]; entry.Name != "binding" || !entry.HasValue {
			t.Errorf("second layout entry = %+v, want a binding with a value", entry)
		}
	})

	t.Run("without instance", func(t *testing.T) {
		block := wantDeclKind[declBlock](t, onlyDecl(t,
			wantParsed(t, versioned("uniform Blk { vec4 color; };"))))
		if block.Instance != "" {
			t.Errorf("instance = %q, want none", block.Instance)
		}
	})

	t.Run("instance array", func(t *testing.T) {
		block := wantDeclKind[declBlock](t, onlyDecl(t,
			wantParsed(t, versioned("uniform Blk { vec4 c; } b[2];"))))
		if len(block.Arrays) != 1 {
			t.Errorf("%d instance dimensions, want 1", len(block.Arrays))
		}
	})

	t.Run("instance and members are names", func(t *testing.T) {
		wantParsed(t, versioned("uniform Blk { vec4 c; } b;\nvoid main() { vec4 x = b; vec4 y = c; }"))
	})

	t.Run("unexpected token after the block", func(t *testing.T) {
		wantParseError(t, versioned("uniform Blk { vec4 c; } 1;"),
			"Expected identifier or Semicolon, found IntConstant(Integer { value: 1, signed: true, width: 32 })")
	})

	t.Run("block name is not a type", func(t *testing.T) {
		// Rust never inserts a block's name into lookup_type, so a following
		// declaration cannot use it as a type (parser/declarations.rs:560-569).
		wantParseError(t, versioned("uniform Blk { vec4 c; } b;\nBlk other;"),
			"Expected Semicolon or end of file, found Identifier(\"Blk\")")
	})
}

func TestParsePrecisionDeclaration(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want precisionQualifier
	}{
		{"highp", "precision highp float;", precisionHigh},
		{"mediump", "precision mediump int;", precisionMedium},
		// Rust's float-and-int check needs the resolved type and is lowering's.
		{"vector", "precision lowp vec4;", precisionLow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decl := wantDeclKind[declPrecision](t, onlyDecl(t, wantParsed(t, versioned(tt.src))))
			if decl.Precision != tt.want {
				t.Errorf("precision = %v, want %v", decl.Precision, tt.want)
			}
			if decl.Typ.Builtin == nil {
				t.Errorf("type %+v carries no builtin type name", decl.Typ)
			}
		})
	}

	t.Run("missing precision qualifier", func(t *testing.T) {
		wantParseError(t, versioned("precision float;"),
			"Expected PrecisionQualifier(High), PrecisionQualifier(Medium) or PrecisionQualifier(Low), "+
				"found TypeName(Type { name: None, inner: Scalar(Scalar { kind: Float, width: 4 }) })")
	})
	t.Run("void type", func(t *testing.T) {
		wantParseError(t, versioned("precision highp void;"), "Type can't be void")
	})
	t.Run("missing semicolon", func(t *testing.T) {
		wantParseError(t, versioned("precision highp float"), "Unexpected end of file")
	})
}

func TestParseQualifierOnlyDeclaration(t *testing.T) {
	t.Run("layout with a value", func(t *testing.T) {
		decl := wantDeclKind[declQualifiers](t, onlyDecl(t,
			wantParsed(t, versioned("layout(local_size_x = 8) in;"))))
		if len(decl.Qualifiers.Layout) != 1 {
			t.Fatalf("%d layout entries, want 1", len(decl.Qualifiers.Layout))
		}
		entry := decl.Qualifiers.Layout[0]
		if entry.Name != "local_size_x" || !entry.HasValue || entry.Value == nil {
			t.Errorf("layout entry = %+v, want local_size_x with a value", entry)
		}
		if decl.Qualifiers.Storage != storageInput {
			t.Errorf("storage = %v, want in", decl.Qualifiers.Storage)
		}
	})

	t.Run("layout without a value", func(t *testing.T) {
		decl := wantDeclKind[declQualifiers](t, onlyDecl(t,
			wantParsed(t, versioned("layout(early_fragment_tests) in;"))))
		if len(decl.Qualifiers.Layout) != 1 || decl.Qualifiers.Layout[0].HasValue {
			t.Errorf("layout = %+v, want one entry without a value", decl.Qualifiers.Layout)
		}
	})

	t.Run("invariant redeclaration", func(t *testing.T) {
		decl := wantDeclKind[declInvariant](t, onlyDecl(t,
			wantParsed(t, versioned("invariant gl_Position;"))))
		if decl.Name != "gl_Position" {
			t.Errorf("name = %q, want gl_Position", decl.Name)
		}
	})

	t.Run("invariant of an unknown name", func(t *testing.T) {
		// Rust accepts a name it cannot resolve here (variables.rs:226-243).
		decl := wantDeclKind[declInvariant](t, onlyDecl(t,
			wantParsed(t, versioned("invariant foo;"))))
		if decl.Name != "foo" {
			t.Errorf("name = %q, want foo", decl.Name)
		}
	})

	t.Run("qualified name without a type", func(t *testing.T) {
		wantParseError(t, versioned("uniform tex;"), "Not implemented: variable qualifier")
	})

	t.Run("unexpected token", func(t *testing.T) {
		wantParseError(t, versioned("uniform 1;"),
			"Expected identifier or Semicolon, found IntConstant(Integer { value: 1, signed: true, width: 32 })")
	})
}

func TestDeclaratorScope(t *testing.T) {
	// An earlier declarator of the same declaration is in scope in a later
	// initializer: Rust declares each variable right after its own initializer
	// (parser/declarations.rs:267), and the GLSL spec agrees.
	for _, src := range []string{
		versioned("int a = 1, b = a;"),
		versioned("void main() { int i = 0, j = i; }"),
		versioned("void main() { for (int i = 0, j = i; i < 2; i++) { } }"),
	} {
		file := parseSource(t, src)
		if len(file.Errors) != 0 {
			t.Errorf("%q errors = %v, want none: an earlier declarator is in scope", src, file.Errors)
		}
	}
	// A declarator is not in scope before its own initializer.
	file := parseSource(t, versioned("int b = a, a = 1;"))
	if !hasError(file, "Unknown variable: a") {
		t.Errorf("errors = %v, want Unknown variable: a", file.Errors)
	}
}

func TestDeclaratorTypesArePerDeclarator(t *testing.T) {
	// Rust naga mutates the declaration's type across the list
	// (parser/declarations.rs:219), so `float a[2], b;` gives b the array type
	// too and the module then fails validation. glslang and the spec keep b a
	// plain float, and the AST keeps the dimensions where they were written.
	file := parseSource(t, versioned("float a[2], b;"))
	if len(file.Errors) != 0 {
		t.Fatalf("errors = %v, want none", file.Errors)
	}
	list := wantDeclKind[declVarList](t, file.Decls[0])
	if len(list.Declarators) != 2 {
		t.Fatalf("%d declarators, want 2", len(list.Declarators))
	}
	if len(list.Declarators[0].Arrays) != 1 {
		t.Errorf("a has %d dimensions, want 1", len(list.Declarators[0].Arrays))
	}
	if len(list.Declarators[1].Arrays) != 0 {
		t.Errorf("b has %d dimensions, want none", len(list.Declarators[1].Arrays))
	}
}

func TestParseVoidDeclaration(t *testing.T) {
	// Rust reports the error, keeps the bumped name consumed and records no
	// declaration (parser/declarations.rs:400-416); the parse keeps its place
	// with a node lowering ignores.
	file := parseSource(t, versioned("void x;"))
	if !hasError(file, "Declaration cannot have void type") {
		t.Fatalf("errors = %v, want the void declaration error", file.Errors)
	}
	if len(file.Decls) != 1 {
		t.Fatalf("%d declarations, want the one placeholder Rust's recovery leaves", len(file.Decls))
	}
	wantDeclKind[declInvalid](t, file.Decls[0])
}

func TestParseDeclarationFallthrough(t *testing.T) {
	p := newTestParser(t, "x = 1.0;")
	decl, err := p.parseDeclaration(false)
	if decl != nil || err != nil {
		t.Fatalf("parseDeclaration of a non-declaration = %v, %v; want nil, nil", decl, err)
	}
}

func TestParseInitializer(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		wantItems []int // the item counts of the braced nodes, outermost first
	}{
		{"expression", "float a = 1.0 + 2.0;", nil},
		{"one item", "float a = { 1.0 };", []int{1}},
		{"trailing comma", "float a = { 1.0, 2.0, };", []int{2}},
		{"nested braces", "struct S { float x; vec2 y; } S = { 1.0, { 2.0, 3.0 } };", []int{2, 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list := wantDeclKind[declVarList](t, onlyDecl(t, wantParsed(t, versioned(tt.src))))
			if len(list.Declarators) != 1 || list.Declarators[0].Init == nil {
				t.Fatalf("declarators = %+v, want one initialized", list.Declarators)
			}
			init := list.Declarators[0].Init
			if len(tt.wantItems) == 0 {
				if init.Root.Items != nil {
					t.Fatalf("root has %d items, want one expression", len(init.Root.Items))
				}
				if _, ok := init.Arena.get(init.Root.Expr).Kind.(hirBinary); !ok {
					t.Errorf("root expression = %T, want a binary expression",
						init.Arena.get(init.Root.Expr).Kind)
				}
				return
			}
			got := []int{}
			var walk func(node initNode)
			walk = func(node initNode) {
				if node.Items == nil {
					return
				}
				got = append(got, len(node.Items))
				for _, item := range node.Items {
					walk(item)
				}
			}
			walk(init.Root)
			if len(got) != len(tt.wantItems) {
				t.Fatalf("braced nodes = %v, want %v", got, tt.wantItems)
			}
			for i := range got {
				if got[i] != tt.wantItems[i] {
					t.Errorf("node %d has %d items, want %d", i, got[i], tt.wantItems[i])
				}
			}
		})
	}

	t.Run("missing comma", func(t *testing.T) {
		wantParseError(t, versioned("float a = { 1.0 2.0 };"), "Expected Comma or RightBrace")
	})
}

func TestParseDeclaratorScopes(t *testing.T) {
	tests := []struct {
		name   string
		src    string
		errors []string // nil means the parse must be clean
	}{
		{
			name:   "local duplicate",
			src:    "void main() { float a; float a; }",
			errors: []string{"Variable already declared: a"},
		},
		{
			name: "local shadowing",
			src:  "void main() { float a; { int a; } }",
		},
		{
			// Rust replaces a module-scope name without reporting it
			// (context.rs:232).
			name: "global duplicate",
			src:  "float a; float a;",
		},
		{
			name:   "parameter duplicate",
			src:    "void f(float x, float x) { }",
			errors: []string{"Variable already declared: x"},
		},
		{
			// The body is a compound statement, a scope of its own inside the one
			// holding the parameters, so a declaration there shadows the
			// parameter as any nested declaration does.
			name: "parameter shadowed in the body",
			src:  "void f(float x) { float x; }",
		},
		{
			name:   "parameter out of scope after the prototype",
			src:    "void f(float x);\nvoid main() { float y = x; }",
			errors: []string{"Unknown variable: x"},
		},
		{
			name:   "block scope ends",
			src:    "void main() { { float a; } a = 1.0; }",
			errors: []string{"Unknown variable: a"},
		},
		{
			name:   "for scope ends",
			src:    "void main() { for (int i = 0; i < 1; i++) { } i = 1; }",
			errors: []string{"Unknown variable: i"},
		},
		{
			name: "for init visible in all clauses",
			src:  "void main() { for (int i = 0; i < 4; i++) { float x = i; } }",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := parseSource(t, versioned(tt.src))
			if len(tt.errors) == 0 {
				if len(file.Errors) > 0 {
					t.Fatalf("parse(%q) reported %v, want none", tt.src, file.Errors)
				}
				return
			}
			if !hasError(file, tt.errors...) {
				t.Fatalf("parse(%q) errors %v do not contain %v", tt.src, file.Errors, tt.errors)
			}
		})
	}
}

func TestParseDeclarationErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "missing semicolon",
			src:  "void main() { float x = 1.0 }",
			want: []string{"Expected Comma or Semicolon, found RightBrace"},
		},
		{
			name: "declarator is not a name",
			src:  "float 1;",
			want: []string{"Expected identifier or Semicolon, found IntConstant(Integer { value: 1, " +
				"signed: true, width: 32 })"},
		},
		{
			name: "function without a body or a semicolon",
			src:  "float f() x;",
			want: []string{"Expected LeftBrace or Semicolon, found Identifier(\"x\")"},
		},
		{
			name: "local function body",
			src:  "void main() { void f() {} }",
			want: []string{"Expected Semicolon, found LeftBrace"},
		},
		{
			name: "stray token at module scope",
			src:  "1;",
			want: []string{"Expected Semicolon or end of file, found IntConstant(Integer { value: 1, " +
				"signed: true, width: 32 })"},
		},
		{
			name: "type without a name",
			src:  "float",
			want: []string{"Unexpected end of file"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantParseError(t, versioned(tt.src), tt.want...)
		})
	}
}

func TestExpressionDepthLimit(t *testing.T) {
	parens := strings.Repeat("(", maxExprDepth+1)
	closing := strings.Repeat(")", maxExprDepth+1)
	src := versioned("float f() { return " + parens + "1.0" + closing + "; }")
	wantParseError(t, src, errExprDepth)
}

func TestParseDeclarationErrorPaths(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"initializer expression", "float a = 1.0 +;", "Expected LeftParen, integer literal, " +
			"float literal or bool literal, found Semicolon"},
		{"initializer at the end of the stream", "float a = { 1", "Unexpected end of file"},
		{"declarator list at the end of the stream", "float a,", "Unexpected end of file"},
		{"declaration at the end of the stream", "float a", "Unexpected end of file"},
		{"parameter list at the end of the stream", "void f(", "Unexpected end of file"},
		{"parameter list without a closing parenthesis", "void f(float x", "Unexpected end of file"},
		{"unterminated block members", "uniform Blk { vec4 c;", "Unexpected end of file"},
		{"instance name at the end of the stream", "uniform Blk { vec4 c; } b", "Unexpected end of file"},
		{"instance array at the end of the stream", "uniform Blk { vec4 c; } b[2", "Unexpected end of file"},
		{"invariant without a semicolon", "invariant foo }", "Expected Semicolon, found RightBrace"},
		{"precision without a semicolon", "precision highp float 1", "Expected Semicolon, found " +
			"IntConstant(Integer { value: 1, signed: true, width: 32 })"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantParseError(t, versioned(tt.src), tt.want)
		})
	}
}
