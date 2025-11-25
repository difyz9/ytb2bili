package handlers

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/difyz9/ytb2bili/internal/chain_task/base"
	"github.com/difyz9/ytb2bili/internal/chain_task/manager"
	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/pkg/cos"
	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
	"gorm.io/gorm"
)

type WhisperHandler struct {
	base.BaseTask
	App       *core.AppServer
	DB        *gorm.DB
	ModelPath string // Whisper 模型路径
	Language  string // 语音识别语言
	Threads   int    // 线程数
}

func NewWhisperHandler(name string, app *core.AppServer, stateManager *manager.StateManager, client *cos.CosClient, modelPath string, language string, threads int) *WhisperHandler {
	if threads <= 0 {
		threads = 4 // 默认使用4个线程
	}
	if language == "" {
		language = "en" // 默认英语
	}
	
	return &WhisperHandler{
		BaseTask: base.BaseTask{
			Name:         name,
			StateManager: stateManager,
			Client:       client,
		},
		App:       app,
		ModelPath: modelPath,
		Language:  language,
		Threads:   threads,
	}
}

func (h *WhisperHandler) Execute(context map[string]interface{}) bool {
	fmt.Println("开始使用 Whisper 转录音频")
	
	// 检查 WAV 音频文件是否存在
	if _, err := os.Stat(h.StateManager.OriginalWAV); os.IsNotExist(err) {
		fmt.Printf("错误: WAV 音频文件不存在: %s\n", h.StateManager.OriginalWAV)
		context["error"] = fmt.Sprintf("WAV 音频文件不存在: %s", h.StateManager.OriginalWAV)
		return false
	}
	
	// 检查模型文件是否存在
	if _, err := os.Stat(h.ModelPath); os.IsNotExist(err) {
		fmt.Printf("错误: Whisper 模型文件不存在: %s\n", h.ModelPath)
		context["error"] = fmt.Sprintf("Whisper 模型文件不存在: %s", h.ModelPath)
		return false
	}
	
	fmt.Printf("📝 使用 Whisper 转录: %s\n", h.StateManager.OriginalWAV)
	fmt.Printf("   模型: %s\n", h.ModelPath)
	fmt.Printf("   语言: %s\n", h.Language)
	fmt.Printf("   线程: %d\n", h.Threads)
	
	// 执行转录，生成 SRT 字幕文件
	if err := h.transcribe(h.ModelPath, h.StateManager.OriginalWAV, h.Language, h.Threads, true, h.StateManager.OriginalSRT); err != nil {
		fmt.Printf("❌ Whisper 转录失败: %v\n", err)
		context["error"] = fmt.Sprintf("Whisper 转录失败: %v", err)
		return false
	}
	
	fmt.Printf("✅ Whisper 转录完成，字幕文件保存至: %s\n", h.StateManager.OriginalSRT)
	context["subtitle_path"] = h.StateManager.OriginalSRT
	return true
}

// transcribe 执行语音识别
func (h *WhisperHandler) transcribe(modelPath, wavPath, language string, threads int, outputSRT bool, outputPath string) error {
	// 加载模型
	model, err := whisper.New(modelPath)
	if err != nil {
		return fmt.Errorf("加载模型失败: %v", err)
	}
	defer model.Close()

	// 读取WAV文件
	samples, err := readWAVFile(wavPath)
	if err != nil {
		return fmt.Errorf("读取WAV文件失败: %v", err)
	}

	// 创建处理上下文
	context, err := model.NewContext()
	if err != nil {
		return fmt.Errorf("创建上下文失败: %v", err)
	}

	// 设置语言
	if language != "auto" {
		if err := context.SetLanguage(language); err != nil {
			return fmt.Errorf("设置语言失败: %v", err)
		}
	}

	// 设置线程数
	context.SetThreads(uint(threads))

	// 启用翻译模式（如果需要）
	context.SetTranslate(false)

	// 处理音频
	if err := context.Process(samples, nil, nil, nil); err != nil {
		return fmt.Errorf("处理音频失败: %v", err)
	}

	// 创建输出文件
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %v", err)
	}
	defer outFile.Close()

	// 收集所有片段
	var segments []whisper.Segment
	for {
		segment, err := context.NextSegment()
		if err != nil {
			break
		}
		segments = append(segments, segment)
	}

	// 根据格式输出
	if outputSRT {
		// 输出SRT格式
		for i, segment := range segments {
			// SRT序号（从1开始）
			fmt.Fprintf(outFile, "%d\n", i+1)

			// SRT时间格式: HH:MM:SS,mmm --> HH:MM:SS,mmm
			fmt.Fprintf(outFile, "%s --> %s\n",
				formatSRTTime(segment.Start),
				formatSRTTime(segment.End))

			// 字幕文本
			fmt.Fprintf(outFile, "%s\n\n", strings.TrimSpace(segment.Text))

			// 同时输出到控制台
			fmt.Printf("[%6s --> %6s]  %s\n",
				segment.Start.Truncate(time.Millisecond),
				segment.End.Truncate(time.Millisecond),
				segment.Text)
		}
	} else {
		// 输出纯文本格式
		for _, segment := range segments {
			// 带时间戳的文本
			fmt.Fprintf(outFile, "[%s --> %s]  %s\n",
				segment.Start.Truncate(time.Millisecond),
				segment.End.Truncate(time.Millisecond),
				segment.Text)

			// 同时输出到控制台
			fmt.Printf("[%6s --> %6s]  %s\n",
				segment.Start.Truncate(time.Millisecond),
				segment.End.Truncate(time.Millisecond),
				segment.Text)
		}
	}

	return nil
}

// readWAVFile 读取WAV文件并返回音频样本
func readWAVFile(wavPath string) ([]float32, error) {
	// 读取文件内容
	data, err := os.ReadFile(wavPath)
	if err != nil {
		return nil, err
	}

	// 跳过WAV文件头（通常44字节）
	const wavHeaderSize = 44
	if len(data) < wavHeaderSize {
		return nil, fmt.Errorf("WAV文件太小")
	}

	// 将PCM数据转换为float32样本
	pcmData := data[wavHeaderSize:]
	samples := make([]float32, len(pcmData)/2)

	for i := 0; i < len(samples); i++ {
		// 读取16位PCM样本 (小端序)
		sample := int16(pcmData[i*2]) | int16(pcmData[i*2+1])<<8
		// 转换为-1.0到1.0的浮点数
		samples[i] = float32(sample) / 32768.0
	}

	return samples, nil
}

// formatSRTTime 格式化时间为SRT格式 (HH:MM:SS,mmm)
func formatSRTTime(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	milliseconds := int(d.Milliseconds()) % 1000

	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, seconds, milliseconds)
}
