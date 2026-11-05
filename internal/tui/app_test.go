package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/baptisteChouraih/LazyEndpoint/internal/spec"
)

func newTestModel(t *testing.T) Model {
	t.Helper()
	doc, err := spec.Load("../../examples/petstore.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	m := New(doc, Options{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return updated.(Model)
}

func key(s string) tea.KeyMsg {
	switch s {
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func press(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, k := range keys {
		updated, _ := m.Update(key(k))
		m = updated.(Model)
	}
	return m
}

func TestViewFillsTerminalWidth(t *testing.T) {
	m := newTestModel(t)
	view := m.View()

	lines := strings.Split(view, "\n")
	if len(lines) != m.height {
		t.Fatalf("view has %d lines, want %d", len(lines), m.height)
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got != m.width {
			t.Fatalf("line %d is %d cells wide, want %d: %q", i, got, m.width, line)
		}
	}
	for _, want := range []string{"LazyEndpoint", "/pets/{petId}", "PETS"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q", want)
		}
	}
}

func TestNavigationAndTabs(t *testing.T) {
	m := newTestModel(t)
	first := m.selected()
	if first == nil {
		t.Fatal("no endpoint selected")
	}

	m = press(t, m, "j")
	if m.selected() == first {
		t.Fatal("j did not move the cursor")
	}

	m = press(t, m, "4")
	if m.tab != tabResponses {
		t.Fatalf("tab = %v, want responses", m.tab)
	}
	if got := m.view.View(); !strings.Contains(got, "422") {
		t.Fatalf("responses tab did not render the response codes:\n%s", got)
	}

	// A short terminal makes the responses tab taller than the pane.
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	short := updated.(Model)
	short = press(t, short, "4", "G")
	if short.view.YOffset == 0 {
		t.Fatal("G did not scroll the details")
	}
}

func TestFilterNarrowsTheList(t *testing.T) {
	m := newTestModel(t)
	before := m.visibleEndpoints()

	m = press(t, m, "/")
	if !m.editing {
		t.Fatal("expected filter input to be active")
	}
	m = press(t, m, "s", "t", "o", "r", "e")
	m = press(t, m, "enter")

	if m.visibleEndpoints() >= before {
		t.Fatalf("filter did not narrow the list: %d -> %d", before, m.visibleEndpoints())
	}
	if got := m.selected(); got == nil || got.Group != "store" {
		t.Fatalf("selected endpoint is not from the store group: %+v", got)
	}

	m = press(t, m, "esc")
	if m.query != "" {
		t.Fatalf("query not cleared: %q", m.query)
	}
	if m.visibleEndpoints() != before {
		t.Fatalf("list not restored: %d != %d", m.visibleEndpoints(), before)
	}
}

func TestInitialTagFilter(t *testing.T) {
	doc, err := spec.Load("../../examples/petstore.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	m := New(doc, Options{InitialTag: "users"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)

	if got := m.visibleEndpoints(); got != 1 {
		t.Fatalf("endpoints = %d, want 1", got)
	}
	if ep := m.selected(); ep == nil || ep.Group != "users" {
		t.Fatalf("selected = %+v", ep)
	}
}

func TestHelpAndSmallTerminal(t *testing.T) {
	m := newTestModel(t)
	view := press(t, m, "?").View()
	if !strings.Contains(view, "keyboard shortcuts") {
		t.Fatal("help overlay not shown")
	}

	m = press(t, m, "x")
	if strings.Contains(m.View(), "keyboard shortcuts") {
		t.Fatal("help overlay did not close")
	}

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 10})
	small := updated.(Model)
	if !small.tooSmall {
		t.Fatal("expected small terminal to be flagged")
	}
	if !strings.Contains(small.View(), "terminal too small") {
		t.Fatal("missing small-terminal message")
	}
}
