package valid_test

import (
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
)

// locationModule is a vertex entry point whose arguments are the types in
// argTypes, each at its own @location, and whose result is @builtin(position).
// It is the shape the GLSL and WGSL frontends lower a location varying to.
func locationModule(argTypes ...ir.TypeInner) *ir.Module {
	types := make([]ir.Type, 0, len(argTypes)+1)
	args := make([]ir.FunctionArgument, 0, len(argTypes))
	for i, inner := range argTypes {
		loc := ir.Binding(ir.LocationBinding{Location: uint32(i)})
		args = append(args, ir.FunctionArgument{Name: "in", Type: ir.TypeHandle(len(types)), Binding: &loc})
		types = append(types, ir.Type{Inner: inner})
	}
	position := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
	resultType := ir.TypeHandle(len(types))
	types = append(types, ir.Type{Inner: testVec4})
	return &ir.Module{
		Types: types,
		EntryPoints: []ir.EntryPoint{{
			Name:  "vs",
			Stage: ir.StageVertex,
			Function: ir.Function{
				Name:      "vs",
				Arguments: args,
				Result:    &ir.FunctionResult{Type: resultType, Binding: &position},
			},
		}},
	}
}

// TestMatrixLocationVaryingRejected is card P1-IR-036: only a numeric scalar
// or vector may hold a user-defined entry point input or output. Rust naga
// checks the type against TypeFlags::IO_SHAREABLE (valid/interface.rs
// VaryingContext::validate_impl 733-739, VaryingError::NotIOShareableType).
// A matrix used to validate here and then broke every text backend: MSL
// writes an attribute and HLSL a LOCn semantic that neither accepts.
func TestMatrixLocationVaryingRejected(t *testing.T) {
	const want = "Only numeric scalars and vectors are allowed"
	matrix := ir.MatrixType{Columns: ir.Vec4, Rows: ir.Vec4, Scalar: testF32}

	t.Run("ir", func(t *testing.T) {
		expectRejected(t, locationModule(matrix), valid.Options{Capabilities: valid.CapAll}, want)
	})
	t.Run("glsl", func(t *testing.T) {
		expectRejected(t, parseGLSL(t, `#version 450
layout(location = 2) in mat4 inst;
void main() {
    gl_Position = vec4(inst[0].x);
}`), valid.Options{Capabilities: valid.CapAll}, want)
	})
}

// TestLocationVaryingTypesAccepted keeps the legal location types valid: a
// numeric scalar, vectors of f32 and, with the capability, of f16.
func TestLocationVaryingTypesAccepted(t *testing.T) {
	f16 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}
	vec2f16 := ir.VectorType{Size: ir.Vec2, Scalar: f16}

	t.Run("ir", func(t *testing.T) {
		expectValid(t, locationModule(testF32, testVec3, vec2f16), valid.Options{Capabilities: valid.CapAll})
	})
	t.Run("glsl", func(t *testing.T) {
		expectValid(t, parseGLSL(t, `#version 450
layout(location = 2) in vec4 inst;
void main() {
    gl_Position = inst;
}`), valid.Options{Capabilities: valid.CapAll})
	})
}
