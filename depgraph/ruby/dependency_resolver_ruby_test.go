package ruby_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustAdjacency(t *testing.T, g depgraph.DependencyGraph) map[string][]string {
	t.Helper()
	adj, err := depgraph.AdjacencyList(g)
	require.NoError(t, err)
	return adj
}

func TestBuildDependencyGraph_RubyConstantReferenceResolvesToSourceFile(t *testing.T) {
	tmpDir := t.TempDir()

	coderPath := filepath.Join(tmpDir, "activesupport", "lib", "active_support", "cache", "coder.rb")
	testPath := filepath.Join(tmpDir, "activesupport", "test", "cache", "cache_coder_test.rb")
	abstractUnitPath := filepath.Join(tmpDir, "activesupport", "test", "abstract_unit.rb")

	require.NoError(t, os.MkdirAll(filepath.Dir(coderPath), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(testPath), 0o755))

	require.NoError(t, os.WriteFile(coderPath, []byte(`module ActiveSupport
  module Cache
    class Coder
    end
  end
end
`), 0o644))

	require.NoError(t, os.WriteFile(testPath, []byte(`# frozen_string_literal: true
require_relative "../abstract_unit"

class CacheCoderTest < ActiveSupport::TestCase
  def test_roundtrip
    ActiveSupport::Cache::Coder.new
  end
end
`), 0o644))

	// Keep abstract_unit out of supplied files, mirroring commit-scoped graphing.
	require.NoError(t, os.MkdirAll(filepath.Dir(abstractUnitPath), 0o755))
	require.NoError(t, os.WriteFile(abstractUnitPath, []byte("class AbstractUnit; end\n"), 0o644))

	files := []string{coderPath, testPath}
	graph, err := depgraph.BuildDependencyGraph(files, vcs.FilesystemContentReader())
	require.NoError(t, err)

	adj := mustAdjacency(t, graph)
	assert.Contains(t, adj[testPath], coderPath)
	assert.NotContains(t, adj[coderPath], coderPath)
}

func TestBuildDependencyGraph_RubyDoesNotCreateSelfDependencyFromConstantDeclaration(t *testing.T) {
	tmpDir := t.TempDir()

	logSubscriberPath := filepath.Join(tmpDir, "activesupport", "lib", "active_support", "log_subscriber.rb")
	require.NoError(t, os.MkdirAll(filepath.Dir(logSubscriberPath), 0o755))

	require.NoError(t, os.WriteFile(logSubscriberPath, []byte(`module ActiveSupport
  class LogSubscriber < Subscriber
  end
end
`), 0o644))

	files := []string{logSubscriberPath}
	graph, err := depgraph.BuildDependencyGraph(files, vcs.FilesystemContentReader())
	require.NoError(t, err)

	adj := mustAdjacency(t, graph)
	assert.NotContains(t, adj[logSubscriberPath], logSubscriberPath)
}

func TestBuildDependencyGraph_RubyConstantResolvesAcrossIntermediateDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	sourcePath := filepath.Join(tmpDir, "actionpack", "lib", "action_controller", "metal", "request_forgery_protection.rb")
	testPath := filepath.Join(tmpDir, "actionpack", "test", "controller", "request_forgery_protection_test.rb")

	require.NoError(t, os.MkdirAll(filepath.Dir(sourcePath), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(testPath), 0o755))

	require.NoError(t, os.WriteFile(sourcePath, []byte(`module ActionController
  module RequestForgeryProtection
  end
end
`), 0o644))

	require.NoError(t, os.WriteFile(testPath, []byte(`class RequestForgeryProtectionTest
  def test_length
    ActionController::RequestForgeryProtection::AUTHENTICITY_TOKEN_LENGTH
  end
end
`), 0o644))

	files := []string{sourcePath, testPath}
	graph, err := depgraph.BuildDependencyGraph(files, vcs.FilesystemContentReader())
	require.NoError(t, err)

	adj := mustAdjacency(t, graph)
	assert.Contains(t, adj[testPath], sourcePath)
}
