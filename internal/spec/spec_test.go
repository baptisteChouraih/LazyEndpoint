package spec

import (
	"strings"
	"testing"
)

func loadPetstore(t *testing.T) *Document {
	t.Helper()
	doc, err := Load("../../examples/petstore.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return doc
}

func TestLoadPetstore(t *testing.T) {
	doc := loadPetstore(t)

	if doc.Title != "Petstore" || doc.Version != "1.0.0" {
		t.Fatalf("unexpected info: %q %q", doc.Title, doc.Version)
	}
	if got, want := doc.EndpointCount(), 8; got != want {
		t.Fatalf("endpoint count = %d, want %d", got, want)
	}
	if got, want := len(doc.Servers), 1; got != want {
		t.Fatalf("servers = %v", doc.Servers)
	}

	// Declared tags come first, in declaration order, untagged last.
	var names []string
	for _, g := range doc.Groups {
		names = append(names, g.Name)
	}
	if got, want := strings.Join(names, ","), "pets,store,users,default"; got != want {
		t.Fatalf("groups = %q, want %q", got, want)
	}
}

func findEndpoint(t *testing.T, doc *Document, method, path string) *Endpoint {
	t.Helper()
	for _, g := range doc.Groups {
		for _, ep := range g.Endpoints {
			if ep.Method == method && ep.Path == path {
				return ep
			}
		}
	}
	t.Fatalf("endpoint %s %s not found", method, path)
	return nil
}

func TestPathParametersAreMergedAndRequired(t *testing.T) {
	doc := loadPetstore(t)
	ep := findEndpoint(t, doc, "GET", "/pets/{petId}")

	if len(ep.Params) != 1 {
		t.Fatalf("params = %+v", ep.Params)
	}
	p := ep.Params[0]
	if p.Name != "petId" || p.In != "path" || !p.Required {
		t.Fatalf("unexpected path param: %+v", p)
	}
	if p.Type != "integer(int64)" {
		t.Fatalf("type = %q", p.Type)
	}
	if ep.Group != "pets" {
		t.Fatalf("group = %q", ep.Group)
	}
}

func TestRequestAndResponseSchemas(t *testing.T) {
	doc := loadPetstore(t)

	create := findEndpoint(t, doc, "POST", "/pets")
	if create.Body == nil || !create.Body.Required {
		t.Fatalf("missing required body: %+v", create.Body)
	}
	if create.Body.ContentType != "application/json" {
		t.Fatalf("content type = %q", create.Body.ContentType)
	}
	body := strings.Join(create.Body.Schema, "\n")
	for _, want := range []string{"NewPet", "name: string  required", "category: Category"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body schema missing %q:\n%s", want, body)
		}
	}

	list := findEndpoint(t, doc, "GET", "/pets")
	if len(list.Responses) != 2 {
		t.Fatalf("responses = %+v", list.Responses)
	}
	if list.Responses[0].Status != "200" || list.Responses[1].Status != "400" {
		t.Fatalf("responses out of order: %+v", list.Responses)
	}
	if got := strings.Join(list.Responses[0].Schema, "\n"); !strings.Contains(got, "array<Pet>") {
		t.Fatalf("array schema not rendered:\n%s", got)
	}

	health := findEndpoint(t, doc, "GET", "/health")
	if health.Group != Untagged {
		t.Fatalf("untagged endpoint group = %q", health.Group)
	}
}

func TestMatchesFiltersByWords(t *testing.T) {
	doc := loadPetstore(t)
	ep := findEndpoint(t, doc, "GET", "/pets/{petId}")

	for _, query := range []string{"", "pets", "GET /pets", "getPetById", "pet"} {
		if !ep.Matches(query) {
			t.Fatalf("expected %q to match", query)
		}
	}
	for _, query := range []string{"store", "post"} {
		if ep.Matches(query) {
			t.Fatalf("expected %q not to match", query)
		}
	}
}
