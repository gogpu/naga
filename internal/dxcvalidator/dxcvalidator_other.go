//go:build !windows

package dxcvalidator

// validatorImpl is empty on non-Windows builds.
type validatorImpl struct{}

func (*validatorImpl) validate([]byte) (Result, error) {
	return Result{}, ErrUnsupported
}

func runImpl(func(*Validator) error) error {
	return ErrUnsupported
}
