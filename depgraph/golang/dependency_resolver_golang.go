package golang

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LegacyCodeHQ/sanity/vcs"
	graphlib "github.com/dominikbraun/graph"
)

// ProjectImportResolver encapsulates Go-specific dependency resolution caches and logic.
type ProjectImportResolver struct {
	dirToFiles             map[string][]string
	goPackageExportIndices map[string]GoPackageExportIndex
	suppliedFiles          map[string]bool
	contentReader          vcs.ContentReader
}

// NewProjectImportResolver creates a Go dependency resolver with precomputed package export indices.
func NewProjectImportResolver(
	dirToFiles map[string][]string,
	suppliedFiles map[string]bool,
	contentReader vcs.ContentReader,
) *ProjectImportResolver {
	return &ProjectImportResolver{
		dirToFiles:             dirToFiles,
		goPackageExportIndices: BuildGoPackageExportIndices(dirToFiles, contentReader),
		suppliedFiles:          suppliedFiles,
		contentReader:          contentReader,
	}
}

// ResolveProjectImports resolves Go project imports for a single file using cached indices.
func (r *ProjectImportResolver) ResolveProjectImports(absPath, filePath string) ([]string, error) {
	return ResolveGoProjectImports(
		absPath,
		filePath,
		r.dirToFiles,
		r.goPackageExportIndices,
		r.suppliedFiles,
		r.contentReader,
	)
}

func BuildGoPackageExportIndices(dirToFiles map[string][]string, contentReader vcs.ContentReader) map[string]GoPackageExportIndex {
	goPackageExportIndices := make(map[string]GoPackageExportIndex) // packageDir -> export index
	for dir, files := range dirToFiles {
		// Check if this directory has Go files
		hasGoFiles := false
		var goFilesInDir []string
		for _, f := range files {
			if filepath.Ext(f) == ".go" {
				hasGoFiles = true
				goFilesInDir = append(goFilesInDir, f)
			}
		}
		if hasGoFiles {
			exportIndex, err := BuildPackageExportIndex(goFilesInDir, vcs.ContentReader(contentReader))
			if err != nil {
				continue
			}
			goPackageExportIndices[dir] = exportIndex
		}
	}

	return goPackageExportIndices
}

func ResolveGoProjectImports(
	absPath string,
	filePath string,
	dirToFiles map[string][]string,
	goPackageExportIndices map[string]GoPackageExportIndex,
	suppliedFiles map[string]bool,
	contentReader vcs.ContentReader,
) ([]string, error) {
	sourceContent, err := contentReader(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", absPath, err)
	}

	imports, err := ParseGoImports(sourceContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse imports in %s: %w", filePath, err)
	}

	var projectImports []string

	// Parse //go:embed directives
	embeds, _ := ParseGoEmbeds(sourceContent)
	for _, embed := range embeds {
		embedPath := resolveGoEmbedPath(absPath, embed.Pattern, suppliedFiles)
		if embedPath != "" {
			projectImports = append(projectImports, embedPath)
		}
	}

	// Extract export info for symbol-level cross-package resolution
	exportInfo, _ := ExtractGoExportInfoFromContent(absPath, sourceContent)

	// Determine if this is a test file
	isTestFile := strings.HasSuffix(absPath, "_test.go")

	for _, imp := range imports {
		var importPath string

		// Check both InternalImport and ExternalImport types
		// resolveGoImportPath will determine if they're actually part of this module
		switch typedImp := imp.(type) {
		case InternalImport:
			importPath = typedImp.Path()
		case ExternalImport:
			importPath = typedImp.Path()
		default:
			continue
		}

		packageDir := resolveGoImportPath(absPath, importPath, contentReader)
		if packageDir == "" {
			continue
		}

		sourceDir := filepath.Dir(absPath)
		sameDir := sourceDir == packageDir
		exportIndex, hasExportIndex := goPackageExportIndices[packageDir]

		var usedSymbols map[string]bool
		if exportInfo != nil {
			usedSymbols = GetUsedSymbolsFromPackage(exportInfo, importPath)
		}

		if files, ok := dirToFiles[packageDir]; ok {
			for _, depFile := range files {
				if depFile == absPath {
					continue
				}

				if strings.HasSuffix(depFile, "_test.go") && !sameDir {
					continue
				}

				if filepath.Ext(depFile) != ".go" {
					continue
				}

				if (!sameDir || isTestFile) && hasExportIndex && usedSymbols != nil && len(usedSymbols) > 0 {
					if !fileDefinesAnyUsedSymbol(depFile, usedSymbols, exportIndex) {
						continue
					}
				}

				projectImports = append(projectImports, depFile)
			}
		}
	}

	return projectImports, nil
}

func fileDefinesAnyUsedSymbol(depFile string, usedSymbols map[string]bool, exportIndex GoPackageExportIndex) bool {
	for symbol := range usedSymbols {
		if definingFiles, ok := exportIndex[symbol]; ok {
			for _, defFile := range definingFiles {
				if defFile == depFile {
					return true
				}
			}
		}
	}

	return false
}

func AddGoIntraPackageDependencies(
	graph graphlib.Graph[string, string],
	goFiles []string,
	contentReader vcs.ContentReader,
) error {
	if len(goFiles) == 0 {
		return nil
	}

	intraDeps, err := BuildIntraPackageDependencies(goFiles, vcs.ContentReader(contentReader))
	if err != nil {
		return err
	}

	for file, deps := range intraDeps {
		if _, err := graph.Vertex(file); err != nil {
			continue
		}
		for _, dep := range deps {
			if _, err := graph.Vertex(dep); err != nil {
				continue
			}
			if err := graph.AddEdge(file, dep); err != nil && !errors.Is(err, graphlib.ErrEdgeAlreadyExists) {
				return err
			}
		}
	}

	return nil
}

// resolveGoImportPath resolves a Go import path to an absolute file path
// The contentReader is used to read go.mod content
func resolveGoImportPath(sourceFile, importPath string, contentReader vcs.ContentReader) string {
	// For Go files, we need to find the module root and resolve the import
	// This is a simplified version that assumes the project follows standard Go module structure

	// Find the go.mod file by walking up from the source file
	moduleRoot := findModuleRoot(filepath.Dir(sourceFile))
	if moduleRoot == "" {
		// If no module root found, return empty string
		return ""
	}

	// Get the module name from go.mod using the content reader
	moduleName := getModuleName(moduleRoot, contentReader)
	if moduleName == "" {
		return ""
	}

	// Check if the import path starts with the module name
	if !strings.HasPrefix(importPath, moduleName) {
		// Not an internal import relative to this module
		return ""
	}

	// Remove module name prefix to get relative path
	relativePath := strings.TrimPrefix(importPath, moduleName+"/")

	// Construct absolute path
	absPath := filepath.Join(moduleRoot, relativePath)

	// For Go, we don't add .go extension here because imports refer to packages (directories)
	// We'll need to look for any .go file in that directory
	// For now, we'll return the directory path
	return filepath.Clean(absPath)
}

// findModuleRoot walks up the directory tree to find the go.mod file
func findModuleRoot(startDir string) string {
	dir := startDir
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err != nil {
			// keep walking up the tree
		} else {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root directory
			return ""
		}
		dir = parent
	}
}

// getModuleName reads the module name from go.mod using the content reader
func getModuleName(moduleRoot string, contentReader vcs.ContentReader) string {
	goModPath := filepath.Join(moduleRoot, "go.mod")
	content, err := contentReader(goModPath)
	if err != nil {
		return ""
	}

	// Parse the module name from the content
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module"))
		}
	}

	return ""
}

// resolveGoEmbedPath resolves a Go embed pattern to an absolute file path
// Returns empty string if the pattern doesn't match any supplied file
func resolveGoEmbedPath(sourceFile, pattern string, suppliedFiles map[string]bool) string {
	// Get directory of source file
	sourceDir := filepath.Dir(sourceFile)

	// For simple file patterns (no glob characters), just resolve directly
	if !strings.ContainsAny(pattern, "*?[") {
		absPath := filepath.Join(sourceDir, pattern)
		absPath = filepath.Clean(absPath)

		// Check if this file is in the supplied files
		if suppliedFiles[absPath] {
			return absPath
		}
		return ""
	}

	// For glob patterns, we need to match against supplied files
	// Create a glob pattern with the full path
	globPattern := filepath.Join(sourceDir, pattern)

	// Check each supplied file to see if it matches the pattern
	for file := range suppliedFiles {
		matched, err := filepath.Match(globPattern, file)
		if err != nil {
			continue
		}
		if matched {
			// Return the first match (for simple cases)
			// TODO: For full glob support, return all matches
			return file
		}
	}

	return ""
}
