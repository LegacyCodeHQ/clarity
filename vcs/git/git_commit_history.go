package git

import (
	"strings"
	"time"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

// GetCommitHistory returns commits in chronological order for a HEAD transition.
func GetCommitHistory(repoPath, fromCommit, toCommit string) ([]vcs.CommitSummary, error) {
	if err := validateGitRef(toCommit); err != nil {
		return nil, err
	}
	args := []string{"log", "--reverse", "--format=%H%x00%h%x00%an%x00%ae%x00%aI%x00%s%x1e"}
	if fromCommit == "" || fromCommit == unbornHeadSignature {
		args = append(args, toCommit)
	} else {
		if err := validateGitRef(fromCommit); err != nil {
			return nil, err
		}
		args = append(args, fromCommit+".."+toCommit)
	}

	stdout, stderr, err := runGitCommand(repoPath, args...)
	if err != nil {
		return nil, gitCommandError(err, stderr)
	}

	return parseCommitHistory(stdout), nil
}

func parseCommitHistory(output []byte) []vcs.CommitSummary {
	raw := strings.TrimSuffix(string(output), "\x1e\n")
	raw = strings.TrimSuffix(raw, "\x1e")
	if raw == "" {
		return nil
	}

	records := strings.Split(raw, "\x1e\n")
	commits := make([]vcs.CommitSummary, 0, len(records))
	for _, record := range records {
		record = strings.TrimSuffix(record, "\x1e")
		if record == "" {
			continue
		}
		fields := strings.SplitN(record, "\x00", 6)
		if len(fields) != 6 {
			continue
		}
		timestamp, err := time.Parse(time.RFC3339, fields[4])
		if err != nil {
			continue
		}
		commits = append(commits, vcs.CommitSummary{
			Hash:      fields[0],
			ShortHash: fields[1],
			Author:    fields[2],
			Email:     fields[3],
			Timestamp: timestamp,
			Subject:   fields[5],
		})
	}
	return commits
}
