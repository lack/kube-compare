package compare

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCapturegroupIndex(t *testing.T) {
	tests := []struct {
		pattern  string
		expected []string
	}{
		{"", []string{}},
		{"Text with no capture groups!", []string{}},
		{"[a-z]+(looks)?(like)?(regex)?", []string{}},
		{"Incomplete (?<bad_name", []string{}},
		{"Incomplete (?<name_escape\\>bad)", []string{}},
		{"Incomplete (?<no_end>[a-z]+", []string{}},
		{"Incomplete (?<false end>[0-9()]+", []string{}},
		{"Incomplete (?<escaped end>here\\)", []string{}},
		{"(?<simple_group>.*)", []string{"(?<simple_group>.*)"}},
		{"  (?<simple_group>.*)  ", []string{"(?<simple_group>.*)"}},
		{"(?<tricky_group>[^)(]*)", []string{"(?<tricky_group>[^)(]*)"}},
		{"(?<escape_group>.*\\).*)", []string{"(?<escape_group>.*\\).*)"}},
		{"(?<cclass_group>[[:alpha:]]+)", []string{"(?<cclass_group>[[:alpha:]]+)"}},
		{"[text before]((?<hidden_group>.*))[text after]", []string{"(?<hidden_group>.*)"}},
		{"(?<group_with_groups>(?<inner1>.*(?<inner2>.*))?)", []string{"(?<group_with_groups>(?<inner1>.*(?<inner2>.*))?)"}},
		{"(?<one>.*)(?<two>.*)", []string{"(?<one>.*)", "(?<two>.*)"}},
		{"Two groups (?<first>.*) in a (?<second>.*) string", []string{"(?<first>.*)", "(?<second>.*)"}},
	}
	for _, c := range tests {
		matches := CapturegroupIndex(c.pattern)
		if !assert.Equal(t, len(c.expected), len(matches), fmt.Sprintf("Result length match for %q", c.pattern)) {
			continue
		}
		for i, m := range matches {
			expected := c.expected[i]
			assert.Equal(t, expected, c.pattern[m[0]:m[1]], fmt.Sprintf("Expected capture group %d", i))
		}
	}
}

func TestCapturegroupQuoteMeta(t *testing.T) {
	tests := []struct {
		pattern  string
		expected string
	}{
		{"", ""},
		{"plain text", "plain text"},
		{"[a-z]+(looks)?(like)?(regex)?", "\\[a-z\\]\\+\\(looks\\)\\?\\(like\\)\\?\\(regex\\)\\?"},
		{"(?<simple_group>.*)", "(?<simple_group>.*)"},
		{"[(?<group_in_brackets>[^\\]]+)]", "\\[(?<group_in_brackets>[^\\]]+)\\]"},
	}
	for _, c := range tests {
		actual := CapturegroupQuoteMeta(c.pattern)
		assert.Equal(t, c.expected, actual)
	}
}

func mlString(lines []string) string {
	return strings.Join(lines, "\n")
}

func TestCapturegroupsDiff(t *testing.T) {
	type Case struct {
		message  string
		value    []string
		expected []string
	}
	suites := []struct {
		message string
		pattern []string
		cases   []Case
	}{
		{
			message: "Empty pattern",
			pattern: []string{""},
			cases: []Case{
				{
					message:  "empty value",
					expected: []string{""},
				},
				{
					message:  "nonempty value",
					value:    []string{"something"},
					expected: []string{""},
				},
			},
		},
		{
			message: "No capturegroups",
			pattern: []string{"one", "two", "three"},
			cases: []Case{
				{
					message:  "empty value",
					expected: []string{"one", "two", "three"},
				},
				{
					message:  "matching value",
					value:    []string{"one", "two", "three"},
					expected: []string{"one", "two", "three"},
				},
				{
					message:  "mismatched value",
					value:    []string{"phone", "a two is here", "threenager"},
					expected: []string{"one", "two", "three"},
				},
			},
		},
		{
			message: "One capturegroup",
			// TODO: Remove need for word boundaries
			pattern: []string{"one", "(?<g1>\\b[a-z]+\\b)", "three"},
			cases: []Case{
				{
					message:  "empty value",
					expected: []string{"one", "(?<g1>\\b[a-z]+\\b)", "three"},
				},
				{
					message:  "mismatches pattern",
					value:    []string{"one", "2two2", "three"},
					expected: []string{"one", "(?<g1>\\b[a-z]+\\b)", "three"},
				},
				{
					message:  "matching pattern",
					value:    []string{"one", "two", "three"},
					expected: []string{"one", "two", "three"},
				},
			},
		},
		{
			message: "One capturegroup allowing spaces",
			pattern: []string{"one", "(?<g1>\\b[a-z\\s]+\\b)", "three"},
			cases: []Case{
				{
					message:  "empty value",
					expected: []string{"one", "(?<g1>\\b[a-z\\s]+\\b)", "three"},
				},
				{
					message:  "mismatches pattern",
					value:    []string{"one", "2two2", "three"},
					expected: []string{"one", "(?<g1>\\b[a-z\\s]+\\b)", "three"},
				},
				{
					message:  "matching pattern",
					value:    []string{"one", "two point five", "three"},
					expected: []string{"one", "two point five", "three"},
				},
			},
		},
	}
	for _, s := range suites {
		for _, c := range s.cases {
			cg := CapturegroupsInlineDiff{}
			actual := cg.Diff(mlString(s.pattern), mlString(c.value))
			assert.Equal(t, mlString(c.expected), actual, fmt.Sprintf("%s: %s", s.message, c.message))
		}
	}
}
