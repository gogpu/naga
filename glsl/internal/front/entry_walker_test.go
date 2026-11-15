package front

// Contracts of the entry-point wrapper: Rust naga's add_entry_point
// (front/glsl/functions.rs:1218-1386) and its arg_type_walker
// (functions.rs:1401-1502). A module-scope in/out variable of a composite
// type is flattened into one argument or result member per leaf, each with its
// own binding; the wrapper stores the arguments into their private globals
// before main and loads the outputs after it.

import (
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/valid"
)

// onlyEntry returns the module's single entry point.
func onlyEntry(t *testing.T, mod *ir.Module) ir.EntryPoint {
	t.Helper()
	if len(mod.EntryPoints) != 1 {
		t.Fatalf("module has %d entry points, want 1", len(mod.EntryPoints))
	}
	return mod.EntryPoints[0]
}

// locationOf tests the binding is a location binding and returns it.
func locationBinding(t *testing.T, what string, binding *ir.Binding) ir.LocationBinding {
	t.Helper()
	if binding == nil {
		t.Fatalf("%s has no binding", what)
	}
	location, ok := (*binding).(ir.LocationBinding)
	if !ok {
		t.Fatalf("%s binding = %T, want a location binding", what, *binding)
	}
	return location
}

// TestEntryArrayedInputTakesConsecutiveLocations is the array arm of Rust's
// arg_type_walker (functions.rs:1410-1444): an array of a location-bound
// variable is walked element by element, each element taking the next
// location, and the element's scalar kind picks the interpolation
// (Perspective for floats).
func TestEntryArrayedInputTakesConsecutiveLocations(t *testing.T) {
	source := `#version 450
layout(location = 0) in vec4 arr[2];
void main() {
    gl_Position = arr[0] + arr[1];
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageVertex})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}
	args := onlyEntry(t, mod).Function.Arguments
	if len(args) != 2 {
		t.Fatalf("entry point has %d arguments, want one per array element", len(args))
	}
	for i, arg := range args {
		if arg.Name != "arr" {
			t.Fatalf("argument %d is named %q, want the array's name", i, arg.Name)
		}
		binding := locationBinding(t, "the element argument", arg.Binding)
		if binding.Location != uint32(i) {
			t.Fatalf("argument %d location = %d, want the consecutive location %d", i, binding.Location, i)
		}
		if binding.Interpolation == nil || *binding.Interpolation != ir.InterpolationPerspective {
			t.Fatalf("argument %d interpolation = %v, want the float element's Perspective", i, binding.Interpolation)
		}
	}
}

// TestEntryArrayedIntegerInputIsFlat is the scalar_kind arm of the array walk
// (functions.rs:1432-1441): an integer element is Flat, not Perspective.
func TestEntryArrayedIntegerInputIsFlat(t *testing.T) {
	source := `#version 450
layout(location = 0) in ivec2 ids[2];
void main() {
    gl_Position = vec4(float(ids[0].x + ids[1].y));
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageVertex})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	args := onlyEntry(t, mod).Function.Arguments
	if len(args) != 2 {
		t.Fatalf("entry point has %d arguments, want 2", len(args))
	}
	for i, arg := range args {
		binding := locationBinding(t, "the element argument", arg.Binding)
		if binding.Location != uint32(i) {
			t.Fatalf("argument %d location = %d, want %d", i, binding.Location, i)
		}
		if binding.Interpolation == nil || *binding.Interpolation != ir.InterpolationFlat {
			t.Fatalf("argument %d interpolation = %v, want Flat for an integer element", i, binding.Interpolation)
		}
	}
}

// TestEntryArrayedOutputSpansLocations is the same walk on the result side:
// each element becomes a member with its own location, and the members are
// laid out end to end by the type's size (functions.rs:1318-1360).
func TestEntryArrayedOutputSpansLocations(t *testing.T) {
	source := `#version 450
layout(location = 2) out vec4 colors[2];
void main() {
    colors[0] = vec4(1.0);
    colors[1] = vec4(2.0);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}
	entry := onlyEntry(t, mod)
	if entry.Function.Result == nil {
		t.Fatal("the entry point has no result struct")
	}
	result := mod.Types[entry.Function.Result.Type].Inner.(ir.StructType)
	if len(result.Members) != 2 {
		t.Fatalf("result struct has %d members, want one per array element: %+v", len(result.Members), result.Members)
	}
	for i, member := range result.Members {
		binding := locationBinding(t, "the result member", member.Binding)
		if binding.Location != uint32(2+i) {
			t.Fatalf("member %d location = %d, want the declared location + %d", i, binding.Location, i)
		}
		if want := uint32(i) * 16; member.Offset != want {
			t.Fatalf("member %d offset = %d, want %d", i, member.Offset, want)
		}
	}
}

// TestEntryStructInputDistributesLocations is the struct arm of Rust's
// arg_type_walker (functions.rs:1446-1487): a member without a binding takes
// the next free location, and its scalar kind picks the interpolation, so an
// integer member is Flat while a float member is Perspective.
func TestEntryStructInputDistributesLocations(t *testing.T) {
	source := `#version 450
struct S { vec3 pos; int id; };
layout(location = 0) in S s;
void main() {
    gl_Position = vec4(s.pos, float(s.id));
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageVertex})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}
	args := onlyEntry(t, mod).Function.Arguments
	if len(args) != 2 {
		t.Fatalf("entry point has %d arguments, want one per struct member", len(args))
	}
	want := []struct {
		name          string
		location      uint32
		interpolation ir.Interpolation
	}{
		{"pos", 0, ir.InterpolationPerspective},
		{"id", 1, ir.InterpolationFlat},
	}
	for i, arg := range args {
		if arg.Name != want[i].name {
			t.Fatalf("argument %d is named %q, want %q", i, arg.Name, want[i].name)
		}
		binding := locationBinding(t, "the member argument", arg.Binding)
		if binding.Location != want[i].location {
			t.Fatalf("argument %q location = %d, want %d", arg.Name, binding.Location, want[i].location)
		}
		if binding.Interpolation == nil || *binding.Interpolation != want[i].interpolation {
			t.Fatalf("argument %q interpolation = %v, want %v", arg.Name, binding.Interpolation, want[i].interpolation)
		}
	}
}

// TestEntryStructOutputDistributesLocations is the struct arm on the result
// side: every member becomes a result member with its own location and the
// members are laid end to end.
func TestEntryStructOutputDistributesLocations(t *testing.T) {
	source := `#version 450
struct O { vec4 a; vec2 b; };
layout(location = 0) out O o;
void main() {
    o.a = vec4(1.0);
    o.b = vec2(0.0);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}
	entry := onlyEntry(t, mod)
	result := mod.Types[entry.Function.Result.Type].Inner.(ir.StructType)
	if len(result.Members) != 2 {
		t.Fatalf("result struct has %d members, want 2: %+v", len(result.Members), result.Members)
	}
	wantOffsets := []uint32{0, 16}
	for i, member := range result.Members {
		binding := locationBinding(t, "the result member", member.Binding)
		if binding.Location != uint32(i) {
			t.Fatalf("member %q location = %d, want %d", member.Name, binding.Location, i)
		}
		if member.Offset != wantOffsets[i] {
			t.Fatalf("member %q offset = %d, want %d", member.Name, member.Offset, wantOffsets[i])
		}
	}
}

// TestEntryWalkerUsesStructMemberBinding is the member.binding arm of Rust's
// struct walk (functions.rs:1462-1465): a member that carries its own binding
// keeps it instead of taking the next location, and the following members
// resume at the shared location.
func TestEntryWalkerUsesStructMemberBinding(t *testing.T) {
	fe, c := newTestContext(t)
	vec4 := fe.registerType(ir.VectorType{Size: ir.Vec4, Scalar: f32Scalar})
	vec2 := fe.registerType(ir.VectorType{Size: ir.Vec2, Scalar: f32Scalar})
	builtin := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
	ty := fe.registerType(ir.StructType{Members: []ir.StructMember{
		{Name: "position", Type: vec4, Binding: &builtin},
		{Name: "uv", Type: vec2},
	}})
	pointer := c.appendExpr(ir.ExprGlobalVariable{Variable: 0})

	type leaf struct {
		name    string
		binding ir.Binding
	}
	var leaves []leaf
	err := fe.argTypeWalker(c, "s", ir.LocationBinding{Location: 3}, pointer, ty,
		func(_ *Context, name string, _ ir.ExpressionHandle, _ ir.TypeHandle, binding ir.Binding) *frontError {
			leaves = append(leaves, leaf{name: name, binding: binding})
			return nil
		})
	if err != nil {
		t.Fatalf("argTypeWalker: %v", err)
	}
	if len(leaves) != 2 {
		t.Fatalf("walked %d leaves, want 2", len(leaves))
	}
	if got, ok := leaves[0].binding.(ir.BuiltinBinding); !ok || got.Builtin != ir.BuiltinPosition {
		t.Fatalf("member with a binding walks with %#v, want its own Position binding", leaves[0].binding)
	}
	location := locationBinding(t, "the unbound member", &leaves[1].binding)
	if location.Location != 3 {
		t.Fatalf("unbound member location = %d, want the shared location 3", location.Location)
	}
}

// TestEntryWrapperStoresArgumentsBeforeCall pins the body order of Rust's
// add_entry_point (functions.rs:1224-1262, 1310-1360): one store per input
// argument, the module-scope initializers, the call of main, then one Emit per
// output load and the result compose, and the return.
func TestEntryWrapperStoresArgumentsBeforeCall(t *testing.T) {
	source := `#version 450
layout(location = 0) in vec3 pos;
layout(location = 0) out vec4 o;
void main() {
    gl_Position = vec4(pos, 1.0);
    o = vec4(1.0);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageVertex})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	entry := onlyEntry(t, mod)
	// One input argument, one output that is not a builtin, the Position
	// builtin, and the result compose.
	wantWrapperShape(t, entry.Function.Body, "store", "call", "emit", "emit", "emit", "return")
	// The argument store reads function argument 0 and writes the input
	// global, whose access chain the argument carried.
	store, ok := entry.Function.Body[0].Kind.(ir.StmtStore)
	if !ok {
		t.Fatalf("body[0] = %T, want the argument store", entry.Function.Body[0].Kind)
	}
	arg, ok := entry.Function.Expressions[store.Value].Kind.(ir.ExprFunctionArgument)
	if !ok || arg.Index != 0 {
		t.Fatalf("the stored value is %#v, want function argument 0", entry.Function.Expressions[store.Value].Kind)
	}
	// Every Emit covers exactly the expression the wrapper appended for it.
	for i, stmt := range entry.Function.Body {
		emit, ok := stmt.Kind.(ir.StmtEmit)
		if !ok {
			continue
		}
		if emit.Range.End-emit.Range.Start != 1 {
			t.Fatalf("body[%d] Emit covers [%d, %d), want one expression", i, emit.Range.Start, emit.Range.End)
		}
	}
}

// TestEntryStructOutputCompilesToSPIRV compiles the flattened struct output
// through the SPIR-V backend: the result compose over the loaded members has
// to be a valid value, which a missing Emit or a wrong member order breaks.
func TestEntryStructOutputCompilesToSPIRV(t *testing.T) {
	source := `#version 450
struct O { vec4 a; vec2 b; };
layout(location = 0) out O o;
void main() {
    o = O(vec4(1.0), vec2(0.5));
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, _, cerr := spirv.Compile(mod, spirv.Options{}); cerr != nil {
		t.Fatalf("the SPIR-V backend refused the module: %v", cerr)
	}
}

// wantWrapperShape asserts the statement kinds of the wrapper body in order,
// with StmtCall named "call" on top of the shared stmtName names.
func wantWrapperShape(t *testing.T, body ir.Block, want ...string) {
	t.Helper()
	got := make([]string, len(body))
	for i, stmt := range body {
		got[i] = stmtName(stmt.Kind)
		if _, ok := stmt.Kind.(ir.StmtCall); ok {
			got[i] = "call"
		}
	}
	if len(got) != len(want) {
		t.Fatalf("wrapper body = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("wrapper body = %v, want %v", got, want)
		}
	}
}

// TestTypeSize mirrors Rust's TypeInner::size for the shapes the result
// struct's offsets are computed with (functions.rs:1330-1338).
func TestTypeSize(t *testing.T) {
	fe, _ := newTestContext(t)
	f32 := fe.registerType(f32Scalar)
	three := uint32(3)
	tests := []struct {
		name string
		ty   ir.TypeHandle
		want uint32
	}{
		{"scalar", f32, 4},
		{"vec3", fe.registerType(ir.VectorType{Size: ir.Vec3, Scalar: f32Scalar}), 12},
		{"mat2x3", fe.registerType(ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec3, Scalar: f32Scalar}), 32},
		{"array of three", fe.registerType(ir.ArrayType{Base: f32, Size: ir.ArraySize{Constant: &three}, Stride: 8}), 24},
		{"runtime array", fe.registerType(ir.ArrayType{Base: f32, Size: ir.ArraySize{}, Stride: 8}), 8},
		{"struct", fe.registerType(ir.StructType{Span: 64}), 64},
		{"opaque", fe.registerType(ir.SamplerType{}), 0},
	}
	for _, tt := range tests {
		if got := fe.typeSize(tt.ty); got != tt.want {
			t.Errorf("%s: typeSize = %d, want %d", tt.name, got, tt.want)
		}
	}
}
