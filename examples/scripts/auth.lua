-- 鉴权决策脚本示例

-- 获取请求参数
local model = request.body.model or ""
local user_id = request.user_id

-- 获取 Key 信息
local total_quota = key_info.total_quota or 0

-- 记录日志
log.info("鉴权决策: user_id=" .. user_id .. ", model=" .. model .. ", total_quota=" .. total_quota)

-- VIP 用户（配额 >= 100万）可以使用所有模型
if total_quota >= 1000000 then
    return {
        allow = true
    }
end

-- 普通用户只能使用 gpt-3.5
if model == "gpt-4" or model == "gpt-4-turbo" then
    return {
        allow = false,
        reason = "您的账户等级不支持 " .. model .. " 模型，请升级为 VIP 用户",
        status_code = 403
    }
end

-- 允许访问
return {
    allow = true
}
