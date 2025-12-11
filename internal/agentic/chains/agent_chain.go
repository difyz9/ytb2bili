package chains

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/difyz9/ytb2bili/internal/agentic"
	"go.uber.org/zap"
)

// AgentChain 基于 LLM 的智能任务链
// 由 AI 自主决定工具调用顺序和参数，适合复杂、动态的任务
type AgentChain struct {
	*BaseChain
	maxIterations int
}

// NewAgentChain 创建智能代理链
func NewAgentChain(agent *agentic.Agent, logger *zap.Logger, maxIterations int) *AgentChain {
	if maxIterations <= 0 {
		maxIterations = 10 // 默认最大迭代次数
	}
	
	return &AgentChain{
		BaseChain:     NewBaseChain("agent_chain", agent, logger),
		maxIterations: maxIterations,
	}
}

// Execute 执行智能任务链
// 这是最灵活的方式：让 LLM 自主决定如何完成任务
func (c *AgentChain) Execute(ctx context.Context, input ChainInput) (*ChainOutput, error) {
	c.logger.Info("Starting agent chain with LLM",
		zap.String("query", input.Query),
		zap.Int("max_iterations", c.maxIterations))
	
	output := &ChainOutput{
		Steps:    make([]StepRecord, 0),
		Metadata: make(map[string]interface{}),
	}
	
	// 构建初始消息
	messages := []agentic.Message{
		{
			Role: "system",
			Content: c.buildSystemPrompt(),
		},
		{
			Role:    "user",
			Content: input.Query,
		},
	}
	
	// 迭代执行，让 LLM 决定每一步
	for i := 0; i < c.maxIterations; i++ {
		c.logger.Info("Agent iteration",
			zap.Int("iteration", i+1),
			zap.Int("max", c.maxIterations))
		
		// 调用 LLM 获取下一步行动
		response, err := c.agent.LLM.Chat(ctx, messages)
		if err != nil {
			return output, fmt.Errorf("LLM chat failed at iteration %d: %w", i+1, err)
		}
		
		// 解析 LLM 响应
		action, err := c.parseAction(response)
		if err != nil {
			return output, fmt.Errorf("failed to parse LLM response: %w", err)
		}
		
		// 检查是否完成
		if action.Type == "finish" {
			output.Result = action.Result
			c.logger.Info("Agent chain completed",
				zap.Int("iterations", i+1),
				zap.String("result", action.Result))
			return output, nil
		}
		
		// 执行工具调用
		if action.Type == "tool" {
			tool, exists := c.agent.Tools[action.ToolName]
			if !exists {
				return output, fmt.Errorf("unknown tool: %s", action.ToolName)
			}
			
			c.logger.Info("Agent calling tool",
				zap.String("tool", action.ToolName),
				zap.String("input", action.ToolInput))
			
			result, err := tool.Call(ctx, action.ToolInput)
			
			step := StepRecord{
				ToolName: action.ToolName,
				Input:    action.ToolInput,
				Output:   result,
				Error:    err,
			}
			output.Steps = append(output.Steps, step)
			
			// 将工具结果添加到对话历史
			messages = append(messages, agentic.Message{
				Role:    "assistant",
				Content: response,
			})
			
			if err != nil {
				messages = append(messages, agentic.Message{
					Role:    "tool",
					Name:    action.ToolName,
					Content: fmt.Sprintf("Error: %v", err),
				})
			} else {
				messages = append(messages, agentic.Message{
					Role:    "tool",
					Name:    action.ToolName,
					Content: result,
				})
			}
		}
	}
	
	return output, fmt.Errorf("reached maximum iterations (%d) without completion", c.maxIterations)
}

// AgentAction LLM 决定的行动
type AgentAction struct {
	Type      string `json:"type"`       // "tool" 或 "finish"
	ToolName  string `json:"tool_name"`  // 工具名称
	ToolInput string `json:"tool_input"` // 工具输入
	Result    string `json:"result"`     // 最终结果（type=finish 时）
	Reasoning string `json:"reasoning"`  // 推理过程
}

// parseAction 解析 LLM 的响应为行动
func (c *AgentChain) parseAction(response string) (*AgentAction, error) {
	var action AgentAction
	
	// 尝试解析为 JSON
	if err := json.Unmarshal([]byte(response), &action); err != nil {
		// 如果不是 JSON，尝试简单的文本解析
		// 这里可以实现更复杂的解析逻辑
		return nil, fmt.Errorf("failed to parse response as JSON: %w", err)
	}
	
	return &action, nil
}

// buildSystemPrompt 构建系统提示词
func (c *AgentChain) buildSystemPrompt() string {
	// 构建可用工具列表
	toolsDesc := "可用工具:\n"
	for name, tool := range c.agent.Tools {
		toolsDesc += fmt.Sprintf("- %s: %s\n", name, tool.Description())
	}
	
	return fmt.Sprintf(`你是一个智能任务执行助手。你需要分析用户的任务，选择合适的工具按顺序执行。

%s

请以 JSON 格式回复，包含以下字段：
{
  "type": "tool" 或 "finish",
  "tool_name": "工具名称（仅 type=tool 时）",
  "tool_input": "工具输入（仅 type=tool 时）",
  "result": "最终结果（仅 type=finish 时）",
  "reasoning": "你的推理过程"
}

规则：
1. 一次只调用一个工具
2. 根据之前的工具输出决定下一步
3. 完成任务后返回 type=finish
4. 如果遇到错误，尝试其他方案或报告失败`, toolsDesc)
}

// ExecuteWithCallback 执行并在每步后调用回调
// 这对于 UI 实时更新或进度跟踪很有用
func (c *AgentChain) ExecuteWithCallback(
	ctx context.Context,
	input ChainInput,
	callback func(step StepRecord, progress float64),
) (*ChainOutput, error) {
	c.logger.Info("Starting agent chain with callback")
	
	output := &ChainOutput{
		Steps:    make([]StepRecord, 0),
		Metadata: make(map[string]interface{}),
	}
	
	messages := []agentic.Message{
		{
			Role:    "system",
			Content: c.buildSystemPrompt(),
		},
		{
			Role:    "user",
			Content: input.Query,
		},
	}
	
	for i := 0; i < c.maxIterations; i++ {
		response, err := c.agent.LLM.Chat(ctx, messages)
		if err != nil {
			return output, err
		}
		
		action, err := c.parseAction(response)
		if err != nil {
			return output, err
		}
		
		if action.Type == "finish" {
			output.Result = action.Result
			return output, nil
		}
		
		if action.Type == "tool" {
			tool, exists := c.agent.Tools[action.ToolName]
			if !exists {
				return output, fmt.Errorf("unknown tool: %s", action.ToolName)
			}
			
			result, err := tool.Call(ctx, action.ToolInput)
			
			step := StepRecord{
				ToolName: action.ToolName,
				Input:    action.ToolInput,
				Output:   result,
				Error:    err,
			}
			output.Steps = append(output.Steps, step)
			
			// 调用回调函数
			progress := float64(i+1) / float64(c.maxIterations) * 100
			if callback != nil {
				callback(step, progress)
			}
			
			messages = append(messages,
				agentic.Message{Role: "assistant", Content: response},
				agentic.Message{
					Role:    "tool",
					Name:    action.ToolName,
					Content: result,
				},
			)
		}
	}
	
	return output, fmt.Errorf("reached maximum iterations")
}
