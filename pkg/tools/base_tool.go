package tools

import (
	"fmt"

	"github.com/bytedance/sonic"
	"go.uber.org/zap"
)

// UnmarshalArgs decodes JSON args with tool-name-prefixed errors for consistent InvokableRun usage.
func UnmarshalArgs[T any](toolName, argsJSON string) (T, error) {
	var params T
	if err := sonic.UnmarshalString(argsJSON, &params); err != nil {
		return params, fmt.Errorf("[%s] failed to unmarshal args: %w", toolName, err)
	}
	return params, nil
}

// RequireString validates a required string parameter is non-empty.
func RequireString(toolName, paramName, value string) error {
	if value == "" {
		return fmt.Errorf("[%s] missing required parameter: %s", toolName, paramName)
	}
	return nil
}

// RequirePositiveInt validates an integer parameter is positive.
func RequirePositiveInt(toolName, paramName string, value int) error {
	if value <= 0 {
		return fmt.Errorf("[%s] parameter %s must be a positive integer, got: %d", toolName, paramName, value)
	}
	return nil
}

// ToolError creates a tool-name-prefixed error with fmt.Errorf semantics.
func ToolError(toolName string, msg string, args ...any) error {
	return fmt.Errorf("[%s] "+msg, append([]any{toolName}, args...)...)
}

// ToolErrorWrap wraps an existing error with a tool-name prefix.
func ToolErrorWrap(toolName string, msg string, err error) error {
	return fmt.Errorf("[%s] %s: %w", toolName, msg, err)
}

// ToolLogger returns a logger with the tool name field for structured logging.
func ToolLogger(logger *zap.Logger, toolName string) *zap.Logger {
	return logger.With(zap.String("tool", toolName))
}
