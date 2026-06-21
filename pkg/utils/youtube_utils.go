package utils

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// extractBvidFromURL 从 B站 URL 中提取 BVID
// 支持的URL格式:
// https://www.bilibili.com/video/BV1234567890
// https://bilibili.com/video/BV1234567890
// https://b23.tv/BV1234567890
// BV1234567890
func ExtractBvidFromURL(url string) string {
	if url == "" {
		return ""
	}

	// 如果直接就是BVID格式，直接返回
	if strings.HasPrefix(url, "BV") && len(url) >= 10 {
		// 简单验证BVID格式 (BV + 10位字符)
		bvidPattern := regexp.MustCompile(`^BV[0-9A-Za-z]{10}`)
		if bvidPattern.MatchString(url) {
			return url[:12] // BV + 10位字符 = 12位
		}
	}

	// 从完整URL中提取BVID
	// 匹配 /video/BV 或 /BV 后面的12位字符
	bvidPattern := regexp.MustCompile(`(?:video/|/)?(BV[0-9A-Za-z]{10})`)
	matches := bvidPattern.FindStringSubmatch(url)
	if len(matches) >= 2 {
		return matches[1]
	}

	return ""
}

func extractBiliVideoID(videoURL string) string {
	// 去掉前缀 ":"
	videoURL = strings.TrimPrefix(videoURL, ":")

	// 解析 URL
	parsedURL, err := url.Parse(videoURL)
	if err != nil {
		fmt.Printf("URL 解析失败: %v\n", err)
		return ""
	}

	// 提取路径中的视频 ID
	pathSegments := strings.Split(parsedURL.Path, "/")
	if len(pathSegments) > 2 {
		videoID := pathSegments[2]
		// 检查是否有分页参数
		if page := parsedURL.Query().Get("p"); page != "" {
			videoID += "_p" + page
		}
		return videoID
	}

	return ""
}

// ExtractVideoID 从 YouTube URL 中提取视频 Id
func extractYoutTuBeVideoID(url string) (string, error) {
	pattern := `(?:v=|\/)([0-9A-Za-z_-]{11})`
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", err
	}
	match := re.FindStringSubmatch(url)
	if len(match) > 1 {
		return match[1], nil
	}
	return "", errors.New("Invalid YouTube URL")
}

func ExtractVideoID(videoURL string) string {

	parsedURL, err := url.Parse(videoURL)
	if err != nil {
		fmt.Printf("URL 解析失败: %v\n", err)
		return ""
	}

	host := parsedURL.Host

	// 1. YouTube - 使用原生视频ID
	if strings.Contains(host, "youtube.com") || strings.Contains(host, "youtu.be") {
		videoId, _ := extractYoutTuBeVideoID(videoURL)
		return videoId
	}

	// 2. Bilibili - 使用BV号
	if strings.Contains(host, "bilibili.com") || strings.Contains(host, "b23.tv") {
		return extractBiliVideoID(videoURL)
	}

	// 3. 其他 yt-dlp 支持的平台 - 生成基于 URL 的唯一 ID
	// yt-dlp 支持 1000+ 个平台，让它自己判断是否支持
	// 生成一个可复现的短ID（基于URL哈希）
	videoID := generateURLHash(videoURL)
	fmt.Printf("🌐 其他平台视频: %s -> ID: %s (yt-dlp 将尝试下载)\n", host, videoID)
	return videoID
}

// generateURLHash 生成基于 URL 的短哈希作为视频 ID
// 这样同一个 URL 会生成相同的 ID，便于去重
func generateURLHash(videoURL string) string {
	// 使用简单的哈希算法生成12位ID
	// 与 YouTube 的 11 位 ID 长度接近
	var hash uint64 = 0
	for _, c := range videoURL {
		hash = hash*31 + uint64(c)
	}

	// 转换为 Base36（0-9, a-z）编码，更短
	const chars = "0123456789abcdefghijklmnopqrstuvwxyz"
	result := make([]byte, 12)
	for i := 11; i >= 0; i-- {
		result[i] = chars[hash%36]
		hash /= 36
	}
	return string(result)
}
