package tools

import (
	"encoding/json"

	"github.com/petal-labs/iris/core"
)

// ParseArgs parses tool call arguments into a typed struct.
// It unmarshals the JSON arguments from the ToolCall into the target type T.
//
// Example:
//
//	type WeatherArgs struct {
//	    Location string `json:"location"`
//	    Unit     string `json:"unit"`
//	}
//
//	args, err := tools.ParseArgs[WeatherArgs](toolCall)
//	if err != nil {
//	    return nil, err
//	}
//	// Use args.Location, args.Unit
func ParseArgs[T any](call core.ToolCall) (*T, error) {
	var result T
	if err := json.Unmarshal(call.Arguments, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
