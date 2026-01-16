-- 请求转换脚本示例

-- 模型名称映射表
local model_map = {
    ["gpt-4"] = "llama-3-70b-instruct",
    ["gpt-3.5-turbo"] = "qwen-72b-chat",
    ["gpt-3.5"] = "qwen-72b-chat"
}

-- 获取原始模型名
local original_model = request.body.model or ""

-- 映射模型名
local new_model = model_map[original_model] or original_model

-- 记录日志
log.info("请求转换: " .. original_model .. " -> " .. new_model)

-- 构造新的请求体
local new_body = {}
for k, v in pairs(request.body) do
    new_body[k] = v
end
new_body.model = new_model

-- 添加自定义请求头
local new_headers = {
    ["X-User-ID"] = request.user_id,
    ["X-Original-Model"] = original_model,
    ["X-Backend-URL"] = request.backend_url
}

return {
    body = new_body,
    headers = new_headers
}
