package correlator

import (
	"testing"
	"time"

	"traceguard/internal/domain"
)

func TestEngineIngestAndCorrelate(t *testing.T) {
	engine := NewEngine(100, "universal_packet_engine")

	span1 := &domain.Span{
		ID:            "span-1",
		TraceID:       "trace-100",
		CorrelationID: "corr-xyz",
		ServiceName:   "api-gateway",
		TargetService: "user-service",
		Protocol:      domain.ProtocolHTTP,
		Name:          "GET /users",
		DurationMs:    15.5,
		StartTime:     time.Now(),
		Status:        domain.StatusOk,
	}

	span2 := &domain.Span{
		ID:            "span-2",
		TraceID:       "trace-100",
		ParentSpanID:  "span-1",
		CorrelationID: "corr-xyz",
		ServiceName:   "user-service",
		TargetService: "postgres-db",
		Protocol:      domain.ProtocolPostgres,
		Name:          "SQL SELECT",
		SQLQuery:      "SELECT * FROM users /* x-correlation-id='corr-xyz' */;",
		DurationMs:    10.2,
		StartTime:     time.Now().Add(2 * time.Millisecond),
		Status:        domain.StatusOk,
	}

	engine.IngestSpan(span1)
	tree := engine.IngestSpan(span2)

	if tree == nil {
		t.Fatalf("expected correlated trace tree, got nil")
	}

	if len(tree.Spans) != 2 {
		t.Errorf("expected 2 spans in tree, got %d", len(tree.Spans))
	}

	snapshot := engine.GetMetricsSnapshot()
	if snapshot.TotalSpansCaptured != 2 {
		t.Errorf("expected 2 total spans captured, got %d", snapshot.TotalSpansCaptured)
	}

	if len(snapshot.TopologyEdges) != 2 {
		t.Errorf("expected 2 topology edges, got %d", len(snapshot.TopologyEdges))
	}
}
