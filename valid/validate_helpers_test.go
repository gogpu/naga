package valid

import (
	"errors"

	"github.com/gogpu/naga/ir"
)

// validateErrs is Validate returning its errors as a slice, nil when the
// module is valid.
func validateErrs(m *ir.Module, caps Capabilities) ([]Error, error) {
	_, err := Validate(m, Options{Capabilities: caps})
	var ve Errors
	if errors.As(err, &ve) {
		return ve, nil
	}
	return nil, err
}
