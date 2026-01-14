# LLMProxy 功能扩展分析报告

## 调研背景
基于 GitHub 上主流 LLM 网关项目的调研，分析当前大模型服务场景下的功能扩展方向。

调研时间：2026-01-14  
调研范围：GitHub 上 stars > 500 的 LLM 网关/代理项目

---

## 一、当前 LLMProxy 核心能力

### 已实现功能
1. **流式/非流式代理** - 零缓冲 SSE 传输
2. **负载均衡** - 加权轮询策略
3. **异步用量计量** - Webhook 上报 token 使用量
4. **健康检查** - 自动摘除不健康节点
5. **监控指标** - Prometheus + Grafana

### 架构优势
- 零性能侵入（不解析响应体）
- 极简业务对接（HTTP Webhook）
- 单二进制部署
- 支持 vLLM、TGI 等 OpenAI 兼容后端

---

## 二、市场主流功能扩展方向

### 🔥 高优先级功能（强烈推荐）

#### 1. API Key 管理与鉴权
**参考项目：** One-API、LiteLLM、BricksLLM

**核心功能：**
- 虚拟 API Key 生成与管理
- 基于 Key 的访问控制（RBAC）
- Key 级别的额度限制
- Key 过期时间设置
- IP 白名单/黑名单
- 模型访问权限控制

**业务价值：**
- 多租户场景支持
- 精细化权限管理
- 防止 API Key 滥用
- 支持 SaaS 化部署

**实现建议：**
```yaml
# 配置示例
api_keys:
  - key: "sk-proj-xxx"
    name: "项目A"
    quota: 1000000  # token 额度
    rate_limit: 100  # 每分钟请求数
    allowed_models: ["gpt-4", "claude-3"]
    allowed_ips: ["192.168.1.0/24"]
    expires_at: "2026-12-31"
```

---

#### 2. 限流与速率控制
**参考项目：** Portkey Gateway、BricksLLM、Bifrost

**核心功能：**
- 全局限流（QPS/QPM）
- 用户级限流
- 模型级限流
- Token 级限流（TPM - Tokens Per Minute）
- 并发请求数限制
- 突发流量控制（令牌桶/漏桶算法）

**业务价值：**
- 保护后端服务
- 防止成本失控
- 公平资源分配
- 符合 SLA 要求

**实现建议：**
```yaml
rate_limits:
  global:
    requests_per_minute: 1000
    tokens_per_minute: 100000
  per_key:
    requests_per_minute: 100
    tokens_per_minute: 10000
  per_model:
    gpt-4:
      requests_per_minute: 50
```

---

#### 3. 智能路由与故障转移
**参考项目：** LiteLLM、Portkey Gateway、Bifrost

**核心功能：**
- **模型映射/别名** - 将用户请求的模型映射到实际模型
- **自动重试** - 失败后自动切换后端
- **故障转移** - 主备切换、多区域容灾
- **成本优化路由** - 根据价格选择最优后端
- **延迟优化路由** - 根据响应时间动态调整
- **A/B 测试** - 流量分配到不同模型版本

**业务价值：**
- 提高可用性（99.9%+）
- 降低成本（自动选择便宜的模型）
- 灵活的模型切换
- 支持灰度发布

**实现建议：**
```yaml
routing:
  # 模型映射
  model_mapping:
    "gpt-4": "azure-gpt-4"  # 将 OpenAI 请求路由到 Azure
    "claude-3": "bedrock-claude-3"
  
  # 故障转移
  fallback:
    - primary: "vllm-1"
      fallback: ["vllm-2", "tgi-1"]
      retry: 2
  
  # 成本优化
  cost_routing:
    enabled: true
    prefer_cheaper: true
```

---

#### 4. 响应缓存
**参考项目：** Portkey Gateway、LiteLLM、TensorZero

**核心功能：**
- **语义缓存** - 相似问题命中缓存（基于 embedding）
- **精确缓存** - 完全相同的请求命中缓存
- **TTL 控制** - 缓存过期时间
- **缓存预热** - 预先加载热门查询
- **缓存统计** - 命中率、节省成本

**业务价值：**
- 大幅降低成本（缓存命中率 30-50%）
- 减少延迟（缓存响应 < 10ms）
- 减轻后端压力

**实现建议：**
```yaml
cache:
  enabled: true
  backend: "redis"  # 或 "memory"
  ttl: 3600  # 1小时
  semantic_cache:
    enabled: true
    similarity_threshold: 0.95
    embedding_model: "text-embedding-3-small"
```

---

#### 5. 多模型聚合（统一 API）
**参考项目：** One-API、LiteLLM、Portkey Gateway

**核心功能：**
- 支持 100+ 模型提供商
- 统一 OpenAI 格式 API
- 自动协议转换（OpenAI ↔ Anthropic ↔ Google）
- 支持国内模型（通义千问、文心一言、讯飞星火等）

**业务价值：**
- 一套代码接入所有模型
- 快速切换模型提供商
- 避免供应商锁定

**支持的提供商：**
- OpenAI、Azure OpenAI
- Anthropic Claude
- Google Gemini、Vertex AI
- AWS Bedrock
- 国内：通义千问、文心一言、讯飞星火、智谱 ChatGLM、DeepSeek、字节豆包、腾讯混元、360 智脑

---

### 🌟 中优先级功能（推荐）

#### 6. 成本追踪与预算控制
**参考项目：** One-API、LiteLLM、BricksLLM

**核心功能：**
- 实时成本统计（按用户/项目/模型）
- 预算告警（达到阈值时通知）
- 成本报表（日/周/月）
- 充值与兑换码系统
- 按美元显示费用

**业务价值：**
- 精细化成本管理
- 防止超支
- 支持计费系统对接

---

#### 7. 日志与可观测性增强
**参考项目：** Portkey Gateway、Langfuse、TensorZero

**核心功能：**
- **请求日志** - 完整的请求/响应记录
- **链路追踪** - OpenTelemetry 集成
- **性能分析** - P50/P95/P99 延迟
- **错误追踪** - 错误率、错误类型统计
- **审计日志** - 操作记录、合规要求

**集成方案：**
- Langfuse（LLM 专用可观测平台）
- OpenTelemetry + Jaeger
- ELK Stack
- Grafana Loki

---

#### 8. Prompt 管理与版本控制
**参考项目：** Portkey Gateway、Langfuse

**核心功能：**
- Prompt 模板管理
- 版本控制与回滚
- A/B 测试
- Prompt 性能评估

**业务价值：**
- 统一管理 Prompt
- 快速迭代优化
- 团队协作

---

#### 9. 内容安全与合规
**参考项目：** Portkey Gateway、Caswaf

**核心功能：**
- **PII 脱敏** - 自动移除敏感信息（手机号、身份证、邮箱等）
- **内容审核** - 敏感词过滤
- **Guardrails** - 输入/输出安全检查
- **合规日志** - GDPR、HIPAA 合规

**业务价值：**
- 数据安全
- 合规要求
- 防止数据泄露

---

#### 10. 多租户与用户管理
**参考项目：** One-API、BricksLLM

**核心功能：**
- 用户注册/登录（邮箱、GitHub、微信等）
- 用户分组与权限
- 租户隔离
- 邀请奖励机制

**业务价值：**
- SaaS 化部署
- 多团队协作
- 精细化权限管理

---

### 💡 低优先级功能（可选）

#### 11. 图像/音频/视频支持
- 支持 DALL-E、Stable Diffusion
- 支持 Whisper（语音转文字）
- 支持 TTS（文字转语音）

#### 12. Agent 框架集成
- LangChain、LlamaIndex 集成
- AutoGen、CrewAI 支持
- MCP（Model Context Protocol）网关

#### 13. 批处理任务
- 批量请求处理
- 异步任务队列
- 结果回调

#### 14. Web UI 管理后台
- 可视化配置管理
- 实时监控面板
- 用户自助服务

---

## 三、功能优先级建议

### 第一阶段（核心增强）
1. **API Key 管理与鉴权** ⭐⭐⭐⭐⭐
2. **限流与速率控制** ⭐⭐⭐⭐⭐
3. **智能路由与故障转移** ⭐⭐⭐⭐

**理由：** 这三个功能是生产环境必备，直接影响系统的安全性、稳定性和可用性。

### 第二阶段（成本与性能优化）
4. **响应缓存** ⭐⭐⭐⭐
5. **成本追踪与预算控制** ⭐⭐⭐⭐
6. **多模型聚合** ⭐⭐⭐⭐

**理由：** 显著降低成本，提升性能，增强产品竞争力。

### 第三阶段（企业级功能）
7. **日志与可观测性增强** ⭐⭐⭐
8. **内容安全与合规** ⭐⭐⭐
9. **多租户与用户管理** ⭐⭐⭐

**理由：** 满足企业级需求，支持 SaaS 化部署。

---

## 四、竞品对比

| 功能 | LLMProxy | One-API | LiteLLM | Portkey | BricksLLM |
|------|----------|---------|---------|---------|-----------|
| 流式代理 | ✅ | ✅ | ✅ | ✅ | ✅ |
| 负载均衡 | ✅ | ✅ | ✅ | ✅ | ✅ |
| 用量计量 | ✅ | ✅ | ✅ | ✅ | ✅ |
| API Key 管理 | ❌ | ✅ | ✅ | ✅ | ✅ |
| 限流控制 | ❌ | ✅ | ✅ | ✅ | ✅ |
| 智能路由 | ❌ | ✅ | ✅ | ✅ | ❌ |
| 响应缓存 | ❌ | ❌ | ✅ | ✅ | ❌ |
| 多模型支持 | 部分 | ✅ | ✅ | ✅ | ✅ |
| 成本追踪 | ❌ | ✅ | ✅ | ✅ | ✅ |
| Web UI | ❌ | ✅ | ✅ | ✅ | ❌ |
| 部署复杂度 | 低 | 中 | 中 | 低 | 低 |
| 性能 | 极高 | 中 | 高 | 高 | 高 |

---

## 五、技术实现建议

### 1. API Key 管理
- 使用 JWT 或 UUID 生成虚拟 Key
- Redis 存储 Key 元数据（额度、权限、过期时间）
- 中间件拦截请求进行鉴权

### 2. 限流
- 使用 Redis + Lua 脚本实现分布式限流
- 支持滑动窗口算法
- 令牌桶算法控制突发流量

### 3. 缓存
- Redis 作为缓存后端
- 使用 MD5(request_body) 作为缓存 Key
- 语义缓存需要集成 embedding 模型

### 4. 智能路由
- 配置文件定义路由规则
- 健康检查 + 动态权重调整
- 失败重试 + 指数退避

### 5. 多模型支持
- 抽象统一的 LLM 接口
- 适配器模式转换不同协议
- 参考 LiteLLM 的实现

---

## 六、总结

### 核心建议
1. **优先实现 API Key 管理、限流、智能路由** - 这是生产环境的基础能力
2. **保持架构简洁** - 不要为了功能而牺牲性能优势
3. **模块化设计** - 功能可插拔，用户按需启用
4. **参考 LiteLLM 和 Portkey** - 它们是当前最成熟的开源方案

### 差异化定位
- **极致性能** - 保持零缓冲、低延迟的优势
- **极简部署** - 单二进制、开箱即用
- **专注代理** - 不做复杂的 Prompt 管理、Agent 编排等
- **企业友好** - 提供 Helm Chart、Operator 等企业级部署方案

### 下一步行动
1. 在 `docs/` 中创建各功能的详细设计文档
2. 更新 `.canon/schema.json`，添加新功能模块
3. 与用户讨论优先级，确定开发路线图
4. 分阶段实施，每个阶段交付可用的功能

---

## 附录：参考项目

### 开源项目
- **LiteLLM**: https://github.com/BerriAI/litellm (15k+ stars)
- **One-API**: https://github.com/songquanpeng/one-api (20k+ stars)
- **Portkey Gateway**: https://github.com/Portkey-AI/gateway (7k+ stars)
- **BricksLLM**: https://github.com/bricks-cloud/BricksLLM (600+ stars)
- **Bifrost**: https://github.com/maximhq/bifrost (声称比 LiteLLM 快 50 倍)
- **TensorZero**: https://github.com/tensorzero/tensorzero (LLM 工程平台)

### 商业产品
- **Portkey Cloud**: https://portkey.ai
- **LiteLLM Hosted**: https://litellm.ai
- **AWS Bedrock Gateway**: https://aws.amazon.com/bedrock

---

**文档版本：** v1.0  
**创建时间：** 2026-01-14  
**作者：** Kiro AI Assistant
