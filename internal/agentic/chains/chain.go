package chains

import (
	"context"
	"fmt"

	"github.com/difyz9/ytb2bili/internal/agentic"
	"go.uber.org/zap"
)

// Chain 任务链接口 - 封装一系列相关工具的执行逻辑
type Chain interface {
	// Name 返回链的名称
	Name() string
	
	// Execute 执行链的核心逻辑
	Execute(ctx context.Context, input ChainInput) (*ChainOutput, error)
}

// ChainInput 链的输入参数
type ChainInput struct {
	// Query 自然语言查询
	Query string
	
	// Metadata 额外的元数据
	Metadata map[string]interface{}
}

// ChainOutput 链的输出结果
type ChainOutput struct {
	// Result 最终结果
	Result string
	
	// Steps 执行步骤记录
	Steps []StepRecord
	
	// Metadata 结果元数据
	Metadata map[string]interface{}
}

// StepRecord 单步执行记录
type StepRecord struct {
	// ToolName 使用的工具名称
	ToolName string
	
	// Input 工具输入
	Input string
	
	// Output 工具输出
	Output string
	
	// Error 错误信息
	Error error
}

// BaseChain 基础链实现 - 提供通用功能
type BaseChain struct {
	name   string
	agent  *agentic.Agent
	logger *zap.Logger
}

// NewBaseChain 创建基础链
func NewBaseChain(name string, agent *agentic.Agent, logger *zap.Logger) *BaseChain {
	return &BaseChain{
		name:   name,
		agent:  agent,
		logger: logger,
	}
}

func (c *BaseChain) Name() string {
	return c.name
}

// ExecuteToolSequence 按顺序执行工具链
func (c *BaseChain) ExecuteToolSequence(ctx context.Context, tools []agentic.Tool, inputs []string) (*ChainOutput, error) {
	if len(tools) != len(inputs) {
		return nil, fmt.Errorf("tools and inputs length mismatch: %d != %d", len(tools), len(inputs))
	}
	
	output := &ChainOutput{
		Steps:    make([]StepRecord, 0, len(tools)),
		Metadata: make(map[string]interface{}),
	}
	
	var lastOutput string
	
	for i, tool := range tools {
		c.logger.Info("Executing tool in chain",
			zap.String("chain", c.name),
			zap.String("tool", tool.Name()),
			zap.Int("step", i+1),
			zap.Int("total", len(tools)))
		
		result, err := tool.Call(ctx, inputs[i])
		
		step := StepRecord{
			ToolName: tool.Name(),
			Input:    inputs[i],
			Output:   result,
			Error:    err,
		}
		output.Steps = append(output.Steps, step)
		
		if err != nil {
			c.logger.Error("Tool execution failed",
				zap.String("tool", tool.Name()),
				zap.Error(err))
			return output, fmt.Errorf("tool %s failed: %w", tool.Name(), err)
		}
		
		lastOutput = result
	}
	
	output.Result = lastOutput
	return output, nil
}
