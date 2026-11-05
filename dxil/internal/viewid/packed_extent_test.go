package viewid

import "testing"

// TestPackedExtent pins the signature extents the PSV0 dependency table is
// laid out by: rows are one past the last packed row (not the element count),
// scalars one past the last packed location including the start column, and
// system-managed elements take no room.
func TestPackedExtent(t *testing.T) {
	scalar := func(row, col uint32) SigElement {
		return SigElement{NumChannels: 1, VectorRow: row, StartCol: col}
	}
	cases := []struct {
		name                  string
		elems                 []SigElement
		wantScalars, wantRows uint32
	}{
		{"empty", nil, 0, 0},
		{"one vec4", []SigElement{{NumChannels: 4}}, 4, 1},
		{
			"position and eight packed scalars",
			[]SigElement{
				{NumChannels: 4},
				scalar(1, 0), scalar(1, 1), scalar(1, 2), scalar(1, 3),
				scalar(2, 0), scalar(2, 1), scalar(2, 2), scalar(2, 3),
			},
			12, 3,
		},
		{"scalar at column 3", []SigElement{{NumChannels: 2}, scalar(0, 3)}, 4, 1},
		{
			"system-managed element",
			[]SigElement{{NumChannels: 2, VectorRow: 1}, {NumChannels: 1, SystemManaged: true, VectorRow: 9}},
			6, 2,
		},
	}
	for _, c := range cases {
		scalars, rows := packedExtent(c.elems)
		if scalars != c.wantScalars || rows != c.wantRows {
			t.Errorf("%s: packedExtent = (%d, %d), want (%d, %d)", c.name, scalars, rows, c.wantScalars, c.wantRows)
		}
	}
}
