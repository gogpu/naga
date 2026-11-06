//go:build windows

package dxcvalidator

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/gogpu/naga/internal/dxcvalidator/bitcheck"
)

// --- COM GUIDs ---

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// CLSID_DxcValidator = 8CA3E215-F728-4CF3-8CDD-88AF917587A1
var clsidDxcValidator = guid{0x8CA3E215, 0xF728, 0x4CF3, [8]byte{0x8C, 0xDD, 0x88, 0xAF, 0x91, 0x75, 0x87, 0xA1}}

// IID_IDxcValidator = A6E82BD2-1FD7-4826-9811-2857E797F49A
var iidIDxcValidator = guid{0xA6E82BD2, 0x1FD7, 0x4826, [8]byte{0x98, 0x11, 0x28, 0x57, 0xE7, 0x97, 0xF4, 0x9A}}

// --- Vtable method indices (from dxcapi.h on DirectXShaderCompiler/main) ---

const (
	// IUnknown
	idxUnknownRelease = 2

	// IDxcValidator (IUnknown[0..2] + Validate[3])
	idxValidatorValidate = 3

	// IDxcOperationResult (IUnknown[0..2] + GetStatus[3] + GetResult[4] + GetErrorBuffer[5])
	idxResultGetStatus      = 3
	idxResultGetErrorBuffer = 5

	// IDxcBlob (IUnknown[0..2] + GetBufferPointer[3] + GetBufferSize[4])
	idxBlobGetBufferPointer = 3
	idxBlobGetBufferSize    = 4
)

// DxcValidatorFlags_InPlaceEdit = 1. Mesa uses this; without it dxil.dll's
// internal state machine can crash on blobs it cannot modify in place.
const validatorFlagInPlaceEdit = 1

// --- COM call helpers ---

// vtableMethod reads the function pointer at `index` from the COM vtable
// for an object whose first qword is its vtable pointer.
func vtableMethod(this unsafe.Pointer, index int) uintptr {
	vtable := *(*unsafe.Pointer)(this)
	return *(*uintptr)(unsafe.Add(vtable, uintptr(index)*unsafe.Sizeof(uintptr(0))))
}

func dxcCreateInstance2(proc, pMalloc uintptr, clsid, iid *guid, out *unsafe.Pointer) uintptr {
	ret, _, _ := syscall.SyscallN(proc,
		pMalloc,
		uintptr(unsafe.Pointer(clsid)),
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(out)))
	return ret
}

// validatorValidate takes the blob and the result out-slot as process-heap
// addresses: Validate calls back into Go through the blob, and the goroutine
// stack may move during those callbacks.
func validatorValidate(validator unsafe.Pointer, blob uintptr, flags uint32, resultOut uintptr) uintptr {
	method := vtableMethod(validator, idxValidatorValidate)
	ret, _, _ := syscall.SyscallN(method,
		uintptr(validator),
		blob,
		uintptr(flags),
		resultOut)
	return ret
}

func resultGetStatus(result unsafe.Pointer, statusOut *uint32) uintptr {
	method := vtableMethod(result, idxResultGetStatus)
	ret, _, _ := syscall.SyscallN(method, uintptr(result), uintptr(unsafe.Pointer(statusOut)))
	return ret
}

func resultGetErrorBuffer(result unsafe.Pointer, blobOut *unsafe.Pointer) uintptr {
	method := vtableMethod(result, idxResultGetErrorBuffer)
	ret, _, _ := syscall.SyscallN(method, uintptr(result), uintptr(unsafe.Pointer(blobOut)))
	return ret
}

func blobGetBufferPointer(blob unsafe.Pointer) uintptr {
	method := vtableMethod(blob, idxBlobGetBufferPointer)
	ret, _, _ := syscall.SyscallN(method, uintptr(blob))
	return ret
}

func blobGetBufferSize(blob unsafe.Pointer) uintptr {
	method := vtableMethod(blob, idxBlobGetBufferSize)
	ret, _, _ := syscall.SyscallN(method, uintptr(blob))
	return ret
}

func release(obj unsafe.Pointer) {
	if obj == nil {
		return
	}
	method := vtableMethod(obj, idxUnknownRelease)
	_, _, _ = syscall.SyscallN(method, uintptr(obj))
}

// readBlob returns the text of a blob dxil.dll returned. GetBufferPointer
// gives its buffer as a bare address, so RtlMoveMemory copies it out instead
// of Go converting the address to a pointer.
func readBlob(blob unsafe.Pointer) string {
	if blob == nil {
		return ""
	}
	ptr := blobGetBufferPointer(blob)
	size := blobGetBufferSize(blob)
	if ptr == 0 || size == 0 {
		return ""
	}
	data := make([]byte, size)
	_, _, _ = syscall.SyscallN(procMoveMemory, uintptr(unsafe.Pointer(&data[0])), ptr, size)
	return strings.TrimRight(string(data), "\x00\n\r ")
}

// --- IDxcBlob implemented in Go, living outside Go memory ---
//
// Mirrors Mesa's ShaderBlob C++ class: an object whose first qword is a
// vtable pointer; the vtable holds 5 function pointers (IUnknown[3] +
// IDxcBlob[2]) implemented via syscall.NewCallback. dxil.dll reads the vtable
// through standard COM dispatch and calls back into Go.
//
// Mesa can keep its ShaderBlob on the C stack; we cannot keep ours on the Go
// stack. dxil.dll calls the blob's methods while Validate runs, and each of
// those callbacks runs Go code on the calling goroutine's stack, where the
// runtime may grow or shrink, and so move, the stack (a collection that scans
// the goroutine inside a callback shrinks a mostly unused stack). Any address
// dxil.dll holds into that stack then points at the old copy: the
// IDxcOperationResult it stores at the end of Validate lands there, and the
// caller reads S_OK with a NULL result. So the blob object, the result
// out-slot and the shader bytes share one process-heap allocation, which
// neither moves nor is seen by the garbage collector.
//
// With DxcValidatorFlags_InPlaceEdit the result object keeps a reference to
// the blob (it is the result's DXC_OUT_OBJECT), so the allocation is
// reference counted as COM requires and freed by the last Release.

// heapBlob is the layout of the allocation's header. The shader bytes follow
// it directly.
type heapBlob struct {
	vtablePtr uintptr // MUST be first field
	data      uintptr // address of the shader bytes, right after the header
	size      uintptr
	refs      int32
	_         int32
	result    uintptr // IDxcOperationResult* out-slot of Validate
}

const (
	heapBlobHeaderSize   = unsafe.Sizeof(heapBlob{})
	heapBlobResultOffset = unsafe.Offsetof(heapBlob{}.result)
	pointerSize          = unsafe.Sizeof(uintptr(0))

	hrENoInterface = 0x80004002 // E_NOINTERFACE
	hrEPointer     = 0x80004003 // E_POINTER
)

// IID_IUnknown = 00000000-0000-0000-C000-000000000046
var iidIUnknown = guid{0x00000000, 0x0000, 0x0000, [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}

// IID_IDxcBlob = 8BA5FB08-5195-40E2-AC58-0D989C3A0102
var iidIDxcBlob = guid{0x8BA5FB08, 0x5195, 0x40E2, [8]byte{0xAC, 0x58, 0x0D, 0x98, 0x9C, 0x3A, 0x01, 0x02}}

// heapBlobVtable is package-global so the vtable address is stable for the
// process lifetime. Initialized lazily under heapBlobOnce.
var (
	heapBlobOnce   sync.Once
	heapBlobVtable [5]uintptr
)

func initHeapBlobVtable() {
	heapBlobOnce.Do(func() {
		// IUnknown::QueryInterface
		heapBlobVtable[0] = syscall.NewCallback(func(this unsafe.Pointer, riid *guid, ppvObject *uintptr) uintptr {
			if ppvObject == nil {
				return hrEPointer
			}
			if riid == nil || (*riid != iidIUnknown && *riid != iidIDxcBlob) {
				*ppvObject = 0
				return hrENoInterface
			}
			atomic.AddInt32(&(*heapBlob)(this).refs, 1)
			*ppvObject = uintptr(this)
			return 0
		})
		// IUnknown::AddRef
		heapBlobVtable[1] = syscall.NewCallback(func(this unsafe.Pointer) uintptr {
			return uintptr(atomic.AddInt32(&(*heapBlob)(this).refs, 1))
		})
		// IUnknown::Release
		heapBlobVtable[2] = syscall.NewCallback(func(this unsafe.Pointer) uintptr {
			refs := atomic.AddInt32(&(*heapBlob)(this).refs, -1)
			if refs == 0 {
				_, _, _ = syscall.SyscallN(procHeapFree, heapHandle, 0, uintptr(this))
			}
			return uintptr(refs)
		})
		// IDxcBlob::GetBufferPointer
		heapBlobVtable[3] = syscall.NewCallback(func(this unsafe.Pointer) uintptr {
			b := (*heapBlob)(this)
			if b.size == 0 {
				return 0
			}
			return b.data
		})
		// IDxcBlob::GetBufferSize
		heapBlobVtable[4] = syscall.NewCallback(func(this unsafe.Pointer) uintptr {
			return (*heapBlob)(this).size
		})
	})
}

// newHeapBlob copies shader into a new process-heap IDxcBlob holding one
// reference, owned by the caller (drop it with releaseHeapBlob). It returns
// the object's address.
func newHeapBlob(shader []byte) (uintptr, error) {
	if err := ensureHeapProcs(); err != nil {
		return 0, err
	}
	initHeapBlobVtable()
	total := heapBlobHeaderSize + uintptr(len(shader))
	// HEAP_ZERO_MEMORY = 0x8: the result out-slot starts NULL.
	obj, _, _ := syscall.SyscallN(procHeapAlloc, heapHandle, 0x8, total)
	if obj == 0 {
		return 0, fmt.Errorf("HeapAlloc(%d) failed", total)
	}
	header := heapBlob{
		vtablePtr: uintptr(unsafe.Pointer(&heapBlobVtable[0])),
		data:      obj + heapBlobHeaderSize,
		size:      uintptr(len(shader)),
		refs:      1,
	}
	_, _, _ = syscall.SyscallN(procMoveMemory, obj, uintptr(unsafe.Pointer(&header)), heapBlobHeaderSize)
	_, _, _ = syscall.SyscallN(procMoveMemory, header.data, uintptr(unsafe.Pointer(&shader[0])), header.size)
	return obj, nil
}

// heapBlobResult reads the IDxcOperationResult out-slot of a heap blob.
func heapBlobResult(obj uintptr) unsafe.Pointer {
	var result unsafe.Pointer
	_, _, _ = syscall.SyscallN(procMoveMemory, uintptr(unsafe.Pointer(&result)), obj+heapBlobResultOffset, pointerSize)
	return result
}

// releaseHeapBlob drops the caller's reference through the object's own
// IUnknown::Release, the same path dxil.dll uses.
func releaseHeapBlob(obj uintptr) {
	_, _, _ = syscall.SyscallN(heapBlobVtable[2], obj)
}

// --- Process heap allocator ---
//
// dxil.dll inspects buffer ownership via HeapSize on the Windows process
// heap. Go-allocated slices live in Go's heap (VirtualAlloc) and would
// trigger a NULL deref inside dxil.dll. Each blob is copied into a
// HeapAlloc'd buffer before validation (newHeapBlob).

var (
	heapProcsOnce  sync.Once
	errHeapProcs   error
	heapHandle     uintptr
	procHeapAlloc  uintptr
	procHeapFree   uintptr
	procMoveMemory uintptr
)

func ensureHeapProcs() error {
	heapProcsOnce.Do(func() {
		k, err := syscall.LoadDLL("kernel32.dll")
		if err != nil {
			errHeapProcs = fmt.Errorf("LoadDLL kernel32.dll: %w", err)
			return
		}
		p, err := k.FindProc("GetProcessHeap")
		if err != nil {
			errHeapProcs = err
			return
		}
		hh, _, _ := syscall.SyscallN(p.Addr())
		heapHandle = hh
		p, err = k.FindProc("HeapAlloc")
		if err != nil {
			errHeapProcs = err
			return
		}
		procHeapAlloc = p.Addr()
		p, err = k.FindProc("HeapFree")
		if err != nil {
			errHeapProcs = err
			return
		}
		procHeapFree = p.Addr()
		p, err = k.FindProc("RtlMoveMemory")
		if err != nil {
			errHeapProcs = err
			return
		}
		procMoveMemory = p.Addr()
	})
	return errHeapProcs
}

// --- Standard COM task allocator ---

func getStandardTaskMalloc() (uintptr, error) {
	ole32, err := syscall.LoadDLL("ole32.dll")
	if err != nil {
		return 0, fmt.Errorf("LoadDLL ole32.dll: %w", err)
	}
	coInit, err := ole32.FindProc("CoInitializeEx")
	if err != nil {
		return 0, fmt.Errorf("FindProc CoInitializeEx: %w", err)
	}
	// COINIT_APARTMENTTHREADED = 2. S_OK / S_FALSE / RPC_E_CHANGED_MODE all OK.
	_, _, _ = syscall.SyscallN(coInit.Addr(), 0, 2)
	coGetMalloc, err := ole32.FindProc("CoGetMalloc")
	if err != nil {
		return 0, fmt.Errorf("FindProc CoGetMalloc: %w", err)
	}
	var pMalloc uintptr
	hr, _, _ := syscall.SyscallN(coGetMalloc.Addr(), 1 /* MEMCTX_TASK */, uintptr(unsafe.Pointer(&pMalloc)))
	if hr != 0 {
		return 0, fmt.Errorf("CoGetMalloc HRESULT=0x%08x", hr)
	}
	return pMalloc, nil
}

// --- DLL loader with Windows 10 SDK fallback ---

func loadDLLWithFallback(name string) (*syscall.DLL, error) {
	if dll, err := syscall.LoadDLL(name); err == nil {
		return dll, nil
	}
	candidates := []string{
		`C:\Program Files (x86)\Windows Kits\10\bin\10.0.26100.0\x64\` + name,
		`C:\Program Files (x86)\Windows Kits\10\bin\10.0.22621.0\x64\` + name,
		`C:\Program Files (x86)\Windows Kits\10\bin\10.0.22000.0\x64\` + name,
	}
	for _, p := range candidates {
		if _, statErr := os.Stat(p); statErr == nil {
			return syscall.LoadDLL(p)
		}
	}
	return nil, fmt.Errorf("could not locate %s in PATH or Windows 10 SDK", name)
}

// --- validatorImpl ---

type validatorImpl struct {
	dxilDLL    *syscall.DLL
	taskMalloc uintptr
	validator  unsafe.Pointer // IDxcValidator*
}

func newValidatorImpl() (*validatorImpl, error) {
	dxilDLL, err := loadDLLWithFallback("dxil.dll")
	if err != nil {
		return nil, fmt.Errorf("LoadDLL dxil.dll: %w", err)
	}
	createInst2, err := dxilDLL.FindProc("DxcCreateInstance2")
	if err != nil {
		return nil, fmt.Errorf("FindProc DxcCreateInstance2: %w", err)
	}
	// dxil.dll's thread-local malloc is set up via DLL_THREAD_ATTACH which
	// only fires for threads created AFTER LoadDLL. Pre-existing Go runtime
	// threads have NULL thread malloc, causing a NULL+0x18 (IMalloc::Alloc
	// vtable slot) AV inside Validate. Passing an explicit IMalloc via
	// DxcCreateInstance2 sidesteps the entire issue.
	pMalloc, err := getStandardTaskMalloc()
	if err != nil {
		return nil, fmt.Errorf("get task malloc: %w", err)
	}
	v := &validatorImpl{
		dxilDLL:    dxilDLL,
		taskMalloc: pMalloc,
	}
	hr := dxcCreateInstance2(createInst2.Addr(), v.taskMalloc, &clsidDxcValidator, &iidIDxcValidator, &v.validator)
	if hr != 0 {
		return nil, fmt.Errorf("DxcCreateInstance2(IDxcValidator) HRESULT=0x%08x", hr)
	}
	return v, nil
}

func (v *validatorImpl) close() {
	release(v.validator)
	v.validator = nil
}

func (v *validatorImpl) validate(blob []byte) (Result, error) {
	if v == nil || v.validator == nil {
		return Result{}, fmt.Errorf("dxcvalidator: validator not initialized")
	}
	if len(blob) == 0 {
		return Result{}, fmt.Errorf("dxcvalidator: empty blob")
	}

	// Defensive layer 1: DXBC container structural pre-check. Runs before
	// we even copy the blob onto the process heap. Rejects truncated /
	// malformed containers and missing required parts so dxil.dll never
	// sees a blob it will choke on.
	// See: FEAT-VALIDATOR-PRECHECK-001, precheck.go.
	if err := PreCheckContainer(blob); err != nil {
		return Result{}, err
	}

	// Defensive layer 2: LLVM 3.7 bitstream metadata walker. Verifies
	// !dx.entryPoints[i][0] is a non-null function reference before the
	// blob reaches dxil.dll. Closes the BUG-DXIL-012 class AV at
	// dxil.dll+0xe9da (NULL+0x18) for any input — our own naga output,
	// DXC output, or third-party tool output.
	// See: FEAT-VALIDATOR-BITCHECK-001, internal/dxcvalidator/bitcheck/.
	if err := bitcheck.Check(blob); err != nil {
		return Result{}, err
	}

	// The blob object, its bytes and the result out-slot live on the process
	// heap, never on the goroutine stack (see heapBlob).
	dxcBlob, err := newHeapBlob(blob)
	if err != nil {
		return Result{}, err
	}
	hr := validatorValidate(v.validator, dxcBlob, validatorFlagInPlaceEdit, dxcBlob+heapBlobResultOffset)
	opResult := heapBlobResult(dxcBlob)
	// The result, when there is one, holds its own reference to the blob.
	releaseHeapBlob(dxcBlob)
	if hr != 0 {
		release(opResult)
		return Result{}, fmt.Errorf("IDxcValidator::Validate HRESULT=0x%08x", hr)
	}
	// validateWithOptDebugModule sets the result whenever it returns S_OK;
	// a NULL here is a broken contract, reported instead of dereferenced.
	if opResult == nil {
		return Result{}, fmt.Errorf("IDxcValidator::Validate returned S_OK with NULL IDxcOperationResult")
	}
	defer release(opResult)

	var status uint32
	if statusHR := resultGetStatus(opResult, &status); statusHR != 0 {
		return Result{}, fmt.Errorf("IDxcOperationResult::GetStatus HRESULT=0x%08x", statusHR)
	}

	res := Result{Status: status, OK: status == 0}
	if status != 0 {
		var errBlob unsafe.Pointer
		if errHR := resultGetErrorBuffer(opResult, &errBlob); errHR == 0 && errBlob != nil {
			res.Error = readBlob(errBlob)
			release(errBlob)
		}
	}
	return res, nil
}

// --- Fresh-thread runner ---
//
// dxil.dll initializes thread-local IMalloc state via DllMain
// (DLL_THREAD_ATTACH). Windows fires that only for threads created AFTER
// LoadLibrary. None of Go runtime's M's qualify. The only reliable fix is
// kernel32!CreateThread, which Windows treats as a fresh thread and fires
// DLL_THREAD_ATTACH for. runtime.LockOSThread does NOT help because it
// pins to an existing M.
//
// All COM work for one Run invocation must happen inside ONE
// syscall.NewCallback execution; nesting goroutines or spawning
// long-lived ones from a callback context is unsafe.

func runImpl(fn func(*Validator) error) error {
	if fn == nil {
		return fmt.Errorf("dxcvalidator: nil callback")
	}

	kernel32, err := syscall.LoadDLL("kernel32.dll")
	if err != nil {
		return fmt.Errorf("LoadDLL kernel32.dll: %w", err)
	}
	createThread, err := kernel32.FindProc("CreateThread")
	if err != nil {
		return fmt.Errorf("FindProc CreateThread: %w", err)
	}
	waitForSingleObject, err := kernel32.FindProc("WaitForSingleObject")
	if err != nil {
		return fmt.Errorf("FindProc WaitForSingleObject: %w", err)
	}
	closeHandle, err := kernel32.FindProc("CloseHandle")
	if err != nil {
		return fmt.Errorf("FindProc CloseHandle: %w", err)
	}

	// Outputs from the worker thread, populated inside the callback.
	var (
		impl    *validatorImpl
		initErr error
		fnErr   error
	)

	threadProc := syscall.NewCallback(func(arg uintptr) uintptr {
		impl, initErr = newValidatorImpl()
		if initErr != nil {
			return 0
		}
		v := &Validator{impl: impl}
		fnErr = fn(v)
		impl.close()
		return 0
	})

	var threadID uint32
	hThread, _, _ := syscall.SyscallN(createThread.Addr(),
		0, 0, threadProc, 0, 0, uintptr(unsafe.Pointer(&threadID)))
	if hThread == 0 {
		return fmt.Errorf("CreateThread returned 0")
	}
	defer func() { _, _, _ = syscall.SyscallN(closeHandle.Addr(), hThread) }()

	// INFINITE = 0xFFFFFFFF
	_, _, _ = syscall.SyscallN(waitForSingleObject.Addr(), hThread, 0xFFFFFFFF)

	if initErr != nil {
		return initErr
	}
	if impl == nil {
		return fmt.Errorf("dxcvalidator: validator initialisation produced nil")
	}
	return fnErr
}
