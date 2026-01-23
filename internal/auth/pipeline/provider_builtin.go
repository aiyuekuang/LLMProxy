package pipeline

import (
	"context"

	"llmproxy/internal/admin"
)

// BuiltinProvider 内置 SQLite Provider
// 从 KeyStore 中读取 API Key 信息
type BuiltinProvider struct {
	BaseProvider
	keyStore *admin.KeyStore // Key 存储
}

// NewBuiltinProvider 创建内置 Provider
// 参数：
//   - name: Provider 名称
//   - keyStore: KeyStore 实例
//
// 返回：
//   - Provider: Provider 实例
func NewBuiltinProvider(name string, keyStore *admin.KeyStore) Provider {
	return &BuiltinProvider{
		BaseProvider: BaseProvider{
			name:         name,
			providerType: ProviderTypeBuiltin,
		},
		keyStore: keyStore,
	}
}

// Query 查询 API Key 信息
// 参数：
//   - ctx: 上下文
//   - apiKey: API Key 字符串
//
// 返回：
//   - *ProviderResult: 查询结果
func (b *BuiltinProvider) Query(ctx context.Context, apiKey string) *ProviderResult {
	// 从 KeyStore 查询
	key, err := b.keyStore.Get(apiKey)
	if err != nil {
		return &ProviderResult{
			Found: false,
			Error: err,
		}
	}

	// 未找到
	if key == nil {
		return &ProviderResult{Found: false}
	}

	// 转换为 map 格式
	data := map[string]interface{}{
		"key":        key.Key,
		"status":     int(key.Status), // 返回整数状态
		"created_at": key.CreatedAt.Unix(),
		"updated_at": key.UpdatedAt.Unix(),
	}

	// 处理可选字段
	if key.Name != "" {
		data["name"] = key.Name
	}
	if key.UserID != "" {
		data["user_id"] = key.UserID
	}
	if key.StartsAt != nil {
		data["starts_at"] = key.StartsAt.Unix()
	}
	if key.ExpiresAt != nil {
		data["expires_at"] = key.ExpiresAt.Unix()
	}

	return &ProviderResult{
		Found: true,
		Data:  data,
	}
}

// GetKeyStore 获取 KeyStore 实例（用于外部访问）
func (b *BuiltinProvider) GetKeyStore() *admin.KeyStore {
	return b.keyStore
}
