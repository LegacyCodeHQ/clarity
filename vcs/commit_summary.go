package vcs

import "time"

// CommitSummary is compact commit metadata for repository history transitions.
type CommitSummary struct {
	Hash      string
	ShortHash string
	Subject   string
	Author    string
	Email     string
	Timestamp time.Time
}
