package metrics

import (
	"net/http"
	"strconv"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/AB-Lindex/rest-rego/internal/types"
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

	customLabels []string
	maxLen       int
	def          string
}

// New creates a new instance of the metrics. customLabels are additional Prometheus
// label names populated per-request from policy "labels" results; maxLen and def
// control sanitisation of their values (see sanitizeLabelValue).
func New(customLabels []string, maxLen int, def string) {
	metrics.reg = prometheus.NewRegistry()
	metrics.customLabels = customLabels
	metrics.maxLen = maxLen
	metrics.def = def

	metrics.reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	metrics.buckets = prometheus.ExponentialBuckets(0.1, 1.5, 5)

	labels := append([]string{"method", "code", "url"}, customLabels...)

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

// StatusWriter is the interface expected by the metrics middleware.
// It is satisfied by the responseTracker in the router package.
type StatusWriter interface {
	http.ResponseWriter
	Status() int
	Size() int
}

// Wrap wraps a handler for metrics-collection
func Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w2, ok := w.(StatusWriter)
		if !ok {
			panic("metrics: StatusWriter expected")
		}

		now := time.Now()

		next.ServeHTTP(w2, r)

		info := types.GetInfo(r)

		labels := append(make([]string, 0, 3+len(metrics.customLabels)), r.Method, strconv.Itoa(w2.Status()), info.URL)
		for _, name := range metrics.customLabels {
			labels = append(labels, sanitizeLabelValue(info.Labels[name], metrics.maxLen, metrics.def))
		}

		metrics.requestDuration.WithLabelValues(labels...).Observe(time.Since(now).Seconds())
		metrics.requestSize.WithLabelValues(labels...).Observe(float64(r.ContentLength))
		metrics.responseSize.WithLabelValues(labels...).Observe(float64(w2.Size()))
		metrics.requestsTotal.WithLabelValues(labels...).Inc()
	})
}

// sanitizeLabelValue coerces a policy-provided value into a value safe to use as a
// Prometheus label: only printable ASCII is kept, the result is truncated to maxLen,
// and def is substituted when the sanitised result is empty.
func sanitizeLabelValue(v string, maxLen int, def string) string {
	buf := make([]byte, 0, len(v))
	for _, r := range v {
		if r < utf8.RuneSelf && unicode.IsPrint(r) {
			buf = append(buf, byte(r))
			if len(buf) >= maxLen {
				break
			}
		}
	}
	if len(buf) == 0 {
		return def
	}
	return string(buf)
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
