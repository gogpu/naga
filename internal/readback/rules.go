package readback

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
)

// knownRequirements are the WebGPU feature names a case may require.
var knownRequirements = []string{"shader-f16"}

// cardRe matches a card ID: an uppercase ID ending in a number, optionally
// prefixed by a gogpu board ("gogpu:", "gogpu/wgpu:").
var cardRe = regexp.MustCompile(`^(gogpu(/[a-z0-9-]+)?:)?[A-Z][A-Z0-9]*(-[A-Z0-9]+)*-\d+$`)

// validate checks the rules of FormatVersion that the JSON decoder does not.
// It reports every broken rule, not only the first.
func (s *Spec) validate() error {
	if s.Format != FormatVersion {
		return fmt.Errorf("format %d: this reader supports format %d", s.Format, FormatVersion)
	}
	var errs []error
	if s.Desc == "" {
		errs = append(errs, errors.New("desc is empty"))
	}
	if s.Entry == "" {
		errs = append(errs, errors.New("entry is empty"))
	}
	if len(s.Workgroups) != 3 || slices.Contains(s.Workgroups, 0) {
		errs = append(errs, fmt.Errorf("workgroups %v: want three counts, each at least 1", s.Workgroups))
	}
	for name := range s.Constants {
		if name == "" {
			errs = append(errs, errors.New("constants: empty constant name"))
		}
	}
	for _, r := range s.Requires {
		if !slices.Contains(knownRequirements, r) {
			errs = append(errs, fmt.Errorf("requires %q: format %d knows %v", r, FormatVersion, knownRequirements))
		}
	}
	errs = append(errs, s.validateBindings()...)
	for i := range s.KnownFailures {
		if err := s.KnownFailures[i].validate(); err != nil {
			errs = append(errs, fmt.Errorf("knownFailures[%d]: %w", i, err))
		}
	}
	return errors.Join(errs...)
}

func (s *Spec) validateBindings() []error {
	if len(s.Bindings) == 0 {
		return []error{errors.New("no bindings")}
	}
	var errs []error
	seen := map[uint32]bool{}
	checked := false
	for i := range s.Bindings {
		b := &s.Bindings[i]
		if seen[b.Binding] {
			errs = append(errs, fmt.Errorf("binding %d twice", b.Binding))
		}
		seen[b.Binding] = true
		if len(b.Expect) > 0 {
			checked = true
		}
		if err := b.validate(); err != nil {
			errs = append(errs, fmt.Errorf("binding %d: %w", b.Binding, err))
		}
	}
	if !checked {
		errs = append(errs, errors.New("no binding has expect: the case checks nothing"))
	}
	return errs
}

func (b *Binding) validate() error {
	var errs []error
	switch b.Kind {
	case KindStorageRead, KindStorageRW, KindUniform:
	default:
		errs = append(errs, fmt.Errorf("kind %q: want %q, %q or %q", b.Kind, KindStorageRead, KindStorageRW, KindUniform))
	}
	switch b.Type {
	case "", "u32", "i32", "f32":
	default:
		errs = append(errs, fmt.Errorf("type %q: want u32, i32 or f32", b.Type))
	}
	switch {
	case b.Words != nil && b.Len != 0:
		errs = append(errs, errors.New("both words and len"))
	case b.Words == nil && b.Len <= 0:
		errs = append(errs, errors.New("needs words or len > 0"))
	case b.Words != nil && b.Fill != nil:
		errs = append(errs, errors.New("fill needs len, not words"))
	}
	if b.Expect != nil {
		errs = append(errs, b.validateExpect()...)
	}
	return errors.Join(errs...)
}

func (b *Binding) validateExpect() []error {
	if b.Kind != KindStorageRW {
		return []error{fmt.Errorf("expect on %s: only %s buffers are written", b.Kind, KindStorageRW)}
	}
	if len(b.Expect) == 0 {
		return []error{errors.New("expect is empty: omit it to leave the binding unchecked")}
	}
	var errs []error
	for i, alt := range b.Expect {
		if len(alt) != b.Size() {
			errs = append(errs, fmt.Errorf("expect[%d] has %d words, binding has %d", i, len(alt), b.Size()))
		}
		if b.Fill == nil && slices.ContainsFunc(alt, func(w ExpectWord) bool { return w.Fill }) {
			errs = append(errs, fmt.Errorf(`expect[%d] uses "fill" without a fill value`, i))
		}
	}
	return errs
}

func (k *KnownFailure) validate() error {
	var errs []error
	if !slices.Contains(Backends, k.Backend) {
		errs = append(errs, fmt.Errorf("backend %q: want one of %v", k.Backend, Backends))
	}
	if !cardRe.MatchString(k.Card) {
		errs = append(errs, fmt.Errorf("card %q: want a card ID such as BUG-DXIL-043 or gogpu/wgpu:P1-INFRA-001", k.Card))
	}
	if k.Reason == "" {
		errs = append(errs, errors.New("reason is empty"))
	}
	return errors.Join(errs...)
}
