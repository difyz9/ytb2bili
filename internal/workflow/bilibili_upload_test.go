package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	biliaccount "github.com/zolagz/ytb2bili/pkg/bilibili"
	"github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// 从环境变量读取测试凭据，避免硬编码敏感信息
//
// 设置方式（终端执行）：
//
//	export BILI_TEST_USER_ID="your_user_id"
//	export BILI_TEST_VIDEO_PATH="/absolute/path/to/video.mp4"         # 本地视频文件
//	export BILI_TEST_COOKIES='{"SESSDATA":"xxx","bili_jct":"xxx"}'    # B站 cookies JSON
//	export BILI_TEST_ACCESS_TOKEN="your_access_token"
//	export BILI_TEST_REFRESH_TOKEN="your_refresh_token"
//	export BILI_TEST_BILI_MID=123456789                               # B站 MID（数字）
//
// 运行：
//
//	go test -v -run TestUploadToBilibili -timeout 300s ./internal/workflow/

// setupTestDB 创建内存 SQLite 数据库并自动迁移测试所需的表
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		if strings.Contains(err.Error(), "requires cgo") {
			t.Skipf("跳过：SQLite 驱动需要 cgo（当前环境可能为 CGO_ENABLED=0）：%v", err)
		}
		t.Fatalf("创建测试数据库失败: %v", err)
	}
	if err := db.AutoMigrate(&model.AccountBinding{}); err != nil {
		t.Fatalf("迁移表失败: %v", err)
	}
	return db
}

// insertEncryptedBinding 向测试数据库插入加密后的B站绑定记录
func insertEncryptedBinding(t *testing.T, svc *biliaccount.Service, db *gorm.DB, logger *zap.Logger) {
	t.Helper()

	userID := os.Getenv("BILI_TEST_USER_ID")
	cookies := os.Getenv("BILI_TEST_COOKIES")
	accessToken := os.Getenv("BILI_TEST_ACCESS_TOKEN")
	refreshToken := os.Getenv("BILI_TEST_REFRESH_TOKEN")
	var biliMid int64
	fmt.Sscanf(os.Getenv("BILI_TEST_BILI_MID"), "%d", &biliMid)

	if userID == "" || cookies == "" || accessToken == "" {
		t.Skip("跳过：未设置 BILI_TEST_USER_ID / BILI_TEST_COOKIES / BILI_TEST_ACCESS_TOKEN 环境变量")
	}
	if biliMid == 0 {
		t.Skip("跳过：未设置 BILI_TEST_BILI_MID 环境变量（或值为0）")
	}

	// 通过 service 加密后写入数据库，确保与解密路径一致
	encCookies, err := svc.Encrypt(cookies)
	if err != nil {
		t.Fatalf("加密 cookies 失败: %v", err)
	}
	encAccess, err := svc.Encrypt(accessToken)
	if err != nil {
		t.Fatalf("加密 access_token 失败: %v", err)
	}
	encRefresh, err := svc.Encrypt(refreshToken)
	if err != nil {
		t.Fatalf("加密 refresh_token 失败: %v", err)
	}

	platformUID := fmt.Sprintf("%d", biliMid)
	platformData := &model.BiliPlatformData{BiliMid: biliMid}
	binding := &model.AccountBinding{
		UserID:       userID,
		Platform:     model.PlatformBilibili,
		PlatformUID:  platformUID,
		Username:     "test_user",
		Cookies:      encCookies,
		AccessToken:  encAccess,
		RefreshToken: encRefresh,
		Status:       model.BindingStatusBound,
		IsPrimary:    true,
	}
	if err := binding.SetBiliData(platformData); err != nil {
		t.Fatalf("SetBiliData 失败: %v", err)
	}
	if err := db.Create(binding).Error; err != nil {
		t.Fatalf("插入测试账号绑定失败: %v", err)
	}
	logger.Info("✅ 测试账号绑定已写入内存数据库",
		zap.String("user_id", userID),
		zap.Int64("bili_mid", biliMid))
}

// TestUploadToBilibili 对上传步骤进行集成测试
//
// 该测试直接调用 UploadToBilibiliStep.Execute，不依赖 HTTP 层或 FX 容器。
// 需要真实的 B站账号凭据和本地视频文件。
func TestUploadToBilibili(t *testing.T) {
	videoPath := os.Getenv("BILI_TEST_VIDEO_PATH")
	userID := os.Getenv("BILI_TEST_USER_ID")

	if videoPath == "" {
		t.Skip("跳过：未设置 BILI_TEST_VIDEO_PATH 环境变量")
	}
	if userID == "" {
		t.Skip("跳过：未设置 BILI_TEST_USER_ID 环境变量")
	}
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		t.Skipf("跳过：视频文件不存在: %s", videoPath)
	}

	logger := zaptest.NewLogger(t)
	db := setupTestDB(t)

	svc := biliaccount.NewService(db, logger, biliaccount.Options{})
	insertEncryptedBinding(t, svc, db, logger)

	step := NewUploadToBilibiliStep(svc, nil, db, logger)

	vctx := &VideoContext{
		VideoID:   "test_video",
		VideoPath: videoPath,
		VideoURL:  "https://www.youtube.com/watch?v=test",
		Title:     "测试上传视频标题",
		Tags:      "测试,上传,Bilibili",
	}

	ctx := context.WithValue(context.Background(), "user_id", userID)

	t.Log("开始执行上传步骤...")
	output, err := step.Execute(ctx, vctx)
	if err != nil {
		t.Fatalf("上传失败: %v", err)
	}

	result, ok := output.(*VideoContext)
	if !ok {
		t.Fatalf("返回类型错误: %T", output)
	}

	if result.BiliBVID == "" {
		t.Error("上传成功但 BVID 为空")
	}

	t.Logf("✅ 上传成功！BVID=%s, AID=%d", result.BiliBVID, result.BiliAID)
	t.Logf("🔗 视频链接: https://www.bilibili.com/video/%s", result.BiliBVID)
}

// TestUploadToBilibili_Chain 通过 BilibiliChain.RunFromVideoPath 测试完整工作流
//
// 在 TestUploadToBilibili 的基础上额外验证元数据生成（无 LLM 时自动跳过）。
func TestUploadToBilibili_Chain(t *testing.T) {
	videoPath := os.Getenv("BILI_TEST_VIDEO_PATH")
	userID := os.Getenv("BILI_TEST_USER_ID")
	videoURL := os.Getenv("BILI_TEST_VIDEO_URL") // 可选，原始 YouTube 链接

	if videoPath == "" {
		t.Skip("跳过：未设置 BILI_TEST_VIDEO_PATH 环境变量")
	}
	if userID == "" {
		t.Skip("跳过：未设置 BILI_TEST_USER_ID 环境变量")
	}
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		t.Skipf("跳过：视频文件不存在: %s", videoPath)
	}

	logger := zaptest.NewLogger(t)
	db := setupTestDB(t)

	svc := biliaccount.NewService(db, logger, biliaccount.Options{})
	insertEncryptedBinding(t, svc, db, logger)

	uploadStep := NewUploadToBilibiliStep(svc, nil, db, logger)
	// 不注入 LLM 客户端 —— 元数据生成步骤会跳过，直接上传
	metadataStep := NewGenerateMetadataStep(nil, nil, svc, db, logger)

	chain := &BilibiliChain{
		metadataStep: metadataStep,
		uploadStep:   uploadStep,
		logger:       logger,
	}

	t.Log("开始执行 BilibiliChain.RunFromVideoPath...")
	bctx, err := chain.RunFromVideoPath(context.Background(), userID, videoPath, videoURL, nil)
	if err != nil {
		t.Fatalf("Chain 上传失败: %v", err)
	}

	if bctx.BiliBVID == "" {
		t.Error("Chain 上传成功但 BVID 为空")
	}

	t.Logf("✅ Chain 上传成功！BVID=%s, AID=%d", bctx.BiliBVID, bctx.BiliAID)
	t.Logf("🔗 视频链接: https://www.bilibili.com/video/%s", bctx.BiliBVID)
}

// TestUploadToBilibili_MissingAccount 验证：无账号绑定时应返回明确错误，不引发 panic
func TestUploadToBilibili_MissingAccount(t *testing.T) {
	logger := zaptest.NewLogger(t)
	db := setupTestDB(t) // 空库，无任何绑定记录

	svc := biliaccount.NewService(db, logger, biliaccount.Options{})
	step := NewUploadToBilibiliStep(svc, nil, db, logger)

	vctx := &VideoContext{
		VideoID:   "test_video",
		VideoPath: "/tmp/fake.mp4",
	}

	ctx := context.WithValue(context.Background(), "user_id", "no_such_user")
	_, err := step.Execute(ctx, vctx)
	if err == nil {
		t.Fatal("预期应返回错误，但得到 nil")
	}
	t.Logf("✅ 正确返回错误: %v", err)
}

// TestUploadToBilibili_MissingUserID 验证：context 中无 user_id 时应返回明确错误
func TestUploadToBilibili_MissingUserID(t *testing.T) {
	logger := zaptest.NewLogger(t)
	db := setupTestDB(t)

	svc := biliaccount.NewService(db, logger, biliaccount.Options{})
	step := NewUploadToBilibiliStep(svc, nil, db, logger)

	vctx := &VideoContext{VideoID: "test_video", VideoPath: "/tmp/fake.mp4"}
	_, err := step.Execute(context.Background(), vctx) // 故意不注入 user_id
	if err == nil {
		t.Fatal("预期应返回错误，但得到 nil")
	}
	t.Logf("✅ 正确返回错误: %v", err)
}

func TestResolveSubmissionCoverPathPrefersSiblingThumbnail(t *testing.T) {
	logger := zaptest.NewLogger(t)
	step := &UploadToBilibiliStep{logger: logger}
	videoDir := t.TempDir()
	videoPath := filepath.Join(videoDir, "video.mp4")
	preferredCoverPath := filepath.Join(videoDir, preferredBilibiliCoverFilename)
	if err := os.WriteFile(videoPath, []byte("video"), 0644); err != nil {
		t.Fatalf("write video file: %v", err)
	}
	if err := os.WriteFile(preferredCoverPath, []byte("cover"), 0644); err != nil {
		t.Fatalf("write cover file: %v", err)
	}

	got := step.resolveSubmissionCoverPath(&VideoContext{
		VideoPath:     videoPath,
		ThumbnailPath: "https://example.com/remote-cover.jpg",
	})
	if got != preferredCoverPath {
		t.Fatalf("expected sibling cover %q, got %q", preferredCoverPath, got)
	}
}

func TestResolveSubmissionCoverPathFallsBackToThumbnailPath(t *testing.T) {
	logger := zaptest.NewLogger(t)
	step := &UploadToBilibiliStep{logger: logger}
	videoDir := t.TempDir()
	videoPath := filepath.Join(videoDir, "video.mp4")
	if err := os.WriteFile(videoPath, []byte("video"), 0644); err != nil {
		t.Fatalf("write video file: %v", err)
	}

	thumbnailPath := "https://example.com/remote-cover.jpg"
	got := step.resolveSubmissionCoverPath(&VideoContext{
		VideoPath:     videoPath,
		ThumbnailPath: thumbnailPath,
	})
	if got != thumbnailPath {
		t.Fatalf("expected thumbnail fallback %q, got %q", thumbnailPath, got)
	}
}

func TestResolveSubmissionCopyrightDefaultsToReprint(t *testing.T) {
	step := &UploadToBilibiliStep{logger: zaptest.NewLogger(t)}

	if got := step.resolveSubmissionCopyright(context.Background(), ""); got != model.DefaultBilibiliSubmissionCopyright {
		t.Fatalf("expected default copyright %d, got %d", model.DefaultBilibiliSubmissionCopyright, got)
	}
}

// TestEncryptDecryptRoundtrip 验证加密/解密对是否一致（服务层回归测试）
func TestEncryptDecryptRoundtrip(t *testing.T) {
	logger := zaptest.NewLogger(t)
	db := setupTestDB(t)
	svc := biliaccount.NewService(db, logger, biliaccount.Options{})

	plaintexts := []string{
		`{"SESSDATA":"abc123","bili_jct":"def456","DedeUserID":"789"}`,
		"access_token_value",
		"refresh_token_value",
		"",
	}

	for _, plain := range plaintexts {
		enc, err := svc.Encrypt(plain)
		if err != nil {
			t.Errorf("加密失败 (input=%q): %v", plain, err)
			continue
		}
		// 通过构造一个临时 AccountBinding 来走 GetDecryptedCookies 解密路径
		binding := &model.AccountBinding{Cookies: enc}
		got, err := svc.GetDecryptedCookies(binding)
		if err != nil {
			t.Errorf("解密失败 (input=%q): %v", plain, err)
			continue
		}
		if got != plain {
			t.Errorf("加解密结果不一致: want %q, got %q", plain, got)
		}
	}
	t.Log("✅ 加密/解密往返测试通过")
}
