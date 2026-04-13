package metrics

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

var defaultHTTPDurationBuckets = []float64{0.05, 0.1, 0.25, 0.5, 1, 1.5, 2, 5}

type HTTPServerMetrics struct {
	service     string
	requests    *CounterVec
	durations   *HistogramVec
	ignoreRoute map[string]struct{}
}

func NewHTTPServerMetrics(service string) *HTTPServerMetrics {
	return &HTTPServerMetrics{
		service: service,
		requests: NewCounterVec(
			"ads2_http_requests_total",
			"Total inbound HTTP requests handled by the service.",
			[]string{"service", "method", "route", "status_class", "status_code"},
		),
		durations: NewHistogram(
			"ads2_http_request_duration_seconds",
			"Inbound HTTP request duration in seconds.",
			defaultHTTPDurationBuckets,
			[]string{"service", "method", "route", "status_class"},
		),
		ignoreRoute: map[string]struct{}{
			"/healthz": {},
			"/metrics": {},
		},
	}
}

func (m *HTTPServerMetrics) Collectors() []Collector {
	return []Collector{m.requests, m.durations}
}

func (m *HTTPServerMetrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		start := time.Now()

		next.ServeHTTP(recorder, r)

		route := routePattern(r)
		if _, ignored := m.ignoreRoute[route]; ignored {
			return
		}

		statusClass := classifyHTTPStatus(recorder.statusCode)
		m.requests.Inc(m.service, r.Method, route, statusClass, strconv.Itoa(recorder.statusCode))
		m.durations.Observe(time.Since(start).Seconds(), m.service, r.Method, route, statusClass)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func routePattern(request *http.Request) string {
	if request.Pattern == "" {
		return request.URL.Path
	}

	parts := strings.SplitN(request.Pattern, " ", 2)
	if len(parts) == 2 {
		return parts[1]
	}

	return request.Pattern
}

func classifyHTTPStatus(statusCode int) string {
	switch {
	case statusCode >= 500:
		return "5xx"
	case statusCode >= 400:
		return "4xx"
	case statusCode >= 300:
		return "3xx"
	case statusCode >= 200:
		return "2xx"
	default:
		return "1xx"
	}
}
