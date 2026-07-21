package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/0xkhdr/aido/internal/config"
)

const secretKey = "sk-must-never-be-printed-0123456789"

const validConfig = `project: taxi
tracked_branch: main
last_sync_commit: abc123
required_docs:
  - okf/architecture.md
llm:
  default_provider: openrouter
  default_model: anthropic/claude-sonnet-4-20250514
  providers:
    openrouter:
      api_key_source: env:OPENROUTER_API_KEY
      base_url: https://openrouter.ai/api/v1
    ollama:
      api_key_source: none
      base_url: http://localhost:11434
auto_sync: false
`

// projectDir lays out a project directory, optionally writing config.yaml and
// a .secrets.yaml holding a key that must never be printed.
func projectDir(t *testing.T, configBody string) string {
	t.Helper()
	dir := t.TempDir()
	// R1.2: even the fixture obtains .aido/ paths from the constructors. Joining
	// them here would be the same violation the test below scans for.
	root := config.NewRoot(dir)
	if err := os.MkdirAll(root.String(), 0o755); err != nil {
		t.Fatal(err)
	}
	if configBody != "" {
		if err := os.WriteFile(root.ConfigPath(), []byte(configBody), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(root.SecretsPath(), []byte("openrouter_api_key: "+secretKey+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// R1.2: no package outside internal/config builds a .aido/ path from string
// parts. internal/config's import-allowlist test cannot see this package, so
// the rule needs its own check on this side of the boundary — an audit found
// this file itself in violation, with nothing to catch it.
func TestNoHandBuiltAidoPaths(t *testing.T) {
	// Assembled rather than written out, so this test does not match itself.
	needle := "." + config.DirName[1:]
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				value, err := strconv.Unquote(lit.Value)
				if err != nil || !strings.Contains(value, needle) {
					return true
				}
				found++
				t.Errorf("%s builds a %s path from a string literal %q; use config.NewRoot and its constructors",
					fset.Position(lit.Pos()), needle, value)
				return true
			})
		}
	}
	if found == 0 && testing.Verbose() {
		t.Logf("no hand-built %s paths in package main", needle)
	}
}

// show runs `config show` against dir and returns exit code, stdout, stderr.
func show(t *testing.T, dir string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run([]string{"config", "show", dir}, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// R6.1: a valid config prints its non-secret values and exits zero.
func TestConfigShowValid(t *testing.T) {
	code, stdout, stderr := show(t, projectDir(t, validConfig))
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty for a valid config", stderr)
	}
	for _, want := range []string{
		"project: taxi",
		"tracked_branch: main",
		"last_sync_commit: abc123",
		"auto_sync: false",
		"llm.default_provider: openrouter",
		"llm.default_model: anthropic/claude-sonnet-4-20250514",
		"required_docs: okf/architecture.md",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q\ngot:\n%s", want, stdout)
		}
	}
}

// R6.3: provider lines carry api_key_source verbatim and no key value.
func TestConfigShowPrintsKeySourceNotKey(t *testing.T) {
	dir := projectDir(t, validConfig)
	t.Setenv("OPENROUTER_API_KEY", secretKey)
	code, stdout, stderr := show(t, dir)
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	for _, want := range []string{
		"provider openrouter: base_url=https://openrouter.ai/api/v1 api_key_source=env:OPENROUTER_API_KEY",
		"provider ollama: base_url=http://localhost:11434 api_key_source=none",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q\ngot:\n%s", want, stdout)
		}
	}
	// The key is reachable from both the environment and .secrets.yaml here.
	if strings.Contains(stdout, secretKey) || strings.Contains(stderr, secretKey) {
		t.Error("a resolved key value reached the output")
	}
}

// R6.2: a missing config reports on stderr and still exits zero.
func TestConfigShowMissingConfigExitsZero(t *testing.T) {
	code, _, stderr := show(t, projectDir(t, ""))
	if code != 0 {
		t.Errorf("exit = %d, want 0 (aido reports, it does not block)", code)
	}
	if !strings.Contains(stderr, "config.yaml") {
		t.Errorf("stderr = %q, want it to name the missing file", stderr)
	}
}

// R6.2, R3.5: every validation problem is printed, and the exit stays zero.
func TestConfigShowInvalidConfigReportsEveryProblem(t *testing.T) {
	code, _, stderr := show(t, projectDir(t, "required_docs:\n  - docs/architecture.md\n"))
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	for _, want := range []string{"project", "tracked_branch", "docs/architecture.md"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr %q does not report %q", stderr, want)
		}
	}
}

// An unknown subcommand is a usage error, which is not a config outcome and so
// is allowed a non-zero exit.
func TestUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"wat"}, &stdout, &stderr); code == 0 {
		t.Error("exit = 0 for an unknown command, want non-zero")
	}
}

// Integration: the built binary, run as a real process against a real .aido/
// tree, honours the same contract (R6.1, R6.2, R6.3 end to end).
func TestConfigShowBinaryIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a binary")
	}
	bin := filepath.Join(t.TempDir(), "aido")
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v: %s", err, out)
	}
	dir := projectDir(t, validConfig)

	cmd := exec.Command(bin, "config", "show", dir)
	cmd.Env = append(os.Environ(), "OPENROUTER_API_KEY="+secretKey)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("aido config show: %v (stderr: %s)", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "project: taxi") {
		t.Errorf("stdout = %q", stdout.String())
	}
	if strings.Contains(stdout.String(), secretKey) || strings.Contains(stderr.String(), secretKey) {
		t.Error("a resolved key value reached the output of the real binary")
	}

	// Negative path at the same boundary: no config at all, still exit zero.
	empty := exec.Command(bin, "config", "show", projectDir(t, ""))
	var estdout, estderr bytes.Buffer
	empty.Stdout, empty.Stderr = &estdout, &estderr
	if err := empty.Run(); err != nil {
		t.Fatalf("missing config exited non-zero: %v (stderr: %s)", err, estderr.String())
	}
	if !strings.Contains(estderr.String(), "config.yaml") {
		t.Errorf("stderr = %q, want the missing file named", estderr.String())
	}
}
