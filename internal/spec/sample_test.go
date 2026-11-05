package spec

import (
	"encoding/json"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestSampleBodyFromRef(t *testing.T) {
	doc := loadPetstore(t)
	create := findEndpoint(t, doc, "POST", "/pets")
	if create.Body == nil || create.Body.Sample == "" {
		t.Fatal("missing request body sample")
	}

	var value map[string]any
	if err := json.Unmarshal([]byte(create.Body.Sample), &value); err != nil {
		t.Fatalf("sample is not valid JSON: %v\n%s", err, create.Body.Sample)
	}
	if _, ok := value["name"]; !ok {
		t.Fatalf("sample is missing the name field:\n%s", create.Body.Sample)
	}
	if value["status"] != "available" {
		t.Fatalf("sample did not use the first enum value: %v", value["status"])
	}
}

func TestSampleUsesDefaultsAndZeroValues(t *testing.T) {
	limit := openapi3.NewSchemaRef("", &openapi3.Schema{
		Type:    &openapi3.Types{"integer"},
		Format:  "int32",
		Default: 20,
	})
	if got := SampleJSON(limit); got != "20" {
		t.Fatalf("default sample = %q", got)
	}

	flag := openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"boolean"}})
	if got := SampleJSON(flag); got != "true" {
		t.Fatalf("boolean sample = %q", got)
	}

	empty := openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}})
	if got := SampleJSON(empty); got != `"string"` {
		t.Fatalf("string sample = %q", got)
	}
}
