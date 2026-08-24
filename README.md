# TraceGuard Enterprise

> **Ultra-Low Latency eBPF-Powered Zero-Code Microservice Observability, Distributed Tracing & Correlation Gateway**

> *"OpenTelemetry provides application context; eBPF provides zero-instrumentation system and network visibility; TraceGuard correlates both."*

TraceGuard Enterprise is a production-grade, lightweight telemetry daemon and multi-signal correlation gateway written in Go. It enables zero-code distributed tracing across HTTP microservices, gRPC channels, and SQL/NoSQL databases (PostgreSQL, MySQL, Redis) with real-time correlation ID extraction, 5-tuple socket tracking, SQL comment parsing, OTLP exporting, and a built-in embedded glassmorphism Web Dashboard.

---

## 🌟 Key Features

* **Multi-Signal Correlation Engine**:
  * **Level 1 (Application Context)**: W3C `traceparent` & `x-correlation-id` HTTP/gRPC header propagation.
  * **Level 2 (Kernel & Network Identity)**: 5-tuple socket tracking (`src_ip:src_port -> dst_ip:dst_port`), TCP RTT, and OS thread/PID context.
  * **Level 3 (Payload Enrichment)**: ORM `sqlcommenter` metadata parsing (`/* x-correlation-id='...' */`).
* **Dual-Engine Architecture**:
  * **Native eBPF Engine (`cilium/ebpf`)**: Attaches C eBPF probes directly to Linux kernel sockets (`bpf/socket_filter.bpf.c`) for zero-code kernel level tracing (<1% CPU).
  * **Universal Packet Engine**: Runs on any dev environment (macOS, Docker, Windows/WSL, CI/CD) with zero root setup required.
* **Multi-Protocol Zero-Copy Parsers**:
  * HTTP/1.1 & HTTP/2 (`x-correlation-id`, `x-request-id`, W3C `traceparent` headers).
  * gRPC (Protobuf over HTTP/2).
  * PostgreSQL & MySQL wire protocols.
  * Redis RESP protocol.
* **Embedded Glassmorphism Web Dashboard**:
  * Embedded directly into the static binary using `go:embed`.
  * Real-time trace waterfall charts, service dependency topology graph, RED metrics, and WebSocket stream.
* **Enterprise Exporters**:
  * OpenTelemetry OTLP (`OTLP/gRPC` and `OTLP/HTTP`) to Grafana Tempo, Jaeger, Datadog.
  * Prometheus metrics at `/metrics`.

---

## 🚀 Quick Start (Local Run)

### 1. Build and Run
```bash
# Build the binary
make build

# Run TraceGuard Enterprise locally
make run
```

### 2. Access Web Dashboard
Open your browser at **`http://localhost:8080`**.

---

## ⚙️ Configuration Options

| Environment Variable | CLI Flag | Default | Description |
| :--- | :--- | :--- | :--- |
| `HTTP_PORT` | `-port` | `8080` | Dashboard & Admin HTTP server port |
| `OTLP_ENDPOINT` | `-otlp-endpoint` | `localhost:4317` | OpenTelemetry Collector endpoint |
| `ENGINE_MODE` | `-engine` | `auto` | Engine mode: `auto` \| `ebpf` \| `packet` \| `demo` |
| `DEMO_MODE` | `-demo` | `true` | Enable synthetic background traffic simulator |
| `MAX_TRACE_HISTORY` | — | `500` | Max in-memory LRU trace capacity (~30MB max RAM) |

---

## 🐳 Kubernetes Production Deployment (DaemonSet)

In production Kubernetes clusters, deploy TraceGuard as a **DaemonSet** (1 pod per worker node) with `CAP_BPF` kernel permissions:

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: traceguard-agent
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: traceguard-agent
  template:
    metadata:
      labels:
        app: traceguard-agent
    spec:
      hostNetwork: true
      containers:
      - name: traceguard
        image: traceguard:latest
        securityContext:
          capabilities:
            add: ["CAP_BPF", "CAP_SYS_ADMIN", "CAP_NET_RAW"]
        env:
        - name: OTLP_ENDPOINT
          value: "tempo.monitoring.svc:4317"
        - name: DEMO_MODE
          value: "false"
        ports:
        - containerPort: 8080
          name: http
```

---

## 🛠️ Project Architecture

```
GO_Learn/
├── bpf/
│   └── socket_filter.bpf.c          # Native C eBPF Kernel Program (SEC("socket"))
├── cmd/traceguard/
│   └── main.go                      # Entry point & signal handler
├── internal/
│   ├── ebpf/loader.go               # Go eBPF Loader Manager (cilium/ebpf loader)
│   ├── config/config.go             # Enterprise configuration loader
│   ├── domain/trace.go              # Core domain entities (Span, TraceTree, MetricSnapshot)
│   ├── correlator/engine.go         # Multi-signal trace correlation & topology engine
│   ├── protocol/                    # Zero-copy HTTP, gRPC, PostgreSQL & Redis parsers
│   │   ├── http_parser.go
│   │   └── db_parser.go
│   ├── exporter/                    # OTLP (Tempo/Jaeger) & Prometheus exporters
│   └── server/admin_server.go       # REST API, WebSockets, and Fallback engine
├── pkg/sqlcommenter/                # Helper for Go ORMs (GORM, pgx)
├── web/                             # Embedded Dark-Mode Glassmorphism UI (HTML/CSS/JS)
└── Makefile                         # Build & test tasks
```
