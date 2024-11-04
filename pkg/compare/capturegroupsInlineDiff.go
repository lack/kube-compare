package compare

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
	"k8s.io/klog/v2"
)

const (
	capturegroups inlineDiffType = "capturegroups"
)

type CapturegroupsInlineDiff struct {
	dmp   *diffmatchpatch.DiffMatchPatch
	diffs []diffmatchpatch.Diff
}

// Return a list of the valid-looking capturegroup indices within the given pattern string.
// Each inner list is a tuple of start:end indices that can be used to extract a capture group.
// For example:
//
//	groups := CaptureGroupIndex(pattern)
//	loc := groups[0]
//	cg := pattern[loc[0],loc[1]]
func CapturegroupIndex(pattern string) [][]int {
	result := make([][]int, 0)
	// The outer loop finds the beginning of the next named capturegroup
	for i := 0; i < len(pattern); i++ {
		idx := strings.Index(pattern[i:], "(?<")
		if idx == -1 {
			break
		}
		cStart := idx + i
		i = cStart + 3
		// Find the end of the capturegroup name
	CgName:
		for ; i < len(pattern); i++ {
			switch pattern[i] {
			case '\\':
				// Escape next character
				i++
			case '>':
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
				result = append(result, []int{cStart, i + 1})
				break
			}
		}
	}
	return result
}

// Transforms all non-capturegroup text in the pattern via Regex.QuoteMeta(), reusing previously-computed group indices
func CapturegroupQuoteMetaWithGroups(pattern string, groups [][]int) string {
	results := []string{}
	last := 0
	for _, group := range groups {
		if last < group[0] {
			// Escape everything up to the capturegroup
			results = append(results, regexp.QuoteMeta(pattern[last:group[0]]))
		}
		// Append the capturegroup verbatim
		results = append(results, pattern[group[0]:group[1]])
		last = group[1]
	}
	if last < len(pattern) {
		// Escape everything after the last capturegroup
		results = append(results, regexp.QuoteMeta(pattern[last:]))
	}
	return strings.Join(results, "")
}

// Transforms all non-capturegroup text in the pattern via Regex.QuoteMeta()
func CapturegroupQuoteMeta(pattern string) string {
	return CapturegroupQuoteMetaWithGroups(pattern, CapturegroupIndex(pattern))
}

// If reconciliation was possible, returns the reconciled text.
// A result of ("", nil) means there were no parsing errors, but the difference was not reconcilable
func (id *CapturegroupsInlineDiff) reconcileViaRegex(deletion, insertion diffmatchpatch.Diff) (string, error) {
	// Quick sanity check
	if deletion.Type != diffmatchpatch.DiffDelete || insertion.Type != diffmatchpatch.DiffInsert {
		return "", fmt.Errorf("deletion.Type %s!=DiffDelete or insertion.Type %s!=DiffInsert", deletion.Type.String(), insertion.Type.String())
	}

	// The delete side is always the pattern
	pattern := deletion.Text
	value := insertion.Text

	// Compile the capturegroup (quoting any adjacent non-capturegroup parts) and attempt to match it
	re, err := regexp.Compile(CapturegroupQuoteMeta(pattern))
	if err != nil {
		// Note: Should not usually be possible, because of the 'validate' function below, but:
		return "", fmt.Errorf("LHS %q regex compilation failed: %w", pattern, err)
	}
	if loc := re.FindStringIndex(value); loc != nil {
		// TODO: Retain the matched capturegroup contents for later validation
		// Regex match!  Return the reconciled string:
		return value[loc[0]:loc[1]], nil
	}
	// Not an error, but it didn't match
	return "", nil
}

// Perform the diff, ensuring the diff parts are on line-boundaries, and recording the parts in id.diffs
func (id *CapturegroupsInlineDiff) doDiff(pattern, value string) {
	id.dmp = diffmatchpatch.New()
	diffs := id.dmp.DiffMain(pattern, value, false)
	// TODO: Need a custom cleanup that preserves full capture groups, not just wordbreaks
	// Until then, we enforce (in the 'validate' function below) that capturegroups must NOT contain spaces or linebreaks)
	id.diffs = id.dmp.DiffCleanupSemantic(diffs)
}

// Return the potentially-reconcilable diff pair to id.diffs[i] (ie, if
// id.diffs[i+1] represents an insert-then-delete pair or delete-then-insert
// pair), or nil if i+1 is out of bounds or does not constitute a proper
// potentially-reconcilable pair.
func (id *CapturegroupsInlineDiff) reconcilableDiffPair(i int) (*diffmatchpatch.Diff, *diffmatchpatch.Diff) {
	a := id.diffs[i]
	if i+1 < len(id.diffs) {
		b := id.diffs[i+1]
		if a.Type == diffmatchpatch.DiffInsert && b.Type == diffmatchpatch.DiffDelete {
			return &a, &b
		}
		if a.Type == diffmatchpatch.DiffDelete && b.Type == diffmatchpatch.DiffInsert {
			return &b, &a
		}
	}
	return nil, nil
}

// Main entrypoint called by compare.go
func (id CapturegroupsInlineDiff) Diff(pattern, value string) string {
	// First do a word-wise diff to isolate only those whole words that differ
	id.doDiff(pattern, value)
	results := make([]string, 0, len(id.diffs))
	// Then try to reconcile the diffs by treating any insert-then-delete or
	// delete-then-insert as a possible regex pair.  The goal is to render the
	// "pattern" side of the diff with any capture-groups replaced with
	// equivalent matches from the "value" side
	for i := 0; i < len(id.diffs); i++ {
		diff := id.diffs[i]
		switch diff.Type {
		case diffmatchpatch.DiffEqual:
			// Full-text match; Append as-is
			results = append(results, diff.Text)
		default:
			// If this is a delete-then-insert or insert-then-delete, check if
			// it's reconcilable via regex match
			if insertion, deletion := id.reconcilableDiffPair(i); insertion != nil && deletion != nil {
				reconciled, err := id.reconcileViaRegex(*deletion, *insertion)
				if err != nil {
					klog.Warningf("capturegroup error: %s", err)
					// Errors are intentionally nonfatal at this time.
					// Preferrably these would be caught in the 'validate'
					// function below.
				}
				if reconciled != "" {
					// Reconciliation successful! Record the reconciled string
					results = append(results, reconciled)
					// Also consume the following diff we have reconciled with
					i += 1
					continue
				}
				// Reconciliation failed: treat as a stand-alone difference
			}
			if diff.Type == diffmatchpatch.DiffDelete {
				// Normally, deletions should be rendered as-is, because they are
				// strings in the "pattern" side that were ommitted by the "value"
				// side, and we want to showcase those as diffs at a higher level.
				// Simple deletion: preserve the "pattern" side
				results = append(results, diff.Text)
			}
			// Normally, insertions should NOT be rendered, because they are
			// strings added by the "value" side and we want to showcase those
			// as diffs at a higher level.
		}
	}

	return strings.Join(results, "")
}

// Validation entrypoint called by referenceV2.go
func (id CapturegroupsInlineDiff) Validate(pattern string) error {
	var errs error
	for i, line := range strings.Split(pattern, "\n") {
		// Find all capturegroups in the line
		groups := CapturegroupIndex(line)
		// For each line, ensure our quoted capturegroup result is regex-compliant by compiling it
		_, err := regexp.Compile(CapturegroupQuoteMetaWithGroups(line, groups))
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("Line %d %w", i+1, err))
			continue
		}
		// Furthermore, ensure each capturegroup is valid for our purposes (ie, has no spaces)
		for _, loc := range groups {
			if strings.ContainsAny(line[loc[0]:loc[1]], " ") {
				errs = errors.Join(errs, fmt.Errorf("Line %d:%d capturegroup contains spaces", i+1, loc[0]))
			}
		}
	}
	return errs
}
