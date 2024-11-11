package compare

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		t.Run(fmt.Sprintf("Pattern %q", c.pattern), func(t *testing.T) {
			matches := CapturegroupIndex(c.pattern)
			require.Equal(t, len(c.expected), len(matches), fmt.Sprintf("Result length match for %q", c.pattern))
			for i, m := range matches {
				t.Run(fmt.Sprintf("Group %d:%d", i+1, len(matches)), func(t *testing.T) {
					expected := c.expected[i]
					assert.Equal(t, expected, c.pattern[m.Start:m.End], fmt.Sprintf("Expected capture group %d", i))
					nameEnd := strings.Index(expected, ">")
					expectedName := expected[3:nameEnd]
					assert.Equal(t, expectedName, m.Name, fmt.Sprintf("Expected capture group %d name match", i))
				})
			}
		})
	}
}
