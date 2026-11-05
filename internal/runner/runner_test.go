package runner_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/baptisteChouraih/LazyEndpoint/internal/runner"
	"github.com/baptisteChouraih/LazyEndpoint/internal/spec"
)

func testEndpoint() *spec.Endpoint {
	return &spec.Endpoint{
		Method: "GET",
		Path:   "/pets/{petId}",
		Params: []spec.Param{
			{Name: "petId", In: "path", Required: true, Type: "integer(int64)"},
			{Name: "limit", In: "query", Type: "integer(int32)"},
			{Name: "X-Token", In: "header", Type: "string"},
			{Name: "session", In: "cookie", Type: "string"},
		},
	}
}

func TestBuildResolvesParameters(t *testing.T) {
	plan, err := runner.Build(testEndpoint(), "https://api.test/v1/", map[string]string{
		"petId":   "7",
		"limit":   "20",
		"X-Token": "abc",
		"session": "s1",
	}, "")
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if want := "https://api.test/v1/pets/7?limit=20"; plan.URL != want {
		t.Fatalf("URL = %q, want %q", plan.URL, want)
	}
	if plan.Headers["X-Token"] != "abc" {
		t.Fatalf("header = %q", plan.Headers["X-Token"])
	}
	if plan.Headers["Cookie"] != "session=s1" {
		t.Fatalf("cookie header = %q", plan.Headers["Cookie"])
	}
	if plan.Body != "" || plan.Headers["Content-Type"] != "" {
		t.Fatalf("unexpected body: %+v", plan)
	}
}

func TestBuildAddsBodyAndScheme(t *testing.T) {
	ep := testEndpoint()
	ep.Method = "POST"
	ep.Body = &spec.Body{ContentType: "application/json"}

	plan, err := runner.Build(ep, "localhost:8080", map[string]string{"petId": "1"}, `{"a":1}`)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if want := "http://localhost:8080/pets/1"; plan.URL != want {
		t.Fatalf("URL = %q, want %q", plan.URL, want)
	}
	if plan.Headers["Content-Type"] != "application/json" {
		t.Fatalf("content type = %q", plan.Headers["Content-Type"])
	}
	if plan.Body != `{"a":1}` {
		t.Fatalf("body = %q", plan.Body)
	}
}

func TestBuildRejectsIncompleteRequests(t *testing.T) {
	cases := []struct {
		name   string
		server string
		values map[string]string
	}{
		{"missing path parameter", "https://api.test", map[string]string{}},
		{"empty server", "", map[string]string{"petId": "1"}},
		{"bad scheme", "ftp://api.test", map[string]string{"petId": "1"}},
	}
	for _, tc := range cases {
		if _, err := runner.Build(testEndpoint(), tc.server, tc.values, ""); err == nil {
			t.Errorf("%s: expected an error", tc.name)
		}
	}
}

func TestExecuteReturnsFormattedResponse(t *testing.T) {
	var gotPath, gotQuery, gotToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotToken = r.Header.Get("X-Token")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"ok":true,"items":[1,2]}`)
	}))
	defer server.Close()

	plan, err := runner.Build(testEndpoint(), server.URL, map[string]string{
		"petId": "9", "limit": "5", "X-Token": "t",
	}, "")
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	result := runner.Execute(plan)
	if result.Err != nil {
		t.Fatalf("Execute: %v", result.Err)
	}
	if result.Status != http.StatusCreated || result.StatusText != "Created" {
		t.Fatalf("status = %d %q", result.Status, result.StatusText)
	}
	if !strings.Contains(result.Body, "\n  \"ok\": true") {
		t.Fatalf("body was not pretty printed:\n%s", result.Body)
	}
	if result.ContentType != "application/json" {
		t.Fatalf("content type = %q", result.ContentType)
	}
	if result.Size == 0 {
		t.Fatal("size not recorded")
	}
	if !hasHeader(result.Headers, "Content-Type: application/json") {
		t.Fatalf("headers = %v", result.Headers)
	}
	if gotPath != "/pets/9" || gotQuery != "limit=5" || gotToken != "t" {
		t.Fatalf("server saw path=%q query=%q token=%q", gotPath, gotQuery, gotToken)
	}
}

func TestExecuteReportsConnectionErrors(t *testing.T) {
	result := runner.Execute(runner.Plan{Method: "GET", URL: "http://127.0.0.1:1/"})
	if result.Err == nil {
		t.Fatal("expected a connection error")
	}
}

func hasHeader(headers []string, want string) bool {
	for _, header := range headers {
		if header == want {
			return true
		}
	}
	return false
}
