package metrics

import "time"

var defaultPaymentClientDurationBuckets = []float64{0.05, 0.1, 0.25, 0.5, 1, 1.5, 2, 5}

type PaymentClientMetrics struct {
	service     string
	target      string
	requests    *CounterVec
	durations   *HistogramVec
	defaultPath string
}

func NewPaymentClientMetrics(service, target string) *PaymentClientMetrics {
	return &PaymentClientMetrics{
		service: service,
		target:  target,
		requests: NewCounterVec(
			"ads2_payment_client_requests_total",
			"Total outbound payment authorization requests made by the order service.",
			[]string{"service", "target_service", "method", "route", "outcome", "status_code"},
		),
		durations: NewHistogram(
			"ads2_payment_client_request_duration_seconds",
			"Outbound payment authorization request duration in seconds.",
			defaultPaymentClientDurationBuckets,
			[]string{"service", "target_service", "method", "route", "outcome"},
		),
		defaultPath: "/payments",
	}
}

func (m *PaymentClientMetrics) Collectors() []Collector {
	return []Collector{m.requests, m.durations}
}

func (m *PaymentClientMetrics) Observe(method, route string, statusCode int, outcome string, duration time.Duration) {
	if route == "" {
		route = m.defaultPath
	}

	statusCodeLabel := "0"
	if statusCode > 0 {
		statusCodeLabel = formatFloat(float64(statusCode))
	}

	m.requests.Inc(m.service, m.target, method, route, outcome, statusCodeLabel)
	m.durations.Observe(duration.Seconds(), m.service, m.target, method, route, outcome)
}
