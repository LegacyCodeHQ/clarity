package depgraph

import "sort"

// EvidenceConfidence describes how directly Clarity established a dependency.
//
// Experimental: evidence API values may change before stabilization.
type EvidenceConfidence string

const (
	EvidenceConfidenceHigh   EvidenceConfidence = "high"
	EvidenceConfidenceMedium EvidenceConfidence = "medium"
	EvidenceConfidenceLow    EvidenceConfidence = "low"
)

// DependencyRelationship describes the semantic reason one file depends on
// another. Unlike Kind, which identifies the language-specific evidence
// extractor, Relationship is a cross-language taxonomy suitable for filtering
// and automation.
//
// Experimental: relationship names and semantics may change before
// stabilization.
type DependencyRelationship string

const (
	RelationshipResolvedDependency DependencyRelationship = "resolved-dependency"
	RelationshipModuleDeclaration  DependencyRelationship = "module-declaration"
	RelationshipImport             DependencyRelationship = "import"
	RelationshipTypeImport         DependencyRelationship = "type-import"
	RelationshipReExport           DependencyRelationship = "re-export"
	RelationshipCall               DependencyRelationship = "call"
	RelationshipTypeReference      DependencyRelationship = "type-reference"
	RelationshipSymbolReference    DependencyRelationship = "symbol-reference"
	RelationshipInheritance        DependencyRelationship = "inheritance"
	RelationshipExtensionMember    DependencyRelationship = "extension-member"
	RelationshipCompanionMember    DependencyRelationship = "companion-member"
	RelationshipNavigation         DependencyRelationship = "navigation"
	RelationshipScript             DependencyRelationship = "script"
	RelationshipStylesheet         DependencyRelationship = "stylesheet"
	RelationshipImage              DependencyRelationship = "image"
	RelationshipEmbeddedResource   DependencyRelationship = "embedded-resource"
)

// DependencyRelationships returns every supported semantic relationship in
// deterministic display order.
//
// Experimental: the returned relationship set may change before stabilization.
func DependencyRelationships() []DependencyRelationship {
	return []DependencyRelationship{
		RelationshipResolvedDependency,
		RelationshipModuleDeclaration,
		RelationshipImport,
		RelationshipTypeImport,
		RelationshipReExport,
		RelationshipCall,
		RelationshipTypeReference,
		RelationshipSymbolReference,
		RelationshipInheritance,
		RelationshipExtensionMember,
		RelationshipCompanionMember,
		RelationshipNavigation,
		RelationshipScript,
		RelationshipStylesheet,
		RelationshipImage,
		RelationshipEmbeddedResource,
	}
}

// DependencyEvidence identifies a declaration/reference pair behind an edge.
//
// Experimental: this type and its JSON representation may change before
// stabilization.
type DependencyEvidence struct {
	Symbol          string                 `json:"symbol,omitempty"`
	Kind            string                 `json:"kind"`
	Relationship    DependencyRelationship `json:"relationship"`
	ReferenceFile   string                 `json:"reference_file,omitempty"`
	ReferenceLine   int                    `json:"reference_line,omitempty"`
	DeclarationFile string                 `json:"declaration_file,omitempty"`
	DeclarationLine int                    `json:"declaration_line,omitempty"`
	Confidence      EvidenceConfidence     `json:"confidence"`
}

// EvidenceRelationships returns the deterministic, deduplicated semantic
// relationships represented by source evidence.
//
// Experimental: this function may change before stabilization.
func EvidenceRelationships(evidenceItems []DependencyEvidence) []DependencyRelationship {
	seen := make(map[DependencyRelationship]bool)
	for _, evidence := range evidenceItems {
		relationship := evidence.Relationship
		if relationship == "" {
			relationship = RelationshipResolvedDependency
		}
		seen[relationship] = true
	}
	result := make([]DependencyRelationship, 0, len(seen))
	for relationship := range seen {
		result = append(result, relationship)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})
	return result
}
