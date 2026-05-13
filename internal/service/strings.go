package service

import "strings"

// joinWithNewline appends input to existing, inserting a newline between them
// unless one is already present at the join point. Shared by memory.append and
// doc.append.
func joinWithNewline(existing, input string) string {
	if !strings.HasSuffix(existing, "\n") && !strings.HasPrefix(input, "\n") {
		return existing + "\n" + input
	}
	return existing + input
}
