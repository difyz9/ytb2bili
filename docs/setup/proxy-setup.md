# Proxy Connection Error Fix

## Problem
The application was failing with **WinError 10061: Connection refused** when trying to download YouTube videos because:
- Proxy was configured in `config.toml` with `use_proxy = true` and `proxy_host = "http://127.0.0.1:7890"`
- The proxy service at port 7890 was not running or not accessible

## Solution Implemented
Added automatic proxy fallback mechanism to gracefully handle proxy connection failures:

### Changes Made

#### 1. `down_load_video.go` (Video Download Handler)
- **Version updated**: `with-cookies-support-v2` → `with-cookies-support-v3`
- **New logic**: 
  - First attempts download with proxy (if configured)
  - If proxy fails, automatically retries without proxy
  - Logs clear messages about which method is being used

#### 2. `get_srt_by_url.go` (Subtitle Handler)
- Added proxy support with automatic fallback
- Implemented `createHTTPClient()` method for consistent proxy handling
- Both subtitle URL fetching and content downloading now support proxy with fallback

### How It Works
```
1. Check if proxy is configured (use_proxy = true && proxy_host != "")
2. If yes:
   a. Try with proxy
   b. If fails → Log warning and retry without proxy
3. If no proxy configured:
   - Proceed directly without proxy
```

## User Options

### Option 1: Fix Your Proxy (Recommended if you need it)
Ensure your proxy service (Clash, V2Ray, etc.) is running on port 7890:
```bash
# Check if proxy is running
netstat -ano | findstr :7890
```

### Option 2: Disable Proxy in Config
Edit `config.toml`:
```toml
[ProxyConfig]
  use_proxy = false
  proxy_host = "http://127.0.0.1:7890"
```

### Option 3: Use the Automatic Fallback (Already Implemented)
The application will now automatically:
- Try with proxy first (if configured)
- Fall back to direct connection if proxy fails
- Continue working without interruption

## Testing
Rebuild and restart the application:
```bash
go build -o ytb2bili.exe .
./ytb2bili.exe
```

The download task should now succeed even if the proxy is unavailable.

## Logs to Expect
With proxy configured but unavailable:
```
🔄 尝试使用代理下载...
📡 使用代理: http://127.0.0.1:7890
❌ 视频下载失败: exit status 1
⚠️ 代理下载失败，尝试不使用代理重试...
🔄 尝试不使用代理下载...
🌐 不使用代理
✓ 视频下载成功: ...
```
