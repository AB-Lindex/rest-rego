package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/AB-Lindex/rest-rego/internal/types"
	"github.com/ninlil/butler/bufferedresponse"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var metrics struct {
	reg     *prometheus.Registry
	buckets []float64

	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	requestSize     *prometheus.SummaryVec
	responseSize    *prometheus.SummaryVec

	blockedHeadersExposed      prometheus.Gauge
	blockedHeadersCaptured     prometheus.Counter
	requestsWithBlockedHeaders prometheus.Counter
}

// New creates a new instance of the metrics
func New() {
	metrics.reg = prometheus.NewRegistry()

	metrics.reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	metrics.buckets = prometheus.ExponentialBuckets(0.1, 1.5, 5)

	labels := []string{"method", "code", "url"}

	metrics.requestsTotal = promauto.With(metrics.reg).NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Tracks the number of HTTP requests.",
		},
		labels,
	)
	metrics.requestDuration = promauto.With(metrics.reg).NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Tracks the latencies for HTTP requests.",
			Buckets: metrics.buckets,
		},
		labels,
	)
	metrics.requestSize = promauto.With(metrics.reg).NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "http_request_size_bytes",
			Help: "Tracks the size of HTTP requests.",
		},
		labels,
	)
	metrics.responseSize = promauto.With(metrics.reg).NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "http_response_size_bytes",
			Help: "Tracks the size of HTTP responses.",
		},
		labels,
	)

	metrics.blockedHeadersExposed = promauto.With(metrics.reg).NewGauge(
		prometheus.GaugeOpts{
			Name: "restrego_blocked_headers_exposed",
			Help: "Indicates if blocked headers feature is enabled (1) or disabled (0).",
		},
	)

	metrics.blockedHeadersCaptured = promauto.With(metrics.reg).NewCounter(
		prometheus.CounterOpts{
			Name: "restrego_blocked_headers_captured_total",
			Help: "Total number of X-Restrego-* headers captured.",
		},
	)

	metrics.requestsWithBlockedHeaders = promauto.With(metrics.reg).NewCounter(
		prometheus.CounterOpts{
			Name: "restrego_requests_with_blocked_headers_total",
			Help: "Total number of requests containing X-Restrego-* headers.",
		},
	)
}

// Handler returns the metrics handler for the /metrics endpoint
func Handler() http.HandlerFunc {
	return promhttp.HandlerFor(metrics.reg, promhttp.HandlerOpts{}).ServeHTTP
}

// Wrap wraps a handler for metrics-collection
func Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w2, ok := w.(*bufferedresponse.ResponseWriter)
		if !ok {
			panic("metrics: bufferedresponse.ResponseWriter expected")
		}

		now := time.Now()

		next.ServeHTTP(w2, r)

		info := types.GetInfo(r)

		labels := make([]string, 3)
		labels[0] = r.Method
		labels[1] = strconv.Itoa(w2.Status())
		labels[2] = info.URL

		metrics.requestDuration.WithLabelValues(labels...).Observe(time.Since(now).Seconds())
		metrics.requestSize.WithLabelValues(labels...).Observe(float64(r.ContentLength))
		metrics.responseSize.WithLabelValues(labels...).Observe(float64(w2.Size()))
		metrics.requestsTotal.WithLabelValues(labels...).Inc()
	})
}

// SetBlockedHeadersExposed sets the gauge value indicating if blocked headers feature is enabled
func SetBlockedHeadersExposed(enabled bool) {
	if enabled {
		metrics.blockedHeadersExposed.Set(1)
	} else {
		metrics.blockedHeadersExposed.Set(0)
	}
}

// IncrementBlockedHeadersCaptured increments the counter for captured blocked headers
func IncrementBlockedHeadersCaptured(count int) {
	metrics.blockedHeadersCaptured.Add(float64(count))
}

// IncrementRequestsWithBlockedHeaders increments the counter for requests containing blocked headers
func IncrementRequestsWithBlockedHeaders() {
	metrics.requestsWithBlockedHeaders.Inc()
}
