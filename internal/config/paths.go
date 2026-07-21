// Package config owns the .aido/ path contract, config.yaml and .secrets.yaml
// parsing, validation, API key resolution, and atomic writes.
package config

import "path/filepath"

// DirName is the aido state directory inside a target project.
const DirName = ".aido"

// Root is the path to a project's .aido/ directory.
//
// Every path under .aido/ is constructed here (structure.md S6). Constructors
// are pure: they perform no I/O and create nothing (R1.3).
type Root string

// NewRoot returns the Root for a project directory. It does not touch disk.
func NewRoot(projectDir string) Root {
	return Root(filepath.Join(projectDir, DirName))
}

// String returns the .aido/ directory path.
func (r Root) String() string { return string(r) }

// ConfigPath is .aido/config.yaml — repo-safe project configuration.
func (r Root) ConfigPath() string { return filepath.Join(string(r), "config.yaml") }

// SecretsPath is .aido/.secrets.yaml — API keys, git-ignored (product.md P7).
func (r Root) SecretsPath() string { return filepath.Join(string(r), ".secrets.yaml") }

// OKFDir is .aido/okf/ — the OKF v0.1 knowledge bundle.
func (r Root) OKFDir() string { return filepath.Join(string(r), "okf") }

// QueriesDir is .aido/queries/ — aido specs, one per slug.
func (r Root) QueriesDir() string { return filepath.Join(string(r), "queries") }

// LinksPath is .aido/links.yaml — slug to OKF concept id mappings.
func (r Root) LinksPath() string { return filepath.Join(string(r), "links.yaml") }

// WitnessDir is .aido/witness/ — append-only observation logs (structure.md S9).
func (r Root) WitnessDir() string { return filepath.Join(string(r), "witness") }

// TemplatesDir is .aido/templates/ — spec generation templates.
func (r Root) TemplatesDir() string { return filepath.Join(string(r), "templates") }
