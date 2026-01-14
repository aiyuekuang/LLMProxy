# Kubernetes 部署指南

本文档说明如何在 Kubernetes 集群中部署 LLMProxy。

## 📦 基础部署

### 1. 创建 ConfigMap

```yaml
# k8s/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: llmproxy-config
  namespace: default
data:
  config.yaml: |
    listen: ":8080"
    
    backends:
      - url: "http://vllm-service:8000"
        weight: 5
      - url: "http://tgi-service:8081"
        weight: 3
    
    usage_hook:
      enabled: true
      url: "https://billing-service.default.svc.cluster.local/llm-usage"
      timeout: 1s
      retry: 2
    
    health_check:
      interval: 10s
      path: /health
```

### 2. 创建 Deployment

```yaml
# k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: llmproxy
  namespace: default
  labels:
    app: llmproxy
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
        image: ghcr.io/aiyuekuang/llmproxy:v1.0.0
        ports:
        - containerPort: 8080
          name: http
          protocol: TCP
        - containerPort: 9090
          name: metrics
          protocol: TCP
        volumeMounts:
        - name: config
          mountPath: /home/llmproxy/config.yaml
          subPath: config.yaml
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 3
          periodSeconds: 5
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
        securityContext:
          runAsNonRoot: true
          runAsUser: 1000
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
      volumes:
      - name: config
        configMap:
          name: llmproxy-config
```

### 3. 创建 Service

```yaml
# k8s/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: llmproxy
  namespace: default
  labels:
    app: llmproxy
spec:
  type: ClusterIP
  ports:
  - port: 8080
    targetPort: 8080
    protocol: TCP
    name: http
  - port: 9090
    targetPort: 9090
    protocol: TCP
    name: metrics
  selector:
    app: llmproxy
```

### 4. 部署

```bash
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
```

## 🌐 暴露服务

### 方式一：Ingress（推荐）

```yaml
# k8s/ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: llmproxy
  namespace: default
  annotations:
    nginx.ingress.kubernetes.io/proxy-body-size: "10m"
    nginx.ingress.kubernetes.io/proxy-read-timeout: "300"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "300"
spec:
  ingressClassName: nginx
  rules:
  - host: llm.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: llmproxy
            port:
              number: 8080
  tls:
  - hosts:
    - llm.example.com
    secretName: llm-tls
```

### 方式二：LoadBalancer

```yaml
apiVersion: v1
kind: Service
metadata:
  name: llmproxy-lb
spec:
  type: LoadBalancer
  ports:
  - port: 80
    targetPort: 8080
  selector:
    app: llmproxy
```

## 📊 监控集成

### ServiceMonitor（Prometheus Operator）

```yaml
# k8s/servicemonitor.yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: llmproxy
  namespace: default
  labels:
    app: llmproxy
spec:
  selector:
    matchLabels:
      app: llmproxy
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
```

## 🔄 水平扩展

### HorizontalPodAutoscaler

```yaml
# k8s/hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: llmproxy
  namespace: default
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: llmproxy
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

## 🔐 安全加固

### NetworkPolicy

```yaml
# k8s/networkpolicy.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: llmproxy
  namespace: default
spec:
  podSelector:
    matchLabels:
      app: llmproxy
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-nginx
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
  - to:
    - podSelector:
        matchLabels:
          app: billing
    ports:
    - protocol: TCP
      port: 80
```

## 📦 Helm Chart（高级）

### 创建 Chart 结构

```bash
helm create llmproxy-chart
cd llmproxy-chart
```

### values.yaml

```yaml
replicaCount: 3

image:
  repository: ghcr.io/aiyuekuang/llmproxy
  pullPolicy: IfNotPresent
  tag: "v1.0.0"

service:
  type: ClusterIP
  port: 8080
  metricsPort: 9090

ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: llm.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: llm-tls
      hosts:
        - llm.example.com

resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi

autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70

config:
  backends:
    - url: "http://vllm:8000"
      weight: 5
  usageHook:
    enabled: true
    url: "https://billing/usage"
    timeout: 1s
    retry: 2
```

### 安装

```bash
helm install llmproxy ./llmproxy-chart \
  --namespace llm \
  --create-namespace \
  --values custom-values.yaml
```

## 🚀 生产环境最佳实践

### 1. 资源配置

```yaml
resources:
  requests:
    cpu: 500m      # 保证基础性能
    memory: 256Mi
  limits:
    cpu: 2000m     # 防止单 Pod 占用过多资源
    memory: 1Gi
```

### 2. 多副本部署

```yaml
replicas: 3  # 最少 3 个副本，保证高可用
```

### 3. Pod 反亲和性

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

### 4. 优雅关闭

```yaml
lifecycle:
  preStop:
    exec:
      command: ["/bin/sh", "-c", "sleep 15"]
terminationGracePeriodSeconds: 30
```

## 📝 故障排查

### 查看日志

```bash
# 查看所有 Pod 日志
kubectl logs -l app=llmproxy -n default --tail=100

# 实时跟踪
kubectl logs -f deployment/llmproxy -n default
```

### 检查健康状态

```bash
# 查看 Pod 状态
kubectl get pods -l app=llmproxy -n default

# 查看详细信息
kubectl describe pod <pod-name> -n default

# 进入容器调试
kubectl exec -it <pod-name> -n default -- /bin/sh
```

### 测试连接

```bash
# 端口转发到本地
kubectl port-forward svc/llmproxy 8080:8080 -n default

# 测试请求
curl http://localhost:8080/health
```

## 🔄 滚动更新

```bash
# 更新镜像版本
kubectl set image deployment/llmproxy \
  llmproxy=ghcr.io/aiyuekuang/llmproxy:v1.1.0 \
  -n default

# 查看更新状态
kubectl rollout status deployment/llmproxy -n default

# 回滚（如有问题）
kubectl rollout undo deployment/llmproxy -n default
```

## 📚 参考资源

- [Kubernetes 官方文档](https://kubernetes.io/docs/)
- [Helm 文档](https://helm.sh/docs/)
- [Prometheus Operator](https://prometheus-operator.dev/)
