package languages

import (
	"bytes"
	"testing"

	"github.com/LegacyCodeHQ/clarity/internal/testhelpers"
)

func TestLanguagesCommand_PrintsSupportedLanguagesAndExtensions(t *testing.T) {
	cmd := NewCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), out.Bytes())
}
