# YouTube Cookies 配置指南

## 问题说明

YouTube 现在会检测机器人访问，导致 yt-dlp 无法直接获取视频信息和下载视频。错误信息：

```
ERROR: [youtube] VLrWGo-WPOI: Sign in to confirm you're not a bot
```

## 解决方案：导出浏览器 Cookies

### 方法 1：使用 yt-dlp 自动导出（推荐）

yt-dlp 可以直接从浏览器读取 cookies，无需手动导出。

#### 1. 在浏览器中登录 YouTube

打开 Chrome/Edge/Firefox，登录您的 YouTube 账号。

#### 2. 修改下载命令

在 `down_load_video.go` 中添加 `--cookies-from-browser` 参数：

```go
// 示例：从 Chrome 读取 cookies
command = append(command, "--cookies-from-browser", "chrome")

// 或从 Edge 读取
command = append(command, "--cookies-from-browser", "edge")

// 或从 Firefox 读取
command = append(command, "--cookies-from-browser", "firefox")
```

### 方法 2：手动导出 Cookies 文件

#### 1. 安装浏览器扩展

**Chrome/Edge:**
- 安装 [Get cookies.txt LOCALLY](https://chrome.google.com/webstore/detail/get-cookiestxt-locally/cclelndahbckbenkjhflpdbgdldlbecc)

**Firefox:**
- 安装 [cookies.txt](https://addons.mozilla.org/en-US/firefox/addon/cookies-txt/)

#### 2. 导出 Cookies

1. 打开 YouTube 并登录
2. 点击扩展图标
3. 点击 "Export" 或 "Download"
4. 保存为 `youtube_cookies.txt`

#### 3. 放置 Cookies 文件

将 `youtube_cookies.txt` 放到项目根目录：
```
E:/githubitem/ytb2bili/youtube_cookies.txt
```

#### 4. 修改代码使用 Cookies

在 `down_load_video.go` 中添加：
```go
command = append(command, "--cookies", "youtube_cookies.txt")
```

## 当前配置状态

✅ 代理已配置：`http://127.0.0.1:7897`
❌ Cookies 未配置：需要添加 cookies 支持

## 推荐配置

使用方法 1（自动从浏览器读取）最简单，无需手动导出和更新 cookies。

### 修改步骤

1. 确保在 Chrome/Edge 中登录了 YouTube
2. 修改 `internal/chain_task/handlers/down_load_video.go`
3. 在构建 yt-dlp 命令时添加 `--cookies-from-browser chrome`
4. 重新编译并启动服务

## 测试

修改后，可以测试：

```powershell
# 测试从浏览器读取 cookies
.\yt-dlp.exe --cookies-from-browser chrome --proxy "http://127.0.0.1:7897" --dump-json --no-download "https://youtube.com/shorts/VLrWGo-WPOI"
```

如果成功，应该能看到视频的 JSON 信息，包括 title 和 description。

## 注意事项

1. **隐私**：cookies 包含您的登录信息，不要分享给他人
2. **有效期**：cookies 会过期，如果失效需要重新登录 YouTube
3. **浏览器选择**：使用您常用的浏览器，确保已登录 YouTube
4. **权限**：yt-dlp 需要读取浏览器数据的权限

## 相关链接

- [yt-dlp FAQ - Cookies](https://github.com/yt-dlp/yt-dlp/wiki/FAQ#how-do-i-pass-cookies-to-yt-dlp)
- [YouTube Cookies 导出指南](https://github.com/yt-dlp/yt-dlp/wiki/Extractors#exporting-youtube-cookies)
