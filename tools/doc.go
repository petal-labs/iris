// Package tools provides tool interfaces, registry, and argument parsing for AI tool calling.
//
// Tool is the executable tool contract: it embeds core.Tool (Name,
// Description, and Schema) and adds Call. Because it embeds core.Tool, any
// Tool implementation can be passed directly to ChatBuilder.Tools or stored
// in ChatRequest.Tools, and every provider transmits its schema:
//
//	type Tool interface {
//	    core.Tool
//	    Call(ctx context.Context, args json.RawMessage) (any, error)
//	}
//
// ToolSchema is an alias of core.ToolSchema; the schema contract is owned by
// core so providers can consume it without importing this package.
//
// The package also provides a tool Registry, middleware (logging, timeout,
// rate limiting, cache, validation, retry, circuit breaking, metrics), and
// the ParseArgs generic helper for decoding tool-call arguments.
package tools
