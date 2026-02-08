package why

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LegacyCodeHQ/sanity/cmd/graph"
	"github.com/LegacyCodeHQ/sanity/depgraph"
	"github.com/LegacyCodeHQ/sanity/vcs"
	"github.com/spf13/cobra"
)

const (
	formatText    = "text"
	formatDOT     = "dot"
	formatMermaid = "mermaid"
)

type whyOptions struct {
	outputFormat string
	repoPath     string
	allowOutside bool
}

type directConnection struct {
	From    string        `json:"from"`
	To      string        `json:"to"`
	Type    string        `json:"type"`
	Members []string      `json:"members,omitempty"`
	Calls   []memberUsage `json:"calls,omitempty"`
}

type memberUsage struct {
	Caller string `json:"caller"`
	Callee string `json:"callee"`
	Line   int    `json:"line"`
}

// Cmd represents the why command.
var Cmd = NewCommand()

// NewCommand returns a new why command instance.
func NewCommand() *cobra.Command {
	opts := &whyOptions{
		outputFormat: formatText,
	}

	cmd := &cobra.Command{
		Use:   "why <from> <to>",
		Short: "Explain whether two files are directly connected.",
		Long:  "Explain whether two files are immediately connected by a direct dependency edge.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWhy(cmd, opts, args[0], args[1])
		},
	}

	cmd.Flags().StringVarP(
		&opts.outputFormat,
		"format",
		"f",
		opts.outputFormat,
		fmt.Sprintf("Output format (%s)", supportedFormats()))
	cmd.Flags().StringVarP(&opts.repoPath, "repo", "r", "", "Git repository path (default: current directory)")
	cmd.Flags().BoolVar(&opts.allowOutside, "allow-outside-repo", false, "Allow input paths outside the repo root")

	return cmd
}

func runWhy(cmd *cobra.Command, opts *whyOptions, fromArg, toArg string) error {
	if !isSupportedFormat(opts.outputFormat) {
		return fmt.Errorf("unknown format: %s (valid options: %s)", opts.outputFormat, supportedFormats())
	}

	repoPath := opts.repoPath
	if repoPath == "" {
		repoPath = "."
	}

	pathResolver, err := graph.NewPathResolver(repoPath, opts.allowOutside)
	if err != nil {
		return fmt.Errorf("failed to create path resolver: %w", err)
	}
	repoPath = pathResolver.BaseDir()

	fromPath, err := pathResolver.Resolve(graph.RawPath(fromArg))
	if err != nil {
		return fmt.Errorf("failed to resolve from file %q: %w", fromArg, err)
	}
	toPath, err := pathResolver.Resolve(graph.RawPath(toArg))
	if err != nil {
		return fmt.Errorf("failed to resolve to file %q: %w", toArg, err)
	}

	filePaths, err := collectSupportedFiles(repoPath)
	if err != nil {
		return fmt.Errorf("failed to collect files from repository: %w", err)
	}
	if len(filePaths) == 0 {
		return fmt.Errorf("no supported files found in repository")
	}

	graphData, err := depgraph.BuildDependencyGraph(filePaths, vcs.FilesystemContentReader())
	if err != nil {
		return fmt.Errorf("failed to build dependency graph: %w", err)
	}

	if !depgraph.ContainsNode(graphData, fromPath.String()) {
		return fmt.Errorf("from file not found in dependency graph: %s", fromArg)
	}
	if !depgraph.ContainsNode(graphData, toPath.String()) {
		return fmt.Errorf("to file not found in dependency graph: %s", toArg)
	}

	connections, err := findDirectConnections(graphData, fromPath.String(), toPath.String())
	if err != nil {
		return err
	}
	enrichMembers(connections)

	output, err := formatOutput(opts.outputFormat, repoPath, fromPath.String(), toPath.String(), connections)
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), output)
	return nil
}

func collectSupportedFiles(root string) ([]string, error) {
	files := make([]string, 0, 256)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		if depgraph.IsSupportedLanguageExtension(filepath.Ext(path)) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func findDirectConnections(g depgraph.DependencyGraph, fromPath, toPath string) ([]directConnection, error) {
	var connections []directConnection

	fromDeps, _, err := depgraph.DependenciesOf(g, fromPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read dependencies for %s: %w", fromPath, err)
	}
	if containsPath(fromDeps, toPath) {
		connections = append(connections, directConnection{
			From: fromPath,
			To:   toPath,
			Type: "dependency",
		})
	}

	toDeps, _, err := depgraph.DependenciesOf(g, toPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read dependencies for %s: %w", toPath, err)
	}
	if containsPath(toDeps, fromPath) {
		connections = append(connections, directConnection{
			From: toPath,
			To:   fromPath,
			Type: "dependency",
		})
	}

	return connections, nil
}

func containsPath(paths []string, target string) bool {
	for _, p := range paths {
		if p == target {
			return true
		}
	}
	return false
}

func formatOutput(format, repoRoot, fromPath, toPath string, connections []directConnection) (string, error) {
	switch strings.ToLower(format) {
	case formatText:
		return formatTextOutput(repoRoot, fromPath, toPath, connections), nil
	case formatDOT:
		return formatDOTOutput(repoRoot, fromPath, toPath, connections), nil
	case formatMermaid:
		return formatMermaidOutput(repoRoot, fromPath, toPath, connections), nil
	default:
		return "", fmt.Errorf("unknown format: %s (valid options: %s)", format, supportedFormats())
	}
}

func formatTextOutput(repoRoot, fromPath, toPath string, connections []directConnection) string {
	fromDisplay := displayPath(repoRoot, fromPath)
	toDisplay := displayPath(repoRoot, toPath)

	if len(connections) == 0 {
		return fmt.Sprintf("No immediate dependency between %s and %s.", fromDisplay, toDisplay)
	}

	lines := []string{
		fmt.Sprintf("Direct connection(s) between %s and %s:", fromDisplay, toDisplay),
	}
	for _, c := range connections {
		lines = append(lines, fmt.Sprintf("- %s depends on %s", displayPath(repoRoot, c.From), displayPath(repoRoot, c.To)))
		if len(c.Members) > 0 {
			lines = append(lines, fmt.Sprintf("  members: %s", strings.Join(c.Members, ", ")))
		}
		if len(c.Calls) > 0 {
			lines = append(lines, "  calls:")
			for _, call := range c.Calls {
				lines = append(lines, fmt.Sprintf("    - %s:%d -> %s", call.Caller, call.Line, call.Callee))
			}
		}
	}
	return strings.Join(lines, "\n")
}

func formatDOTOutput(repoRoot, fromPath, toPath string, connections []directConnection) string {
	var b strings.Builder
	b.WriteString("digraph G {\n")
	b.WriteString("  rankdir=LR;\n")
	b.WriteString(fmt.Sprintf("  %q [label=%q];\n", fromPath, displayPath(repoRoot, fromPath)))
	b.WriteString(fmt.Sprintf("  %q [label=%q];\n", toPath, displayPath(repoRoot, toPath)))
	for _, c := range connections {
		if len(c.Members) == 0 {
			b.WriteString(fmt.Sprintf("  %q -> %q;\n", c.From, c.To))
			continue
		}
		if len(c.Calls) > 0 {
			callerNodes := make(map[string]string)
			for _, call := range c.Calls {
				if _, ok := callerNodes[call.Caller]; ok {
					continue
				}
				callerNode := c.From + "::caller::" + call.Caller
				callerNodes[call.Caller] = callerNode
				b.WriteString(fmt.Sprintf("  %q [label=%q, shape=ellipse];\n", callerNode, call.Caller+"()"))
				b.WriteString(fmt.Sprintf("  %q -> %q [style=dotted];\n", c.From, callerNode))
			}
			memberNodes := make(map[string]string)
			for _, member := range c.Members {
				memberNode := c.To + "::" + member
				memberNodes[member] = memberNode
				b.WriteString(fmt.Sprintf("  %q [label=%q, shape=ellipse];\n", memberNode, member))
				b.WriteString(fmt.Sprintf("  %q -> %q [style=dashed, dir=none];\n", memberNode, c.To))
			}
			for _, call := range c.Calls {
				b.WriteString(fmt.Sprintf("  %q -> %q [label=%q];\n", callerNodes[call.Caller], memberNodes[call.Callee], fmt.Sprintf("L%d", call.Line)))
			}
			continue
		}
		for _, member := range c.Members {
			memberNode := c.To + "::" + member
			b.WriteString(fmt.Sprintf("  %q [label=%q, shape=ellipse];\n", memberNode, member))
			b.WriteString(fmt.Sprintf("  %q -> %q [label=%q];\n", c.From, memberNode, "uses"))
			b.WriteString(fmt.Sprintf("  %q -> %q [style=dashed, dir=none];\n", memberNode, c.To))
		}
	}
	b.WriteString("}")
	return b.String()
}

func formatMermaidOutput(repoRoot, fromPath, toPath string, connections []directConnection) string {
	var b strings.Builder
	b.WriteString("flowchart LR\n")

	hasMemberDetails := false
	for _, c := range connections {
		if len(c.Members) > 0 {
			hasMemberDetails = true
			break
		}
	}

	if !hasMemberDetails {
		b.WriteString(fmt.Sprintf("  n0[%q]\n", displayPath(repoRoot, fromPath)))
		b.WriteString(fmt.Sprintf("  n1[%q]\n", displayPath(repoRoot, toPath)))

		for _, c := range connections {
			fromNode := "n0"
			toNode := "n1"
			if c.From == toPath && c.To == fromPath {
				fromNode = "n1"
				toNode = "n0"
			}
			b.WriteString(fmt.Sprintf("  %s --> %s\n", fromNode, toNode))
		}
		return b.String()
	}

	for connIdx, c := range connections {
		if len(c.Members) == 0 {
			continue
		}

		callerDisplay := displayPath(repoRoot, c.From)
		calleeDisplay := displayPath(repoRoot, c.To)
		callerGroupID := fmt.Sprintf("sg_caller_%d", connIdx)
		calleeGroupID := fmt.Sprintf("sg_callee_%d", connIdx)
		callerNodeID := fmt.Sprintf("caller_%d", connIdx)

		b.WriteString(fmt.Sprintf("  subgraph %s[%q]\n", callerGroupID, callerDisplay))
		if len(c.Calls) == 0 {
			b.WriteString(fmt.Sprintf("    %s[%q]\n", callerNodeID, "calls members"))
		}
		callerNodes := make(map[string]string)
		if len(c.Calls) > 0 {
			callers := uniqueCallers(c.Calls)
			for callerIdx, caller := range callers {
				id := fmt.Sprintf("caller_%d_%d", connIdx, callerIdx)
				callerNodes[caller] = id
				b.WriteString(fmt.Sprintf("    %s[%q]\n", id, caller+"()"))
			}
		}
		b.WriteString("  end\n")
		b.WriteString(fmt.Sprintf("  subgraph %s[%q]\n", calleeGroupID, calleeDisplay))
		memberNodes := make(map[string]string)
		for memberIdx, member := range c.Members {
			memberID := fmt.Sprintf("m_%s_%d_%d", sanitizeMermaidID(member), connIdx, memberIdx)
			memberNodes[member] = memberID
			b.WriteString(fmt.Sprintf("    %s[%q]\n", memberID, member))
		}
		b.WriteString("  end\n")
		if len(c.Calls) == 0 {
			for _, member := range c.Members {
				b.WriteString(fmt.Sprintf("  %s -->|%q| %s\n", callerNodeID, "calls", memberNodes[member]))
			}
		} else {
			for _, call := range c.Calls {
				callerID := callerNodes[call.Caller]
				memberID := memberNodes[call.Callee]
				b.WriteString(fmt.Sprintf("  %s -->|%q| %s\n", callerID, fmt.Sprintf("L%d", call.Line), memberID))
			}
		}
	}

	return b.String()
}

func uniqueCallers(calls []memberUsage) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(calls))
	for _, call := range calls {
		if _, ok := seen[call.Caller]; ok {
			continue
		}
		seen[call.Caller] = struct{}{}
		result = append(result, call.Caller)
	}
	sort.Strings(result)
	return result
}

func enrichMembers(connections []directConnection) {
	for i := range connections {
		calls, err := findReferencedMembers(connections[i].From, connections[i].To)
		if err != nil {
			continue
		}
		connections[i].Calls = calls
		connections[i].Members = collectMembersFromCalls(calls)
	}
}

func findReferencedMembers(fromPath, toPath string) ([]memberUsage, error) {
	if filepath.Ext(fromPath) != ".go" || filepath.Ext(toPath) != ".go" {
		return nil, nil
	}

	targetMembers, err := parseGoTopLevelMembers(toPath)
	if err != nil {
		return nil, err
	}
	if len(targetMembers) == 0 {
		return nil, nil
	}

	calls, err := parseGoCalledIdentifiers(fromPath, targetMembers)
	if err != nil {
		return nil, err
	}
	return calls, nil
}

func parseGoTopLevelMembers(path string) (map[string]struct{}, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, 0)
	if err != nil {
		return nil, err
	}

	members := make(map[string]struct{})
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name != nil {
				members[d.Name.Name] = struct{}{}
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					members[s.Name.Name] = struct{}{}
				case *ast.ValueSpec:
					for _, n := range s.Names {
						members[n.Name] = struct{}{}
					}
				}
			}
		}
	}
	return members, nil
}

func parseGoCalledIdentifiers(path string, targetMembers map[string]struct{}) ([]memberUsage, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, 0)
	if err != nil {
		return nil, err
	}

	var calls []memberUsage
	ast.Walk(&goCallVisitor{
		fileSet:       fileSet,
		targetMembers: targetMembers,
		calls:         &calls,
	}, file)
	sort.Slice(calls, func(i, j int) bool {
		if calls[i].Caller != calls[j].Caller {
			return calls[i].Caller < calls[j].Caller
		}
		if calls[i].Callee != calls[j].Callee {
			return calls[i].Callee < calls[j].Callee
		}
		return calls[i].Line < calls[j].Line
	})
	return calls, nil
}

type goCallVisitor struct {
	fileSet       *token.FileSet
	targetMembers map[string]struct{}
	calls         *[]memberUsage
	funcStack     []string
	nodeStack     []ast.Node
}

func (v *goCallVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		if len(v.nodeStack) == 0 {
			return v
		}
		last := v.nodeStack[len(v.nodeStack)-1]
		v.nodeStack = v.nodeStack[:len(v.nodeStack)-1]
		if _, ok := last.(*ast.FuncDecl); ok && len(v.funcStack) > 0 {
			v.funcStack = v.funcStack[:len(v.funcStack)-1]
		}
		return v
	}

	v.nodeStack = append(v.nodeStack, node)
	if fn, ok := node.(*ast.FuncDecl); ok {
		if fn.Name != nil {
			v.funcStack = append(v.funcStack, fn.Name.Name)
		} else {
			v.funcStack = append(v.funcStack, "<function>")
		}
	}

	callExpr, ok := node.(*ast.CallExpr)
	if !ok {
		return v
	}

	callee := calledIdentifier(callExpr.Fun)
	if callee == "" {
		return v
	}
	if _, ok := v.targetMembers[callee]; !ok {
		return v
	}

	caller := "<file-scope>"
	if len(v.funcStack) > 0 {
		caller = v.funcStack[len(v.funcStack)-1]
	}
	pos := v.fileSet.Position(callExpr.Pos())
	*v.calls = append(*v.calls, memberUsage{
		Caller: caller,
		Callee: callee,
		Line:   pos.Line,
	})
	return v
}

func calledIdentifier(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	default:
		return ""
	}
}

func collectMembersFromCalls(calls []memberUsage) []string {
	seen := make(map[string]struct{})
	members := make([]string, 0, len(calls))
	for _, c := range calls {
		if _, ok := seen[c.Callee]; ok {
			continue
		}
		seen[c.Callee] = struct{}{}
		members = append(members, c.Callee)
	}
	sort.Strings(members)
	return members
}

func sanitizeMermaidID(input string) string {
	var b strings.Builder
	for _, r := range input {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "m"
	}
	return b.String()
}

func displayPath(repoRoot, absolutePath string) string {
	rel, err := filepath.Rel(repoRoot, absolutePath)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return absolutePath
	}
	return rel
}

func isSupportedFormat(format string) bool {
	switch strings.ToLower(format) {
	case formatText, formatDOT, formatMermaid:
		return true
	default:
		return false
	}
}

func supportedFormats() string {
	return strings.Join([]string{formatText, formatDOT, formatMermaid}, ", ")
}
