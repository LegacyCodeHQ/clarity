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
