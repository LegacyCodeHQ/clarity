package zig

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LegacyCodeHQ/clarity/vcs"
	sitter "github.com/smacker/go-tree-sitter"
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
	declarations := parseZigConstDeclarations(sourceCode)
	importAliases := r.resolveImportAliases(sourceFile, declarations.imports)
	qualifiedAliases := declarations.qualified

	var resolved []string
	for _, ref := range declarations.directReferences {
		path, ok := r.resolveQualifiedReference(sourceFile, ref, importAliases, nil)
		if !ok || path == sourceFile || !r.suppliedFiles[path] {
			continue
		}
		resolved = append(resolved, path)
	}

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

	exports := r.resolveImportAliases(filePath, parseZigConstDeclarations(content).imports)
	r.exportsCache[filePath] = exports
	return exports
}

func (r *zigReexportResolver) resolveImportAliases(sourceFile string, importAliases map[string]string) map[string]string {
	aliases := make(map[string]string)
	for name, path := range importAliases {
		resolved, ok := r.resolveImportAliasPath(sourceFile, path)
		if !ok {
			continue
		}
		aliases[name] = resolved
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
		candidates := []string{
			filepath.Join(dir, "std", "std.zig"),
			filepath.Join(dir, "lib", "std", "std.zig"),
		}
		for _, candidate := range candidates {
			if _, err := r.contentReader(candidate); err == nil {
				return filepath.Clean(candidate), true
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
	}
}

func expandQualifiedAlias(segments []string, aliases map[string]string) []string {
	if len(aliases) == 0 {
		return segments
	}

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

type zigConstDeclarations struct {
	imports            map[string]string
	qualified          map[string]string
	directReferences   []string
	directReferenceSet map[string]bool
}

func parseZigConstDeclarations(sourceCode []byte) zigConstDeclarations {
	declarations := zigConstDeclarations{
		imports:            make(map[string]string),
		qualified:          make(map[string]string),
		directReferenceSet: make(map[string]bool),
	}

	parser := sitter.NewParser()
	parser.SetLanguage(zigLanguage)

	tree, err := parser.ParseCtx(context.Background(), nil, sourceCode)
	if err != nil {
		return declarations
	}
	defer tree.Close()

	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}
		if node.Type() == "variable_declaration" {
			name, value, ok := zigConstDeclarationParts(node, sourceCode)
			if ok {
				if importPath, ok := zigImportNodePath(value, sourceCode); ok {
					declarations.imports[name] = importPath
				} else if qualifiedPath, ok := zigQualifiedPathNode(value, sourceCode); ok {
					declarations.qualified[name] = qualifiedPath
				}
			}
		}

		if node.Type() == "field_expression" && !hasZigFieldExpressionParent(node) {
			qualifiedPath, ok := zigQualifiedPathNode(node, sourceCode)
			if ok && isDirectZigQualifiedReference(qualifiedPath) && !declarations.directReferenceSet[qualifiedPath] {
				declarations.directReferenceSet[qualifiedPath] = true
				declarations.directReferences = append(declarations.directReferences, qualifiedPath)
			}
		}

		for i := 0; i < int(node.NamedChildCount()); i++ {
			walk(node.NamedChild(i))
		}
	}

	walk(tree.RootNode())
	return declarations
}

func isDirectZigQualifiedReference(path string) bool {
	return strings.Count(path, ".") == 1
}

func hasZigFieldExpressionParent(node *sitter.Node) bool {
	parent := node.Parent()
	return parent != nil && parent.Type() == "field_expression"
}

func zigConstDeclarationParts(node *sitter.Node, sourceCode []byte) (string, *sitter.Node, bool) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child.Type() == "identifier" {
				nameNode = child
				break
			}
		}
	}
	if nameNode == nil {
		return "", nil, false
	}

	header := strings.TrimSpace(string(sourceCode[node.StartByte():nameNode.StartByte()]))
	if !strings.Contains(header, "const") || strings.Contains(header, "var") {
		return "", nil, false
	}

	var valueNode *sitter.Node
	for i := int(node.NamedChildCount()) - 1; i >= 0; i-- {
		child := node.NamedChild(i)
		if child == nil || child.Equal(nameNode) || child.Type() == "identifier" && child.Content(sourceCode) == nameNode.Content(sourceCode) {
			continue
		}
		valueNode = child
		break
	}
	if valueNode == nil {
		return "", nil, false
	}

	return nameNode.Content(sourceCode), valueNode, true
}

func zigQualifiedPathNode(node *sitter.Node, sourceCode []byte) (string, bool) {
	segments, ok := zigQualifiedPathSegments(node, sourceCode)
	if !ok || len(segments) < 2 {
		return "", false
	}
	return strings.Join(segments, "."), true
}

func zigQualifiedPathSegments(node *sitter.Node, sourceCode []byte) ([]string, bool) {
	if node == nil {
		return nil, false
	}

	switch node.Type() {
	case "identifier":
		return []string{node.Content(sourceCode)}, true
	case "pointer_type":
		for i := int(node.NamedChildCount()) - 1; i >= 0; i-- {
			if segments, ok := zigQualifiedPathSegments(node.NamedChild(i), sourceCode); ok {
				return segments, true
			}
		}
		return nil, false
	case "field_expression":
		objectSegments, ok := zigQualifiedPathSegments(node.ChildByFieldName("object"), sourceCode)
		if !ok {
			return nil, false
		}
		member := node.ChildByFieldName("member")
		if member == nil || member.Type() != "identifier" {
			return nil, false
		}
		return append(objectSegments, member.Content(sourceCode)), true
	default:
		return nil, false
	}
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
