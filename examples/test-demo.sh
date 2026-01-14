#!/bin/bash
# LLMProxy Demo 测试脚本

LLMPROXY_URL="http://localhost:8000"

echo "=========================================="
echo "LLMProxy Demo 测试"
echo "=========================================="
echo ""

echo "1. 检查 LLMProxy 健康状态..."
curl -s "$LLMPROXY_URL/health"
echo -e "\n"

echo "2. 测试非流式请求..."
curl -X POST "$LLMPROXY_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen2.5:0.5b",
    "messages": [
      {"role": "user", "content": "你好，请用一句话介绍你自己"}
    ],
    "stream": false,
    "max_tokens": 50
  }'
echo -e "\n\n"

echo "3. 测试流式请求..."
curl -X POST "$LLMPROXY_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen2.5:0.5b",
    "messages": [
      {"role": "user", "content": "1+1等于几？"}
    ],
    "stream": true,
    "max_tokens": 30
  }'
echo -e "\n\n"

echo "4. 查看 Prometheus 指标..."
curl -s "$LLMPROXY_URL/metrics" | grep llmproxy
echo -e "\n"

echo "=========================================="
echo "测试完成！"
echo "=========================================="
echo ""
echo "提示："
echo "- 查看 Webhook 接收的数据: docker compose -f deployments/docker-compose-demo.yml logs webhook-receiver"
echo "- 查看 LLMProxy 日志: docker compose -f deployments/docker-compose-demo.yml logs llmproxy"
echo "- 查看 Ollama 日志: docker compose -f deployments/docker-compose-demo.yml logs ollama"
