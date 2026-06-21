package llm

import (
	"context"
	"fmt"
	"io"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

func toEinoMessages(messages []Message) []*schema.Message {
	out := make([]*schema.Message, 0, len(messages))
	for _, msg := range messages {
		role := schema.User
		switch msg.Role {
		case string(schema.System):
			role = schema.System
		case string(schema.Assistant):
			role = schema.Assistant
		case string(schema.Tool):
			role = schema.Tool
		}
		out = append(out, &schema.Message{
			Role:    role,
			Content: msg.Content,
		})
	}
	return out
}

func (c *EinoChatClient) Chat(ctx context.Context, messages []Message) (string, error) {
	return c.ChatWithOptions(ctx, messages, ChatOptions{})
}

func (c *EinoChatClient) ChatWithOptions(ctx context.Context, messages []Message, opts ChatOptions) (string, error) {
	if c == nil || c.chatModel == nil {
		return "", fmt.Errorf("llm client is not initialized")
	}

	chatModel := c.chatModel
	if opts.Model != "" || opts.MaxTokens != nil || opts.Temperature != nil {
		modelName := c.modelName
		if opts.Model != "" {
			modelName = opts.Model
		}

		cfg := &einoopenai.ChatModelConfig{
			Model:   modelName,
			APIKey:  c.apiKey,
			BaseURL: c.baseURL,
			Timeout: c.timeout,
		}
		if opts.MaxTokens != nil {
			cfg.MaxTokens = opts.MaxTokens
		}
		if opts.Temperature != nil {
			cfg.Temperature = opts.Temperature
		}

		var err error
		chatModel, err = einoopenai.NewChatModel(ctx, cfg)
		if err != nil {
			return "", fmt.Errorf("failed to create per-request Eino chat model: %w", err)
		}
	}

	resp, err := chatModel.Generate(ctx, toEinoMessages(messages))
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", fmt.Errorf("chat model returned nil response")
	}
	return resp.Content, nil
}

func (c *EinoChatClient) ChatStream(ctx context.Context, messages []Message) (<-chan string, error) {
	if c == nil || c.chatModel == nil {
		return nil, fmt.Errorf("llm client is not initialized")
	}

	stream, err := c.chatModel.Stream(ctx, toEinoMessages(messages))
	if err != nil {
		return nil, err
	}

	chunks := make(chan string)
	go func() {
		defer close(chunks)
		defer stream.Close()

		for {
			msg, recvErr := stream.Recv()
			if recvErr != nil {
				if recvErr == io.EOF {
					return
				}
				c.logger.Warn("Eino stream receive failed", zapError(recvErr))
				return
			}
			if msg == nil || msg.Content == "" {
				continue
			}

			select {
			case <-ctx.Done():
				return
			case chunks <- msg.Content:
			}
		}
	}()

	return chunks, nil
}
