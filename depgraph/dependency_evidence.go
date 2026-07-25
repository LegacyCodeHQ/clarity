package depgraph

// EvidenceConfidence describes how directly Clarity established a dependency.
type EvidenceConfidence string

const (
	EvidenceConfidenceHigh   EvidenceConfidence = "high"
	EvidenceConfidenceMedium EvidenceConfidence = "medium"
	EvidenceConfidenceLow    EvidenceConfidence = "low"
)

// DependencyEvidence identifies a declaration/reference pair behind an edge.
type DependencyEvidence struct {
	Symbol          string             `json:"symbol,omitempty"`
	Kind            string             `json:"kind"`
	ReferenceFile   string             `json:"reference_file,omitempty"`
	ReferenceLine   int                `json:"reference_line,omitempty"`
	DeclarationFile string             `json:"declaration_file,omitempty"`
	DeclarationLine int                `json:"declaration_line,omitempty"`
	Confidence      EvidenceConfidence `json:"confidence"`
}
