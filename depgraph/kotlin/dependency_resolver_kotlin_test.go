package kotlin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveKotlinSamePackageDependencies_IgnoresSelfDeclaredTopLevelTypes(t *testing.T) {
	tmpDir := t.TempDir()

	commonDir := filepath.Join(tmpDir, "src", "commonMain", "kotlin", "io", "github", "oshai", "kotlinlogging")
	jvmDir := filepath.Join(tmpDir, "src", "jvmMain", "kotlin", "io", "github", "oshai", "kotlinlogging")
	require.NoError(t, os.MkdirAll(commonDir, 0o755))
	require.NoError(t, os.MkdirAll(jvmDir, 0o755))

	commonConfig := filepath.Join(commonDir, "KotlinLoggingConfiguration.kt")
	jvmConfig := filepath.Join(jvmDir, "KotlinLoggingConfiguration.kt")
	levelFile := filepath.Join(commonDir, "Level.kt")

	require.NoError(t, os.WriteFile(commonConfig, []byte(`
package io.github.oshai.kotlinlogging

expect object KotlinLoggingConfiguration {
  var logLevel: Level
}
`), 0o644))
	require.NoError(t, os.WriteFile(jvmConfig, []byte(`
package io.github.oshai.kotlinlogging

actual object KotlinLoggingConfiguration {
  actual var logLevel: Level = Level.INFO
}
`), 0o644))
	require.NoError(t, os.WriteFile(levelFile, []byte(`
package io.github.oshai.kotlinlogging

enum class Level { INFO, DEBUG }
`), 0o644))

	contentReader := vcs.FilesystemContentReader()
	kotlinFiles := []string{commonConfig, jvmConfig, levelFile}
	packageIndex, packageTypes, filePackages := BuildKotlinIndices(kotlinFiles, contentReader)

	suppliedFiles := map[string]bool{
		commonConfig: true,
		jvmConfig:    true,
		levelFile:    true,
	}

	deps, err := ResolveKotlinProjectImports(
		jvmConfig,
		jvmConfig,
		packageIndex,
		packageTypes,
		filePackages,
		suppliedFiles,
		contentReader)
	require.NoError(t, err)
	assert.Contains(t, deps, levelFile)
	assert.NotContains(t, deps, commonConfig)
}

func TestResolveKotlinProjectImports_SpecificImportResolvesOnlyImportedSymbol(t *testing.T) {
	tmpDir := t.TempDir()
	internalDir := filepath.Join(tmpDir, "src", "darwinMain", "kotlin", "io", "github", "oshai", "kotlinlogging", "internal")
	mainDir := filepath.Join(tmpDir, "src", "darwinMain", "kotlin", "io", "github", "oshai", "kotlinlogging")
	require.NoError(t, os.MkdirAll(internalDir, 0o755))
	require.NoError(t, os.MkdirAll(mainDir, 0o755))

	darwinKLogger := filepath.Join(mainDir, "DarwinKLogger.kt")
	darwinFormatter := filepath.Join(internalDir, "DarwinFormatter.kt")
	darwinFormatterTest := filepath.Join(internalDir, "DarwinFormatterTest.kt")

	require.NoError(t, os.WriteFile(darwinKLogger, []byte(`
package io.github.oshai.kotlinlogging

import io.github.oshai.kotlinlogging.internal.DarwinFormatter

class DarwinKLogger {
  fun at() {
    DarwinFormatter.getFormattedMessage()
  }
}
`), 0o644))
	require.NoError(t, os.WriteFile(darwinFormatter, []byte(`
package io.github.oshai.kotlinlogging.internal

object DarwinFormatter {
  fun getFormattedMessage(): String = "ok"
}
`), 0o644))
	require.NoError(t, os.WriteFile(darwinFormatterTest, []byte(`
package io.github.oshai.kotlinlogging.internal

class DarwinFormatterTest
`), 0o644))

	contentReader := vcs.FilesystemContentReader()
	kotlinFiles := []string{darwinKLogger, darwinFormatter, darwinFormatterTest}
	packageIndex, packageTypes, filePackages := BuildKotlinIndices(kotlinFiles, contentReader)
	suppliedFiles := map[string]bool{
		darwinKLogger:       true,
		darwinFormatter:     true,
		darwinFormatterTest: true,
	}

	deps, err := ResolveKotlinProjectImports(
		darwinKLogger,
		darwinKLogger,
		packageIndex,
		packageTypes,
		filePackages,
		suppliedFiles,
		contentReader)
	require.NoError(t, err)
	assert.Contains(t, deps, darwinFormatter)
	assert.NotContains(t, deps, darwinFormatterTest)
}

func TestResolveKotlinProjectImports_WildcardImportResolvesOnlyUsedSymbols(t *testing.T) {
	tmpDir := t.TempDir()
	internalDir := filepath.Join(tmpDir, "src", "main", "kotlin", "com", "example", "internal")
	mainDir := filepath.Join(tmpDir, "src", "main", "kotlin", "com", "example")
	require.NoError(t, os.MkdirAll(internalDir, 0o755))
	require.NoError(t, os.MkdirAll(mainDir, 0o755))

	mainFile := filepath.Join(mainDir, "Main.kt")
	usedType := filepath.Join(internalDir, "DarwinFormatter.kt")
	unusedType := filepath.Join(internalDir, "UnusedType.kt")

	require.NoError(t, os.WriteFile(mainFile, []byte(`
package com.example

import com.example.internal.*

class Main {
  fun run() {
    DarwinFormatter.getFormattedMessage()
  }
}
`), 0o644))
	require.NoError(t, os.WriteFile(usedType, []byte(`
package com.example.internal

object DarwinFormatter {
  fun getFormattedMessage(): String = "ok"
}
`), 0o644))
	require.NoError(t, os.WriteFile(unusedType, []byte(`
package com.example.internal

class UnusedType
`), 0o644))

	contentReader := vcs.FilesystemContentReader()
	kotlinFiles := []string{mainFile, usedType, unusedType}
	packageIndex, packageTypes, filePackages := BuildKotlinIndices(kotlinFiles, contentReader)
	suppliedFiles := map[string]bool{
		mainFile:   true,
		usedType:   true,
		unusedType: true,
	}

	deps, err := ResolveKotlinProjectImports(
		mainFile,
		mainFile,
		packageIndex,
		packageTypes,
		filePackages,
		suppliedFiles,
		contentReader)
	require.NoError(t, err)
	assert.Contains(t, deps, usedType)
	assert.NotContains(t, deps, unusedType)
}

func TestResolveKotlinProjectImports_SkipsAmbiguousTypeDefinitions(t *testing.T) {
	tmpDir := t.TempDir()
	commonDir := filepath.Join(tmpDir, "src", "commonMain", "kotlin", "com", "example")
	jvmDir := filepath.Join(tmpDir, "src", "jvmMain", "kotlin", "com", "example")
	require.NoError(t, os.MkdirAll(commonDir, 0o755))
	require.NoError(t, os.MkdirAll(jvmDir, 0o755))

	mainFile := filepath.Join(commonDir, "Main.kt")
	commonConfig := filepath.Join(commonDir, "KotlinLoggingConfiguration.kt")
	jvmConfig := filepath.Join(jvmDir, "KotlinLoggingConfiguration.kt")

	require.NoError(t, os.WriteFile(mainFile, []byte(`
package com.example

class Main {
  fun f() {
    KotlinLoggingConfiguration.configure()
  }
}
`), 0o644))
	require.NoError(t, os.WriteFile(commonConfig, []byte(`
package com.example

expect object KotlinLoggingConfiguration {
  fun configure()
}
`), 0o644))
	require.NoError(t, os.WriteFile(jvmConfig, []byte(`
package com.example

actual object KotlinLoggingConfiguration {
  actual fun configure() {}
}
`), 0o644))

	contentReader := vcs.FilesystemContentReader()
	kotlinFiles := []string{mainFile, commonConfig, jvmConfig}
	packageIndex, packageTypes, filePackages := BuildKotlinIndices(kotlinFiles, contentReader)
	suppliedFiles := map[string]bool{
		mainFile:     true,
		commonConfig: true,
		jvmConfig:    true,
	}

	deps, err := ResolveKotlinProjectImports(
		mainFile,
		mainFile,
		packageIndex,
		packageTypes,
		filePackages,
		suppliedFiles,
		contentReader)
	require.NoError(t, err)
	assert.NotContains(t, deps, commonConfig)
	assert.NotContains(t, deps, jvmConfig)
}
