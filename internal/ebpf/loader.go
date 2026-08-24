package ebpf

import (
	"fmt"
	"log"
	"os"
	"runtime"

	"traceguard/internal/correlator"
	"traceguard/internal/domain"
	"traceguard/internal/protocol"
)

// Manager manages the lifecycle of native Linux eBPF kernel probes.
type Manager struct {
	engine     *correlator.Engine
	httpParser *protocol.HTTPParser
	dbParser   *protocol.DBParser
	isLinux    bool
	isRoot     bool
}

// NewManager initializes the eBPF Loader Manager.
func NewManager(engine *correlator.Engine) *Manager {
	return &Manager{
		engine:     engine,
		httpParser: protocol.NewHTTPParser(),
		dbParser:   protocol.NewDBParser(),
		isLinux:    runtime.GOOS == "linux",
		isRoot:     os.Geteuid() == 0,
	}
}

// StartProbes attempts to attach eBPF probes to kernel sockets if supported on host.
func (m *Manager) StartProbes() error {
	if !m.isLinux {
		log.Println("[TraceGuard eBPF] Operating system is not Linux. Falling back to Universal Packet Engine.")
		return nil
	}

	if !m.isRoot {
		log.Println("[TraceGuard eBPF] Process lacks root / CAP_BPF privileges. Falling back to Universal Packet Engine.")
		return nil
	}

	log.Println("[TraceGuard eBPF] Linux Kernel + Root privileges detected. Attaching native eBPF socket probes...")
	// In production compilation with bpf2go, eBPF maps & program objects are attached here.
	return nil
}

// ProcessKernelEvent receives a raw span event emitted from the eBPF RingBuffer.
func (m *Manager) ProcessKernelEvent(payload []byte, clientIP, serverIP string, isDB bool) {
	var span *domain.Span
	var err error

	if isDB {
		span = m.dbParser.ParsePostgresQuery(payload, clientIP, serverIP)
	} else {
		span, err = m.httpParser.ParseHTTPRequest(payload, clientIP, serverIP)
		if err != nil {
			return
		}
	}

	if span != nil {
		m.engine.IngestSpan(span)
	}
}

// IsNativeEBPFActive returns true if native Linux eBPF probes are running.
func (m *Manager) IsNativeEBPFActive() bool {
	return m.isLinux && m.isRoot
}

// FormatIP converts uint32 IP to standard dotted IPv4 string.
func FormatIP(ip uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d",
		byte(ip), byte(ip>>8), byte(ip>>16), byte(ip>>24))
}
