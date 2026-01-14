# 智能路由与故障转移设计方案

## 一、功能概述

实现智能路由策略，支持模型映射、自动重试、故障转移、成本优化等高级路由功能。

## 二、核心功能

### 2.1 模型映射
- 将用户请求的模型名映射到实际后端模型
- 支持模型别名（如 gpt-4 -> azure-gpt-4）
- 支持版本映射（如 gpt-4 -> gpt-4-0125-preview）

### 2.2 自动重试
- 请求失败后自动重试
- 指数退避策略
- 可配置重试次数和超时时间

### 2.3 故障转移（Fallback）
- 主后端失败后自动切换到备用后端
- 支持多级 fallback
- 健康检查集成

### 2.4 负载均衡策略扩展
- 加权轮询（已实现）
- 最少连接数
- 响应时间优先
- 成本优先

### 2.5 成本优化路由
- 根据模型价格选择最优后端
- 支持成本阈值设置

## 三、数据结构设计

### 3.1 路由配置
```go
type RoutingConfig struct {
    // 模型映射
    ModelMapping map[string]string `yaml:"model_mapping"`
    
    // 重试配置
    Retry RetryConfig `yaml:"retry"`
    
    // 故障转移配置
    Fallback []FallbackRule `yaml:"fallback"`
    
    // 负载均衡策略
    LoadBalanceStrategy string `yaml:"load_balance_strategy"`
    
    // 成本优化
    CostOptimization CostOptimizationConfig `yaml:"cost_optimization"`
}

type RetryConfig struct {
    Enabled     bool          `yaml:"enabled"`
    MaxRetries  int           `yaml:"max_retries"`
    InitialWait time.Duration `yaml:"initial_wait"`
    MaxWait     time.Duration `yaml:"max_wait"`
    Multiplier  float64       `yaml:"multiplier"`  // 指数退避倍数
}

type FallbackRule struct {
    Primary  string   `yaml:"primary"`   // 主后端
    Fallback []string `yaml:"fallback"`  // 备用后端列表
    Models   []string `yaml:"models"`    // 适用的模型，空表示全部
}

type CostOptimizationConfig struct {
    Enabled       bool               `yaml:"enabled"`
    PreferCheaper bool               `yaml:"prefer_cheaper"`
    ModelPrices   map[string]float64 `yaml:"model_prices"`  // 每 1M tokens 价格
}
```

### 3.2 负载均衡策略接口
```go
type LoadBalancer interface {
    // 选择后端
    Next(model string) (*Backend, error)
    
    // 更新后端健康状态
    UpdateHealth(backend *Backend, healthy bool)
    
    // 记录请求结果（用于统计）
    RecordResult(backend *Backend, latency time.Duration, err error)
}

// 最少连接数策略
type LeastConnectionsBalancer struct {
    backends   []*Backend
    mu         sync.RWMutex
    concurrent map[string]int  // 每个后端的当前并发数
}

// 响应时间优先策略
type LatencyBasedBalancer struct {
    backends []*Backend
    mu       sync.RWMutex
    latency  map[string]time.Duration  // 每个后端的平均延迟
}

// 成本优先策略
type CostBasedBalancer struct {
    backends []*Backend
    mu       sync.RWMutex
    prices   map[string]float64  // 每个后端的价格
}
```

## 四、实现方案

### 4.1 模型映射

```go
func (r *Router) MapModel(requestedModel string) string {
    if mapped, ok := r.config.ModelMapping[requestedModel]; ok {
        log.Infof("Model mapped: %s -> %s", requestedModel, mapped)
        return mapped
    }
    return requestedModel
}
```

### 4.2 自动重试（指数退避）

```go
func (r *Router) ProxyWithRetry(c *gin.Context, backend *Backend) error {
    var lastErr error
    wait := r.config.Retry.InitialWait
    
    for attempt := 0; attempt <= r.config.Retry.MaxRetries; attempt++ {
        if attempt > 0 {
            log.Infof("Retry attempt %d/%d after %v", 
                attempt, r.config.Retry.MaxRetries, wait)
            time.Sleep(wait)
            
            // 指数退避
            wait = time.Duration(float64(wait) * r.config.Retry.Multiplier)
            if wait > r.config.Retry.MaxWait {
                wait = r.config.Retry.MaxWait
            }
        }
        
        err := r.proxyRequest(c, backend)
        if err == nil {
            return nil
        }
        
        lastErr = err
        
        // 判断是否应该重试
        if !shouldRetry(err) {
            return err
        }
    }
    
    return fmt.Errorf("max retries exceeded: %w", lastErr)
}

func shouldRetry(err error) bool {
    // 网络错误、超时、5xx 错误应该重试
    // 4xx 客户端错误不应该重试
    if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
        return true
    }
    
    if httpErr, ok := err.(*HTTPError); ok {
        return httpErr.StatusCode >= 500
    }
    
    return false
}
```

### 4.3 故障转移

```go
func (r *Router) ProxyWithFallback(c *gin.Context, model string) error {
    // 查找适用的 fallback 规则
    rule := r.findFallbackRule(model)
    if rule == nil {
        // 没有 fallback 规则，使用默认负载均衡
        backend, err := r.loadBalancer.Next(model)
        if err != nil {
            return err
        }
        return r.ProxyWithRetry(c, backend)
    }
    
    // 尝试主后端
    primary := r.getBackend(rule.Primary)
    if primary != nil && primary.Healthy {
        err := r.ProxyWithRetry(c, primary)
        if err == nil {
            return nil
        }
        log.Warnf("Primary backend %s failed: %v", rule.Primary, err)
    }
    
    // 尝试备用后端
    for _, fallbackURL := range rule.Fallback {
        backend := r.getBackend(fallbackURL)
        if backend == nil || !backend.Healthy {
            continue
        }
        
        log.Infof("Falling back to %s", fallbackURL)
        err := r.ProxyWithRetry(c, backend)
        if err == nil {
            metrics.RecordFallback(rule.Primary, fallbackURL)
            return nil
        }
        log.Warnf("Fallback backend %s failed: %v", fallbackURL, err)
    }
    
    return fmt.Errorf("all backends failed for model %s", model)
}

func (r *Router) findFallbackRule(model string) *FallbackRule {
    for _, rule := range r.config.Fallback {
        if len(rule.Models) == 0 {
            return &rule  // 适用于所有模型
        }
        for _, m := range rule.Models {
            if m == model {
                return &rule
            }
        }
    }
    return nil
}
```

### 4.4 最少连接数负载均衡

```go
func (lb *LeastConnectionsBalancer) Next(model string) (*Backend, error) {
    lb.mu.RLock()
    defer lb.mu.RUnlock()
    
    var selected *Backend
    minConnections := int(^uint(0) >> 1)  // 最大整数
    
    for _, backend := range lb.backends {
        if !backend.Healthy {
            continue
        }
        
        connections := lb.concurrent[backend.URL]
        if connections < minConnections {
            minConnections = connections
            selected = backend
        }
    }
    
    if selected == nil {
        return nil, fmt.Errorf("no healthy backend available")
    }
    
    // 增加并发计数
    lb.mu.Lock()
    lb.concurrent[selected.URL]++
    lb.mu.Unlock()
    
    return selected, nil
}

func (lb *LeastConnectionsBalancer) RecordResult(backend *Backend, latency time.Duration, err error) {
    lb.mu.Lock()
    defer lb.mu.Unlock()
    
    // 减少并发计数
    if lb.concurrent[backend.URL] > 0 {
        lb.concurrent[backend.URL]--
    }
}
```

### 4.5 响应时间优先负载均衡

```go
func (lb *LatencyBasedBalancer) Next(model string) (*Backend, error) {
    lb.mu.RLock()
    defer lb.mu.RUnlock()
    
    var selected *Backend
    minLatency := time.Duration(1<<63 - 1)  // 最大时间
    
    for _, backend := range lb.backends {
        if !backend.Healthy {
            continue
        }
        
        latency := lb.latency[backend.URL]
        if latency == 0 {
            latency = 100 * time.Millisecond  // 默认延迟
        }
        
        if latency < minLatency {
            minLatency = latency
            selected = backend
        }
    }
    
    if selected == nil {
        return nil, fmt.Errorf("no healthy backend available")
    }
    
    return selected, nil
}

func (lb *LatencyBasedBalancer) RecordResult(backend *Backend, latency time.Duration, err error) {
    lb.mu.Lock()
    defer lb.mu.Unlock()
    
    // 使用指数移动平均（EMA）更新延迟
    alpha := 0.3  // 平滑系数
    oldLatency := lb.latency[backend.URL]
    if oldLatency == 0 {
        lb.latency[backend.URL] = latency
    } else {
        lb.latency[backend.URL] = time.Duration(
            alpha*float64(latency) + (1-alpha)*float64(oldLatency),
        )
    }
}
```

### 4.6 成本优先负载均衡

```go
func (lb *CostBasedBalancer) Next(model string) (*Backend, error) {
    lb.mu.RLock()
    defer lb.mu.RUnlock()
    
    var selected *Backend
    minPrice := float64(1<<63 - 1)
    
    for _, backend := range lb.backends {
        if !backend.Healthy {
            continue
        }
        
        // 获取该后端的模型价格
        price := lb.getPrice(backend, model)
        if price < minPrice {
            minPrice = price
            selected = backend
        }
    }
    
    if selected == nil {
        return nil, fmt.Errorf("no healthy backend available")
    }
    
    return selected, nil
}

func (lb *CostBasedBalancer) getPrice(backend *Backend, model string) float64 {
    key := fmt.Sprintf("%s:%s", backend.URL, model)
    if price, ok := lb.prices[key]; ok {
        return price
    }
    
    // 默认价格
    if price, ok := lb.prices[model]; ok {
        return price
    }
    
    return 1.0  // 默认价格
}
```

## 五、配置示例

```yaml
# config.yaml
routing:
  # 模型映射
  model_mapping:
    "gpt-4": "azure-gpt-4"
    "gpt-4-turbo": "gpt-4-0125-preview"
    "claude-3-opus": "bedrock-claude-3-opus"
  
  # 重试配置
  retry:
    enabled: true
    max_retries: 3
    initial_wait: 1s
    max_wait: 10s
    multiplier: 2.0  # 1s -> 2s -> 4s -> 8s
  
  # 故障转移
  fallback:
    - primary: "http://vllm-1:8000"
      fallback:
        - "http://vllm-2:8000"
        - "http://tgi-1:8081"
      models: ["llama-3", "mistral"]
    
    - primary: "http://openai-proxy:8000"
      fallback:
        - "http://azure-openai:8000"
      models: ["gpt-4", "gpt-3.5-turbo"]
  
  # 负载均衡策略
  load_balance_strategy: "least_connections"  # round_robin, least_connections, latency_based, cost_based
  
  # 成本优化
  cost_optimization:
    enabled: true
    prefer_cheaper: true
    model_prices:
      "gpt-4": 30.0           # $30 per 1M tokens
      "gpt-3.5-turbo": 0.5    # $0.5 per 1M tokens
      "claude-3-opus": 15.0
      "llama-3-70b": 0.8
```

## 六、监控指标

```go
// Prometheus 指标
routing_requests_total{strategy, backend, model}      // 路由请求总数
routing_retries_total{backend, model}                 // 重试次数
routing_fallback_total{primary, fallback, model}      // 故障转移次数
routing_backend_latency_ms{backend, model}            // 后端延迟
routing_backend_errors_total{backend, model, reason}  // 后端错误数
```

## 七、实现优先级

### Phase 1（核心功能）
- [x] 模型映射
- [x] 自动重试（指数退避）
- [x] 故障转移（Fallback）

### Phase 2（负载均衡扩展）
- [ ] 最少连接数策略
- [ ] 响应时间优先策略

### Phase 3（成本优化）
- [ ] 成本优先策略
- [ ] 成本统计与报表

---

**设计版本：** v1.0  
**创建时间：** 2026-01-14
