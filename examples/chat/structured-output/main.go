// Example: Structured Output
//
// This example demonstrates how to use structured output to constrain
// model responses to valid JSON or a specific JSON Schema.
//
// Run with:
//
//	export OPENAI_API_KEY=your-key
//	go run main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/openai"
)

// Language represents a programming language
type Language struct {
	Name        string `json:"name"`
	Year        int    `json:"year"`
	Creator     string `json:"creator"`
	Description string `json:"description"`
}

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "OPENAI_API_KEY environment variable not set")
		os.Exit(1)
	}

	provider := openai.New(apiKey)
	client := core.NewClient(provider)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Example 1: JSON mode - model outputs valid JSON
	fmt.Println("=== JSON Mode ===")
	resp, err := client.Chat("gpt-4o-mini").
		System("You are a helpful assistant that responds in JSON format.").
		User("List 3 programming languages with their name, year created, creator, and a brief description.").
		ResponseJSON().
		GetResponse(ctx)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Raw JSON response:")
	fmt.Println(resp.Output)

	// Parse the response
	var result struct {
		Languages []Language `json:"languages"`
	}
	if err := json.Unmarshal([]byte(resp.Output), &result); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse JSON: %v\n", err)
	} else {
		fmt.Println("\nParsed languages:")
		for _, lang := range result.Languages {
			fmt.Printf("  - %s (%d) by %s\n", lang.Name, lang.Year, lang.Creator)
		}
	}

	// Example 2: JSON Schema mode - strict schema enforcement
	fmt.Println("\n=== JSON Schema Mode ===")
	schema := &core.JSONSchemaDefinition{
		Name:        "person_info",
		Description: "Information about a person",
		Strict:      true,
		Schema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {
					"type": "string",
					"description": "The person's full name"
				},
				"age": {
					"type": "integer",
					"description": "The person's age in years"
				},
				"occupation": {
					"type": "string",
					"description": "The person's job or profession"
				},
				"hobbies": {
					"type": "array",
					"items": {"type": "string"},
					"description": "List of hobbies"
				}
			},
			"required": ["name", "age", "occupation", "hobbies"],
			"additionalProperties": false
		}`),
	}

	resp, err = client.Chat("gpt-4o-mini").
		User("Extract information: Sarah is a 28-year-old software engineer who enjoys hiking, reading, and playing chess.").
		ResponseJSONSchema(schema).
		GetResponse(ctx)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Schema-constrained response:")
	fmt.Println(resp.Output)

	// Parse the structured response
	var person struct {
		Name       string   `json:"name"`
		Age        int      `json:"age"`
		Occupation string   `json:"occupation"`
		Hobbies    []string `json:"hobbies"`
	}
	if err := json.Unmarshal([]byte(resp.Output), &person); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse: %v\n", err)
	} else {
		fmt.Printf("\nExtracted: %s, age %d, works as %s\n", person.Name, person.Age, person.Occupation)
		fmt.Printf("Hobbies: %v\n", person.Hobbies)
	}

	fmt.Printf("\nTokens used: %d\n", resp.Usage.TotalTokens)
}
