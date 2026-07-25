// Package explain attaches source-level provenance to resolved file edges.
package explain

import (
	"bytes"
	"path/filepath"
	"sort"
	"unicode"

	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/markdown"
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
		case ".swift":
			metadata.Evidence = swiftEvidence(edge, reader)
		case ".md", ".markdown":
			metadata.Evidence = markdownEvidence(edge, supplied, reader)
		}
		if len(metadata.Evidence) == 0 {
			metadata.Evidence = []depgraph.DependencyEvidence{{
				Kind:            "resolved-dependency",
				ReferenceFile:   edge.From,
				DeclarationFile: edge.To,
				Confidence:      depgraph.EvidenceConfidenceMedium,
			}}
		}
		graph.Meta.Edges[edge] = metadata
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
			evidence = append(evidence, depgraph.DependencyEvidence{
				Symbol:          declaration.Name,
				Kind:            declaration.Kind,
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
