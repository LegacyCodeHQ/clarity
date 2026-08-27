package show

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/LegacyCodeHQ/clarity/clarityconfig"
	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/LegacyCodeHQ/clarity/vcs/git"
)

const modulesRelPath = ".clarity/modules.json"

type snapshotResolver struct {
	repoPath string
	commit   string

	treeFiles []string
}

func newSnapshotResolver(repoPath, commit string) *snapshotResolver {
	return &snapshotResolver{repoPath: repoPath, commit: commit}
}

func (r *snapshotResolver) ContentReader() vcs.ContentReader {
	if r.commit != "" {
		return git.GitCommitContentReader(r.repoPath, r.commit)
	}
	return vcs.FilesystemContentReader()
}

func (r *snapshotResolver) TreeFiles() ([]string, error) {
	if r.treeFiles != nil {
		return append([]string(nil), r.treeFiles...), nil
	}

	var (
		files []string
		err   error
	)
	if r.commit != "" {
		files, err = git.GetCommitTreeFiles(r.repoPath, r.commit)
	} else {
		files, err = expandPaths([]string{r.repoPath}, false)
	}
	if err != nil {
		return nil, err
	}

	r.treeFiles = append([]string(nil), files...)
	return files, nil
}

func (r *snapshotResolver) FilesUnder(pathResolver PathResolver, includes []string) ([]string, error) {
	resolvedIncludes := make([]string, 0, len(includes))
	for _, include := range includes {
		resolvedInclude, err := pathResolver.Resolve(RawPath(include))
		if err != nil {
			return nil, err
		}
		resolvedIncludes = append(resolvedIncludes, resolveSymlinks(filepath.Clean(resolvedInclude.String())))
	}

	if r.commit == "" {
		return expandPaths(resolvedIncludes, true)
	}

	treeFiles, err := r.TreeFiles()
	if err != nil {
		return nil, err
	}
	return filterFilesUnder(treeFiles, resolvedIncludes), nil
}

func (r *snapshotResolver) Modules(enabled bool) ([]depgraph.Module, error) {
	if !enabled {
		return nil, nil
	}
	if r.commit == "" {
		return loadConfigModules(r.repoPath)
	}

	treeFiles, err := r.TreeFiles()
	if err != nil {
		return nil, err
	}

	if !treeContainsRelPath(r.repoPath, treeFiles, modulesRelPath) {
		return nil, nil
	}

	data, err := git.GetFileContentFromCommit(r.repoPath, r.commit, modulesRelPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	return clarityconfig.LoadModulesFromContent(r.repoPath, r.commit+":"+modulesRelPath, data, treeFiles)
}

func (r *snapshotResolver) ExistingModuleFiles(paths []string) ([]string, error) {
	if r.commit == "" {
		return existingFiles(paths), nil
	}

	treeFiles, err := r.TreeFiles()
	if err != nil {
		return nil, err
	}
	return existingSnapshotFiles(paths, treeFiles), nil
}

func filterFilesUnder(files, includes []string) []string {
	filtered := make([]string, 0, len(files))
	seen := make(map[string]struct{}, len(files))
	for _, filePath := range files {
		cleanFilePath := resolveSymlinks(filepath.Clean(filePath))
		for _, includePath := range includes {
			if cleanFilePath == includePath || strings.HasPrefix(cleanFilePath, includePath+string(filepath.Separator)) {
				if _, ok := seen[filePath]; ok {
					break
				}
				seen[filePath] = struct{}{}
				filtered = append(filtered, filePath)
				break
			}
		}
	}
	return filtered
}

func treeContainsRelPath(repoPath string, treeFiles []string, want string) bool {
	for _, file := range treeFiles {
		rel, err := filepath.Rel(repoPath, file)
		if err == nil && filepath.ToSlash(rel) == want {
			return true
		}
	}
	return false
}
