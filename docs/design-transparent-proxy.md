# 透明代理优化方案

## 一、核心理念

**LLMProxy 应该是完全透明的 HTTP 代理，不关心业务参数。**

### 1.1 当前问题

**过度耦合业务概念：**
- 代码中硬编码了 `model` 字段
- 假设所有请求都有 `model` 参数
- 模型映射、权限控制都依赖 `model`

**限制了灵活性：**
- 用户不能用自定义参数
- Webhook 只能收到 `model` 字段
- 不适合非 LLM 场景

### 1.2 解决方案

**完全透明代理：**
- ✅ 不解析业务参数（model, service_type 等）
- ✅ 完整透传用户的所有参数
- ✅ Webhook 收到完整的请求参数
- ✅ 只做网关该做的事：鉴权、限流、路由、监控

---

## 二、架构调整

### 2.1 请求处理流程（简化）

```
用户请求
  ↓
鉴权中间件（验证 API Key）
  ↓
限流中间件（全局限流 + Key 级限流）
  ↓
代理处理器
  ├─ 读取请求体（完整保存，不解析业务字段）
  ├─ 选择后端（单后端直接用，多后端需要路由规则）
  └─ 透传请求
  ↓
返回响应
  ↓
异步处理
  ├─ 提取用量信息（从响应的 usage 字段）
  ├─ 扣减额度
  └─ 发送 Webhook（包含完整请求参数）
```

### 2.2 删除的概念

**不再关心：**
- ❌ model 参数
- ❌ 模型映射（model_mapping）
- ❌ 模型级权限（allowed_models）
- ❌ 模型级限流（per_model）

**保留的功能：**
- ✅ 鉴权（API Key 验证）
- ✅ 额度管理（Token 配额）
- ✅ 限流（全局 + Key 级）
- ✅ 路由（单后端/多后端）
- ✅ 监控（Prometheus 指标）
- ✅ Webhook（用量上报）

---

## 三、配置简化

### 3.1 单后端配置（最常见）

```yaml
listen: ":8000"

backends:
  - url: "http://backend:8000"

auth:
  enabled: true

api_keys:
  - key: "sk-test-001"
    user_id: "user_001"
    total_quota: 1000000
    quota_reset_period: "monthly"

rate_limit:
  enabled: true
  per_key:
    requests_per_second: 10
    max_concurrent: 5

usage_hook:
  enabled: true
  url: "http://billing:8080/api/usage"
```

**特点：**
- 不需要配置 allowed_models
- 不需要配置 model_mapping
- 完全透传

---

### 3.2 多后端配置（需要路由）

**方案 A：用户在请求头中指定后端**
```yaml
backends:
  - url: "http://backend-1:8000"
    id: "backend-1"
  - url: "http://backend-2:8000"
    id: "backend-2"

routing:
  # 从请求头读取后端 ID
  backend_header: "X-Backend-ID"
```

**用户请求：**
```bash
curl http://llmproxy:8000/v1/chat/completions \
  -H "X-Backend-ID: backend-1" \
  -d '{"messages": [...]}'
```

---

**方案 B：根据请求路径路由**
```yaml
backends:
  - url: "http://backend-1:8000"
    paths: ["/v1/chat/completions"]
  - url: "http://backend-2:8000"
    paths: ["/v1/embeddings"]
```

---

**方案 C：轮询/最少连接数（不需要用户指定）**
```yaml
backends:
  - url: "http://backend-1:8000"
  - url: "http://backend-2:8000"

routing:
  strategy: "round_robin"  # 或 "least_connections"
```

---

## 四、Webhook 数据结构

### 4.1 新的 UsageRecord

```go
type UsageRecord struct {
    // 请求标识
    RequestID        string    `json:"request_id"`
    Timestamp        time.Time `json:"timestamp"`
    
    // 用户信息
    UserID           string    `json:"user_id,omitempty"`
    APIKey           string    `json:"api_key,omitempty"`
    
    // 完整的请求参数（不解析，完整透传）
    RequestBody      map[string]interface{} `json:"request_body"`
    
    // 用量信息（从响应中提取）
    Usage            *UsageInfo `json:"usage,omitempty"`
    
    // 元数据
    Method           string    `json:"method"`
    Path             string    `json:"path"`
    BackendURL       string    `json:"backend_url"`
    StatusCode       int       `json:"status_code"`
    Latency          int64     `json:"latency_ms"`
}

type UsageInfo struct {
    PromptTokens     int `json:"prompt_tokens"`
    CompletionTokens int `json:"completion_tokens"`
    TotalTokens      int `json:"total_tokens"`
}
```

### 4.2 Webhook 示例数据

**场景 1：LLM 请求**
```json
{
  "request_id": "req-123",
  "timestamp": "2026-01-16T10:30:00Z",
  "user_id": "user_001",
  "api_key": "sk-test-001",
  
  "request_body": {
    "model": "qwen2.5:0.5b",
    "messages": [{"role": "user", "content": "你好"}],
    "temperature": 0.7
  },
  
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 20,
    "total_tokens": 30
  },
  
  "method": "POST",
  "path": "/v1/chat/completions",
  "backend_url": "http://ollama:11434",
  "status_code": 200,
  "latency_ms": 1500
}
```

**场景 2：自定义服务请求**
```json
{
  "request_id": "req-456",
  "timestamp": "2026-01-16T10:31:00Z",
  "user_id": "user_002",
  "api_key": "sk-test-002",
  
  "request_body": {
    "service_type": "translation",
    "source_lang": "en",
    "target_lang": "zh",
    "text": "Hello world",
    "priority": "high"
  },
  
  "usage": {
    "prompt_tokens": 5,
    "completion_tokens": 5,
    "total_tokens": 10
  },
  
  "method": "POST",
  "path": "/api/translate",
  "backend_url": "http://custom-service:8000",
  "status_code": 200,
  "latency_ms": 800
}
```

**Webhook 接收方的灵活性：**
```python
@app.post("/api/usage")
def handle_usage(data: dict):
    body = data["request_body"]
    usage = data["usage"]
    
    # 根据不同的请求参数做不同的处理
    if "model" in body:
        # LLM 场景
        cost = calculate_llm_cost(body["model"], usage)
    elif "service_type" in body:
        # 自定义服务场景
        cost = calculate_service_cost(body["service_type"], usage)
    else:
        # 默认计费
        cost = usage["total_tokens"] * 0.001
    
    save_billing(data["user_id"], cost, body)
```

---

## 五、权限控制的调整

### 5.1 删除模型级权限

**当前（删除）：**
```yaml
api_keys:
  - key: "sk-test-001"
    allowed_models: ["qwen-72b"]  # 删除这个
```

**原因：**
- LLMProxy 不应该理解业务参数
- 权限控制应该由后端服务或 Webhook 接收方处理

---

### 5.2 权限控制的新方式

**方案 A：后端服务自己控制**

LLMProxy 在请求头中传递用户信息：
```
X-User-ID: user_001
X-API-Key: sk-test-001
```

后端服务根据这些信息做权限检查：
```python
@app.post("/v1/chat/completions")
def chat(request: Request):
    user_id = request.headers.get("X-User-ID")
    model = request.json.get("model")
    
    # 后端自己检查权限
    if not check_permission(user_id, model):
        return {"error": "Permission denied"}, 403
    
    return process_request(request.json)
```

---

**方案 B：Webhook 接收方事后检查**

LLMProxy 先透传请求，Webhook 接收方事后检查：
```python
@app.post("/api/usage")
def handle_usage(data: dict):
    user_id = data["user_id"]
    body = data["request_body"]
    
    # 事后检查权限
    if not check_permission(user_id, body):
        # 记录违规行为
        log_violation(user_id, body)
        # 可以选择扣费或不扣费
    
    # 正常计费
    save_billing(user_id, calculate_cost(body, data["usage"]))
```

---

## 六、实现计划

### Phase 1: 删除 model 相关代码

**删除：**
1. ❌ `RequestBody` 结构体中的 `Model` 字段解析
2. ❌ 模型映射逻辑（`MapModel`）
3. ❌ 模型级权限检查（`CheckModelAllowed`）
4. ❌ 模型级限流配置（`per_model`）
5. ❌ `allowed_models` 配置项

**保留：**
1. ✅ API Key 验证
2. ✅ 额度管理（基于 Token 数量）
3. ✅ Key 级限流
4. ✅ 负载均衡

---

### Phase 2: Webhook 完整透传

**修改：**
1. `UsageRecord` 结构体，添加 `RequestBody` 字段
2. `collectUsage` 函数，保存完整请求体
3. 文档更新

---

### Phase 3: 多后端路由优化

**新增：**
1. 支持请求头路由（`X-Backend-ID`）
2. 支持路径路由
3. 文档更新

---

## 七、向后兼容

**破坏性变更：**
- ❌ `allowed_models` 配置项不再生效
- ❌ `model_mapping` 配置项不再生效
- ❌ `per_model` 限流配置不再生效

**迁移指南：**
1. 如果使用了 `allowed_models` → 改为后端服务自己检查
2. 如果使用了 `model_mapping` → 改为后端服务自己处理
3. 如果使用了 `per_model` 限流 → 改为 Key 级限流

---

## 八、方案确认

**核心改进：**
1. ✅ 删除 model 概念，完全透传
2. ✅ Webhook 收到完整请求参数
3. ✅ 简化配置，降低复杂度
4. ✅ 提升灵活性，适用更多场景

**你同意这个方案吗？**
