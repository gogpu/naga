// Package readback reads the GPU readback corpus under testdata/readback:
// small compute shaders with the values a GPU must write when it runs them.
//
// naga cannot run a GPU. The corpus lives here, next to the compiler it
// checks; gogpu/wgpu runs it on every backend and adapter and compares the
// buffers it reads back. naga's own tests use this package to keep every case
// well formed and compilable on every backend, so a case that rots fails naga
// CI before it reaches a GPU run. testdata/readback/README.md is the format
// specification wgpu implements; this package is its reference reader.
package readback

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// FormatVersion is the case format this package reads. A case declares its
// format; a reader rejects any other value, so a newer corpus fails loudly
// instead of being misread.
const FormatVersion = 1

// Spec is one case's JSON file.
type Spec struct {
	// Format is the case format version, FormatVersion.
	Format int `json:"format"`
	// Desc says what the case checks and why it exists.
	Desc string `json:"desc"`
	// Entry is the name of the compute entry point to run.
	Entry string `json:"entry"`
	// Workgroups is the dispatch size: exactly three counts, each at least 1.
	Workgroups []uint32 `json:"workgroups"`
	// Constants are pipeline-constant values by override name or @id.
	Constants map[string]float64 `json:"constants,omitempty"`
	// Requires lists the WebGPU features the case needs; an adapter without
	// one skips the case.
	Requires []string `json:"requires,omitempty"`
	// Bindings are the buffers of @group(0), one per @binding.
	Bindings []Binding `json:"bindings"`
	// KnownFailures are the backends and adapters on which the case is
	// expected to fail, each tracked by a card.
	KnownFailures []KnownFailure `json:"knownFailures,omitempty"`
}

// Kind is the kind of buffer a binding is.
type Kind string

// Buffer kinds of format 1.
const (
	KindStorageRead Kind = "storage-read" // var<storage, read>
	KindStorageRW   Kind = "storage-rw"   // var<storage, read_write>
	KindUniform     Kind = "uniform"      // var<uniform>
)

// Binding is one buffer at @group(0) @binding(Binding). Its initial contents
// are either Words, or Len copies of Fill.
type Binding struct {
	Binding uint32 `json:"binding"`
	Kind    Kind   `json:"kind"`
	// Words are the initial contents.
	Words []Word `json:"words,omitempty"`
	// Len is the size in words when the buffer starts as Len copies of Fill.
	Len int `json:"len,omitempty"`
	// Fill is the initial value of every word when Len is set (default 0);
	// "fill" in Expect stands for it.
	Fill *Word `json:"fill,omitempty"`
	// Expect lists the acceptable contents after the dispatch (any of them);
	// each has exactly Size words. Only storage-rw bindings have it.
	Expect [][]ExpectWord `json:"expect,omitempty"`
	// Type says how to print the words in a failure: u32 (default), i32 or
	// f32. It does not change the comparison, which is bit for bit.
	Type string `json:"type,omitempty"`
}

// Size returns the buffer size in 32-bit words.
func (b *Binding) Size() int {
	if b.Words != nil {
		return len(b.Words)
	}
	return b.Len
}

// fill returns the value of every initial word when Len is set.
func (b *Binding) fill() uint32 {
	if b.Fill == nil {
		return 0
	}
	return uint32(*b.Fill)
}

// Initial returns the buffer contents before the dispatch.
func (b *Binding) Initial() []uint32 {
	if b.Words != nil {
		out := make([]uint32, len(b.Words))
		for i, w := range b.Words {
			out[i] = uint32(w)
		}
		return out
	}
	out := make([]uint32, b.Len)
	for i := range out {
		out[i] = b.fill()
	}
	return out
}

// Expected returns the acceptable contents after the dispatch, with "fill"
// replaced by the fill value.
func (b *Binding) Expected() [][]uint32 {
	out := make([][]uint32, len(b.Expect))
	for i, alt := range b.Expect {
		out[i] = make([]uint32, len(alt))
		for j, w := range alt {
			if w.Fill {
				out[i][j] = b.fill()
			} else {
				out[i][j] = w.Value
			}
		}
	}
	return out
}

// Backend names a wgpu backend and, for DX12, the shader compiler path. A
// naga backend serves each: vulkan and software run SPIR-V, dx12-fxc runs
// HLSL through FXC, dx12-dxil runs naga's DXIL, gles runs GLSL and metal MSL.
type Backend string

// Backends of format 1.
const (
	BackendVulkan   Backend = "vulkan"
	BackendDX12FXC  Backend = "dx12-fxc"
	BackendDX12DXIL Backend = "dx12-dxil"
	BackendGLES     Backend = "gles"
	BackendMetal    Backend = "metal"
	BackendSoftware Backend = "software"
)

// Backends lists every backend of format 1.
var Backends = []Backend{BackendVulkan, BackendDX12FXC, BackendDX12DXIL, BackendGLES, BackendMetal, BackendSoftware}

// KnownFailure is an expected failure, as Rust wgpu's FailureCase: it
// matches a run on Backend whose adapter name and driver contain Adapter and
// Driver (case-insensitive; empty matches any). A matched case that passes is
// an error, so a fixed bug cannot hide behind a stale entry.
type KnownFailure struct {
	Backend Backend `json:"backend"`
	Adapter string  `json:"adapter,omitempty"`
	Driver  string  `json:"driver,omitempty"`
	// Card is the kanban card that tracks the bug: a naga card ID, or
	// "gogpu/<repo>:<ID>" for a card of another gogpu repository.
	Card   string `json:"card"`
	Reason string `json:"reason"`
}

// AllAdapters reports whether the entry matches every adapter of its backend.
func (k *KnownFailure) AllAdapters() bool { return k.Adapter == "" && k.Driver == "" }

// Word is a 32-bit buffer word: a JSON integer in [0, 2^32) or a string
// "0x" followed by one to eight hex digits.
type Word uint32

// UnmarshalJSON implements json.Unmarshaler.
func (w *Word) UnmarshalJSON(data []byte) error {
	v, err := parseWord(data)
	if err != nil {
		return err
	}
	*w = Word(v)
	return nil
}

// ExpectWord is an expected word: a Word, or the string "fill" for the
// binding's fill value (the sentinel must still be there).
type ExpectWord struct {
	Value uint32
	Fill  bool
}

// UnmarshalJSON implements json.Unmarshaler.
func (w *ExpectWord) UnmarshalJSON(data []byte) error {
	if string(data) == `"fill"` {
		*w = ExpectWord{Fill: true}
		return nil
	}
	v, err := parseWord(data)
	if err != nil {
		return err
	}
	*w = ExpectWord{Value: v}
	return nil
}

func parseWord(data []byte) (uint32, error) {
	s := string(data)
	if hex, ok := strings.CutPrefix(s, `"0x`); ok {
		hex, ok = strings.CutSuffix(hex, `"`)
		if ok && len(hex) >= 1 && len(hex) <= 8 {
			if v, err := strconv.ParseUint(hex, 16, 32); err == nil {
				return uint32(v), nil
			}
		}
		return 0, fmt.Errorf("word %s: want \"0x\" and 1 to 8 hex digits", s)
	}
	v, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("word %s: want an integer in [0, 4294967295] or a \"0x…\" string", s)
	}
	return uint32(v), nil
}

// ParseSpec decodes one case's JSON and checks it against the rules of
// FormatVersion. Unknown fields are errors.
func ParseSpec(data []byte) (Spec, error) {
	var s Spec
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return Spec{}, err
	}
	if dec.More() {
		return Spec{}, errors.New("data after the case object")
	}
	if err := s.validate(); err != nil {
		return Spec{}, err
	}
	return s, nil
}
