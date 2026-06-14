package kios

import (
	tools "github.com/sipeed/picoclaw/pkg/tools"
)

// schemaCompatTool wraps a kios tool and widens its parameter schema so that
// numeric and boolean fields also accept string values.
//
// Why: Groq (and other strict OpenAI-compatible backends) validate the model's
// generated tool call against the JSON schema we advertise. Small open models
// such as meta-llama/llama-4-scout and llama-3.3-70b frequently emit numbers as
// strings (e.g. {"harga":"5000"}) or include optional numeric fields with
// string placeholders. Groq then rejects the call with HTTP 400
// "tool call validation failed: parameters for tool kios_stok did not match
// schema: errors: [/harga: expected integer ...]". This is server-side and
// happens regardless of tool_schema_transform, because that transform keeps
// simple scalar types like "integer" untouched.
//
// The kios argument parsers (argInt/argIntPtr/argBool) already tolerate string
// input, so relaxing the advertised type to a union {integer|string} is purely
// additive: it lets the provider accept loosely-typed model output while the Go
// side behaves exactly as before. This makes every kios tool robust against
// strict numeric/boolean validation without editing each tool's schema.
type schemaCompatTool struct {
	tools.Tool
	params map[string]any
}

func (t *schemaCompatTool) Parameters() map[string]any { return t.params }

// withSchemaCompat wraps a tool so its advertised parameter schema permits
// string values wherever a scalar number or boolean is expected.
func withSchemaCompat(t tools.Tool) tools.Tool {
	return &schemaCompatTool{Tool: t, params: widenSchemaTypes(t.Parameters())}
}

// widenSchemaTypes deep-clones a JSON schema and rewrites scalar "type"
// declarations (integer/number/boolean) to unions that also permit "string",
// recursing into "properties" and "items". Other keys are copied as-is.
func widenSchemaTypes(node map[string]any) map[string]any {
	if node == nil {
		return nil
	}
	out := make(map[string]any, len(node))
	for key, value := range node {
		switch key {
		case "type":
			out[key] = widenScalarType(value)
		case "properties":
			if props, ok := value.(map[string]any); ok {
				widened := make(map[string]any, len(props))
				for name, raw := range props {
					if sub, ok := raw.(map[string]any); ok {
						widened[name] = widenSchemaTypes(sub)
					} else {
						widened[name] = raw
					}
				}
				out[key] = widened
			} else {
				out[key] = value
			}
		case "items":
			if sub, ok := value.(map[string]any); ok {
				out[key] = widenSchemaTypes(sub)
			} else {
				out[key] = value
			}
		default:
			out[key] = value
		}
	}
	return out
}

// widenScalarType maps a single scalar JSON-schema type to a union that also
// accepts a string. Non-scalar or already-union types are returned unchanged.
func widenScalarType(raw any) any {
	name, ok := raw.(string)
	if !ok {
		return raw
	}
	switch name {
	case "integer":
		return []string{"integer", "string"}
	case "number":
		return []string{"number", "string"}
	case "boolean":
		return []string{"boolean", "string"}
	default:
		return raw
	}
}
