package subgroupmask

import "testing"

// fullRange is every bit an invocation of a subgroup of the given size owns:
// bits 0 to size-1, and nothing above.
func fullRange(size uint32) [4]uint32 {
	var mask [4]uint32
	for i := uint32(0); i < size; i++ {
		mask[i/32] |= 1 << (i % 32)
	}
	return mask
}

func andNot(a, b [4]uint32) [4]uint32 {
	var out [4]uint32
	for i := range out {
		out[i] = a[i] &^ b[i]
	}
	return out
}

func or(a, b [4]uint32) [4]uint32 {
	var out [4]uint32
	for i := range out {
		out[i] = a[i] | b[i]
	}
	return out
}

func equal(a, b [4]uint32) bool {
	return a == b
}

// TestMaskEquations checks every mask for every power-of-two size up to 128
// and every lane against the relations the spec's equations imply: Eq is the
// lane's single bit, Ge is the range minus the lanes below, Gt is Ge without
// Eq, Le is the range minus the lanes above, Lt is Le without Eq, and no bit
// above size-1 is ever set.
func TestMaskEquations(t *testing.T) {
	for size := uint32(1); size <= 128; size <<= 1 {
		full := fullRange(size)
		for lane := uint32(0); lane < size; lane++ {
			eq := Mask(Eq, lane, size)
			ge := Mask(Ge, lane, size)
			gt := Mask(Gt, lane, size)
			le := Mask(Le, lane, size)
			lt := Mask(Lt, lane, size)

			if want := [4]uint32{0, 0, 0, 0}; eq == want {
				t.Fatalf("size %d lane %d: Eq is zero", size, lane)
			}
			// Eq sets exactly the lane's bit.
			word, bit := lane/32, lane%32
			var wantEq [4]uint32
			wantEq[word] = 1 << bit
			if !equal(eq, wantEq) {
				t.Fatalf("size %d lane %d: Eq = %08x, want %08x", size, lane, eq, wantEq)
			}
			// No bit above size-1: every mask is a subset of the full range.
			for kind, mask := range map[Kind][4]uint32{Eq: eq, Ge: ge, Gt: gt, Le: le, Lt: lt} {
				if !equal(andNot(mask, full), [4]uint32{}) {
					t.Fatalf("size %d lane %d kind %d: bits above size-1 set (%08x)", size, lane, kind, mask)
				}
			}
			if !equal(or(ge, lt), full) {
				t.Fatalf("size %d lane %d: Ge | Lt != the full range", size, lane)
			}
			if !equal(or(gt, eq), ge) {
				t.Fatalf("size %d lane %d: Gt | Eq != Ge", size, lane)
			}
			if !equal(or(le, gt), full) {
				t.Fatalf("size %d lane %d: Le | Gt != the full range", size, lane)
			}
			if !equal(or(lt, eq), le) {
				t.Fatalf("size %d lane %d: Lt | Eq != Le", size, lane)
			}
			// Ge and Lt are complements inside the range, as are Le and Gt.
			if !equal(ge, andNot(full, lt)) || !equal(le, andNot(full, gt)) {
				t.Fatalf("size %d lane %d: the complement relations fail", size, lane)
			}
			// Edge lanes: lane 0 has the whole range in Ge/Le, the last lane
			// the whole range in Le and a single bit in Ge.
			if lane == 0 {
				if !equal(ge, full) || ge[0]&1 == 0 {
					t.Fatalf("size %d lane 0: Ge must be the full range", size)
				}
			}
			if lane == size-1 {
				if !equal(le, full) {
					t.Fatalf("size %d last lane: Le must be the full range", size)
				}
				if !equal(ge, eq) {
					t.Fatalf("size %d last lane: Ge must equal Eq", size)
				}
			}
		}
	}
}

// TestMaskWordBoundaries checks the 32-bit word boundaries at sizes 33 to 128:
// word 0 ends at bit 31, word 1 starts at 32, and the sizes that do not fill
// the last word leave its high bits zero.
func TestMaskWordBoundaries(t *testing.T) {
	// size 64, lane 32: Ge starts at bit 32 of word 1.
	ge := Mask(Ge, 32, 64)
	if ge[0] != 0 || ge[1] != 0xFFFFFFFF {
		t.Fatalf("size 64 lane 32: Ge = %08x, want word 0 zero and word 1 all ones", ge)
	}
	// size 128, lane 127: Le is everything, Ge is bit 127 = word 3 bit 31.
	le := Mask(Le, 127, 128)
	if want := fullRange(128); !equal(le, want) {
		t.Fatalf("size 128 lane 127: Le = %08x, want the full range", le)
	}
	ge = Mask(Ge, 127, 128)
	if ge[3] != 0x80000000 || ge[0]|ge[1]|ge[2] != 0 {
		t.Fatalf("size 128 lane 127: Ge = %08x, want only word 3 bit 31", ge)
	}
	// size 96 lane 95: Le fills words 0-2, word 3 stays zero.
	le = Mask(Le, 95, 96)
	if le[0] != 0xFFFFFFFF || le[1] != 0xFFFFFFFF || le[2] != 0xFFFFFFFF || le[3] != 0 {
		t.Fatalf("size 96 lane 95: Le = %08x, want words 0-2 full and word 3 zero", le)
	}
}

// TestMaskOutOfRange: a lane outside the subgroup or a kind outside the enum
// is the zero mask.
func TestMaskOutOfRange(t *testing.T) {
	if got := Mask(Eq, 64, 64); got != ([4]uint32{}) {
		t.Errorf("lane == size: %08x, want zero", got)
	}
	if got := Mask(Ge, 0, 0); got != ([4]uint32{}) {
		t.Errorf("size 0: %08x, want zero", got)
	}
	if got := Mask(Kind(99), 0, 64); got != ([4]uint32{}) {
		t.Errorf("kind 99: %08x, want zero", got)
	}
}
