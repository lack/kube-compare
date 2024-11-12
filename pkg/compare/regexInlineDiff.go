package compare

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	regex inlineDiffType = "regex"
)

type RegexInlineDiff struct{}

func (id RegexInlineDiff) Diff(regex, crValue string) string {
	cgMatches, fullMatch, err := AppendCgMatches(nil, regex, crValue)
	if err != nil || fullMatch == "" {
		return regex
	}
	result := fullMatch
	warnings := CgWarnings(cgMatches)
	if len(warnings) > 0 {
		result += "\n" + strings.Join(warnings, "\n")
	}
	return result
}

func (id RegexInlineDiff) Validate(regex string) error {
	_, err := regexp.Compile(regex)
	if err != nil {
		return fmt.Errorf("invalid regex passed to inline rgegex diff function: %w", err)
	}
	return nil
}
