package java

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePackageDeclaration(t *testing.T) {
	src := []byte(`package com.example.core;

public class App {}
`)
	assert.Equal(t, "com.example.core", ParsePackageDeclaration(src))
}

func TestParseJavaImports_ClassifiesInternalAndStandard(t *testing.T) {
	src := []byte(`package com.example;

import com.example.util.Helper;
import java.util.List;
import org.slf4j.Logger;
`)
	projectPackages := map[string]bool{
		"com.example":      true,
		"com.example.util": true,
	}

	imports := ParseJavaImports(src, projectPackages)
	require.Len(t, imports, 3)

	_, isInternal := imports[0].(InternalImport)
	_, isStandard := imports[1].(StandardLibraryImport)
	_, isExternal := imports[2].(ExternalImport)

	assert.True(t, isInternal)
	assert.True(t, isStandard)
	assert.True(t, isExternal)
}

func TestParseTopLevelTypeNames(t *testing.T) {
	src := []byte(`package com.example;

public class App {}
interface Service {}
enum Mode { ON, OFF }
record User(String name) {}
`)
	types := ParseTopLevelTypeNames(src)
	assert.ElementsMatch(t, []string{"App", "Service", "Mode", "User"}, types)
}

func TestIsTestFile(t *testing.T) {
	assert.True(t, IsTestFile("/project/src/test/java/com/example/AppTest.java"))
	assert.True(t, IsTestFile("/project/module/test/com/example/AppTests.java"))
	assert.False(t, IsTestFile("/project/src/main/java/com/example/App.java"))
}

func TestExtractTypeIdentifiers(t *testing.T) {
	src := []byte(`package com.example.model;

public class Cart {
    private PaymentMethod paymentMethod;
    // DeliveryOption should be ignored in comments
    private String note = "Money should be ignored in string";
}
`)

	identifiers := ExtractTypeIdentifiers(src)
	assert.Contains(t, identifiers, "Cart")
	assert.Contains(t, identifiers, "PaymentMethod")
	assert.Contains(t, identifiers, "String")
	assert.NotContains(t, identifiers, "DeliveryOption")
	assert.NotContains(t, identifiers, "Money")
}

func TestExtractTypeIdentifiers_StaticMethodCallQualifier(t *testing.T) {
	// A same-package type referenced only as the qualifier of a static method
	// call (e.g. `NumberLimits.parseBigDecimal(...)`) must still be extracted, so
	// a dependency edge is formed. Mirrors the existing static field-access rule.
	src := []byte(`package com.example.model;

public class LazilyParsedNumber {
    public java.math.BigDecimal bigDecimalValue(String value) {
        return NumberLimits.parseBigDecimal(value);
    }
}
`)

	identifiers := ExtractTypeIdentifiers(src)
	assert.Contains(t, identifiers, "NumberLimits")
}
