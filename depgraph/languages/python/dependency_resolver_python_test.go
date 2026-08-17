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

// Regression test for CLR-66, pytest shape: `from _pytest import nodes` is an
// absolute self-referential import naming a sibling submodule. It must
// resolve to that submodule's file, not to the package's own __init__.py,
// when the submodule file actually exists.
func TestResolvePythonProjectImports_AbsoluteFromImportPrefersSubmodule(t *testing.T) {
	absPath := "/project/src/_pytest/fixtures.py"
	nodesPath := "/project/src/_pytest/nodes.py"
	initPath := "/project/src/_pytest/__init__.py"
	suppliedFiles := map[string]bool{
		absPath:   true,
		nodesPath: true,
		initPath:  true,
	}

	source := []byte("from _pytest import nodes\n")
	contentReader := func(path string) ([]byte, error) {
		return source, nil
	}

	resolved, err := ResolvePythonProjectImports(absPath, "src/_pytest/fixtures.py", ".py", suppliedFiles, contentReader)

	require.NoError(t, err)
	assert.Contains(t, resolved, nodesPath)
	assert.NotContains(t, resolved, initPath)
}

// Regression test for CLR-66, loguru shape: `from . import logger` where
// `logger` is an instance defined directly in the package's __init__.py, not
// a logger.py submodule. With no submodule file to prefer, this must fall
// back to the package's own file rather than producing no edge at all.
func TestResolvePythonProjectImports_BareRelativeFallsBackWhenNoSubmoduleExists(t *testing.T) {
	absPath := "/project/src/loguru/_logger.py"
	initPath := "/project/src/loguru/__init__.py"
	suppliedFiles := map[string]bool{
		absPath:  true,
		initPath: true,
	}

	source := []byte("from . import logger\n")
	contentReader := func(path string) ([]byte, error) {
		return source, nil
	}

	resolved, err := ResolvePythonProjectImports(absPath, "src/loguru/_logger.py", ".py", suppliedFiles, contentReader)

	require.NoError(t, err)
	assert.Contains(t, resolved, initPath)
}
