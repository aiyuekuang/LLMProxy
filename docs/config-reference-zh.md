# LLMProxy 配置参考文档

本文档包含 LLMProxy 所有配置项的完整说明。

## 目录

- [概述](#概述)
- [服务器配置 (server)](#服务器配置-server)
- [系统日志配置 (log)](#系统日志配置-log)
- [存储配置 (storage)](#存储配置-storage)
- [后端服务 (backends)](#后端服务-backends)
- [服务发现 (discovery)](#服务发现-discovery)
- [Admin API (admin)](#admin-api-admin)
- [鉴权配置 (auth)](#鉴权配置-auth)
- [请求/访问日志 (logging)](#请求访问日志-logging)
- [限流配置 (rate_limit)](#限流配置-rate_limit)
- [路由配置 (routing)](#路由配置-routing)
- [健康检查 (health_check)](#健康检查-health_check)
- [指标配置 (metrics)](#指标配置-metrics)
- [用量上报 (usage)](#用量上报-usage)
- [生命周期钩子 (hooks)](#生命周期钩子-hooks)
- [废弃字段](#废弃字段)

---

## 概述

LLMProxy 使用 YAML 格式的配置文件，采用模块化设计。

### 命名规范

| 字段名 | 含义 | 示例 |
|-------|------|------|
| `enabled` | 模块/功能开关 | `enabled: true` |
| `driver` | 驱动类型 | `driver: "mysql"` |
| `storage` | 引用顶层存储连接 | `storage: "primary"` |
| `interval` | 时间间隔 | `interval: 30s` |
| `timeout` | 超时时间 | `timeout: 5s` |
| `path` | 路径（文件或 URL 路径） | `path: "/health"` |
| `url` | 完整 URL 地址 | `url: "http://..."` |
| `addr` | 地址（host:port 格式） | `addr: "localhost:6379"` |

### 时间格式

支持 Go 标准时间格式：`1s`, `30s`, `1m`, `5m`, `1h`

### 配置文件结构

```
config.yaml
├── server              # 服务器配置
├── log                 # 系统日志配置
├── storage             # 存储连接配置
│   ├── databases       # 数据库连接池
│   └── caches          # 缓存连接池
├── backends            # 静态后端列表
├── discovery           # 服务发现
├── admin               # Admin API
├── auth                # 鉴权配置
├── logging             # 请求/访问日志
├── rate_limit          # 限流配置
├── routing             # 路由配置
├── health_check        # 健康检查
├── metrics             # 指标配置
├── usage               # 用量上报
└── hooks               # 生命周期钩子
```

---

## 服务器配置 (server)

HTTP 服务器相关配置。

```yaml
server:
  listen: ":8000"                  # 监听地址，格式: ":端口" 或 "IP:端口"
  read_timeout: 30s                # 读取超时
  write_timeout: 60s               # 写入超时（流式响应时实际为 0）
  idle_timeout: 120s               # 空闲连接超时
  max_header_bytes: 1048576        # 最大请求头大小 (默认 1MB)
  max_body_size: 10485760          # 最大请求体大小 (默认 10MB)
  
  # CORS 跨域配置
  cors:
    enabled: false                 # 是否启用
    allowed_origins:               # 允许的来源
      - "*"
    allowed_methods:               # 允许的方法
      - "GET"
      - "POST"
      - "OPTIONS"
    allowed_headers:               # 允许的请求头
      - "Authorization"
      - "Content-Type"
      - "X-API-Key"
    expose_headers: []             # 暴露的响应头
    allow_credentials: false       # 是否允许携带凭证
    max_age: 86400                 # 预检请求缓存时间（秒）
  
  # TLS/HTTPS 配置
  tls:
    enabled: false                 # 是否启用 HTTPS
    cert_file: "./certs/server.crt"
    key_file: "./certs/server.key"
    client_ca_file: ""             # 客户端 CA 证书（用于双向 TLS）
    client_auth: "none"            # 客户端认证: none / request / require
```

### 字段说明

| 字段 | 类型 | 默认值 | 说明 |
|-----|------|-------|------|
| `listen` | string | `:8000` | 监听地址 |
| `read_timeout` | duration | `30s` | 读取请求的超时时间 |
| `write_timeout` | duration | `60s` | 写入响应的超时时间 |
| `idle_timeout` | duration | `120s` | 空闲连接超时时间 |
| `max_header_bytes` | int | `1048576` | 最大请求头大小（字节） |
| `max_body_size` | int64 | `10485760` | 最大请求体大小（字节） |

> **注意**: 对于流式响应 (streaming)，`write_timeout` 会被设置为 0 以避免长时间流被中断。

---

## 系统日志配置 (log)

LLMProxy 运行时日志配置。

> **注意**: `log` 是系统运行日志，`logging` 是请求/访问日志，两者用途不同。

```yaml
log:
  level: "info"                    # 日志级别: debug / info / warn / error
  format: "json"                   # 格式: json / text
  output: "stdout"                 # 输出: stdout / stderr / file
  
  # 当 output=file 时的配置
  file:
    path: "./logs/llmproxy.log"
    rotate: "daily"                # 轮转: daily / hourly / size
    max_size: 100                  # MB (当 rotate=size 时)
    max_age: 7                     # 保留天数
    compress: true                 # 是否压缩旧日志
```

### 字段说明

| 字段 | 类型 | 默认值 | 说明 |
|-----|------|-------|------|
| `level` | string | `info` | 日志级别 |
| `format` | string | `json` | 输出格式 |
| `output` | string | `stdout` | 输出目标 |

---

## 存储配置 (storage)

定义数据库和缓存连接池，供其他模块引用。

### 数据库连接池 (storage.databases)

支持 MySQL、PostgreSQL、SQLite。

```yaml
storage:
  databases:
    - name: "primary"              # 连接名称，供其他模块引用
      enabled: true                # 是否启用
      driver: "mysql"              # 驱动: mysql / postgres / sqlite
      host: "localhost"            # 主机地址
      port: 3306                   # 端口
      user: "root"                 # 用户名
      password: "password"         # 密码
      database: "llmproxy"         # 数据库名
      # dsn: ""                    # 或直接指定 DSN（优先级更高）
      max_open_conns: 100          # 最大打开连接数
      max_idle_conns: 10           # 最大空闲连接数
      conn_max_lifetime: 1h        # 连接最大生命周期
      conn_max_idle_time: 10m      # 空闲连接最大时间
    
    - name: "local"                # SQLite 示例
      enabled: true
      driver: "sqlite"
      path: "./data/local.db"      # SQLite 文件路径
```

### 数据库字段说明

| 字段 | 类型 | 说明 |
|-----|------|------|
| `name` | string | 连接名称（必填），用于其他模块引用 |
| `enabled` | bool | 是否启用此连接 |
| `driver` | string | 驱动类型: `mysql` / `postgres` / `sqlite` |
| `dsn` | string | 直接指定 DSN 连接字符串（优先级高于其他字段） |
| `host` | string | 数据库主机地址 |
| `port` | int | 端口号（MySQL 默认 3306，PostgreSQL 默认 5432） |
| `user` | string | 用户名 |
| `password` | string | 密码 |
| `database` | string | 数据库名 |
| `path` | string | SQLite 数据库文件路径 |
| `max_open_conns` | int | 最大打开连接数 |
| `max_idle_conns` | int | 最大空闲连接数 |
| `conn_max_lifetime` | duration | 连接最大生命周期 |
| `conn_max_idle_time` | duration | 空闲连接最大时间 |

### 缓存连接池 (storage.caches)

支持 Redis、Memory。

```yaml
storage:
  caches:
    - name: "primary"              # 连接名称
      enabled: true                # 是否启用
      driver: "redis"              # 驱动: redis / memory
      addr: "localhost:6379"       # Redis 地址
      password: ""                 # Redis 密码
      db: 0                        # Redis 数据库编号
      pool_size: 100               # 连接池大小
      min_idle_conns: 10           # 最小空闲连接数
      dial_timeout: 5s             # 连接超时
      read_timeout: 3s             # 读取超时
      write_timeout: 3s            # 写入超时
    
    - name: "local"                # 本地内存缓存
      enabled: true
      driver: "memory"
      max_size: 10000              # 最大条目数
      ttl: 5m                      # 默认过期时间
```

### 缓存字段说明

| 字段 | 类型 | 说明 |
|-----|------|------|
| `name` | string | 连接名称（必填） |
| `enabled` | bool | 是否启用 |
| `driver` | string | 驱动: `redis` / `memory` |
| `addr` | string | Redis 地址（host:port） |
| `password` | string | Redis 密码 |
| `db` | int | Redis 数据库编号 |
| `pool_size` | int | 连接池大小 |
| `min_idle_conns` | int | 最小空闲连接数 |
| `dial_timeout` | duration | 连接超时 |
| `read_timeout` | duration | 读取超时 |
| `write_timeout` | duration | 写入超时 |
| `max_size` | int | 内存缓存最大条目数 |
| `ttl` | duration | 内存缓存默认 TTL |

### 引用方式

其他模块通过 `storage: "<name>"` 引用：

```yaml
auth:
  pipeline:
    - redis:
        storage: "primary"         # 引用 caches[name=primary]

logging:
  request:
    storage: "logs"                # 引用 databases[name=logs]

rate_limit:
  redis: "primary"                 # 引用 caches[name=primary]
```

---

## 后端服务 (backends)

静态配置的后端服务列表。

```yaml
backends:
  - name: "vllm-1"                 # 后端名称（用于日志和监控）
    url: "http://localhost:8000"   # 后端服务 URL
    weight: 5                      # 负载均衡权重
    timeout: 60s                   # 请求超时
    connect_timeout: 5s            # 连接超时
    max_idle_conns: 100            # 最大空闲连接
    headers:                       # 自定义请求头（可选）
      X-Backend-ID: "backend-1"
  
  - name: "vllm-2"
    url: "http://localhost:8001"
    weight: 3
```

### 字段说明

| 字段 | 类型 | 默认值 | 说明 |
|-----|------|-------|------|
| `name` | string | - | 后端名称 |
| `url` | string | - | 后端服务 URL（必填） |
| `weight` | int | `1` | 负载均衡权重 |
| `timeout` | duration | `60s` | 请求超时 |
| `connect_timeout` | duration | `5s` | 连接超时 |
| `max_idle_conns` | int | `100` | 最大空闲连接 |
| `headers` | map | - | 自定义请求头 |

---

## 服务发现 (discovery)

从多种数据源动态加载后端服务配置。

```yaml
discovery:
  enabled: true                    # 是否启用
  mode: "merge"                    # 模式: merge(合并所有源) / first(首个有效源)
  interval: 30s                    # 全局同步间隔
  
  sources:                         # 发现源列表
    # 数据库发现
    - name: "db_discovery"
      type: "database"
      enabled: true
      database:
        storage: "primary"         # 引用 storage.databases[name]
        table: "services"          # 服务表名
        fields:                    # 字段映射
          name: "name"
          url: "endpoint"
          weight: "weight"
          status: "status"
      script:                      # Lua 后处理脚本（可选）
        enabled: false
        path: "./scripts/discovery_filter.lua"
    
    # 静态配置
    - name: "static_discovery"
      type: "static"
      enabled: false
      static:
        backends:
          - name: "static-1"
            url: "http://localhost:8000"
            weight: 5
    
    # Consul 服务发现
    - name: "consul_discovery"
      type: "consul"
      enabled: false
      consul:
        addr: "http://consul:8500"
        service: "llm-backend"
        tag: "production"
        interval: 10s
    
    # Kubernetes 服务发现
    - name: "k8s_discovery"
      type: "kubernetes"
      enabled: false
      kubernetes:
        namespace: "llm"
        service: "vllm"
        port: 8000
        label_selector: "app=vllm"
    
    # Etcd 服务发现
    - name: "etcd_discovery"
      type: "etcd"
      enabled: false
      etcd:
        endpoints:
          - "http://etcd:2379"
        prefix: "/services/llm"
        username: ""
        password: ""
    
    # HTTP 服务发现
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

### 发现源类型

| type | 说明 | 适用场景 |
|-----|------|---------|
| `database` | 从数据库读取 | Admin 管理 |
| `static` | 配置文件静态定义 | 简单部署 |
| `consul` | Consul 服务发现 | 微服务架构 |
| `kubernetes` | K8s Service/Endpoints | 云原生 |
| `etcd` | Etcd KV 存储 | 分布式系统 |
| `http` | HTTP API 获取 | 自定义注册中心 |

### 模式说明

| mode | 说明 |
|-----|------|
| `merge` | 合并所有源的服务列表 |
| `first` | 使用第一个可用源 |

---

## Admin API (admin)

内置管理 API，提供 API Key 的 CRUD 操作和用量查询。

```yaml
admin:
  enabled: true                    # 是否启用
  token: "your-secure-admin-token" # 访问令牌（必填）
  listen: ""                       # 监听地址（留空则挂载到主服务器）
  db_path: "./data/keys.db"        # SQLite 数据库路径
```

### 字段说明

| 字段 | 类型 | 默认值 | 说明 |
|-----|------|-------|------|
| `enabled` | bool | `false` | 是否启用 Admin API |
| `token` | string | - | 访问令牌（必填），通过 `X-Admin-Token` Header 传递 |
| `listen` | string | `""` | 独立监听地址，留空则与主服务共用端口 |
| `db_path` | string | `./data/keys.db` | SQLite 数据库路径 |

### Admin API 端点

| 端点 | 说明 |
|------|------|
| `POST /admin/keys/create` | 创建 API Key |
| `POST /admin/keys/update` | 更新 API Key |
| `POST /admin/keys/delete` | 删除 API Key |
| `POST /admin/keys/get` | 获取 API Key |
| `POST /admin/keys/list` | 列出 API Key |
| `POST /admin/keys/sync` | 批量同步 API Key |

> **注意**: `auth.pipeline` 中的 `builtin` 类型和 `usage.reporters` 中的 `builtin` 类型都依赖此模块。

---

## 鉴权配置 (auth)

API Key 验证配置，支持多种验证方式的管道模式。

```yaml
auth:
  enabled: true                    # 是否启用
  mode: "first_match"              # 模式: first_match / all
  
  skip_paths:                      # 跳过鉴权的路径
    - "/health"
    - "/ready"
    - "/metrics"
  
  header_names:                    # 认证头名称列表
    - "Authorization"
    - "X-API-Key"
  
  # 状态码配置（可选）
  status_codes:
    disabled:
      http_code: 403
      message: "API Key 已被禁用"
    expired:
      http_code: 403
      message: "API Key 已过期"
    quota_exceeded:
      http_code: 429
      message: "额度已用尽"
    not_found:
      http_code: 401
      message: "无效的 API Key"
  
  pipeline:                        # 鉴权管道（按顺序执行）
    # ... 见下方各类型详细配置
```

### 鉴权模式

| 模式 | 说明 |
|-----|------|
| `first_match` | 首个提供者通过即可 |
| `all` | 所有启用的提供者都必须通过 |

### 提供者类型

#### Builtin (内置 SQLite 存储)

使用 Admin 模块的 SQLite 数据库，需要启用 `admin.enabled: true`。

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
    storage: "primary"             # 引用 storage.caches[name]
    key_pattern: "llmproxy:key:{api_key}"
  script:                          # Lua 后处理脚本（可选）
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
    storage: "primary"             # 引用 storage.databases[name]
    table: "api_keys"              # 表名
    key_column: "key"              # API Key 列名
    fields:                        # 需要查询的字段
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
    path: "./scripts/auth.lua"     # 脚本文件路径
    # script: |                    # 或内联脚本
    #   return true
    timeout: 1s
    max_memory: 10                 # MB
```

#### Static (静态配置)

```yaml
- name: "static_auth"
  type: "static"
  enabled: true
  static:
    keys:
      - key: "sk-test-key-1"
        name: "测试 Key 1"
        user_id: "user_001"
        status: "enabled"          # enabled / disabled
        total_quota: 1000000       # 总配额（Token）
        used_quota: 0
        quota_reset_period: "monthly"  # daily / weekly / monthly / never
        allowed_ips: []            # IP 白名单
        denied_ips: []             # IP 黑名单
        expires_at: null           # 过期时间
```

### API Key 字段说明

| 字段 | 类型 | 说明 |
|-----|------|------|
| `key` | string | API Key 值 |
| `name` | string | Key 名称 |
| `user_id` | string | 用户 ID |
| `status` | string | 状态: `enabled` / `disabled` |
| `total_quota` | int64 | 总配额（Token） |
| `used_quota` | int64 | 已用配额 |
| `quota_reset_period` | string | 配额重置周期: `daily` / `weekly` / `monthly` / `never` |
| `allowed_ips` | []string | IP 白名单 |
| `denied_ips` | []string | IP 黑名单 |
| `expires_at` | time | 过期时间 |

---

## 请求/访问日志 (logging)

请求日志和访问日志配置。

```yaml
logging:
  enabled: true
  
  # 请求日志（详细记录每个 API 请求）
  request:
    enabled: true
    storage: "primary"             # 引用 storage.databases[name]
    table: "request_logs"          # 表名
    include_body: false            # 是否记录请求/响应体
    script:
      enabled: false
      path: "./scripts/log_filter.lua"
    # 文件存储配置（可选）
    file:
      path: "./logs/requests.log"
      max_size_mb: 100
      max_backups: 7
  
  # 访问日志（类似 Nginx access log）
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

### 字段说明

| 字段 | 类型 | 说明 |
|-----|------|------|
| `storage` | string | 数据库存储引用 |
| `table` | string | 表名 |
| `include_body` | bool | 是否记录请求/响应体 |
| `format` | string | 访问日志格式: `combined` / `json` |
| `output` | string | 输出目标: `file` / `stdout` |

---

## 限流配置 (rate_limit)

请求频率限制配置。

```yaml
rate_limit:
  enabled: true
  storage: "memory"                # 存储: memory / redis
  redis: "primary"                 # 当 storage=redis 时，引用 storage.caches[name]
  
  script:                          # Lua 自定义限流脚本
    enabled: false
    path: "./scripts/ratelimit.lua"
    timeout: 1s
    max_memory: 10
  
  # 全局限流
  global:
    enabled: true
    requests_per_second: 100       # 每秒请求数
    requests_per_minute: 1000      # 每分钟请求数
    burst_size: 200                # 突发容量
  
  # 按 Key 限流
  per_key:
    enabled: true
    requests_per_second: 10        # 每秒请求数
    requests_per_minute: 60        # 每分钟请求数
    tokens_per_minute: 100000      # 每分钟 Token 数
    max_concurrent: 10             # 最大并发数
    burst_size: 20                 # 突发容量
```

### 字段说明

| 字段 | 类型 | 默认值 | 说明 |
|-----|------|-------|------|
| `storage` | string | `memory` | 存储类型 |
| `redis` | string | - | Redis 缓存引用 |
| `requests_per_second` | int | - | 每秒请求数限制 |
| `requests_per_minute` | int | - | 每分钟请求数限制 |
| `tokens_per_minute` | int64 | - | 每分钟 Token 数限制 |
| `max_concurrent` | int | - | 最大并发请求数 |
| `burst_size` | int | - | 令牌桶突发容量 |

---

## 路由配置 (routing)

负载均衡、重试和故障转移配置。

```yaml
routing:
  enabled: true
  load_balance: "round_robin"      # 策略: round_robin / least_connections / latency_based
  
  timeout: 60s                     # 总请求超时
  connect_timeout: 5s              # 连接超时
  
  script:                          # Lua 自定义路由脚本
    enabled: false
    path: "./scripts/routing.lua"
    timeout: 1s
    max_memory: 10
  
  # 重试配置
  retry:
    enabled: true
    max_retries: 3                 # 最大重试次数
    initial_wait: 1s               # 初始等待时间
    max_wait: 10s                  # 最大等待时间
    multiplier: 2.0                # 退避乘数
    retry_on:                      # 重试条件
      - "5xx"                      # 5xx 服务端错误
      - "connect_failure"          # 连接失败
      - "timeout"                  # 超时
  
  # 故障转移配置
  fallback:
    - models: []                   # 适用的模型（空表示所有）
      primary: "http://localhost:8000"
      fallback:
        - "http://localhost:8001"
        - "http://localhost:8002"
```

### 负载均衡策略

| 策略 | 说明 |
|-----|------|
| `round_robin` | 轮询 |
| `least_connections` | 最少连接 |
| `latency_based` | 基于延迟 |

### 重试配置字段

| 字段 | 类型 | 默认值 | 说明 |
|-----|------|-------|------|
| `max_retries` | int | `3` | 最大重试次数 |
| `initial_wait` | duration | `1s` | 初始等待时间 |
| `max_wait` | duration | `10s` | 最大等待时间 |
| `multiplier` | float64 | `2.0` | 指数退避乘数 |
| `retry_on` | []string | - | 重试条件列表 |

---

## 健康检查 (health_check)

后端服务健康检查配置。

```yaml
health_check:
  enabled: true
  interval: 30s                    # 检查间隔
  timeout: 5s                      # 超时时间
  method: "GET"                    # HTTP 方法
  path: "/health"                  # 健康检查路径
  expected_status: 200             # 期望的状态码
  unhealthy_threshold: 3           # 连续失败次数判定为不健康
  healthy_threshold: 2             # 连续成功次数判定为健康
  
  script:                          # Lua 自定义健康判断脚本
    enabled: false
    path: "./scripts/health_check.lua"
    timeout: 1s
    max_memory: 10
```

### 字段说明

| 字段 | 类型 | 默认值 | 说明 |
|-----|------|-------|------|
| `interval` | duration | `30s` | 检查间隔 |
| `timeout` | duration | `5s` | 超时时间 |
| `method` | string | `GET` | HTTP 方法 |
| `path` | string | `/health` | 健康检查路径 |
| `expected_status` | int | `200` | 期望的状态码 |
| `unhealthy_threshold` | int | `3` | 不健康阈值 |
| `healthy_threshold` | int | `2` | 健康阈值 |

---

## 指标配置 (metrics)

Prometheus 指标暴露配置。

```yaml
metrics:
  enabled: true
  path: "/metrics"                 # 指标端点路径
  
  custom_labels:                   # 自定义标签
    - "user_id"
    - "api_key"
  
  latency_buckets:                 # 延迟直方图桶 (秒)
    - 0.01
    - 0.05
    - 0.1
    - 0.5
    - 1.0
    - 5.0
    - 10.0
```

### 字段说明

| 字段 | 类型 | 默认值 | 说明 |
|-----|------|-------|------|
| `path` | string | `/metrics` | 指标端点路径 |
| `custom_labels` | []string | - | 自定义标签列表 |
| `latency_buckets` | []float64 | - | 延迟直方图桶配置 |

---

## 用量上报 (usage)

Token 用量统计上报配置。

```yaml
usage:
  enabled: true
  
  reporters:                       # 上报器列表（可配置多个）
    # 内置 SQLite 存储
    - name: "local"
      type: "builtin"
      enabled: true
      builtin:
        retention_days: 30         # 数据保留天数，0=永久
    
    # Webhook 上报
    - name: "billing"
      type: "webhook"
      enabled: true
      webhook:
        url: "https://billing.example.com/usage"
        method: "POST"
        timeout: 5s
        retry: 3                   # 重试次数
        headers:
          Authorization: "Bearer xxx"
      script:
        enabled: false
        path: "./scripts/usage_filter.lua"
    
    # 数据库上报
    - name: "db_usage"
      type: "database"
      enabled: false
      database:
        storage: "primary"         # 引用 storage.databases[name]
        table: "usage_records"     # 表名
      script:
        enabled: false
        path: "./scripts/usage_db.lua"
```

### 上报器类型

| 类型 | 说明 | 依赖 |
|-----|------|------|
| `builtin` | 内置 SQLite 存储 | 需要启用 `admin` |
| `webhook` | HTTP Webhook 上报 | - |
| `database` | 外部数据库存储 | 需要配置 `storage.databases` |

### Builtin 配置

| 字段 | 类型 | 默认值 | 说明 |
|-----|------|-------|------|
| `retention_days` | int | `0` | 数据保留天数，0 表示永久 |

### Webhook 配置

| 字段 | 类型 | 说明 |
|-----|------|------|
| `url` | string | Webhook URL |
| `method` | string | HTTP 方法 |
| `timeout` | duration | 超时时间 |
| `retry` | int | 重试次数 |
| `headers` | map | 自定义请求头 |

---

## 生命周期钩子 (hooks)

请求处理的全局 Lua 钩子。

### 执行顺序

```
on_request → on_auth → on_route → [后端处理] → on_response → on_complete
                                        ↓
                                   on_error (如发生错误)
```

### 配置示例

```yaml
hooks:
  enabled: true
  
  # 请求进入时
  on_request:
    enabled: true
    path: "./scripts/on_request.lua"
    # script: |                    # 或内联脚本
    #   -- 可用变量: request
    #   return { continue = true }
    timeout: 100ms
    max_memory: 10
  
  # 鉴权完成后
  on_auth:
    enabled: false
    path: "./scripts/on_auth.lua"
    timeout: 100ms
    max_memory: 10
  
  # 路由选择时
  on_route:
    enabled: false
    path: "./scripts/on_route.lua"
    timeout: 100ms
    max_memory: 10
  
  # 响应返回前
  on_response:
    enabled: false
    path: "./scripts/on_response.lua"
    timeout: 100ms
    max_memory: 10
  
  # 发生错误时
  on_error:
    enabled: false
    path: "./scripts/on_error.lua"
    timeout: 100ms
    max_memory: 10
  
  # 请求完成时（异步执行）
  on_complete:
    enabled: false
    path: "./scripts/on_complete.lua"
    timeout: 100ms
    max_memory: 10
```

### 钩子说明

| 钩子 | 触发时机 | 可用变量 | 用途 |
|-----|---------|---------|------|
| `on_request` | 请求进入 | `request` | 添加追踪 ID、修改请求头、拦截请求 |
| `on_auth` | 鉴权通过后 | `request`, `auth_result` | 获取用户信息、权限检查 |
| `on_route` | 路由选择时 | `request`, `backends` | 自定义路由逻辑 |
| `on_response` | 响应返回前 | `request`, `response` | 修改响应内容 |
| `on_error` | 发生错误时 | `request`, `error_message` | 自定义错误响应 |
| `on_complete` | 请求完成后 | `request`, `response` | 清理资源、统计上报 |

### Lua 脚本示例

#### on_request 示例

```lua
-- 可用变量: request (method, path, client_ip, headers, body, api_key, user_id)
-- 返回: { continue = true/false, error = "...", headers = {}, metadata = {} }

if request.path == "/v1/completions" then
    return { continue = false, error = "不支持此端点" }
end

-- 添加追踪 ID
request.headers["X-Request-ID"] = generate_uuid()

return { continue = true }
```

#### on_response 示例

```lua
-- 可用变量: request, response (status_code, headers, body, latency_ms, backend_url)
log("Response status: " .. response.status_code)
return { continue = true }
```

---

## Lua 脚本扩展

所有动态数据模块都支持 Lua 脚本扩展。

### 通用脚本配置

```yaml
script:
  enabled: true                    # 是否启用
  path: "./scripts/xxx.lua"        # 脚本文件路径
  # script: |                      # 或内联脚本
  #   return process(ctx, data)
  timeout: 1s                      # 执行超时
  max_memory: 10                   # 最大内存 (MB)
```

### 支持脚本的模块

| 模块 | 配置路径 | 用途 |
|-----|---------|------|
| `discovery.sources[]` | `script` | 过滤/转换服务列表 |
| `auth.pipeline[]` | `script` | 自定义鉴权逻辑 |
| `logging.request` | `script` | 决定是否记录日志 |
| `logging.access` | `script` | 过滤访问日志 |
| `rate_limit` | `script` | 自定义限流规则 |
| `routing` | `script` | 自定义路由选择 |
| `health_check` | `script` | 自定义健康判断 |
| `usage.reporters[]` | `script` | 过滤/转换用量数据 |
| `hooks.*` | 各钩子 | 生命周期处理 |

---

## 废弃字段

以下字段已废弃，请使用新字段：

| 废弃字段 | 新字段 |
|---------|-------|
| `listen` | `server.listen` |
| `usage_hook` | `usage.reporters` |
| `api_keys` | `auth.pipeline[].static.keys` |
| `auth.storage: file` | `auth.pipeline[].type: static` |
| `load_balance_strategy` | `routing.load_balance` |

---

## 完整配置示例

请参阅 [config-reference.yaml](./config-reference.yaml) 获取完整的配置示例。

## 快速开始示例

### 最小配置

```yaml
server:
  listen: ":8000"

backends:
  - url: "http://localhost:11434"
```

### 带鉴权的配置

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

### 生产环境配置

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
