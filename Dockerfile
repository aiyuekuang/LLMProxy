# 构建阶段
FROM golang:1.22 AS builder

WORKDIR /app

# 复制 go.mod 和 go.sum
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 编译二进制文件
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o llmproxy ./cmd

# 运行阶段
FROM alpine:latest

# 安装 CA 证书（用于 HTTPS 请求）
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# 从构建阶段复制二进制文件
COPY --from=builder /app/llmproxy .

# 复制配置文件示例
COPY config.yaml.example /etc/llmproxy/config.yaml

# 暴露端口
EXPOSE 8080

# 启动命令
CMD ["./llmproxy", "--config", "/etc/llmproxy/config.yaml"]
