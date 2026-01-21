---
name: go-backend
description: Go backend development best practices for LLMProxy. Use when writing Go code, handlers, middleware, or backend services.
---

# Go Backend Skill

Best practices for Go backend development, specifically tailored for LLMProxy architecture.

## When to Use This Skill

- Writing Go HTTP handlers
- Implementing middleware
- Database operations
- Error handling and logging
- Performance optimization

---

# 📁 Project Structure

```
.
├── cmd/
│   └── main.go              # Application entry point
├── internal/
│   ├── auth/                # Authentication logic
│   ├── config/              # Configuration management
│   ├── database/            # Database layer
│   ├── lb/                  # Load balancing
│   ├── middleware/          # HTTP middleware
│   ├── proxy/               # Proxy handlers
│   └── metrics/             # Monitoring
├── pkg/                     # Public packages
├── deployments/             # Deployment configs
└── docs/                    # Documentation
```

---

# 🔧 Handler Patterns

## Standard Handler Structure

```go
func (h *Handler) HandleRequest(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // 1. Parse and validate input
    var req RequestDTO
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        h.respondError(w, http.StatusBadRequest, "invalid request body")
        return
    }
    
    if err := h.validator.Validate(req); err != nil {
        h.respondError(w, http.StatusBadRequest, err.Error())
        return
    }
    
    // 2. Execute business logic
    result, err := h.service.Process(ctx, req)
    if err != nil {
        h.handleError(w, err)
        return
    }
    
    // 3. Return response
    h.respondJSON(w, http.StatusOK, result)
}
```

## Response Helpers

```go
type Response struct {
    Success bool        `json:"success"`
    Data    interface{} `json:"data,omitempty"`
    Error   *ErrorInfo  `json:"error,omitempty"`
}

type ErrorInfo struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}

func (h *Handler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(Response{
        Success: true,
        Data:    data,
    })
}

func (h *Handler) respondError(w http.ResponseWriter, status int, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(Response{
        Success: false,
        Error: &ErrorInfo{
            Code:    http.StatusText(status),
            Message: message,
        },
    })
}
```

---

# ⚠️ Error Handling

## Custom Error Types

```go
type AppError struct {
    Code       string
    Message    string
    HTTPStatus int
    Err        error
}

func (e *AppError) Error() string {
    if e.Err != nil {
        return fmt.Sprintf("%s: %v", e.Message, e.Err)
    }
    return e.Message
}

func (e *AppError) Unwrap() error {
    return e.Err
}

// Predefined errors
var (
    ErrNotFound      = &AppError{Code: "NOT_FOUND", HTTPStatus: 404}
    ErrUnauthorized  = &AppError{Code: "UNAUTHORIZED", HTTPStatus: 401}
    ErrForbidden     = &AppError{Code: "FORBIDDEN", HTTPStatus: 403}
    ErrBadRequest    = &AppError{Code: "BAD_REQUEST", HTTPStatus: 400}
    ErrInternal      = &AppError{Code: "INTERNAL_ERROR", HTTPStatus: 500}
)

func NewNotFoundError(resource string) *AppError {
    return &AppError{
        Code:       "NOT_FOUND",
        Message:    fmt.Sprintf("%s not found", resource),
        HTTPStatus: http.StatusNotFound,
    }
}
```

## Error Handling in Handlers

```go
func (h *Handler) handleError(w http.ResponseWriter, err error) {
    var appErr *AppError
    if errors.As(err, &appErr) {
        h.respondError(w, appErr.HTTPStatus, appErr.Message)
        return
    }
    
    // Log unexpected errors
    h.logger.Error("unexpected error", zap.Error(err))
    h.respondError(w, http.StatusInternalServerError, "internal server error")
}
```

---

# 🔒 Middleware Patterns

## Middleware Chain

```go
type Middleware func(http.Handler) http.Handler

func Chain(middlewares ...Middleware) Middleware {
    return func(final http.Handler) http.Handler {
        for i := len(middlewares) - 1; i >= 0; i-- {
            final = middlewares[i](final)
        }
        return final
    }
}

// Usage
handler := Chain(
    LoggingMiddleware,
    RecoveryMiddleware,
    AuthMiddleware,
    RateLimitMiddleware,
)(finalHandler)
```

## Logging Middleware

```go
func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        // Wrap response writer to capture status
        wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}
        
        next.ServeHTTP(wrapped, r)
        
        logger.Info("request",
            zap.String("method", r.Method),
            zap.String("path", r.URL.Path),
            zap.Int("status", wrapped.status),
            zap.Duration("duration", time.Since(start)),
        )
    })
}

type responseWriter struct {
    http.ResponseWriter
    status int
}

func (w *responseWriter) WriteHeader(status int) {
    w.status = status
    w.ResponseWriter.WriteHeader(status)
}
```

## Recovery Middleware

```go
func RecoveryMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                logger.Error("panic recovered",
                    zap.Any("error", err),
                    zap.String("stack", string(debug.Stack())),
                )
                http.Error(w, "Internal Server Error", http.StatusInternalServerError)
            }
        }()
        next.ServeHTTP(w, r)
    })
}
```

## Auth Middleware

```go
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        if token == "" {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        
        // Validate token
        claims, err := validateToken(strings.TrimPrefix(token, "Bearer "))
        if err != nil {
            http.Error(w, "invalid token", http.StatusUnauthorized)
            return
        }
        
        // Add claims to context
        ctx := context.WithValue(r.Context(), userClaimsKey, claims)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

---

# 💾 Database Patterns

## Repository Pattern

```go
type UserRepository interface {
    GetByID(ctx context.Context, id string) (*User, error)
    Create(ctx context.Context, user *User) error
    Update(ctx context.Context, user *User) error
    Delete(ctx context.Context, id string) error
    List(ctx context.Context, opts ListOptions) ([]*User, error)
}

type userRepository struct {
    db *sql.DB
}

func (r *userRepository) GetByID(ctx context.Context, id string) (*User, error) {
    query := `SELECT id, name, email, created_at FROM users WHERE id = $1`
    
    var user User
    err := r.db.QueryRowContext(ctx, query, id).Scan(
        &user.ID, &user.Name, &user.Email, &user.CreatedAt,
    )
    if err == sql.ErrNoRows {
        return nil, ErrNotFound
    }
    if err != nil {
        return nil, fmt.Errorf("query user: %w", err)
    }
    
    return &user, nil
}
```

## Transaction Handling

```go
func (r *userRepository) CreateWithProfile(ctx context.Context, user *User, profile *Profile) error {
    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("begin transaction: %w", err)
    }
    defer tx.Rollback()  // No-op if committed
    
    // Insert user
    _, err = tx.ExecContext(ctx, 
        `INSERT INTO users (id, name, email) VALUES ($1, $2, $3)`,
        user.ID, user.Name, user.Email,
    )
    if err != nil {
        return fmt.Errorf("insert user: %w", err)
    }
    
    // Insert profile
    _, err = tx.ExecContext(ctx,
        `INSERT INTO profiles (user_id, bio) VALUES ($1, $2)`,
        user.ID, profile.Bio,
    )
    if err != nil {
        return fmt.Errorf("insert profile: %w", err)
    }
    
    if err := tx.Commit(); err != nil {
        return fmt.Errorf("commit transaction: %w", err)
    }
    
    return nil
}
```

---

# ⚡ Performance Best Practices

## Connection Pooling

```go
db, err := sql.Open("postgres", connString)
if err != nil {
    return err
}

// Configure pool
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(5 * time.Minute)
db.SetConnMaxIdleTime(1 * time.Minute)
```

## Context Timeouts

```go
func (h *Handler) HandleRequest(w http.ResponseWriter, r *http.Request) {
    // Add timeout to context
    ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
    defer cancel()
    
    result, err := h.service.Process(ctx)
    if errors.Is(err, context.DeadlineExceeded) {
        h.respondError(w, http.StatusGatewayTimeout, "request timeout")
        return
    }
    // ...
}
```

## Sync Pool for Reusable Objects

```go
var bufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

func processData(data []byte) {
    buf := bufferPool.Get().(*bytes.Buffer)
    defer func() {
        buf.Reset()
        bufferPool.Put(buf)
    }()
    
    // Use buffer...
}
```

---

# 🧪 Testing Patterns

```go
func TestHandler_CreateUser(t *testing.T) {
    // Setup
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    
    mockService := NewMockUserService(ctrl)
    handler := NewHandler(mockService)
    
    // Expectations
    mockService.EXPECT().
        Create(gomock.Any(), gomock.Any()).
        Return(&User{ID: "123", Name: "Test"}, nil)
    
    // Request
    body := `{"name": "Test", "email": "test@example.com"}`
    req := httptest.NewRequest("POST", "/users", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    
    // Execute
    handler.CreateUser(w, req)
    
    // Assert
    assert.Equal(t, http.StatusCreated, w.Code)
}
```

---

# 📚 References

- [Effective Go](https://golang.org/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
