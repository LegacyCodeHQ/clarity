package moduleapi

// Context contains precomputed project data shared across language providers.
type Context struct {
	SuppliedFiles map[string]bool
	DirToFiles    map[string][]string
	JavaFiles     []string
	KotlinFiles   []string
	GoFiles       []string
}
