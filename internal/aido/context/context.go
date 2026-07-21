// Package context resolves explicit repository document references.
package context

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// Result describes one independently resolved path[#heading] reference.
type Result struct {
	Reference string
	Path      string
	Heading   string
	Content   string
	Resolved  bool
	Error     string
}

// Resolve reads references beneath root in input order. An invalid or missing
// reference is reported in its own result and does not stop later resolutions.
func Resolve(root string, references []string) []Result {
	results := make([]Result, len(references))
	for i, reference := range references {
		results[i] = resolve(root, reference)
	}
	return results
}

func resolve(root, reference string) Result {
	result := Result{Reference: reference}
	path, fragment, _ := strings.Cut(reference, "#")
	result.Path = filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if fragment != "" {
		decoded, err := url.PathUnescape(fragment)
		if err != nil {
			result.Error = fmt.Sprintf("invalid heading in %q: %v", reference, err)
			return result
		}
		result.Heading = decoded
	}

	fullPath, err := containedPath(root, path)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		result.Error = fmt.Sprintf("resolve %q: %v", reference, err)
		return result
	}
	if result.Heading == "" {
		result.Content = string(data)
		result.Resolved = true
		return result
	}

	content, matches := section(string(data), result.Heading)
	switch matches {
	case 0:
		result.Error = fmt.Sprintf("resolve %q: heading not found", reference)
	case 1:
		result.Content = content
		result.Resolved = true
	default:
		result.Error = fmt.Sprintf("resolve %q: heading is ambiguous (%d matches)", reference, matches)
	}
	return result
}

func containedPath(root, reference string) (string, error) {
	if reference == "" {
		return "", fmt.Errorf("resolve %q: empty document path", reference)
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve %q: %v", reference, err)
	}
	path := filepath.Clean(filepath.Join(root, filepath.FromSlash(reference)))
	if !within(root, path) {
		return "", fmt.Errorf("resolve %q: path escapes context root", reference)
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve %q: %v", reference, err)
	}
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path, nil // Preserve the more useful ReadFile error for missing paths.
	}
	if !within(realRoot, realPath) {
		return "", fmt.Errorf("resolve %q: path escapes context root", reference)
	}
	return realPath, nil
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func section(document, wanted string) (string, int) {
	lines := strings.SplitAfter(document, "\n")
	wanted = slug(wanted)
	start, level, matches := -1, 0, 0
	for i, line := range lines {
		heading, headingLevel, ok := markdownHeading(line)
		if !ok {
			continue
		}
		if slug(heading) == wanted {
			matches++
			if start < 0 {
				start, level = i, headingLevel
			}
			continue
		}
		if start >= 0 && headingLevel <= level {
			if matches == 1 {
				return strings.Join(lines[start:i], ""), matches
			}
			start = -1
		}
	}
	if start >= 0 && matches == 1 {
		return strings.Join(lines[start:], ""), matches
	}
	return "", matches
}

func markdownHeading(line string) (string, int, bool) {
	line = strings.TrimSpace(line)
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || len(line) == level || !unicode.IsSpace(rune(line[level])) {
		return "", 0, false
	}
	return strings.TrimSpace(strings.TrimRight(strings.TrimSpace(line[level:]), "#")), level, true
}

func slug(value string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			dash = false
		case unicode.IsSpace(r) || r == '-':
			if b.Len() > 0 && !dash {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}
