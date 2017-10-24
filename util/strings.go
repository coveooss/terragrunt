package util

import (
	"strings"
)

// UnIndent returns the unindented version of the supplied string
func UnIndent(s string) string {
	if !strings.HasPrefix(s, " ") || !strings.Contains(s, "\n") {
		// There is no indentation on the first line
		return s
	}
	split := strings.Split(s, "\n")

	// Count the number of spaces on the first line
	i := 0
	for ; i < len(split[0]); i++ {
		if split[0][i] != ' ' {
			break
		}
	}

	// Remove the indentation (preserving the indentation relative to the first line)
	prefix := strings.Repeat(" ", i)
	for i := range split {
		split[i] = strings.TrimPrefix(split[i], prefix)
	}
	return strings.Join(split, "\n")
}

// Indent returns the indented version of the supplied string
func Indent(s string, indent int) string {
	split := strings.Split(s, "\n")
	space := strings.Repeat(" ", indent)

	for i := range split {
		split[i] = space + split[i]
	}
	return strings.Join(split, "\n")
}
