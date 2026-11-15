package valid_test

import (
	"testing"

	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
)

// parseGLSL lowers a vertex shader for a validation test.
func parseGLSL(t *testing.T, src string) *ir.Module {
	t.Helper()
	m, _, err := glsl.Parse(src, glsl.ParseOptions{Stage: ir.StageVertex})
	if err != nil {
		t.Fatalf("glsl parse: %v", err)
	}
	return m
}

var (
	testF32  = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	testI32  = ir.ScalarType{Kind: ir.ScalarSint, Width: 4}
	testVec3 = ir.VectorType{Size: ir.Vec3, Scalar: testF32}
	testVec4 = ir.VectorType{Size: ir.Vec4, Scalar: testF32}
)

// storeModule is a module with one function whose body stores the value of
// expression [1] through the pointer to the local variable of expression [0];
// the value is a zero value of valueType.
func storeModule(localType, valueType ir.TypeInner) *ir.Module {
	return &ir.Module{
		Types: []ir.Type{{Inner: localType}, {Inner: valueType}},
		Functions: []ir.Function{{
			Name:      "f",
			LocalVars: []ir.LocalVariable{{Name: "a", Type: 0}},
			Expressions: []ir.Expression{
				{Kind: ir.ExprLocalVariable{Variable: 0}},
				{Kind: ir.ExprZeroValue{Type: 1}},
			},
			Body: []ir.Statement{{Kind: ir.StmtStore{Pointer: 0, Value: 1}}},
		}},
	}
}

// localInitModule is a module with one function whose only local variable has
// type varType and the initializer expression [0].
func localInitModule(varType ir.TypeInner, init ir.ExpressionKind) *ir.Module {
	initHandle := ir.ExpressionHandle(0)
	return &ir.Module{
		Types: []ir.Type{{Inner: varType}},
		Functions: []ir.Function{{
			Name:        "f",
			LocalVars:   []ir.LocalVariable{{Name: "i", Type: 0, Init: &initHandle}},
			Expressions: []ir.Expression{{Kind: init}},
		}},
	}
}

// TestStoreTypeMismatch is card P0-IR-035: a store whose value does not have
// the pointer's base type is invalid. Rust naga compares the value's type with
// the pointer's base type in its Statement::Store arm (valid/function.rs
// 1063-1122, FunctionError::InvalidStoreTypes, "The type of [v] doesn't match
// the type stored in [p]"). The GLSL frontend keeps the declared type, so the
// three shaders below reach the validator with the mismatch Rust rejects; on
// main Validate accepts them and spirv-val then rejects the emitted OpStore.
func TestStoreTypeMismatch(t *testing.T) {
	const want = "doesn't match the type stored in"
	tests := []struct {
		name string
		src  string
	}{
		{"matrix dimensions", `#version 450
void main() {
    mat3 m = mat3(1.0);
    mat3x2 c = m;
    gl_Position = vec4(0.0);
}`},
		{"vector size", `#version 450
void main() {
    vec4 v = vec4(1.0);
    vec3 w = v;
    gl_Position = vec4(w, 1.0);
}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectRejected(t, parseGLSL(t, tt.src), valid.Options{Capabilities: valid.CapAll}, want)
		})
	}
}

// TestStoreTypeMismatchIR is the same rule on a module built by hand, the way
// the SPIR-V frontend lowers a mismatched OpStore, with Rust's exact message.
func TestStoreTypeMismatchIR(t *testing.T) {
	m := storeModule(testVec3, testVec4)
	expectRejected(t, m, valid.Options{Capabilities: valid.CapAll},
		"The type of [1] doesn't match the type stored in [0]")
}

// TestLocalInitializerTypeMismatch is the initializer half of P0-IR-035: a
// local variable whose initializer does not have the variable's type is
// invalid (Rust valid/function.rs validate_local_var 1858-1861,
// LocalVariableError::InitializerType). GLSL converts an initializer only when
// the conversion raises its type's power, so `int i = 1.9` keeps an f32
// initializer for an i32 variable, as in Rust.
func TestLocalInitializerTypeMismatch(t *testing.T) {
	const want = "Initializer doesn't match the variable type"
	t.Run("glsl", func(t *testing.T) {
		expectRejected(t, parseGLSL(t, `#version 450
void main() {
    int i = 1.9;
    gl_Position = vec4(float(i));
}`), valid.Options{Capabilities: valid.CapAll}, want)
	})
	t.Run("ir", func(t *testing.T) {
		m := localInitModule(testI32, ir.Literal{Value: ir.LiteralF32(1.9)})
		expectRejected(t, m, valid.Options{Capabilities: valid.CapAll},
			"Local variable [0] 'i' is invalid: Initializer doesn't match the variable type")
	})
}

// TestAtomicStoreOfScalarAccepted keeps Rust's exception from the store rule:
// the IR allows storing a scalar through a pointer to an atomic of that
// scalar (valid/function.rs:1101-1103), which is how a SPIR-V OpAtomicStore
// and a WGSL atomicStore lower.
func TestAtomicStoreOfScalarAccepted(t *testing.T) {
	u32 := ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	m := &ir.Module{
		Types: []ir.Type{
			{Inner: u32},
			{Inner: ir.AtomicType{Scalar: u32}},
		},
		GlobalVariables: []ir.GlobalVariable{{
			Name:   "a",
			Space:  ir.SpaceStorage,
			Type:   1,
			Access: ir.StorageAccessLoad | ir.StorageAccessStore,
		}},
		Functions: []ir.Function{{
			Name: "f",
			Expressions: []ir.Expression{
				{Kind: ir.ExprGlobalVariable{Variable: 0}},
				{Kind: ir.ExprZeroValue{Type: 0}},
			},
			Body: []ir.Statement{{Kind: ir.StmtStore{Pointer: 0, Value: 1}}},
		}},
	}
	expectValid(t, m, valid.Options{Capabilities: valid.CapAll})
}

// TestStoreAndInitializerTypesAccepted keeps the well-typed neighbors valid:
// a store of the pointer's own type, a local initialized with a value of its
// type, and the GLSL shaders the mismatch cases derive from.
func TestStoreAndInitializerTypesAccepted(t *testing.T) {
	t.Run("store of the pointer's type", func(t *testing.T) {
		expectValid(t, storeModule(testVec3, testVec3), valid.Options{Capabilities: valid.CapAll})
	})
	t.Run("initializer of the variable's type", func(t *testing.T) {
		m := localInitModule(testI32, ir.Literal{Value: ir.LiteralI32(1)})
		expectValid(t, m, valid.Options{Capabilities: valid.CapAll})
	})
	t.Run("glsl matching store and initializer", func(t *testing.T) {
		expectValid(t, parseGLSL(t, `#version 450
void main() {
    vec4 v = vec4(1.0);
    vec3 w = v.xyz;
    int i = 3;
    gl_Position = vec4(w, float(i));
}`), valid.Options{Capabilities: valid.CapAll})
	})
}
