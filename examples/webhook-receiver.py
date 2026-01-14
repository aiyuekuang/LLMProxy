#!/usr/bin/env python3
"""
LLMProxy Webhook 接收服务示例
用于测试用量上报功能
"""

from flask import Flask, request, jsonify
import json
from datetime import datetime

app = Flask(__name__)

@app.route('/llm-usage', methods=['POST'])
def receive_usage():
    """
    接收 LLMProxy 发送的用量数据
    """
    try:
        data = request.json
        
        # 打印接收到的数据
        print(f"\n{'='*60}")
        print(f"[{datetime.now().strftime('%Y-%m-%d %H:%M:%S')}] 收到用量数据:")
        print(json.dumps(data, indent=2, ensure_ascii=False))
        print(f"{'='*60}\n")
        
        # 这里可以添加你的业务逻辑：
        # 1. 写入数据库
        # 2. 更新用户配额
        # 3. 触发计费流程
        # 等等...
        
        # 示例：写入文件
        with open('usage_log.jsonl', 'a', encoding='utf-8') as f:
            f.write(json.dumps(data, ensure_ascii=False) + '\n')
        
        return jsonify({"status": "ok", "message": "用量数据已接收"}), 200
        
    except Exception as e:
        print(f"错误: {e}")
        return jsonify({"status": "error", "message": str(e)}), 500

@app.route('/health', methods=['GET'])
def health():
    """健康检查"""
    return jsonify({"status": "healthy"}), 200

if __name__ == '__main__':
    print("Webhook 接收服务启动中...")
    print("监听地址: http://0.0.0.0:3001")
    print("接收端点: http://0.0.0.0:3001/llm-usage")
    app.run(host='0.0.0.0', port=3001, debug=True)
