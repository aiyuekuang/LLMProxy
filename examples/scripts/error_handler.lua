-- 错误处理脚本示例

-- 错误类型映射表
local error_messages = {
    backend_error = "后端服务异常，请稍后重试",
    timeout = "请求超时，请稍后重试",
    rate_limit = "请求过于频繁，请稍后重试",
    auth_error = "认证失败，请检查 API Key"
}

-- 获取错误信息
local error_type = error.type or "unknown"
local error_message = error.message or ""
local status_code = error.status_code or 500

-- 获取友好的错误消息
local friendly_message = error_messages[error_type] or "未知错误"

-- 记录日志
log.error("错误处理: type=" .. error_type .. ", message=" .. error_message)

-- 返回统一的错误响应
return {
    status_code = status_code,
    body = {
        error = {
            message = friendly_message,
            type = error_type,
            request_id = request.id,
            details = error_message
        }
    }
}
