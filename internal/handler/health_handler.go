package handler

import (
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
)

// 构建信息（由 main 包注入）
var (
	Version   = "dev"
	BuildTime = "unknown"
	CommitSHA = "unknown"
	startedAt = time.Now()
)

// SetBuildInfo 设置构建信息
func SetBuildInfo(version, buildTime, commitSHA string) {
	Version = version
	BuildTime = buildTime
	CommitSHA = commitSHA
}

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	OK        bool   `json:"ok"`
	Version   string `json:"version"`
	BuildTime string `json:"buildTime"`
	CommitSHA string `json:"commitSHA,omitempty"`
}

type SystemUsageMetric struct {
	TotalBytes  uint64  `json:"total_bytes"`
	UsedBytes   uint64  `json:"used_bytes"`
	FreeBytes   uint64  `json:"free_bytes"`
	UsedPercent float64 `json:"used_percent"`
}

type SystemUsageResponse struct {
	Disk          SystemUsageMetric `json:"disk"`
	Memory        SystemUsageMetric `json:"memory"`
	DiskPath      string            `json:"disk_path"`
	CPUPercent    float64           `json:"cpu_percent"`
	UptimeSeconds int64             `json:"uptime_seconds"`
}

// Health godoc
// @Summary      Health check
// @Description  Check if the API service is running and get build info
// @Tags         health
// @Accept       json
// @Produce      json
// @Success      200  {object}  HealthResponse
// @Router       /health [get]
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(200, HealthResponse{
		OK:        true,
		Version:   Version,
		BuildTime: BuildTime,
		CommitSHA: CommitSHA,
	})
}

func currentDiskUsagePath() string {
	workingDir, err := os.Getwd()
	if err != nil || workingDir == "" {
		return "/"
	}
	return workingDir
}

// SystemUsage godoc
// @Summary      System usage
// @Description  Get current disk and memory usage for the running service host
// @Tags         health
// @Accept       json
// @Produce      json
// @Success      200  {object}  Response{data=SystemUsageResponse}
// @Failure      500  {object}  Response
// @Router       /api/v1/system/usage [get]
func (h *HealthHandler) SystemUsage(c *gin.Context) {
	diskPath := currentDiskUsagePath()
	diskUsage, err := disk.Usage(diskPath)
	if err != nil {
		InternalServerError(c, "获取磁盘用量失败")
		return
	}

	virtualMemory, err := mem.VirtualMemory()
	if err != nil {
		InternalServerError(c, "获取内存用量失败")
		return
	}

	cpuPercentages, err := cpu.Percent(0, false)
	if err != nil {
		InternalServerError(c, "获取 CPU 用量失败")
		return
	}

	var cpuPercent float64
	if len(cpuPercentages) > 0 {
		cpuPercent = cpuPercentages[0]
	}

	Success(c, SystemUsageResponse{
		Disk: SystemUsageMetric{
			TotalBytes:  diskUsage.Total,
			UsedBytes:   diskUsage.Used,
			FreeBytes:   diskUsage.Free,
			UsedPercent: diskUsage.UsedPercent,
		},
		Memory: SystemUsageMetric{
			TotalBytes:  virtualMemory.Total,
			UsedBytes:   virtualMemory.Used,
			FreeBytes:   virtualMemory.Available,
			UsedPercent: virtualMemory.UsedPercent,
		},
		DiskPath:      diskPath,
		CPUPercent:    cpuPercent,
		UptimeSeconds: int64(time.Since(startedAt).Seconds()),
	})
}

func (h *HealthHandler) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", h.Health)
	r.GET("/api/v1/system/usage", h.SystemUsage)
}
