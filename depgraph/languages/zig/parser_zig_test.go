package zig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseImports(t *testing.T) {
	source := []byte(`
const std = @import("std");
const helper = @import("helper.zig");
const nested = @import("pkg/nested.zig");
const escaped = @import("dir/with_\"quote\".zig");
// const ignored = @import("ignored.zig");
`)

	imports, err := ParseImports(source)
	require.NoError(t, err)

	assert.Equal(t, []Import{
		{Path: "std"},
		{Path: "helper.zig"},
		{Path: "pkg/nested.zig"},
		{Path: "dir/with_\"quote\".zig"},
	}, imports)
}

func TestResolveZigProjectImports(t *testing.T) {
	tmpDir := t.TempDir()
	mainPath := filepath.Join(tmpDir, "src", "main.zig")
	helperPath := filepath.Join(tmpDir, "src", "helper.zig")
	nestedPath := filepath.Join(tmpDir, "src", "pkg", "nested.zig")
	externalPath := filepath.Join(tmpDir, "src", "external.zig")

	suppliedFiles := map[string]bool{
		mainPath:   true,
		helperPath: true,
		nestedPath: true,
	}
	contentReader := func(path string) ([]byte, error) {
		if path == mainPath {
			return []byte(`
const std = @import("std");
const helper = @import("helper.zig");
const nested = @import("pkg/nested.zig");
const missing = @import("external.zig");
`), nil
		}
		return vcs.FilesystemContentReader()(path)
	}

	imports, err := ResolveZigProjectImports(mainPath, mainPath, ".zig", suppliedFiles, contentReader)
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{helperPath, nestedPath}, imports)
	assert.NotContains(t, imports, externalPath)
}

func TestResolveZigProjectImportsThroughStdBuildReexports(t *testing.T) {
	tmpDir := t.TempDir()

	stdPath := filepath.Join(tmpDir, "lib", "std", "std.zig")
	buildPath := filepath.Join(tmpDir, "lib", "std", "Build.zig")
	buildRunnerPath := filepath.Join(tmpDir, "lib", "compiler", "build_runner.zig")
	fuzzPath := filepath.Join(tmpDir, "lib", "std", "Build", "Fuzz.zig")
	stepPath := filepath.Join(tmpDir, "lib", "std", "Build", "Step.zig")
	cachePath := filepath.Join(tmpDir, "lib", "std", "Build", "Cache.zig")
	webServerPath := filepath.Join(tmpDir, "lib", "std", "Build", "WebServer.zig")

	writeFile(t, stdPath, `pub const Build = @import("Build.zig");`)
	writeFile(t, buildPath, `
pub const Cache = @import("Build/Cache.zig");
pub const Fuzz = @import("Build/Fuzz.zig");
pub const Step = @import("Build/Step.zig");
pub const WebServer = @import("Build/WebServer.zig");
`)
	writeFile(t, buildRunnerPath, `
const std = @import("std");
const Step = std.Build.Step;
const WebServer = std.Build.WebServer;
cache: std.Build.Cache,
fuzz: std.Build.Fuzz,
`)
	writeFile(t, fuzzPath, `
const std = @import("../std.zig");
const Build = std.Build;
const Cache = Build.Cache;
const Step = std.Build.Step;

run_steps: []const *Step.Run,
cache_path: Build.Cache.Path,
other_cache_path: Cache.Path,
`)
	writeFile(t, stepPath, `pub const Run = @import("Step/Run.zig");`)
	writeFile(t, cachePath, `pub const Path = @import("Cache/Path.zig");`)
	writeFile(t, webServerPath, `const WebServer = @This();`)

	suppliedFiles := map[string]bool{
		buildRunnerPath: true,
		fuzzPath:        true,
		stepPath:        true,
		cachePath:       true,
		webServerPath:   true,
	}

	imports, err := ResolveZigProjectImports(fuzzPath, fuzzPath, ".zig", suppliedFiles, vcs.FilesystemContentReader())
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{stepPath, cachePath}, imports)

	imports, err = ResolveZigProjectImports(buildRunnerPath, buildRunnerPath, ".zig", suppliedFiles, vcs.FilesystemContentReader())
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{stepPath, webServerPath}, imports)
}

func TestResolveZigProjectImportsIgnoresIncidentalQualifiedRefs(t *testing.T) {
	tmpDir := t.TempDir()

	stdPath := filepath.Join(tmpDir, "lib", "std", "std.zig")
	buildPath := filepath.Join(tmpDir, "lib", "std", "Build.zig")
	runPath := filepath.Join(tmpDir, "lib", "std", "Build", "Step", "Run.zig")
	fuzzPath := filepath.Join(tmpDir, "lib", "std", "Build", "Fuzz.zig")
	stepPath := filepath.Join(tmpDir, "lib", "std", "Build", "Step.zig")
	cachePath := filepath.Join(tmpDir, "lib", "std", "Build", "Cache.zig")

	writeFile(t, stdPath, `pub const Build = @import("Build.zig");`)
	writeFile(t, buildPath, `
pub const Cache = @import("Build/Cache.zig");
pub const Fuzz = @import("Build/Fuzz.zig");
pub const Step = @import("Build/Step.zig");
`)
	writeFile(t, runPath, `
const std = @import("std");
const Build = std.Build;
const Step = Build.Step;
const Path = Build.Cache.Path;

fn coverage(run: *Run, fuzz: *std.Build.Fuzz) void {
    _ = fuzz;
    _ = run;
}
`)
	writeFile(t, fuzzPath, `const Fuzz = @This();`)
	writeFile(t, stepPath, `pub const Run = @import("Step/Run.zig");`)
	writeFile(t, cachePath, `pub const Path = @import("Cache/Path.zig");`)

	suppliedFiles := map[string]bool{
		runPath:   true,
		fuzzPath:  true,
		stepPath:  true,
		cachePath: true,
	}

	imports, err := ResolveZigProjectImports(runPath, runPath, ".zig", suppliedFiles, vcs.FilesystemContentReader())
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{stepPath, cachePath}, imports)
	assert.NotContains(t, imports, fuzzPath)
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}

func TestIsTestFile(t *testing.T) {
	assert.True(t, IsTestFile("/project/src/math_test.zig"))
	assert.True(t, IsTestFile("/project/test/math.zig"))
	assert.True(t, IsTestFile("/project/tests/math.zig"))
	assert.False(t, IsTestFile("/project/src/main.zig"))
	assert.False(t, IsTestFile("/project/src/math_test.go"))
}
