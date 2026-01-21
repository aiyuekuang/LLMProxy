---
name: debugging
description: Systematic debugging methodology for finding and fixing bugs. Use when investigating issues, tracing errors, or fixing production problems.
---

# Debugging Skill

Methodical problem-solving approach for identifying and resolving bugs efficiently.

## When to Use This Skill

- Investigating bug reports
- Tracing error origins
- Fixing production issues
- Understanding unexpected behavior

---

# 🔍 Systematic Debugging Process

## 1. Reproduce the Issue

Before fixing, confirm you can reproduce:

```markdown
**Reproduction Steps:**
1. Navigate to /dashboard
2. Click "Create New" button
3. Fill form with empty values
4. Click submit
5. **Expected**: Validation error shown
6. **Actual**: Page crashes with error
```

## 2. Isolate the Problem

```
┌─────────────────────────────────────────┐
│  Is the bug in frontend or backend?    │
│           ↓                             │
│  Which component/function?              │
│           ↓                             │
│  What input triggers it?                │
│           ↓                             │
│  What's the expected vs actual?         │
└─────────────────────────────────────────┘
```

## 3. Form Hypothesis

Based on symptoms, hypothesize potential causes:
- Recent code changes?
- Data-related issue?
- Race condition?
- Configuration problem?
- External dependency failure?

## 4. Test Hypothesis

Add targeted logging or use debugger to verify:

```go
// Add strategic logging
log.Printf("[DEBUG] Handler input: %+v", input)
log.Printf("[DEBUG] Database result: %+v, err: %v", result, err)
log.Printf("[DEBUG] Response: %+v", response)
```

## 5. Fix and Verify

- Make minimal fix for root cause
- Add regression test
- Verify fix doesn't break other functionality

---

# 🐛 Common Bug Patterns

## Null/Nil Reference

```go
// ❌ Bug: Nil pointer dereference
func (h *Handler) GetName(user *User) string {
    return user.Name  // Crashes if user is nil
}

// ✅ Fix: Check for nil
func (h *Handler) GetName(user *User) string {
    if user == nil {
        return ""
    }
    return user.Name
}
```

## Off-by-One Errors

```go
// ❌ Bug: Index out of bounds
for i := 0; i <= len(items); i++ {
    process(items[i])
}

// ✅ Fix: Correct boundary
for i := 0; i < len(items); i++ {
    process(items[i])
}
```

## Race Conditions

```go
// ❌ Bug: Data race
var counter int
go func() { counter++ }()
go func() { counter++ }()

// ✅ Fix: Use sync primitives
var counter int64
go func() { atomic.AddInt64(&counter, 1) }()
go func() { atomic.AddInt64(&counter, 1) }()
```

## Memory Leaks

```typescript
// ❌ Bug: Event listener not cleaned up
useEffect(() => {
  window.addEventListener('resize', handleResize);
}, []);

// ✅ Fix: Cleanup on unmount
useEffect(() => {
  window.addEventListener('resize', handleResize);
  return () => window.removeEventListener('resize', handleResize);
}, []);
```

## Async/Await Issues

```typescript
// ❌ Bug: Missing await
async function fetchData() {
  const response = fetch('/api/data');  // Missing await
  return response.json();  // Error: response is a Promise
}

// ✅ Fix: Proper async handling
async function fetchData() {
  const response = await fetch('/api/data');
  return response.json();
}
```

---

# 🔧 Debugging Tools

## Go Debugging

### Delve Debugger
```bash
# Install
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug
dlv debug ./cmd/main.go

# Set breakpoint
(dlv) break main.go:42
(dlv) continue
(dlv) print variableName
(dlv) next
(dlv) step
```

### Printf Debugging
```go
import "github.com/davecgh/go-spew/spew"

// Pretty print complex structures
spew.Dump(complexObject)

// Or use JSON for readability
data, _ := json.MarshalIndent(obj, "", "  ")
log.Printf("Object: %s", data)
```

### Profiling
```go
import _ "net/http/pprof"

// Start pprof server
go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()

// Access: http://localhost:6060/debug/pprof/
```

## Frontend Debugging

### Browser DevTools
- **Console**: Error messages, console.log output
- **Network**: API requests/responses
- **Sources**: Breakpoints, step debugging
- **React DevTools**: Component state, props

### React Query DevTools
```tsx
import { ReactQueryDevtools } from '@tanstack/react-query-devtools';

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <MyApp />
      <ReactQueryDevtools initialIsOpen={false} />
    </QueryClientProvider>
  );
}
```

### Strategic Console Logging
```typescript
// Group related logs
console.group('Form Submission');
console.log('Form data:', formData);
console.log('Validation result:', validationResult);
console.groupEnd();

// Trace execution path
console.trace('How did we get here?');

// Measure performance
console.time('API Call');
await fetchData();
console.timeEnd('API Call');
```

---

# 🔬 Root Cause Analysis

## The 5 Whys Technique

```markdown
**Problem**: User cannot login

1. Why? → Login API returns 500 error
2. Why? → Database query fails
3. Why? → Connection pool exhausted
4. Why? → Connections not being released
5. Why? → Missing defer db.Close() in handler

**Root Cause**: Resource leak due to missing cleanup
**Fix**: Add proper connection release
```

## Bisect to Find Culprit

```bash
# Use git bisect to find the commit that introduced the bug
git bisect start
git bisect bad HEAD          # Current version is broken
git bisect good v1.0.0       # v1.0.0 was working

# Git will checkout middle commit, test it, then:
git bisect good  # or
git bisect bad

# Continue until culprit is found
git bisect reset  # When done
```

---

# 📊 Logging Best Practices

## Structured Logging

```go
import "go.uber.org/zap"

logger, _ := zap.NewProduction()
defer logger.Sync()

logger.Info("request processed",
    zap.String("method", r.Method),
    zap.String("path", r.URL.Path),
    zap.Int("status", status),
    zap.Duration("duration", time.Since(start)),
    zap.String("request_id", requestID),
)
```

## Log Levels

- **ERROR**: Failures requiring immediate attention
- **WARN**: Unexpected but recoverable situations
- **INFO**: Important business events
- **DEBUG**: Detailed diagnostic information

## What to Log

✅ **Do Log**:
- Request/response metadata (not sensitive data)
- Error details with context
- Performance metrics
- Security events (login attempts, permission denials)

❌ **Don't Log**:
- Passwords, API keys, tokens
- Personal identifiable information (PII)
- Credit card numbers
- Health information

---

# 📚 References

- [obra/systematic-debugging](https://github.com/obra/superpowers/blob/main/skills/systematic-debugging/SKILL.md)
- [obra/root-cause-tracing](https://github.com/obra/superpowers/blob/main/skills/root-cause-tracing/SKILL.md)
- [getsentry/find-bugs](https://github.com/getsentry/skills/tree/main/plugins/sentry-skills/skills/find-bugs)
