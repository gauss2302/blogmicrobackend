package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	reg     = prometheus.NewRegistry()
	httpReq *prometheus.CounterVec
	httpDur *prometheus.HistogramVec
)

// Init registers collectors and HTTP metrics for this process.
func Init() {
	httpReq = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "microblog",
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "Total HTTP requests.",
	}, []string{"service", "method", "route", "status"})
	httpDur = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "microblog",
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "HTTP request duration in seconds.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"service", "method", "route"})

	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		httpReq,
		httpDur,
	)
}

// Handler exposes /metrics for Prometheus scraping.
func Handler() http.Handler {
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg})
}

// GinMiddleware records request counts and latency after routing.
func GinMiddleware(service string) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		status := strconv.Itoa(c.Writer.Status())
		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}
		method := c.Request.Method
		httpReq.WithLabelValues(service, method, route, status).Inc()
		httpDur.WithLabelValues(service, method, route).Observe(time.Since(start).Seconds())
	}
}
