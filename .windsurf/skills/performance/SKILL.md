---
name: performance
description: Performance optimization for web applications. Use when optimizing frontend rendering, backend response times, database queries, or system resources.
---

# Performance Skill

Performance optimization techniques for frontend, backend, and database layers.

## When to Use This Skill

- Optimizing page load times
- Reducing API response latency
- Improving database query performance
- Memory and CPU optimization
- Caching strategies

---

# ⚡ Frontend Performance

## Core Web Vitals

| Metric | Target | Description |
|--------|--------|-------------|
| LCP | < 2.5s | Largest Contentful Paint |
| FID | < 100ms | First Input Delay |
| CLS | < 0.1 | Cumulative Layout Shift |

## Bundle Optimization

```typescript
// Dynamic imports for code splitting
const Dashboard = lazy(() => import('./pages/Dashboard'));

// Route-based splitting
const routes = [
  {
    path: '/dashboard',
    component: lazy(() => import('./pages/Dashboard')),
  },
];
```

## Image Optimization

```tsx
// Next.js Image component
import Image from 'next/image';

<Image
  src="/hero.jpg"
  alt="Hero"
  width={1200}
  height={600}
  priority  // Above the fold
  placeholder="blur"
/>

// Lazy load below-fold images
<Image
  src="/feature.jpg"
  loading="lazy"
/>
```

## React Optimization

```tsx
// Memoize expensive calculations
const sortedData = useMemo(() => {
  return data.sort((a, b) => a.name.localeCompare(b.name));
}, [data]);

// Memoize callbacks
const handleClick = useCallback((id: string) => {
  setSelected(id);
}, []);

// Memoize components
const ExpensiveList = memo(({ items }) => (
  <ul>
    {items.map(item => <li key={item.id}>{item.name}</li>)}
  </ul>
));

// Virtualize long lists
import { FixedSizeList } from 'react-window';

<FixedSizeList
  height={400}
  itemCount={10000}
  itemSize={35}
>
  {({ index, style }) => (
    <div style={style}>{items[index].name}</div>
  )}
</FixedSizeList>
```

## CSS Performance

```css
/* Avoid expensive selectors */
/* ❌ Bad */
div.container ul li a span { }

/* ✅ Good */
.nav-link-text { }

/* Use will-change for animations */
.animated-element {
  will-change: transform;
}

/* Contain paint for isolated elements */
.card {
  contain: layout paint;
}
```

---

# 🚀 Backend Performance

## Go Optimization

### Reduce Allocations

```go
// ❌ Bad: Allocates on each iteration
for i := 0; i < 1000; i++ {
    result = append(result, processItem(i))
}

// ✅ Good: Pre-allocate slice
result := make([]Item, 0, 1000)
for i := 0; i < 1000; i++ {
    result = append(result, processItem(i))
}
```

### Use sync.Pool

```go
var bufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

func processRequest(data []byte) {
    buf := bufferPool.Get().(*bytes.Buffer)
    defer func() {
        buf.Reset()
        bufferPool.Put(buf)
    }()
    
    buf.Write(data)
    // Process...
}
```

### Efficient JSON Handling

```go
// ❌ Bad: Creates new encoder each time
json.NewEncoder(w).Encode(data)

// ✅ Good: Reuse encoder or use sonic for speed
import "github.com/bytedance/sonic"

sonic.Marshal(data)
```

### Goroutine Patterns

```go
// Worker pool pattern
func processItems(items []Item, workers int) []Result {
    jobs := make(chan Item, len(items))
    results := make(chan Result, len(items))

    // Start workers
    for w := 0; w < workers; w++ {
        go func() {
            for item := range jobs {
                results <- process(item)
            }
        }()
    }

    // Send jobs
    for _, item := range items {
        jobs <- item
    }
    close(jobs)

    // Collect results
    output := make([]Result, len(items))
    for i := range output {
        output[i] = <-results
    }
    return output
}
```

---

# 💾 Caching Strategies

## Cache Patterns

```
┌─────────────┐    miss    ┌─────────────┐
│   Client    │ ────────── │   Database  │
└─────────────┘            └─────────────┘
       │                          │
       │ hit                      │
       ▼                          │
┌─────────────┐                   │
│    Cache    │ ◄─────────────────┘
└─────────────┘     populate
```

## Redis Caching

```go
import "github.com/redis/go-redis/v9"

type CacheService struct {
    client *redis.Client
    ttl    time.Duration
}

func (c *CacheService) Get(ctx context.Context, key string, dest interface{}) error {
    val, err := c.client.Get(ctx, key).Result()
    if err == redis.Nil {
        return ErrCacheMiss
    }
    if err != nil {
        return err
    }
    return json.Unmarshal([]byte(val), dest)
}

func (c *CacheService) Set(ctx context.Context, key string, value interface{}) error {
    data, err := json.Marshal(value)
    if err != nil {
        return err
    }
    return c.client.Set(ctx, key, data, c.ttl).Err()
}

// Cache-aside pattern
func (s *Service) GetUser(ctx context.Context, id string) (*User, error) {
    cacheKey := "user:" + id
    
    // Try cache first
    var user User
    if err := s.cache.Get(ctx, cacheKey, &user); err == nil {
        return &user, nil
    }
    
    // Cache miss - fetch from DB
    user, err := s.db.GetUser(ctx, id)
    if err != nil {
        return nil, err
    }
    
    // Populate cache
    s.cache.Set(ctx, cacheKey, user)
    
    return user, nil
}
```

## Cache Invalidation

```go
// Invalidate on update
func (s *Service) UpdateUser(ctx context.Context, user *User) error {
    if err := s.db.UpdateUser(ctx, user); err != nil {
        return err
    }
    
    // Invalidate cache
    cacheKey := "user:" + user.ID
    s.cache.Delete(ctx, cacheKey)
    
    return nil
}

// Use cache tags for bulk invalidation
func (s *Service) InvalidateUserCaches(ctx context.Context, userID string) {
    keys := []string{
        "user:" + userID,
        "user:profile:" + userID,
        "user:settings:" + userID,
    }
    s.cache.Delete(ctx, keys...)
}
```

---

# 🗃️ Database Performance

## Query Optimization

```sql
-- Use EXPLAIN ANALYZE
EXPLAIN ANALYZE
SELECT u.*, COUNT(o.id) as order_count
FROM users u
LEFT JOIN orders o ON o.user_id = u.id
WHERE u.created_at > '2024-01-01'
GROUP BY u.id;

-- Add appropriate indexes
CREATE INDEX idx_users_created_at ON users(created_at);
CREATE INDEX idx_orders_user_id ON orders(user_id);
```

## Connection Pooling

```go
db, err := sql.Open("postgres", connString)
if err != nil {
    return err
}

// Configure pool
db.SetMaxOpenConns(25)           // Max connections
db.SetMaxIdleConns(5)            // Keep 5 idle
db.SetConnMaxLifetime(5 * time.Minute)
db.SetConnMaxIdleTime(1 * time.Minute)
```

## N+1 Query Prevention

```go
// ❌ Bad: N+1 queries
users := db.GetAllUsers()
for _, user := range users {
    orders := db.GetOrdersByUser(user.ID)  // N queries!
}

// ✅ Good: Single query with JOIN or preload
users := db.Preload("Orders").Find(&users)

// Or batch fetch
userIDs := extractIDs(users)
ordersByUser := db.GetOrdersForUsers(userIDs)
```

---

# 📊 Monitoring & Profiling

## Go Profiling

```go
import _ "net/http/pprof"

// Enable pprof server
go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()

// Access profiles:
// http://localhost:6060/debug/pprof/
// http://localhost:6060/debug/pprof/heap
// http://localhost:6060/debug/pprof/goroutine
```

```bash
# CPU profile
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# Memory profile
go tool pprof http://localhost:6060/debug/pprof/heap

# Visualize
go tool pprof -http=:8080 cpu.prof
```

## Request Tracing

```go
func TimingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        wrapped := &responseWriter{ResponseWriter: w}
        next.ServeHTTP(wrapped, r)
        
        duration := time.Since(start)
        
        logger.Info("request",
            zap.String("method", r.Method),
            zap.String("path", r.URL.Path),
            zap.Int("status", wrapped.status),
            zap.Duration("duration", duration),
        )
        
        // Alert slow requests
        if duration > 2*time.Second {
            logger.Warn("slow request", zap.Duration("duration", duration))
        }
    })
}
```

---

# 📚 References

- [web.dev Performance](https://web.dev/performance/)
- [Go Performance Tips](https://github.com/dgryski/go-perfbook)
- [React Performance](https://react.dev/learn/render-and-commit)
