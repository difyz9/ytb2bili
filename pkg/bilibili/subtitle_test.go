package bilibili

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap/zaptest"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSubtitleTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		if strings.Contains(err.Error(), "requires cgo") {
			t.Skipf("skip sqlite-backed test in current environment: %v", err)
		}
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Video{}, &model.BiliSubtitleUpload{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestBuildSubtitleCandidates(t *testing.T) {
	video := model.Video{VideoID: "1xipg02Wu8s"}
	candidates := BuildSubtitleCandidates(video)

	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	if candidates[0].Filename != "1xipg02Wu8s.zh.srt" || candidates[0].Language != model.BiliSubtitleLanguageZh {
		t.Fatalf("unexpected zh candidate: %+v", candidates[0])
	}
	if candidates[1].Filename != "1xipg02Wu8s.en.srt" || candidates[1].Language != model.BiliSubtitleLanguageEn {
		t.Fatalf("unexpected en candidate: %+v", candidates[1])
	}
}

func TestSyncSubtitleUploadStatesAndAggregate(t *testing.T) {
	db := setupSubtitleTestDB(t)
	logger := zaptest.NewLogger(t)
	svc := NewService(db, logger, Options{})

	videoID := "1xipg02Wu8s"
	videoDir := t.TempDir()
	videoPath := filepath.Join(videoDir, videoID+".mp4")
	if err := os.WriteFile(videoPath, []byte("video"), 0644); err != nil {
		t.Fatalf("write video: %v", err)
	}
	if err := os.WriteFile(filepath.Join(videoDir, videoID+".zh.srt"), []byte("zh"), 0644); err != nil {
		t.Fatalf("write zh subtitle: %v", err)
	}
	if err := os.WriteFile(filepath.Join(videoDir, videoID+".en.srt"), []byte("en"), 0644); err != nil {
		t.Fatalf("write en subtitle: %v", err)
	}

	video := model.Video{
		UserID:      "user-1",
		VideoID:     videoID,
		URL:         "https://example.com/video",
		Status:      model.VideoStatusCompleted,
		VideoPath:   videoPath,
		BiliBVID:    "BV1test123",
		Platform:    "youtube",
		Title:       "test",
		Description: "test",
	}
	if err := db.Create(&video).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}

	records, err := svc.syncSubtitleUploadStates(&video)
	if err != nil {
		t.Fatalf("sync states: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 subtitle records, got %d", len(records))
	}
	for _, record := range records {
		if record.Status != model.BiliSubtitleStatusPending {
			t.Fatalf("expected pending status, got %+v", record)
		}
	}

	if err := db.First(&video, video.ID).Error; err != nil {
		t.Fatalf("reload video: %v", err)
	}
	if video.BiliSubtitleUploaded {
		t.Fatal("expected aggregate subtitle flag to remain false before uploads succeed")
	}

	for _, record := range records {
		if err := svc.markSubtitleUploadSucceeded(record.ID); err != nil {
			t.Fatalf("mark upload succeeded: %v", err)
		}
	}
	if err := svc.refreshVideoSubtitleAggregateStatus(&video); err != nil {
		t.Fatalf("refresh aggregate: %v", err)
	}
	if err := db.First(&video, video.ID).Error; err != nil {
		t.Fatalf("reload video after aggregate refresh: %v", err)
	}
	if !video.BiliSubtitleUploaded {
		t.Fatal("expected aggregate subtitle flag to become true after both uploads succeed")
	}
}


func TestMapSubtitleUploadLanguage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "simplified chinese", input: "zh-CN", expected: "zh"},
		{name: "plain zh", input: "zh", expected: "zh"},
		{name: "traditional chinese", input: "zh-TW", expected: "zh-TW"},
		{name: "us english", input: "en-US", expected: "en"},
		{name: "english", input: "en", expected: "en"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := mapSubtitleUploadLanguage(test.input); got != test.expected {
				t.Fatalf("mapSubtitleUploadLanguage(%q) = %q, want %q", test.input, got, test.expected)
			}
		})
	}
}

type stubSubtitleUploader struct {
	calls     []string
	errByLang map[string]error
}

func (s *stubSubtitleUploader) UploadSubtitle(_ string, _ string, language string) error {
	s.calls = append(s.calls, language)
	if s.errByLang == nil {
		return nil
	}
	return s.errByLang[language]
}

func TestUploadSubtitleWithFallbackRetriesInvalidLanguage(t *testing.T) {
	svc := NewService(nil, zaptest.NewLogger(t), Options{})
	uploader := &stubSubtitleUploader{
		errByLang: map[string]error{
			"zh-CN": errors.New("save subtitle info failed: code=79011, message=不合法的语言"),
		},
	}

	if err := svc.uploadSubtitleWithFallback(uploader, "BV1test123", "/tmp/test.srt", "zh-CN"); err != nil {
		t.Fatalf("expected fallback retry to succeed, got %v", err)
	}
	if len(uploader.calls) != 2 || uploader.calls[0] != "zh-CN" || uploader.calls[1] != "zh" {
		t.Fatalf("unexpected upload call sequence: %+v", uploader.calls)
	}
}

func TestUploadSubtitleWithFallbackDoesNotRetryOtherErrors(t *testing.T) {
	svc := NewService(nil, zaptest.NewLogger(t), Options{})
	wantErr := errors.New("network timeout")
	uploader := &stubSubtitleUploader{
		errByLang: map[string]error{
			"zh": wantErr,
		},
	}

	err := svc.uploadSubtitleWithFallback(uploader, "BV1test123", "/tmp/test.srt", "zh")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected original error, got %v", err)
	}
	if len(uploader.calls) != 1 || uploader.calls[0] != "zh" {
		t.Fatalf("unexpected upload call sequence: %+v", uploader.calls)
	}
}