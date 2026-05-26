package typescript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// tsConfig captures the subset of tsconfig.json / jsconfig.json that affects
// import path resolution.
type tsConfig struct {
	baseURL string
	paths   map[string][]string
}

var (
	tsConfigCache sync.Map // dir -> *tsConfigEntry
)

type tsConfigEntry struct {
	cfg *tsConfig
}

// loadTsConfigFor walks up from sourceFile looking for tsconfig.json (then
// jsconfig.json). Returns nil if none is found or if parsing fails.
func loadTsConfigFor(sourceFile string) *tsConfig {
	dir := filepath.Dir(sourceFile)
	visited := []string{}
	for {
		if cached, ok := tsConfigCache.Load(dir); ok {
			cfg := cached.(*tsConfigEntry).cfg
			for _, d := range visited {
				tsConfigCache.Store(d, &tsConfigEntry{cfg: cfg})
			}
			return cfg
		}
		visited = append(visited, dir)

		for _, name := range []string{"tsconfig.json", "jsconfig.json"} {
			path := filepath.Join(dir, name)
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			cfg := parseTsConfig(dir, data)
			for _, d := range visited {
				tsConfigCache.Store(d, &tsConfigEntry{cfg: cfg})
			}
			return cfg
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			for _, d := range visited {
				tsConfigCache.Store(d, &tsConfigEntry{cfg: nil})
			}
			return nil
		}
		dir = parent
	}
}

func parseTsConfig(dir string, data []byte) *tsConfig {
	cleaned := stripJSONC(data)
	var raw struct {
		CompilerOptions struct {
			BaseURL string              `json:"baseUrl"`
			Paths   map[string][]string `json:"paths"`
		} `json:"compilerOptions"`
	}
	if err := json.Unmarshal(cleaned, &raw); err != nil {
		return nil
	}

	baseURL := raw.CompilerOptions.BaseURL
	if baseURL == "" {
		baseURL = "."
	}
	absBase := filepath.Clean(filepath.Join(dir, baseURL))

	return &tsConfig{
		baseURL: absBase,
		paths:   raw.CompilerOptions.Paths,
	}
}

var trailingCommaRe = regexp.MustCompile(`,(\s*[}\]])`)

// stripJSONC removes line comments, block comments, and trailing commas so
// that JSONC tsconfig files parse with the standard library. Comments inside
// string literals must be preserved — naive regex stripping mistakes glob
// patterns like "@scope/pkg/*" for block-comment openers and corrupts the
// file.
func stripJSONC(data []byte) []byte {
	out := make([]byte, 0, len(data))
	inString := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if inString {
			out = append(out, c)
			if c == '\\' && i+1 < len(data) {
				out = append(out, data[i+1])
				i++
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}
		if c == '/' && i+1 < len(data) {
			next := data[i+1]
			if next == '/' {
				j := i + 2
				for j < len(data) && data[j] != '\n' {
					j++
				}
				i = j - 1
				continue
			}
			if next == '*' {
				j := i + 2
				for j+1 < len(data) && !(data[j] == '*' && data[j+1] == '/') {
					j++
				}
				if j+1 < len(data) {
					i = j + 1
				} else {
					i = len(data)
				}
				continue
			}
		}
		out = append(out, c)
	}
	return trailingCommaRe.ReplaceAll(out, []byte("$1"))
}

// resolveAlias maps an import like "@/lib/db" through compilerOptions.paths.
// Returns one absolute base path per matching target (no extension).
func (c *tsConfig) resolveAlias(importPath string) []string {
	if c == nil || len(c.paths) == 0 {
		return nil
	}
	var out []string
	for pattern, targets := range c.paths {
		if strings.HasSuffix(pattern, "/*") {
			prefix := strings.TrimSuffix(pattern, "/*")
			if !(importPath == prefix || strings.HasPrefix(importPath, prefix+"/")) {
				continue
			}
			rest := strings.TrimPrefix(strings.TrimPrefix(importPath, prefix), "/")
			for _, target := range targets {
				targetPrefix := strings.TrimSuffix(target, "/*")
				out = append(out, filepath.Clean(filepath.Join(c.baseURL, targetPrefix, rest)))
			}
			continue
		}
		if pattern == importPath {
			for _, target := range targets {
				out = append(out, filepath.Clean(filepath.Join(c.baseURL, target)))
			}
		}
	}
	return out
}
