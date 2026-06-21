package analytics

// 事件名称常量
const (
	// 用户事件
	EventUserLogin    = "user_login"
	EventUserLogout   = "user_logout"
	EventUserRegister = "user_register"
	
	// 视频处理事件
	EventVideoDownloadStart    = "video_download_start"
	EventVideoDownloadComplete = "video_download_complete"
	EventVideoDownloadFailed   = "video_download_failed"
	EventVideoUploadStart      = "video_upload_start"
	EventVideoUploadComplete   = "video_upload_complete"
	EventVideoUploadFailed     = "video_upload_failed"
	EventVideoProcessStart     = "video_process_start"
	EventVideoProcessComplete  = "video_process_complete"
	EventVideoProcessFailed    = "video_process_failed"
	
	// 字幕事件
	EventSubtitleExtract   = "subtitle_extract"
	EventSubtitleTranslate = "subtitle_translate"
	EventSubtitleGenerate  = "subtitle_generate"
	
	// TTS事件
	EventTTSSynthesizeStart    = "tts_synthesize_start"
	EventTTSSynthesizeComplete = "tts_synthesize_complete"
	EventTTSSynthesizeFailed   = "tts_synthesize_failed"
	
	// 任务事件
	EventTaskCreate   = "task_create"
	EventTaskStart    = "task_start"
	EventTaskComplete = "task_complete"
	EventTaskFailed   = "task_failed"
	
	// 订阅事件
	EventTbSubscriptionCreate = "TbSubscription_create"
	EventTbSubscriptionUpdate = "TbSubscription_update"
	EventTbSubscriptionCancel = "TbSubscription_cancel"
	
	// 支付事件
	EventPaymentStart    = "payment_start"
	EventPaymentComplete = "payment_complete"
	EventPaymentFailed   = "payment_failed"

	// B站关键成功事件
	EventBilibiliAccountBindingSuccess = "bilibili_account_binding_success"
	EventBilibiliVideoUploadSuccess    = "bilibili_video_upload_success"
	
	// 错误事件
	EventError = "error"
	EventPanic = "panic"
)

// TrackVideoDownload 记录视频下载事件
func (c *Client) TrackVideoDownload(videoID string, status string, properties map[string]interface{}) {
	if properties == nil {
		properties = make(map[string]interface{})
	}
	properties["video_id"] = videoID
	properties["status"] = status
	
	var eventName string
	switch status {
	case "start":
		eventName = EventVideoDownloadStart
	case "complete":
		eventName = EventVideoDownloadComplete
	case "failed":
		eventName = EventVideoDownloadFailed
	default:
		eventName = "video_download_" + status
	}
	
	c.Track(eventName, properties)
}

// TrackVideoUpload 记录视频上传事件
func (c *Client) TrackVideoUpload(videoID string, status string, properties map[string]interface{}) {
	if properties == nil {
		properties = make(map[string]interface{})
	}
	properties["video_id"] = videoID
	properties["status"] = status
	
	var eventName string
	switch status {
	case "start":
		eventName = EventVideoUploadStart
	case "complete":
		eventName = EventVideoUploadComplete
	case "failed":
		eventName = EventVideoUploadFailed
	default:
		eventName = "video_upload_" + status
	}
	
	c.Track(eventName, properties)
}

// TrackVideoProcess 记录视频处理事件
func (c *Client) TrackVideoProcess(videoID string, status string, properties map[string]interface{}) {
	if properties == nil {
		properties = make(map[string]interface{})
	}
	properties["video_id"] = videoID
	properties["status"] = status
	
	var eventName string
	switch status {
	case "start":
		eventName = EventVideoProcessStart
	case "complete":
		eventName = EventVideoProcessComplete
	case "failed":
		eventName = EventVideoProcessFailed
	default:
		eventName = "video_process_" + status
	}
	
	c.Track(eventName, properties)
}

// TrackSubtitle 记录字幕相关事件
func (c *Client) TrackSubtitle(action string, properties map[string]interface{}) {
	if properties == nil {
		properties = make(map[string]interface{})
	}
	
	var eventName string
	switch action {
	case "extract":
		eventName = EventSubtitleExtract
	case "translate":
		eventName = EventSubtitleTranslate
	case "generate":
		eventName = EventSubtitleGenerate
	default:
		eventName = "subtitle_" + action
	}
	
	c.Track(eventName, properties)
}

// TrackTTS 记录TTS事件
func (c *Client) TrackTTS(status string, properties map[string]interface{}) {
	if properties == nil {
		properties = make(map[string]interface{})
	}
	properties["status"] = status
	
	var eventName string
	switch status {
	case "start":
		eventName = EventTTSSynthesizeStart
	case "complete":
		eventName = EventTTSSynthesizeComplete
	case "failed":
		eventName = EventTTSSynthesizeFailed
	default:
		eventName = "tts_" + status
	}
	
	c.Track(eventName, properties)
}

// TrackError 记录错误事件
func (c *Client) TrackError(errorType string, err error, properties map[string]interface{}) {
	if properties == nil {
		properties = make(map[string]interface{})
	}
	properties["error_type"] = errorType
	if err != nil {
		properties["error_message"] = err.Error()
	}
	
	c.Track(EventError, properties)
}

// TrackUser 记录用户事件
func (c *Client) TrackUser(action string, userID string, properties map[string]interface{}) {
	if properties == nil {
		properties = make(map[string]interface{})
	}
	properties["user_id"] = userID
	
	var eventName string
	switch action {
	case "login":
		eventName = EventUserLogin
	case "logout":
		eventName = EventUserLogout
	case "register":
		eventName = EventUserRegister
	default:
		eventName = "user_" + action
	}
	
	c.Track(eventName, properties)
}

// TrackPayment 记录支付事件
func (c *Client) TrackPayment(status string, properties map[string]interface{}) {
	if properties == nil {
		properties = make(map[string]interface{})
	}
	properties["status"] = status
	
	var eventName string
	switch status {
	case "start":
		eventName = EventPaymentStart
	case "complete":
		eventName = EventPaymentComplete
	case "failed":
		eventName = EventPaymentFailed
	default:
		eventName = "payment_" + status
	}
	
	c.Track(eventName, properties)
}

// TrackBilibiliAccountBindingSuccess 记录B站账号绑定成功事件。
func (c *Client) TrackBilibiliAccountBindingSuccess(properties map[string]interface{}) {
	if c == nil {
		return
	}
	if properties == nil {
		properties = make(map[string]interface{})
	}
	properties["platform"] = "bilibili"
	properties["status"] = "success"
	c.Track(EventBilibiliAccountBindingSuccess, properties)
}

// TrackBilibiliVideoUploadSuccess 记录B站视频上传成功事件。
func (c *Client) TrackBilibiliVideoUploadSuccess(properties map[string]interface{}) {
	if c == nil {
		return
	}
	if properties == nil {
		properties = make(map[string]interface{})
	}
	properties["platform"] = "bilibili"
	properties["status"] = "success"
	c.Track(EventBilibiliVideoUploadSuccess, properties)
}
