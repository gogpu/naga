// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl

import "testing"

func TestBindTarget_Default(t *testing.T) {
	bt := DefaultBindTarget()

	if bt.Space != 0 {
		t.Errorf("Space = %d, want 0", bt.Space)
	}
	if bt.Register != 0 {
		t.Errorf("Register = %d, want 0", bt.Register)
	}
	if bt.BindingArraySize != nil {
		t.Error("BindingArraySize should be nil")
	}
}

func TestBindTarget_WithSpace(t *testing.T) {
	bt := DefaultBindTarget().WithSpace(5)

	if bt.Space != 5 {
		t.Errorf("Space = %d, want 5", bt.Space)
	}
	if bt.Register != 0 {
		t.Errorf("Register = %d, want 0 (unchanged)", bt.Register)
	}
}

func TestBindTarget_WithRegister(t *testing.T) {
	bt := DefaultBindTarget().WithRegister(10)

	if bt.Space != 0 {
		t.Errorf("Space = %d, want 0 (unchanged)", bt.Space)
	}
	if bt.Register != 10 {
		t.Errorf("Register = %d, want 10", bt.Register)
	}
}

func TestBindTarget_WithArraySize(t *testing.T) {
	bt := DefaultBindTarget().WithArraySize(16)

	if bt.BindingArraySize == nil {
		t.Fatal("BindingArraySize should not be nil")
	}
	if *bt.BindingArraySize != 16 {
		t.Errorf("BindingArraySize = %d, want 16", *bt.BindingArraySize)
	}
}

func TestBindTarget_Chaining(t *testing.T) {
	bt := DefaultBindTarget().
		WithSpace(2).
		WithRegister(5).
		WithArraySize(8)

	if bt.Space != 2 {
		t.Errorf("Space = %d, want 2", bt.Space)
	}
	if bt.Register != 5 {
		t.Errorf("Register = %d, want 5", bt.Register)
	}
	if bt.BindingArraySize == nil || *bt.BindingArraySize != 8 {
		t.Error("BindingArraySize should be 8")
	}
}

func TestRegisterType_String(t *testing.T) {
	tests := []struct {
		rt   RegisterType
		want string
	}{
		{RegisterTypeB, "b"},
		{RegisterTypeT, "t"},
		{RegisterTypeS, "s"},
		{RegisterTypeU, "u"},
		{RegisterType(255), "b"}, // Unknown defaults to b
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.rt.String()
			if got != tt.want {
				t.Errorf("RegisterType.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResourceBinding(t *testing.T) {
	rb := ResourceBinding{Group: 1, Binding: 5}

	if rb.Group != 1 {
		t.Errorf("Group = %d, want 1", rb.Group)
	}
	if rb.Binding != 5 {
		t.Errorf("Binding = %d, want 5", rb.Binding)
	}
}

func TestSamplerHeapBindTargets(t *testing.T) {
	targets := SamplerHeapBindTargets{
		StandardSamplers: BindTarget{
			Space:    0,
			Register: 0,
		},
		ComparisonSamplers: BindTarget{
			Space:    0,
			Register: 1,
		},
	}

	if targets.StandardSamplers.Register != 0 {
		t.Errorf("StandardSamplers.Register = %d, want 0", targets.StandardSamplers.Register)
	}
	if targets.ComparisonSamplers.Register != 1 {
		t.Errorf("ComparisonSamplers.Register = %d, want 1", targets.ComparisonSamplers.Register)
	}
}

func TestBindTarget_Immutability(t *testing.T) {
	// Ensure WithX methods don't modify the original
	original := DefaultBindTarget()
	_ = original.WithSpace(5)

	if original.Space != 0 {
		t.Error("WithSpace should not modify original")
	}

	_ = original.WithRegister(10)
	if original.Register != 0 {
		t.Error("WithRegister should not modify original")
	}

	_ = original.WithArraySize(8)
	if original.BindingArraySize != nil {
		t.Error("WithArraySize should not modify original")
	}
}
