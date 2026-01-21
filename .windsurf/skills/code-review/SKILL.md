---
name: code-review
description: Perform thorough code reviews with security, performance, and quality checks. Use when reviewing PRs, auditing code changes, or finding potential bugs.
---

# Code Review Skill

Systematic code review process following industry best practices from Sentry, Trail of Bits, and Google engineering teams.

## When to Use This Skill

- Reviewing pull requests
- Auditing code for security vulnerabilities
- Finding bugs and code smells
- Ensuring code quality standards

---

# 🔍 Review Process

## 1. Understand Context First

Before reviewing code:
- Read the PR description and linked issues
- Understand the business requirements
- Check if there are related tests or documentation

## 2. Review Checklist

### ✅ Correctness
- [ ] Does the code do what it's supposed to do?
- [ ] Are edge cases handled?
- [ ] Is error handling appropriate?
- [ ] Are there any obvious bugs?

### ✅ Security
- [ ] Input validation present?
- [ ] No hardcoded secrets/credentials?
- [ ] SQL injection prevention?
- [ ] XSS prevention?
- [ ] Authentication/authorization correct?
- [ ] Sensitive data properly handled?

### ✅ Performance
- [ ] No N+1 queries?
- [ ] Appropriate indexing?
- [ ] No memory leaks?
- [ ] Efficient algorithms used?
- [ ] Caching considered where appropriate?

### ✅ Code Quality
- [ ] Code is readable and self-documenting?
- [ ] No unnecessary complexity?
- [ ] DRY - no code duplication?
- [ ] Single responsibility principle followed?
- [ ] Consistent naming conventions?

### ✅ Testing
- [ ] Unit tests for new functionality?
- [ ] Edge cases tested?
- [ ] Error paths tested?
- [ ] Integration tests where needed?

### ✅ Documentation
- [ ] Public APIs documented?
- [ ] Complex logic explained?
- [ ] README updated if needed?

---

# 🐛 Common Issues to Look For

## Go Code

```go
// ❌ Bad: Ignoring errors
result, _ := doSomething()

// ✅ Good: Handle errors
result, err := doSomething()
if err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}
```

```go
// ❌ Bad: Defer in loop
for _, item := range items {
    f, _ := os.Open(item)
    defer f.Close()  // Won't close until function returns
}

// ✅ Good: Close immediately or use closure
for _, item := range items {
    func() {
        f, _ := os.Open(item)
        defer f.Close()
        // process f
    }()
}
```

## TypeScript/React Code

```tsx
// ❌ Bad: Missing dependency in useEffect
useEffect(() => {
  fetchData(userId);
}, []);  // userId missing

// ✅ Good: Include all dependencies
useEffect(() => {
  fetchData(userId);
}, [userId]);
```

```tsx
// ❌ Bad: Inline object causing re-renders
<Component style={{ margin: 10 }} />

// ✅ Good: Stable reference
const style = useMemo(() => ({ margin: 10 }), []);
<Component style={style} />
```

## SQL/Database

```sql
-- ❌ Bad: SQL injection risk
query := "SELECT * FROM users WHERE id = " + userInput

-- ✅ Good: Parameterized query
query := "SELECT * FROM users WHERE id = $1"
db.Query(query, userInput)
```

---

# 📝 Review Feedback Guidelines

## Be Constructive

```markdown
// ❌ Bad feedback
"This code is wrong"

// ✅ Good feedback
"This might cause a race condition when multiple goroutines access `sharedMap`. 
Consider using `sync.RWMutex` or `sync.Map` for thread-safe access."
```

## Categorize Comments

- **[Must Fix]**: Security issues, bugs, data loss risks
- **[Should Fix]**: Performance issues, code smells
- **[Suggestion]**: Style improvements, optional enhancements
- **[Question]**: Seeking clarification

## Provide Examples

```markdown
// Instead of just pointing out the issue, show the fix:

**Issue**: Potential nil pointer dereference

**Current code**:
```go
return user.Name
```

**Suggested fix**:
```go
if user == nil {
    return ""
}
return user.Name
```
```

---

# 🔐 Security Review Focus

## OWASP Top 10 Checklist

1. **Injection** - SQL, NoSQL, OS command injection
2. **Broken Authentication** - Session management, credential storage
3. **Sensitive Data Exposure** - Encryption, data masking
4. **XML External Entities** - XXE attacks
5. **Broken Access Control** - Authorization checks
6. **Security Misconfiguration** - Default credentials, error messages
7. **Cross-Site Scripting (XSS)** - Input/output encoding
8. **Insecure Deserialization** - Object validation
9. **Known Vulnerabilities** - Dependency versions
10. **Insufficient Logging** - Audit trails

---

# 📚 References

- [Google Code Review Guidelines](https://google.github.io/eng-practices/review/)
- [getsentry/code-review](https://github.com/getsentry/skills/tree/main/plugins/sentry-skills/skills/code-review)
- [trailofbits/differential-review](https://github.com/trailofbits/skills/tree/main/plugins/differential-review)
- [obra/requesting-code-review](https://github.com/obra/superpowers/blob/main/skills/requesting-code-review/SKILL.md)
