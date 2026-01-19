# LLMProxy

High-performance gateway for LLM services, supporting seamless streaming/non-streaming proxy with asynchronous usage metering via HTTP Webhook.

[![Docker Build](https://github.com/aiyuekuang/LLMProxy/actions/workflows/release.yml/badge.svg)](https://github.com/aiyuekuang/LLMProxy/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/github/go-mod/go-version/aiyuekuang/LLMProxy)](go.mod)
[![GitHub Stars](https://img.shields.io/github/stars/aiyuekuang/LLMProxy?style=social)](https://github.com/aiyuekuang/LLMProxy/stargazers)
[![GitHub Forks](https://img.shields.io/github/forks/aiyuekuang/LLMProxy?style=social)](https://github.com/aiyuekuang/LLMProxy/network/members)

[中文文档](README.md) | English

## Core Features

### 🚀 High Performance
- ✅ **Zero-Buffer Streaming** - SSE responses forwarded token-by-token without increasing TTFT
- ✅ **Zero Performance Overhead** - Main request path doesn't parse response body
- ✅ **Connection Reuse** - HTTP client connection pooling

### 🎯 Transparent Proxy
- ✅ **Full Passthrough** - Doesn't parse business parameters, fully transparent
- ✅ **Auto Retry** - Exponential backoff strategy
- ✅ **Multiple Load Balancing** - Round-robin, least connections, latency-based

### 🔐 Orchestrable Auth Pipeline (v0.3.0+)
- ✅ **Multiple Data Sources** - Config file / Redis / Database (MySQL/PostgreSQL/SQLite) / Webhook
- ✅ **Lua Script Decision** - Custom auth logic with Lua scripts
- ✅ **Orchestrable Order** - Freely adjust Provider execution order
- ✅ **Two Pipeline Modes** - `first_match` (first success passes) or `all` (all must pass)

### 📊 Usage Reporting (v0.3.2+)
- ✅ **Multiple Reporters** - Configure multiple Webhook and Database reporters simultaneously
- ✅ **Direct Database Write** - Support MySQL/PostgreSQL/SQLite direct write
- ✅ **Independent Switches** - Each reporter can be enabled/disabled independently

## Real-World Scenarios

### Scenario 1: Self-Hosted OpenCode AI Coding Assistant (Private Code Assistant)

A tech team deploys Qwen2.5-Coder-32B model using vLLM to provide developers with a private AI coding assistant.

**Architecture:**
```
Developer IDE (OpenCode) → LLMProxy → vLLM (Qwen2.5-Coder-32B)
```

**LLMProxy Configuration:**
```yaml
backends:
  - url: "http://vllm-coder:8000"
    weight: 10

auth:
  enabled: true
  storage: "file"
  header_names: ["Authorization", "X-API-Key"]

api_keys:
  - key: "sk-llmproxy-dev-001"
    name: "Dev Team"
    total_quota: 1000000
    allowed_ips: ["10.0.0.0/8"]

rate_limit:
  per_key:
    requests_per_minute: 60
    max_concurrent: 3
```

**vLLM Startup Command:**
```bash
python -m vllm.entrypoints.openai.api_server \
  --model Qwen/Qwen2.5-Coder-32B-Instruct \
  --enable-auto-tool-choice \
  --tool-call-parser hermes \
  --return-detailed-tokens \
  --port 8000
```

**OpenCode Configuration (opencode.json):**
```json
{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "llmproxy": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "LLMProxy",
      "options": {
        "baseURL": "http://your-llmproxy-host:8000/v1"
      },
      "models": {
        "qwen-coder": {
          "name": "Qwen2.5-Coder-32B-Instruct",
          "limit": {
            "context": 131072,
            "output": 8192
          }
        }
      }
    }
  },
  "model": "llmproxy/qwen-coder"
}
```

**Results:**
- Code data stays fully private within the intranet
- Supports Tool Calling for file read/write and command execution
- Unified API Key management and usage monitoring
- Coding assistant response latency < 500ms

For detailed configuration, see: [OpenCode Integration Guide](docs/opencode-integration.md)

---

## Quick Start

### Option 1: Use Official Image (Recommended)

```bash
# 1. Create config file
curl -o config.yaml https://raw.githubusercontent.com/aiyuekuang/LLMProxy/main/config.yaml.example

# 2. Edit config file, modify backend addresses
vim config.yaml

# 3. Run container
docker run -d \
  --name llmproxy \
  -p 8000:8000 \
  -v $(pwd)/config.yaml:/home/llmproxy/config.yaml \
  ghcr.io/aiyuekuang/llmproxy:latest
```

**Supported Architectures:** `linux/amd64`, `linux/arm64`

### Option 2: Build Locally

```bash
# Download dependencies
go mod download

# Copy config file
cp config.yaml.example config.yaml

# Edit config file
vim config.yaml

# Run
go run cmd/main.go --config config.yaml
```

### Option 3: Docker Compose (with vLLM)

```bash
cd deployments
docker compose up -d
```

Access:
- LLMProxy: http://localhost:8000
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin/admin)

## Configuration

```yaml
# Listen address
listen: ":8000"

# Backend server list
backends:
  - url: "http://vllm:8000"
    weight: 5
  - url: "http://tgi:8081"
    weight: 3

# Usage reporting (supports multiple reporters)
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

# Health check configuration
health_check:
  interval: 10s
  path: /health
```

## Backend Requirements

### vLLM

**Must enable `--return-detailed-tokens` parameter:**

```bash
python -m vllm.entrypoints.openai.api_server \
  --model meta-llama/Llama-3-8b-Instruct \
  --return-detailed-tokens \
  --port 8000
```

### TGI

Supported by default, no additional configuration needed.

## Webhook Data Format

LLMProxy sends POST requests to the configured Webhook URL:

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

### Business System Receiver Example (Python Flask)

```python
@app.route('/llm-usage', methods=['POST'])
def record_usage():
    data = request.json
    # Write to database
    db.execute(
        "INSERT INTO billing_events (customer, input_tk, output_tk, model) VALUES (?, ?, ?, ?)",
        data['user_id'], data['prompt_tokens'], data['completion_tokens'], data['model']
    )
    return {"status": "ok"}
```

## Monitoring Metrics

LLMProxy exposes Prometheus metrics at `/metrics`:

| Metric Name | Type | Description |
|------------|------|-------------|
| `llmproxy_requests_total` | Counter | Total requests (labels: path, stream, backend, status) |
| `llmproxy_latency_ms` | Histogram | Request latency (milliseconds) |
| `llmproxy_webhook_success_total` | Counter | Successful webhook deliveries |
| `llmproxy_webhook_failure_total` | Counter | Failed webhook deliveries |
| `llmproxy_usage_tokens_total` | Counter | Token usage (labels: type=prompt/completion) |

## API Usage Examples

### Non-Streaming Request

```bash
curl http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "meta-llama/Llama-3-8b-Instruct",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": false
  }'
```

### Streaming Request

```bash
curl http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "meta-llama/Llama-3-8b-Instruct",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": true
  }'
```

## Architecture

```
+------------------+
|     Client       | ← curl / SDK
+--------+---------+
         |
         | POST /v1/chat/completions { "stream": true, ... }
         v
+--------+---------+
|    LLMProxy      | ← Go service (single binary)
|  ┌─────────────┐ |
|  │  Router     │ |←── Routes LLM API paths only
|  └──────┬──────┘ |
|  ┌──────▼──────┐ |
|  │LoadBalancer │ |←── Round-robin/weighted/least-conn
|  └──────┬──────┘ |
|  ┌──────▼──────┐ |
|  │ProxyEngine  │ |←── Core: Pass-through request/response (zero-buffer!)
|  └──────┬──────┘ |
|  ┌──────▼──────┐ |
|  │ UsageHook   │ |←── After request, spawn background goroutine
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

## Performance Characteristics

- **Zero-Buffer Streaming**: Uses `io.Copy` for kernel-level splice, minimal CPU overhead
- **Async Usage Reporting**: Executed in goroutines, doesn't block main request
- **Connection Reuse**: HTTP client reuses connections, reduces handshake overhead
- **Health Checks**: Automatically removes unhealthy nodes, improves availability

## Project Structure

```
llmproxy/
├── cmd/
│   └── main.go                 # Entry point
├── internal/
│   ├── config/                 # Config parsing
│   │   └── config.go
│   ├── proxy/                  # Core proxy engine
│   │   ├── handler.go          # Request handling
│   │   └── usage_hook.go       # Webhook reporting
│   ├── lb/                     # Load balancer
│   │   └── roundrobin.go
│   └── metrics/                # Prometheus metrics
│       └── metrics.go
├── deployments/
│   ├── docker-compose.yml      # Local testing
│   ├── config.yaml             # Docker config
│   └── prometheus.yml          # Prometheus config
├── grafana/
│   └── dashboard.json          # Grafana dashboard
├── config.yaml.example         # Config example
├── Dockerfile
├── go.mod
└── README.md
```

## FAQ

### 1. Why is there no usage information in the response?

Ensure your backend has usage reporting enabled:
- vLLM: Add `--return-detailed-tokens` parameter
- TGI: Supported by default

### 2. What happens if webhook delivery fails?

LLMProxy will automatically retry (based on configured `retry` count). Failures are logged only and don't affect the main request.

### 3. How to view monitoring metrics?

Visit `http://localhost:8000/metrics` to see Prometheus metrics.

### 4. What load balancing strategies are supported?

Currently supports Weighted Round Robin. Additional strategies (least connections, etc.) can be added.

## License

This project is licensed under the [MIT License](LICENSE).

This means you can:
- ✅ Freely use, modify, and distribute this software
- ✅ Use it in commercial projects
- ✅ Create derivative works

The only requirement: Retain the original copyright notice and license statement.

## Contributing

We welcome all forms of contributions! See [CONTRIBUTORS.md](CONTRIBUTORS_EN.md) for the list of contributors.

### How to Contribute

1. Fork this repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

For detailed contribution guidelines, see [CONTRIBUTORS.md](CONTRIBUTORS_EN.md).

## Support the Project

If LLMProxy helps you, please consider:

- ⭐ Star the project
- 🐛 Report bugs or suggest improvements
- 📝 Improve documentation or add examples
- 💬 Share your experience in the community
- 🔗 Add "Powered by LLMProxy" badge to your project:

```markdown
[![Powered by LLMProxy](https://img.shields.io/badge/Powered%20by-LLMProxy-blue)](https://github.com/aiyuekuang/LLMProxy)
```

## Contact

- 📧 Issues: [GitHub Issues](https://github.com/aiyuekuang/LLMProxy/issues)
- 💬 Discussions: [GitHub Discussions](https://github.com/aiyuekuang/LLMProxy/discussions)
