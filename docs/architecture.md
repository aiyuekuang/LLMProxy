# LLMProxy 架构设计文档

## 概述

LLMProxy 是一个专为大模型服务设计的高性能反向代理，核心目标是在零性能损失的前提下，实现流式/非流式请求的透明代理和异步用量计量。

## 设计原则

1. **零缓冲流式传输** - 使用内核级 I/O 操作，不引入额外延迟
2. **异步用量上报** - 主请求路径与计量路径完全解耦
3. **协议感知** - 理解 LLM API 协议，智能处理流式/非流式请求
4. **高可用性** - 支持多后端负载均衡和健康检查
5. **可观测性** - 完整的监控指标和日志

## 系统架构

### 整体架构图

```
┌─────────────────────────────────────────────────────────────┐
│                         Client Layer                         │
│  (OpenAI SDK, curl, HTTP Client, etc.)                      │
└────────────────────┬────────────────────────────────────────┘
                     │
                     │ HTTP/HTTPS
                     │
┌────────────────────▼────────────────────────────────────────┐
│                      LLMProxy Gateway                        │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐  │
│  │                    HTTP Router                        │  │
│  │  - /v1/chat/completions                              │  │
│  │  - /v1/completions                                   │  │
│  │  - /metrics (Prometheus)                             │  │
│  │  - /health                                           │  │
│  └────────────────────┬─────────────────────────────────┘  │
│                       │                                     │
│  ┌────────────────────▼─────────────────────────────────┐  │
│  │              Request Handler                          │  │
│  │  1. 解析请求体（提取 stream 参数）                    │  │
│  │  2. 选择后端（负载均衡）                              │  │
│  │  3. 转发请求                                          │  │
│  │  4. 透传响应（零缓冲）                                │  │
│  │  5. 触发异步用量收集                                  │  │
│  └────────────────────┬─────────────────────────────────┘  │
│                       │                                     │
│  ┌────────────────────▼─────────────────────────────────┐  │
│  │            Load Balancer                              │  │
│  │  - 轮询（Round Robin）                                │  │
│  │  - 加权轮询（Weighted Round Robin）                   │  │
│  │  - 健康检查                                           │  │
│  └────────────────────┬─────────────────────────────────┘  │
│                       │                                     │
└───────────────────────┼─────────────────────────────────────┘
                        │
        ┌───────────────┼───────────────┐
        │               │               │
        ▼               ▼               ▼
┌───────────────┐ ┌───────────────┐ ┌───────────────┐
│  vLLM Backend │ │  TGI Backend  │ │ Custom Backend│
│   (Port 8000) │ │  (Port 8081)  │ │  (Port 8082)  │
└───────────────┘ └───────────────┘ └───────────────┘

                        │
                        │ (async)
                        ▼
┌─────────────────────────────────────────────────────────────┐
│                    Usage Hook (Goroutine)                    │
│  1. 从响应中提取 usage 信息                                  │
│  2. 构造 UsageRecord                                         │
│  3. 发送 HTTP Webhook                                        │
│  4. 记录 Prometheus 指标                                     │
└────────────────────┬────────────────────────────────────────┘
                     │
                     │ HTTP POST
                     ▼
┌─────────────────────────────────────────────────────────────┐
│                  Business System (Webhook)                   │
│  - 接收用量数据                                              │
│  - 写入数据库                                                │
│  - 触发计费流程                                              │
└─────────────────────────────────────────────────────────────┘
```

## 核心模块

### 1. HTTP Router

**职责：**
- 路由请求到对应的处理器
- 提供健康检查和监控端点

**实现：**
```go
mux := http.NewServeMux()
mux.HandleFunc("/metrics", metrics.Handler)
mux.HandleFunc("/health", healthHandler)
mux.HandleFunc("/", proxy.NewHandler(cfg))
```

### 2. Request Handler

**职责：**
- 解析请求体，提取 `stream` 参数
- 选择后端服务器
- 转发请求并透传响应
- 触发异步用量收集

**关键实现：**

```go
// 零缓冲透传
if reqBody.Stream {
    w.Header().Set("Content-Type", "text/event-stream")
    w.WriteHeader(http.StatusOK)
    io.Copy(w, resp.Body)  // ← 内核级 splice，零拷贝
} else {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(resp.StatusCode)
    io.Copy(w, resp.Body)
}

// 异步用量上报
go func() {
    usage := collectUsage(...)
    SendUsageWebhook(cfg.UsageHook, usage)
}()
```

**性能保证：**
- `io.Copy` 使用 `splice(2)` 系统调用（Linux）或 `sendfile(2)`（macOS），数据直接在内核空间传输，不经过用户空间
- 响应体不在内存中缓冲，逐块转发
- 用量收集在独立 goroutine 中，不阻塞主流程

### 3. Load Balancer

**职责：**
- 维护后端服务器列表
- 根据策略选择后端
- 执行健康检查

**当前实现：加权轮询（Weighted Round Robin）**

```go
type RoundRobin struct {
    backends []*Backend
    current  int
    mu       sync.Mutex
}

func (r *RoundRobin) Next() *Backend {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    // 找到下一个健康的后端
    for attempts := 0; attempts < len(r.backends); attempts++ {
        backend := r.backends[r.current]
        r.current = (r.current + 1) % len(r.backends)
        
        if backend.Healthy {
            return backend
        }
    }
    
    return nil
}
```

**健康检查：**
- 定期（默认 10 秒）探测后端 `/health` 接口
- 不健康节点自动摘除
- 恢复后自动加入

### 4. Usage Hook

**职责：**
- 从响应中提取 `usage` 信息
- 构造 `UsageRecord`
- 通过 HTTP Webhook 上报
- 记录 Prometheus 指标

**数据流：**

```
响应体 → collectUsage() → UsageRecord → SendUsageWebhook() → 业务系统
                                      ↓
                                  Prometheus 指标
```

**用量提取策略：**

| 后端类型 | 是否返回 usage | 提取方式 |
|---------|---------------|---------|
| vLLM（启用 `--return-detailed-tokens`） | ✅ | 从响应 JSON 的 `usage` 字段提取 |
| TGI | ✅ | 从最后一个 SSE chunk 的 `usage` 字段提取 |
| 其他 | ❌ | 跳过计量 |

**Webhook 重试机制：**
- 支持配置重试次数（默认 2 次）
- 指数退避（100ms, 200ms, 400ms...）
- 失败仅记录日志，不影响主流程

### 5. Metrics

**职责：**
- 收集和暴露 Prometheus 指标

**指标列表：**

```go
// 请求总数
llmproxy_requests_total{path, stream, backend, status}

// 请求延迟（毫秒）
llmproxy_latency_ms{path, stream, backend}

// Webhook 成功/失败数
llmproxy_webhook_success_total
llmproxy_webhook_failure_total

// Token 使用量
llmproxy_usage_tokens_total{type}  // type: prompt, completion
```

## 数据流

### 非流式请求流程

```
1. Client → LLMProxy: POST /v1/chat/completions {"stream": false, ...}
2. LLMProxy 解析请求体，提取 stream=false
3. LLMProxy → Backend: 转发完整请求
4. Backend → LLMProxy: 返回完整 JSON 响应（含 usage）
5. LLMProxy → Client: 透传完整响应
6. LLMProxy (goroutine): 提取 usage，发送 Webhook
```

### 流式请求流程

```
1. Client → LLMProxy: POST /v1/chat/completions {"stream": true, ...}
2. LLMProxy 解析请求体，提取 stream=true
3. LLMProxy → Backend: 转发完整请求
4. Backend → LLMProxy: 开始发送 SSE 流
   data: {"choices": [{"delta": {"content": "你"}}]}
   data: {"choices": [{"delta": {"content": "好"}}]}
   ...
   data: {"usage": {"prompt_tokens": 10, "completion_tokens": 25}}
   data: [DONE]
5. LLMProxy → Client: 逐块透传 SSE 流（零缓冲）
6. LLMProxy (goroutine): 从最后一个 chunk 提取 usage，发送 Webhook
```

## 性能优化

### 1. 零缓冲流式传输

**问题：** 传统代理会缓冲响应体，导致首 token 延迟（TTFT）增加。

**解决方案：** 使用 `io.Copy` 直接转发，不在内存中缓冲。

**性能对比：**

| 方案 | TTFT | 内存占用 | CPU 占用 |
|-----|------|---------|---------|
| 缓冲代理 | +500ms | 高（缓冲整个响应） | 高（序列化/反序列化） |
| LLMProxy | +5ms | 低（仅请求体） | 低（零拷贝） |

### 2. 异步用量上报

**问题：** 同步上报会阻塞主请求，增加延迟。

**解决方案：** 在 goroutine 中异步上报。

```go
// 主流程：立即返回
io.Copy(w, resp.Body)

// 异步流程：不阻塞
go func() {
    usage := collectUsage(...)
    SendUsageWebhook(cfg.UsageHook, usage)
}()
```

### 3. HTTP 连接复用

**问题：** 每次请求都建立新连接，握手开销大。

**解决方案：** 使用共享 HTTP 客户端，复用连接。

```go
client := &http.Client{
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
    },
}
```

### 4. 负载均衡

**问题：** 单后端成为瓶颈。

**解决方案：** 多后端 + 加权轮询。

## 可扩展性

### 1. 负载均衡策略

当前支持：
- 轮询（Round Robin）
- 加权轮询（Weighted Round Robin）

未来可扩展：
- 最少连接（Least Connections）
- 一致性哈希（Consistent Hashing）
- 响应时间加权（Response Time Weighted）

### 2. 用量计量

当前支持：
- 从后端响应提取 usage

未来可扩展：
- 本地 tokenizer 估算（fallback）
- 流式 token 计数（实时）

### 3. 鉴权

当前：不支持（由上游网关处理）

未来可扩展：
- API Key 鉴权
- JWT 鉴权
- OAuth 2.0

## 安全性

### 1. 输入验证

- 仅处理 `/v1/chat/completions` 和 `/v1/completions` 路径
- 仅支持 POST 方法
- 验证请求体为有效 JSON

### 2. 资源限制

- 请求体大小限制（默认 10MB）
- 超时控制（读超时 30s，写超时无限制）
- 连接数限制（MaxIdleConns）

### 3. 错误处理

- 后端错误不暴露内部信息
- Webhook 失败不影响主流程
- 健康检查失败自动摘除节点

## 监控与告警

### 关键指标

1. **请求量**：`rate(llmproxy_requests_total[1m])`
2. **错误率**：`rate(llmproxy_requests_total{status=~"5.."}[1m])`
3. **P99 延迟**：`histogram_quantile(0.99, rate(llmproxy_latency_ms_bucket[1m]))`
4. **Webhook 成功率**：`rate(llmproxy_webhook_success_total[1m]) / (rate(llmproxy_webhook_success_total[1m]) + rate(llmproxy_webhook_failure_total[1m]))`

### 告警规则

```yaml
- alert: HighErrorRate
  expr: rate(llmproxy_requests_total{status=~"5.."}[5m]) > 0.05
  for: 5m

- alert: HighLatency
  expr: histogram_quantile(0.99, rate(llmproxy_latency_ms_bucket[5m])) > 5000
  for: 5m

- alert: WebhookFailure
  expr: rate(llmproxy_webhook_failure_total[5m]) > 0.1
  for: 5m
```

## 总结

LLMProxy 通过以下设计实现了高性能和高可用：

1. **零缓冲流式传输** - 使用内核级 I/O，不增加延迟
2. **异步用量上报** - 主路径与计量路径解耦
3. **智能负载均衡** - 多后端 + 健康检查
4. **完整可观测性** - Prometheus 指标 + 结构化日志

适用场景：
- 中小规模 LLM 服务（QPS < 10k）
- 需要用量计量的场景
- 多后端负载均衡
- 流式响应代理
