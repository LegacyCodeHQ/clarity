package zig

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

var (
	zigImportConstPattern    = regexp.MustCompile(`(?m)\b(?:pub\s+)?const\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*@import\s*\(\s*("(?:\\.|[^"\\])*")\s*\)`)
	zigQualifiedConstPattern = regexp.MustCompile(`(?m)\bconst\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)+)\s*;`)
)

func ResolveZigProjectImports(
	absPath string,
	filePath string,
	ext string,
	suppliedFiles map[string]bool,
	contentReader vcs.ContentReader,
) ([]string, error) {
	content, err := contentReader(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", absPath, err)
	}

	imports, err := ParseImports(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse imports in %s: %w", filePath, err)
	}

	projectImports := make([]string, 0, len(imports))
	for _, imp := range imports {
		resolvedPath, ok := resolveImportPath(absPath, imp.Path, ext, suppliedFiles)
		if ok {
			projectImports = append(projectImports, resolvedPath)
		}
	}

	reexportResolver := newZigReexportResolver(ext, suppliedFiles, contentReader)
	projectImports = append(projectImports, reexportResolver.resolveQualifiedReferences(absPath, content)...)

	return dedupePaths(projectImports), nil
}

func resolveImportPath(sourceFile, importPath, fileExt string, suppliedFiles map[string]bool) (string, bool) {
	if !strings.HasSuffix(importPath, fileExt) {
		return "", false
	}

	resolved := importPath
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(filepath.Dir(sourceFile), resolved)
	}
	resolved = filepath.Clean(resolved)

	if !suppliedFiles[resolved] {
		return "", false
	}

	return resolved, true
}

type zigReexportResolver struct {
	fileExt       string
	suppliedFiles map[string]bool
	contentReader vcs.ContentReader
	exportsCache  map[string]map[string]string
}

func newZigReexportResolver(fileExt string, suppliedFiles map[string]bool, contentReader vcs.ContentReader) *zigReexportResolver {
	return &zigReexportResolver{
		fileExt:       fileExt,
		suppliedFiles: suppliedFiles,
		contentReader: contentReader,
		exportsCache:  make(map[string]map[string]string),
	}
}

func (r *zigReexportResolver) resolveQualifiedReferences(sourceFile string, sourceCode []byte) []string {
	importAliases := r.parseZigImportAliases(sourceFile, sourceCode)
	qualifiedAliases := parseZigQualifiedAliases(sourceCode)

	var resolved []string
	for _, ref := range qualifiedAliases {
		path, ok := r.resolveQualifiedReference(sourceFile, ref, importAliases, qualifiedAliases)
		if !ok || path == sourceFile || !r.suppliedFiles[path] {
			continue
		}
		resolved = append(resolved, path)
	}

	return resolved
}

func (r *zigReexportResolver) resolveQualifiedReference(
	sourceFile string,
	ref string,
	importAliases map[string]string,
	qualifiedAliases map[string]string,
) (string, bool) {
	segments := strings.Split(ref, ".")
	if len(segments) < 2 {
		return "", false
	}

	segments = expandQualifiedAlias(segments, qualifiedAliases)
	if len(segments) < 2 {
		return "", false
	}

	currentFile, ok := importAliases[segments[0]]
	if !ok {
		return "", false
	}
	lastSuppliedFile := ""
	if r.suppliedFiles[currentFile] {
		lastSuppliedFile = currentFile
	}

	for _, segment := range segments[1:] {
		exports := r.exportsForFile(currentFile)
		nextFile, ok := exports[segment]
		if !ok {
			break
		}
		currentFile = nextFile
		if r.suppliedFiles[currentFile] {
			lastSuppliedFile = currentFile
		}
	}

	if lastSuppliedFile != "" && lastSuppliedFile != sourceFile {
		return lastSuppliedFile, true
	}
	return "", false
}

func (r *zigReexportResolver) exportsForFile(filePath string) map[string]string {
	if exports, ok := r.exportsCache[filePath]; ok {
		return exports
	}

	content, err := r.contentReader(filePath)
	if err != nil {
		r.exportsCache[filePath] = make(map[string]string)
		return r.exportsCache[filePath]
	}

	exports := r.parseZigImportAliases(filePath, content)
	r.exportsCache[filePath] = exports
	return exports
}

func (r *zigReexportResolver) parseZigImportAliases(sourceFile string, sourceCode []byte) map[string]string {
	aliases := make(map[string]string)
	for _, match := range zigImportConstPattern.FindAllSubmatch(sourceCode, -1) {
		if len(match) != 3 {
			continue
		}
		path, ok := cleanZigImportLiteral(string(match[2]))
		if !ok {
			continue
		}
		resolved, ok := r.resolveImportAliasPath(sourceFile, path)
		if !ok {
			continue
		}
		aliases[string(match[1])] = resolved
	}
	return aliases
}

func (r *zigReexportResolver) resolveImportAliasPath(sourceFile string, importPath string) (string, bool) {
	if strings.HasSuffix(importPath, r.fileExt) {
		return filepath.Clean(filepath.Join(filepath.Dir(sourceFile), importPath)), true
	}

	if importPath == "std" {
		return r.resolveStdImportPath(sourceFile)
	}

	return "", false
}

func (r *zigReexportResolver) resolveStdImportPath(sourceFile string) (string, bool) {
	for dir := filepath.Dir(sourceFile); ; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, "std", "std.zig")
		if _, err := r.contentReader(candidate); err == nil {
			return filepath.Clean(candidate), true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
	}
}

func parseZigQualifiedAliases(sourceCode []byte) map[string]string {
	aliases := make(map[string]string)
	for _, match := range zigQualifiedConstPattern.FindAllSubmatch(sourceCode, -1) {
		if len(match) != 3 {
			continue
		}
		aliases[string(match[1])] = string(match[2])
	}
	return aliases
}

func expandQualifiedAlias(segments []string, aliases map[string]string) []string {
	seen := make(map[string]bool)
	for len(segments) > 0 {
		alias := segments[0]
		expansion, ok := aliases[alias]
		if !ok || seen[alias] {
			return segments
		}
		seen[alias] = true
		expanded := strings.Split(expansion, ".")
		segments = append(expanded, segments[1:]...)
	}
	return segments
}

func cleanZigImportLiteral(raw string) (string, bool) {
	path, err := strconv.Unquote(raw)
	if err != nil || path == "" {
		return "", false
	}
	return path, true
}

func dedupePaths(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	deduped := make([]string, 0, len(paths))
	for _, path := range paths {
		if seen[path] {
			continue
		}
		seen[path] = true
		deduped = append(deduped, path)
	}
	return deduped
}
