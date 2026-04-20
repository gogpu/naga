package run

import (
	"fmt"
	"io"
	"os"

	"github.com/gogpu/naga/internal/dxcvalidator"
)

// runSingle validates a pre-built DXIL container.
func runSingle(path string, stdout, stderr io.Writer) int {
	blob, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "dxilval: read %s: %v\n", path, err)
		return ExitFail
	}

	var (
		res    dxcvalidator.Result
		valErr error
	)
	runErr := dxcvalidator.Run(func(v *dxcvalidator.Validator) error {
		res, valErr = v.Validate(blob)
		return nil
	})
	if runErr != nil {
		fmt.Fprintf(stderr, "dxilval: validator init: %v\n", runErr)
		return ExitFail
	}
	if valErr != nil {
		fmt.Fprintf(stderr, "dxilval: validate: %v\n", valErr)
		return ExitFail
	}

	if res.OK {
		fmt.Fprintf(stdout, "%s: VALID (S_OK)\n", path)
		return ExitOK
	}
	fmt.Fprintf(stdout, "%s: INVALID 0x%08x\n", path, res.Status)
	if res.Error != "" {
		fmt.Fprintln(stdout, res.Error)
	}
	return ExitValidation
}
