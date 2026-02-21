// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// Version represents a GLSL version.
type Version struct {
	Major uint8
	Minor uint8
	ES    bool // true for GLSL ES (OpenGL ES / WebGL)
}

// Common GLSL versions.
var (
	// Desktop OpenGL versions
	Version330 = Version{Major: 3, Minor: 30, ES: false} // OpenGL 3.3 Core
	Version400 = Version{Major: 4, Minor: 0, ES: false}  // OpenGL 4.0
	Version410 = Version{Major: 4, Minor: 10, ES: false} // OpenGL 4.1
	Version420 = Version{Major: 4, Minor: 20, ES: false} // OpenGL 4.2
	Version430 = Version{Major: 4, Minor: 30, ES: false} // OpenGL 4.3 (compute shaders)
	Version450 = Version{Major: 4, Minor: 50, ES: false} // OpenGL 4.5
	Version460 = Version{Major: 4, Minor: 60, ES: false} // OpenGL 4.6

	// OpenGL ES / WebGL versions
	VersionES300 = Version{Major: 3, Minor: 0, ES: true}  // ES 3.0 / WebGL 2.0
	VersionES310 = Version{Major: 3, Minor: 10, ES: true} // ES 3.1 (compute shaders)
	VersionES320 = Version{Major: 3, Minor: 20, ES: true} // ES 3.2
)

// String returns the version as a GLSL version directive value.
func (v Version) String() string {
	if v.ES {
		return fmt.Sprintf("%d%02d es", v.Major, v.Minor)
	}
	return fmt.Sprintf("%d%02d core", v.Major, v.Minor)
}

// VersionNumber returns just the numeric version (e.g., "330", "300").
func (v Version) VersionNumber() string {
	return fmt.Sprintf("%d%02d", v.Major, v.Minor)
}

// versionLessThan returns true if the numeric version (Major*100+Minor) is
// less than the given number. For example, versionLessThan(410) returns true
// for GLSL 330 (3*100+30=330 < 410) and false for GLSL 410 (4*100+10=410).
func (v Version) versionLessThan(number int) bool {
	return int(v.Major)*100+int(v.Minor) < number
}

// SupportsCompute returns true if this version supports compute shaders.
func (v Version) SupportsCompute() bool {
	if v.ES {
		return v.Major > 3 || (v.Major == 3 && v.Minor >= 10)
	}
	return v.Major > 4 || (v.Major == 4 && v.Minor >= 30)
}

// SupportsStorageBuffers returns true if this version supports storage buffers.
func (v Version) SupportsStorageBuffers() bool {
	if v.ES {
		return v.Major > 3 || (v.Major == 3 && v.Minor >= 10)
	}
	return v.Major > 4 || (v.Major == 4 && v.Minor >= 30)
}

// WriterFlags control output formatting.
type WriterFlags uint32

const (
	// WriterFlagNone uses default settings.
	WriterFlagNone WriterFlags = 0

	// WriterFlagExplicitTypes forces explicit type annotations.
	WriterFlagExplicitTypes WriterFlags = 1 << iota

	// WriterFlagDebugInfo adds source comments for debugging.
	WriterFlagDebugInfo

	// WriterFlagMinify removes unnecessary whitespace.
	WriterFlagMinify
)

// Options configures GLSL code generation.
type Options struct {
	// LangVersion is the target GLSL version.
	// Defaults to Version330 if zero.
	LangVersion Version

	// EntryPoint specifies which entry point to compile.
	// If empty, the first entry point is compiled.
	EntryPoint string

	// SamplerBindingBase adds offset to sampler binding indices.
	SamplerBindingBase uint32

	// TextureBindingBase adds offset to texture binding indices.
	TextureBindingBase uint32

	// UniformBindingBase adds offset to uniform buffer binding indices.
	UniformBindingBase uint32

	// StorageBindingBase adds offset to storage buffer binding indices.
	StorageBindingBase uint32

	// WriterFlags control output formatting.
	WriterFlags WriterFlags

	// ForceHighPrecision forces highp precision for all float types (ES only).
	// If false, uses default precision qualifiers.
	ForceHighPrecision bool
}

// DefaultOptions returns sensible default options for GLSL generation.
func DefaultOptions() Options {
	return Options{
		LangVersion:        Version330,
		ForceHighPrecision: true,
	}
}

// TranslationInfo contains metadata about the translation.
type TranslationInfo struct {
	// EntryPointNames maps original entry point names to generated GLSL names.
	EntryPointNames map[string]string

	// UsedExtensions lists GLSL extensions required by the shader.
	UsedExtensions []string

	// RequiredVersion is the minimum GLSL version needed for this shader.
	// May be higher than the requested version if features require it.
	RequiredVersion Version

	// TextureSamplerPairs lists the combined texture-sampler pairs generated.
	// Each entry is "textureName_samplerName".
	TextureSamplerPairs []string
}

// Compile generates GLSL source code from an IR module.
// Returns the GLSL source as a string, translation info, or an error.
func Compile(module *ir.Module, options Options) (string, TranslationInfo, error) {
	// Apply defaults for zero values
	if options.LangVersion.Major == 0 {
		options.LangVersion = Version330
	}

	// Create writer
	w := newWriter(module, &options)

	// Generate GLSL code
	if err := w.writeModule(); err != nil {
		return "", TranslationInfo{}, fmt.Errorf("glsl: %w", err)
	}

	info := TranslationInfo{
		EntryPointNames:     w.entryPointNames,
		UsedExtensions:      w.extensions,
		RequiredVersion:     w.requiredVersion,
		TextureSamplerPairs: w.textureSamplerPairs,
	}

	return w.String(), info, nil
}
