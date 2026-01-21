package pipeline

import (
	"time"

	"llmproxy/internal/config"
)

// FromConfig 从主配置创建管道配置
// 参数：
//   - cfg: 主配置中的鉴权配置
// 返回：
//   - *PipelineConfig: 管道配置
func FromConfig(cfg *config.AuthConfig) *PipelineConfig {
	if cfg == nil || !cfg.Enabled {
		return nil
	}

	pipelineConfig := &PipelineConfig{
		Enabled:     true,
		HeaderNames: cfg.HeaderNames,
		Mode:        PipelineMode(cfg.Mode),
		Providers:   make([]*ProviderConfig, 0),
	}

	// 设置默认模式
	if pipelineConfig.Mode == "" {
		pipelineConfig.Mode = PipelineModeFirstMatch
	}

	// 如果配置了 pipeline，使用新的管道模式
	if len(cfg.Pipeline) > 0 {
		for _, p := range cfg.Pipeline {
			providerCfg := &ProviderConfig{
				Name:    p.Name,
				Type:    ProviderType(p.Type),
				Enabled: p.Enabled,
			}

			// 转换 Lua 配置
			if p.Lua != nil {
				providerCfg.LuaScript = p.Lua.Script
				providerCfg.LuaScriptFile = p.Lua.Path
			}

			// 转换 Redis 配置
			if p.Redis != nil {
				providerCfg.Redis = &RedisConfig{
					Storage:    p.Redis.Storage,
					KeyPattern: p.Redis.KeyPattern,
				}
			}

			// 转换数据库配置
			if p.Database != nil {
				providerCfg.Database = &DatabaseConfig{
					Storage:   p.Database.Storage,
					Table:     p.Database.Table,
					KeyColumn: p.Database.KeyColumn,
					Fields:    p.Database.Fields,
				}
			}

			// 转换 Webhook 配置
			if p.Webhook != nil {
				timeout := p.Webhook.Timeout
				if timeout == 0 {
					timeout = 5 * time.Second
				}
				providerCfg.Webhook = &WebhookConfig{
					URL:     p.Webhook.URL,
					Method:  p.Webhook.Method,
					Timeout: timeout,
					Headers: p.Webhook.Headers,
				}
			}

			// 转换静态配置
			if p.Static != nil {
				providerCfg.StaticKeys = p.Static.Keys
			}

			pipelineConfig.Providers = append(pipelineConfig.Providers, providerCfg)
		}
	} else {
		// 兼容旧配置：如果没有 pipeline，使用 storage 配置
		// 默认使用 file 类型
		pipelineConfig.Providers = append(pipelineConfig.Providers, &ProviderConfig{
			Name:    "default_file",
			Type:    ProviderTypeFile,
			Enabled: true,
		})
	}

	return pipelineConfig
}
