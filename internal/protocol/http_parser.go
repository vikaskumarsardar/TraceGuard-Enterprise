package protocol

import (
	"bufio"
	"bytes"
	"net/http"
	"strings"
	"time"

	"traceguard/internal/domain"

	"github.com/google/uuid"
)

// HTTPParser decodes HTTP/1.1 and HTTP/2 requests and responses from raw socket bytes.
type HTTPParser struct{}

func NewHTTPParser() *HTTPParser {
	return &HTTPParser{}
}

// ParseHTTPRequest parses raw byte payload into a domain Span.
func (p *HTTPParser) ParseHTTPRequest(payload []byte, clientIP, serverIP string) (*domain.Span, error) {
	reader := bufio.NewReader(bytes.NewReader(payload))
	req, err := http.ReadRequest(reader)
	if err != nil {
		return nil, err
	}

	correlationID := req.Header.Get("x-correlation-id")
	if correlationID == "" {
		correlationID = req.Header.Get("x-request-id")
	}

	traceID := ""
	parentSpanID := ""

	// Parse W3C traceparent header: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
	traceparent := req.Header.Get("traceparent")
	if traceparent != "" {
		parts := strings.Split(traceparent, "-")
		if len(parts) >= 3 {
			traceID = parts[1]
			parentSpanID = parts[2]
		}
	}

	if traceID == "" {
		if correlationID != "" {
			traceID = correlationID
		} else {
			traceID = uuid.New().String()
			correlationID = traceID
		}
	}

	serviceName := req.Header.Get("x-service-name")
	if serviceName == "" {
		serviceName = extractServiceNameFromHost(req.Host, serverIP)
	}

	span := &domain.Span{
		ID:            uuid.New().String(),
		TraceID:       traceID,
		ParentSpanID:  parentSpanID,
		CorrelationID: correlationID,
		ServiceName:   serviceName,
		Protocol:      domain.ProtocolHTTP,
		Name:          req.Method + " " + req.URL.Path,
		Method:        req.Method,
		Path:          req.URL.Path,
		StatusCode:    200,
		StartTime:     time.Now(),
		Status:        domain.StatusOk,
		ClientIP:      clientIP,
		ServerIP:      serverIP,
		Attributes: map[string]string{
			"http.host":       req.Host,
			"http.user_agent": req.UserAgent(),
		},
	}

	return span, nil
}

func extractServiceNameFromHost(host, ip string) string {
	if host != "" {
		parts := strings.Split(host, ":")
		return parts[0]
	}
	return "service-" + ip
}
