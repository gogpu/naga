package overrides

import (
	"errors"
	"fmt"
)

// The kinds of pipeline-constant error, Rust naga's PipelineConstantError
// variants. The root package exports them.
var (
	ErrNotFound              = errors.New("pipeline-overridable constant not found in the shader")
	ErrMissingValue          = errors.New("missing value for pipeline-overridable constant")
	ErrValueNotFinite        = errors.New("pipeline constant value is not finite")
	ErrValueOutOfRange       = errors.New("pipeline constant value does not fit the override's type")
	ErrNegativeWorkgroupSize = errors.New("workgroup_size override isn't strictly positive")
	ErrNegativeMeshOutputMax = errors.New("max vertices or max primitives is negative")
	ErrEntryPointNotFound    = errors.New("entry point not found")
)

// KeyError is an error about one pipeline constant or override.
type KeyError struct {
	Key string // the pipeline-constant key or the override's identifier
	Err error  // one of the Err* kinds above
}

func (e *KeyError) Error() string { return Message(e.Key, e.Err) }

func (e *KeyError) Unwrap() error { return e.Err }

// Message is Rust naga's message for a pipeline-constant error of kind err
// about key.
func Message(key string, err error) string {
	switch {
	case errors.Is(err, ErrNotFound):
		return fmt.Sprintf("pipeline-overridable constant '%s' not found in the shader", key)
	case errors.Is(err, ErrMissingValue):
		return fmt.Sprintf("Missing value for pipeline-overridable constant with identifier string: '%s'", key)
	case errors.Is(err, ErrValueNotFinite):
		// "Inifinites" is Rust's spelling; kept for identical messages.
		return "Source f64 value needs to be finite (NaNs and Inifinites are not allowed) for number destinations"
	case errors.Is(err, ErrValueOutOfRange):
		return "Source f64 value doesn't fit in destination"
	case err == nil:
		return "pipeline constant '" + key + "'"
	default:
		return err.Error()
	}
}
