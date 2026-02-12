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

func TestIsDevelopmentBuild(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		devCommandsFlag string
		want            bool
	}{
		{name: "flag disabled", devCommandsFlag: "false", want: false},
		{name: "flag enabled", devCommandsFlag: "true", want: true},
		{name: "invalid flag", devCommandsFlag: "not-a-bool", want: false},
		{name: "empty flag", devCommandsFlag: "", want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := isDevelopmentBuild(tc.devCommandsFlag); got != tc.want {
				t.Fatalf("isDevelopmentBuild(%q) = %v, want %v", tc.devCommandsFlag, got, tc.want)
			}
		})
	}
}
