// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package ir

import (
	"fmt"
	"reflect"
	"sync"
)

// Clone returns a deep copy of the module: no slice, map or pointer in the
// result is shared with m, so the copy can be changed freely without
// affecting m. It returns nil for a nil module.
//
// Backends that rewrite a module before emitting it (override processing,
// inlining, the DXIL passes) work on a clone, so a caller can compile one
// module many times, for different pipelines or backends.
//
// Matches Rust naga's #[derive(Clone)] on Module, which
// back::pipeline_constants::process_overrides clones before changing it.
// Like the derive, the copy is complete by construction: it follows the
// module's types, so a new IR field or statement kind is copied too.
func (m *Module) Clone() *Module {
	if m == nil {
		return nil
	}
	c := deepCopy(reflect.ValueOf(*m)).Interface().(Module)
	return &c
}

// copier returns a copy of a value of one type that shares no memory with
// it. A nil copier means the type holds no references (no pointer, slice,
// map or interface), so assignment already copies it.
type copier func(reflect.Value) reflect.Value

// copiers caches the copier of each type. A copier is built once per type,
// so copying a value does not inspect its type again.
var copiers sync.Map // reflect.Type -> copier

// deepCopy returns a copy of v that shares no memory with it.
func deepCopy(v reflect.Value) reflect.Value {
	if c := copierFor(v.Type()); c != nil {
		return c(v)
	}
	return v
}

var buildMu sync.Mutex

func copierFor(t reflect.Type) copier {
	if c, ok := copiers.Load(t); ok {
		return c.(copier)
	}
	buildMu.Lock()
	defer buildMu.Unlock()
	if c, ok := copiers.Load(t); ok {
		return c.(copier)
	}
	building := map[reflect.Type]*buildSlot{}
	c := buildCopier(t, building)
	for bt, slot := range building {
		copiers.Store(bt, slot.copy)
	}
	return c
}

// buildSlot holds a copier while copierFor builds it.
type buildSlot struct {
	copy copier
	done bool
}

// buildCopier builds the copier of t. IR data is plain data: values,
// pointers, slices, maps and interfaces over value types; handles are
// indices, so values form no cycles. A recursive type is still handled
// (through building) in case one is added. A function, channel or unsafe
// pointer in the IR is a programming error and panics, as does an
// unexported struct field, which reflect cannot set.
func buildCopier(t reflect.Type, building map[reflect.Type]*buildSlot) copier {
	if c, ok := copiers.Load(t); ok {
		return c.(copier)
	}
	if slot, ok := building[t]; ok {
		if slot.done {
			return slot.copy
		}
		// t is being built further up, so it is recursive and holds
		// references: its copier is non-nil and set by the time a value
		// is copied.
		return func(v reflect.Value) reflect.Value { return slot.copy(v) }
	}
	slot := &buildSlot{}
	building[t] = slot

	var c copier
	switch t.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.String:
		c = nil
	case reflect.Pointer:
		elem := buildCopier(t.Elem(), building)
		c = func(v reflect.Value) reflect.Value {
			if v.IsNil() {
				return v
			}
			p := reflect.New(t.Elem())
			if elem == nil {
				p.Elem().Set(v.Elem())
			} else {
				p.Elem().Set(elem(v.Elem()))
			}
			return p
		}
	case reflect.Interface:
		// The dynamic type is known only per value.
		c = func(v reflect.Value) reflect.Value {
			if v.IsNil() {
				return v
			}
			inner := v.Elem()
			ic := copierFor(inner.Type())
			if ic == nil {
				return v
			}
			out := reflect.New(t).Elem()
			out.Set(ic(inner))
			return out
		}
	case reflect.Slice:
		elem := buildCopier(t.Elem(), building)
		c = func(v reflect.Value) reflect.Value {
			if v.IsNil() {
				return v
			}
			out := reflect.MakeSlice(t, v.Len(), v.Len())
			if elem == nil {
				reflect.Copy(out, v)
				return out
			}
			for i := range v.Len() {
				out.Index(i).Set(elem(v.Index(i)))
			}
			return out
		}
	case reflect.Map:
		key := buildCopier(t.Key(), building)
		val := buildCopier(t.Elem(), building)
		c = func(v reflect.Value) reflect.Value {
			if v.IsNil() {
				return v
			}
			out := reflect.MakeMapWithSize(t, v.Len())
			for it := v.MapRange(); it.Next(); {
				k, e := it.Key(), it.Value()
				if key != nil {
					k = key(k)
				}
				if val != nil {
					e = val(e)
				}
				out.SetMapIndex(k, e)
			}
			return out
		}
	case reflect.Array:
		elem := buildCopier(t.Elem(), building)
		if elem != nil {
			c = func(v reflect.Value) reflect.Value {
				out := reflect.New(t).Elem()
				out.Set(v)
				for i := range v.Len() {
					out.Index(i).Set(elem(v.Index(i)))
				}
				return out
			}
		}
	case reflect.Struct:
		type fieldCopier struct {
			index int
			copy  copier
		}
		var fields []fieldCopier
		for i := range t.NumField() {
			f := t.Field(i)
			fc := buildCopier(f.Type, building)
			if fc == nil {
				continue
			}
			if !f.IsExported() {
				panic(fmt.Sprintf("ir: cannot clone unexported field %s.%s", t, f.Name))
			}
			fields = append(fields, fieldCopier{i, fc})
		}
		if len(fields) > 0 {
			c = func(v reflect.Value) reflect.Value {
				out := reflect.New(t).Elem()
				out.Set(v)
				for _, f := range fields {
					out.Field(f.index).Set(f.copy(v.Field(f.index)))
				}
				return out
			}
		}
	default:
		panic(fmt.Sprintf("ir: cannot clone a value of kind %s (%s)", t.Kind(), t))
	}
	slot.copy, slot.done = c, true
	return c
}
