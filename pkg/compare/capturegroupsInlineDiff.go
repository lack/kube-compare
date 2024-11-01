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

func isCapturegroup(pattern string) bool {
	// TODO: Maybe be more exact?
	return strings.HasPrefix(pattern, "(?<")
}

// If reconciliation was possible, returns the reconciled text.
// error is 'nil' only when reconciliation succeeds
func (id *CapturegroupsInlineDiff) reconcileViaRegex(deletion, insertion diffmatchpatch.Diff) (string, error) {
	// Quick sanity check
	if deletion.Type != diffmatchpatch.DiffDelete || insertion.Type != diffmatchpatch.DiffInsert {
		return "", fmt.Errorf("deletion.Type %s!=DiffDelete or insertion.Type %s!=DiffInsert", deletion.Type.String(), insertion.Type.String())
	}

	// The delete side is always the pattern
	pattern := deletion.Text
	value := insertion.Text
	// Ensure we're only working with capturegroups
	if !isCapturegroup(pattern) {
		// Not a capturegroup: No reconciliation possible
		return "", fmt.Errorf("LHS %q is not a capturegroup", pattern)
	}

	// Compile the capturegroup and attempt to match it
	re, err := regexp.Compile(pattern)
	if err != nil {
		// Note: Should not usually be possible, because of the 'validate' function below, but:
		return "", fmt.Errorf("LHS %q regex compilation failed: %w", pattern, err)
	}
	if re.MatchString(value) {
		// TODO: Retain the matched capturegroup contents for later validation
		// Regex match!  Return the reconciled string:
		return value, nil
	}
	// Not an error, but it didn't match
	return "", nil
}

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
		// For each line, look for capturegroup words and ensure they compile as regex
		for j, word := range strings.Split(line, " ") {
			// Note: Because we're splitting on line boundaries and spaces in
			// these loops, this has a side-effect of enforcing that the
			// capturegroup we're trying to compile here MUST NOT contain
			// linebreaks or spaces (See note in the diffHelper function to
			// learn why). (But the capture group CAN match spaces, with [\s] or similar)
			if isCapturegroup(word) {
				_, err := regexp.Compile(line)
				if err != nil {
					errs = errors.Join(errs, fmt.Errorf("Line %d word %d %w", i+1, j+1, err))
				}
			}
		}
	}
	return errs
}
