package utils

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
)

// NormalizeString replaces full-width characters with their half-width counterparts
// and trims leading/trailing spaces.
// Usage: NormalizeString("yourInputString")
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

// Default short hash length
const defaultHashLength = 9

// ShortHash generates a short hash of the input string.
// Usage: ShortHash("yourInputString", desiredLength)
func ShortHash(input string, length ...int) string {
	if input == "" {
		return ""
	}
	l := defaultHashLength
	if len(length) > 0 && length[0] > 0 {
		l = length[0]
	}

	hash := sha1.Sum([]byte(input))
	hashStr := hex.EncodeToString(hash[:])

	// Ensure the hash string is at least as long as the desired length
	if l > len(hashStr) {
		return hashStr // Return the full hash if it's shorter than the desired length
	}

	return hashStr[:l]
}
