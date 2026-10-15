package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

// gitRepo reads committed objects of a local git repository. Content is taken
// from the object database, never from the working tree, so it is byte-exact
// whatever line-ending conversion the checkout uses.
type gitRepo struct {
	dir string
}

func (g gitRepo) command(args ...string) *exec.Cmd {
	// The binary is fixed; the arguments are subcommands, flags, object ids and
	// pathspecs built by this tool, plus the repository directory chosen by the
	// maintainer running it.
	return exec.Command("git", append([]string{"-C", g.dir}, args...)...) //nolint:gosec // see above
}

func (g gitRepo) output(args ...string) ([]byte, error) {
	cmd := g.command(args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// head returns the full commit id and committer date (YYYY-MM-DD) of HEAD.
func (g gitRepo) head() (commit, date string, err error) {
	out, err := g.output("show", "-s", "--format=%H %cs", "HEAD")
	if err != nil {
		return "", "", err
	}
	fields := strings.Fields(string(out))
	if len(fields) != 2 {
		return "", "", fmt.Errorf("unexpected `git show` output %q", out)
	}
	return fields[0], fields[1], nil
}

// status returns `git status --porcelain` output; empty means a clean tree.
func (g gitRepo) status() (string, error) {
	out, err := g.output("status", "--porcelain")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// treeEntry is a blob listed by `git ls-tree`.
type treeEntry struct {
	path string // slash-separated, relative to the repository root
	blob string
}

// lsTree lists the blobs of HEAD under the given pathspecs.
func (g gitRepo) lsTree(pathspecs ...string) ([]treeEntry, error) {
	out, err := g.output(append([]string{"ls-tree", "-r", "-z", "--full-tree", "HEAD", "--"}, pathspecs...)...)
	if err != nil {
		return nil, err
	}
	var entries []treeEntry
	for _, rec := range strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00") {
		if rec == "" {
			continue
		}
		// "<mode> SP <type> SP <object> TAB <path>"
		meta, p, ok := strings.Cut(rec, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 {
			return nil, fmt.Errorf("unexpected `git ls-tree` record %q", rec)
		}
		if fields[1] != "blob" {
			return nil, fmt.Errorf("%s: unsupported object type %s", p, fields[1])
		}
		entries = append(entries, treeEntry{path: p, blob: fields[2]})
	}
	return entries, nil
}

// readBlobs returns the content of each blob id, in order, through a single
// `git cat-file --batch` process.
func (g gitRepo) readBlobs(ids []string) (contents [][]byte, err error) {
	cmd := g.command("cat-file", "--batch")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	defer func() {
		if waitErr := cmd.Wait(); waitErr != nil && err == nil {
			err = fmt.Errorf("git cat-file: %w: %s", waitErr, strings.TrimSpace(stderr.String()))
		}
	}()

	// Write requests concurrently so a large batch cannot deadlock on full pipes.
	writeErr := make(chan error, 1)
	go func() {
		var werr error
		for _, id := range ids {
			if _, werr = io.WriteString(stdin, id+"\n"); werr != nil {
				break
			}
		}
		writeErr <- errors.Join(werr, stdin.Close())
	}()

	r := bufio.NewReader(stdout)
	contents = make([][]byte, 0, len(ids))
	for _, id := range ids {
		content, readErr := readBatchObject(r, id)
		if readErr != nil {
			_, _ = io.Copy(io.Discard, r)
			<-writeErr
			return nil, readErr
		}
		contents = append(contents, content)
	}
	if werr := <-writeErr; werr != nil {
		return nil, werr
	}
	return contents, nil
}

// readBatchObject reads one "<id> <type> <size>\n<content>\n" response.
func readBatchObject(r *bufio.Reader, id string) ([]byte, error) {
	header, err := r.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("git cat-file: reading header for %s: %w", id, err)
	}
	fields := strings.Fields(header)
	if len(fields) != 3 || fields[0] != id || fields[1] != "blob" {
		return nil, fmt.Errorf("git cat-file: unexpected header %q for %s", strings.TrimSpace(header), id)
	}
	size, err := strconv.Atoi(fields[2])
	if err != nil || size < 0 {
		return nil, fmt.Errorf("git cat-file: bad size in %q", strings.TrimSpace(header))
	}
	content := make([]byte, size+1) // content plus the trailing LF
	if _, err := io.ReadFull(r, content); err != nil {
		return nil, fmt.Errorf("git cat-file: reading %s: %w", id, err)
	}
	if content[size] != '\n' {
		return nil, fmt.Errorf("git cat-file: missing terminator after %s", id)
	}
	return content[:size], nil
}
