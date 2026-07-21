package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewRootRelativeAndAbsolute(t *testing.T) {
	if got, want := string(NewRoot("proj")), filepath.Join("proj", ".aido"); got != want {
		t.Errorf("relative: got %q, want %q", got, want)
	}
	if got, want := string(NewRoot("/srv/proj")), "/srv/proj/.aido"; got != want {
		t.Errorf("absolute: got %q, want %q", got, want)
	}
}

// constructors returns every path constructor on Root, so a new one added
// without a test here still gets the purity check below.
func constructors(r Root) map[string]string {
	return map[string]string{
		"ConfigPath":   r.ConfigPath(),
		"SecretsPath":  r.SecretsPath(),
		"OKFDir":       r.OKFDir(),
		"QueriesDir":   r.QueriesDir(),
		"LinksPath":    r.LinksPath(),
		"WitnessDir":   r.WitnessDir(),
		"TemplatesDir": r.TemplatesDir(),
	}
}

func TestConstructorsMatchOnDiskContract(t *testing.T) {
	r := NewRoot("/srv/proj")
	want := map[string]string{
		"ConfigPath":   "/srv/proj/.aido/config.yaml",
		"SecretsPath":  "/srv/proj/.aido/.secrets.yaml",
		"OKFDir":       "/srv/proj/.aido/okf",
		"QueriesDir":   "/srv/proj/.aido/queries",
		"LinksPath":    "/srv/proj/.aido/links.yaml",
		"WitnessDir":   "/srv/proj/.aido/witness",
		"TemplatesDir": "/srv/proj/.aido/templates",
	}
	for name, got := range constructors(r) {
		if got != want[name] {
			t.Errorf("%s: got %q, want %q", name, got, want[name])
		}
	}
}

// R1.3: asking for a path must not create it, nor any parent.
func TestConstructorsCreateNothing(t *testing.T) {
	dir := t.TempDir()
	r := NewRoot(dir)

	for name, p := range constructors(r) {
		if !strings.HasPrefix(p, dir) {
			t.Errorf("%s: %q escaped the project dir %q", name, p, dir)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("constructors created %d entries in an empty project dir: %v", len(entries), entries)
	}
	if _, err := os.Stat(string(r)); !os.IsNotExist(err) {
		t.Errorf("NewRoot created %s (stat err = %v)", r, err)
	}
}
