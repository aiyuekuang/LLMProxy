-- 用量计算脚本示例

-- 获取 VIP 等级
local viplevel = request.body.viplevel or 0

-- 获取原始用量
local prompt_tokens = usage.prompt_tokens or 0
local completion_tokens = usage.completion_tokens or 0
local total_tokens = usage.total_tokens or 0

-- 根据 VIP 等级计算折扣
local discount = 1.0
if viplevel >= 3 then
    discount = 0.5  -- VIP 3+: 5折
elseif viplevel >= 1 then
    discount = 0.8  -- VIP 1-2: 8折
end

-- 计算计费 tokens
local billable_tokens = math.floor(total_tokens * discount)

-- 计算成本（假设每 1000 tokens = 0.01 元）
local cost = billable_tokens * 0.00001

-- 记录日志
log.info("用量计算: viplevel=" .. viplevel .. ", total_tokens=" .. total_tokens .. ", billable_tokens=" .. billable_tokens .. ", cost=" .. cost)

-- 返回修改后的用量数据
return {
    prompt_tokens = prompt_tokens,
    completion_tokens = completion_tokens,
    total_tokens = total_tokens,
    billable_tokens = billable_tokens,
    discount = discount,
    cost = cost,
    viplevel = viplevel
}
