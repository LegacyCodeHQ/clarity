package swift

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// SwiftReferenceLocation is one source-level symbol reference.
type SwiftReferenceLocation struct {
	Name      string
	Qualifier string
	Line      int
}

// SwiftDeclarationLocation is one cross-file-visible declaration.
type SwiftDeclarationLocation struct {
	Name  string
	Owner string
	Kind  string
	Line  int
}

// ExtractSwiftReferenceLocations returns the references used by dependency
// resolution with their 1-based source lines.
func ExtractSwiftReferenceLocations(sourceCode []byte) []SwiftReferenceLocation {
	tree, err := parseSwift(sourceCode)
	if err != nil {
		return nil
	}
	defer tree.Close()

	declared := collectSwiftDeclaredNames(tree.RootNode(), sourceCode)
	var result []SwiftReferenceLocation
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil || n.Type() == "import_declaration" {
			return
		}
		if isSwiftReferenceNode(n) &&
			!isQualifiedSwiftMemberName(n, sourceCode) &&
			!isSwiftDeclarationName(n) &&
			!isImplicitSwiftCatchError(n, sourceCode) {
			name := strings.TrimSpace(n.Content(sourceCode))
			if name != "" && !declared[name] {
				result = append(result, SwiftReferenceLocation{
					Name: name,
					Line: int(n.StartPoint().Row) + 1,
				})
			}
		}
		if n.Type() == "navigation_expression" {
			target := n.ChildByFieldName("target")
			suffix := n.ChildByFieldName("suffix")
			memberNode := firstDescendant(suffix, "simple_identifier", "identifier")
			if target != nil && memberNode != nil {
				qualifier := strings.TrimSpace(target.Content(sourceCode))
				if qualifier == "Self" || isLikelySwiftTypeName(qualifier) {
					result = append(result, SwiftReferenceLocation{
						Name:      strings.TrimSpace(memberNode.Content(sourceCode)),
						Qualifier: qualifier,
						Line:      int(memberNode.StartPoint().Row) + 1,
					})
				}
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(tree.RootNode())
	return result
}

// ParseSwiftDeclarationLocations returns declarations eligible for cross-file
// resolution with their 1-based source lines.
func ParseSwiftDeclarationLocations(sourceCode []byte) []SwiftDeclarationLocation {
	tree, err := parseSwift(sourceCode)
	if err != nil {
		return nil
	}
	defer tree.Close()

	var result []SwiftDeclarationLocation
	root := tree.RootNode()
	for i := 0; i < int(root.ChildCount()); i++ {
		node := root.Child(i)
		if node == nil || isPrivateSwiftDeclaration(node, sourceCode) {
			continue
		}
		if isSwiftExtensionDeclaration(node) {
			owner := extractSwiftDeclarationName(node, sourceCode)
			body := firstDirectChildOfType(node, "class_body", "enum_class_body")
			if owner == "" || body == nil {
				continue
			}
			for j := 0; j < int(body.ChildCount()); j++ {
				member := body.Child(j)
				if member == nil || !isSwiftDeclarationNode(member) ||
					isPrivateSwiftDeclaration(member, sourceCode) {
					continue
				}
				for _, name := range swiftDeclarationNames(member, sourceCode) {
					result = append(result, SwiftDeclarationLocation{
						Name: name, Owner: owner, Kind: "swift-extension-member",
						Line: int(member.StartPoint().Row) + 1,
					})
				}
			}
			continue
		}
		if !isTopLevelSwiftSymbolDeclaration(node) {
			continue
		}
		for _, name := range swiftDeclarationNames(node, sourceCode) {
			result = append(result, SwiftDeclarationLocation{
				Name: name, Kind: "swift-symbol", Line: int(node.StartPoint().Row) + 1,
			})
		}
	}
	return result
}
