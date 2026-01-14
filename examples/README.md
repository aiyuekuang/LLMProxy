# LLMProxy 示例

本目录包含 LLMProxy 的使用示例和测试工具。

## 文件说明

- `webhook-receiver.py` - Webhook 接收服务示例（Python Flask）
- `test-request.sh` - 测试脚本（发送请求到 LLMProxy）

## 使用方法

### 1. 启动 Webhook 接收服务

```bash
# 安装依赖
pip install flask

# 运行服务
python webhook-receiver.py
```

服务将监听 `http://localhost:3001/llm-usage`

### 2. 配置 LLMProxy

修改 `config.yaml`，设置 Webhook URL：

```yaml
usage_hook:
  enabled: true
  url: "http://localhost:3001/llm-usage"
  timeout: 1s
  retry: 2
```

### 3. 启动 LLMProxy

```bash
# 在项目根目录
go run cmd/main.go --config config.yaml
```

### 4. 发送测试请求

```bash
# 赋予执行权限
chmod +x test-request.sh

# 运行测试
./test-request.sh
```

### 5. 查看用量数据

Webhook 接收服务会：
- 在控制台打印接收到的数据
- 将数据追加到 `usage_log.jsonl` 文件

```bash
# 查看用量日志
cat usage_log.jsonl | jq
```

## 用量数据格式

```json
{
  "request_id": "req_abc123",
  "user_id": "user_alice",
  "api_key": "sk-prod-xxx",
  "model": "meta-llama/Llama-3-8b-Instruct",
  "prompt_tokens": 15,
  "completion_tokens": 42,
  "total_tokens": 57,
  "is_stream": true,
  "endpoint": "/v1/chat/completions",
  "timestamp": "2026-01-14T10:30:00Z",
  "backend_url": "http://vllm:8000"
}
```

## 业务集成示例

### Python + SQLite

```python
import sqlite3

@app.route('/llm-usage', methods=['POST'])
def receive_usage():
    data = request.json
    
    conn = sqlite3.connect('usage.db')
    cursor = conn.cursor()
    
    cursor.execute('''
        INSERT INTO usage_records 
        (request_id, model, prompt_tokens, completion_tokens, total_tokens, timestamp)
        VALUES (?, ?, ?, ?, ?, ?)
    ''', (
        data['request_id'],
        data['model'],
        data['prompt_tokens'],
        data['completion_tokens'],
        data['total_tokens'],
        data['timestamp']
    ))
    
    conn.commit()
    conn.close()
    
    return jsonify({"status": "ok"}), 200
```

### Node.js + MongoDB

```javascript
app.post('/llm-usage', async (req, res) => {
  const data = req.body;
  
  await db.collection('usage_records').insertOne({
    requestId: data.request_id,
    model: data.model,
    promptTokens: data.prompt_tokens,
    completionTokens: data.completion_tokens,
    totalTokens: data.total_tokens,
    timestamp: new Date(data.timestamp)
  });
  
  res.json({ status: 'ok' });
});
```

## 注意事项

1. Webhook 接收服务应该快速响应（< 1s），避免阻塞 LLMProxy
2. 建议使用消息队列（如 RabbitMQ、Kafka）处理高并发场景
3. 实现幂等性，防止重复计费（使用 `request_id` 去重）
