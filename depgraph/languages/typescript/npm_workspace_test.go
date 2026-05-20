package typescript

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: build a synthetic workspace under tmp and return the workspace root.
func makeWorkspace(t *testing.T, rootManifest string, packages map[string]string) string {
	t.Helper()
	tmp := t.TempDir()
	if rootManifest != "" {
		require.NoError(t, os.WriteFile(filepath.Join(tmp, "package.json"), []byte(rootManifest), 0644))
	}
	for relDir, pkgJSON := range packages {
		dir := filepath.Join(tmp, relDir)
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0644))
	}
	return tmp
}

func TestParseWorkspacesFromPackageJSON_ArrayShape(t *testing.T) {
	patterns := parseWorkspacesFromPackageJSON([]byte(`{
  "name": "root",
  "workspaces": ["packages/*", "apps/*"]
}`))
	assert.ElementsMatch(t, []string{"packages/*", "apps/*"}, patterns)
}

func TestParseWorkspacesFromPackageJSON_ObjectShape(t *testing.T) {
	patterns := parseWorkspacesFromPackageJSON([]byte(`{
  "name": "root",
  "workspaces": {
    "packages": ["packages/*", "tools/*"],
    "nohoist": ["foo"]
  }
}`))
	assert.ElementsMatch(t, []string{"packages/*", "tools/*"}, patterns)
}

func TestParseWorkspacesFromPackageJSON_None(t *testing.T) {
	patterns := parseWorkspacesFromPackageJSON([]byte(`{"name": "leaf"}`))
	assert.Empty(t, patterns)
}

func TestParsePnpmWorkspaceYAML_BasicList(t *testing.T) {
	patterns := parsePnpmWorkspaceYAML([]byte(`packages:
  - 'packages/*'
  - "apps/**"
  - tools/foo
`))
	assert.ElementsMatch(t, []string{"packages/*", "apps/**", "tools/foo"}, patterns)
}

func TestParsePnpmWorkspaceYAML_WithComments(t *testing.T) {
	patterns := parsePnpmWorkspaceYAML([]byte(`# top-level comment
packages:
  # commented-out: 'old/*'
  - 'packages/*'
  - 'apps/*'

# trailing comment
`))
	assert.ElementsMatch(t, []string{"packages/*", "apps/*"}, patterns)
}

func TestParsePnpmWorkspaceYAML_StopsAtNextTopLevelKey(t *testing.T) {
	patterns := parsePnpmWorkspaceYAML([]byte(`packages:
  - 'packages/*'
publicHoistPattern:
  - 'eslint-*'
`))
	assert.ElementsMatch(t, []string{"packages/*"}, patterns)
}

func TestLoadNpmWorkspace_PackageJsonWorkspaces(t *testing.T) {
	root := makeWorkspace(t,
		`{"name": "root", "workspaces": ["packages/*"]}`,
		map[string]string{
			"packages/core":    `{"name": "@org/core"}`,
			"packages/ui":      `{"name": "@org/ui"}`,
			"packages/no-name": `{}`,
			"unrelated":        `{"name": "should-be-ignored"}`,
		})

	// Pretend a source file lives inside packages/ui.
	src := filepath.Join(root, "packages", "ui", "src", "index.ts")
	ws := loadNpmWorkspaceFor(src)

	require.NotNil(t, ws)
	assert.Equal(t, root, ws.rootDir)
	assert.Equal(t, filepath.Join(root, "packages", "core"), ws.packages["@org/core"])
	assert.Equal(t, filepath.Join(root, "packages", "ui"), ws.packages["@org/ui"])
	_, present := ws.packages["should-be-ignored"]
	assert.False(t, present, "unrelated package outside workspaces glob must not be indexed")
}

func TestLoadNpmWorkspace_PnpmYAML(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "package.json"),
		[]byte(`{"name": "monorepo", "private": true}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "pnpm-workspace.yaml"),
		[]byte("packages:\n  - 'packages/*'\n"), 0644))

	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "packages", "a", "src"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "packages", "a", "package.json"),
		[]byte(`{"name": "@my/a"}`), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "packages", "b", "src"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "packages", "b", "package.json"),
		[]byte(`{"name": "@my/b"}`), 0644))

	src := filepath.Join(tmp, "packages", "a", "src", "index.ts")
	ws := loadNpmWorkspaceFor(src)

	require.NotNil(t, ws)
	assert.Equal(t, filepath.Join(tmp, "packages", "a"), ws.packages["@my/a"])
	assert.Equal(t, filepath.Join(tmp, "packages", "b"), ws.packages["@my/b"])
}

func TestLoadNpmWorkspace_NoWorkspace(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "package.json"),
		[]byte(`{"name": "leaf"}`), 0644))

	src := filepath.Join(tmp, "src", "index.ts")
	ws := loadNpmWorkspaceFor(src)
	assert.Nil(t, ws)
}

func TestResolveWorkspacePackage_BareAndSubpath(t *testing.T) {
	ws := &npmWorkspace{
		rootDir: "/repo",
		packages: map[string]string{
			"@tanstack/query-core":  "/repo/packages/query-core",
			"@tanstack/react-query": "/repo/packages/react-query",
			"plain-name":            "/repo/packages/plain-name",
		},
	}

	// Bare-package import resolves the package directory and empty subpath.
	dir, sub, ok := ws.resolveWorkspacePackage("@tanstack/query-core")
	assert.True(t, ok)
	assert.Equal(t, "/repo/packages/query-core", dir)
	assert.Equal(t, "", sub)

	// Sub-path import keeps the sub-path.
	dir, sub, ok = ws.resolveWorkspacePackage("@tanstack/query-core/devtools")
	assert.True(t, ok)
	assert.Equal(t, "/repo/packages/query-core", dir)
	assert.Equal(t, "devtools", sub)

	// Deep sub-path.
	dir, sub, ok = ws.resolveWorkspacePackage("@tanstack/query-core/lib/util")
	assert.True(t, ok)
	assert.Equal(t, "/repo/packages/query-core", dir)
	assert.Equal(t, "lib/util", sub)

	// Unscoped package name.
	dir, sub, ok = ws.resolveWorkspacePackage("plain-name/foo")
	assert.True(t, ok)
	assert.Equal(t, "/repo/packages/plain-name", dir)
	assert.Equal(t, "foo", sub)

	// Non-workspace import returns false.
	_, _, ok = ws.resolveWorkspacePackage("react")
	assert.False(t, ok)
	_, _, ok = ws.resolveWorkspacePackage("@some-other/scope")
	assert.False(t, ok)
}
