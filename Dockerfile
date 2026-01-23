# 构建阶段
FROM golang:1.24-alpine AS builder

# 安装 CA 证书和 git（必须在下载依赖之前）
RUN apk update && apk --no-cache add ca-certificates git

WORKDIR /app

# 复制 go.mod 和 go.sum
COPY go.mod go.sum* ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 编译二进制文件（静态链接，优化体积）
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o llmproxy ./cmd

# 运行阶段（极简镜像）
FROM alpine:latest

# 安装 CA 证书和时区数据，创建非 root 用户
RUN apk --no-cache add ca-certificates tzdata && \
    adduser -D -s /bin/sh -u 1000 llmproxy

WORKDIR /home/llmproxy

# 从构建阶段复制二进制文件
COPY --from=builder /app/llmproxy .

# 复制配置文件示例
COPY config.yaml.example ./config.yaml

# 设置文件权限
RUN chown -R llmproxy:llmproxy . && chmod +x llmproxy

# 切换到非 root 用户
USER llmproxy

# 暴露端口
EXPOSE 8000

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8000/health || exit 1

# 启动命令
ENTRYPOINT ["./llmproxy"]
CMD ["--config", "./config.yaml"]
