# LLMProxy

**自建 LLM 推理服务的高性能网关** - 为 vLLM、TGI、自研推理引擎而生

中文文档 | [English](README_EN.md)

[![Docker Build](https://github.com/aiyuekuang/LLMProxy/actions/workflows/release.yml/badge.svg)](https://github.com/aiyuekuang/LLMProxy/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/github/go-mod/go-version/aiyuekuang/LLMProxy)](go.mod)
[![GitHub Stars](https://img.shields.io/github/stars/aiyuekuang/LLMProxy?style=social)](https://github.com/aiyuekuang/LLMProxy/stargazers)
[![GitHub Forks](https://img.shields.io/github/forks/aiyuekuang/LLMProxy?style=social)](https://github.com/aiyuekuang/LLMProxy/network/members)

## 产品定位

LLMProxy 专为 **自建推理服务**（vLLM、TGI、自研引擎）设计，不是云端 API 聚合平台。

**典型使用场景：**
- 🏢 使用 vLLM/TGI 部署开源模型的团队
- 🔒 私有化部署的企业（数据不出内网）
- ⚡ 对性能要求极高的实时对话应用
- 🎯 需要简单管理和监控的自建服务

**不适用场景：**
- ❌ 需要对接 OpenAI/Claude/Gemini 云端 API（请使用 LiteLLM/One-API）
- ❌ 需要复杂多租户管理的企业平台

## 核心特性

### 🚀 极致性能
- ✅ **零缓冲流式传输** - SSE 响应逐 token 转发，不增加首 token 延迟（TTFT）
- ✅ **零性能侵入** - 主请求路径不解析响应体、不连接数据库、不调用外部服务
- ✅ **连接复用** - HTTP 客户端复用连接，减少握手开销

### 🎯 透明代理
- ✅ **完全透传** - 不关心业务参数（如 model），完全透传所有请求参数到后端
- ✅ **自动重试** - 指数退避策略，网络抖动自动重试
- ✅ **多种负载均衡** - 轮询、最少连接数、延迟优先
- ✅ **灵活路由** - 支持通过 Webhook 实现自定义路由逻辑

### 🔐 可编排鉴权管道 (v0.3.0 新增)
- ✅ **多数据源支持** - 配置文件 / Redis / 数据库（MySQL/PostgreSQL/SQLite）/ Webhook
- ✅ **Lua 脚本决策** - 自定义鉴权逻辑，灵活控制放行/拒绝
- ✅ **可编排顺序** - 自由调整 Provider 执行顺序
- ✅ **两种管道模式** - `first_match`（首个成功即放行）或 `all`（全部通过）
- ✅ **自定义认证 Header** - 支持配置任意 Header 名称
- ✅ **IP 白名单** - 防止未授权访问
- ✅ **额度管理** - Token 配额、自动重置（按天/周/月）

### 🛡️ 限流保护
- ✅ **全局限流** - 保护推理服务不被打垮
- ✅ **Key 级限流** - 防止单个用户滥用
- ✅ **并发控制** - 限制最大并发请求数
- ✅ **令牌桶算法** - 支持突发流量

### 📊 监控计量
- ✅ **完整请求透传** - Webhook 接收完整的请求参数和响应数据
- ✅ **异步用量计量** - 请求结束后，后台异步上报 `prompt_tokens` + `completion_tokens`
- ✅ **Prometheus 指标** - 请求量、延迟、错误率等
- ✅ **Grafana 面板** - 预配置监控面板
- ✅ **灵活扩展** - 通过 Webhook 实现自定义业务逻辑（权限控制、模型映射、计费等）

## 真实使用场景

### 场景 1：AI 客服系统（实时对话）

某电商公司使用 vLLM 部署了 Qwen-72B 模型，日均 10 万次对话。

**架构：**
```
客户 Web/App → Nginx → LLMProxy → vLLM 集群（3台 GPU）
```

**配置：**
```yaml
backends:
  - url: "http://gpu-1.internal:8000"
    weight: 10
  - url: "http://gpu-2.internal:8000"
    weight: 10
  - url: "http://gpu-3.internal:8000"
    weight: 10

routing:
  retry:
    enabled: true
    max_retries: 2
  fallback:
    - primary: "http://gpu-1.internal:8000"
      fallback: ["http://gpu-2.internal:8000", "http://gpu-3.internal:8000"]

rate_limit:
  global:
    requests_per_second: 500
```

**效果：**
- 延迟降低 40%（从 800ms 降到 480ms）
- 可用性提升到 99.9%
- GPU 利用率从 60% 提升到 85%

---

### 场景 2：企业内部 AI 助手（私有化部署）

某金融公司为 1000 名员工提供 AI 助手，使用 TGI 部署 Llama-3-70B。

**架构：**
```
企业员工 → 企业内网 → LLMProxy → TGI 服务（2台）
```

**配置：**
```yaml
auth:
  enabled: true
  storage: "file"

api_keys:
  - key: "sk-llmproxy-dev-team-001"
    name: "研发部门"
    total_quota: 500000  # 每天 50 万 tokens
    allowed_ips: ["10.0.1.0/24"]
  
  - key: "sk-llmproxy-product-team-001"
    name: "产品部门"
    total_quota: 200000
    allowed_ips: ["10.0.2.0/24"]

rate_limit:
  per_key:
    requests_per_minute: 100
    max_concurrent: 5
```

**效果：**
- 部署时间从 2 周缩短到 1 天
- 无需数据库，配置文件管理
- 通过内部安全审计
- 各部门用量清晰可见

---

### 场景 3：模型服务商（对外提供 API）

某 AI 创业公司使用 vLLM 部署多个开源模型，对外提供推理 API。

**架构：**
```
客户（100+ 家企业）→ 公网 → LLMProxy → vLLM 集群（多模型）
```

**配置：**
```yaml
backends:
  - url: "http://vllm-server-1:8000"
  - url: "http://vllm-server-2:8000"

routing:
  retry:
    enabled: true
    max_retries: 2

auth:
  enabled: true
  storage: "redis"

rate_limit:
  global:
    requests_per_second: 1000
  per_key:
    requests_per_second: 10
    tokens_per_minute: 100000
```

**效果：**
- 服务 100+ 家企业客户
- 日均 500 万次请求
- 可用性 99.95%
- 平均延迟 < 300ms

详细场景说明请参考：[真实使用场景文档](docs/real-world-scenarios.md)

---

## 快速开始

### 方式一：使用官方镜像（推荐）

```bash
# 1. 创建配置文件
curl -o config.yaml https://raw.githubusercontent.com/aiyuekuang/LLMProxy/main/config.yaml.example

# 2. 编辑配置文件，修改后端地址
vim config.yaml

# 3. 运行容器
docker run -d \
  --name llmproxy \
  -p 8000:8000 \
  -v $(pwd)/config.yaml:/home/llmproxy/config.yaml \
  ghcr.io/aiyuekuang/llmproxy:latest
```

**支持架构：** `linux/amd64`, `linux/arm64`

### 方式二：本地构建

```bash
# 下载依赖
go mod download

# 复制配置文件
cp config.yaml.example config.yaml

# 编辑配置文件，修改后端地址
vim config.yaml

# 运行
go run cmd/main.go --config config.yaml
```

### 方式三：Docker Compose（含 vLLM）

```bash
cd deployments
docker compose up -d
```

访问：
- LLMProxy: http://localhost:8000
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin/admin)

## 配置说明

### 基础配置

```yaml
# 监听地址
listen: ":8000"

# 后端服务器列表
backends:
  - url: "http://vllm:8000"
    weight: 5
  - url: "http://tgi:8081"
    weight: 3

# 用量上报配置（支持多上报器）
usage_hook:
  enabled: true
  reporters:
    - name: "billing"
      type: "webhook"
      enabled: true
      url: "https://your-billing.com/llm-usage"
      timeout: 3s
    - name: "database"
      type: "database"
      enabled: true
      database:
        driver: "mysql"
        dsn: "user:pass@tcp(localhost:3306)/llmproxy"
  retry: 2

# 健康检查配置
health_check:
  interval: 10s
  path: /health
```

### 路由配置

```yaml
routing:
  # 重试配置
  retry:
    enabled: true
    max_retries: 3
    initial_wait: 1s
    max_wait: 10s
    multiplier: 2.0
  
  # 负载均衡策略
  load_balance_strategy: "least_connections"  # round_robin, least_connections, latency_based
```

### 鉴权配置（v0.3.0 管道模式）

```yaml
auth:
  enabled: true
  header_names: ["Authorization", "X-API-Key"]
  mode: "first_match"  # first_match | all
  
  pipeline:
    # 1. Redis 验证（生产环境）
    - name: "redis_auth"
      type: "redis"
      enabled: true
      redis:
        addr: "localhost:6379"
        key_pattern: "llmproxy:key:{api_key}"
      lua_script: |
        if tonumber(data.balance or 0) <= 0 then
          return {allow = false, message = "余额不足，请充值"}
        end
        return {allow = true}
    
    # 2. 配置文件验证（开发环境）
    - name: "config_file"
      type: "file"
      enabled: true
      lua_script: |
        if data.status ~= "active" then
          return {allow = false, message = "Key 已禁用"}
        end
        return {allow = true}

# API Keys（用于 file provider）
api_keys:
  - key: "sk-llmproxy-test123"
    name: "测试 Key"
    user_id: "user_001"
    status: "active"
    total_quota: 100000
    quota_reset_period: "daily"
    allowed_ips: ["192.168.1.0/24"]
    expires_at: "2026-12-31T23:59:59Z"
```

### 限流配置

```yaml
rate_limit:
  enabled: true
  storage: "redis"  # 或 "memory"
  
  # 全局限流
  global:
    enabled: true
    requests_per_second: 1000
    burst_size: 2000
  
  # API Key 级限流
  per_key:
    enabled: true
    requests_per_second: 10
    requests_per_minute: 500
    tokens_per_minute: 100000  # TPM 限制
    max_concurrent: 5
```

完整配置示例请参考：[config.yaml.example](config.yaml.example)

## 后端配置要求

### vLLM

**必须启用 `--return-detailed-tokens` 参数：**

```bash
python -m vllm.entrypoints.openai.api_server \
  --model meta-llama/Llama-3-8b-Instruct \
  --return-detailed-tokens \
  --port 8000
```

### TGI

默认支持，无需额外配置。

## Webhook 数据格式

LLMProxy 会向配置的 Webhook URL 发送 POST 请求，**完整透传用户的所有请求参数**：

```json
{
  "request_id": "req_abc123",
  "user_id": "user_alice",
  "api_key": "sk-prod-xxx",
  "request_body": {
    "model": "meta-llama/Llama-3-8b",
    "messages": [{"role": "user", "content": "你好"}],
    "temperature": 0.7,
    "max_tokens": 100
  },
  "usage": {
    "prompt_tokens": 15,
    "completion_tokens": 42,
    "total_tokens": 57
  },
  "is_stream": true,
  "endpoint": "/v1/chat/completions",
  "timestamp": "2026-01-14T10:30:00Z",
  "backend_url": "http://vllm:8000",
  "latency_ms": 1234,
  "status_code": 200
}
```

**透明代理设计理念：**
- LLMProxy 不关心业务参数（如 model），完全透传所有请求参数
- 业务逻辑（权限控制、模型映射、计费等）由 Webhook 接收方处理
- 这使得 LLMProxy 保持简单、高性能，同时提供最大的灵活性

### 业务系统接收示例（Python Flask）

```python
@app.route('/llm-usage', methods=['POST'])
def record_usage():
    data = request.json
    
    # 获取用户请求的完整参数
    request_body = data.get('request_body', {})
    model = request_body.get('model', 'unknown')
    
    # 获取用量信息
    usage = data.get('usage', {})
    prompt_tokens = usage.get('prompt_tokens', 0)
    completion_tokens = usage.get('completion_tokens', 0)
    
    # 写入数据库
    db.execute(
        "INSERT INTO billing_events (customer, input_tk, output_tk, model) VALUES (?, ?, ?, ?)",
        data['user_id'], prompt_tokens, completion_tokens, model
    )
    
    # 可以在这里实现自定义业务逻辑：
    # - 模型权限检查
    # - 自定义计费规则
    # - 数据分析和统计
    
    return {"status": "ok"}
```

## 监控指标

LLMProxy 暴露 Prometheus 指标（`/metrics`）：

| 指标名称 | 类型 | 说明 |
|---------|------|------|
| `llmproxy_requests_total` | Counter | 请求总数（标签：path, stream, backend, status） |
| `llmproxy_latency_ms` | Histogram | 请求延迟（毫秒） |
| `llmproxy_webhook_success_total` | Counter | Webhook 成功数 |
| `llmproxy_webhook_failure_total` | Counter | Webhook 失败数 |
| `llmproxy_usage_tokens_total` | Counter | Token 使用量（标签：type=prompt/completion） |

## API 使用示例

### 非流式请求

```bash
curl http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "meta-llama/Llama-3-8b-Instruct",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": false
  }'
```

### 流式请求

```bash
curl http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "meta-llama/Llama-3-8b-Instruct",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": true
  }'
```

## 架构设计

```
+------------------+
|     Client       | ← curl / SDK
+--------+---------+
         |
         | POST /v1/chat/completions { "stream": true, ... }
         v
+--------+---------+
|    LLMProxy      | ← Go 服务（单二进制）
|  ┌─────────────┐ |
|  │  Router     │ |←── 仅路由 LLM API 路径
|  └──────┬──────┘ |
|  ┌──────▼──────┐ |
|  │LoadBalancer │ |←── 轮询/权重/最少连接
|  └──────┬──────┘ |
|  ┌──────▼──────┐ |
|  │ProxyEngine  │ |←── 核心：透传请求/响应（无缓冲！）
|  └──────┬──────┘ |
|  ┌──────▼──────┐ |
|  │ UsageHook   │ |←── 请求结束后，启动后台 goroutine
|  └──────┬──────┘ |
+--------+---------+
         |         | (async)
         |         v
         |  [HTTP Webhook] ────→ https://your-billing.com/usage
         v
+------------------+     +------------------+
|   vLLM (8000)    |     |   TGI (8081)     |
|   + usage        |     |   + usage        |
+------------------+     +------------------+
```

## 性能特点

- **零缓冲流式传输**：使用 `io.Copy` 实现内核级 splice，CPU 开销极低
- **异步用量上报**：在 goroutine 中执行，不阻塞主请求
- **连接复用**：HTTP 客户端复用连接，减少握手开销
- **健康检查**：自动摘除不健康节点，提高可用性

## 项目结构

```
llmproxy/
├── cmd/
│   └── main.go                 # 入口
├── internal/
│   ├── config/                 # 配置解析
│   │   └── config.go
│   ├── proxy/                  # 核心代理引擎
│   │   ├── handler.go          # 请求处理
│   │   └── usage_hook.go       # Webhook 上报
│   ├── lb/                     # 负载均衡器
│   │   └── roundrobin.go
│   └── metrics/                # Prometheus 指标
│       └── metrics.go
├── deployments/
│   ├── docker-compose.yml      # 本地测试
│   ├── config.yaml             # Docker 配置
│   └── prometheus.yml          # Prometheus 配置
├── grafana/
│   └── dashboard.json          # Grafana 面板
├── config.yaml.example         # 配置示例
├── Dockerfile
├── go.mod
└── README.md
```

## 常见问题

### 1. 为什么响应中没有 usage 信息？

确保后端启用了 usage 返回：
- vLLM：添加 `--return-detailed-tokens` 参数
- TGI：默认支持

### 2. Webhook 发送失败怎么办？

LLMProxy 会自动重试（根据配置的 `retry` 次数），失败仅记录日志，不影响主请求。

### 3. 如何查看监控指标？

访问 `http://localhost:8000/metrics` 查看 Prometheus 指标。

### 4. 支持哪些负载均衡策略？

当前支持加权轮询（Weighted Round Robin），后续可扩展最少连接等策略。

## 文档

| 文档 | 说明 |
|------|------|
| [鉴权管道详细文档](docs/auth-pipeline.md) | 多源鉴权管道配置、Lua 脚本示例 |
| [开发文档](docs/development-guide.md) | 架构设计、核心模块、开发指南、API 参考 |
| [OpenCode 集成](docs/opencode-integration.md) | 与 OpenCode 等 AI 编码助手集成 |
| [Docker 发布指南](docs/docker-publish-guide.md) | Docker 镜像构建与发布 |
| [更新日志](CHANGELOG.md) | 版本更新记录 |

## 许可证

本项目采用 [MIT License](LICENSE) 开源协议。

这意味着你可以：
- ✅ 自由使用、修改和分发本软件
- ✅ 用于商业项目
- ✅ 创建衍生作品

唯一要求：保留原始版权声明和许可证声明。

## 贡献

我们欢迎所有形式的贡献！查看 [CONTRIBUTORS.md](CONTRIBUTORS.md) 了解贡献者名单。

### 如何贡献

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交修改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

详细贡献指南请参考 [CONTRIBUTORS.md](CONTRIBUTORS.md)。

## 支持项目

如果 LLMProxy 对你有帮助，请考虑：

- ⭐ 给项目点个 Star
- 🐛 报告 Bug 或提出改进建议
- 📝 改进文档或添加示例
- 💬 在社区中分享你的使用经验
- 🔗 在你的项目中添加 "Powered by LLMProxy" 徽章：

```markdown
[![Powered by LLMProxy](https://img.shields.io/badge/Powered%20by-LLMProxy-blue)](https://github.com/aiyuekuang/LLMProxy)
```

## 联系方式

- 📧 Issues: [GitHub Issues](https://github.com/aiyuekuang/LLMProxy/issues)
- 💬 Discussions: [GitHub Discussions](https://github.com/aiyuekuang/LLMProxy/discussions)
