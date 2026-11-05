package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// truncate shortens s to at most w cells, appending an ellipsis. It is aware of
// ANSI escape codes, so styled text can be truncated safely.
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}
	return ansi.Truncate(s, w-1, "…")
}

// fillWidth pads s with spaces to exactly w cells, truncating when needed.
func fillWidth(s string, w int) string {
	if lipgloss.Width(s) > w {
		s = truncate(s, w)
	}
	if gap := w - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

func padRight(s string, w int) string {
	gap := w - lipgloss.Width(s)
	if gap <= 0 {
		return s
	}
	return s + strings.Repeat(" ", gap)
}

// wrapText word-wraps plain text to w cells, preserving blank lines.
func wrapText(s string, w int) []string {
	if w < 8 {
		w = 8
	}
	s = strings.TrimRight(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	if strings.TrimSpace(s) == "" {
		return nil
	}

	var lines []string
	for _, para := range strings.Split(s, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		current := ""
		for _, word := range words {
			for lipgloss.Width(word) > w {
				chunk, rest := splitAt(word, w)
				if current != "" {
					lines = append(lines, current)
					current = ""
				}
				lines = append(lines, chunk)
				word = rest
			}
			switch {
			case current == "":
				current = word
			case lipgloss.Width(current)+1+lipgloss.Width(word) <= w:
				current += " " + word
			default:
				lines = append(lines, current)
				current = word
			}
		}
		if current != "" {
			lines = append(lines, current)
		}
	}
	return lines
}

// splitAt cuts s after w cells.
func splitAt(s string, w int) (head, tail string) {
	used := 0
	for i, r := range s {
		rw := lipgloss.Width(string(r))
		if used+rw > w {
			return s[:i], s[i:]
		}
		used += rw
	}
	return s, ""
}

// indented wraps text and prefixes every produced line.
func indented(s string, w int, prefix string) []string {
	wrapped := wrapText(s, w-lipgloss.Width(prefix))
	out := make([]string, 0, len(wrapped))
	for _, line := range wrapped {
		out = append(out, prefix+line)
	}
	return out
}

// metaLines renders "Label  value" rows with a fixed label column.
func metaLines(pairs [][2]string, w int) []string {
	const labelWidth = 13
	var out []string
	for _, pair := range pairs {
		if pair[1] == "" {
			continue
		}
		label := dimStyle.Render(padRight(pair[0], labelWidth))
		for i, line := range indented(pair[1], w-labelWidth, "") {
			if i == 0 {
				out = append(out, label+line)
				continue
			}
			out = append(out, strings.Repeat(" ", labelWidth)+line)
		}
	}
	return out
}
