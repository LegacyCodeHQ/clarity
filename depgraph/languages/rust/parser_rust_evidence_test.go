package rust

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseRustEvidenceLocations(t *testing.T) {
	source := []byte(`pub mod worker;
pub use crate::model::Job;
use crate::runner::run;

pub struct Queue {
    job: Job,
}

fn execute() {
    run();
}
`)

	assert.Contains(t, ParseRustDeclarationLocations(source), RustDeclarationLocation{
		Name: "Queue", Kind: "struct", Line: 5,
	})
	references := ExtractRustReferenceLocations(source)
	assert.Contains(t, references, RustReferenceLocation{
		Name: "worker", Kind: "module-declaration", Line: 1,
	})
	assert.Contains(t, references, RustReferenceLocation{
		Name: "Job", Kind: "re-export", Line: 2,
	})
	assert.Contains(t, references, RustReferenceLocation{
		Name: "run", Kind: "import", Line: 3,
	})
	assert.Contains(t, references, RustReferenceLocation{
		Name: "Job", Kind: "type-reference", Line: 6,
	})
	assert.Contains(t, references, RustReferenceLocation{
		Name: "run", Kind: "call", Line: 10,
	})
}
