# Go-Agentic Tools

This directory contains ready-to-use tools for the go-agentic framework.

## Available Tools

### Media Tools

#### 1. Download Video Tool
Downloads YouTube videos using yt-dlp.

**Dependencies**: `yt-dlp` (install: `pip install yt-dlp` or `brew install yt-dlp`)

**Example**:
```go
config := tools.DownloadVideoConfig{
    DownloadDir: "./downloads",
    CookiesFile: "./cookies.txt",  // optional
    ProxyURL:    "http://localhost:1080",  // optional
}
tool, _ := tools.NewDownloadVideoTool(config, logger)
videoPath, _ := tool.Call(ctx, "dQw4w9WgXcQ")
```

#### 2. Download Thumbnail Tool
Downloads YouTube video thumbnails with quality fallback.

**Example**:
```go
tool, _ := tools.NewDownloadThumbnailTool("./downloads", logger)
thumbnailPath, _ := tool.Call(ctx, "dQw4w9WgXcQ")
```

#### 3. Extract Audio Tool
Extracts audio from video files using FFmpeg.

**Dependencies**: `ffmpeg` (install: `brew install ffmpeg` or `apt install ffmpeg`)

**Example**:
```go
tool, _ := tools.NewExtractAudioTool("", logger)  // empty = auto-find ffmpeg
audioPath, _ := tool.Call(ctx, "/path/to/video.mp4")

// Extract WAV for speech recognition
wavPath, _ := tool.ExtractWAV(ctx, "/path/to/video.mp4")
```

#### 4. Transcode Video Tool
Transcodes a local video into another format, codec, resolution, or frame rate using FFmpeg.

**Dependencies**: `ffmpeg` (install: `brew install ffmpeg` or `apt install ffmpeg`)

**Example**:
```go
tool, _ := tools.NewTranscodeVideoTool("", logger)
outputPath, _ := tool.Call(ctx, "/path/to/video.mov")

result, _ := tool.InvokableRun(ctx, `{"video_path":"/path/to/video.mov","output_format":"mp4","resolution":"1080p","video_codec":"h264","crf":23}`)
_ = result
```

### AI Tools

#### 5. Generate Image Prompt Tool
Uses LLM to convert simple descriptions to detailed image prompts.

**Example**:
```go
tool := tools.NewGenerateImagePromptTool(llmClient, logger)
result, _ := tool.Call(ctx, `{"description": "futuristic city", "style": "digital_art"}`)
```

#### 6. Generate Image Tool
Generates images using kie.ai Nano Banana Pro API.

**Example**:
```go
tool := tools.NewGenerateImageTool("your-api-key", "./output", logger)
result, _ := tool.Call(ctx, `{"prompt": "...", "aspect_ratio": "16:9", "resolution": "2K"}`)
```

### Database Tools

#### 7. SQL Database Tool
Executes SQL SELECT queries safely and returns JSON results.

**Example**:
```go
tool := tools.NewSQLDatabaseTool(db, logger)
result, _ := tool.Call(ctx, "SELECT COUNT(*) as count FROM users")
```

## Creating Custom Tools

### Basic Tool Template

```go
package tools

import (
    "context"
    "github.com/zolagz/ytb2bili/"
    "go.uber.org/zap"
)

type MyTool struct {
    *agentic.BaseTool
    logger *zap.Logger
}

func NewMyTool(logger *zap.Logger) *MyTool {
    return &MyTool{
        BaseTool: agentic.NewBaseTool(
            "my_tool",
            `Tool description here.
Input format: description
Output: description`,
        ),
        logger: logger,
    }
}

func (t *MyTool) Call(ctx context.Context, input string) (string, error) {
    t.logger.Info("Tool called", zap.String("input", input))
    // Your implementation
    return "result", nil
}
```

### Service Integration Tools

For tools that require external services (TTS, ASR, Translation, etc.), create interface-based tools:

```go
// Define interface
type SpeechRecognizer interface {
    Recognize(ctx context.Context, audioPath string) (string, error)
}

// Create tool with interface
type GenerateSubtitleTool struct {
    *agentic.BaseTool
    recognizer SpeechRecognizer
    logger     *zap.Logger
}

// User provides their implementation
type MyASRService struct {
    apiKey string
}

func (s *MyASRService) Recognize(ctx context.Context, audioPath string) (string, error) {
    // Call your ASR service
    return "recognized text", nil
}

// Usage
asrService := &MyASRService{apiKey: "xxx"}
tool := NewGenerateSubtitleTool(asrService, logger)
```

## Tool Categories

### Media Processing
- Video download (YouTube, etc.)
- Thumbnail extraction
- Audio extraction
- Video format conversion

### AI Generation
- Image prompt generation
- Image generation
- Text generation
- Code generation

### Data & Storage
- SQL database queries
- File operations
- Cloud storage (S3, COS, etc.)

### External Services
- HTTP requests
- API integrations
- Webhook handling

## Best Practices

1. **Keep tools focused**: Each tool should do one thing well
2. **Handle errors gracefully**: Return descriptive error messages
3. **Add logging**: Use structured logging for debugging
4. **Support context**: Respect context cancellation
5. **Document clearly**: Include examples in Description
6. **Make configurable**: Use constructor parameters for configuration
7. **Check dependencies**: Validate external dependencies in constructor
8. **Return structured data**: Use JSON for complex results

## Testing Tools

```go
func TestMyTool(t *testing.T) {
    logger, _ := zap.NewDevelopment()
    tool := NewMyTool(logger)
    
    ctx := context.Background()
    result, err := tool.Call(ctx, "test input")
    
    if err != nil {
        t.Fatalf("Tool failed: %v", err)
    }
    
    if result == "" {
        t.Error("Expected non-empty result")
    }
}
```

## Contributing

When adding new tools:
1. Keep minimal dependencies
2. Make tools reusable across projects
3. Provide clear documentation
4. Add usage examples
5. Include error handling
6. Write tests for critical functionality
