package proxy

import (
	"log"
	"sync"

	"llmproxy/internal/admin"
)

// builtinUsageStore 内置用量存储
var builtinUsageStore *admin.UsageStore
var builtinUsageMutex sync.RWMutex

// InitBuiltinUsage 初始化内置用量存储
// 参数：
//   - store: 用量存储实例（从 admin 包获取）
func InitBuiltinUsage(store *admin.UsageStore) {
	builtinUsageMutex.Lock()
	defer builtinUsageMutex.Unlock()

	builtinUsageStore = store
	log.Printf("内置用量存储已初始化")
}

// SendUsageToBuiltin 发送用量数据到内置存储
// 参数：
//   - usage: 用量记录
func SendUsageToBuiltin(usage *UsageRecord) {
	if usage == nil {
		return
	}

	builtinUsageMutex.RLock()
	store := builtinUsageStore
	builtinUsageMutex.RUnlock()

	if store == nil {
		return
	}

	// 转换为 admin.UsageRecord 格式
	record := &admin.UsageRecord{
		RequestID:  usage.RequestID,
		APIKey:     usage.APIKey,
		UserID:     usage.UserID,
		Endpoint:   usage.Path,
		BackendURL: usage.BackendURL,
		StatusCode: usage.StatusCode,
		LatencyMs:  usage.LatencyMs,
		Streaming:  false, // 从 UsageRecord 中无法直接获取，默认 false
		CreatedAt:  usage.Timestamp,
	}

	// 从请求体中提取 model
	if usage.RequestBody != nil {
		if model, ok := usage.RequestBody["model"].(string); ok {
			record.Model = model
		}
		// 检查是否是流式请求
		if stream, ok := usage.RequestBody["stream"].(bool); ok {
			record.Streaming = stream
		}
	}

	// 填充用量信息
	if usage.Usage != nil {
		record.PromptTokens = usage.Usage.PromptTokens
		record.CompletionTokens = usage.Usage.CompletionTokens
		record.TotalTokens = usage.Usage.TotalTokens
	}

	// 写入存储
	if err := store.Record(record); err != nil {
		log.Printf("写入内置用量存储失败: %v", err)
	}
}

// GetBuiltinUsageStore 获取内置用量存储
// 返回：
//   - *admin.UsageStore: 用量存储实例
func GetBuiltinUsageStore() *admin.UsageStore {
	builtinUsageMutex.RLock()
	defer builtinUsageMutex.RUnlock()
	return builtinUsageStore
}
