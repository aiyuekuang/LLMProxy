-- 响应转换脚本示例

-- 模型名称反向映射表
local model_map = {
    ["llama-3-70b-instruct"] = "gpt-4",
    ["qwen-72b-chat"] = "gpt-3.5-turbo"
}

-- 获取后端返回的模型名
local backend_model = response.body.model or ""

-- 映射回原始模型名
local original_model = model_map[backend_model] or backend_model

-- 记录日志
log.info("响应转换: " .. backend_model .. " -> " .. original_model)

-- 构造新的响应体
local new_body = {}
for k, v in pairs(response.body) do
    new_body[k] = v
end
new_body.model = original_model

-- 添加元数据
new_body.metadata = {
    backend = backend_url,
    latency_ms = latency_ms,
    original_model = backend_model
}

return {
    status_code = response.status_code,
    body = new_body,
    headers = response.headers
}
