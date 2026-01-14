# LLMProxy 项目设计方案讨论

## 项目概述
LLMProxy 是一个面向大模型服务的高性能网关，支持流式/非流式无缝代理 + 异步用量计量（HTTP Webhook）。

## 核心功能需求
1. ✅ LLM 协议感知代理 - 自动识别 stream 参数
2. ✅ 零缓冲流式传输 - SSE 响应逐 token 转发
3. ✅ 多后端负载均衡 - 支持 vLLM、TGI 等
4. ✅ 异步用量计量 - 后台异步上报 token 使用量
5. ✅ 零性能侵入 - 主请求路径不解析响应体
6. ✅ 极简业务对接 - HTTP Webhook 推送用量数据

## 技术栈选择方案

### 方案 A：Go 语言实现（推荐）
**优势：**
- 高性能：原生支持并发（goroutine），内存占用低
- 零缓冲流式传输：io.Copy 使用内核级 splice，CPU 开销极低
- 部署简单：单二进制文件，无运行时依赖
- 生态成熟：丰富的 HTTP 库和中间件

**劣势：**
- 开发周期相对较长（相比 Node.js）

### 方案 B：Node.js 实现
**优势：**
- 开发速度快
- 异步 I/O 天然支持

**劣势：**
- 性能不如 Go
- 内存占用较高
- 流式处理需要额外注意背压问题

### 方案 C：Rust 实现
**优势：**
- 极致性能
- 内存安全

**劣势：**
- 开发周期长
- 学习曲线陡峭
- 生态相对不成熟

## 架构设计方案

### 方案 1：单体架构（推荐）
```
Client → LLMProxy (单进程) → Backend (vLLM/TGI)
              ↓
         Webhook (异步)
```

**优势：**
- 部署简单，运维成本低
- 延迟最低（无额外网络跳转）
- 适合中小规模场景

**劣势：**
- 单点故障风险（可通过多实例 + LB 解决）

### 方案 2：微服务架构
```
Client → API Gateway → Proxy Service → Backend
                     → Usage Service → Webhook
```

**优势：**
- 职责分离，易于扩展
- 用量服务可独立扩容

**劣势：**
- 增加延迟
- 运维复杂度高
- 过度设计（对于当前需求）

## 核心模块设计

### 1. 代理引擎（ProxyEngine）
**职责：**
- 接收客户端请求
- 识别 stream 参数
- 透传请求到后端
- 零缓冲转发响应

**关键实现：**
- 使用 `io.Copy` 实现零缓冲
- 不解析响应体内容
- 支持 SSE 流式传输

### 2. 负载均衡器（LoadBalancer）
**支持策略：**
- 轮询（Round Robin）- 默认
- 加权轮询（Weighted Round Robin）
- 最少连接（Least Connections）

**健康检查：**
- 定期探测后端健康状态
- 自动摘除不健康节点

### 3. 用量计量钩子（UsageHook）
**职责：**
- 请求结束后异步收集用量
- 通过 HTTP Webhook 上报

**数据来源：**
- vLLM：需启用 `--return-detailed-tokens`
- TGI：默认返回 usage 字段
- 其他：跳过计量（不估算）

**上报格式：**
```json
{
  "request_id": "req_abc123",
  "user_id": "user_alice",
  "api_key": "sk-prod-xxx",
  "model": "meta-llama/Llama-3-8b",
  "prompt_tokens": 15,
  "completion_tokens": 42,
  "total_tokens": 57,
  "is_stream": true,
  "endpoint": "/v1/chat/completions",
  "timestamp": "2026-01-14T10:30:00Z",
  "backend_url": "http://vllm:8000"
}
```

### 4. 配置管理（Config）
**配置项：**
- 监听地址和端口
- 后端列表（URL + 权重）
- Webhook 配置（URL、超时、重试）
- 健康检查配置

**配置格式：** YAML

### 5. 监控指标（Metrics）
**Prometheus 指标：**
- `llmproxy_requests_total` - 请求总数
- `llmproxy_latency_ms_bucket` - 延迟分布
- `llmproxy_webhook_success_total` - Webhook 成功数
- `llmproxy_webhook_failure_total` - Webhook 失败数

## 项目结构方案

### 方案 1：标准 Go 项目结构（推荐）
```
llmproxy/
├── cmd/
│   └── main.go                 # 入口
├── internal/
│   ├── config/                 # 配置解析
│   ├── proxy/                  # 核心代理引擎
│   ├── lb/                     # 负载均衡器
│   └── metrics/                # Prometheus 指标
├── deployments/
│   ├── docker-compose.yml      # 本地测试
│   └── helm/                   # K8s 部署
├── grafana/
│   └── dashboard.json          # 监控面板
├── config.yaml.example         # 配置示例
├── Dockerfile
├── go.mod
├── go.sum
└── README.md
```

### 方案 2：扁平化结构
```
llmproxy/
├── main.go
├── config.go
├── proxy.go
├── lb.go
├── metrics.go
└── ...
```

**对比：**
- 方案 1 更适合中大型项目，职责清晰
- 方案 2 适合小型项目，快速开发

## 部署方案

### 1. Docker 部署
- 提供 Dockerfile
- 提供 docker-compose.yml（含 vLLM 示例）

### 2. Kubernetes 部署
- 提供 Helm Chart
- 支持 ConfigMap 注入配置
- 支持 Prometheus ServiceMonitor

### 3. 二进制部署
- 提供编译好的二进制文件
- 支持 systemd 服务

## 安全性考虑

### 1. API Key 鉴权（可选）
- 从 Header 提取 API Key
- 验证 API Key 有效性
- 提取 user_id 用于计费

### 2. 限流（可选）
- 基于 IP 的限流
- 基于 API Key 的限流

### 3. HTTPS 支持
- 支持 TLS 证书配置
- 支持 Let's Encrypt 自动证书

## 待确认问题

### 问题 1：是否需要 API Key 鉴权？
- 选项 A：不需要，由上游网关处理
- 选项 B：需要，LLMProxy 内置鉴权

### 问题 2：是否需要限流功能？
- 选项 A：不需要，由上游网关处理
- 选项 B：需要，防止后端过载

### 问题 3：用量计量失败时的处理策略？
- 选项 A：仅记录日志，不影响主流程
- 选项 B：重试 N 次后写入本地队列，定期重试
- 选项 C：写入本地文件，提供补偿机制

### 问题 4：是否需要支持多租户？
- 选项 A：不需要，单租户场景
- 选项 B：需要，支持租户隔离和配额管理

### 问题 5：监控和日志方案？
- 选项 A：仅 Prometheus 指标
- 选项 B：Prometheus + 结构化日志（JSON）
- 选项 C：Prometheus + 日志 + 分布式追踪（OpenTelemetry）

## 推荐方案总结

**技术栈：** Go 语言  
**架构：** 单体架构  
**项目结构：** 标准 Go 项目结构  
**核心功能：**
1. 零缓冲流式代理
2. 轮询负载均衡 + 健康检查
3. 异步 Webhook 用量上报
4. Prometheus 监控指标

**可选功能（根据需求决定）：**
- API Key 鉴权
- 限流
- 多租户支持

## 下一步行动

请确认：
1. 是否采用推荐的技术栈和架构？
2. 对于"待确认问题"的选择？
3. 是否需要调整或补充功能？

确认后，我将更新 `.canon/schema.json` 设计文档，并开始实施开发。
