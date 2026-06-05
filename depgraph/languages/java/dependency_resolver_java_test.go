package java

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveJavaProjectImports(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src", "main", "java", "com", "example")
	utilDir := filepath.Join(srcDir, "util")
	require.NoError(t, os.MkdirAll(utilDir, 0o755))

	appPath := filepath.Join(srcDir, "App.java")
	require.NoError(t, os.WriteFile(appPath, []byte(`package com.example;

import com.example.util.Helper;
import java.util.List;

public class App {}
`), 0o644))

	helperPath := filepath.Join(utilDir, "Helper.java")
	require.NoError(t, os.WriteFile(helperPath, []byte(`package com.example.util;

public class Helper {}
`), 0o644))

	reader := vcs.FilesystemContentReader()
	indicesFiles := []string{appPath, helperPath}
	pkgIndex, typeIndex, filePackages := BuildJavaIndices(indicesFiles, reader)
	supplied := map[string]bool{
		appPath:    true,
		helperPath: true,
	}

	imports, err := ResolveJavaProjectImports(appPath, appPath, pkgIndex, typeIndex, filePackages, supplied, reader)
	require.NoError(t, err)
	assert.Equal(t, []string{helperPath}, imports)
}

func TestResolveJavaProjectImports_SamePackageInference(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src", "main", "java", "com", "example", "model")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	cartPath := filepath.Join(srcDir, "Cart.java")
	require.NoError(t, os.WriteFile(cartPath, []byte(`package com.example.model;

public class Cart {
    private PaymentMethod paymentMethod;
}
`), 0o644))

	paymentPath := filepath.Join(srcDir, "PaymentMethod.java")
	require.NoError(t, os.WriteFile(paymentPath, []byte(`package com.example.model;

public class PaymentMethod {}
`), 0o644))

	reader := vcs.FilesystemContentReader()
	files := []string{cartPath, paymentPath}
	pkgIndex, typeIndex, filePackages := BuildJavaIndices(files, reader)
	supplied := map[string]bool{
		cartPath:    true,
		paymentPath: true,
	}

	imports, err := ResolveJavaProjectImports(cartPath, cartPath, pkgIndex, typeIndex, filePackages, supplied, reader)
	require.NoError(t, err)
	assert.Contains(t, imports, paymentPath)
}

func TestResolveJavaProjectImports_SamePackageFieldAccessInference(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src", "main", "java", "com", "example", "sql")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	supportPath := filepath.Join(srcDir, "SqlTypesSupport.java")
	require.NoError(t, os.WriteFile(supportPath, []byte(`package com.example.sql;

public final class SqlTypesSupport {
    static final Object DATE_FACTORY = SqlDateTypeAdapter.FACTORY;
    static final Object TIME_FACTORY = SqlTimeTypeAdapter.FACTORY;
}
`), 0o644))

	datePath := filepath.Join(srcDir, "SqlDateTypeAdapter.java")
	require.NoError(t, os.WriteFile(datePath, []byte(`package com.example.sql;

final class SqlDateTypeAdapter {
    static final Object FACTORY = new Object();
}
`), 0o644))

	timePath := filepath.Join(srcDir, "SqlTimeTypeAdapter.java")
	require.NoError(t, os.WriteFile(timePath, []byte(`package com.example.sql;

final class SqlTimeTypeAdapter {
    static final Object FACTORY = new Object();
}
`), 0o644))

	reader := vcs.FilesystemContentReader()
	files := []string{supportPath, datePath, timePath}
	pkgIndex, typeIndex, filePackages := BuildJavaIndices(files, reader)
	supplied := map[string]bool{
		supportPath: true,
		datePath:    true,
		timePath:    true,
	}

	imports, err := ResolveJavaProjectImports(
		supportPath,
		supportPath,
		pkgIndex,
		typeIndex,
		filePackages,
		supplied,
		reader)
	require.NoError(t, err)
	assert.Contains(t, imports, datePath)
	assert.Contains(t, imports, timePath)
}

func TestResolveJavaProjectImports_MissingExplicitTypeDoesNotResolveWholePackage(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src", "main", "java", "com", "example")
	internalDir := filepath.Join(srcDir, "internal")
	require.NoError(t, os.MkdirAll(internalDir, 0o755))

	helperPath := filepath.Join(srcDir, "reflect", "ReflectionHelper.java")
	require.NoError(t, os.MkdirAll(filepath.Dir(helperPath), 0o755))
	require.NoError(t, os.WriteFile(helperPath, []byte(`package com.example.reflect;

import com.example.internal.GeneratedBuildConfig;

public class ReflectionHelper {
    String version() {
        return GeneratedBuildConfig.VERSION;
    }
}
`), 0o644))

	troubleshootingPath := filepath.Join(internalDir, "TroubleshootingGuide.java")
	require.NoError(t, os.WriteFile(troubleshootingPath, []byte(`package com.example.internal;

public final class TroubleshootingGuide {}
`), 0o644))

	typesPath := filepath.Join(internalDir, "GsonTypes.java")
	require.NoError(t, os.WriteFile(typesPath, []byte(`package com.example.internal;

public final class GsonTypes {}
`), 0o644))

	reader := vcs.FilesystemContentReader()
	files := []string{helperPath, troubleshootingPath, typesPath}
	pkgIndex, typeIndex, filePackages := BuildJavaIndices(files, reader)
	supplied := map[string]bool{
		helperPath:          true,
		troubleshootingPath: true,
		typesPath:           true,
	}

	imports, err := ResolveJavaProjectImports(
		helperPath,
		helperPath,
		pkgIndex,
		typeIndex,
		filePackages,
		supplied,
		reader)
	require.NoError(t, err)
	assert.Empty(t, imports)
}

func TestResolveJavaProjectImports_StaticMemberImportResolvesDeclaringType(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src", "main", "java", "com", "example", "stream")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	writerPath := filepath.Join(srcDir, "JsonWriter.java")
	require.NoError(t, os.WriteFile(writerPath, []byte(`package com.example.stream;

import static com.example.stream.JsonScope.EMPTY_ARRAY;

public class JsonWriter {
    int scope() {
        return EMPTY_ARRAY;
    }
}
`), 0o644))

	scopePath := filepath.Join(srcDir, "JsonScope.java")
	require.NoError(t, os.WriteFile(scopePath, []byte(`package com.example.stream;

final class JsonScope {
    static final int EMPTY_ARRAY = 1;
}
`), 0o644))

	reader := vcs.FilesystemContentReader()
	files := []string{writerPath, scopePath}
	pkgIndex, typeIndex, filePackages := BuildJavaIndices(files, reader)
	supplied := map[string]bool{
		writerPath: true,
		scopePath:  true,
	}

	imports, err := ResolveJavaProjectImports(
		writerPath,
		writerPath,
		pkgIndex,
		typeIndex,
		filePackages,
		supplied,
		reader)
	require.NoError(t, err)
	assert.Contains(t, imports, scopePath)
}

func TestResolveJavaProjectImports_SamePackageAnnotationInference(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src", "main", "java", "com", "example", "base")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	functionPath := filepath.Join(srcDir, "Function.java")
	require.NoError(t, os.WriteFile(functionPath, []byte(`package com.example.base;

public interface Function<F, T> {
    @ParametricNullness
    T apply(@ParametricNullness F input);
}
`), 0o644))

	annotationPath := filepath.Join(srcDir, "ParametricNullness.java")
	require.NoError(t, os.WriteFile(annotationPath, []byte(`package com.example.base;

@interface ParametricNullness {}
`), 0o644))

	reader := vcs.FilesystemContentReader()
	files := []string{functionPath, annotationPath}
	pkgIndex, typeIndex, filePackages := BuildJavaIndices(files, reader)
	supplied := map[string]bool{
		functionPath:   true,
		annotationPath: true,
	}

	imports, err := ResolveJavaProjectImports(
		functionPath,
		functionPath,
		pkgIndex,
		typeIndex,
		filePackages,
		supplied,
		reader)
	require.NoError(t, err)
	assert.Contains(t, imports, annotationPath)
}
