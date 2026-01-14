package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"llmproxy/internal/config"
	"llmproxy/internal/lb"
	"llmproxy/internal/metrics"
	"llmproxy/internal/proxy"
)

func main() {
	// 解析命令行参数
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	log.Printf("LLMProxy 启动中...")
	log.Printf("监听地址: %s", cfg.Listen)
	log.Printf("后端数量: %d", len(cfg.Backends))

	// 创建 HTTP 路由
	mux := http.NewServeMux()

	// 注册 Prometheus metrics 端点
	mux.HandleFunc("/metrics", metrics.Handler)
	log.Println("Prometheus metrics 端点: /metrics")

	// 注册健康检查端点
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	log.Println("健康检查端点: /health")

	// 注册代理处理器
	mux.HandleFunc("/", proxy.NewHandler(cfg))
	log.Println("代理端点: /v1/chat/completions, /v1/completions")

	// 启动负载均衡器健康检查
	if cfg.HealthCheck != nil {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		loadBalancer := lb.NewRoundRobin(cfg.Backends, cfg.HealthCheck)
		go loadBalancer.Start(ctx)
	}

	// 创建 HTTP 服务器
	server := &http.Server{
		Addr:         cfg.Listen,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // 流式响应不设置写超时
		IdleTimeout:  120 * time.Second,
	}

	// 启动服务器（在 goroutine 中）
	go func() {
		log.Printf("LLMProxy 已启动，监听 %s", cfg.Listen)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务器启动失败: %v", err)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在关闭服务器...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("服务器关闭失败: %v", err)
	}

	log.Println("服务器已关闭")
}
