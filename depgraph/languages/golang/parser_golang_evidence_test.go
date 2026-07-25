package golang

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGoEvidenceLocations(t *testing.T) {
	source := []byte(`package worker

import model "example.com/app/model"

type Runner struct {
    model.Job
}

func Run(job model.Job) {
    model.Execute(job)
    local()
}
`)

	assert.Contains(t, ParseGoDeclarationLocations("worker.go", source), GoDeclarationLocation{
		Name: "Runner", Kind: "type", Package: "worker", Line: 5,
	})
	pkg, references := ExtractGoReferenceLocations("worker.go", source)
	assert.Equal(t, "worker", pkg)
	assert.Contains(t, references, GoReferenceLocation{
		Name: "model", ImportPath: "example.com/app/model", Kind: "import", Line: 3,
	})
	assert.Contains(t, references, GoReferenceLocation{
		Name: "Job", Qualifier: "model", ImportPath: "example.com/app/model",
		Kind: "inheritance", Line: 6,
	})
	assert.Contains(t, references, GoReferenceLocation{
		Name: "Execute", Qualifier: "model", ImportPath: "example.com/app/model",
		Kind: "call", Line: 10,
	})
	assert.Contains(t, references, GoReferenceLocation{
		Name: "local", Kind: "call", Line: 11,
	})
}
