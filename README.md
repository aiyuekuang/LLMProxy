# LLMProxy

面向大模型服务的高性能网关，支持流式/非流式无缝代理 + 异步用量计量（HTTP Webhook）。

中文文档 | [English](README_EN.md)

[![Docker Build](https://github.com/aiyuekuang/LLMProxy/actions/workflows/release.yml/badge.svg)](https://github.com/aiyuekuang/LLMProxy/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/github/go-mod/go-version/aiyuekuang/LLMProxy)](go.mod)
[![GitHub Stars](https://img.shields.io/github/stars/aiyuekuang/LLMProxy?style=social)](https://github.com/aiyuekuang/LLMProxy/stargazers)
[![GitHub Forks](https://img.shields.io/github/forks/aiyuekuang/LLMProxy?style=social)](https://github.com/aiyuekuang/LLMProxy/network/members)

## 核心特性

- ✅ **LLM 协议感知代理** - 自动识别 `/v1/chat/completions` 请求中的 `stream=true/false`
- ✅ **零缓冲流式传输** - SSE 响应逐 token 转发，不增加首 token 延迟（TTFT）
- ✅ **多后端负载均衡** - 支持 vLLM、TGI、自研服务等 OpenAI 兼容后端
- ✅ **异步用量计量** - 请求结束后，后台异步上报 `prompt_tokens` + `completion_tokens`
- ✅ **零性能侵入** - 主请求路径不解析响应体、不连接数据库、不调用外部服务
- ✅ **极简业务对接** - 通过 HTTP Webhook 将用量数据推送给业务系统

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

```yaml
# 监听地址
listen: ":8000"

# 后端服务器列表
backends:
  - url: "http://vllm:8000"
    weight: 5
  - url: "http://tgi:8081"
    weight: 3

# 用量上报 Webhook 配置
usage_hook:
  enabled: true
  url: "https://your-billing.com/llm-usage"
  timeout: 1s
  retry: 2

# 健康检查配置
health_check:
  interval: 10s
  path: /health
```

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

LLMProxy 会向配置的 Webhook URL 发送 POST 请求：

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

### 业务系统接收示例（Python Flask）

```python
@app.route('/llm-usage', methods=['POST'])
def record_usage():
    data = request.json
    # 写入数据库
    db.execute(
        "INSERT INTO billing_events (customer, input_tk, output_tk, model) VALUES (?, ?, ?, ?)",
        data['user_id'], data['prompt_tokens'], data['completion_tokens'], data['model']
    )
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
