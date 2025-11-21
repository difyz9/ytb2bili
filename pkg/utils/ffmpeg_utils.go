package utils

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// TranscodeVideo 使用 H.264 编码器转码视频文件
func TranscodeVideo(inputVideoPath, outputVideoPath, preset string, crf int, audioBitrate string, fps int) error {
	// 构建 ffmpeg 命令参数
	cmd := []string{
		"-y",
		"-i", inputVideoPath,
		"-c:v", "libx264",
		"-preset", preset,
		"-crf", strconv.Itoa(crf),
		"-r", strconv.Itoa(fps),
		"-c:a", "aac",
		"-b:a", audioBitrate,
		outputVideoPath,
	}

	// 创建 ffmpeg 命令对象
	ffmpegCmd := exec.Command("ffmpeg", cmd...)

	// 执行命令并捕获输出
	output, err := ffmpegCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("视频转码过程出现错误: %v\n%s\n", err, string(output))
		return err
	}

	fmt.Println("视频转码成功")
	return nil
}

// ExtractWaveAudio 从视频文件中分离出WAV格式的音频
func ExtractWaveAudio(inputFile, outputFile string) error {
	// 构造 ffmpeg 命令，提取音频并转换为WAV格式
	cmd := exec.Command(
		"ffmpeg",
		"-y",                    // 覆盖输出文件
		"-i", inputFile,         // 输入文件
		"-vn",                   // 不处理视频流
        "-acodec", "pcm_s16le",  // PCM 16位
	   "-ar", "16000",          // 16kHz 采样率（whisper 标准）
        "-ac", "1",              // 单声道（语音识别足够）
        outputFile,
	)

	// 设置标准输出和标准错误
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// 执行命令
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("ffmpeg 提取WAV音频失败: %v", err)
	}

	fmt.Printf("成功从 %s 提取WAV音频到 %s\n", inputFile, outputFile)
	return nil
}

// ExtractAudio 从视频文件中分离出音频
func ExtractAudio(inputFile, outputFile string) error {
	// 构造 ffmpeg 命令
	cmd := exec.Command(
		"ffmpeg",
		"-y",
		"-i", inputFile, // 输入文件
		"-q:a", "0", // 音频质量（0 表示最高质量）
		"-map", "a", // 只提取音频流
		outputFile, // 输出文件
	)

	// 设置标准输出和标准错误
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// 执行命令
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("ffmpeg 执行失败: %v", err)
	}

	return nil
}

//测试不能使用

func Split_audio_byray(inputFile, outputFile string) error {
	// 构造 ffmpeg 命令
	cmd := exec.Command(
		"ffmpeg",
		"-y",
		"-i", inputFile, // 输入文件
		"-vn",
		"-ac",
		"1",
		"-b:a",
		"192k",
		"-c:a",
		"aac",
		outputFile, // 输出文件
	)

	// 设置标准输出和标准错误
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// 执行命令
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("ffmpeg 执行失败: %v", err)
	}

	return nil
}

// CheckAudioFile 检查音频文件是否存在且大小不为零
func CheckAudioFile(filePath string) (bool, error) {
	// 获取文件信息
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在
			return false, nil
		}
		// 其他错误
		return false, err
	}

	// 检查文件大小是否为零
	return fileInfo.Size() > 0, nil
}

// ExtractVideoWithoutAudio 从视频中分离无音视频并编码为 H.264
func ExtractVideoWithoutAudio(inputVideoPath, outputVideoPath string) error {
	// 构建 ffmpeg 命令及其参数
	cmd := exec.Command(
		"ffmpeg",
		"-y",
		"-i", inputVideoPath,
		"-an",             // 去掉音频流
		"-c:v", "libx264", // 使用 H.264 编码器
		"-preset", "medium", // 编码预设，可根据需要调整
		"-crf", "23", // 恒定速率因子，可根据需要调整
		outputVideoPath)

	// 执行命令并捕获输出
	output, err := cmd.CombinedOutput()
	if err != nil {
		// 若执行出错，打印错误信息和命令输出
		log.Printf("执行 ffmpeg 命令时出错: %v", err)
		log.Printf("命令输出: %s", string(output))
		return err
	}

	// 若执行成功，打印成功信息
	fmt.Println("ffmpeg 命令执行成功")
	return nil
}

func ExtractThumbnail(videoPath, outputPath string) error {
	// 构建 ffmpeg 命令
	cmd := exec.Command("ffmpeg", "-y", "-i", videoPath, "-ss", "00:00:01", "-vframes", "1", outputPath)

	// 执行命令
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("执行 ffmpeg 命令出错: %v", err)
	}

	return nil
}

func ConvertToHLS(inputPath, outputDir string) error {
	// 确保输出目录存在
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	//fileName, err := GetFileNameWithoutExtension(inputPath)
	//if err != nil {
	//	return err
	//}

	// FFmpeg 命令：将 MP4 转为 HLS
	cmd := exec.Command(
		"ffmpeg",
		"-i", inputPath, // 输入文件
		"-c:v", "libx264", // 视频编码 H.264
		"-c:a", "aac", // 音频编码 AAC
		"-f", "hls", // 输出格式 HLS
		"-hls_time", "5", // 每个 TS 切片 6 秒
		"-hls_list_size", "0", // M3U8 中保留所有分段
		"-hls_segment_filename", outputDir+"/vid_%04d.ts", // TS 文件名格式
		outputDir+"/output.m3u8", // 输出 M3U8 文件
	)

	// 打印执行的命令（调试用）
	log.Println("Executing:", cmd.String())

	// 运行命令并捕获输出
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

// 解析M3U8文件并返回包含的所有文件名
func ParseM3U8File(filePath string) ([]string, error) {
	// 获取M3U8文件所在目录
	m3u8Dir := filepath.Dir(filePath)
	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	var fullPaths []string
	scanner := bufio.NewScanner(file)

	// 跳过M3U8文件头
	if scanner.Scan() {
		line := scanner.Text()
		if line != "#EXTM3U" {
			return nil, fmt.Errorf("不是有效的M3U8文件: 缺少#EXTM3U头部")
		}
	}

	// 逐行解析
	for scanner.Scan() {
		line := scanner.Text()

		// 跳过注释行和标签行
		if strings.HasPrefix(line, "#") {
			continue
		}

		// 提取文件名并构建完整路径
		if line != "" {
			// 去除可能的空白字符
			relativePath := strings.TrimSpace(line)

			// 构建完整路径（M3U8目录 + 相对路径）
			// 注意：如果TS文件是绝对路径，filepath.Join会正确处理
			fullPath := filepath.Join(m3u8Dir, relativePath)

			fullPaths = append(fullPaths, fullPath)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	return fullPaths, nil
}
