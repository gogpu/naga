package irutil

import (
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/gogpu/naga/ir"
)

// typeHandleOffset is what the remap table adds to every type handle.
const typeHandleOffset = 100000

// TestTypeHandleWalkersCoverEveryKind pins the walkers that carry type
// handles out of expressions: markExprTypeRefs (type compaction),
// visitExprTypeHandles (type use order) and remapExprTypeHandles (type
// compaction and reordering). Most expression kinds hold no type handle, so
// unlike the expression-handle walkers these have no unknown-kind branch to
// panic in: a kind they have no case for just passes through. What is pinned
// is therefore the other direction. The test builds every expression kind,
// once per variant of its nested interfaces, with each TypeHandle field set
// to a distinct handle, and requires that, for every kind, each walker
//   - visits exactly the TypeHandle fields the kind has (none for a kind
//     with no such field), so a kind that gains a TypeHandle field fails
//     here until the walkers learn it;
//   - marks exactly those handles;
//   - maps each of them to its value plus typeHandleOffset in the copy it
//     returns, leaves the input as it was and the expression handles alone.
//
// A statement kind that holds a TypeHandle fails too: no walker looks into
// statements for type handles.
func TestTypeHandleWalkersCoverEveryKind(t *testing.T) {
	declared := parseMarkerImplementations(t, filepath.Join("..", "..", "ir"))
	registry := map[string]reflect.Type{}
	for _, v := range walkerRegistry {
		registry[reflect.TypeOf(v).Name()] = reflect.TypeOf(v)
	}
	b := &kindBuilder{registry: registry, checked: map[reflect.Type]bool{}}
	b.checkInterface(t, exprKindType, declared)
	b.checkInterface(t, stmtKindType, declared)
	if t.Failed() {
		t.FailNow()
	}
	b.declared = declared

	for _, kind := range b.implementations(stmtKindType) {
		t.Run(kind.Name(), func(t *testing.T) {
			for variant := 0; variant < b.variantsOf(t, kind); variant++ {
				if got := collectTypeHandles(reflect.ValueOf(b.build(t, kind, variant))); len(got) != 0 {
					t.Errorf("%v holds type handles %v: add a type-handle walker for statements", kind, got)
				}
			}
		})
	}
	for _, kind := range b.implementations(exprKindType) {
		t.Run(kind.Name(), func(t *testing.T) {
			for variant := 0; variant < b.variantsOf(t, kind); variant++ {
				checkTypeHandleWalkers(t, b.build(t, kind, variant).(ir.ExpressionKind))
			}
		})
	}
}

func checkTypeHandleWalkers(t *testing.T, kind ir.ExpressionKind) {
	t.Helper()
	before := collectTypeHandles(reflect.ValueOf(kind))
	exprBefore := collectHandles(reflect.ValueOf(kind))
	slices.Sort(before)

	var seen []ir.TypeHandle
	visitExprTypeHandles(kind, func(h ir.TypeHandle) { seen = append(seen, h) })
	slices.Sort(seen)
	if !slices.Equal(seen, before) {
		t.Errorf("visitExprTypeHandles(%#v) visited %v, want %v", kind, seen, before)
	}

	maxHandle := ir.TypeHandle(0)
	for _, h := range before {
		maxHandle = max(maxHandle, h)
	}
	referenced := make([]bool, maxHandle+1)
	markExprTypeRefs(kind, referenced)
	var marked []ir.TypeHandle
	for h, ok := range referenced {
		if ok {
			marked = append(marked, ir.TypeHandle(h))
		}
	}
	if !slices.Equal(marked, before) {
		t.Errorf("markExprTypeRefs(%#v) marked %v, want %v", kind, marked, before)
	}

	remap := make([]ir.TypeHandle, maxHandle+1)
	for i := range remap {
		remap[i] = ir.TypeHandle(i) + typeHandleOffset
	}
	var got ir.ExpressionKind
	if err := noPanic(func() { got = remapExprTypeHandles(kind, remap) }); err != nil {
		t.Fatalf("remapExprTypeHandles(%#v): %v", kind, err)
	}
	after := collectTypeHandles(reflect.ValueOf(got))
	slices.Sort(after)
	want := make([]ir.TypeHandle, len(before))
	for i, h := range before {
		want[i] = h + typeHandleOffset
	}
	if !slices.Equal(after, want) {
		t.Errorf("remapExprTypeHandles(%#v) gave type handles %v, want %v", kind, after, want)
	}
	if !slices.Equal(collectHandles(reflect.ValueOf(got)), exprBefore) {
		t.Errorf("remapExprTypeHandles(%#v) changed the expression handles", kind)
	}
	if now := collectTypeHandles(reflect.ValueOf(kind)); len(now) > 0 {
		sorted := slices.Clone(now)
		slices.Sort(sorted)
		if !slices.Equal(sorted, before) {
			t.Errorf("remapExprTypeHandles(%T) changed its input: %v, was %v", kind, sorted, before)
		}
	}
}

// collectTypeHandles returns every TypeHandle in v, in field order.
func collectTypeHandles(v reflect.Value) []ir.TypeHandle {
	var out []ir.TypeHandle
	var walk func(v reflect.Value)
	walk = func(v reflect.Value) {
		if v.Type() == typeHandleType {
			out = append(out, ir.TypeHandle(v.Uint()))
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
			panic(fmt.Sprintf("collectTypeHandles: map in %v", v.Type()))
		default:
		}
	}
	walk(v)
	return out
}
