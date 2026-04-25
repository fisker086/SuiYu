package logger

import (
	"fmt"
	"unicode/utf8"
)

// CallIDForLog returns a short fingerprint for log fields. Model-provided tool call IDs can be
// very long (e.g. base64); logging the full value floods logs and may embed sensitive material.
// The original byte length is appended so you can still correlate or spot anomalies.
func CallIDForLog(id string) string {
	if id == "" {
		return ""
	}
	n := len(id)
	if n <= 80 && utf8.RuneCountInString(id) <= 48 {
		return id
	}
	r := []rune(id)
	const maxRunes = 32
	if len(r) > maxRunes {
		r = r[:maxRunes]
	}
	return fmt.Sprintf("%s…(len=%d)", string(r), n)
}
