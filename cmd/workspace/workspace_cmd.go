package workspace

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/LegacyCodeHQ/clarity/cmd/show/formatters"
	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/spf13/cobra"
)

const (
	langAuto = "auto"
	langGo   = "go"
	langRust = "rust"
)

var (
	tomlInlinePackageName = regexp.MustCompile(`package\s*=\s*"([^"]+)"`)
)

type workspaceOptions struct {
	outputFormat string
	repoPath     string
	generateURL  bool
	direction    string
	language     string
	manifestPath string
}

// Cmd represents the experimental workspace graph command.
var Cmd = NewCommand()

// NewCommand returns a new workspace command instance.
func NewCommand() *cobra.Command {
	opts := &workspaceOptions{
		outputFormat: formatters.OutputFormatDOT.String(),
		direction:    formatters.DefaultDirection.StringLower(),
		language:     langAuto,
	}

	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Workspace relationship graph for Go modules and Rust crates (experimental)",
		Long:  "Workspace relationship graph for Go modules and Rust crates (experimental).",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkspace(cmd, opts)
		},
	}

	cmd.Flags().StringVarP(
		&opts.outputFormat,
		"format",
		"f",
		opts.outputFormat,
		fmt.Sprintf("Output format (%s)", formatters.SupportedFormats()))
	cmd.Flags().StringVarP(&opts.repoPath, "repo", "r", "", "Repository path (default: current directory)")
	cmd.Flags().BoolVarP(&opts.generateURL, "url", "u", false, "Generate visualization URL (supported formats: dot, mermaid)")
	cmd.Flags().StringVarP(
		&opts.direction,
		"direction",
		"d",
		opts.direction,
		fmt.Sprintf("Graph direction (%s)", formatters.SupportedDirections()))
	cmd.Flags().StringVar(&opts.language, "language", opts.language, "Workspace language filter (auto, go, rust)")
	cmd.Flags().StringVar(&opts.manifestPath, "manifest", "", "Prune workspace graph to the dependency subgraph rooted at this manifest path")

	return cmd
}

func runWorkspace(cmd *cobra.Command, opts *workspaceOptions) error {
	if err := validateWorkspaceOptions(opts); err != nil {
		return err
	}

	if opts.repoPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to determine current working directory: %w", err)
		}
		opts.repoPath = cwd
	}
	repoPath, err := filepath.Abs(opts.repoPath)
	if err != nil {
		return fmt.Errorf("failed to resolve repo path: %w", err)
	}

	adjacency := make(map[string][]string)
	manifestNodeByPath := make(map[string]string)
	foundLanguageGraph := false

	if opts.language == langAuto || opts.language == langGo {
		goAdj, goManifestNodes, ok, goErr := buildGoWorkspaceAdjacency(repoPath)
		if goErr != nil {
			return goErr
		}
		if ok {
			foundLanguageGraph = true
			mergeAdjacency(adjacency, goAdj)
			mergeManifestNodes(manifestNodeByPath, goManifestNodes)
		}
	}

	if opts.language == langAuto || opts.language == langRust {
		rustAdj, rustManifestNodes, ok, rustErr := buildRustWorkspaceAdjacency(repoPath)
		if rustErr != nil {
			return rustErr
		}
		if ok {
			foundLanguageGraph = true
			mergeAdjacency(adjacency, rustAdj)
			mergeManifestNodes(manifestNodeByPath, rustManifestNodes)
		}
	}

	if !foundLanguageGraph {
		if opts.language == langAuto {
			return fmt.Errorf("no Go or Rust workspace metadata found under %s", repoPath)
		}
		return fmt.Errorf("no %s workspace metadata found under %s", opts.language, repoPath)
	}

	if opts.manifestPath != "" {
		manifestPath := opts.manifestPath
		if !filepath.IsAbs(manifestPath) {
			manifestPath = filepath.Join(repoPath, manifestPath)
		}
		manifestPath = filepath.Clean(manifestPath)

		targetNode, ok := manifestNodeByPath[manifestPath]
		if !ok {
			return fmt.Errorf("manifest not found in workspace graph: %s", opts.manifestPath)
		}

		adjacency = extractManifestDependencySubgraph(adjacency, targetNode)
	}

	graph, err := depgraph.NewDependencyGraphFromAdjacency(adjacency)
	if err != nil {
		return fmt.Errorf("failed to build workspace dependency graph: %w", err)
	}

	fileGraph, err := depgraph.NewFileDependencyGraph(graph, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to build workspace graph metadata: %w", err)
	}

	formatter, err := formatters.NewFormatter(opts.outputFormat)
	if err != nil {
		return err
	}
	direction, _ := formatters.ParseDirection(opts.direction)
	output, err := formatter.Format(fileGraph, formatters.RenderOptions{
		Label:     fmt.Sprintf("workspace relationships • %s", filepath.Base(repoPath)),
		Direction: direction,
	})
	if err != nil {
		return fmt.Errorf("failed to format workspace graph: %w", err)
	}

	if opts.generateURL {
		if urlStr, ok := formatter.GenerateURL(output); ok {
			fmt.Fprintln(cmd.OutOrStdout(), urlStr)
			return nil
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: URL generation is not supported for %s format\n\n", opts.outputFormat)
	}

	fmt.Fprintln(cmd.OutOrStdout(), output)
	return nil
}

func validateWorkspaceOptions(opts *workspaceOptions) error {
	_, ok := formatters.ParseOutputFormat(opts.outputFormat)
	if !ok {
		return fmt.Errorf("unknown format: %s (valid options: %s)", opts.outputFormat, formatters.SupportedFormats())
	}

	switch strings.ToLower(strings.TrimSpace(opts.language)) {
	case langAuto, langGo, langRust:
		opts.language = strings.ToLower(strings.TrimSpace(opts.language))
		return nil
	default:
		return fmt.Errorf("unknown language: %s (valid options: %s, %s, %s)", opts.language, langAuto, langGo, langRust)
	}
}

func mergeAdjacency(dst map[string][]string, src map[string][]string) {
	for node, deps := range src {
		existing := make(map[string]bool)
		for _, dep := range dst[node] {
			existing[dep] = true
		}
		for _, dep := range deps {
			if !existing[dep] {
				dst[node] = append(dst[node], dep)
				existing[dep] = true
			}
		}
		sort.Strings(dst[node])
	}
}

func mergeManifestNodes(dst map[string]string, src map[string]string) {
	for manifestPath, node := range src {
		dst[manifestPath] = node
	}
}

func buildGoWorkspaceAdjacency(repoPath string) (map[string][]string, map[string]string, bool, error) {
	moduleDirs, err := discoverGoWorkspaceModuleDirs(repoPath)
	if err != nil {
		return nil, nil, false, err
	}
	if len(moduleDirs) == 0 {
		return nil, nil, false, nil
	}

	modulePathByDir := make(map[string]string, len(moduleDirs))
	nodeIDByModulePath := make(map[string]string, len(moduleDirs))
	manifestNodeByPath := make(map[string]string, len(moduleDirs))
	for _, dir := range moduleDirs {
		modulePath, err := parseGoModModuleName(filepath.Join(dir, "go.mod"))
		if err != nil || modulePath == "" {
			continue
		}
		modulePathByDir[dir] = modulePath
		nodeID := "[go] " + modulePath
		nodeIDByModulePath[modulePath] = nodeID
		manifestNodeByPath[filepath.Clean(filepath.Join(dir, "go.mod"))] = nodeID
	}
	if len(modulePathByDir) == 0 {
		return nil, nil, false, nil
	}

	adj := make(map[string][]string, len(modulePathByDir))
	for dir, modulePath := range modulePathByDir {
		sourceNode := "[go] " + modulePath
		if _, exists := adj[sourceNode]; !exists {
			adj[sourceNode] = []string{}
		}

		requires, replaceTargets, err := parseGoModDependencies(filepath.Join(dir, "go.mod"))
		if err != nil {
			return nil, nil, false, err
		}
		for _, reqPath := range requires {
			if targetNode, ok := nodeIDByModulePath[reqPath]; ok {
				adj[sourceNode] = append(adj[sourceNode], targetNode)
			}
		}
		for _, replaceTarget := range replaceTargets {
			targetDir := replaceTarget
			if !filepath.IsAbs(targetDir) {
				targetDir = filepath.Join(dir, targetDir)
			}
			targetDir = filepath.Clean(targetDir)
			if modulePath, ok := modulePathByDir[targetDir]; ok {
				adj[sourceNode] = append(adj[sourceNode], "[go] "+modulePath)
			}
		}
		adj[sourceNode] = dedupeSorted(adj[sourceNode])
	}

	return adj, manifestNodeByPath, true, nil
}

func discoverGoWorkspaceModuleDirs(repoPath string) ([]string, error) {
	goWorkPath := filepath.Join(repoPath, "go.work")
	if _, err := os.Stat(goWorkPath); err == nil {
		dirs, parseErr := parseGoWorkUseDirs(goWorkPath)
		if parseErr != nil {
			return nil, parseErr
		}
		resolved := make([]string, 0, len(dirs))
		for _, dir := range dirs {
			if !filepath.IsAbs(dir) {
				dir = filepath.Join(repoPath, dir)
			}
			resolved = append(resolved, filepath.Clean(dir))
		}
		return dedupeSorted(resolved), nil
	}

	var moduleDirs []string
	walkErr := filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "go.mod" {
			return nil
		}
		moduleDirs = append(moduleDirs, filepath.Dir(path))
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("failed to scan Go modules: %w", walkErr)
	}

	return dedupeSorted(moduleDirs), nil
}

func parseGoWorkUseDirs(goWorkPath string) ([]string, error) {
	content, err := os.ReadFile(goWorkPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", goWorkPath, err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	inUseBlock := false
	var dirs []string

	for scanner.Scan() {
		line := trimGoComment(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "use (") {
			inUseBlock = true
			continue
		}
		if inUseBlock {
			if line == ")" {
				inUseBlock = false
				continue
			}
			dir := firstToken(line)
			if dir != "" {
				dirs = append(dirs, trimQuotes(dir))
			}
			continue
		}
		if strings.HasPrefix(line, "use ") {
			dir := strings.TrimSpace(strings.TrimPrefix(line, "use"))
			dir = firstToken(dir)
			if dir != "" {
				dirs = append(dirs, trimQuotes(dir))
			}
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", goWorkPath, scanErr)
	}

	return dirs, nil
}

func parseGoModModuleName(goModPath string) (string, error) {
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", goModPath, err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := trimGoComment(scanner.Text())
		if !strings.HasPrefix(line, "module ") {
			continue
		}
		modulePath := strings.TrimSpace(strings.TrimPrefix(line, "module"))
		modulePath = trimQuotes(modulePath)
		if modulePath != "" {
			return modulePath, nil
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return "", fmt.Errorf("failed to parse %s: %w", goModPath, scanErr)
	}

	return "", nil
}

func parseGoModDependencies(goModPath string) ([]string, []string, error) {
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read %s: %w", goModPath, err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	inRequireBlock := false
	inReplaceBlock := false

	var requires []string
	var replaceTargets []string

	for scanner.Scan() {
		line := trimGoComment(scanner.Text())
		if line == "" {
			continue
		}

		switch {
		case strings.HasPrefix(line, "require ("):
			inRequireBlock = true
			continue
		case strings.HasPrefix(line, "replace ("):
			inReplaceBlock = true
			continue
		case line == ")":
			inRequireBlock = false
			inReplaceBlock = false
			continue
		}

		if inRequireBlock {
			req := parseGoRequireLine(line)
			if req != "" {
				requires = append(requires, req)
			}
			continue
		}

		if inReplaceBlock {
			if target := parseGoReplaceLine(line); target != "" {
				replaceTargets = append(replaceTargets, target)
			}
			continue
		}

		if strings.HasPrefix(line, "require ") {
			req := parseGoRequireLine(strings.TrimSpace(strings.TrimPrefix(line, "require")))
			if req != "" {
				requires = append(requires, req)
			}
			continue
		}
		if strings.HasPrefix(line, "replace ") {
			if target := parseGoReplaceLine(strings.TrimSpace(strings.TrimPrefix(line, "replace"))); target != "" {
				replaceTargets = append(replaceTargets, target)
			}
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return nil, nil, fmt.Errorf("failed to parse %s: %w", goModPath, scanErr)
	}

	return dedupeSorted(requires), dedupeSorted(replaceTargets), nil
}

func parseGoRequireLine(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	if fields[0] == "(" || fields[0] == ")" {
		return ""
	}
	return trimQuotes(fields[0])
}

func parseGoReplaceLine(line string) string {
	parts := strings.Split(line, "=>")
	if len(parts) != 2 {
		return ""
	}
	targetFields := strings.Fields(strings.TrimSpace(parts[1]))
	if len(targetFields) == 0 {
		return ""
	}
	target := trimQuotes(targetFields[0])
	if target == "" {
		return ""
	}
	if strings.HasPrefix(target, "./") || strings.HasPrefix(target, "../") || filepath.IsAbs(target) {
		return target
	}
	return ""
}

func trimGoComment(line string) string {
	trimmed := strings.TrimSpace(line)
	if idx := strings.Index(trimmed, "//"); idx >= 0 {
		trimmed = strings.TrimSpace(trimmed[:idx])
	}
	return trimmed
}

func buildRustWorkspaceAdjacency(repoPath string) (map[string][]string, map[string]string, bool, error) {
	manifestPaths, err := discoverCargoTomlFiles(repoPath)
	if err != nil {
		return nil, nil, false, err
	}
	if len(manifestPaths) == 0 {
		return nil, nil, false, nil
	}

	type crateInfo struct {
		name string
		deps []string
	}

	crates := make(map[string]crateInfo)
	crateManifestPath := make(map[string]string)
	for _, manifest := range manifestPaths {
		name, deps, err := parseCargoManifest(manifest)
		if err != nil {
			return nil, nil, false, err
		}
		if name == "" {
			continue
		}
		crates[name] = crateInfo{name: name, deps: deps}
		crateManifestPath[name] = filepath.Clean(manifest)
	}
	if len(crates) == 0 {
		return nil, nil, false, nil
	}

	nodeByCrate := make(map[string]string, len(crates))
	manifestNodeByPath := make(map[string]string, len(crates))
	for name := range crates {
		nodeID := "[rust] " + name
		nodeByCrate[name] = nodeID
		if manifestPath, ok := crateManifestPath[name]; ok {
			manifestNodeByPath[manifestPath] = nodeID
		}
	}

	adj := make(map[string][]string, len(crates))
	for name, info := range crates {
		sourceNode := nodeByCrate[name]
		if _, exists := adj[sourceNode]; !exists {
			adj[sourceNode] = []string{}
		}
		for _, dep := range info.deps {
			if targetNode, ok := nodeByCrate[dep]; ok {
				adj[sourceNode] = append(adj[sourceNode], targetNode)
			}
		}
		adj[sourceNode] = dedupeSorted(adj[sourceNode])
	}

	return adj, manifestNodeByPath, true, nil
}

func extractManifestDependencySubgraph(adjacency map[string][]string, targetNode string) map[string][]string {
	seen := map[string]bool{targetNode: true}
	queue := []string{targetNode}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, next := range adjacency[current] {
			if seen[next] {
				continue
			}
			seen[next] = true
			queue = append(queue, next)
		}
	}

	pruned := make(map[string][]string, len(seen))
	for node := range seen {
		deps := adjacency[node]
		filtered := make([]string, 0, len(deps))
		for _, dep := range deps {
			if seen[dep] {
				filtered = append(filtered, dep)
			}
		}
		pruned[node] = filtered
	}

	return pruned
}

func discoverCargoTomlFiles(repoPath string) ([]string, error) {
	var manifests []string
	walkErr := filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "target" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == "Cargo.toml" {
			manifests = append(manifests, path)
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("failed to scan Cargo manifests: %w", walkErr)
	}
	return dedupeSorted(manifests), nil
}

func parseCargoManifest(cargoTomlPath string) (string, []string, error) {
	content, err := os.ReadFile(cargoTomlPath)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read %s: %w", cargoTomlPath, err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	section := ""
	crateName := ""
	deps := []string{}

	for scanner.Scan() {
		line := trimTomlComment(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}

		if section == "package" {
			if key, value, ok := parseTomlKeyValue(line); ok && key == "name" {
				crateName = trimQuotes(value)
			}
			continue
		}

		if !isRustDependencySection(section) {
			continue
		}

		key, value, ok := parseTomlKeyValue(line)
		if !ok {
			continue
		}
		depName := dependencyNameFromTomlEntry(key, value)
		if depName != "" {
			deps = append(deps, depName)
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return "", nil, fmt.Errorf("failed to parse %s: %w", cargoTomlPath, scanErr)
	}

	return crateName, dedupeSorted(deps), nil
}

func isRustDependencySection(section string) bool {
	if section == "dependencies" || section == "dev-dependencies" || section == "build-dependencies" {
		return true
	}
	if strings.HasPrefix(section, "target.") && strings.HasSuffix(section, ".dependencies") {
		return true
	}
	return false
}

func parseTomlKeyValue(line string) (string, string, bool) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	if key == "" || value == "" {
		return "", "", false
	}
	return key, value, true
}

func dependencyNameFromTomlEntry(key, value string) string {
	baseKey := strings.TrimSpace(strings.Split(key, ".")[0])
	baseKey = trimQuotes(baseKey)
	if baseKey == "" {
		return ""
	}
	if match := tomlInlinePackageName.FindStringSubmatch(value); len(match) == 2 {
		return match[1]
	}
	return baseKey
}

func trimTomlComment(line string) string {
	inDoubleQuote := false
	inSingleQuote := false
	for i, r := range line {
		switch r {
		case '"':
			if !inSingleQuote {
				inDoubleQuote = !inDoubleQuote
			}
		case '\'':
			if !inDoubleQuote {
				inSingleQuote = !inSingleQuote
			}
		case '#':
			if !inDoubleQuote && !inSingleQuote {
				return strings.TrimSpace(line[:i])
			}
		}
	}
	return strings.TrimSpace(line)
}

func trimQuotes(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"`)
	s = strings.Trim(s, "'")
	return strings.TrimSpace(s)
}

func firstToken(s string) string {
	fields := strings.Fields(strings.TrimSpace(s))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func dedupeSorted(values []string) []string {
	if len(values) == 0 {
		return values
	}
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
