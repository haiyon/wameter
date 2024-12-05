package utils

import "strings"

// NormalizeString replaces full-width characters with their half-width counterparts
// and trims leading/trailing spaces.
func NormalizeString(str string) string {
	replacer := strings.NewReplacer(
		"，", ",",
		"；", ";",
		"：", ":",
		"。", ".",
		"！", "!",
		"？", "?",
		"（", "(",
		"）", ")",
		"【", "[",
		"】", "]",
		"“", "\"",
		"”", "\"",
		"‘", "'",
		"’", "'",
		" ", " ",
	)
	return strings.TrimSpace(replacer.Replace(str))
}
