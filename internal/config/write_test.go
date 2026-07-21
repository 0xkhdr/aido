package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// entries lists dir, failing the test on error.
func entries(t *testing.T, dir string) []string {
	t.Helper()
	found, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(found))
	for _, e := range found {
		names = append(names, e.Name())
	}
	return names
}

// R5.1: a fresh path is written, and nothing else is left behind.
func TestWriteFileCreates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := WriteFile(path, []byte("project: taxi\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() = %v, want nil", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "project: taxi\n" {
		t.Errorf("contents = %q", got)
	}
	if names := entries(t, dir); len(names) != 1 {
		t.Errorf("directory holds %v, want only config.yaml (no temp file left)", names)
	}
}

// R5.1: overwriting replaces the whole file rather than truncating in place.
func TestWriteFileOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("old and considerably longer\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(path, []byte("new\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() = %v, want nil", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new\n" {
		t.Errorf("contents = %q, want %q", got, "new\n")
	}
	if names := entries(t, dir); len(names) != 1 {
		t.Errorf("directory holds %v, want only config.yaml", names)
	}
}

// R5.2, R5.3: a failure before the rename leaves the destination byte-for-byte
// unchanged and removes nothing but its own temp file. The failure is induced
// by making the destination directory unwritable, which is what a real
// permission or read-only-filesystem failure looks like.
func TestWriteFileFailureLeavesDestinationIntact(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions; the failure cannot be induced")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	original := []byte("project: taxi\ntracked_branch: main\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	err := WriteFile(path, []byte("clobbered\n"), 0o644)
	if err == nil {
		t.Fatal("WriteFile() = nil, want an error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not name the destination %q", err, path)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Errorf("destination = %q, want the original bytes %q", got, original)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if names := entries(t, dir); len(names) != 1 {
		t.Errorf("directory holds %v, want only config.yaml (no temp file left)", names)
	}
}

// R5.3: a failure at the rename itself also removes the temp file. Renaming
// onto a non-empty directory fails on every supported platform.
func TestWriteFileRenameFailureRemovesTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.MkdirAll(filepath.Join(path, "occupied"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(path, []byte("new\n"), 0o644); err == nil {
		t.Fatal("WriteFile() = nil, want a rename error")
	}
	if names := entries(t, dir); len(names) != 1 || names[0] != "config.yaml" {
		t.Errorf("directory holds %v, want only the pre-existing config.yaml", names)
	}
}

// R5.1: the temp file lives in the destination directory, not in TMPDIR — a
// cross-filesystem rename is not atomic and would defeat the whole primitive.
func TestWriteFileTempStaysInDestinationDirectory(t *testing.T) {
	tmpdir := t.TempDir()
	t.Setenv("TMPDIR", tmpdir)
	dir := t.TempDir()
	if err := WriteFile(filepath.Join(dir, "config.yaml"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() = %v, want nil", err)
	}
	if names := entries(t, tmpdir); len(names) != 0 {
		t.Errorf("TMPDIR holds %v, want it untouched", names)
	}
}

// R5.4: the caller's mode is honoured on create, so .secrets.yaml lands 0600.
func TestWriteFileHonoursMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets.yaml")
	if err := WriteFile(path, []byte("openai_api_key: sk-x\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() = %v, want nil", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %#o, want 0600", got)
	}
}

// A missing destination directory is an error, never a silent mkdir.
func TestWriteFileMissingDirectoryIsAnError(t *testing.T) {
	dir := t.TempDir()
	if err := WriteFile(filepath.Join(dir, "nope", "config.yaml"), []byte("x\n"), 0o644); err == nil {
		t.Fatal("WriteFile() = nil, want an error for a missing directory")
	}
	if names := entries(t, dir); len(names) != 0 {
		t.Errorf("WriteFile created %v, want no directory created", names)
	}
}
