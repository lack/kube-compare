package compare

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// CgInfo helps record each named capturegroup in a regular expression
type CgInfo struct {
	Name  string
	Start int
	End   int
}

// Return a list of the valid-looking capturegroup indices within the given pattern string.
// Each inner list is a tuple of start:end indices that can be used to extract a capture group.
// For example:
//
//	groups := CaptureGroupIndex(pattern)
//	loc := groups[0]
//	cg := pattern[loc[0],loc[1]]
func CapturegroupIndex(pattern string) []CgInfo {
	result := make([]CgInfo, 0)
	// The outer loop finds the beginning of the next named capturegroup
	for i := 0; i < len(pattern); i++ {
		idx := strings.Index(pattern[i:], "(?<")
		if idx == -1 {
			break
		}
		cg := CgInfo{
			Start: idx + i,
		}
		i = cg.Start + 3
		// Find the end of the capturegroup name
	CgName:
		for ; i < len(pattern); i++ {
			switch pattern[i] {
			case '\\':
				// Escape next character
				i++
			case '>':
				cg.Name = pattern[(cg.Start + 3):i]
				i++
				break CgName
			}
		}
		pDepth := 0
		cDepth := 0
		// Find the end of this capturegroup
		for ; i < len(pattern); i++ {
			switch pattern[i] {
			case '\\':
				// Escape next character
				i++
			case '(':
				if cDepth > 0 {
					continue
				}
				pDepth++
			case ')':
				if cDepth > 0 {
					continue
				}
				pDepth--
			case '[':
				cDepth++
			case ']':
				cDepth--
			}
			if pDepth < 0 {
				// Exited this capture group; record it
				cg.End = i + 1
				result = append(result, cg)
				break
			}
		}
	}
	return result
}

type CgMatches map[string][]string

// Returns the full matching pattern and the related CgMatches structure, appending to the given previous capture list, if given
// Returns: captures, fullmatch, err
func AppendCgMatches(captures CgMatches, pattern, value string) (CgMatches, string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return captures, "", fmt.Errorf("%q regex compilation failed: %w", pattern, err)
	}
	if matches := re.FindStringSubmatch(value); matches != nil {
		if captures == nil {
			captures = make(map[string][]string)
		}
		// Record all named subgroups
		for i, cgName := range re.SubexpNames() {
			if i == 0 || cgName == "" {
				continue
			}
			if !slices.Contains(captures[cgName], matches[i]) {
				captures[cgName] = append(captures[cgName], matches[i])
			}
		}
		return captures, matches[0], nil
	}
	return captures, "", nil
}
