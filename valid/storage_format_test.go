// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package valid

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestValidateStorageImageNeedsFormat: the zero StorageFormat is
// StorageFormatUndefined, never R8Unorm; a storage image without a format is
// a validation error (ADR-010 R1), and a sampled image needs none.
func TestValidateStorageImageNeedsFormat(t *testing.T) {
	if ir.StorageFormatUndefined != 0 {
		t.Fatalf("StorageFormatUndefined = %d, want the zero value", ir.StorageFormatUndefined)
	}
	for _, tc := range []struct {
		name    string
		image   ir.ImageType
		wantErr bool
	}{
		{"storage, no format", ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassStorage, StorageAccess: ir.StorageAccessStore}, true},
		{"storage, r8unorm", ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassStorage, StorageFormat: ir.StorageFormatR8Unorm, StorageAccess: ir.StorageAccessStore}, false},
		{"sampled", ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &ir.Module{Types: []ir.Type{{Inner: tc.image}}}
			errs, err := validateErrs(m, 0)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, e := range errs {
				found = found || strings.Contains(e.Message, "storage image has no format")
			}
			if found != tc.wantErr {
				t.Errorf("errors %v, want the no-format error: %v", errs, tc.wantErr)
			}
		})
	}
}
