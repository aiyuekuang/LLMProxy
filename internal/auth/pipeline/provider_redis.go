package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisProvider Redis Provider
// 从 Redis 读取 API Key 信息
type RedisProvider struct {
	BaseProvider
	client     *redis.Client // Redis 客户端
	keyPattern string        // Key 模式
}

// NewRedisProvider 创建 Redis Provider
// 参数：
//   - name: Provider 名称
//   - cfg: Redis 配置
// 返回：
//   - Provider: Provider 实例
//   - error: 错误信息
func NewRedisProvider(name string, cfg *RedisConfig) (Provider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("Redis 配置不能为空")
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("Redis 连接失败: %w", err)
	}

	keyPattern := cfg.KeyPattern
	if keyPattern == "" {
		keyPattern = "llmproxy:key:{api_key}"
	}

	return &RedisProvider{
		BaseProvider: BaseProvider{
			name:         name,
			providerType: ProviderTypeRedis,
		},
		client:     client,
		keyPattern: keyPattern,
	}, nil
}

// Query 查询 API Key 信息
// 参数：
//   - ctx: 上下文
//   - apiKey: API Key 字符串
// 返回：
//   - *ProviderResult: 查询结果
func (r *RedisProvider) Query(ctx context.Context, apiKey string) *ProviderResult {
	// 替换 key pattern 中的占位符
	key := strings.ReplaceAll(r.keyPattern, "{api_key}", apiKey)

	// 尝试获取 Hash 类型数据
	result, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		return &ProviderResult{
			Found: false,
			Error: fmt.Errorf("Redis 查询失败: %w", err),
		}
	}

	if len(result) == 0 {
		// 尝试获取 String 类型数据（JSON 格式）
		strResult, err := r.client.Get(ctx, key).Result()
		if err == redis.Nil {
			return &ProviderResult{Found: false}
		}
		if err != nil {
			return &ProviderResult{
				Found: false,
				Error: fmt.Errorf("Redis 查询失败: %w", err),
			}
		}

		// 解析 JSON
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(strResult), &data); err != nil {
			return &ProviderResult{
				Found: false,
				Error: fmt.Errorf("JSON 解析失败: %w", err),
			}
		}

		return &ProviderResult{
			Found: true,
			Data:  data,
		}
	}

	// 转换 Hash 结果为 map[string]interface{}
	data := make(map[string]interface{})
	for k, v := range result {
		data[k] = v
	}

	return &ProviderResult{
		Found: true,
		Data:  data,
	}
}

// Close 关闭 Redis 连接
func (r *RedisProvider) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

// Set 设置 API Key 信息（供业务系统调用）
// 参数：
//   - ctx: 上下文
//   - apiKey: API Key 字符串
//   - data: 要存储的数据
//   - expiration: 过期时间（0 表示不过期）
// 返回：
//   - error: 错误信息
func (r *RedisProvider) Set(ctx context.Context, apiKey string, data map[string]interface{}, expiration time.Duration) error {
	key := strings.ReplaceAll(r.keyPattern, "{api_key}", apiKey)

	// 转换为 map[string]string
	fields := make(map[string]interface{})
	for k, v := range data {
		fields[k] = fmt.Sprintf("%v", v)
	}

	if err := r.client.HSet(ctx, key, fields).Err(); err != nil {
		return fmt.Errorf("Redis 写入失败: %w", err)
	}

	if expiration > 0 {
		if err := r.client.Expire(ctx, key, expiration).Err(); err != nil {
			return fmt.Errorf("设置过期时间失败: %w", err)
		}
	}

	return nil
}
