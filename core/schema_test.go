package core

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestValidateStrictSchemaRejectsMissingAdditionalProps(t *testing.T) {
	s := json.RawMessage(`{"type":"object","properties":{"n":{"type":"string"}},"required":["n"]}`)
	if err := validateStrictSchema(s); !errors.Is(err, ErrInvalidSchema) {
		t.Fatalf("err = %v, want ErrInvalidSchema", err)
	}
}

func TestValidateStrictSchemaAcceptsWellFormed(t *testing.T) {
	s := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"n":{"type":"string"}},"required":["n"]}`)
	if err := validateStrictSchema(s); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestValidateStrictSchemaRejectsMissingRequiredEntry(t *testing.T) {
	s := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"n":{"type":"string"},"age":{"type":"integer"}},"required":["n"]}`)
	if err := validateStrictSchema(s); !errors.Is(err, ErrInvalidSchema) {
		t.Fatalf("err = %v, want ErrInvalidSchema", err)
	}
}

func TestValidateStrictSchemaRejectsNestedObjectMissingAdditionalProps(t *testing.T) {
	// The root object is well-formed, but the nested "address" object is not:
	// it declares "city" in properties without additionalProperties:false or
	// listing "city" in required.
	s := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"properties": {
			"name": {"type": "string"},
			"address": {
				"type": "object",
				"properties": {
					"city": {"type": "string"}
				},
				"required": ["city"]
			}
		},
		"required": ["name", "address"]
	}`)

	err := validateStrictSchema(s)
	if !errors.Is(err, ErrInvalidSchema) {
		t.Fatalf("err = %v, want ErrInvalidSchema", err)
	}
	if !strings.Contains(err.Error(), ".address") {
		t.Errorf("err = %v, want it to name the nested path .address", err)
	}
}

func TestValidateStrictSchemaAcceptsWellFormedNestedObject(t *testing.T) {
	s := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"properties": {
			"name": {"type": "string"},
			"address": {
				"type": "object",
				"additionalProperties": false,
				"properties": {
					"city": {"type": "string"},
					"zip": {"type": "string"}
				},
				"required": ["city", "zip"]
			}
		},
		"required": ["name", "address"]
	}`)

	if err := validateStrictSchema(s); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestValidateStrictSchemaRejectsAdditionalPropertiesTrue(t *testing.T) {
	s := json.RawMessage(`{"type":"object","additionalProperties":true,"properties":{"n":{"type":"string"}},"required":["n"]}`)
	if err := validateStrictSchema(s); !errors.Is(err, ErrInvalidSchema) {
		t.Fatalf("err = %v, want ErrInvalidSchema", err)
	}
}

func TestValidateStrictSchemaAcceptsEmptyObject(t *testing.T) {
	s := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{},"required":[]}`)
	if err := validateStrictSchema(s); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestValidateStrictSchemaAcceptsNonObjectRoot(t *testing.T) {
	s := json.RawMessage(`{"type":"string"}`)
	if err := validateStrictSchema(s); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestValidateStrictSchemaValidatesArrayItems(t *testing.T) {
	s := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"properties": {
			"tags": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"label": {"type": "string"}
					}
				}
			}
		},
		"required": ["tags"]
	}`)

	err := validateStrictSchema(s)
	if !errors.Is(err, ErrInvalidSchema) {
		t.Fatalf("err = %v, want ErrInvalidSchema", err)
	}
	if !strings.Contains(err.Error(), ".tags.items") {
		t.Errorf("err = %v, want it to name the path .tags.items", err)
	}
}

func TestValidateStrictSchemaValidatesDefs(t *testing.T) {
	s := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"properties": {
			"address": {"$ref": "#/$defs/address"}
		},
		"required": ["address"],
		"$defs": {
			"address": {
				"type": "object",
				"properties": {
					"city": {"type": "string"}
				},
				"required": ["city"]
			}
		}
	}`)

	err := validateStrictSchema(s)
	if !errors.Is(err, ErrInvalidSchema) {
		t.Fatalf("err = %v, want ErrInvalidSchema", err)
	}
	if !strings.Contains(err.Error(), "$defs.address") {
		t.Errorf("err = %v, want it to name the path in $defs.address", err)
	}
}

func TestValidateStrictSchemaSkipsRefNodes(t *testing.T) {
	// The $ref node itself has no additionalProperties/required of its own,
	// and must not be treated as a violation just because it points at an
	// object schema.
	s := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"properties": {
			"address": {"$ref": "#/$defs/address"}
		},
		"required": ["address"],
		"$defs": {
			"address": {
				"type": "object",
				"additionalProperties": false,
				"properties": {
					"city": {"type": "string"}
				},
				"required": ["city"]
			}
		}
	}`)

	if err := validateStrictSchema(s); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestValidateStrictSchemaEmptyRawMessage(t *testing.T) {
	if err := validateStrictSchema(nil); err != nil {
		t.Fatalf("unexpected err for nil schema: %v", err)
	}
	if err := validateStrictSchema(json.RawMessage{}); err != nil {
		t.Fatalf("unexpected err for empty schema: %v", err)
	}
}

func TestValidateStrictSchemaInvalidJSON(t *testing.T) {
	err := validateStrictSchema(json.RawMessage(`{not valid json`))
	if !errors.Is(err, ErrInvalidSchema) {
		t.Fatalf("err = %v, want ErrInvalidSchema", err)
	}
}
