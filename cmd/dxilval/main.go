// Command dxilval validates DXIL containers using Microsoft's IDxcValidator.
//
// Three modes:
//
//	dxilval shader.dxil                # validate a single DXIL container
//	dxilval --wgsl shader.wgsl         # compile WGSL through naga, then validate
//	dxilval --corpus snapshot/testdata/in   # walk a directory and report
//
// IDxcValidator lives in dxil.dll (Windows 10 SDK). dxilval is a Windows-only
// tool; on other platforms it exits with a clear message.
package main

import (
	"os"

	"github.com/gogpu/naga/cmd/dxilval/internal/run"
)

func main() {
	os.Exit(run.Main(os.Args[1:], os.Stdout, os.Stderr))
}
