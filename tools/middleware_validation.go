package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// SchemaValidator validates arguments against a JSON schema.
type SchemaValidator interface {
	Validate(schema json.RawMessage, data json.RawMessage) error
}

// WithValidation creates middleware that validates arguments against the tool's schema.
func WithValidation(validator SchemaValidator) Middleware {
	return func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			schema, ok := ToolSchemaFromContext(ctx)
			if !ok {
				return next(ctx, args)
			}

			if err := validator.Validate(schema, args); err != nil {
				return nil, fmt.Errorf("argument validation failed: %w", err)
			}

			return next(ctx, args)
		}
	}
}

// WithBasicValidation creates middleware that performs basic JSON validation.
func WithBasicValidation() Middleware {
	return func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			if !json.Valid(args) {
				return nil, errors.New("invalid JSON arguments")
			}
			return next(ctx, args)
		}
	}
}
