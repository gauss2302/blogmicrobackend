package metrics

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	reg     = prometheus.NewRegistry()
	httpReq *prometheus.CounterVec
	httpDur *prometheus.HistogramVec
	grpcReq *prometheus.CounterVec
	grpcDur *prometheus.HistogramVec
)

// Init registers collectors and HTTP/gRPC metrics for this process.
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
	grpcReq = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "microblog",
		Subsystem: "grpc",
		Name:      "server_requests_total",
		Help:      "Total gRPC unary server requests.",
	}, []string{"service", "grpc_method", "grpc_code"})
	grpcDur = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "microblog",
		Subsystem: "grpc",
		Name:      "server_request_duration_seconds",
		Help:      "gRPC unary server request duration in seconds.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"service", "grpc_method"})

	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		httpReq,
		httpDur,
		grpcReq,
		grpcDur,
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

// UnaryServerInterceptor records gRPC unary call counts, codes, and latency.
func UnaryServerInterceptor(service string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		code := codes.OK
		if err != nil {
			if st, ok := status.FromError(err); ok {
				code = st.Code()
			} else {
				code = codes.Unknown
			}
		}
		grpcReq.WithLabelValues(service, info.FullMethod, code.String()).Inc()
		grpcDur.WithLabelValues(service, info.FullMethod).Observe(time.Since(start).Seconds())
		return resp, err
	}
}
