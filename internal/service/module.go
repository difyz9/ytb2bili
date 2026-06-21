package service

import (
	"go.uber.org/fx"
)

// Module 提供所有服务的依赖注入模块
var Module = fx.Module("service",
	fx.Provide(
		NewAgentOpenService,
		NewSystemSettingsClient,
		NewYouTubeBindingService,
		NewYouTubeClientFactory,

		NewEmbeddedTTSVoiceCatalog,
		NewUserSettingsClient,

		// License 激活验证
		NewLicenseClient,

		// 新服务
		NewVideoService,
		NewYouTubeService,
		NewBindingService,
	),
)
