package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	root := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("architecture.md", "# Architecture\n\nintro\n\n## Event Flow\n\ndetails\n\n## Other\n\nnope\n")
	write("duplicate.md", "# Same\nfirst\n# Same\nsecond\n")

	results := Resolve(root, []string{
		"architecture.md",
		"architecture.md#event-flow",
		"missing.md",
		"architecture.md#missing",
		"duplicate.md#same",
		"../secret.md",
	})
	if !results[0].Resolved || !strings.Contains(results[0].Content, "Event Flow") {
		t.Fatalf("file resolution: %+v", results[0])
	}
	if !results[1].Resolved || !strings.Contains(results[1].Content, "details") || strings.Contains(results[1].Content, "nope") {
		t.Fatalf("heading resolution: %+v", results[1])
	}
	for i := 2; i < len(results); i++ {
		if results[i].Resolved || results[i].Error == "" {
			t.Fatalf("reference %d should be unresolved: %+v", i, results[i])
		}
	}
}

func TestResolveRejectsSymlinkEscape(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	secret := filepath.Join(outside, "secret.md")
	if err := os.WriteFile(secret, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(root, "linked.md")); err != nil {
		t.Fatal(err)
	}
	result := Resolve(root, []string{"linked.md"})[0]
	if result.Resolved || !strings.Contains(result.Error, "escapes") {
		t.Fatalf("symlink escape should fail: %+v", result)
	}
}
