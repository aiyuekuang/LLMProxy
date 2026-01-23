# LLMProxy Configuration Reference

This document contains the complete reference for all LLMProxy configuration options.

## Table of Contents

- [Overview](#overview)
- [Server Configuration (server)](#server-configuration-server)
- [System Logging (log)](#system-logging-log)
- [Storage Configuration (storage)](#storage-configuration-storage)
- [Backend Services (backends)](#backend-services-backends)
- [Service Discovery (discovery)](#service-discovery-discovery)
- [Admin API (admin)](#admin-api-admin)
- [Authentication (auth)](#authentication-auth)
- [Request/Access Logging (logging)](#requestaccess-logging-logging)
- [Rate Limiting (rate_limit)](#rate-limiting-rate_limit)
- [Routing Configuration (routing)](#routing-configuration-routing)
- [Health Check (health_check)](#health-check-health_check)
- [Metrics (metrics)](#metrics-metrics)
- [Usage Reporting (usage)](#usage-reporting-usage)
- [Lifecycle Hooks (hooks)](#lifecycle-hooks-hooks)
- [Deprecated Fields](#deprecated-fields)

---

## Overview

LLMProxy uses YAML configuration files with a modular design.

### Naming Conventions

| Field | Meaning | Example |
|-------|---------|---------|
| `enabled` | Module/feature toggle | `enabled: true` |
| `driver` | Driver type | `driver: "mysql"` |
| `storage` | Reference to top-level storage | `storage: "primary"` |
| `interval` | Time interval | `interval: 30s` |
| `timeout` | Timeout duration | `timeout: 5s` |
| `path` | Path (file or URL path) | `path: "/health"` |
| `url` | Complete URL | `url: "http://..."` |
| `addr` | Address (host:port) | `addr: "localhost:6379"` |

### Duration Format

Supports Go standard duration format: `1s`, `30s`, `1m`, `5m`, `1h`

### Configuration File Structure

```
config.yaml
├── server              # Server configuration
├── log                 # System logging
├── storage             # Storage connections
│   ├── databases       # Database connection pool
│   └── caches          # Cache connection pool
├── backends            # Static backend list
├── discovery           # Service discovery
├── admin               # Admin API
├── auth                # Authentication
├── logging             # Request/access logging
├── rate_limit          # Rate limiting
├── routing             # Routing configuration
├── health_check        # Health check
├── metrics             # Metrics
├── usage               # Usage reporting
└── hooks               # Lifecycle hooks
```

---

## Server Configuration (server)

HTTP server related configuration.

```yaml
server:
  listen: ":8000"                  # Listen address, format: ":port" or "IP:port"
  read_timeout: 30s                # Read timeout
  write_timeout: 60s               # Write timeout (set to 0 for streaming)
  idle_timeout: 120s               # Idle connection timeout
  max_header_bytes: 1048576        # Max header size (default 1MB)
  max_body_size: 10485760          # Max body size (default 10MB)
  
  # CORS configuration
  cors:
    enabled: false                 # Enable CORS
    allowed_origins:               # Allowed origins
      - "*"
    allowed_methods:               # Allowed methods
      - "GET"
      - "POST"
      - "OPTIONS"
    allowed_headers:               # Allowed headers
      - "Authorization"
      - "Content-Type"
      - "X-API-Key"
    expose_headers: []             # Exposed headers
    allow_credentials: false       # Allow credentials
    max_age: 86400                 # Preflight cache time (seconds)
  
  # TLS/HTTPS configuration
  tls:
    enabled: false                 # Enable HTTPS
    cert_file: "./certs/server.crt"
    key_file: "./certs/server.key"
    client_ca_file: ""             # Client CA certificate (for mutual TLS)
    client_auth: "none"            # Client auth: none / request / require
```

### Field Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `listen` | string | `:8000` | Listen address |
| `read_timeout` | duration | `30s` | Request read timeout |
| `write_timeout` | duration | `60s` | Response write timeout |
| `idle_timeout` | duration | `120s` | Idle connection timeout |
| `max_header_bytes` | int | `1048576` | Max header size in bytes |
| `max_body_size` | int64 | `10485760` | Max body size in bytes |

> **Note**: For streaming responses, `write_timeout` is set to 0 to avoid interrupting long-running streams.

---

## System Logging (log)

LLMProxy runtime logging configuration.

> **Note**: `log` is for system runtime logs, `logging` is for request/access logs. They serve different purposes.

```yaml
log:
  level: "info"                    # Log level: debug / info / warn / error
  format: "json"                   # Format: json / text
  output: "stdout"                 # Output: stdout / stderr / file
  
  # File configuration (when output=file)
  file:
    path: "./logs/llmproxy.log"
    rotate: "daily"                # Rotation: daily / hourly / size
    max_size: 100                  # MB (when rotate=size)
    max_age: 7                     # Retention days
    compress: true                 # Compress old logs
```

### Field Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `level` | string | `info` | Log level |
| `format` | string | `json` | Output format |
| `output` | string | `stdout` | Output target |

---

## Storage Configuration (storage)

Define database and cache connection pools for use by other modules.

### Database Connection Pool (storage.databases)

Supports MySQL, PostgreSQL, SQLite.

```yaml
storage:
  databases:
    - name: "primary"              # Connection name for reference
      enabled: true                # Enable this connection
      driver: "mysql"              # Driver: mysql / postgres / sqlite
      host: "localhost"            # Host address
      port: 3306                   # Port
      user: "root"                 # Username
      password: "password"         # Password
      database: "llmproxy"         # Database name
      # dsn: ""                    # Or specify DSN directly (higher priority)
      max_open_conns: 100          # Max open connections
      max_idle_conns: 10           # Max idle connections
      conn_max_lifetime: 1h        # Connection max lifetime
      conn_max_idle_time: 10m      # Idle connection max time
    
    - name: "local"                # SQLite example
      enabled: true
      driver: "sqlite"
      path: "./data/local.db"      # SQLite file path
```

### Database Field Reference

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Connection name (required), used for reference |
| `enabled` | bool | Enable this connection |
| `driver` | string | Driver type: `mysql` / `postgres` / `sqlite` |
| `dsn` | string | DSN connection string (higher priority) |
| `host` | string | Database host address |
| `port` | int | Port (MySQL default 3306, PostgreSQL default 5432) |
| `user` | string | Username |
| `password` | string | Password |
| `database` | string | Database name |
| `path` | string | SQLite database file path |
| `max_open_conns` | int | Max open connections |
| `max_idle_conns` | int | Max idle connections |
| `conn_max_lifetime` | duration | Connection max lifetime |
| `conn_max_idle_time` | duration | Idle connection max time |

### Cache Connection Pool (storage.caches)

Supports Redis, Memory.

```yaml
storage:
  caches:
    - name: "primary"              # Connection name
      enabled: true                # Enable
      driver: "redis"              # Driver: redis / memory
      addr: "localhost:6379"       # Redis address
      password: ""                 # Redis password
      db: 0                        # Redis database number
      pool_size: 100               # Pool size
      min_idle_conns: 10           # Min idle connections
      dial_timeout: 5s             # Dial timeout
      read_timeout: 3s             # Read timeout
      write_timeout: 3s            # Write timeout
    
    - name: "local"                # Local memory cache
      enabled: true
      driver: "memory"
      max_size: 10000              # Max entries
      ttl: 5m                      # Default TTL
```

### Cache Field Reference

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Connection name (required) |
| `enabled` | bool | Enable |
| `driver` | string | Driver: `redis` / `memory` |
| `addr` | string | Redis address (host:port) |
| `password` | string | Redis password |
| `db` | int | Redis database number |
| `pool_size` | int | Connection pool size |
| `min_idle_conns` | int | Min idle connections |
| `dial_timeout` | duration | Dial timeout |
| `read_timeout` | duration | Read timeout |
| `write_timeout` | duration | Write timeout |
| `max_size` | int | Memory cache max entries |
| `ttl` | duration | Memory cache default TTL |

### Reference Method

Other modules reference by `storage: "<name>"`:

```yaml
auth:
  pipeline:
    - redis:
        storage: "primary"         # Reference caches[name=primary]

logging:
  request:
    storage: "logs"                # Reference databases[name=logs]

rate_limit:
  redis: "primary"                 # Reference caches[name=primary]
```

---

## Backend Services (backends)

Statically configured backend service list.

```yaml
backends:
  - name: "vllm-1"                 # Backend name (for logs and monitoring)
    url: "http://localhost:8000"   # Backend service URL
    weight: 5                      # Load balancing weight
    timeout: 60s                   # Request timeout
    connect_timeout: 5s            # Connection timeout
    max_idle_conns: 100            # Max idle connections
    headers:                       # Custom headers (optional)
      X-Backend-ID: "backend-1"
  
  - name: "vllm-2"
    url: "http://localhost:8001"
    weight: 3
```

### Field Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Backend name |
| `url` | string | - | Backend service URL (required) |
| `weight` | int | `1` | Load balancing weight |
| `timeout` | duration | `60s` | Request timeout |
| `connect_timeout` | duration | `5s` | Connection timeout |
| `max_idle_conns` | int | `100` | Max idle connections |
| `headers` | map | - | Custom request headers |

---

## Service Discovery (discovery)

Dynamically load backend service configurations from various data sources.

```yaml
discovery:
  enabled: true                    # Enable
  mode: "merge"                    # Mode: merge (all sources) / first (first valid)
  interval: 30s                    # Global sync interval
  
  sources:                         # Discovery source list
    # Database discovery
    - name: "db_discovery"
      type: "database"
      enabled: true
      database:
        storage: "primary"         # Reference storage.databases[name]
        table: "services"          # Service table name
        fields:                    # Field mapping
          name: "name"
          url: "endpoint"
          weight: "weight"
          status: "status"
      script:                      # Lua post-processing script (optional)
        enabled: false
        path: "./scripts/discovery_filter.lua"
    
    # Static configuration
    - name: "static_discovery"
      type: "static"
      enabled: false
      static:
        backends:
          - name: "static-1"
            url: "http://localhost:8000"
            weight: 5
    
    # Consul service discovery
    - name: "consul_discovery"
      type: "consul"
      enabled: false
      consul:
        addr: "http://consul:8500"
        service: "llm-backend"
        tag: "production"
        interval: 10s
    
    # Kubernetes service discovery
    - name: "k8s_discovery"
      type: "kubernetes"
      enabled: false
      kubernetes:
        namespace: "llm"
        service: "vllm"
        port: 8000
        label_selector: "app=vllm"
    
    # Etcd service discovery
    - name: "etcd_discovery"
      type: "etcd"
      enabled: false
      etcd:
        endpoints:
          - "http://etcd:2379"
        prefix: "/services/llm"
        username: ""
        password: ""
    
    # HTTP service discovery
    - name: "http_discovery"
      type: "http"
      enabled: false
      http:
        url: "http://registry/api/services"
        method: "GET"
        interval: 30s
        timeout: 5s
        headers:
          Authorization: "Bearer xxx"
```

### Discovery Source Types

| type | Description | Use Case |
|------|-------------|----------|
| `database` | Read from database | Admin management |
| `static` | Static configuration | Simple deployment |
| `consul` | Consul service discovery | Microservices |
| `kubernetes` | K8s Service/Endpoints | Cloud native |
| `etcd` | Etcd KV store | Distributed systems |
| `http` | HTTP API | Custom registry |

### Mode Description

| mode | Description |
|------|-------------|
| `merge` | Merge service lists from all sources |
| `first` | Use first available source |

---

## Admin API (admin)

Built-in management API for API Key CRUD operations and usage queries.

```yaml
admin:
  enabled: true                    # Enable
  token: "your-secure-admin-token" # Access token (required)
  listen: ""                       # Listen address (empty = mount on main server)
  db_path: "./data/keys.db"        # SQLite database path
```

### Field Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable Admin API |
| `token` | string | - | Access token (required), passed via `X-Admin-Token` header |
| `listen` | string | `""` | Standalone listen address, empty = share with main server |
| `db_path` | string | `./data/keys.db` | SQLite database path |

### Admin API Endpoints

| Endpoint | Description |
|----------|-------------|
| `POST /admin/keys/create` | Create API Key |
| `POST /admin/keys/update` | Update API Key |
| `POST /admin/keys/delete` | Delete API Key |
| `POST /admin/keys/get` | Get API Key |
| `POST /admin/keys/list` | List API Keys |
| `POST /admin/keys/sync` | Batch sync API Keys |

> **Note**: Both `builtin` type in `auth.pipeline` and `builtin` type in `usage.reporters` depend on this module.

---

## Authentication (auth)

API Key verification configuration with pipeline mode supporting multiple verification methods.

```yaml
auth:
  enabled: true                    # Enable
  mode: "first_match"              # Mode: first_match / all
  
  skip_paths:                      # Paths to skip authentication
    - "/health"
    - "/ready"
    - "/metrics"
  
  header_names:                    # Authentication header names
    - "Authorization"
    - "X-API-Key"
  
  # Status code configuration (optional)
  status_codes:
    disabled:
      http_code: 403
      message: "API Key is disabled"
    expired:
      http_code: 403
      message: "API Key has expired"
    quota_exceeded:
      http_code: 429
      message: "Quota exceeded"
    not_found:
      http_code: 401
      message: "Invalid API Key"
  
  pipeline:                        # Auth pipeline (executed in order)
    # ... see detailed provider configurations below
```

### Authentication Modes

| Mode | Description |
|------|-------------|
| `first_match` | First provider to pass is sufficient |
| `all` | All enabled providers must pass |

### Provider Types

#### Builtin (Built-in SQLite Storage)

Uses Admin module's SQLite database, requires `admin.enabled: true`.

```yaml
- name: "builtin_auth"
  type: "builtin"
  enabled: true
```

#### Redis

```yaml
- name: "redis_auth"
  type: "redis"
  enabled: true
  redis:
    storage: "primary"             # Reference storage.caches[name]
    key_pattern: "llmproxy:key:{api_key}"
  script:                          # Lua post-processing script (optional)
    enabled: false
    path: "./scripts/auth_redis.lua"
    timeout: 1s
    max_memory: 10
```

#### Database

```yaml
- name: "db_auth"
  type: "database"
  enabled: true
  database:
    storage: "primary"             # Reference storage.databases[name]
    table: "api_keys"              # Table name
    key_column: "key"              # API Key column name
    fields:                        # Fields to query
      - "user_id"
      - "quota"
      - "status"
  script:
    enabled: false
    path: "./scripts/auth_db.lua"
```

#### Webhook

```yaml
- name: "webhook_auth"
  type: "webhook"
  enabled: true
  webhook:
    url: "https://auth.example.com/verify"
    method: "POST"
    timeout: 5s
    headers:
      X-Service: "llmproxy"
  script:
    enabled: false
    path: "./scripts/auth_webhook.lua"
```

#### Lua

```yaml
- name: "lua_auth"
  type: "lua"
  enabled: true
  lua:
    path: "./scripts/auth.lua"     # Script file path
    # script: |                    # Or inline script
    #   return true
    timeout: 1s
    max_memory: 10                 # MB
```

#### Static

```yaml
- name: "static_auth"
  type: "static"
  enabled: true
  static:
    keys:
      - key: "sk-test-key-1"
        name: "Test Key 1"
        user_id: "user_001"
        status: "enabled"          # enabled / disabled
        total_quota: 1000000       # Total quota (tokens)
        used_quota: 0
        quota_reset_period: "monthly"  # daily / weekly / monthly / never
        allowed_ips: []            # IP whitelist
        denied_ips: []             # IP blacklist
        expires_at: null           # Expiration time
```

### API Key Field Reference

| Field | Type | Description |
|-------|------|-------------|
| `key` | string | API Key value |
| `name` | string | Key name |
| `user_id` | string | User ID |
| `status` | string | Status: `enabled` / `disabled` |
| `total_quota` | int64 | Total quota (tokens) |
| `used_quota` | int64 | Used quota |
| `quota_reset_period` | string | Reset period: `daily` / `weekly` / `monthly` / `never` |
| `allowed_ips` | []string | IP whitelist |
| `denied_ips` | []string | IP blacklist |
| `expires_at` | time | Expiration time |

---

## Request/Access Logging (logging)

Request logging and access logging configuration.

```yaml
logging:
  enabled: true
  
  # Request logging (detailed API request records)
  request:
    enabled: true
    storage: "primary"             # Reference storage.databases[name]
    table: "request_logs"          # Table name
    include_body: false            # Include request/response body
    script:
      enabled: false
      path: "./scripts/log_filter.lua"
    # File storage configuration (optional)
    file:
      path: "./logs/requests.log"
      max_size_mb: 100
      max_backups: 7
  
  # Access logging (similar to Nginx access log)
  access:
    enabled: false
    format: "combined"             # combined / json
    output: "file"                 # file / stdout
    script:
      enabled: false
      path: "./scripts/access_filter.lua"
    file:
      path: "./logs/access.log"
      max_size_mb: 100
      max_backups: 7
```

### Field Reference

| Field | Type | Description |
|-------|------|-------------|
| `storage` | string | Database storage reference |
| `table` | string | Table name |
| `include_body` | bool | Include request/response body |
| `format` | string | Access log format: `combined` / `json` |
| `output` | string | Output target: `file` / `stdout` |

---

## Rate Limiting (rate_limit)

Request rate limiting configuration.

```yaml
rate_limit:
  enabled: true
  storage: "memory"                # Storage: memory / redis
  redis: "primary"                 # When storage=redis, reference storage.caches[name]
  
  script:                          # Lua custom rate limiting script
    enabled: false
    path: "./scripts/ratelimit.lua"
    timeout: 1s
    max_memory: 10
  
  # Global rate limiting
  global:
    enabled: true
    requests_per_second: 100       # Requests per second
    requests_per_minute: 1000      # Requests per minute
    burst_size: 200                # Burst capacity
  
  # Per-key rate limiting
  per_key:
    enabled: true
    requests_per_second: 10        # Requests per second
    requests_per_minute: 60        # Requests per minute
    tokens_per_minute: 100000      # Tokens per minute
    max_concurrent: 10             # Max concurrent requests
    burst_size: 20                 # Burst capacity
```

### Field Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `storage` | string | `memory` | Storage type |
| `redis` | string | - | Redis cache reference |
| `requests_per_second` | int | - | Requests per second limit |
| `requests_per_minute` | int | - | Requests per minute limit |
| `tokens_per_minute` | int64 | - | Tokens per minute limit |
| `max_concurrent` | int | - | Max concurrent requests |
| `burst_size` | int | - | Token bucket burst capacity |

---

## Routing Configuration (routing)

Load balancing, retry, and failover configuration.

```yaml
routing:
  enabled: true
  load_balance: "round_robin"      # Strategy: round_robin / least_connections / latency_based
  
  timeout: 60s                     # Total request timeout
  connect_timeout: 5s              # Connection timeout
  
  script:                          # Lua custom routing script
    enabled: false
    path: "./scripts/routing.lua"
    timeout: 1s
    max_memory: 10
  
  # Retry configuration
  retry:
    enabled: true
    max_retries: 3                 # Max retry attempts
    initial_wait: 1s               # Initial wait time
    max_wait: 10s                  # Max wait time
    multiplier: 2.0                # Backoff multiplier
    retry_on:                      # Retry conditions
      - "5xx"                      # 5xx server errors
      - "connect_failure"          # Connection failure
      - "timeout"                  # Timeout
  
  # Fallback configuration
  fallback:
    - models: []                   # Applicable models (empty = all)
      primary: "http://localhost:8000"
      fallback:
        - "http://localhost:8001"
        - "http://localhost:8002"
```

### Load Balancing Strategies

| Strategy | Description |
|----------|-------------|
| `round_robin` | Round robin |
| `least_connections` | Least connections |
| `latency_based` | Latency based |

### Retry Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `max_retries` | int | `3` | Max retry attempts |
| `initial_wait` | duration | `1s` | Initial wait time |
| `max_wait` | duration | `10s` | Max wait time |
| `multiplier` | float64 | `2.0` | Exponential backoff multiplier |
| `retry_on` | []string | - | Retry conditions list |

---

## Health Check (health_check)

Backend service health check configuration.

```yaml
health_check:
  enabled: true
  interval: 30s                    # Check interval
  timeout: 5s                      # Timeout
  method: "GET"                    # HTTP method
  path: "/health"                  # Health check path
  expected_status: 200             # Expected status code
  unhealthy_threshold: 3           # Consecutive failures for unhealthy
  healthy_threshold: 2             # Consecutive successes for healthy
  
  script:                          # Lua custom health check script
    enabled: false
    path: "./scripts/health_check.lua"
    timeout: 1s
    max_memory: 10
```

### Field Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `interval` | duration | `30s` | Check interval |
| `timeout` | duration | `5s` | Timeout |
| `method` | string | `GET` | HTTP method |
| `path` | string | `/health` | Health check path |
| `expected_status` | int | `200` | Expected status code |
| `unhealthy_threshold` | int | `3` | Unhealthy threshold |
| `healthy_threshold` | int | `2` | Healthy threshold |

---

## Metrics (metrics)

Prometheus metrics exposure configuration.

```yaml
metrics:
  enabled: true
  path: "/metrics"                 # Metrics endpoint path
  
  custom_labels:                   # Custom labels
    - "user_id"
    - "api_key"
  
  latency_buckets:                 # Latency histogram buckets (seconds)
    - 0.01
    - 0.05
    - 0.1
    - 0.5
    - 1.0
    - 5.0
    - 10.0
```

### Field Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `path` | string | `/metrics` | Metrics endpoint path |
| `custom_labels` | []string | - | Custom label list |
| `latency_buckets` | []float64 | - | Latency histogram bucket configuration |

---

## Usage Reporting (usage)

Token usage statistics reporting configuration.

```yaml
usage:
  enabled: true
  
  reporters:                       # Reporter list (multiple allowed)
    # Built-in SQLite storage
    - name: "local"
      type: "builtin"
      enabled: true
      builtin:
        retention_days: 30         # Data retention days, 0=forever
    
    # Webhook reporting
    - name: "billing"
      type: "webhook"
      enabled: true
      webhook:
        url: "https://billing.example.com/usage"
        method: "POST"
        timeout: 5s
        retry: 3                   # Retry count
        headers:
          Authorization: "Bearer xxx"
      script:
        enabled: false
        path: "./scripts/usage_filter.lua"
    
    # Database reporting
    - name: "db_usage"
      type: "database"
      enabled: false
      database:
        storage: "primary"         # Reference storage.databases[name]
        table: "usage_records"     # Table name
      script:
        enabled: false
        path: "./scripts/usage_db.lua"
```

### Reporter Types

| Type | Description | Dependency |
|------|-------------|------------|
| `builtin` | Built-in SQLite storage | Requires `admin` enabled |
| `webhook` | HTTP Webhook reporting | - |
| `database` | External database storage | Requires `storage.databases` |

### Builtin Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `retention_days` | int | `0` | Data retention days, 0 = forever |

### Webhook Configuration

| Field | Type | Description |
|-------|------|-------------|
| `url` | string | Webhook URL |
| `method` | string | HTTP method |
| `timeout` | duration | Timeout |
| `retry` | int | Retry count |
| `headers` | map | Custom request headers |

---

## Lifecycle Hooks (hooks)

Global Lua hooks for request processing.

### Execution Order

```
on_request → on_auth → on_route → [Backend Processing] → on_response → on_complete
                                        ↓
                                   on_error (if error occurs)
```

### Configuration Example

```yaml
hooks:
  enabled: true
  
  # Request entry
  on_request:
    enabled: true
    path: "./scripts/on_request.lua"
    # script: |                    # Or inline script
    #   -- Available: request
    #   return { continue = true }
    timeout: 100ms
    max_memory: 10
  
  # After authentication
  on_auth:
    enabled: false
    path: "./scripts/on_auth.lua"
    timeout: 100ms
    max_memory: 10
  
  # Route selection
  on_route:
    enabled: false
    path: "./scripts/on_route.lua"
    timeout: 100ms
    max_memory: 10
  
  # Before response
  on_response:
    enabled: false
    path: "./scripts/on_response.lua"
    timeout: 100ms
    max_memory: 10
  
  # On error
  on_error:
    enabled: false
    path: "./scripts/on_error.lua"
    timeout: 100ms
    max_memory: 10
  
  # Request complete (async)
  on_complete:
    enabled: false
    path: "./scripts/on_complete.lua"
    timeout: 100ms
    max_memory: 10
```

### Hook Reference

| Hook | Trigger | Available Variables | Purpose |
|------|---------|---------------------|---------|
| `on_request` | Request entry | `request` | Add trace ID, modify headers, intercept requests |
| `on_auth` | After auth | `request`, `auth_result` | Get user info, permission checks |
| `on_route` | Route selection | `request`, `backends` | Custom routing logic |
| `on_response` | Before response | `request`, `response` | Modify response content |
| `on_error` | On error | `request`, `error_message` | Custom error response |
| `on_complete` | Request complete | `request`, `response` | Cleanup, statistics |

### Lua Script Examples

#### on_request Example

```lua
-- Available: request (method, path, client_ip, headers, body, api_key, user_id)
-- Return: { continue = true/false, error = "...", headers = {}, metadata = {} }

if request.path == "/v1/completions" then
    return { continue = false, error = "Endpoint not supported" }
end

-- Add trace ID
request.headers["X-Request-ID"] = generate_uuid()

return { continue = true }
```

#### on_response Example

```lua
-- Available: request, response (status_code, headers, body, latency_ms, backend_url)
log("Response status: " .. response.status_code)
return { continue = true }
```

---

## Lua Script Extension

All dynamic data modules support Lua script extension.

### Common Script Configuration

```yaml
script:
  enabled: true                    # Enable
  path: "./scripts/xxx.lua"        # Script file path
  # script: |                      # Or inline script
  #   return process(ctx, data)
  timeout: 1s                      # Execution timeout
  max_memory: 10                   # Max memory (MB)
```

### Modules Supporting Scripts

| Module | Config Path | Purpose |
|--------|-------------|---------|
| `discovery.sources[]` | `script` | Filter/transform service list |
| `auth.pipeline[]` | `script` | Custom auth logic |
| `logging.request` | `script` | Determine log recording |
| `logging.access` | `script` | Filter access logs |
| `rate_limit` | `script` | Custom rate limiting rules |
| `routing` | `script` | Custom route selection |
| `health_check` | `script` | Custom health judgment |
| `usage.reporters[]` | `script` | Filter/transform usage data |
| `hooks.*` | Various hooks | Lifecycle processing |

---

## Deprecated Fields

The following fields are deprecated, please use the new fields:

| Deprecated Field | New Field |
|------------------|-----------|
| `listen` | `server.listen` |
| `usage_hook` | `usage.reporters` |
| `api_keys` | `auth.pipeline[].static.keys` |
| `auth.storage: file` | `auth.pipeline[].type: static` |
| `load_balance_strategy` | `routing.load_balance` |

---

## Complete Configuration Example

See [config-reference.yaml](./config-reference.yaml) for a complete configuration example.

## Quick Start Examples

### Minimal Configuration

```yaml
server:
  listen: ":8000"

backends:
  - url: "http://localhost:11434"
```

### With Authentication

```yaml
server:
  listen: ":8000"

backends:
  - url: "http://localhost:11434"

admin:
  enabled: true
  token: "your-admin-token"

auth:
  enabled: true
  mode: first_match
  skip_paths:
    - /health
    - /metrics
  pipeline:
    - name: builtin_auth
      type: builtin
      enabled: true

usage:
  enabled: true
  reporters:
    - name: local
      type: builtin
      enabled: true
```

### Production Configuration

```yaml
server:
  listen: ":8000"
  read_timeout: 30s
  idle_timeout: 120s

log:
  level: info
  format: json

storage:
  databases:
    - name: primary
      driver: mysql
      host: db.example.com
      port: 3306
      user: llmproxy
      password: "${DB_PASSWORD}"
      database: llmproxy
      max_open_conns: 50
      max_idle_conns: 10
  caches:
    - name: primary
      driver: redis
      addr: redis.example.com:6379
      password: "${REDIS_PASSWORD}"
      pool_size: 50

backends:
  - name: vllm-1
    url: "http://vllm-1:8000"
    weight: 5
  - name: vllm-2
    url: "http://vllm-2:8000"
    weight: 5

auth:
  enabled: true
  mode: first_match
  skip_paths:
    - /health
    - /metrics
  pipeline:
    - name: redis_auth
      type: redis
      enabled: true
      redis:
        storage: primary
        key_pattern: "llmproxy:key:{api_key}"

rate_limit:
  enabled: true
  storage: redis
  redis: primary
  global:
    enabled: true
    requests_per_second: 1000
    burst_size: 2000
  per_key:
    enabled: true
    requests_per_second: 50
    max_concurrent: 20

routing:
  enabled: true
  load_balance: least_connections
  retry:
    enabled: true
    max_retries: 2

health_check:
  enabled: true
  interval: 10s
  timeout: 3s
  unhealthy_threshold: 3

metrics:
  enabled: true
  path: /metrics
```
