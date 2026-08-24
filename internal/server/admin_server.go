package server

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"traceguard/internal/config"
	"traceguard/internal/correlator"
	"traceguard/internal/domain"
	"traceguard/internal/exporter"
	"traceguard/internal/protocol"
	"traceguard/internal/web"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// AdminServer runs the HTTP REST API, WebSocket stream, and embedded dashboard UI.
type AdminServer struct {
	cfg         *config.Config
	correlator  *correlator.Engine
	otlpExp     *exporter.OTLPExporter
	promExp     *exporter.PrometheusExporter
	httpParser  *protocol.HTTPParser
	dbParser    *protocol.DBParser
	wsClients   map[*websocket.Conn]bool
	wsMu        sync.Mutex
	broadcast   chan interface{}
	stopChan    chan struct{}
}

func NewAdminServer(cfg *config.Config, engine *correlator.Engine, otlp *exporter.OTLPExporter) *AdminServer {
	return &AdminServer{
		cfg:        cfg,
		correlator: engine,
		otlpExp:    otlp,
		promExp:    exporter.NewPrometheusExporter(),
		httpParser: protocol.NewHTTPParser(),
		dbParser:   protocol.NewDBParser(),
		wsClients:  make(map[*websocket.Conn]bool),
		broadcast:  make(chan interface{}, 500),
		stopChan:   make(chan struct{}),
	}
}

// Start launches the HTTP server and background websocket broadcast loop.
func (s *AdminServer) Start() error {
	mux := http.NewServeMux()

	// 1. Embedded Web Dashboard Static File Server
	staticFS, err := web.GetFileSystem()
	if err != nil {
		return fmt.Errorf("failed to load embedded static filesystem: %w", err)
	}
	fileServer := http.FileServer(staticFS)
	mux.Handle("/", fileServer)

	// 2. REST API Endpoints
	mux.HandleFunc("/api/v1/metrics", s.handleGetMetrics)
	mux.HandleFunc("/api/v1/traces", s.handleGetTraces)
	mux.HandleFunc("/api/v1/simulate", s.handleSimulateTraffic)

	// 3. Prometheus Endpoint
	mux.HandleFunc("/metrics", s.handlePrometheusMetrics)

	// 4. WebSocket Endpoint
	mux.HandleFunc("/ws", s.handleWebSocket)

	go s.broadcastWorker()

	// If demo mode is enabled, run synthetic background traffic generator
	if s.cfg.DemoMode {
		go s.runSyntheticTrafficGenerator()
	}

	addr := fmt.Sprintf(":%d", s.cfg.HTTPPort)
	log.Printf("[TraceGuard] Admin Server & Web Dashboard listening on http://localhost%s\n", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *AdminServer) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	snapshot := s.correlator.GetMetricsSnapshot()
	_ = json.NewEncoder(w).Encode(snapshot)
}

func (s *AdminServer) handleGetTraces(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	traces := s.correlator.GetRecentTraces(50)
	_ = json.NewEncoder(w).Encode(traces)
}

func (s *AdminServer) handleSimulateTraffic(w http.ResponseWriter, r *http.Request) {
	go s.generateSimulatedTraceBatch()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "simulated"})
}

func (s *AdminServer) handlePrometheusMetrics(w http.ResponseWriter, r *http.Request) {
	snapshot := s.correlator.GetMetricsSnapshot()
	s.promExp.ServeHTTP(w, r, snapshot)
}

func (s *AdminServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	s.wsMu.Lock()
	s.wsClients[conn] = true
	s.wsMu.Unlock()

	// Keep-alive read loop
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			s.wsMu.Lock()
			delete(s.wsClients, conn)
			s.wsMu.Unlock()
			_ = conn.Close()
			break
		}
	}
}

func (s *AdminServer) broadcastWorker() {
	for {
		select {
		case msg := <-s.broadcast:
			s.wsMu.Lock()
			for client := range s.wsClients {
				if err := client.WriteJSON(msg); err != nil {
					_ = client.Close()
					delete(s.wsClients, client)
				}
			}
			s.wsMu.Unlock()
		case <-s.stopChan:
			return
		}
	}
}

// IngestRawSpan processes any newly captured span and broadcasts to UI & OTLP.
func (s *AdminServer) IngestRawSpan(span *domain.Span) {
	tree := s.correlator.IngestSpan(span)
	s.otlpExp.ExportSpan(span)

	// Broadcast via WebSocket to dashboard
	s.broadcast <- map[string]interface{}{
		"type": "span",
		"span": span,
		"tree": tree,
	}
}

func (s *AdminServer) runSyntheticTrafficGenerator() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.generateSimulatedTraceBatch()
	}
}

func (s *AdminServer) generateSimulatedTraceBatch() {
	correlationID := fmt.Sprintf("req-%s", uuid.New().String()[:8])
	traceID := fmt.Sprintf("tr-%s", uuid.New().String()[:12])

	services := []string{"api-gateway", "order-service", "user-service", "payment-service"}
	routes := []string{"/api/v1/orders/checkout", "/api/v1/users/profile", "/api/v1/payments/charge"}
	dbs := []string{"postgres-primary", "redis-cache"}

	rootService := services[rand.Intn(len(services))]
	route := routes[rand.Intn(len(routes))]

	startTime := time.Now()
	dur1 := float64(rand.Intn(30)+5) + rand.Float64()

	// Span 1: HTTP Gateway Ingress
	span1 := &domain.Span{
		ID:            uuid.New().String(),
		TraceID:       traceID,
		CorrelationID: correlationID,
		ServiceName:   rootService,
		TargetService: "order-service",
		Protocol:      domain.ProtocolHTTP,
		Name:          "POST " + route,
		Method:        "POST",
		Path:          route,
		StatusCode:    200,
		DurationMs:    dur1,
		StartTime:     startTime,
		EndTime:       startTime.Add(time.Duration(dur1) * time.Millisecond),
		Status:        domain.StatusOk,
	}
	s.IngestRawSpan(span1)

	// Span 2: Internal Microservice gRPC call
	time.Sleep(10 * time.Millisecond)
	dur2 := float64(rand.Intn(40)+10) + rand.Float64()
	span2 := &domain.Span{
		ID:            uuid.New().String(),
		TraceID:       traceID,
		ParentSpanID:  span1.ID,
		CorrelationID: correlationID,
		ServiceName:   "order-service",
		TargetService: "user-service",
		Protocol:      domain.ProtocolGRPC,
		Name:          "gRPC /OrderService/CreateOrder",
		Method:        "POST",
		Path:          "/OrderService/CreateOrder",
		StatusCode:    200,
		DurationMs:    dur2,
		StartTime:     startTime.Add(10 * time.Millisecond),
		EndTime:       startTime.Add(time.Duration(10+dur2) * time.Millisecond),
		Status:        domain.StatusOk,
	}
	s.IngestRawSpan(span2)

	// Span 3: Database PostgreSQL Query with SQL Commenter
	time.Sleep(5 * time.Millisecond)
	dur3 := float64(rand.Intn(25)+5) + rand.Float64()
	dbName := dbs[rand.Intn(len(dbs))]
	isError := rand.Float32() < 0.1 // 10% simulated error rate for testing
	status := domain.StatusOk
	if isError {
		status = domain.StatusError
	}

	span3 := &domain.Span{
		ID:            uuid.New().String(),
		TraceID:       traceID,
		ParentSpanID:  span2.ID,
		CorrelationID: correlationID,
		ServiceName:   "user-service",
		TargetService: dbName,
		Protocol:      domain.ProtocolPostgres,
		Name:          "SQL SELECT users",
		SQLQuery:      fmt.Sprintf("SELECT * FROM users WHERE id = $1 /* x-correlation-id='%s' */;", correlationID),
		DurationMs:    dur3,
		StartTime:     startTime.Add(15 * time.Millisecond),
		EndTime:       startTime.Add(time.Duration(15+dur3) * time.Millisecond),
		Status:        status,
	}
	s.IngestRawSpan(span3)
}
