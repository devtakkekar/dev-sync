package ignore

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// DefaultIgnores are always active, ensuring .devsync state and temp files are never synced.
var DefaultIgnores = []string{
	".devsync",
	".devsync/**",
	".git",
	".git/**",
	".devsync-tmp.*",
	"*.devsync-tmp.*",
}

// IgnoreRule represents a parsed ignore pattern.
type IgnoreRule struct {
	Pattern   string
	Negated   bool
	DirOnly   bool
	Recursive bool
}

// IgnoreMatcher matches relative paths against ignore rules.
type IgnoreMatcher struct {
	rules []IgnoreRule
}

// NewMatcher creates an IgnoreMatcher with default built-in rules.
func NewMatcher() *IgnoreMatcher {
	m := &IgnoreMatcher{}
	for _, p := range DefaultIgnores {
		m.AddRule(p)
	}
	return m
}

// LoadIgnoreFile reads a .devsyncignore file from disk and parses it.
func LoadIgnoreFile(path string) (*IgnoreMatcher, error) {
	m := NewMatcher()
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m.AddRule(line)
	}

	return m, scanner.Err()
}

// AddRule parses a single ignore line and adds it to the matcher.
func (m *IgnoreMatcher) AddRule(line string) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return
	}

	negated := false
	if strings.HasPrefix(line, "!") {
		negated = true
		line = strings.TrimPrefix(line, "!")
	}

	// Normalize slashes
	line = filepath.ToSlash(line)
	dirOnly := strings.HasSuffix(line, "/")
	line = strings.TrimSuffix(line, "/")

	recursive := strings.Contains(line, "**")

	m.rules = append(m.rules, IgnoreRule{
		Pattern:   line,
		Negated:   negated,
		DirOnly:   dirOnly,
		Recursive: recursive,
	})
}

// ShouldIgnore tests if a given relative path should be ignored.
// isDir specifies if the item is a directory.
func (m *IgnoreMatcher) ShouldIgnore(relPath string, isDir bool) bool {
	// Normalize to forward slashes and trim leading/trailing slashes
	normalized := filepath.ToSlash(relPath)
	normalized = strings.Trim(normalized, "/")
	if normalized == "" || normalized == "." {
		return false
	}

	ignored := false

	for _, rule := range m.rules {
		if rule.DirOnly && !isDir {
			continue
		}

		matched := matchPattern(rule.Pattern, normalized, isDir)
		if matched {
			if rule.Negated {
				ignored = false
			} else {
				ignored = true
			}
		}
	}

	return ignored
}

func matchPattern(pattern, path string, isDir bool) bool {
	pattern = strings.Trim(pattern, "/")

	// Direct match
	if pattern == path {
		return true
	}

	// Double asterisk wildcard: **/abc or abc/** or a/**/b
	if strings.Contains(pattern, "**") {
		parts := strings.Split(pattern, "/**/")
		if len(parts) == 2 {
			prefix := parts[0]
			suffix := parts[1]
			if strings.HasPrefix(path, prefix+"/") && strings.HasSuffix(path, "/"+suffix) {
				return true
			}
		}
		if strings.HasPrefix(pattern, "**/") {
			suffix := strings.TrimPrefix(pattern, "**/")
			if strings.HasSuffix(path, "/"+suffix) || path == suffix {
				return true
			}
			matched, _ := filepath.Match(suffix, filepath.Base(path))
			if matched {
				return true
			}
		}
		if strings.HasSuffix(pattern, "/**") {
			prefix := strings.TrimSuffix(pattern, "/**")
			if strings.HasPrefix(path, prefix+"/") || path == prefix {
				return true
			}
		}
	}

	// Check if path is inside directory
	if strings.HasPrefix(path, pattern+"/") {
		return true
	}

	// Simple wildcard match for filename
	base := filepath.Base(path)
	if matched, _ := filepath.Match(pattern, base); matched {
		return true
	}

	// Full path wildcard match
	if matched, _ := filepath.Match(pattern, path); matched {
		return true
	}

	return false
}
