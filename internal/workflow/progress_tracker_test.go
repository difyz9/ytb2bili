package workflow

import (
	"context"
	"testing"

	storemodel "github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type trackerTestStep struct {
	BaseStep
}

func (s trackerTestStep) Execute(ctx context.Context, input any) (any, error) {
	return input, nil
}

func TestProgressTrackerInitStepsPreservesSkippedStatus(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&storemodel.TaskStep{}); err != nil {
		t.Fatalf("migrate task steps: %v", err)
	}

	videoID := "video-123"
	seed := []storemodel.TaskStep{
		{VideoID: videoID, StepName: StepNameDownloadThumbnail, StepOrder: 3, Status: storemodel.TaskStepStatusSkipped},
		{VideoID: videoID, StepName: StepNameSynthesizeSubtitle, StepOrder: 7, Status: storemodel.TaskStepStatusFailed, ErrorMsg: "boom"},
	}
	if err := db.Create(&seed).Error; err != nil {
		t.Fatalf("seed task steps: %v", err)
	}

	tracker := NewProgressTracker(db, zap.NewNop())
	steps := []Step{
		trackerTestStep{BaseStep: NewBaseStepWithOrder(StepNameDownloadThumbnail, false, 3)},
		trackerTestStep{BaseStep: NewBaseStepWithOrder(StepNameSynthesizeSubtitle, false, 7)},
	}

	if err := tracker.InitSteps(videoID, steps); err != nil {
		t.Fatalf("init steps: %v", err)
	}

	var thumbnailStep storemodel.TaskStep
	if err := db.Where("video_id = ? AND step_name = ?", videoID, StepNameDownloadThumbnail).First(&thumbnailStep).Error; err != nil {
		t.Fatalf("load thumbnail step: %v", err)
	}
	if thumbnailStep.Status != storemodel.TaskStepStatusSkipped {
		t.Fatalf("expected thumbnail step to stay skipped, got %q", thumbnailStep.Status)
	}

	var synthStep storemodel.TaskStep
	if err := db.Where("video_id = ? AND step_name = ?", videoID, StepNameSynthesizeSubtitle).First(&synthStep).Error; err != nil {
		t.Fatalf("load synth step: %v", err)
	}
	if synthStep.Status != storemodel.TaskStepStatusPending {
		t.Fatalf("expected synth step to reset to pending, got %q", synthStep.Status)
	}
	if synthStep.ErrorMsg != "" {
		t.Fatalf("expected synth step error to be cleared, got %q", synthStep.ErrorMsg)
	}
}