package manager

import (
	"github.com/difyz9/ytb2bili/internal/core/services"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/difyz9/ytb2bili/internal/core/models"
)

// StateManager 任务状态管理器
type StateManager struct {
	Id          uint
	VideoID     string
	ProjectRoot string
	CurrentDir  string

	// 文件路径
	InputVideoPath  string
	NoviceVideoPath string
	OutVideoPath    string
	ImageCover      string
	OriginalMP3     string
	OriginalWAV     string // WAV音频文件（用于Whisper）
	TranslateMP3    string
	OriginalJSON    string
	TranslateJSON   string
	OriginalSRT     string
	M3u8FileName    string
	M3u8FileDir     string
	TranslateSRT    string
	TranslateVtt    string
	TranslateTXT    string
	// 目录路径
	AudioDir       string
	SaveUrlService *services.TbVideoService

	// 内存缓存
	cache map[string]interface{}
	mu    sync.RWMutex
}

// NewStateManager 创建状态管理器
func NewStateManager(Id uint, videoID, projectRoot string, createTim time.Time) *StateManager {
	currentDir := filepath.Join(projectRoot, GetCurrentDateYYYYMMDD(createTim), videoID)

	os.MkdirAll(currentDir, os.ModePerm)

	//audioDir := filepath.Join(currentDir, "audio")
	//m8u3Dir := filepath.Join(currentDir, "m3u8")
	//
	//os.MkdirAll(audioDir, os.ModePerm)
	//os.MkdirAll(m8u3Dir, os.ModePerm)

	return &StateManager{
		Id:             Id,
		VideoID:        videoID,
		ProjectRoot:    projectRoot,
		CurrentDir:     currentDir,
		InputVideoPath: filepath.Join(currentDir, videoID+".mp4"),
		OutVideoPath:   filepath.Join(currentDir, videoID+"out.mp4"),
		OriginalWAV:    filepath.Join(currentDir, videoID+".wav"),
		OriginalMP3:    filepath.Join(currentDir, videoID+".mp3"),
		ImageCover:     filepath.Join(currentDir, "cover.jpg"),
		OriginalSRT:    filepath.Join(currentDir, "en.srt"),
		OriginalJSON:   filepath.Join(currentDir, "en.json"),
		TranslateJSON:  filepath.Join(currentDir, "zh.json"),
		TranslateSRT:   filepath.Join(currentDir, "zh.srt"),
		TranslateVtt:   filepath.Join(currentDir, "zh.vtt"),
		TranslateTXT:   filepath.Join(currentDir, videoID+"_trans.txt"),
		//AudioDir:       audioDir,
		//M3u8FileDir:    m8u3Dir,
		//M3u8FileName:   filepath.Join(m8u3Dir, "output.m3u8"),
		cache: make(map[string]interface{}),
	}
}

// GetCache 获取缓存
func (s *StateManager) GetCache(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.cache[key]
	return val, ok
}

// SetCache 设置缓存
func (s *StateManager) SetCache(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[key] = value
}

// UpdateTBVideo 更新TBVideo记录并通过MQTT通知
func (s *StateManager) UpdateTBVideo(item *models.TbVideo) error {
	// 使用 GORM 的 Updates 方法，仅更新非空字段
	if err := s.SaveUrlService.UpdateItem(item); err != nil {
		return err
	}
	return nil
}

// GetCurrentDateYYYYMMDD 返回当前日期的yyyymmdd格式字符串
func GetCurrentDateYYYYMMDD(time2 time.Time) string {
	return time2.Format("2006-01-02")
}
