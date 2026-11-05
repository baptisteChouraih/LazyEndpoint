package tui

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/baptisteChouraih/LazyEndpoint/internal/spec"
)

// overviewLines renders the summary, metadata and an example request.
func overviewLines(ep *spec.Endpoint, width int) []string {
	var out []string

	head := ep.Method + " " + ep.Path
	if lipgloss.Width(head) > width {
		out = append(out, methodStyle(ep.Method).Render(ep.Method))
		out = append(out, titleStyle.Render(truncate(ep.Path, width)))
	} else {
		out = append(out, methodStyle(ep.Method).Render(ep.Method)+" "+titleStyle.Render(ep.Path))
	}
	out = append(out, "")

	if summary := strings.TrimSpace(ep.Summary); summary != "" {
		out = append(out, wrapText(summary, width)...)
		out = append(out, "")
	}
	if desc := strings.TrimSpace(ep.Description); desc != "" {
		for _, line := range wrapText(desc, width) {
			out = append(out, dimStyle.Render(line))
		}
		out = append(out, "")
	}

	servers := strings.Join(ep.Servers, ", ")
	meta := metaLines([][2]string{
		{"Tags", strings.Join(ep.Tags, ", ")},
		{"Group", ep.Group},
		{"Operation", ep.OperationID},
		{"Servers", servers},
		{"Deprecated", boolLabel(ep.Deprecated)},
	}, width)
	out = append(out, meta...)

	out = append(out, "")
	out = append(out, curlLines(ep, width)...)
	return out
}

func boolLabel(v bool) string {
	if v {
		return "yes"
	}
	return ""
}

// curlLines renders an example curl invocation for the endpoint.
func curlLines(ep *spec.Endpoint, width int) []string {
	base := ""
	if len(ep.Servers) > 0 {
		base = ep.Servers[0]
	}
	url := strings.TrimSuffix(base, "/") + ep.Path

	parts := []string{"curl -X " + ep.Method + " " + url}
	if ep.Body != nil && ep.Body.ContentType != "" {
		parts = append(parts,
			"  -H 'Content-Type: "+ep.Body.ContentType+"'",
			"  -d '<body>'",
		)
	}

	out := []string{dimStyle.Render("EXAMPLE REQUEST")}
	for i, part := range parts {
		suffix := ""
		if i < len(parts)-1 {
			suffix = " \\"
		}
		for _, line := range wrapText(part, width-1) {
			out = append(out, " "+line)
		}
		if suffix != "" && len(out) > 0 {
			out[len(out)-1] += suffix
		}
	}
	return out
}

// paramLines renders the parameters of an endpoint.
func paramLines(ep *spec.Endpoint, width int) []string {
	if len(ep.Params) == 0 {
		return []string{"", dimStyle.Render("No parameters.")}
	}

	var out []string
	for i, p := range ep.Params {
		if i > 0 {
			out = append(out, "")
		}

		head := titleStyle.Render(p.Name) + " " + dimStyle.Render("("+p.In+")")
		if p.Required {
			head += requiredStyle.Render(" required")
		}
		out = append(out, head)

		details := p.Type
		if p.Enum != "" {
			details += " · one of: " + p.Enum
		}
		if p.Default != "" {
			details += " · default: " + p.Default
		}
		if details != "" {
			out = append(out, "  "+dimStyle.Render(details))
		}
		if p.Description != "" {
			out = append(out, indented(p.Description, width, "  ")...)
		}
	}
	return out
}

// bodyLines renders the request body of an endpoint.
func bodyLines(ep *spec.Endpoint, width int) []string {
	if ep.Body == nil {
		return []string{"", dimStyle.Render("This operation has no request body.")}
	}

	head := "REQUEST BODY"
	if ep.Body.Required {
		head += " (required)"
	} else {
		head += " (optional)"
	}
	out := []string{dimStyle.Render(head)}

	if ep.Body.ContentType != "" {
		out = append(out, dimStyle.Render(ep.Body.ContentType))
	}
	if ep.Body.Description != "" {
		out = append(out, "")
		out = append(out, wrapText(ep.Body.Description, width)...)
	}
	out = append(out, "")
	for _, line := range ep.Body.Schema {
		out = append(out, truncate(line, width))
	}
	return out
}

// responseLines renders the documented responses of an endpoint.
func responseLines(ep *spec.Endpoint, width int) []string {
	if len(ep.Responses) == 0 {
		return []string{"", dimStyle.Render("No responses documented.")}
	}

	var out []string
	for i, r := range ep.Responses {
		if i > 0 {
			out = append(out, "")
		}

		head := statusStyle(r.Status).Render(r.Status)
		if text := statusText(r.Status); text != "" {
			head += " " + dimStyle.Render(text)
		}
		out = append(out, head)

		if r.Description != "" {
			out = append(out, indented(r.Description, width, "  ")...)
		}
		if r.ContentType != "" {
			out = append(out, "  "+dimStyle.Render(r.ContentType))
		}
		for _, line := range r.Schema {
			out = append(out, "  "+truncate(line, width-2))
		}
	}
	return out
}

func statusText(status string) string {
	if status == "default" {
		return "default"
	}
	code := 0
	if _, err := fmt.Sscanf(status, "%d", &code); err != nil {
		return ""
	}
	return http.StatusText(code)
}
