// Package spec turns an OpenAPI document into the flat, display-oriented
// model that the TUI renders.
package spec

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// Untagged is the group used for operations that declare no tag.
const Untagged = "default"

// Document is an OpenAPI document prepared for display.
type Document struct {
	Title       string
	Version     string
	Description string
	Servers     []string
	Groups      []Group
	Source      string
}

// Group is a tag and the endpoints that belong to it.
type Group struct {
	Name      string
	Endpoints []*Endpoint
}

// Endpoint is a single operation of a single path.
type Endpoint struct {
	Method      string
	Path        string
	Summary     string
	Description string
	OperationID string
	Tags        []string
	Deprecated  bool
	Group       string
	Servers     []string
	Params      []Param
	Body        *Body
	Responses   []Response
}

// Param is a path/query/header/cookie parameter.
type Param struct {
	Name        string
	In          string
	Type        string
	Description string
	Required    bool
	Enum        string
	Default     string
}

// Body is an operation request body.
type Body struct {
	Required    bool
	ContentType string
	Description string
	Schema      []string
	Sample      string
}

// Response is one documented response of an operation.
type Response struct {
	Status      string
	Description string
	ContentType string
	Schema      []string
}

// Label renders the endpoint as "METHOD /path".
func (e *Endpoint) Label() string { return e.Method + " " + e.Path }

// Matches reports whether the endpoint matches a filter query.
func (e *Endpoint) Matches(query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return true
	}
	haystack := strings.ToLower(strings.Join([]string{
		e.Method, e.Path, e.Summary, e.OperationID, e.Group, strings.Join(e.Tags, " "),
	}, " "))
	for _, word := range strings.Fields(q) {
		if !strings.Contains(haystack, word) {
			return false
		}
	}
	return true
}

// EndpointCount returns the number of endpoints in the document.
func (d *Document) EndpointCount() int {
	n := 0
	for _, g := range d.Groups {
		n += len(g.Endpoints)
	}
	return n
}

// BaseURL is the first server of the document, if any.
func (d *Document) BaseURL() string {
	if len(d.Servers) == 0 {
		return ""
	}
	return d.Servers[0]
}

// Load reads an OpenAPI document from a file path or an http(s) URL.
func Load(location string) (*Document, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	var (
		doc  *openapi3.T
		base *url.URL
		err  error
	)
	if isRemote(location) {
		u, perr := url.Parse(location)
		if perr != nil {
			return nil, fmt.Errorf("invalid spec URL %q: %w", location, perr)
		}
		base = u
		doc, err = loader.LoadFromURI(u)
	} else {
		doc, err = loader.LoadFromFile(location)
	}
	if err != nil {
		return nil, fmt.Errorf("cannot load %s: %w", location, err)
	}
	return fromDocument(doc, location, base)
}

// FromDocument builds the display model from an already loaded document.
// Relative server URLs cannot be resolved without a base URL and are kept as
// written.
func FromDocument(doc *openapi3.T, source string) (*Document, error) {
	return fromDocument(doc, source, nil)
}

// fromDocument builds the display model. When base is set (the URL the document
// was fetched from) relative server URLs are resolved against it.
func fromDocument(doc *openapi3.T, source string, base *url.URL) (*Document, error) {
	if doc == nil {
		return nil, fmt.Errorf("empty specification")
	}

	d := &Document{Source: source, Servers: serverURLs(doc.Servers, base)}
	if doc.Info != nil {
		d.Title, d.Version, d.Description = doc.Info.Title, doc.Info.Version, doc.Info.Description
	}
	if d.Title == "" {
		d.Title = "Untitled API"
	}

	order := make([]string, 0, 8)
	byGroup := map[string][]*Endpoint{}

	if doc.Paths != nil {
		paths := doc.Paths.Keys()
		sort.Strings(paths)
		for _, path := range paths {
			item := doc.Paths.Value(path)
			if item == nil {
				continue
			}
			for _, op := range operationsOf(item) {
				ep := buildEndpoint(path, item, op.method, op.operation, doc, base)
				if _, ok := byGroup[ep.Group]; !ok {
					order = append(order, ep.Group)
				}
				byGroup[ep.Group] = append(byGroup[ep.Group], ep)
			}
		}
	}

	// Tags declared in the spec come first, in declaration order; the rest
	// follow in the order they appear, with untagged operations last.
	declared := map[string]bool{}
	for _, t := range doc.Tags {
		if t == nil || t.Name == "" || declared[t.Name] {
			continue
		}
		declared[t.Name] = true
		if eps := byGroup[t.Name]; len(eps) > 0 {
			d.Groups = append(d.Groups, Group{Name: t.Name, Endpoints: eps})
		}
	}
	for _, name := range order {
		if declared[name] || name == Untagged {
			continue
		}
		declared[name] = true
		d.Groups = append(d.Groups, Group{Name: name, Endpoints: byGroup[name]})
	}
	if eps := byGroup[Untagged]; len(eps) > 0 {
		d.Groups = append(d.Groups, Group{Name: Untagged, Endpoints: eps})
	}

	return d, nil
}

type operation struct {
	method    string
	operation *openapi3.Operation
}

// operationsOf returns the operations of a path item in a stable order.
func operationsOf(item *openapi3.PathItem) []operation {
	methods := []struct {
		name string
		op   *openapi3.Operation
	}{
		{"GET", item.Get},
		{"POST", item.Post},
		{"PUT", item.Put},
		{"PATCH", item.Patch},
		{"DELETE", item.Delete},
		{"HEAD", item.Head},
		{"OPTIONS", item.Options},
		{"TRACE", item.Trace},
	}
	out := make([]operation, 0, len(methods))
	for _, m := range methods {
		if m.op != nil {
			out = append(out, operation{method: m.name, operation: m.op})
		}
	}
	return out
}

func buildEndpoint(path string, item *openapi3.PathItem, method string, op *openapi3.Operation, doc *openapi3.T, base *url.URL) *Endpoint {
	ep := &Endpoint{
		Method:      method,
		Path:        path,
		Summary:     op.Summary,
		Description: op.Description,
		OperationID: op.OperationID,
		Tags:        append([]string(nil), op.Tags...),
		Deprecated:  op.Deprecated,
		Group:       Untagged,
	}
	if len(op.Tags) > 0 {
		ep.Group = op.Tags[0]
	}

	switch {
	case op.Servers != nil && len(*op.Servers) > 0:
		ep.Servers = serverURLs(*op.Servers, base)
	case len(item.Servers) > 0:
		ep.Servers = serverURLs(item.Servers, base)
	default:
		ep.Servers = serverURLs(doc.Servers, base)
	}

	ep.Params = paramsOf(item, op)
	ep.Body = bodyOf(op)
	ep.Responses = responsesOf(op)
	return ep
}

func paramsOf(item *openapi3.PathItem, op *openapi3.Operation) []Param {
	seen := map[string]bool{}
	var out []Param

	add := func(refs openapi3.Parameters) {
		for _, pref := range refs {
			if pref == nil || pref.Value == nil {
				continue
			}
			p := pref.Value
			key := p.In + ":" + p.Name
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, Param{
				Name:        p.Name,
				In:          p.In,
				Type:        TypeLabel(p.Schema),
				Description: p.Description,
				// Path parameters are always required, even when the spec
				// forgets to mark them.
				Required: p.Required || p.In == "path",
				Enum:     enumOf(p.Schema),
				Default:  defaultOf(p.Schema),
			})
		}
	}
	if item != nil {
		add(item.Parameters)
	}
	add(op.Parameters)
	return out
}

func bodyOf(op *openapi3.Operation) *Body {
	if op.RequestBody == nil || op.RequestBody.Value == nil {
		return nil
	}
	rb := op.RequestBody.Value
	ct, media := pickMedia(rb.Content)

	body := &Body{Required: rb.Required, ContentType: ct, Description: rb.Description}
	if media != nil && media.Schema != nil {
		body.Schema = SchemaBlock(media.Schema)
		body.Sample = SampleJSON(media.Schema)
	}
	if len(body.Schema) == 0 {
		body.Schema = []string{"(no schema)"}
	}
	return body
}

func responsesOf(op *openapi3.Operation) []Response {
	if op.Responses == nil {
		return nil
	}
	codes := make([]string, 0, len(op.Responses.Map()))
	for code := range op.Responses.Map() {
		codes = append(codes, code)
	}
	sortStatuses(codes)

	var out []Response
	for _, code := range codes {
		rr := op.Responses.Value(code)
		if rr == nil || rr.Value == nil {
			continue
		}
		resp := Response{Status: code, Description: deref(rr.Value.Description)}
		ct, media := pickMedia(rr.Value.Content)
		resp.ContentType = ct
		if media != nil && media.Schema != nil {
			resp.Schema = SchemaBlock(media.Schema)
		}
		out = append(out, resp)
	}
	return out
}

// pickMedia prefers JSON, otherwise the first content type alphabetically.
func pickMedia(content openapi3.Content) (string, *openapi3.MediaType) {
	if len(content) == 0 {
		return "", nil
	}
	for _, ct := range []string{"application/json", "application/*+json"} {
		if media, ok := content[ct]; ok {
			return ct, media
		}
	}
	types := make([]string, 0, len(content))
	for ct := range content {
		types = append(types, ct)
	}
	sort.Strings(types)
	return types[0], content[types[0]]
}

// sortStatuses orders responses numerically, with "default" last.
func sortStatuses(codes []string) {
	rank := func(code string) (int, int) {
		if code == "default" {
			return 1 << 30, 0
		}
		if n, err := strconv.Atoi(code); err == nil {
			return n, 0
		}
		return 1 << 29, 0
	}
	sort.SliceStable(codes, func(i, j int) bool {
		ri, _ := rank(codes[i])
		rj, _ := rank(codes[j])
		return ri < rj
	})
}

func serverURLs(servers openapi3.Servers, base *url.URL) []string {
	var out []string
	for _, s := range servers {
		if s == nil || s.URL == "" {
			continue
		}
		u := s.URL
		for name, v := range s.Variables {
			if v != nil {
				u = strings.ReplaceAll(u, "{"+name+"}", v.Default)
			}
		}
		// OpenAPI allows server URLs relative to the document location.
		if base != nil {
			if parsed, err := url.Parse(u); err == nil && !parsed.IsAbs() {
				u = base.ResolveReference(parsed).String()
			}
		}
		out = append(out, u)
	}
	return out
}

func isRemote(location string) bool {
	lower := strings.ToLower(location)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}
