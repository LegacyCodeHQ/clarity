package moduleapi

// Context contains precomputed project data shared across language providers.
type Context struct {
	SuppliedFiles map[string]bool
	DirToFiles    map[string][]string
	// FilesByExtension groups supplied files by their extension (e.g. ".go").
	// Providers fetch the extensions they declare, so this shared contract never
	// enumerates specific languages.
	FilesByExtension map[string][]string
}
