# LLMProxy 部署指南

本文档详细说明 LLMProxy 的各种部署方式。

## 目录

1. [本地开发部署](#本地开发部署)
2. [Docker 部署](#docker-部署)
3. [Docker Compose 部署](#docker-compose-部署)
4. [Kubernetes 部署](#kubernetes-部署)
5. [生产环境最佳实践](#生产环境最佳实践)

---

## 本地开发部署

### 前置要求

- Go 1.22+
- 后端服务（vLLM 或 TGI）

### 步骤

1. **克隆项目**

```bash
git clone https://github.com/your-org/llmproxy.git
cd llmproxy
```

2. **安装依赖**

```bash
go mod download
```

3. **配置文件**

```bash
cp config.yaml.example config.yaml
vim config.yaml
```

修改后端地址：

```yaml
backends:
  - url: "http://localhost:8000"  # 你的 vLLM 地址
    weight: 5
```

4. **运行**

```bash
go run cmd/main.go --config config.yaml
```

或编译后运行：

```bash
make build
./llmproxy --config config.yaml
```

5. **测试**

```bash
curl http://localhost:8080/health
```

---

## Docker 部署

### 构建镜像

```bash
docker build -t llmproxy:latest .
```

### 运行容器

```bash
docker run -d \
  --name llmproxy \
  -p 8080:8080 \
  -v $(pwd)/config.yaml:/etc/llmproxy/config.yaml \
  llmproxy:latest
```

### 查看日志

```bash
docker logs -f llmproxy
```

### 停止容器

```bash
docker stop llmproxy
docker rm llmproxy
```

---

## Docker Compose 部署

适合本地测试和小规模部署。

### 启动服务

```bash
cd deployments
docker compose up -d
```

这将启动：
- vLLM（端口 8000）
- LLMProxy（端口 8080）
- Prometheus（端口 9090）
- Grafana（端口 3000）

### 访问服务

- LLMProxy: http://localhost:8080
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000（用户名/密码：admin/admin）

### 查看日志

```bash
docker compose logs -f llmproxy
```

### 停止服务

```bash
docker compose down
```

---

## Kubernetes 部署

### 使用 Helm Chart

1. **添加 Helm 仓库**（如果已发布）

```bash
helm repo add llmproxy https://your-org.github.io/llmproxy
helm repo update
```

2. **创建配置文件**

创建 `values.yaml`：

```yaml
replicaCount: 3

image:
  repository: your-registry/llmproxy
  tag: v1.0.0

config:
  backends:
    - url: "http://vllm-service:8000"
      weight: 5
  usage_hook:
    enabled: true
    url: "https://billing.yourcompany.com/llm-usage"
    timeout: 1s
    retry: 2

service:
  type: ClusterIP
  port: 8080

ingress:
  enabled: true
  className: nginx
  hosts:
    - host: llmproxy.example.com
      paths:
        - path: /
          pathType: Prefix

resources:
  limits:
    cpu: 1000m
    memory: 512Mi
  requests:
    cpu: 500m
    memory: 256Mi

autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 10
  targetCPUUtilizationPercentage: 80
```

3. **部署**

```bash
helm install llmproxy llmproxy/llmproxy -f values.yaml
```

4. **升级**

```bash
helm upgrade llmproxy llmproxy/llmproxy -f values.yaml
```

5. **卸载**

```bash
helm uninstall llmproxy
```

### 手动部署（不使用 Helm）

1. **创建 ConfigMap**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: llmproxy-config
data:
  config.yaml: |
    listen: ":8080"
    backends:
      - url: "http://vllm-service:8000"
        weight: 5
    usage_hook:
      enabled: true
      url: "https://billing.yourcompany.com/llm-usage"
      timeout: 1s
      retry: 2
    health_check:
      interval: 10s
      path: /health
```

2. **创建 Deployment**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: llmproxy
spec:
  replicas: 3
  selector:
    matchLabels:
      app: llmproxy
  template:
    metadata:
      labels:
        app: llmproxy
    spec:
      containers:
      - name: llmproxy
        image: your-registry/llmproxy:v1.0.0
        ports:
        - containerPort: 8080
        volumeMounts:
        - name: config
          mountPath: /etc/llmproxy
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          limits:
            cpu: 1000m
            memory: 512Mi
          requests:
            cpu: 500m
            memory: 256Mi
      volumes:
      - name: config
        configMap:
          name: llmproxy-config
```

3. **创建 Service**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: llmproxy
spec:
  selector:
    app: llmproxy
  ports:
  - port: 8080
    targetPort: 8080
  type: ClusterIP
```

4. **创建 Ingress**

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: llmproxy
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  ingressClassName: nginx
  rules:
  - host: llmproxy.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: llmproxy
            port:
              number: 8080
```

5. **应用配置**

```bash
kubectl apply -f configmap.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
kubectl apply -f ingress.yaml
```

---

## 生产环境最佳实践

### 1. 高可用部署

- **多副本**：至少 3 个副本
- **反亲和性**：确保 Pod 分布在不同节点

```yaml
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
    - weight: 100
      podAffinityTerm:
        labelSelector:
          matchExpressions:
          - key: app
            operator: In
            values:
            - llmproxy
        topologyKey: kubernetes.io/hostname
```

### 2. 资源限制

根据实际负载调整：

```yaml
resources:
  limits:
    cpu: 2000m
    memory: 1Gi
  requests:
    cpu: 1000m
    memory: 512Mi
```

### 3. 自动扩缩容

基于 CPU 或自定义指标：

```yaml
autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 20
  targetCPUUtilizationPercentage: 70
```

### 4. 监控告警

配置 Prometheus 告警规则：

```yaml
groups:
- name: llmproxy
  rules:
  - alert: LLMProxyHighErrorRate
    expr: rate(llmproxy_requests_total{status=~"5.."}[5m]) > 0.05
    for: 5m
    annotations:
      summary: "LLMProxy 错误率过高"
  
  - alert: LLMProxyHighLatency
    expr: histogram_quantile(0.99, rate(llmproxy_latency_ms_bucket[5m])) > 5000
    for: 5m
    annotations:
      summary: "LLMProxy P99 延迟过高"
```

### 5. 日志收集

使用 Fluentd 或 Filebeat 收集日志：

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: fluentd-config
data:
  fluent.conf: |
    <source>
      @type tail
      path /var/log/containers/llmproxy-*.log
      pos_file /var/log/fluentd-llmproxy.pos
      tag llmproxy
      <parse>
        @type json
      </parse>
    </source>
    
    <match llmproxy>
      @type elasticsearch
      host elasticsearch.logging.svc.cluster.local
      port 9200
      index_name llmproxy
    </match>
```

### 6. 安全加固

- **TLS 终止**：在 Ingress 层配置 HTTPS
- **网络策略**：限制 Pod 间通信
- **Secret 管理**：使用 Kubernetes Secret 存储敏感配置

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: llmproxy-netpol
spec:
  podSelector:
    matchLabels:
      app: llmproxy
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app: ingress-nginx
    ports:
    - protocol: TCP
      port: 8080
  egress:
  - to:
    - podSelector:
        matchLabels:
          app: vllm
    ports:
    - protocol: TCP
      port: 8000
```

### 7. 备份与恢复

定期备份配置：

```bash
kubectl get configmap llmproxy-config -o yaml > backup/configmap-$(date +%Y%m%d).yaml
```

---

## 故障排查

### 查看日志

```bash
# Docker
docker logs -f llmproxy

# Kubernetes
kubectl logs -f deployment/llmproxy
```

### 检查健康状态

```bash
curl http://localhost:8080/health
```

### 查看监控指标

```bash
curl http://localhost:8080/metrics
```

### 常见问题

1. **后端连接失败**
   - 检查后端服务是否正常
   - 检查网络连通性
   - 查看健康检查日志

2. **Webhook 发送失败**
   - 检查 Webhook URL 是否可访问
   - 检查超时配置
   - 查看重试日志

3. **高延迟**
   - 检查后端性能
   - 增加副本数
   - 优化负载均衡策略

---

## 性能调优

### 1. Go 运行时参数

```bash
GOMAXPROCS=8 ./llmproxy --config config.yaml
```

### 2. HTTP 客户端优化

在 `handler.go` 中调整连接池：

```go
Transport: &http.Transport{
    MaxIdleConns:        200,
    MaxIdleConnsPerHost: 20,
    IdleConnTimeout:     90 * time.Second,
}
```

### 3. 负载均衡策略

根据实际情况选择：
- 轮询：适合后端性能均衡
- 加权轮询：适合后端性能不均
- 最少连接：适合长连接场景

---

## 联系支持

如有问题，请：
1. 查看 [GitHub Issues](https://github.com/your-org/llmproxy/issues)
2. 提交新 Issue
3. 联系技术支持
