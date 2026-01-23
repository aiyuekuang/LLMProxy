package proxy

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"llmproxy/internal/config"
)

// RequestLog 请求日志记录
type RequestLog struct {
	RequestID    string            `json:"request_id"`
	Timestamp    time.Time         `json:"timestamp"`
	ClientIP     string            `json:"client_ip"`
	Method       string            `json:"method"`
	Path         string            `json:"path"`
	Headers      map[string]string `json:"headers,omitempty"`
	RequestBody  string            `json:"request_body,omitempty"`
	ResponseBody string            `json:"response_body,omitempty"`
	StatusCode   int               `json:"status_code"`
	LatencyMs    int64             `json:"latency_ms"`
	BackendURL   string            `json:"backend_url"`
	APIKey       string            `json:"api_key,omitempty"`
	UserID       string            `json:"user_id,omitempty"`
	Model        string            `json:"model,omitempty"`
	IsStream     bool              `json:"is_stream"`
	Error        string            `json:"error,omitempty"`
}

// Logger 日志记录器
type Logger struct {
	requestCfg *config.RequestLoggingConfig
	accessCfg  *config.AccessLoggingConfig
	db         *sql.DB
	driver     string
	table      string
	accessFile *os.File
	mu         sync.Mutex
}

// NewLogger 创建日志记录器
func NewLogger(cfg *config.LoggingConfig, db *sql.DB, driver string) (*Logger, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, nil
	}

	logger := &Logger{
		requestCfg: cfg.Request,
		accessCfg:  cfg.Access,
		db:         db,
		driver:     driver,
	}

	// 初始化请求日志表
	if cfg.Request != nil && cfg.Request.Enabled && db != nil {
		table := cfg.Request.Table
		if table == "" {
			table = "request_logs"
		}
		logger.table = table

		if err := logger.initRequestLogTable(); err != nil {
			return nil, fmt.Errorf("初始化请求日志表失败: %w", err)
		}
		log.Printf("请求日志已启用，表: %s", table)
	}

	// 初始化访问日志文件
	if cfg.Access != nil && cfg.Access.Enabled {
		if cfg.Access.Output == "file" && cfg.Access.File != nil && cfg.Access.File.Path != "" {
			file, err := os.OpenFile(cfg.Access.File.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return nil, fmt.Errorf("打开访问日志文件失败: %w", err)
			}
			logger.accessFile = file
			log.Printf("访问日志已启用，文件: %s", cfg.Access.File.Path)
		} else {
			log.Println("访问日志已启用，输出: stdout")
		}
	}

	return logger, nil
}

// initRequestLogTable 初始化请求日志表
func (l *Logger) initRequestLogTable() error {
	if l.db == nil {
		return nil
	}

	var createSQL string
	switch l.driver {
	case "mysql":
		createSQL = fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id BIGINT AUTO_INCREMENT PRIMARY KEY,
				request_id VARCHAR(64),
				timestamp DATETIME NOT NULL,
				client_ip VARCHAR(64),
				method VARCHAR(10),
				path VARCHAR(256),
				headers TEXT,
				request_body MEDIUMTEXT,
				response_body MEDIUMTEXT,
				status_code INT,
				latency_ms BIGINT,
				backend_url VARCHAR(256),
				api_key VARCHAR(128),
				user_id VARCHAR(64),
				model VARCHAR(64),
				is_stream BOOLEAN DEFAULT FALSE,
				error TEXT,
				INDEX idx_timestamp (timestamp),
				INDEX idx_api_key (api_key),
				INDEX idx_user_id (user_id)
			)
		`, l.table)
	case "postgres":
		createSQL = fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id BIGSERIAL PRIMARY KEY,
				request_id VARCHAR(64),
				timestamp TIMESTAMP NOT NULL,
				client_ip VARCHAR(64),
				method VARCHAR(10),
				path VARCHAR(256),
				headers TEXT,
				request_body TEXT,
				response_body TEXT,
				status_code INT,
				latency_ms BIGINT,
				backend_url VARCHAR(256),
				api_key VARCHAR(128),
				user_id VARCHAR(64),
				model VARCHAR(64),
				is_stream BOOLEAN DEFAULT FALSE,
				error TEXT
			);
			CREATE INDEX IF NOT EXISTS idx_%s_timestamp ON %s(timestamp);
			CREATE INDEX IF NOT EXISTS idx_%s_api_key ON %s(api_key);
			CREATE INDEX IF NOT EXISTS idx_%s_user_id ON %s(user_id)
		`, l.table, l.table, l.table, l.table, l.table, l.table, l.table)
	case "sqlite":
		createSQL = fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				request_id TEXT,
				timestamp DATETIME NOT NULL,
				client_ip TEXT,
				method TEXT,
				path TEXT,
				headers TEXT,
				request_body TEXT,
				response_body TEXT,
				status_code INTEGER,
				latency_ms INTEGER,
				backend_url TEXT,
				api_key TEXT,
				user_id TEXT,
				model TEXT,
				is_stream INTEGER DEFAULT 0,
				error TEXT
			);
			CREATE INDEX IF NOT EXISTS idx_%s_timestamp ON %s(timestamp);
			CREATE INDEX IF NOT EXISTS idx_%s_api_key ON %s(api_key);
			CREATE INDEX IF NOT EXISTS idx_%s_user_id ON %s(user_id)
		`, l.table, l.table, l.table, l.table, l.table, l.table, l.table)
	default:
		return fmt.Errorf("不支持的数据库驱动: %s", l.driver)
	}

	_, err := l.db.Exec(createSQL)
	return err
}

// LogRequest 异步记录请求日志
func (l *Logger) LogRequest(reqLog *RequestLog) {
	if l == nil {
		return
	}

	// 异步写入请求日志到数据库
	if l.requestCfg != nil && l.requestCfg.Enabled && l.db != nil {
		go l.writeRequestLog(reqLog)
	}

	// 写入访问日志
	if l.accessCfg != nil && l.accessCfg.Enabled {
		go l.writeAccessLog(reqLog)
	}
}

// writeRequestLog 写入请求日志到数据库
func (l *Logger) writeRequestLog(reqLog *RequestLog) {
	if l.db == nil || reqLog == nil {
		return
	}

	// 序列化 headers
	headersJSON := "{}"
	if reqLog.Headers != nil {
		if b, err := json.Marshal(reqLog.Headers); err == nil {
			headersJSON = string(b)
		}
	}

	// 如果不包含 body，清空
	requestBody := reqLog.RequestBody
	responseBody := reqLog.ResponseBody
	if l.requestCfg != nil && !l.requestCfg.IncludeBody {
		requestBody = ""
		responseBody = ""
	}

	insertSQL := fmt.Sprintf(`
		INSERT INTO %s (
			request_id, timestamp, client_ip, method, path, headers,
			request_body, response_body, status_code, latency_ms,
			backend_url, api_key, user_id, model, is_stream, error
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, l.table)

	_, err := l.db.Exec(
		insertSQL,
		reqLog.RequestID, reqLog.Timestamp, reqLog.ClientIP, reqLog.Method, reqLog.Path, headersJSON,
		requestBody, responseBody, reqLog.StatusCode, reqLog.LatencyMs,
		reqLog.BackendURL, reqLog.APIKey, reqLog.UserID, reqLog.Model, reqLog.IsStream, reqLog.Error,
	)
	if err != nil {
		log.Printf("写入请求日志失败: %v", err)
	}
}

// writeAccessLog 写入访问日志
func (l *Logger) writeAccessLog(reqLog *RequestLog) {
	if reqLog == nil {
		return
	}

	var logLine string
	format := "combined"
	if l.accessCfg != nil && l.accessCfg.Format != "" {
		format = l.accessCfg.Format
	}

	switch format {
	case "json":
		// JSON 格式
		logEntry := map[string]interface{}{
			"timestamp":   reqLog.Timestamp.Format(time.RFC3339),
			"client_ip":   reqLog.ClientIP,
			"method":      reqLog.Method,
			"path":        reqLog.Path,
			"status_code": reqLog.StatusCode,
			"latency_ms":  reqLog.LatencyMs,
			"backend_url": reqLog.BackendURL,
			"api_key":     maskKey(reqLog.APIKey),
			"user_id":     reqLog.UserID,
			"model":       reqLog.Model,
			"is_stream":   reqLog.IsStream,
		}
		if reqLog.Error != "" {
			logEntry["error"] = reqLog.Error
		}
		if b, err := json.Marshal(logEntry); err == nil {
			logLine = string(b)
		}
	default:
		// Combined 格式（类似 nginx）
		// 192.168.1.1 - user_id [timestamp] "POST /v1/chat/completions" 200 123ms "backend_url"
		logLine = fmt.Sprintf(`%s - %s [%s] "%s %s" %d %dms "%s" model=%s stream=%v`,
			reqLog.ClientIP,
			defaultIfEmpty(reqLog.UserID, "-"),
			reqLog.Timestamp.Format("02/Jan/2006:15:04:05 -0700"),
			reqLog.Method,
			reqLog.Path,
			reqLog.StatusCode,
			reqLog.LatencyMs,
			reqLog.BackendURL,
			defaultIfEmpty(reqLog.Model, "-"),
			reqLog.IsStream,
		)
		if reqLog.Error != "" {
			logLine += fmt.Sprintf(" error=%q", reqLog.Error)
		}
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.accessFile != nil {
		_, _ = fmt.Fprintln(l.accessFile, logLine)
	} else {
		fmt.Println(logLine)
	}
}

// Close 关闭日志记录器
func (l *Logger) Close() error {
	if l == nil {
		return nil
	}
	if l.accessFile != nil {
		return l.accessFile.Close()
	}
	return nil
}

// ExtractClientIP 从请求中提取客户端 IP
func ExtractClientIP(r *http.Request) string {
	// 优先从 X-Forwarded-For 获取
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	// 其次从 X-Real-IP 获取
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// 最后使用 RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// ExtractHeaders 从请求中提取需要记录的 headers
func ExtractHeaders(r *http.Request) map[string]string {
	headers := make(map[string]string)
	// 只记录部分重要的 headers
	importantHeaders := []string{
		"Content-Type",
		"User-Agent",
		"X-Request-ID",
		"X-Forwarded-For",
		"X-Real-IP",
	}
	for _, h := range importantHeaders {
		if v := r.Header.Get(h); v != "" {
			headers[h] = v
		}
	}
	return headers
}

// ExtractModel 从请求体中提取模型名称
func ExtractModel(body []byte) string {
	var req struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &req); err == nil {
		return req.Model
	}
	return ""
}

// maskKey 掩码 API Key
func maskKey(key string) string {
	if key == "" {
		return "-"
	}
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "***" + key[len(key)-4:]
}

// defaultIfEmpty 如果为空则返回默认值
func defaultIfEmpty(s, defaultVal string) string {
	if s == "" {
		return defaultVal
	}
	return s
}
