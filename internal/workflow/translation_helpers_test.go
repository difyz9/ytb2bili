package workflow

import "testing"

func TestBuildSubtitleAudiosFromTranscript(t *testing.T) {
	segments := []transcriptTextSegment{
		{Text: "Hello", Start: 0, End: 1.2},
		{Text: "World", Start: 1.2, End: 2.4},
	}

	subtitles := buildSubtitleAudiosFromTranscript(segments)
	if len(subtitles) != 2 {
		t.Fatalf("expected 2 subtitle audios, got %d", len(subtitles))
	}
	if subtitles[0].OriginalText != "Hello" || subtitles[0].TranslatedText != "Hello" {
		t.Fatalf("expected first subtitle to reuse original transcript text, got %+v", subtitles[0])
	}
	if subtitles[1].StartTime != 1.2 || subtitles[1].EndTime != 2.4 {
		t.Fatalf("expected transcript timestamps to be preserved, got %+v", subtitles[1])
	}
}