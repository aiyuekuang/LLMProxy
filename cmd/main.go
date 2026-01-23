package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"llmproxy/internal/admin"
	"llmproxy/internal/auth/pipeline"
	"llmproxy/internal/config"
	"llmproxy/internal/database"
	"llmproxy/internal/hooks"
	"llmproxy/internal/lb"
	"llmproxy/internal/metrics"
	"llmproxy/internal/middleware"
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
	defer func() {
		if err := storageManager.Close(); err != nil {
			log.Printf("关闭存储管理器失败: %v", err)
		}
	}()

	// 初始化数据库 Store（如果启用服务发现）
	var dbStore *database.Store
	if cfg.Discovery != nil && cfg.Discovery.Enabled {
		// 查找数据库类型的发现源
		for _, source := range cfg.Discovery.Sources {
			if source.Type == "database" && source.Enabled && source.Database != nil {
				// 获取数据库连接
				dbConn := storageManager.GetDatabase(source.Database.Storage)
				if dbConn != nil {
					// 获取驱动类型
					driver := ""
					if dbCfg := cfg.Storage.GetDatabase(source.Database.Storage); dbCfg != nil {
						driver = dbCfg.Driver
					}
					var err error
					dbStore, err = database.NewStoreFromDBWithDriver(dbConn, source.Database.Table, driver)
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

	// 后端配置：优先使用数据库中的服务（如果有），否则使用配置文件中的 backends
	allBackends := cfg.Backends
	if dbStore != nil {
		dbBackends := dbStore.GetBackends()
		if len(dbBackends) > 0 {
			allBackends = dbBackends
			log.Printf("使用数据库服务发现: %d 个后端", len(dbBackends))
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
	case "weighted":
		loadBalancer = lb.NewWeighted(allBackends, cfg.HealthCheck)
		log.Println("负载均衡策略: 加权轮询")
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

	// 初始化 Admin API（如果启用）
	var keyStore *admin.KeyStore
	var adminServer *admin.Server

	if cfg.Admin != nil && cfg.Admin.Enabled {
		// 确定数据库路径
		dbPath := cfg.Admin.DBPath
		if dbPath == "" {
			dbPath = "./data/keys.db"
		}

		// 创建 KeyStore
		var err error
		keyStore, err = admin.NewKeyStore(dbPath)
		if err != nil {
			log.Fatalf("初始化 KeyStore 失败: %v", err)
		}
		log.Printf("KeyStore 已初始化: %s", dbPath)

		// 创建 Admin Server
		if cfg.Admin.Token != "" {
			listen := cfg.Admin.Listen
			if listen == "" {
				// 未指定单独端口，后续将挂载到主服务器
				adminServer = admin.NewServer(keyStore, cfg.Admin.Token, "")
				log.Println("Admin API 将挂载到主服务器")
			} else {
				adminServer = admin.NewServer(keyStore, cfg.Admin.Token, listen)
				go func() {
					if err := adminServer.Start(); err != nil && err != http.ErrServerClosed {
						log.Printf("Admin API 服务器启动失败: %v", err)
					}
				}()
				log.Printf("Admin API 已启动: %s", listen)
			}
		} else {
			log.Println("警告: Admin API 已启用但未配置 token，Admin API 服务器未启动")
		}
	}

	// 创建鉴权管道执行器（如果启用鉴权）
	var pipelineExecutor *pipeline.Executor

	if cfg.Auth != nil && cfg.Auth.Enabled {
		pipelineCfg := pipeline.FromConfig(cfg.Auth)
		var err error
		pipelineExecutor, err = pipeline.NewExecutorWithStorage(pipelineCfg, storageManager, nil, keyStore, cfg.Auth.StatusCodes)
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
			switch reporter.Type {
			case "database":
				if reporter.Database != nil && reporter.Database.Storage != "" {
					// 获取数据库连接
					dbConn := storageManager.GetDatabase(reporter.Database.Storage)
					if dbConn != nil {
						// 获取驱动类型
						driver := ""
						if dbCfg := cfg.Storage.GetDatabase(reporter.Database.Storage); dbCfg != nil {
							driver = dbCfg.Driver
						}
						if err := proxy.InitUsageDatabaseWithConnection(reporter.Name, dbConn, driver, reporter.Database.Table); err != nil {
							log.Fatalf("初始化用量数据库 [%s] 失败: %v", reporter.Name, err)
						}
					} else {
						log.Printf("警告: 用量数据库 [%s] 未找到存储连接: %s", reporter.Name, reporter.Database.Storage)
					}
				}
			case "webhook":
				if reporter.Webhook != nil {
					log.Printf("用量 Webhook [%s] 已配置: %s", reporter.Name, reporter.Webhook.URL)
				}
			case "builtin":
				// 内置用量存储（使用 admin 的 KeyStore 数据库）
				if keyStore != nil {
					retentionDays := 0
					if reporter.Builtin != nil {
						retentionDays = reporter.Builtin.RetentionDays
					}
					usageStore, err := admin.NewUsageStore(keyStore.GetDB(), retentionDays)
					if err != nil {
						log.Printf("警告: 初始化内置用量存储失败: %v", err)
					} else {
						proxy.InitBuiltinUsage(usageStore)
						log.Printf("内置用量存储 [%s] 已启用 (保留: %d 天)", reporter.Name, retentionDays)
					}
				} else {
					log.Printf("警告: 内置用量存储需要启用 Admin 模块")
				}
			}
		}
	}

	// 初始化请求日志记录器（如果启用）
	var logger *proxy.Logger
	if cfg.Logging != nil && cfg.Logging.Enabled {
		var dbConn *sql.DB
		var driver string

		// 获取请求日志的数据库连接
		if cfg.Logging.Request != nil && cfg.Logging.Request.Enabled && cfg.Logging.Request.Storage != "" {
			dbConn = storageManager.GetDatabase(cfg.Logging.Request.Storage)
			if dbCfg := cfg.Storage.GetDatabase(cfg.Logging.Request.Storage); dbCfg != nil {
				driver = dbCfg.Driver
			}
			if dbConn == nil {
				log.Printf("警告: 请求日志数据库 [%s] 未找到", cfg.Logging.Request.Storage)
			}
		}

		var err error
		logger, err = proxy.NewLogger(cfg.Logging, dbConn, driver)
		if err != nil {
			log.Fatalf("初始化请求日志记录器失败: %v", err)
		}
		log.Println("请求日志记录器已启用")
	}

	// 初始化 Hooks 执行器（如果启用）
	var hooksExecutor *hooks.Executor
	if cfg.Hooks != nil && cfg.Hooks.Enabled {
		var err error
		hooksExecutor, err = hooks.NewExecutor(cfg.Hooks)
		if err != nil {
			log.Fatalf("初始化 Hooks 执行器失败: %v", err)
		}
		log.Println("Hooks 执行器已启用")
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
		_, _ = w.Write([]byte("OK"))
	})
	log.Println("健康检查端点: /health")

	// 注册 Admin API 路由（如果未配置单独端口）
	if adminServer != nil && (cfg.Admin == nil || cfg.Admin.Listen == "") {
		adminServer.RegisterRoutes(mux)
	}

	// 创建代理处理器
	var proxyHandler http.HandlerFunc
	if dbStore != nil {
		proxyHandler = proxy.NewDatabaseHandler(cfg, loadBalancer, router, nil, limiter, dbStore)
		log.Println("使用数据库集成处理器")
	} else {
		proxyHandler = proxy.NewHandlerWithOptions(&proxy.HandlerOptions{
			Config:       cfg,
			LoadBalancer: loadBalancer,
			Router:       router,
			KeyStore:     nil,
			Limiter:      limiter,
			Logger:       logger,
			Hooks:        hooksExecutor,
		})
	}

	// 应用中间件链
	handler := proxyHandler

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

	// 应用 CORS 中间件（如果启用）
	var finalHandler http.Handler = mux
	if cfg.Server != nil && cfg.Server.CORS != nil && cfg.Server.CORS.Enabled {
		finalHandler = middleware.CORSMiddleware(cfg.Server.CORS, mux)
		log.Println("CORS 已启用")
	}

	// 创建 HTTP 服务器
	server := &http.Server{
		Addr:         cfg.GetListen(),
		Handler:      finalHandler,
		WriteTimeout: 0, // 流式响应不设置写超时（忽略配置中的 write_timeout）
	}
	if cfg.Server != nil {
		server.ReadTimeout = cfg.Server.ReadTimeout
		server.IdleTimeout = cfg.Server.IdleTimeout
		server.MaxHeaderBytes = cfg.Server.MaxHeaderBytes
	}

	// 配置 TLS（如果启用）
	tlsEnabled := cfg.Server != nil && cfg.Server.TLS != nil && cfg.Server.TLS.Enabled
	if tlsEnabled {
		tlsCfg := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		server.TLSConfig = tlsCfg
	}

	// 启动服务器（在 goroutine 中）
	go func() {
		if tlsEnabled {
			log.Printf("LLMProxy 已启动 (HTTPS)，监听 %s", cfg.GetListen())
			if err := server.ListenAndServeTLS(cfg.Server.TLS.CertFile, cfg.Server.TLS.KeyFile); err != nil && err != http.ErrServerClosed {
				log.Fatalf("服务器启动失败: %v", err)
			}
		} else {
			log.Printf("LLMProxy 已启动，监听 %s", cfg.GetListen())
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("服务器启动失败: %v", err)
			}
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

	// 关闭 Admin Server
	if adminServer != nil {
		if err := adminServer.Stop(); err != nil {
			log.Printf("Admin API 服务器关闭失败: %v", err)
		}
	}

	// 关闭 KeyStore
	if keyStore != nil {
		if err := keyStore.Close(); err != nil {
			log.Printf("KeyStore 关闭失败: %v", err)
		}
	}

	// 关闭 Logger
	if logger != nil {
		if err := logger.Close(); err != nil {
			log.Printf("Logger 关闭失败: %v", err)
		}
	}

	// 关闭 Hooks 执行器
	if hooksExecutor != nil {
		if err := hooksExecutor.Close(); err != nil {
			log.Printf("Hooks 执行器关闭失败: %v", err)
		}
	}

	log.Println("服务器已关闭")
}
