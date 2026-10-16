package main

import (
	"errors"
	"strings"
	"testing"
)

// failingWriter fails every write, as stdin of a git process that died.
type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }
func (w failingWriter) Close() error              { return nil }

// A failed read must not hide why the requests could not be written: the
// write error is usually the cause.
func TestCatFileBatchReportsWriteErrorWithReadError(t *testing.T) {
	errWrite := errors.New("broken pipe")
	_, err := catFileBatch(failingWriter{errWrite}, strings.NewReader(""), []string{"0123abcd"})
	if err == nil {
		t.Fatal("catFileBatch succeeded without any response")
	}
	if !errors.Is(err, errWrite) {
		t.Errorf("error = %q, want it to include the write error %q", err, errWrite)
	}
	if !strings.Contains(err.Error(), "reading header for 0123abcd") {
		t.Errorf("error = %q, want it to include the read error", err)
	}
}

func TestCatFileBatchReadsObjects(t *testing.T) {
	var stdin strings.Builder
	w := nopWriteCloser{&stdin}
	out := "aa blob 3\nfoo\nbb blob 0\n\n"
	got, err := catFileBatch(w, strings.NewReader(out), []string{"aa", "bb"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || string(got[0]) != "foo" || len(got[1]) != 0 {
		t.Errorf("contents = %q", got)
	}
	if stdin.String() != "aa\nbb\n" {
		t.Errorf("requests = %q", stdin.String())
	}
}

type nopWriteCloser struct{ *strings.Builder }

func (nopWriteCloser) Close() error { return nil }
