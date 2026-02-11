package diff

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/LegacyCodeHQ/clarity/depgraph/registry"
	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/LegacyCodeHQ/clarity/vcs/git"
)

type snapshot struct {
	ref         string
	filePaths   []string
	contentRead vcs.ContentReader
}

type snapshotPair struct {
	mode   diffMode
	base   snapshot
	target snapshot
}

func resolveSnapshots(repoPath string, comparison commitComparison) (snapshotPair, error) {
	switch comparison.mode {
	case diffModeCommit:
		return resolveCommitModeSnapshots(repoPath, comparison)
	case diffModeWorkingTree:
		return resolveWorkingTreeSnapshots(repoPath)
	default:
		return snapshotPair{}, fmt.Errorf("unknown diff mode: %s", comparison.mode)
	}
}

func resolveWorkingTreeSnapshots(repoPath string) (snapshotPair, error) {
	baseRef := "HEAD"
	if err := git.ValidateCommit(repoPath, baseRef); err != nil {
		return snapshotPair{}, err
	}

	baseFiles, err := git.GetCommitTreeFiles(repoPath, baseRef)
	if err != nil {
		return snapshotPair{}, fmt.Errorf("failed to load base snapshot from %s: %w", baseRef, err)
	}

	targetFiles, err := loadWorkingSnapshotFiles(repoPath)
	if err != nil {
		return snapshotPair{}, fmt.Errorf("failed to load working snapshot: %w", err)
	}

	return snapshotPair{
		mode: diffModeWorkingTree,
		base: snapshot{
			ref:         baseRef,
			filePaths:   baseFiles,
			contentRead: git.GitCommitContentReader(repoPath, baseRef),
		},
		target: snapshot{
			ref:         "WORKING_TREE",
			filePaths:   targetFiles,
			contentRead: vcs.FilesystemContentReader(),
		},
	}, nil
}

func resolveCommitModeSnapshots(repoPath string, comparison commitComparison) (snapshotPair, error) {
	if comparison.baseRef != "" {
		baseFiles, err := git.GetCommitTreeFiles(repoPath, comparison.baseRef)
		if err != nil {
			return snapshotPair{}, fmt.Errorf("failed to load base snapshot from %s: %w", comparison.baseRef, err)
		}
		targetFiles, err := git.GetCommitTreeFiles(repoPath, comparison.targetRef)
		if err != nil {
			return snapshotPair{}, fmt.Errorf("failed to load target snapshot from %s: %w", comparison.targetRef, err)
		}

		return snapshotPair{
			mode: diffModeCommit,
			base: snapshot{
				ref:         comparison.baseRef,
				filePaths:   baseFiles,
				contentRead: git.GitCommitContentReader(repoPath, comparison.baseRef),
			},
			target: snapshot{
				ref:         comparison.targetRef,
				filePaths:   targetFiles,
				contentRead: git.GitCommitContentReader(repoPath, comparison.targetRef),
			},
		}, nil
	}

	firstParent, hasParent, err := git.ResolveFirstParent(repoPath, comparison.targetRef)
	if err != nil {
		return snapshotPair{}, fmt.Errorf("failed to resolve base snapshot for %s: %w", comparison.targetRef, err)
	}

	targetFiles, err := git.GetCommitTreeFiles(repoPath, comparison.targetRef)
	if err != nil {
		return snapshotPair{}, fmt.Errorf("failed to load target snapshot from %s: %w", comparison.targetRef, err)
	}

	base := snapshot{ref: "EMPTY", filePaths: nil, contentRead: nil}
	if hasParent {
		baseFiles, err := git.GetCommitTreeFiles(repoPath, firstParent)
		if err != nil {
			return snapshotPair{}, fmt.Errorf("failed to load base snapshot from %s: %w", firstParent, err)
		}
		base = snapshot{
			ref:         firstParent,
			filePaths:   baseFiles,
			contentRead: git.GitCommitContentReader(repoPath, firstParent),
		}
	}

	return snapshotPair{
		mode: diffModeCommit,
		base: base,
		target: snapshot{
			ref:         comparison.targetRef,
			filePaths:   targetFiles,
			contentRead: git.GitCommitContentReader(repoPath, comparison.targetRef),
		},
	}, nil
}

func loadWorkingSnapshotFiles(repoPath string) ([]string, error) {
	tracked, err := git.ListTrackedFiles(repoPath)
	if err != nil {
		return nil, err
	}
	untracked, err := git.ListUntrackedFiles(repoPath)
	if err != nil {
		return nil, err
	}

	files := make(map[string]struct{}, len(tracked)+len(untracked))
	for _, path := range tracked {
		exists, err := git.FileExists(path)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}
		files[path] = struct{}{}
	}

	for _, path := range untracked {
		if !registry.IsSupportedLanguageExtension(filepath.Ext(path)) {
			continue
		}
		exists, err := git.FileExists(path)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}
		files[path] = struct{}{}
	}

	result := make([]string, 0, len(files))
	for path := range files {
		result = append(result, path)
	}
	sort.Strings(result)
	return result, nil
}
