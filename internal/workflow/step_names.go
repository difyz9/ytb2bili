package workflow

// 步骤名称常量，与各步骤 NewBaseStepWithOrder 中的名称保持一致。
// 在所有需要字符串引用步骤名的地方（重试、前端 API、测试）统一使用这里的常量。
const (
	StepNameInitialize          = "Initialize"
	StepNameResolveDouyinShare  = "ResolveDouyinShare"
	StepNameDownloadVideo       = "DownloadVideo"
	StepNameDownloadDouyinVideo = "DownloadDouyinVideo"
	StepNameDownloadThumbnail   = "DownloadThumbnail"
	StepNameExtractAudio        = "ExtractAudio"
	StepNameTranscribe          = "Transcribe"
	StepNameTranslate           = "Translate"
	StepNameLLMTranslate    = "LLMTranslate"
	StepNameDeepseekTranslate   = "DeepseekTranslate"
	StepNameSynthesizeSubtitle  = "SynthesizeSubtitleAudio"
	StepNameGenerateMetadata    = "GenerateMetadata"
	StepNameAddWatermark        = "AddWatermark"
	StepNameSaveDatabase        = "SaveDatabase"
	StepNameUploadToBilibili    = "UploadToBilibili"
)
