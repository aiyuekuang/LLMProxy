# LLMProxy Admin API 设计方案

## 概述

为 LLMProxy 提供可选的管理 API，支持 API Key 管理、用量查询、后端状态监控等功能。

**设计原则：**
- **性能隔离** - Admin API 独立端口，不影响代理主路径
- **可选部署** - 通过配置开关，用户可选择启用或关闭
- **安全可控** - 支持只读模式、Token 鉴权、IP 白名单

## 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                        LLMProxy                              │
│                                                              │
│  ┌────────────────────────┐  ┌────────────────────────────┐ │
│  │    Proxy Engine        │  │      Admin API             │ │
│  │    (:8000)             │  │      (:8001)               │ │
│  │                        │  │                            │ │
│  │  - 零缓冲流式转发       │  │  - /admin/keys            │ │
│  │  - 负载均衡            │  │  - /admin/usage           │ │
│  │  - 鉴权/限流           │  │  - /admin/backends        │ │
│  │                        │  │  - /admin/stats           │ │
│  └────────────────────────┘  └────────────────────────────┘ │
│              │                           │                   │
│              │ 共享数据存储               │                   │
│              └───────────┬───────────────┘                   │
│                          ▼                                   │
│              ┌────────────────────────┐                      │
│              │   Redis / Memory       │                      │
│              │   (API Keys, Usage)    │                      │
│              └────────────────────────┘                      │
└─────────────────────────────────────────────────────────────┘
```

### 性能保证

| 特性 | 说明 |
|------|------|
| **端口隔离** | 代理流量 `:8000`，管理流量 `:8001`，互不干扰 |
| **代码路径分离** | 代理请求完全不经过 Admin 逻辑 |
| **独立连接池** | Admin API 使用独立的 Redis 连接池 |
| **可完全关闭** | `admin.enabled: false` 时零开销 |

## 配置说明

```yaml
# config.yaml

listen: ":8000"  # 代理服务端口（必须）

# Admin API 配置（可选）
admin:
  enabled: true              # 是否启用，默认 false
  listen: ":8001"            # Admin API 端口
  read_only: false           # 只读模式，仅暴露 GET 接口
  
  # 鉴权配置
  auth:
    enabled: true            # 是否启用鉴权
    token: "your-admin-secret-token"  # Bearer Token
  
  # 访问控制
  allowed_ips:               # IP 白名单（可选）
    - "127.0.0.1"
    - "10.0.0.0/8"
    - "192.168.0.0/16"
```

### 配置场景示例

| 场景 | 配置 |
|------|------|
| **生产环境（公网）** | `enabled: false`，通过 Redis 直接管理 |
| **生产环境（内网）** | `enabled: true` + `allowed_ips` 白名单 |
| **监控对接（Grafana）** | `enabled: true` + `read_only: true` |
| **完整管理后台** | `enabled: true` + `auth.enabled: true` |
| **开发/测试** | `enabled: true`，无鉴权 |

## API 接口

### 只读接口（GET）

这些接口相对安全，可在 `read_only: true` 模式下使用。

#### 1. 列出所有 API Key

```
GET /admin/keys
```

**响应：**
```json
{
  "keys": [
    {
      "id": "key_001",
      "name": "开发团队",
      "key_prefix": "sk-llmproxy-***",
      "status": "active",
      "total_quota": 1000000,
      "used_quota": 150000,
      "created_at": "2025-01-01T00:00:00Z",
      "expires_at": "2026-12-31T23:59:59Z"
    }
  ],
  "total": 10
}
```

#### 2. 查看单个 Key 详情

```
GET /admin/keys/:id
```

**响应：**
```json
{
  "id": "key_001",
  "name": "开发团队",
  "key_prefix": "sk-llmproxy-***",
  "user_id": "user_001",
  "status": "active",
  "total_quota": 1000000,
  "used_quota": 150000,
  "allowed_ips": ["10.0.0.0/8"],
  "rate_limit": {
    "requests_per_minute": 60,
    "max_concurrent": 5
  },
  "created_at": "2025-01-01T00:00:00Z",
  "expires_at": "2026-12-31T23:59:59Z",
  "last_used_at": "2025-01-15T10:30:00Z"
}
```

#### 3. 用量统计汇总

```
GET /admin/usage?start=2025-01-01&end=2025-01-31&group_by=day
```

**参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| `start` | string | 开始日期（ISO 8601） |
| `end` | string | 结束日期（ISO 8601） |
| `group_by` | string | 聚合维度：`hour`, `day`, `month` |
| `key_id` | string | 按 Key 过滤（可选） |

**响应：**
```json
{
  "summary": {
    "total_requests": 50000,
    "total_prompt_tokens": 10000000,
    "total_completion_tokens": 5000000,
    "total_tokens": 15000000
  },
  "breakdown": [
    {
      "date": "2025-01-01",
      "requests": 1500,
      "prompt_tokens": 300000,
      "completion_tokens": 150000
    }
  ]
}
```

#### 4. 单个 Key 用量明细

```
GET /admin/usage/:key_id?start=2025-01-01&end=2025-01-31
```

**响应：**
```json
{
  "key_id": "key_001",
  "key_name": "开发团队",
  "period": {
    "start": "2025-01-01T00:00:00Z",
    "end": "2025-01-31T23:59:59Z"
  },
  "usage": {
    "total_requests": 5000,
    "total_prompt_tokens": 1000000,
    "total_completion_tokens": 500000,
    "total_tokens": 1500000
  },
  "daily_breakdown": [
    {
      "date": "2025-01-01",
      "requests": 150,
      "tokens": 45000
    }
  ]
}
```

#### 5. 后端服务状态

```
GET /admin/backends
```

**响应：**
```json
{
  "backends": [
    {
      "url": "http://vllm-1:8000",
      "weight": 10,
      "healthy": true,
      "last_check": "2025-01-15T10:30:00Z",
      "latency_ms": 15,
      "active_connections": 5
    },
    {
      "url": "http://vllm-2:8000",
      "weight": 10,
      "healthy": false,
      "last_check": "2025-01-15T10:30:00Z",
      "error": "connection refused"
    }
  ]
}
```

#### 6. 系统统计

```
GET /admin/stats
```

**响应：**
```json
{
  "uptime_seconds": 86400,
  "requests": {
    "total": 100000,
    "success": 99500,
    "failed": 500,
    "qps_1m": 15.5
  },
  "connections": {
    "active": 25,
    "idle": 75
  },
  "memory": {
    "alloc_mb": 45,
    "sys_mb": 120
  }
}
```

#### 7. 当前运行配置

```
GET /admin/config
```

**响应（敏感信息脱敏）：**
```json
{
  "listen": ":8000",
  "backends_count": 3,
  "auth_enabled": true,
  "rate_limit_enabled": true,
  "usage_hook_enabled": true
}
```

---

### 读写接口（POST/PUT/DELETE）

这些接口需要 `read_only: false` 且通过鉴权。

#### 1. 创建 API Key

```
POST /admin/keys
Authorization: Bearer your-admin-secret-token
Content-Type: application/json
```

**请求体：**
```json
{
  "name": "新项目 Key",
  "user_id": "user_002",
  "total_quota": 500000,
  "allowed_ips": ["192.168.1.0/24"],
  "rate_limit": {
    "requests_per_minute": 30,
    "max_concurrent": 3
  },
  "expires_at": "2026-12-31T23:59:59Z"
}
```

**响应：**
```json
{
  "id": "key_002",
  "key": "sk-llmproxy-abc123xyz789",
  "name": "新项目 Key",
  "created_at": "2025-01-15T10:30:00Z"
}
```

> ⚠️ **注意**：完整 Key 仅在创建时返回一次，请妥善保存。

#### 2. 更新 API Key

```
PUT /admin/keys/:id
Authorization: Bearer your-admin-secret-token
Content-Type: application/json
```

**请求体（仅包含要更新的字段）：**
```json
{
  "name": "更新后的名称",
  "status": "disabled",
  "total_quota": 1000000,
  "allowed_ips": ["10.0.0.0/8"]
}
```

**响应：**
```json
{
  "id": "key_002",
  "name": "更新后的名称",
  "status": "disabled",
  "updated_at": "2025-01-15T11:00:00Z"
}
```

#### 3. 删除 API Key

```
DELETE /admin/keys/:id
Authorization: Bearer your-admin-secret-token
```

**响应：**
```json
{
  "message": "Key deleted successfully",
  "id": "key_002"
}
```

#### 4. 热重载配置

```
POST /admin/config/reload
Authorization: Bearer your-admin-secret-token
```

**响应：**
```json
{
  "message": "Configuration reloaded",
  "timestamp": "2025-01-15T11:00:00Z"
}
```

---

## 鉴权说明

### Bearer Token 鉴权

```bash
curl -H "Authorization: Bearer your-admin-secret-token" \
     http://localhost:8001/admin/keys
```

### 错误响应

**401 Unauthorized：**
```json
{
  "error": "unauthorized",
  "message": "Invalid or missing authorization token"
}
```

**403 Forbidden（IP 不在白名单）：**
```json
{
  "error": "forbidden",
  "message": "IP address not allowed"
}
```

**405 Method Not Allowed（只读模式下尝试写操作）：**
```json
{
  "error": "method_not_allowed",
  "message": "Write operations disabled in read-only mode"
}
```

---

## 前端管理界面（可选）

Admin API 可对接独立的前端管理界面。

### 推荐技术栈

| 组件 | 技术选型 |
|------|---------|
| 框架 | React 18 + TypeScript |
| UI 库 | shadcn/ui + TailwindCSS |
| 图表 | Recharts |
| HTTP | Axios / fetch |
| 部署 | Nginx 静态托管 / Vercel / Netlify |

### 功能页面

1. **Dashboard** - 系统概览、实时 QPS、后端状态
2. **API Keys** - Key 列表、创建、编辑、删除
3. **Usage** - 用量统计图表、按 Key/时间筛选
4. **Backends** - 后端健康状态、延迟监控
5. **Settings** - 配置查看

---

## 部署架构示例

### 最小部署

```
┌──────────────┐
│  LLMProxy    │
│  :8000 代理   │
│  :8001 Admin │
└──────────────┘
```

### 生产部署（带管理界面）

```
                    ┌─────────────────┐
                    │   Nginx         │
                    │   :80/:443      │
                    └────────┬────────┘
                             │
           ┌─────────────────┼─────────────────┐
           │                 │                 │
           ▼                 ▼                 ▼
    ┌─────────────┐  ┌─────────────┐  ┌─────────────┐
    │ LLMProxy    │  │ Admin UI    │  │ LLMProxy    │
    │ :8000       │  │ (静态文件)   │  │ :8001       │
    │ (代理流量)   │  │             │  │ (Admin API) │
    └─────────────┘  └─────────────┘  └─────────────┘
```

**Nginx 配置示例：**

```nginx
# 代理流量
location /v1/ {
    proxy_pass http://llmproxy:8000;
}

# 管理界面
location /admin/ {
    alias /var/www/admin-ui/;
    try_files $uri $uri/ /admin/index.html;
}

# Admin API
location /api/admin/ {
    proxy_pass http://llmproxy:8001/admin/;
    # 仅允许内网访问
    allow 10.0.0.0/8;
    deny all;
}
```

---

## 行业参考

| 网关 | Admin API 方案 | 特点 |
|------|---------------|------|
| Kong | `:8001` 独立端口 | 完整 REST API，可配合 Konga UI |
| APISIX | `:9180` 独立端口 | etcd 存储，有官方 Dashboard |
| Traefik | 内嵌只读 Dashboard | 轻量，无写操作 |
| Envoy | 无内置，靠控制面 | xDS API 动态配置 |

LLMProxy 的设计参考了 Kong 和 APISIX 的模式：**Admin API 内嵌 + 端口隔离 + 可选前端**。

---

## 实现计划

### Phase 1：只读 API（MVP）
- [ ] `/admin/keys` GET
- [ ] `/admin/usage` GET
- [ ] `/admin/backends` GET
- [ ] `/admin/stats` GET
- [ ] 配置解析和端口启动

### Phase 2：读写 API
- [ ] `/admin/keys` POST/PUT/DELETE
- [ ] Token 鉴权
- [ ] IP 白名单

### Phase 3：前端界面（独立项目）
- [ ] React 管理界面
- [ ] 用量可视化图表

---

## 总结

| 特性 | 说明 |
|------|------|
| **性能无损** | 独立端口，不影响代理主路径 |
| **灵活可控** | 可完全关闭、只读模式、完整管理 |
| **安全可靠** | Token 鉴权 + IP 白名单 |
| **易于扩展** | 可对接独立前端或现有系统 |
