package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"traceguard/internal/config"
	"traceguard/internal/correlator"
	"traceguard/internal/ebpf"
	"traceguard/internal/exporter"
	"traceguard/internal/server"
)

const version = "1.0.0-enterprise"

func main() {
	printBanner()

	// 1. Load Configuration
	cfg := config.LoadConfig()

	log.Printf("[TraceGuard] Initializing TraceGuard Enterprise v%s...\n", version)
	log.Printf("[TraceGuard] Engine Mode: %s (Demo Traffic Mode: %v)\n", cfg.EngineMode, cfg.DemoMode)
	log.Printf("[TraceGuard] OTLP Exporter Endpoint: %s\n", cfg.OTLPEndpoint)

	// 2. Initialize Trace Correlation Engine
	correlatorEngine := correlator.NewEngine(cfg.MaxTraceHistory, cfg.EngineMode)

	// 3. Initialize & Start eBPF Manager
	ebpfManager := ebpf.NewManager(correlatorEngine)
	if err := ebpfManager.StartProbes(); err != nil {
		log.Printf("[TraceGuard] eBPF Probe warning: %v\n", err)
	}

	if ebpfManager.IsNativeEBPFActive() {
		log.Println("[TraceGuard] 🔥 Native Linux Kernel eBPF Engine is ACTIVE!")
	} else {
		log.Println("[TraceGuard] ⚡ Running Universal Packet Engine (Dev / Non-eBPF Mode).")
	}

	// 4. Initialize OTLP Exporter
	otlpExp := exporter.NewOTLPExporter(cfg.OTLPEndpoint)
	defer otlpExp.Stop()

	// 5. Initialize & Launch Admin Web Server
	adminServer := server.NewAdminServer(cfg, correlatorEngine, otlpExp)

	go func() {
		if err := adminServer.Start(); err != nil {
			log.Fatalf("[TraceGuard] Admin server fatal error: %v\n", err)
		}
	}()

	log.Println("[TraceGuard] Enterprise Agent is running! Press Ctrl+C to stop.")

	// 6. Handle Graceful Shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("\n[TraceGuard] Shutting down gracefully...")
}

func printBanner() {
	banner := `
  _____                     ____                 _ 
 |_   _| __ __ _  ___ ___  / ___|_   u __ _ _ __| |
   | || '__/ _` + "`" + ` |/ __/ _ \| |  _| | | / _` + "`" + ` | '__/ _` + "`" + `|
   | || | | (_| | (_|  __/| |_| | |_| | (_| | | | (_| |
   |_||_|  \__,_|\___\___| \____|\__,_|\__,_|_|  \__,_|
   Enterprise eBPF Telemetry & Correlation Gateway v1.0
`
	fmt.Println(banner)
}
