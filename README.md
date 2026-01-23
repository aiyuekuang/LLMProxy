# LLMProxy

**High-performance reverse proxy for LLM inference services** — Like nginx for web servers, LLMProxy for LLM inference engines.

**Single Binary** | **Zero Buffer** | **Millisecond TTFT** | **Ready to Use**

[![Docker Build](https://github.com/aiyuekuang/LLMProxy/actions/workflows/release.yml/badge.svg)](https://github.com/aiyuekuang/LLMProxy/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/github/go-mod/go-version/aiyuekuang/LLMProxy)](go.mod)

English | [中文文档](README_CN.md)

---

## Why LLMProxy?

| Comparison | Direct Connection | API Gateway (Kong/APISIX) | LLMProxy |
|------------|-------------------|---------------------------|----------|
| SSE Streaming Latency | ✅ Optimal | ❌ Buffer causes delay | ✅ Zero-buffer forwarding |
| Token Usage Metering | ❌ Build yourself | ❌ Plugin required | ✅ Native support |
| Deployment Complexity | Low | High (requires database) | Low (single binary) |
| LLM Optimization | None | General gateway | ✅ Built for LLM |
| Multi-backend Load Balancing | ❌ Not supported | ✅ Supported | ✅ Supported |
| Lua Script Extension | ❌ Not supported | ✅ Supported | ✅ Supported |

---

## Quick Start

**Start in 30 seconds:**

```bash
# Download config file
curl -o config.yaml https://raw.githubusercontent.com/aiyuekuang/LLMProxy/main/config.yaml.example

# Edit backend address
vim config.yaml

# Start
docker run -d -p 8000:8000 -v $(pwd)/config.yaml:/home/llmproxy/config.yaml ghcr.io/aiyuekuang/llmproxy:latest
```

Access `http://localhost:8000/v1/chat/completions` to use.

<details>
<summary><b>🔧 More Installation Options</b></summary>

**Build Locally:**
```bash
go mod download && cp config.yaml.example config.yaml
go run cmd/main.go --config config.yaml
```

**Docker Compose (with monitoring):**
```bash
cd deployments && docker compose up -d
```
Access: LLMProxy `:8000` | Prometheus `:9090` | Grafana `:3000` (admin/admin)

</details>

**Supported Architectures**: `linux/amd64`, `linux/arm64`

> 📖 **[Complete Configuration Reference](docs/configuration.md)** - All config options, Admin API, Auth Pipeline, Usage Reporting

---

## Core Features

| Feature | Description |
|---------|-------------|
| **Zero-Buffer Streaming** | SSE responses forwarded token-by-token, no TTFT increase |
| **Token Usage Statistics** | Auto-count `prompt_tokens` + `completion_tokens`, supports Webhook/Redis/Database |
| **API Key Auth** | Key validation, quota control, IP whitelist, expiration, Lua custom logic |
| **Load Balancing** | Round-robin, weighted, least connections, latency-based strategies |
| **Rate Limiting** | Global/Key-level rate limiting, concurrency control, token bucket algorithm |
| **Single Binary Deployment** | No Redis/MySQL dependencies, just YAML config |

### Data Integration Options

| Option | Use Case | Description |
|--------|----------|-------------|
| **Webhook** | Existing billing/management system | Async POST to your endpoint with full request and usage data |
| **Redis** | High concurrency, distributed deployment | Rate limiting counters, Key quota storage, cluster mode support |
| **Config File** | Small scale, quick deployment | YAML manages API Keys directly, no external dependencies |
| **Prometheus** | Monitoring & alerting | Exposes `/metrics` endpoint, integrates with Grafana |

---

## Performance

| Metric | Value |
|--------|-------|
| First Token Latency Overhead | < 1ms |
| Memory Usage | < 50MB |
| Concurrent Connections | 10,000+ |

**Design Principles:**
- **Zero Buffer** - Uses `io.Copy` for kernel-level splice, SSE responses forwarded token-by-token
- **Zero Intrusion** - Main request path doesn't parse JSON response body, usage stats reported async
- **Full Passthrough** - Doesn't care about business params (like `model`), all request params passed through

---

## Typical Use Case: Self-Hosted AI Coding Assistant

Provide a private API gateway for AI coding tools like [OpenCode](https://github.com/anomalyco/opencode), Cursor, Aider:

```
Developer IDE (OpenCode / Cursor) → LLMProxy → vLLM (Qwen2.5-Coder-32B)
```

**LLMProxy Configuration:**
```yaml
backends:
  - url: "http://vllm-coder:8000"
    weight: 10

admin:
  enabled: true
  token: "your-admin-token"

auth:
  enabled: true
  header_names: ["Authorization", "X-API-Key"]
  skip_paths: ["/health", "/metrics"]
  pipeline:
    - name: builtin_auth
      type: builtin
      enabled: true

rate_limit:
  enabled: true
  per_key:
    requests_per_minute: 60
    max_concurrent: 3
```

**Results:** Fully private code data | Tool Calling support | Unified API Key management | Response latency < 500ms

For detailed configuration, see: [OpenCode Integration Guide](docs/opencode-integration.md) | [More Scenarios](docs/real-world-scenarios.md)

---

## Configuration

```yaml
server:
  listen: ":8000"

backends:
  - url: "http://vllm:8000"
    weight: 5
  - url: "http://tgi:8081"
    weight: 3

# Admin API (for API Key management)
admin:
  enabled: true
  token: "your-secure-admin-token"
  db_path: "./data/keys.db"

# Usage reporting
usage:
  enabled: true
  reporters:
    - name: local
      type: builtin
      enabled: true
    - name: billing
      type: webhook
      enabled: true
      webhook:
        url: "https://your-billing.com/llm-usage"
        timeout: 3s

health_check:
  enabled: true
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

## Admin API

LLMProxy provides a built-in Admin API for managing API Keys:

| Endpoint | Description |
|----------|-------------|
| `POST /admin/keys/create` | Create API Key |
| `POST /admin/keys/update` | Update API Key |
| `POST /admin/keys/delete` | Delete API Key |
| `POST /admin/keys/get` | Get API Key |
| `POST /admin/keys/list` | List API Keys |
| `POST /admin/keys/sync` | Batch sync API Keys |

Requires `X-Admin-Token` header for authentication. Enable in config:

```yaml
admin:
  enabled: true
  token: "your-secure-admin-token"
  db_path: "./data/keys.db"  # SQLite database path
```

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

## Project Structure

```
llmproxy/
├── cmd/
│   └── main.go                 # Entry point
├── internal/
│   ├── admin/                  # Admin API module
│   │   ├── keystore.go         # API Key storage (SQLite)
│   │   ├── server.go           # Admin API server
│   │   └── usage.go            # Usage storage
│   ├── auth/                   # Authentication module
│   │   ├── middleware.go       # Auth middleware (legacy)
│   │   └── pipeline/           # Auth pipeline (new)
│   │       ├── provider_*.go   # Various providers
│   │       ├── executor.go     # Pipeline executor
│   │       └── middleware.go   # Pipeline middleware
│   ├── config/                 # Config parsing
│   │   └── config.go
│   ├── lb/                     # Load balancer
│   │   ├── round_robin.go
│   │   ├── least_connections.go
│   │   └── latency_based.go
│   ├── proxy/                  # Core proxy engine
│   │   ├── handler.go          # Request handling
│   │   └── usage_reporter.go   # Usage reporting
│   ├── ratelimit/              # Rate limiting
│   │   ├── memory.go           # In-memory limiter
│   │   └── redis_limiter.go    # Redis limiter
│   ├── routing/                # Intelligent routing
│   │   └── router.go
│   ├── storage/                # Storage abstraction
│   │   └── manager.go
│   ├── types/                  # Common types
│   │   └── status.go
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

Supports round_robin, least_connections, and latency_based strategies.

## 📚 Documentation

| Document | Description |
|----------|-------------|
| **[📖 Configuration Reference](docs/configuration.md)** | **Complete config options, module details, Lua extensions** |
| [Auth Pipeline Guide](docs/auth-pipeline.md) | Multi-source auth pipeline, Lua script examples |
| [Development Guide](docs/development-guide.md) | Architecture, core modules, dev guide, API reference |
| [OpenCode Integration](docs/opencode-integration.md) | Integration with OpenCode and other AI coding assistants |
| [Docker Publish Guide](docs/docker-publish-guide.md) | Docker image build and publish |
| [Changelog](CHANGELOG.md) | Version history |

## License

This project is licensed under the [MIT License](LICENSE).

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
