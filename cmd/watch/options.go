package watch

import "github.com/LegacyCodeHQ/clarity/cmd/show/formatters"

type watchOptions struct {
	repoPath        string
	port            int
	direction       string
	format          string
	includeExt      string
	excludeExt      string
	includes        []string
	excludes        []string
	betweenFiles    []string
	moduleSelect    string
	moduleDirection string
	reach           string
	depthLevel      int
	pruneFiles      []string
	all             bool
	collapse        bool
	edgeLabels      bool
	noStats         bool
	noPhantom       bool
}

func defaultWatchOptions() *watchOptions {
	return &watchOptions{
		port:            4900,
		direction:       formatters.DefaultDirection.StringLower(),
		format:          formatters.OutputFormatDOT.String(),
		depthLevel:      1,
		moduleDirection: "none",
	}
}
