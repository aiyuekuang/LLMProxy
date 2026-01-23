package admin

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Server Admin API 服务器
type Server struct {
	keyStore *KeyStore    // Key 存储
	token    string       // 访问令牌
	listen   string       // 监听地址
	server   *http.Server // HTTP 服务器
}

// NewServer 创建 Admin API 服务器
// 参数：
//   - keyStore: KeyStore 实例
//   - token: 访问令牌
//   - listen: 监听地址（可选，默认 :8080）
//
// 返回：
//   - *Server: Server 实例
func NewServer(keyStore *KeyStore, token string, listen string) *Server {
	if listen == "" {
		listen = ":8080"
	}
	return &Server{
		keyStore: keyStore,
		token:    token,
		listen:   listen,
	}
}

// Start 启动 Admin API 服务器
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// 注册路由
	mux.HandleFunc("/admin/keys/create", s.authMiddleware(s.handleCreate))
	mux.HandleFunc("/admin/keys/update", s.authMiddleware(s.handleUpdate))
	mux.HandleFunc("/admin/keys/delete", s.authMiddleware(s.handleDelete))
	mux.HandleFunc("/admin/keys/get", s.authMiddleware(s.handleGet))
	mux.HandleFunc("/admin/keys/list", s.authMiddleware(s.handleList))
	mux.HandleFunc("/admin/keys/sync", s.authMiddleware(s.handleSync))

	s.server = &http.Server{
		Addr:         s.listen,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("Admin API 服务器启动，监听: %s", s.listen)
	return s.server.ListenAndServe()
}

// Stop 停止服务器
func (s *Server) Stop() error {
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

// RegisterRoutes 将 Admin API 路由注册到外部 ServeMux
// 用于将 Admin API 挂载到主服务器上
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/admin/keys/create", s.authMiddleware(s.handleCreate))
	mux.HandleFunc("/admin/keys/update", s.authMiddleware(s.handleUpdate))
	mux.HandleFunc("/admin/keys/delete", s.authMiddleware(s.handleDelete))
	mux.HandleFunc("/admin/keys/get", s.authMiddleware(s.handleGet))
	mux.HandleFunc("/admin/keys/list", s.authMiddleware(s.handleList))
	mux.HandleFunc("/admin/keys/sync", s.authMiddleware(s.handleSync))
	log.Println("Admin API 路由已注册到主服务器")
}

// authMiddleware Token 鉴权中间件
func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 检查请求方法
		if r.Method != http.MethodPost {
			s.writeError(w, http.StatusMethodNotAllowed, "只允许 POST 请求")
			return
		}

		// 检查 Token
		token := r.Header.Get("X-Admin-Token")
		if token == "" {
			s.writeError(w, http.StatusUnauthorized, "缺少 X-Admin-Token 头")
			return
		}
		if token != s.token {
			s.writeError(w, http.StatusForbidden, "无效的 Token")
			return
		}

		next(w, r)
	}
}

// ============================================================
//                    请求/响应结构
// ============================================================

// CreateRequest 创建 Key 请求
type CreateRequest struct {
	Key       string `json:"key"`                  // API Key
	Name      string `json:"name,omitempty"`       // 名称/备注
	UserID    string `json:"user_id,omitempty"`    // 用户标识
	Status    int    `json:"status"`               // 状态: 0=active, 1=disabled, 2=quota_exceeded, 3=expired
	StartsAt  string `json:"starts_at"`            // 开始时间（RFC3339 格式，必填）
	ExpiresAt string `json:"expires_at,omitempty"` // 过期时间（RFC3339 格式）
}

// UpdateRequest 更新 Key 请求
type UpdateRequest struct {
	Key       string  `json:"key"`                  // API Key
	Name      *string `json:"name,omitempty"`       // 名称/备注（可选）
	UserID    *string `json:"user_id,omitempty"`    // 用户标识（可选）
	Status    *int    `json:"status,omitempty"`     // 状态（可选）
	StartsAt  *string `json:"starts_at,omitempty"`  // 开始时间（可选，空字符串表示清除）
	ExpiresAt *string `json:"expires_at,omitempty"` // 过期时间（可选，空字符串表示清除）
}

// DeleteRequest 删除 Key 请求
type DeleteRequest struct {
	Key string `json:"key"` // API Key
}

// GetRequest 获取 Key 请求
type GetRequest struct {
	Key string `json:"key"` // API Key
}

// ListRequest 列表请求
type ListRequest struct {
	Offset int `json:"offset"` // 偏移量
	Limit  int `json:"limit"`  // 限制数量
}

// SyncRequest 同步请求
type SyncRequest struct {
	Keys []SyncKeyItem `json:"keys"` // Key 列表
	Mode string        `json:"mode"` // 同步模式: full=全量覆盖, incremental=增量更新
}

// SyncKeyItem 同步的单个 Key
type SyncKeyItem struct {
	Key       string `json:"key"`                  // API Key
	Name      string `json:"name,omitempty"`       // 名称/备注
	UserID    string `json:"user_id,omitempty"`    // 用户标识
	Status    int    `json:"status"`               // 状态
	StartsAt  string `json:"starts_at"`            // 开始时间（必填）
	ExpiresAt string `json:"expires_at,omitempty"` // 过期时间
}

// Response 通用响应
type Response struct {
	Success bool        `json:"success"`         // 是否成功
	Message string      `json:"message"`         // 消息
	Data    interface{} `json:"data,omitempty"`  // 数据
	Error   string      `json:"error,omitempty"` // 错误信息
}

// ListResponse 列表响应数据
type ListResponse struct {
	Keys  []*APIKey `json:"keys"`  // Key 列表
	Total int       `json:"total"` // 总数
}

// ============================================================
//                    处理函数
// ============================================================

// handleCreate 创建 Key
func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "请求解析失败: "+err.Error())
		return
	}

	if req.Key == "" {
		s.writeError(w, http.StatusBadRequest, "key 不能为空")
		return
	}

	// starts_at 必填
	if req.StartsAt == "" {
		s.writeError(w, http.StatusBadRequest, "starts_at 不能为空")
		return
	}

	// 检查是否已存在
	if s.keyStore.Exists(req.Key) {
		s.writeError(w, http.StatusConflict, "Key 已存在")
		return
	}

	// 构建 APIKey
	key := &APIKey{
		Key:    req.Key,
		Name:   req.Name,
		UserID: req.UserID,
		Status: KeyStatus(req.Status),
	}

	// 解析开始时间（必填）
	startsAt, err := time.Parse(time.RFC3339, req.StartsAt)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "starts_at 格式错误，请使用 RFC3339 格式")
		return
	}
	key.StartsAt = &startsAt

	// 解析过期时间
	if req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "expires_at 格式错误，请使用 RFC3339 格式")
			return
		}
		key.ExpiresAt = &t
	}

	// 创建
	if err := s.keyStore.Create(key); err != nil {
		s.writeError(w, http.StatusInternalServerError, "创建失败: "+err.Error())
		return
	}

	s.writeSuccess(w, "创建成功", key)
}

// handleUpdate 更新 Key
func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "请求解析失败: "+err.Error())
		return
	}

	if req.Key == "" {
		s.writeError(w, http.StatusBadRequest, "key 不能为空")
		return
	}

	// 获取现有 Key
	key, err := s.keyStore.Get(req.Key)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}
	if key == nil {
		s.writeError(w, http.StatusNotFound, "Key 不存在")
		return
	}

	// 更新字段
	if req.Name != nil {
		key.Name = *req.Name
	}
	if req.UserID != nil {
		key.UserID = *req.UserID
	}
	if req.Status != nil {
		key.Status = KeyStatus(*req.Status)
	}
	if req.StartsAt != nil {
		if *req.StartsAt == "" {
			// 清除开始时间
			key.StartsAt = nil
		} else {
			t, err := time.Parse(time.RFC3339, *req.StartsAt)
			if err != nil {
				s.writeError(w, http.StatusBadRequest, "starts_at 格式错误，请使用 RFC3339 格式")
				return
			}
			key.StartsAt = &t
		}
	}
	if req.ExpiresAt != nil {
		if *req.ExpiresAt == "" {
			// 清除过期时间
			key.ExpiresAt = nil
		} else {
			t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
			if err != nil {
				s.writeError(w, http.StatusBadRequest, "expires_at 格式错误，请使用 RFC3339 格式")
				return
			}
			key.ExpiresAt = &t
		}
	}

	// 更新
	if err := s.keyStore.Update(key); err != nil {
		s.writeError(w, http.StatusInternalServerError, "更新失败: "+err.Error())
		return
	}

	s.writeSuccess(w, "更新成功", key)
}

// handleDelete 删除 Key
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	var req DeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "请求解析失败: "+err.Error())
		return
	}

	if req.Key == "" {
		s.writeError(w, http.StatusBadRequest, "key 不能为空")
		return
	}

	// 删除
	if err := s.keyStore.Delete(req.Key); err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	s.writeSuccess(w, "删除成功", nil)
}

// handleGet 获取 Key
func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	var req GetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "请求解析失败: "+err.Error())
		return
	}

	if req.Key == "" {
		s.writeError(w, http.StatusBadRequest, "key 不能为空")
		return
	}

	// 查询
	key, err := s.keyStore.Get(req.Key)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}
	if key == nil {
		s.writeError(w, http.StatusNotFound, "Key 不存在")
		return
	}

	s.writeSuccess(w, "查询成功", key)
}

// handleList 列出 Key
func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	var req ListRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "请求解析失败: "+err.Error())
		return
	}

	// 默认值
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	// 查询
	keys, total, err := s.keyStore.List(req.Offset, req.Limit)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}

	s.writeSuccess(w, "查询成功", ListResponse{
		Keys:  keys,
		Total: total,
	})
}

// handleSync 批量同步 Key
// 支持两种模式:
//   - full: 全量覆盖（默认），先清空所有 Key 再插入
//   - incremental: 增量更新，存在则更新，不存在则插入
func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	var req SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "请求解析失败: "+err.Error())
		return
	}

	// 验证同步模式
	mode := SyncModeFull // 默认全量
	if req.Mode != "" {
		switch req.Mode {
		case "full":
			mode = SyncModeFull
		case "incremental":
			mode = SyncModeIncremental
		default:
			s.writeError(w, http.StatusBadRequest, "mode 无效，应为 full 或 incremental")
			return
		}
	}

	// 转换
	keys := make([]*APIKey, 0, len(req.Keys))
	for i, item := range req.Keys {
		if item.Key == "" {
			s.writeError(w, http.StatusBadRequest, fmt.Sprintf("keys[%d].key 不能为空", i))
			return
		}

		// starts_at 必填
		if item.StartsAt == "" {
			s.writeError(w, http.StatusBadRequest, fmt.Sprintf("keys[%d].starts_at 不能为空", i))
			return
		}

		key := &APIKey{
			Key:    item.Key,
			Name:   item.Name,
			UserID: item.UserID,
			Status: KeyStatus(item.Status),
		}

		// 解析开始时间
		startsAt, err := time.Parse(time.RFC3339, item.StartsAt)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, fmt.Sprintf("keys[%d].starts_at 格式错误", i))
			return
		}
		key.StartsAt = &startsAt

		if item.ExpiresAt != "" {
			t, err := time.Parse(time.RFC3339, item.ExpiresAt)
			if err != nil {
				s.writeError(w, http.StatusBadRequest, fmt.Sprintf("keys[%d].expires_at 格式错误", i))
				return
			}
			key.ExpiresAt = &t
		}

		keys = append(keys, key)
	}

	// 同步
	if err := s.keyStore.SyncWithMode(keys, mode); err != nil {
		s.writeError(w, http.StatusInternalServerError, "同步失败: "+err.Error())
		return
	}

	modeStr := "全量覆盖"
	if mode == SyncModeIncremental {
		modeStr = "增量更新"
	}
	s.writeSuccess(w, fmt.Sprintf("同步成功，共 %d 个 Key（%s）", len(keys), modeStr), nil)
}

// ============================================================
//                    辅助函数
// ============================================================

// writeSuccess 写入成功响应
func (s *Server) writeSuccess(w http.ResponseWriter, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(Response{
		Success: true,
		Message: message,
		Data:    data,
	}); err != nil {
		log.Printf("写入响应失败: %v", err)
	}
}

// writeError 写入错误响应
func (s *Server) writeError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(Response{
		Success: false,
		Error:   message,
	}); err != nil {
		log.Printf("写入错误响应失败: %v", err)
	}
}

// GetKeyStore 获取 KeyStore 实例
func (s *Server) GetKeyStore() *KeyStore {
	return s.keyStore
}
