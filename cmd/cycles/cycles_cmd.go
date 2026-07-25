// Package cycles implements the experimental `clarity cycles` command, which
// lists circular dependencies between files within a scoped set of directories.
package cycles

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LegacyCodeHQ/clarity/cmd/show/formatters"
	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/LegacyCodeHQ/clarity/depgraph/explain"
	"github.com/LegacyCodeHQ/clarity/depgraph/registry"
	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/spf13/cobra"
)

const cycleArrow = " → "
const maxHumanEvidencePerEdge = 20

// Cmd represents the cycles command.
var Cmd = NewCommand()

// NewCommand returns a new cycles command instance.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cycles [path...]",
		Short: "Analyze cyclic components and break sets (experimental)",
		Long: `List cyclic dependency components between files within a scope.

Scopes to the directories (or files) you pass, defaulting to the current
directory, and reports every strongly connected group found within that scope.
Each component includes one representative loop and a verified set of dependency
edges that can be removed to make the component acyclic. Bounded components
receive exact minimum sets; larger ones receive a labelled heuristic.

With --url, each complete component is rendered as its own focused diagram and
the command emits a shareable visualization URL beneath it.

EXPERIMENTAL API: The command name, flags, relationship taxonomy, human output,
and JSON schema may change without compatibility notice before stabilization.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCycles(cmd, args)
		},
	}
	cmd.Flags().BoolP("url", "u", false, "Emit a visualization URL for each component")
	cmd.Flags().Bool("explain", false, "Show every internal edge, evidence, and break alternative")
	cmd.Flags().Bool("code-only", false, "Exclude documentation relationships (Markdown files)")
	cmd.Flags().StringSlice("include-kind", nil, "Keep only dependency relationship kinds")
	cmd.Flags().StringSlice("exclude-kind", nil, "Exclude dependency relationship kinds")
	cmd.Flags().StringP("format", "f", "human", "Output format: human or json")
	return cmd
}

func runCycles(cmd *cobra.Command, args []string) error {
	inputs := args
	if len(inputs) == 0 {
		inputs = []string{"."}
	}
	emitURL, _ := cmd.Flags().GetBool("url")
	explainOutput, _ := cmd.Flags().GetBool("explain")
	codeOnly, _ := cmd.Flags().GetBool("code-only")
	includeKindValues, _ := cmd.Flags().GetStringSlice("include-kind")
	excludeKindValues, _ := cmd.Flags().GetStringSlice("exclude-kind")
	outputFormat, _ := cmd.Flags().GetString("format")
	outputFormat = strings.ToLower(outputFormat)
	if outputFormat != "human" && outputFormat != "json" {
		return fmt.Errorf("unsupported format %q (expected human or json)", outputFormat)
	}
	if emitURL && outputFormat == "json" {
		return fmt.Errorf("--url cannot be combined with --format json")
	}
	includeKinds, includeKindList, err := parseRelationshipKinds(includeKindValues)
	if err != nil {
		return err
	}
	excludeKinds, excludeKindList, err := parseRelationshipKinds(excludeKindValues)
	if err != nil {
		return err
	}

	files, err := expandInputs(inputs)
	if err != nil {
		return err
	}
	if codeOnly {
		files = filterCodeFiles(files)
	}

	contentReader := vcs.FilesystemContentReader()
	graph, err := depgraph.BuildDependencyGraph(files, contentReader)
	if err != nil {
		return fmt.Errorf("failed to build dependency graph: %w", err)
	}

	fileGraph, err := depgraph.NewFileDependencyGraph(graph, nil, contentReader)
	if err != nil {
		return fmt.Errorf("failed to analyze dependency graph: %w", err)
	}
	explain.AttachEvidence(&fileGraph, contentReader)
	if len(includeKinds) > 0 || len(excludeKinds) > 0 {
		fileGraph, err = filterGraphByRelationships(
			fileGraph, includeKinds, excludeKinds, contentReader)
		if err != nil {
			return fmt.Errorf("failed to filter dependency graph: %w", err)
		}
	}
	depgraph.AnalyzeCycleBreakSets(fileGraph.Meta.Cycles)
	rankBreakSets(fileGraph.Meta.Cycles, fileGraph.Meta.Edges)

	out := cmd.OutOrStdout()
	scope := strings.Join(inputs, ", ")
	base := scopeBase(inputs)
	items := renderCycles(fileGraph.Meta.Cycles, fileGraph.Meta.Edges, base)

	if outputFormat == "json" {
		return renderJSON(
			out, scope, base, codeOnly, includeKindList, excludeKindList, items)
	}

	renderRelationshipFilters(out, includeKindList, excludeKindList)
	if len(items) == 0 {
		fmt.Fprintf(out, "No cyclic components found in %s.\n", scope)
		return nil
	}

	noun := "cyclic components"
	if len(items) == 1 {
		noun = "cyclic component"
	}
	fmt.Fprintf(out, "Found %d %s in %s:\n\n", len(items), noun, scope)
	for i, item := range items {
		fmt.Fprintf(
			out,
			"  %d. %d files, %d internal dependencies\n",
			i+1,
			len(item.component.Nodes),
			len(item.component.Edges))
		fmt.Fprintf(out, "     Representative loop: %s\n", item.line)
		renderBreakSummary(out, item, base, explainOutput)
		if explainOutput {
			renderEvidence(out, item, base)
		}
		if emitURL {
			url, err := cycleURL(item.files, base, contentReader)
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "     %s\n", url)
		}
	}
	return nil
}

func parseRelationshipKinds(
	values []string,
) (map[depgraph.DependencyRelationship]bool, []depgraph.DependencyRelationship, error) {
	known := make(map[string]depgraph.DependencyRelationship)
	for _, relationship := range depgraph.DependencyRelationships() {
		known[string(relationship)] = relationship
	}
	selected := make(map[depgraph.DependencyRelationship]bool)
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		relationship, ok := known[normalized]
		if !ok {
			valid := make([]string, 0, len(known))
			for _, item := range depgraph.DependencyRelationships() {
				valid = append(valid, string(item))
			}
			return nil, nil, fmt.Errorf(
				"unknown dependency relationship %q (expected one of: %s)",
				value,
				strings.Join(valid, ", "))
		}
		selected[relationship] = true
	}
	ordered := make([]depgraph.DependencyRelationship, 0, len(selected))
	for _, relationship := range depgraph.DependencyRelationships() {
		if selected[relationship] {
			ordered = append(ordered, relationship)
		}
	}
	return selected, ordered, nil
}

func filterGraphByRelationships(
	original depgraph.FileDependencyGraph,
	include map[depgraph.DependencyRelationship]bool,
	exclude map[depgraph.DependencyRelationship]bool,
	contentReader vcs.ContentReader,
) (depgraph.FileDependencyGraph, error) {
	adjacency := make(map[string][]string, len(original.Meta.Files))
	for file := range original.Meta.Files {
		adjacency[file] = nil
	}
	filteredMetadata := make(map[depgraph.FileEdge]depgraph.EdgeMetadata)
	for edge, metadata := range original.Meta.Edges {
		evidence := make([]depgraph.DependencyEvidence, 0, len(metadata.Evidence))
		for _, item := range metadata.Evidence {
			relationship := item.Relationship
			if relationship == "" {
				relationship = depgraph.RelationshipResolvedDependency
			}
			if len(include) > 0 && !include[relationship] {
				continue
			}
			if exclude[relationship] {
				continue
			}
			evidence = append(evidence, item)
		}
		if len(evidence) == 0 {
			continue
		}
		adjacency[edge.From] = append(adjacency[edge.From], edge.To)
		metadata.Evidence = evidence
		filteredMetadata[edge] = metadata
	}
	graph, err := depgraph.NewDependencyGraphFromAdjacency(adjacency)
	if err != nil {
		return depgraph.FileDependencyGraph{}, err
	}
	filtered, err := depgraph.NewFileDependencyGraph(graph, nil, contentReader)
	if err != nil {
		return depgraph.FileDependencyGraph{}, err
	}
	for edge, metadata := range filtered.Meta.Edges {
		source := filteredMetadata[edge]
		metadata.Evidence = source.Evidence
		filtered.Meta.Edges[edge] = metadata
	}
	return filtered, nil
}

func renderRelationshipFilters(
	out interface{ Write([]byte) (int, error) },
	include []depgraph.DependencyRelationship,
	exclude []depgraph.DependencyRelationship,
) {
	if len(include) == 0 && len(exclude) == 0 {
		return
	}
	parts := []string{}
	if len(include) > 0 {
		parts = append(parts, "include="+joinRelationships(include))
	}
	if len(exclude) > 0 {
		parts = append(parts, "exclude="+joinRelationships(exclude))
	}
	fmt.Fprintf(out, "Relationship filters: %s\n", strings.Join(parts, " "))
}

func joinRelationships(relationships []depgraph.DependencyRelationship) string {
	values := make([]string, 0, len(relationships))
	for _, relationship := range relationships {
		values = append(values, string(relationship))
	}
	return strings.Join(values, ",")
}

// rankBreakSets keeps minimum cardinality as the primary guarantee, then
// orders equivalent alternatives by evidence confidence and reference count.
// A low-confidence edge is worth investigating before a high-confidence code
// dependency; among equally trustworthy edges, fewer references imply a
// smaller likely edit.
func rankBreakSets(
	components []depgraph.FileCycle,
	metadata map[depgraph.FileEdge]depgraph.EdgeMetadata,
) {
	for index := range components {
		sort.SliceStable(components[index].BreakSets, func(i, j int) bool {
			leftConfidence, leftReferences := breakSetCost(
				components[index].BreakSets[i],
				metadata)
			rightConfidence, rightReferences := breakSetCost(
				components[index].BreakSets[j],
				metadata)
			if leftConfidence != rightConfidence {
				return leftConfidence < rightConfidence
			}
			if leftReferences != rightReferences {
				return leftReferences < rightReferences
			}
			return breakSetKey(components[index].BreakSets[i]) <
				breakSetKey(components[index].BreakSets[j])
		})
	}
}

func breakSetCost(
	breakSet depgraph.CycleBreakSet,
	metadata map[depgraph.FileEdge]depgraph.EdgeMetadata,
) (int, int) {
	confidenceCost := 0
	referenceCost := 0
	for _, edge := range breakSet.Edges {
		evidence := metadata[edge].Evidence
		if len(evidence) == 0 {
			confidenceCost++
			referenceCost++
			continue
		}
		referenceCost += len(evidence)
		for _, item := range evidence {
			switch item.Confidence {
			case depgraph.EvidenceConfidenceLow:
			case depgraph.EvidenceConfidenceMedium:
				confidenceCost++
			case depgraph.EvidenceConfidenceHigh:
				confidenceCost += 2
			}
		}
	}
	return confidenceCost, referenceCost
}

func breakSetKey(breakSet depgraph.CycleBreakSet) string {
	var parts []string
	for _, edge := range breakSet.Edges {
		parts = append(parts, edge.From+"\x00"+edge.To)
	}
	return strings.Join(parts, "\x01")
}

// renderedCycle pairs a cycle's display line with the files it spans, so a
// per-cycle diagram can be rendered for the same cycle the line describes.
type renderedCycle struct {
	line      string
	files     []string
	component depgraph.FileCycle
	metadata  map[depgraph.FileEdge]depgraph.EdgeMetadata
}

// renderCycles turns detected cycles into display items, each line closing the
// loop back to its first file (a → b → a). Sorted for deterministic output.
func renderCycles(
	cycles []depgraph.FileCycle,
	metadata map[depgraph.FileEdge]depgraph.EdgeMetadata,
	base string,
) []renderedCycle {
	items := make([]renderedCycle, 0, len(cycles))
	for _, cycle := range cycles {
		if len(cycle.Path) < 2 {
			continue
		}
		names := make([]string, 0, len(cycle.Path)+1)
		for _, file := range cycle.Path {
			names = append(names, relName(base, file))
		}
		names = append(names, relName(base, cycle.Path[0]))
		items = append(items, renderedCycle{
			line:      strings.Join(names, cycleArrow),
			files:     cycle.Nodes,
			component: cycle,
			metadata:  metadata,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].line < items[j].line })
	return items
}

func renderBreakSummary(out interface{ Write([]byte) (int, error) }, item renderedCycle, base string, explainOutput bool) {
	if len(item.component.BreakSets) == 0 {
		return
	}
	mode := "heuristic"
	if item.component.BreakSetExact {
		mode = "exact"
	}
	first := item.component.BreakSets[0]
	label := "Verified break set"
	if item.component.BreakSetExact {
		label = "Smallest break set"
	}
	fmt.Fprintf(out, "     %s (%s, %d %s):\n", label, mode, len(first.Edges), pluralEdge(len(first.Edges)))
	for _, edge := range first.Edges {
		fmt.Fprintf(out, "       - %s\n", renderEdge(edge, base))
	}
	if !explainOutput && len(item.component.BreakSets) > 1 {
		fmt.Fprintf(
			out,
			"     %d retained equivalent minimum alternatives; use --explain to show all.\n",
			len(item.component.BreakSets)-1)
	}
	if explainOutput {
		for index, alternative := range item.component.BreakSets[1:] {
			fmt.Fprintf(out, "     Alternative %d:\n", index+2)
			for _, edge := range alternative.Edges {
				fmt.Fprintf(out, "       - %s\n", renderEdge(edge, base))
			}
		}
	}
}

func renderEvidence(out interface{ Write([]byte) (int, error) }, item renderedCycle, base string) {
	fmt.Fprintln(out, "     Internal dependencies:")
	for _, edge := range item.component.Edges {
		fmt.Fprintf(out, "       - %s\n", renderEdge(edge, base))
		evidenceItems := item.metadata[edge].Evidence
		visible := evidenceItems
		if len(visible) > maxHumanEvidencePerEdge {
			visible = visible[:maxHumanEvidencePerEdge]
		}
		for _, evidence := range visible {
			reference := relName(base, evidence.ReferenceFile)
			if evidence.ReferenceLine > 0 {
				reference = fmt.Sprintf("%s:%d", reference, evidence.ReferenceLine)
			}
			declaration := relName(base, evidence.DeclarationFile)
			if evidence.DeclarationLine > 0 {
				declaration = fmt.Sprintf("%s:%d", declaration, evidence.DeclarationLine)
			}
			symbol := evidence.Symbol
			if symbol == "" {
				symbol = "(file relationship)"
			}
			fmt.Fprintf(
				out,
				"         %s/%s %q at %s → %s [%s]\n",
				evidence.Kind,
				evidence.Relationship,
				symbol,
				reference,
				declaration,
				evidence.Confidence)
		}
		if hidden := len(evidenceItems) - len(visible); hidden > 0 {
			fmt.Fprintf(
				out,
				"         … %d more references; use --format json for complete evidence\n",
				hidden)
		}
	}
}

func renderEdge(edge depgraph.FileEdge, base string) string {
	return relName(base, edge.From) + cycleArrow + relName(base, edge.To)
}

func pluralEdge(count int) string {
	if count == 1 {
		return "edge"
	}
	return "edges"
}

type jsonCyclesOutput struct {
	Scope        string                            `json:"scope"`
	CodeOnly     bool                              `json:"code_only"`
	IncludeKinds []depgraph.DependencyRelationship `json:"include_kinds,omitempty"`
	ExcludeKinds []depgraph.DependencyRelationship `json:"exclude_kinds,omitempty"`
	Components   []jsonCycleComponent              `json:"components"`
}

type jsonCycleComponent struct {
	Nodes              []string              `json:"nodes"`
	Edges              []jsonCycleEdge       `json:"edges"`
	RepresentativeLoop []string              `json:"representative_loop"`
	BreakAnalysis      string                `json:"break_analysis"`
	BreakSets          [][]depgraph.FileEdge `json:"break_sets"`
}

type jsonCycleEdge struct {
	From     string                        `json:"from"`
	To       string                        `json:"to"`
	Evidence []depgraph.DependencyEvidence `json:"evidence"`
}

func renderJSON(
	out interface{ Write([]byte) (int, error) },
	scope string,
	base string,
	codeOnly bool,
	includeKinds []depgraph.DependencyRelationship,
	excludeKinds []depgraph.DependencyRelationship,
	items []renderedCycle,
) error {
	payload := jsonCyclesOutput{
		Scope: scope, CodeOnly: codeOnly,
		IncludeKinds: includeKinds, ExcludeKinds: excludeKinds,
	}
	for _, item := range items {
		component := jsonCycleComponent{
			Nodes:              relativeFiles(item.component.Nodes, base),
			RepresentativeLoop: relativeFiles(item.component.Path, base),
			BreakAnalysis:      "heuristic",
		}
		if item.component.BreakSetExact {
			component.BreakAnalysis = "exact"
		}
		for _, edge := range item.component.Edges {
			component.Edges = append(component.Edges, jsonCycleEdge{
				From:     relName(base, edge.From),
				To:       relName(base, edge.To),
				Evidence: relativeEvidence(item.metadata[edge].Evidence, base),
			})
		}
		for _, breakSet := range item.component.BreakSets {
			edges := make([]depgraph.FileEdge, 0, len(breakSet.Edges))
			for _, edge := range breakSet.Edges {
				edges = append(edges, depgraph.FileEdge{
					From: relName(base, edge.From),
					To:   relName(base, edge.To),
				})
			}
			component.BreakSets = append(component.BreakSets, edges)
		}
		payload.Components = append(payload.Components, component)
	}
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(encoded))
	return err
}

func relativeEvidence(
	evidence []depgraph.DependencyEvidence,
	base string,
) []depgraph.DependencyEvidence {
	result := make([]depgraph.DependencyEvidence, 0, len(evidence))
	for _, item := range evidence {
		item.ReferenceFile = relName(base, item.ReferenceFile)
		item.DeclarationFile = relName(base, item.DeclarationFile)
		result = append(result, item)
	}
	return result
}

func relativeFiles(files []string, base string) []string {
	result := make([]string, 0, len(files))
	for _, file := range files {
		result = append(result, relName(base, file))
	}
	return result
}

func filterCodeFiles(files []string) []string {
	result := make([]string, 0, len(files))
	for _, file := range files {
		switch strings.ToLower(filepath.Ext(file)) {
		case ".md", ".markdown":
			continue
		default:
			result = append(result, file)
		}
	}
	return result
}

// cycleURL renders the cycle's files as a focused dependency diagram and returns
// a shareable visualization URL, mirroring `clarity show <files> -u`.
func cycleURL(files []string, base string, contentReader vcs.ContentReader) (string, error) {
	graph, err := depgraph.BuildDependencyGraph(files, contentReader)
	if err != nil {
		return "", fmt.Errorf("failed to build cycle graph: %w", err)
	}
	fileGraph, err := depgraph.NewFileDependencyGraph(graph, nil, contentReader)
	if err != nil {
		return "", fmt.Errorf("failed to analyze cycle graph: %w", err)
	}

	formatter, err := formatters.NewFormatter(formatters.OutputFormatDOT.String())
	if err != nil {
		return "", err
	}
	output, err := formatter.Format(fileGraph, formatters.RenderOptions{
		Direction: formatters.DefaultDirection,
		BasePath:  base,
	})
	if err != nil {
		return "", fmt.Errorf("failed to render cycle diagram: %w", err)
	}

	url, ok := formatter.GenerateURL(output)
	if !ok {
		return "", fmt.Errorf("visualization URL generation is unsupported")
	}
	return url, nil
}

func relName(base, file string) string {
	if base == "" {
		return file
	}
	if rel, err := filepath.Rel(base, file); err == nil {
		return rel
	}
	return file
}

// scopeBase returns the longest common directory of the inputs, used to shorten
// file paths in the output relative to the scope the user asked about.
func scopeBase(inputs []string) string {
	var dirs []string
	for _, p := range inputs {
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if info, err := os.Stat(abs); err == nil && info.IsDir() {
			dirs = append(dirs, abs)
		} else {
			dirs = append(dirs, filepath.Dir(abs))
		}
	}
	return longestCommonDir(dirs)
}

func longestCommonDir(dirs []string) string {
	if len(dirs) == 0 {
		return ""
	}
	common := strings.Split(filepath.Clean(dirs[0]), string(filepath.Separator))
	for _, dir := range dirs[1:] {
		parts := strings.Split(filepath.Clean(dir), string(filepath.Separator))
		max := len(common)
		if len(parts) < max {
			max = len(parts)
		}
		i := 0
		for i < max && common[i] == parts[i] {
			i++
		}
		common = common[:i]
	}
	return strings.Join(common, string(filepath.Separator))
}

// expandInputs expands directories and files into a flat list of absolute
// source-file paths, filtered to languages clarity can parse.
//
// The directory-listing helpers below mirror those in cmd/show. They are
// duplicated rather than shared while this command is experimental; if cycles
// graduates, lift them into a package both commands can use.
func expandInputs(paths []string) ([]string, error) {
	var result []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("failed to access %s: %w", p, err)
		}
		if !info.IsDir() {
			abs, err := filepath.Abs(p)
			if err != nil {
				return nil, err
			}
			result = append(result, abs)
			continue
		}

		files, err := listDirFiles(p)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if registry.IsSupportedLanguageExtension(filepath.Ext(f)) {
				result = append(result, f)
			}
		}
	}
	return result, nil
}

func listDirFiles(dir string) ([]string, error) {
	if files, err := listGitFiles(dir); err == nil {
		return files, nil
	}
	return walkDirFiles(dir)
}

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
		if p := strings.TrimSpace(part); p != "" {
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

func walkDirFiles(dir string) ([]string, error) {
	var result []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			if walkSkippedDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		result = append(result, path)
		return nil
	})
	return result, err
}
