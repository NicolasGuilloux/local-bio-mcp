package cli

import "strings"

// joinArgs joins positional args into a single query string.
func joinArgs(args []string) string {
	return strings.TrimSpace(strings.Join(args, " "))
}
