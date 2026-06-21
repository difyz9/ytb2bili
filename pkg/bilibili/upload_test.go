package bilibili

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap/zaptest"
)

type stubCoverUploader struct {
	uploadPathArg string
	uploadURLArg  string
	pathResult    string
	urlResult     string
	pathErr       error
	urlErr        error
	pathCalls     int
	urlCalls      int
}

func (s *stubCoverUploader) UploadCover(imagePath string) (string, error) {
	s.pathCalls++
	s.uploadPathArg = imagePath
	return s.pathResult, s.pathErr
}

func (s *stubCoverUploader) UploadCoverFromURL(imageURL string) (string, error) {
	s.urlCalls++
	s.uploadURLArg = imageURL
	return s.urlResult, s.urlErr
}

func TestResolveSubmissionCoverFromLocalFile(t *testing.T) {
	svc := NewService(nil, zaptest.NewLogger(t), Options{})
	coverUploader := &stubCoverUploader{pathResult: "https://i0.hdslb.com/test-cover.jpg"}
	coverDir := t.TempDir()
	coverPath := filepath.Join(coverDir, "cover.jpg")
	if err := os.WriteFile(coverPath, []byte("cover"), 0644); err != nil {
		t.Fatalf("write cover file: %v", err)
	}

	coverURL, err := svc.resolveSubmissionCover(coverUploader, coverPath)
	if err != nil {
		t.Fatalf("resolve local cover: %v", err)
	}
	if coverURL != coverUploader.pathResult {
		t.Fatalf("expected uploaded cover URL %q, got %q", coverUploader.pathResult, coverURL)
	}
	if coverUploader.pathCalls != 1 || coverUploader.urlCalls != 0 {
		t.Fatalf("expected UploadCover only, got pathCalls=%d urlCalls=%d", coverUploader.pathCalls, coverUploader.urlCalls)
	}
	if coverUploader.uploadPathArg != coverPath {
		t.Fatalf("expected local cover path %q, got %q", coverPath, coverUploader.uploadPathArg)
	}
}

func TestResolveSubmissionCoverFromRemoteURL(t *testing.T) {
	svc := NewService(nil, zaptest.NewLogger(t), Options{})
	coverUploader := &stubCoverUploader{urlResult: "https://i0.hdslb.com/test-cover.jpg"}
	coverURLInput := "https://example.com/cover.jpg"

	coverURL, err := svc.resolveSubmissionCover(coverUploader, coverURLInput)
	if err != nil {
		t.Fatalf("resolve remote cover: %v", err)
	}
	if coverURL != coverUploader.urlResult {
		t.Fatalf("expected uploaded cover URL %q, got %q", coverUploader.urlResult, coverURL)
	}
	if coverUploader.urlCalls != 1 || coverUploader.pathCalls != 0 {
		t.Fatalf("expected UploadCoverFromURL only, got pathCalls=%d urlCalls=%d", coverUploader.pathCalls, coverUploader.urlCalls)
	}
	if coverUploader.uploadURLArg != coverURLInput {
		t.Fatalf("expected remote cover URL %q, got %q", coverURLInput, coverUploader.uploadURLArg)
	}
}

func TestResolveSubmissionCoverMissingLocalFile(t *testing.T) {
	svc := NewService(nil, zaptest.NewLogger(t), Options{})
	coverUploader := &stubCoverUploader{}

	_, err := svc.resolveSubmissionCover(coverUploader, filepath.Join(t.TempDir(), "missing.jpg"))
	if err == nil {
		t.Fatal("expected missing local cover to return an error")
	}
	if coverUploader.pathCalls != 0 || coverUploader.urlCalls != 0 {
		t.Fatalf("expected no uploader calls for missing file, got pathCalls=%d urlCalls=%d", coverUploader.pathCalls, coverUploader.urlCalls)
	}
}

func TestResolveSubmissionCoverRemoteUploadError(t *testing.T) {
	svc := NewService(nil, zaptest.NewLogger(t), Options{})
	coverUploader := &stubCoverUploader{urlErr: errors.New("boom")}

	_, err := svc.resolveSubmissionCover(coverUploader, "https://example.com/cover.jpg")
	if err == nil {
		t.Fatal("expected remote upload error")
	}
	if coverUploader.urlCalls != 1 {
		t.Fatalf("expected one UploadCoverFromURL call, got %d", coverUploader.urlCalls)
	}
}