package snapshot_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
)

// TestModuleCloneSharesNothing checks ir.Module.Clone on every corpus module:
// the copy equals the source and no pointer, slice or map in it refers to the
// source's memory, so changing the copy cannot change the source.
func TestModuleCloneSharesNothing(t *testing.T) {
	cloned := 0
	for _, sh := range loadCorpus(t) {
		module, err := tryCompileToIR(sh.source)
		if err != nil {
			continue // lowering errors are covered by the snapshot tests
		}
		cloned++
		clone := module.Clone()
		if !reflect.DeepEqual(clone, module) {
			t.Errorf("%s: clone differs from the source", sh.name)
			continue
		}
		if path, shared := sharedMemory(reflect.ValueOf(*module), reflect.ValueOf(*clone), "Module"); shared {
			t.Errorf("%s: clone shares memory with the source at %s", sh.name, path)
		}
	}
	if cloned < 150 {
		t.Fatalf("only %d corpus modules lowered; expected the whole corpus", cloned)
	}
}

// sharedMemory walks a and b (equal values of the same type) in parallel and
// reports the first place where they refer to the same memory: the same
// pointer, overlapping slice backing arrays, or the same map.
func sharedMemory(a, b reflect.Value, path string) (string, bool) {
	switch a.Kind() {
	case reflect.Pointer:
		if a.IsNil() || b.IsNil() {
			return "", false
		}
		if a.Pointer() == b.Pointer() {
			return path, true
		}
		return sharedMemory(a.Elem(), b.Elem(), "(*"+path+")")
	case reflect.Interface:
		if a.IsNil() || b.IsNil() {
			return "", false
		}
		return sharedMemory(a.Elem(), b.Elem(), path)
	case reflect.Slice:
		if a.Cap() > 0 && b.Cap() > 0 {
			size := a.Type().Elem().Size()
			aStart, bStart := a.Pointer(), b.Pointer()
			aEnd, bEnd := aStart+uintptr(a.Cap())*size, bStart+uintptr(b.Cap())*size
			if size > 0 && aStart < bEnd && bStart < aEnd {
				return path, true
			}
		}
		for i := range min(a.Len(), b.Len()) {
			if p, shared := sharedMemory(a.Index(i), b.Index(i), fmt.Sprintf("%s[%d]", path, i)); shared {
				return p, true
			}
		}
	case reflect.Map:
		if a.IsNil() || b.IsNil() {
			return "", false
		}
		if a.Pointer() == b.Pointer() {
			return path, true
		}
		for it := a.MapRange(); it.Next(); {
			if p, shared := sharedMemory(it.Value(), b.MapIndex(it.Key()), fmt.Sprintf("%s[%v]", path, it.Key())); shared {
				return p, true
			}
		}
	case reflect.Struct:
		for i := range a.NumField() {
			if p, shared := sharedMemory(a.Field(i), b.Field(i), path+"."+a.Type().Field(i).Name); shared {
				return p, true
			}
		}
	case reflect.Array:
		for i := range a.Len() {
			if p, shared := sharedMemory(a.Index(i), b.Index(i), fmt.Sprintf("%s[%d]", path, i)); shared {
				return p, true
			}
		}
	}
	return "", false
}

// TestBackendsDoNotMutateModule: every backend's Compile treats its module as
// read-only (BUG-IR-CLONE-001), so one lowered module can be compiled for
// several backends and pipelines. Each corpus module is compiled by every
// backend, with the shader's pipeline constants where it has them, and must
// still equal an independent lowering of the same source afterwards. Compile
// errors are ignored here: they are covered by the backend tests, and a
// failing Compile must not change its input either.
func TestBackendsDoNotMutateModule(t *testing.T) {
	for _, sh := range loadCorpus(t) {
		module, err := tryCompileToIR(sh.source)
		if err != nil {
			continue
		}
		want, err := tryCompileToIR(sh.source)
		if err != nil {
			t.Fatalf("%s: second lowering failed: %v", sh.name, err)
		}
		constants := readSPVPipelineConstants(sh.name)

		backends := []struct {
			name    string
			compile func()
		}{
			{"spirv", func() { _, _ = spirv.NewBackend(spirv.DefaultOptions()).Compile(module) }},
			{"msl", func() {
				opts := msl.DefaultOptions()
				opts.PipelineConstants = constants
				_, _, _ = msl.Compile(module, opts)
			}},
			{"hlsl", func() { _, _, _ = hlsl.Compile(module, hlsl.DefaultOptions()) }},
			{"glsl", func() {
				for _, ep := range module.EntryPoints {
					opts := glsl.Options{LangVersion: glsl.VersionES310, EntryPoint: ep.Name, PipelineConstants: constants}
					_, _, _ = glsl.Compile(module, opts)
				}
			}},
			{"dxil", func() { _, _ = dxil.Compile(module, dxil.DefaultOptions()) }},
			{"clone+ProcessOverrides", func() { _ = ir.ProcessOverrides(module.Clone(), constants) }},
		}
		for _, b := range backends {
			b.compile()
			if !reflect.DeepEqual(module, want) {
				t.Errorf("%s: %s changed its input module", sh.name, b.name)
				module, _ = tryCompileToIR(sh.source) // keep checking the other backends
			}
		}
	}
}
