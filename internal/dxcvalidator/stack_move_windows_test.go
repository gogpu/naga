//go:build windows

package dxcvalidator_test

import (
	"os"
	"runtime/debug"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/internal/dxcvalidator"
	"github.com/gogpu/naga/wgsl"
)

// stackMoveShader is a small compute shader; any valid container works, the
// test is about the call into dxil.dll, not about the shader.
const stackMoveShader = `@group(0) @binding(0) var<storage, read_write> buf: array<u32>;
@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    buf[id.x] = buf[id.x] * 2u + 1u;
}`

// growStack touches about depth*256 bytes of goroutine stack and returns, so
// the stack keeps its grown size while the caller runs shallow again.
//
//go:noinline
func growStack(depth int) byte {
	var pad [256]byte
	pad[depth%len(pad)] = byte(depth)
	if depth == 0 {
		return pad[0]
	}
	return growStack(depth-1) + pad[depth%len(pad)]
}

// TestValidateSurvivesGoroutineStackMove validates while the garbage
// collector runs nonstop. dxil.dll calls back into Go (the IDxcBlob methods)
// while Validate runs, and those callbacks run on the calling goroutine's
// stack. The goroutine is shallow on a large stack, so a collection that scans
// it inside a callback shrinks the stack, which moves it. Whatever Validate
// handed dxil.dll by address (the blob object, the IDxcOperationResult
// out-slot) must not live on that stack: otherwise dxil.dll writes the result
// into the old copy and Validate sees S_OK with a NULL result, or calls
// through a stale blob pointer.
func TestValidateSurvivesGoroutineStackMove(t *testing.T) {
	module, _, err := wgsl.Parse(stackMoveShader, wgsl.ParseOptions{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	blob, _, err := dxil.Compile(module, dxil.DefaultOptions())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	calls := 400
	if n, convErr := strconv.Atoi(os.Getenv("DXCVALIDATOR_STACK_MOVE_CALLS")); convErr == nil && n > 0 {
		calls = n
	}

	// Keep a collection always running: a tiny GC percent plus a goroutine
	// that does nothing but allocate.
	defer debug.SetGCPercent(debug.SetGCPercent(1))
	var stop atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var sink [][]byte
		for !stop.Load() {
			sink = append(sink, make([]byte, 4096))
			if len(sink) == 256 {
				sink = sink[:0]
			}
		}
	}()

	var failures []string
	runErr := dxcvalidator.Run(func(v *dxcvalidator.Validator) error {
		for i := 0; i < calls; i++ {
			growStack(1024)
			res, vErr := v.Validate(blob)
			switch {
			case vErr != nil:
				failures = append(failures, vErr.Error())
			case !res.OK:
				failures = append(failures, res.Error)
			}
		}
		return nil
	})
	stop.Store(true)
	wg.Wait()
	if runErr != nil {
		t.Skipf("Run failed (likely no dxil.dll): %v", runErr)
	}
	if len(failures) > 0 {
		t.Fatalf("%d of %d Validate calls failed; first: %s", len(failures), calls, failures[0])
	}
}
