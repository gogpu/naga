// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package ir

import (
	"strings"
	"testing"
)

// TestValidateStorageImageNeedsFormat: the zero StorageFormat is
// StorageFormatUndefined, never R8Unorm; a storage image without a format is
// a validation error (ADR-010 R1), and a sampled image needs none.
func TestValidateStorageImageNeedsFormat(t *testing.T) {
	if StorageFormatUndefined != 0 {
		t.Fatalf("StorageFormatUndefined = %d, want the zero value", StorageFormatUndefined)
	}
	for _, tc := range []struct {
		name    string
		image   ImageType
		wantErr bool
	}{
		{"storage, no format", ImageType{Dim: Dim2D, Class: ImageClassStorage, StorageAccess: StorageAccessWrite}, true},
		{"storage, r8unorm", ImageType{Dim: Dim2D, Class: ImageClassStorage, StorageFormat: StorageFormatR8Unorm, StorageAccess: StorageAccessWrite}, false},
		{"sampled", ImageType{Dim: Dim2D, Class: ImageClassSampled, SampledKind: ScalarFloat}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &Module{Types: []Type{{Inner: tc.image}}}
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
