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
