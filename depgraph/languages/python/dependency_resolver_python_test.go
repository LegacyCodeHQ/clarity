package python

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Regression test: a project file named prophet.py that imports the
// third-party `prophet` package must not resolve to itself. The naive
// suffix-matching resolver previously produced a self-edge.
func TestResolvePythonProjectImports_NoSelfEdgeFromThirdPartyLookalike(t *testing.T) {
	absPath := "/project/superset/utils/pandas_postprocessing/prophet.py"
	suppliedFiles := map[string]bool{
		absPath: true,
	}

	source := []byte("from prophet import Prophet\n")
	contentReader := func(path string) ([]byte, error) {
		return source, nil
	}

	resolved, err := ResolvePythonProjectImports(absPath,
		"superset/utils/pandas_postprocessing/prophet.py",
		".py",
		suppliedFiles,
		contentReader)

	require.NoError(t, err)
	assert.NotContains(t, resolved, absPath, "file must not depend on itself")
}
