// Package clarityconfig loads the opt-in configuration stored under a repo's
// .clarity directory. It is the single source of truth for module declarations
// so every consumer (`clarity show --modules`, `clarity show --module <name>`,
// and `clarity modules`) resolves the same set and cannot drift apart.
package clarityconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LegacyCodeHQ/clarity/depgraph"
)

// configDir is the per-repo configuration directory auto-discovered at the
// repository root. It holds opt-in configuration files such as modules.json.
const configDir = ".clarity"

// modulesFile names the file inside .clarity that declares modules to collapse
// in rendered graphs.
const modulesFile = "modules.json"

// moduleConfigEntry is a single module declaration in .clarity/modules.json.
// Files may be literal repo-relative paths or globs (e.g. pkg/*.go).
type moduleConfigEntry struct {
	Name  string   `json:"name"`
	Files []string `json:"files"`
}

type modulesConfig struct {
	Modules []moduleConfigEntry `json:"modules"`
}

// LoadModules reads .clarity/modules.json from repoRoot and returns the declared
// modules with their file patterns expanded (globs) and resolved to absolute
// graph-node paths. It returns nil with no error when the file is absent, so
// module config is purely opt-in. Globs that match nothing are not an error:
// CollapseModules already ignores members absent from the graph.
func LoadModules(repoRoot string) ([]depgraph.Module, error) {
	path := filepath.Join(repoRoot, configDir, modulesFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var cfg modulesConfig
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	modules := make([]depgraph.Module, 0, len(cfg.Modules))
	for _, entry := range cfg.Modules {
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			return nil, fmt.Errorf("invalid module in %s: name is required", path)
		}
		if len(entry.Files) == 0 {
			return nil, fmt.Errorf("invalid module %q in %s: no files specified", name, path)
		}
		files, err := resolveModulePatterns(repoRoot, entry.Files)
		if err != nil {
			return nil, fmt.Errorf("module %q in %s: %w", name, path, err)
		}
		modules = append(modules, depgraph.Module{Name: name, Files: files})
	}
	return modules, nil
}

// resolveModulePatterns turns repo-relative literal paths and globs into
// absolute, de-duplicated graph-node paths rooted at repoRoot.
func resolveModulePatterns(repoRoot string, patterns []string) ([]string, error) {
	var files []string
	seen := make(map[string]bool)
	add := func(p string) {
		clean := filepath.Clean(p)
		if !seen[clean] {
			seen[clean] = true
			files = append(files, clean)
		}
	}

	for _, raw := range patterns {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		joined := filepath.Join(repoRoot, raw)
		if hasGlobMeta(raw) {
			matches, err := filepath.Glob(joined)
			if err != nil {
				return nil, fmt.Errorf("invalid glob %q: %w", raw, err)
			}
			for _, m := range matches {
				add(m)
			}
			continue
		}
		add(joined)
	}
	return files, nil
}

func hasGlobMeta(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[")
}
