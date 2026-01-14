.PHONY: build run test clean docker-build docker-run

# 编译
build:
	go build -o llmproxy ./cmd

# 运行
run:
	go run ./cmd/main.go --config config.yaml

# 测试
test:
	go test -v ./...

# 清理
clean:
	rm -f llmproxy

# 构建 Docker 镜像
docker-build:
	docker build -t llmproxy:latest .

# 运行 Docker 容器
docker-run:
	docker run -d -p 8000:8000 -v $(PWD)/config.yaml:/etc/llmproxy/config.yaml llmproxy:latest

# 启动 Docker Compose
compose-up:
	cd deployments && docker compose up -d

# 停止 Docker Compose
compose-down:
	cd deployments && docker compose down

# 查看日志
logs:
	cd deployments && docker compose logs -f llmproxy

# 下载依赖
deps:
	go mod download

# 格式化代码
fmt:
	go fmt ./...

# 代码检查
lint:
	golangci-lint run
