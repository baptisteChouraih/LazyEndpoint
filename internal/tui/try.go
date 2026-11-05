package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/baptisteChouraih/LazyEndpoint/internal/runner"
	"github.com/baptisteChouraih/LazyEndpoint/internal/spec"
)

const (
	tryFocusRequest = iota
	tryFocusResponse
)

type tryField struct {
	label  string
	name   string
	value  string
	server bool
	isBody bool
}

type tryState struct {
	ep     *spec.Endpoint
	fields []tryField
	body   string
	cursor int
	offset int

	editing     bool
	input       textinput.Model
	editingBody bool
	bodyEditor  textarea.Model

	focus   int
	result  *runner.Result
	plan    *runner.Plan
	sending bool

	response viewport.Model
}

// trySentMsg carries the outcome of a request back into the model.
type trySentMsg struct {
	plan   runner.Plan
	result runner.Result
}

// newTryState builds the editable request form for an endpoint.
func newTryState(ep *spec.Endpoint) *tryState {
	input := textinput.New()
	input.Prompt = ""
	input.CharLimit = 1024

	editor := textarea.New()
	editor.Placeholder = `{"key": "value"}`
	editor.ShowLineNumbers = false

	server := ""
	if len(ep.Servers) > 0 {
		server = ep.Servers[0]
	}

	t := &tryState{
		ep:         ep,
		input:      input,
		bodyEditor: editor,
		response:   viewport.New(0, 0),
	}

	t.fields = append(t.fields, tryField{label: "server", value: server, server: true})
	for _, p := range ep.Params {
		t.fields = append(t.fields, tryField{
			label: p.Name + " (" + p.In + ")",
			name:  p.Name,
			value: prefillParam(p),
		})
	}
	if ep.Body != nil {
		t.body = ep.Body.Sample
		if strings.TrimSpace(t.body) == "" {
			t.body = "{}"
		}
		t.fields = append(t.fields, tryField{label: "body", isBody: true})
	}
	return t
}

// prefillParam guesses a useful starting value for a parameter.
func prefillParam(p spec.Param) string {
	if p.Default != "" {
		return p.Default
	}
	if p.Enum != "" {
		return strings.TrimSpace(strings.Split(p.Enum, "|")[0])
	}
	if p.In != "path" {
		return ""
	}
	switch {
	case strings.HasPrefix(p.Type, "integer"), strings.HasPrefix(p.Type, "number"):
		return "1"
	case strings.HasPrefix(p.Type, "string"):
		return "sample"
	}
	return ""
}

func (t *tryState) server() string {
	if len(t.fields) == 0 {
		return ""
	}
	return t.fields[0].value
}

func (t *tryState) valueAt(index int) string {
	if index < 0 || index >= len(t.fields) {
		return ""
	}
	if t.fields[index].isBody {
		return t.body
	}
	return t.fields[index].value
}

func (t *tryState) setValue(index int, value string) {
	if index < 0 || index >= len(t.fields) {
		return
	}
	if t.fields[index].isBody {
		t.body = value
		return
	}
	t.fields[index].value = value
}

func (t *tryState) values() map[string]string {
	out := map[string]string{}
	for _, f := range t.fields {
		if f.server || f.isBody || f.name == "" {
			continue
		}
		out[f.name] = f.value
	}
	return out
}

func (t *tryState) moveCursor(delta int) {
	next := t.cursor + delta
	if next < 0 || next >= len(t.fields) {
		return
	}
	t.cursor = next
}

func (t *tryState) adjustOffset(visible int) {
	if visible < 1 {
		return
	}
	if t.cursor < t.offset {
		t.offset = t.cursor
	}
	if t.cursor >= t.offset+visible {
		t.offset = t.cursor - visible + 1
	}
	if t.offset < 0 {
		t.offset = 0
	}
}

// previewURL resolves the request URL for the URL line in the request pane.
func (t *tryState) previewURL() (string, string) {
	plan, err := runner.Build(t.ep, t.server(), t.values(), t.body)
	if err != nil {
		return "", err.Error()
	}
	return plan.URL, ""
}

// updateTry handles keys while the request tester is open.
func (m Model) updateTry(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	t := m.try

	if t.editingBody {
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			t.body = t.bodyEditor.Value()
			t.editingBody = false
			t.bodyEditor.Blur()
			m.layoutTry()
			return m, nil
		}
		var cmd tea.Cmd
		t.bodyEditor, cmd = t.bodyEditor.Update(msg)
		t.body = t.bodyEditor.Value()
		return m, cmd
	}

	if t.editing {
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			t.editing = false
			t.input.Blur()
			return m, nil
		case "enter":
			t.setValue(t.cursor, t.input.Value())
			t.editing = false
			t.input.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		t.input, cmd = t.input.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "esc", "t":
		if msg.String() == "esc" && t.focus == tryFocusResponse {
			t.focus = tryFocusRequest
			return m, nil
		}
		m.try = nil
		return m, nil

	case "tab":
		if t.focus == tryFocusRequest {
			t.focus = tryFocusResponse
		} else {
			t.focus = tryFocusRequest
		}

	case "j", "down":
		if t.focus == tryFocusRequest {
			t.moveCursor(1)
		} else {
			t.response.LineDown(1)
		}

	case "k", "up":
		if t.focus == tryFocusRequest {
			t.moveCursor(-1)
		} else {
			t.response.LineUp(1)
		}

	case "enter":
		if t.focus == tryFocusRequest {
			return m, m.startEditTry()
		}

	case "s":
		return m, m.sendTry()

	case "g", "home":
		t.response.GotoTop()
	case "G", "end":
		t.response.GotoBottom()
	case "ctrl+d":
		t.response.HalfPageDown()
	case "ctrl+u":
		t.response.HalfPageUp()
	}
	return m, nil
}

func (m *Model) startEditTry() tea.Cmd {
	t := m.try
	if t.cursor < 0 || t.cursor >= len(t.fields) {
		return nil
	}
	if t.fields[t.cursor].isBody {
		t.editingBody = true
		t.bodyEditor.SetValue(t.body)
		t.bodyEditor.Focus()
		m.layoutTry()
		return textarea.Blink
	}
	t.editing = true
	t.input.SetValue(t.fields[t.cursor].value)
	t.input.CursorEnd()
	t.input.Focus()
	return textinput.Blink
}

// sendTry builds the request and returns a command that performs it.
func (m Model) sendTry() tea.Cmd {
	t := m.try
	if t == nil || t.sending {
		return nil
	}

	plan, err := runner.Build(t.ep, t.server(), t.values(), t.body)
	if err != nil {
		failure := runner.Result{Err: err}
		t.result = &failure
		m.syncTryResponse()
		return nil
	}

	t.sending = true
	t.result = nil
	t.plan = &plan
	m.syncTryResponse()
	return func() tea.Msg {
		return trySentMsg{plan: plan, result: runner.Execute(plan)}
	}
}

func (m *Model) applyTryResult(msg trySentMsg) {
	if m.try == nil {
		return
	}
	t := m.try
	result, plan := msg.result, msg.plan
	t.sending = false
	t.result = &result
	t.plan = &plan
	t.focus = tryFocusResponse
	m.syncTryResponse()
	t.response.GotoTop()
}

// layoutTry recomputes the tester panes for the current terminal size.
func (m *Model) layoutTry() {
	if m.try == nil || !m.ready || m.tooSmall {
		return
	}
	t := m.try
	width := m.width - 2
	requestHeight := m.tryRequestHeight()

	requestInner := requestHeight - 2
	responseInner := m.bodyHeight() - requestHeight - 2

	t.input.Width = max(8, width-19)
	t.bodyEditor.SetWidth(max(10, width-1))
	t.bodyEditor.SetHeight(max(3, requestInner))
	t.response.Width = max(10, width)
	t.response.Height = max(1, responseInner-1)
	m.syncTryResponse()
}

func (m *Model) syncTryResponse() {
	t := m.try
	if t == nil {
		return
	}
	t.response.SetContent(strings.Join(t.responseLines(t.response.Width), "\n"))
}

// tryRequestHeight is the total height of the request pane, borders included.
func (m Model) tryRequestHeight() int {
	t := m.try
	if t == nil {
		return 4
	}
	body := m.bodyHeight()

	if t.editingBody {
		height := body * 2 / 3
		if height > body-3 {
			height = body - 3
		}
		if height < 8 {
			height = min(8, body-3)
		}
		return max(4, height)
	}

	needed := len(t.fields) + 4
	if needed < 7 {
		needed = 7
	}
	if limit := body - 4; needed > limit {
		needed = limit
	}
	return max(4, needed)
}

func (m Model) tryView() string {
	t := m.try
	width := m.width - 2
	requestHeight := m.tryRequestHeight()
	responseHeight := m.bodyHeight() - requestHeight

	var requestBox string
	if t.editingBody {
		lines := strings.Split(t.bodyEditor.View(), "\n")
		for i, line := range lines {
			lines[i] = truncate(line, width)
		}
		requestBox = renderBox(lines, width, requestHeight-2, t.focus == tryFocusRequest)
	} else {
		requestBox = renderBox(m.tryFieldLines(width, requestHeight-2), width, requestHeight-2, t.focus == tryFocusRequest)
	}

	return lipgloss.JoinVertical(lipgloss.Left, requestBox, m.tryResponseBox(heightAtLeast(responseHeight)))
}

func heightAtLeast(h int) int {
	if h < 4 {
		return 4
	}
	return h
}

func (m Model) tryFieldLines(width, innerHeight int) []string {
	t := m.try
	visible := max(1, innerHeight-2)
	t.adjustOffset(visible)

	var out []string
	end := min(t.offset+visible, len(t.fields))
	for i := t.offset; i < end; i++ {
		out = append(out, m.tryRow(i, width))
	}
	out = append(out, "")
	out = append(out, m.tryURLLine(width))
	return out
}

func (m Model) tryRow(index, width int) string {
	t := m.try
	field := t.fields[index]
	selected := index == t.cursor
	const labelWidth = 16

	raw := t.valueAt(index)
	plain := raw
	lineCount := 0
	if field.isBody {
		plain = firstLine(raw)
		lineCount = strings.Count(raw, "\n") + 1
	}
	empty := strings.TrimSpace(raw) == ""
	if empty {
		plain = "(empty)"
	} else if lineCount > 1 {
		plain += fmt.Sprintf(" … (%d lines)", lineCount)
	}
	plain = truncate(plain, max(4, width-labelWidth-3))
	label := padRight(truncate(field.label, labelWidth), labelWidth)

	if selected && t.editing {
		return "▸ " + titleStyle.Render(label) + t.input.View()
	}
	if selected {
		return lipgloss.NewStyle().Width(width).
			Background(selBG).Foreground(selFG).Bold(true).
			Render(truncate("▸ "+label+plain, width))
	}

	line := " " + dimStyle.Render(label)
	if empty || field.isBody {
		return line + " " + dimStyle.Render(plain)
	}
	return line + " " + plain
}

func (m Model) tryURLLine(width int) string {
	url, errText := m.try.previewURL()
	if errText != "" {
		return requiredStyle.Render(truncate(" ✗ "+errText, width))
	}
	return dimStyle.Render(truncate(" → "+m.try.ep.Method+" "+url, width))
}

func (m Model) tryResponseBox(height int) string {
	t := m.try
	width := m.width - 2
	inner := height - 2
	if inner < 1 {
		return renderBox(nil, width, max(1, inner), t.focus == tryFocusResponse)
	}

	t.response.Width = max(10, width)
	t.response.Height = max(1, inner-1)

	lines := []string{m.tryStatusLine(width)}
	lines = append(lines, strings.Split(t.response.View(), "\n")...)
	return renderBox(lines, width, inner, t.focus == tryFocusResponse)
}

func (m Model) tryStatusLine(width int) string {
	t := m.try
	switch {
	case t.sending:
		return " " + dimStyle.Render("sending…")
	case t.result == nil:
		return " " + dimStyle.Render("no response yet · press s to send")
	}

	r := t.result
	if r.Err != nil {
		return " " + requiredStyle.Render(truncate("request failed: "+r.Err.Error(), width-2))
	}

	head := statusStyle(fmt.Sprint(r.Status)).
		Render(fmt.Sprintf("%d %s", r.Status, r.StatusText))
	meta := formatDuration(r.Duration) + " · " + formatSize(r.Size)
	if r.Truncated {
		meta += " · truncated"
	}
	if r.ContentType != "" {
		meta += " · " + r.ContentType
	}

	room := width - lipgloss.Width(head) - 3
	if room < 8 {
		return " " + head
	}
	return " " + head + "  " + dimStyle.Render(truncate(meta, room))
}

func (t *tryState) responseLines(width int) []string {
	if t.sending {
		return []string{dimStyle.Render(" waiting for the server…")}
	}
	if t.result == nil {
		return []string{
			dimStyle.Render(" Values are prefilled from the spec."),
			dimStyle.Render(" Enter edits a field, s sends the request."),
		}
	}

	r := t.result
	if r.Err != nil {
		return wrapText("request failed: "+r.Err.Error(), width)
	}

	var out []string
	if len(r.Headers) > 0 {
		for _, header := range r.Headers {
			out = append(out, dimStyle.Render(" "+truncate(header, width-1)))
		}
		out = append(out, "")
	}
	for _, line := range strings.Split(r.Body, "\n") {
		out = append(out, " "+truncate(line, width-1))
	}
	return out
}

func (m Model) tryFooterView() string {
	t := m.try
	var hints string
	switch {
	case t.editingBody:
		hints = "esc done editing the body · it is sent as the request body"
	case t.editing:
		hints = "enter confirm · esc cancel"
	case t.sending:
		hints = "sending…"
	default:
		hints = "j/k field · enter edit · s send · tab pane · t docs · esc back"
	}
	return fillWidth(" "+dimStyle.Render(hints), m.width)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%d µs", d.Microseconds())
	}
	return fmt.Sprintf("%d ms", d.Milliseconds())
}

func formatSize(size int) string {
	switch {
	case size >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(size)/(1<<20))
	case size >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(size)/(1<<10))
	default:
		return fmt.Sprintf("%d B", size)
	}
}
