package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LegacyCodeHQ/clarity/internal/testhelpers"
	"github.com/spf13/cobra"
)

func TestRunRoot_PrintsSplashForEmptyInvocation(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	if err := runRoot(cmd, nil); err != nil {
		t.Fatalf("runRoot() error = %v", err)
	}

	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), out.Bytes())
}

func TestRunRoot_PrintsHelpForFlagOnlyInvocation(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{
		Use:   "clarity",
		Short: "test command",
		RunE:  runRoot,
	}
	cmd.SetOut(&out)
	cmd.Flags().Bool("verbose", false, "Enable verbose/debug output")
	cmd.SetArgs([]string{"--verbose"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Usage:") {
		t.Fatalf("expected cobra usage in output, got:\n%s", output)
	}
	if strings.Contains(output, "Software design maps for AI-native development") {
		t.Fatalf("expected help instead of splash, got:\n%s", output)
	}
}
