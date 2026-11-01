package ir

import "errors"

// validateErrs is Validate returning its errors as a slice, nil when the
// module is valid.
func validateErrs(m *Module, caps Capabilities) ([]ValidationError, error) {
	_, err := Validate(m, ValidateOptions{Capabilities: caps})
	var ve ValidationErrors
	if errors.As(err, &ve) {
		return ve, nil
	}
	return nil, err
}
