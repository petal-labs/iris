package core

import (
	"encoding/json"
	"fmt"
)

// validateStrictSchema checks that raw is a JSON Schema compatible with
// provider "strict" structured-output modes (e.g. OpenAI's strict function
// calling / response format). Strict mode requires that every object node in
// the schema:
//
//   - sets "additionalProperties": false, and
//   - lists every key present in "properties" inside the "required" array.
//
// Providers reject non-compliant schemas at request time with an opaque
// error; validating locally produces a precise, actionable error instead.
//
// The schema is walked recursively through "properties", "items" (both the
// single-schema and tuple-array forms), "$defs", and the legacy
// "definitions" keyword. Nodes containing "$ref" are not recursed into: a
// reference cannot be validated without resolving it, and if the referenced
// definition is declared locally under $defs/definitions it is validated
// independently as part of that walk.
func validateStrictSchema(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}

	var node any
	if err := json.Unmarshal(raw, &node); err != nil {
		return fmt.Errorf("%w: schema is not valid JSON: %v", ErrInvalidSchema, err)
	}

	return validateStrictNode(node, "")
}

// validateStrictNode recursively validates a single decoded JSON Schema node.
// path identifies the node's location using a JSON-path-like notation for
// error messages, e.g. ".address.city"; the root node uses "".
func validateStrictNode(node any, path string) error {
	obj, ok := node.(map[string]any)
	if !ok {
		// Non-object-shaped nodes (booleans, string enum values, etc.) carry
		// no strict-mode constraints of their own.
		return nil
	}

	// A node containing "$ref" is a pointer to another schema; it cannot be
	// validated inline. If the target is declared locally under $defs or
	// definitions, it is validated separately when we walk those maps.
	if _, hasRef := obj["$ref"]; hasRef {
		return nil
	}

	if isObjectType(obj["type"]) {
		if err := validateObjectConstraints(obj, path); err != nil {
			return err
		}
	}

	if props, ok := obj["properties"].(map[string]any); ok {
		for key, val := range props {
			if err := validateStrictNode(val, path+"."+key); err != nil {
				return err
			}
		}
	}

	if items, ok := obj["items"]; ok {
		switch v := items.(type) {
		case map[string]any:
			if err := validateStrictNode(v, path+".items"); err != nil {
				return err
			}
		case []any:
			for i, item := range v {
				if err := validateStrictNode(item, fmt.Sprintf("%s.items[%d]", path, i)); err != nil {
					return err
				}
			}
		}
	}

	for _, defsKey := range [...]string{"$defs", "definitions"} {
		if defs, ok := obj[defsKey].(map[string]any); ok {
			for key, val := range defs {
				if err := validateStrictNode(val, fmt.Sprintf("%s.%s.%s", path, defsKey, key)); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// validateObjectConstraints enforces the two strict-mode rules that apply to
// a single object-typed schema node: additionalProperties must be exactly
// false, and every declared property must appear in required.
func validateObjectConstraints(obj map[string]any, path string) error {
	addl, hasAddl := obj["additionalProperties"]
	if !hasAddl {
		return fmt.Errorf("%w: %s: object schema missing \"additionalProperties\": false", ErrInvalidSchema, pathOrRoot(path))
	}
	if b, isBool := addl.(bool); !isBool || b != false {
		return fmt.Errorf("%w: %s: \"additionalProperties\" must be false, got %#v", ErrInvalidSchema, pathOrRoot(path), addl)
	}

	props, _ := obj["properties"].(map[string]any)

	required := make(map[string]bool, len(props))
	if reqRaw, ok := obj["required"].([]any); ok {
		for _, r := range reqRaw {
			if s, ok := r.(string); ok {
				required[s] = true
			}
		}
	}

	for key := range props {
		if !required[key] {
			return fmt.Errorf("%w: %s: property %q is not listed in \"required\"", ErrInvalidSchema, pathOrRoot(path), key)
		}
	}

	return nil
}

// isObjectType reports whether a schema's "type" value designates an object.
// It handles both the common string form ("type": "object") and the union
// form ("type": ["object", "null"]).
func isObjectType(t any) bool {
	switch v := t.(type) {
	case string:
		return v == "object"
	case []any:
		for _, e := range v {
			if s, ok := e.(string); ok && s == "object" {
				return true
			}
		}
	}
	return false
}

// pathOrRoot renders the root path as "root" for readability in error
// messages, since an empty string is easy to misread as missing information.
func pathOrRoot(path string) string {
	if path == "" {
		return "root"
	}
	return path
}
