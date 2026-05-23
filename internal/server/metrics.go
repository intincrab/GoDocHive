package server

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hiver_http_requests_total",
			Help: "Total number of HTTP requests by method and response code.",
		},
		[]string{"method", "code"},
	)
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "hiver_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds by method.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
	)
)

func init() {
	prometheus.MustRegister(requestsTotal, requestDuration)
}

// observe records a single request's outcome into the metrics.
func observe(method string, status int, seconds float64) {
	requestsTotal.WithLabelValues(method, strconv.Itoa(status)).Inc()
	requestDuration.WithLabelValues(method).Observe(seconds)
}
