package codegen

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestHLSL_LodQueryRefusals pins the four image classes the HLSL backend
// cannot write for a level-of-detail query. Each is a gap of HLSL or of dxc,
// never of this writer, and each must stay a refusal: the wrapper would
// otherwise call a member dxc has no declaration for. The GLSL frontend
// produces only the sampled and depth classes, so the multisampled and
// non-sampled arms are the writer's contract for any other IR producer too.
func TestHLSL_LodQueryRefusals(t *testing.T) {
	cases := []struct {
		name    string
		img     ir.ImageType
		wantErr string
	}{
		{
			name: "float sampled",
			img:  ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat},
		},
		{
			name:    "depth",
			img:     ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassDepth, SampledKind: ir.ScalarFloat},
			wantErr: "SamplerComparisonState",
		},
		{
			name: "multisampled",
			img: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, Multisampled: true,
				SampledKind: ir.ScalarFloat},
			wantErr: "no mip levels",
		},
		{
			name:    "integer sampled",
			img:     ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarSint},
			wantErr: "CalculateLevelOfDetail exists only for floating-point",
		},
		{
			name: "storage",
			img: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassStorage,
				StorageFormat: ir.StorageFormatR32Float},
			wantErr: "only a sampled image has one",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			img := tt.img
			err := hlslLodQueryError(&img)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("hlslLodQueryError(%+v) = %v, want nil", img, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("hlslLodQueryError(%+v) = nil, want a refusal containing %q", img, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}
