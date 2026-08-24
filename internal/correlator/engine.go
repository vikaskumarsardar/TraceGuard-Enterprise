package correlator

import (
	"sync"
	"time"

	"traceguard/internal/domain"
)

// Engine correlates individual spans across HTTP services and DB queries into unified Trace Trees.
type Engine struct {
	mu            sync.RWMutex
	store         *domain.SafeSpanStore
	traces        map[string]*domain.TraceTree
	correlationMap map[string]string // Maps correlationID -> traceID
	topologyEdges map[string]*domain.TopologyEdge
	maxTraces     int
	engineMode    string
}

func NewEngine(maxTraces int, engineMode string) *Engine {
	return &Engine{
		store:          domain.NewSafeSpanStore(),
		traces:         make(map[string]*domain.TraceTree),
		correlationMap: make(map[string]string),
		topologyEdges:  make(map[string]*domain.TopologyEdge),
		maxTraces:      maxTraces,
		engineMode:     engineMode,
	}
}

// IngestSpan processes an incoming captured span, links it to existing trace trees, and updates topology.
func (e *Engine) IngestSpan(span *domain.Span) *domain.TraceTree {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.store.AddSpan(span)

	// Resolve actual traceID via correlation map if available
	if span.CorrelationID != "" {
		if existingTraceID, found := e.correlationMap[span.CorrelationID]; found {
			span.TraceID = existingTraceID
		} else {
			e.correlationMap[span.CorrelationID] = span.TraceID
		}
	}

	tree, exists := e.traces[span.TraceID]
	if !exists {
		tree = &domain.TraceTree{
			TraceID:       span.TraceID,
			CorrelationID: span.CorrelationID,
			RootSpan:      span,
			Spans:         []*domain.Span{span},
			StartTime:     span.StartTime,
			ServiceCount:  1,
			HasErrors:     span.Status == domain.StatusError,
		}
		e.traces[span.TraceID] = tree
	} else {
		tree.Spans = append(tree.Spans, span)
		if span.Status == domain.StatusError {
			tree.HasErrors = true
		}
		// Recalculate duration
		if span.EndTime.After(tree.StartTime) {
			tree.TotalDurationMs = float64(span.EndTime.Sub(tree.StartTime).Microseconds()) / 1000.0
		}
		tree.ServiceCount = countUniqueServices(tree.Spans)
	}

	// Update topology dependencies
	if span.TargetService != "" && span.ServiceName != "" {
		edgeKey := span.ServiceName + "->" + span.TargetService
		edge, ok := e.topologyEdges[edgeKey]
		if !ok {
			edge = &domain.TopologyEdge{
				SourceService: span.ServiceName,
				TargetService: span.TargetService,
				Protocol:      string(span.Protocol),
			}
			e.topologyEdges[edgeKey] = edge
		}
		edge.RequestCount++
		if span.Status == domain.StatusError {
			edge.ErrorCount++
		}
		edge.AvgLatencyMs = (edge.AvgLatencyMs*float64(edge.RequestCount-1) + span.DurationMs) / float64(edge.RequestCount)
	}

	// Evict oldest trace if size limit exceeded
	if len(e.traces) > e.maxTraces {
		e.evictOldestTrace()
	}

	return tree
}

// GetRecentTraces returns the latest N correlated trace trees.
func (e *Engine) GetRecentTraces(limit int) []*domain.TraceTree {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*domain.TraceTree, 0, len(e.traces))
	for _, tree := range e.traces {
		result = append(result, tree)
	}

	// Truncate to limit if needed
	if limit > 0 && len(result) > limit {
		return result[len(result)-limit:]
	}
	return result
}

// GetMetricsSnapshot computes RED telemetry metrics and service topology stats.
func (e *Engine) GetMetricsSnapshot() *domain.MetricSnapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var totalSpans int64
	var totalErrors int64
	var totalDuration float64
	protocolCount := make(map[domain.ProtocolType]int)
	servicesMap := make(map[string]bool)
	edges := make([]domain.TopologyEdge, 0, len(e.topologyEdges))

	for _, tree := range e.traces {
		for _, span := range tree.Spans {
			totalSpans++
			if span.Status == domain.StatusError {
				totalErrors++
			}
			totalDuration += span.DurationMs
			protocolCount[span.Protocol]++
			if span.ServiceName != "" {
				servicesMap[span.ServiceName] = true
			}
			if span.TargetService != "" {
				servicesMap[span.TargetService] = true
			}
		}
	}

	avgLatency := 0.0
	if totalSpans > 0 {
		avgLatency = totalDuration / float64(totalSpans)
	}

	activeServices := make([]string, 0, len(servicesMap))
	for s := range servicesMap {
		activeServices = append(activeServices, s)
	}

	for _, edge := range e.topologyEdges {
		edges = append(edges, *edge)
	}

	return &domain.MetricSnapshot{
		Timestamp:          time.Now(),
		TotalSpansCaptured: totalSpans,
		ActiveTraces:       int64(len(e.traces)),
		TotalErrors:        totalErrors,
		AverageLatencyMs:   avgLatency,
		ProtocolBreakdown:  protocolCount,
		EngineMode:         e.engineMode,
		ActiveServices:     activeServices,
		TopologyEdges:      edges,
	}
}

func (e *Engine) evictOldestTrace() {
	var oldestKey string
	var oldestTime time.Time

	first := true
	for key, tree := range e.traces {
		if first || tree.StartTime.Before(oldestTime) {
			oldestTime = tree.StartTime
			oldestKey = key
			first = false
		}
	}
	if oldestKey != "" {
		if tree, found := e.traces[oldestKey]; found {
			if tree.CorrelationID != "" {
				delete(e.correlationMap, tree.CorrelationID)
			}
		}
		delete(e.traces, oldestKey)
	}
}

func countUniqueServices(spans []*domain.Span) int {
	seen := make(map[string]bool)
	for _, s := range spans {
		if s.ServiceName != "" {
			seen[s.ServiceName] = true
		}
		if s.TargetService != "" {
			seen[s.TargetService] = true
		}
	}
	return len(seen)
}
