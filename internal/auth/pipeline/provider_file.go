package pipeline

import (
	"context"
	"sync"
	"time"

	"llmproxy/internal/config"
)

// FileProvider 配置文件 Provider
// 从内存中的配置读取 API Key 信息
type FileProvider struct {
	BaseProvider
	keys map[string]*config.APIKey // key -> APIKey 映射
	mu   sync.RWMutex              // 读写锁
}

// NewFileProvider 创建配置文件 Provider
// 参数：
//   - name: Provider 名称
//   - keys: API Key 列表
//
// 返回：
//   - Provider: Provider 实例
func NewFileProvider(name string, keys []*config.APIKey) Provider {
	provider := &FileProvider{
		BaseProvider: BaseProvider{
			name:         name,
			providerType: ProviderTypeFile,
		},
		keys: make(map[string]*config.APIKey),
	}

	// 初始化 Key 映射
	for _, key := range keys {
		if key.Status == "" {
			key.Status = "active"
		}
		if key.CreatedAt.IsZero() {
			key.CreatedAt = time.Now()
		}
		provider.keys[key.Key] = key
	}

	return provider
}

// Query 查询 API Key 信息
// 参数：
//   - ctx: 上下文
//   - apiKey: API Key 字符串
//
// 返回：
//   - *ProviderResult: 查询结果
func (f *FileProvider) Query(ctx context.Context, apiKey string) *ProviderResult {
	f.mu.RLock()
	defer f.mu.RUnlock()

	key, ok := f.keys[apiKey]
	if !ok {
		return &ProviderResult{Found: false}
	}

	// 转换为 map 格式
	data := map[string]interface{}{
		"key":                key.Key,
		"name":               key.Name,
		"user_id":            key.UserID,
		"status":             key.Status,
		"total_quota":        key.TotalQuota,
		"used_quota":         key.UsedQuota,
		"quota_reset_period": key.QuotaResetPeriod,
		"allowed_ips":        key.AllowedIPs,
		"denied_ips":         key.DeniedIPs,
		"created_at":         key.CreatedAt.Unix(),
		"updated_at":         key.UpdatedAt.Unix(),
	}

	// 处理可选的过期时间
	if key.ExpiresAt != nil {
		data["expires_at"] = key.ExpiresAt.Unix()
	}

	return &ProviderResult{
		Found: true,
		Data:  data,
	}
}

// UpdateKey 更新 API Key（用于动态更新）
// 参数：
//   - key: API Key 对象
func (f *FileProvider) UpdateKey(key *config.APIKey) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.keys[key.Key] = key
}

// IncrementUsedQuota 增加已使用额度
// 参数：
//   - apiKey: API Key 字符串
//   - tokens: 消耗的 tokens
//
// 返回：
//   - error: 错误信息
func (f *FileProvider) IncrementUsedQuota(apiKey string, tokens int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if key, ok := f.keys[apiKey]; ok {
		key.UsedQuota += tokens
		key.UpdatedAt = time.Now()
	}
	return nil
}
