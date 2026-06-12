package show

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/LegacyCodeHQ/clarity/cmd/show/formatters"
	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/LegacyCodeHQ/clarity/depgraph/registry"
	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/LegacyCodeHQ/clarity/vcs/git"

	"github.com/spf13/cobra"
)

type graphOptions struct {
	outputFormat    string
	repoPath        string
	commitID        string
	generateURL     bool
	orientation     string
	allowOutside    bool
	includeExt      string
	includeExts     []string
	excludeExt      string
	excludeExts     []string
	includes        []string
	excludes        []string
	betweenFiles    []string
	targetFile      string
	depthLevel      int
	reach           string
	all             bool
	collapse        bool
	pruneFiles      []string
	alsoPatterns    []string
	moduleSelect    string
	moduleDirection string
	edgeLabels      bool
	noStats         bool
	noPhantom       bool
}

const (
	reachDown = "down"
	reachUp   = "up"
	reachBoth = "both"

	moduleDirectionNone = "none"
	moduleDirectionIn   = "in"
	moduleDirectionOut  = "out"
	moduleDirectionBoth = "both"
)

var moduleMajorSuffix = regexp.MustCompile(`^v[0-9]+$`)

// Cmd represents the graph command
var Cmd = NewCommand()

// NewCommand returns a new graph command instance.
func NewCommand() *cobra.Command {
	opts := &graphOptions{
		outputFormat:    formatters.OutputFormatDOT.String(),
		orientation:     formatters.DefaultDirection.StringLower(),
		depthLevel:      1,
		moduleDirection: moduleDirectionNone,
	}

	cmd := &cobra.Command{
		Use:   "show [paths...]",
		Short: "Show a scoped file-based dependency graph",
		Long:  `Show a scoped file-based dependency graph.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.includes = append(opts.includes, args...)
			return runGraph(cmd, opts)
		},
	}

	// Add format flag
	cmd.Flags().StringVarP(
		&opts.outputFormat,
		"format",
		"f",
		opts.outputFormat,
		fmt.Sprintf("Output format (%s)", formatters.SupportedFormats()))
	// Add repo flag
	cmd.Flags().StringVarP(&opts.repoPath, "repo", "r", "", "Git repository path (default: current directory)")
	// Add allow outside repo flag
	cmd.Flags().BoolVar(&opts.allowOutside, "allow-outside-repo", false, "Allow input paths outside the repo root")
	// Add commit flag
	cmd.Flags().StringVarP(&opts.commitID, "commit", "c", "", "Git commit or range to analyze (e.g., f0459ec, HEAD~3, f0459ec...be3d11a)")
	// Add URL flag
	cmd.Flags().BoolVarP(&opts.generateURL, "url", "u", false, "Generate visualization URL (supported formats: dot, mermaid)")
	cmd.Flags().StringVarP(
		&opts.orientation,
		"orientation",
		"o",
		opts.orientation,
		fmt.Sprintf("Graph layout orientation (%s)", formatters.SupportedDirections()))
	// Add exclude flag for removing explicit files/directories from graph inputs
	cmd.Flags().StringSliceVar(&opts.excludes, "exclude", nil, "Exclude specific files and/or directories from graph inputs (comma-separated)")
	// Add extension inclusion flag
	cmd.Flags().StringVar(&opts.includeExt, "include-ext", "", "Include only files with these extensions (comma-separated, e.g. .go,.java)")
	// Add extension exclusion flag
	cmd.Flags().StringVar(&opts.excludeExt, "exclude-ext", "", "Exclude files with these extensions (comma-separated, e.g. .go,.java)")
	// Add between flag for finding paths between files
	cmd.Flags().StringSliceVarP(&opts.betweenFiles, "between", "w", nil, "Find all paths between specified files (comma-separated)")
	// Add depth flag for limiting reach traversal
	cmd.Flags().IntVarP(&opts.depthLevel, "depth", "l", opts.depthLevel, "Depth for --reach (0 = unlimited)")
	cmd.Flags().StringVar(&opts.reach, "reach", "", "Walk dependencies from the anchor: up, down, both")
	cmd.Flags().BoolVar(&opts.all, "all", false, "Render the whole tree at this snapshot")
	cmd.Flags().StringSliceVar(&opts.pruneFiles, "prune", nil, "Show node but skip its subtree (requires --reach; shown with dashed border)")
	cmd.Flags().StringSliceVar(&opts.alsoPatterns, "also", nil, "Include files matching glob patterns that connect to the --reach graph (requires --reach)")
	cmd.Flags().BoolVar(&opts.collapse, "collapse", false, "Collapse files into the modules declared in .clarity/modules.json")
	cmd.Flags().StringVarP(&opts.moduleSelect, "module", "m", "", "Render the named module's files inside a box, alongside any files already in scope such as working-set changes (quote names with spaces)")
	cmd.Flags().BoolVar(&opts.edgeLabels, "label", false, "Add deterministic short labels to edges")
	cmd.Flags().BoolVar(&opts.noStats, "no-stats", false, "Skip file addition/deletion statistics for faster rendering")
	cmd.Flags().BoolVar(&opts.noPhantom, "no-phantom", false, "Suppress phantom test nodes (Rust files with #[cfg(test)] regions are rendered as a single node)")

	return cmd
}

func runGraph(cmd *cobra.Command, opts *graphOptions) error {
	if err := validateGraphOptions(opts); err != nil {
		return err
	}

	ensureRepoPath(opts)
	pathResolver, err := NewPathResolver(opts.repoPath, opts.allowOutside)
	if err != nil {
		return fmt.Errorf("failed to create path resolver: %w", err)
	}
	opts.repoPath = pathResolver.BaseDir()

	fromCommit, toCommit, isCommitRange, err := parseCommitRange(opts)
	if err != nil {
		return err
	}
	snapshot := newSnapshotResolver(opts.repoPath, toCommit)

	filePaths, anchorFiles, done, err := determineFilePaths(cmd, opts, pathResolver, snapshot, fromCommit, toCommit, isCommitRange)
	if err != nil {
		return err
	}
	if done {
		return nil
	}

	// Bring deleted files into the bare commit view as nodes. They're excluded
	// from the changed-file list (they no longer exist in the tree), so collect
	// them separately and load their content from the parent ref.
	var deletedContent map[string][]byte
	var deletedFiles []string
	var deletedBaseRef string
	if isPlainCommitView(opts) {
		deletedFiles, deletedBaseRef, err = collectCommitDeletedFiles(opts, fromCommit, toCommit, isCommitRange)
		if err != nil {
			return err
		}
		if len(deletedFiles) > 0 {
			deletedContent, err = loadDeletedFileContent(opts.repoPath, deletedBaseRef, deletedFiles)
			if err != nil {
				return err
			}
			filePaths = append(filePaths, deletedFiles...)
			anchorFiles = append(anchorFiles, deletedFiles...)
		}
	}

	filePaths, err = applyExcludePathFilter(opts, pathResolver, filePaths)
	if err != nil {
		return err
	}

	filePaths, err = applyIncludeExtensionFilter(opts, filePaths)
	if err != nil {
		return err
	}

	filePaths, err = applyExcludeExtensionFilter(opts, filePaths)
	if err != nil {
		return err
	}

	emitUnsupportedFileWarning(filePaths)

	contentReader := snapshot.ContentReader()
	if len(deletedContent) > 0 {
		contentReader = deletedAwareContentReader(contentReader, deletedContent)
	}

	modules, err := snapshot.Modules(opts.collapse || opts.moduleSelect != "")
	if err != nil {
		return err
	}
	// --module is an anchor: the module's own files render (inside a box) alongside
	// whatever else is in scope. With --reach, also bring in the module's immediate
	// dependents/dependencies as context, which requires the snapshot's full tree so
	// they are discoverable; the graph is subset back down after it is built.
	changeFiles := anchorFiles
	var moduleMembers []string
	if opts.moduleSelect != "" {
		selected, selErr := findDeclaredModule(opts.moduleSelect, modules)
		if selErr != nil {
			return selErr
		}
		moduleMembers, err = snapshot.ExistingModuleFiles(selected.Files)
		if err != nil {
			return err
		}
		if len(moduleMembers) == 0 {
			return fmt.Errorf("module %q resolves to no files", opts.moduleSelect)
		}
		if opts.moduleDirection == moduleDirectionNone {
			filePaths = unionPaths(filePaths, moduleMembers)
		} else {
			repoFiles, repoErr := snapshot.TreeFiles()
			if repoErr != nil {
				return fmt.Errorf("failed to expand repository for --reach: %w", repoErr)
			}
			filePaths = unionPaths(repoFiles, changeFiles)
		}
	}

	graph, err := depgraph.BuildDependencyGraph(filePaths, contentReader)
	if err != nil {
		return fmt.Errorf("failed to build dependency graph: %w", err)
	}

	// Use git's own rename detection (`git diff -M`) and collapse each move onto
	// the single new-path node, leaving only genuine removals marked as deletions.
	// Reflecting git keeps the graph honest to what the developer sees in git.
	renames, err := collectCommitRenames(opts, fromCommit, toCommit, isCommitRange)
	if err != nil {
		return err
	}
	graph, err = depgraph.CollapseRenames(graph, renames)
	if err != nil {
		return err
	}
	// CollapseRenames dropped the old-path nodes; resync filePaths to the graph
	// so downstream steps and the title count see one node per rename.
	filePaths = graphFiles(graph)
	deletedFiles = filterRenamed(deletedFiles, renames)

	// Reconstruct each deleted file's pre-deletion edges (who imported it, what it
	// imported) from the parent snapshot so removed nodes show their old links
	// instead of floating. MarkDeletedFiles styles these as removed edges below.
	if len(deletedFiles) > 0 {
		parentReader := git.GitCommitContentReader(opts.repoPath, deletedBaseRef)
		graph, err = depgraph.MergeDeletedNeighborhood(graph, filePaths, deletedFiles, parentReader)
		if err != nil {
			return err
		}
	}

	var fullAdjacency map[string][]string
	if len(opts.alsoPatterns) > 0 {
		fullAdjacency, err = depgraph.AdjacencyList(graph)
		if err != nil {
			return fmt.Errorf("failed to build adjacency list: %w", err)
		}
	}

	var prunedNodes map[string]bool
	graph, filePaths, prunedNodes, err = applyReachFilter(opts, pathResolver, graph, filePaths, anchorFiles)
	if err != nil {
		return err
	}

	if len(opts.alsoPatterns) > 0 && opts.targetFile != "" {
		graph, filePaths, err = applyAlsoFilter(opts, pathResolver, graph, filePaths, fullAdjacency)
		if err != nil {
			return err
		}
	}

	graph, filePaths, err = applyBetweenFilter(opts, pathResolver, graph, filePaths)
	if err != nil {
		return err
	}

	// Derive the render base path from real files before any collapse, so a
	// synthetic module node (not rooted under the repo) cannot defeat the
	// relative-path detection used to shorten node labels.
	renderBasePath := resolveRenderBasePath(opts.repoPath, filePaths)

	var collapse depgraph.Collapse
	var moduleBoundary *moduleBoundaryResult
	switch {
	case opts.moduleSelect != "":
		if opts.moduleDirection == moduleDirectionNone {
			// Just the module's members boxed, alongside every file already in
			// scope (e.g. working-set changes). Nothing is filtered out.
			moduleBoundary = &moduleBoundaryResult{
				Name:    opts.moduleSelect,
				Members: membersInGraph(graph, moduleMembers),
			}
		} else {
			// Boundary view: subset the repo graph to the members, their
			// immediate dependents/dependencies (per -d), and the changes.
			graph, filePaths, moduleBoundary, err = selectModuleBoundary(graph, moduleMembers, changeFiles, opts.moduleSelect, opts.moduleDirection)
			if err != nil {
				return err
			}
		}
	case opts.collapse && len(modules) > 0:
		graph, collapse, err = depgraph.CollapseModules(graph, modules)
		if err != nil {
			return err
		}
		filePaths = graphFiles(graph)
	}

	format, ok := formatters.ParseOutputFormat(opts.outputFormat)
	if !ok {
		return fmt.Errorf("unknown format: %s (valid options: %s)", opts.outputFormat, formatters.SupportedFormats())
	}
	fileStats := collectFileStats(cmd, opts, format, fromCommit, toCommit, isCommitRange)
	fileGraph, err := depgraph.NewFileDependencyGraph(graph, fileStats, contentReader)
	if err != nil {
		return fmt.Errorf("failed to build file graph metadata: %w", err)
	}

	depgraph.MarkDeletedFiles(&fileGraph, deletedFiles)
	depgraph.MarkRenamedFiles(&fileGraph, renames, deletedContent, contentReader)

	for node := range prunedNodes {
		if md, ok := fileGraph.Meta.Files[node]; ok {
			md.IsPruned = true
			fileGraph.Meta.Files[node] = md
		}
	}

	for moduleNode, members := range collapse.Members {
		depgraph.AnnotateModule(&fileGraph, moduleNode, members, fileStats)
	}
	fileGraph.Meta.EdgeOrigins = collapse.EdgeOrigins
	applyModuleBoundary(&fileGraph, moduleBoundary)

	if !opts.noPhantom {
		annotateRustPhantoms(&fileGraph, opts, contentReader, fromCommit, toCommit, isCommitRange)
	}

	formatter, err := formatters.NewFormatter(opts.outputFormat)
	if err != nil {
		return err
	}

	// Built here (not earlier) so the deleted count is read from the marked
	// graph; filePaths already tracks the post-collapse graph, so a rename
	// counts as one file rather than an old+new pair.
	label := buildGraphLabel(opts, format, fromCommit, toCommit, isCommitRange, filePaths, len(collapse.Members), countDeletedFiles(fileGraph))

	orientation, _ := formatters.ParseDirection(opts.orientation)
	renderOpts := formatters.RenderOptions{
		Label:      label,
		Direction:  orientation,
		BasePath:   renderBasePath,
		EdgeLabels: opts.edgeLabels,
	}

	output, err := formatter.Format(fileGraph, renderOpts)
	if err != nil {
		return fmt.Errorf("failed to format graph: %w", err)
	}

	return emitOutput(cmd, opts, format, formatter, output)
}

func resolveRenderBasePath(repoPath string, filePaths []string) string {
	if repoPath != "" && allPathsWithinBase(repoPath, filePaths) {
		return repoPath
	}

	common := commonPathPrefix(filePaths)
	if common == "" || common == string(filepath.Separator) {
		return ""
	}
	return common
}

func allPathsWithinBase(basePath string, filePaths []string) bool {
	base := filepath.Clean(basePath)
	for _, path := range filePaths {
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return false
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return false
		}
	}
	return true
}

func commonPathPrefix(paths []string) string {
	if len(paths) == 0 {
		return ""
	}

	splitPath := func(path string) (string, []string) {
		clean := filepath.Clean(path)
		volume := filepath.VolumeName(clean)
		rest := strings.TrimPrefix(clean, volume)
		rest = strings.TrimPrefix(rest, string(filepath.Separator))
		if rest == "" {
			return volume, nil
		}
		return volume, strings.Split(rest, string(filepath.Separator))
	}

	volume, parts := splitPath(paths[0])
	commonParts := append([]string(nil), parts...)
	for _, path := range paths[1:] {
		v, p := splitPath(path)
		if !strings.EqualFold(v, volume) {
			return ""
		}
		max := len(commonParts)
		if len(p) < max {
			max = len(p)
		}
		i := 0
		for i < max && commonParts[i] == p[i] {
			i++
		}
		commonParts = commonParts[:i]
		if len(commonParts) == 0 {
			break
		}
	}

	if len(commonParts) == 0 {
		return volume + string(filepath.Separator)
	}

	joined := filepath.Join(commonParts...)
	if volume != "" {
		return filepath.Join(volume+string(filepath.Separator), joined)
	}
	if strings.HasPrefix(paths[0], string(filepath.Separator)) {
		return filepath.Join(string(filepath.Separator), joined)
	}
	return joined
}

func validateGraphOptions(opts *graphOptions) error {
	if opts.collapse && opts.moduleSelect != "" {
		return fmt.Errorf("--collapse cannot be used with --module")
	}

	orientation, ok := formatters.ParseDirection(opts.orientation)
	if !ok {
		return fmt.Errorf("unknown orientation: %s (valid options: %s)", opts.orientation, formatters.SupportedDirections())
	}
	opts.orientation = orientation.StringLower()

	if opts.includeExt != "" {
		includeExts, err := normalizeExtensions("--include-ext", opts.includeExt)
		if err != nil {
			return err
		}
		opts.includeExts = includeExts
	}

	if opts.excludeExt != "" {
		excludeExts, err := normalizeExtensions("--exclude-ext", opts.excludeExt)
		if err != nil {
			return err
		}
		opts.excludeExts = excludeExts
	}

	reach := strings.ToLower(strings.TrimSpace(opts.reach))
	switch reach {
	case "":
	case reachDown, reachUp, reachBoth:
		opts.reach = reach
	default:
		return fmt.Errorf("unknown reach: %s (valid options: up, down, both)", opts.reach)
	}

	if opts.reach != "" && opts.targetFile == "" && len(opts.includes) == 1 {
		opts.targetFile = opts.includes[0]
		opts.includes = nil
	}

	if len(opts.betweenFiles) > 0 && len(opts.includes) > 0 {
		return fmt.Errorf("--between cannot be used with paths")
	}
	if len(opts.betweenFiles) > 0 && opts.reach != "" {
		return fmt.Errorf("--reach cannot be used with --between")
	}
	if len(opts.betweenFiles) > 0 && opts.collapse {
		return fmt.Errorf("--collapse cannot be used with --between")
	}
	if opts.reach != "" && opts.collapse {
		return fmt.Errorf("--reach cannot be used with --collapse")
	}
	if opts.all && len(opts.includes) > 0 {
		return fmt.Errorf("--all cannot be used with paths")
	}
	if opts.all && len(opts.betweenFiles) > 0 {
		return fmt.Errorf("--all cannot be used with --between")
	}
	if opts.all && opts.moduleSelect != "" {
		return fmt.Errorf("--all cannot be used with --module")
	}
	if opts.all && opts.reach != "" {
		return fmt.Errorf("--reach cannot be used with --all")
	}

	if opts.depthLevel < 0 {
		return fmt.Errorf("--depth must be at least 0")
	}

	if len(opts.pruneFiles) > 0 && opts.targetFile == "" && opts.reach == "" {
		return fmt.Errorf("--prune requires --reach")
	}

	if len(opts.alsoPatterns) > 0 && opts.targetFile == "" {
		return fmt.Errorf("--also requires a single path with --reach")
	}

	if opts.moduleSelect != "" {
		if opts.targetFile != "" {
			return fmt.Errorf("--module cannot be used with a file anchor")
		}
		if len(opts.betweenFiles) > 0 {
			return fmt.Errorf("--module cannot be used with --between flag")
		}
	}

	// --reach over a selected module maps to the internal in/out/both direction.
	if opts.moduleSelect != "" && opts.reach != "" {
		switch opts.reach {
		case reachDown:
			opts.moduleDirection = moduleDirectionOut
		case reachUp:
			opts.moduleDirection = moduleDirectionIn
		case reachBoth:
			opts.moduleDirection = moduleDirectionBoth
		}
	}

	return nil
}

func normalizeExtensions(flagName, rawExts string) ([]string, error) {
	parts := strings.Split(rawExts, ",")
	exts := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))

	for _, part := range parts {
		ext := strings.TrimSpace(part)
		if ext == "" {
			return nil, fmt.Errorf("%s cannot contain empty extensions", flagName)
		}
		if strings.Contains(ext, string(filepath.Separator)) {
			return nil, fmt.Errorf("%s must be file extensions, got %q", flagName, part)
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		if ext == "." {
			return nil, fmt.Errorf("%s must include extension characters", flagName)
		}

		ext = strings.ToLower(ext)
		if _, ok := seen[ext]; ok {
			continue
		}
		seen[ext] = struct{}{}
		exts = append(exts, ext)
	}

	if len(exts) == 0 {
		return nil, fmt.Errorf("%s cannot be empty", flagName)
	}

	return exts, nil
}

func ensureRepoPath(opts *graphOptions) {
	if opts.repoPath == "" {
		opts.repoPath = "."
	}
}

func parseCommitRange(opts *graphOptions) (string, string, bool, error) {
	var fromCommit, toCommit string
	var isCommitRange bool

	if opts.commitID == "" {
		return "", "", false, nil
	}

	fromCommit, toCommit, isCommitRange = git.ParseCommitRange(opts.commitID)
	if !isCommitRange {
		return fromCommit, toCommit, isCommitRange, nil
	}

	fromCommit, toCommit, _, err := git.NormalizeCommitRange(opts.repoPath, fromCommit, toCommit)
	if err != nil {
		return "", "", false, fmt.Errorf("failed to normalize commit range: %w", err)
	}

	return fromCommit, toCommit, isCommitRange, nil
}

func determineFilePaths(cmd *cobra.Command, opts *graphOptions, pathResolver PathResolver, snapshot *snapshotResolver, fromCommit, toCommit string, isCommitRange bool) ([]string, []string, bool, error) {
	if opts.all {
		filePaths, err := snapshot.TreeFiles()
		if err != nil {
			if opts.commitID != "" {
				return nil, nil, false, fmt.Errorf("failed to get files from commit tree: %w", err)
			}
			return nil, nil, false, fmt.Errorf("failed to expand working directory: %w", err)
		}
		if len(filePaths) == 0 {
			if opts.commitID != "" {
				return nil, nil, false, fmt.Errorf("no files found in commit %s", toCommit)
			}
			return nil, nil, false, fmt.Errorf("no supported files found in working directory")
		}
		return filePaths, filePaths, false, nil
	}

	if len(opts.includes) > 0 {
		anchorFiles, err := snapshot.FilesUnder(pathResolver, opts.includes)
		if err != nil {
			return nil, nil, false, fmt.Errorf("failed to resolve input paths: %w", err)
		}
		if len(anchorFiles) == 0 {
			return nil, nil, false, fmt.Errorf("no files found in specified paths")
		}
		if opts.reach != "" {
			filePaths, err := snapshot.TreeFiles()
			if err != nil {
				if opts.commitID != "" {
					return nil, nil, false, fmt.Errorf("failed to get files from commit tree: %w", err)
				}
				return nil, nil, false, fmt.Errorf("failed to expand working directory: %w", err)
			}
			return filePaths, anchorFiles, false, nil
		}
		return anchorFiles, anchorFiles, false, nil
	}

	if len(opts.betweenFiles) > 0 {
		filePaths, err := collectBetweenFilePaths(opts, snapshot, toCommit)
		if err != nil {
			return nil, nil, false, err
		}
		return filePaths, filePaths, false, nil
	}

	if opts.targetFile != "" {
		filePaths, err := snapshot.TreeFiles()
		if err != nil {
			if opts.commitID != "" {
				return nil, nil, false, fmt.Errorf("failed to get files from commit tree: %w", err)
			}
			return nil, nil, false, fmt.Errorf("failed to expand working directory: %w", err)
		}
		if len(filePaths) == 0 {
			if opts.commitID != "" {
				return nil, nil, false, fmt.Errorf("no files found in commit %s", toCommit)
			}
			return nil, nil, false, fmt.Errorf("no supported files found in working directory")
		}
		return filePaths, nil, false, nil
	}

	if opts.commitID != "" {
		anchorFiles, err := collectCommitFilePaths(opts, fromCommit, toCommit, isCommitRange)
		if err != nil {
			return nil, nil, false, err
		}
		if opts.reach != "" {
			filePaths, treeErr := snapshot.TreeFiles()
			if treeErr != nil {
				return nil, nil, false, fmt.Errorf("failed to get files from commit tree: %w", treeErr)
			}
			return filePaths, anchorFiles, false, nil
		}
		return anchorFiles, anchorFiles, false, nil
	}

	anchorFiles, err := git.GetUncommittedFiles(opts.repoPath)
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to get uncommitted files: %w", err)
	}

	if len(anchorFiles) == 0 {
		// --module supplies its own files as the scope, so a clean tree is fine:
		// fall through with no changes and let the module fill the scope.
		if opts.moduleSelect != "" {
			return nil, nil, false, nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Working directory is clean (no uncommitted changes).")
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "To visualize the most recent commit:")
		fmt.Fprintln(cmd.OutOrStdout(), "  clarity show -c HEAD")
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "To visualize a specific commit:")
		fmt.Fprintln(cmd.OutOrStdout(), "  clarity show -c <commit-hash>")
		return nil, nil, true, nil
	}

	if opts.reach != "" {
		filePaths, treeErr := snapshot.TreeFiles()
		if treeErr != nil {
			return nil, nil, false, fmt.Errorf("failed to expand working directory: %w", treeErr)
		}
		return filePaths, anchorFiles, false, nil
	}

	return anchorFiles, anchorFiles, false, nil
}

func collectBetweenFilePaths(opts *graphOptions, snapshot *snapshotResolver, toCommit string) ([]string, error) {
	filePaths, err := snapshot.TreeFiles()
	if err != nil {
		if opts.commitID != "" {
			return nil, fmt.Errorf("failed to get files from commit tree: %w", err)
		}
		return nil, fmt.Errorf("failed to expand working directory: %w", err)
	}
	if len(filePaths) == 0 {
		if opts.commitID != "" {
			return nil, fmt.Errorf("no files found in commit %s", toCommit)
		}
		return nil, fmt.Errorf("no supported files found in working directory")
	}
	return filePaths, nil
}

func collectCommitFilePaths(opts *graphOptions, fromCommit, toCommit string, isCommitRange bool) ([]string, error) {
	if isCommitRange {
		filePaths, err := git.GetCommitRangeFiles(opts.repoPath, fromCommit, toCommit)
		if err != nil {
			return nil, fmt.Errorf("failed to get files from commit range: %w", err)
		}
		if len(filePaths) == 0 {
			return nil, fmt.Errorf("no files changed in commit range %s", opts.commitID)
		}
		return filePaths, nil
	}

	filePaths, err := git.GetCommitDartFiles(opts.repoPath, toCommit)
	if err != nil {
		return nil, fmt.Errorf("failed to get files from commit: %w", err)
	}
	if len(filePaths) == 0 {
		return nil, fmt.Errorf("no files changed in commit %s", toCommit)
	}
	return filePaths, nil
}

// isPlainCommitView reports whether this is a bare `-c <commit>` view, as opposed
// to one narrowed by positional paths, --between, or --reach. Only the plain view
// injects deleted files as nodes; the narrowed views resolve against the commit tree.
func isPlainCommitView(opts *graphOptions) bool {
	return opts.commitID != "" && len(opts.includes) == 0 && len(opts.betweenFiles) == 0 && opts.targetFile == ""
}

// collectCommitDeletedFiles returns the files removed by the commit (or range)
// and the ref to read their pre-deletion content from: the commit's first parent
// for a single commit, or the lower bound for a range. A root commit (no parent)
// can have no deletions, so it returns nothing.
func collectCommitDeletedFiles(opts *graphOptions, fromCommit, toCommit string, isCommitRange bool) (paths []string, baseRef string, err error) {
	if isCommitRange {
		paths, err = git.GetCommitRangeDeletedFiles(opts.repoPath, fromCommit, toCommit)
		return paths, fromCommit, err
	}

	parent, hasParent, err := git.ResolveFirstParent(opts.repoPath, toCommit)
	if err != nil {
		return nil, "", err
	}
	if !hasParent {
		return nil, "", nil
	}
	paths, err = git.GetCommitDeletedFiles(opts.repoPath, toCommit)
	return paths, parent, err
}

// collectCommitRenames returns git's own rename detection (old path -> new path,
// absolute) for the commit or range, mirroring `git diff -M`. It is empty
// outside a plain commit view, where there are no deleted files to pair.
func collectCommitRenames(opts *graphOptions, fromCommit, toCommit string, isCommitRange bool) (map[string]string, error) {
	if !isPlainCommitView(opts) {
		return nil, nil
	}
	if isCommitRange {
		return git.GetCommitRangeRenames(opts.repoPath, fromCommit, toCommit)
	}
	return git.GetCommitRenames(opts.repoPath, toCommit)
}

// loadDeletedFileContent reads each deleted file's content from baseRef so the
// graph can still parse its dependencies and render it as a node.
func loadDeletedFileContent(repoPath, baseRef string, deletedFiles []string) (map[string][]byte, error) {
	if len(deletedFiles) == 0 {
		return nil, nil
	}

	reader := git.GitCommitContentReader(repoPath, baseRef)
	content := make(map[string][]byte, len(deletedFiles))
	for _, absPath := range deletedFiles {
		bytes, err := reader(absPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read deleted file %s from %s: %w", absPath, baseRef, err)
		}
		content[absPath] = bytes
	}
	return content, nil
}

// filterRenamed drops rename sources from the deleted set so they're marked as
// moves rather than plain deletions.
func filterRenamed(deletedFiles []string, renames map[string]string) []string {
	if len(renames) == 0 {
		return deletedFiles
	}
	pure := make([]string, 0, len(deletedFiles))
	for _, d := range deletedFiles {
		if _, renamed := renames[d]; !renamed {
			pure = append(pure, d)
		}
	}
	return pure
}

// deletedAwareContentReader serves deleted-file content from the preloaded map
// (read from the parent ref) and delegates everything else to the base reader.
func deletedAwareContentReader(base vcs.ContentReader, deletedContent map[string][]byte) vcs.ContentReader {
	return func(absPath string) ([]byte, error) {
		if content, ok := deletedContent[absPath]; ok {
			return content, nil
		}
		return base(absPath)
	}
}

// annotateRustPhantoms attaches phantom-test metadata to .rs files in the
// graph. Commit-scoped views use watch-mode rules (phantom only when the test
// region actually changed in the diff); other views use show-mode rules
// (phantom whenever an in-file test region exists).
func annotateRustPhantoms(
	fg *depgraph.FileDependencyGraph,
	opts *graphOptions,
	newContent vcs.ContentReader,
	fromCommit, toCommit string,
	isCommitRange bool,
) {
	if opts.commitID == "" || toCommit == "" {
		depgraph.AnnotateRustPhantomsShow(fg, newContent)
		return
	}

	var diffs map[string]vcs.FileDiff
	var oldContent vcs.ContentReader
	var err error

	switch {
	case isCommitRange:
		diffs, err = git.GetCommitRangeFileDiffs(opts.repoPath, fromCommit, toCommit)
		oldContent = git.GitCommitContentReader(opts.repoPath, fromCommit)
	default:
		diffs, err = git.GetCommitFileDiffs(opts.repoPath, toCommit)
		oldContent = git.GitCommitContentReader(opts.repoPath, toCommit+"~")
	}

	if err != nil {
		depgraph.AnnotateRustPhantomsShow(fg, newContent)
		return
	}
	depgraph.AnnotateRustPhantomsWatch(fg, diffs, oldContent, newContent)
}

func applyReachFilter(opts *graphOptions, pathResolver PathResolver, graph depgraph.DependencyGraph, filePaths, anchorFiles []string) (depgraph.DependencyGraph, []string, map[string]bool, error) {
	// A --module view consumes --reach through the module boundary (its in/out
	// direction), so the file-reach filter must not also run and prune members.
	if opts.moduleSelect != "" {
		return graph, filePaths, nil, nil
	}
	if opts.targetFile == "" && opts.reach == "" {
		return graph, filePaths, nil, nil
	}

	var targets []string
	if opts.targetFile != "" {
		absTargetFile, err := pathResolver.Resolve(RawPath(opts.targetFile))
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to resolve file path: %w", err)
		}
		if !depgraph.ContainsNode(graph, absTargetFile.String()) {
			return nil, nil, nil, fmt.Errorf("file not found in graph: %s", opts.targetFile)
		}
		targets = []string{absTargetFile.String()}
	} else {
		targets = anchorFiles
	}

	pruneSet := make(map[string]bool, len(opts.pruneFiles))
	for _, pf := range opts.pruneFiles {
		absPrunePath, err := pathResolver.Resolve(RawPath(pf))
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to resolve prune path %q: %w", pf, err)
		}
		pruneSet[absPrunePath.String()] = true
	}

	reach := opts.reach
	if reach == "" {
		reach = reachDown
	}
	graph, prunedNodes := filterGraphByLevel(graph, targets, opts.depthLevel, reach, pruneSet)
	filePaths = graphFiles(graph)

	return graph, filePaths, prunedNodes, nil
}

func applyAlsoFilter(opts *graphOptions, pathResolver PathResolver, graph depgraph.DependencyGraph, filePaths []string, fullAdjacency map[string][]string) (depgraph.DependencyGraph, []string, error) {
	if len(opts.alsoPatterns) == 0 {
		return graph, filePaths, nil
	}

	scopedNodes := make(map[string]bool, len(filePaths))
	for _, fp := range filePaths {
		scopedNodes[fp] = true
	}

	// Build reverse adjacency from full graph (who imports each node).
	reverseAdj := make(map[string][]string, len(fullAdjacency))
	for source, deps := range fullAdjacency {
		for _, dep := range deps {
			reverseAdj[dep] = append(reverseAdj[dep], source)
		}
	}

	baseDir := pathResolver.BaseDir()

	// Find candidate files from full graph that match patterns.
	candidates := make(map[string]bool)
	for node := range fullAdjacency {
		if scopedNodes[node] {
			continue
		}
		relPath, err := filepath.Rel(baseDir, node)
		if err != nil {
			continue
		}
		for _, pattern := range opts.alsoPatterns {
			if matchAlsoPattern(pattern, relPath) {
				candidates[node] = true
				break
			}
		}
	}

	// Keep only candidates connected to the scoped graph.
	connected := make(map[string]bool)
	for candidate := range candidates {
		for _, dep := range fullAdjacency[candidate] {
			if scopedNodes[dep] {
				connected[candidate] = true
				break
			}
		}
		if connected[candidate] {
			continue
		}
		for _, source := range reverseAdj[candidate] {
			if scopedNodes[source] {
				connected[candidate] = true
				break
			}
		}
	}

	if len(connected) == 0 {
		return graph, filePaths, nil
	}

	// Merge connected nodes into the scoped graph.
	allNodes := make(map[string]bool, len(scopedNodes)+len(connected))
	for n := range scopedNodes {
		allNodes[n] = true
	}
	for n := range connected {
		allNodes[n] = true
	}

	merged := make(map[string][]string, len(allNodes))
	for node := range allNodes {
		var deps []string
		for _, dep := range fullAdjacency[node] {
			if allNodes[dep] {
				deps = append(deps, dep)
			}
		}
		merged[node] = deps
	}

	newGraph, err := depgraph.NewDependencyGraphFromAdjacency(merged)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build merged graph: %w", err)
	}

	newFilePaths := make([]string, 0, len(merged))
	for f := range merged {
		newFilePaths = append(newFilePaths, f)
	}

	return newGraph, newFilePaths, nil
}

// matchAlsoPattern matches a glob pattern against a file path.
// If the pattern contains a path separator, it matches against the full relative path.
// Otherwise, it matches against the basename only (so *.test.ts matches at any depth).
func matchAlsoPattern(pattern, relPath string) bool {
	if strings.ContainsRune(pattern, filepath.Separator) || strings.Contains(pattern, "/") {
		matched, _ := filepath.Match(pattern, relPath)
		return matched
	}
	matched, _ := filepath.Match(pattern, filepath.Base(relPath))
	return matched
}

func applyBetweenFilter(opts *graphOptions, pathResolver PathResolver, graph depgraph.DependencyGraph, filePaths []string) (depgraph.DependencyGraph, []string, error) {
	if len(opts.betweenFiles) == 0 {
		return graph, filePaths, nil
	}

	resolvedPaths, missingPaths := resolveAndValidatePaths(opts.betweenFiles, pathResolver, graph)
	if len(missingPaths) > 0 {
		return nil, nil, fmt.Errorf("files not found in graph: %v", missingPaths)
	}
	if len(resolvedPaths) < 2 {
		return nil, nil, fmt.Errorf("at least 2 files required for --between, found %d in graph", len(resolvedPaths))
	}

	graph = depgraph.FindPathNodes(graph, resolvedPaths)
	filePaths = graphFiles(graph)

	return graph, filePaths, nil
}

type moduleBoundaryResult struct {
	Name    string
	Members []string
	// External are neighbor nodes drawn as pruned context. Neighbors that also
	// changed are excluded so they keep their change styling (changed beats pruned).
	External []string
}

// selectModuleBoundary subsets a repo-wide graph to the module's members, their
// immediate dependents and/or dependencies (per direction), and the changes.
// Members stay boxed; non-changed neighbors are returned as External so the
// caller can style them as pruned context; every change is kept, related or not.
func selectModuleBoundary(graph depgraph.DependencyGraph, members, changes []string, name, direction string) (depgraph.DependencyGraph, []string, *moduleBoundaryResult, error) {
	adjacency, err := depgraph.AdjacencyList(graph)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to build adjacency list: %w", err)
	}

	memberSet := make(map[string]bool, len(members))
	for _, f := range members {
		if _, ok := adjacency[f]; ok {
			memberSet[f] = true
		}
	}
	if len(memberSet) == 0 {
		return nil, nil, nil, fmt.Errorf("module %q has no files in the current scope", name)
	}

	includeIn := direction == moduleDirectionIn || direction == moduleDirectionBoth
	includeOut := direction == moduleDirectionOut || direction == moduleDirectionBoth

	neighborSet := make(map[string]bool)
	if includeOut {
		for member := range memberSet {
			for _, dep := range adjacency[member] {
				if !memberSet[dep] {
					neighborSet[dep] = true
				}
			}
		}
	}
	if includeIn {
		for source, deps := range adjacency {
			if memberSet[source] {
				continue
			}
			for _, dep := range deps {
				if memberSet[dep] {
					neighborSet[source] = true
					break
				}
			}
		}
	}

	changeSet := make(map[string]bool, len(changes))
	for _, c := range changes {
		changeSet[c] = true
	}

	// Keep members, neighbors, and every change in the graph (even unrelated ones).
	keep := make(map[string]bool, len(memberSet)+len(neighborSet)+len(changeSet))
	for n := range memberSet {
		keep[n] = true
	}
	for n := range neighborSet {
		keep[n] = true
	}
	for c := range changeSet {
		if _, ok := adjacency[c]; ok {
			keep[c] = true
		}
	}

	kept := make(map[string][]string, len(keep))
	for node := range keep {
		var deps []string
		for _, dep := range adjacency[node] {
			if keep[dep] {
				deps = append(deps, dep)
			}
		}
		kept[node] = deps
	}
	filtered, err := depgraph.NewDependencyGraphFromAdjacency(kept)
	if err != nil {
		return nil, nil, nil, err
	}

	prune := make(map[string]bool, len(neighborSet))
	for n := range neighborSet {
		if !changeSet[n] {
			prune[n] = true
		}
	}

	return filtered, graphFiles(filtered), &moduleBoundaryResult{
		Name:     name,
		Members:  sortedSet(memberSet),
		External: sortedSet(prune),
	}, nil
}

// sortedSet returns a set's keys in sorted order for deterministic output.
func sortedSet(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// existingFiles keeps only paths that exist as regular files, dropping declared
// module members whose paths no longer resolve (matching how a mistyped pattern
// surfaces as zero files).
func existingFiles(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
			out = append(out, p)
		}
	}
	return out
}

func existingSnapshotFiles(paths []string, snapshotFiles []string) []string {
	if snapshotFiles == nil {
		return existingFiles(paths)
	}

	present := make(map[string]bool, len(snapshotFiles))
	for _, file := range snapshotFiles {
		present[filepath.Clean(file)] = true
	}

	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if present[filepath.Clean(p)] {
			out = append(out, p)
		}
	}
	return out
}

// unionPaths appends paths from extra that are not already in base, preserving
// base's order. Used to fold a module's own files into the active scope.
func unionPaths(base, extra []string) []string {
	seen := make(map[string]bool, len(base))
	for _, p := range base {
		seen[p] = true
	}
	out := append([]string(nil), base...)
	for _, p := range extra {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// membersInGraph returns the module members that became graph nodes, so the
// boundary box frames only files that are actually rendered.
func membersInGraph(graph depgraph.DependencyGraph, members []string) []string {
	adjacency, err := depgraph.AdjacencyList(graph)
	if err != nil {
		return nil
	}
	present := make([]string, 0, len(members))
	for _, m := range members {
		if _, ok := adjacency[m]; ok {
			present = append(present, m)
		}
	}
	return present
}

// findDeclaredModule returns the module declared under name, or a helpful error
// listing the available names. Names may contain spaces and are matched verbatim.
func findDeclaredModule(name string, modules []depgraph.Module) (depgraph.Module, error) {
	available := make([]string, 0, len(modules))
	for _, m := range modules {
		if m.Name == name {
			return m, nil
		}
		available = append(available, m.Name)
	}
	if len(available) == 0 {
		return depgraph.Module{}, fmt.Errorf("unknown module %q: no modules declared in .clarity/modules.json", name)
	}
	sort.Strings(available)
	return depgraph.Module{}, fmt.Errorf("unknown module %q (available: %s)", name, strings.Join(available, ", "))
}

// applyModuleBoundary marks the immediate dependents/dependencies of a selected
// module as pruned (dashed) and, when any exist, records the module cluster so
// the renderer draws a labeled boundary around the members. With no externals
// the cluster is left unset and the members render without a box.
func applyModuleBoundary(fg *depgraph.FileDependencyGraph, boundary *moduleBoundaryResult) {
	if boundary == nil || len(boundary.Members) == 0 {
		return
	}
	// Neighbor context renders as pruned (dashed) — but only the ones that did
	// not change; changed neighbors are absent from External and keep their stats.
	for _, ext := range boundary.External {
		if md, ok := fg.Meta.Files[ext]; ok {
			md.IsPruned = true
			fg.Meta.Files[ext] = md
		}
	}
	fg.Meta.ModuleCluster = &depgraph.ModuleCluster{
		Name:    boundary.Name,
		Members: boundary.Members,
	}
}

func graphFiles(graph depgraph.DependencyGraph) []string {
	adjacency, err := depgraph.AdjacencyList(graph)
	if err != nil {
		return nil
	}
	filePaths := make([]string, 0, len(adjacency))
	for f := range adjacency {
		filePaths = append(filePaths, f)
	}
	return filePaths
}

func collectFileStats(cmd *cobra.Command, opts *graphOptions, format formatters.OutputFormat, fromCommit, toCommit string, isCommitRange bool) map[string]vcs.FileStats {
	if opts.noStats {
		return nil
	}

	if format != formatters.OutputFormatDOT && format != formatters.OutputFormatMermaid {
		return nil
	}

	var (
		fileStats map[string]vcs.FileStats
		err       error
	)

	if opts.commitID != "" {
		if isCommitRange {
			fileStats, err = git.GetCommitRangeFileStats(opts.repoPath, fromCommit, toCommit)
		} else {
			fileStats, err = git.GetCommitFileStats(opts.repoPath, toCommit)
		}
	} else {
		fileStats, err = git.GetUncommittedFileStats(opts.repoPath)
	}

	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to get file statistics: %v\n", err)
		return nil
	}

	return fileStats
}

func buildGraphLabel(opts *graphOptions, format formatters.OutputFormat, fromCommit, toCommit string, isCommitRange bool, filePaths []string, moduleCount, deletedCount int) string {
	if format != formatters.OutputFormatDOT && format != formatters.OutputFormatMermaid {
		return ""
	}

	labelRepoPath := opts.repoPath
	if labelRepoPath == "" {
		labelRepoPath = "."
	}

	label := fmt.Sprintf("%s • ", repoLabelName(labelRepoPath))
	var err error

	var commitLabel string
	if opts.commitID != "" {
		if isCommitRange {
			commitLabel, err = git.GetCommitRangeLabel(labelRepoPath, fromCommit, toCommit)
		} else {
			commitLabel, err = git.GetShortCommitHash(labelRepoPath, toCommit)
		}
	} else {
		commitLabel, err = git.GetCurrentCommitHash(labelRepoPath)
	}

	if err != nil {
		return ""
	}

	label += commitLabel
	if opts.commitID == "" {
		isDirty, err := git.HasUncommittedChanges(labelRepoPath)
		if err == nil && isDirty {
			label += "-dirty"
		}
	}

	presentFiles := len(filePaths) - moduleCount - deletedCount
	if presentFiles < 0 {
		presentFiles = 0
	}
	label += " • " + graphCountLabel(moduleCount, presentFiles, deletedCount)

	return label
}

// graphCountLabel summarizes the graph's node counts for the title: the node
// composition (modules and present files) and a separate deleted-file count.
// Each part is dropped when zero, so a commit removing files reads
// "4 files • 13 deleted" and an all-deletion view reads "13 deleted"; an
// otherwise-empty graph still reads "0 files".
func graphCountLabel(moduleCount, fileCount, deletedCount int) string {
	var parts []string
	if composition := nodeCountLabel(moduleCount, fileCount); composition != "" {
		parts = append(parts, composition)
	}
	if deletedCount > 0 {
		parts = append(parts, fmt.Sprintf("%d deleted", deletedCount))
	}
	if len(parts) == 0 {
		parts = append(parts, pluralize(0, "file"))
	}
	return strings.Join(parts, " • ")
}

// countDeletedFiles counts file nodes marked deleted in the rendered graph.
func countDeletedFiles(fg depgraph.FileDependencyGraph) int {
	n := 0
	for _, md := range fg.Meta.Files {
		if md.State == depgraph.FileStateDeleted {
			n++
		}
	}
	return n
}

// nodeCountLabel summarizes the graph's node composition for the title. With
// --modules the graph is a mix of collapsed module nodes and plain file nodes,
// so we report each present count rather than mislabeling every node as a
// "file". Zero-valued terms are dropped; it is empty when there is nothing to
// report (the caller supplies an "0 files" fallback).
func nodeCountLabel(moduleCount, fileCount int) string {
	var parts []string
	if moduleCount > 0 {
		parts = append(parts, pluralize(moduleCount, "module"))
	}
	if fileCount > 0 {
		parts = append(parts, pluralize(fileCount, "file"))
	}
	return strings.Join(parts, ", ")
}

func pluralize(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func repoLabelName(repoPath string) string {
	if moduleName := goModuleLabelName(repoPath); moduleName != "" {
		return moduleName
	}

	name := filepath.Base(filepath.Clean(repoPath))
	if name == "." || name == string(filepath.Separator) || name == "" {
		return "repo"
	}
	return name
}

func goModuleLabelName(repoPath string) string {
	content, err := os.ReadFile(filepath.Join(repoPath, "go.mod"))
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "module ") {
			continue
		}

		modulePath := strings.TrimSpace(strings.TrimPrefix(trimmed, "module "))
		modulePath = strings.Trim(modulePath, "\"")
		return modulePathLabel(modulePath)
	}

	return ""
}

func modulePathLabel(modulePath string) string {
	modulePath = strings.TrimSpace(modulePath)
	if modulePath == "" {
		return ""
	}

	parts := strings.Split(modulePath, "/")
	last := parts[len(parts)-1]
	if last == "" {
		return ""
	}

	if moduleMajorSuffix.MatchString(last) && len(parts) > 1 {
		last = parts[len(parts)-2]
	}

	return last
}

func emitOutput(cmd *cobra.Command, opts *graphOptions, format formatters.OutputFormat, formatter formatters.Formatter, output string) error {
	if opts.generateURL {
		if urlStr, ok := formatter.GenerateURL(output); ok {
			fmt.Fprintln(cmd.OutOrStdout(), urlStr)
		} else {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: URL generation is not supported for %s format\n\n", format)
			fmt.Fprintln(cmd.OutOrStdout(), output)
		}
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), output)
	}

	return nil
}

func applyIncludeExtensionFilter(opts *graphOptions, filePaths []string) ([]string, error) {
	if len(opts.includeExts) == 0 {
		return filePaths, nil
	}

	includedExts := make(map[string]struct{}, len(opts.includeExts))
	for _, ext := range opts.includeExts {
		includedExts[ext] = struct{}{}
	}

	filtered := make([]string, 0, len(filePaths))
	for _, filePath := range filePaths {
		if _, ok := includedExts[strings.ToLower(filepath.Ext(filePath))]; ok {
			filtered = append(filtered, filePath)
		}
	}

	if len(filtered) == 0 {
		return nil, fmt.Errorf("no files remain after applying --include-ext %q", opts.includeExt)
	}

	return filtered, nil
}

func applyExcludeExtensionFilter(opts *graphOptions, filePaths []string) ([]string, error) {
	if len(opts.excludeExts) == 0 {
		return filePaths, nil
	}

	excludedExts := make(map[string]struct{}, len(opts.excludeExts))
	for _, ext := range opts.excludeExts {
		excludedExts[ext] = struct{}{}
	}

	filtered := make([]string, 0, len(filePaths))
	for _, filePath := range filePaths {
		if _, ok := excludedExts[strings.ToLower(filepath.Ext(filePath))]; ok {
			continue
		}
		filtered = append(filtered, filePath)
	}

	if len(filtered) == 0 {
		return nil, fmt.Errorf("no files remain after applying --exclude-ext %q", opts.excludeExt)
	}

	return filtered, nil
}

func applyExcludePathFilter(opts *graphOptions, pathResolver PathResolver, filePaths []string) ([]string, error) {
	if len(opts.excludes) == 0 {
		return filePaths, nil
	}

	excludedPaths := make([]string, 0, len(opts.excludes))
	for _, exclude := range opts.excludes {
		resolvedExclude, err := pathResolver.Resolve(RawPath(exclude))
		if err != nil {
			return nil, fmt.Errorf("failed to resolve exclude path %q: %w", exclude, err)
		}
		excludedPaths = append(excludedPaths, resolveSymlinks(filepath.Clean(resolvedExclude.String())))
	}

	filtered := make([]string, 0, len(filePaths))
	for _, filePath := range filePaths {
		cleanPath := resolveSymlinks(filepath.Clean(filePath))
		if isPathExcluded(cleanPath, excludedPaths) {
			continue
		}
		filtered = append(filtered, filePath)
	}

	if len(filtered) == 0 {
		return nil, fmt.Errorf("no files remain after applying --exclude %q", strings.Join(opts.excludes, ","))
	}

	return filtered, nil
}

func isPathExcluded(filePath string, excludedPaths []string) bool {
	for _, excludedPath := range excludedPaths {
		if filePath == excludedPath {
			return true
		}
		if strings.HasPrefix(filePath, excludedPath+string(filepath.Separator)) {
			return true
		}
	}

	return false
}

// expandPaths expands file paths and directories into individual file paths.
// Directories are recursively walked and regular files are included based on includeUnsupportedFiles.
// For directories inside a git repository, git ls-files is used to respect .gitignore rules.
func expandPaths(paths []string, includeUnsupportedFiles bool) ([]string, error) {
	var result []string

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("failed to access %s: %w", path, err)
		}

		if info.IsDir() {
			files, err := listGitFiles(path)
			if err != nil {
				// Not a git repo or git not available; fall back to walk
				files, err = walkDirectoryFiles(path)
				if err != nil {
					return nil, fmt.Errorf("failed to walk directory %s: %w", path, err)
				}
			}

			for _, f := range files {
				if includeUnsupportedFiles {
					result = append(result, f)
					continue
				}
				ext := filepath.Ext(f)
				if registry.IsSupportedLanguageExtension(ext) {
					result = append(result, f)
				}
			}
		} else {
			// Regular file - include it directly
			result = append(result, path)
		}
	}

	return result, nil
}

// listGitFiles returns absolute paths for all non-ignored files in a git repository,
// including files inside submodules. It combines tracked files (--recurse-submodules)
// with untracked but non-ignored files (--others --exclude-standard).
func listGitFiles(dir string) ([]string, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	tracked, err := gitLsFiles(absDir, "--cached", "--recurse-submodules")
	if err != nil {
		return nil, err
	}

	untracked, err := gitLsFiles(absDir, "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(tracked)+len(untracked))
	for _, rel := range append(tracked, untracked...) {
		result = append(result, filepath.Join(absDir, rel))
	}
	return result, nil
}

func gitLsFiles(dir string, args ...string) ([]string, error) {
	cmdArgs := append([]string{"ls-files", "-z"}, args...)
	cmd := exec.Command("git", cmdArgs...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var paths []string
	for _, part := range strings.Split(string(out), "\x00") {
		p := strings.TrimSpace(part)
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

var walkSkippedDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"target":       true,
	".dart_tool":   true,
	"build":        true,
	"__pycache__":  true,
	".gradle":      true,
	".idea":        true,
	".vscode":      true,
}

// walkDirectoryFiles is a fallback for non-git directories.
func walkDirectoryFiles(dir string) ([]string, error) {
	var result []string
	err := filepath.Walk(dir, func(filePath string, fileInfo os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if fileInfo.IsDir() {
			if walkSkippedDirs[fileInfo.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		result = append(result, filePath)
		return nil
	})
	return result, err
}

func emitUnsupportedFileWarning(filePaths []string) {
	unsupportedCount := 0
	unsupportedByExt := make(map[string]bool)

	for _, filePath := range filePaths {
		ext := filepath.Ext(filePath)
		if registry.IsSupportedLanguageExtension(ext) {
			continue
		}

		unsupportedCount++
		if ext == "" {
			unsupportedByExt["<no extension>"] = true
			continue
		}
		unsupportedByExt[ext] = true
	}

	if unsupportedCount == 0 {
		return
	}

	unsupportedExts := make([]string, 0, len(unsupportedByExt))
	for ext := range unsupportedByExt {
		unsupportedExts = append(unsupportedExts, ext)
	}
	sort.Strings(unsupportedExts)

	slog.Debug("dependency extraction is unsupported for some files; rendering standalone nodes without dependency edges",
		"unsupported_file_count", unsupportedCount,
		"unsupported_extensions", unsupportedExts)
}

// resolveAndValidatePaths resolves file paths to absolute paths and validates they exist in the graph.
// Returns the list of resolved paths that exist in the graph and the list of paths that were not found.
func resolveAndValidatePaths(paths []string, pathResolver PathResolver, graph depgraph.DependencyGraph) (resolved []string, missing []string) {
	for _, p := range paths {
		absPath, err := pathResolver.Resolve(RawPath(p))
		if err != nil {
			missing = append(missing, p)
			continue
		}

		if depgraph.ContainsNode(graph, absPath.String()) {
			resolved = append(resolved, absPath.String())
		} else {
			missing = append(missing, p)
		}
	}
	return
}

// filterGraphByLevel filters the dependency graph to include only nodes within
// the specified number of levels from the target file, according to reach.
// A level of 0 means unlimited traversal depth.
// Nodes in pruneSet are included in the graph but their subtrees are not traversed.
// Returns the filtered graph and the set of pruned nodes that were actually visited.
func filterGraphByLevel(graph depgraph.DependencyGraph, targetFiles []string, level int, reach string, pruneSet map[string]bool) (depgraph.DependencyGraph, map[string]bool) {
	adjacency, err := depgraph.AdjacencyList(graph)
	if err != nil {
		return depgraph.NewDependencyGraph(), nil
	}
	reverseAdjacency := make(map[string][]string, len(adjacency))
	for source, deps := range adjacency {
		if _, ok := reverseAdjacency[source]; !ok {
			reverseAdjacency[source] = nil
		}
		for _, dep := range deps {
			reverseAdjacency[dep] = append(reverseAdjacency[dep], source)
		}
	}

	// BFS to find all nodes within the specified level (or all reachable nodes when level=0)
	visited := make(map[string]bool)
	currentLevel := make([]string, 0, len(targetFiles))
	for _, targetFile := range targetFiles {
		if _, ok := adjacency[targetFile]; !ok {
			continue
		}
		if !visited[targetFile] {
			visited[targetFile] = true
			currentLevel = append(currentLevel, targetFile)
		}
	}
	if len(currentLevel) == 0 {
		return depgraph.NewDependencyGraph(), nil
	}

	for l := 0; (level == 0 || l < level) && len(currentLevel) > 0; l++ {
		nextLevel := []string{}
		for _, file := range currentLevel {
			// Pruned nodes stay in the graph but their subtrees are not explored.
			if pruneSet[file] {
				continue
			}
			if reach == reachDown || reach == reachBoth {
				// Add direct dependencies (files this file imports).
				for _, dep := range adjacency[file] {
					if !visited[dep] {
						visited[dep] = true
						nextLevel = append(nextLevel, dep)
					}
				}
			}
			if reach == reachUp || reach == reachBoth {
				// Add direct dependents (files that import this file).
				for _, dependent := range reverseAdjacency[file] {
					if !visited[dependent] {
						visited[dependent] = true
						nextLevel = append(nextLevel, dependent)
					}
				}
			}
		}
		currentLevel = nextLevel
	}

	// Build filtered graph with only visited nodes
	filtered := make(map[string][]string)
	for file := range visited {
		// Only include edges where both source and target are in the filtered set
		var filteredDeps []string
		for _, dep := range adjacency[file] {
			if visited[dep] {
				filteredDeps = append(filteredDeps, dep)
			}
		}
		filtered[file] = filteredDeps
	}

	// Collect pruned nodes that were actually visited
	actuallyPruned := make(map[string]bool)
	for file := range pruneSet {
		if visited[file] {
			actuallyPruned[file] = true
		}
	}

	return depgraph.MustDependencyGraph(filtered), actuallyPruned
}
