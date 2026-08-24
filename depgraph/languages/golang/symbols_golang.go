package golang

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

// GoSymbolInfo tracks symbols defined and referenced in a Go file
type GoSymbolInfo struct {
	FilePath        string
	Package         string
	Defined         map[string]bool // Symbols defined in this file
	Referenced      map[string]bool // Symbols referenced in this file
	MethodReceivers map[string]bool // Receiver type names of methods defined here
}

// GoExportInfo tracks exported symbols and import usage in a Go file
type GoExportInfo struct {
	FilePath      string
	Package       string
	Exports       map[string]bool            // Exported symbols (capitalized) defined in this file
	ImportAliases map[string]string          // Maps import path to alias used (or package name if no alias)
	DotImports    map[string]bool            // Tracks import paths imported via dot import
	QualifiedRefs map[string]map[string]bool // Maps package alias -> set of symbols accessed
	UnqualRefs    map[string]bool            // Unqualified refs, used for dot-import symbol filtering
}

// GoPackageExportIndex maps exported symbols to their defining files within a package directory
type GoPackageExportIndex map[string][]string // symbol name -> list of files defining it

// GoFileAnalysis holds all parse-derived metadata for a Go file.
type GoFileAnalysis struct {
	Imports    []GoImport
	Embeds     []GoEmbed
	SymbolInfo *GoSymbolInfo
	ExportInfo *GoExportInfo
}

// AnalyzeGoFileFromContent parses a Go file once and extracts import paths,
// embed directives, symbol metadata, and export/import-usage metadata.
func AnalyzeGoFileFromContent(filePath string, content []byte) ([]GoImport, []GoEmbed, *GoSymbolInfo, *GoExportInfo, error) {
	analysis, err := AnalyzeGoFileDetailsFromContent(filePath, content)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return analysis.Imports, analysis.Embeds, analysis.SymbolInfo, analysis.ExportInfo, nil
}

// AnalyzeGoFileDetailsFromContent parses a Go file once and returns a reusable analysis.
func AnalyzeGoFileDetailsFromContent(filePath string, content []byte) (*GoFileAnalysis, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	imports := make([]GoImport, 0, len(node.Imports))
	for _, imp := range node.Imports {
		importPath := strings.Trim(imp.Path.Value, "\"")
		imports = append(imports, classifyGoImport(importPath))
	}

	var embeds []GoEmbed
	for _, group := range node.Comments {
		for _, comment := range group.List {
			content := strings.TrimSpace(comment.Text)
			if !strings.HasPrefix(content, "//go:embed ") {
				continue
			}
			patterns := strings.TrimPrefix(content, "//go:embed ")
			for _, pattern := range strings.Fields(patterns) {
				pattern = strings.TrimPrefix(pattern, "all:")
				embeds = append(embeds, GoEmbed{Pattern: pattern})
			}
		}
	}

	exportInfo, err := extractExportInfoFromAST(filePath, node)
	if err != nil {
		return nil, err
	}

	symbolInfo, err := extractSymbolsFromAST(filePath, node)
	if err != nil {
		return nil, err
	}

	return &GoFileAnalysis{
		Imports:    imports,
		Embeds:     embeds,
		SymbolInfo: symbolInfo,
		ExportInfo: exportInfo,
	}, nil
}

// ExtractGoSymbols analyzes a Go file and extracts defined and referenced symbols
func ExtractGoSymbols(filePath string) (*GoSymbolInfo, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	return extractSymbolsFromAST(filePath, node)
}

// ExtractGoSymbolsFromContent analyzes Go source code and extracts defined and referenced symbols
func ExtractGoSymbolsFromContent(filePath string, content []byte) (*GoSymbolInfo, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	return extractSymbolsFromAST(filePath, node)
}

// receiverTypeName returns the bare type name of a method receiver,
// unwrapping a pointer receiver and dropping generic type parameters:
//
//	(f *dotFormatter)  -> "dotFormatter"
//	(f dotFormatter)   -> "dotFormatter"
//	(s *Stack[T])      -> "Stack"
//
// Returns "" if the receiver shape isn't a plain (possibly generic) type name.
func receiverTypeName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}
	expr := recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	switch e := expr.(type) {
	case *ast.IndexExpr:
		expr = e.X
	case *ast.IndexListExpr:
		expr = e.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// extractSymbolsFromAST extracts symbols from a parsed AST
func extractSymbolsFromAST(filePath string, node *ast.File) (*GoSymbolInfo, error) {

	info := &GoSymbolInfo{
		FilePath:        filePath,
		Package:         node.Name.Name,
		Defined:         make(map[string]bool),
		Referenced:      make(map[string]bool),
		MethodReceivers: make(map[string]bool),
	}

	// Extract defined symbols (top-level declarations)
	for _, decl := range node.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			// Only track top-level functions, not methods, as defined symbols.
			// Methods are scoped to their receiver type and don't create
			// package-level symbol dependencies (e.g., DOTFormatter.Format is
			// different from JSONFormatter.Format, even though both are named
			// "Format"). We still record the receiver type name, so the file
			// that defines the type can be linked to the files that add methods
			// to it (see method-ownership edges in buildPackageDependencies).
			if d.Recv == nil {
				info.Defined[d.Name.Name] = true
			} else if recv := receiverTypeName(d.Recv); recv != "" {
				info.MethodReceivers[recv] = true
			}
		case *ast.GenDecl:
			// Type, const, var, or import
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					info.Defined[s.Name.Name] = true
				case *ast.ValueSpec:
					for _, name := range s.Names {
						info.Defined[name.Name] = true
					}
				}
			}
		}
	}

	// Build a set of built-in types and functions that should be ignored
	builtins := map[string]bool{
		// Built-in types
		"bool": true, "byte": true, "complex64": true, "complex128": true,
		"error": true, "float32": true, "float64": true, "int": true,
		"int8": true, "int16": true, "int32": true, "int64": true,
		"rune": true, "string": true, "uint": true, "uint8": true,
		"uint16": true, "uint32": true, "uint64": true, "uintptr": true,
		// Built-in constants
		"true": true, "false": true, "iota": true, "nil": true,
		// Built-in functions
		"append": true, "cap": true, "close": true, "complex": true,
		"copy": true, "delete": true, "imag": true, "len": true,
		"make": true, "new": true, "panic": true, "print": true,
		"println": true, "real": true, "recover": true,
		// Special functions that don't create dependencies
		"init": true, "main": true,
	}

	// A SelectorExpr's Sel field (e.g. "New" in "errors.New") is itself an
	// *ast.Ident, so ast.Inspect's default recursion visits it a second time
	// as a bare identifier. Track those positions up front so the Ident case
	// below can skip them - otherwise a qualified call like errors.New gets
	// recorded as a reference to "New" with the "errors." qualifier silently
	// dropped, which can collide with an unrelated same-package New (see
	// CLR-73: errors.New falsely resolving to appstate.New).
	selectorFields := make(map[*ast.Ident]bool)
	ast.Inspect(node, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			selectorFields[sel.Sel] = true
		}
		return true
	})

	// Extract referenced symbols - only track identifiers that could be package-level symbols
	// Filter out built-ins, package names, and locally defined symbols
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Ident:
			if selectorFields[x] {
				return true
			}
			// Only track identifiers that:
			// 1. Don't have a local object (x.Obj == nil) - meaning they might be from another file
			// 2. Are not the blank identifier
			// 3. Are not the package name
			// 4. Are not built-in types/functions/constants (including init/main)
			// 5. Are not already defined in this file (checked via x.Obj == nil)
			if x.Obj == nil && x.Name != "_" && x.Name != info.Package && !builtins[x.Name] {
				info.Referenced[x.Name] = true
			}
		case *ast.SelectorExpr:
			// For qualified identifiers like fmt.Println, we only care about
			// package-local references, not external packages
			if ident, ok := x.X.(*ast.Ident); ok {
				// This is a selector like x.Field - track x only if it could be a package-level symbol
				if ident.Obj == nil && ident.Name != info.Package && !builtins[ident.Name] {
					info.Referenced[ident.Name] = true
				}
			}
		}
		return true
	})

	return info, nil
}

// ExtractGoExportInfo analyzes a Go file and extracts exported symbols and import usage
func ExtractGoExportInfo(filePath string) (*GoExportInfo, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	return extractExportInfoFromAST(filePath, node)
}

// ExtractGoExportInfoFromContent analyzes Go source code and extracts exported symbols and import usage
func ExtractGoExportInfoFromContent(filePath string, content []byte) (*GoExportInfo, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	return extractExportInfoFromAST(filePath, node)
}

// extractExportInfoFromAST extracts export information from a parsed AST
func extractExportInfoFromAST(filePath string, node *ast.File) (*GoExportInfo, error) {
	info := &GoExportInfo{
		FilePath:      filePath,
		Package:       node.Name.Name,
		Exports:       make(map[string]bool),
		ImportAliases: make(map[string]string),
		DotImports:    make(map[string]bool),
		QualifiedRefs: make(map[string]map[string]bool),
		UnqualRefs:    make(map[string]bool),
	}

	// Extract import aliases
	for _, imp := range node.Imports {
		importPath := strings.Trim(imp.Path.Value, "\"")

		// Determine the alias (explicit or derived from package path)
		var alias string
		if imp.Name != nil {
			alias = imp.Name.Name
			if alias == "." {
				info.DotImports[importPath] = true
				continue
			}
			if alias == "_" {
				// Dot imports or blank imports - skip for now
				continue
			}
		} else {
			// Use last component of import path as alias
			parts := strings.Split(importPath, "/")
			alias = parts[len(parts)-1]
		}

		info.ImportAliases[importPath] = alias
	}

	// Extract exported symbols (capitalized top-level declarations)
	for _, decl := range node.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name.IsExported() {
				if d.Recv == nil {
					// Exported top-level function
					info.Exports[d.Name.Name] = true
				} else {
					// Exported method - track separately if needed
					info.Exports[d.Name.Name] = true
				}
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if s.Name.IsExported() {
						info.Exports[s.Name.Name] = true
					}
				case *ast.ValueSpec:
					for _, name := range s.Names {
						if name.IsExported() {
							info.Exports[name.Name] = true
						}
					}
				}
			}
		}
	}

	// Build reverse map from alias to import path for quick lookup
	aliasToPath := make(map[string]string)
	for path, alias := range info.ImportAliases {
		aliasToPath[alias] = path
	}

	// Extract qualified references (e.g., formatters.NewFormatter)
	ast.Inspect(node, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			// Check if the X is an identifier (package alias)
			if ident, ok := sel.X.(*ast.Ident); ok {
				alias := ident.Name
				// Check if this alias is an imported package (not a local variable)
				if _, isImport := aliasToPath[alias]; isImport {
					// This is a qualified reference to an imported package
					if info.QualifiedRefs[alias] == nil {
						info.QualifiedRefs[alias] = make(map[string]bool)
					}
					info.QualifiedRefs[alias][sel.Sel.Name] = true
				}
			}
		}
		return true
	})

	builtins := map[string]bool{
		"bool": true, "byte": true, "complex64": true, "complex128": true,
		"error": true, "float32": true, "float64": true, "int": true,
		"int8": true, "int16": true, "int32": true, "int64": true,
		"rune": true, "string": true, "uint": true, "uint8": true,
		"uint16": true, "uint32": true, "uint64": true, "uintptr": true,
		"true": true, "false": true, "iota": true, "nil": true,
		"append": true, "cap": true, "close": true, "complex": true,
		"copy": true, "delete": true, "imag": true, "len": true,
		"make": true, "new": true, "panic": true, "print": true,
		"println": true, "real": true, "recover": true,
		"init": true, "main": true,
	}

	// Extract unqualified references used for dot-import resolution.
	ast.Inspect(node, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		if ident.Obj != nil {
			return true
		}
		if ident.Name == "_" || ident.Name == info.Package || builtins[ident.Name] {
			return true
		}
		info.UnqualRefs[ident.Name] = true
		return true
	})

	return info, nil
}

// BuildPackageExportIndex builds an index of exported symbols for files in a package directory.
// The contentReader function is used to read file contents, allowing the caller to control
// whether files are read from the filesystem, a git commit, or another source.
func BuildPackageExportIndex(filePaths []string, contentReader vcs.ContentReader) (GoPackageExportIndex, error) {
	index := make(GoPackageExportIndex)

	for _, filePath := range filePaths {
		// Skip test files for export index
		if strings.HasSuffix(filePath, "_test.go") {
			continue
		}

		content, err := contentReader(filePath)
		if err != nil {
			continue
		}

		info, err := ExtractGoExportInfoFromContent(filePath, content)
		if err != nil {
			continue
		}

		// Add exported symbols to index
		for symbol := range info.Exports {
			index[symbol] = append(index[symbol], filePath)
		}
	}

	return index, nil
}

// GetUsedSymbolsFromPackage extracts which symbols from a specific import path are actually used
func GetUsedSymbolsFromPackage(exportInfo *GoExportInfo, importPath string) map[string]bool {
	if exportInfo.DotImports[importPath] {
		return exportInfo.UnqualRefs
	}

	alias, ok := exportInfo.ImportAliases[importPath]
	if !ok {
		return nil
	}

	return exportInfo.QualifiedRefs[alias]
}

// BuildIntraPackageDependencies builds dependencies between files in the same Go package.
// The contentReader function is used to read file contents, allowing the caller to control
// whether files are read from the filesystem, a git commit, or another source.
func BuildIntraPackageDependencies(filePaths []string, contentReader vcs.ContentReader) (map[string][]string, error) {
	return BuildIntraPackageDependenciesWithSymbolLookup(filePaths, contentReader, nil)
}

// BuildIntraPackageDependenciesWithSymbolLookup builds dependencies between files
// in the same Go package and optionally reuses caller-provided symbol metadata.
func BuildIntraPackageDependenciesWithSymbolLookup(
	filePaths []string,
	contentReader vcs.ContentReader,
	symbolLookup func(filePath string) (*GoSymbolInfo, bool),
) (map[string][]string, error) {
	// Group files by package
	packageFiles := make(map[string][]string)
	for _, filePath := range filePaths {
		if filepath.Ext(filePath) != ".go" {
			continue
		}

		absPath, err := filepath.Abs(filePath)
		if err != nil {
			continue
		}

		// Get package directory
		pkgDir := filepath.Dir(absPath)
		packageFiles[pkgDir] = append(packageFiles[pkgDir], absPath)
	}

	packageGroups := make([][]string, 0, len(packageFiles))
	for _, files := range packageFiles {
		packageGroups = append(packageGroups, files)
	}

	dependencies := make(map[string][]string)
	workerCount := runtime.GOMAXPROCS(0)
	if workerCount < 1 {
		workerCount = 1
	}
	if workerCount > len(packageGroups) {
		workerCount = len(packageGroups)
	}
	if workerCount < 1 {
		workerCount = 1
	}

	jobs := make(chan []string)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for files := range jobs {
				packageDeps := buildPackageDependencies(files, contentReader, symbolLookup)
				mu.Lock()
				for file, deps := range packageDeps {
					dependencies[file] = deps
				}
				mu.Unlock()
			}
		}()
	}

	for _, files := range packageGroups {
		jobs <- files
	}
	close(jobs)
	wg.Wait()

	return dependencies, nil
}

func buildPackageDependencies(
	files []string,
	contentReader vcs.ContentReader,
	symbolLookup func(filePath string) (*GoSymbolInfo, bool),
) map[string][]string {
	// Separate test and non-test files.
	var testFiles, nonTestFiles []*GoSymbolInfo

	// Extract symbols from all files in the package.
	for _, file := range files {
		var info *GoSymbolInfo
		if symbolLookup != nil {
			if cached, ok := symbolLookup(file); ok && cached != nil {
				info = cached
			}
		}
		if info == nil {
			content, err := contentReader(file)
			if err != nil {
				// Skip files that can't be read.
				continue
			}
			parsed, err := ExtractGoSymbolsFromContent(file, content)
			if err != nil {
				// Skip files that can't be parsed.
				continue
			}
			info = parsed
		}

		if strings.HasSuffix(file, "_test.go") {
			testFiles = append(testFiles, info)
		} else {
			nonTestFiles = append(nonTestFiles, info)
		}
	}

	// Build symbol maps separately for test and non-test files.
	nonTestSymbolToFiles := make(map[string][]string)
	for _, info := range nonTestFiles {
		for symbol := range info.Defined {
			nonTestSymbolToFiles[symbol] = append(nonTestSymbolToFiles[symbol], info.FilePath)
		}
	}

	allSymbolToFiles := make(map[string][]string)
	for _, info := range append(nonTestFiles, testFiles...) {
		for symbol := range info.Defined {
			allSymbolToFiles[symbol] = append(allSymbolToFiles[symbol], info.FilePath)
		}
	}

	// Accumulate each file's dependency set, then flatten once at the end, so
	// reference edges and method-ownership edges can both contribute.
	depSets := make(map[string]map[string]bool)
	for _, info := range nonTestFiles {
		depSets[info.FilePath] = make(map[string]bool)
	}
	for _, info := range testFiles {
		depSets[info.FilePath] = make(map[string]bool)
	}

	// Reference edges. Non-test files may only depend on other non-test files;
	// test files may depend on any file (test or non-test).
	addReferenceEdges := func(info *GoSymbolInfo, symbolToFiles map[string][]string) {
		deps := depSets[info.FilePath]
		for symbol := range info.Referenced {
			for _, defFile := range symbolToFiles[symbol] {
				if defFile != info.FilePath {
					deps[defFile] = true
				}
			}
		}
	}
	for _, info := range nonTestFiles {
		addReferenceEdges(info, nonTestSymbolToFiles)
	}
	for _, info := range testFiles {
		addReferenceEdges(info, allSymbolToFiles)
	}

	// Method-ownership edges. A type and its methods may live in different
	// files of the same package. A method file defines no package-level symbol
	// that anyone references, so the reference pass never edges into it — its
	// only edge runs method -> type (the receiver reference). Add the reverse,
	// type -> method, so reaching the type reaches the behaviour attached to
	// it. Without this, a constructor-reached type drops all of its
	// out-of-file methods (and their private helpers) from the graph.
	// Only non-test method files, linked to non-test type files: a test file
	// may define methods on a production type, but production must never gain
	// an edge to a test file. (Test files already get their method -> type
	// reference edge from the reference pass above.)
	for _, info := range nonTestFiles {
		for recvType := range info.MethodReceivers {
			for _, typeFile := range nonTestSymbolToFiles[recvType] {
				if typeFile != info.FilePath {
					depSets[typeFile][info.FilePath] = true
				}
			}
		}
	}

	dependencies := make(map[string][]string, len(depSets))
	for file, deps := range depSets {
		dependencies[file] = dependencySetToSlice(deps)
	}

	return dependencies
}

func dependencySetToSlice(deps map[string]bool) []string {
	depSlice := make([]string, 0, len(deps))
	for dep := range deps {
		depSlice = append(depSlice, dep)
	}
	return depSlice
}
