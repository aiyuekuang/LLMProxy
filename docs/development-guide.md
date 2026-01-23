# LLMProxy 开发文档

## 目录

- [项目概述](#项目概述)
- [架构设计](#架构设计)
- [目录结构](#目录结构)
- [核心模块](#核心模块)
- [配置说明](#配置说明)
- [开发指南](#开发指南)
- [API 参考](#api-参考)
- [部署指南](#部署指南)

---

## 项目概述

LLMProxy 是一个专为自建 LLM 推理服务设计的高性能网关，支持 vLLM、TGI、自研推理引擎等后端。

### 核心特性

| 特性 | 说明 |
|------|------|
| **零缓冲流式传输** | SSE 响应逐 token 转发，不增加首 token 延迟 |
| **透明代理** | 完全透传请求参数，不关心业务逻辑 |
| **多源鉴权管道** | 支持配置文件、Redis、数据库、Webhook，可编排 |
| **Lua 脚本决策** | 自定义鉴权逻辑，灵活控制放行/拒绝 |
| **负载均衡** | 轮询、最少连接数、延迟优先 |
| **限流保护** | 全局限流、Key 级限流、并发控制 |
| **监控计量** | Prometheus 指标、Webhook 用量上报 |

### 技术栈

- **语言**：Go 1.22+
- **依赖管理**：Go Modules
- **容器化**：Docker / Docker Compose
- **监控**：Prometheus + Grafana

---

## 架构设计

### 请求处理流程

```
客户端请求
    ↓
[限流中间件] → 检查全局/Key级限流
    ↓
[鉴权管道] → 多源验证 + Lua 脚本决策
    ↓
[代理处理器] → 选择后端 + 转发请求
    ↓
[流式响应] → 零缓冲逐 token 转发
    ↓
[Webhook] → 异步用量上报
```

### 鉴权管道架构

```
请求 → 提取 API Key
           ↓
    [Provider 1: File] → Lua 脚本
           ↓
    [Provider 2: Redis] → Lua 脚本
           ↓
    [Provider 3: Database] → Lua 脚本
           ↓
    [Provider 4: Webhook] → Lua 脚本
           ↓
      放行 / 拒绝
```

---

## 目录结构

```
LLMProxy/
├── cmd/
│   └── main.go                 # 程序入口
├── internal/
│   ├── admin/                  # Admin API 模块
│   │   ├── keystore.go         # API Key 存储（SQLite）
│   │   ├── server.go           # Admin API 服务器
│   │   └── usage.go            # 用量存储
│   ├── auth/                   # 鉴权模块
│   │   ├── keystore.go         # Key 存储接口
│   │   ├── middleware.go       # 鉴权中间件（旧）
│   │   └── pipeline/           # 鉴权管道（新）
│   │       ├── types.go        # 类型定义
│   │       ├── provider.go     # Provider 接口
│   │       ├── provider_file.go      # 配置文件 Provider
│   │       ├── provider_redis.go     # Redis Provider
│   │       ├── provider_database.go  # 数据库 Provider
│   │       ├── provider_webhook.go   # Webhook Provider
│   │       ├── provider_builtin.go   # 内置 SQLite Provider
│   │       ├── lua_executor.go       # Lua 脚本执行器
│   │       ├── executor.go           # 管道执行器
│   │       ├── middleware.go         # 管道中间件
│   │       └── config.go             # 配置转换
│   ├── config/
│   │   └── config.go           # 配置加载
│   ├── lb/                     # 负载均衡
│   │   ├── loadbalancer.go     # 接口定义
│   │   ├── round_robin.go      # 轮询策略
│   │   ├── least_connections.go # 最少连接数
│   │   └── latency_based.go    # 延迟优先
│   ├── metrics/
│   │   └── metrics.go          # Prometheus 指标
│   ├── proxy/
│   │   ├── handler.go          # 代理处理器
│   │   └── usage_reporter.go   # 用量上报
│   ├── ratelimit/              # 限流模块
│   │   ├── ratelimiter.go      # 限流接口
│   │   ├── memory.go           # 内存限流器
│   │   ├── redis_limiter.go    # Redis 限流器
│   │   └── middleware.go       # 限流中间件
│   ├── routing/
│   │   └── router.go           # 智能路由
│   ├── storage/                # 存储抽象层
│   │   └── manager.go          # 连接池管理
│   ├── types/                  # 公共类型
│   │   └── status.go           # Key 状态等
│   └── utils/
│       └── http.go             # HTTP 工具函数
├── docs/                       # 文档
│   ├── configuration.md        # 配置参考
│   ├── auth-pipeline.md        # 鉴权管道文档
│   ├── development-guide.md    # 开发文档（本文件）
│   └── opencode-integration.md # OpenCode 集成文档
├── deployments/                # 部署配置
│   ├── docker-compose.yml
│   └── prometheus.yml
├── examples/                   # 示例
│   └── webhook-receiver.py
├── Dockerfile
├── go.mod
├── go.sum
├── config.yaml.example
├── CHANGELOG.md
└── README.md
```

---

## 核心模块

### 1. 鉴权管道 (`internal/auth/pipeline/`)

可编排的多源鉴权系统，支持 Lua 脚本自定义决策逻辑。

#### Provider 接口

```go
type Provider interface {
    Name() string
    Type() ProviderType
    Query(ctx context.Context, apiKey string) *ProviderResult
    Close() error
}
```

#### 支持的 Provider 类型

| 类型 | 说明 | 数据格式 |
|------|------|----------|
| `builtin` | 内置 SQLite | Admin API 管理，需启用 admin |
| `file` | 配置文件 | YAML 中的 `api_keys` 列表 |
| `redis` | Redis | Hash 或 JSON String |
| `database` | 数据库 | MySQL/PostgreSQL/SQLite |
| `webhook` | HTTP 服务 | JSON 请求/响应 |
| `static` | 静态配置 | Pipeline 中直接配置 |

#### Lua 脚本执行

```go
// 可用变量
api_key   // 当前 API Key
data      // 从 Provider 查询到的数据
request   // 请求信息（method, path, ip, headers）
now()     // 当前时间戳

// 返回格式
return {
    allow = true/false,
    message = "错误原因",
    metadata = {user_id = "xxx"}
}
```

### 2. 代理处理器 (`internal/proxy/handler.go`)

核心请求处理逻辑，负责：
- 后端选择（通过负载均衡器）
- 请求转发
- 流式响应处理
- Webhook 用量上报

```go
func NewHandler(
    cfg *config.Config,
    lb lb.LoadBalancer,
    router *routing.Router,
    keyStore auth.KeyStore,
    limiter ratelimit.RateLimiter,
) http.HandlerFunc
```

### 3. 负载均衡 (`internal/lb/`)

支持三种策略：

| 策略 | 配置值 | 说明 |
|------|--------|------|
| 轮询 | `round_robin` | 按权重轮询分配 |
| 最少连接 | `least_connections` | 选择当前连接数最少的后端 |
| 延迟优先 | `latency_based` | 选择平均延迟最低的后端 |

### 4. 限流器 (`internal/ratelimit/`)

基于令牌桶算法实现：

```go
type RateLimiter interface {
    Allow(key string) bool
    AllowN(key string, n int) bool
}
```

---

## 配置说明

### 完整配置示例

```yaml
# 监听地址
listen: ":8000"

# 后端服务器
backends:
  - url: "http://vllm-1:8000"
    weight: 10
    models: ["qwen-72b"]
  - url: "http://vllm-2:8000"
    weight: 10

# 路由配置
routing:
  load_balance_strategy: "round_robin"  # round_robin | least_connections | latency_based
  retry:
    enabled: true
    max_retries: 3
    initial_wait: 1s
    max_wait: 10s
    multiplier: 2.0
  fallback:
    - primary: "http://vllm-1:8000"
      fallback: ["http://vllm-2:8000"]

# 鉴权配置
auth:
  enabled: true
  header_names: ["Authorization", "X-API-Key"]
  mode: "first_match"  # first_match | all
  
  pipeline:
    - name: "redis_auth"
      type: "redis"
      enabled: true
      redis:
        addr: "localhost:6379"
        key_pattern: "llmproxy:key:{api_key}"
      lua_script: |
        if tonumber(data.balance) <= 0 then
          return {allow = false, message = "余额不足"}
        end
        return {allow = true}

# 限流配置
rate_limit:
  enabled: true
  storage: "memory"
  global:
    enabled: true
    requests_per_second: 100
    burst_size: 200
  per_key:
    enabled: true
    requests_per_second: 10
    max_concurrent: 5

# 健康检查
health_check:
  interval: 10s
  path: "/health"

# Webhook 用量上报
usage_hook:
  enabled: true
  url: "http://billing:3000/api/usage"
  timeout: 3s
  retry: 3

# API Keys（用于 file provider）
api_keys:
  - key: "sk-xxx"
    name: "生产 Key"
    user_id: "user_001"
    status: "active"
    total_quota: 1000000
```

---

## 开发指南

### 环境准备

```bash
# 安装 Go 1.22+
go version

# 克隆项目
git clone https://github.com/aiyuekuang/LLMProxy.git
cd LLMProxy

# 下载依赖
go mod tidy
```

### 本地运行

```bash
# 复制配置文件
cp config.yaml.example config.yaml

# 编辑配置
vim config.yaml

# 运行
go run ./cmd --config config.yaml
```

### 构建

```bash
# 本地构建
go build -o llmproxy ./cmd

# Docker 构建
docker build -t llmproxy:latest .
```

### 测试

```bash
# 运行所有测试
go test ./...

# 运行特定模块测试
go test ./internal/auth/pipeline/...
```

### 添加新的 Provider

1. 在 `internal/auth/pipeline/` 创建 `provider_xxx.go`
2. 实现 `Provider` 接口
3. 在 `executor.go` 的 `createProvider()` 中添加 case
4. 在 `types.go` 中添加 `ProviderType` 常量
5. 更新配置结构和文档

```go
// provider_xxx.go
type XXXProvider struct {
    BaseProvider
    // 自定义字段
}

func NewXXXProvider(name string, cfg *XXXConfig) (Provider, error) {
    // 初始化逻辑
}

func (x *XXXProvider) Query(ctx context.Context, apiKey string) *ProviderResult {
    // 查询逻辑
}
```

---

## API 参考

### 代理端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/v1/chat/completions` | POST | OpenAI Chat Completions API |
| `/v1/completions` | POST | OpenAI Completions API |

### 管理端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/health` | GET | 健康检查 |
| `/metrics` | GET | Prometheus 指标 |

### 请求头

| Header | 说明 |
|--------|------|
| `Authorization: Bearer sk-xxx` | API Key（Bearer Token） |
| `X-API-Key: sk-xxx` | API Key（自定义 Header） |
| `Content-Type: application/json` | 请求体格式 |

### 错误响应

```json
{
  "error": "余额不足，请充值",
  "code": 403
}
```

---

## 部署指南

### Docker 单机部署

```bash
docker run -d \
  --name llmproxy \
  -p 8000:8000 \
  -v /path/to/config.yaml:/home/llmproxy/config.yaml \
  ghcr.io/aiyuekuang/llmproxy:latest
```

### Docker Compose 部署

```yaml
version: '3.8'
services:
  llmproxy:
    image: ghcr.io/aiyuekuang/llmproxy:latest
    ports:
      - "8000:8000"
    volumes:
      - ./config.yaml:/home/llmproxy/config.yaml
    depends_on:
      - redis
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    volumes:
      - redis-data:/data

volumes:
  redis-data:
```

### Kubernetes 部署

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: llmproxy
spec:
  replicas: 3
  selector:
    matchLabels:
      app: llmproxy
  template:
    metadata:
      labels:
        app: llmproxy
    spec:
      containers:
      - name: llmproxy
        image: ghcr.io/aiyuekuang/llmproxy:latest
        ports:
        - containerPort: 8000
        volumeMounts:
        - name: config
          mountPath: /home/llmproxy/config.yaml
          subPath: config.yaml
        livenessProbe:
          httpGet:
            path: /health
            port: 8000
          initialDelaySeconds: 5
          periodSeconds: 10
      volumes:
      - name: config
        configMap:
          name: llmproxy-config
```

---

## 性能优化建议

### 1. 鉴权性能

| 方案 | 延迟 | 适用场景 |
|------|------|----------|
| 配置文件（内存） | ~0.1ms | 开发测试、Key 数量少 |
| Redis | ~1ms | 生产环境、多实例 |
| SQLite | ~0.5ms | 单实例、数据量中等 |
| MySQL/PostgreSQL | ~5-20ms | 需要事务、复杂查询 |
| Webhook | ~50-200ms | 复杂业务逻辑 |

### 2. 建议配置

```yaml
# 生产环境推荐
auth:
  mode: "first_match"
  pipeline:
    # 1. 先查 Redis（快速路径）
    - name: "redis"
      type: "redis"
      enabled: true
    
    # 2. 降级查数据库
    - name: "database"
      type: "database"
      enabled: true
```

### 3. 连接池

```yaml
# 数据库连接池（代码中默认）
# MaxOpenConns: 10
# MaxIdleConns: 5
```

---

## 常见问题

### Q: 如何从旧版本迁移？

不配置 `pipeline` 时自动使用旧的 `storage: file` 模式，完全兼容。

### Q: Lua 脚本报错怎么排查？

查看日志中的 `Lua 脚本执行错误` 信息，包含详细错误原因。

### Q: 如何实现多实例共享状态？

使用 Redis Provider，所有实例连接同一个 Redis。

### Q: 如何自定义错误消息？

在 Lua 脚本中返回 `{allow = false, message = "自定义消息"}`。

---

## 相关文档

- [鉴权管道详细文档](auth-pipeline.md)
- [OpenCode 集成文档](opencode-integration.md)
- [Docker 发布指南](docker-publish-guide.md)
- [更新日志](../CHANGELOG.md)
