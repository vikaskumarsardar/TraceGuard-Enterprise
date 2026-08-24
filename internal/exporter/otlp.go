package exporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"traceguard/internal/domain"
)

// OTLPExporter exports domain spans to OpenTelemetry Collector endpoints over OTLP/HTTP or OTLP/gRPC.
type OTLPExporter struct {
	endpoint string
	client   *http.Client
	queue    chan *domain.Span
	mu       sync.Mutex
	stopChan chan struct{}
}

func NewOTLPExporter(endpoint string) *OTLPExporter {
	tr := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	}
	exp := &OTLPExporter{
		endpoint: endpoint,
		client:   &http.Client{Timeout: 5 * time.Second, Transport: tr},
		queue:    make(chan *domain.Span, 1000),
		stopChan: make(chan struct{}),
	}
	go exp.workerLoop()
	return exp
}

func (e *OTLPExporter) ExportSpan(span *domain.Span) {
	select {
	case e.queue <- span:
	default:
		// Queue full, drop span to maintain zero app backpressure
	}
}

func (e *OTLPExporter) workerLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var batch []*domain.Span

	for {
		select {
		case span := <-e.queue:
			batch = append(batch, span)
			if len(batch) >= 100 {
				e.flushBatch(batch)
				batch = nil
			}
		case <-ticker.C:
			if len(batch) > 0 {
				e.flushBatch(batch)
				batch = nil
			}
		case <-e.stopChan:
			if len(batch) > 0 {
				e.flushBatch(batch)
			}
			return
		}
	}
}

func (e *OTLPExporter) flushBatch(spans []*domain.Span) {
	if len(spans) == 0 || e.endpoint == "" {
		return
	}

	payload := map[string]interface{}{
		"resourceSpans": []map[string]interface{}{
			{
				"resource": map[string]interface{}{
					"attributes": []map[string]string{
						{"key": "service.name", "value": "traceguard-agent"},
					},
				},
				"scopeSpans": []map[string]interface{}{
					{
						"spans": formatOTLPSpans(spans),
					},
				},
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	url := fmt.Sprintf("http://%s/v1/traces", e.endpoint)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err == nil && resp != nil {
		_ = resp.Body.Close()
	}
}

func formatOTLPSpans(spans []*domain.Span) []map[string]interface{} {
	otlpSpans := make([]map[string]interface{}, len(spans))
	for i, s := range spans {
		otlpSpans[i] = map[string]interface{}{
			"traceId":           s.TraceID,
			"spanId":            s.ID,
			"parentSpanId":      s.ParentSpanID,
			"name":              s.Name,
			"kind":              "SPAN_KIND_SERVER",
			"startTimeUnixNano": s.StartTime.UnixNano(),
			"endTimeUnixNano":   s.EndTime.UnixNano(),
			"attributes": []map[string]string{
				{"key": "http.method", "value": s.Method},
				{"key": "http.target", "value": s.Path},
				{"key": "correlation.id", "value": s.CorrelationID},
				{"key": "service.name", "value": s.ServiceName},
			},
		}
	}
	return otlpSpans
}

func (e *OTLPExporter) Stop() {
	close(e.stopChan)
}
