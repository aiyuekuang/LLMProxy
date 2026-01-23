package admin

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"llmproxy/internal/types"

	_ "modernc.org/sqlite"
)

// KeyStatus 别名，方便引用
type KeyStatus = types.KeyStatus

// 状态常量别名
const (
	KeyStatusActive        = types.KeyStatusActive
	KeyStatusDisabled      = types.KeyStatusDisabled
	KeyStatusQuotaExceeded = types.KeyStatusQuotaExceeded
	KeyStatusExpired       = types.KeyStatusExpired
)

// APIKey API Key 数据模型
type APIKey struct {
	Key       string     `json:"key"`                  // API Key
	Name      string     `json:"name,omitempty"`       // 名称/备注
	UserID    string     `json:"user_id,omitempty"`    // 用户标识（用于用量上报）
	Status    KeyStatus  `json:"status"`               // 状态: 0=active, 1=disabled, 2=quota_exceeded, 3=expired
	StartsAt  *time.Time `json:"starts_at,omitempty"`  // 生效时间（可选）
	ExpiresAt *time.Time `json:"expires_at,omitempty"` // 过期时间（可选）
	CreatedAt time.Time  `json:"created_at"`           // 创建时间
	UpdatedAt time.Time  `json:"updated_at"`           // 更新时间
}

// KeyStore API Key 存储
type KeyStore struct {
	db     *sql.DB
	dbPath string
	mu     sync.RWMutex
}

// NewKeyStore 创建 KeyStore
// 参数：
//   - dbPath: SQLite 数据库路径
//
// 返回：
//   - *KeyStore: KeyStore 实例
//   - error: 错误信息
func NewKeyStore(dbPath string) (*KeyStore, error) {
	// 确保目录存在
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据库目录失败: %w", err)
	}

	// 打开数据库
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// 测试连接
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("数据库连接失败: %w", err)
	}

	store := &KeyStore{
		db:     db,
		dbPath: dbPath,
	}

	// 初始化表结构
	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("初始化表结构失败: %w", err)
	}

	log.Printf("KeyStore 已初始化，数据库路径: %s", dbPath)
	return store, nil
}

// initSchema 初始化数据库表结构
func (s *KeyStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS api_keys (
		key TEXT PRIMARY KEY,
		name TEXT,
		user_id TEXT,
		status INTEGER NOT NULL DEFAULT 0,
		starts_at DATETIME,
		expires_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_api_keys_status ON api_keys(status);
	CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	// 迁移：添加新字段（如果不存在）
	s.migrateSchema()
	return nil
}

// migrateSchema 迁移数据库结构（添加新字段）
func (s *KeyStore) migrateSchema() {
	// 尝试添加 name 字段（忽略已存在错误）
	_, _ = s.db.Exec(`ALTER TABLE api_keys ADD COLUMN name TEXT`)
	// 尝试添加 user_id 字段（忽略已存在错误）
	_, _ = s.db.Exec(`ALTER TABLE api_keys ADD COLUMN user_id TEXT`)
	// 尝试添加 starts_at 字段（忽略已存在错误）
	_, _ = s.db.Exec(`ALTER TABLE api_keys ADD COLUMN starts_at DATETIME`)
	// 尝试创建 user_id 索引（忽略已存在错误）
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id)`)
}

// Create 创建 API Key
// 参数：
//   - key: API Key 数据
//
// 返回：
//   - error: 错误信息
func (s *KeyStore) Create(key *APIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	key.CreatedAt = now
	key.UpdatedAt = now

	query := `
	INSERT INTO api_keys (key, name, user_id, status, starts_at, expires_at, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.Exec(query, key.Key, key.Name, key.UserID, key.Status, key.StartsAt, key.ExpiresAt, key.CreatedAt, key.UpdatedAt)
	if err != nil {
		return fmt.Errorf("创建 API Key 失败: %w", err)
	}

	keyPrefix := key.Key
	if len(keyPrefix) > 8 {
		keyPrefix = keyPrefix[:8]
	}
	log.Printf("KeyStore: 已创建 Key [%s...] status=%d", keyPrefix, key.Status)
	return nil
}

// Update 更新 API Key
// 参数：
//   - key: API Key 数据
//
// 返回：
//   - error: 错误信息
func (s *KeyStore) Update(key *APIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key.UpdatedAt = time.Now()

	query := `
	UPDATE api_keys
	SET name = ?, user_id = ?, status = ?, starts_at = ?, expires_at = ?, updated_at = ?
	WHERE key = ?
	`
	result, err := s.db.Exec(query, key.Name, key.UserID, key.Status, key.StartsAt, key.ExpiresAt, key.UpdatedAt, key.Key)
	if err != nil {
		return fmt.Errorf("更新 API Key 失败: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("API Key 不存在")
	}

	keyPrefix := key.Key
	if len(keyPrefix) > 8 {
		keyPrefix = keyPrefix[:8]
	}
	log.Printf("KeyStore: 已更新 Key [%s...] status=%d", keyPrefix, key.Status)
	return nil
}

// Delete 删除 API Key
// 参数：
//   - keyStr: API Key 字符串
//
// 返回：
//   - error: 错误信息
func (s *KeyStore) Delete(keyStr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `DELETE FROM api_keys WHERE key = ?`
	result, err := s.db.Exec(query, keyStr)
	if err != nil {
		return fmt.Errorf("删除 API Key 失败: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("API Key 不存在")
	}

	keyPrefix := keyStr
	if len(keyPrefix) > 8 {
		keyPrefix = keyPrefix[:8]
	}
	log.Printf("KeyStore: 已删除 Key [%s...]", keyPrefix)
	return nil
}

// Get 获取 API Key
// 参数：
//   - keyStr: API Key 字符串
//
// 返回：
//   - *APIKey: API Key 数据
//   - error: 错误信息
func (s *KeyStore) Get(keyStr string) (*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
	SELECT key, name, user_id, status, starts_at, expires_at, created_at, updated_at
	FROM api_keys WHERE key = ?
	`
	row := s.db.QueryRow(query, keyStr)

	var key APIKey
	var name, userID sql.NullString
	var startsAt, expiresAt sql.NullTime
	err := row.Scan(&key.Key, &name, &userID, &key.Status, &startsAt, &expiresAt, &key.CreatedAt, &key.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil // 未找到
	}
	if err != nil {
		return nil, fmt.Errorf("查询 API Key 失败: %w", err)
	}

	if name.Valid {
		key.Name = name.String
	}
	if userID.Valid {
		key.UserID = userID.String
	}
	if startsAt.Valid {
		key.StartsAt = &startsAt.Time
	}
	if expiresAt.Valid {
		key.ExpiresAt = &expiresAt.Time
	}

	return &key, nil
}

// List 列出所有 API Key
// 参数：
//   - offset: 偏移量
//   - limit: 限制数量
//
// 返回：
//   - []*APIKey: API Key 列表
//   - int: 总数
//   - error: 错误信息
func (s *KeyStore) List(offset, limit int) ([]*APIKey, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 查询总数
	var total int
	countQuery := `SELECT COUNT(*) FROM api_keys`
	if err := s.db.QueryRow(countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("查询总数失败: %w", err)
	}

	// 查询列表
	query := `
	SELECT key, name, user_id, status, starts_at, expires_at, created_at, updated_at
	FROM api_keys
	ORDER BY created_at DESC
	LIMIT ? OFFSET ?
	`
	rows, err := s.db.Query(query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("查询 API Key 列表失败: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var keys []*APIKey
	for rows.Next() {
		var key APIKey
		var name, userID sql.NullString
		var startsAt, expiresAt sql.NullTime
		if err := rows.Scan(&key.Key, &name, &userID, &key.Status, &startsAt, &expiresAt, &key.CreatedAt, &key.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("扫描行失败: %w", err)
		}
		if name.Valid {
			key.Name = name.String
		}
		if userID.Valid {
			key.UserID = userID.String
		}
		if startsAt.Valid {
			key.StartsAt = &startsAt.Time
		}
		if expiresAt.Valid {
			key.ExpiresAt = &expiresAt.Time
		}
		keys = append(keys, &key)
	}

	return keys, total, nil
}

// SyncMode 同步模式
type SyncMode string

const (
	SyncModeFull        SyncMode = "full"        // 全量覆盖
	SyncModeIncremental SyncMode = "incremental" // 增量更新
)

// Sync 批量同步 API Key（全量覆盖）
// 参数：
//   - keys: API Key 列表
//
// 返回：
//   - error: 错误信息
func (s *KeyStore) Sync(keys []*APIKey) error {
	return s.SyncWithMode(keys, SyncModeFull)
}

// SyncWithMode 根据模式同步 API Key
// 参数：
//   - keys: API Key 列表
//   - mode: 同步模式 (full=全量覆盖, incremental=增量更新)
//
// 返回：
//   - error: 错误信息
func (s *KeyStore) SyncWithMode(keys []*APIKey, mode SyncMode) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer func() {
		_ = tx.Rollback() // 忽略回滚错误，因为可能已 Commit
	}()

	// 全量模式：先清空表
	if mode == SyncModeFull {
		if _, err := tx.Exec(`DELETE FROM api_keys`); err != nil {
			return fmt.Errorf("清空表失败: %w", err)
		}
	}

	now := time.Now()

	if mode == SyncModeFull {
		// 全量模式：直接插入
		stmt, err := tx.Prepare(`
		INSERT INTO api_keys (key, name, user_id, status, starts_at, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			return fmt.Errorf("准备语句失败: %w", err)
		}
		defer func() {
			_ = stmt.Close()
		}()

		for _, key := range keys {
			if key.CreatedAt.IsZero() {
				key.CreatedAt = now
			}
			key.UpdatedAt = now
			if _, err := stmt.Exec(key.Key, key.Name, key.UserID, key.Status, key.StartsAt, key.ExpiresAt, key.CreatedAt, key.UpdatedAt); err != nil {
				return fmt.Errorf("插入 API Key 失败: %w", err)
			}
		}
	} else {
		// 增量模式：使用 UPSERT
		stmt, err := tx.Prepare(`
		INSERT INTO api_keys (key, name, user_id, status, starts_at, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			name = excluded.name,
			user_id = excluded.user_id,
			status = excluded.status,
			starts_at = excluded.starts_at,
			expires_at = excluded.expires_at,
			updated_at = excluded.updated_at
		`)
		if err != nil {
			return fmt.Errorf("准备语句失败: %w", err)
		}
		defer func() {
			_ = stmt.Close()
		}()

		for _, key := range keys {
			if key.CreatedAt.IsZero() {
				key.CreatedAt = now
			}
			key.UpdatedAt = now
			if _, err := stmt.Exec(key.Key, key.Name, key.UserID, key.Status, key.StartsAt, key.ExpiresAt, key.CreatedAt, key.UpdatedAt); err != nil {
				return fmt.Errorf("插入/更新 API Key 失败: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	log.Printf("KeyStore: 已同步 %d 个 Key (模式: %s)", len(keys), mode)
	return nil
}

// Close 关闭数据库连接
func (s *KeyStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Exists 检查 Key 是否存在
// 参数：
//   - keyStr: API Key 字符串
//
// 返回：
//   - bool: 是否存在
func (s *KeyStore) Exists(keyStr string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	query := `SELECT COUNT(*) FROM api_keys WHERE key = ?`
	if err := s.db.QueryRow(query, keyStr).Scan(&count); err != nil {
		return false
	}
	return count > 0
}

// GetDB 获取数据库连接（供外部使用，如用量存储）
// 返回：
//   - *sql.DB: 数据库连接
func (s *KeyStore) GetDB() *sql.DB {
	return s.db
}
