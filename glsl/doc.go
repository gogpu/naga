// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

// Package glsl provides a GLSL (OpenGL Shading Language) backend for naga.
//
// This package generates GLSL source code from naga's IR representation.
// It supports multiple GLSL versions for different target platforms:
//
//   - GLSL ES 3.00: WebGL 2.0, Mobile OpenGL ES 3.0
//   - GLSL 3.30 Core: Desktop OpenGL 3.3+
//   - GLSL ES 3.10: Android 5.0+ with compute shaders
//   - GLSL 4.30 Core: Desktop OpenGL 4.3+ with compute shaders
//
// # Basic Usage
//
//	source, info, err := glsl.Compile(module, glsl.Options{
//	    LangVersion: glsl.Version330,
//	})
//
// # Texture/Sampler Handling
//
// WGSL separates textures and samplers, but GLSL combines them.
// The backend automatically generates combined sampler uniforms
// for texture-sampler pairs used together.
//
// # Reserved Words
//
// GLSL has over 500 reserved words (including future reserved).
// The backend automatically escapes conflicting identifier names
// by prefixing them with an underscore.
//
// # Concurrency
//
// Every exported function is safe for concurrent use. Compile never modifies
// its input module, so one module can be compiled for several targets at once.
package glsl
