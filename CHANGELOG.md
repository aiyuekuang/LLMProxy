# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]


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
