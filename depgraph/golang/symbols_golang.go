package golang

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

// GoSymbolInfo tracks symbols defined and referenced in a Go file
type GoSymbolInfo struct {
	FilePath   string
	Package    string
	Defined    map[string]bool // Symbols defined in this file
	Referenced map[string]bool // Symbols referenced in this file
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

// extractSymbolsFromAST extracts symbols from a parsed AST
func extractSymbolsFromAST(filePath string, node *ast.File) (*GoSymbolInfo, error) {

	info := &GoSymbolInfo{
		FilePath:   filePath,
		Package:    node.Name.Name,
		Defined:    make(map[string]bool),
		Referenced: make(map[string]bool),
	}

	// Extract defined symbols (top-level declarations)
	for _, decl := range node.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			// Only track top-level functions, not methods
			// Methods are scoped to their receiver type and don't create
			// package-level dependencies (e.g., DOTFormatter.Format is different
			// from JSONFormatter.Format, even though both are named "Format")
			if d.Recv == nil {
				info.Defined[d.Name.Name] = true
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

	// Extract referenced symbols - only track identifiers that could be package-level symbols
	// Filter out built-ins, package names, and locally defined symbols
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Ident:
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

	// Build dependencies for each package
	dependencies := make(map[string][]string)

	for _, files := range packageFiles {
		// Separate test and non-test files
		var testFiles, nonTestFiles []*GoSymbolInfo

		// Extract symbols from all files in the package
		for _, file := range files {
			content, err := contentReader(file)
			if err != nil {
				// Skip files that can't be read
				continue
			}

			info, err := ExtractGoSymbolsFromContent(file, content)
			if err != nil {
				// Skip files that can't be parsed
				continue
			}

			if strings.HasSuffix(file, "_test.go") {
				testFiles = append(testFiles, info)
			} else {
				nonTestFiles = append(nonTestFiles, info)
			}
		}

		// Build symbol maps separately for test and non-test files
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

		// For non-test files, only allow dependencies on other non-test files
		for _, info := range nonTestFiles {
			deps := make(map[string]bool)
			for symbol := range info.Referenced {
				// Only look in non-test files
				if definingFiles, ok := nonTestSymbolToFiles[symbol]; ok {
					for _, defFile := range definingFiles {
						// Don't add self-dependencies
						if defFile != info.FilePath {
							deps[defFile] = true
						}
					}
				}
			}

			// Convert set to slice
			depSlice := make([]string, 0, len(deps))
			for dep := range deps {
				depSlice = append(depSlice, dep)
			}
			dependencies[info.FilePath] = depSlice
		}

		// For test files, allow dependencies on all files (test and non-test)
		for _, info := range testFiles {
			deps := make(map[string]bool)
			for symbol := range info.Referenced {
				// Look in all files
				if definingFiles, ok := allSymbolToFiles[symbol]; ok {
					for _, defFile := range definingFiles {
						// Don't add self-dependencies
						if defFile != info.FilePath {
							deps[defFile] = true
						}
					}
				}
			}

			// Convert set to slice
			depSlice := make([]string, 0, len(deps))
			for dep := range deps {
				depSlice = append(depSlice, dep)
			}
			dependencies[info.FilePath] = depSlice
		}
	}

	return dependencies, nil
}
