package utils

import (
	"fmt"
	"regexp"
)

// ConvertPlaceholders converts SQL query placeholders between positional (`?`) and PostgreSQL format (`$1`, `$2`, ...).
// If reverse is true, it converts `$1`, `$2`, ... back to `?`. If not provided, defaults to forward conversion.
func ConvertPlaceholders(query string, reverse ...bool) string {
	// Determine if we are reversing placeholders (default to false)
	isReverse := len(reverse) > 0 && reverse[0]

	if isReverse {
		// Reverse: Convert "$1", "$2", ... back to "?"
		return regexp.MustCompile(`\$\d+`).ReplaceAllString(query, "?")
	}

	// Forward: Convert "?" to "$1", "$2", ...
	count := 0
	return regexp.MustCompile(`\?`).ReplaceAllStringFunc(query, func(_ string) string {
		count++
		return fmt.Sprintf("$%d", count)
	})
}
