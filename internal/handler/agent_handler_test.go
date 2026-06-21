package handler

import (
	"context"
	"testing"

	"github.com/zolagz/ytb2bili/internal/config"
	agentcfg "github.com/zolagz/ytb2bili/pkg/agent"
	"github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
)

func TestAllowedModelsIncludesConfiguredAgentModel(t *testing.T) {
	handler := &AgentHandler{
		cfg: &config.AppConfig{
			Agent: &agentcfg.Config{
				LLM: agentcfg.LLMConfig{
					Provider: "deepseek",
					Model:    "deepseek-v4-flash",
				},
			},
		},
		logger: zap.NewNop(),
	}

	allowed := handler.allowedModels(context.Background(), "", "", model.TierFree)
	if !containsAgentModel(allowed, "deepseek-v4-flash") {
		t.Fatalf("expected configured model to be included in allowed models, got %+v", allowed)
	}

	if !handler.canUseModel(context.Background(), "", "deepseek-v4-flash", "", model.TierFree) {
		t.Fatal("expected configured model to be usable even when it is not part of the static catalog")
	}
}

func containsAgentModel(options []AgentModelOption, modelID string) bool {
	for _, option := range options {
		if option.ID == modelID {
			return true
		}
	}
	return false
}