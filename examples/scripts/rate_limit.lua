-- 限流决策脚本示例

-- 获取当前时间
local hour = current_time.hour or 0

-- 获取用户配额
local total_quota = key_info.total_quota or 0

-- 获取限流状态
local key_remaining = rate_limit_status.key_remaining or 0

-- 记录日志
log.info("限流决策: hour=" .. hour .. ", total_quota=" .. total_quota .. ", key_remaining=" .. key_remaining)

-- 高峰期（9-18点）更严格的限流
if hour >= 9 and hour <= 18 then
    -- 普通用户（配额 < 100万）在高峰期限流更严格
    if total_quota < 1000000 then
        if key_remaining < 10 then
            return {
                allow = false,
                reason = "高峰期限流，请稍后重试",
                retry_after = 60
            }
        end
    end
end

-- 使用标准限流逻辑
return nil
