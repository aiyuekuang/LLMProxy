---
name: database
description: Database design and optimization. Use for schema design, queries, migrations, indexing, and performance tuning.
---

# Database Skill

Database design, query optimization, and best practices for PostgreSQL/MySQL.

## When to Use This Skill

- Designing database schemas
- Writing efficient queries
- Creating migrations
- Performance optimization
- Index design

---

# 📐 Schema Design

## Table Naming

```sql
-- ✅ Good: snake_case, plural, descriptive
CREATE TABLE users (...);
CREATE TABLE api_keys (...);
CREATE TABLE model_providers (...);
CREATE TABLE request_logs (...);

-- ❌ Bad: CamelCase, singular, abbreviations
CREATE TABLE User (...);
CREATE TABLE apiKey (...);
CREATE TABLE mdl_prvdr (...);
```

## Column Naming

```sql
-- ✅ Good: snake_case, clear names
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    display_name VARCHAR(100),
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);
```

## Common Patterns

### Timestamps
```sql
-- Always include these columns
created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()

-- Auto-update updated_at with trigger
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();
```

### Soft Deletes
```sql
deleted_at TIMESTAMP WITH TIME ZONE,

-- Query active records
SELECT * FROM users WHERE deleted_at IS NULL;
```

### UUIDs vs Auto-increment
```sql
-- UUID (recommended for distributed systems)
id UUID PRIMARY KEY DEFAULT gen_random_uuid()

-- Auto-increment (simpler, better for single DB)
id SERIAL PRIMARY KEY
-- or for larger tables
id BIGSERIAL PRIMARY KEY
```

---

# 🔗 Relationships

## One-to-Many

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL
);

CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_hash VARCHAR(255) NOT NULL,
    name VARCHAR(100)
);

CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
```

## Many-to-Many

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY
);

CREATE TABLE roles (
    id UUID PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE
);

CREATE TABLE user_roles (
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID REFERENCES roles(id) ON DELETE CASCADE,
    granted_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (user_id, role_id)
);
```

---

# 🔍 Query Optimization

## Use EXPLAIN ANALYZE

```sql
EXPLAIN ANALYZE
SELECT u.*, COUNT(ak.id) as key_count
FROM users u
LEFT JOIN api_keys ak ON ak.user_id = u.id
WHERE u.is_active = true
GROUP BY u.id;
```

## Avoid SELECT *

```sql
-- ❌ Bad: Fetches all columns
SELECT * FROM users WHERE id = $1;

-- ✅ Good: Only needed columns
SELECT id, name, email FROM users WHERE id = $1;
```

## Use Proper JOINs

```sql
-- ❌ Bad: Implicit join
SELECT * FROM users, orders WHERE users.id = orders.user_id;

-- ✅ Good: Explicit JOIN
SELECT u.name, o.total
FROM users u
INNER JOIN orders o ON o.user_id = u.id;
```

## Batch Operations

```sql
-- ❌ Bad: Multiple INSERT statements
INSERT INTO logs (message) VALUES ('log1');
INSERT INTO logs (message) VALUES ('log2');
INSERT INTO logs (message) VALUES ('log3');

-- ✅ Good: Single batch INSERT
INSERT INTO logs (message) VALUES 
    ('log1'),
    ('log2'),
    ('log3');
```

## Use EXISTS instead of IN for large sets

```sql
-- ❌ Slower for large subqueries
SELECT * FROM users 
WHERE id IN (SELECT user_id FROM active_sessions);

-- ✅ Better performance
SELECT * FROM users u
WHERE EXISTS (
    SELECT 1 FROM active_sessions s WHERE s.user_id = u.id
);
```

---

# 📇 Indexing

## When to Create Indexes

- Columns in WHERE clauses
- Columns in JOIN conditions
- Columns in ORDER BY
- Columns with high selectivity (many unique values)

## Index Types

```sql
-- B-tree (default, most common)
CREATE INDEX idx_users_email ON users(email);

-- Composite index (order matters!)
CREATE INDEX idx_logs_user_date ON request_logs(user_id, created_at DESC);

-- Partial index (filtered)
CREATE INDEX idx_active_users ON users(email) WHERE is_active = true;

-- GIN index (for arrays, JSONB)
CREATE INDEX idx_users_tags ON users USING GIN(tags);

-- Unique index
CREATE UNIQUE INDEX idx_users_email_unique ON users(email);
```

## Index Guidelines

```sql
-- ✅ Good: Frequently queried columns
CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash);

-- ✅ Good: Foreign keys
CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);

-- ❌ Bad: Low cardinality columns
CREATE INDEX idx_users_is_active ON users(is_active);  -- Only true/false

-- ❌ Bad: Too many indexes (slow writes)
```

---

# 🔄 Migrations

## Migration Best Practices

```sql
-- migrations/001_create_users.sql

-- Up
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(100),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);

-- Down
DROP TABLE IF EXISTS users;
```

## Safe Schema Changes

```sql
-- ✅ Safe: Add column with default (PostgreSQL 11+)
ALTER TABLE users ADD COLUMN status VARCHAR(20) DEFAULT 'active';

-- ✅ Safe: Create index concurrently (no lock)
CREATE INDEX CONCURRENTLY idx_users_status ON users(status);

-- ⚠️ Careful: Add NOT NULL column
-- First add nullable, backfill, then add constraint
ALTER TABLE users ADD COLUMN role VARCHAR(50);
UPDATE users SET role = 'user' WHERE role IS NULL;
ALTER TABLE users ALTER COLUMN role SET NOT NULL;
```

---

# 🔒 Security

## Parameterized Queries

```go
// ❌ NEVER: String concatenation (SQL injection!)
query := "SELECT * FROM users WHERE email = '" + email + "'"

// ✅ ALWAYS: Parameterized query
query := "SELECT * FROM users WHERE email = $1"
row := db.QueryRow(query, email)
```

## Least Privilege

```sql
-- Create read-only user for reporting
CREATE USER report_user WITH PASSWORD 'secure_password';
GRANT SELECT ON ALL TABLES IN SCHEMA public TO report_user;

-- App user with limited permissions
CREATE USER app_user WITH PASSWORD 'secure_password';
GRANT SELECT, INSERT, UPDATE, DELETE ON users, api_keys TO app_user;
```

---

# 📊 JSONB Patterns (PostgreSQL)

```sql
-- Store flexible data
CREATE TABLE model_configs (
    id UUID PRIMARY KEY,
    model_name VARCHAR(100),
    parameters JSONB DEFAULT '{}'::jsonb
);

-- Query JSONB
SELECT * FROM model_configs 
WHERE parameters->>'temperature' = '0.7';

-- Index JSONB
CREATE INDEX idx_configs_params ON model_configs USING GIN(parameters);

-- Update JSONB
UPDATE model_configs 
SET parameters = parameters || '{"max_tokens": 1000}'::jsonb
WHERE id = $1;
```

---

# 📚 References

- [PostgreSQL Documentation](https://www.postgresql.org/docs/)
- [Use The Index, Luke](https://use-the-index-luke.com/)
- [sanjay3290/postgres](https://github.com/sanjay3290/ai-skills/tree/main/skills/postgres)
