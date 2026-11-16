package snapshot_test

// The image-query implementer audit: every type that implements ir.ImageQuery
// is listed here, and the table is checked against the method set of
// ir/expression.go, so a sixth query kind cannot join the interface without a
// row. Each row pins the query's IR result type and drives the query through
// valid.Validate and the four text backends, which is what "handled" means for
// a marker interface with a type switch in every consumer.
//
// The matrix that drives the frontends is elsewhere: the GLSL frontend's
// textureQueryLod rows live in glsl_texture_query_lod_test.go, and the image
// and sampling matrix (glsl_image_sampling_matrix_test.go) covers the WGSL-
// reachable queries. This file is about the IR interface itself.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"sort"
	"testing"

	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/internal/typifier"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/valid"
)

// imageQueryKindRow is one implementer of ir.ImageQuery.
type imageQueryKindRow struct {
	// name is the implementer's type name in ir.
	name string
	// image is the image the query runs on: the shape the IR permits.
	image ir.ImageType
	// lod marks the level-of-detail query, which needs a sampler global and a
	// coordinate alongside the image.
	lod bool
	// query builds the query over the given operand handles.
	query func(image, sampler, coordinate ir.ExpressionHandle) ir.ImageQuery
	// want is the query's IR result type, the one the typifier must report.
	want ir.TypeInner
}

var (
	auditSampled2D = ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}
	auditUScalar   = ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	auditUVec2     = ir.VectorType{Size: ir.Vec2, Scalar: auditUScalar}
	auditFVec2     = ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
)

// imageQueryKindRows lists every ir.ImageQuery implementer. The order is the
// order of the interface's declarations in ir/expression.go.
var imageQueryKindRows = []imageQueryKindRow{
	{
		name:  "ImageQuerySize",
		image: auditSampled2D,
		query: func(_, _, _ ir.ExpressionHandle) ir.ImageQuery { return ir.ImageQuerySize{} },
		want:  auditUVec2,
	},
	{
		name:  "ImageQueryNumLevels",
		image: auditSampled2D,
		query: func(_, _, _ ir.ExpressionHandle) ir.ImageQuery { return ir.ImageQueryNumLevels{} },
		want:  auditUScalar,
	},
	{
		name:  "ImageQueryNumLayers",
		image: ir.ImageType{Dim: ir.Dim2D, Arrayed: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat},
		query: func(_, _, _ ir.ExpressionHandle) ir.ImageQuery { return ir.ImageQueryNumLayers{} },
		want:  auditUScalar,
	},
	{
		name:  "ImageQueryNumSamples",
		image: ir.ImageType{Dim: ir.Dim2D, Multisampled: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat},
		query: func(_, _, _ ir.ExpressionHandle) ir.ImageQuery { return ir.ImageQueryNumSamples{} },
		want:  auditUScalar,
	},
	{
		name:  "ImageQueryLod",
		image: auditSampled2D,
		lod:   true,
		query: func(_, sampler, coordinate ir.ExpressionHandle) ir.ImageQuery {
			return ir.ImageQueryLod{Sampler: sampler, Coordinate: coordinate}
		},
		want: auditFVec2,
	},
}

// buildImageQueryKindModule builds a fragment module that computes one image
// query over a global image (and, for the lod query, a global sampler and a
// splatted coordinate) and stores the result into a local variable. It returns
// the module and the query expression's handle.
func buildImageQueryKindModule(row imageQueryKindRow) (*ir.Module, ir.ExpressionHandle) {
	var samplerHandle ir.ExpressionHandle
	expressions := []ir.Expression{{Kind: ir.ExprGlobalVariable{Variable: 0}}}
	firstEmitted := ir.ExpressionHandle(1)
	if row.lod {
		expressions = append(expressions,
			ir.Expression{Kind: ir.ExprGlobalVariable{Variable: 1}},
			ir.Expression{Kind: ir.Literal{Value: ir.LiteralF32(0)}},
			// A vec2 splat of the literal: the coordinate of a 2D lod query.
			ir.Expression{Kind: ir.ExprSplat{Size: ir.Vec2, Value: 2}},
		)
		samplerHandle = 1
		firstEmitted = 3
	}
	queryHandle := ir.ExpressionHandle(len(expressions))
	expressions = append(expressions, ir.Expression{Kind: ir.ExprImageQuery{
		Image: 0,
		Query: row.query(0, samplerHandle, 3),
	}})
	localHandle := ir.ExpressionHandle(len(expressions))
	expressions = append(expressions, ir.Expression{Kind: ir.ExprLocalVariable{Variable: 0}})

	globals := []ir.GlobalVariable{{
		Name:    "image",
		Space:   ir.SpaceHandle,
		Type:    0,
		Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
	}}
	if row.lod {
		globals = append(globals, ir.GlobalVariable{
			Name:    "sampler",
			Space:   ir.SpaceHandle,
			Type:    1,
			Binding: &ir.ResourceBinding{Group: 0, Binding: 1},
		})
	}

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "image", Inner: row.image},
			{Name: "sampler", Inner: ir.SamplerType{}},
			{Name: "result", Inner: row.want},
		},
		GlobalVariables: globals,
		EntryPoints: []ir.EntryPoint{{
			Name:  "main",
			Stage: ir.StageFragment,
			Function: ir.Function{
				Name:        "main",
				LocalVars:   []ir.LocalVariable{{Name: "result", Type: 2}},
				Expressions: expressions,
				Body: []ir.Statement{
					// Globals and literals are pre-emit expressions; the Emit
					// statement introduces what the query needs computed.
					{Kind: ir.StmtEmit{Range: ir.Range{Start: firstEmitted, End: queryHandle + 1}}},
					{Kind: ir.StmtStore{Pointer: localHandle, Value: queryHandle}},
				},
			},
		}},
	}
	return module, queryHandle
}

// TestImageQueryKindAudit drives every ir.ImageQuery implementer through the
// typifier, the validator and the four backends.
func TestImageQueryKindAudit(t *testing.T) {
	implementers := imageQueryImplementers(t)
	listed := make([]string, len(imageQueryKindRows))
	for i, row := range imageQueryKindRows {
		listed[i] = row.name
	}
	sort.Strings(listed)
	if !reflect.DeepEqual(implementers, listed) {
		t.Fatalf("ir.ImageQuery implementers are %v, but imageQueryKindRows lists %v; "+
			"a new query kind needs a row here and a case in every consumer", implementers, listed)
	}

	for _, row := range imageQueryKindRows {
		t.Run(row.name, func(t *testing.T) {
			module, queryHandle := buildImageQueryKindModule(row)

			types, err := typifier.Function(module, &module.EntryPoints[0].Function)
			if err != nil {
				t.Fatalf("typify: %v", err)
			}
			if got := resolutionInner(module, types.Get(queryHandle)); !reflect.DeepEqual(got, row.want) {
				t.Fatalf("result type is %v, want %v", got, row.want)
			}
			if _, err := valid.Validate(module, valid.Options{Capabilities: valid.CapAll}); err != nil {
				t.Fatalf("validate: %v", err)
			}
			if _, _, err := spirv.Compile(module, spirv.Options{}); err != nil {
				t.Fatalf("spirv: %v", err)
			}
			// The MSL backend refuses the lod query below MSL 2.2 (SPIRV-Cross's
			// supports_msl_version check for OpImageQueryLod), so the audit
			// writes at 2.2.
			if _, _, err := msl.Compile(module, msl.Options{LangVersion: msl.Version{Major: 2, Minor: 2}}); err != nil {
				t.Fatalf("msl: %v", err)
			}
			if _, _, err := hlsl.Compile(module, hlsl.Options{}); err != nil {
				t.Fatalf("hlsl: %v", err)
			}
			if _, _, err := glsl.Compile(module, glsl.Options{LangVersion: glsl.Version450}); err != nil {
				t.Fatalf("glsl: %v", err)
			}
		})
	}
}

// imageQueryImplementers returns the names of the types whose method set
// includes imageQuery, read from ir/expression.go's own source. The scan is
// the point: a hand-kept list drifts the moment a kind is added.
func imageQueryImplementers(t *testing.T) []string {
	t.Helper()
	const path = "../ir/expression.go"
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var names []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name.Name != "imageQuery" {
			continue
		}
		if len(fn.Recv.List) != 1 {
			continue
		}
		recv := fn.Recv.List[0].Type
		if star, ok := recv.(*ast.StarExpr); ok {
			recv = star.X
		}
		if ident, ok := recv.(*ast.Ident); ok {
			names = append(names, ident.Name)
		}
	}
	if len(names) == 0 {
		t.Fatalf("no imageQuery methods found in %s; its shape changed, update this scan", path)
	}
	sort.Strings(names)
	return names
}

// resolutionInner returns the TypeInner a typifier resolution names: the module
// type of a handle, or the inline value.
func resolutionInner(module *ir.Module, r ir.TypeResolution) ir.TypeInner {
	if r.Handle != nil && int(*r.Handle) < len(module.Types) {
		return module.Types[*r.Handle].Inner
	}
	return r.Value
}
