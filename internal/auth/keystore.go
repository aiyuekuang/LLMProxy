package auth

import (
	"fmt"
	"sync"
	"time"
)

// KeyStore Key 存储接口
type KeyStore interface {
	// Get 获取 API Key
	Get(key string) (*APIKey, error)
	
	// Update 更新 API Key
	Update(key *APIKey) error
	
	// IncrementUsedQuota 增加已使用额度（原子操作）
	IncrementUsedQuota(key string, tokens int64) error
}

// FileKeyStore 基于配置文件的 Key 存储
type FileKeyStore struct {
	keys map[string]*APIKey // key -> APIKey 映射
	mu   sync.RWMutex       // 读写锁
}

// NewFileKeyStore 创建文件 Key 存储
// 参数：
//   - keys: API Key 列表
// 返回：
//   - KeyStore: Key 存储实例
func NewFileKeyStore(keys []*APIKey) KeyStore {
	store := &FileKeyStore{
		keys: make(map[string]*APIKey),
	}
	
	// 初始化 Key 映射
	for _, key := range keys {
		// 设置默认值
		if key.Status == "" {
			key.Status = "active"
		}
		if key.CreatedAt.IsZero() {
			key.CreatedAt = time.Now()
		}
		if key.UpdatedAt.IsZero() {
			key.UpdatedAt = time.Now()
		}
		if key.LastResetAt.IsZero() {
			key.LastResetAt = time.Now()
		}
		
		store.keys[key.Key] = key
	}
	
	return store
}

// Get 获取 API Key
// 参数：
//   - key: API Key 字符串
// 返回：
//   - *APIKey: API Key 对象
//   - error: 错误信息
func (fs *FileKeyStore) Get(key string) (*APIKey, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	
	apiKey, ok := fs.keys[key]
	if !ok {
		return nil, fmt.Errorf("API Key 不存在")
	}
	
	// 检查是否需要重置额度
	if ResetQuotaIfNeeded(apiKey) {
		// 注意：这里只是内存中重置，不会持久化到文件
		// 如果需要持久化，需要实现配置文件写入功能
	}
	
	return apiKey, nil
}

// Update 更新 API Key
// 参数：
//   - key: API Key 对象
// 返回：
//   - error: 错误信息
func (fs *FileKeyStore) Update(key *APIKey) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	
	if _, ok := fs.keys[key.Key]; !ok {
		return fmt.Errorf("API Key 不存在")
	}
	
	key.UpdatedAt = time.Now()
	fs.keys[key.Key] = key
	
	return nil
}

// IncrementUsedQuota 增加已使用额度
// 参数：
//   - key: API Key 字符串
//   - tokens: 消耗的 tokens
// 返回：
//   - error: 错误信息
func (fs *FileKeyStore) IncrementUsedQuota(key string, tokens int64) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	
	apiKey, ok := fs.keys[key]
	if !ok {
		return fmt.Errorf("API Key 不存在")
	}
	
	apiKey.UsedQuota += tokens
	apiKey.UpdatedAt = time.Now()
	
	return nil
}
