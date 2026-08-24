package domain

import (
	"sync"
	"time"
)

// ProtocolType represents supported application & database wire protocols.
type ProtocolType string

const (
	ProtocolHTTP     ProtocolType = "HTTP"
	ProtocolGRPC     ProtocolType = "gRPC"
	ProtocolPostgres ProtocolType = "PostgreSQL"
	ProtocolMySQL    ProtocolType = "MySQL"
	ProtocolRedis    ProtocolType = "Redis"
)

// SpanStatus represents the outcome of a span execution.
type SpanStatus string

const (
	StatusOk    SpanStatus = "OK"
	StatusError SpanStatus = "ERROR"
)

// Span represents an individual trace span captured from socket or eBPF probe.
type Span struct {
	ID            string            `json:"id"`
	TraceID       string            `json:"trace_id"`
	ParentSpanID  string            `json:"parent_span_id,omitempty"`
	CorrelationID string            `json:"correlation_id"`
	ServiceName   string            `json:"service_name"`
	TargetService string            `json:"target_service,omitempty"`
	Protocol      ProtocolType      `json:"protocol"`
	Name          string            `json:"name"`
	Method        string            `json:"method,omitempty"`
	Path          string            `json:"path,omitempty"`
	StatusCode    int               `json:"status_code,omitempty"`
	DurationMs    float64           `json:"duration_ms"`
	StartTime     time.Time         `json:"start_time"`
	EndTime       time.Time         `json:"end_time"`
	Status        SpanStatus        `json:"status"`
	Attributes    map[string]string `json:"attributes,omitempty"`
	SQLQuery      string            `json:"sql_query,omitempty"`
	ClientIP      string            `json:"client_ip,omitempty"`
	ServerIP      string            `json:"server_ip,omitempty"`
}

// TraceTree represents a full tree of correlated spans forming an end-to-end distributed request.
type TraceTree struct {
	TraceID         string    `json:"trace_id"`
	CorrelationID   string    `json:"correlation_id"`
	RootSpan        *Span     `json:"root_span"`
	Spans           []*Span   `json:"spans"`
	TotalDurationMs float64   `json:"total_duration_ms"`
	StartTime       time.Time `json:"start_time"`
	ServiceCount    int       `json:"service_count"`
	HasErrors       bool      `json:"has_errors"`
}

// TopologyEdge represents a directional service dependency link for graph mapping.
type TopologyEdge struct {
	SourceService string  `json:"source_service"`
	TargetService string  `json:"target_service"`
	Protocol      string  `json:"protocol"`
	RequestCount  int64   `json:"request_count"`
	ErrorCount    int64   `json:"error_count"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
}

// MetricSnapshot represents system-wide telemetry RED metrics and engine status.
type MetricSnapshot struct {
	Timestamp          time.Time            `json:"timestamp"`
	TotalSpansCaptured int64                `json:"total_spans_captured"`
	ActiveTraces       int64                `json:"active_traces"`
	TotalErrors        int64                `json:"total_errors"`
	AverageLatencyMs   float64              `json:"average_latency_ms"`
	ProtocolBreakdown  map[ProtocolType]int `json:"protocol_breakdown"`
	EngineMode         string               `json:"engine_mode"`
	ActiveServices     []string             `json:"active_services"`
	TopologyEdges      []TopologyEdge       `json:"topology_edges"`
}

// SafeSpanStore provides thread-safe in-memory caching for spans and trace aggregation.
type SafeSpanStore struct {
	mu     sync.RWMutex
	spans  map[string]*Span
	traces map[string][]*Span
}

func NewSafeSpanStore() *SafeSpanStore {
	return &SafeSpanStore{
		spans:  make(map[string]*Span),
		traces: make(map[string][]*Span),
	}
}

func (s *SafeSpanStore) AddSpan(span *Span) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.spans[span.ID] = span
	s.traces[span.TraceID] = append(s.traces[span.TraceID], span)
}

func (s *SafeSpanStore) GetTrace(traceID string) []*Span {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.traces[traceID]
}
