package show

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/LegacyCodeHQ/clarity/clarityconfig"
	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/LegacyCodeHQ/clarity/vcs/git"
)

// loadConfigModules reads .clarity/modules.json from repoRoot via the shared
// clarityconfig loader, so `show` and `clarity modules` resolve modules
// identically. It is kept as a package-local wrapper for show's call sites.
func loadConfigModules(repoRoot string) ([]depgraph.Module, error) {
	return clarityconfig.LoadModules(repoRoot)
}

// resolveConfigModules returns modules declared in .clarity/modules.json, but
// only when enabled via --modules (or --module <name>). Module collapsing is off
// by default so a bare render never groups, even when the config file is present.
func resolveConfigModules(repoPath string, enabled bool, commitID string) ([]depgraph.Module, error) {
	if !enabled {
		return nil, nil
	}

	if commitID != "" {
		return loadConfigModulesFromCommit(repoPath, commitID)
	}

	return loadConfigModules(repoPath)
}

func loadConfigModulesFromCommit(repoPath, commitID string) ([]depgraph.Module, error) {
	treeFiles, err := git.GetCommitTreeFiles(repoPath, commitID)
	if err != nil {
		return nil, err
	}

	const modulesRelPath = ".clarity/modules.json"
	var hasConfig bool
	for _, file := range treeFiles {
		rel, relErr := filepath.Rel(repoPath, file)
		if relErr == nil && filepath.ToSlash(rel) == modulesRelPath {
			hasConfig = true
			break
		}
	}
	if !hasConfig {
		return nil, nil
	}

	data, err := git.GetFileContentFromCommit(repoPath, commitID, modulesRelPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || strings.Contains(err.Error(), "exists on disk, but not in") {
			return nil, nil
		}
		return nil, err
	}

	return clarityconfig.LoadModulesFromContent(repoPath, commitID+":"+modulesRelPath, data, treeFiles)
}
