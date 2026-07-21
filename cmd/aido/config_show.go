package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sort"

	"github.com/0xkhdr/aido/internal/config"
)

// configShow prints the loaded config's non-secret values.
//
// It always returns 0. aido reports and does not block (product.md P5, R6.2):
// a missing or invalid config is described on stderr, never turned into a
// failing exit code.
//
// It never resolves a key, so no key value can reach either stream (R6.3).
func configShow(args []string, stdout, stderr io.Writer) int {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	root := config.NewRoot(dir)
	c, err := config.Load(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(stderr, "aido: no %s found\n", root.ConfigPath())
		} else {
			fmt.Fprintf(stderr, "aido: %v\n", err)
		}
		return 0
	}
	if err := c.Validate(); err != nil {
		var ve *config.ValidationError
		if errors.As(err, &ve) {
			for _, problem := range ve.Problems {
				fmt.Fprintf(stderr, "aido: %s\n", problem)
			}
		} else {
			fmt.Fprintf(stderr, "aido: %v\n", err)
		}
	}
	writeConfig(stdout, c)
	return 0
}

// writeConfig writes the non-secret view of c.
func writeConfig(w io.Writer, c *config.Config) {
	fmt.Fprintf(w, "project: %s\n", c.Project)
	fmt.Fprintf(w, "tracked_branch: %s\n", c.TrackedBranch)
	fmt.Fprintf(w, "last_sync_commit: %s\n", c.LastSyncCommit)
	fmt.Fprintf(w, "auto_sync: %t\n", c.AutoSync)
	fmt.Fprintf(w, "llm.default_provider: %s\n", c.LLM.DefaultProvider)
	fmt.Fprintf(w, "llm.default_model: %s\n", c.LLM.DefaultModel)
	for _, doc := range c.RequiredDocs {
		fmt.Fprintf(w, "required_docs: %s\n", doc)
	}
	names := make([]string, 0, len(c.LLM.Providers))
	for name := range c.LLM.Providers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		p := c.LLM.Providers[name]
		// api_key_source is printed verbatim: it is a source *reference*, never
		// a key (R6.3, product.md P7).
		fmt.Fprintf(w, "provider %s: base_url=%s api_key_source=%s\n", name, p.BaseURL, p.APIKeySource)
	}
}
