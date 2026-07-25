package depgraph_test

import (
	"testing"

	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/stretchr/testify/assert"
)

func TestEvidenceRelationships_DeduplicatesAndSorts(t *testing.T) {
	evidence := []depgraph.DependencyEvidence{
		{Relationship: depgraph.RelationshipTypeReference},
		{Relationship: depgraph.RelationshipCall},
		{Relationship: depgraph.RelationshipTypeReference},
		{},
	}

	assert.Equal(t, []depgraph.DependencyRelationship{
		depgraph.RelationshipCall,
		depgraph.RelationshipResolvedDependency,
		depgraph.RelationshipTypeReference,
	}, depgraph.EvidenceRelationships(evidence))
}
