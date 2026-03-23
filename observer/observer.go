package observer

import (
	"log/slog"
	"os"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	Registry         *prometheus.Registry
	HTTPRequests     *prometheus.CounterVec
	HTTPDuration     *prometheus.HistogramVec
	SessionEvents    prometheus.Counter
	PromptDispatchNs prometheus.Histogram
}

func NewLogger(service string) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{AddSource: false})).With("service", service)
}

func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		Registry: reg,
		HTTPRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "agentd",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "HTTP requests.",
		}, []string{"route", "code", "method"}),
		HTTPDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "agentd",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"route", "method"}),
		SessionEvents: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "agentd",
			Subsystem: "session",
			Name:      "events_total",
			Help:      "Published session events.",
		}),
		PromptDispatchNs: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "agentd",
			Subsystem: "session",
			Name:      "prompt_dispatch_seconds",
			Help:      "Prompt dispatch latency.",
			Buckets:   prometheus.DefBuckets,
		}),
	}
	reg.MustRegister(m.HTTPRequests, m.HTTPDuration, m.SessionEvents, m.PromptDispatchNs)
	return m
}
