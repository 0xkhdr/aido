package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrKeyNotFound reports that no source held a key for a provider. It is
// distinct from an I/O error so a caller can tell "you have not set this up"
// from "your disk is broken" (R4.3).
var ErrKeyNotFound = errors.New("api key not found")

// ErrNotGitIgnored reports a refusal to write a resolved key to a path git
// would track (R4.6).
var ErrNotGitIgnored = errors.New("refusing to write a key to a path that is not git-ignored")

// secretsKeys maps a provider name to its key in .aido/.secrets.yaml. Only
// nvidia_nim differs from <provider>_api_key (blueprint §4.4).
var secretsKeys = map[string]string{"nvidia_nim": "nvidia_api_key"}

// secretsKey is the .secrets.yaml key holding provider's API key.
func secretsKey(provider string) string {
	if k, ok := secretsKeys[provider]; ok {
		return k
	}
	return provider + "_api_key"
}

// ResolveKey returns the API key for provider, consulting the environment
// variable named by api_key_source first and .aido/.secrets.yaml second
// (blueprint §4.5 steps 1-2). Keyring and interactive prompting are non-goals.
//
// Invariant I1: no error returned here contains the key, any substring of it,
// or any content read from .secrets.yaml. Errors name providers, variables, and
// paths only.
func (c *Config) ResolveKey(r Root, provider string) (string, error) {
	p, ok := c.LLM.Providers[provider]
	if !ok {
		return "", fmt.Errorf("provider %s has no entry under llm.providers: %w", provider, ErrKeyNotFound)
	}
	// R4.4: an explicitly keyless provider (ollama) is not a failure.
	if p.APIKeySource == "none" {
		return "", nil
	}
	consulted := make([]string, 0, 2)
	if envName, ok := strings.CutPrefix(p.APIKeySource, "env:"); ok {
		consulted = append(consulted, "$"+envName)
		// R4.2: set-but-empty is treated as unset and falls through.
		if v := os.Getenv(envName); v != "" {
			return v, nil
		}
	}
	path := r.SecretsPath()
	consulted = append(consulted, path)
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		// R4.3: an absent secrets file is not-found, not an I/O failure.
		return "", fmt.Errorf("%w for provider %s (consulted %s)", ErrKeyNotFound, provider, strings.Join(consulted, ", "))
	case err != nil:
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	var secrets map[string]string
	if err := yaml.Unmarshal(data, &secrets); err != nil {
		// The yaml error is deliberately NOT wrapped: it quotes the offending
		// line, which is a key value (I1, R4.5). The message is rebuilt.
		return "", fmt.Errorf("parse %s: file is not valid YAML", path)
	}
	if v := secrets[secretsKey(provider)]; v != "" {
		return v, nil
	}
	return "", fmt.Errorf("%w for provider %s (consulted %s)", ErrKeyNotFound, provider, strings.Join(consulted, ", "))
}

// WriteSecrets writes .aido/.secrets.yaml at mode 0600 (R5.4), and only after
// confirming git ignores it (R4.6). Both guards are refusals, not warnings: a
// key written to a tracked path is a leak the moment anyone commits.
//
// It is the only function in this package that writes a key anywhere.
func WriteSecrets(r Root, secrets map[string]string) error {
	path := r.SecretsPath()
	ignored, err := gitIgnores(r.String(), path)
	if err != nil {
		return err
	}
	if !ignored {
		return fmt.Errorf("%w: %s", ErrNotGitIgnored, path)
	}
	data, err := yaml.Marshal(secrets)
	if err != nil {
		// Reached only if a value is unmarshalable, which for map[string]string
		// it is not — but the error must still never carry the map.
		return fmt.Errorf("encode secrets for %s", path)
	}
	return WriteFile(path, data, 0o600)
}

// gitIgnores reports whether git ignores path. A path outside any repository
// counts as not ignored: nothing is protecting it.
func gitIgnores(dir, path string) (bool, error) {
	cmd := exec.Command("git", "check-ignore", "-q", "--", path)
	cmd.Dir = dir
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		// 1 = not ignored, 128 = not a git repository. Both mean unprotected.
		if code := exit.ExitCode(); code == 1 || code == 128 {
			return false, nil
		}
	}
	return false, fmt.Errorf("git check-ignore %s: %w", path, err)
}
