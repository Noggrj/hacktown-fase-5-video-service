// Package metrics exposes a Prometheus /metrics endpoint plus a thin HTTP
// middleware recording request count and latency. Scraped by the
// Prometheus Agent deployed alongside the service in EKS.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests processed, labeled by path/method/status.",
	}, []string{"path", "method", "status"})

	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"path", "method"})

	// Business metric — exported so internal/video/usecase can record it
	// directly. Counts an upload as soon as it's accepted (raw video in
	// S3, row persisted) — same "already accepted" moment the 202
	// response represents, regardless of whether the Kafka publish that
	// follows succeeds.
	VideosUploaded = promauto.NewCounter(prometheus.CounterOpts{
		Name: "fiapx_videos_uploaded_total",
		Help: "Total videos successfully uploaded and accepted for processing.",
	})
)

// Handler serves the /metrics scrape endpoint.
func Handler() http.Handler {
	return promhttp.Handler()
}

// Middleware records one observation per request. Uses r.URL.Path
// directly (not a route-pattern) — acceptable cardinality since this
// service only exposes a handful of fixed, non-parameterized routes.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		httpRequests.WithLabelValues(r.URL.Path, r.Method, strconv.Itoa(sw.status)).Inc()
		httpDuration.WithLabelValues(r.URL.Path, r.Method).Observe(time.Since(start).Seconds())
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
