package kios

import (
	"reflect"
	"testing"
)

func TestWidenSchemaTypes_NumericAndBooleanBecomeUnions(t *testing.T) {
	in := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action":      map[string]any{"type": "string", "enum": []string{"jual"}},
			"qty":         map[string]any{"type": "integer", "description": "jumlah"},
			"harga":       map[string]any{"type": "integer"},
			"auto_create": map[string]any{"type": "boolean"},
			"ratio":       map[string]any{"type": "number"},
			"items": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "object"},
			},
		},
		"required": []string{"action"},
	}

	out := widenSchemaTypes(in)
	props := out["properties"].(map[string]any)

	cases := map[string]any{
		"qty":         []string{"integer", "string"},
		"harga":       []string{"integer", "string"},
		"auto_create": []string{"boolean", "string"},
		"ratio":       []string{"number", "string"},
	}
	for field, want := range cases {
		got := props[field].(map[string]any)["type"]
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s type = %#v, want %#v", field, got, want)
		}
	}

	// String/enum and object types must stay untouched.
	if got := props["action"].(map[string]any)["type"]; got != "string" {
		t.Errorf("action type = %#v, want \"string\"", got)
	}
	if got := props["action"].(map[string]any)["enum"]; !reflect.DeepEqual(got, []string{"jual"}) {
		t.Errorf("action enum mangled: %#v", got)
	}
	if got := props["qty"].(map[string]any)["description"]; got != "jumlah" {
		t.Errorf("qty description lost: %#v", got)
	}

	// Original schema must be untouched (deep clone, not mutation).
	if got := in["properties"].(map[string]any)["harga"].(map[string]any)["type"]; got != "integer" {
		t.Errorf("input schema mutated: harga type = %#v", got)
	}
}

func TestWithSchemaCompat_PreservesIdentityAndWidens(t *testing.T) {
	tool := withSchemaCompat(&StokTool{})
	if tool.Name() != "kios_stok" {
		t.Fatalf("Name() = %q, want kios_stok", tool.Name())
	}
	props := tool.Parameters()["properties"].(map[string]any)
	if got := props["harga"].(map[string]any)["type"]; !reflect.DeepEqual(got, []string{"integer", "string"}) {
		t.Errorf("harga not widened: %#v", got)
	}
}
