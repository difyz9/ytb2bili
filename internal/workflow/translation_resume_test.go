package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap/zaptest"
)

func TestRestoreTranscriptFromSavedSubtitlesPrefersEnglishSource(t *testing.T) {
	tempDir := t.TempDir()
	videoID := "JiMap1t4okA"
	videoDir := filepath.Join(tempDir, videoID)
	videoPath := filepath.Join(videoDir, videoID+".mp4")
	defaultSRTPath := filepath.Join(videoDir, videoID+".srt")
	enSRTPath := filepath.Join(videoDir, videoID+".en.srt")

	if err := os.MkdirAll(videoDir, 0o755); err != nil {
		t.Fatalf("mkdir video dir: %v", err)
	}
	if err := os.WriteFile(videoPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write video: %v", err)
	}
	if err := os.WriteFile(defaultSRTPath, []byte("1\n00:00:00,000 --> 00:00:01,000\n这是中文\n"), 0o644); err != nil {
		t.Fatalf("write default srt: %v", err)
	}
	if err := os.WriteFile(enSRTPath, []byte("1\n00:00:00,000 --> 00:00:01,000\nThis is English\n"), 0o644); err != nil {
		t.Fatalf("write en srt: %v", err)
	}

	yc := &YouTubeChain{logger: zaptest.NewLogger(t), downloadDir: tempDir}
	video := &model.Video{
		VideoID:      videoID,
		VideoPath:    videoPath,
		SubtitlePath: defaultSRTPath,
	}
	vctx := &VideoContext{VideoID: videoID, VideoPath: videoPath}

	yc.restoreTranscriptFromSavedSubtitles(video, vctx)

	if vctx.Transcript == nil {
		t.Fatal("expected transcript to be restored")
	}
	if got := vctx.Transcript.SRTPath; got != enSRTPath {
		t.Fatalf("expected english subtitle path %q, got %q", enSRTPath, got)
	}
	if !strings.Contains(vctx.Transcript.FullText, "This is English") {
		t.Fatalf("expected english transcript, got %q", vctx.Transcript.FullText)
	}
}

