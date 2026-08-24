package config

import (
	"flag"
	"os"
	"strconv"
)

// Config represents enterprise configuration settings for TraceGuard.
type Config struct {
	HTTPPort          int
	OTLPEndpoint      string
	PrometheusEnabled bool
	LogLevel          string
	EngineMode        string // "auto", "ebpf", "packet_sniffer", "demo"
	DemoMode          bool
	EnableWS          bool
	MaxTraceHistory   int
}

// LoadConfig initializes configuration with defaults, environment overrides, and CLI flags.
func LoadConfig() *Config {
	cfg := &Config{
		HTTPPort:          getEnvInt("HTTP_PORT", 8080),
		OTLPEndpoint:      getEnv("OTLP_ENDPOINT", "localhost:4317"),
		PrometheusEnabled: getEnvBool("PROMETHEUS_ENABLED", true),
		LogLevel:          getEnv("LOG_LEVEL", "INFO"),
		EngineMode:        getEnv("ENGINE_MODE", "auto"),
		DemoMode:          getEnvBool("DEMO_MODE", true), // enabled by default for standalone testing
		EnableWS:          getEnvBool("ENABLE_WS", true),
		MaxTraceHistory:   getEnvInt("MAX_TRACE_HISTORY", 500),
	}

	// CLI flags
	flag.IntVar(&cfg.HTTPPort, "port", cfg.HTTPPort, "HTTP Admin & Dashboard Server Port")
	flag.StringVar(&cfg.OTLPEndpoint, "otlp-endpoint", cfg.OTLPEndpoint, "OpenTelemetry gRPC Collector Endpoint")
	flag.StringVar(&cfg.EngineMode, "engine", cfg.EngineMode, "Engine mode: auto | ebpf | packet | demo")
	flag.BoolVar(&cfg.DemoMode, "demo", cfg.DemoMode, "Enable synthetic traffic generator demo")
	flag.Parse()

	return cfg
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return defaultVal
}
