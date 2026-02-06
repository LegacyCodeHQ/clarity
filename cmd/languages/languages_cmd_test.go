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
C           .c, .h
C++         .cc, .cpp, .cxx, .hpp, .hh, .hxx
C#          .cs
Dart        .dart
Go          .go
JavaScript  .js, .jsx
Java        .java
Kotlin      .kt, .kts
Python      .py
Rust        .rs
Swift       .swift
TypeScript  .ts, .tsx
`

	if out.String() != expected {
		t.Fatalf("output = %q, want %q", out.String(), expected)
	}
}
