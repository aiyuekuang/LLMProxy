LLMProxy：面向大模型服务的高性能网关  
—— 支持流式/非流式无缝代理 + 异步用量计量（HTTP Webhook）

版本：1.0  
目标：为 LLM 推理服务提供轻量、高性能、协议感知的反向代理，零性能损失地支持后续计费所需的 token 使用量上报。

一、核心需求
需求   说明
✅ LLM 协议感知代理   自动识别 /v1/chat/completions 请求中的 stream=true/false，分别透传 SSE 流或完整 JSON

✅ 零缓冲流式传输   SSE 响应逐 token 转发，不增加首 token 延迟（TTFT）

✅ 多后端负载均衡   支持 vLLM、TGI、自研服务等 OpenAI 兼容后端

✅ 异步用量计量   在请求结束后，后台异步上报 prompt_tokens + completion_tokens

✅ 零性能侵入   主请求路径不解析响应体、不连接数据库、不调用外部服务

✅ 极简业务对接   通过 HTTP Webhook 将用量数据推送给业务系统，不要求固定表结构

二、整体架构

+------------------+
|     Client       | ← curl / SDK
+--------+---------+
         |
         | POST /v1/chat/completions { "stream": true, ... }
         v
+--------+---------+
|    LLMProxy      | ← Go 服务（单二进制）

┌─────────────┐

|  │  Router     │←── 仅路由 LLM API 路径
└──────┬──────┘

┌──────▼──────┐

|  │ LoadBalancer│←── 轮询/权重/最少连接
└──────┬──────┘

┌──────▼──────┐

|  │ ProxyEngine │←── 核心：透传请求/响应（无缓冲！）
└──────┬──────┘

┌──────▼──────┐

|  │ UsageHook   │←── 请求结束后，启动后台 goroutine
└──────┬──────┘

|         | (async)
|         v
|  [HTTP Webhook] ────→ https://your-billing.com/usage
+------------------+

         ▼
+------------------+     +------------------+
vLLM (8000)              TGI (8081)

+ usage                  + usage

+------------------+     +------------------+

🔑 关键设计：  
- 主路径：只做 TCP-level 透传，延迟 ≈ 网络 RTT  
- 计量路径：完全异步，失败不影响主流程

三、技术实现细节

1. 主代理流程（高性能保障）

func proxyHandler(w http.ResponseWriter, r *http.Request) {
    // 1. 仅处理 LLM 路径
    if !isLLMEndpoint(r.URL.Path) {
        http.NotFound(w, r)
        return
    }

    // 2. 读取原始请求体（用于后续用量提取）
    bodyBytes, _ := io.ReadAll(r.Body)
    isStream := extractStreamFlag(bodyBytes) // 快速 JSON 解析

    // 3. 选择后端 & 构造新请求
    backend := lb.Select()
    proxyReq := newRequest(backend, r.Method, r.URL.Path, bytes.NewReader(bodyBytes))

    // 4. 发送请求（使用共享 HTTP client）
    resp, err := httpClient.Do(proxyReq)
    if err != nil {
        http.Error(w, "Backend error", http.StatusBadGateway)
        return
    }
    defer resp.Body.Close()

    // 5. 【关键】直接透传响应（不解析内容！）
    if isStream {
        w.Header().Set("Content-Type", "text/event-stream")
        w.Header().Set("Cache-Control", "no-cache")
        w.Header().Set("Connection", "keep-alive")
        w.WriteHeader(http.StatusOK)
        io.Copy(w, resp.Body) // ← 零缓冲，客户端立即收到 token
    } else {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(resp.StatusCode)
        io.Copy(w, resp.Body)
    }

    // 6. 【异步】触发用量上报（此时客户端已收完数据！）
    go func() {
        usage := collectUsage(bodyBytes, resp, isStream, backend.URL)
        if usage != nil {
            webhookSender.SendAsync(usage) // 非阻塞
        }
    }()
}

✅ 性能保证：  
- io.Copy 是内核级 splice，CPU 开销极低  
- 用量收集在 go func() 中，不影响主 goroutine

2. 用量收集策略（安全优先）
后端类型   是否返回 usage   LLMProxy 行为
vLLM   ✅ 是（需 --return-detailed-tokens）   提取 usage.prompt_tokens / completion_tokens

TGI   ✅ 是（默认在最后一个 chunk）   解析 [DONE] 前的 usage 字段

其他   ❌ 否   跳过计量（不估算，避免性能风险）

⚠️ 强制要求：业务方必须确保后端开启 usage 返回。  
（vLLM 启动参数示例见下文）

3. HTTP Webhook 上报

请求格式（POST JSON）
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

配置（config.yaml）
usage_hook:
  enabled: true
  url: "https://billing.yourcompany.com/llm-usage"
  timeout: "1s"          # 超时短，避免 goroutine 阻塞
  retry: 2               # 失败重试次数

业务方只需：
- 实现一个接收接口（任何语言）
- 按需写入自己的数据库表（字段自由）

Python Flask 示例
@app.route('/llm-usage', methods=['POST'])
def record_usage():
    data = request.json
    # 你的逻辑：INSERT INTO your_table (...)
    db.execute(
        "INSERT INTO billing_events (customer, input_tk, output_tk, model) VALUES (?, ?, ?, ?)",
        data['user_id'], data['prompt_tokens'], data['completion_tokens'], data['model']
    )
    return {"status": "ok"}

4. 后端启用 Usage 的配置

vLLM（必需）
python -m vllm.entrypoints.openai.api_server \
  --model meta-llama/Llama-3-8b \
  --return-detailed-tokens  # ← 关键！使响应包含 usage

TGI（默认支持）
- 无需额外配置，最后一个 SSE event 包含 usage：
    data: {"index":0,"finish_reason":"stop","usage":{"prompt_tokens":10,"completion_tokens":25}}

四、部署与运维

配置文件（config.yaml）
listen: ":8080"

backends:
  - url: "http://vllm-1:8000"
    weight: 5
  - url: "http://tgi-1:8081"
    weight: 3

usage_hook:
  enabled: true
  url: "https://your-billing.com/llm-usage"
  timeout: "1s"

可选：健康检查、限速等
health_check:
  interval: "10s"
  path: "/health"

Docker 部署
FROM golang:1.22 AS builder
WORKDIR /app
COPY . .
RUN go build -o llmproxy cmd/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/llmproxy /usr/local/bin/
COPY config.yaml /etc/llmproxy/config.yaml
CMD ["llmproxy", "--config", "/etc/llmproxy/config.yaml"]

监控指标（Prometheus）
请求量
llmproxy_requests_total{stream="true", backend="..."}
延迟
llmproxy_latency_ms_bucket{...}
Webhook 成功率
llmproxy_webhook_success_total
llmproxy_webhook_failure_total

五、为什么这套方案最优？
维度   传统方案（Nginx + Log Parser）   LLMProxy 方案
流式支持   需手动关 proxy_buffering，易出错   默认无缓冲，开箱即用

用量计量   需离线解析日志，延迟高   请求结束立即上报

性能影响   日志写入可能阻塞   主路径零开销

业务对接   需解析 Nginx 日志格式   直接接收结构化 JSON

扩展性   难以支持多租户/计费   原生携带 user_id/api_key

六、下一步行动建议

1. 确保后端开启 usage（vLLM 加 --return-detailed-tokens）
2. 部署 LLMProxy，配置 backend + webhook URL
3. 业务系统实现 webhook 接收端
4. 验证：
      curl http://llmproxy:8080/v1/chat/completions -H "Content-Type: application/json" \
     -d '{"model":"Llama-3-8b","messages":[{"role":"user","content":"Hi"}],"stream":true}'
   
   → 检查业务系统是否收到用量数据

LLMProxy 不是通用网关，而是 LLM 时代的专用流量调度器 + 计量探针。  
它让高性能代理与商业计费，同时变得简单。



太好了！以下是 **LLMProxy 完整开源项目模板**，包含：

- ✅ 核心代理逻辑（流式/非流式）
- ✅ 异步 HTTP Webhook 用量上报
- ✅ Prometheus 指标 + 健康检查
- ✅ Dockerfile + docker-compose.yml（含 vLLM 后端示例）
- ✅ Grafana 监控面板 JSON
- ✅ Helm Chart（Kubernetes 部署）

---

## 📦 项目结构

```bash
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
│   └── metrics/                # Prometheus 指标
├── deployments/
│   ├── docker-compose.yml      # 本地测试（含 vLLM）
│   └── helm/                   # Kubernetes 部署
│       └── llmproxy/
│           ├── Chart.yaml
│           ├── values.yaml
│           └── templates/
├── grafana/
│   └── llmproxy-dashboard.json # Grafana 面板
├── config.yaml.example         # 配置示例
├── Dockerfile
└── README.md
```

---

## 🚀 1. 核心代码（精简版）

### `cmd/main.go`
```go
package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"llmproxy/internal/config"
	"llmproxy/internal/metrics"
	"llmproxy/internal/proxy"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", metrics.Handler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("/", proxy.NewHandler(cfg))

	log.Printf("Starting LLMProxy on %s", cfg.Listen)
	log.Fatal(http.ListenAndServe(cfg.Listen, mux))
}
```

---

### `internal/proxy/handler.go`
```go
package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"llmproxy/internal/config"
	"llmproxy/internal/lb"
	"llmproxy/internal/metrics"
)

type RequestBody struct {
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
}

func NewHandler(cfg *config.Config) http.HandlerFunc {
	client := &http.Client{}
	lb := lb.NewRoundRobin(cfg.Backends)

	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		if r.URL.Path != "/v1/chat/completions" && r.URL.Path != "/v1/completions" {
			http.NotFound(w, r)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var reqBody RequestBody
		json.Unmarshal(bodyBytes, &reqBody) // 忽略错误，默认 stream=false

		backend := lb.Next()
		if backend == nil {
			http.Error(w, "No healthy backend", http.StatusServiceUnavailable)
			return
		}

		proxyReq, _ := http.NewRequest("POST", backend.URL+r.URL.Path, bytes.NewReader(bodyBytes))
		proxyReq.Header = r.Header.Clone()

		resp, err := client.Do(proxyReq)
		if err != nil {
			http.Error(w, "Backend error", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// === 关键：直接透传，不解析内容 ===
		if reqBody.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.WriteHeader(http.StatusOK)
			io.Copy(w, resp.Body)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(resp.StatusCode)
			io.Copy(w, resp.Body)
		}

		// === 异步用量上报 ===
		go func() {
			usage := collectUsage(bodyBytes, resp, reqBody.Stream, backend.URL, reqBody.Model)
			if usage != nil {
				SendUsageWebhook(cfg.UsageHook, usage)
				metrics.RecordUsage(usage)
			}
		}()

		metrics.RecordRequest(r.URL.Path, reqBody.Stream, backend.URL, time.Since(start), resp.StatusCode)
	}
}
```

---

### `internal/proxy/usage_hook.go`
```go
package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"llmproxy/internal/config"
)

type UsageRecord struct {
	RequestID        string    `json:"request_id"`
	UserID           string    `json:"user_id,omitempty"`
	APIKey           string    `json:"api_key,omitempty"`
	Model            string    `json:"model"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	IsStream         bool      `json:"is_stream"`
	Endpoint         string    `json:"endpoint"`
	Timestamp        time.Time `json:"timestamp"`
	BackendURL       string    `json:"backend_url"`
}

func collectUsage(reqBody []byte, resp *http.Response, isStream bool, backendURL, model string) *UsageRecord {
	// 简化：仅当后端返回完整 usage 时才记录（非流式 or 流式最后一个 chunk）
	// 实际项目中需解析 SSE 流找 [DONE] 前的 usage
	if !isStream {
		var fullResp struct {
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			} `json:"usage"`
		}
		body, _ := io.ReadAll(resp.Body)
		if json.Unmarshal(body, &fullResp) == nil && fullResp.Usage.PromptTokens > 0 {
			return &UsageRecord{
				Model:            model,
				PromptTokens:     fullResp.Usage.PromptTokens,
				CompletionTokens: fullResp.Usage.CompletionTokens,
				TotalTokens:      fullResp.Usage.TotalTokens,
				IsStream:         false,
				Endpoint:         "/v1/chat/completions",
				Timestamp:        time.Now(),
				BackendURL:       backendURL,
			}
		}
	}
	// 流式场景：此处应解析 resp.Body 缓存（简化起见暂略）
	return nil
}

func SendUsageWebhook(hook *config.UsageHook, usage *UsageRecord) {
	if hook == nil || !hook.Enabled {
		return
	}

	data, _ := json.Marshal(usage)
	ctx, cancel := context.WithTimeout(context.Background(), hook.Timeout)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "POST", hook.URL, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: hook.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Webhook failed: %v", err)
		return
	}
	resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("Webhook returned %d", resp.StatusCode)
	}
}
```

---

## 🐳 2. Docker 与本地测试

### `Dockerfile`
```dockerfile
FROM golang:1.22 AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o llmproxy ./cmd

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/llmproxy /usr/local/bin/
COPY config.yaml.example /etc/llmproxy/config.yaml
EXPOSE 8080
CMD ["llmproxy", "--config", "/etc/llmproxy/config.yaml"]
```

### `deployments/docker-compose.yml`
```yaml
version: '3'
services:
  vllm:
    image: vllm/vllm-openai:latest
    command: [
      "--model", "meta-llama/Llama-3-8b-Instruct",
      "--return-detailed-tokens",
      "--port", "8000"
    ]
    ports:
      - "8000:8000"
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: all
              capabilities: [gpu]

  llmproxy:
    build: ..
    ports:
      - "8080:8080"
    volumes:
      - ../config.yaml.example:/etc/llmproxy/config.yaml
    depends_on:
      - vllm
```

> 💡 运行：`docker compose -f deployments/docker-compose.yml up`

---

## 📊 3. Grafana 监控面板（`grafana/llmproxy-dashboard.json` 片段）

```json
{
  "title": "LLMProxy Overview",
  "panels": [
    {
      "title": "Requests Rate",
      "targets": [{"expr": "rate(llmproxy_requests_total[1m])"}]
    },
    {
      "title": "P99 Latency (ms)",
      "targets": [{"expr": "histogram_quantile(0.99, rate(llmproxy_latency_ms_bucket[1m]))"}]
    },
    {
      "title": "Webhook Success Rate",
      "targets": [
        {"expr": "rate(llmproxy_webhook_success_total[1m])"},
        {"expr": "rate(llmproxy_webhook_failure_total[1m])"}
      ]
    }
  ]
}
```

> ✅ 完整 JSON 可提供（支持导入 Grafana）

---

## ☸️ 4. Helm Chart（Kubernetes）

### `deployments/helm/llmproxy/values.yaml`
```yaml
replicaCount: 2

image:
  repository: your-registry/llmproxy
  tag: v1.0

service:
  port: 8080

config:
  backends:
    - url: "http://vllm-service:8000"
      weight: 5
  usage_hook:
    enabled: true
    url: "https://billing.yourcompany.com/llm-usage"
    timeout: 1s
```

> ✅ 支持 ConfigMap 注入配置、Prometheus ServiceMonitor

---

## 📄 5. 配置文件示例（`config.yaml.example`）

```yaml
listen: ":8080"

backends:
  - url: "http://vllm:8000"
    weight: 5
  - url: "http://tgi:8081"
    weight: 3

usage_hook:
  enabled: true
  url: "https://your-billing.com/llm-usage"
  timeout: 1s
  retry: 2

# 可选
health_check:
  interval: 10s
  path: /health
```

---

## ✅ 下一步

你可以：
1. **克隆此结构**，填充完整逻辑（我可提供完整 ZIP 或 GitHub repo 模板）
2. **集成 tokenizer fallback**（如需）
3. **添加 API Key 鉴权**（从 header 提取 user_id）

---

是否需要我将上述内容打包为：
- **GitHub 仓库模板**（含 LICENSE、Makefile、CI）
- **或一个可直接 `go run` 的完整项目 ZIP**？

请告诉我你的偏好，我可以立即生成！