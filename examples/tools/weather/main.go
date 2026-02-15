// Example: Tool/Function Calling
//
// This example demonstrates how to define tools that the model
// can call to perform actions or retrieve information, and how to
// apply tool middleware (validation, timeout, logging).
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
	"log"
	"os"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/openai"
	"github.com/petal-labs/iris/tools"
)

// WeatherTool is a simple tool that "gets" weather for a location.
// In a real application, this would call a weather API.
type WeatherTool struct{}

func (w *WeatherTool) Name() string {
	return "get_weather"
}

func (w *WeatherTool) Description() string {
	return "Get the current weather in a given location"
}

func (w *WeatherTool) Schema() tools.ToolSchema {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"location": map[string]any{
				"type":        "string",
				"description": "The city and state, e.g., San Francisco, CA",
			},
			"unit": map[string]any{
				"type":        "string",
				"enum":        []string{"celsius", "fahrenheit"},
				"description": "The temperature unit to use",
			},
		},
		"required": []string{"location"},
	}
	schemaJSON, _ := json.Marshal(schema)
	return tools.ToolSchema{JSONSchema: schemaJSON}
}

func (w *WeatherTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	// Parse arguments
	var params struct {
		Location string `json:"location"`
		Unit     string `json:"unit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}

	// Simulate weather lookup (in reality, call a weather API)
	unit := params.Unit
	if unit == "" {
		unit = "fahrenheit"
	}

	temp := 72
	if unit == "celsius" {
		temp = 22
	}

	return map[string]any{
		"location":    params.Location,
		"temperature": temp,
		"unit":        unit,
		"conditions":  "sunny",
		"humidity":    45,
	}, nil
}

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "OPENAI_API_KEY environment variable not set")
		os.Exit(1)
	}

	provider := openai.New(apiKey)
	client := core.NewClient(provider, core.WithWarningHandler(func(msg string) {
		log.Printf("iris warning: %s", msg)
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create tool
	weatherTool := &WeatherTool{}
	logger := log.New(os.Stdout, "tool-middleware ", log.LstdFlags)
	wrappedWeatherTool := tools.ApplyMiddleware(
		weatherTool,
		tools.WithBasicValidation(),
		tools.WithTimeout(5*time.Second),
		tools.WithLogging(logger),
	)

	// Send request with tool
	fmt.Println("Asking about weather...")
	resp, err := client.Chat("gpt-4o-mini").
		User("What's the weather like in San Francisco and New York?").
		Tools(wrappedWeatherTool).
		GetResponse(ctx)

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	// Check if model wants to call tools
	if len(resp.ToolCalls) > 0 {
		fmt.Printf("Model requested %d tool call(s):\n", len(resp.ToolCalls))
		for i, call := range resp.ToolCalls {
			fmt.Printf("\n--- Tool Call %d ---\n", i+1)
			fmt.Printf("Tool: %s\n", call.Name)
			fmt.Printf("ID: %s\n", call.ID)
			fmt.Printf("Arguments: %s\n", string(call.Arguments))

			// Execute the tool
			result, err := wrappedWeatherTool.Call(ctx, call.Arguments)
			if err != nil {
				fmt.Printf("Error calling tool: %v\n", err)
				continue
			}

			resultJSON, _ := json.MarshalIndent(result, "", "  ")
			fmt.Printf("Result: %s\n", resultJSON)
		}
	} else {
		// Model responded directly without calling tools
		fmt.Println("Model response:", resp.Output)
	}
}
