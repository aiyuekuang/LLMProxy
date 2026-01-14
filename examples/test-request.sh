#!/bin/bash
# LLMProxy 测试脚本

LLMPROXY_URL="http://localhost:8080"

echo "=========================================="
echo "测试 1: 非流式请求"
echo "=========================================="
curl -X POST "$LLMPROXY_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "meta-llama/Llama-3-8b-Instruct",
    "messages": [
      {"role": "user", "content": "你好，请介绍一下你自己"}
    ],
    "stream": false,
    "max_tokens": 100
  }'

echo -e "\n\n=========================================="
echo "测试 2: 流式请求"
echo "=========================================="
curl -X POST "$LLMPROXY_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "meta-llama/Llama-3-8b-Instruct",
    "messages": [
      {"role": "user", "content": "写一首关于春天的诗"}
    ],
    "stream": true,
    "max_tokens": 100
  }'

echo -e "\n\n=========================================="
echo "测试 3: 查看监控指标"
echo "=========================================="
curl "$LLMPROXY_URL/metrics"

echo -e "\n\n=========================================="
echo "测试 4: 健康检查"
echo "=========================================="
curl "$LLMPROXY_URL/health"

echo -e "\n\n测试完成！"
