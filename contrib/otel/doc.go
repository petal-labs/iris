// Package otel provides OpenTelemetry instrumentation for the Iris SDK.
//
// This package implements the core.ContextualTelemetryHook interface to create
// OpenTelemetry spans for LLM requests. Spans follow the OpenTelemetry Semantic
// Conventions for GenAI (gen_ai.*).
//
// # Usage
//
// Create an OTelHook and pass it to the Iris client:
//
//	import (
//	    "github.com/petal-labs/iris/core"
//	    "github.com/petal-labs/iris/providers/openai"
//	    irisotel "github.com/petal-labs/iris/contrib/otel"
//	)
//
//	// Create provider
//	provider, err := openai.NewFromEnv()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Create client with OTel hook
//	hook := irisotel.New()
//	client := core.NewClient(provider, core.WithTelemetry(hook))
//
//	// All LLM calls now emit OTel spans
//	resp, err := client.Chat("gpt-4o").User("Hello!").GetResponse(ctx)
//
// # Span Attributes
//
// Each span includes attributes following the OpenTelemetry Semantic Conventions
// for GenAI:
//
//   - gen_ai.system: Provider name (e.g., "openai", "anthropic")
//   - gen_ai.request.model: Model identifier
//   - gen_ai.usage.input_tokens: Number of input tokens
//   - gen_ai.usage.output_tokens: Number of output tokens
//
// # Custom Configuration
//
// The hook can be configured with options:
//
//	hook := irisotel.New(
//	    irisotel.WithTracerProvider(customProvider),
//	    irisotel.WithTracerName("my-app/llm"),
//	    irisotel.WithAttributes(
//	        attribute.String("service.name", "my-service"),
//	    ),
//	)
//
// # Security
//
// Following Iris's security design, spans never include sensitive data such as:
//   - API keys or credentials
//   - Prompt content (user messages)
//   - Response content (model outputs)
//
// Only operational metadata (provider, model, timing, token counts) is captured.
package otel
