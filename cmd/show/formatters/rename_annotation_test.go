package formatters

import "testing"

func TestRenameAnnotation(t *testing.T) {
	const base = "/repo"
	tests := []struct {
		name    string
		oldPath string
		newPath string
		want    string
	}{
		{
			name:    "rename keeps directory, changes basename",
			oldPath: "/repo/cmd/graph_cmd.go",
			newPath: "/repo/cmd/show_cmd.go",
			want:    "(from graph_cmd.go)",
		},
		{
			name:    "rename at repo root",
			oldPath: "/repo/usage_clarity.md",
			newPath: "/repo/usage-clarity.md",
			want:    "(from usage_clarity.md)",
		},
		{
			name:    "move keeps basename, changes directory",
			oldPath: "/repo/tree_sitter_external/dart/binding.go",
			newPath: "/repo/internal/tree_sitter/dart/binding.go",
			want:    "(from tree_sitter_external/dart/)",
		},
		{
			name:    "move and rename change both",
			oldPath: "/repo/parser/foo.go",
			newPath: "/repo/parsers/bar.go",
			want:    "(from parser/foo.go)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renameAnnotation(tt.oldPath, tt.newPath, base); got != tt.want {
				t.Fatalf("renameAnnotation(%q, %q) = %q, want %q", tt.oldPath, tt.newPath, got, tt.want)
			}
		})
	}
}
