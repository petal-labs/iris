# Structured Output

Constrain model output to valid JSON or to a specific JSON Schema, with
validation performed before the request is sent.

Constrain model output to valid JSON or a specific JSON Schema:

```go
// JSON mode - model outputs valid JSON
resp, err := client.Chat("gpt-5.6").
    User("List 3 programming languages with their year created").
    ResponseJSON().
    GetResponse(ctx)

// Parse the JSON response
var languages []struct {
    Name string `json:"name"`
    Year int    `json:"year"`
}
json.Unmarshal([]byte(resp.Output), &languages)
```

For strict schema enforcement:

```go
schema := &core.JSONSchemaDefinition{
    Name:   "person",
    Strict: true,
    Schema: json.RawMessage(`{
        "type": "object",
        "additionalProperties": false,
        "properties": {
            "name": {"type": "string"},
            "age": {"type": "integer"}
        },
        "required": ["name", "age"]
    }`),
}

resp, err := client.Chat("gpt-5.6").
    User("Extract: John is 30 years old").
    ResponseJSONSchema(schema).
    GetResponse(ctx)

// Output is guaranteed to match the schema
```

`ResponseJSONSchema` is strict-by-default: it forces `schema.Strict = true` and validates the schema up front, so every object node in the schema must set `"additionalProperties": false` and list all of its properties in `"required"`. A schema that doesn't meet those constraints returns `core.ErrInvalidSchema` before any request is sent. Use `ResponseJSONSchemaNonStrict(schema)` to opt out and skip that validation. Requesting a schema against a provider or model that doesn't support structured output returns `core.ErrStructuredOutputUnsupported`, also before the call is made. Structured output is currently supported on OpenAI (both the Chat Completions and Responses API, GPT-5.x) and Google Gemini; other providers reject `ResponseJSONSchema` requests with that error. Plain `ResponseJSON()` (JSON mode) is not gated and works across providers that support `json_object`-style output.

---

See also: [Provider Comparison](../PROVIDERS.md#feature-support-matrix) · [Documentation index](../README.md)
