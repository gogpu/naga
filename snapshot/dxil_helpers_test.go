package snapshot_test

import (
	"errors"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/ir"
)

// dxilAtNeededModel compiles as dxil.Compile does and, when the entry point
// needs a newer shader model than opts asks for, compiles again at that
// model: the corpora check every shader at the lowest model it can take, as
// a device that supports it would be asked for (dxil.Compile refuses instead
// of raising the model).
func dxilAtNeededModel(module *ir.Module, opts dxil.Options) ([]byte, error) {
	blob, _, err := dxil.Compile(module, opts)
	var vErr *backend.VersionError
	if errors.As(err, &vErr) && vErr.Space == backend.VersionShaderModel {
		opts.ShaderModel = backend.ShaderModel(vErr.Required)
		blob, _, err = dxil.Compile(module, opts)
	}
	return blob, err
}
