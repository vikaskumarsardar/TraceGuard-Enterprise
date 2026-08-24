package exporter

import (
	"fmt"
	"net/http"

	"traceguard/internal/domain"
)

// PrometheusExporter formats metric snapshots into standard Prometheus text format.
type PrometheusExporter struct{}

func NewPrometheusExporter() *PrometheusExporter {
	return &PrometheusExporter{}
}

func (p *PrometheusExporter) ServeHTTP(w http.ResponseWriter, r *http.Request, snapshot *domain.MetricSnapshot) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	fmt.Fprintf(w, "# HELP traceguard_spans_total Total captured spans\n")
	fmt.Fprintf(w, "# TYPE traceguard_spans_total counter\n")
	fmt.Fprintf(w, "traceguard_spans_total %d\n\n", snapshot.TotalSpansCaptured)

	fmt.Fprintf(w, "# HELP traceguard_active_traces Total active correlated traces\n")
	fmt.Fprintf(w, "# TYPE traceguard_active_traces gauge\n")
	fmt.Fprintf(w, "traceguard_active_traces %d\n\n", snapshot.ActiveTraces)

	fmt.Fprintf(w, "# HELP traceguard_errors_total Total error spans\n")
	fmt.Fprintf(w, "# TYPE traceguard_errors_total counter\n")
	fmt.Fprintf(w, "traceguard_errors_total %d\n\n", snapshot.TotalErrors)

	fmt.Fprintf(w, "# HELP traceguard_average_latency_ms Average trace duration in milliseconds\n")
	fmt.Fprintf(w, "# TYPE traceguard_average_latency_ms gauge\n")
	fmt.Fprintf(w, "traceguard_average_latency_ms %.2f\n\n", snapshot.AverageLatencyMs)

	for protocol, count := range snapshot.ProtocolBreakdown {
		fmt.Fprintf(w, "traceguard_protocol_requests{protocol=\"%s\"} %d\n", string(protocol), count)
	}
}
