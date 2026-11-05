package tui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/baptisteChouraih/LazyEndpoint/internal/spec"
)

func tryDoc(server string) *spec.Document {
	return &spec.Document{
		Title:   "Try Test",
		Version: "1.0.0",
		Servers: []string{server},
		Groups: []spec.Group{{
			Name: "things",
			Endpoints: []*spec.Endpoint{{
				Method:  "GET",
				Path:    "/things/{id}",
				Group:   "things",
				Summary: "Get a thing",
				Servers: []string{server},
				Params: []spec.Param{
					{Name: "id", In: "path", Required: true, Type: "integer(int64)"},
					{Name: "verbose", In: "query", Type: "boolean"},
				},
			}},
		}},
	}
}

func newTryModel(t *testing.T, doc *spec.Document) Model {
	t.Helper()
	m := New(doc, Options{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return updated.(Model)
}

func assertFrame(t *testing.T, m Model) {
	t.Helper()
	lines := strings.Split(m.View(), "\n")
	if len(lines) != m.height {
		t.Fatalf("view has %d lines, want %d", len(lines), m.height)
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got != m.width {
			t.Fatalf("line %d is %d cells, want %d: %q", i, got, m.width, line)
		}
	}
}

func TestTryViewShowsEditableFields(t *testing.T) {
	m := newTryModel(t, tryDoc("https://api.test"))
	m = press(t, m, "t")
	if m.try == nil {
		t.Fatal("t did not open the tester")
	}
	if len(m.try.fields) != 3 {
		t.Fatalf("fields = %+v", m.try.fields)
	}

	view := m.View()
	for _, want := range []string{"server", "id (path)", "verbose (query)"} {
		if !strings.Contains(view, want) {
			t.Fatalf("request pane missing %q:\n%s", want, view)
		}
	}
	if !strings.Contains(view, "https://api.test/things/1") {
		t.Fatalf("resolved URL missing:\n%s", view)
	}
	assertFrame(t, m)

	m = press(t, m, "j", "enter")
	if !m.try.editing {
		t.Fatal("enter did not start editing the selected field")
	}
	m = press(t, m, "4", "2", "enter")
	if got := m.try.fields[1].value; !strings.HasSuffix(got, "42") {
		t.Fatalf("edited value = %q", got)
	}
	if !strings.Contains(m.View(), "/things/142") {
		t.Fatalf("URL did not follow the edited value:\n%s", m.View())
	}

	m = press(t, m, "esc")
	if m.try != nil {
		t.Fatal("esc did not close the tester")
	}
	assertFrame(t, m)
}

func TestTrySendsRequestAndShowsResponse(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":7,"name":"Rex"}`)
	}))
	defer server.Close()

	m := newTryModel(t, tryDoc(server.URL))
	m = press(t, m, "t")

	updated, cmd := m.Update(key("s"))
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("s did not start a request")
	}
	if !m.try.sending {
		t.Fatal("expected the request to be in flight")
	}

	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if m.try.result == nil || m.try.result.Err != nil {
		t.Fatalf("result = %+v", m.try.result)
	}
	if m.try.result.Status != http.StatusCreated {
		t.Fatalf("status = %d", m.try.result.Status)
	}

	view := m.View()
	for _, want := range []string{"201 Created", "Rex", "application/json"} {
		if !strings.Contains(view, want) {
			t.Fatalf("response pane missing %q:\n%s", want, view)
		}
	}
	if gotQuery != "" {
		t.Fatalf("unexpected query string %q", gotQuery)
	}
	assertFrame(t, m)
}

func TestTryReportsRequestErrors(t *testing.T) {
	m := newTryModel(t, tryDoc("http://127.0.0.1:1"))
	m = press(t, m, "t")

	updated, cmd := m.Update(key("s"))
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected a request command")
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)

	if m.try.result == nil || m.try.result.Err == nil {
		t.Fatal("expected an error result")
	}
	if !strings.Contains(m.View(), "request failed") {
		t.Fatalf("error missing from the response pane:\n%s", m.View())
	}
	assertFrame(t, m)
}

func TestTryEditsAndSendsBody(t *testing.T) {
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	doc := tryDoc(server.URL)
	ep := doc.Groups[0].Endpoints[0]
	ep.Method = "POST"
	ep.Body = &spec.Body{ContentType: "application/json", Sample: `{"name":"string"}`}

	m := newTryModel(t, doc)
	m = press(t, m, "t")
	for i := 0; i < len(m.try.fields)-1; i++ {
		m = press(t, m, "j")
	}
	if !m.try.fields[m.try.cursor].isBody {
		t.Fatalf("cursor is not on the body row: %+v", m.try.fields[m.try.cursor])
	}

	m = press(t, m, "enter")
	if !m.try.editingBody {
		t.Fatal("enter did not open the body editor")
	}
	m = press(t, m, "esc")
	if m.try.editingBody {
		t.Fatal("esc did not close the body editor")
	}

	updated, cmd := m.Update(key("s"))
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected a request command")
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)

	if !strings.Contains(gotBody, `"name"`) {
		t.Fatalf("body was not sent: %q", gotBody)
	}
	assertFrame(t, m)
}
