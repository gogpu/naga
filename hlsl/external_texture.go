// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// isExternalTexture checks if a type is an external texture image.
func (w *Writer) isExternalTexture(ty ir.TypeHandle) bool {
	if int(ty) >= len(w.module.Types) {
		return false
	}
	img, ok := w.module.Types[ty].Inner.(ir.ImageType)
	return ok && img.Class == ir.ImageClassExternal
}

// resolveExternalTextureBinding resolves the external texture bind target for a resource binding.
// Returns nil if the binding is not in the external_texture_binding_map.
func (w *Writer) resolveExternalTextureBinding(rb *ir.ResourceBinding) *ExternalTextureBindTarget {
	if w.options.ExternalTextureBindingMap == nil || rb == nil {
		return nil
	}
	key := ResourceBinding{Group: rb.Group, Binding: rb.Binding}
	if target, ok := w.options.ExternalTextureBindingMap[key]; ok {
		return &target
	}
	if w.options.FakeMissingBindings {
		// Generate fake bindings: planes at t0/t1/t2, params at b3
		target := ExternalTextureBindTarget{
			Planes: [3]BindTarget{
				{Register: 0},
				{Register: 1},
				{Register: 2},
			},
			Params: BindTarget{Register: 3},
		}
		return &target
	}
	return nil
}

// writeGlobalExternalTexture writes the decomposed global declarations for an external texture.
// Instead of a single Texture2D, it writes 3 plane textures and a params cbuffer.
// Matches Rust naga's write_global_external_texture.
func (w *Writer) writeGlobalExternalTexture(gvHandle ir.GlobalVariableHandle, gv *ir.GlobalVariable) {
	binding := w.resolveExternalTextureBinding(gv.Binding)
	if binding == nil {
		return
	}

	baseName := w.names[nameKey{kind: nameKeyGlobalVariable, handle1: uint32(gvHandle)}]
	if baseName == "" {
		baseName = gv.Name
	}

	// Generate plane names
	var names [4]string
	for i := 0; i < 3; i++ {
		names[i] = w.namer.call(fmt.Sprintf("%s_plane%d_", baseName, i))
	}
	names[3] = w.namer.call(fmt.Sprintf("%s_params", baseName))

	// Write plane declarations
	for i := 0; i < 3; i++ {
		bt := binding.Planes[i]
		regStr := formatRegister("t", bt.Register, bt.Space)
		w.writeLine("Texture2D<float4> %s: %s;", names[i], regStr)
	}

	// Write params cbuffer
	paramsTypeName := w.getExternalTextureParamsTypeName()
	bt := binding.Params
	regStr := fmt.Sprintf("register(b%d", bt.Register)
	if bt.Space != 0 {
		regStr += fmt.Sprintf(", space%d", bt.Space)
	}
	regStr += ")"
	w.writeLine("cbuffer %s: %s { %s %s; };", names[3], regStr, paramsTypeName, names[3])

	// Store the names for later expression writing
	w.externalTextureGlobals[gvHandle] = *binding
	w.externalTextureGlobalNames[gvHandle] = names
}

// getExternalTextureParamsTypeName returns the type name for NagaExternalTextureParams.
func (w *Writer) getExternalTextureParamsTypeName() string {
	if w.module.SpecialTypes.ExternalTextureParams != nil {
		th := *w.module.SpecialTypes.ExternalTextureParams
		if name, ok := w.typeNames[th]; ok {
			return name
		}
	}
	return "NagaExternalTextureParams"
}

// writeFunctionExternalTextureArgument writes expanded arguments for an external texture parameter.
// A single external texture arg becomes: Texture2D<float4> plane0, plane1, plane2, ParamsType params
func (w *Writer) writeFunctionExternalTextureArgument(funcHandle ir.FunctionHandle, argIndex int, argName string) {
	paramsTypeName := w.getExternalTextureParamsTypeName()

	var names [4]string
	for i := 0; i < 3; i++ {
		names[i] = w.namer.call(fmt.Sprintf("%s_plane%d_", argName, i))
	}
	names[3] = w.namer.call(fmt.Sprintf("%s_params", argName))

	fmt.Fprintf(&w.out, "Texture2D<float4> %s, Texture2D<float4> %s, Texture2D<float4> %s, %s %s",
		names[0], names[1], names[2], paramsTypeName, names[3])

	w.externalTextureFuncArgNames[externalTextureFuncArgKey{
		funcHandle: funcHandle,
		argIndex:   uint32(argIndex),
	}] = names
}

// writeExternalTextureGlobalExpression writes the expanded global variable reference
// for an external texture (comma-separated plane0, plane1, plane2, params).
func (w *Writer) writeExternalTextureGlobalExpression(gvHandle ir.GlobalVariableHandle) {
	names := w.externalTextureGlobalNames[gvHandle]
	fmt.Fprintf(&w.out, "%s, %s, %s, %s", names[0], names[1], names[2], names[3])
}

// writeExternalTextureFuncArgExpression writes the expanded function argument reference
// for an external texture parameter.
func (w *Writer) writeExternalTextureFuncArgExpression(funcHandle ir.FunctionHandle, argIndex uint32) {
	key := externalTextureFuncArgKey{funcHandle: funcHandle, argIndex: argIndex}
	names := w.externalTextureFuncArgNames[key]
	fmt.Fprintf(&w.out, "%s, %s, %s, %s", names[0], names[1], names[2], names[3])
}

// writeExternalTextureHelpers writes the nagaTextureSampleBaseClampToEdge,
// nagaTextureLoadExternal, and NagaExternalDimensions2D helper functions.
// These are massive helper functions matching Rust naga's output exactly.
func (w *Writer) writeExternalTextureSampleHelper() {
	if w.externalTextureSampleHelperWritten {
		return
	}
	w.externalTextureSampleHelperWritten = true

	paramsType := w.getExternalTextureParamsTypeName()

	w.out.WriteString("float4 nagaTextureSampleBaseClampToEdge(\n")
	w.out.WriteString("    Texture2D<float4> plane0,\n")
	w.out.WriteString("    Texture2D<float4> plane1,\n")
	w.out.WriteString("    Texture2D<float4> plane2,\n")
	fmt.Fprintf(&w.out, "    %s params,\n", paramsType)
	w.out.WriteString("    SamplerState samp,\n")
	w.out.WriteString("    float2 coords)\n")
	w.out.WriteString("{\n")
	w.out.WriteString("    float2 plane0_size;\n")
	w.out.WriteString("    plane0.GetDimensions(plane0_size.x, plane0_size.y);\n")
	w.out.WriteString("    float3x2 sample_transform = float3x2(\n")
	w.out.WriteString("        params.sample_transform_0,\n")
	w.out.WriteString("        params.sample_transform_1,\n")
	w.out.WriteString("        params.sample_transform_2\n")
	w.out.WriteString("    );\n")
	w.out.WriteString("    coords = mul(float3(coords, 1.0), sample_transform);\n")
	w.out.WriteString("    float2 bounds_min = mul(float3(0.0, 0.0, 1.0), sample_transform);\n")
	w.out.WriteString("    float2 bounds_max = mul(float3(1.0, 1.0, 1.0), sample_transform);\n")
	w.out.WriteString("    float4 bounds = float4(min(bounds_min, bounds_max), max(bounds_min, bounds_max));\n")
	w.out.WriteString("    float2 plane0_half_texel = float2(0.5, 0.5) / plane0_size;\n")
	w.out.WriteString("    float2 plane0_coords = clamp(coords, bounds.xy + plane0_half_texel, bounds.zw - plane0_half_texel);\n")
	w.out.WriteString("    if (params.num_planes == 1u) {\n")
	w.out.WriteString("        return plane0.SampleLevel(samp, plane0_coords, 0.0f);\n")
	w.out.WriteString("    } else {\n")
	w.out.WriteString("        float2 plane1_size;\n")
	w.out.WriteString("        plane1.GetDimensions(plane1_size.x, plane1_size.y);\n")
	w.out.WriteString("        float2 plane1_half_texel = float2(0.5, 0.5) / plane1_size;\n")
	w.out.WriteString("        float2 plane1_coords = clamp(coords, bounds.xy + plane1_half_texel, bounds.zw - plane1_half_texel);\n")
	w.out.WriteString("        float y = plane0.SampleLevel(samp, plane0_coords, 0.0f).x;\n")
	w.out.WriteString("        float2 uv;\n")
	w.out.WriteString("        if (params.num_planes == 2u) {\n")
	w.out.WriteString("            uv = plane1.SampleLevel(samp, plane1_coords, 0.0f).xy;\n")
	w.out.WriteString("        } else {\n")
	w.out.WriteString("            float2 plane2_size;\n")
	w.out.WriteString("            plane2.GetDimensions(plane2_size.x, plane2_size.y);\n")
	w.out.WriteString("            float2 plane2_half_texel = float2(0.5, 0.5) / plane2_size;\n")
	w.out.WriteString("            float2 plane2_coords = clamp(coords, bounds.xy + plane2_half_texel, bounds.zw - plane2_half_texel);\n")
	w.out.WriteString("            uv = float2(plane1.SampleLevel(samp, plane1_coords, 0.0f).x, plane2.SampleLevel(samp, plane2_coords, 0.0f).x);\n")
	w.out.WriteString("        }\n")
	w.writeYUVToRGBConversion("        ", "params")
	w.out.WriteString("    }\n")
	w.out.WriteString("}\n\n")
}

// writeExternalTextureLoadHelper writes the nagaTextureLoadExternal helper.
func (w *Writer) writeExternalTextureLoadHelper() {
	if w.externalTextureLoadHelperWritten {
		return
	}
	w.externalTextureLoadHelperWritten = true

	paramsType := w.getExternalTextureParamsTypeName()

	w.out.WriteString("float4 nagaTextureLoadExternal(\n")
	w.out.WriteString("    Texture2D<float4> plane0,\n")
	w.out.WriteString("    Texture2D<float4> plane1,\n")
	w.out.WriteString("    Texture2D<float4> plane2,\n")
	fmt.Fprintf(&w.out, "    %s params,\n", paramsType)
	w.out.WriteString("    uint2 coords)\n")
	w.out.WriteString("{\n")
	w.out.WriteString("    uint2 plane0_size;\n")
	w.out.WriteString("    plane0.GetDimensions(plane0_size.x, plane0_size.y);\n")
	w.out.WriteString("    uint2 cropped_size = any(params.size) ? params.size : plane0_size;\n")
	w.out.WriteString("    coords = min(coords, cropped_size - 1);\n")
	w.out.WriteString("    float3x2 load_transform = float3x2(\n")
	w.out.WriteString("        params.load_transform_0,\n")
	w.out.WriteString("        params.load_transform_1,\n")
	w.out.WriteString("        params.load_transform_2\n")
	w.out.WriteString("    );\n")
	w.out.WriteString("    uint2 plane0_coords = uint2(round(mul(float3(coords, 1.0), load_transform)));\n")
	w.out.WriteString("    if (params.num_planes == 1u) {\n")
	w.out.WriteString("        return plane0.Load(uint3(plane0_coords, 0u));\n")
	w.out.WriteString("    } else {\n")
	w.out.WriteString("        uint2 plane1_size;\n")
	w.out.WriteString("        plane1.GetDimensions(plane1_size.x, plane1_size.y);\n")
	w.out.WriteString("        uint2 plane1_coords = uint2(floor(float2(plane0_coords) * float2(plane1_size) / float2(plane0_size)));\n")
	w.out.WriteString("        float y = plane0.Load(uint3(plane0_coords, 0u)).x;\n")
	w.out.WriteString("        float2 uv;\n")
	w.out.WriteString("        if (params.num_planes == 2u) {\n")
	w.out.WriteString("            uv = plane1.Load(uint3(plane1_coords, 0u)).xy;\n")
	w.out.WriteString("        } else {\n")
	w.out.WriteString("            uint2 plane2_size;\n")
	w.out.WriteString("            plane2.GetDimensions(plane2_size.x, plane2_size.y);\n")
	w.out.WriteString("            uint2 plane2_coords = uint2(floor(float2(plane0_coords) * float2(plane2_size) / float2(plane0_size)));\n")
	w.out.WriteString("            uv = float2(plane1.Load(uint3(plane1_coords, 0u)).x, plane2.Load(uint3(plane2_coords, 0u)).x);\n")
	w.out.WriteString("        }\n")
	w.writeYUVToRGBConversion("        ", "params")
	w.out.WriteString("    }\n")
	w.out.WriteString("}\n\n")
}

// writeExternalTextureDimensionsHelper writes the NagaExternalDimensions2D helper.
func (w *Writer) writeExternalTextureDimensionsHelper() {
	if w.externalTextureDimensionsWritten {
		return
	}
	w.externalTextureDimensionsWritten = true

	paramsType := w.getExternalTextureParamsTypeName()

	fmt.Fprintf(&w.out, "uint2 NagaExternalDimensions2D(Texture2D<float4> plane0, Texture2D<float4> plane1, Texture2D<float4> plane2, %s params) {\n", paramsType)
	w.out.WriteString("    if (any(params.size)) {\n")
	w.out.WriteString("        return params.size;\n")
	w.out.WriteString("    } else {\n")
	w.out.WriteString("        uint2 ret;\n")
	w.out.WriteString("        plane0.GetDimensions(ret.x, ret.y);\n")
	w.out.WriteString("        return ret;\n")
	w.out.WriteString("    }\n")
	w.out.WriteString("}\n\n")
}

// writeYUVToRGBConversion writes the common YUV to RGB conversion code used by
// both the sample and load helpers.
func (w *Writer) writeYUVToRGBConversion(indent string, params string) {
	fmt.Fprintf(&w.out, "%sfloat3 srcGammaRgb = mul(float4(y, uv, 1.0), %s.yuv_conversion_matrix).rgb;\n", indent, params)
	fmt.Fprintf(&w.out, "%sfloat3 srcLinearRgb = srcGammaRgb < %s.src_tf.k * %s.src_tf.b ?\n", indent, params, params)
	fmt.Fprintf(&w.out, "%s    srcGammaRgb / %s.src_tf.k :\n", indent, params)
	fmt.Fprintf(&w.out, "%s    pow((srcGammaRgb + %s.src_tf.a - 1.0) / %s.src_tf.a, %s.src_tf.g);\n", indent, params, params, params)
	fmt.Fprintf(&w.out, "%sfloat3 dstLinearRgb = mul(srcLinearRgb, %s.gamut_conversion_matrix);\n", indent, params)
	fmt.Fprintf(&w.out, "%sfloat3 dstGammaRgb = dstLinearRgb < %s.dst_tf.b ?\n", indent, params)
	fmt.Fprintf(&w.out, "%s    %s.dst_tf.k * dstLinearRgb :\n", indent, params)
	fmt.Fprintf(&w.out, "%s    %s.dst_tf.a * pow(dstLinearRgb, 1.0 / %s.dst_tf.g) - (%s.dst_tf.a - 1);\n", indent, params, params, params)
	fmt.Fprintf(&w.out, "%sreturn float4(dstGammaRgb, 1.0);\n", indent)
}

// writeExternalTextureLoadHelperIfNeeded scans a function for ImageLoad expressions
// on external textures and writes the nagaTextureLoadExternal helper if needed.
func (w *Writer) writeExternalTextureLoadHelperIfNeeded(fn *ir.Function) {
	for _, expr := range fn.Expressions {
		il, ok := expr.Kind.(ir.ExprImageLoad)
		if !ok {
			continue
		}
		imgType := w.resolveImageTypeFromFn(fn, il.Image)
		if imgType != nil && imgType.Class == ir.ImageClassExternal {
			w.writeExternalTextureLoadHelper()
			return
		}
	}
}

// shouldDecomposeExternalTextureStruct checks if the NagaExternalTextureParams struct
// needs mat3x2 members decomposed into float2 pairs.
// Returns true if the struct has any mat3x2 members.
func (w *Writer) shouldDecomposeExternalTextureStruct(th ir.TypeHandle) bool {
	if w.module.SpecialTypes.ExternalTextureParams == nil {
		return false
	}
	return th == *w.module.SpecialTypes.ExternalTextureParams
}

// writeDecomposedMat3x2Member writes a mat3x2 struct member as 3 float2 members.
// E.g., "sample_transform" -> "float2 sample_transform_0; float2 sample_transform_1; float2 sample_transform_2;"
func (w *Writer) writeDecomposedMat3x2Member(memberName string) {
	fmt.Fprintf(&w.out, "float2 %s_0; float2 %s_1; float2 %s_2;", memberName, memberName, memberName)
}
