package metrics

import (
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// requestsTotal 请求总数
	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llmproxy_requests_total",
			Help: "Total number of requests",
		},
		[]string{"path", "stream", "backend", "status"},
	)

	// latencyMs 请求延迟（毫秒）
	latencyMs = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "llmproxy_latency_ms",
			Help:    "Request latency in milliseconds",
			Buckets: []float64{10, 50, 100, 200, 500, 1000, 2000, 5000, 10000},
		},
		[]string{"path", "stream", "backend"},
	)

	// webhookSuccess Webhook 成功数
	webhookSuccess = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "llmproxy_webhook_success_total",
			Help: "Total number of successful webhook calls",
		},
	)

	// webhookFailure Webhook 失败数
	webhookFailure = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "llmproxy_webhook_failure_total",
			Help: "Total number of failed webhook calls",
		},
	)

	// usageTokens Token 使用量
	usageTokens = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llmproxy_usage_tokens_total",
			Help: "Total number of tokens used",
		},
		[]string{"type"}, // type: prompt, completion
	)
)

func init() {
	// 注册所有指标
	prometheus.MustRegister(requestsTotal)
	prometheus.MustRegister(latencyMs)
	prometheus.MustRegister(webhookSuccess)
	prometheus.MustRegister(webhookFailure)
	prometheus.MustRegister(usageTokens)
}

// Handler 返回 Prometheus metrics handler
func Handler(w http.ResponseWriter, r *http.Request) {
	promhttp.Handler().ServeHTTP(w, r)
}

// RecordRequest 记录请求指标
// 参数：
//   - path: 请求路径
//   - isStream: 是否为流式请求
//   - backend: 后端 URL
//   - latency: 请求延迟
//   - statusCode: HTTP 状态码
func RecordRequest(path string, isStream bool, backend string, latency float64, statusCode int) {
	streamStr := strconv.FormatBool(isStream)
	statusStr := strconv.Itoa(statusCode)

	requestsTotal.WithLabelValues(path, streamStr, backend, statusStr).Inc()
	latencyMs.WithLabelValues(path, streamStr, backend).Observe(latency)
}

// RecordUsage 记录 Token 使用量
// 参数：
//   - promptTokens: 输入 token 数
//   - completionTokens: 输出 token 数
func RecordUsage(promptTokens, completionTokens int) {
	usageTokens.WithLabelValues("prompt").Add(float64(promptTokens))
	usageTokens.WithLabelValues("completion").Add(float64(completionTokens))
}

// RecordWebhookSuccess 记录 Webhook 成功
func RecordWebhookSuccess() {
	webhookSuccess.Inc()
}

// RecordWebhookFailure 记录 Webhook 失败
func RecordWebhookFailure() {
	webhookFailure.Inc()
}
