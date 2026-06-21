package workflow

import (
	"strings"

	"github.com/zolagz/ytb2bili/pkg/tools"
)

type transcriptTextSegment struct {
	Text  string
	Start float64
	End   float64
}

func collectTranscriptTextSegments(transcript *tools.TranscriptResult) []transcriptTextSegment {
	if transcript == nil || len(transcript.Segments) == 0 {
		return nil
	}

	segments := make([]transcriptTextSegment, 0, len(transcript.Segments))
	for _, segment := range transcript.Segments {
		text := strings.TrimSpace(segment.Text)
		if text == "" {
			continue
		}
		segments = append(segments, transcriptTextSegment{
			Text:  text,
			Start: segment.Start,
			End:   segment.End,
		})
	}
	return segments
}

func transcriptTexts(segments []transcriptTextSegment) []string {
	texts := make([]string, 0, len(segments))
	for _, segment := range segments {
		texts = append(texts, segment.Text)
	}
	return texts
}

func buildSubtitleAudiosFromTranslations(segments []transcriptTextSegment, translatedTexts []string) []SubtitleAudio {
	count := len(segments)
	if len(translatedTexts) < count {
		count = len(translatedTexts)
	}

	subtitles := make([]SubtitleAudio, 0, count)
	for index := 0; index < count; index++ {
		segment := segments[index]
		subtitles = append(subtitles, SubtitleAudio{
			OriginalText:   segment.Text,
			TranslatedText: translatedTexts[index],
			StartTime:      segment.Start,
			EndTime:        segment.End,
		})
	}
	return subtitles
}

func buildSubtitleAudiosFromTranscript(segments []transcriptTextSegment) []SubtitleAudio {
	subtitles := make([]SubtitleAudio, 0, len(segments))
	for _, segment := range segments {
		subtitles = append(subtitles, SubtitleAudio{
			OriginalText:   segment.Text,
			TranslatedText: segment.Text,
			StartTime:      segment.Start,
			EndTime:        segment.End,
		})
	}
	return subtitles
}

func totalSubtitleRuneCount(subtitles []SubtitleAudio) int64 {
	var total int64
	for _, subtitle := range subtitles {
		total += int64(len([]rune(subtitle.TranslatedText)))
	}
	return total
}