# LLMProxy Demo 快速开始

本 Demo 使用 Ollama + 小模型（qwen2.5:0.5b，约 2GB）进行完整测试。

## 系统要求

- Docker 和 Docker Compose
- 至少 4GB 可用磁盘空间
- 至少 4GB 可用内存

## 快速启动

### 1. 启动所有服务

```bash
cd deployments
docker compose -f docker-compose-demo.yml up -d
```

首次启动会：
- 构建 LLMProxy 镜像
- 下载 Ollama 镜像
- 自动拉取 qwen2.5:0.5b 模型（约 2GB，需要几分钟）

### 2. 查看启动进度

```bash
# 查看所有服务状态
docker compose -f docker-compose-demo.yml ps

# 查看 Ollama 日志（等待模型下载完成）
docker compose -f docker-compose-demo.yml logs -f ollama
```

等待看到类似输出：
```
ollama-1  | pulling manifest
ollama-1  | success
```

### 3. 运行测试

```bash
# 赋予执行权限
chmod +x ../examples/test-demo.sh

# 运行测试
../examples/test-demo.sh
```

### 4. 查看用量数据

```bash
# 查看 Webhook 接收的用量数据
docker compose -f docker-compose-demo.yml logs webhook-receiver
```

你会看到类似输出：
```json
{
  "request_id": "chatcmpl-xxx",
  "model": "qwen2.5:0.5b",
  "prompt_tokens": 15,
  "completion_tokens": 42,
  "total_tokens": 57,
  "is_stream": false,
  "endpoint": "/v1/chat/completions",
  "timestamp": "2026-01-14T10:30:00Z",
  "backend_url": "http://ollama:11434/v1"
}
```

## 访问服务

- **LLMProxy**: http://localhost:8000
- **Ollama**: http://localhost:11434
- **Webhook 接收器**: http://localhost:3001
- **Prometheus 指标**: http://localhost:8000/metrics

## 手动测试

### 非流式请求

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen2.5:0.5b",
    "messages": [
      {"role": "user", "content": "你好"}
    ],
    "stream": false
  }'
```

### 流式请求

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen2.5:0.5b",
    "messages": [
      {"role": "user", "content": "你好"}
    ],
    "stream": true
  }'
```

## 停止服务

```bash
docker compose -f docker-compose-demo.yml down
```

保留数据（下次启动更快）：
```bash
docker compose -f docker-compose-demo.yml down
```

完全清理（包括模型数据）：
```bash
docker compose -f docker-compose-demo.yml down -v
```

## 故障排查

### 1. Ollama 模型下载失败

```bash
# 手动进入容器下载
docker compose -f docker-compose-demo.yml exec ollama bash
ollama pull qwen2.5:0.5b
```

### 2. LLMProxy 连接 Ollama 失败

```bash
# 检查 Ollama 是否正常
curl http://localhost:11434/api/tags

# 查看 LLMProxy 日志
docker compose -f docker-compose-demo.yml logs llmproxy
```

### 3. 内存不足

如果内存不足，可以使用更小的模型：

修改 `docker-compose-demo.yml` 中的模型：
```yaml
ollama pull qwen2.5:0.5b  # 改为 tinyllama (约 600MB)
```

## 使用其他模型

Ollama 支持多种模型，修改 `docker-compose-demo.yml`：

```yaml
# 超小模型（约 600MB）
ollama pull tinyllama

# 小模型（约 2GB）
ollama pull qwen2.5:0.5b

# 中等模型（约 4GB）
ollama pull qwen2.5:1.5b

# 大模型（约 8GB）
ollama pull qwen2.5:3b
```

更多模型：https://ollama.com/library

## 性能测试

```bash
# 安装 Apache Bench（如果没有）
# macOS: brew install httpd
# Ubuntu: sudo apt-get install apache2-utils

# 并发测试
ab -n 100 -c 10 -p request.json -T application/json http://localhost:8000/v1/chat/completions
```

其中 `request.json`:
```json
{
  "model": "qwen2.5:0.5b",
  "messages": [{"role": "user", "content": "Hello"}],
  "stream": false
}
```

## 下一步

1. 查看 [架构文档](../docs/architecture.md) 了解设计细节
2. 查看 [部署指南](../docs/deployment-guide.md) 了解生产部署
3. 修改配置文件测试不同场景
