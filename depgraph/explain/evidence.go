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
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/markdown"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/rust"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/swift"
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
