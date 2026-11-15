package front

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// declarationQualifiers returns the qualifiers of a declaration that carries
// them: a variable declaration list or a qualifier-only declaration.
func declarationQualifiers(t *testing.T, decl astDecl) typeQualifiers {
	t.Helper()
	switch kind := decl.Kind.(type) {
	case declVarList:
		return kind.Qualifiers
	case declQualifiers:
		return kind.Qualifiers
	default:
		t.Fatalf("declaration kind = %T, want a variable list or a qualifier list", decl.Kind)
		return typeQualifiers{}
	}
}

func TestParseDuplicateQualifiers(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "invariant",
			src:  "invariant invariant gl_Position;",
			want: "Cannot use more than one invariant qualifier per declaration",
		},
		{
			name: "interpolation",
			src:  "flat smooth in vec2 v;",
			want: "Cannot use more than one interpolation qualifier per declaration",
		},
		{
			name: "storage",
			src:  "in out vec4 v;",
			want: "Cannot use more than one storage qualifier per declaration",
		},
		{
			name: "sampling",
			src:  "centroid sample in vec2 v;",
			want: "Cannot use more than one sampling qualifier per declaration",
		},
		{
			name: "precision",
			src:  "highp mediump float x;",
			want: "Cannot use more than one precision qualifier per declaration",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := parseSource(t, versioned(tt.src))
			if len(file.Errors) != 1 {
				t.Fatalf("errors = %v, want exactly one", file.Errors)
			}
			if got := file.Errors[0].Message; got != tt.want {
				t.Errorf("message = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseMemoryQualifiers(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		access    ir.StorageAccess
		hasAccess bool
		errCount  int
		wantErr   string
	}{
		{
			name:      "readonly",
			src:       "readonly buffer Buf { float d[]; } b;",
			access:    ir.StorageAccessLoad,
			hasAccess: true,
		},
		{
			name:      "writeonly",
			src:       "writeonly buffer Buf { float d[]; } b;",
			access:    ir.StorageAccessStore,
			hasAccess: true,
		},
		{
			// Rust's merge: each qualifier narrows the access, so both together
			// leave neither bit and are not an error (parser/types.rs:277-294).
			name:      "readonly then writeonly",
			src:       "readonly writeonly buffer Buf { float d[]; } b;",
			hasAccess: true,
		},
		{
			name:      "writeonly then readonly",
			src:       "writeonly readonly buffer Buf { float d[]; } b;",
			hasAccess: true,
		},
		{
			name:      "duplicate readonly",
			src:       "readonly readonly buffer Buf { float d[]; } b;",
			access:    ir.StorageAccessLoad,
			hasAccess: true,
			errCount:  1,
			wantErr:   "The same memory qualifier can only be used once",
		},
		{
			// restrict affects nothing in the IR and is accepted silently.
			name: "restrict",
			src:  "restrict buffer Buf { float d[]; } b;",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := parseSource(t, versioned(tt.src))
			if len(file.Errors) != tt.errCount {
				t.Fatalf("errors = %v, want %d", file.Errors, tt.errCount)
			}
			if tt.wantErr != "" && !hasError(file, tt.wantErr) {
				t.Errorf("errors = %v, want the message %q", file.Errors, tt.wantErr)
			}
			block := wantDeclKind[declBlock](t, onlyDecl(t, file))
			q := block.Qualifiers
			if q.StorageAccess != tt.access {
				t.Errorf("storage access = %v, want %v", q.StorageAccess, tt.access)
			}
			if q.HasStorageAccess != tt.hasAccess {
				t.Errorf("access set = %t, want %t", q.HasStorageAccess, tt.hasAccess)
			}
		})
	}
}

func TestParseLayoutQualifiers(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		kind      layoutKind
		entryName string
		hasValue  bool
		structure structLayout
		format    ir.StorageFormat
		storage   storageQualifier
	}{
		{
			name:      "std140",
			src:       "layout(std140) uniform vec4 v;",
			kind:      layoutStructure,
			structure: structLayoutStd140,
			storage:   storageUniform,
		},
		{
			name:      "std430",
			src:       "layout(std430) buffer float f;",
			kind:      layoutStructure,
			structure: structLayoutStd430,
			storage:   storageBuffer,
		},
		{
			name:      "index",
			src:       "layout(index = 2) out vec4 color;",
			kind:      layoutIndex,
			entryName: "index",
			hasValue:  true,
			storage:   storageOutput,
		},
		{
			name:      "image format",
			src:       "layout(rgba8) uniform image2D img;",
			kind:      layoutFormat,
			entryName: "rgba8",
			format:    ir.StorageFormatRgba8Unorm,
			storage:   storageUniform,
		},
		{
			// A named entry with a constant value, on a qualifier-only declaration.
			name:      "named value",
			src:       "layout(local_size_x = 8) in;",
			kind:      layoutNamed,
			entryName: "local_size_x",
			hasValue:  true,
			storage:   storageInput,
		},
		{
			name:      "nameless entry",
			src:       "layout(early_fragment_tests) in;",
			kind:      layoutNamed,
			entryName: "early_fragment_tests",
			storage:   storageInput,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := declarationQualifiers(t, onlyDecl(t, wantParsed(t, versioned(tt.src))))
			if len(q.Layout) != 1 {
				t.Fatalf("%d layout entries, want 1: %+v", len(q.Layout), q.Layout)
			}
			entry := q.Layout[0]
			if entry.Kind != tt.kind {
				t.Errorf("kind = %v, want %v", entry.Kind, tt.kind)
			}
			if entry.Name != tt.entryName {
				t.Errorf("name = %q, want %q", entry.Name, tt.entryName)
			}
			if entry.HasValue != tt.hasValue {
				t.Errorf("has value = %t, want %t", entry.HasValue, tt.hasValue)
			}
			if entry.Struct != tt.structure {
				t.Errorf("structure = %v, want %v", entry.Struct, tt.structure)
			}
			if entry.Format != tt.format {
				t.Errorf("format = %v, want %v", entry.Format, tt.format)
			}
			if q.Storage != tt.storage {
				t.Errorf("storage = %v, want %v", q.Storage, tt.storage)
			}
			if tt.hasValue != (entry.Value != nil) {
				t.Errorf("value expression present = %t, want %t", entry.Value != nil, tt.hasValue)
			}
		})
	}
}

func TestParseLayoutEntryWithoutIdentifier(t *testing.T) {
	// Rust records the bad entry and keeps the list going
	// (parser/types.rs:325-389); the declaration still parses.
	file := parseSource(t, versioned("layout(1) in;"))
	const want = "Expected identifier, found IntConstant(Integer { value: 1, signed: true, width: 32 })"
	if len(file.Errors) != 1 {
		t.Fatalf("errors = %v, want exactly the bad-entry diagnostic", file.Errors)
	}
	if got := file.Errors[0].Message; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
	decl := wantDeclKind[declQualifiers](t, onlyDecl(t, file))
	if len(decl.Qualifiers.Layout) != 0 {
		t.Errorf("%d layout entries, want none: the rejected entry is not kept", len(decl.Qualifiers.Layout))
	}
	if decl.Qualifiers.Storage != storageInput {
		t.Errorf("storage = %v, want in", decl.Qualifiers.Storage)
	}
}

func TestParseUnknownType(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"struct member", "struct S { T x; };"},
		{"precision declaration", "precision highp T;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := parseSource(t, versioned(tt.src))
			if len(file.Errors) != 1 {
				t.Fatalf("errors = %v, want exactly one", file.Errors)
			}
			const want = "Unknown type: T"
			if got := file.Errors[0].Message; got != want {
				t.Errorf("message = %q, want %q", got, want)
			}
		})
	}
}
