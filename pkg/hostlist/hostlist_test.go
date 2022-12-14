package hostlist

import (
	"testing"
)

func TestExpand(t *testing.T) {

	// Compare two slices of strings
	equal := func(a, b []string) bool {
		if len(a) != len(b) {
			return false
		}
		for i, v := range a {
			if v != b[i] {
				return false
			}
		}
		return true
	}

	type testDefinition struct {
		input          string
		resultExpected []string
		errorExpected  bool
	}

	expandTests := []testDefinition{
		{
			// Single node
			input:          "n1",
			resultExpected: []string{"n1"},
			errorExpected:  false,
		},
		{
			// Single node, duplicated
			input:          "n1,n1",
			resultExpected: []string{"n1"},
			errorExpected:  false,
		},
		{
			// Single node with padding
			input:          "n[01]",
			resultExpected: []string{"n01"},
			errorExpected:  false,
		},
		{
			// Single node with suffix
			input:          "n[01]-p",
			resultExpected: []string{"n01-p"},
			errorExpected:  false,
		},
		{
			// Multiple nodes with a single range
			input:          "n[1-2]",
			resultExpected: []string{"n1", "n2"},
			errorExpected:  false,
		},
		{
			// Multiple nodes with a single range and a single index
			input:          "n[1-2,3]",
			resultExpected: []string{"n1", "n2", "n3"},
			errorExpected:  false,
		},
		{
			// Multiple nodes with different prefixes
			input:          "n[1-2],m[1,2]",
			resultExpected: []string{"m1", "m2", "n1", "n2"},
			errorExpected:  false,
		},
		{
			// Multiple nodes with different suffixes
			input:          "n[1-2]-p,n[1,2]-q",
			resultExpected: []string{"n1-p", "n1-q", "n2-p", "n2-q"},
			errorExpected:  false,
		},
		{
			// Multiple nodes with and without node ranges
			input:          " n09, n[01-04,06-07,09] , , n10,n04",
			resultExpected: []string{"n01", "n02", "n03", "n04", "n06", "n07", "n09", "n10"},
			errorExpected:  false,
		},
		{
			// Forbidden DNS character
			input:          "n@",
			resultExpected: []string{},
			errorExpected:  true,
		},
		{
			// Forbidden range
			input:          "n[1-2-2,3]",
			resultExpected: []string{},
			errorExpected:  true,
		},
		{
			// Forbidden range limits
			input:          "n[2-1]",
			resultExpected: []string{},
			errorExpected:  true,
		},
	}

	for _, expandTest := range expandTests {
		result, err := Expand(expandTest.input)

		hasError := err != nil
		if hasError != expandTest.errorExpected && hasError {
			t.Errorf("Expand('%s') failed: unexpected error '%v'",
				expandTest.input, err)
			continue
		}
		if hasError != expandTest.errorExpected && !hasError {
			t.Errorf("Expand('%s') did not fail as expected: got result '%+v'",
				expandTest.input, result)
			continue
		}
		if !hasError && !equal(result, expandTest.resultExpected) {
			t.Errorf("Expand('%s') failed: got result '%+v', expected result '%v'",
				expandTest.input, result, expandTest.resultExpected)
			continue
		}

		t.Logf("Checked hostlist.Expand('%s'): result = '%+v', err = '%v'",
			expandTest.input, result, err)
	}
}
