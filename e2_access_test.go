package naga

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
	"github.com/gogpu/naga/wgsl"
)

// TestStorageAccessFromWGSL: the WGSL access words map to Rust naga's
// StorageAccess bits (front/wgsl/parse/conv.rs:51-61). atomic lowered to
// read_write and an unknown word to write; a storage texture without an
// access was accepted.
func TestStorageAccessFromWGSL(t *testing.T) {
	const load, store, atomic = ir.StorageAccessLoad, ir.StorageAccessStore, ir.StorageAccessAtomic
	for _, tc := range []struct {
		decl string
		want ir.StorageAccess
	}{
		{"var<storage> b: array<u32>;", load},
		{"var<storage, read> b: array<u32>;", load},
		{"var<storage, read_write> b: array<u32>;", load | store},
		{"var t: texture_storage_2d<r32uint, read>;", load},
		{"var t: texture_storage_2d<r32uint, write>;", store},
		{"var t: texture_storage_2d<r32uint, read_write>;", load | store},
		{"var t: texture_storage_2d<r32uint, atomic>;", atomic | load | store},
	} {
		src := "@group(0) @binding(0) " + tc.decl
		m, _, err := wgsl.Parse(src, wgsl.ParseOptions{})
		if err != nil {
			t.Errorf("%s: %v", tc.decl, err)
			continue
		}
		gv := m.GlobalVariables[0]
		got := gv.Access
		if img, ok := m.Types[gv.Type].Inner.(ir.ImageType); ok {
			got = img.StorageAccess
		}
		if got != tc.want {
			t.Errorf("%s: access %b, want %b", tc.decl, got, tc.want)
		}
	}

	for _, tc := range []struct{ decl, want string }{
		{"var<storage, bogus> b: array<u32>;", "unknown access: `bogus`"},
		{"var t: texture_storage_2d<r32uint, bogus>;", "unknown access: `bogus`"},
		{"var t: texture_storage_2d<r32uint>;", "expected a storage format and an access"},
	} {
		_, _, err := wgsl.Parse("@group(0) @binding(0) "+tc.decl, wgsl.ParseOptions{})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %v, want %q", tc.decl, err, tc.want)
		}
	}
}

// TestValidateStorageAccess: an empty access is "unset" and rejected on a
// storage variable or image (ADR-010 E2, as R1), a write-only storage
// variable is Rust's StorageAddressSpaceWriteOnlyNotSupported, and an access
// on another space is rejected: Rust keeps it inside AddressSpace::Storage.
func TestValidateStorageAccess(t *testing.T) {
	u32 := ir.Type{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}
	for _, tc := range []struct {
		name string
		gv   ir.GlobalVariable
		img  *ir.ImageType
		want string
	}{
		{"storage, empty", ir.GlobalVariable{Name: "b", Space: ir.SpaceStorage}, nil, "storage variable has no access"},
		{"storage, write-only", ir.GlobalVariable{Name: "b", Space: ir.SpaceStorage, Access: ir.StorageAccessStore}, nil,
			"Storage address space doesn't support write-only access"},
		{"uniform with access", ir.GlobalVariable{Name: "b", Space: ir.SpaceUniform, Access: ir.StorageAccessLoad}, nil,
			"only a storage variable has an access"},
		{"storage image, empty", ir.GlobalVariable{Name: "t", Space: ir.SpaceHandle},
			&ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassStorage, StorageFormat: ir.StorageFormatR32Uint},
			"storage image has no access"},
		{"storage, read", ir.GlobalVariable{Name: "b", Space: ir.SpaceStorage, Access: ir.StorageAccessLoad}, nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &ir.Module{Types: []ir.Type{u32}}
			if tc.img != nil {
				m.Types = append(m.Types, ir.Type{Inner: *tc.img})
				tc.gv.Type = 1
			}
			tc.gv.Binding = &ir.ResourceBinding{}
			m.GlobalVariables = []ir.GlobalVariable{tc.gv}
			_, err := valid.Validate(m, valid.Options{})
			switch {
			case tc.want == "" && err != nil:
				t.Errorf("error %v, want none", err)
			case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
				t.Errorf("error %v, want %q", err, tc.want)
			}
		})
	}
}
