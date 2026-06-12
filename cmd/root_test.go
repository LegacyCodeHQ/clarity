package cmd

import "testing"

func TestRootCommand_AlwaysRegistersWatch(t *testing.T) {
	t.Parallel()

	for _, c := range rootCmd.Commands() {
		if c.Name() == "watch" {
			return
		}
	}

	t.Fatal("expected watch command to be registered")
}
