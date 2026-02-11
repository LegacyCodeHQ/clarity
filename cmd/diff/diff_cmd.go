package diff

import (
	"fmt"
	"strings"

	"github.com/LegacyCodeHQ/clarity/cmd/graph/formatters"
	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/LegacyCodeHQ/clarity/vcs/git"
	"github.com/spf13/cobra"
)

var snapshotSelectorFlags = []string{"staged", "unstaged", "untracked"}

type diffOptions struct {
	repoPath   string
	outputFmt  string
	summary    bool
	commitSpec string
}

// Cmd represents the diff command.
var Cmd = NewCommand()

// NewCommand returns a new diff command instance.
func NewCommand() *cobra.Command {
	opts := &diffOptions{
		outputFmt: formatters.OutputFormatDOT.String(),
	}

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show dependency-graph changes between snapshots.",
		Long:  "Show dependency-graph changes between snapshots.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiff(cmd, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.repoPath, "repo", "r", "", "Git repository path (default: current directory)")
	cmd.Flags().StringVarP(&opts.outputFmt, "format", "f", opts.outputFmt, fmt.Sprintf("Output format (%s)", formatters.SupportedFormats()))
	cmd.Flags().BoolVar(&opts.summary, "summary", false, "Print text summary only")
	cmd.Flags().StringVarP(&opts.commitSpec, "commit", "c", "", "Compare committed snapshots (<commit> or <A>,<B>)")

	// Reserved snapshot selectors for future working-tree controls.
	cmd.Flags().Bool("staged", false, "Include staged changes")
	_ = cmd.Flags().MarkHidden("staged")
	cmd.Flags().Bool("unstaged", false, "Include unstaged changes")
	_ = cmd.Flags().MarkHidden("unstaged")
	cmd.Flags().Bool("untracked", false, "Include untracked changes")
	_ = cmd.Flags().MarkHidden("untracked")

	return cmd
}

func runDiff(cmd *cobra.Command, opts *diffOptions) error {
	repoPath := opts.repoPath
	if repoPath == "" {
		repoPath = "."
	}

	comparison, err := resolveModeAndCommitComparison(cmd, repoPath, opts.commitSpec)
	if err != nil {
		return err
	}

	snapshots, err := resolveSnapshots(repoPath, comparison)
	if err != nil {
		return err
	}

	baseGraph, err := buildGraphFromSnapshot(snapshots.base)
	if err != nil {
		return fmt.Errorf("failed to build base dependency graph: %w", err)
	}
	targetGraph, err := buildGraphFromSnapshot(snapshots.target)
	if err != nil {
		return fmt.Errorf("failed to build target dependency graph: %w", err)
	}

	delta, err := buildGraphDelta(baseGraph, targetGraph)
	if err != nil {
		return err
	}
	delta.changedNodes, err = resolveChangedNodes(repoPath, comparison)
	if err != nil {
		return err
	}
	delta, err = applySemanticAnalyzers(baseGraph, targetGraph, delta, nil)
	if err != nil {
		return fmt.Errorf("failed to compute semantic findings: %w", err)
	}

	if opts.summary {
		fmt.Fprintln(cmd.OutOrStdout(), renderSummary(delta))
		return nil
	}

	out, err := renderDelta(opts.outputFmt, delta)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), out)

	return nil
}

func buildGraphFromSnapshot(s snapshot) (depgraph.DependencyGraph, error) {
	if len(s.filePaths) == 0 {
		return depgraph.NewDependencyGraph(), nil
	}
	if s.contentRead == nil {
		return nil, fmt.Errorf("content reader is required for non-empty snapshot %q", s.ref)
	}
	return depgraph.BuildDependencyGraph(s.filePaths, s.contentRead)
}

func resolveChangedNodes(repoPath string, comparison commitComparison) (map[string]struct{}, error) {
	var (
		changed []string
		err     error
	)

	switch comparison.mode {
	case diffModeWorkingTree:
		changed, err = git.GetUncommittedFiles(repoPath)
	case diffModeCommit:
		if comparison.baseRef != "" {
			changed, err = git.GetCommitRangeFiles(repoPath, comparison.baseRef, comparison.targetRef)
		} else {
			changed, err = git.GetCommitDartFiles(repoPath, comparison.targetRef)
		}
	default:
		return nil, fmt.Errorf("unknown diff mode: %s", comparison.mode)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to resolve changed files: %w", err)
	}

	changedSet := make(map[string]struct{}, len(changed))
	for _, path := range changed {
		changedSet[path] = struct{}{}
	}
	return changedSet, nil
}

func resolveModeAndCommitComparison(cmd *cobra.Command, repoPath, commitSpec string) (commitComparison, error) {
	trimmedCommit := strings.TrimSpace(commitSpec)
	if trimmedCommit == "" {
		return commitComparison{mode: diffModeWorkingTree}, nil
	}

	if err := validateCommitModeConflicts(cmd); err != nil {
		return commitComparison{}, err
	}

	baseRef, targetRef, err := parseCommitSpec(trimmedCommit)
	if err != nil {
		return commitComparison{}, err
	}

	if baseRef == "" {
		if err := git.ValidateCommit(repoPath, targetRef); err != nil {
			return commitComparison{}, err
		}
		return commitComparison{baseRef: "", targetRef: targetRef, mode: diffModeCommit}, nil
	}

	if err := git.ValidateCommit(repoPath, baseRef); err != nil {
		return commitComparison{}, err
	}
	if err := git.ValidateCommit(repoPath, targetRef); err != nil {
		return commitComparison{}, err
	}

	return commitComparison{baseRef: baseRef, targetRef: targetRef, mode: diffModeCommit}, nil
}

func validateCommitModeConflicts(cmd *cobra.Command) error {
	for _, flagName := range snapshotSelectorFlags {
		flag := cmd.Flags().Lookup(flagName)
		if flag != nil && flag.Changed {
			return fmt.Errorf("--commit cannot be combined with --%s", flagName)
		}
	}
	return nil
}

func parseCommitSpec(commitSpec string) (baseRef string, targetRef string, err error) {
	if commitSpec == "" {
		return "", "", fmt.Errorf("--commit requires a value")
	}

	commaCount := strings.Count(commitSpec, ",")
	if commaCount == 0 {
		return "", commitSpec, nil
	}
	if commaCount != 1 {
		return "", "", fmt.Errorf("invalid --commit value %q: expected <commit> or <A>,<B>", commitSpec)
	}

	parts := strings.SplitN(commitSpec, ",", 2)
	left := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])
	if left == "" || right == "" {
		return "", "", fmt.Errorf("invalid --commit value %q: both refs are required in <A>,<B>", commitSpec)
	}

	return left, right, nil
}
