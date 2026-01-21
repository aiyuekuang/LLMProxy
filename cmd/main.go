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

	"llmproxy/internal/database"
	"llmproxy/internal/auth/pipeline"
	"llmproxy/internal/config"
	"llmproxy/internal/lb"
	"llmproxy/internal/metrics"
	"llmproxy/internal/proxy"
	"llmproxy/internal/ratelimit"
	"llmproxy/internal/routing"
	"llmproxy/internal/storage"
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
	log.Printf("监听地址: %s", cfg.GetListen())

	// 初始化存储管理器
	storageManager := storage.NewManager()
	if err := storageManager.Initialize(cfg.Storage); err != nil {
		log.Fatalf("初始化存储管理器失败: %v", err)
	}
	defer storageManager.Close()

	// 初始化数据库 Store（如果启用服务发现）
	var dbStore *database.Store
	if cfg.Discovery != nil && cfg.Discovery.Enabled {
		// 查找数据库类型的发现源
		for _, source := range cfg.Discovery.Sources {
			if source.Type == "database" && source.Enabled && source.Database != nil {
				// 获取数据库连接
				dbConn := storageManager.GetDatabase(source.Database.Storage)
				if dbConn != nil {
					var err error
					dbStore, err = database.NewStoreFromDB(dbConn, source.Database.Table)
					if err != nil {
						log.Fatalf("初始化数据库 Store 失败: %v", err)
					}
					dbStore.StartSync()
					log.Println("数据库服务发现已启用")
					break
				}
			}
		}
	}

	// 合并后端配置（优先使用数据库中的服务）
	allBackends := cfg.Backends
	if dbStore != nil {
		dbBackends := dbStore.GetBackends()
		if len(dbBackends) > 0 {
			allBackends = dbBackends
			log.Printf("使用数据库服务: %d 个", len(dbBackends))
		}
	}
	log.Printf("后端数量: %d", len(allBackends))

	// 创建负载均衡器
	var loadBalancer lb.LoadBalancer
	var backends []*lb.Backend
	
	// 转换后端配置
	for _, b := range allBackends {
		weight := b.Weight
		if weight <= 0 {
			weight = 1
		}
		backends = append(backends, &lb.Backend{
			URL:     b.URL,
			Weight:  weight,
			Healthy: true,
		})
	}
	
	// 根据配置选择负载均衡策略
	strategy := "round_robin"
	if cfg.Routing != nil && cfg.Routing.LoadBalance != "" {
		strategy = cfg.Routing.LoadBalance
	}
	
	switch strategy {
	case "least_connections":
		loadBalancer = lb.NewLeastConnections(allBackends, cfg.HealthCheck)
		log.Println("负载均衡策略: 最少连接数")
	case "latency_based":
		loadBalancer = lb.NewLatencyBased(allBackends, cfg.HealthCheck)
		log.Println("负载均衡策略: 延迟优先")
	default:
		loadBalancer = lb.NewRoundRobin(allBackends, cfg.HealthCheck)
		log.Println("负载均衡策略: 轮询")
	}

	// 创建智能路由器（如果配置了）
	var router *routing.Router
	if cfg.Routing != nil && cfg.Routing.Enabled {
		router = routing.NewRouter(cfg.Routing, loadBalancer, backends)
		log.Println("智能路由已启用")
		if cfg.Routing.Retry != nil && cfg.Routing.Retry.Enabled {
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
		pipelineExecutor, err = pipeline.NewExecutorWithStorage(pipelineCfg, storageManager, nil)
		if err != nil {
			log.Fatalf("创建鉴权管道失败: %v", err)
		}
		log.Println("鉴权管道已启用")
	}

	// 初始化用量上报器（支持多个）
	if cfg.Usage != nil && cfg.Usage.Enabled {
		for _, reporter := range cfg.Usage.Reporters {
			if reporter == nil || !reporter.Enabled {
				continue
			}
			if reporter.Type == "database" && reporter.Database != nil {
				// 获取数据库连接
				dbConn := storageManager.GetDatabase(reporter.Database.Storage)
				if dbConn != nil {
					if err := proxy.InitUsageDatabaseWithConnection(reporter.Name, dbConn, reporter.Database.Table); err != nil {
						log.Fatalf("初始化用量数据库 [%s] 失败: %v", reporter.Name, err)
					}
				} else {
					log.Printf("警告: 用量数据库 [%s] 未找到存储连接: %s", reporter.Name, reporter.Database.Storage)
				}
			} else if reporter.Type == "webhook" && reporter.Webhook != nil {
				log.Printf("用量 Webhook [%s] 已配置: %s", reporter.Name, reporter.Webhook.URL)
			}
		}
	}

	// 创建限流器（如果启用限流）
	var limiter ratelimit.RateLimiter
	if cfg.RateLimit != nil && cfg.RateLimit.Enabled {
		switch cfg.RateLimit.Storage {
		case "memory", "":
			limiter = ratelimit.NewMemoryRateLimiter()
			log.Println("限流已启用: 内存存储")
		case "redis":
			// 从存储管理器获取 Redis 连接
			cacheName := cfg.RateLimit.Redis
			if cacheName == "" {
				cacheName = "default"
			}
			redisClient := storageManager.GetCache(cacheName)
			if redisClient != nil {
				limiter = ratelimit.NewRedisRateLimiter(redisClient, "llmproxy:ratelimit:")
				log.Println("限流已启用: Redis 存储")
			} else {
				log.Printf("警告: Redis 缓存 [%s] 未找到，降级为内存限流", cacheName)
				limiter = ratelimit.NewMemoryRateLimiter()
			}
		default:
			log.Printf("警告: 不支持的限流存储方式: %s, 使用内存限流", cfg.RateLimit.Storage)
			limiter = ratelimit.NewMemoryRateLimiter()
		}
		
		if cfg.RateLimit.Global != nil && cfg.RateLimit.Global.Enabled {
			log.Printf("全局限流: %d req/s", cfg.RateLimit.Global.RequestsPerSecond)
		}
		if cfg.RateLimit.PerKey != nil && cfg.RateLimit.PerKey.Enabled {
			log.Printf("Key 级限流: %d req/s", cfg.RateLimit.PerKey.RequestsPerSecond)
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
	var proxyHandler http.HandlerFunc
	if dbStore != nil {
		proxyHandler = proxy.NewDatabaseHandler(cfg, loadBalancer, router, nil, limiter, dbStore)
		log.Println("使用数据库集成处理器")
	} else {
		proxyHandler = proxy.NewHandler(cfg, loadBalancer, router, nil, limiter)
	}

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
	if cfg.HealthCheck != nil && cfg.HealthCheck.Enabled {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go loadBalancer.Start(ctx)
	}

	// 创建 HTTP 服务器
	serverCfg := &http.Server{
		Addr:         cfg.GetListen(),
		Handler:      mux,
		WriteTimeout: 0, // 流式响应不设置写超时
	}
	if cfg.Server != nil {
		serverCfg.ReadTimeout = cfg.Server.ReadTimeout
		serverCfg.IdleTimeout = cfg.Server.IdleTimeout
	}
	server := serverCfg

	// 启动服务器（在 goroutine 中）
	go func() {
		log.Printf("LLMProxy 已启动，监听 %s", cfg.GetListen())
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
		log.Printf("HTTP 服务器关闭失败: %v", err)
	}

	// 关闭数据库 Store
	if dbStore != nil {
		if err := dbStore.Close(); err != nil {
			log.Printf("数据库 Store 关闭失败: %v", err)
		}
	}

	// 关闭鉴权管道
	if pipelineExecutor != nil {
		if err := pipelineExecutor.Close(); err != nil {
			log.Printf("鉴权管道关闭失败: %v", err)
		}
	}

	log.Println("服务器已关闭")
}
