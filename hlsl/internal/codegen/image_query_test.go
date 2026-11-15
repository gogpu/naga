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

// TestCompile_ImageQueryMixedSampledKinds pins the scalar kind in the wrapper
// key: two sampled textures of the same shape but different element types get
// one wrapper each, and their names coincide — HLSL resolves that as
// overloads, as Rust naga's output does (its ImageClass::Sampled carries the
// kind). Without the kind in the key one wrapper of the first type received
// the other texture and dxc rejected the call.
func TestCompile_ImageQueryMixedSampledKinds(t *testing.T) {
	const src = `
@group(0) @binding(0) var tf: texture_2d<f32>;
@group(0) @binding(1) var ti: texture_2d<i32>;

@fragment fn main() -> @location(0) vec4<f32> {
    let a = textureDimensions(tf);
    let b = textureDimensions(ti);
    return vec4<f32>(f32(a.x + b.x), 0.0, 0.0, 1.0);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	for _, want := range []string{
		"uint2 NagaDimensions2D(Texture2D<float4> tex)",
		"uint2 NagaDimensions2D(Texture2D<int4> tex)",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("missing wrapper overload %q in output:\n%s", want, code)
		}
	}
}

// TestCompile_ImageQueryMixedStorageFormats pins the storage format in the
// wrapper key: the parameter type is format-specific
// (RWTexture2D<float> vs RWTexture2D<unorm float4>), so two formats need two
// wrappers. Rust's key carries the format for the same reason.
func TestCompile_ImageQueryMixedStorageFormats(t *testing.T) {
	const src = `
@group(0) @binding(0) var rf: texture_storage_2d<r32float, write>;
@group(0) @binding(1) var ru: texture_storage_2d<rgba8unorm, write>;

@compute @workgroup_size(1)
fn cs_main() {
    let a = textureDimensions(rf);
    let b = textureDimensions(ru);
    textureStore(rf, vec2<i32>(0, 0), vec4<f32>(1.0));
    textureStore(ru, vec2<i32>(0, 0), vec4<f32>(f32(a.x + b.x)));
}
`
	code := compileWGSLToHLSL(t, src, nil)
	for _, want := range []string{
		"uint2 NagaRWDimensions2D(RWTexture2D<float> tex)",
		"uint2 NagaRWDimensions2D(RWTexture2D<unorm float4> tex)",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("missing wrapper overload %q in output:\n%s", want, code)
		}
	}
}

// TestCompile_ImageQueryStorageAccessSharesWrapper pins the deliberate
// difference from Rust: the storage access is not part of the key, so two
// storage images of the same shape and format but different access share one
// wrapper. Rust writes two same-named, same-signature definitions there,
// which dxc rejects as a redefinition (the images.frag deviation of
// TestGLSLTextReference).
func TestCompile_ImageQueryStorageAccessSharesWrapper(t *testing.T) {
	const src = `
@group(0) @binding(0) var rw: texture_storage_2d<rgba8unorm, read_write>;
@group(0) @binding(1) var wo: texture_storage_2d<rgba8unorm, write>;

@compute @workgroup_size(1)
fn cs_main() {
    let a = textureDimensions(rw);
    let b = textureDimensions(wo);
    textureStore(rw, vec2<i32>(0, 0), vec4<f32>(f32(a.x + b.x)));
}
`
	code := compileWGSLToHLSL(t, src, nil)
	if got := strings.Count(code, "uint2 NagaRWDimensions2D(RWTexture2D<unorm float4> tex)"); got != 1 {
		t.Errorf("NagaRWDimensions2D defined %d times, want exactly 1 shared wrapper:\n%s", got, code)
	}
}
