package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/baptisteChouraih/LazyEndpoint/internal/spec"
)

const (
	minWidth  = 60
	minHeight = 14
)

type tab int

const (
	tabOverview tab = iota
	tabParams
	tabBody
	tabResponses
)

var tabNames = []string{"Overview", "Parameters", "Body", "Responses"}

type pane int

const (
	paneList pane = iota
	paneDetail
)

type rowKind int

const (
	rowHeader rowKind = iota
	rowEndpoint
)

type row struct {
	kind rowKind
	name string
	ep   *spec.Endpoint
}

// Options configures the initial state of the model.
type Options struct {
	InitialTag string
}

// Model is the Bubble Tea model of the application.
type Model struct {
	doc  *spec.Document
	opts Options

	width, height int
	ready         bool
	tooSmall      bool

	rows      []row
	cursor    int
	offset    int
	query     string
	tagFilter string
	input     textinput.Model
	editing   bool

	pane     pane
	tab      tab
	view     viewport.Model
	showHelp bool
	try      *tryState
}

// New builds the model for a document.
func New(doc *spec.Document, opts Options) Model {
	input := textinput.New()
	input.Prompt = "/ "
	input.Placeholder = "filter endpoints"
	input.CharLimit = 80

	m := Model{
		doc:       doc,
		opts:      opts,
		input:     input,
		pane:      paneList,
		tab:       tabOverview,
		view:      viewport.New(0, 0),
		tagFilter: strings.TrimSpace(opts.InitialTag),
	}
	if doc != nil {
		m.rebuild()
	}
	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ready = true
		m.layout()
		return m, nil

	case trySentMsg:
		m.applyTryResult(msg)
		return m, nil

	case tea.KeyMsg:
		if m.showHelp {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			}
			m.showHelp = false
			return m, nil
		}
		if m.try != nil {
			return m.updateTry(msg)
		}
		if m.editing {
			return m.updateFilter(msg)
		}
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.editing = false
		m.input.Blur()
		m.query = ""
		m.input.SetValue("")
		m.rebuild()
		m.refresh(true)
		return m, nil
	case "enter":
		m.editing = false
		m.input.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if query := m.input.Value(); query != m.query {
		m.query = query
		m.rebuild()
		m.refresh(true)
	}
	return m, cmd
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "?":
		m.showHelp = true

	case "t":
		if ep := m.selected(); ep != nil {
			m.try = newTryState(ep)
			m.layoutTry()
		}

	case "/":
		m.editing = true
		m.input.Focus()
		m.input.CursorEnd()
		return m, textinput.Blink

	case "esc":
		if m.query != "" {
			m.query = ""
			m.input.SetValue("")
			m.rebuild()
			m.refresh(true)
		} else {
			m.pane = paneList
		}

	case "tab":
		if m.pane == paneList {
			m.pane = paneDetail
		} else {
			m.pane = paneList
		}

	case "left", "h":
		m.pane = paneList

	case "right", "l":
		m.pane = paneDetail

	case "1", "2", "3", "4":
		m.tab = tab(msg.String()[0] - '1')
		m.pane = paneDetail
		m.refresh(true)

	case "[", "]":
		next := (int(m.tab) + 1) % len(tabNames)
		if msg.String() == "[" {
			next = (int(m.tab) + len(tabNames) - 1) % len(tabNames)
		}
		m.tab = tab(next)
		m.refresh(true)

	case "j", "down":
		if m.pane == paneList {
			m.move(1)
		} else {
			m.view.LineDown(1)
		}

	case "k", "up":
		if m.pane == paneList {
			m.move(-1)
		} else {
			m.view.LineUp(1)
		}

	case "ctrl+d":
		m.view.HalfPageDown()
	case "ctrl+u":
		m.view.HalfPageUp()
	case "pgdown", " ":
		m.view.LineDown(m.view.Height - 1)
	case "pgup":
		m.view.LineUp(m.view.Height - 1)
	case "g", "home":
		m.view.GotoTop()
	case "G", "end":
		m.view.GotoBottom()
	}
	return m, nil
}

// rebuild recomputes the visible rows from the document and the filters.
func (m *Model) rebuild() {
	previous := m.selected()
	m.rows = m.rows[:0]
	m.offset = 0

	if m.doc != nil {
		for _, group := range m.doc.Groups {
			if m.tagFilter != "" && !strings.EqualFold(group.Name, m.tagFilter) {
				continue
			}
			var matched []*spec.Endpoint
			for _, ep := range group.Endpoints {
				if ep.Matches(m.query) {
					matched = append(matched, ep)
				}
			}
			if len(matched) == 0 {
				continue
			}
			m.rows = append(m.rows, row{kind: rowHeader, name: group.Name})
			for _, ep := range matched {
				m.rows = append(m.rows, row{kind: rowEndpoint, ep: ep})
			}
		}
	}

	m.cursor = m.firstEndpoint()
	if previous != nil {
		for i, r := range m.rows {
			if r.kind == rowEndpoint && r.ep == previous {
				m.cursor = i
				break
			}
		}
	}
}

func (m Model) firstEndpoint() int {
	for i, r := range m.rows {
		if r.kind == rowEndpoint {
			return i
		}
	}
	return 0
}

// selected returns the endpoint under the cursor, if any.
func (m Model) selected() *spec.Endpoint {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	r := m.rows[m.cursor]
	if r.kind != rowEndpoint {
		return nil
	}
	return r.ep
}

// move steps the cursor by delta, skipping group headers.
func (m *Model) move(delta int) {
	if len(m.rows) == 0 {
		return
	}
	for i := m.cursor + delta; i >= 0 && i < len(m.rows); i += delta {
		if m.rows[i].kind == rowEndpoint {
			m.cursor = i
			m.refresh(true)
			return
		}
	}
}

func (m Model) visibleEndpoints() int {
	n := 0
	for _, r := range m.rows {
		if r.kind == rowEndpoint {
			n++
		}
	}
	return n
}

// layout recomputes pane sizes for the current terminal size.
func (m *Model) layout() {
	m.tooSmall = m.width < minWidth || m.height < minHeight
	if m.tooSmall {
		return
	}
	viewWidth := m.detailWidth() - 2
	viewHeight := m.bodyHeight() - 4
	if viewWidth < 10 || viewHeight < 1 {
		m.tooSmall = true
		return
	}
	m.view.Width = viewWidth
	m.view.Height = viewHeight
	m.input.Width = max(8, m.listWidth()-5)
	m.refresh(false)
	m.layoutTry()
}

// refresh re-renders the detail pane; reset scrolls it back to the top.
func (m *Model) refresh(reset bool) {
	if !m.ready || m.tooSmall {
		return
	}
	ep := m.selected()
	width := m.detailWidth() - 2

	var lines []string
	switch {
	case m.doc == nil || m.doc.EndpointCount() == 0:
		lines = []string{"", dimStyle.Render("  This specification documents no endpoints.")}
	case ep == nil:
		lines = []string{"", dimStyle.Render("  No endpoint matches the current filter.")}
	default:
		lines = m.tabLines(ep, width)
	}

	offset := m.view.YOffset
	m.view.SetContent(strings.Join(lines, "\n"))
	if reset {
		m.view.GotoTop()
	} else {
		m.view.SetYOffset(offset)
	}
}

func (m Model) tabLines(ep *spec.Endpoint, width int) []string {
	switch m.tab {
	case tabParams:
		return paramLines(ep, width)
	case tabBody:
		return bodyLines(ep, width)
	case tabResponses:
		return responseLines(ep, width)
	default:
		return overviewLines(ep, width)
	}
}

func (m Model) listWidth() int {
	w := m.width / 3
	if w < 30 {
		w = 30
	}
	if w > 48 {
		w = 48
	}
	if limit := m.width - 24; w > limit {
		w = limit
	}
	return w
}

func (m Model) detailWidth() int { return m.width - m.listWidth() }

func (m Model) bodyHeight() int { return m.height - 2 }

// View implements tea.Model.
func (m Model) View() string {
	if !m.ready {
		return "Loading LazyEndpoint…"
	}
	if m.tooSmall {
		msg := fmt.Sprintf("terminal too small — resize to at least %d×%d", minWidth, minHeight)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, msg)
	}
	if m.showHelp {
		return m.helpView()
	}
	if m.try != nil {
		return lipgloss.JoinVertical(lipgloss.Left, m.headerView(), m.tryView(), m.tryFooterView())
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.headerView(), m.mainView(), m.footerView())
}

func (m Model) mainView() string {
	return lipgloss.JoinHorizontal(lipgloss.Top, m.listView(), m.detailView())
}

func (m Model) headerView() string {
	line := " " + brandStyle.Render("LazyEndpoint")
	if meta := strings.TrimSpace(m.doc.Title + " " + m.doc.Version); meta != "" {
		line += "  " + dimStyle.Render(meta)
	}
	if server := m.doc.BaseURL(); server != "" {
		tail := " " + dimStyle.Render(server) + " "
		if gap := m.width - lipgloss.Width(line) - lipgloss.Width(tail); gap > 2 {
			line += strings.Repeat(" ", gap) + tail
		}
	}
	return fillWidth(line, m.width)
}

func (m Model) footerView() string {
	hints := "j/k move · / filter · tab pane · 1-4 tab · ? help · q quit"
	return fillWidth(" "+dimStyle.Render(hints), m.width)
}

func (m Model) listView() string {
	width := m.listWidth() - 2
	height := m.bodyHeight() - 2
	visible := height - 1

	lines := []string{m.listHeaderLine(width)}
	m.adjustOffset(visible)

	end := min(m.offset+visible, len(m.rows))
	for i := m.offset; i < end; i++ {
		lines = append(lines, m.rowView(m.rows[i], width, i == m.cursor))
	}
	if len(m.rows) == 0 {
		lines = append(lines, "", dimStyle.Render("  no matching endpoint"))
	}
	return renderBox(lines, width, height, m.pane == paneList)
}

func (m Model) listHeaderLine(width int) string {
	switch {
	case m.editing:
		return truncate(" "+m.input.View(), width)
	case m.query != "":
		return dimStyle.Render(truncate(" / "+m.query+"  (esc clears)", width))
	case m.tagFilter != "":
		return dimStyle.Render(truncate(fmt.Sprintf(" tag: %s · %d endpoints", m.tagFilter, m.visibleEndpoints()), width))
	default:
		return dimStyle.Render(truncate(fmt.Sprintf(" %d endpoints", m.visibleEndpoints()), width))
	}
}

func (m Model) rowView(r row, width int, selected bool) string {
	if r.kind == rowHeader {
		return dimStyle.Bold(true).Render(truncate(" ▾ "+strings.ToUpper(r.name), width))
	}

	ep := r.ep
	path := truncate(ep.Path, max(4, width-9))
	if selected {
		plain := " " + padRight(ep.Method, 6) + " " + path
		return lipgloss.NewStyle().
			Width(width).MaxWidth(width).
			Background(selBG).Foreground(selFG).Bold(true).
			Render(plain)
	}
	return lipgloss.NewStyle().Width(width).MaxWidth(width).
		Render(" " + methodStyle(ep.Method).Render(padRight(ep.Method, 6)) + " " + path)
}

func (m *Model) adjustOffset(visible int) {
	if visible < 1 {
		return
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visible {
		m.offset = m.cursor - visible + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m Model) detailView() string {
	width := m.detailWidth() - 2
	height := m.bodyHeight() - 2

	lines := []string{m.tabBar(width), ""}
	lines = append(lines, strings.Split(m.view.View(), "\n")...)
	return renderBox(lines, width, height, m.pane == paneDetail)
}

// renderBox draws a bordered pane whose content area is exactly
// innerWidth x innerHeight.
func renderBox(lines []string, innerWidth, innerHeight int, focused bool) string {
	if len(lines) > innerHeight {
		lines = lines[:innerHeight]
	}
	for len(lines) < innerHeight {
		lines = append(lines, "")
	}
	return paneStyle(focused).
		Width(innerWidth).Height(innerHeight).
		Render(strings.Join(lines, "\n"))
}

func (m Model) tabBar(width int) string {
	var b strings.Builder
	b.WriteString(" ")
	for i, name := range tabNames {
		label := fmt.Sprintf("%d %s", i+1, name)
		if tab(i) == m.tab {
			b.WriteString(activeTabStyle.Render(label))
		} else {
			b.WriteString(dimStyle.Render(label))
		}
		b.WriteString(" ")
	}
	return truncate(b.String(), width)
}

func (m Model) helpView() string {
	rows := [][2]string{
		{"j / k, ↑ / ↓", "move the cursor, scroll the details"},
		{"tab", "switch the focused pane"},
		{"1 - 4", "switch detail tab"},
		{"[ / ]", "previous / next detail tab"},
		{"/", "filter endpoints"},
		{"t", "open the request tester"},
		{"esc", "clear the filter, focus the list"},
		{"g / G", "top / bottom of the details"},
		{"?", "toggle this help"},
		{"q", "quit"},
	}

	width := 64
	if width > m.width-6 {
		width = m.width - 6
	}

	lines := []string{brandStyle.Render("LazyEndpoint") + dimStyle.Render("  keyboard shortcuts"), ""}
	for _, row := range rows {
		lines = append(lines, " "+keyStyle.Render(padRight(row[0], 14))+" "+dimStyle.Render(row[1]))
	}
	lines = append(lines, "", dimStyle.Render(" press any other key to close"))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(0, 1).
		MaxWidth(width).
		Render(strings.Join(lines, "\n"))

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
