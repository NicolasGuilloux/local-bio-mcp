package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// emitJSON writes v as indented JSON.
func emitJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// table renders a simple aligned text table.
func table(w io.Writer, headers []string, rows [][]string) {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = utf8.RuneCountInString(h)
	}
	for _, r := range rows {
		for i, c := range r {
			if w := utf8.RuneCountInString(c); i < len(widths) && w > widths[i] {
				widths[i] = w
			}
		}
	}
	printRow := func(cols []string) {
		parts := make([]string, len(cols))
		for i, c := range cols {
			parts[i] = padRight(c, widths[i])
		}
		fmt.Fprintln(w, strings.TrimRight(strings.Join(parts, "  "), " "))
	}
	printRow(headers)
	sep := make([]string, len(headers))
	for i := range sep {
		sep[i] = strings.Repeat("-", widths[i])
	}
	printRow(sep)
	for _, r := range rows {
		printRow(r)
	}
}

func padRight(s string, n int) string {
	w := utf8.RuneCountInString(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

// categoryLabel turns a raw categoryId like "vegetables~mash" into "vegetables".
func categoryLabel(id string) string {
	if id == "" {
		return ""
	}
	if i := strings.IndexByte(id, '~'); i >= 0 {
		return id[:i]
	}
	return id
}

// qty formats a possibly-fractional quantity without trailing zeros.
func qty(v float64) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	return s
}

func money(v float64) string {
	if v == 0 {
		return ""
	}
	return fmt.Sprintf("%.2f€", v)
}
