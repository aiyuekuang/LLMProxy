package database

import (
	"database/sql"
	"log"
	"sync"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"llmproxy/internal/config"
)

// Service 服务模型（与 Admin 数据库同步）
type Service struct {
	ID           uint   `gorm:"primaryKey"`
	Name         string `gorm:"size:64"`
	URL          string `gorm:"size:256"`
	Type         string `gorm:"size:32"`
	Models       string `gorm:"size:1024"`
	Weight       int    `gorm:"default:1"`
	Priority     int    `gorm:"default:0"`
	Status       string `gorm:"size:16"`
	HealthStatus string `gorm:"size:16"`
}

func (Service) TableName() string {
	return "services"
}

// RequestLog 请求日志
type RequestLog struct {
	ID               uint   `gorm:"primaryKey"`
	UserID           uint   `gorm:"index"`
	Username         string `gorm:"size:64"`
	APIKeyID         uint   `gorm:"index"`
	APIKeyName       string `gorm:"size:64"`
	ChannelID        uint   `gorm:"index"`
	ChannelName      string `gorm:"size:64"`
	Model            string `gorm:"size:64;index"`
	RequestModel     string `gorm:"size:64"`
	ActualModel      string `gorm:"size:64"`
	PromptTokens     int    `gorm:"default:0"`
	CompletionTokens int    `gorm:"default:0"`
	TotalTokens      int    `gorm:"default:0"`
	Quota            int64  `gorm:"default:0"`
	Duration         int    `gorm:"default:0"`
	Status           int    `gorm:"default:200"`
	Endpoint         string `gorm:"size:128"`
	ClientIP         string `gorm:"size:64"`
	ErrorMessage     string `gorm:"size:512"`
	IsStream         bool   `gorm:"default:false"`
	CreatedAt        int64  `gorm:"index"`
}

func (RequestLog) TableName() string {
	return "request_logs"
}

// Store 数据库存储
type Store struct {
	db           *gorm.DB
	conn         *config.DatabaseConnection
	tableName    string
	syncInterval time.Duration
	services     []Service
	mu           sync.RWMutex
	stopCh       chan struct{} // 停止信号
}

// NewStoreFromConnection 从数据库连接配置创建 Store
func NewStoreFromConnection(conn *config.DatabaseConnection, tableName string) (*Store, error) {
	if conn == nil {
		return nil, nil
	}

	dsn := conn.GetDSN()
	if dsn == "" {
		return nil, nil
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	// 设置连接池
	sqlDB, err := db.DB()
	if err == nil {
		if conn.MaxOpenConns > 0 {
			sqlDB.SetMaxOpenConns(conn.MaxOpenConns)
		}
		if conn.MaxIdleConns > 0 {
			sqlDB.SetMaxIdleConns(conn.MaxIdleConns)
		}
		if conn.ConnMaxLifetime > 0 {
			sqlDB.SetConnMaxLifetime(conn.ConnMaxLifetime)
		}
		if conn.ConnMaxIdleTime > 0 {
			sqlDB.SetConnMaxIdleTime(conn.ConnMaxIdleTime)
		}
	}

	if tableName == "" {
		tableName = "services"
	}

	store := &Store{
		db:           db,
		conn:         conn,
		tableName:    tableName,
		syncInterval: 30 * time.Second,
	}

	// 初始加载
	store.syncServices()

	return store, nil
}

// NewStoreFromDB 从已创建的数据库连接创建 Store
func NewStoreFromDB(sqlDB *sql.DB, tableName string) (*Store, error) {
	if sqlDB == nil {
		return nil, nil
	}

	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn: sqlDB,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	if tableName == "" {
		tableName = "services"
	}

	store := &Store{
		db:           db,
		tableName:    tableName,
		syncInterval: 30 * time.Second,
	}

	// 初始加载
	store.syncServices()

	return store, nil
}

// StartSync 启动定时同步
func (s *Store) StartSync() {
	if s.syncInterval <= 0 {
		s.syncInterval = 30 * time.Second
	}

	s.stopCh = make(chan struct{})

	go func() {
		ticker := time.NewTicker(s.syncInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.syncServices()
			case <-s.stopCh:
				log.Println("服务同步已停止")
				return
			}
		}
	}()

	log.Printf("服务同步已启动，间隔: %v", s.syncInterval)
}

// StopSync 停止定时同步
func (s *Store) StopSync() {
	if s.stopCh != nil {
		close(s.stopCh)
	}
}

// Close 关闭数据库连接
func (s *Store) Close() error {
	s.StopSync()
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// syncServices 同步服务配置
func (s *Store) syncServices() {
	var services []Service
	if err := s.db.Where("status = ?", "enabled").Find(&services).Error; err != nil {
		log.Printf("同步服务失败: %v", err)
		return
	}

	s.mu.Lock()
	s.services = services
	s.mu.Unlock()

	log.Printf("已同步 %d 个服务", len(services))
}

// GetServices 获取服务列表
func (s *Store) GetServices() []Service {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.services
}

// GetBackends 转换服务为后端配置
func (s *Store) GetBackends() []*config.Backend {
	s.mu.RLock()
	defer s.mu.RUnlock()

	backends := make([]*config.Backend, 0, len(s.services))
	for _, svc := range s.services {
		backends = append(backends, &config.Backend{
			Name:   svc.Name,
			URL:    svc.URL,
			Weight: svc.Weight,
		})
	}
	return backends
}

// LogRequest 记录请求日志
func (s *Store) LogRequest(reqLog *RequestLog) error {
	if reqLog.CreatedAt == 0 {
		reqLog.CreatedAt = time.Now().Unix()
	}

	return s.db.Create(reqLog).Error
}

// GetServiceByURL 根据 URL 获取服务
func (s *Store) GetServiceByURL(url string) *Service {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.services {
		if s.services[i].URL == url {
			return &s.services[i]
		}
	}
	return nil
}
