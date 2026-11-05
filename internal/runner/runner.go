// Package runner builds and executes the HTTP requests described by an
// endpoint, so the TUI can offer a "try it out" mode.
package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/baptisteChouraih/LazyEndpoint/internal/spec"
)

const (
	timeout      = 30 * time.Second
	maxBodyBytes = 1 << 20
)

// Plan is a fully resolved HTTP request.
type Plan struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    string
}

// Result is the outcome of a Plan.
type Result struct {
	Status      int
	StatusText  string
	Headers     []string
	ContentType string
	Body        string
	Size        int
	Truncated   bool
	Duration    time.Duration
	Err         error
}

// Build resolves server, path, query, header and cookie values into a request
// plan. values is keyed by parameter name; empty values are skipped, except
// for path parameters, which are required.
func Build(ep *spec.Endpoint, server string, values map[string]string, body string) (Plan, error) {
	plan := Plan{Method: ep.Method, Headers: map[string]string{}}

	path := ep.Path
	for _, p := range ep.Params {
		if p.In != "path" {
			continue
		}
		value := strings.TrimSpace(values[p.Name])
		if value == "" {
			return Plan{}, fmt.Errorf("missing path parameter %q", p.Name)
		}
		path = strings.ReplaceAll(path, "{"+p.Name+"}", url.PathEscape(value))
	}

	base := strings.TrimSpace(server)
	if base == "" {
		return Plan{}, fmt.Errorf("server URL is empty")
	}
	if !strings.Contains(base, "://") {
		base = "http://" + base
	}
	plan.URL = strings.TrimSuffix(base, "/") + path

	query := url.Values{}
	var cookies []string
	for _, p := range ep.Params {
		value := strings.TrimSpace(values[p.Name])
		if value == "" || p.In == "path" {
			continue
		}
		switch p.In {
		case "query":
			query.Set(p.Name, value)
		case "header":
			plan.Headers[p.Name] = value
		case "cookie":
			cookies = append(cookies, p.Name+"="+value)
		}
	}
	if len(query) > 0 {
		plan.URL += "?" + query.Encode()
	}
	if len(cookies) > 0 {
		plan.Headers["Cookie"] = strings.Join(cookies, "; ")
	}

	if strings.TrimSpace(body) != "" {
		plan.Body = body
		contentType := "application/json"
		if ep.Body != nil && ep.Body.ContentType != "" {
			contentType = ep.Body.ContentType
		}
		plan.Headers["Content-Type"] = contentType
	}

	parsed, err := url.Parse(plan.URL)
	if err != nil {
		return Plan{}, fmt.Errorf("invalid request URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return Plan{}, fmt.Errorf("unsupported URL scheme %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return Plan{}, fmt.Errorf("URL %q has no host", plan.URL)
	}
	return plan, nil
}

// Execute performs the request and reads at most maxBodyBytes of the response.
func Execute(plan Plan) Result {
	var body io.Reader
	if plan.Body != "" {
		body = strings.NewReader(plan.Body)
	}
	request, err := http.NewRequest(plan.Method, plan.URL, body)
	if err != nil {
		return Result{Err: err}
	}
	keys := make([]string, 0, len(plan.Headers))
	for key := range plan.Headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		request.Header.Set(key, plan.Headers[key])
	}
	request.Header.Set("User-Agent", "LazyEndpoint/0.1")

	client := &http.Client{Timeout: timeout}
	start := time.Now()
	response, err := client.Do(request)
	if err != nil {
		return Result{Duration: time.Since(start), Err: err}
	}
	defer response.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes+1))
	if err != nil {
		return Result{Duration: time.Since(start), Err: err}
	}

	result := Result{
		Status:      response.StatusCode,
		StatusText:  statusText(response),
		ContentType: response.Header.Get("Content-Type"),
		Duration:    time.Since(start),
		Size:        len(raw),
	}
	if len(raw) > maxBodyBytes {
		raw = raw[:maxBodyBytes]
		result.Truncated = true
		result.Size = len(raw)
	}
	result.Body = formatBody(raw)

	for _, key := range sortedHeaderKeys(response.Header) {
		for _, value := range response.Header[key] {
			result.Headers = append(result.Headers, key+": "+value)
		}
	}
	return result
}

func statusText(response *http.Response) string {
	text := strings.TrimSpace(strings.TrimPrefix(response.Status, fmt.Sprint(response.StatusCode)))
	if text == "" {
		return http.StatusText(response.StatusCode)
	}
	return text
}

// formatBody pretty prints JSON bodies and leaves everything else untouched.
func formatBody(raw []byte) string {
	if strings.TrimSpace(string(raw)) == "" {
		return "(empty body)"
	}
	if json.Valid(raw) {
		var buf bytes.Buffer
		if err := json.Indent(&buf, raw, "", "  "); err == nil {
			return buf.String()
		}
	}
	return string(raw)
}

func sortedHeaderKeys(header http.Header) []string {
	keys := make([]string, 0, len(header))
	for key := range header {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
