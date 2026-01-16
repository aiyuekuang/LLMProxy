-- 路由决策脚本示例

-- 获取请求参数
local viplevel = request.body.viplevel or 0
local user_id = request.user_id
local model = request.body.model or ""

-- 处理嵌套数据
if request.body.user and request.body.user.profile then
    viplevel = request.body.user.profile.viplevel or viplevel
end

-- 记录日志
log.info("路由决策: user_id=" .. user_id .. ", viplevel=" .. viplevel .. ", model=" .. model)

-- 路由逻辑
if viplevel >= 3 then
    -- VIP 3 及以上：超高性能后端
    return "ultra"
elseif viplevel >= 1 then
    -- VIP 1-2：高性能后端
    return "high-performance"
elseif model == "gpt-4" then
    -- GPT-4 模型：高性能后端
    return "high-performance"
else
    -- 普通用户：标准后端
    return "standard"
end
