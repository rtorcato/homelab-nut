package ups

import (
	"fmt"
	"strings"
)

// parseLine tokenizes one NUT protocol response line. Bare tokens are
// whitespace-separated; quoted strings (delimited by ") are kept as a
// single token with NUT's backslash escapes processed (`\"` → `"`,
// `\\` → `\`, anything else after `\` is taken literally).
//
// The input must not include the trailing newline.
func parseLine(line string) ([]string, error) {
	var tokens []string
	var cur strings.Builder
	i := 0
	for i < len(line) {
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		if i >= len(line) {
			break
		}
		if line[i] == '"' {
			i++
			cur.Reset()
			for i < len(line) && line[i] != '"' {
				if line[i] == '\\' && i+1 < len(line) {
					cur.WriteByte(line[i+1])
					i += 2
					continue
				}
				cur.WriteByte(line[i])
				i++
			}
			if i >= len(line) {
				return nil, fmt.Errorf("%w: unterminated quoted string in %q", ErrProtocol, line)
			}
			i++ // closing quote
			tokens = append(tokens, cur.String())
			continue
		}
		// bare token
		cur.Reset()
		for i < len(line) && line[i] != ' ' && line[i] != '\t' {
			cur.WriteByte(line[i])
			i++
		}
		tokens = append(tokens, cur.String())
	}
	return tokens, nil
}

// mapErr converts a NUT "ERR <reason>" reason token into a typed error.
// Known reasons map to sentinels callers can match with errors.Is;
// anything else falls through to *Error preserving the raw reason.
func mapErr(reason string) error {
	switch reason {
	case "ACCESS-DENIED":
		return ErrAccessDenied
	case "UNKNOWN-UPS":
		return ErrUnknownUPS
	case "VAR-NOT-SUPPORTED":
		return ErrVarNotSupported
	}
	return &Error{Reason: reason}
}
