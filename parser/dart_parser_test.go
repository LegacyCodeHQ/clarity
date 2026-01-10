package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseImports_BasicImports(t *testing.T) {
	source := `
		import 'dart:io';
		import 'dart:async';
		import 'package:flutter/material.dart';
		
		void main() {
		  print('Hello');
		}
`
	imports, err := ParseImports([]byte(source))

	require.NoError(t, err)
	assert.Len(t, imports, 3)
	assert.Contains(t, imports, "dart:io")
	assert.Contains(t, imports, "dart:async")
	assert.Contains(t, imports, "package:flutter/material.dart")
}

func TestParseImports_WithPrefixes(t *testing.T) {
	source := `
		import 'package:lib1/lib1.dart' as lib1;
		import 'package:lib2/lib2.dart' as lib2;
`
	imports, err := ParseImports([]byte(source))

	require.NoError(t, err)
	assert.Len(t, imports, 2)
	assert.Contains(t, imports, "package:lib1/lib1.dart")
	assert.Contains(t, imports, "package:lib2/lib2.dart")
}

func TestParseImports_WithShowHide(t *testing.T) {
	source := `
		import 'package:lib1/lib1.dart' show foo, bar;
		import 'package:lib2/lib2.dart' hide baz;
`
	imports, err := ParseImports([]byte(source))

	require.NoError(t, err)
	assert.Len(t, imports, 2)
	assert.Contains(t, imports, "package:lib1/lib1.dart")
	assert.Contains(t, imports, "package:lib2/lib2.dart")
}

func TestParseImports_RelativePaths(t *testing.T) {
	source := `
		import 'src/helper.dart';
		import '../utils/common.dart';
		import 'models/user.dart';
`
	imports, err := ParseImports([]byte(source))

	require.NoError(t, err)
	assert.Len(t, imports, 3)
	assert.Contains(t, imports, "src/helper.dart")
	assert.Contains(t, imports, "../utils/common.dart")
	assert.Contains(t, imports, "models/user.dart")
}

func TestParseImports_EmptyFile(t *testing.T) {
	source := ``
	imports, err := ParseImports([]byte(source))

	require.NoError(t, err)
	assert.Empty(t, imports)
}

func TestParseImports_NoImports(t *testing.T) {
	source := `
		void main() {
		  print('No imports here');
		}
`
	imports, err := ParseImports([]byte(source))

	require.NoError(t, err)
	assert.Empty(t, imports)
}

func TestParseImports_MixedQuotes(t *testing.T) {
	source := `
		import 'dart:io';
		import "package:flutter/material.dart";
`
	imports, err := ParseImports([]byte(source))

	require.NoError(t, err)
	assert.Len(t, imports, 2)
	assert.Contains(t, imports, "dart:io")
	assert.Contains(t, imports, "package:flutter/material.dart")
}

func TestParseImports_InvalidDartCode(t *testing.T) {
	source := `
		this is not valid dart code @#$%^
`
	// Should not panic, might return empty or error
	imports, err := ParseImports([]byte(source))

	// Either error or empty result is acceptable
	if err == nil {
		assert.NotNil(t, imports)
	}
}

func TestExtractImports_FileNotFound(t *testing.T) {
	_, err := ExtractImports("/nonexistent/file/path.dart")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read file")
}

func TestExtractImports_ValidFile(t *testing.T) {
	// Create a temporary Dart file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.dart")

	content := `
		import 'dart:io';
		import 'package:flutter/material.dart';
		
		void main() {}
`
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	// Extract imports
	imports, err := ExtractImports(tmpFile)

	require.NoError(t, err)
	assert.Len(t, imports, 2)
	assert.Contains(t, imports, "dart:io")
	assert.Contains(t, imports, "package:flutter/material.dart")
}

func TestParseImports_ComplexExample(t *testing.T) {
	source := `
		// This is a comment
		import 'dart:io';
		import 'dart:async';
		import 'package:flutter/material.dart';
		import 'package:provider/provider.dart' as provider;
		import 'src/models/user.dart';
		import '../utils/helper.dart' show formatDate, formatTime;
		import 'services/api.dart' hide privateFunction;
		
		class MyApp extends StatelessWidget {
		  @override
		  Widget build(BuildContext context) {
			return MaterialApp(
			  title: 'My App',
			);
		  }
		}
`
	imports, err := ParseImports([]byte(source))

	require.NoError(t, err)
	assert.Len(t, imports, 7)
	assert.Contains(t, imports, "dart:io")
	assert.Contains(t, imports, "dart:async")
	assert.Contains(t, imports, "package:flutter/material.dart")
	assert.Contains(t, imports, "package:provider/provider.dart")
	assert.Contains(t, imports, "src/models/user.dart")
	assert.Contains(t, imports, "../utils/helper.dart")
	assert.Contains(t, imports, "services/api.dart")
}
