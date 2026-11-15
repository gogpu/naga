package codegen

// The image query helpers of Rust naga's HLSL backend (help.rs
// write_wrapped_image_query_function): the number of GetDimensions outputs the
// wrapper fills and the component it returns depend on the query, the image
// dimension, whether the image is arrayed and the image class. Each row below
// is the helper Rust writes for that shape, so a change here without the same
// change in the Rust writer is a parity regression.

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

func TestWrappedImageQueryFunctionBody(t *testing.T) {
	tests := []struct {
		name    string
		img     ir.ImageType
		key     wrappedImageQueryKey
		want    string
		wantRet string
	}{
		{
			name: "storage 1D size",
			img:  ir.ImageType{Class: ir.ImageClassStorage, Dim: ir.Dim1D, StorageFormat: ir.StorageFormatRgba8Unorm},
			key:  wrappedImageQueryKey{dim: ir.Dim1D, class: ir.ImageClassStorage, query: imageQuerySize},
			// uint NagaRWDimensions1D: tex.GetDimensions(ret.x); return ret.x;
			want:    "    tex.GetDimensions(ret.x);\n",
			wantRet: "    return ret.x;\n",
		},
		{
			name: "sampled 2D size",
			img:  ir.ImageType{Class: ir.ImageClassSampled, Dim: ir.Dim2D},
			key:  wrappedImageQueryKey{dim: ir.Dim2D, class: ir.ImageClassSampled, query: imageQuerySize},
			// The zero mip level is asked for with the size.
			want:    "    tex.GetDimensions(0, ret.x, ret.y, ret.z);\n",
			wantRet: "    return ret.xy;\n",
		},
		{
			name: "storage 2D array size",
			img:  ir.ImageType{Class: ir.ImageClassStorage, Dim: ir.Dim2D, Arrayed: true, StorageFormat: ir.StorageFormatRgba8Unorm},
			key:  wrappedImageQueryKey{dim: ir.Dim2D, arrayed: true, class: ir.ImageClassStorage, query: imageQuerySize},
			// A storage image has neither a level nor a sample count to report,
			// so the element count is the last output.
			want:    "    tex.GetDimensions(ret.x, ret.y, ret.z);\n",
			wantRet: "    return ret.xy;\n",
		},
		{
			name:    "sampled 3D size",
			img:     ir.ImageType{Class: ir.ImageClassSampled, Dim: ir.Dim3D},
			key:     wrappedImageQueryKey{dim: ir.Dim3D, class: ir.ImageClassSampled, query: imageQuerySize},
			want:    "    tex.GetDimensions(0, ret.x, ret.y, ret.z, ret.w);\n",
			wantRet: "    return ret.xyz;\n",
		},
		{
			name:    "sampled 1D mip size",
			img:     ir.ImageType{Class: ir.ImageClassSampled, Dim: ir.Dim1D},
			key:     wrappedImageQueryKey{dim: ir.Dim1D, class: ir.ImageClassSampled, query: imageQuerySizeLevel},
			want:    "    tex.GetDimensions(mip_level, ret.x, ret.y);\n",
			wantRet: "    return ret.x;\n",
		},
		{
			name: "sampled 1D num levels",
			img:  ir.ImageType{Class: ir.ImageClassSampled, Dim: ir.Dim1D},
			key:  wrappedImageQueryKey{dim: ir.Dim1D, class: ir.ImageClassSampled, query: imageQueryNumLevels},
			// 1D reports the level count as the second output.
			want:    "    tex.GetDimensions(0, ret.x, ret.y);\n",
			wantRet: "    return ret.y;\n",
		},
		{
			name:    "sampled 2D num levels",
			img:     ir.ImageType{Class: ir.ImageClassSampled, Dim: ir.Dim2D},
			key:     wrappedImageQueryKey{dim: ir.Dim2D, class: ir.ImageClassSampled, query: imageQueryNumLevels},
			want:    "    tex.GetDimensions(0, ret.x, ret.y, ret.z);\n",
			wantRet: "    return ret.z;\n",
		},
		{
			name: "multisampled 2D num samples",
			img:  ir.ImageType{Class: ir.ImageClassSampled, Dim: ir.Dim2D, Multisampled: true},
			key:  wrappedImageQueryKey{dim: ir.Dim2D, class: ir.ImageClassSampled, multi: true, query: imageQueryNumSamples},
			// A multisampled image has no levels: no leading zero level argument.
			want:    "    tex.GetDimensions(ret.x, ret.y, ret.z);\n",
			wantRet: "    return ret.z;\n",
		},
		{
			name:    "sampled 3D num levels",
			img:     ir.ImageType{Class: ir.ImageClassSampled, Dim: ir.Dim3D},
			key:     wrappedImageQueryKey{dim: ir.Dim3D, class: ir.ImageClassSampled, query: imageQueryNumLevels},
			want:    "    tex.GetDimensions(0, ret.x, ret.y, ret.z, ret.w);\n",
			wantRet: "    return ret.w;\n",
		},
		{
			name: "sampled 2D array num layers",
			img:  ir.ImageType{Class: ir.ImageClassSampled, Dim: ir.Dim2D, Arrayed: true},
			key:  wrappedImageQueryKey{dim: ir.Dim2D, arrayed: true, class: ir.ImageClassSampled, query: imageQueryNumLayers},
			// The element count is the third output even though the wrapper
			// must fill the level count too; only the count returned differs
			// from the level query.
			want:    "    tex.GetDimensions(0, ret.x, ret.y, ret.z, ret.w);\n",
			wantRet: "    return ret.z;\n",
		},
		{
			name:    "sampled cube array num layers",
			img:     ir.ImageType{Class: ir.ImageClassSampled, Dim: ir.DimCube, Arrayed: true},
			key:     wrappedImageQueryKey{dim: ir.DimCube, arrayed: true, class: ir.ImageClassSampled, query: imageQueryNumLayers},
			want:    "    tex.GetDimensions(0, ret.x, ret.y, ret.z, ret.w);\n",
			wantRet: "    return ret.z;\n",
		},
		{
			name:    "storage 2D array num layers",
			img:     ir.ImageType{Class: ir.ImageClassStorage, Dim: ir.Dim2D, Arrayed: true, StorageFormat: ir.StorageFormatRgba8Unorm},
			key:     wrappedImageQueryKey{dim: ir.Dim2D, arrayed: true, class: ir.ImageClassStorage, query: imageQueryNumLayers},
			want:    "    tex.GetDimensions(ret.x, ret.y, ret.z);\n",
			wantRet: "    return ret.z;\n",
		},
		{
			name:    "depth 2D array num layers",
			img:     ir.ImageType{Class: ir.ImageClassDepth, Dim: ir.Dim2D, Arrayed: true},
			key:     wrappedImageQueryKey{dim: ir.Dim2D, arrayed: true, class: ir.ImageClassDepth, query: imageQueryNumLayers},
			want:    "    tex.GetDimensions(0, ret.x, ret.y, ret.z, ret.w);\n",
			wantRet: "    return ret.z;\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &Writer{module: &ir.Module{}}
			w.Out.Reset()
			img := tt.img
			w.writeWrappedImageQueryFunction(tt.key, &img)
			got := w.Out.String()
			if !strings.Contains(got, tt.want) {
				t.Errorf("GetDimensions call:\ngot  %q\nwant %q\nfull output:\n%s", got, tt.want, got)
			}
			if !strings.Contains(got, tt.wantRet) {
				t.Errorf("return statement:\ngot  %q\nwant %q\nfull output:\n%s", got, tt.wantRet, got)
			}
		})
	}
}
