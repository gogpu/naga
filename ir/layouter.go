package ir

import "fmt"

// TypeLayout is the size and alignment of a type in memory, in bytes, as
// Rust naga's proc::TypeLayout.
type TypeLayout struct {
	// Size is the byte size of the type (Rust TypeInner::try_size): a
	// runtime-sized array counts one element, an array whose size is an
	// override not evaluated yet counts none, and an opaque type (image,
	// sampler, pointer, binding array, ...) has size 0, a pointer too (Rust
	// gives a pointer 4).
	Size uint32
	// Alignment is the byte alignment of the type. An opaque type has
	// alignment 1; a binding array has the alignment of its element type
	// (Rust gives it 1).
	Alignment uint32
}

// Layouter derives the layout of every type of a module by WGSL's default
// memory layout rules ("Alignment and Size"), as Rust naga's proc::Layouter.
// wgpu uses it for the minimum binding size of a buffer binding.
//
// The zero Layouter is ready to use: Update lays out the types of a module
// and Layout returns the layout of one of them.
type Layouter struct {
	layouts []TypeLayout
}

// Update lays out the types of m that l has no layout for yet: on the first
// call all of them, on a later call those m's type arena gained since, as
// Rust's Layouter::update does for a frontend that keeps adding types. A
// type laid out before is not computed again, so a module whose types
// changed in place needs a new Layouter.
//
// Unlike Rust's, it lets a type refer to a later one. It fails for a type
// that refers to a type outside the arena (Rust's InvalidArrayElementType
// and InvalidStructMemberType) or, through its members, to itself; l is then
// left as it was.
func (l *Layouter) Update(m *Module) error {
	start := len(l.layouts)
	if start >= len(m.Types) {
		return nil
	}
	c := layoutContext{
		m:       m,
		start:   start,
		layouts: make([]TypeLayout, len(m.Types)-start),
		state:   make([]layoutState, len(m.Types)-start),
		done:    l.layouts,
	}
	for h := start; h < len(m.Types); h++ {
		if _, err := c.layout(TypeHandle(h)); err != nil {
			return err
		}
	}
	l.layouts = append(l.layouts, c.layouts...)
	return nil
}

// Layout returns the layout of the type h. A handle Update has not laid out
// has size 0 and alignment 1, as an opaque type.
func (l *Layouter) Layout(h TypeHandle) TypeLayout {
	if int(h) >= len(l.layouts) {
		return TypeLayout{Size: 0, Alignment: 1}
	}
	return l.layouts[h]
}

// layoutState tracks a type of one Update: not laid out yet, being laid out
// (a reference back to it is a cycle) or laid out.
type layoutState uint8

const (
	layoutPending layoutState = iota
	layoutInProgress
	layoutDone
)

// layoutContext is one Update: the types from start on are laid out into
// layouts, those before it are in done.
type layoutContext struct {
	m       *Module
	start   int
	layouts []TypeLayout
	state   []layoutState
	done    []TypeLayout
}

func (c *layoutContext) layout(h TypeHandle) (TypeLayout, error) {
	if int(h) < c.start {
		return c.done[h], nil
	}
	i := int(h) - c.start
	switch c.state[i] {
	case layoutDone:
		return c.layouts[i], nil
	case layoutInProgress:
		return TypeLayout{}, fmt.Errorf("layout of type %d: the type contains itself", h)
	}
	c.state[i] = layoutInProgress
	inner := c.m.Types[h].Inner
	alignment, err := c.alignment(h, inner)
	if err != nil {
		return TypeLayout{}, err
	}
	size, ok := typeInnerSize(c.m, inner)
	if !ok {
		return TypeLayout{}, fmt.Errorf("layout of type %d: size exceeds the limit of %d bytes", h, MaxTypeSize)
	}
	c.layouts[i] = TypeLayout{Size: size, Alignment: alignment}
	c.state[i] = layoutDone
	return c.layouts[i], nil
}

// alignment is the alignment of the type h, whose inner type is inner.
func (c *layoutContext) alignment(h TypeHandle, inner TypeInner) (uint32, error) {
	switch t := inner.(type) {
	case ScalarType:
		return max(uint32(t.Width), 1), nil
	case AtomicType:
		return max(uint32(t.Scalar.Width), 1), nil
	case VectorType:
		return vectorAlignment(t.Size) * uint32(t.Scalar.Width), nil
	case MatrixType:
		return vectorAlignment(t.Rows) * uint32(t.Scalar.Width), nil
	case ArrayType:
		return c.elementAlignment(h, "array element", t.Base)
	case BindingArrayType:
		return c.elementAlignment(h, "binding array element", t.Base)
	case StructType:
		alignment := uint32(1)
		for i, m := range t.Members {
			a, err := c.elementAlignment(h, fmt.Sprintf("struct member[%d]", i), m.Type)
			if err != nil {
				return 0, err
			}
			alignment = max(alignment, a)
		}
		return alignment, nil
	default:
		return 1, nil
	}
}

// elementAlignment is the alignment of the type base that the type h refers
// to as what.
func (c *layoutContext) elementAlignment(h TypeHandle, what string, base TypeHandle) (uint32, error) {
	if int(base) >= len(c.m.Types) {
		return 0, fmt.Errorf("layout of type %d: %s type %d does not exist", h, what, base)
	}
	l, err := c.layout(base)
	if err != nil {
		return 0, err
	}
	return l.Alignment, nil
}

// MaxTypeSize is Rust naga's valid::MAX_TYPE_SIZE (valid/mod.rs:41): the
// largest size in bytes a type may have — i32::MAX, the bound Rust's
// TypeInner::try_size (proc/type_methods.rs:232-279) holds every type to. A
// size computed beyond it is an error, never a wrapped value.
const MaxTypeSize uint32 = 1<<31 - 1

// typeInnerSize is the byte size of inner following WGSL/WebGPU alignment
// rules, as Rust naga's TypeInner::try_size(gctx): 0 for opaque types
// (samplers, images, pointers) and for an array whose size is given by an
// override not yet evaluated; a runtime-sized array counts one element. ok is
// false when the size exceeds MaxTypeSize — an array's element count times
// its stride (computed in 64-bit arithmetic, so the product cannot wrap) or a
// struct's span.
func typeInnerSize(module *Module, inner TypeInner) (size uint32, ok bool) {
	switch t := inner.(type) {
	case ScalarType:
		return uint32(t.Width), true
	case AtomicType:
		return uint32(t.Scalar.Width), true
	case VectorType:
		return uint32(t.Size) * uint32(t.Scalar.Width), true
	case MatrixType:
		// Matrices are arrays of aligned columns. Column vector alignment
		// follows WGSL rules: vec2→align 2, vec3/vec4→align 4.
		colAlign := vectorAlignment(t.Rows)
		colStride := colAlign * uint32(t.Scalar.Width)
		return colStride * uint32(t.Columns), true
	case ArrayType:
		// Rust try_size: a runtime-sized array counts one element; a pending
		// size counts its override's value, or 0 while it is not evaluated.
		// A count above the limit is refused before the product, as Rust
		// does: a zero stride would otherwise hide it.
		l, err := t.Size.Resolve(module)
		var count uint64
		switch {
		case err != nil:
			count = 0
		case l.Dynamic:
			count = 1
		default:
			count = uint64(l.Known)
		}
		if count > uint64(MaxTypeSize) {
			return 0, false
		}
		return sizeWithinLimit(count, uint64(t.Stride))
	case StructType:
		// Rust's try_size returns a struct's span unchecked
		// (proc/type_methods.rs:272). We bound it: a struct span over
		// MAX_TYPE_SIZE is the same wrong layout the WGSL frontend refuses at
		// creation (card P2-WGSL-122, endorsed by the expert), and a struct
		// the frontend did not build — a GLSL frontend's std140 block or a
		// SPIR-V module's — must not smuggle an oversized type past the
		// bound the rest of the IR holds to.
		if t.Span > MaxTypeSize {
			return 0, false
		}
		return t.Span, true
	default:
		// Pointers, images, samplers, acceleration structures, ray queries
		// and binding arrays.
		return 0, true
	}
}

// sizeWithinLimit returns count × stride when it fits MaxTypeSize. The
// operands are both 32-bit, so the product cannot wrap 64 bits.
func sizeWithinLimit(count, stride uint64) (uint32, bool) {
	product := count * stride
	if product > uint64(MaxTypeSize) {
		return 0, false
	}
	return uint32(product), true
}

// vectorAlignment returns the alignment in components for a vector size.
// vec2→2, vec3→4, vec4→4 (matches Rust Alignment::from(VectorSize)).
func vectorAlignment(size VectorSize) uint32 {
	switch size {
	case Vec2:
		return 2
	case Vec3, Vec4:
		return 4
	default:
		return 4
	}
}
