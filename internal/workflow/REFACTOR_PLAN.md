# Workflow 优化规划

> 目标：保持现有接口不变，逐步修复，每批可独立验证，不引入破坏性变更。

---

## Phase 1 — 正确性修复（必须）

### Fix-1: `InitSteps` 不再重置已完成步骤

**文件**：`progress_tracker.go`

**问题**：`count > 0` 时把 **全部** 步骤重置为 `pending`，导致任务每次被 cron 捡起时已完成进度被抹掉。

**改法**：只重置非 `completed` 状态的步骤。

```go
// before
Where("video_id = ?", videoID).Updates(...)

// after
Where("video_id = ? AND status != ?", videoID, model.TaskStepStatusCompleted).Updates(...)
```

---

### Fix-2: 步骤 `Order` 冲突

**文件**：`save_database_step.go`、`llm_translate_step.go`、`translate_step.go`

**问题**：三个步骤都是 order=6，`sort.Slice` 不稳定，SaveDatabase 可能在翻译前执行。

**改法**：统一按如下序号，并在 `step.go` 用常量定义，避免散落：

```
Initialize=1, DownloadVideo=2, DownloadThumbnail=3,
ExtractAudio=4, Transcribe=5, Translate/LLMTranslate=6,
SynthesizeSubtitleAudio=7, SaveDatabase=8, UploadToBilibili=100
```

---

### Fix-3: 消除裸类型断言 panic 风险

**文件**：`save_database_step.go`、`transcribe_step.go`、`llm_translate_step.go`、`download_video_step.go` 等

**问题**：`vctx := input.(*VideoContext)` 在 input 为 nil 或类型不符时直接 panic。

**改法**：统一使用安全断言，提取辅助函数：

```go
// step.go 新增
func mustVideoContext(input any) (*VideoContext, error) {
    vctx, ok := input.(*VideoContext)
    if !ok || vctx == nil {
        return nil, fmt.Errorf("expected *VideoContext, got %T", input)
    }
    return vctx, nil
}
```

各步骤改为 `vctx, err := mustVideoContext(input); if err != nil { return nil, err }`.

---

## Phase 2 — 设计改善（应该）

### Fix-4: 步骤名提取为常量

**文件**：新建 `step_names.go`

**问题**：步骤名在步骤定义、`RetryStepByName`、前端 API 中三处用字符串字面量，改名时静默失效。

**改法**：

```go
// step_names.go
const (
    StepNameInitialize           = "Initialize"
    StepNameDownloadVideo        = "DownloadVideo"
    StepNameDownloadThumbnail    = "DownloadThumbnail"
    StepNameExtractAudio         = "ExtractAudio"
    StepNameTranscribe           = "Transcribe"
    StepNameLLMTranslate         = "LLMTranslate"
    StepNameSynthesizeSubtitle   = "SynthesizeSubtitleAudio"
    StepNameSaveDatabase         = "SaveDatabase"
    StepNameUploadToBilibili     = "UploadToBilibili"
)
```

---

### Fix-5: 提取 `defaultVideoContext()` 工厂函数

**文件**：`youtube_workflow.go`、`init_step.go`

**问题**：`TranslationConfig` / `SpeechSynthesisConfig` 默认值在 4 处重复定义，语言配置不一致更新困难。

**改法**：在 `youtube_workflow.go` 新增：

```go
func defaultVideoContext() *VideoContext {
    return &VideoContext{
        TranslationConfig:     &TranslationConfig{SourceLanguage: "en", TargetLanguage: "zh-Hans"},
        SpeechSynthesisConfig: &SpeechSynthesisConfig{Language: "zh-CN", VoiceName: "zh-CN-XiaoxiaoNeural", Format: "mp3"},
    }
}
```

---

### Fix-6: SRT 时间解析移入 `pkg/tools`

**文件**：`youtube_workflow.go` → `pkg/tools/llm_subtitle_translator.go`

**问题**：`parseSRTTimeCode` / `parseSRTTime` / `srtEntriesToTranscript` 是文本解析逻辑，属于 tools 层，现在越层放在 workflow。

**改法**：将三个函数移入 `pkg/tools`，导出（首字母大写），`youtube_workflow.go` 直接调用。

---

### Fix-7: `Chain.clone()` 改为深拷贝

**文件**：`chain.go`

**问题**：`newChain := *c` 只拷贝 struct header，`steps []Step` 共享底层数组，并发 `clone+WithSteps` 会污染原链。

**改法**：

```go
func (c *Chain) clone() *Chain {
    newChain := *c
    newChain.steps = make([]Step, len(c.steps))
    copy(newChain.steps, c.steps)
    return &newChain
}
```

---

### Fix-8: `AfterStep` 合并为单次查询

**文件**：`progress_tracker.go`

**问题**：先 `First` 取 `start_time` 计算 duration，再 `Updates`——两次 DB 往返。

**改法**：用数据库原生时间差计算，单条 SQL 完成：

```go
updates["duration"] = gorm.Expr("TIMESTAMPDIFF(MICROSECOND, start_time, NOW()) / 1000")
// 删除 First 查询
```

---

## Phase 3 — 质量收尾（建议）

### Fix-9: 步骤间检查 ctx 取消

**文件**：`chain.go` `Run()` 循环开头

```go
select {
case <-ctx.Done():
    result.Success, result.Error = false, ctx.Err()
    return result
default:
}
```

---

### Fix-10: 删除死代码 `StepResult`

**文件**：`step.go`

`StepResult` 接口无任何实现和引用，直接删除。

---

### Fix-11: 修复 `transcribe_step.go` 缩进

**文件**：`transcribe_step.go` 第 53 行

`vctx.Transcript = &transcript` 缺少 tab 缩进。

---

## 执行顺序

```
Phase 1 (Fix-1 → Fix-3)  →  go build + 手测重试流程
Phase 2 (Fix-4 → Fix-8)  →  go build + go test ./internal/workflow/...
Phase 3 (Fix-9 → Fix-11) →  go build
```

**估计改动规模**：~150 行修改，~30 行新增，~20 行删除。无接口变更，无数据库迁移。
