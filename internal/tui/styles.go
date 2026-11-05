package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	accent = lipgloss.AdaptiveColor{Light: "#5B3FD6", Dark: "#C4B5FD"}
	muted  = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#7C8394"}
	text   = lipgloss.AdaptiveColor{Light: "#111827", Dark: "#E5E7EB"}
	selBG  = lipgloss.AdaptiveColor{Light: "#DDD6FE", Dark: "#4C1D95"}
	selFG  = lipgloss.AdaptiveColor{Light: "#2E1065", Dark: "#F5F3FF"}
	red    = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#F87171"}
	green  = lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#4ADE80"}
	yellow = lipgloss.AdaptiveColor{Light: "#A16207", Dark: "#FACC15"}
	blue   = lipgloss.AdaptiveColor{Light: "#1D4ED8", Dark: "#60A5FA"}
	orange = lipgloss.AdaptiveColor{Light: "#C2410C", Dark: "#FB923C"}
	gray   = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
)

var (
	brandStyle     = lipgloss.NewStyle().Bold(true).Foreground(accent)
	dimStyle       = lipgloss.NewStyle().Foreground(muted)
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(text)
	keyStyle       = lipgloss.NewStyle().Bold(true).Foreground(accent)
	activeTabStyle = lipgloss.NewStyle().Bold(true).Foreground(selFG).Background(selBG).Padding(0, 1)
	requiredStyle  = lipgloss.NewStyle().Foreground(red)
)

var methodColors = map[string]lipgloss.AdaptiveColor{
	"GET":     blue,
	"POST":    green,
	"PUT":     yellow,
	"PATCH":   orange,
	"DELETE":  red,
	"HEAD":    gray,
	"OPTIONS": gray,
	"TRACE":   gray,
}

func methodStyle(method string) lipgloss.Style {
	c, ok := methodColors[strings.ToUpper(method)]
	if !ok {
		c = gray
	}
	return lipgloss.NewStyle().Bold(true).Foreground(c)
}

func statusStyle(status string) lipgloss.Style {
	switch {
	case strings.HasPrefix(status, "2"):
		return lipgloss.NewStyle().Bold(true).Foreground(green)
	case strings.HasPrefix(status, "3"):
		return lipgloss.NewStyle().Bold(true).Foreground(yellow)
	case strings.HasPrefix(status, "4"):
		return lipgloss.NewStyle().Bold(true).Foreground(orange)
	case strings.HasPrefix(status, "5"):
		return lipgloss.NewStyle().Bold(true).Foreground(red)
	default:
		return lipgloss.NewStyle().Bold(true).Foreground(gray)
	}
}

func paneStyle(focused bool) lipgloss.Style {
	border := muted
	if focused {
		border = accent
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border)
}
