// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl

import (
	"github.com/gogpu/naga/ir"
)

// Features represents required GLSL features as bitflags.
// Matches Rust naga's back::glsl::Features.
type Features uint32

const (
	FeatureBufferStorage         Features = 1 << 0
	FeatureArrayOfArrays         Features = 1 << 1
	FeatureDoubleType            Features = 1 << 2
	FeatureFullImageFormats      Features = 1 << 3
	FeatureMultisampledTextures  Features = 1 << 4
	FeatureMultisampledTexArrays Features = 1 << 5
	FeatureCubeTexturesArray     Features = 1 << 6
	FeatureComputeShader         Features = 1 << 7
	FeatureImageLoadStore        Features = 1 << 8
	FeatureConservativeDepth     Features = 1 << 9
	FeatureNoPerspective         Features = 1 << 11
	FeatureSampleQualifier       Features = 1 << 12
	FeatureClipDistance          Features = 1 << 13
	FeatureCullDistance          Features = 1 << 14
	FeatureSampleVariables       Features = 1 << 15
	FeatureDynamicArraySize      Features = 1 << 16
	FeatureMultiView             Features = 1 << 17
	FeatureTextureSamples        Features = 1 << 18
	FeatureTextureLevels         Features = 1 << 19
	FeatureImageSize             Features = 1 << 20
	FeatureDualSourceBlending    Features = 1 << 21
	FeatureInstanceIndex         Features = 1 << 22
	FeatureTextureShadowLod      Features = 1 << 23
	FeatureSubgroupOperations    Features = 1 << 24
	FeatureTextureAtomics        Features = 1 << 25
	FeatureShaderBarycentrics    Features = 1 << 26
)

// featuresManager collects and writes required features.
type featuresManager struct {
	required Features
}

func (fm *featuresManager) request(f Features) {
	fm.required |= f
}

func (fm *featuresManager) contains(f Features) bool {
	return fm.required&f == f
}

// writeExtensions writes all required GL extension directives.
// Matches Rust naga's FeaturesManager::write.
func (fm *featuresManager) writeExtensions(w *Writer) {
	opts := w.options

	if fm.contains(FeatureComputeShader) && !opts.LangVersion.ES {
		w.writeLine("#extension GL_ARB_compute_shader : require")
	}

	if fm.contains(FeatureBufferStorage) && !opts.LangVersion.ES {
		w.writeLine("#extension GL_ARB_shader_storage_buffer_object : require")
	}

	if fm.contains(FeatureDoubleType) && !opts.LangVersion.ES && opts.LangVersion.versionLessThan(400) {
		w.writeLine("#extension GL_ARB_gpu_shader_fp64 : require")
	}

	if fm.contains(FeatureCubeTexturesArray) {
		if opts.LangVersion.ES {
			w.writeLine("#extension GL_EXT_texture_cube_map_array : require")
		} else if opts.LangVersion.versionLessThan(400) {
			w.writeLine("#extension GL_ARB_texture_cube_map_array : require")
		}
	}

	if fm.contains(FeatureMultisampledTexArrays) && opts.LangVersion.ES {
		w.writeLine("#extension GL_OES_texture_storage_multisample_2d_array : require")
	}

	if fm.contains(FeatureImageLoadStore) {
		if !opts.LangVersion.ES && opts.LangVersion.versionLessThan(420) {
			w.writeLine("#extension GL_ARB_shader_image_load_store : require")
		}
	}

	if fm.contains(FeatureConservativeDepth) {
		if opts.LangVersion.ES {
			w.writeLine("#extension GL_EXT_conservative_depth : require")
		} else if opts.LangVersion.versionLessThan(420) {
			w.writeLine("#extension GL_ARB_conservative_depth : require")
		}
	}

	if (fm.contains(FeatureClipDistance) || fm.contains(FeatureCullDistance)) && opts.LangVersion.ES {
		w.writeLine("#extension GL_EXT_clip_cull_distance : require")
	}

	if fm.contains(FeatureSampleVariables) && opts.LangVersion.ES {
		w.writeLine("#extension GL_OES_sample_variables : require")
	}

	if fm.contains(FeatureMultiView) {
		if opts.LangVersion.ES && opts.LangVersion.isWebGL() {
			w.writeLine("#extension GL_OVR_multiview2 : require")
		} else {
			w.writeLine("#extension GL_EXT_multiview : require")
		}
	}

	if fm.contains(FeatureTextureSamples) {
		w.writeLine("#extension GL_ARB_shader_texture_image_samples : require")
	}

	if fm.contains(FeatureTextureLevels) && !opts.LangVersion.ES && opts.LangVersion.versionLessThan(430) {
		w.writeLine("#extension GL_ARB_texture_query_levels : require")
	}

	if fm.contains(FeatureDualSourceBlending) && opts.LangVersion.ES {
		w.writeLine("#extension GL_EXT_blend_func_extended : require")
	}

	if fm.contains(FeatureTextureShadowLod) {
		w.writeLine("#extension GL_EXT_texture_shadow_lod : require")
	}

	if fm.contains(FeatureSubgroupOperations) {
		w.writeLine("#extension GL_KHR_shader_subgroup_basic : require")
		w.writeLine("#extension GL_KHR_shader_subgroup_vote : require")
		w.writeLine("#extension GL_KHR_shader_subgroup_arithmetic : require")
		w.writeLine("#extension GL_KHR_shader_subgroup_ballot : require")
		w.writeLine("#extension GL_KHR_shader_subgroup_shuffle : require")
		w.writeLine("#extension GL_KHR_shader_subgroup_shuffle_relative : require")
		w.writeLine("#extension GL_KHR_shader_subgroup_quad : require")
	}

	if fm.contains(FeatureTextureAtomics) {
		w.writeLine("#extension GL_OES_shader_image_atomic : require")
	}

	if fm.contains(FeatureShaderBarycentrics) {
		w.writeLine("#extension GL_EXT_fragment_shader_barycentric : require")
	}
}

// collectFeatures scans the module and entry point to determine required features.
// Matches Rust naga's Writer::collect_required_features.
func (w *Writer) collectFeatures() {
	ep := w.getSelectedEntryPoint()
	if ep == nil {
		return
	}

	// Compute shader
	if ep.Stage == ir.StageCompute {
		w.features.request(FeatureComputeShader)
	}

	// Writer flags can request features
	if w.options.WriterFlags&WriterFlagTextureShadowLod != 0 {
		w.features.request(FeatureTextureShadowLod)
	}

	// Early depth test
	if ep.EarlyDepthTest != nil {
		switch ep.EarlyDepthTest.Conservative {
		case ir.ConservativeDepthUnchanged:
			w.features.request(FeatureImageLoadStore)
		default:
			w.features.request(FeatureConservativeDepth)
		}
	}

	// Scan types for features
	for _, typ := range w.module.Types {
		switch inner := typ.Inner.(type) {
		case ir.ScalarType:
			if inner.Kind == ir.ScalarFloat && inner.Width == 8 {
				w.features.request(FeatureDoubleType)
			}
		case ir.ImageType:
			if inner.Arrayed && inner.Dim == ir.DimCube {
				w.features.request(FeatureCubeTexturesArray)
			}
			if inner.Multisampled {
				w.features.request(FeatureMultisampledTextures)
				// Note: FeatureTextureSamples only requested when ImageQuery::NumSamples is used
				if inner.Arrayed {
					w.features.request(FeatureMultisampledTexArrays)
				}
			}
		}
	}

	// Scan globals for features
	for _, global := range w.module.GlobalVariables {
		switch global.Space {
		case ir.SpaceWorkGroup:
			w.features.request(FeatureComputeShader)
		case ir.SpaceStorage:
			w.features.request(FeatureBufferStorage)
		}
	}

	// Scan entry point varyings for features
	w.scanVaryingFeatures(ep)

	// Scan expressions for features
	w.scanExpressionFeatures(ep)
}

// scanVaryingFeatures checks entry point IO for required features.
func (w *Writer) scanVaryingFeatures(ep *ir.EntryPoint) {
	fn := &ep.Function

	for _, arg := range fn.Arguments {
		w.checkVaryingBinding(arg.Binding, arg.Type)
	}
	if fn.Result != nil {
		w.checkVaryingBinding(fn.Result.Binding, fn.Result.Type)
	}
}

// checkVaryingBinding checks a single varying binding for required features.
func (w *Writer) checkVaryingBinding(binding *ir.Binding, typeHandle ir.TypeHandle) {
	if binding == nil {
		// Struct — check members
		if int(typeHandle) < len(w.module.Types) {
			if st, ok := w.module.Types[typeHandle].Inner.(ir.StructType); ok {
				for _, m := range st.Members {
					w.checkVaryingBinding(m.Binding, m.Type)
				}
			}
		}
		return
	}
	switch b := (*binding).(type) {
	case ir.LocationBinding:
		if b.Interpolation != nil {
			if b.Interpolation.Kind == ir.InterpolationLinear {
				w.features.request(FeatureNoPerspective)
			}
			if b.Interpolation.Sampling == ir.SamplingSample {
				w.features.request(FeatureSampleQualifier)
			}
		}
		if b.BlendSrc != nil {
			w.features.request(FeatureDualSourceBlending)
		}
	case ir.BuiltinBinding:
		if b.Builtin == ir.BuiltinSampleIndex {
			w.features.request(FeatureSampleVariables)
		}
		if b.Builtin == ir.BuiltinBarycentric {
			w.features.request(FeatureShaderBarycentrics)
		}
		if b.Builtin == ir.BuiltinViewIndex {
			w.features.request(FeatureMultiView)
		}
		if b.Builtin == ir.BuiltinInstanceIndex {
			w.features.request(FeatureInstanceIndex)
		}
	}
}

// scanExpressionFeatures scans all expressions for required features.
func (w *Writer) scanExpressionFeatures(ep *ir.EntryPoint) {
	scanFn := func(fn *ir.Function) {
		for _, expr := range fn.Expressions {
			switch k := expr.Kind.(type) {
			case ir.ExprSubgroupBallotResult:
				w.features.request(FeatureSubgroupOperations)
			case ir.ExprSubgroupOperationResult:
				w.features.request(FeatureSubgroupOperations)
			case ir.ExprImageQuery:
				switch k.Query.(type) {
				case ir.ImageQueryNumSamples:
					w.features.request(FeatureTextureSamples)
				case ir.ImageQueryNumLevels:
					w.features.request(FeatureTextureLevels)
				case ir.ImageQuerySize:
					// Check if image is a storage image — needs IMAGE_SIZE feature.
					// Resolve the image type through the expression chain.
					if int(k.Image) < len(fn.Expressions) {
						imgExpr := fn.Expressions[k.Image]
						if gv, ok := imgExpr.Kind.(ir.ExprGlobalVariable); ok {
							if int(gv.Variable) < len(w.module.GlobalVariables) {
								g := &w.module.GlobalVariables[gv.Variable]
								if int(g.Type) < len(w.module.Types) {
									if imgType, ok := w.module.Types[g.Type].Inner.(ir.ImageType); ok {
										if imgType.Class == ir.ImageClassStorage {
											w.features.request(FeatureImageSize)
										}
									}
								}
							}
						}
					}
				}
			case ir.ExprImageLoad:
				// Bounds-checked image loads with sample/level need extension.
				// Matches Rust naga: only when BoundsCheckPolicy != Unchecked.
				if w.options.BoundsCheckPolicies.ImageLoad != BoundsCheckUnchecked {
					if k.Sample != nil {
						w.features.request(FeatureTextureSamples)
					}
					if k.Level != nil {
						w.features.request(FeatureTextureLevels)
					}
				}
			case ir.ExprImageSample:
				_ = k // scanned for shadow LOD below
			default:
				_ = k
			}
		}
		// Check statements for image atomics
		w.scanStatementsForFeatures(fn.Body)
	}

	// Scan entry point function
	scanFn(&ep.Function)

	// Scan all reachable functions
	for handle, fn := range w.module.Functions {
		if w.reachable != nil && !w.reachable.hasFunction(ir.FunctionHandle(handle)) {
			continue
		}
		scanFn(&fn)
	}
}

// scanStatementsForFeatures checks statements for features.
func (w *Writer) scanStatementsForFeatures(block ir.Block) {
	for _, stmt := range block {
		switch stmt.Kind.(type) {
		case ir.StmtImageAtomic:
			w.features.request(FeatureTextureAtomics)
		}
	}
}

// isWebGL returns true if this is a WebGL version.
func (v Version) isWebGL() bool {
	return v.ES && v.Major == 3 && v.Minor == 0
	// This is a simplification — WebGL 2.0 is ES 3.0
}
