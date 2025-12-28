// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl

import "testing"

func TestShaderModel_String(t *testing.T) {
	tests := []struct {
		name string
		sm   ShaderModel
		want string
	}{
		{"SM 5.0", ShaderModel5_0, "SM 5.0"},
		{"SM 5.1", ShaderModel5_1, "SM 5.1"},
		{"SM 6.0", ShaderModel6_0, "SM 6.0"},
		{"SM 6.1", ShaderModel6_1, "SM 6.1"},
		{"SM 6.2", ShaderModel6_2, "SM 6.2"},
		{"SM 6.3", ShaderModel6_3, "SM 6.3"},
		{"SM 6.4", ShaderModel6_4, "SM 6.4"},
		{"SM 6.5", ShaderModel6_5, "SM 6.5"},
		{"SM 6.6", ShaderModel6_6, "SM 6.6"},
		{"SM 6.7", ShaderModel6_7, "SM 6.7"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.sm.String()
			if got != tt.want {
				t.Errorf("ShaderModel.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShaderModel_ProfileSuffix(t *testing.T) {
	tests := []struct {
		name string
		sm   ShaderModel
		want string
	}{
		{"SM 5.0 suffix", ShaderModel5_0, "5_0"},
		{"SM 5.1 suffix", ShaderModel5_1, "5_1"},
		{"SM 6.0 suffix", ShaderModel6_0, "6_0"},
		{"SM 6.5 suffix", ShaderModel6_5, "6_5"},
		{"SM 6.7 suffix", ShaderModel6_7, "6_7"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.sm.ProfileSuffix()
			if got != tt.want {
				t.Errorf("ShaderModel.ProfileSuffix() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShaderModel_Version(t *testing.T) {
	tests := []struct {
		name      string
		sm        ShaderModel
		wantMajor uint8
		wantMinor uint8
	}{
		{"SM 5.0", ShaderModel5_0, 5, 0},
		{"SM 5.1", ShaderModel5_1, 5, 1},
		{"SM 6.0", ShaderModel6_0, 6, 0},
		{"SM 6.3", ShaderModel6_3, 6, 3},
		{"SM 6.7", ShaderModel6_7, 6, 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMajor := tt.sm.Major()
			gotMinor := tt.sm.Minor()
			if gotMajor != tt.wantMajor {
				t.Errorf("Major() = %d, want %d", gotMajor, tt.wantMajor)
			}
			if gotMinor != tt.wantMinor {
				t.Errorf("Minor() = %d, want %d", gotMinor, tt.wantMinor)
			}
		})
	}
}

func TestShaderModel_SupportsDXIL(t *testing.T) {
	tests := []struct {
		name string
		sm   ShaderModel
		want bool
	}{
		{"SM 5.0 no DXIL", ShaderModel5_0, false},
		{"SM 5.1 no DXIL", ShaderModel5_1, false},
		{"SM 6.0 DXIL", ShaderModel6_0, true},
		{"SM 6.5 DXIL", ShaderModel6_5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.sm.SupportsDXIL()
			if got != tt.want {
				t.Errorf("SupportsDXIL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShaderModel_SupportsWaveOps(t *testing.T) {
	tests := []struct {
		name string
		sm   ShaderModel
		want bool
	}{
		{"SM 5.1 no wave ops", ShaderModel5_1, false},
		{"SM 6.0 wave ops", ShaderModel6_0, true},
		{"SM 6.5 wave ops", ShaderModel6_5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.sm.SupportsWaveOps()
			if got != tt.want {
				t.Errorf("SupportsWaveOps() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShaderModel_SupportsMeshShaders(t *testing.T) {
	tests := []struct {
		name string
		sm   ShaderModel
		want bool
	}{
		{"SM 6.4 no mesh", ShaderModel6_4, false},
		{"SM 6.5 mesh", ShaderModel6_5, true},
		{"SM 6.6 mesh", ShaderModel6_6, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.sm.SupportsMeshShaders()
			if got != tt.want {
				t.Errorf("SupportsMeshShaders() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShaderModel_SupportsRayTracing(t *testing.T) {
	tests := []struct {
		name string
		sm   ShaderModel
		want bool
	}{
		{"SM 6.2 no ray tracing", ShaderModel6_2, false},
		{"SM 6.3 ray tracing", ShaderModel6_3, true},
		{"SM 6.5 ray tracing", ShaderModel6_5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.sm.SupportsRayTracing()
			if got != tt.want {
				t.Errorf("SupportsRayTracing() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShaderModel_Supports64BitAtomics(t *testing.T) {
	tests := []struct {
		name string
		sm   ShaderModel
		want bool
	}{
		{"SM 6.5 no 64-bit atomics", ShaderModel6_5, false},
		{"SM 6.6 64-bit atomics", ShaderModel6_6, true},
		{"SM 6.7 64-bit atomics", ShaderModel6_7, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.sm.Supports64BitAtomics()
			if got != tt.want {
				t.Errorf("Supports64BitAtomics() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShaderModel_SupportsFloat16(t *testing.T) {
	tests := []struct {
		name string
		sm   ShaderModel
		want bool
	}{
		{"SM 6.1 no float16", ShaderModel6_1, false},
		{"SM 6.2 float16", ShaderModel6_2, true},
		{"SM 6.5 float16", ShaderModel6_5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.sm.SupportsFloat16()
			if got != tt.want {
				t.Errorf("SupportsFloat16() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShaderModel_SupportsVariableRateShading(t *testing.T) {
	tests := []struct {
		name string
		sm   ShaderModel
		want bool
	}{
		{"SM 6.3 no VRS", ShaderModel6_3, false},
		{"SM 6.4 VRS", ShaderModel6_4, true},
		{"SM 6.6 VRS", ShaderModel6_6, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.sm.SupportsVariableRateShading()
			if got != tt.want {
				t.Errorf("SupportsVariableRateShading() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShaderModel_Unknown(t *testing.T) {
	// Test unknown shader model falls back to 5.1
	unknown := ShaderModel(255)
	if unknown.Major() != 5 {
		t.Errorf("Unknown shader model major = %d, want 5", unknown.Major())
	}
	if unknown.Minor() != 1 {
		t.Errorf("Unknown shader model minor = %d, want 1", unknown.Minor())
	}
}
