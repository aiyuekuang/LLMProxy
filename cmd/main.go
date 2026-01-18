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

	"llmproxy/internal/auth/pipeline"
	"llmproxy/internal/config"
	"llmproxy/internal/lb"
	"llmproxy/internal/metrics"
	"llmproxy/internal/proxy"
	"llmproxy/internal/ratelimit"
	"llmproxy/internal/routing"
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

	// 创建负载均衡器
	var loadBalancer lb.LoadBalancer
	var backends []*lb.Backend
	
	// 转换后端配置
	for _, b := range cfg.Backends {
		weight := b.Weight
		if weight <= 0 {
			weight = 1
		}
		backends = append(backends, &lb.Backend{
			URL:     b.URL,
			Weight:  weight,
			Healthy: true,
			Models:  b.Models,
		})
	}
	
	// 根据配置选择负载均衡策略
	strategy := "round_robin"
	if cfg.Routing != nil && cfg.Routing.LoadBalanceStrategy != "" {
		strategy = cfg.Routing.LoadBalanceStrategy
	}
	
	switch strategy {
	case "least_connections":
		loadBalancer = lb.NewLeastConnections(cfg.Backends, cfg.HealthCheck)
		log.Println("负载均衡策略: 最少连接数")
	case "latency_based":
		loadBalancer = lb.NewLatencyBased(cfg.Backends, cfg.HealthCheck)
		log.Println("负载均衡策略: 延迟优先")
	default:
		loadBalancer = lb.NewRoundRobin(cfg.Backends, cfg.HealthCheck)
		log.Println("负载均衡策略: 轮询")
	}

	// 创建智能路由器（如果配置了）
	var router *routing.Router
	if cfg.Routing != nil {
		router = routing.NewRouter(cfg.Routing, loadBalancer, backends)
		log.Println("智能路由已启用")
		if cfg.Routing.Retry.Enabled {
			log.Printf("自动重试已启用: 最大 %d 次", cfg.Routing.Retry.MaxRetries)
		}
		if len(cfg.Routing.Fallback) > 0 {
			log.Printf("故障转移规则: %d 个", len(cfg.Routing.Fallback))
		}
	}

	// 创建鉴权管道执行器（如果启用鉴权）
	var pipelineExecutor *pipeline.Executor
	
	if cfg.Auth != nil && cfg.Auth.Enabled {
		pipelineCfg := pipeline.FromConfig(cfg.Auth)
		var err error
		pipelineExecutor, err = pipeline.NewExecutor(pipelineCfg, cfg.APIKeys)
		if err != nil {
			log.Fatalf("创建鉴权管道失败: %v", err)
		}
		log.Println("鉴权管道已启用")
	}

	// 创建限流器（如果启用限流）
	var limiter ratelimit.RateLimiter
	if cfg.RateLimit != nil && cfg.RateLimit.Enabled {
		if cfg.RateLimit.Storage == "memory" {
			limiter = ratelimit.NewMemoryRateLimiter()
			log.Println("限流已启用: 内存存储")
			if cfg.RateLimit.Global != nil && cfg.RateLimit.Global.Enabled {
				log.Printf("全局限流: %d req/s", cfg.RateLimit.Global.RequestsPerSecond)
			}
			if cfg.RateLimit.PerKey != nil && cfg.RateLimit.PerKey.Enabled {
				log.Printf("Key 级限流: %d req/s", cfg.RateLimit.PerKey.RequestsPerSecond)
			}
		} else {
			log.Printf("警告: 不支持的限流存储方式: %s, 限流功能未启用", cfg.RateLimit.Storage)
		}
	}

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

	// 创建代理处理器
	proxyHandler := proxy.NewHandler(cfg, loadBalancer, router, nil, limiter)

	// 应用中间件链
	var handler http.HandlerFunc = proxyHandler
	
	// 限流中间件（最外层）
	if limiter != nil && cfg.RateLimit != nil && cfg.RateLimit.Enabled {
		handler = ratelimit.Middleware(limiter, cfg.RateLimit, handler)
	}
	
	// 鉴权中间件
	if pipelineExecutor != nil {
		handler = pipeline.Middleware(pipelineExecutor, handler)
	}

	// 注册代理处理器
	mux.HandleFunc("/", handler)
	log.Println("代理端点: /v1/chat/completions, /v1/completions")

	// 启动负载均衡器健康检查
	if cfg.HealthCheck != nil {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

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
