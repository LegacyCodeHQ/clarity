package show

import (
	"github.com/LegacyCodeHQ/clarity/clarityconfig"
	"github.com/LegacyCodeHQ/clarity/depgraph"
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
func resolveConfigModules(repoPath string, enabled bool) ([]depgraph.Module, error) {
	if !enabled {
		return nil, nil
	}

	return loadConfigModules(repoPath)
}
