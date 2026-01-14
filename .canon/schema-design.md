# Schema 设计规范

## 目的
本文档定义 `.canon/schema.json` 的编写规范，确保设计文档的一致性和可执行性。

## 基本原则
1. **唯一权威来源**：schema.json 是项目功能结构的唯一权威来源
2. **可执行性**：文档必须足够详细，可直接指导开发
3. **结构化**：使用 JSON 格式，便于程序解析
4. **完整性**：覆盖所有模块、功能、配置、部署方案

## 结构规范

### 1. 项目元信息（project）
必需字段：
- `name`：项目名称
- `version`：版本号
- `description`：项目描述
- `language`：主要编程语言
- `architecture`：架构类型（单体/微服务/分层等）

### 2. 模块定义（modules）
每个模块必须包含：
- `name`：模块名称
- `path`：模块路径
- `documentation`：详细文档说明（功能、职责、实现要点）
- `dependencies`：依赖的其他模块（可选）
- `files`：模块内的文件列表及说明（可选）

### 3. 功能开关（features）
每个功能必须包含：
- `enabled`：是否启用（true/false）
- `documentation`：功能说明、实现方式、注意事项

### 4. 配置规范（configuration）
必须包含：
- `format`：配置文件格式（YAML/JSON/TOML）
- `example`：配置示例

### 5. 部署方案（deployment）
必须包含：
- 各部署方式的 `documentation`
- 关键配置说明

### 6. 监控日志（monitoring/logging）
必须包含：
- 监控指标定义
- 日志格式和级别

## 编写要求

### documentation 字段编写规范
1. **清晰明确**：说明模块/功能的用途和职责
2. **实现要点**：列出关键实现细节（算法、技术选型、注意事项）
3. **分点列举**：使用数字列表，便于阅读
4. **技术细节**：包含必要的技术参数（如超时时间、重试次数）

### 示例（好的 documentation）
```json
"documentation": "负载均衡模块，支持多种策略。当前实现轮询（Round Robin）算法。功能：1. 维护后端列表 2. 根据权重选择后端 3. 健康检查（定期探测后端 /health 接口）4. 自动摘除不健康节点 5. 线程安全（使用 sync.Mutex）"
```

### 反例（不好的 documentation）
```json
"documentation": "负载均衡模块"  // ❌ 过于简单，无法指导开发
```

## 更新流程
1. 需求变更时，首先更新 schema.json
2. 在 `docs/` 中创建方案讨论文档
3. 确认方案后，更新 schema.json 的 documentation 字段
4. 基于 schema.json 进行开发

## 版本管理
- schema.json 纳入版本控制
- 重大变更需更新 project.version
- 保持与代码实现同步
