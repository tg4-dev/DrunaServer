package handler

import (
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "druna_http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"method", "path", "status"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "druna_http_request_duration_seconds",
		Help:    "HTTP request duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})
)

func metricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		timer := prometheus.NewTimer(httpRequestDuration.WithLabelValues(c.Request.Method, path))
		c.Next()
		timer.ObserveDuration()
		httpRequestsTotal.WithLabelValues(c.Request.Method, path, strconv.Itoa(c.Writer.Status())).Inc()
	}
}

func registerMetricsRoute(router *gin.Engine) {
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
}

func metricsEnabled() bool {
	value := os.Getenv("METRICS_ENABLED")
	if value == "" {
		return false
	}
	enabled, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}
	return enabled
}
