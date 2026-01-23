package discovery

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"llmproxy/internal/config"
)

// Manager 服务发现管理器
// 负责管理多个发现源，并定期同步后端服务列表
type Manager struct {
	cfg      *config.DiscoveryConfig
	sources  []Source
	backends []*config.Backend
	mu       sync.RWMutex
	stopCh   chan struct{}

	// 存储管理器引用（用于创建数据库发现源）
	storageManager interface {
		GetDatabase(name string) *sql.DB
	}
	storageCfg *config.StorageConfig
}

// NewManager 创建服务发现管理器
// 参数：
//   - cfg: 服务发现配置
//   - storageManager: 存储管理器（用于数据库发现源）
//   - storageCfg: 存储配置（用于获取驱动类型）
//
// 返回：
//   - *Manager: 管理器实例
//   - error: 错误信息
func NewManager(cfg *config.DiscoveryConfig, storageManager interface {
	GetDatabase(name string) *sql.DB
}, storageCfg *config.StorageConfig) (*Manager, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, nil
	}

	m := &Manager{
		cfg:            cfg,
		sources:        make([]Source, 0),
		backends:       make([]*config.Backend, 0),
		storageManager: storageManager,
		storageCfg:     storageCfg,
	}

	// 初始化所有发现源
	for _, sourceCfg := range cfg.Sources {
		if sourceCfg == nil || !sourceCfg.Enabled {
			continue
		}

		source, err := m.createSource(sourceCfg)
		if err != nil {
			log.Printf("警告: 创建发现源 [%s] 失败: %v", sourceCfg.Name, err)
			continue
		}

		m.sources = append(m.sources, source)
		log.Printf("服务发现源 [%s] (类型: %s) 已加载", sourceCfg.Name, sourceCfg.Type)
	}

	if len(m.sources) == 0 {
		return nil, fmt.Errorf("服务发现: 没有可用的发现源")
	}

	// 初始发现
	m.discover()

	return m, nil
}

// createSource 创建发现源
func (m *Manager) createSource(cfg *config.DiscoverySource) (Source, error) {
	switch cfg.Type {
	case "static":
		if cfg.Static == nil {
			return nil, fmt.Errorf("static 发现源配置为空")
		}
		return NewStaticSource(cfg.Name, cfg.Static.Backends), nil

	case "http":
		if cfg.HTTP == nil {
			return nil, fmt.Errorf("http 发现源配置为空")
		}
		return NewHTTPSource(cfg.Name, cfg.HTTP)

	case "consul":
		if cfg.Consul == nil {
			return nil, fmt.Errorf("consul 发现源配置为空")
		}
		return NewConsulSource(cfg.Name, cfg.Consul)

	case "kubernetes":
		if cfg.Kubernetes == nil {
			return nil, fmt.Errorf("kubernetes 发现源配置为空")
		}
		return NewKubernetesSource(cfg.Name, cfg.Kubernetes)

	case "etcd":
		if cfg.Etcd == nil {
			return nil, fmt.Errorf("etcd 发现源配置为空")
		}
		return NewEtcdSource(cfg.Name, cfg.Etcd)

	case "database":
		if cfg.Database == nil {
			return nil, fmt.Errorf("database 发现源配置为空")
		}
		if m.storageManager == nil {
			return nil, fmt.Errorf("database 发现源需要 StorageManager")
		}
		dbConn := m.storageManager.GetDatabase(cfg.Database.Storage)
		if dbConn == nil {
			return nil, fmt.Errorf("数据库 [%s] 未找到", cfg.Database.Storage)
		}
		driver := ""
		if m.storageCfg != nil {
			if dbCfg := m.storageCfg.GetDatabase(cfg.Database.Storage); dbCfg != nil {
				driver = dbCfg.Driver
			}
		}
		return NewDatabaseSource(cfg.Name, dbConn, driver, cfg.Database)

	default:
		return nil, fmt.Errorf("未知的发现源类型: %s", cfg.Type)
	}
}

// Start 启动定期同步
func (m *Manager) Start() {
	if m == nil {
		return
	}

	interval := m.cfg.Interval
	if interval <= 0 {
		interval = 30 * time.Second
	}

	m.stopCh = make(chan struct{})

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.discover()
			case <-m.stopCh:
				log.Println("服务发现同步已停止")
				return
			}
		}
	}()

	log.Printf("服务发现同步已启动，间隔: %v", interval)
}

// Stop 停止定期同步
func (m *Manager) Stop() {
	if m == nil {
		return
	}
	if m.stopCh != nil {
		close(m.stopCh)
	}
}

// discover 执行服务发现
func (m *Manager) discover() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var allBackends []*config.Backend

	switch m.cfg.Mode {
	case "first":
		// first 模式：使用第一个成功返回的发现源
		for _, source := range m.sources {
			backends, err := source.Discover(ctx)
			if err != nil {
				log.Printf("发现源 [%s] 错误: %v", source.Name(), err)
				continue
			}
			if len(backends) > 0 {
				allBackends = backends
				log.Printf("使用发现源 [%s] 的 %d 个后端", source.Name(), len(backends))
				break
			}
		}

	case "merge":
		fallthrough
	default:
		// merge 模式（默认）：合并所有发现源的结果
		seen := make(map[string]bool)
		for _, source := range m.sources {
			backends, err := source.Discover(ctx)
			if err != nil {
				log.Printf("发现源 [%s] 错误: %v", source.Name(), err)
				continue
			}
			for _, bk := range backends {
				if !seen[bk.URL] {
					seen[bk.URL] = true
					allBackends = append(allBackends, bk)
				}
			}
		}
	}

	m.mu.Lock()
	m.backends = allBackends
	m.mu.Unlock()

	log.Printf("服务发现: 共 %d 个后端服务", len(allBackends))
}

// GetBackends 获取当前发现的后端服务列表
func (m *Manager) GetBackends() []*config.Backend {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.backends
}

// Close 关闭所有发现源
func (m *Manager) Close() error {
	if m == nil {
		return nil
	}
	m.Stop()
	for _, source := range m.sources {
		if err := source.Close(); err != nil {
			log.Printf("关闭发现源 [%s] 失败: %v", source.Name(), err)
		}
	}
	return nil
}
