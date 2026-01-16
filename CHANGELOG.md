# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- **[Breaking]** 删除模型相关概念，实现完全透明代理
  - 删除 `allowed_models` 配置项（API Key 不再限制模型访问）
  - 删除 `model_mapping` 配置项（不再做模型名映射）
  - 删除 `per_model` 限流配置（不再支持模型级限流）
  - Webhook 数据结构优化，包含完整请求参数（`request_body`）而不只是 `model` 字段

### Added
- Webhook 现在接收完整的用户请求参数，支持任意自定义字段
- 新增 `request_body`、`status_code`、`latency_ms` 等元数据字段到 Webhook

### Removed
- `APIKey.AllowedModels` 字段
- `RoutingConfig.ModelMapping` 字段
- `RateLimitConfig.PerModel` 字段
- `auth.CheckModelAllowed()` 函数
- `routing.MapModel()` 函数

### Migration Guide
- 如果使用了 `allowed_models`：改为后端服务自己检查权限
- 如果使用了 `model_mapping`：改为后端服务自己处理模型名转换
- 如果使用了 `per_model` 限流：改为使用 Key 级限流（`per_key`）
- Webhook 接收方需要更新代码，从 `request_body` 字段读取用户参数


## [0.2.0] - 2026-01-14

### Added
- GitHub Actions 自动化 Docker 镜像构建和发布
- 多架构支持（linux/amd64, linux/arm64）
- Docker 镜像安全加固（非 root 用户运行）
- 完整的发布文档和检查清单

### Changed
- 优化 Dockerfile，减小镜像体积
- 更新 README，添加官方镜像使用说明

### Security
- 使用非 root 用户运行容器
- 添加健康检查机制

## [0.1.0] - 2026-01-14

### Added
- 初始版本发布
- LLM 协议感知代理
- 零缓冲流式传输
- 多后端负载均衡
- 异步用量计量
- Prometheus 监控指标
- Docker 和 Docker Compose 支持

[Unreleased]: https://github.com/aiyuekuang/LLMProxy/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/aiyuekuang/LLMProxy/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/aiyuekuang/LLMProxy/releases/tag/v0.1.0
