package show

import (
	"bytes"
	"strings"
	"testing"
)

func runShowErr(t *testing.T, args ...string) error {
	t.Helper()
	cmd := NewCommand()
	cmd.SetArgs(args)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	return cmd.Execute()
}

func TestShowCommand_ModuleSelectRendersBoundary(t *testing.T) {
	repoDir, srcDir := writeJavaPair(t)
	// App.java imports Helper.java; collapse Helper into the "support" module.
	writeModulesConfig(t, repoDir, `{
  "modules": [
    { "name": "support", "files": ["src/main/java/com/example/util/Helper.java"] }
  ]
}`)

	out := runShow(t, "-i", srcDir, "-r", repoDir, "-f", "dot", "--module", "support")
	if !strings.Contains(out, `"support"`) {
		t.Fatalf("expected the selected module node \"support\" in output:\n%s", out)
	}
	// The dependent file stays as the module's boundary.
	if !strings.Contains(out, "App.java") {
		t.Fatalf("expected App.java (a dependent) on the module boundary:\n%s", out)
	}
	// Members are collapsed into the module, not rendered individually.
	if strings.Contains(out, "Helper.java") {
		t.Fatalf("expected Helper.java collapsed into the module, but it is still a node:\n%s", out)
	}
}

func TestShowCommand_ModuleSelectImpliesCollapse(t *testing.T) {
	repoDir, srcDir := writeJavaPair(t)
	writeModulesConfig(t, repoDir, `{
  "modules": [
    { "name": "support", "files": ["src/main/java/com/example/util/Helper.java"] }
  ]
}`)

	// No --modules flag: --module alone must enable collapsing.
	out := runShow(t, "-i", srcDir, "-r", repoDir, "-f", "dot", "--module", "support")
	if !strings.Contains(out, `"support"`) {
		t.Fatalf("expected --module to imply collapse and render \"support\":\n%s", out)
	}
}

func TestShowCommand_ModuleSelectUnknownName(t *testing.T) {
	repoDir, srcDir := writeJavaPair(t)
	writeModulesConfig(t, repoDir, `{
  "modules": [
    { "name": "support", "files": ["src/main/java/com/example/util/Helper.java"] }
  ]
}`)

	err := runShowErr(t, "-i", srcDir, "-r", repoDir, "-f", "dot", "--module", "nope")
	if err == nil {
		t.Fatal("expected an error for an unknown module name, got nil")
	}
	if !strings.Contains(err.Error(), "support") {
		t.Fatalf("expected the error to list available modules, got: %v", err)
	}
}

func TestShowCommand_ModuleSelectNoMembersInScope(t *testing.T) {
	repoDir, srcDir := writeJavaPair(t)
	// "ghost" is declared but its file is not in the rendered scope.
	writeModulesConfig(t, repoDir, `{
  "modules": [
    { "name": "ghost", "files": ["src/main/java/com/example/Missing.java"] }
  ]
}`)

	err := runShowErr(t, "-i", srcDir, "-r", repoDir, "-f", "dot", "--module", "ghost")
	if err == nil {
		t.Fatal("expected an error when the module has no files in scope, got nil")
	}
	if !strings.Contains(err.Error(), "no files in the current scope") {
		t.Fatalf("expected a scope error, got: %v", err)
	}
}
