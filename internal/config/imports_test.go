package config

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// allowedModules is the entire non-stdlib import surface this package may have
// (tech.md T1). Adding to it is a steering decision, not a code change.
// Entries are module paths; a subpackage of an allowed module is allowed.
// go-billy is go-git's filesystem interface and arrives only through it, so it
// rides on the same T1 entry rather than being a separate dependency decision.
var allowedModules = []string{
	"gopkg.in/yaml.v3",
	"github.com/go-git/go-git/v5",
	"github.com/go-git/go-billy/v5",
}

// forbiddenSubtrees are packages inside an allowed module that are not allowed.
// Prefix matching on a module is convenient but blunt: go-git's transport
// packages speak HTTP and SSH, and importing one would drag net/http and
// crypto/tls back into a package whose whole claim is that it makes no network
// call. The dependency-graph assertion in cmd/aido is the real guard; this
// names the mistake at the import site, where it is cheaper to see.
var forbiddenSubtrees = map[string]string{
	"github.com/go-git/go-git/v5/plumbing/transport": "go-git's transport packages pull in net/http and crypto/tls (design.md I5)",
}

// allowedModule reports whether path is one of the allowed modules or lives
// inside one, and is not in a forbidden subtree.
func allowedModule(path string) bool {
	for prefix := range forbiddenSubtrees {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return false
		}
	}
	for _, module := range allowedModules {
		if path == module || strings.HasPrefix(path, module+"/") {
			return true
		}
	}
	return false
}

// forbidden names imports that are banned outright regardless of origin:
// net/http because this package makes no network call (invariant I5, T5), C
// because the build must stay CGO_ENABLED=0 (R1.1, T3), and os/exec because
// T3's other half — "a runtime that requires the git binary on PATH is
// refused" — is unenforceable otherwise. os/exec is standard library, so the
// allowlist alone would have let it through, and did: this package shipped a
// `git check-ignore` subprocess past every gate until an audit caught it.
var forbidden = map[string]string{
	"net/http": "internal/config makes no network call (design.md I5)",
	"C":        "the build must stay CGO_ENABLED=0 (R1.1, tech.md T3)",
	"os/exec":  "no shelling out; git operations go through go-git (tech.md T3)",
}

// disallowedImport returns the reason path may not be imported, or "".
func disallowedImport(path string) string {
	if reason, ok := forbidden[path]; ok {
		return reason
	}
	// A stdlib path's first element never contains a dot; every module path's
	// does (a domain).
	first, _, _ := strings.Cut(path, "/")
	if !strings.Contains(first, ".") {
		return ""
	}
	if allowedModule(path) {
		return ""
	}
	return "not in the tech.md T1 allowlist"
}

// imports parses every .go file in dir and returns each import path with the
// file it appears in.
func imports(t *testing.T, dir string) map[string]string {
	t.Helper()
	found := map[string]string{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, spec := range file.Imports {
			value, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatalf("%s: bad import literal %s", path, spec.Path.Value)
			}
			found[value] = e.Name()
		}
	}
	if len(found) == 0 {
		t.Fatalf("no imports found in %s; the check would pass vacuously", dir)
	}
	return found
}

// T1, T3, T5, T7: the package's real import set stays inside the allowlist.
func TestPackageImportsStayInAllowlist(t *testing.T) {
	for path, file := range imports(t, ".") {
		if reason := disallowedImport(path); reason != "" {
			t.Errorf("%s imports %q: %s", file, path, reason)
		}
	}
}

// The check is not vacuous: a disallowed import is actually caught.
func TestDisallowedImportIsCaught(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"os", false},
		{"encoding/json", false},
		{"gopkg.in/yaml.v3", false},
		{"net/http", true},
		{"C", true},
		{"github.com/spf13/viper", true},
		{"github.com/zalando/go-keyring", true},
		{"os/exec", true},
		{"github.com/go-git/go-git/v5", false},
		{"github.com/go-git/go-git/v5/plumbing/format/gitignore", false},
		{"github.com/go-git/go-billy/v5/osfs", false},
		{"github.com/go-git/go-git-evil/v5", true},
		{"github.com/go-git/go-git/v5/plumbing/transport/http", true},
		{"github.com/go-git/go-git/v5/plumbing/transport", true},
	}
	for _, tt := range tests {
		got := disallowedImport(tt.path) != ""
		if got != tt.want {
			t.Errorf("disallowedImport(%q) disallowed = %t, want %t", tt.path, got, tt.want)
		}
	}
}

// The parser-driven half is not vacuous either: a file carrying a banned import
// is detected by the same code path the real check uses.
func TestParserCatchesBannedImportInSource(t *testing.T) {
	dir := t.TempDir()
	src := "package config\n\nimport (\n\t\"net/http\"\n\t\"github.com/spf13/viper\"\n)\n\nvar _ = http.StatusOK\nvar _ = viper.GetString\n"
	if err := os.WriteFile(filepath.Join(dir, "bad.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	var caught []string
	for path := range imports(t, dir) {
		if disallowedImport(path) != "" {
			caught = append(caught, path)
		}
	}
	if len(caught) != 2 {
		t.Errorf("caught %v, want both net/http and github.com/spf13/viper", caught)
	}
}

// Every ast import in the package is a plain path, so no blank or dot import
// slips the allowlist by aliasing.
func TestNoAliasedImportsHideOrigin(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			for _, spec := range file.Imports {
				if spec.Name != nil && (spec.Name.Name == "." || spec.Name.Name == "_") {
					t.Errorf("%s: %s import of %s hides its origin", name, spec.Name.Name, spec.Path.Value)
				}
			}
		}
	}
}
