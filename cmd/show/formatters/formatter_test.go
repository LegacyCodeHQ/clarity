package formatters

import "testing"

func TestNewFormatter_DOT(t *testing.T) {
	f, err := NewFormatter("dot")
	if err != nil {
		t.Fatalf("NewFormatter(dot) error = %v", err)
	}

	if _, ok := f.(dotFormatter); !ok {
		t.Fatalf("NewFormatter(dot) returned %T, want formatters.dotFormatter", f)
	}
}

func TestNewFormatter_Mermaid(t *testing.T) {
	f, err := NewFormatter("mermaid")
	if err != nil {
		t.Fatalf("NewFormatter(mermaid) error = %v", err)
	}

	if _, ok := f.(mermaidFormatter); !ok {
		t.Fatalf("NewFormatter(mermaid) returned %T, want formatters.mermaidFormatter", f)
	}
}

func TestNewFormatter_UnknownFormat(t *testing.T) {
	_, err := NewFormatter("unknown")
	if err == nil {
		t.Fatalf("NewFormatter(unknown) expected error, got nil")
	}
}
