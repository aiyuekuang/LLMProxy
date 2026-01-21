package auth

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"llmproxy/internal/config"
)

// RedisKeyStore 基于 Redis 的 Key 存储
type RedisKeyStore struct {
	client     *redis.Client
	keyPattern string // Key 模式，如 "llmproxy:key:{api_key}"
}

// NewRedisKeyStore 创建 Redis Key 存储
func NewRedisKeyStore(cfg *config.RedisAuthConfig) (KeyStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("Redis 配置为空")
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
		return nil, fmt.Errorf("连接 Redis 失败: %w", err)
	}

	keyPattern := cfg.KeyPattern
	if keyPattern == "" {
		keyPattern = "llmproxy:key:{api_key}"
	}

	return &RedisKeyStore{
		client:     client,
		keyPattern: keyPattern,
	}, nil
}

// buildKey 构建 Redis Key
func (rs *RedisKeyStore) buildKey(apiKey string) string {
	return strings.Replace(rs.keyPattern, "{api_key}", apiKey, 1)
}

// Get 从 Redis 获取 API Key
func (rs *RedisKeyStore) Get(key string) (*APIKey, error) {
	ctx := context.Background()
	redisKey := rs.buildKey(key)

	// 获取所有字段
	data, err := rs.client.HGetAll(ctx, redisKey).Result()
	if err != nil {
		return nil, fmt.Errorf("Redis 查询失败: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("API Key 不存在")
	}

	// 解析数据
	apiKey := &APIKey{
		Key:    key,
		UserID: data["user_id"],
		Name:   data["name"],
		Status: data["status"],
	}

	// 状态映射: enabled -> active (兼容 Admin 的格式)
	if apiKey.Status == "enabled" {
		apiKey.Status = "active"
	}

	// 解析额度
	if v, ok := data["quota"]; ok {
		apiKey.TotalQuota, _ = strconv.ParseInt(v, 10, 64)
	}
	if v, ok := data["used"]; ok {
		apiKey.UsedQuota, _ = strconv.ParseInt(v, 10, 64)
	}

	// 解析过期时间
	if v, ok := data["expire_at"]; ok {
		if ts, err := strconv.ParseInt(v, 10, 64); err == nil && ts > 0 {
			t := time.Unix(ts, 0)
			apiKey.ExpiresAt = &t
		}
	}

	// 解析 IP 白名单
	if v, ok := data["allowed_ips"]; ok && v != "" {
		apiKey.AllowedIPs = strings.Split(v, ",")
	}

	// 解析时间戳
	if v, ok := data["created_at"]; ok {
		if ts, err := strconv.ParseInt(v, 10, 64); err == nil {
			apiKey.CreatedAt = time.Unix(ts, 0)
		}
	}
	if v, ok := data["updated_at"]; ok {
		if ts, err := strconv.ParseInt(v, 10, 64); err == nil {
			apiKey.UpdatedAt = time.Unix(ts, 0)
		}
	}

	return apiKey, nil
}

// Update 更新 API Key
func (rs *RedisKeyStore) Update(key *APIKey) error {
	ctx := context.Background()
	redisKey := rs.buildKey(key.Key)

	data := map[string]interface{}{
		"user_id":    key.UserID,
		"name":       key.Name,
		"status":     key.Status,
		"quota":      key.TotalQuota,
		"used":       key.UsedQuota,
		"updated_at": time.Now().Unix(),
	}

	if len(key.AllowedIPs) > 0 {
		data["allowed_ips"] = strings.Join(key.AllowedIPs, ",")
	}
	if key.ExpiresAt != nil {
		data["expire_at"] = key.ExpiresAt.Unix()
	}

	return rs.client.HSet(ctx, redisKey, data).Err()
}

// IncrementUsedQuota 增加已使用额度（原子操作）
func (rs *RedisKeyStore) IncrementUsedQuota(key string, tokens int64) error {
	ctx := context.Background()
	redisKey := rs.buildKey(key)

	// 使用 HINCRBY 原子增加
	pipe := rs.client.Pipeline()
	pipe.HIncrBy(ctx, redisKey, "used", tokens)
	pipe.HSet(ctx, redisKey, "updated_at", time.Now().Unix())
	_, err := pipe.Exec(ctx)

	return err
}

// Close 关闭 Redis 连接
func (rs *RedisKeyStore) Close() error {
	return rs.client.Close()
}
