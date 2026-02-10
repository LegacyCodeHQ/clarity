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
