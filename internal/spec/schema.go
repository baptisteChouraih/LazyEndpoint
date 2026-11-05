package spec

import (
	"fmt"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

const maxSchemaDepth = 3

// TypeLabel renders a short, human readable type for a schema reference,
// e.g. "string", "integer(int64)", "array<Pet>".
func TypeLabel(ref *openapi3.SchemaRef) string {
	if ref == nil {
		return "any"
	}
	if name := refName(ref.Ref); name != "" {
		return name
	}
	return typeLabel(ref.Value)
}

func typeLabel(s *openapi3.Schema) string {
	if s == nil {
		return "any"
	}

	typ := ""
	if s.Type != nil {
		for _, candidate := range *s.Type {
			if candidate == "null" {
				continue
			}
			typ = candidate
			break
		}
	}

	switch typ {
	case "array":
		return "array<" + TypeLabel(s.Items) + ">"
	case "":
		switch {
		case len(s.Properties) > 0 || len(s.AllOf) > 0:
			return "object"
		case len(s.OneOf) > 0:
			return "oneOf"
		case len(s.AnyOf) > 0:
			return "anyOf"
		default:
			return "any"
		}
	}

	if s.Format != "" {
		return typ + "(" + s.Format + ")"
	}
	return typ
}

// SchemaBlock renders a schema as indented, human readable lines:
//
//	Pet
//	  id: integer(int64)  required
//	  name: string  required
//	  category: Category
//	    id: integer(int64)
func SchemaBlock(ref *openapi3.SchemaRef) []string {
	if ref == nil || ref.Value == nil {
		return []string{"(no schema)"}
	}
	if lines := describeSchema(ref, 0, map[string]bool{}, true); len(lines) > 0 {
		return lines
	}
	return []string{TypeLabel(ref)}
}

// describeSchema renders ref at the given indentation level. top marks the root
// of a block: a named schema prints its name and a scalar prints its type.
// Nested schemas are introduced by their parent's "name: type" line, so they
// only expand their fields.
func describeSchema(ref *openapi3.SchemaRef, depth int, seen map[string]bool, top bool) []string {
	if ref == nil || ref.Value == nil || depth > maxSchemaDepth {
		return nil
	}
	s := ref.Value
	indent := strings.Repeat("  ", depth)
	var out []string

	if name := refName(ref.Ref); name != "" {
		if seen[name] {
			if top {
				return []string{indent + name + "  (recursive)"}
			}
			return nil
		}
		seen[name] = true
		if top {
			out = append(out, indent+name)
			depth++
			indent = strings.Repeat("  ", depth)
		}
	}

	// allOf is flattened: render the fields of every member at this level.
	if len(s.AllOf) > 0 && len(s.Properties) == 0 {
		for _, sub := range s.AllOf {
			out = append(out, describeSchema(sub, depth, seen, false)...)
		}
		return out
	}

	if isArray(s) {
		if top {
			out = append(out, indent+TypeLabel(ref))
		}
		if s.Items != nil && hasFields(s.Items) {
			itemDepth := depth + 1
			if sub := describeSchema(s.Items, itemDepth, seen, false); len(sub) > 0 {
				out = append(out, indent+"items:")
				out = append(out, sub...)
			}
		}
		return out
	}

	if len(s.Properties) > 0 {
		required := map[string]bool{}
		for _, name := range s.Required {
			required[name] = true
		}
		for _, key := range sortedKeys(s.Properties) {
			prop := s.Properties[key]
			line := indent + key + ": " + TypeLabel(prop)
			if enum := enumOf(prop); enum != "" {
				line = indent + key + ": enum(" + enum + ")"
			}
			if required[key] {
				line += "  required"
			}
			out = append(out, line)

			if desc := oneLine(propDescription(prop)); desc != "" {
				out = append(out, indent+"    # "+desc)
			}
			out = append(out, describeSchema(prop, depth+1, seen, false)...)
		}
		return out
	}

	if top && len(out) == 0 {
		out = append(out, indent+TypeLabel(ref))
	}
	return out
}

// hasFields reports whether expanding ref produces any nested lines.
func hasFields(ref *openapi3.SchemaRef) bool {
	if ref == nil || ref.Value == nil {
		return false
	}
	s := ref.Value
	switch {
	case len(s.Properties) > 0, len(s.AllOf) > 0, isArray(s) && refName(ref.Ref) != "":
		return true
	default:
		return false
	}
}

func propDescription(ref *openapi3.SchemaRef) string {
	if ref == nil || ref.Value == nil {
		return ""
	}
	return ref.Value.Description
}

func isArray(s *openapi3.Schema) bool {
	if s == nil || s.Type == nil {
		return false
	}
	for _, t := range *s.Type {
		if t == "array" {
			return true
		}
	}
	return false
}

func enumOf(ref *openapi3.SchemaRef) string {
	if ref == nil || ref.Value == nil || len(ref.Value.Enum) == 0 {
		return ""
	}
	values := make([]string, 0, len(ref.Value.Enum))
	for _, v := range ref.Value.Enum {
		values = append(values, fmt.Sprint(v))
	}
	return strings.Join(values, " | ")
}

func defaultOf(ref *openapi3.SchemaRef) string {
	if ref == nil || ref.Value == nil || ref.Value.Default == nil {
		return ""
	}
	return fmt.Sprint(ref.Value.Default)
}

// refName turns "#/components/schemas/Pet" into "Pet".
func refName(ref string) string {
	if ref == "" {
		return ""
	}
	name := ref
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimPrefix(name, "#")
	name = strings.ReplaceAll(name, "~1", "/")
	name = strings.ReplaceAll(name, "~0", "~")
	return name
}

func sortedKeys(m openapi3.Schemas) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func oneLine(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}
