// Package explain attaches source-level provenance to resolved file edges.
package explain

import (
	"bytes"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/golang"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/html"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/kotlin"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/markdown"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/rust"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/swift"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/typescript"
	"github.com/LegacyCodeHQ/clarity/vcs"
)

// AttachEvidence populates every edge's best-effort source-level provenance.
func AttachEvidence(graph *depgraph.FileDependencyGraph, reader vcs.ContentReader) {
	if graph == nil || reader == nil {
		return
	}
	supplied := make(map[string]bool, len(graph.Meta.Files))
	for file := range graph.Meta.Files {
		supplied[file] = true
	}
	for edge, metadata := range graph.Meta.Edges {
		if !metadata.InCycle {
			continue
		}
		switch filepath.Ext(edge.From) {
		case ".html", ".htm":
			metadata.Evidence = htmlEvidence(edge, supplied, reader)
		case ".kt", ".kts":
			metadata.Evidence = kotlinEvidence(edge, reader)
		case ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx", ".mts", ".cts":
			metadata.Evidence = ecmaScriptEvidence(edge, reader)
		case ".go":
			metadata.Evidence = goEvidence(edge, reader)
		case ".rs":
			metadata.Evidence = rustEvidence(edge, reader)
		case ".swift":
			metadata.Evidence = swiftEvidence(edge, reader)
		case ".md", ".markdown":
			metadata.Evidence = markdownEvidence(edge, supplied, reader)
		}
		if len(metadata.Evidence) == 0 {
			metadata.Evidence = []depgraph.DependencyEvidence{{
				Kind:            "resolved-dependency",
				Relationship:    depgraph.RelationshipResolvedDependency,
				ReferenceFile:   edge.From,
				DeclarationFile: edge.To,
				Confidence:      depgraph.EvidenceConfidenceMedium,
			}}
		}
		graph.Meta.Edges[edge] = metadata
	}
}

func htmlEvidence(
	edge depgraph.FileEdge,
	supplied map[string]bool,
	reader vcs.ContentReader,
) []depgraph.DependencyEvidence {
	source, err := reader(edge.From)
	if err != nil {
		return nil
	}
	var evidence []depgraph.DependencyEvidence
	for _, link := range html.ParseHTMLLinkLocations(source) {
		resolved := html.ResolveHTMLLinkPath(edge.From, link.Path, supplied)
		if len(resolved) != 1 || resolved[0] != edge.To {
			continue
		}
		evidence = append(evidence, depgraph.DependencyEvidence{
			Symbol:          link.Path,
			Kind:            "html-" + link.Element + "-" + link.Attribute,
			Relationship:    htmlRelationship(link, edge.To),
			ReferenceFile:   edge.From,
			ReferenceLine:   link.Line,
			DeclarationFile: edge.To,
			DeclarationLine: 1,
			Confidence:      depgraph.EvidenceConfidenceHigh,
		})
	}
	sortEvidence(evidence)
	return deduplicateEvidence(evidence)
}

func htmlRelationship(
	link html.HTMLLinkLocation,
	target string,
) depgraph.DependencyRelationship {
	switch link.Element {
	case "a", "area":
		return depgraph.RelationshipNavigation
	case "script":
		return depgraph.RelationshipScript
	case "img", "picture":
		return depgraph.RelationshipImage
	case "link":
		if strings.EqualFold(filepath.Ext(target), ".css") {
			return depgraph.RelationshipStylesheet
		}
		return depgraph.RelationshipEmbeddedResource
	case "iframe", "embed", "object", "source", "video", "audio":
		return depgraph.RelationshipEmbeddedResource
	default:
		switch strings.ToLower(filepath.Ext(target)) {
		case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".avif":
			return depgraph.RelationshipImage
		default:
			return depgraph.RelationshipEmbeddedResource
		}
	}
}

func kotlinEvidence(edge depgraph.FileEdge, reader vcs.ContentReader) []depgraph.DependencyEvidence {
	source, sourceErr := reader(edge.From)
	target, targetErr := reader(edge.To)
	if sourceErr != nil || targetErr != nil {
		return nil
	}
	declarations := kotlin.ParseKotlinDeclarationLocations(target)
	imports := kotlin.ParseKotlinImportLocations(source)
	sourcePackage := kotlin.ExtractPackageDeclaration(source)
	targetPackage := ""
	if len(declarations) > 0 {
		targetPackage = declarations[0].Package
	} else {
		targetPackage = kotlin.ExtractPackageDeclaration(target)
	}
	samePackage := sourcePackage != "" && sourcePackage == targetPackage
	localNames := make(map[string]string)
	var evidence []depgraph.DependencyEvidence
	for _, item := range imports {
		if !kotlinImportMatchesTarget(item, targetPackage, declarations) {
			continue
		}
		if item.Wildcard {
			for _, declaration := range declarations {
				localNames[declaration.Name] = declaration.Name
			}
		} else {
			localNames[item.Local] = item.Imported
		}
		declaration := matchingKotlinDeclaration(
			declarations, item.Imported, item.Local)
		evidence = append(evidence, depgraph.DependencyEvidence{
			Symbol:          item.Path,
			Kind:            "kotlin-import",
			Relationship:    depgraph.RelationshipImport,
			ReferenceFile:   edge.From,
			ReferenceLine:   item.Line,
			DeclarationFile: edge.To,
			DeclarationLine: declaration.Line,
			Confidence:      depgraph.EvidenceConfidenceHigh,
		})
	}
	if samePackage {
		for _, declaration := range declarations {
			localNames[declaration.Name] = declaration.Name
		}
	}
	if len(localNames) == 0 {
		return nil
	}
	for _, reference := range kotlin.ExtractKotlinReferenceLocations(source) {
		imported, ok := localNames[reference.Name]
		if !ok {
			continue
		}
		declaration := matchingKotlinDeclaration(
			declarations, imported, reference.Name)
		scope := "imported"
		if samePackage {
			scope = "same-package"
		}
		evidence = append(evidence, depgraph.DependencyEvidence{
			Symbol:          imported,
			Kind:            "kotlin-" + scope + "-" + reference.Kind,
			Relationship:    kotlinRelationship(reference.Kind, declaration.Kind),
			ReferenceFile:   edge.From,
			ReferenceLine:   reference.Line,
			DeclarationFile: edge.To,
			DeclarationLine: declaration.Line,
			Confidence:      depgraph.EvidenceConfidenceMedium,
		})
	}
	sortEvidence(evidence)
	return deduplicateEvidence(evidence)
}

func kotlinImportMatchesTarget(
	item kotlin.KotlinImportLocation,
	targetPackage string,
	declarations []kotlin.KotlinDeclarationLocation,
) bool {
	if item.Wildcard {
		return strings.TrimSuffix(item.Path, ".*") == targetPackage
	}
	for _, declaration := range declarations {
		if item.Imported == declaration.Name &&
			strings.HasPrefix(item.Path, targetPackage+".") {
			return true
		}
	}
	return false
}

func matchingKotlinDeclaration(
	declarations []kotlin.KotlinDeclarationLocation,
	imported string,
	local string,
) kotlin.KotlinDeclarationLocation {
	for _, declaration := range declarations {
		if declaration.Name == imported || declaration.Name == local {
			return declaration
		}
	}
	return kotlin.KotlinDeclarationLocation{Line: 1}
}

func kotlinRelationship(
	referenceKind string,
	declarationKind string,
) depgraph.DependencyRelationship {
	switch referenceKind {
	case "call":
		return depgraph.RelationshipCall
	case "inheritance":
		return depgraph.RelationshipInheritance
	case "companion-member":
		return depgraph.RelationshipCompanionMember
	case "type-reference":
		return depgraph.RelationshipTypeReference
	default:
		switch declarationKind {
		case "class", "interface", "object", "typealias":
			return depgraph.RelationshipTypeReference
		default:
			return depgraph.RelationshipSymbolReference
		}
	}
}

func ecmaScriptEvidence(edge depgraph.FileEdge, reader vcs.ContentReader) []depgraph.DependencyEvidence {
	source, sourceErr := reader(edge.From)
	target, targetErr := reader(edge.To)
	if sourceErr != nil || targetErr != nil {
		return nil
	}
	imports := typescript.ParseECMAScriptImportLocations(source)
	declarations := typescript.ParseECMAScriptDeclarationLocations(target)
	language := "javascript"
	switch filepath.Ext(edge.From) {
	case ".ts", ".tsx", ".mts", ".cts":
		language = "typescript"
	}

	selected := make(map[string]typescript.ECMAScriptImportLocation)
	var evidence []depgraph.DependencyEvidence
	for _, item := range imports {
		if !ecmaScriptImportTargetsEdge(edge, item.Path) {
			continue
		}
		selected[item.Local] = item
		declaration := matchingECMAScriptDeclaration(declarations, item.Imported, item.Local)
		relationship := depgraph.RelationshipImport
		switch item.Kind {
		case "type-import":
			relationship = depgraph.RelationshipTypeImport
		case "re-export":
			relationship = depgraph.RelationshipReExport
		}
		evidence = append(evidence, depgraph.DependencyEvidence{
			Symbol:          item.Imported,
			Kind:            language + "-" + item.Kind,
			Relationship:    relationship,
			ReferenceFile:   edge.From,
			ReferenceLine:   item.Line,
			DeclarationFile: edge.To,
			DeclarationLine: declaration.Line,
			Confidence:      depgraph.EvidenceConfidenceHigh,
		})
	}
	if len(selected) == 0 {
		return nil
	}
	for _, reference := range typescript.ExtractECMAScriptReferenceLocations(source, imports) {
		local := ecmaLocalForImported(selected, reference.Name, reference.Path)
		item, ok := selected[local]
		if !ok {
			continue
		}
		declaration := matchingECMAScriptDeclaration(
			declarations, item.Imported, item.Local)
		evidence = append(evidence, depgraph.DependencyEvidence{
			Symbol:          reference.Name,
			Kind:            language + "-" + reference.Kind,
			Relationship:    ecmaScriptRelationship(reference.Kind),
			ReferenceFile:   edge.From,
			ReferenceLine:   reference.Line,
			DeclarationFile: edge.To,
			DeclarationLine: declaration.Line,
			Confidence:      depgraph.EvidenceConfidenceMedium,
		})
	}
	sortEvidence(evidence)
	return deduplicateEvidence(evidence)
}

func ecmaLocalForImported(
	selected map[string]typescript.ECMAScriptImportLocation,
	imported string,
	path string,
) string {
	for local, item := range selected {
		if item.Imported == imported && item.Path == path {
			return local
		}
	}
	return ""
}

func matchingECMAScriptDeclaration(
	declarations []typescript.ECMAScriptDeclarationLocation,
	imported string,
	local string,
) typescript.ECMAScriptDeclarationLocation {
	for _, declaration := range declarations {
		if declaration.Name == imported ||
			(imported == "default" && declaration.Name == local) {
			return declaration
		}
	}
	return typescript.ECMAScriptDeclarationLocation{Line: 1}
}

func ecmaScriptImportTargetsEdge(edge depgraph.FileEdge, importPath string) bool {
	if !strings.HasPrefix(importPath, ".") {
		return false
	}
	resolved := filepath.Clean(filepath.Join(filepath.Dir(edge.From), importPath))
	if resolved == edge.To {
		return true
	}
	targetWithoutExtension := strings.TrimSuffix(edge.To, filepath.Ext(edge.To))
	if resolved == targetWithoutExtension {
		return true
	}
	return resolved == filepath.Dir(edge.To) &&
		strings.HasPrefix(filepath.Base(edge.To), "index.")
}

func ecmaScriptRelationship(kind string) depgraph.DependencyRelationship {
	switch kind {
	case "call":
		return depgraph.RelationshipCall
	case "type-reference":
		return depgraph.RelationshipTypeReference
	case "inheritance":
		return depgraph.RelationshipInheritance
	default:
		return depgraph.RelationshipSymbolReference
	}
}

func goEvidence(edge depgraph.FileEdge, reader vcs.ContentReader) []depgraph.DependencyEvidence {
	source, sourceErr := reader(edge.From)
	target, targetErr := reader(edge.To)
	if sourceErr != nil || targetErr != nil {
		return nil
	}
	declarations := golang.ParseGoDeclarationLocations(edge.To, target)
	sourcePackage, references := golang.ExtractGoReferenceLocations(edge.From, source)
	targetPackage := ""
	if len(declarations) > 0 {
		targetPackage = declarations[0].Package
	} else {
		targetPackage, _ = golang.ExtractGoReferenceLocations(edge.To, target)
	}
	samePackage := sourcePackage != "" && sourcePackage == targetPackage

	var evidence []depgraph.DependencyEvidence
	matchedImport := false
	for _, reference := range references {
		if reference.Kind == "import" {
			if !samePackage && goImportMatchesTarget(reference, targetPackage, edge.To) {
				matchedImport = true
				evidence = append(evidence, depgraph.DependencyEvidence{
					Symbol:          reference.ImportPath,
					Kind:            "go-import",
					Relationship:    depgraph.RelationshipImport,
					ReferenceFile:   edge.From,
					ReferenceLine:   reference.Line,
					DeclarationFile: edge.To,
					DeclarationLine: 1,
					Confidence:      depgraph.EvidenceConfidenceHigh,
				})
			}
			continue
		}
		for _, declaration := range declarations {
			if reference.Name != declaration.Name {
				continue
			}
			if samePackage && reference.Qualifier != "" {
				continue
			}
			if !samePackage && (reference.Qualifier == "" ||
				!goImportPathMatchesPackage(reference.ImportPath, targetPackage)) {
				continue
			}
			relationship := goRelationship(reference.Kind, declaration.Kind)
			scope := "imported"
			if samePackage {
				scope = "same-package"
			}
			evidence = append(evidence, depgraph.DependencyEvidence{
				Symbol:          declaration.Name,
				Kind:            "go-" + scope + "-" + reference.Kind,
				Relationship:    relationship,
				ReferenceFile:   edge.From,
				ReferenceLine:   reference.Line,
				DeclarationFile: edge.To,
				DeclarationLine: declaration.Line,
				Confidence:      depgraph.EvidenceConfidenceHigh,
			})
		}
	}
	if !samePackage && !matchedImport {
		return nil
	}
	sortEvidence(evidence)
	return deduplicateEvidence(evidence)
}

func goImportMatchesTarget(
	reference golang.GoReferenceLocation,
	targetPackage string,
	targetFile string,
) bool {
	return goImportPathMatchesPackage(reference.ImportPath, targetPackage) ||
		filepath.Base(filepath.Dir(targetFile)) == filepath.Base(reference.ImportPath)
}

func goImportPathMatchesPackage(importPath, packageName string) bool {
	if importPath == "" || packageName == "" {
		return false
	}
	return filepath.Base(importPath) == packageName
}

func goRelationship(referenceKind, declarationKind string) depgraph.DependencyRelationship {
	switch referenceKind {
	case "call":
		return depgraph.RelationshipCall
	case "inheritance":
		return depgraph.RelationshipInheritance
	default:
		if declarationKind == "type" {
			return depgraph.RelationshipTypeReference
		}
		return depgraph.RelationshipSymbolReference
	}
}

func rustEvidence(edge depgraph.FileEdge, reader vcs.ContentReader) []depgraph.DependencyEvidence {
	source, sourceErr := reader(edge.From)
	target, targetErr := reader(edge.To)
	if sourceErr != nil || targetErr != nil {
		return nil
	}

	declarations := rust.ParseRustDeclarationLocations(target)
	references := rust.ExtractRustReferenceLocations(source)
	targetModule := rustModuleName(edge.To)
	allowedSymbols := make(map[string]bool)
	for _, reference := range references {
		if reference.Kind != "import" && reference.Kind != "re-export" {
			continue
		}
		for _, declaration := range declarations {
			if reference.Name == declaration.Name {
				allowedSymbols[reference.Name] = true
			}
		}
	}
	var evidence []depgraph.DependencyEvidence
	for _, reference := range references {
		if reference.Kind == "module-declaration" && reference.Name == targetModule {
			evidence = append(evidence, depgraph.DependencyEvidence{
				Symbol:          reference.Name,
				Kind:            "rust-module-declaration",
				Relationship:    depgraph.RelationshipModuleDeclaration,
				ReferenceFile:   edge.From,
				ReferenceLine:   reference.Line,
				DeclarationFile: edge.To,
				DeclarationLine: 1,
				Confidence:      depgraph.EvidenceConfidenceHigh,
			})
			continue
		}
		matchedDeclaration := false
		for _, declaration := range declarations {
			if reference.Name != declaration.Name {
				continue
			}
			if reference.Kind != "import" && reference.Kind != "re-export" &&
				!allowedSymbols[reference.Name] &&
				!rustPathContainsModule(reference.Path, targetModule) {
				continue
			}
			matchedDeclaration = true
			evidence = append(evidence, depgraph.DependencyEvidence{
				Symbol:          declaration.Name,
				Kind:            "rust-" + reference.Kind,
				Relationship:    rustRelationship(reference.Kind),
				ReferenceFile:   edge.From,
				ReferenceLine:   reference.Line,
				DeclarationFile: edge.To,
				DeclarationLine: declaration.Line,
				Confidence:      rustEvidenceConfidence(reference.Kind),
			})
		}
		if !matchedDeclaration &&
			(reference.Kind == "import" || reference.Kind == "re-export") &&
			reference.Name == targetModule {
			evidence = append(evidence, depgraph.DependencyEvidence{
				Symbol:          reference.Name,
				Kind:            "rust-" + reference.Kind,
				Relationship:    rustRelationship(reference.Kind),
				ReferenceFile:   edge.From,
				ReferenceLine:   reference.Line,
				DeclarationFile: edge.To,
				DeclarationLine: 1,
				Confidence:      depgraph.EvidenceConfidenceHigh,
			})
		}
	}
	sortEvidence(evidence)
	return deduplicateEvidence(evidence)
}

func rustPathContainsModule(path, module string) bool {
	if path == "" || module == "" {
		return false
	}
	for _, part := range strings.Split(path, "::") {
		if part == module {
			return true
		}
	}
	return false
}

func rustModuleName(path string) string {
	base := filepath.Base(path)
	if base == "mod.rs" {
		return filepath.Base(filepath.Dir(path))
	}
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func rustRelationship(kind string) depgraph.DependencyRelationship {
	switch kind {
	case "module-declaration":
		return depgraph.RelationshipModuleDeclaration
	case "import":
		return depgraph.RelationshipImport
	case "re-export":
		return depgraph.RelationshipReExport
	case "call":
		return depgraph.RelationshipCall
	case "type-reference":
		return depgraph.RelationshipTypeReference
	default:
		return depgraph.RelationshipSymbolReference
	}
}

func rustEvidenceConfidence(kind string) depgraph.EvidenceConfidence {
	switch kind {
	case "module-declaration", "import", "re-export":
		return depgraph.EvidenceConfidenceHigh
	default:
		return depgraph.EvidenceConfidenceMedium
	}
}

func swiftEvidence(edge depgraph.FileEdge, reader vcs.ContentReader) []depgraph.DependencyEvidence {
	source, sourceErr := reader(edge.From)
	target, targetErr := reader(edge.To)
	if sourceErr != nil || targetErr != nil {
		return nil
	}

	owners := make(map[string]bool)
	for _, owner := range swift.ParseSwiftTopLevelTypeNames(source) {
		owners[owner] = true
	}
	for _, owner := range swift.ParseSwiftExtensionOwners(source) {
		owners[owner] = true
	}

	var evidence []depgraph.DependencyEvidence
	for _, declaration := range swift.ParseSwiftDeclarationLocations(target) {
		for _, reference := range swift.ExtractSwiftReferenceLocations(source) {
			if reference.Name != declaration.Name {
				continue
			}
			matches := false
			confidence := depgraph.EvidenceConfidenceMedium
			if declaration.Owner == "" {
				matches = reference.Qualifier == ""
				if matches && startsUpper(declaration.Name) {
					confidence = depgraph.EvidenceConfidenceHigh
				}
			} else if reference.Qualifier == "" {
				matches = owners[declaration.Owner]
				confidence = depgraph.EvidenceConfidenceHigh
			} else {
				matches = reference.Qualifier == declaration.Owner ||
					(reference.Qualifier == "Self" && owners[declaration.Owner])
				confidence = depgraph.EvidenceConfidenceHigh
			}
			if !matches {
				continue
			}
			relationship := depgraph.RelationshipSymbolReference
			if declaration.Kind == "swift-extension-member" {
				relationship = depgraph.RelationshipExtensionMember
			} else if startsUpper(declaration.Name) {
				relationship = depgraph.RelationshipTypeReference
			}
			evidence = append(evidence, depgraph.DependencyEvidence{
				Symbol:          declaration.Name,
				Kind:            declaration.Kind,
				Relationship:    relationship,
				ReferenceFile:   edge.From,
				ReferenceLine:   reference.Line,
				DeclarationFile: edge.To,
				DeclarationLine: declaration.Line,
				Confidence:      confidence,
			})
		}
	}
	sortEvidence(evidence)
	return deduplicateEvidence(evidence)
}

func markdownEvidence(
	edge depgraph.FileEdge,
	supplied map[string]bool,
	reader vcs.ContentReader,
) []depgraph.DependencyEvidence {
	content, err := reader(edge.From)
	if err != nil {
		return nil
	}
	var evidence []depgraph.DependencyEvidence
	for _, link := range markdown.ParseMarkdownLinks(content) {
		for _, target := range markdown.ResolveMarkdownLinkPath(edge.From, link.Path(), supplied) {
			if target != edge.To {
				continue
			}
			evidence = append(evidence, depgraph.DependencyEvidence{
				Symbol:          link.Path(),
				Kind:            "markdown-link",
				Relationship:    depgraph.RelationshipNavigation,
				ReferenceFile:   edge.From,
				ReferenceLine:   lineOf(content, link.Path()),
				DeclarationFile: edge.To,
				DeclarationLine: 1,
				Confidence:      depgraph.EvidenceConfidenceHigh,
			})
		}
	}
	sortEvidence(evidence)
	return deduplicateEvidence(evidence)
}

func lineOf(content []byte, needle string) int {
	index := bytes.Index(content, []byte(needle))
	if index < 0 {
		return 0
	}
	return bytes.Count(content[:index], []byte{'\n'}) + 1
}

func startsUpper(value string) bool {
	for _, r := range value {
		return unicode.IsUpper(r)
	}
	return false
}

func sortEvidence(evidence []depgraph.DependencyEvidence) {
	sort.Slice(evidence, func(i, j int) bool {
		if evidence[i].ReferenceLine == evidence[j].ReferenceLine {
			if evidence[i].DeclarationLine == evidence[j].DeclarationLine {
				return evidence[i].Symbol < evidence[j].Symbol
			}
			return evidence[i].DeclarationLine < evidence[j].DeclarationLine
		}
		return evidence[i].ReferenceLine < evidence[j].ReferenceLine
	})
}

func deduplicateEvidence(evidence []depgraph.DependencyEvidence) []depgraph.DependencyEvidence {
	if len(evidence) < 2 {
		return evidence
	}
	result := evidence[:0]
	for _, item := range evidence {
		if len(result) > 0 && result[len(result)-1] == item {
			continue
		}
		result = append(result, item)
	}
	return result
}
