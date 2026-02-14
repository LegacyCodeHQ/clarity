package watch

import (
	"embed"
	"io/fs"
)

//go:embed dist
var distFS embed.FS

// getDistFS returns the embedded dist directory as a filesystem.
func getDistFS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
