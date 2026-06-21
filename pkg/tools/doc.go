// Package tools provides reusable tool implementations for go-agentic.
//
// This package includes various ready-to-use tools that can be integrated
// with the go-agentic framework:
//
// Image Generation Tools:
//   - GenerateImagePromptTool: Convert simple text to detailed image prompts
//   - GenerateImageTool: Generate images using kie.ai Nano Banana Pro API
//
// Database Tools:
//   - SQLDatabaseTool: Query databases using natural language
//
// Audio/Video Tools:
//   - ExtractAudioTool: Extract audio from video files using FFmpeg
//   - BcutTranscriberTool: Transcribe audio to text using Bilibili BCut API
//   - DownloadVideoTool: Download videos from various platforms
//   - DownloadThumbnailTool: Download video thumbnails
//
// Usage:
//
//	import "github.com/zolagz/ytb2bili/pkg/tools"
//
//	// Create image prompt tool
//	promptTool := tools.NewGenerateImagePromptTool(llmClient, logger)
//
//	// Create image generator tool
//	imageTool := tools.NewGenerateImageTool(apiKey, outputDir, logger)
//
//	// Create SQL database tool
//	sqlTool, err := tools.NewSQLDatabaseTool(db, "mysql", llm, logger)
//
//	// Create audio extraction tool
//	audioTool, err := tools.NewExtractAudioTool("", logger)
//
//	// Create BCut transcriber tool
//	transcribeTool := tools.NewBcutTranscriberTool(logger)
//
//	// Register with agent
//	agent.RegisterTools(promptTool, imageTool, sqlTool, audioTool, transcribeTool)
package tools
