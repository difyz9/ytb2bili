package workflow

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// ============================================================================
// 测试步骤
// ============================================================================

type testStep struct {
	BaseStep
	shouldFail bool
	executed   *bool
}

func (s *testStep) Execute(ctx context.Context, input any) (any, error) {
	*s.executed = true
	if s.shouldFail {
		return nil, errors.New("test error")
	}
	return input, nil
}

type testStepWithSkip struct {
	BaseStep
	skip bool
}

func (s *testStepWithSkip) Execute(ctx context.Context, input any) (any, error) {
	return input, nil
}

func (s *testStepWithSkip) ShouldSkip(ctx context.Context, input any) bool {
	return s.skip
}

type testStepWithHooks struct {
	BaseStep
	onSuccessCalled *bool
	onErrorCalled   *bool
}

func (s *testStepWithHooks) Execute(ctx context.Context, input any) (any, error) {
	return input, errors.New("test error")
}

func (s *testStepWithHooks) OnSuccess(ctx context.Context, output any) error {
	*s.onSuccessCalled = true
	return nil
}

func (s *testStepWithHooks) OnError(ctx context.Context, err error) error {
	*s.onErrorCalled = true
	return nil
}

type testSkippedErrorStep struct {
	BaseStep
	executed      *bool
	onErrorCalled *bool
}

func (s *testSkippedErrorStep) Execute(ctx context.Context, input any) (any, error) {
	*s.executed = true
	return input, &StepSkippedError{
		Step:   s.Name(),
		Cause:  errors.New("non-fatal error"),
		Output: input,
	}
}

func (s *testSkippedErrorStep) OnError(ctx context.Context, err error) error {
	*s.onErrorCalled = true
	return nil
}

// ============================================================================
// 测试用例
// ============================================================================

func TestChain_BasicExecution(t *testing.T) {
	executed1 := false
	executed2 := false

	step1 := &testStep{
		BaseStep: NewBaseStep("step1", true),
		executed: &executed1,
	}
	step2 := &testStep{
		BaseStep: NewBaseStep("step2", true),
		executed: &executed2,
	}

	chain := &Chain{
		name:   "test",
		steps:  []Step{step1, step2},
		logger: zaptest.NewLogger(t),
	}

	ctx := context.Background()
	result := chain.Run(ctx, "input")

	if !result.Success {
		t.Errorf("Expected success, got error: %v", result.Error)
	}
	if !executed1 || !executed2 {
		t.Error("Not all steps were executed")
	}
	if result.ExecutedSteps != 2 {
		t.Errorf("Expected 2 executed steps, got %d", result.ExecutedSteps)
	}
}

func TestChain_RequiredStepFailure(t *testing.T) {
	executed1 := false
	executed2 := false

	step1 := &testStep{
		BaseStep:   NewBaseStep("step1", true),
		shouldFail: true,
		executed:   &executed1,
	}
	step2 := &testStep{
		BaseStep: NewBaseStep("step2", true),
		executed: &executed2,
	}

	chain := &Chain{
		name:   "test",
		steps:  []Step{step1, step2},
		logger: zaptest.NewLogger(t),
	}

	ctx := context.Background()
	result := chain.Run(ctx, "input")

	if result.Success {
		t.Error("Expected failure but got success")
	}
	if !executed1 {
		t.Error("First step should have been executed")
	}
	if executed2 {
		t.Error("Second step should not have been executed after required step failure")
	}
	if result.FailedSteps != 1 {
		t.Errorf("Expected 1 failed step, got %d", result.FailedSteps)
	}
}

func TestChain_OptionalStepFailure(t *testing.T) {
	executed1 := false
	executed2 := false

	step1 := &testStep{
		BaseStep:   NewBaseStep("step1", false), // optional
		shouldFail: true,
		executed:   &executed1,
	}
	step2 := &testStep{
		BaseStep: NewBaseStep("step2", true),
		executed: &executed2,
	}

	chain := &Chain{
		name:   "test",
		steps:  []Step{step1, step2},
		logger: zaptest.NewLogger(t),
	}

	ctx := context.Background()
	result := chain.Run(ctx, "input")

	if !result.Success {
		t.Errorf("Expected success despite optional failure, got: %v", result.Error)
	}
	if !executed1 || !executed2 {
		t.Error("All steps should have been attempted")
	}
	if result.FailedSteps != 1 {
		t.Errorf("Expected 1 failed step, got %d", result.FailedSteps)
	}
}

func TestChain_SkipStep(t *testing.T) {
	step1 := &testStepWithSkip{
		BaseStep: NewBaseStep("step1", false),
		skip:     true,
	}

	chain := &Chain{
		name:   "test",
		steps:  []Step{step1},
		logger: zaptest.NewLogger(t),
	}

	ctx := context.Background()
	result := chain.Run(ctx, "input")

	if !result.Success {
		t.Errorf("Expected success, got: %v", result.Error)
	}
	if result.SkippedSteps != 1 {
		t.Errorf("Expected 1 skipped step, got %d", result.SkippedSteps)
	}
	if result.ExecutedSteps != 0 {
		t.Errorf("Expected 0 executed steps, got %d", result.ExecutedSteps)
	}
}

func TestChain_NonFatalStepErrorMarkedSkipped(t *testing.T) {
	executed1 := false
	executed2 := false
	onErrorCalled := false

	step1 := &testSkippedErrorStep{
		BaseStep:      NewBaseStep("step1", false),
		executed:      &executed1,
		onErrorCalled: &onErrorCalled,
	}
	step2 := &testStep{
		BaseStep: NewBaseStep("step2", true),
		executed: &executed2,
	}

	chain := &Chain{
		name:   "test",
		steps:  []Step{step1, step2},
		logger: zaptest.NewLogger(t),
	}

	result := chain.Run(context.Background(), "input")

	if !result.Success {
		t.Fatalf("expected success, got error: %v", result.Error)
	}
	if !executed1 || !executed2 {
		t.Fatal("expected both steps to be attempted")
	}
	if !onErrorCalled {
		t.Fatal("expected error hook to be called for skipped error")
	}
	if result.SkippedSteps != 1 {
		t.Fatalf("expected 1 skipped step, got %d", result.SkippedSteps)
	}
	if result.FailedSteps != 0 {
		t.Fatalf("expected 0 failed steps, got %d", result.FailedSteps)
	}
	detail := result.StepDetails["step1"]
	if detail == nil || !detail.Skipped {
		t.Fatal("expected step1 detail to be marked skipped")
	}
}

func TestOptionalToolStepsUseSkipOnError(t *testing.T) {
	thumbnailStep := NewDownloadThumbnailStep(DownloadThumbnailStepParams{Logger: zaptest.NewLogger(t)})
	if !thumbnailStep.skipOnError {
		t.Fatal("expected download thumbnail step to skip on error")
	}

	transcribeStep := NewTranscribeStep(TranscribeStepParams{Logger: zaptest.NewLogger(t)})
	if !transcribeStep.skipOnError {
		t.Fatal("expected transcribe step to skip on error")
	}
}

func TestChain_RestartFromStepSkipsEarlierSteps(t *testing.T) {
	executed1 := false
	executed2 := false
	executed3 := false

	step1 := &testStep{
		BaseStep: NewBaseStepWithOrder("step1", true, 1),
		executed: &executed1,
	}
	step2 := &testStep{
		BaseStep: NewBaseStepWithOrder("step2", true, 2),
		executed: &executed2,
	}
	step3 := &testStep{
		BaseStep: NewBaseStepWithOrder("step3", true, 3),
		executed: &executed3,
	}

	chain := &Chain{
		name:   "test",
		steps:  []Step{step1, step2, step3},
		logger: zaptest.NewLogger(t),
	}

	vctx := &VideoContext{RestartFromStep: "step2"}
	result := chain.Run(context.Background(), vctx)

	if !result.Success {
		t.Fatalf("expected success, got error: %v", result.Error)
	}
	if executed1 {
		t.Fatal("expected step1 to be skipped before restart target")
	}
	if !executed2 || !executed3 {
		t.Fatal("expected target step and following steps to execute")
	}
	if result.SkippedSteps != 1 {
		t.Fatalf("expected 1 skipped step, got %d", result.SkippedSteps)
	}
	if result.ExecutedSteps != 2 {
		t.Fatalf("expected 2 executed steps, got %d", result.ExecutedSteps)
	}
	if !vctx.restartStepActivated {
		t.Fatal("expected restart target to be activated once reached")
	}
}

func TestExtractAudioStep_ShouldSkip_WhenTranscribeDisabled(t *testing.T) {
	step := NewExtractAudioStep(ExtractAudioStepParams{
		Logger: zaptest.NewLogger(t),
	})

	if !step.ShouldSkip(context.Background(), &VideoContext{
		TaskChainSettings: &TaskChainSettings{Transcribe: false},
	}) {
		t.Fatal("expected extract audio step to skip when transcribe is disabled")
	}

	if step.ShouldSkip(context.Background(), &VideoContext{
		TaskChainSettings: &TaskChainSettings{Transcribe: true},
	}) {
		t.Fatal("expected extract audio step to run when transcribe is enabled")
	}
}

func TestChain_Hooks(t *testing.T) {
	successCalled := false
	errorCalled := false

	step := &testStepWithHooks{
		BaseStep:        NewBaseStep("step1", false),
		onSuccessCalled: &successCalled,
		onErrorCalled:   &errorCalled,
	}

	chain := &Chain{
		name:   "test",
		steps:  []Step{step},
		logger: zaptest.NewLogger(t),
	}

	ctx := context.Background()
	chain.Run(ctx, "input")

	if !errorCalled {
		t.Error("OnError hook was not called")
	}
	if successCalled {
		t.Error("OnSuccess hook should not have been called")
	}
}

// ============================================================================
// fx 集成测试
// ============================================================================

func TestFxIntegration(t *testing.T) {
	executed1 := false
	executed2 := false

	// 定义步骤构造函数
	newStep1 := func() *testStep {
		return &testStep{
			BaseStep: NewBaseStep("step1", true),
			executed: &executed1,
		}
	}

	newStep2 := func() *testStep {
		return &testStep{
			BaseStep: NewBaseStep("step2", true),
			executed: &executed2,
		}
	}

	// 创建测试应用
	app := fxtest.New(t,
		fx.Provide(func() *zap.Logger { return zaptest.NewLogger(t) }),

		// 注册步骤
		fx.Provide(AsStep(newStep1)),
		fx.Provide(AsStep(newStep2)),

		// 创建链
		fx.Provide(NewChain),

		// 执行测试
		fx.Invoke(func(chain *Chain) {
			ctx := context.Background()
			result := chain.Run(ctx, "test input")

			if !result.Success {
				t.Errorf("Chain failed: %v", result.Error)
			}
			if !executed1 || !executed2 {
				t.Error("Not all steps were executed")
			}
		}),
	)

	app.RequireStart()
	app.RequireStop()
}

func TestFxModule(t *testing.T) {
	// 定义一个测试模块
	testModule := fx.Module("test_workflow",
		fx.Provide(AsStep(func() *testStep {
			executed := false
			return &testStep{
				BaseStep: NewBaseStep("test_step", true),
				executed: &executed,
			}
		})),
		fx.Provide(NewChain),
	)

	app := fxtest.New(t,
		fx.Provide(func() *zap.Logger { return zaptest.NewLogger(t) }),
		testModule,

		fx.Invoke(func(chain *Chain) {
			if len(chain.steps) == 0 {
				t.Error("Expected at least one step")
			}
		}),
	)

	app.RequireStart()
	app.RequireStop()
}

