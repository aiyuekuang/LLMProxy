package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"

	"llmproxy/internal/config"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
	"github.com/redis/go-redis/v9"
)

// Manager 存储管理器
// 负责管理数据库和 Redis 连接池
type Manager struct {
	databases map[string]*sql.DB    // 数据库连接池
	caches    map[string]*redis.Client // Redis 连接池
	mu        sync.RWMutex
}

// NewManager 创建存储管理器
func NewManager() *Manager {
	return &Manager{
		databases: make(map[string]*sql.DB),
		caches:    make(map[string]*redis.Client),
	}
}

// Initialize 初始化所有存储连接
func (m *Manager) Initialize(cfg *config.StorageConfig) error {
	if cfg == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 初始化数据库连接
	for _, dbCfg := range cfg.Databases {
		if dbCfg == nil || !dbCfg.Enabled {
			continue
		}
		db, err := m.createDatabase(dbCfg)
		if err != nil {
			return fmt.Errorf("初始化数据库 [%s] 失败: %w", dbCfg.Name, err)
		}
		m.databases[dbCfg.Name] = db
		log.Printf("数据库 [%s] 已连接: %s", dbCfg.Name, dbCfg.Driver)
	}

	// 初始化缓存连接
	for _, cacheCfg := range cfg.Caches {
		if cacheCfg == nil || !cacheCfg.Enabled {
			continue
		}
		cache, err := m.createCache(cacheCfg)
		if err != nil {
			return fmt.Errorf("初始化缓存 [%s] 失败: %w", cacheCfg.Name, err)
		}
		m.caches[cacheCfg.Name] = cache
		log.Printf("缓存 [%s] 已连接: %s", cacheCfg.Name, cacheCfg.Addr)
	}

	return nil
}

// createDatabase 创建数据库连接
func (m *Manager) createDatabase(cfg *config.DatabaseConnection) (*sql.DB, error) {
	dsn := cfg.GetDSN()
	if dsn == "" {
		return nil, fmt.Errorf("无法生成 DSN")
	}
	db, err := sql.Open(cfg.Driver, dsn)
	if err != nil {
		return nil, err
	}

	// 设置连接池参数
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}

	// 测试连接
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// createCache 创建缓存连接
func (m *Manager) createCache(cfg *config.CacheConnection) (*redis.Client, error) {
	// 内存缓存不需要创建 Redis 连接
	if cfg.Driver == "memory" {
		log.Printf("缓存 [%s] 使用内存模式，跳过 Redis 连接", cfg.Name)
		return nil, nil
	}

	// Redis 缓存
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
	})

	// 测试连接
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, err
	}

	return client, nil
}

// GetDatabase 获取数据库连接
func (m *Manager) GetDatabase(name string) *sql.DB {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.databases[name]
}

// GetCache 获取缓存连接
func (m *Manager) GetCache(name string) *redis.Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.caches[name]
}

// Close 关闭所有连接
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error

	// 关闭数据库连接
	for name, db := range m.databases {
		if err := db.Close(); err != nil {
			log.Printf("关闭数据库 [%s] 失败: %v", name, err)
			lastErr = err
		}
	}

	// 关闭缓存连接
	for name, cache := range m.caches {
		if err := cache.Close(); err != nil {
			log.Printf("关闭缓存 [%s] 失败: %v", name, err)
			lastErr = err
		}
	}

	m.databases = make(map[string]*sql.DB)
	m.caches = make(map[string]*redis.Client)

	return lastErr
}
