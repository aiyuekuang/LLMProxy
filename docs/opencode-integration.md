# OpenCode 接入 LLMProxy 方案

## 概述

本文档描述 OpenCode（AI 编程助手）如何接入 LLMProxy 网关使用自建的 LLM 推理服务。

## OpenCode 简介

OpenCode 是一个开源的 AI 编程助手，支持终端、桌面应用和 IDE 插件。它使用 AI SDK 支持 75+ 个 LLM 提供商，并支持本地模型。

- 官方文档：https://opencode.ai/docs/
- GitHub：https://github.com/opencode-ai/opencode

## API 格式要求

### 1. 端点格式

OpenCode 需要 **OpenAI 兼容的 API 格式**：

```
POST /v1/chat/completions
POST /v1/completions
```

### 2. 请求格式

```json
{
  "model": "模型名称",
  "messages": [
    {"role": "system", "content": "系统提示"},
    {"role": "user", "content": "用户消息"}
  ],
  "stream": true,
  "temperature": 0.7,
  "max_tokens": 4096
}
```

### 3. 认证方式

使用 Bearer Token 认证：

```
Authorization: Bearer sk-xxx
```

### 4. 响应格式

#### 非流式响应

```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion",
  "created": 1234567890,
  "model": "模型名称",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "响应内容"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 20,
    "total_tokens": 30
  }
}
```

#### 流式响应（SSE 格式）

```
Content-Type: text/event-stream

data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","created":1234567890,"model":"模型名称","choices":[{"index":0,"delta":{"content":"响"},"finish_reason":null}]}

data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","created":1234567890,"model":"模型名称","choices":[{"index":0,"delta":{"content":"应"},"finish_reason":null}]}

data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","created":1234567890,"model":"模型名称","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
```

## LLMProxy 兼容性

LLMProxy 已支持以下特性：

| 特性 | 支持状态 | 说明 |
|------|---------|------|
| `/v1/chat/completions` | ✅ 已支持 | 主要端点 |
| `/v1/completions` | ✅ 已支持 | 文本补全端点 |
| 流式响应 | ✅ 已支持 | SSE 格式 |
| 非流式响应 | ✅ 已支持 | JSON 格式 |
| Bearer Token 认证 | ✅ 已支持 | `Authorization: Bearer xxx` |
| X-API-Key 认证 | ✅ 已支持 | `X-API-Key: xxx` |
| 自定义认证 Header | ✅ 已支持 | 可配置任意 Header 名称 |
| `/v1/models` | ❌ 待实现 | 模型列表查询 |
| Tool Calling | ⚠️ 透传 | 依赖后端支持 |

## OpenCode 配置方法

OpenCode 使用 [AI SDK](https://ai-sdk.dev/) 支持自定义 Provider，通过 `@ai-sdk/openai-compatible` 包接入任何 OpenAI 兼容的 API。

### 步骤 1：添加凭证

运行 `/connect` 命令，选择 **Other**：

```bash
$ /connect

┌  Add credential
│
◆  Select provider
│  ...
│  ● Other
└

┌  Add credential
│
◇  Enter provider id
│  llmproxy          # 自定义 ID，需要与配置文件中一致
└

┌  Add credential
│
◇  Enter your API key
│  sk-llmproxy-xxx   # 你的 LLMProxy API Key
└
```

凭证会保存到 `~/.local/share/opencode/auth.json`。

### 步骤 2：配置 Provider

在项目目录或 `~/.config/opencode/` 创建 `opencode.json`：

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
        "qwen-72b": {
          "name": "Qwen2.5-72B-Instruct",
          "limit": {
            "context": 131072,
            "output": 8192
          }
        },
        "deepseek-coder": {
          "name": "DeepSeek-Coder-V2",
          "limit": {
            "context": 128000,
            "output": 8192
          }
        }
      }
    }
  },
  "model": "llmproxy/qwen-72b"
}
```

**配置说明**：

| 字段 | 必填 | 说明 |
|------|------|------|
| `npm` | ✅ | 固定为 `@ai-sdk/openai-compatible` |
| `name` | ✅ | UI 显示名称 |
| `options.baseURL` | ✅ | LLMProxy 地址，需要包含 `/v1` |
| `models` | ✅ | 可用模型列表 |
| `limit.context` | ⚠️ 推荐 | 最大输入 Token 数 |
| `limit.output` | ⚠️ 推荐 | 最大输出 Token 数 |

### 步骤 3：选择模型

```bash
/models
```

你配置的模型会显示在列表中，格式为 `llmproxy/qwen-72b`。

### 高级配置

#### 使用环境变量设置 API Key

```json
{
  "provider": {
    "llmproxy": {
      "npm": "@ai-sdk/openai-compatible",
      "options": {
        "baseURL": "http://llmproxy:8000/v1",
        "apiKey": "{env:LLMPROXY_API_KEY}"
      }
    }
  }
}
```

#### 添加自定义 Header

```json
{
  "provider": {
    "llmproxy": {
      "npm": "@ai-sdk/openai-compatible",
      "options": {
        "baseURL": "http://llmproxy:8000/v1",
        "headers": {
          "X-Custom-Header": "value"
        }
      }
    }
  }
}
```

---

## LLMProxy 自定义认证配置

LLMProxy 支持自定义认证 Header，可以在 `config.yaml` 中配置：

### 默认认证方式

不配置 `header_names` 时，LLMProxy 按以下顺序提取 API Key：

1. `Authorization: Bearer sk-xxx` - 提取 Bearer Token
2. `X-API-Key: sk-xxx` - 直接使用值

### 自定义认证 Header

```yaml
auth:
  enabled: true
  storage: "file"
  header_names:
    - "Authorization"      # 支持 Bearer Token 格式
    - "X-API-Key"          # 自定义 Header
    - "Api-Key"            # 另一个自定义 Header
    - "X-Custom-Auth"      # 任意 Header 名称
```

**配置说明**：
- `header_names` 是一个列表，按顺序依次尝试提取
- `Authorization` Header 会特殊处理，提取 `Bearer ` 后面的内容
- 其他 Header 直接使用值作为 API Key
- 找到第一个非空值即返回

### OpenCode 配合使用

如果 LLMProxy 配置了自定义 Header（如 `Api-Key`），OpenCode 需要相应配置：

```json
{
  "provider": {
    "llmproxy": {
      "npm": "@ai-sdk/openai-compatible",
      "options": {
        "baseURL": "http://llmproxy:8000/v1",
        "headers": {
          "Api-Key": "{env:LLMPROXY_API_KEY}"
        }
      }
    }
  }
}
```

## 业务场景分析

### 目标场景

```
用户自建推理服务 (vLLM/TGI) → LLMProxy 网关 → OpenCode 客户端
```

**价值**：
- 用户可以使用自己部署的开源模型（Qwen、Llama、DeepSeek 等）
- 通过 LLMProxy 获得负载均衡、鉴权、限流、监控等能力
- 使用 OpenCode 作为编程助手，数据不出内网

### OpenCode 核心功能分析

| 功能 | 依赖的 API 能力 | 说明 |
|------|----------------|------|
| 代码理解/问答 | Chat Completions + 流式 | 基础对话能力 |
| 读写文件 | **Tool Calling** | 必须支持 |
| 代码搜索 | **Tool Calling** | 必须支持 |
| 执行命令 | **Tool Calling** | 必须支持 |
| Plan/Build 模式 | Chat Completions | 多轮对话 |
| 图片理解 | Vision（可选） | 拖拽图片 |

**结论**：OpenCode 的核心功能 **强依赖 Tool Calling**，没有 Tool Calling 就无法读写代码。

---

## LLMProxy 当前能力评估

### ✅ 已满足的需求

| 需求 | 状态 | 说明 |
|------|------|------|
| `/v1/chat/completions` | ✅ | 核心端点已支持 |
| 流式响应 (SSE) | ✅ | 已支持 |
| Bearer Token 认证 | ✅ | API Key 鉴权已支持 |
| 负载均衡 | ✅ | 轮询、最少连接等 |
| 限流保护 | ✅ | 全局/Key 级限流 |
| 用量统计 | ✅ | Webhook 上报 |

### ⚠️ 需要验证的能力

| 需求 | 状态 | 说明 |
|------|------|------|
| Tool Calling 透传 | ⚠️ 待验证 | LLMProxy 本身透传请求，但需要后端模型支持 |
| 长连接稳定性 | ⚠️ 待验证 | 编程任务可能持续数分钟 |

### ❌ 需要新增的能力

| 需求 | 优先级 | 说明 |
|------|--------|------|
| `/v1/models` 接口 | 中 | 返回可用模型列表，方便 OpenCode 选择 |

---

## 优化方案

### 方案 1：最小化改动（推荐）

**原理**：LLMProxy 作为透明代理，只要后端模型支持 Tool Calling，无需任何改动。

**前提条件**：
- 后端使用支持 Tool Calling 的模型（如 Qwen-2.5、Llama-3.1、DeepSeek-V2.5 等）
- vLLM 启用 `--enable-auto-tool-choice` 参数

**vLLM 启动命令示例**：
```bash
python -m vllm.entrypoints.openai.api_server \
  --model Qwen/Qwen2.5-72B-Instruct \
  --enable-auto-tool-choice \
  --tool-call-parser hermes \
  --return-detailed-tokens \
  --port 8000
```

**优点**：零代码改动，立即可用
**缺点**：依赖后端模型能力

### 方案 2：新增 `/v1/models` 接口

在 LLMProxy 中实现模型列表接口，方便 OpenCode 查询可用模型。

**实现位置**：`internal/proxy/handler.go`

**响应格式**：
```json
{
  "object": "list",
  "data": [
    {
      "id": "qwen-72b",
      "object": "model",
      "created": 1706745600,
      "owned_by": "llmproxy"
    }
  ]
}
```

**配置方式**（config.yaml）：
```yaml
models:
  - id: "qwen-72b"
    name: "Qwen2.5-72B-Instruct"
    description: "通义千问 72B 指令模型"
  - id: "deepseek-coder"
    name: "DeepSeek-Coder-V2"
    description: "DeepSeek 代码模型"
```

### 方案 3：增强监控（可选）

针对 OpenCode 场景增加特定监控：

- Tool Calling 调用统计
- 单次会话 Token 消耗
- 长连接超时告警

---

## 后端模型要求

### 支持 Tool Calling 的开源模型

| 模型 | Tool Calling | 推荐度 | 说明 |
|------|-------------|--------|------|
| Qwen2.5-72B-Instruct | ✅ | ⭐⭐⭐⭐⭐ | 最佳选择，中文友好 |
| Qwen2.5-Coder-32B | ✅ | ⭐⭐⭐⭐⭐ | 代码专用，Tool Calling 优秀 |
| DeepSeek-V2.5 | ✅ | ⭐⭐⭐⭐ | 性价比高 |
| Llama-3.1-70B-Instruct | ✅ | ⭐⭐⭐⭐ | 英文优秀 |
| Llama-3.3-70B-Instruct | ✅ | ⭐⭐⭐⭐ | 最新版本 |
| Mistral-Large | ✅ | ⭐⭐⭐ | 支持 Tool Calling |

### 不支持 Tool Calling 的模型（不推荐）

- 纯 base 模型（未经指令微调）
- 部分小参数模型（7B 以下）
- 早期版本的模型

---

## Tool Calling 数据格式

### 请求格式

```json
{
  "model": "qwen-72b",
  "messages": [
    {"role": "user", "content": "读取 src/main.go 文件内容"}
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "read_file",
        "description": "读取指定文件的内容",
        "parameters": {
          "type": "object",
          "properties": {
            "path": {
              "type": "string",
              "description": "文件路径"
            }
          },
          "required": ["path"]
        }
      }
    }
  ],
  "tool_choice": "auto"
}
```

### 响应格式

```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": null,
        "tool_calls": [
          {
            "id": "call_abc123",
            "type": "function",
            "function": {
              "name": "read_file",
              "arguments": "{\"path\": \"src/main.go\"}"
            }
          }
        ]
      },
      "finish_reason": "tool_calls"
    }
  ],
  "usage": {
    "prompt_tokens": 50,
    "completion_tokens": 20,
    "total_tokens": 70
  }
}
```

---

## 部署架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        用户环境                                   │
│  ┌──────────────┐                                               │
│  │   OpenCode   │ ◄─── 开发者使用的 AI 编程助手                    │
│  │   (客户端)    │                                               │
│  └──────┬───────┘                                               │
│         │ HTTPS + Bearer Token                                  │
│         ▼                                                       │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐    │
│  │   LLMProxy   │────►│    vLLM      │────►│   GPU 集群    │    │
│  │   (网关)      │     │  (推理服务)   │     │              │    │
│  └──────────────┘     └──────────────┘     └──────────────┘    │
│         │                                                       │
│         ▼ Webhook                                               │
│  ┌──────────────┐                                               │
│  │  业务系统     │ ◄─── 计费、审计、监控                           │
│  └──────────────┘                                               │
└─────────────────────────────────────────────────────────────────┘
```

---

## 快速验证步骤

### 1. 检查后端 Tool Calling 支持

```bash
curl http://your-vllm:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen-72b",
    "messages": [{"role": "user", "content": "调用 get_weather 函数查询北京天气"}],
    "tools": [{
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "获取天气",
        "parameters": {
          "type": "object",
          "properties": {"city": {"type": "string"}},
          "required": ["city"]
        }
      }
    }]
  }'
```

**期望响应**：包含 `tool_calls` 字段

### 2. 通过 LLMProxy 测试

```bash
curl http://llmproxy:8000/v1/chat/completions \
  -H "Authorization: Bearer sk-your-key" \
  -H "Content-Type: application/json" \
  -d '{ ... 同上 ... }'
```

### 3. 配置 OpenCode

```json
{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "llmproxy": {
      "options": {
        "baseURL": "http://llmproxy:8000/v1"
      },
      "models": {
        "qwen-72b": {}
      }
    }
  },
  "model": "llmproxy/qwen-72b"
}
```

---

## 总结

| 问题 | 结论 |
|------|------|
| LLMProxy 能否支持 OpenCode？ | ✅ **可以**，作为透明代理已具备基础能力 |
| 需要改动 LLMProxy 吗？ | ⚠️ **不一定**，取决于是否需要 `/v1/models` 接口 |
| 关键依赖是什么？ | 🔑 **后端模型必须支持 Tool Calling** |
| 推荐的模型？ | Qwen2.5-72B-Instruct 或 Qwen2.5-Coder-32B |

## 参考资料

- [OpenCode 官方文档](https://opencode.ai/docs/)
- [OpenCode Models 配置](https://opencode.ai/docs/models/)
- [OpenCode Providers 配置](https://opencode.ai/docs/providers/)
- [LLMProxy README](../README.md)
