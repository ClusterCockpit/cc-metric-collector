package hostlist

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func Expand(in string) (result []string, err error) {

	// Create ranges regular expression
	reStNumber := "[[:digit:]]+"
	reStRange := reStNumber + "-" + reStNumber
	reStOptionalNumberOrRange := "(" + reStNumber + ",|" + reStRange + ",)*"
	reStNumberOrRange := "(" + reStNumber + "|" + reStRange + ")"
	reStBraceLeft := "[[]"
	reStBraceRight := "[]]"
	reStRanges := reStBraceLeft +
		reStOptionalNumberOrRange +
		reStNumberOrRange +
		reStBraceRight
	reRanges := regexp.MustCompile(reStRanges)

	// Create host list regular expression
	reStDNSChars := "[a-zA-Z0-9-]+"
	reStPrefix := "^(" + reStDNSChars + ")"
	reStOptionalSuffix := "(" + reStDNSChars + ")?"
	re := regexp.MustCompile(reStPrefix + "([[][0-9,-]+[]])?" + reStOptionalSuffix)

	// Remove all delimiters from the input
	in = strings.TrimLeft(in, ", ")

	for len(in) > 0 {
		if v := re.FindStringSubmatch(in); v != nil {

			// Remove matched part from the input
			lenPrefix := len(v[0])
			in = in[lenPrefix:]

			// Remove all delimiters from the input
			in = strings.TrimLeft(in, ", ")

			// matched prefix, range and suffix
			hlPrefix := v[1]
			hlRanges := v[2]
			hlSuffix := v[3]

			// Single node without ranges
			if hlRanges == "" {
				result = append(result, hlPrefix)
				continue
			}

			// Node with ranges
			if v := reRanges.FindStringSubmatch(hlRanges); v != nil {

				// Remove braces
				hlRanges = hlRanges[1 : len(hlRanges)-1]

				// Split host ranges at ,
				for _, hlRange := range strings.Split(hlRanges, ",") {

					// Split host range at -
					RangeStartEnd := strings.Split(hlRange, "-")

					// Range is only a single number
					if len(RangeStartEnd) == 1 {
						result = append(result, hlPrefix+RangeStartEnd[0]+hlSuffix)
						continue
					}

					// Range has a start and an end
					widthRangeStart := len(RangeStartEnd[0])
					widthRangeEnd := len(RangeStartEnd[1])
					iStart, _ := strconv.ParseUint(RangeStartEnd[0], 10, 64)
					iEnd, _ := strconv.ParseUint(RangeStartEnd[1], 10, 64)
					if iStart > iEnd {
						return nil, fmt.Errorf("single range start is greater than end: %s", hlRange)
					}

					// Create print format string for range numbers
					doPadding := widthRangeStart == widthRangeEnd
					widthPadding := widthRangeStart
					var formatString string
					if doPadding {
						formatString = "%0" + fmt.Sprint(widthPadding) + "d"
					} else {
						formatString = "%d"
					}
					formatString = hlPrefix + formatString + hlSuffix

					// Add nodes from this range
					for i := iStart; i <= iEnd; i++ {
						result = append(result, fmt.Sprintf(formatString, i))
					}
				}
			} else {
				return nil, fmt.Errorf("not at hostlist range: %s", hlRanges)
			}
		} else {
			return nil, fmt.Errorf("not a hostlist: %s", in)
		}
	}

	if result != nil {
		// sort
		sort.Strings(result)

		// uniq
		previous := 1
		for current := 1; current < len(result); current++ {
			if result[current-1] != result[current] {
				if previous != current {
					result[previous] = result[current]
				}
				previous++
			}
		}
		result = result[:previous]
	}

	return
}
