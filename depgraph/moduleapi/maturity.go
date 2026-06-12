package moduleapi

// MaturityLevel describes how complete a language's analysis support is.
type MaturityLevel int

const (
	MaturityUntested MaturityLevel = iota
	MaturityBasicTests
	MaturityActivelyTested
	MaturityStable
)

func (level MaturityLevel) DisplayName() string {
	switch level {
	case MaturityUntested:
		return "Untested"
	case MaturityBasicTests:
		return "Basic Tests"
	case MaturityActivelyTested:
		return "Actively Tested"
	case MaturityStable:
		return "Stable"
	default:
		return "Unknown"
	}
}

// MachineName returns a stable, machine-readable slug for the maturity level,
// suitable for JSON output and programmatic consumers. Unlike DisplayName it is
// lowercase and underscore-separated, and unlike the underlying iota value it is
// stable across reordering of the enum.
func (level MaturityLevel) MachineName() string {
	switch level {
	case MaturityUntested:
		return "untested"
	case MaturityBasicTests:
		return "basic_tests"
	case MaturityActivelyTested:
		return "actively_tested"
	case MaturityStable:
		return "stable"
	default:
		return "unknown"
	}
}

func (level MaturityLevel) Symbol() string {
	switch level {
	case MaturityUntested:
		return "○"
	case MaturityBasicTests:
		return "◐"
	case MaturityActivelyTested:
		return "●"
	case MaturityStable:
		return "✓"
	default:
		return "?"
	}
}

// MaturityLevels returns the ordered set of known maturity levels.
func MaturityLevels() []MaturityLevel {
	return []MaturityLevel{
		MaturityUntested,
		MaturityBasicTests,
		MaturityActivelyTested,
		MaturityStable,
	}
}
