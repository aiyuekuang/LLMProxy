package proxy

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"llmproxy/internal/config"
	"llmproxy/internal/metrics"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// UsageDBWriter 用量数据库写入器
type UsageDBWriter struct {
	name  string  // 上报器名称
	db    *sql.DB // 数据库连接
	table string  // 表名
	mu    sync.Mutex
}

// usageDBWriters 全局用量数据库写入器映射（支持多个）
var usageDBWriters = make(map[string]*UsageDBWriter)
var usageDBMutex sync.RWMutex

// InitUsageDatabase 初始化用量数据库
// 参数：
//   - name: 上报器名称
//   - cfg: 数据库配置
// 返回：
//   - error: 错误信息
func InitUsageDatabase(name string, cfg *config.UsageDatabaseConfig) error {
	if cfg == nil {
		return fmt.Errorf("数据库配置不能为空")
	}

	// 打开数据库连接
	db, err := sql.Open(cfg.Driver, cfg.DSN)
	if err != nil {
		return fmt.Errorf("数据库连接失败: %w", err)
	}

	// 测试连接
	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("数据库 Ping 失败: %w", err)
	}

	// 设置连接池
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	table := cfg.Table
	if table == "" {
		table = "usage_records"
	}

	// 自动创建表（如果不存在）
	if err := createUsageTable(db, cfg.Driver, table); err != nil {
		log.Printf("警告: 创建用量表失败: %v（请手动创建）", err)
	}

	// 存储到全局映射
	usageDBMutex.Lock()
	usageDBWriters[name] = &UsageDBWriter{
		name:  name,
		db:    db,
		table: table,
	}
	usageDBMutex.Unlock()

	log.Printf("用量数据库 [%s] 已初始化: %s, 表: %s", name, cfg.Driver, table)
	return nil
}

// InitUsageDatabaseWithConnection 使用已创建的数据库连接初始化用量数据库
func InitUsageDatabaseWithConnection(name string, db *sql.DB, table string) error {
	if db == nil {
		return fmt.Errorf("数据库连接不能为空")
	}

	if table == "" {
		table = "usage_records"
	}

	// 自动创建表（如果不存在）
	if err := createUsageTable(db, "mysql", table); err != nil {
		log.Printf("警告: 创建用量表失败: %v（请手动创建）", err)
	}

	// 存储到全局映射
	usageDBMutex.Lock()
	usageDBWriters[name] = &UsageDBWriter{
		name:  name,
		db:    db,
		table: table,
	}
	usageDBMutex.Unlock()

	log.Printf("用量数据库 [%s] 已初始化: 表: %s", name, table)
	return nil
}

// createUsageTable 创建用量表
func createUsageTable(db *sql.DB, driver, table string) error {
	var createSQL string

	switch driver {
	case "mysql":
		createSQL = fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id BIGINT AUTO_INCREMENT PRIMARY KEY,
				request_id VARCHAR(64),
				timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
				api_key VARCHAR(128),
				user_id VARCHAR(64),
				method VARCHAR(10),
				path VARCHAR(128),
				backend_url VARCHAR(256),
				status_code INT,
				latency_ms BIGINT,
				prompt_tokens INT DEFAULT 0,
				completion_tokens INT DEFAULT 0,
				total_tokens INT DEFAULT 0,
				request_body JSON,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				INDEX idx_api_key (api_key),
				INDEX idx_timestamp (timestamp),
				INDEX idx_user_id (user_id)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
		`, table)

	case "postgres":
		createSQL = fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id BIGSERIAL PRIMARY KEY,
				request_id VARCHAR(64),
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				api_key VARCHAR(128),
				user_id VARCHAR(64),
				method VARCHAR(10),
				path VARCHAR(128),
				backend_url VARCHAR(256),
				status_code INT,
				latency_ms BIGINT,
				prompt_tokens INT DEFAULT 0,
				completion_tokens INT DEFAULT 0,
				total_tokens INT DEFAULT 0,
				request_body JSONB,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			);
			CREATE INDEX IF NOT EXISTS idx_%s_api_key ON %s(api_key);
			CREATE INDEX IF NOT EXISTS idx_%s_timestamp ON %s(timestamp);
			CREATE INDEX IF NOT EXISTS idx_%s_user_id ON %s(user_id)
		`, table, table, table, table, table, table, table)

	case "sqlite":
		createSQL = fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				request_id TEXT,
				timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
				api_key TEXT,
				user_id TEXT,
				method TEXT,
				path TEXT,
				backend_url TEXT,
				status_code INTEGER,
				latency_ms INTEGER,
				prompt_tokens INTEGER DEFAULT 0,
				completion_tokens INTEGER DEFAULT 0,
				total_tokens INTEGER DEFAULT 0,
				request_body TEXT,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);
			CREATE INDEX IF NOT EXISTS idx_api_key ON %s(api_key);
			CREATE INDEX IF NOT EXISTS idx_timestamp ON %s(timestamp);
			CREATE INDEX IF NOT EXISTS idx_user_id ON %s(user_id)
		`, table, table, table, table)

	default:
		return fmt.Errorf("不支持的数据库驱动: %s", driver)
	}

	_, err := db.Exec(createSQL)
	return err
}

// SendUsageToDatabaseByName 写入用量数据到指定数据库
// 参数：
//   - name: 上报器名称
//   - usage: 用量记录
func SendUsageToDatabaseByName(name string, usage *UsageRecord) {
	if usage == nil {
		return
	}

	usageDBMutex.RLock()
	writer, ok := usageDBWriters[name]
	usageDBMutex.RUnlock()

	if !ok || writer == nil {
		log.Printf("[%s] 用量数据库未初始化", name)
		return
	}

	writer.mu.Lock()
	defer writer.mu.Unlock()

	// 序列化请求体
	requestBodyJSON, err := json.Marshal(usage.RequestBody)
	if err != nil {
		log.Printf("序列化请求体失败: %v", err)
		requestBodyJSON = []byte("{}")
	}

	// 提取用量信息
	promptTokens := 0
	completionTokens := 0
	totalTokens := 0
	if usage.Usage != nil {
		promptTokens = usage.Usage.PromptTokens
		completionTokens = usage.Usage.CompletionTokens
		totalTokens = usage.Usage.TotalTokens
	}

	// 插入数据
	insertSQL := fmt.Sprintf(`
		INSERT INTO %s (
			request_id, timestamp, api_key, user_id, method, path, 
			backend_url, status_code, latency_ms, 
			prompt_tokens, completion_tokens, total_tokens, request_body
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, writer.table)

	_, err = writer.db.Exec(
		insertSQL,
		usage.RequestID,
		usage.Timestamp,
		usage.APIKey,
		usage.UserID,
		usage.Method,
		usage.Path,
		usage.BackendURL,
		usage.StatusCode,
		usage.LatencyMs,
		promptTokens,
		completionTokens,
		totalTokens,
		string(requestBodyJSON),
	)

	if err != nil {
		log.Printf("[%s] 写入用量数据失败: %v", name, err)
		metrics.RecordWebhookFailure()
		return
	}

	metrics.RecordWebhookSuccess()
	log.Printf("[%s] 用量数据已写入数据库: request_id=%s, tokens=%d", name, usage.RequestID, totalTokens)
}

// CloseAllUsageDatabases 关闭所有用量数据库连接
func CloseAllUsageDatabases() {
	usageDBMutex.Lock()
	defer usageDBMutex.Unlock()

	for name, writer := range usageDBWriters {
		if writer != nil && writer.db != nil {
			writer.db.Close()
			log.Printf("[%s] 用量数据库连接已关闭", name)
		}
	}
	usageDBWriters = make(map[string]*UsageDBWriter)
}
