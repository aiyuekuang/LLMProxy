---
name: documentation
description: Technical documentation best practices. Use for writing READMEs, API docs, code comments, and project documentation.
---

# Documentation Skill

Write clear, maintainable technical documentation.

## When to Use This Skill

- Writing README files
- API documentation
- Code comments and JSDoc
- Architecture documentation
- User guides

---

# 📖 README Structure

```markdown
# Project Name

Brief description of what this project does.

## Features

- Feature 1
- Feature 2

## Quick Start

\`\`\`bash
# Install
npm install

# Run
npm start
\`\`\`

## Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `API_URL` | API endpoint | `http://localhost:8080` |

## API Reference

See [API Documentation](./docs/api.md)

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md)

## License

MIT
```

---

# 💬 Code Comments

## Good Comments

```go
// calculateRateLimit returns the rate limit for a user based on their tier.
// Premium users get 1000 req/min, free users get 100 req/min.
func calculateRateLimit(tier UserTier) int {
    // ...
}
```

```typescript
/**
 * Fetches user data from the API with automatic retry.
 * @param userId - The unique identifier of the user
 * @param options - Optional configuration
 * @returns The user object or null if not found
 * @throws {NetworkError} When the API is unreachable
 */
async function fetchUser(userId: string, options?: FetchOptions): Promise<User | null> {
    // ...
}
```

## Avoid These

```go
// ❌ Obvious comments
i++ // increment i

// ❌ Outdated comments
// Returns user by email  <- but function uses ID now
func GetUserByID(id string) *User
```

---

# 📚 References

- [Google Technical Writing](https://developers.google.com/tech-writing)
- [anthropics/doc-coauthoring](https://github.com/anthropics/skills/tree/main/skills/doc-coauthoring)
