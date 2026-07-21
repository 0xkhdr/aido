package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// fsync is a seam, not a feature. The failures between "temp file created" and
// "renamed into place" are the ones R5.2 and R5.3 are about, and none of them
// (a failing write, chmod, or fsync) can be induced from outside the process on
// a healthy filesystem. Overriding this in a test is the only way to prove the
// cleanup path runs rather than assuming it — an audit found that path at zero
// coverage, with the assertion that covered it passing vacuously.
var fsync = func(f *os.File) error { return f.Sync() }

// WriteFile writes data to path atomically: a temporary file in the *same
// directory*, fsynced, then renamed into place (R5.1, invariant I2). A reader
// sees either the whole old file or the whole new one, never a truncated one.
//
// Any failure before the rename removes the temporary file (R5.3) and leaves an
// existing destination byte-for-byte unchanged (R5.2).
//
// perm is explicit rather than inferred from path — see design.md Alternatives.
// The temporary file is created 0600 by os.CreateTemp and only widened to perm
// afterwards, so a secret is never briefly world-readable.
//
// A missing destination directory is an error. WriteFile never creates one:
// scaffolding .aido/ belongs to a later spec, not to a write primitive.
func WriteFile(path string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", path, err)
	}
	tmp := f.Name()
	// Past this point every failure path must undo the temp file.
	abort := func(verb string, cause error) error {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("%s %s: %w", verb, path, cause)
	}
	if _, err := f.Write(data); err != nil {
		return abort("write", err)
	}
	if err := f.Chmod(perm); err != nil {
		return abort("chmod", err)
	}
	if err := fsync(f); err != nil {
		return abort("fsync", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("close %s: %w", path, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename into %s: %w", path, err)
	}
	// The rename is only durable once the directory entry is. After this point
	// the destination already holds the new bytes, so a failure here is
	// reported but nothing is undone.
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open %s to sync: %w", dir, err)
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("fsync %s: %w", dir, err)
	}
	return nil
}
