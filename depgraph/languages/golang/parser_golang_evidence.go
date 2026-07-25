package golang

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

// GoDeclarationLocation is one package-level declaration.
type GoDeclarationLocation struct {
	Name    string
	Kind    string
	Package string
	Line    int
}

// GoReferenceLocation is one AST-backed dependency reference.
type GoReferenceLocation struct {
	Name       string
	Qualifier  string
	ImportPath string
	Kind       string
	Line       int
}

// ParseGoDeclarationLocations returns package-level declarations with their
// 1-based source lines.
func ParseGoDeclarationLocations(filename string, source []byte) []GoDeclarationLocation {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, source, 0)
	if err != nil {
		return nil
	}
	result := []GoDeclarationLocation{}
	for _, declaration := range file.Decls {
		switch item := declaration.(type) {
		case *ast.FuncDecl:
			if item.Recv == nil {
				result = append(result, GoDeclarationLocation{
					Name: item.Name.Name, Kind: "function", Package: file.Name.Name,
					Line: fset.Position(item.Name.Pos()).Line,
				})
			}
		case *ast.GenDecl:
			for _, spec := range item.Specs {
				switch value := spec.(type) {
				case *ast.TypeSpec:
					result = append(result, GoDeclarationLocation{
						Name: value.Name.Name, Kind: "type", Package: file.Name.Name,
						Line: fset.Position(value.Name.Pos()).Line,
					})
				case *ast.ValueSpec:
					for _, name := range value.Names {
						result = append(result, GoDeclarationLocation{
							Name: name.Name, Kind: strings.ToLower(item.Tok.String()),
							Package: file.Name.Name, Line: fset.Position(name.Pos()).Line,
						})
					}
				}
			}
		}
	}
	return result
}

// ExtractGoReferenceLocations returns imports, calls, type embeddings, and
// identifier references with their 1-based source lines.
func ExtractGoReferenceLocations(filename string, source []byte) (string, []GoReferenceLocation) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, source, 0)
	if err != nil {
		return "", nil
	}

	result := []GoReferenceLocation{}
	importsByAlias := make(map[string]string)
	for _, item := range file.Imports {
		path, unquoteErr := strconv.Unquote(item.Path.Value)
		if unquoteErr != nil {
			continue
		}
		alias := importAlias(path, item.Name)
		importsByAlias[alias] = path
		result = append(result, GoReferenceLocation{
			Name: alias, ImportPath: path, Kind: "import",
			Line: fset.Position(item.Path.Pos()).Line,
		})
	}

	specialKinds := make(map[token.Pos]string)
	qualifiers := make(map[token.Pos]string)
	skip := make(map[token.Pos]bool)
	ast.Inspect(file, func(node ast.Node) bool {
		switch item := node.(type) {
		case *ast.CallExpr:
			switch function := item.Fun.(type) {
			case *ast.Ident:
				specialKinds[function.Pos()] = "call"
			case *ast.SelectorExpr:
				specialKinds[function.Sel.Pos()] = "call"
			}
		case *ast.SelectorExpr:
			if qualifier, ok := item.X.(*ast.Ident); ok {
				qualifiers[item.Sel.Pos()] = qualifier.Name
				skip[qualifier.Pos()] = true
			}
		case *ast.StructType:
			for _, field := range item.Fields.List {
				if len(field.Names) > 0 {
					continue
				}
				switch embedded := field.Type.(type) {
				case *ast.Ident:
					specialKinds[embedded.Pos()] = "inheritance"
				case *ast.SelectorExpr:
					specialKinds[embedded.Sel.Pos()] = "inheritance"
				}
			}
		}
		return true
	})

	ast.Inspect(file, func(node ast.Node) bool {
		identifier, ok := node.(*ast.Ident)
		if !ok || identifier.Obj != nil || skip[identifier.Pos()] ||
			identifier.Name == "_" || identifier.Name == file.Name.Name {
			return true
		}
		if _, imported := importsByAlias[identifier.Name]; imported {
			return true
		}
		kind := specialKinds[identifier.Pos()]
		if kind == "" {
			kind = "symbol-reference"
		}
		qualifier := qualifiers[identifier.Pos()]
		result = append(result, GoReferenceLocation{
			Name: identifier.Name, Qualifier: qualifier,
			ImportPath: importsByAlias[qualifier], Kind: kind,
			Line: fset.Position(identifier.Pos()).Line,
		})
		return true
	})
	return file.Name.Name, result
}

func importAlias(path string, name *ast.Ident) string {
	if name != nil {
		return name.Name
	}
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")
	return parts[len(parts)-1]
}
