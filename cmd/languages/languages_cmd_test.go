package languages

import (
	"bytes"
	"testing"
)

func TestLanguagesCommand_PrintsSupportedLanguagesAndExtensions(t *testing.T) {
	cmd := NewCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	expected := `Language    Extensions
--------    ----------
Dart        .dart
Go          .go
Java        .java
Kotlin      .kt, .kts
TypeScript  .ts, .tsx
`

	if out.String() != expected {
		t.Fatalf("output = %q, want %q", out.String(), expected)
	}
}
