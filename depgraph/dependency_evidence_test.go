package depgraph_test

import (
	"testing"

	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/stretchr/testify/assert"
)

func TestEdgeRelationships_DeduplicatesAndSorts(t *testing.T) {
	metadata := depgraph.EdgeMetadata{Evidence: []depgraph.DependencyEvidence{
		{Relationship: depgraph.RelationshipTypeReference},
		{Relationship: depgraph.RelationshipCall},
		{Relationship: depgraph.RelationshipTypeReference},
		{},
	}}

	assert.Equal(t, []depgraph.DependencyRelationship{
		depgraph.RelationshipCall,
		depgraph.RelationshipResolvedDependency,
		depgraph.RelationshipTypeReference,
	}, depgraph.EdgeRelationships(metadata))
}
