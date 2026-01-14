# Docker 镜像发布指南

本文档说明如何将 LLMProxy 发布到 GitHub Container Registry (GHCR)，让全球用户一键使用。

## 📦 自动化发布流程

### 1. 发布新版本

```bash
# 1. 确保代码已提交
git add .
git commit -m "feat: 新功能描述"

# 2. 打语义化版本标签
git tag v1.0.0

# 3. 推送代码和标签
git push origin main
git push origin v1.0.0
```

### 2. 自动构建

推送 tag 后，GitHub Actions 会自动：
- ✅ 构建 `linux/amd64` 和 `linux/arm64` 双架构镜像
- ✅ 推送到 `ghcr.io/aiyuekuang/llmproxy`
- ✅ 生成以下标签：
  - `v1.0.0` - 完整版本号
  - `v1.0` - 次版本号
  - `v1` - 主版本号
  - `latest` - 最新稳定版

### 3. 查看发布结果

访问 GitHub 仓库的 Packages 页面：
```
https://github.com/aiyuekuang/llmproxy/pkgs/container/llmproxy
```

## 🔧 配置说明

### GitHub Actions 工作流

项目包含两个工作流：

1. **`.github/workflows/docker-test.yml`** - PR 和主分支推送时测试构建
2. **`.github/workflows/release.yml`** - Tag 推送时正式发布

### 镜像仓库权限

首次发布后，需要设置镜像为公开：

1. 访问 `https://github.com/aiyuekuang?tab=packages`
2. 点击 `llmproxy` 包
3. 点击右侧 `Package settings`
4. 滚动到底部 `Danger Zone`
5. 点击 `Change visibility` → 选择 `Public`

## 📝 版本管理策略

### 语义化版本 (SemVer)

遵循 `MAJOR.MINOR.PATCH` 格式：

- **MAJOR** (v1.0.0 → v2.0.0) - 不兼容的 API 变更
- **MINOR** (v1.0.0 → v1.1.0) - 向后兼容的新功能
- **PATCH** (v1.0.0 → v1.0.1) - 向后兼容的 Bug 修复

### 标签策略

| 标签 | 说明 | 更新频率 |
|------|------|----------|
| `v1.0.0` | 不可变版本 | 永久保留 |
| `v1.0` | 次版本锁定 | 每次 v1.0.x 发布时更新 |
| `v1` | 主版本锁定 | 每次 v1.x.x 发布时更新 |
| `latest` | 最新稳定版 | 每次发布时更新 |

### 用户使用建议

```bash
# 生产环境：锁定完整版本
docker pull ghcr.io/aiyuekuang/llmproxy:v1.0.0

# 开发环境：使用次版本（自动获取补丁更新）
docker pull ghcr.io/aiyuekuang/llmproxy:v1.0

# 测试最新功能
docker pull ghcr.io/aiyuekuang/llmproxy:latest
```

## 🔒 安全最佳实践

### Dockerfile 安全特性

- ✅ 多阶段构建，最终镜像 < 20MB
- ✅ 使用非 root 用户运行（UID 1000）
- ✅ 静态链接二进制，无外部依赖
- ✅ 包含 CA 证书，支持 HTTPS
- ✅ 内置健康检查

### 镜像扫描（可选）

在 `.github/workflows/release.yml` 中添加 Trivy 扫描：

```yaml
- name: Run Trivy vulnerability scanner
  uses: aquasecurity/trivy-action@master
  with:
    image-ref: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:${{ steps.meta.outputs.version }}
    format: 'sarif'
    output: 'trivy-results.sarif'

- name: Upload Trivy results to GitHub Security
  uses: github/codeql-action/upload-sarif@v2
  with:
    sarif_file: 'trivy-results.sarif'
```

## 🚀 发布到 Docker Hub（可选）

如果希望同时发布到 Docker Hub：

### 1. 创建 Docker Hub Token

1. 访问 https://hub.docker.com/settings/security
2. 点击 `New Access Token`
3. 复制生成的 token

### 2. 添加 GitHub Secrets

在仓库设置中添加：
- `DOCKERHUB_USERNAME` - Docker Hub 用户名
- `DOCKERHUB_TOKEN` - 上一步生成的 token

### 3. 修改 release.yml

在 `Log in to GitHub Container Registry` 步骤后添加：

```yaml
- name: Log in to Docker Hub
  uses: docker/login-action@v3
  with:
    username: ${{ secrets.DOCKERHUB_USERNAME }}
    password: ${{ secrets.DOCKERHUB_TOKEN }}
```

修改 `Extract metadata` 步骤：

```yaml
- name: Extract metadata
  id: meta
  uses: docker/metadata-action@v5
  with:
    images: |
      ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
      docker.io/${{ secrets.DOCKERHUB_USERNAME }}/llmproxy
    tags: |
      type=semver,pattern={{version}}
      type=semver,pattern={{major}}.{{minor}}
      type=semver,pattern={{major}}
      type=raw,value=latest
```

## 📊 监控发布状态

### GitHub Actions 徽章

在 README.md 中添加：

```markdown
[![Docker Build](https://github.com/aiyuekuang/LLMProxy/actions/workflows/release.yml/badge.svg)](https://github.com/aiyuekuang/LLMProxy/actions/workflows/release.yml)
```

### 镜像大小徽章

```markdown
![Docker Image Size](https://ghcr-badge.egpl.dev/aiyuekuang/llmproxy/size?tag=latest)
```

## 🐛 常见问题

### Q: 推送 tag 后没有触发构建？

**A:** 检查：
1. Tag 格式是否为 `v*.*.*`（如 v1.0.0）
2. GitHub Actions 是否启用（仓库 Settings → Actions）
3. 查看 Actions 页面是否有错误日志

### Q: 构建失败提示权限不足？

**A:** 确保仓库设置中：
- Settings → Actions → General
- Workflow permissions 设置为 `Read and write permissions`

### Q: 如何删除已发布的镜像？

**A:** 
1. 访问 Package 页面
2. 点击右侧 `Package settings`
3. 选择要删除的版本
4. 点击 `Delete`

### Q: 如何支持更多架构（如 arm/v7）？

**A:** 修改 `release.yml` 中的 `platforms`：

```yaml
platforms: linux/amd64,linux/arm64,linux/arm/v7
```

## 📚 参考资源

- [GitHub Container Registry 文档](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
- [Docker 多架构构建](https://docs.docker.com/build/building/multi-platform/)
- [语义化版本规范](https://semver.org/lang/zh-CN/)
- [Docker 安全最佳实践](https://docs.docker.com/develop/security-best-practices/)

## 🎯 下一步

发布完成后，建议：

1. ✅ 在 README 中添加 Docker 使用示例
2. ✅ 提交到 awesome-go / awesome-llm 列表
3. ✅ 在 Reddit / Hacker News 分享
4. ✅ 创建 Helm Chart（Kubernetes 用户）
5. ✅ 编写详细的部署文档
