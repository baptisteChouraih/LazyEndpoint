package spec

import (
	"net/url"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestRelativeServersResolveAgainstDocumentURL(t *testing.T) {
	paths := openapi3.NewPaths()
	paths.Set("/pet/findByStatus", &openapi3.PathItem{Get: &openapi3.Operation{}})

	doc := &openapi3.T{
		OpenAPI: "3.0.3",
		Info:    &openapi3.Info{Title: "Petstore", Version: "1.0.0"},
		Servers: openapi3.Servers{&openapi3.Server{URL: "/api/v3"}},
		Paths:   paths,
	}

	source := "https://petstore3.swagger.io/api/v3/openapi.json"
	base, err := url.Parse(source)
	if err != nil {
		t.Fatal(err)
	}

	got, err := fromDocument(doc, source, base)
	if err != nil {
		t.Fatalf("fromDocument: %v", err)
	}
	const want = "https://petstore3.swagger.io/api/v3"
	if got.BaseURL() != want {
		t.Fatalf("base URL = %q, want %q", got.BaseURL(), want)
	}
	servers := got.Groups[0].Endpoints[0].Servers
	if len(servers) != 1 || servers[0] != want {
		t.Fatalf("endpoint servers = %v, want [%s]", servers, want)
	}
}

func TestAbsoluteServersAreKept(t *testing.T) {
	doc := &openapi3.T{
		OpenAPI: "3.0.3",
		Info:    &openapi3.Info{Title: "T", Version: "1"},
		Servers: openapi3.Servers{&openapi3.Server{URL: "https://api.test/v1"}},
		Paths:   openapi3.NewPaths(),
	}

	got, err := fromDocument(doc, "https://example.com/spec.yaml", nil)
	if err != nil {
		t.Fatalf("fromDocument: %v", err)
	}
	if want := "https://api.test/v1"; got.BaseURL() != want {
		t.Fatalf("base URL = %q, want %q", got.BaseURL(), want)
	}
}
