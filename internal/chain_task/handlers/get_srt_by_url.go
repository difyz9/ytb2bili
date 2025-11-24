package handlers

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"gorm.io/gorm"
	"html"
	"io/ioutil"
	"net/http"
	"net/url"

	"os"
	"regexp"
	"strconv"
	"strings"
	"github.com/difyz9/ytb2bili/internal/chain_task/base"
	"github.com/difyz9/ytb2bili/internal/chain_task/manager"
	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/pkg/cos"
)

// XMLText XML 字幕中的文本元素
type XMLText struct {
	XMLName  xml.Name `xml:"text"`
	Start    string   `xml:"start,attr"`
	Duration string   `xml:"dur,attr"`
	Content  string   `xml:",chardata"`
}

// XMLTranscript XML 字幕文档
type XMLTranscript struct {
	XMLName xml.Name  `xml:"transcript"`
	Texts   []XMLText `xml:"text"`
}

// TextInfo 字幕信息
type TextInfo struct {
	StartTime float64 `json:"start_time"`
	Duration  float64 `json:"duration"`
	Content   string  `json:"content"`
}

// TranscriptData 字幕数据
type TranscriptData struct {
	Transcript []TextInfo `json:"transcript"`
}

// Task03Handler 获取字幕任务
type Task03Handler struct {
	base.BaseTask
	App *core.AppServer
	DB  *gorm.DB
}

// NewGetSubtitlesTask 创建获取字幕任务
func NewTask03Handler(name string, app *core.AppServer, db *gorm.DB, stateManager *manager.StateManager, client *cos.CosClient) *Task03Handler {
	return &Task03Handler{
		BaseTask: base.BaseTask{
			Name:         name, // "GetSubtitles",
			StateManager: stateManager,
			Client:       client,
		},
		App: app,
		DB:  db,
	}
}

// Execute 执行任务
func (t *Task03Handler) Execute(context map[string]interface{}) bool {
	videoID := t.StateManager.VideoID

	// 获取字幕 URL
	srtURL, err := t.getVideoSrtURL(videoID)
	if err != nil {
		fmt.Printf("获取字幕 URL 失败: %v\n", err)
		return false
	}

	// 获取字幕内容
	transcript, err := t.getSrtFile(srtURL)
	if err != nil {
		fmt.Printf("获取字幕内容失败: %v\n", err)
		return false
	}

	// 保存字幕到文件
	//transcriptFile := filepath.Join(t.StateManager.CurrentDir, "transcript.json")
	data, err := json.MarshalIndent(transcript, "", "  ")
	if err != nil {
		fmt.Printf("序列化字幕数据失败: %v\n", err)
		return false
	}
	//print(transcriptFile)
	if err := os.WriteFile(t.StateManager.OriginalJSON, data, 0644); err != nil {
		fmt.Printf("保存字幕文件失败: %v\n", err)
		return false
	}

	// 将字幕数据添加到上下文
	context["transcript"] = transcript

	fmt.Println("字幕获取成功")
	return true
}

// getVideoSrtURL 获取视频字幕 URL
func (t *Task03Handler) getVideoSrtURL(videoID string) (string, error) {
	videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)

	// 尝试使用代理（如果配置了）
	useProxy := t.App.Config != nil && t.App.Config.ProxyConfig != nil && 
		t.App.Config.ProxyConfig.UseProxy && t.App.Config.ProxyConfig.ProxyHost != ""

	if useProxy {
		t.App.Logger.Info("🔄 尝试使用代理获取字幕URL...")
		srtURL, err := t.fetchSrtURL(videoURL, true)
		if err == nil {
			return srtURL, nil
		}
		t.App.Logger.Warnf("⚠️ 代理获取字幕URL失败: %v，尝试不使用代理重试...", err)
	}

	// 不使用代理重试
	t.App.Logger.Info("🔄 尝试不使用代理获取字幕URL...")
	return t.fetchSrtURL(videoURL, false)
}

// fetchSrtURL 实际获取字幕URL的方法
func (t *Task03Handler) fetchSrtURL(videoURL string, useProxy bool) (string, error) {
	req, err := http.NewRequest("GET", videoURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	client := t.createHTTPClient(useProxy)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("请求失败，状态码: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	pattern := regexp.MustCompile(`https://www.youtube.com/api/timedtext\?v=[^"]*`)
	matches := pattern.FindStringSubmatch(string(body))
	if len(matches) == 0 {
		return "", fmt.Errorf("未找到字幕 URL")
	}

	srtURL := matches[0]
	srtURL = regexp.MustCompile(`\\u0026`).ReplaceAllString(srtURL, "&")

	return srtURL, nil
}

// getSrtFile 获取字幕文件内容
func (t *Task03Handler) getSrtFile(srtURL string) (*TranscriptData, error) {
	// 尝试使用代理（如果配置了）
	useProxy := t.App.Config != nil && t.App.Config.ProxyConfig != nil && 
		t.App.Config.ProxyConfig.UseProxy && t.App.Config.ProxyConfig.ProxyHost != ""

	if useProxy {
		t.App.Logger.Info("🔄 尝试使用代理获取字幕内容...")
		transcript, err := t.fetchSrtContent(srtURL, true)
		if err == nil {
			return transcript, nil
		}
		t.App.Logger.Warnf("⚠️ 代理获取字幕内容失败: %v，尝试不使用代理重试...", err)
	}

	// 不使用代理重试
	t.App.Logger.Info("🔄 尝试不使用代理获取字幕内容...")
	return t.fetchSrtContent(srtURL, false)
}

// fetchSrtContent 实际获取字幕内容的方法
func (t *Task03Handler) fetchSrtContent(srtURL string, useProxy bool) (*TranscriptData, error) {
	req, err := http.NewRequest("GET", srtURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3")

	client := t.createHTTPClient(useProxy)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("请求失败，状态码: %d", resp.StatusCode)
	}

	var transcript XMLTranscript
	if err := xml.NewDecoder(resp.Body).Decode(&transcript); err != nil {
		return nil, err
	}

	var textInfos []TextInfo
	for _, text := range transcript.Texts {
		startTime, err := strconv.ParseFloat(text.Start, 64)
		if err != nil {
			return nil, fmt.Errorf("无法解析起始时间: %v", err)
		}

		duration, err := strconv.ParseFloat(text.Duration, 64)
		if err != nil {
			return nil, fmt.Errorf("无法解析持续时间: %v", err)
		}

		// 处理特殊字符
		textInfos = append(textInfos, TextInfo{
			StartTime: startTime,
			Duration:  duration,
			Content:   strings.ReplaceAll(html.UnescapeString(text.Content), "\u00A0", " "),
		})
	}

	return &TranscriptData{Transcript: textInfos}, nil
}

// createHTTPClient 创建HTTP客户端（支持代理）
func (t *Task03Handler) createHTTPClient(useProxy bool) *http.Client {
	client := &http.Client{}

	if useProxy && t.App.Config != nil && t.App.Config.ProxyConfig != nil && 
		t.App.Config.ProxyConfig.UseProxy && t.App.Config.ProxyConfig.ProxyHost != "" {
		proxyURL, err := url.Parse(t.App.Config.ProxyConfig.ProxyHost)
		if err == nil {
			transport := &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			}
			client = &http.Client{
				Transport: transport,
			}
			t.App.Logger.Infof("📡 使用代理: %s", t.App.Config.ProxyConfig.ProxyHost)
		} else {
			t.App.Logger.Warnf("⚠️ 代理URL解析失败: %v", err)
		}
	}

	return client
}
