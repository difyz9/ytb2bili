package updater

import (
	"context"
	"fmt"
	"os"
	"time"
)

// CLICommands 提供命令行接口
type CLICommands struct {
	updater *Updater
}

// NewCLI 创建 CLI 命令处理器
func NewCLI(updater *Updater) *CLICommands {
	return &CLICommands{
		updater: updater,
	}
}

// Version 显示当前版本
func (c *CLICommands) Version() {
	fmt.Printf("ytb2bili version %s\n", c.updater.GetCurrentVersion())
}

// CheckUpdate 检查更新
func (c *CLICommands) CheckUpdate() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	fmt.Println("正在检查更新...")
	hasUpdate, latestVersion, err := c.updater.CheckForUpdates(ctx)
	if err != nil {
		return fmt.Errorf("检查更新失败: %w", err)
	}
	
	if hasUpdate {
		fmt.Printf("✅ 发现新版本: %s (当前版本: %s)\n", latestVersion, c.updater.GetCurrentVersion())
		fmt.Println("运行 'ytb2bili update' 来安装更新")
	} else {
		fmt.Printf("✅ 已是最新版本: %s\n", c.updater.GetCurrentVersion())
	}
	
	return nil
}

// Update 执行更新
func (c *CLICommands) Update() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	
	fmt.Println("正在检查更新...")
	hasUpdate, _, err := c.updater.CheckForUpdates(ctx)
	if err != nil {
		return fmt.Errorf("检查更新失败: %w", err)
	}
	
	if !hasUpdate {
		fmt.Println("✅ 已是最新版本，无需更新")
		return nil
	}
	
	fmt.Println("开始下载并安装更新...")
	if err := c.updater.DoUpdate(ctx); err != nil {
		return fmt.Errorf("更新失败: %w", err)
	}
	
	fmt.Println("✅ 更新成功！")
	fmt.Println("请重启应用以应用更新")
	os.Exit(0)
	
	return nil
}
