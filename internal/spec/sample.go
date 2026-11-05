package spec

import (
	"encoding/json"

	"github.com/getkin/kin-openapi/openapi3"
)

// SampleJSON renders a JSON example body for a schema, preferring explicit
// examples, then defaults, then the first enum value, then zero values.
func SampleJSON(ref *openapi3.SchemaRef) string {
	data, err := json.MarshalIndent(sampleValue(ref, 0, map[string]bool{}), "", "  ")
	if err != nil {
		return "{}"
	}
	return string(data)
}

func sampleValue(ref *openapi3.SchemaRef, depth int, seen map[string]bool) any {
	if ref == nil || ref.Value == nil || depth > 5 {
		return nil
	}
	if name := refName(ref.Ref); name != "" {
		if seen[name] {
			return nil
		}
		seen[name] = true
	}

	s := ref.Value
	switch {
	case s.Example != nil:
		return s.Example
	case s.Default != nil:
		return s.Default
	case len(s.Enum) > 0:
		return s.Enum[0]
	}

	if len(s.OneOf) > 0 {
		return sampleValue(s.OneOf[0], depth+1, seen)
	}
	if len(s.AnyOf) > 0 {
		return sampleValue(s.AnyOf[0], depth+1, seen)
	}
	if len(s.AllOf) > 0 && len(s.Properties) == 0 {
		merged := map[string]any{}
		for _, sub := range s.AllOf {
			if fields, ok := sampleValue(sub, depth+1, seen).(map[string]any); ok {
				for key, value := range fields {
					merged[key] = value
				}
			}
		}
		return merged
	}

	if len(s.Properties) > 0 {
		object := map[string]any{}
		for _, key := range sortedKeys(s.Properties) {
			object[key] = sampleValue(s.Properties[key], depth+1, seen)
		}
		return object
	}

	if isArray(s) {
		if s.Items == nil {
			return []any{}
		}
		return []any{sampleValue(s.Items, depth+1, seen)}
	}

	switch baseType(s) {
	case "integer", "number":
		return 0
	case "boolean":
		return true
	case "string":
		return sampleString(s.Format)
	}
	return nil
}

func baseType(s *openapi3.Schema) string {
	if s == nil || s.Type == nil {
		return ""
	}
	for _, candidate := range *s.Type {
		if candidate != "null" {
			return candidate
		}
	}
	return ""
}

func sampleString(format string) string {
	switch format {
	case "date-time":
		return "2024-01-01T00:00:00Z"
	case "date":
		return "2024-01-01"
	case "email":
		return "user@example.com"
	case "uuid":
		return "00000000-0000-0000-0000-000000000000"
	case "uri", "url":
		return "https://example.com"
	}
	return "string"
}
