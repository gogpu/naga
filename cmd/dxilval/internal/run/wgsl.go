package run

import (
	"fmt"
	"io"

	"github.com/gogpu/naga/internal/dxcvalidator"
)

// runWGSL compiles a WGSL file through naga, then validates each emitted
// entry point. Reports per-entry-point results and exits non-zero if any
// of them returned INVALID.
func runWGSL(path, entry string, stdout, stderr io.Writer) int {
	blobs, names, stages, err := compileWGSL(path, entry)
	if err != nil {
		fmt.Fprintf(stderr, "dxilval: compile %s: %v\n", path, err)
		return ExitFail
	}
	if len(blobs) == 0 {
		fmt.Fprintf(stderr, "dxilval: %s has no compilable entry points\n", path)
		return ExitFail
	}

	type result struct {
		name   string
		stage  string
		res    dxcvalidator.Result
		valErr error
	}
	results := make([]result, len(blobs))

	runErr := dxcvalidator.Run(func(v *dxcvalidator.Validator) error {
		for i, blob := range blobs {
			r := result{name: names[i], stage: stageString(stages[i])}
			r.res, r.valErr = v.Validate(blob)
			results[i] = r
		}
		return nil
	})
	if runErr != nil {
		fmt.Fprintf(stderr, "dxilval: validator init: %v\n", runErr)
		return ExitFail
	}

	exitCode := ExitOK
	for _, r := range results {
		switch {
		case r.valErr != nil:
			fmt.Fprintf(stdout, "%s/%s: VALIDATE_ERROR %v\n", r.stage, r.name, r.valErr)
			exitCode = ExitFail
		case r.res.OK:
			fmt.Fprintf(stdout, "%s/%s: VALID (S_OK)\n", r.stage, r.name)
		default:
			fmt.Fprintf(stdout, "%s/%s: INVALID 0x%08x\n", r.stage, r.name, r.res.Status)
			if r.res.Error != "" {
				fmt.Fprintln(stdout, indent(r.res.Error, "  "))
			}
			if exitCode == ExitOK {
				exitCode = ExitValidation
			}
		}
	}
	return exitCode
}

// indent prefixes every line of s with prefix.
func indent(s, prefix string) string {
	out := make([]byte, 0, len(s)+len(prefix))
	out = append(out, prefix...)
	for i := 0; i < len(s); i++ {
		out = append(out, s[i])
		if s[i] == '\n' && i+1 < len(s) {
			out = append(out, prefix...)
		}
	}
	return string(out)
}
