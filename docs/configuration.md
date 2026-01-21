# LLMProxy 配置文档

## 目录

- [概述](#概述)
- [配置结构](#配置结构)
- [存储配置](#存储配置-storage)
- [后端服务](#后端服务-backends)
- [服务发现](#服务发现-discovery)
- [鉴权模块](#鉴权模块-auth)
- [日志模块](#日志模块-logging)
- [限流模块](#限流模块-rate_limit)
- [路由模块](#路由模块-routing)
- [健康检查](#健康检查-health_check)
- [指标模块](#指标模块-metrics)
- [用量上报](#用量上报-usage)
- [Lua 脚本扩展](#lua-脚本扩展)
- [生命周期钩子](#生命周期钩子-hooks)
- [完整示例](#完整示例)

---

## 概述

LLMProxy 使用 YAML 格式的配置文件，采用模块化设计，每个功能模块独立配置。

### 命名规范

| 字段名 | 含义 | 示例 |
|-------|------|------|
| `enabled` | 模块/功能开关 | `enabled: true` |
| `driver` | 驱动类型 | `driver: "mysql"` |
| `storage` | 引用顶层存储 | `storage: "database"` |
| `interval` | 时间间隔 | `interval: 30s` |
| `timeout` | 超时时间 | `timeout: 5s` |
| `path` | 路径（文件或 URL） | `path: "/health"` |
| `url` | 完整 URL | `url: "http://..."` |
| `addr` | 地址（host:port） | `addr: "localhost:6379"` |

### 时间格式

支持 Go 标准时间格式：`1s`, `30s`, `1m`, `5m`, `1h`

---

## 配置结构

```
config.yaml
├── listen              # 监听地址
├── storage             # 存储连接配置
│   ├── database        # 数据库连接
│   └── cache           # 缓存连接
├── backends            # 静态后端列表
├── discovery           # 服务发现模块
├── auth                # 鉴权模块
├── logging             # 日志模块
├── rate_limit          # 限流模块
├── routing             # 路由模块
├── health_check        # 健康检查模块
├── metrics             # 指标模块
├── usage               # 用量上报模块
├── scripts             # Lua 脚本模块
└── api_keys            # 静态 API Key
```

---

## 存储配置 (storage)

顶层存储连接池配置，供其他模块通过 `name` 引用。

### 数据库连接池 (storage.databases)

支持 MySQL、PostgreSQL、SQLite，可配置多个。

```yaml
storage:
  databases:
    - name: "primary"            # 连接名称
      driver: "mysql"            # mysql / postgres / sqlite
      host: "localhost"
      port: 3306
      user: "root"
      password: "password"
      database: "llmproxy"
    
    - name: "logs"               # 日志专用库
      driver: "mysql"
      host: "logs-db"
      database: "llmproxy_logs"
    
    - name: "local"              # 本地 SQLite
      driver: "sqlite"
      path: "./data/local.db"
```

### 缓存连接池 (storage.caches)

支持 Redis、Memory，可配置多个。

```yaml
storage:
  caches:
    - name: "primary"            # 主缓存
      driver: "redis"
      addr: "localhost:6379"
      password: ""
      db: 0
    
    - name: "ratelimit"          # 限流专用
      driver: "redis"
      addr: "redis-ratelimit:6379"
      db: 1
    
    - name: "local"              # 本地内存
      driver: "memory"
      max_size: 10000
```

### 引用方式

其他模块通过 `storage: "<name>"` 引用：

```yaml
discovery:
  sources:
    - database:
        storage: "primary"       # 引用 databases[name=primary]

logging:
  request:
    storage: "logs"              # 引用 databases[name=logs]

rate_limit:
  storage: "ratelimit"           # 引用 caches[name=ratelimit]
```

---

## 后端服务 (backends)

静态配置的后端服务列表。

```yaml
backends:
  - url: "http://localhost:8000"
    weight: 5                    # 负载均衡权重
  
  - url: "http://localhost:8001"
    weight: 3
```

| 字段 | 类型 | 必填 | 说明 |
|-----|------|-----|------|
| `url` | string | 是 | 后端服务 URL |
| `weight` | int | 否 | 权重，默认 1 |

---

## 服务发现 (discovery)

从多种数据源动态加载后端服务，类似 auth 的 pipeline 模式。

```yaml
discovery:
  enabled: true
  mode: "merge"                  # merge(合并) / first(首个有效)
  interval: 30s                  # 全局同步间隔
  sources:
    - name: "db_discovery"
      type: "database"
      enabled: true
      database:
        storage: "database"
        table: "services"
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

### 模式

| mode | 说明 |
|-----|------|
| `merge` | 合并所有源的服务列表 |
| `first` | 使用第一个可用源 |

### 示例：数据库发现

```yaml
- name: "db_discovery"
  type: "database"
  enabled: true
  database:
    storage: "primary"
    table: "services"
  script:                        # Lua 后处理（可选）
    enabled: true
    path: "./scripts/discovery_filter.lua"
```

### 示例：Consul 发现

```yaml
- name: "consul_discovery"
  type: "consul"
  enabled: true
  consul:
    addr: "http://consul:8500"
    service: "llm-backend"
    tag: "production"
```

### 示例：Kubernetes 发现

```yaml
- name: "k8s_discovery"
  type: "kubernetes"
  enabled: true
  kubernetes:
    namespace: "llm"
    service: "vllm"
    port: 8000
    label_selector: "app=vllm"
```

---

## 鉴权模块 (auth)

API Key 验证，支持管道模式。

```yaml
auth:
  enabled: true
  mode: "first_match"            # first_match / all
  header_names:
    - "Authorization"
    - "X-API-Key"
  pipeline:
    - name: "redis_auth"
      type: "redis"
      enabled: true
      redis:
        storage: "cache"
        key_pattern: "llmproxy:key:{api_key}"
```

### 鉴权模式

| 模式 | 说明 |
|-----|------|
| `first_match` | 首个通过即可 |
| `all` | 所有提供者都必须通过 |

### 提供者类型

#### Redis

```yaml
- name: "redis_auth"
  type: "redis"
  enabled: true
  redis:
    storage: "cache"
    key_pattern: "llmproxy:key:{api_key}"
```

#### Database

```yaml
- name: "db_auth"
  type: "database"
  enabled: true
  database:
    storage: "database"
    table: "api_keys"
    key_column: "key"
    fields: ["user_id", "quota", "status"]
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
```

#### Lua

```yaml
- name: "lua_auth"
  type: "lua"
  enabled: true
  lua:
    path: "./scripts/auth.lua"
    timeout: 1s
    max_memory: 10
```

#### Static (静态配置)

```yaml
- name: "static_auth"
  type: "static"
  enabled: true
  static:
    keys:
      - key: "sk-test-key-1"
        name: "测试 Key"
        user_id: "user_001"
        status: "enabled"
        total_quota: 1000000
        used_quota: 0
        quota_reset_period: "monthly"
        allowed_ips: []
        denied_ips: []
        expires_at: null
```

---

## 日志模块 (logging)

请求日志和访问日志。

```yaml
logging:
  enabled: true
  
  request:
    enabled: true
    storage: "database"          # database / file / stdout
  
  access:
    enabled: false
    storage: "file"
    file:
      path: "./logs/access.log"
      rotate: "daily"
      max_age: 7
```

### 存储类型

| 类型 | 说明 |
|-----|------|
| `database` | 写入数据库 |
| `file` | 写入文件 |
| `stdout` | 输出到控制台 |

---

## 限流模块 (rate_limit)

请求频率限制。

```yaml
rate_limit:
  enabled: true
  storage: "cache"               # cache / memory
  
  global:
    enabled: true
    requests_per_second: 100
    requests_per_minute: 1000
    burst_size: 200
  
  per_key:
    enabled: true
    requests_per_second: 10
    requests_per_minute: 60
    tokens_per_minute: 100000
    max_concurrent: 10
    burst_size: 20
```

---

## 路由模块 (routing)

负载均衡、重试和故障转移。

```yaml
routing:
  enabled: true
  load_balance: "weighted"       # round_robin / weighted / least_conn / random
  
  retry:
    enabled: true
    max_retries: 3
    initial_wait: 1s
    max_wait: 10s
    multiplier: 2.0
  
  fallback:
    - primary: "http://localhost:8000"
      fallback:
        - "http://localhost:8001"
        - "http://localhost:8002"
```

### 负载均衡策略

| 策略 | 说明 |
|-----|------|
| `round_robin` | 轮询 |
| `weighted` | 加权轮询 |
| `least_conn` | 最少连接 |
| `random` | 随机 |

---

## 健康检查 (health_check)

后端服务健康检查。

```yaml
health_check:
  enabled: true
  interval: 30s
  timeout: 5s
  path: "/health"
  unhealthy_threshold: 3
  healthy_threshold: 2
```

| 字段 | 说明 |
|-----|------|
| `interval` | 检查间隔 |
| `timeout` | 超时时间 |
| `path` | 健康检查路径 |
| `unhealthy_threshold` | 连续失败次数判定不健康 |
| `healthy_threshold` | 连续成功次数判定健康 |

---

## 指标模块 (metrics)

Prometheus 指标暴露。

```yaml
metrics:
  enabled: true
  path: "/metrics"
```

---

## 用量上报 (usage)

Token 用量统计上报。

```yaml
usage:
  enabled: true
  reporters:
    - name: "billing"
      type: "webhook"
      enabled: true
      webhook:
        url: "https://billing.example.com/usage"
        method: "POST"
        timeout: 5s
        retry: 3
    
    - name: "db"
      type: "database"
      enabled: false
      database:
        storage: "database"
        table: "usage_records"
```

---

## Lua 脚本 (scripts)

Lua 脚本扩展。

```yaml
scripts:
  enabled: true
  
  routing:
    enabled: true
    path: "./scripts/routing.lua"
    timeout: 1s
    max_memory: 10
  
  request_transform:
    enabled: false
    path: "./scripts/request.lua"
  
  response_transform:
    enabled: false
    path: "./scripts/response.lua"
  
  error_handler:
    enabled: false
    path: "./scripts/error.lua"
```

---

## Lua 脚本扩展

所有动态数据模块都支持 Lua 脚本扩展，用于自定义处理逻辑。

### 通用配置

```yaml
script:
  enabled: true                  # 是否启用
  path: "./scripts/xxx.lua"      # 脚本文件路径
  # code: |                      # 或内联脚本
  #   return process(ctx, data)
  timeout: 1s                    # 执行超时
  max_memory: 10                 # 最大内存 (MB)
```

### 支持 Lua 扩展的模块

| 模块 | 扩展点 | 用途 |
|-----|-------|------|
| `discovery.sources[]` | 后处理 | 过滤/转换服务列表 |
| `auth.pipeline[]` | 后处理 | 自定义鉴权逻辑 |
| `logging.request` | 前处理 | 决定是否记录日志 |
| `rate_limit` | 前处理 | 自定义限流规则 |
| `routing` | 前处理 | 自定义路由选择 |

### 示例：日志过滤

```lua
-- scripts/log_filter.lua
function process(ctx, log)
    -- 不记录健康检查
    if ctx.request.path == "/health" then
        return { skip = true }
    end
    -- 不记录内网请求
    if ctx.request.client_ip:match("^10%.") then
        return { skip = true }
    end
    return { skip = false }
end
```

### 示例：自定义限流

```lua
-- scripts/ratelimit.lua
function process(ctx, usage)
    -- VIP 用户不限流
    if ctx.user_id:match("^vip_") then
        return { skip = true }
    end
    return { allowed = true }
end
```

### 示例：数据库鉴权后处理

```lua
-- scripts/auth_db_post.lua
function process(ctx, db_result)
    -- 组合多字段判断
    if db_result.status ~= "enabled" then
        return { allowed = false, error = "Key disabled" }
    end
    if db_result.quota <= 0 then
        return { allowed = false, error = "Quota exceeded" }
    end
    return { allowed = true, user_id = db_result.user_id }
end
```

---

## 生命周期钩子 (hooks)

请求处理的全局 Lua 钩子，在请求生命周期的不同阶段触发。

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
  
  on_request:                    # 请求进入
    enabled: true
    path: "./scripts/on_request.lua"
  
  on_auth:                       # 鉴权完成后
    enabled: false
    path: "./scripts/on_auth.lua"
  
  on_route:                      # 路由选择时
    enabled: false
    path: "./scripts/on_route.lua"
  
  on_response:                   # 响应返回前
    enabled: true
    path: "./scripts/on_response.lua"
  
  on_error:                      # 发生错误时
    enabled: true
    path: "./scripts/on_error.lua"
  
  on_complete:                   # 请求完成时
    enabled: false
    path: "./scripts/on_complete.lua"
```

### 钩子说明

| 钩子 | 触发时机 | 用途 |
|-----|---------|------|
| `on_request` | 请求进入 | 添加追踪 ID、修改请求头 |
| `on_auth` | 鉴权通过后 | 获取用户信息、权限检查 |
| `on_route` | 路由选择时 | 自定义路由逻辑 |
| `on_response` | 响应返回前 | 修改响应内容、添加字段 |
| `on_error` | 发生错误时 | 自定义错误响应格式 |
| `on_complete` | 请求完成后 | 清理资源、统计上报 |

---

## 完整示例

参见 [config-reference.yaml](./config-reference.yaml)
