// Package backend holds the types that every naga backend shares: the
// bounds-check policies, the workgroup zero-initialization mode, the
// entry-point selector, the frag_depth clamp, the DirectX shader model and
// the error categories a caller can branch on with errors.Is.
//
// One concept has one name, one Go type and one zero-value meaning in every
// backend that has it. The zero value of every type here is the behavior that
// is safe on every device. Mirrors Rust naga's back module and
// proc::BoundsCheckPolicies; no type here assumes a particular GPU API.
//
// Every exported function of naga is safe for concurrent use, and these types
// are plain values.
package backend

import (
	"errors"
	"strconv"

	"github.com/gogpu/naga/ir"
)

// BoundsCheckPolicy is how generated code handles an index that is out of
// bounds. Mirrors Rust naga's proc::BoundsCheckPolicy, with the constants in
// Rust's order. The zero value is BoundsCheckRestrict, while Rust's default
// is Unchecked: a caller that does not know whether the device checks
// accesses itself gets the safe policy.
type BoundsCheckPolicy uint8

// BoundsCheckPolicy values.
const (
	// BoundsCheckRestrict replaces an out-of-bounds index with an in-bounds
	// one, as WebGPU requires for array, vector and matrix accesses. This is
	// not necessarily a clamp: taking the unsigned minimum with the last
	// valid index maps a negative index to the last element.
	BoundsCheckRestrict BoundsCheckPolicy = iota

	// BoundsCheckReadZeroSkipWrite makes an out-of-bounds read return zero
	// and an out-of-bounds write or atomic do nothing. In a chain of
	// accesses such as a[i][j].b[k] every index is evaluated, and the whole
	// access is skipped if any of them is out of bounds.
	BoundsCheckReadZeroSkipWrite

	// BoundsCheckUnchecked adds no checks. An out-of-bounds access is
	// undefined behavior unless the device checks it, for example with
	// robust buffer access.
	BoundsCheckUnchecked
)

// String returns the policy's name as in Rust naga.
func (p BoundsCheckPolicy) String() string {
	switch p {
	case BoundsCheckRestrict:
		return "Restrict"
	case BoundsCheckReadZeroSkipWrite:
		return "ReadZeroSkipWrite"
	case BoundsCheckUnchecked:
		return "Unchecked"
	}
	return "BoundsCheckPolicy(" + strconv.Itoa(int(p)) + ")"
}

// BoundsCheckPolicies selects a BoundsCheckPolicy per kind of access.
// Mirrors Rust naga's proc::BoundsCheckPolicies. Each backend documents, for
// every field and value, whether it emits the check, relies on the target API
// to provide it, or rejects it.
type BoundsCheckPolicies struct {
	// Index applies to array, vector and matrix indices outside storage and
	// uniform buffers: function, private and workgroup memory, and values.
	Index BoundsCheckPolicy

	// Buffer applies to indices into globals in the storage and uniform
	// address spaces. A device with robust buffer access already confines
	// these accesses to the buffer, so it can use BoundsCheckUnchecked.
	Buffer BoundsCheckPolicy

	// ImageLoad applies to the coordinates, array index, level and sample of
	// image loads. There is no policy for image stores: every target already
	// discards an out-of-bounds store.
	ImageLoad BoundsCheckPolicy

	// BindingArray applies to indices into binding arrays.
	BindingArray BoundsCheckPolicy
}

// ZeroInitializeWorkgroupMemoryMode is how compute entry points zero their
// workgroup variables, as WebGPU requires. Mirrors Rust naga's
// ZeroInitializeWorkgroupMemoryMode; the zero value is Rust's default.
type ZeroInitializeWorkgroupMemoryMode uint8

// ZeroInitializeWorkgroupMemoryMode values.
const (
	// ZeroInitWorkgroupPolyfill has the generated code store zero to the
	// variables, followed by a workgroup barrier. It works on every device.
	ZeroInitWorkgroupPolyfill ZeroInitializeWorkgroupMemoryMode = iota

	// ZeroInitWorkgroupNative declares the variables with a null initializer
	// and leaves the zeroing to the driver, which needs Vulkan's
	// shaderZeroInitializeWorkgroupMemory feature. Only the SPIR-V backend
	// supports it.
	ZeroInitWorkgroupNative

	// ZeroInitWorkgroupNone leaves workgroup memory as it is.
	ZeroInitWorkgroupNone
)

// String returns the mode's name as in Rust naga.
func (m ZeroInitializeWorkgroupMemoryMode) String() string {
	switch m {
	case ZeroInitWorkgroupPolyfill:
		return "Polyfill"
	case ZeroInitWorkgroupNative:
		return "Native"
	case ZeroInitWorkgroupNone:
		return "None"
	}
	return "ZeroInitializeWorkgroupMemoryMode(" + strconv.Itoa(int(m)) + ")"
}

// EntryPointSelector selects one entry point of a module by stage and name.
// It is not ir.EntryPoint.
type EntryPointSelector struct {
	// Stage is the entry point's shader stage.
	Stage ir.ShaderStage
	// Name is the entry point's name in the module.
	Name string
}

// ClampFragDepth is where the range of the frag_depth clamp is: two f32
// values, min then max, in the immediates. WebGPU clamps frag_depth to the
// viewport's depth range, which the caller writes there.
type ClampFragDepth struct {
	// Offset is the byte offset of min in the immediates, a multiple of 4;
	// max follows.
	Offset uint32
}

// ShaderModel is a DirectX shader model version, such as 6.0. Each backend
// documents which versions it writes and what the zero value means (the
// HLSL backend's floor is 5.1, the DXIL backend's 6.0).
type ShaderModel struct {
	Major uint8
	Minor uint8
}

// String returns the version as "6.0".
func (sm ShaderModel) String() string {
	return strconv.Itoa(int(sm.Major)) + "." + strconv.Itoa(int(sm.Minor))
}

// ErrEntryPointNotFound reports an Options.EntryPoint the module has no entry
// point for, or a nil one where the output holds a single entry point and the
// module has several. Every backend's error for it wraps this value. Rust
// naga reports it as EntryPointNotFound.
var ErrEntryPointNotFound = errors.New("entry point not found")

// ErrUnsupported reports a module or an option the target cannot express: an
// option value a backend refuses (ADR-009 rule 7: refuse loudly, never weaken
// silently), a feature above the target's version or shader model, a built-in
// the target has no name for. Every backend's refusal wraps it, so a caller
// can tell "this device or target cannot" from a broken module with
// errors.Is; the message says what and why.
var ErrUnsupported = errors.New("unsupported")

// VersionSpace names the kind of version a VersionError is about: the
// option a backend takes for the version of its target.
type VersionSpace uint8

// VersionSpace values.
const (
	// VersionSPIRV is a SPIR-V version, spirv's Options.Version.
	VersionSPIRV VersionSpace = iota + 1
	// VersionShaderModel is a DirectX shader model, Options.ShaderModel of
	// hlsl and dxil.
	VersionShaderModel
	// VersionMSL is a Metal Shading Language version, msl's
	// Options.LangVersion.
	VersionMSL
)

// String returns "SPIR-V", "shader model" or "MSL".
func (s VersionSpace) String() string {
	switch s {
	case VersionSPIRV:
		return "SPIR-V"
	case VersionShaderModel:
		return "shader model"
	case VersionMSL:
		return "MSL"
	}
	return "VersionSpace(" + strconv.Itoa(int(s)) + ")"
}

// option returns the name of the Options field that holds a version of
// this space.
func (s VersionSpace) option() string {
	switch s {
	case VersionSPIRV:
		return "Version"
	case VersionShaderModel:
		return "ShaderModel"
	case VersionMSL:
		return "LangVersion"
	}
	return "version"
}

// Version is a major.minor version in a VersionSpace.
type Version struct {
	Major uint8
	Minor uint8
}

// String returns the version as "6.5".
func (v Version) String() string {
	return strconv.Itoa(int(v.Major)) + "." + strconv.Itoa(int(v.Minor))
}

// VersionError is a feature of the shader that needs a newer version of the
// target than the options ask for: a SPIR-V version (spirv), a shader model
// (hlsl, dxil) or an MSL version (msl). The backends never raise the
// version: a module newer than the device fails only when the driver loads
// it, as Rust naga refuses too (SpirvVersionTooLow, ShaderModelTooLow, the
// MSL writer's Unsupported* errors). Pass the device's highest supported
// version. Every backend returns it wrapped, so find it with errors.As.
type VersionError struct {
	// Space is the kind of version.
	Space VersionSpace
	// Feature is what needs the version, in WGSL terms where WGSL has them.
	Feature string
	// Required is the lowest version that has Feature.
	Required Version
	// Requested is the version the options ask for.
	Requested Version
}

// Error returns "<feature> needs <space> <required>, Options.<field> is
// <requested>", such as "a ray query needs MSL 2.4, Options.LangVersion is
// 2.1".
func (e *VersionError) Error() string {
	return e.Feature + " needs " + e.Space.String() + " " + e.Required.String() +
		", Options." + e.Space.option() + " is " + e.Requested.String()
}

// Unwrap returns ErrUnsupported: the target version cannot express the
// feature.
func (e *VersionError) Unwrap() error { return ErrUnsupported }

// ErrMissingBinding reports a resource the module uses that has no entry in a
// non-nil binding map. Every backend's error for it wraps this value. Rust
// naga reports it as MissingBinding.
var ErrMissingBinding = errors.New("resource binding missing from the binding map")
