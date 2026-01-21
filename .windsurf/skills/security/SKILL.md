---
name: security
description: Security best practices for web applications. Use when implementing authentication, authorization, input validation, or reviewing code for vulnerabilities.
---

# Security Skill

Security guidelines based on OWASP, Trail of Bits, and industry best practices.

## When to Use This Skill

- Implementing authentication/authorization
- Validating and sanitizing user input
- Reviewing code for security vulnerabilities
- Configuring secure deployments
- Handling sensitive data

---

# 🔐 Authentication

## Password Handling

```go
import "golang.org/x/crypto/bcrypt"

// Hash password before storing
func hashPassword(password string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    return string(bytes), err
}

// Verify password
func checkPassword(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}
```

## JWT Token Handling

```go
import "github.com/golang-jwt/jwt/v5"

type Claims struct {
    UserID string `json:"user_id"`
    Role   string `json:"role"`
    jwt.RegisteredClaims
}

func generateToken(userID, role string, secret []byte) (string, error) {
    claims := Claims{
        UserID: userID,
        Role:   role,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            Issuer:    "llmproxy",
        },
    }
    
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(secret)
}

func validateToken(tokenString string, secret []byte) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
        if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
        }
        return secret, nil
    })
    
    if err != nil {
        return nil, err
    }
    
    if claims, ok := token.Claims.(*Claims); ok && token.Valid {
        return claims, nil
    }
    
    return nil, fmt.Errorf("invalid token")
}
```

## Secure Session Management

- Use secure, httpOnly cookies
- Implement session timeout
- Regenerate session ID after login
- Invalidate session on logout

```go
http.SetCookie(w, &http.Cookie{
    Name:     "session",
    Value:    sessionToken,
    HttpOnly: true,           // Not accessible via JavaScript
    Secure:   true,           // HTTPS only
    SameSite: http.SameSiteStrictMode,
    MaxAge:   3600,           // 1 hour
    Path:     "/",
})
```

---

# 🛡️ Authorization

## Role-Based Access Control (RBAC)

```go
type Permission string

const (
    PermissionRead   Permission = "read"
    PermissionWrite  Permission = "write"
    PermissionDelete Permission = "delete"
    PermissionAdmin  Permission = "admin"
)

type Role struct {
    Name        string
    Permissions []Permission
}

var roles = map[string]Role{
    "viewer": {Name: "viewer", Permissions: []Permission{PermissionRead}},
    "editor": {Name: "editor", Permissions: []Permission{PermissionRead, PermissionWrite}},
    "admin":  {Name: "admin", Permissions: []Permission{PermissionRead, PermissionWrite, PermissionDelete, PermissionAdmin}},
}

func hasPermission(userRole string, required Permission) bool {
    role, ok := roles[userRole]
    if !ok {
        return false
    }
    for _, p := range role.Permissions {
        if p == required {
            return true
        }
    }
    return false
}
```

## Authorization Middleware

```go
func RequirePermission(permission Permission) Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            claims := getUserClaims(r.Context())
            if claims == nil {
                http.Error(w, "unauthorized", http.StatusUnauthorized)
                return
            }
            
            if !hasPermission(claims.Role, permission) {
                http.Error(w, "forbidden", http.StatusForbidden)
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }
}
```

---

# 🔍 Input Validation

## Validation Rules

```go
import "github.com/go-playground/validator/v10"

type CreateUserRequest struct {
    Name     string `json:"name" validate:"required,min=2,max=100"`
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8,max=72"`
    Age      int    `json:"age" validate:"omitempty,min=0,max=150"`
}

var validate = validator.New()

func validateRequest(req interface{}) error {
    if err := validate.Struct(req); err != nil {
        var validationErrors validator.ValidationErrors
        if errors.As(err, &validationErrors) {
            // Convert to user-friendly messages
            return formatValidationErrors(validationErrors)
        }
        return err
    }
    return nil
}
```

## SQL Injection Prevention

```go
// ❌ NEVER: String concatenation
query := "SELECT * FROM users WHERE id = " + userInput

// ✅ ALWAYS: Parameterized queries
query := "SELECT * FROM users WHERE id = $1"
row := db.QueryRow(query, userInput)

// ✅ With named parameters (sqlx)
query := "SELECT * FROM users WHERE name = :name AND status = :status"
rows, err := db.NamedQuery(query, map[string]interface{}{
    "name":   name,
    "status": status,
})
```

## XSS Prevention

```go
import "html"

// Escape HTML in output
func sanitizeOutput(input string) string {
    return html.EscapeString(input)
}

// In templates, use text/template for auto-escaping
// or html/template for HTML-aware escaping
```

```typescript
// React automatically escapes by default
// ❌ Dangerous: bypasses escaping
<div dangerouslySetInnerHTML={{ __html: userInput }} />

// ✅ Safe: auto-escaped
<div>{userInput}</div>
```

---

# 🔑 Secrets Management

## Environment Variables

```go
// ❌ NEVER: Hardcoded secrets
const apiKey = "sk-1234567890abcdef"

// ✅ ALWAYS: Environment variables
apiKey := os.Getenv("API_KEY")
if apiKey == "" {
    log.Fatal("API_KEY environment variable required")
}
```

## Configuration Structure

```go
type Config struct {
    Database struct {
        Host     string `env:"DB_HOST" envDefault:"localhost"`
        Port     int    `env:"DB_PORT" envDefault:"5432"`
        Password string `env:"DB_PASSWORD,required"`
    }
    JWT struct {
        Secret     string        `env:"JWT_SECRET,required"`
        Expiration time.Duration `env:"JWT_EXPIRATION" envDefault:"24h"`
    }
}
```

## .gitignore Secrets

```gitignore
# Environment files
.env
.env.local
.env.*.local

# Config files with secrets
config.yaml
secrets/

# Key files
*.pem
*.key
```

---

# 🌐 HTTP Security Headers

```go
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Prevent MIME sniffing
        w.Header().Set("X-Content-Type-Options", "nosniff")
        
        // Prevent clickjacking
        w.Header().Set("X-Frame-Options", "DENY")
        
        // Enable XSS filter
        w.Header().Set("X-XSS-Protection", "1; mode=block")
        
        // Content Security Policy
        w.Header().Set("Content-Security-Policy", 
            "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'")
        
        // Strict Transport Security (HTTPS only)
        w.Header().Set("Strict-Transport-Security", 
            "max-age=31536000; includeSubDomains")
        
        // Referrer Policy
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
        
        next.ServeHTTP(w, r)
    })
}
```

---

# ⚡ Rate Limiting

```go
import "golang.org/x/time/rate"

type RateLimiter struct {
    limiters map[string]*rate.Limiter
    mu       sync.RWMutex
    rate     rate.Limit
    burst    int
}

func NewRateLimiter(r rate.Limit, b int) *RateLimiter {
    return &RateLimiter{
        limiters: make(map[string]*rate.Limiter),
        rate:     r,
        burst:    b,
    }
}

func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    
    limiter, exists := rl.limiters[key]
    if !exists {
        limiter = rate.NewLimiter(rl.rate, rl.burst)
        rl.limiters[key] = limiter
    }
    
    return limiter
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ip := getClientIP(r)
        limiter := rl.getLimiter(ip)
        
        if !limiter.Allow() {
            http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
            return
        }
        
        next.ServeHTTP(w, r)
    })
}
```

---

# 📋 Security Checklist

## Authentication
- [ ] Passwords hashed with bcrypt/argon2
- [ ] Secure session management
- [ ] Account lockout after failed attempts
- [ ] Password complexity requirements

## Authorization
- [ ] Principle of least privilege
- [ ] Authorization checked on every request
- [ ] Resource ownership verified

## Input Validation
- [ ] All inputs validated and sanitized
- [ ] Parameterized queries for SQL
- [ ] Output encoding for XSS prevention

## Data Protection
- [ ] Sensitive data encrypted at rest
- [ ] TLS for data in transit
- [ ] No secrets in code or logs

## Logging & Monitoring
- [ ] Security events logged
- [ ] No sensitive data in logs
- [ ] Alerting for suspicious activity

---

# 📚 References

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [OWASP Cheat Sheets](https://cheatsheetseries.owasp.org/)
- [trailofbits/static-analysis](https://github.com/trailofbits/skills/tree/main/plugins/static-analysis)
- [trailofbits/sharp-edges](https://github.com/trailofbits/skills/tree/main/plugins/sharp-edges)
- [CWE Top 25](https://cwe.mitre.org/top25/)
