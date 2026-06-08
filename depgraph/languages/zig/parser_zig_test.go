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

func TestResolveZigProjectImportsContinuesThroughSuppliedIntermediateReexports(t *testing.T) {
	tmpDir := t.TempDir()

	stdPath := filepath.Join(tmpDir, "lib", "std", "std.zig")
	osPath := filepath.Join(tmpDir, "lib", "std", "os.zig")
	windowsPath := filepath.Join(tmpDir, "lib", "std", "os", "windows.zig")
	kernel32Path := filepath.Join(tmpDir, "lib", "std", "os", "windows", "kernel32.zig")
	posixPath := filepath.Join(tmpDir, "lib", "std", "posix.zig")

	writeFile(t, stdPath, `
pub const os = @import("os.zig");
pub const posix = @import("posix.zig");
`)
	writeFile(t, osPath, `pub const windows = @import("os/windows.zig");`)
	writeFile(t, windowsPath, `pub const kernel32 = @import("windows/kernel32.zig");`)
	writeFile(t, kernel32Path, `
const std = @import("../../std.zig");
const windows = std.os.windows;
const BOOL = windows.BOOL;
`)
	writeFile(t, posixPath, `
const std = @import("std.zig");
const windows = std.os.windows;
const UnexpectedError = std.posix.UnexpectedError;
`)

	suppliedFiles := map[string]bool{
		osPath:       true,
		windowsPath:  true,
		kernel32Path: true,
		posixPath:    true,
	}

	imports, err := ResolveZigProjectImports(kernel32Path, kernel32Path, ".zig", suppliedFiles, vcs.FilesystemContentReader())
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{windowsPath}, imports)
	assert.NotContains(t, imports, osPath)

	imports, err = ResolveZigProjectImports(posixPath, posixPath, ".zig", suppliedFiles, vcs.FilesystemContentReader())
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{windowsPath}, imports)
	assert.NotContains(t, imports, osPath)
	assert.NotContains(t, imports, posixPath)
}

func TestResolveZigProjectImportsIncludesDirectStdTargetReferences(t *testing.T) {
	tmpDir := t.TempDir()

	stdPath := filepath.Join(tmpDir, "lib", "std", "std.zig")
	targetPath := filepath.Join(tmpDir, "lib", "std", "Target.zig")
	libCDirsPath := filepath.Join(tmpDir, "lib", "std", "zig", "LibCDirs.zig")
	llvmPath := filepath.Join(tmpDir, "src", "codegen", "llvm.zig")

	writeFile(t, stdPath, `pub const Target = @import("Target.zig");`)
	writeFile(t, targetPath, `pub const Abi = enum { none, gnu };`)
	writeFile(t, libCDirsPath, `
const std = @import("../std.zig");

pub fn detect(target: *const std.Target) void {
    _ = target;
}
`)
	writeFile(t, llvmPath, `
const std = @import("std");

pub fn targetTriple(target: *const std.Target) void {
    _ = target;
}
`)

	suppliedFiles := map[string]bool{
		targetPath:   true,
		libCDirsPath: true,
		llvmPath:     true,
	}

	imports, err := ResolveZigProjectImports(libCDirsPath, libCDirsPath, ".zig", suppliedFiles, vcs.FilesystemContentReader())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{targetPath}, imports)

	imports, err = ResolveZigProjectImports(llvmPath, llvmPath, ".zig", suppliedFiles, vcs.FilesystemContentReader())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{targetPath}, imports)
}

func TestResolveZigProjectImportsThroughPackageRootReexports(t *testing.T) {
	tmpDir := t.TempDir()

	bunRootPath := filepath.Join(tmpDir, "src", "bun.zig")
	outputPath := filepath.Join(tmpDir, "src", "bun_core", "output.zig")
	sourceMapPath := filepath.Join(tmpDir, "src", "sourcemap", "sourcemap.zig")
	internalSourceMapPath := filepath.Join(tmpDir, "src", "sourcemap", "InternalSourceMap.zig")
	consumerPath := filepath.Join(tmpDir, "src", "sourcemap_jsc", "internal_jsc.zig")

	writeFile(t, bunRootPath, `
pub const Output = @import("./bun_core/output.zig");
pub const SourceMap = @import("./sourcemap/sourcemap.zig");
`)
	writeFile(t, outputPath, `pub const prettyFmt = struct {};`)
	writeFile(t, sourceMapPath, `pub const InternalSourceMap = @import("./InternalSourceMap.zig").InternalSourceMap;`)
	writeFile(t, internalSourceMapPath, `pub const InternalSourceMap = struct {};`)
	writeFile(t, consumerPath, `
const bun = @import("bun");
const Output = bun.Output;
const InternalSourceMap = bun.SourceMap.InternalSourceMap;
`)

	suppliedFiles := map[string]bool{
		bunRootPath:           true,
		outputPath:            true,
		sourceMapPath:         true,
		internalSourceMapPath: true,
		consumerPath:          true,
	}

	imports, err := ResolveZigProjectImports(consumerPath, consumerPath, ".zig", suppliedFiles, vcs.FilesystemContentReader())
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{bunRootPath, outputPath, internalSourceMapPath}, imports)
}

func TestResolveZigProjectImportsIndexesMemberReexports(t *testing.T) {
	tmpDir := t.TempDir()

	uwsPath := filepath.Join(tmpDir, "src", "uws", "uws.zig")
	socketPath := filepath.Join(tmpDir, "src", "uws_sys", "socket.zig")
	consumerPath := filepath.Join(tmpDir, "src", "runtime", "socket", "socket.zig")

	writeFile(t, uwsPath, `pub const SocketTLS = @import("../uws_sys/socket.zig").SocketTLS;`)
	writeFile(t, socketPath, `pub const SocketTLS = struct {};`)
	writeFile(t, consumerPath, `
const uws = @import("../../uws/uws.zig");
const SocketTLS = uws.SocketTLS;
`)

	suppliedFiles := map[string]bool{
		uwsPath:      true,
		socketPath:   true,
		consumerPath: true,
	}

	imports, err := ResolveZigProjectImports(consumerPath, consumerPath, ".zig", suppliedFiles, vcs.FilesystemContentReader())
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{uwsPath, socketPath}, imports)
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
