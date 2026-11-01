package dce

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// This file is the enumeration of the IR's sum types that the visitor tests
// of this package share. The same file, with another package clause and the
// path of package ir, sits in dxil/internal/emit and
// dxil/internal/passes/dce, and in internal/irutil as walk_coverage_test.go
// (kept in step by hand: a test package cannot import another's test files).
//
// Go cannot list a package's types at run time, so irKindRegistry is written
// out. newIRKinds compares it with the declarations it parses from package
// ir and fails when they differ, so a kind added there without an entry here
// fails every visitor test that uses it.

// irKindRegistry holds one zero value of every type that implements an IR
// marker interface visitors have to look into: the expression and statement
// kinds and the variants of the interfaces nested in them.
var irKindRegistry = []any{
	// ir.ExpressionKind
	ir.Literal{}, ir.ExprConstant{}, ir.ExprOverride{}, ir.ExprZeroValue{},
	ir.ExprCompose{}, ir.ExprAccess{}, ir.ExprAccessIndex{}, ir.ExprSplat{},
	ir.ExprSwizzle{}, ir.ExprFunctionArgument{}, ir.ExprGlobalVariable{},
	ir.ExprLocalVariable{}, ir.ExprLoad{}, ir.ExprAlias{}, ir.ExprPhi{},
	ir.ExprImageSample{}, ir.ExprImageLoad{}, ir.ExprImageQuery{},
	ir.ExprUnary{}, ir.ExprBinary{}, ir.ExprSelect{}, ir.ExprDerivative{},
	ir.ExprRelational{}, ir.ExprMath{}, ir.ExprAs{}, ir.ExprCallResult{},
	ir.ExprArrayLength{}, ir.ExprAtomicResult{},
	ir.ExprWorkGroupUniformLoadResult{}, ir.ExprRayQueryProceedResult{},
	ir.ExprRayQueryGetIntersection{}, ir.ExprSubgroupBallotResult{},
	ir.ExprSubgroupOperationResult{},
	// ir.StatementKind
	ir.StmtEmit{}, ir.StmtBlock{}, ir.StmtIf{}, ir.StmtSwitch{}, ir.StmtLoop{},
	ir.StmtBreak{}, ir.StmtContinue{}, ir.StmtReturn{}, ir.StmtKill{},
	ir.StmtControlBarrier{}, ir.StmtMemoryBarrier{}, ir.StmtStore{},
	ir.StmtImageStore{}, ir.StmtAtomic{}, ir.StmtImageAtomic{},
	ir.StmtWorkGroupUniformLoad{}, ir.StmtCall{}, ir.StmtRayQuery{},
	ir.StmtSubgroupBallot{}, ir.StmtSubgroupCollectiveOperation{},
	ir.StmtSubgroupGather{},
	// ir.LiteralValue
	ir.LiteralF64(0), ir.LiteralF32(0), ir.LiteralF16(0), ir.LiteralU32(0),
	ir.LiteralI32(0), ir.LiteralU64(0), ir.LiteralI64(0), ir.LiteralBool(false),
	ir.LiteralAbstractInt(0), ir.LiteralAbstractFloat(0),
	// ir.SampleLevel
	ir.SampleLevelAuto{}, ir.SampleLevelZero{}, ir.SampleLevelExact{},
	ir.SampleLevelBias{}, ir.SampleLevelGradient{},
	// ir.ImageQuery
	ir.ImageQuerySize{}, ir.ImageQueryNumLevels{}, ir.ImageQueryNumLayers{},
	ir.ImageQueryNumSamples{},
	// ir.SwitchValue
	ir.SwitchValueI32(0), ir.SwitchValueU32(0), ir.SwitchValueDefault{},
	// ir.AtomicFunction
	ir.AtomicAdd{}, ir.AtomicSubtract{}, ir.AtomicAnd{}, ir.AtomicExclusiveOr{},
	ir.AtomicInclusiveOr{}, ir.AtomicMin{}, ir.AtomicMax{}, ir.AtomicExchange{},
	ir.AtomicStore{}, ir.AtomicLoad{},
	// ir.RayQueryFunction
	ir.RayQueryInitialize{}, ir.RayQueryProceed{}, ir.RayQueryTerminate{},
	ir.RayQueryGenerateIntersection{}, ir.RayQueryConfirmIntersection{},
	// ir.GatherMode
	ir.GatherBroadcastFirst{}, ir.GatherBroadcast{}, ir.GatherShuffle{},
	ir.GatherShuffleDown{}, ir.GatherShuffleUp{}, ir.GatherShuffleXor{},
	ir.GatherQuadBroadcast{}, ir.GatherQuadSwap{},
}

var (
	irHandleType    = reflect.TypeFor[ir.ExpressionHandle]()
	irTypeHandle    = reflect.TypeFor[ir.TypeHandle]()
	irStatementType = reflect.TypeFor[ir.Statement]()
	irExprKindType  = reflect.TypeFor[ir.ExpressionKind]()
	irStmtKindType  = reflect.TypeFor[ir.StatementKind]()
	irEmitType      = reflect.TypeFor[ir.StmtEmit]()
)

// irKinds builds every kind of package ir with distinct handles.
type irKinds struct {
	registry map[string]reflect.Type
	declared map[string][]string
	checked  map[reflect.Type]bool
	broken   bool // the registry and package ir disagree
	next     ir.ExpressionHandle
	nextType ir.TypeHandle
}

// newIRKinds parses package ir in irDir and fails the test unless the
// registry lists exactly the types ir declares for each marker interface.
func newIRKinds(t *testing.T, irDir string) *irKinds {
	t.Helper()
	b := &irKinds{
		registry: map[string]reflect.Type{},
		declared: parseIRMarkers(t, irDir),
		checked:  map[reflect.Type]bool{},
	}
	for _, v := range irKindRegistry {
		ty := reflect.TypeOf(v)
		if _, dup := b.registry[ty.Name()]; dup {
			t.Fatalf("irKindRegistry lists %s twice", ty.Name())
		}
		b.registry[ty.Name()] = ty
	}
	b.checkInterface(t, irExprKindType)
	b.checkInterface(t, irStmtKindType)
	if b.broken {
		t.FailNow()
	}
	return b
}

// implementations returns the registered types that implement iface, sorted
// by name.
func (b *irKinds) implementations(iface reflect.Type) []reflect.Type {
	var out []reflect.Type
	for _, ty := range b.registry {
		if ty.Implements(iface) {
			out = append(out, ty)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// checkInterface requires the registry to list exactly the types package ir
// declares with iface's marker method.
func (b *irKinds) checkInterface(t *testing.T, iface reflect.Type) {
	t.Helper()
	if b.checked[iface] {
		return
	}
	b.checked[iface] = true
	if iface.NumMethod() != 1 {
		b.broken = true
		t.Errorf("%v: want a single marker method, has %d methods", iface, iface.NumMethod())
		return
	}
	marker := iface.Method(0).Name
	want := slices.Clone(b.declared[marker])
	sort.Strings(want)
	var got []string
	for _, ty := range b.implementations(iface) {
		got = append(got, ty.Name())
	}
	for _, name := range want {
		if !slices.Contains(got, name) {
			b.broken = true
			t.Errorf("package ir declares %s with %s(), but irKindRegistry lacks it: add it there and give the visitors under test a case for it", name, marker)
		}
	}
	for _, name := range got {
		if !slices.Contains(want, name) {
			b.broken = true
			t.Errorf("irKindRegistry lists %s as %v, but package ir declares no %s.%s()", name, iface, name, marker)
		}
	}
}

// exprKinds and stmtKinds return the registered expression and statement
// kinds.
func (b *irKinds) exprKinds() []reflect.Type { return b.implementations(irExprKindType) }
func (b *irKinds) stmtKinds() []reflect.Type { return b.implementations(irStmtKindType) }

// variants returns how many instances of kind build needs so that every
// variant of each interface nested in it is used once: the largest number of
// variants among them, at least one.
func (b *irKinds) variants(t *testing.T, kind reflect.Type) int {
	t.Helper()
	n := 1
	seen := map[reflect.Type]bool{}
	var visit func(ty reflect.Type)
	visit = func(ty reflect.Type) {
		if seen[ty] || ty == irStatementType {
			return
		}
		seen[ty] = true
		switch ty.Kind() {
		case reflect.Interface:
			b.checkInterface(t, ty)
			impls := b.implementations(ty)
			n = max(n, len(impls))
			for _, impl := range impls {
				visit(impl)
			}
		case reflect.Pointer, reflect.Slice, reflect.Array:
			visit(ty.Elem())
		case reflect.Struct:
			for i := range ty.NumField() {
				visit(ty.Field(i).Type)
			}
		default:
		}
	}
	visit(kind)
	if b.broken {
		t.FailNow()
	}
	return n
}

// build returns a value of kind with every ExpressionHandle field set to a
// distinct handle and every TypeHandle field to a distinct type handle; an
// interface field gets its variant number variant modulo the number of
// variants.
func (b *irKinds) build(t *testing.T, kind reflect.Type, variant int) any {
	t.Helper()
	b.next, b.nextType = 1, 1
	v := reflect.New(kind).Elem()
	b.fill(t, v, variant)
	return v.Interface()
}

func (b *irKinds) fill(t *testing.T, v reflect.Value, variant int) {
	t.Helper()
	ty := v.Type()
	switch ty {
	case irHandleType:
		v.SetUint(uint64(b.next))
		// Gaps keep an Emit range's End-1 distinct from the next handle.
		b.next += 3
		return
	case irTypeHandle:
		v.SetUint(uint64(b.nextType))
		b.nextType += 3
		return
	case irStatementType:
		// A statement nested in a block: a store whose handles prove the
		// visitor went into the block.
		store := reflect.New(reflect.TypeFor[ir.StmtStore]()).Elem()
		b.fill(t, store, variant)
		v.Field(0).Set(store)
		return
	}
	switch ty.Kind() {
	case reflect.Interface:
		impls := b.implementations(ty)
		if len(impls) == 0 {
			t.Fatalf("no registered variant of %v", ty)
		}
		impl := reflect.New(impls[variant%len(impls)]).Elem()
		b.fill(t, impl, variant)
		v.Set(impl)
	case reflect.Pointer:
		p := reflect.New(ty.Elem())
		b.fill(t, p.Elem(), variant)
		v.Set(p)
	case reflect.Slice:
		s := reflect.MakeSlice(ty, 2, 2)
		for i := range 2 {
			b.fill(t, s.Index(i), variant)
		}
		v.Set(s)
	case reflect.Array:
		for i := range v.Len() {
			b.fill(t, v.Index(i), variant)
		}
	case reflect.Struct:
		for i := range v.NumField() {
			b.fill(t, v.Field(i), variant)
		}
	case reflect.Map:
		t.Fatalf("%v holds a map; teach the builder and collectExprHandles about it", ty)
	default:
		// Numbers, booleans and strings hold no handle.
	}
}

// collectExprHandles returns every ExpressionHandle in v, in field order.
func collectExprHandles(v reflect.Value) []ir.ExpressionHandle {
	var out []ir.ExpressionHandle
	var walk func(v reflect.Value)
	walk = func(v reflect.Value) {
		if v.Type() == irHandleType {
			out = append(out, ir.ExpressionHandle(v.Uint()))
			return
		}
		switch v.Kind() {
		case reflect.Interface, reflect.Pointer:
			if !v.IsNil() {
				walk(v.Elem())
			}
		case reflect.Struct:
			for i := range v.NumField() {
				walk(v.Field(i))
			}
		case reflect.Slice, reflect.Array:
			for i := range v.Len() {
				walk(v.Index(i))
			}
		case reflect.Map:
			panic(fmt.Sprintf("collectExprHandles: map in %v", v.Type()))
		default:
		}
	}
	walk(v)
	return out
}

// expectHandles reports every handle of want the visitor did not visit and
// every handle it visited that is not in want.
func expectHandles(t *testing.T, what string, want []ir.ExpressionHandle, got map[ir.ExpressionHandle]bool) {
	t.Helper()
	wantSet := map[ir.ExpressionHandle]bool{}
	for _, h := range want {
		wantSet[h] = true
		if !got[h] {
			t.Errorf("%s: handle %d never visited", what, h)
		}
	}
	for h := range got {
		if !wantSet[h] {
			t.Errorf("%s: visited handle %d, which the kind does not hold", what, h)
		}
	}
}

// emitRange returns the handles an Emit statement covers.
func emitRange(r ir.Range) []ir.ExpressionHandle {
	var out []ir.ExpressionHandle
	for h := r.Start; h < r.End; h++ {
		out = append(out, h)
	}
	return out
}

// parseIRMarkers parses the non-test Go files of dir and returns, for each
// method name, the types declaring a method of that name with no parameters
// and no results: the marker methods of package ir's sum-type interfaces
// (expressionKind, statementKind, sampleLevel, ...).
func parseIRMarkers(t *testing.T, dir string) map[string][]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	out := map[string][]string{}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) != 1 || ast.IsExported(fn.Name.Name) {
				continue
			}
			if fn.Type.Params.NumFields() != 0 || fn.Type.Results.NumFields() != 0 {
				continue
			}
			recv := fn.Recv.List[0].Type
			if star, ok := recv.(*ast.StarExpr); ok {
				recv = star.X
			}
			if id, ok := recv.(*ast.Ident); ok {
				out[fn.Name.Name] = append(out[fn.Name.Name], id.Name)
			}
		}
	}
	if len(out["expressionKind"]) == 0 || len(out["statementKind"]) == 0 {
		t.Fatalf("found no expressionKind or statementKind methods in %s", dir)
	}
	return out
}
