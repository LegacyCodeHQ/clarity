package golang

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGoImports_BasicImports(t *testing.T) {
	source := `
package main

import "fmt"
import "os"
import "io"

func main() {
	fmt.Println("Hello")
}
`
	imports, err := ParseGoImports([]byte(source))

	require.NoError(t, err)
	assert.Len(t, imports, 3)

	assert.Contains(t, imports, StandardLibraryImport{"fmt"})
	assert.Contains(t, imports, StandardLibraryImport{"os"})
	assert.Contains(t, imports, StandardLibraryImport{"io"})
}

func TestParseGoImports_MultipleImports(t *testing.T) {
	source := `
package main

import (
	"fmt"
	"os"
	"net/http"
	"encoding/json"
)

func main() {}
`
	imports, err := ParseGoImports([]byte(source))

	require.NoError(t, err)
	assert.Len(t, imports, 4)

	assert.Contains(t, imports, StandardLibraryImport{"fmt"})
	assert.Contains(t, imports, StandardLibraryImport{"os"})
	assert.Contains(t, imports, StandardLibraryImport{"net/http"})
	assert.Contains(t, imports, StandardLibraryImport{"encoding/json"})
}

func TestParseGoImports_ExternalImports(t *testing.T) {
	source := `
package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func main() {}
`
	imports, err := ParseGoImports([]byte(source))

	require.NoError(t, err)
	assert.Len(t, imports, 3)

	assert.Contains(t, imports, StandardLibraryImport{"fmt"})
	assert.Contains(t, imports, ExternalImport{"github.com/spf13/cobra"})
	assert.Contains(t, imports, ExternalImport{"github.com/stretchr/testify/assert"})
}

func TestParseGoImports_InternalImports(t *testing.T) {
	source := `
package main

import (
	"fmt"
	"clarity/parsers"
	"clarity/cmd"
)

func main() {}
`
	imports, err := ParseGoImports([]byte(source))

	require.NoError(t, err)
	assert.Len(t, imports, 3)

	assert.Contains(t, imports, StandardLibraryImport{"fmt"})
	assert.Contains(t, imports, InternalImport{"clarity/parsers"})
	assert.Contains(t, imports, InternalImport{"clarity/cmd"})
}

func TestParseGoImports_AliasedImports(t *testing.T) {
	source := `
package main

import (
	f "fmt"
	h "net/http"
	. "os"
)

func main() {}
`
	imports, err := ParseGoImports([]byte(source))

	require.NoError(t, err)
	assert.Len(t, imports, 3)

	assert.Contains(t, imports, StandardLibraryImport{"fmt"})
	assert.Contains(t, imports, StandardLibraryImport{"net/http"})
	assert.Contains(t, imports, StandardLibraryImport{"os"})
}

func TestParseGoImports_BlankImports(t *testing.T) {
	source := `
package main

import (
	"fmt"
	_ "database/sql"
	_ "github.com/lib/pq"
)

func main() {}
`
	imports, err := ParseGoImports([]byte(source))

	require.NoError(t, err)
	assert.Len(t, imports, 3)

	assert.Contains(t, imports, StandardLibraryImport{"fmt"})
	assert.Contains(t, imports, StandardLibraryImport{"database/sql"})
	assert.Contains(t, imports, ExternalImport{"github.com/lib/pq"})
}

func TestParseGoImports_EmptyFile(t *testing.T) {
	source := ``
	imports, err := ParseGoImports([]byte(source))

	require.NoError(t, err)
	assert.Empty(t, imports)
}

func TestParseGoImports_NoImports(t *testing.T) {
	source := `
package main

func main() {
	println("No imports here")
}
`
	imports, err := ParseGoImports([]byte(source))

	require.NoError(t, err)
	assert.Empty(t, imports)
}

func TestParseGoImports_InvalidGoCode(t *testing.T) {
	source := `
this is not valid go code @#$%^
`
	// Should not panic, might return empty or error
	imports, err := ParseGoImports([]byte(source))

	// Either error or empty result is acceptable
	if err == nil {
		assert.NotNil(t, imports)
	}
}

func TestGoImports_FileNotFound(t *testing.T) {
	_, err := GoImports("/nonexistent/file/path.go")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read file")
}

func TestGoImports_ValidFile(t *testing.T) {
	// Create a temporary Go file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.go")

	content := `
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("Hello, World!")
}
`
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	// Extract imports
	imports, err := GoImports(tmpFile)

	require.NoError(t, err)
	assert.Len(t, imports, 2)

	assert.Contains(t, imports, StandardLibraryImport{"fmt"})
	assert.Contains(t, imports, StandardLibraryImport{"os"})
}

func TestParseGoImports_ComplexExample(t *testing.T) {
	source := `
// This is a comment
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/smacker/go-tree-sitter"
	"github.com/spf13/cobra"

	"clarity/parsers"
	"clarity/cmd"
)

func main() {
	fmt.Println("Complex example")
}
`
	imports, err := ParseGoImports([]byte(source))

	require.NoError(t, err)
	assert.Len(t, imports, 8)

	// Standard library imports
	assert.Contains(t, imports, StandardLibraryImport{"context"})
	assert.Contains(t, imports, StandardLibraryImport{"fmt"})
	assert.Contains(t, imports, StandardLibraryImport{"net/http"})
	assert.Contains(t, imports, StandardLibraryImport{"os"})

	// External imports
	assert.Contains(t, imports, ExternalImport{"github.com/smacker/go-tree-sitter"})
	assert.Contains(t, imports, ExternalImport{"github.com/spf13/cobra"})

	// Internal imports
	assert.Contains(t, imports, InternalImport{"clarity/parsers"})
	assert.Contains(t, imports, InternalImport{"clarity/cmd"})
}

func TestParseGoImports_MixedStandardAndExternal(t *testing.T) {
	source := `
package main

import (
	"context"
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

func main() {}
`
	imports, err := ParseGoImports([]byte(source))

	require.NoError(t, err)
	assert.Len(t, imports, 4)

	assert.Contains(t, imports, StandardLibraryImport{"context"})
	assert.Contains(t, imports, StandardLibraryImport{"fmt"})
	assert.Contains(t, imports, ExternalImport{"github.com/smacker/go-tree-sitter"})
	assert.Contains(t, imports, ExternalImport{"github.com/smacker/go-tree-sitter/golang"})
}

func TestParseGoEmbeds_SingleFile(t *testing.T) {
	source := `
package main

import _ "embed"

//go:embed README.md
var readme string

func main() {}
`
	embeds, err := ParseGoEmbeds([]byte(source))

	require.NoError(t, err)
	assert.Len(t, embeds, 1)
	assert.Equal(t, "README.md", embeds[0].Pattern)
}

func TestParseGoEmbeds_MultipleEmbeds(t *testing.T) {
	source := `
package main

import _ "embed"

//go:embed config.json
var config string

//go:embed templates/index.html
var indexTemplate string

func main() {}
`
	embeds, err := ParseGoEmbeds([]byte(source))

	require.NoError(t, err)
	assert.Len(t, embeds, 2)
	assert.Equal(t, "config.json", embeds[0].Pattern)
	assert.Equal(t, "templates/index.html", embeds[1].Pattern)
}

func TestParseGoEmbeds_MultiplePatternsSingleLine(t *testing.T) {
	source := `
package main

import "embed"

//go:embed file1.txt file2.txt file3.txt
var content embed.FS

func main() {}
`
	embeds, err := ParseGoEmbeds([]byte(source))

	require.NoError(t, err)
	assert.Len(t, embeds, 3)
	assert.Equal(t, "file1.txt", embeds[0].Pattern)
	assert.Equal(t, "file2.txt", embeds[1].Pattern)
	assert.Equal(t, "file3.txt", embeds[2].Pattern)
}

func TestParseGoEmbeds_GlobPattern(t *testing.T) {
	source := `
package main

import "embed"

//go:embed templates/*.html
var templates embed.FS

func main() {}
`
	embeds, err := ParseGoEmbeds([]byte(source))

	require.NoError(t, err)
	assert.Len(t, embeds, 1)
	assert.Equal(t, "templates/*.html", embeds[0].Pattern)
}

func TestParseGoEmbeds_AllPrefix(t *testing.T) {
	source := `
package main

import "embed"

//go:embed all:templates
var templates embed.FS

func main() {}
`
	embeds, err := ParseGoEmbeds([]byte(source))

	require.NoError(t, err)
	assert.Len(t, embeds, 1)
	// The all: prefix should be stripped
	assert.Equal(t, "templates", embeds[0].Pattern)
}

func TestParseGoEmbeds_NoEmbeds(t *testing.T) {
	source := `
package main

import "fmt"

func main() {
	fmt.Println("No embeds here")
}
`
	embeds, err := ParseGoEmbeds([]byte(source))

	require.NoError(t, err)
	assert.Empty(t, embeds)
}

func TestParseGoEmbeds_EmptyFile(t *testing.T) {
	source := ``
	embeds, err := ParseGoEmbeds([]byte(source))

	require.NoError(t, err)
	assert.Empty(t, embeds)
}

func TestParseGoEmbeds_RegularComments(t *testing.T) {
	source := `
package main

import "fmt"

// This is a regular comment
// go:embed is not at the start, so it's not an embed directive
// Not a directive: go:embed fake.txt

func main() {
	fmt.Println("Hello")
}
`
	embeds, err := ParseGoEmbeds([]byte(source))

	require.NoError(t, err)
	assert.Empty(t, embeds)
}
