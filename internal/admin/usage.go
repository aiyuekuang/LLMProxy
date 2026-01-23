package admin

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// UsageRecord 用量记录
type UsageRecord struct {
	ID               int64     `json:"id"`
	RequestID        string    `json:"request_id,omitempty"`
	APIKey           string    `json:"api_key,omitempty"`
	UserID           string    `json:"user_id,omitempty"`
	Model            string    `json:"model,omitempty"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	Endpoint         string    `json:"endpoint,omitempty"`
	BackendURL       string    `json:"backend_url,omitempty"`
	StatusCode       int       `json:"status_code"`
	LatencyMs        int64     `json:"latency_ms"`
	Streaming        bool      `json:"streaming"`
	CreatedAt        time.Time `json:"created_at"`
}

// UsageStore 用量存储
type UsageStore struct {
	db            *sql.DB
	retentionDays int // 保留天数，0=永久
}

// NewUsageStore 创建用量存储
// 使用与 KeyStore 相同的数据库连接
func NewUsageStore(db *sql.DB, retentionDays int) (*UsageStore, error) {
	store := &UsageStore{
		db:            db,
		retentionDays: retentionDays,
	}

	// 初始化表结构
	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("初始化用量表结构失败: %w", err)
	}

	log.Printf("UsageStore 已初始化，保留天数: %d (0=永久)", retentionDays)
	return store, nil
}

// initSchema 初始化数据库表结构
func (s *UsageStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS usage_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		request_id TEXT,
		api_key TEXT,
		user_id TEXT,
		model TEXT,
		prompt_tokens INTEGER DEFAULT 0,
		completion_tokens INTEGER DEFAULT 0,
		total_tokens INTEGER DEFAULT 0,
		endpoint TEXT,
		backend_url TEXT,
		status_code INTEGER,
		latency_ms INTEGER,
		streaming BOOLEAN DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_usage_api_key ON usage_records(api_key);
	CREATE INDEX IF NOT EXISTS idx_usage_user_id ON usage_records(user_id);
	CREATE INDEX IF NOT EXISTS idx_usage_created_at ON usage_records(created_at);
	CREATE INDEX IF NOT EXISTS idx_usage_model ON usage_records(model);
	`
	_, err := s.db.Exec(schema)
	return err
}

// Record 记录用量
func (s *UsageStore) Record(record *UsageRecord) error {
	query := `
	INSERT INTO usage_records (
		request_id, api_key, user_id, model,
		prompt_tokens, completion_tokens, total_tokens,
		endpoint, backend_url, status_code, latency_ms, streaming, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}

	_, err := s.db.Exec(query,
		record.RequestID, record.APIKey, record.UserID, record.Model,
		record.PromptTokens, record.CompletionTokens, record.TotalTokens,
		record.Endpoint, record.BackendURL, record.StatusCode, record.LatencyMs,
		record.Streaming, record.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("记录用量失败: %w", err)
	}

	return nil
}

// UsageQueryParams 用量查询参数
type UsageQueryParams struct {
	APIKey    string     // 按 API Key 筛选
	UserID    string     // 按用户 ID 筛选
	Model     string     // 按模型筛选
	StartTime *time.Time // 开始时间
	EndTime   *time.Time // 结束时间
	Offset    int        // 偏移量
	Limit     int        // 限制数量
}

// Query 查询用量记录
func (s *UsageStore) Query(params *UsageQueryParams) ([]*UsageRecord, int, error) {
	// 构建查询条件
	where := "1=1"
	args := []interface{}{}

	if params.APIKey != "" {
		where += " AND api_key = ?"
		args = append(args, params.APIKey)
	}
	if params.UserID != "" {
		where += " AND user_id = ?"
		args = append(args, params.UserID)
	}
	if params.Model != "" {
		where += " AND model = ?"
		args = append(args, params.Model)
	}
	if params.StartTime != nil {
		where += " AND created_at >= ?"
		args = append(args, *params.StartTime)
	}
	if params.EndTime != nil {
		where += " AND created_at <= ?"
		args = append(args, *params.EndTime)
	}

	// 查询总数
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM usage_records WHERE %s", where)
	var total int
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("查询用量总数失败: %w", err)
	}

	// 设置默认值
	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 1000 {
		limit = 1000
	}

	// 查询列表
	query := fmt.Sprintf(`
		SELECT id, request_id, api_key, user_id, model,
			prompt_tokens, completion_tokens, total_tokens,
			endpoint, backend_url, status_code, latency_ms, streaming, created_at
		FROM usage_records
		WHERE %s
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, where)

	args = append(args, limit, params.Offset)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询用量列表失败: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var records []*UsageRecord
	for rows.Next() {
		var r UsageRecord
		var requestID, apiKey, userID, model, endpoint, backendURL sql.NullString
		if err := rows.Scan(
			&r.ID, &requestID, &apiKey, &userID, &model,
			&r.PromptTokens, &r.CompletionTokens, &r.TotalTokens,
			&endpoint, &backendURL, &r.StatusCode, &r.LatencyMs, &r.Streaming, &r.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("扫描用量记录失败: %w", err)
		}
		if requestID.Valid {
			r.RequestID = requestID.String
		}
		if apiKey.Valid {
			r.APIKey = apiKey.String
		}
		if userID.Valid {
			r.UserID = userID.String
		}
		if model.Valid {
			r.Model = model.String
		}
		if endpoint.Valid {
			r.Endpoint = endpoint.String
		}
		if backendURL.Valid {
			r.BackendURL = backendURL.String
		}
		records = append(records, &r)
	}

	return records, total, nil
}

// UsageStats 用量统计
type UsageStats struct {
	TotalRequests    int64 `json:"total_requests"`
	TotalTokens      int64 `json:"total_tokens"`
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	AvgLatencyMs     int64 `json:"avg_latency_ms"`
}

// Stats 统计用量
func (s *UsageStore) Stats(params *UsageQueryParams) (*UsageStats, error) {
	// 构建查询条件
	where := "1=1"
	args := []interface{}{}

	if params.APIKey != "" {
		where += " AND api_key = ?"
		args = append(args, params.APIKey)
	}
	if params.UserID != "" {
		where += " AND user_id = ?"
		args = append(args, params.UserID)
	}
	if params.Model != "" {
		where += " AND model = ?"
		args = append(args, params.Model)
	}
	if params.StartTime != nil {
		where += " AND created_at >= ?"
		args = append(args, *params.StartTime)
	}
	if params.EndTime != nil {
		where += " AND created_at <= ?"
		args = append(args, *params.EndTime)
	}

	query := fmt.Sprintf(`
		SELECT 
			COUNT(*) as total_requests,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			COALESCE(SUM(prompt_tokens), 0) as prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) as completion_tokens,
			COALESCE(AVG(latency_ms), 0) as avg_latency_ms
		FROM usage_records
		WHERE %s
	`, where)

	var stats UsageStats
	if err := s.db.QueryRow(query, args...).Scan(
		&stats.TotalRequests,
		&stats.TotalTokens,
		&stats.PromptTokens,
		&stats.CompletionTokens,
		&stats.AvgLatencyMs,
	); err != nil {
		return nil, fmt.Errorf("统计用量失败: %w", err)
	}

	return &stats, nil
}

// Cleanup 清理过期数据
func (s *UsageStore) Cleanup() (int64, error) {
	if s.retentionDays <= 0 {
		return 0, nil // 不清理
	}

	cutoff := time.Now().AddDate(0, 0, -s.retentionDays)
	query := `DELETE FROM usage_records WHERE created_at < ?`
	result, err := s.db.Exec(query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("清理用量数据失败: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows > 0 {
		log.Printf("UsageStore: 已清理 %d 条过期用量记录", rows)
	}
	return rows, nil
}

// GetDB 获取数据库连接（供外部使用）
func (s *UsageStore) GetDB() *sql.DB {
	return s.db
}
