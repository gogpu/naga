package irutil

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

// walkerRegistry lists one zero value of every type that implements an IR
// marker interface the walkers have to look into: the expression and
// statement kinds and the variants of the interfaces nested in them. Go
// cannot list a package's types at run time, so the list is written out;
// TestWalkersCoverEveryKind compares it with the declarations it parses
// from package ir, so a kind added there without an entry here fails.
var walkerRegistry = []any{
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
	handleType    = reflect.TypeFor[ir.ExpressionHandle]()
	statementType = reflect.TypeFor[ir.Statement]()
	emitType      = reflect.TypeFor[ir.StmtEmit]()
	exprKindType  = reflect.TypeFor[ir.ExpressionKind]()
	stmtKindType  = reflect.TypeFor[ir.StatementKind]()
)

// handleOffset is what the remap table adds to every handle, so a handle a
// walker skips keeps its old value and one it maps from the wrong field gets
// the wrong value.
const handleOffset = 100000

// TestWalkersCoverEveryKind pins the coverage of the expression walker
// (mapExprHandles) and the statement walker (blockMapper), the one copy of
// each that compaction, inlining and pipeline-constant processing share.
//
// It parses package ir for every type with an expressionKind or
// statementKind method, and for the variants of every marker interface
// nested in them (sample level, image query, atomic and ray-query
// functions, gather mode, ...), and requires walkerRegistry to list exactly
// those. It then builds every kind, once per variant of its nested
// interfaces, with each ExpressionHandle field (also inside slices, pointers,
// structs, interfaces and nested blocks) set to a distinct handle, and
// checks that
//   - the walkers do not panic, which they do on a kind they have no case
//     for;
//   - remapping through a table that adds handleOffset changes every handle
//     field to its own value plus handleOffset, in place and as a fresh copy;
//   - the expression walker and the fresh statement walker leave their input
//     alone;
//   - the tracers compaction runs (markExprHandleRefs,
//     markStmtExprRefsForCompact) mark every handle, except those of an Emit
//     range.
func TestWalkersCoverEveryKind(t *testing.T) {
	declared := parseMarkerImplementations(t, filepath.Join("..", "..", "ir"))
	registry := map[string]reflect.Type{}
	for _, v := range walkerRegistry {
		ty := reflect.TypeOf(v)
		if _, dup := registry[ty.Name()]; dup {
			t.Fatalf("walkerRegistry lists %s twice", ty.Name())
		}
		registry[ty.Name()] = ty
	}

	b := &kindBuilder{registry: registry, checked: map[reflect.Type]bool{}}
	b.checkInterface(t, exprKindType, declared)
	b.checkInterface(t, stmtKindType, declared)
	if t.Failed() {
		t.FailNow()
	}
	b.declared = declared

	for _, kind := range b.implementations(exprKindType) {
		t.Run(kind.Name(), func(t *testing.T) {
			for variant := 0; variant < b.variantsOf(t, kind); variant++ {
				checkExpressionWalker(t, b.build(t, kind, variant).(ir.ExpressionKind))
			}
		})
	}
	for _, kind := range b.implementations(stmtKindType) {
		t.Run(kind.Name(), func(t *testing.T) {
			for variant := 0; variant < b.variantsOf(t, kind); variant++ {
				checkStatementWalker(t, b, kind, variant)
			}
		})
	}
}

func checkExpressionWalker(t *testing.T, kind ir.ExpressionKind) {
	t.Helper()
	before := collectHandles(reflect.ValueOf(kind))
	remap := offsetTable(before)

	var got ir.ExpressionKind
	if err := noPanic(func() { got = RemapExpressionHandles(kind, remap) }); err != nil {
		t.Fatalf("%#v: %v", kind, err)
	}
	expectOffset(t, fmt.Sprintf("RemapExpressionHandles(%T)", kind), before, collectHandles(reflect.ValueOf(got)))
	if after := collectHandles(reflect.ValueOf(kind)); !slices.Equal(after, before) {
		t.Errorf("RemapExpressionHandles(%T) changed its input: %v, was %v", kind, after, before)
	}

	used := make([]bool, len(remap))
	if err := noPanic(func() { markExprHandleRefs(kind, used) }); err != nil {
		t.Fatalf("markExprHandleRefs(%#v): %v", kind, err)
	}
	expectMarked(t, fmt.Sprintf("markExprHandleRefs(%T)", kind), before, used)
}

func checkStatementWalker(t *testing.T, b *kindBuilder, kind reflect.Type, variant int) {
	t.Helper()

	// In place, as compaction and pipeline-constant processing run it.
	block := ir.Block{{Kind: b.build(t, kind, variant).(ir.StatementKind)}}
	before := collectHandles(reflect.ValueOf(block))
	remap := offsetTable(before)
	if err := noPanic(func() { RemapStatementExpressionHandles(block, remap) }); err != nil {
		t.Fatalf("%#v: %v", block[0].Kind, err)
	}
	expectOffset(t, fmt.Sprintf("RemapStatementExpressionHandles(%v)", kind), before, collectHandles(reflect.ValueOf(block)))

	// As a fresh copy, as inlining runs it: the input stays as it was.
	block = ir.Block{{Kind: b.build(t, kind, variant).(ir.StatementKind)}}
	rm := remapThrough(remap)
	fresh := blockMapper{handle: rm, emit: adjustEmitRange(rm), fresh: true}
	var got ir.Block
	if err := noPanic(func() { got = fresh.block(block) }); err != nil {
		t.Fatalf("fresh %#v: %v", block[0].Kind, err)
	}
	expectOffset(t, fmt.Sprintf("fresh blockMapper(%v)", kind), before, collectHandles(reflect.ValueOf(got)))
	if after := collectHandles(reflect.ValueOf(block)); !slices.Equal(after, before) {
		t.Errorf("fresh blockMapper(%v) changed its input: %v, was %v", kind, after, before)
	}

	// The compaction tracer marks every handle but those of an Emit range.
	used := make([]bool, len(remap))
	if err := noPanic(func() { markStmtExprRefsForCompact(block, used) }); err != nil {
		t.Fatalf("markStmtExprRefsForCompact(%#v): %v", block[0].Kind, err)
	}
	var live []ir.ExpressionHandle
	if kind != emitType {
		live = before
	}
	expectMarked(t, fmt.Sprintf("markStmtExprRefsForCompact(%v)", kind), live, used)
}

// expectOffset reports every handle that is not its old value plus
// handleOffset.
func expectOffset(t *testing.T, what string, before, after []ir.ExpressionHandle) {
	t.Helper()
	if len(after) != len(before) {
		t.Errorf("%s: %d handles after, %d before", what, len(after), len(before))
		return
	}
	for i := range before {
		if after[i] != before[i]+handleOffset {
			t.Errorf("%s: handle #%d is %d, want %d (was %d)", what, i, after[i], before[i]+handleOffset, before[i])
		}
	}
}

// expectMarked reports a handle of live that is not marked in used, and a
// marked handle that is not in live.
func expectMarked(t *testing.T, what string, live []ir.ExpressionHandle, used []bool) {
	t.Helper()
	want := make([]bool, len(used))
	for _, h := range live {
		want[h] = true
	}
	for h := range used {
		if used[h] != want[h] {
			t.Errorf("%s: handle %d marked %v, want %v", what, h, used[h], want[h])
		}
	}
}

// offsetTable returns a dense remap table that adds handleOffset to every
// handle up to the largest one in hs and the one after it (an Emit range's
// exclusive end).
func offsetTable(hs []ir.ExpressionHandle) []ir.ExpressionHandle {
	n := ir.ExpressionHandle(0)
	for _, h := range hs {
		n = max(n, h)
	}
	table := make([]ir.ExpressionHandle, n+2)
	for i := range table {
		table[i] = ir.ExpressionHandle(i) + handleOffset
	}
	return table
}

func noPanic(f func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	f()
	return nil
}

// collectHandles returns every ExpressionHandle in v, in field order.
func collectHandles(v reflect.Value) []ir.ExpressionHandle {
	var out []ir.ExpressionHandle
	var walk func(v reflect.Value)
	walk = func(v reflect.Value) {
		if v.Type() == handleType {
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
			panic(fmt.Sprintf("collectHandles: map in %v", v.Type()))
		default:
		}
	}
	walk(v)
	return out
}

// kindBuilder builds kinds with every handle field set to a distinct handle.
type kindBuilder struct {
	registry map[string]reflect.Type
	declared map[string][]string
	checked  map[reflect.Type]bool
	next     ir.ExpressionHandle
}

// implementations returns the registered types that implement iface, sorted
// by name.
func (b *kindBuilder) implementations(iface reflect.Type) []reflect.Type {
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
func (b *kindBuilder) checkInterface(t *testing.T, iface reflect.Type, declared map[string][]string) {
	t.Helper()
	if b.checked[iface] {
		return
	}
	b.checked[iface] = true
	if iface.NumMethod() != 1 {
		t.Errorf("%v: want a single marker method, has %d methods", iface, iface.NumMethod())
		return
	}
	marker := iface.Method(0).Name
	want := slices.Clone(declared[marker])
	sort.Strings(want)
	var got []string
	for _, ty := range b.implementations(iface) {
		got = append(got, ty.Name())
	}
	for _, name := range want {
		if !slices.Contains(got, name) {
			t.Errorf("package ir declares %s with %s(), but walkerRegistry lacks it: add it there and give the walkers a case for it", name, marker)
		}
	}
	for _, name := range got {
		if !slices.Contains(want, name) {
			t.Errorf("walkerRegistry lists %s as %v, but package ir declares no %s.%s()", name, iface, name, marker)
		}
	}
}

// variantsOf returns how many instances of kind build needs so that every
// variant of each interface nested in it is used once: the largest number of
// variants among them, at least one.
func (b *kindBuilder) variantsOf(t *testing.T, kind reflect.Type) int {
	n := 1
	seen := map[reflect.Type]bool{}
	var visit func(ty reflect.Type)
	visit = func(ty reflect.Type) {
		if seen[ty] || ty == statementType {
			return
		}
		seen[ty] = true
		switch ty.Kind() {
		case reflect.Interface:
			b.checkInterface(t, ty, b.declared)
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
	return n
}

// build returns a value of kind with distinct handles; an interface field
// gets its variant number variant modulo the number of variants.
func (b *kindBuilder) build(t *testing.T, kind reflect.Type, variant int) any {
	t.Helper()
	b.next = 1
	v := reflect.New(kind).Elem()
	b.fill(t, v, variant)
	return v.Interface()
}

func (b *kindBuilder) fill(t *testing.T, v reflect.Value, variant int) {
	t.Helper()
	ty := v.Type()
	if ty == handleType {
		v.SetUint(uint64(b.next))
		// Gaps keep an Emit range's End-1 distinct from the next handle.
		b.next += 3
		return
	}
	if ty == statementType {
		// A statement nested in a block: a store whose handles prove the
		// walker went into the block.
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
		t.Fatalf("%v holds a map; teach the builder and collectHandles about it", ty)
	default:
		// Numbers, booleans and strings hold no handle.
	}
}

// parseMarkerImplementations parses the non-test Go files of dir and
// returns, for each method name, the types declaring a method of that name
// with no parameters and no results: the marker methods of package ir's
// sum-type interfaces (expressionKind, statementKind, sampleLevel, ...).
func parseMarkerImplementations(t *testing.T, dir string) map[string][]string {
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
