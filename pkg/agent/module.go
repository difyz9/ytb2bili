package agent

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Module is the fx module for the eino-based agent.
var Module = fx.Module("agent",
	fx.Provide(ProvideAgent),
)

// AgentParams are the fx-injected inputs for ProvideAgent.
type AgentParams struct {
	fx.In
	Config *Config
	Logger *zap.Logger
	Tools  []tool.BaseTool `group:"tools"`
}

// ProvideAgent constructs a NanoAgent for dependency injection.
// Returns (nil, nil) when the LLM is not configured so the app can still start.
func ProvideAgent(params AgentParams) (*NanoAgent, error) {
	a, err := New(context.Background(), params.Config, params.Tools, params.Logger)
	if err != nil {
		params.Logger.Warn("agent not available",
			zap.Error(err))
		return nil, nil
	}
	return a, nil
}
