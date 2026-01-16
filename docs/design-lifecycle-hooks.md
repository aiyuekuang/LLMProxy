# LLMProxy 生命周期与可编程扩展点分析

## 文档概述

本文档分析 LLMProxy 的请求生命周期，识别所有可编程扩展点，并提出通过参数透传增强灵活性的方案。

## 当前请求生命周期

```
客户端请求
    ↓
[1] 路由判断 (isLLMEndpoint)
    ↓
[2] 鉴权中间件 (auth.Middleware)
    ├─ 提取 API Key
    ├─ 验证 Key 有效性
    ├─ 检查状态和过期时间
    ├─ 检查 IP 白名单/黑名单
    └─ 检查额度
    ↓
[3] 限流中间件 (ratelimit.Middleware)
    ├─ 全局限流
    ├─ Key 级限流
    └─ 并发数限流
    ↓
[4] 代理处理器 (proxy.Handler)
    ├─ 读取请求体
    ├─ 解析 stream 参数
    └─ 选择后端
    ↓
[5] 智能路由 (routing.Router)
    ├─ 查找 fallback 规则
    ├─ 负载均衡选择
    ├─ 重试逻辑
    └─ 故障转移
    ↓
[6] 后端请求 (sendRequest)
    ├─ 构造请求
    ├─ 复制请求头
    └─ 发送到后端
    ↓
[7] 响应处理
    ├─ 读取响应体
    ├─ 透传到客户端
    └─ 记录指标
    ↓
[8] 异步后处理 (goroutine)
    ├─ 收集用量信息
    ├─ 扣减额度
    ├─ 发送 Webhook
    └─ 记录指标
```

## 可编程扩展点分析

### 1. 鉴权前置钩子 (Pre-Auth Hook)

**位置**: 在鉴权中间件之前

**用途**:
- 自定义请求预处理
- 请求日志记录
- 自定义请求验证
- 请求参数转换

**可透传参数**:
```json
{
  "request_id": "req_xxx",
  "method": "POST",
  "path": "/v1/chat/completions",
  "headers": {...},
  "body": {...},
  "client_ip": "1.2.3.4",
  "timestamp": "2026-01-16T10:00:00Z"
}
```

**返回值**:
```json
{
  "allow": true,
  "modified_body": {...},  // 可选：修改后的请求体
  "modified_headers": {...},  // 可选：修改后的请求头
  "error": "拒绝原因"  // 如果 allow=false
}
```

---

### 2. 鉴权决策钩子 (Auth Decision Hook)

**位置**: 在鉴权中间件内部，标准检查之后

**用途**:
- 自定义鉴权逻辑
- 多因素认证
- 动态权限控制
- 与外部系统集成

**可透传参数**:
```json
{
  "request_id": "req_xxx",
  "api_key": "sk-xxx",
  "key_info": {
    "user_id": "user_001",
    "name": "测试用户",
    "status": "active",
    "total_quota": 100000,
    "used_quota": 50000,
    "allowed_ips": ["192.168.1.0/24"]
  },
  "client_ip": "192.168.1.100",
  "request_body": {...},
  "standard_checks": {
    "key_valid": true,
    "status_active": true,
    "not_expired": true,
    "ip_allowed": true,
    "quota_available": true
  }
}
```

**返回值**:
```json
{
  "allow": true,
  "reason": "自定义拒绝原因",
  "metadata": {
    "custom_field": "value"
  }
}
```

---

### 3. 路由决策钩子 (Routing Decision Hook)

**位置**: 在负载均衡器选择后端之前

**用途**:
- 自定义路由逻辑
- 基于请求内容的路由
- A/B 测试
- 灰度发布
- 多租户路由

**可透传参数**:
```json
{
  "request_id": "req_xxx",
  "api_key": "sk-xxx",
  "user_id": "user_001",
  "request_body": {
    "model": "gpt-4",
    "messages": [...],
    "temperature": 0.7,
    "custom_param": "value"
  },
  "available_backends": [
    {
      "url": "http://backend-1:8000",
      "healthy": true,
      "weight": 10,
      "latency_ms": 120,
      "active_connections": 5
    },
    {
      "url": "http://backend-2:8000",
      "healthy": true,
      "weight": 10,
      "latency_ms": 150,
      "active_connections": 3
    }
  ],
  "client_ip": "1.2.3.4",
  "headers": {...}
}
```

**返回值**:
```json
{
  "backend_url": "http://backend-1:8000",  // 指定后端
  "use_default": false,  // true 表示使用默认负载均衡
  "modified_body": {...},  // 可选：修改请求体
  "metadata": {
    "routing_reason": "基于用户 ID 路由"
  }
}
```

**应用场景**:
- **基于模型路由**: 根据 `request_body.model` 参数路由到不同后端
- **基于用户路由**: VIP 用户路由到高性能后端
- **基于地域路由**: 根据 IP 路由到最近的后端
- **基于负载路由**: 根据后端负载动态调整
- **灰度发布**: 10% 流量路由到新版本后端

---

### 4. 请求转换钩子 (Request Transform Hook)

**位置**: 在发送到后端之前

**用途**:
- 请求参数转换
- 添加/修改请求头
- 模型名称映射
- 参数标准化

**可透传参数**:
```json
{
  "request_id": "req_xxx",
  "api_key": "sk-xxx",
  "user_id": "user_001",
  "backend_url": "http://backend-1:8000",
  "original_body": {...},
  "original_headers": {...}
}
```

**返回值**:
```json
{
  "body": {...},  // 转换后的请求体
  "headers": {...},  // 转换后的请求头
  "skip_transform": false  // true 表示不转换
}
```

---

### 5. 响应转换钩子 (Response Transform Hook)

**位置**: 在返回给客户端之前

**用途**:
- 响应格式转换
- 添加/修改响应头
- 响应内容过滤
- 错误信息标准化

**可透传参数**:
```json
{
  "request_id": "req_xxx",
  "api_key": "sk-xxx",
  "user_id": "user_001",
  "backend_url": "http://backend-1:8000",
  "status_code": 200,
  "original_body": {...},
  "original_headers": {...},
  "latency_ms": 1234
}
```

**返回值**:
```json
{
  "body": {...},  // 转换后的响应体
  "headers": {...},  // 转换后的响应头
  "status_code": 200,  // 可选：修改状态码
  "skip_transform": false
}
```

---

### 6. 用量上报钩子 (Usage Report Hook) ✅ 已实现

**位置**: 请求完成后，异步执行

**用途**:
- 用量计费
- 数据分析
- 审计日志
- 业务统计

**当前透传参数**:
```json
{
  "request_id": "req_xxx",
  "user_id": "user_001",
  "api_key": "sk-xxx",
  "request_body": {
    "model": "gpt-4",
    "messages": [...],
    "temperature": 0.7,
    "custom_param": "value"
  },
  "usage": {
    "prompt_tokens": 15,
    "completion_tokens": 42,
    "total_tokens": 57
  },
  "is_stream": true,
  "endpoint": "/v1/chat/completions",
  "timestamp": "2026-01-16T10:00:00Z",
  "backend_url": "http://backend-1:8000",
  "latency_ms": 1234,
  "status_code": 200
}
```

**优化建议**: 已经很完善，建议添加：
- `response_body`: 完整响应内容（可选，用于审计）
- `error`: 错误信息（如果请求失败）
- `retry_count`: 重试次数
- `fallback_used`: 是否使用了故障转移

---

### 7. 错误处理钩子 (Error Handler Hook)

**位置**: 请求失败时

**用途**:
- 自定义错误响应
- 错误日志记录
- 告警通知
- 降级处理

**可透传参数**:
```json
{
  "request_id": "req_xxx",
  "api_key": "sk-xxx",
  "user_id": "user_001",
  "error_type": "backend_error",  // auth_error, rate_limit, backend_error, timeout
  "error_message": "连接超时",
  "backend_url": "http://backend-1:8000",
  "request_body": {...},
  "retry_count": 3,
  "timestamp": "2026-01-16T10:00:00Z"
}
```

**返回值**:
```json
{
  "custom_response": {
    "error": "服务暂时不可用，请稍后重试",
    "code": "SERVICE_UNAVAILABLE"
  },
  "status_code": 503,
  "use_default": false  // true 表示使用默认错误响应
}
```

---

### 8. 健康检查钩子 (Health Check Hook)

**位置**: 定期执行

**用途**:
- 自定义健康检查逻辑
- 后端状态监控
- 自动摘除/恢复节点

**可透传参数**:
```json
{
  "backend_url": "http://backend-1:8000",
  "last_check_time": "2026-01-16T09:59:50Z",
  "current_status": "healthy",
  "metrics": {
    "latency_ms": 120,
    "active_connections": 5,
    "error_rate": 0.01
  }
}
```

**返回值**:
```json
{
  "healthy": true,
  "reason": "自定义检查失败原因",
  "metadata": {
    "custom_metric": "value"
  }
}
```

---

## 实现方案

### 方案 1: Webhook 方式（推荐）

**优点**:
- 实现简单，不需要修改核心代码
- 支持任何编程语言
- 易于调试和测试
- 支持动态更新

**缺点**:
- 增加网络延迟
- 需要处理 Webhook 失败

**配置示例**:
```yaml
hooks:
  # 鉴权决策钩子
  auth_decision:
    enabled: true
    url: "http://auth-service:8080/api/v1/auth-decision"
    timeout: 500ms
    retry: 2
    
  # 路由决策钩子
  routing_decision:
    enabled: true
    url: "http://routing-service:8080/api/v1/routing-decision"
    timeout: 100ms
    retry: 1
    
  # 请求转换钩子
  request_transform:
    enabled: true
    url: "http://transform-service:8080/api/v1/request-transform"
    timeout: 200ms
    
  # 用量上报钩子（已有）
  usage_report:
    enabled: true
    url: "http://billing-service:8080/api/v1/usage"
    timeout: 2s
    retry: 3
```

---

### 方案 2: 插件方式

**优点**:
- 性能最优，无网络开销
- 支持复杂逻辑

**缺点**:
- 需要编译到二进制
- 更新需要重启服务
- 调试困难

**实现**: 使用 Go Plugin 或 WebAssembly

---

### 方案 3: Lua 脚本方式

**优点**:
- 性能较好
- 支持动态加载
- 语法简单

**缺点**:
- 需要集成 Lua 引擎
- 功能受限

---

## 推荐实现优先级

### Phase 1: 核心钩子（高优先级）

1. **路由决策钩子** - 最灵活，解决多后端路由问题
2. **用量上报钩子优化** - 添加更多上下文信息
3. **错误处理钩子** - 提升用户体验

### Phase 2: 扩展钩子（中优先级）

4. **鉴权决策钩子** - 支持复杂鉴权逻辑
5. **请求转换钩子** - 支持参数映射和转换

### Phase 3: 高级钩子（低优先级）

6. **响应转换钩子** - 响应格式标准化
7. **鉴权前置钩子** - 请求预处理
8. **健康检查钩子** - 自定义健康检查

---

## 透传参数设计原则

1. **完整性**: 透传所有可能有用的信息
2. **一致性**: 所有钩子使用统一的数据结构
3. **可扩展性**: 支持添加自定义字段
4. **安全性**: 敏感信息脱敏（如 API Key）
5. **性能**: 避免序列化大对象

---

## 配置示例：完整的钩子配置

```yaml
# LLMProxy 配置
listen: ":8000"

backends:
  - url: "http://backend-1:8000"
    weight: 10
  - url: "http://backend-2:8000"
    weight: 10

# 钩子配置
hooks:
  # 路由决策钩子
  routing_decision:
    enabled: true
    url: "http://routing-service:8080/decide"
    timeout: 100ms
    retry: 1
    # 失败时的行为
    on_failure: "use_default"  # use_default, reject, ignore
    
  # 鉴权决策钩子
  auth_decision:
    enabled: true
    url: "http://auth-service:8080/check"
    timeout: 500ms
    retry: 2
    on_failure: "reject"
    
  # 请求转换钩子
  request_transform:
    enabled: false
    url: "http://transform-service:8080/transform"
    timeout: 200ms
    
  # 用量上报钩子
  usage_report:
    enabled: true
    url: "http://billing-service:8080/usage"
    timeout: 2s
    retry: 3
    async: true  # 异步执行
    
  # 错误处理钩子
  error_handler:
    enabled: true
    url: "http://error-service:8080/handle"
    timeout: 500ms
    on_failure: "use_default"
```

---

## 使用场景示例

### 场景 1: 基于请求参数的 VIP 路由

**需求**: 根据请求参数中的 `viplevel` 字段，将不同等级的用户路由到不同性能的后端

**客户端请求示例**:
```bash
curl -X POST http://llmproxy:8000/v1/chat/completions \
  -H "Authorization: Bearer sk-llmproxy-test-123" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "你好"}],
    "viplevel": 3,
    "temperature": 0.7
  }'
```

**LLMProxy 配置**:
```yaml
# config.yaml
listen: ":8000"

backends:
  - url: "http://standard-backend:8000"
    weight: 10
  - url: "http://high-performance-backend:8000"
    weight: 10
  - url: "http://ultra-backend:8000"
    weight: 10

# 路由决策钩子
hooks:
  routing_decision:
    enabled: true
    url: "http://routing-service:8080/api/v1/routing-decision"
    timeout: 100ms
    retry: 1
    on_failure: "use_default"  # 钩子失败时使用默认负载均衡
```

**路由服务实现（Python Flask）**:
```python
from flask import Flask, request, jsonify

app = Flask(__name__)

@app.route('/api/v1/routing-decision', methods=['POST'])
def routing_decision():
    """
    路由决策钩子
    根据请求参数中的 viplevel 字段决定路由到哪个后端
    """
    data = request.json
    
    # LLMProxy 会透传以下信息：
    # - request_id: 请求 ID
    # - api_key: API Key
    # - user_id: 用户 ID
    # - request_body: 完整的请求参数（包含 viplevel）
    # - available_backends: 可用的后端列表
    # - client_ip: 客户端 IP
    
    request_body = data.get('request_body', {})
    viplevel = request_body.get('viplevel', 0)  # 默认为 0（普通用户）
    
    # 根据 VIP 等级路由
    if viplevel >= 3:
        # VIP 3 及以上：超高性能后端
        return jsonify({
            "backend_url": "http://ultra-backend:8000",
            "use_default": False,
            "metadata": {
                "routing_reason": f"VIP Level {viplevel} - Ultra Backend",
                "viplevel": viplevel
            }
        })
    elif viplevel >= 1:
        # VIP 1-2：高性能后端
        return jsonify({
            "backend_url": "http://high-performance-backend:8000",
            "use_default": False,
            "metadata": {
                "routing_reason": f"VIP Level {viplevel} - High Performance Backend",
                "viplevel": viplevel
            }
        })
    else:
        # 普通用户：标准后端
        return jsonify({
            "backend_url": "http://standard-backend:8000",
            "use_default": False,
            "metadata": {
                "routing_reason": "Standard User - Standard Backend",
                "viplevel": 0
            }
        })

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8080)
```

**路由服务实现（Go）**:
```go
package main

import (
    "encoding/json"
    "log"
    "net/http"
)

// RoutingRequest LLMProxy 发送的请求
type RoutingRequest struct {
    RequestID         string                 `json:"request_id"`
    APIKey            string                 `json:"api_key"`
    UserID            string                 `json:"user_id"`
    RequestBody       map[string]interface{} `json:"request_body"`
    AvailableBackends []Backend              `json:"available_backends"`
    ClientIP          string                 `json:"client_ip"`
}

type Backend struct {
    URL               string  `json:"url"`
    Healthy           bool    `json:"healthy"`
    Weight            int     `json:"weight"`
    LatencyMS         float64 `json:"latency_ms"`
    ActiveConnections int     `json:"active_connections"`
}

// RoutingResponse 返回给 LLMProxy 的响应
type RoutingResponse struct {
    BackendURL string                 `json:"backend_url,omitempty"`
    UseDefault bool                   `json:"use_default"`
    Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

func routingDecisionHandler(w http.ResponseWriter, r *http.Request) {
    var req RoutingRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }
    
    // 从请求参数中提取 viplevel
    viplevel := 0
    if level, ok := req.RequestBody["viplevel"].(float64); ok {
        viplevel = int(level)
    }
    
    var resp RoutingResponse
    
    // 根据 VIP 等级路由
    switch {
    case viplevel >= 3:
        // VIP 3 及以上：超高性能后端
        resp = RoutingResponse{
            BackendURL: "http://ultra-backend:8000",
            UseDefault: false,
            Metadata: map[string]interface{}{
                "routing_reason": "VIP Level 3+ - Ultra Backend",
                "viplevel":       viplevel,
            },
        }
    case viplevel >= 1:
        // VIP 1-2：高性能后端
        resp = RoutingResponse{
            BackendURL: "http://high-performance-backend:8000",
            UseDefault: false,
            Metadata: map[string]interface{}{
                "routing_reason": "VIP Level 1-2 - High Performance Backend",
                "viplevel":       viplevel,
            },
        }
    default:
        // 普通用户：标准后端
        resp = RoutingResponse{
            BackendURL: "http://standard-backend:8000",
            UseDefault: false,
            Metadata: map[string]interface{}{
                "routing_reason": "Standard User - Standard Backend",
                "viplevel":       0,
            },
        }
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
    
    log.Printf("路由决策: user_id=%s, viplevel=%d, backend=%s", 
        req.UserID, viplevel, resp.BackendURL)
}

func main() {
    http.HandleFunc("/api/v1/routing-decision", routingDecisionHandler)
    log.Println("路由服务启动在 :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

**路由服务实现（Node.js Express）**:
```javascript
const express = require('express');
const app = express();

app.use(express.json());

app.post('/api/v1/routing-decision', (req, res) => {
    const {
        request_id,
        api_key,
        user_id,
        request_body,
        available_backends,
        client_ip
    } = req.body;
    
    // 从请求参数中提取 viplevel
    const viplevel = request_body.viplevel || 0;
    
    let backendUrl;
    let routingReason;
    
    // 根据 VIP 等级路由
    if (viplevel >= 3) {
        // VIP 3 及以上：超高性能后端
        backendUrl = 'http://ultra-backend:8000';
        routingReason = `VIP Level ${viplevel} - Ultra Backend`;
    } else if (viplevel >= 1) {
        // VIP 1-2：高性能后端
        backendUrl = 'http://high-performance-backend:8000';
        routingReason = `VIP Level ${viplevel} - High Performance Backend`;
    } else {
        // 普通用户：标准后端
        backendUrl = 'http://standard-backend:8000';
        routingReason = 'Standard User - Standard Backend';
    }
    
    console.log(`路由决策: user_id=${user_id}, viplevel=${viplevel}, backend=${backendUrl}`);
    
    res.json({
        backend_url: backendUrl,
        use_default: false,
        metadata: {
            routing_reason: routingReason,
            viplevel: viplevel
        }
    });
});

app.listen(8080, () => {
    console.log('路由服务启动在 :8080');
});
```

**完整流程**:

1. **客户端发送请求**，包含 `viplevel` 参数
2. **LLMProxy 接收请求**，提取所有参数
3. **调用路由决策钩子**，发送 POST 请求到 `http://routing-service:8080/api/v1/routing-decision`，包含：
   ```json
   {
     "request_id": "req_abc123",
     "api_key": "sk-llmproxy-test-123",
     "user_id": "user_001",
     "request_body": {
       "model": "gpt-4",
       "messages": [...],
       "viplevel": 3,
       "temperature": 0.7
     },
     "available_backends": [
       {"url": "http://standard-backend:8000", "healthy": true, ...},
       {"url": "http://high-performance-backend:8000", "healthy": true, ...},
       {"url": "http://ultra-backend:8000", "healthy": true, ...}
     ],
     "client_ip": "192.168.1.100"
   }
   ```
4. **路由服务处理**，根据 `viplevel` 返回：
   ```json
   {
     "backend_url": "http://ultra-backend:8000",
     "use_default": false,
     "metadata": {
       "routing_reason": "VIP Level 3 - Ultra Backend",
       "viplevel": 3
     }
   }
   ```
5. **LLMProxy 使用指定后端**，将请求转发到 `http://ultra-backend:8000`
6. **响应返回给客户端**

**高级场景：结合数据库查询**:
```python
@app.route('/api/v1/routing-decision', methods=['POST'])
def routing_decision():
    data = request.json
    user_id = data.get('user_id')
    request_body = data.get('request_body', {})
    
    # 优先使用请求参数中的 viplevel
    viplevel = request_body.get('viplevel')
    
    # 如果请求中没有 viplevel，从数据库查询
    if viplevel is None:
        viplevel = db.query(
            "SELECT vip_level FROM users WHERE id = ?", 
            user_id
        )
    
    # 根据 VIP 等级路由
    if viplevel >= 3:
        return jsonify({
            "backend_url": "http://ultra-backend:8000",
            "use_default": False
        })
    elif viplevel >= 1:
        return jsonify({
            "backend_url": "http://high-performance-backend:8000",
            "use_default": False
        })
    else:
        return jsonify({
            "backend_url": "http://standard-backend:8000",
            "use_default": False
        })
```

---

### 场景 2: 基于模型的路由

**需求**: 不同模型路由到不同的后端集群

**实现**:
```python
@app.route('/decide', methods=['POST'])
def routing_decision():
    data = request.json
    model = data['request_body'].get('model', '')
    
    # 模型路由映射
    routing_map = {
        'gpt-4': 'http://gpt4-cluster:8000',
        'gpt-3.5': 'http://gpt35-cluster:8000',
        'llama-3': 'http://llama-cluster:8000'
    }
    
    backend_url = routing_map.get(model)
    if backend_url:
        return {
            "backend_url": backend_url,
            "use_default": False
        }
    
    return {"use_default": True}
```

---

### 场景 3: 自定义鉴权逻辑

**需求**: 检查用户是否有权限使用特定模型

**实现**:
```python
@app.route('/check', methods=['POST'])
def auth_decision():
    data = request.json
    user_id = data['user_id']
    model = data['request_body'].get('model', '')
    
    # 查询用户权限
    allowed_models = db.query(
        "SELECT allowed_models FROM user_permissions WHERE user_id=?",
        user_id
    )
    
    if model not in allowed_models:
        return {
            "allow": False,
            "reason": f"您没有权限使用模型 {model}"
        }
    
    return {"allow": True}
```

---

## 总结

通过引入生命周期钩子机制，LLMProxy 可以在保持核心简单高效的同时，提供强大的可编程能力。建议优先实现**路由决策钩子**和**用量上报钩子优化**，这两个钩子可以解决大部分实际业务需求。
