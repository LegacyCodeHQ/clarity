package typescript

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestECMAScriptEvidenceLocations(t *testing.T) {
	source := []byte(`import type { Job } from "./model";
import Runner, { execute as run } from "./runner";
export { Result } from "./result";

export class Worker extends Runner {
  handle(job: Job) {
    return run(job);
  }
}
`)

	assert.Contains(t, ParseECMAScriptDeclarationLocations(source), ECMAScriptDeclarationLocation{
		Name: "Worker", Kind: "class", Line: 5,
	})
	imports := ParseECMAScriptImportLocations(source)
	assert.Contains(t, imports, ECMAScriptImportLocation{
		Path: "./model", Imported: "Job", Local: "Job", Kind: "type-import", Line: 1,
	})
	assert.Contains(t, imports, ECMAScriptImportLocation{
		Path: "./runner", Imported: "execute", Local: "run", Kind: "import", Line: 2,
	})
	assert.Contains(t, imports, ECMAScriptImportLocation{
		Path: "./result", Imported: "Result", Local: "Result", Kind: "re-export", Line: 3,
	})
	references := ExtractECMAScriptReferenceLocations(source, imports)
	assert.Contains(t, references, ECMAScriptReferenceLocation{
		Name: "default", Path: "./runner", Kind: "inheritance", Line: 5,
	})
	assert.Contains(t, references, ECMAScriptReferenceLocation{
		Name: "Job", Path: "./model", Kind: "type-reference", Line: 6,
	})
	assert.Contains(t, references, ECMAScriptReferenceLocation{
		Name: "execute", Path: "./runner", Kind: "call", Line: 7,
	})
}
