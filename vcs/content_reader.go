package vcs

import "os"

// ContentReader is a function that reads file content given a file path.
// This allows the caller to control how files are read (filesystem, git, etc.)
type ContentReader func(filePath string) ([]byte, error)

// FilesystemContentReader returns a ContentReader that reads from the filesystem.
func FilesystemContentReader() ContentReader {
	return func(absPath string) ([]byte, error) {
		return os.ReadFile(absPath)
	}
}
